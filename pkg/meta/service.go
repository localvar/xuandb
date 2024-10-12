package meta

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb/v2"
	"github.com/localvar/xuandb/pkg/config"
	"github.com/localvar/xuandb/pkg/logger"
	"github.com/localvar/xuandb/pkg/utils"
)

// service represents the meta service.
var service struct {
	raft     *raft.Raft
	metadata *Data
	nodes    map[string]*NodeInfo
	stop     chan struct{}
	wg       sync.WaitGroup
}

// joinRequest is the request to join a raft cluster.
type joinRequest struct {
	ClusterName string   `json:"clusterName"`
	ID          string   `json:"id"`
	Addr        string   `json:"addr"`
	Roles       []string `json:"roles"`
}

// join joins the current node to the raft cluster via addr.
func join(addr string) error {
	nc := config.CurrentNode()

	jr := &joinRequest{
		ClusterName: config.ClusterName(),
		ID:          config.NodeID(),
		Addr:        nc.Meta.RaftAddr,
	}
	if nc.Meta.RaftVoter {
		jr.Roles = append(jr.Roles, "meta")
	}
	if nc.Data != nil {
		jr.Roles = append(jr.Roles, "data")
	}
	if nc.Query != nil {
		jr.Roles = append(jr.Roles, "query")
	}

	body, err := json.Marshal(jr)
	if err != nil {
		slog.Error(
			"failed to marshal join request",
			slog.String("error", err.Error()),
		)
		return err
	}

	urlJoin := "http://" + addr + "/meta/node"
	resp, err := http.Post(urlJoin, "application/json", bytes.NewReader(body))
	if err != nil {
		slog.Error(
			"failed to join cluster",
			slog.String("leaderAddr", addr),
			slog.String("error", err.Error()),
		)
		return err
	}

	defer func() {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()

	if resp.StatusCode < 300 {
		slog.Info("join cluster succeeded", slog.String("leaderAddr", addr))
		return nil
	}

	err = utils.FromHTTPResponse(resp)
	slog.Error(
		"failed to join cluster",
		slog.String("leaderAddr", addr),
		slog.Int("httpStatus", resp.StatusCode),
		slog.String("message", err.Error()),
	)
	return err
}

// handleAddNode handles the add node, i.e. join request.
func handleAddNode(w http.ResponseWriter, r *http.Request) {
	var jr joinRequest
	err := json.NewDecoder(r.Body).Decode(&jr)
	if err != nil {
		http.Error(w, "failed to decode join request", http.StatusBadRequest)
		return
	}
	if jr.ID == "" || jr.Addr == "" {
		http.Error(w, "invalid join request", http.StatusBadRequest)
		return
	}
	if jr.ClusterName != config.ClusterName() {
		http.Error(w, "wrong cluster name", http.StatusForbidden)
		return
	}

	slog.Info(
		"add node request received",
		slog.String("nodeId", jr.ID),
		slog.String("nodeAddr", jr.Addr),
	)

	ra := service.raft
	if ra.State() != raft.Leader {
		slog.Info("refuse due to not leader", slog.String("nodeId", jr.ID))
		http.Error(w, "not leader", http.StatusServiceUnavailable)
		return
	}

	id, addr := raft.ServerID(jr.ID), raft.ServerAddress(jr.Addr)
	if slices.Contains(jr.Roles, "meta") {
		err = ra.AddVoter(id, addr, 0, 0).Error()
	} else {
		err = ra.AddNonvoter(id, addr, 0, 0).Error()
	}

	if err == nil {
		slog.Info("node added", slog.String("nodeId", jr.ID))
		return
	}

	slog.Error(
		"failed to add node",
		slog.String("nodeId", jr.ID),
		slog.String("error", err.Error()),
	)

	http.Error(w, err.Error(), http.StatusInternalServerError)
}

// handleRemoveNode handles the remove node request.
func handleRemoveNode(w http.ResponseWriter, r *http.Request) {
	id := r.FormValue("id")
	if id == "" {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	slog.Info("remove node request received", slog.String("nodeId", id))

	ra := service.raft
	if ra.State() != raft.Leader {
		slog.Info("cannot remove node due to not leader", slog.String("nodeId", id))
		http.Error(w, "not leader", http.StatusServiceUnavailable)
		return
	}

	err := service.raft.RemoveServer(raft.ServerID(id), 0, 0).Error()
	if err == nil {
		slog.Info("node removed", slog.String("nodeId", id))
		return
	}

	slog.Error(
		"failed to remove node",
		slog.String("nodeId", id),
		slog.String("error", err.Error()),
	)

	http.Error(w, err.Error(), http.StatusInternalServerError)
}

// isLeader returns whether the current node is the leader.
func isLeader() bool {
	if service.raft == nil {
		return false
	}
	return service.raft.State() == raft.Leader
}

// createRaftSnapshotStore creates a raft snapshot store.
func createRaftSnapshotStore(mc *config.MetaConfig) (raft.SnapshotStore, error) {
	switch mc.RaftSnapshotStore {
	case "discard":
		return raft.NewDiscardSnapshotStore(), nil
	case "memory":
		return raft.NewInmemSnapshotStore(), nil
	case "file":
		return raft.NewFileSnapshotStoreWithLogger(mc.DataDir, 1, logger.HashiCorp(nil))
	default:
		panic("should not reach here")
	}
}

// createRaftStore creates a raft store.
func createRaftStore(mc *config.MetaConfig) (raft.LogStore, raft.StableStore, error) {
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

// construct creates raft and its dependencies objects.
func construct() (bool, error) {
	mc := config.CurrentNode().Meta

	logger := logger.HashiCorp(nil)
	trans, err := raft.NewTCPTransportWithLogger(mc.RaftAddr, nil, 3, 10*time.Second, logger)
	if err != nil {
		slog.Error("failed to create tcp transport", slog.String("error", err.Error()))
		return false, err
	}

	snapshot, err := createRaftSnapshotStore(mc)
	if err != nil {
		slog.Error("failed to create snapshot store", slog.String("error", err.Error()))
		return false, err
	}

	ls, ss, err := createRaftStore(mc)
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
	service.metadata = newData()

	ra, err := raft.NewRaft(cfg, service.metadata, ls, ss, snapshot, trans)
	if err != nil {
		slog.Error("failed to create raft", slog.String("error", err.Error()))
		return false, err
	}
	service.raft = ra

	return hasState, nil
}

func updateNodeStatus() {
	defer service.wg.Done()

	t := time.NewTicker(1 * time.Second)
	defer t.Stop()

	for {
		select {
		case <-service.stop:
			return
		case <-t.C:
		}

		if isLeader() {
		} else {
		}
	}
}

// StartService starts the meta service.
func StartService() error {
	hasState, err := construct()
	if err != nil {
		return err
	}

	// from now on, we don't return an error, but the APIs should be available
	// only after we tried to join or bootstrap the cluster.
	defer registerAPIs()

	// already has existing state, no need to join or bootstrap.
	if hasState {
		return nil
	}

	// try join first, but collect nodes info for bootstrap at the same time.
	svrs := make([]raft.Server, 0, len(config.Nodes()))
	for _, nc := range config.Nodes() {
		svr := raft.Server{
			ID:      raft.ServerID(nc.ID),
			Address: raft.ServerAddress(nc.Meta.RaftAddr),
		}
		if !nc.Meta.RaftVoter {
			svr.Suffrage = raft.Nonvoter
		}
		svrs = append(svrs, svr)

		if svr.Suffrage == raft.Nonvoter || nc.ID == config.NodeID() {
			continue
		}

		if join(nc.HTTPAddr) == nil {
			return nil
		}
	}

	// non-voter never bootstraps a cluster, so return and wait the leader to
	// add this node.
	if !config.CurrentNode().Meta.RaftVoter {
		slog.Error("failed to join an existing cluster")
		return nil
	}

	// try bootstrap, note it is ok for 2 or more nodes to bootstrap,
	// and if bootstrap fails, just wait for the leader to add this node.
	slog.Info("cannot join an existing cluster, trying to bootstrap")
	err = service.raft.BootstrapCluster(raft.Configuration{Servers: svrs}).Error()
	if err == nil {
		slog.Info("meta service bootstrapped")
	} else {
		slog.Error("failed to bootstrap cluster", slog.String("error", err.Error()))
	}

	return nil
}

// ShutdownService shuts down the meta service.
func ShutdownService() {
	if service.raft == nil {
		return
	}

	close(service.stop)

	if err := service.raft.Shutdown().Error(); err != nil {
		slog.Error("failed to shutdown raft", slog.String("error", err.Error()))
	}

	service.wg.Wait()
	slog.Info("meta service stopped")
}
