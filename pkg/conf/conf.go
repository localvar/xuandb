package conf

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
)

// hasKeyFunc checks if a key is defined in the configuration file.
type hasKeyFunc func(string) bool

// CommonConf contains common/shared configurations for all nodes.
type CommonConf struct {
	ClusterName string `toml:"cluster-name" json:"clusterName"`
}

// tidy fills missing configuration items with default values, normalizes all
// values and validates the configuration.
func (cc *CommonConf) tidy() error {
	return nil
}

// LoggerConf contains logger configuration.
type LoggerConf struct {
	Format    string     `toml:"format" json:"format"`
	Level     slog.Level `toml:"level" json:"level"`
	AddSource bool       `toml:"add-source" json:"addSource"`
	OutputTo  string     `toml:"output-to" json:"outputTo"`
}

// defaultLoggerConf contains the default values for LoggerConf.
var defaultLoggerConf = &LoggerConf{
	Format:    "json",
	Level:     slog.LevelInfo,
	AddSource: true,
	OutputTo:  "stderr",
}

// tidy fills missing configuration items with default values, normalizes all
// values and validates the configuration.
func (lc *LoggerConf) tidy(dflt *LoggerConf, hasKey hasKeyFunc) error {
	if lc.Format == "" {
		lc.Format = dflt.Format
	}

	if !hasKey("level") {
		lc.Level = dflt.Level
	}

	if !hasKey("add-source") {
		lc.AddSource = dflt.AddSource
	}

	if lc.OutputTo == "" {
		lc.OutputTo = dflt.OutputTo
	}

	switch v := strings.ToLower(lc.Format); v {
	case "json", "text":
		lc.Format = v
	default:
		return fmt.Errorf("unknown log format: %s", lc.Format)
	}

	return nil
}

// MetaConf contains configuration for the meta service.
type MetaConf struct {
	RaftVoter         bool   `toml:"raft-voter" json:"raftVoter"`
	RaftAddr          string `toml:"raft-addr" json:"raftAddr"`
	RaftStore         string `toml:"raft-store" json:"raftStore"`
	RaftSnapshotStore string `toml:"raft-snapshot-store" json:"raftSnapshotStore"`
	DataDir           string `toml:"data-dir" json:"dataDir"`
}

// defaultMetaConf contains the default values for MetaConf.
var defaultMetaConf = &MetaConf{
	RaftStore:         "boltdb",
	RaftSnapshotStore: "file",
}

// tidy fills missing configuration items with default values, normalizes all
// values and validates the configuration.
func (mc *MetaConf) tidy(dflt *MetaConf, hasKey hasKeyFunc, dfltNode bool) error {
	if !hasKey("raft-addr") {
		mc.RaftVoter = dflt.RaftVoter
	}

	// the raft address of the default node should be empty, while should not
	// be empty if not default node.
	if dfltNode {
		mc.RaftAddr = ""
	} else if mc.RaftAddr == "" {
		return fmt.Errorf("'raft-addr' is required")
	} else if !mc.RaftVoter {
		mc.RaftStore = "memory"
		mc.RaftSnapshotStore = "discard"
		mc.DataDir = ""
		return nil
	}

	switch strings.ToLower(mc.RaftStore) {
	case "inmem", "memory":
		mc.RaftStore = "memory"
	case "boltdb":
		mc.RaftStore = "boltdb"
	case "":
		mc.RaftStore = dflt.RaftStore
	default:
		return fmt.Errorf("invalid 'raft-store': %s", mc.RaftStore)
	}

	switch strings.ToLower(mc.RaftSnapshotStore) {
	case "discard", "none", "null":
		mc.RaftSnapshotStore = "discard"
	case "inmem", "memory":
		mc.RaftSnapshotStore = "memory"
	case "file":
		mc.RaftSnapshotStore = "file"
	case "":
		mc.RaftSnapshotStore = dflt.RaftSnapshotStore
	default:
		return fmt.Errorf("invalid 'raft-snapshot-store': %s", mc.RaftSnapshotStore)
	}

	if mc.DataDir == "" {
		mc.DataDir = dflt.DataDir
	}

	// if default node, no need to check the relationship between 'raft-store',
	// 'raft-snapshot-store' and 'data-dir'
	if dfltNode {
		return nil
	}

	if mc.RaftStore != "boltdb" && mc.RaftSnapshotStore != "file" {
		return nil
	}

	if mc.DataDir == "" {
		return fmt.Errorf("'data-dir' is required")
	}

	return nil
}

