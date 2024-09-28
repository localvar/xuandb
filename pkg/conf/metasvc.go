package conf

import (
	"fmt"
	"strings"
)

// MetaServiceConf contains configuration for the meta service.
type MetaServiceConf struct {
	RaftAddr          string `toml:"raft-addr" json:"raftAddr"`
	RaftStore         string `toml:"raft-store" json:"raftStore"`
	RaftSnapshotStore string `toml:"raft-snapshot-store" json:"raftSnapshotStore"`
	DataDir           string `toml:"data-dir" json:"dataDir"`
}

// defaultMetaServiceConf contains the default values for MetaServiceConf.
var defaultMetaServiceConf = &MetaServiceConf{
	RaftStore:         "boltdb",
	RaftSnapshotStore: "file",
}

// filleDefaults fills the default values for MetaServiceConf.
func (msc *MetaServiceConf) fillDefaults(dflt *MetaServiceConf) {
	if msc.RaftStore == "" {
		msc.RaftStore = dflt.RaftStore
	}
	if msc.RaftSnapshotStore == "" {
		msc.RaftSnapshotStore = dflt.RaftSnapshotStore
	}
	if msc.DataDir == "" {
		msc.DataDir = dflt.DataDir
	}
}

// normalizeAndValidate normalizes & validates the MetaServiceConf.
func (msc *MetaServiceConf) normalizeAndValidate() error {
	if msc == nil {
		return nil
	}

	switch strings.ToLower(msc.RaftStore) {
	case "inmem", "memory":
		msc.RaftStore = "memory"
	case "boltdb", "":
		msc.RaftStore = "boltdb"
	default:
		return fmt.Errorf("invalid 'raft-store': %s", msc.RaftStore)
	}

	switch strings.ToLower(msc.RaftSnapshotStore) {
	case "discard", "none", "null":
		msc.RaftSnapshotStore = "discard"
	case "inmem", "memory":
		msc.RaftSnapshotStore = "memory"
	case "file", "":
		msc.RaftSnapshotStore = "file"
	default:
		return fmt.Errorf("invalid 'raft-snapshot-store': %s", msc.RaftSnapshotStore)
	}

	if msc.RaftStore == "boltdb" || msc.RaftSnapshotStore == "file" {
		if msc.DataDir == "" {
			return fmt.Errorf("'data-dir' is required when 'raft-store' is 'boltdb' or 'raft-snapshot-store' is 'file'")
		}
	}

	return nil
}
