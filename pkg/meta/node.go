package meta

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"time"

	"github.com/hashicorp/raft"
	"github.com/localvar/xuandb/pkg/config"
	"github.com/localvar/xuandb/pkg/httpserver"
	"github.com/localvar/xuandb/pkg/xerrors"
)

// registerNodeAPIHandlers registers the node API handlers.
func registerNodeAPIHandlers() {
	httpserver.HandleFunc("POST /meta/nodes", handleAddNode)
	httpserver.HandleFunc("DELETE /meta/nodes", handleDropNode)
	httpserver.HandleFunc("POST /meta/node/heartbeat", handleNodeHeartbeat)
}

// joinRequest is the request to join a raft cluster.
type joinRequest struct {
	ClusterName string `json:"clusterName"`
	ID          string `json:"id"`
	Addr        string `json:"addr"`
	Voter       bool   `json:"voter"`
}

// join joins the current node to the raft cluster via addr.
func join(addr string) error {
	mc := config.CurrentNode().Meta
	jr, _ := json.Marshal(&joinRequest{
		ClusterName: config.ClusterName(),
		ID:          config.NodeID(),
		Addr:        mc.RaftAddr,
		Voter:       mc.RaftVoter,
	})

	urlJoin := "http://" + addr + "/meta/nodes"
	resp, err := http.Post(urlJoin, "application/json", bytes.NewReader(jr))
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

	err = xerrors.FromHTTPResponse(resp)
	slog.Error(
		"failed to join cluster",
		slog.String("leaderAddr", addr),
		slog.Int("httpStatus", resp.StatusCode),
		slog.String("message", err.Error()),
	)
	return err
}

func leaderAddNode(id, addr string, voter bool) error {
	var err error

	sid, saddr := raft.ServerID(id), raft.ServerAddress(addr)
	if voter {
		err = svcInst.raft.AddVoter(sid, saddr, 0, 0).Error()
	} else {
		err = svcInst.raft.AddNonvoter(sid, saddr, 0, 0).Error()
	}

	if err == nil {
		slog.Info("node added", slog.String("nodeId", id))
		return nil
	}

	slog.Debug(
		"failed to add node",
		slog.String("nodeId", id),
		slog.String("error", err.Error()),
	)

	return xerrors.Wrap(err, http.StatusInternalServerError)
}

