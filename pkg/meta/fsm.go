package meta

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"sync"

	"github.com/hashicorp/raft"
	"github.com/localvar/xuandb/pkg/xerrors"
)

// Data is the meta data that managed by the meta service.
//
// We expect updates to the data is much less frequent than reads, so we
// optimize for reads by making the objects in the data immutable, that is,
// we never modify the objects in the data, instead, we create a new object
// and replace the old one. On the other hand, readers should not modify the
// returned objects as it may be shared by multiple readers in multiple
// goroutines.
//
// The update operations are done by the dataApplyFuncs to make the code more
// readable and maintainable.
type Data struct {
	l         sync.Mutex           `json:"-"`
	Users     map[string]*User     `json:"users"`
	Databases map[string]*Database `json:"databases"`
}

// newData creates a new Data.
func newData() *Data {
	return &Data{
		Users:     map[string]*User{},
		Databases: map[string]*Database{},
	}
}

// clone clones the data.
func (d *Data) clone() *Data {
	r := newData()

	d.lock()
	defer d.unlock()

	for k, v := range d.Users {
		r.Users[k] = v
	}

	return r
}

// lock locks the data.
func (d *Data) lock() {
	d.l.Lock()
}

// unlock unlocks the data.
func (d *Data) unlock() {
	d.l.Unlock()
}

// Persist implements [raft.FSMSnapshot]
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

// Release implements [raft.FSMSnapshot]
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

// Apply implements [raft.FSM]
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

// Snapshot implements [raft.FSM]
func (s *service) Snapshot() (raft.FSMSnapshot, error) {
	return s.md.clone(), nil
}

// Restore implements [raft.FSM]
func (s *service) Restore(rc io.ReadCloser) error {
	d := newData()
	if err := json.NewDecoder(rc).Decode(d); err != nil {
		return err
	}
	s.md = d
	return nil
}
