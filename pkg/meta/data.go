package meta

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/hashicorp/raft"
	"github.com/localvar/xuandb/pkg/config"
	"github.com/localvar/xuandb/pkg/httpserver"
	"github.com/localvar/xuandb/pkg/utils"
)

// Data is the meta data that managed by the meta service.
type Data struct {
	lock  sync.Mutex
	Users map[string]*User
	Nodes map[string]*NodeInfo
}

// newData creates a new Data.
func newData() *Data {
	return &Data{
		Users: map[string]*User{},
		Nodes: map[string]*NodeInfo{},
	}
}

// List of data operations, for each operation, there are two handlers, one
// for handling client requests, with name like "handleXXXXXX", the other for
// applying the operation, with name like "applyXXXXXX".
const (
	opAddUser = iota + 1
	opRemoveUser
	opSetPassword
	opLast
)

// registerDataAPIs registers the client request handlers.
func registerDataAPIs() {
	httpserver.HandleFunc("POST /meta/user", handleAddUser)
	httpserver.HandleFunc("DELETE /meta/user", handleRemoveUser)
}

// dataApplyFuncs is the list of functions to apply data operations.
var dataApplyFuncs = [opLast]func(*Data, *raft.Log) any{
	opAddUser:     applyAddUser,
	opRemoveUser:  applyRemoveUser,
	opSetPassword: applySetPassword,
}

// baseCommand is the base of all data operation commands.
type baseCommand struct {
	Op uint32 `json:"op"`
}

// raftApply is a helper function to apply a command to the Raft log.
func raftApply(w http.ResponseWriter, data []byte) {
	future := service.raft.Apply(data, 0)

	if err := future.Error(); err != nil {
		// return a hint of the leader address to the client.
		addr, _ := service.raft.LeaderWithID()
		w.Header().Set(LeaderHintHeader, string(addr))

		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if resp := future.Response(); resp != nil {
		if se, ok := resp.(*utils.StatusError); ok {
			http.Error(w, se.Msg, se.Code)
		} else if err, ok := resp.(error); ok {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		} else {
			json.NewEncoder(w).Encode(resp)
		}
	}
}

// handlers for the add user command.
type addUserCommand struct {
	baseCommand
	User
}

func handleAddUser(w http.ResponseWriter, r *http.Request) {
	cmd := addUserCommand{baseCommand: baseCommand{Op: opAddUser}}

	err := json.NewDecoder(r.Body).Decode(&cmd.User)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if cmd.Name == "" || cmd.Password == "" {
		http.Error(w, "name and password are required", http.StatusBadRequest)
		return
	}

	// check if the user already exists for the 1st time.
	service.metadata.lock.Lock()
	u := service.metadata.Users[strings.ToLower(cmd.Name)]
	service.metadata.lock.Unlock()
	if u != nil {
		msg := fmt.Sprintf("user already exists: %s", cmd.Name)
		http.Error(w, msg, http.StatusConflict)
		return
	}

	v, err := json.Marshal(&cmd)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	slog.Info("add user command received", slog.String("name", cmd.Name))
	raftApply(w, v)
}

func applyAddUser(d *Data, l *raft.Log) any {
	var cmd addUserCommand
	if err := json.Unmarshal(l.Data, &cmd); err != nil {
		return err
	}

	// though we have made a check at handleAddUser, it is still possible that
	// the user already exists because of the Raft 2 phase commit, so we need
	// to check again here.
	key := strings.ToLower(cmd.Name)
	d.lock.Lock()
	u := d.Users[key]
	if u == nil {
		d.Users[key] = &cmd.User
	}
	d.lock.Unlock()

	if u == nil {
		slog.Info("user added", slog.String("name", cmd.Name))
		return nil
	}

	return &utils.StatusError{
		Code: http.StatusConflict,
		Msg:  fmt.Sprintf("user already exists: %s", cmd.Name),
	}
}

// handlers for the remove user command.
type removeUserCommand struct {
	baseCommand
	Name string `json:"name"`
}

func handleRemoveUser(w http.ResponseWriter, r *http.Request) {
	cmd := removeUserCommand{
		baseCommand: baseCommand{Op: opRemoveUser},
		Name:        r.FormValue("name"),
	}

	if cmd.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	// check if the user already exists for the 1st time.
	service.metadata.lock.Lock()
	u := service.metadata.Users[strings.ToLower(cmd.Name)]
	service.metadata.lock.Unlock()

	// if the user does not exist, consider it as success.
	if u == nil {
		return
	}

	v, err := json.Marshal(&cmd)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	slog.Info("remove user command received", slog.String("name", cmd.Name))
	raftApply(w, v)
}

func applyRemoveUser(d *Data, l *raft.Log) any {
	var cmd removeUserCommand
	if err := json.Unmarshal(l.Data, &cmd); err != nil {
		return err
	}

	key := strings.ToLower(cmd.Name)
	d.lock.Lock()
	u := d.Users[key]
	if u != nil {
		delete(d.Users, key)
	}
	d.lock.Unlock()

	if u != nil {
		slog.Info("user removed", slog.String("name", cmd.Name))
	} else {
		slog.Info("user does not exist", slog.String("name", cmd.Name))
	}

	return nil
}

// handlers for the set password command.
func applySetPassword(d *Data, l *raft.Log) any {
	return nil
}

// Apply implements method Apply of the raft.FSM interface.
func (d *Data) Apply(l *raft.Log) any {
	var cmd baseCommand
	if err := json.Unmarshal(l.Data, &cmd); err != nil {
		return fmt.Errorf("failed to unmarshal command: %w", err)
	}

	if cmd.Op >= opLast {
		return fmt.Errorf("invalid command: %d", cmd.Op)
	}

	fn := dataApplyFuncs[cmd.Op]
	if fn == nil {
		return fmt.Errorf("command has no handler: %d", cmd.Op)
	}

	return fn(d, l)
}

// Snapshot implements method Snapshot of the raft.FSM interface.
func (d *Data) Snapshot() (raft.FSMSnapshot, error) {
	return &snapshot{store: map[string]string{}}, nil
}

// Restore implements method Restore of the raft.FSM interface.
func (d *Data) Restore(snapshot io.ReadCloser) error {
	/*
		o := make(map[string]string)
		if err := json.NewDecoder(rc).Decode(&o); err != nil {
			return err
		}

		// Set the state from the snapshot, no lock required according to
		// Hashicorp docs.
		f.m = o
	*/
	return nil
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

// registerAPIs registers the meta service API handlers.
func registerAPIs() {
	httpserver.HandleFunc("GET /meta/node", handleListNode)

	// only voter nodes expose node & data management APIs
	if !config.CurrentNode().Meta.RaftVoter {
		return
	}

	httpserver.HandleFunc("POST /meta/node", handleAddNode)
	httpserver.HandleFunc("DELETE /meta/node", handleRemoveNode)
	registerDataAPIs()
}
