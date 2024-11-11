package meta

import (
	"log/slog"
	"path/filepath"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb/v2"
	"github.com/localvar/xuandb/pkg/config"
	"github.com/localvar/xuandb/pkg/logger"
)

// service represents the meta service.
type service struct {
	raft *raft.Raft

	md *Data // metadata

	nodesLock sync.Mutex
	nodes     map[string]*NodeInfo

	stop chan struct{}
	wg   sync.WaitGroup
}

// newService creates a new meta service instance.
func newService() *service {
	svc := &service{}
	svc.md = newData()
	svc.nodes = make(map[string]*NodeInfo)
	svc.stop = make(chan struct{})
	return svc
}

func (s *service) lockNodes() {
	s.nodesLock.Lock()
}

func (s *service) unlockNodes() {
	s.nodesLock.Unlock()
}

// isLeader returns whether the current node is the leader.
func (s *service) isLeader() bool {
	if s.raft == nil {
		return false
	}
	return s.raft.State() == raft.Leader
}

// createRaftSnapshotStore creates a raft snapshot store according to the
// configuration.
func createRaftSnapshotStore(logger hclog.Logger) (raft.SnapshotStore, error) {
	mc := config.CurrentNode().Meta
	switch mc.RaftSnapshotStore {
	case "discard":
		return raft.NewDiscardSnapshotStore(), nil
	case "memory":
		return raft.NewInmemSnapshotStore(), nil
	case "file":
		return raft.NewFileSnapshotStoreWithLogger(mc.DataDir, 1, logger)
	default:
		panic("should not reach here")
	}
}

// createRaftStore creates a raft store as the log store and stable store
// according to the configuration.
func createRaftStore() (raft.LogStore, raft.StableStore, error) {
	mc := config.CurrentNode().Meta
	switch mc.RaftStore {
	case "memory":
		db := raft.NewInmemStore()
		return db, db, nil
	case "boltdb":
		opt := raftboltdb.Options{Path: filepath.Join(mc.DataDir, "raft.db")}
		db, err := raftboltdb.New(opt)
		if err != nil {
			return nil, nil, err
		}
		return db, db, nil
	default:
		panic("should not reach here")
	}
}

// start start meta service by creating raft and its dependencies.
func (s *service) start() (bool, error) {
	mc := config.CurrentNode().Meta

	logger := logger.HashiCorp(nil)
	trans, err := raft.NewTCPTransportWithLogger(mc.RaftAddr, nil, 3, 10*time.Second, logger)
	if err != nil {
		slog.Error("failed to create tcp transport", slog.String("error", err.Error()))
		return false, err
	}

	snapshot, err := createRaftSnapshotStore(logger)
	if err != nil {
		slog.Error("failed to create snapshot store", slog.String("error", err.Error()))
		return false, err
	}

	ls, ss, err := createRaftStore()
	if err != nil {
		slog.Error("failed to create raft store", slog.String("error", err.Error()))
		return false, err
	}

	hasState, err := raft.HasExistingState(ls, ss, snapshot)
	if err != nil {
		slog.Error("failed to check existing state", slog.String("error", err.Error()))
		return false, err
	}

	cfg := raft.DefaultConfig()
	cfg.LocalID = raft.ServerID(config.NodeID())
	cfg.Logger = logger

	ra, err := raft.NewRaft(cfg, s, ls, ss, snapshot, trans)
	if err != nil {
		slog.Error("failed to create raft", slog.String("error", err.Error()))
		return false, err
	}
	s.raft = ra

	return hasState, nil
}

// joinOrBootstrap tries to join an existing cluster or bootstrap a new cluster.
func (s *service) joinOrBootstrap() {
	// try join first, but collect nodes info for bootstrap at the same time.
	svrs := make([]raft.Server, 0, len(config.Nodes()))
	for _, nc := range config.Nodes() {
		addr := nc.ToExternalAddress(nc.Meta.RaftAddr)
		svr := raft.Server{
			ID:      raft.ServerID(nc.ID),
			Address: raft.ServerAddress(addr),
		}
		if !nc.Meta.RaftVoter {
			svr.Suffrage = raft.Nonvoter
		}
		svrs = append(svrs, svr)

		if svr.Suffrage == raft.Nonvoter || nc.ID == config.NodeID() {
			continue
		}

		addr = nc.ToExternalAddress(nc.HTTPAddr)
		if join(addr) == nil {
			return
		}
	}

	// non-voter never bootstraps a cluster, so return and wait the leader to
	// add this node.
	if !config.CurrentNode().Meta.RaftVoter {
		slog.Error("failed to join an existing cluster")
		return
	}

	// try bootstrap, note it is ok for 2 or more nodes to bootstrap,
	// and if bootstrap fails, just wait for the leader to add this node.
	slog.Info("cannot join an existing cluster, trying to bootstrap")
	err := s.raft.BootstrapCluster(raft.Configuration{Servers: svrs}).Error()
	if err == nil {
		slog.Info("meta service bootstrapped")
	} else {
		slog.Error("failed to bootstrap cluster", slog.String("error", err.Error()))
	}
}

// shutdown shuts down the meta service.
func (s *service) shutdown() {
	close(s.stop)

	if err := s.raft.Shutdown().Error(); err != nil {
		slog.Error("failed to shutdown raft", slog.String("error", err.Error()))
	}

	s.wg.Wait()

	slog.Info("meta service stopped")
}

// serInst is the singleton of the meta service.
var svcInst *service

// StartService starts the meta service.
func StartService() error {
	inst := newService()

	hasState, err := inst.start()
	if err != nil {
		return err
	}

	svcInst = inst
	if !hasState {
		svcInst.joinOrBootstrap()
	}

	registerNodeHandlers()
	registerUserHandlers()
	registerDatabaseHandlers()

	svcInst.updateNodeInfo()
	return nil
}

// ShutdownService shuts down the meta service.
func ShutdownService() {
	if svcInst != nil {
		svcInst.shutdown()
		svcInst = nil
	}
}
