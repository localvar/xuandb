package meta

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/hashicorp/raft"
	"github.com/localvar/xuandb/pkg/xerrors"
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

// clone clones the data.
func (d *Data) clone() *Data {
	r := newData()

	for k, v := range d.Users {
		u := *v
		r.Users[k] = &u
	}

	return r
}

// Persist implements the raft.FSMSnapshot interface.
func (d *Data) Persist(sink raft.SnapshotSink) error {
	err := func() error {
		// Encode data.
		b, err := json.Marshal(d)
		if err != nil {
			return err
		}

		// Write data to sink.
		if _, err := sink.Write(b); err != nil {
			return err
		}

		// Close the sink.
		return sink.Close()
	}()

	if err != nil {
		sink.Cancel()
	}

	return err
}

// Release implements the raft.FSMSnapshot interface.
func (d *Data) Release() {
}

// dataApplyFuncs is the list of functions to apply data operations. Most of
// the operations (but not all) includes 4 functions:
//
//   - applyXXXXXX : applies the raft log of the operation to the current node,
//     it is called by raft and are listed in this map.
//
//   - leaderXXXXXX: for leader to create a raft log of the operation and apply
//     the log to raft.
//
//   - handleXXXXXX: for leader to handle client HTTP request of the operation,
//     it does some validation and then call leaderXXXXXX.
//
//   - XXXXXX      : exported function for client to call, it builds and sends
//     an HTTP request to the leader if called from a follower, and call
//     leaderXXXXXX directly if called from the leader.
var dataApplyFuncs = map[string]func(*raft.Log) any{}

// registerDataApplyFunc registers a data apply function.
func registerDataApplyFunc(op string, fn func(*raft.Log) any) {
	if dataApplyFuncs[op] != nil {
		panic("duplicate data apply function: " + op)
	}
	dataApplyFuncs[op] = fn
}

// baseCommand is the base of all data operation commands.
type baseCommand struct {
	Op string `json:"op"`
}

// raftApply is a helper function to apply a command to the Raft log.
func (s *service) raftApply(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return xerrors.Wrap(err, http.StatusInternalServerError)
	}

	future := s.raft.Apply(data, 0)
	if err := future.Error(); err != nil {
		return xerrors.Wrap(err, http.StatusInternalServerError)
	}

	resp := future.Response()
	if resp == nil {
		return nil
	}

	if err, ok := resp.(error); ok {
		return xerrors.Wrap(err, http.StatusInternalServerError)
	} else {
		panic("unexpected response")
	}
}

// Apply implements the raft.FSM interface.
func (s *service) Apply(l *raft.Log) any {
	var cmd baseCommand
	if err := json.Unmarshal(l.Data, &cmd); err != nil {
		slog.Error(
			"failed to unmarshal data operation command",
			slog.String("error", err.Error()),
		)
		return err
	}

	fn := dataApplyFuncs[cmd.Op]
	if fn == nil {
		panic("unknown data operation: " + cmd.Op)
	}

	return fn(l)
}

// Snapshot implements the raft.FSM interface.
func (s *service) Snapshot() (raft.FSMSnapshot, error) {
	s.lockMetadata()
	d := s.metadata.clone()
	s.unlockMetadata()

	return d, nil
}

// Restore implements the raft.FSM interface.
func (s *service) Restore(rc io.ReadCloser) error {
	d := newData()
	if err := json.NewDecoder(rc).Decode(d); err != nil {
		return err
	}
	s.metadata = d
	return nil
}
