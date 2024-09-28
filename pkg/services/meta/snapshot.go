package meta

import (
	"encoding/json"

	"github.com/hashicorp/raft"
)

type snapshot struct {
	store map[string]string
}

func (s *snapshot) Persist(sink raft.SnapshotSink) error {
	err := func() error {
		// Encode data.
		b, err := json.Marshal(s.store)
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

func (f *snapshot) Release() {
}
