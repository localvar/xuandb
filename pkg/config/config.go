package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
)

// hasKeyFunc checks if a key is defined in the configuration file.
type hasKeyFunc func(string) bool

// LoggerConfig contains logger configuration.
type LoggerConfig struct {
	Format    string     `toml:"format" json:"format"`
	Level     slog.Level `toml:"level" json:"level"`
	AddSource bool       `toml:"add-source" json:"addSource"`
	OutputTo  string     `toml:"output-to" json:"outputTo"`
}

// dfltLoggerCfg contains the default values for LoggerConfig.
var dfltLoggerCfg = &LoggerConfig{
	Format:    "json",
	Level:     slog.LevelInfo,
	AddSource: true,
	OutputTo:  "stderr",
}

// updateDefault updates the default configuration with the values from the
// current configuration.
func (lc *LoggerConfig) updateDefault(hasKey hasKeyFunc) error {
	dflt := dfltLoggerCfg

	switch v := strings.ToLower(lc.Format); v {
	case "json", "text":
		dflt.Format = v
	case "":
		// do nothing
	default:
		return fmt.Errorf("unknown log format: %s", lc.Format)
	}

	if hasKey("level") {
		dflt.Level = lc.Level
	}

	if hasKey("add-source") {
		dflt.AddSource = lc.AddSource
	}

	if lc.OutputTo != "" {
		dflt.OutputTo = lc.OutputTo
	}

	return nil
}

