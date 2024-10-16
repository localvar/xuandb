package meta

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/raft"
	"github.com/localvar/xuandb/pkg/utils"
)

// Data is the meta data that managed by the meta service.
type Data struct {
	Users map[string]*User
}

// newData creates a new Data.
func newData() *Data {
	return &Data{
		Users: map[string]*User{},
	}
}

// baseCommand is the base of all data operation commands.
type baseCommand struct {
	Op uint32 `json:"op"`
}

// List of data operations, for each operation, there are two handlers, one
// for handling client requests, with name like "handleXXXXXX", the other for
// applying the raft log, with name like "applyXXXXXX".
const (
	opUpdateNodeList = iota + 1
	opCreateUser
	opDropUser
	opSetPassword
	opLast
)

// dataApplyFuncs is the list of functions to apply data operations.
var dataApplyFuncs = [opLast]func(*raft.Log) any{
	opUpdateNodeList: applyUpdateNodeList,
	opCreateUser:     applyCreateUser,
	opDropUser:       applyDropUser,
	opSetPassword:    applySetPassword,
}

// raftApply is a helper function to apply a command to the Raft log.
func (s *service) raftApply(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return &utils.StatusError{
			Code: http.StatusInternalServerError,
			Msg:  err.Error(),
		}
	}

	future := s.raft.Apply(data, 0)
	if err := future.Error(); err != nil {
		return err
	}

	resp := future.Response()
	if resp == nil {
		return nil
	}

	if se, ok := resp.(*utils.StatusError); ok {
		return se
	} else if err, ok := resp.(error); ok {
		return err
	} else {
		panic("unexpected response")
	}
}

// Apply implements the raft.FSM interface.
func (s *service) Apply(l *raft.Log) any {
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

	return fn(l)
}

// Snapshot implements the raft.FSM interface.
func (s *service) Snapshot() (raft.FSMSnapshot, error) {
	return &snapshot{store: map[string]string{}}, nil
}

// Restore implements the raft.FSM interface.
func (s *service) Restore(io.ReadCloser) error {
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
