package meta

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"path/filepath"
	"time"

	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb/v2"
	"github.com/localvar/xuandb/pkg/config"
	"github.com/localvar/xuandb/pkg/httpserver"
	"github.com/localvar/xuandb/pkg/logger"
)

// service represents the meta service.
var service struct {
	raft *raft.Raft
	data *Data
}

// joinCluster joins the current node to the raft cluster specified by addr.
func joinCluster(addr string) error {
	mc := config.CurrentNode().Meta

	query := url.Values{"id": {config.NodeID()}, "addr": {mc.RaftAddr}}
	if mc.RaftVoter {
		query.Set("voter", "true")
	}

	urlJoin := "http://" + addr + "/meta/node?" + query.Encode()
	resp, err := http.Post(urlJoin, "application/json", nil)
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

	slog.Error(
		"failed to join cluster",
		slog.String("leaderAddr", addr),
		slog.Int("httpStatus", resp.StatusCode),
	)
	return raft.ErrNotLeader
}

// handleAddNode handles the add node, i.e. join request.
func handleAddNode(w http.ResponseWriter, r *http.Request) {
	id, addr := r.FormValue("id"), r.FormValue("addr")
	if id == "" || addr == "" {
		http.Error(w, "invalid join request", http.StatusBadRequest)
		return
	}

	slog.Info(
		"add node request received",
		slog.String("nodeId", id),
		slog.String("nodeAddr", addr),
	)

	ra := service.raft
	if ra.State() != raft.Leader {
		slog.Info("cannot add node due to not leader", slog.String("nodeId", id))
		http.Error(w, "not leader", http.StatusServiceUnavailable)
		return
	}

	var err error
	if r.FormValue("voter") == "true" {
		err = ra.AddVoter(raft.ServerID(id), raft.ServerAddress(addr), 0, 0).Error()
	} else {
		err = ra.AddNonvoter(raft.ServerID(id), raft.ServerAddress(addr), 0, 0).Error()
	}

	if err == nil {
		slog.Info("node added", slog.String("nodeId", id))
		return
	}

	slog.Error(
		"failed to add node",
		slog.String("nodeId", id),
		slog.String("error", err.Error()),
	)

	http.Error(w, err.Error(), http.StatusInternalServerError)
}

// handleListNode handles the list node request.
func handleListNode(w http.ResponseWriter, _ *http.Request) {
	ra := service.raft

	_, leaderID := ra.LeaderWithID()
	future := ra.GetConfiguration()
	if err := future.Error(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	svrs := future.Configuration().Servers

	result := make([]struct {
		ID       string `json:"id"`
		Addr     string `json:"addr"`
		IsLeader bool   `json:"isLeader"`
	}, len(svrs))

	for i, svr := range svrs {
		result[i].ID = string(svr.ID)
		result[i].Addr = string(svr.Address)
		result[i].IsLeader = svr.ID == leaderID
	}

	json.NewEncoder(w).Encode(result)
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

// StartService starts the meta data service.
func StartService() error {
	mc := config.CurrentNode().Meta

	// create raft and its dependencies objects.
	logger := logger.HashiCorp(nil)
	trans, err := raft.NewTCPTransportWithLogger(mc.RaftAddr, nil, 3, 10*time.Second, logger)
	if err != nil {
		slog.Error("failed to create tcp transport", slog.String("error", err.Error()))
		return err
	}

	snapshot, err := createRaftSnapshotStore(mc)
	if err != nil {
		slog.Error("failed to create snapshot store", slog.String("error", err.Error()))
		return err
	}

	ls, ss, err := createRaftStore(mc)
	if err != nil {
		slog.Error("failed to create raft store", slog.String("error", err.Error()))
		return err
	}

	hasState, err := raft.HasExistingState(ls, ss, snapshot)
	if err != nil {
		slog.Error("failed to check existing state", slog.String("error", err.Error()))
		return err
	}

	cfg := raft.DefaultConfig()
	cfg.LocalID = raft.ServerID(config.NodeID())
	cfg.Logger = logger
	service.data = newData()

	ra, err := raft.NewRaft(cfg, service.data, ls, ss, snapshot, trans)
	if err != nil {
		slog.Error("failed to create raft", slog.String("error", err.Error()))
		return err
	}
	service.raft = ra

	// only voter nodes expose APIs
	if mc.RaftVoter {
		defer func() {
			// node management APIs
			httpserver.HandleFunc("GET /meta/node", handleListNode)
			httpserver.HandleFunc("POST /meta/node", handleAddNode)
			httpserver.HandleFunc("DELETE /meta/node", handleRemoveNode)
			registerDataAPIs()
		}()
	}

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

		if joinCluster(nc.HTTPAddr) == nil {
			return nil
		}
	}

	// non-voter never bootstraps a cluster, so return and wait the leader to
	// add this node.
	if !mc.RaftVoter {
		slog.Info("cannot join an existing cluster")
		return nil
	}

	// try bootstrap, note it is ok for 2 or more nodes to bootstrap,
	// and if bootstrap fails, just wait for the leader to add this node.
	slog.Info("cannot join an existing cluster, trying to bootstrap")
	err = ra.BootstrapCluster(raft.Configuration{Servers: svrs}).Error()
	if err == nil {
		slog.Info("meta service bootstrapped")
	} else {
		slog.Error("failed to bootstrap cluster", slog.String("error", err.Error()))
	}

	return nil
}

// ShutdownService shuts down the meta data service.
func ShutdownService() {
	if service.raft == nil {
		return
	}

	if err := service.raft.Shutdown().Error(); err != nil {
		slog.Error("failed to shutdown raft", slog.String("error", err.Error()))
	}

	slog.Info("meta service stopped")
}