// StoreConf contains configuration for the store service.
type StoreConf struct {
	DataDir string `toml:"data-dir" json:"dataDir"`
}

// defaultStoreConf contains the default values for StoreConf.
var defaultStoreConf = &StoreConf{}

// tidy fills missing configuration items with default values, normalizes all
// values and validates the configuration.
func (sc *StoreConf) tidy(dflt *StoreConf, hasKey hasKeyFunc) error {
	if sc.DataDir == "" {
		sc.DataDir = dflt.DataDir
	}
	return nil
}

// QueryConf contains configuration for the query service.
type QueryConf struct {
}

// defaultQueryConf contains the default values for QueryConf.
var defaultQueryConf = &QueryConf{}

// tidy fills missing configuration items with default values, normalizes all
// values and validates the configuration.
func (qc *QueryConf) tidy(dflt *QueryConf, hasKey hasKeyFunc) error {
	return nil
}

// NodeConf contains configuration for a node.
type NodeConf struct {
	ID          string      `toml:"id" json:"id"`
	HTTPAddr    string      `toml:"http-addr" json:"httpAddr"`
	EnablePprof bool        `toml:"enable-pprof" json:"enablePprof"`
	Logger      *LoggerConf `toml:"logger,omitempty" json:"logger,omitempty"`
	Meta        *MetaConf   `toml:"meta,omitempty" json:"meta,omitempty"`
	Store       *StoreConf  `toml:"store,omitempty" json:"store,omitempty"`
	Query       *QueryConf  `toml:"query,omitempty" json:"query,omitempty"`
}

// defaultNodeConf contains the default values for NodeConf.
var defaultNodeConf = &NodeConf{
	Logger: defaultLoggerConf,
	Meta:   defaultMetaConf,
	Store:  defaultStoreConf,
	Query:  defaultQueryConf,
}

// tidy fills missing configuration items with default values, normalizes all
// values and validates the configuration.
func (nc *NodeConf) tidy(dflt *NodeConf, hasKey hasKeyFunc, dfltNode bool) error {
	if nc.ID == "" {
		return errors.New("'id' is required for each node")
	}

	// the HTTP address of the default node should be empty, while should not
	// be empty if not default node.
	if dfltNode {
		nc.HTTPAddr = ""
	} else if nc.HTTPAddr == "" {
		return fmt.Errorf("'http-addr' is required for node '%s'", nc.ID)
	}

	if !hasKey("enable-pprof") {
		nc.EnablePprof = defaultNodeConf.EnablePprof
	}

	if nc.Logger != nil {
		hasKey1 := func(key string) bool { return hasKey("logger." + key) }
		if err := nc.Logger.tidy(dflt.Logger, hasKey1); err != nil {
			return err
		}
	} else {
		nc.Logger = dflt.Logger
	}

	if nc.Meta != nil {
		hasKey1 := func(key string) bool { return hasKey("meta." + key) }
		if err := nc.Meta.tidy(dflt.Meta, hasKey1, dfltNode); err != nil {
			return err
		}
	} else if dfltNode {
		nc.Meta = dflt.Meta
	} else {
		return fmt.Errorf("'meta' section is required for node '%s'", nc.ID)
	}

	if nc.Store != nil {
		hasKey1 := func(key string) bool { return hasKey("store." + key) }
		if err := nc.Store.tidy(dflt.Store, hasKey1); err != nil {
			return err
		}
	} else if dfltNode {
		nc.Store = dflt.Store
	}

	if nc.Query != nil {
		hasKey1 := func(key string) bool { return hasKey("query." + key) }
		if err := nc.Query.tidy(dflt.Query, hasKey1); err != nil {
			return err
		}
	} else if dfltNode {
		nc.Query = dflt.Query
	}

	return nil
}
