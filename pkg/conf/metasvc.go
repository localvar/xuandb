package conf

import (
	"fmt"
	"strconv"
	"strings"
)

// MetaServiceConf contains configuration for the meta service.
type MetaServiceConf struct {
	RaftVoter         string `toml:"raft-voter" json:"raftVoter"`
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
		return fmt.Errorf("'meta-service' section is required")
	}

	if v, err := strconv.ParseBool(msc.RaftVoter); err == nil {
		msc.RaftVoter = strconv.FormatBool(v)
	} else {
		return fmt.Errorf("invalid 'raft-voter': %s", msc.RaftVoter)
	}

	if msc.RaftAddr == "" {
		return fmt.Errorf("'raft-addr' is required")
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

	if msc.RaftVoter != "true" {
		msc.RaftStore = "memory"
		msc.RaftSnapshotStore = "discard"
		msc.DataDir = ""
		return nil
	}

	if msc.RaftStore != "boltdb" && msc.RaftSnapshotStore != "file" {
		return nil
	}

	if msc.DataDir == "" {
		return fmt.Errorf("'data-dir' is required")
	}

	return nil
}