// handleAddNode handles the add node (i.e. join) request.
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

	slog.Debug(
		"add node request received",
		slog.String("nodeId", jr.ID),
		slog.String("nodeAddr", jr.Addr),
	)

	if !svcInst.isLeader() {
		slog.Debug("refuse due to not leader", slog.String("nodeId", jr.ID))
		http.Error(w, "not leader", http.StatusServiceUnavailable)
		return
	}

	if err = leaderAddNode(jr.ID, jr.Addr, jr.Voter); err != nil {
		se := err.(*xerrors.StatusError)
		http.Error(w, se.Msg, se.StatusCode)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func leaderDropNode(id string) error {
	err := svcInst.raft.RemoveServer(raft.ServerID(id), 0, 0).Error()
	if err == nil {
		slog.Info("node dropped", slog.String("nodeId", id))
		return nil
	}

	slog.Debug(
		"failed to drop node",
		slog.String("nodeId", id),
		slog.String("error", err.Error()),
	)
	return xerrors.Wrap(err, http.StatusInternalServerError)
}

// handleDropNode handles the drop node request.
func handleDropNode(w http.ResponseWriter, r *http.Request) {
	id := r.FormValue("id")
	if id == "" {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	slog.Debug("drop node request received", slog.String("nodeId", id))

	s := svcInst
	if !s.isLeader() {
		slog.Debug("refuse due to not leader", slog.String("nodeId", id))
		http.Error(w, "not leader", http.StatusServiceUnavailable)
		return
	}

	if err := leaderDropNode(id); err != nil {
		se := err.(*xerrors.StatusError)
		http.Error(w, se.Msg, se.StatusCode)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// NodeRole represents the role of a node in the cluster.
type NodeRole int

const (
	NodeRoleMeta = 1 << iota
	NodeRoleData
	NodeRoleQuery
)

func (r NodeRole) String() string {
	var s string
	if r&NodeRoleMeta != 0 {
		s = "meta"
	}
	if r&NodeRoleData != 0 {
		if len(s) > 0 {
			s += ",data"
		} else {
			s = "data"
		}
	}
	if r&NodeRoleQuery != 0 {
		if len(s) > 0 {
			s += ",query"
		} else {
			s = "query"
		}
	}

	return s
}

// NodeInfo is the runtime information of a node.
type NodeInfo struct {
	ID                string    `json:"id"`
	Addr              string    `json:"addr"` // HTTP address of the node
	Role              NodeRole  `json:"role"`
	LastHeartbeatTime time.Time `json:"lastHeartbeatTime"`
}

// init initializes the NodeInfo according to configuration of the current node.
func (ni *NodeInfo) init() {
	nc := config.CurrentNode()
	ni.ID = nc.ID
	ni.Addr = nc.HTTPAddr

	if nc.Meta.RaftVoter {
		ni.Role |= NodeRoleMeta
	}
	if nc.Data != nil {
		ni.Role |= NodeRoleData
	}
	if nc.Query != nil {
		ni.Role |= NodeRoleQuery
	}
}

// clone returns a copy of the NodeInfo.
func (ni *NodeInfo) clone() *NodeInfo {
	ni1 := *ni
	return &ni1
}

// NodeStatus extends NodeInfo by adding several status fields which will only
// be set as the response of the node status API.
type NodeStatus struct {
	NodeInfo
	Leader bool   `json:"leader"`
	State  string `json:"state"`
}

// sendHeartbeatToLeader sends a heartbeat of the current node to the leader.
func (s *service) sendHeartbeatToLeader(ni *NodeInfo) {
	err := sendPostRequestToLeader("/meta/node/heartbeat", ni)
	if err != nil {
		slog.Error(
			"failed to send heartbeat to leader",
			slog.String("error", err.Error()),
		)
		return
	}
	slog.Debug("send heartbeat to leader succeeded")
}

// handleNodeHeartbeat handles a node heartbeat message.
func handleNodeHeartbeat(w http.ResponseWriter, r *http.Request) {
	var hb NodeInfo
	if err := json.NewDecoder(r.Body).Decode(&hb); err != nil {
		slog.Debug(
			"failed to decode node info",
			slog.String("error", err.Error()),
		)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if time.Until(hb.LastHeartbeatTime) > 10*time.Second {
		const msg = "heartbeat time is in the distant future"
		slog.Debug(msg, slog.String("nodeID", hb.ID))
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	slog.Debug("node heartbeat received", slog.String("nodeID", hb.ID))

	s := svcInst
	s.lockNodes()

	// save the node status only if it is already in the map.
	ni := s.nodes[hb.ID]
	if ni != nil {
		*ni = hb
	}

	s.unlockNodes()

	if ni == nil {
		slog.Debug("node does not exist", slog.String("nodeID", hb.ID))
	}
}

// updateNodeListCommand is the raft FSM command to update node list.
type updateNodeListCommand struct {
	baseCommand
	Nodes map[string]*NodeInfo `json:"nodes"`
}

// applyUpdateNodeList applies a updateNodeListCommand to current node.
func applyUpdateNodeList(l *raft.Log) any {
	var cmd updateNodeListCommand
	if err := json.Unmarshal(l.Data, &cmd); err != nil {
		slog.Error(
			"failed to unmarshal update node list command",
			slog.String("error", err.Error()),
		)
		return err
	}

	s := svcInst

	s.lockNodes()

	// remove nodes that are not in the command.
	for id := range s.nodes {
		if _, ok := cmd.Nodes[id]; !ok {
			delete(s.nodes, id)
		}
	}

	// update node info only if the new info is newer.
	for id, ni := range cmd.Nodes {
		ni1 := s.nodes[id]
		if ni1 == nil || ni1.LastHeartbeatTime.Before(ni.LastHeartbeatTime) {
			s.nodes[id] = ni
		}
	}

	s.unlockNodes()

	slog.Debug("node list updated")
	return nil
}

// sendNodeListToFollower sends the info of all nodes to all servers of the
// raft cluster via an updateNodeListCommand.
func (s *service) sendNodeListToFollower() {
	fGet := s.raft.GetConfiguration()
	if err := fGet.Error(); err != nil {
		slog.Error(
			"failed to get raft configuration",
			slog.String("error", err.Error()),
		)
		return
	}

	svrs := fGet.Configuration().Servers
	cmd := &updateNodeListCommand{
		baseCommand: baseCommand{Op: opUpdateNodeList},
		Nodes:       make(map[string]*NodeInfo, len(svrs)),
	}

	s.lockNodes()
	for _, svr := range svrs {
		id := string(svr.ID)
		if ni := s.nodes[id]; ni != nil {
			cmd.Nodes[id] = ni
		} else {
			cmd.Nodes[id] = &NodeInfo{ID: id}
		}
	}

	// must marshal before unlock, because we are reusing the NodeInfos.
	data, err := json.Marshal(cmd)
	s.unlockNodes()

	if err != nil {
		slog.Error(
			"failed to marshal update node list command",
			slog.String("error", err.Error()),
		)
		return
	}

	fApply := s.raft.Apply(data, 0)
	if err := fApply.Error(); err != nil {
		slog.Error(
			"failed to apply update node list command",
			slog.String("error", err.Error()),
		)
		return
	}

	if resp := fApply.Response(); resp != nil {
		if err, ok := resp.(error); ok {
			slog.Error(
				"apply update node list command got error response",
				slog.String("error", err.Error()),
			)
		}
		return
	}

	slog.Debug("send update node list command to follower succeeded")
}

// updateNodeInfo updates the info of the current node periodically.
// If this node is a follower, it will also send its info to the leader as a
// heartbeat; if this node is the leader, it will send its node list to all
// servers of the raft cluster periodically.
func (s *service) updateNodeInfo() {
	s.wg.Add(1)

	go func() {
		defer s.wg.Done()

		t := time.NewTicker(1 * time.Second)
		defer t.Stop()

		ni := &NodeInfo{}
		ni.init()

		for ticks := uint(0); ; ticks++ {
			select {
			case <-s.stop:
				return
			case <-t.C:
			}

			ni.LastHeartbeatTime = time.Now()

			// update current node info locally.
			s.lockNodes()
			if ni1 := s.nodes[ni.ID]; ni1 != nil {
				ni1.LastHeartbeatTime = ni.LastHeartbeatTime
			} else {
				s.nodes[ni.ID] = ni.clone()
			}
			s.unlockNodes()

			if !s.isLeader() {
				s.sendHeartbeatToLeader(ni)
			} else if ticks%5 == 0 {
				s.sendNodeListToFollower()
			}
		}
	}()
}

// AddNode add a new node into the cluster.
func AddNode(id, addr string, voter bool) error {
	if svcInst.isLeader() {
		return leaderAddNode(id, addr, voter)
	}

	jr := joinRequest{
		ClusterName: config.ClusterName(),
		ID:          id,
		Addr:        addr,
		Voter:       voter,
	}
	return sendPostRequestToLeader("/meta/nodes", jr)
}

// DropNode removes the current node from the cluster.
func DropNode(id string) error {
	if svcInst.isLeader() {
		leaderDropNode(id)
	}
	return sendDeleteRequestToLeader("/meta/nodes?id=" + url.QueryEscape(id))
}

// Nodes returns a list of all nodes in the cluster.
func Nodes() []NodeInfo {
	result := make([]NodeInfo, 0, len(config.Nodes()))

	svcInst.lockNodes()
	for _, ni := range svcInst.nodes {
		result = append(result, *ni)
	}
	svcInst.unlockNodes()

	// sort the result by node ID.
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})

	return result
}

// NodeByID returns the node info by ID. It returns nil if not found.
func NodeByID(id string) *NodeInfo {
	svcInst.lockNodes()
	defer svcInst.unlockNodes()
	if ni := svcInst.nodes[id]; ni != nil {
		return ni.clone()
	}
	return nil
}

// LeaderNode returns the info of the leader node.
// It returns nil if there's no leader.
func LeaderNode() *NodeInfo {
	_, id := svcInst.raft.LeaderWithID()
	if id == "" {
		return nil
	}
	return NodeByID(string(id))
}

// NodeHTTPAddr returns the HTTP address of the node by ID.
// It returns an empty string if not found.
func NodeHTTPAddr(id string) string {
	svcInst.lockNodes()
	defer svcInst.unlockNodes()
	if ni := svcInst.nodes[id]; ni != nil {
		return ni.Addr
	}
	return ""
}

// LeaderHTTPAddr returns the HTTP address of the leader node.
// It returns an empty string if there's no leader.
func LeaderHTTPAddr() string {
	_, id := svcInst.raft.LeaderWithID()
	if id == "" {
		return ""
	}
	return NodeHTTPAddr(string(id))
}

// NodeStatuses returns the status of all nodes in the cluster.
func NodeStatuses() []NodeStatus {
	result := make([]NodeStatus, 0, len(config.Nodes()))

	svcInst.lockNodes()
	for _, ni := range svcInst.nodes {
		result = append(result, NodeStatus{NodeInfo: *ni})
	}
	svcInst.unlockNodes()

	// sort the result by node ID.
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})

	_, leaderID := svcInst.raft.LeaderWithID()
	now := time.Now()
	for i := 0; i < len(result); i++ {
		ns := &result[i]
		ns.Leader = ns.ID == string(leaderID)
		if d := now.Sub(ns.LastHeartbeatTime); d >= 30*time.Second {
			ns.State = "dead"
		} else if d >= 10*time.Second {
			ns.State = "unknown"
		} else {
			ns.State = "alive"
		}
	}

	return result
}