// tidy fills missing configuration items with default values, normalizes all
// values and validates the configuration.
func (lc *LoggerConfig) tidy(hasKey hasKeyFunc) error {
	dflt := dfltLoggerCfg

	switch v := strings.ToLower(lc.Format); v {
	case "json", "text":
		lc.Format = v
	case "":
		lc.Format = dflt.Format
	default:
		return fmt.Errorf("unknown log format: %s", lc.Format)
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

	return nil
}

// MetaConfig contains configuration for the meta service.
type MetaConfig struct {
	RaftVoter         bool   `toml:"raft-voter" json:"raftVoter"`
	RaftAddr          string `toml:"raft-addr" json:"raftAddr"`
	RaftStore         string `toml:"raft-store" json:"raftStore"`
	RaftSnapshotStore string `toml:"raft-snapshot-store" json:"raftSnapshotStore"`
	DataDir           string `toml:"data-dir" json:"dataDir"`
}

// dfltMetaCfg contains the default values for MetaConfig.
var dfltMetaCfg = &MetaConfig{
	RaftStore:         "boltdb",
	RaftSnapshotStore: "file",
}

// updateDefault updates the default configuration with the values from the
// current configuration.
func (mc *MetaConfig) updateDefault(hasKey hasKeyFunc) error {
	dflt := dfltMetaCfg

	if hasKey("raft-voter") {
		dflt.RaftVoter = mc.RaftVoter
	}

	if mc.RaftAddr != "" {
		ap, err := netip.ParseAddrPort(mc.RaftAddr)
		if err != nil {
			return err
		}
		if !ap.Addr().IsUnspecified() {
			return errors.New("IP of 'raft-addr' of the default node can only be '0.0.0.0' or '[::]'")
		}
		if ap.Port() == 0 {
			return errors.New("port of 'raft-addr' cannot be 0")
		}
		dflt.RaftAddr = mc.RaftAddr
	}

	switch strings.ToLower(mc.RaftStore) {
	case "inmem", "memory":
		dflt.RaftStore = "memory"
	case "boltdb":
		dflt.RaftStore = "boltdb"
	case "":
		// do nothing
	default:
		return fmt.Errorf("invalid 'raft-store': %s", mc.RaftStore)
	}

	switch strings.ToLower(mc.RaftSnapshotStore) {
	case "discard", "none", "null":
		dflt.RaftSnapshotStore = "discard"
	case "inmem", "memory":
		dflt.RaftSnapshotStore = "memory"
	case "file":
		dflt.RaftSnapshotStore = "file"
	case "":
		// do nothing
	default:
		return fmt.Errorf("invalid 'raft-snapshot-store': %s", mc.RaftSnapshotStore)
	}

	if mc.DataDir != "" {
		dflt.DataDir = mc.DataDir
	}

	return nil
}

// tidy fills missing configuration items with default values, normalizes all
// values and validates the configuration.
func (mc *MetaConfig) tidy(hasKey hasKeyFunc) error {
	dflt := dfltMetaCfg

	if !hasKey("raft-voter") {
		mc.RaftVoter = dflt.RaftVoter
	}

	if mc.RaftAddr == "" {
		mc.RaftAddr = dflt.RaftAddr
	}
	if mc.RaftAddr == "" {
		return fmt.Errorf("'raft-addr' is required")
	} else if ap, err := netip.ParseAddrPort(mc.RaftAddr); err != nil {
		return err
	} else if ap.Port() == 0 {
		return errors.New("port of 'raft-addr' cannot be 0")
	}

	if !mc.RaftVoter {
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

	if mc.RaftStore != "boltdb" && mc.RaftSnapshotStore != "file" {
		mc.DataDir = ""
		return nil
	}

	if mc.DataDir == "" {
		mc.DataDir = dflt.DataDir
		if mc.DataDir == "" {
			return fmt.Errorf("'data-dir' is required")
		}
	}

	return nil
}

// DataConfig contains configuration for the data service.
type DataConfig struct {
	DataDir string `toml:"data-dir" json:"dataDir"`
}

// dfltDataCfg contains the default values for DataConfig.
var dfltDataCfg = &DataConfig{}

// updateDefault updates the default configuration with the values from the
// current configuration.
func (dc *DataConfig) updateDefault(hasKey hasKeyFunc) error {
	dflt := dfltDataCfg

	if dc.DataDir != "" {
		dflt.DataDir = dc.DataDir
	}

	return nil
}

// tidy fills missing configuration items with default values, normalizes all
// values and validates the configuration.
func (dc *DataConfig) tidy(hasKey hasKeyFunc) error {
	dflt := dfltDataCfg

	if dc.DataDir == "" {
		dc.DataDir = dflt.DataDir
	}

	return nil
}

// QueryConfig contains configuration for the query service.
type QueryConfig struct {
}

// dfltQueryCfg contains the default values for QueryConfig.
var dfltQueryCfg = &QueryConfig{}

// updateDefault updates the default configuration with the values from the
// current configuration.
func (qc *QueryConfig) updateDefault(hasKey hasKeyFunc) error {
	return nil
}

// tidy fills missing configuration items with default values, normalizes all
// values and validates the configuration.
func (qc *QueryConfig) tidy(hasKey hasKeyFunc) error {
	return nil
}

// NodeConfig contains configuration for a node.
type NodeConfig struct {
	ID          string        `toml:"id" json:"id"`
	DomainName  string        `toml:"domain-name" json:"domainName"`
	HTTPAddr    string        `toml:"http-addr" json:"httpAddr"`
	EnablePprof bool          `toml:"enable-pprof" json:"enablePprof"`
	Logger      *LoggerConfig `toml:"logger,omitempty" json:"logger,omitempty"`
	Meta        *MetaConfig   `toml:"meta,omitempty" json:"meta,omitempty"`
	Data        *DataConfig   `toml:"data,omitempty" json:"data,omitempty"`
	Query       *QueryConfig  `toml:"query,omitempty" json:"query,omitempty"`
}

// dfltNodeCfg contains the default values for NodeConfig.
var dfltNodeCfg = &NodeConfig{
	Logger: dfltLoggerCfg,
	Meta:   dfltMetaCfg,
	Data:   dfltDataCfg,
	Query:  dfltQueryCfg,
}

// ToExternalAddress converts an internal address to an external address.
func (nc *NodeConfig) ToExternalAddress(addr string) string {
	if nc.DomainName == "" {
		return addr
	}

	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		// This should not happen because this function is designed only for
		// 'nc.HTTPAddr' and 'nc.Meta.RaftAddr'.
		panic(err)
	}

	return net.JoinHostPort(nc.DomainName, port)
}

// updateDefault updates the default configuration with the values from the
// current configuration.
func (nc *NodeConfig) updateDefault(hasKey hasKeyFunc) error {
	dflt := dfltNodeCfg

	// skip 'ID' and 'DomainName' as they should not have default values.

	if nc.HTTPAddr != "" {
		ap, err := netip.ParseAddrPort(nc.HTTPAddr)
		if err != nil {
			return err
		}
		if !ap.Addr().IsUnspecified() {
			return errors.New("IP of 'http-addr' of the default node can only be '0.0.0.0' or '[::]'")
		}
		if ap.Port() == 0 {
			return errors.New("port of 'http-addr' cannot be 0")
		}
		dflt.HTTPAddr = nc.HTTPAddr
	}

	if hasKey("enable-pprof") {
		dflt.EnablePprof = nc.EnablePprof
	}

	if nc.Logger != nil {
		hasKey1 := func(key string) bool { return hasKey("logger." + key) }
		if err := nc.Logger.updateDefault(hasKey1); err != nil {
			return err
		}
	}

	if nc.Meta != nil {
		hasKey1 := func(key string) bool { return hasKey("meta." + key) }
		if err := nc.Meta.updateDefault(hasKey1); err != nil {
			return err
		}
	}

	if nc.Data != nil {
		hasKey1 := func(key string) bool { return hasKey("data." + key) }
		if err := nc.Data.updateDefault(hasKey1); err != nil {
			return err
		}
	}

	if nc.Query != nil {
		hasKey1 := func(key string) bool { return hasKey("query." + key) }
		if err := nc.Query.updateDefault(hasKey1); err != nil {
			return err
		}
	}

	return nil
}

// tidy fills missing configuration items with default values, normalizes all
// values and validates the configuration.
func (nc *NodeConfig) tidy(hasKey hasKeyFunc) error {
	dflt := dfltNodeCfg

	if nc.ID == "" {
		return errors.New("'id' is required for all node")
	}

	if nc.HTTPAddr == "" {
		nc.HTTPAddr = dflt.HTTPAddr
	}
	if nc.HTTPAddr == "" {
		return fmt.Errorf("'http-addr' is required for node '%s'", nc.ID)
	} else if ap, err := netip.ParseAddrPort(nc.HTTPAddr); err != nil {
		return err
	} else if ap.Port() == 0 {
		return errors.New("port of 'http-addr' cannot be 0")
	}

	if !hasKey("enable-pprof") {
		nc.EnablePprof = dfltNodeCfg.EnablePprof
	}

	if nc.Logger != nil {
		hasKey1 := func(key string) bool { return hasKey("logger." + key) }
		if err := nc.Logger.tidy(hasKey1); err != nil {
			return err
		}
	} else {
		nc.Logger = dflt.Logger
	}

	if nc.Meta != nil {
		hasKey1 := func(key string) bool { return hasKey("meta." + key) }
		if err := nc.Meta.tidy(hasKey1); err != nil {
			return err
		}
	} else {
		return fmt.Errorf("'meta' section is required for node '%s'", nc.ID)
	}

	if nc.Data != nil {
		hasKey1 := func(key string) bool { return hasKey("data." + key) }
		if err := nc.Data.tidy(hasKey1); err != nil {
			return err
		}
	}

	if nc.Query != nil {
		hasKey1 := func(key string) bool { return hasKey("query." + key) }
		if err := nc.Query.tidy(hasKey1); err != nil {
			return err
		}
	}

	return nil
}

// Config contains all configurations.
type Config struct {
	ClusterName string        `toml:"cluster-name" json:"clusterName"`
	Nodes       []*NodeConfig `toml:"node" json:"nodes"`
}

// extracts the keys for each node.
func extractNodeKeys(keys []toml.Key) [][]string {
	var result [][]string

	for _, k := range keys {
		if k[0] != "node" {
			continue
		}

		if len(k) == 1 {
			result = append(result, nil)
			continue
		}

		idx := len(result) - 1
		if idx < 0 {
			// 'node.xxx' appears before a '[[node]]'.
			panic("unexpected key: " + k.String())
		}

		result[idx] = append(result[idx], k[1:].String())
	}

	return result
}

// tidyNodes tidies all node configuraions.
func (c *Config) tidyNodes(definedKeys []toml.Key) error {
	allNodeKeys := extractNodeKeys(definedKeys)

	// curNodeKeys contains defined keys of the current node
	var curNodeKeys []string
	hasKey := func(key string) bool {
		return slices.Contains(curNodeKeys, key)
	}

	// find the default node and remove it from the slice.
	var dflt *NodeConfig
	for i, nc := range allCfg.Nodes {
		if nc.ID == "#default#" {
			if dflt != nil {
				return errors.New("duplicated default node")
			}
			dflt = nc
			curNodeKeys = allNodeKeys[i]
		} else if dflt != nil {
			// left shift nodes after the default to remove the default.
			allCfg.Nodes[i-1] = nc
			allNodeKeys[i-1] = allNodeKeys[i]
		}
	}

	// if the default node is defined, shrink the slice as we have removed it,
	// and apply its values to the internal default.
	if dflt != nil {
		allCfg.Nodes = allCfg.Nodes[:len(allCfg.Nodes)-1]
		allNodeKeys = allNodeKeys[:len(allNodeKeys)-1]
		if err := dflt.updateDefault(hasKey); err != nil {
			return err
		}
	}

	// tidy all other nodes.
	numVoter := 0
	ids := make(map[string]struct{})
	for i, nc := range allCfg.Nodes {
		if _, ok := ids[nc.ID]; ok {
			return fmt.Errorf("duplicated node id: %s", nc.ID)
		}
		ids[nc.ID] = struct{}{}

		curNodeKeys = allNodeKeys[i]
		if err := nc.tidy(hasKey); err != nil {
			return err
		}

		if nc.Meta.RaftVoter {
			numVoter++
		}
	}

	if numVoter == 0 {
		return errors.New("no raft voter node")
	}

	if numVoter%2 == 0 {
		return errors.New("even number of raft voter nodes")
	}

	return nil
}

// tidy fills missing configuration items with default values, normalizes all
// values and validates the configuration.
func (c *Config) tidy(definedKeys []toml.Key) error {
	return c.tidyNodes(definedKeys)
}
