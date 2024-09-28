package conf

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/BurntSushi/toml"
)

// CommonConf contains common/shared configurations.
type CommonConf struct {
	Logger LoggerConf `toml:"logger" json:"logger"`
}

// fillDefaults fills the default values for configurations.
func (cc *CommonConf) fillDefaults() {
	cc.Logger.fillDefaults()
}

// normalizeAndValidate normalizes & validates the configurations.
func (cc *CommonConf) normalizeAndValidate() error {
	return cc.Logger.normalizeAndValidate()
}

// LoggerConf contains logger configuration.
type LoggerConf struct {
	Format    string `toml:"format" json:"format"`
	Level     string `toml:"level" json:"level"`
	AddSource bool   `toml:"add-source" json:"addSource"`
	OutputTo  string `toml:"output-to" json:"outputTo"`
}

// fillDefaults fills the default values for configurations.
func (lc *LoggerConf) fillDefaults() {
	if lc.Level == "" {
		lc.Level = "info"
	}
	if lc.Format == "" {
		lc.Format = "text"
	}
	if lc.OutputTo == "" {
		lc.OutputTo = "stderr"
	}
}

// normalizeAndValidate normalizes & validates the configurations.
func (lc *LoggerConf) normalizeAndValidate() error {
	switch v := strings.ToLower(lc.Level); v {
	case "debug", "info", "warn", "error":
		lc.Level = v
	default:
		return fmt.Errorf("unknown log level: %s", lc.Level)
	}
	return nil
}

// NodeConf contains configuration for a node.
type NodeConf struct {
	ID           string            `toml:"id" json:"id"`
	HTTPAddr     string            `toml:"http-addr" json:"httpAddr"`
	MetaService  *MetaServiceConf  `toml:"meta-service,omitempty" json:"metaService,omitempty"`
	DataService  *DataServiceConf  `toml:"data-service,omitempty" json:"dataService,omitempty"`
	QueryService *QueryServiceConf `toml:"query-service,omitempty" json:"queryService,omitempty"`
}

// defaultNodeConf contains the default values for NodeConf.
var defaultNodeConf = &NodeConf{
	MetaService:  defaultMetaServiceConf,
	DataService:  defaultDataServiceConf,
	QueryService: defaultQueryServiceConf,
}

// allConf contains all of the configuration.
var allConf struct {
	CommonConf
	Nodes []*NodeConf `toml:"node" json:"nodes"`
}

// common command line arguments for both client and server.
var (
	confPath    string
	showVersion bool
)

func init() {
	flag.StringVar(&confPath, "config", "", "path to config file")
	flag.BoolVar(&showVersion, "version", false, "show version information")
}

// ShowVersion returns true if the version information should be shown.
func ShowVersion() bool {
	return showVersion
}

// Common returns the common/shared configurations.
func Common() *CommonConf {
	return &allConf.CommonConf
}

// Nodes returns all node configurations.
func Nodes() []*NodeConf {
	return allConf.Nodes
}

// NodeByID returns the server configuration by ID.
// It returns nil if not found.
func NodeByID(id string) *NodeConf {
	for _, n := range allConf.Nodes {
		if n.ID == id {
			return n
		}
	}
	return nil
}

// getConfigPath returns the path of the configuration file.
func getConfigPath() string {
	// 1st, try command line argument.
	if confPath != "" {
		return confPath
	}

	// 2nd, try environment variable.
	path := os.Getenv("XUANDB_CONFIG_PATH")
	if path != "" {
		return path
	}

	// 3rd, try current working directory.
	path = "xuandb.toml"
	fi, err := os.Stat(path)
	if err == nil && !fi.IsDir() {
		return path
	}

	// 4th, try executable directory.
	exe, err := os.Executable()
	if err == nil {
		path = filepath.Join(filepath.Dir(exe), path)
		fi, err = os.Stat(path)
		if err == nil && !fi.IsDir() {
			return path
		}
	}

	// finally, try O/S specific configuration directory.
	switch runtime.GOOS {
	case "linux", "darwin", "freebsd", "openbsd":
		path = "/etc/xuandb/xuandb.toml"
	default:
		return ""
	}
	fi, err = os.Stat(path)
	if err == nil && !fi.IsDir() {
		return path
	}

	return ""
}

// fillDefaults fills the default values for configurations.
func fillDefaults() {
	allConf.CommonConf.fillDefaults()

	// find the default node configuration and remove it from the list.
	var dflt *NodeConf
	for i, nc := range allConf.Nodes {
		if nc.ID == "#default#" {
			dflt = nc
			continue
		}
		// left shift nodes after the default node to remove the default node.
		if dflt != nil {
			allConf.Nodes[i-1] = nc
		}
	}

	if dflt == nil {
		// there's no default node, use the global default values.
		dflt = defaultNodeConf
	} else {
		// shrink the slice as we have removed the default node.
		allConf.Nodes = allConf.Nodes[:len(allConf.Nodes)-1]

		// dflt may have blank fields, so fill the global default values.
		if dflt.MetaService == nil {
			dflt.MetaService = defaultMetaServiceConf
		} else {
			dflt.MetaService.fillDefaults(defaultMetaServiceConf)
		}

		if dflt.DataService == nil {
			dflt.DataService = defaultDataServiceConf
		} else {
			dflt.DataService.fillDefaults(defaultDataServiceConf)
		}

		if dflt.QueryService == nil {
			dflt.QueryService = defaultQueryServiceConf
		} else {
			dflt.QueryService.fillDefaults(defaultQueryServiceConf)
		}
	}

	// copy default values from default to all nodes.
	for _, nc := range allConf.Nodes {
		if nc.MetaService != nil {
			nc.MetaService.fillDefaults(dflt.MetaService)
		}
		if nc.DataService != nil {
			nc.DataService.fillDefaults(dflt.DataService)
		}
		if nc.QueryService != nil {
			nc.QueryService.fillDefaults(dflt.QueryService)
		}
	}
}

// normalizeAndValidate normalizes & validates the configurations.
func normalizeAndValidate() error {
	if err := allConf.normalizeAndValidate(); err != nil {
		return err
	}

	ids := make(map[string]struct{})
	for _, nc := range allConf.Nodes {
		if nc.ID == "" {
			return errors.New("'id' is required for each node")
		}
		if _, ok := ids[nc.ID]; ok {
			return fmt.Errorf("duplicated node id: %s", nc.ID)
		}
		ids[nc.ID] = struct{}{}

		if nc.HTTPAddr == "" {
			return fmt.Errorf("'http-addr' is required for node '%s'", nc.ID)
		}

		if err := nc.MetaService.normalizeAndValidate(); err != nil {
			return err
		}
		if err := nc.DataService.normalizeAndValidate(); err != nil {
			return err
		}
		if err := nc.QueryService.normalizeAndValidate(); err != nil {
			return err
		}
	}

	return nil
}

// load loads configurations from file, fills missing values with default,
// normalizes the configurations and validates the results.
func load() error {
	path := getConfigPath()
	if path == "" {
		return errors.New("no available configuration file")
	}

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = toml.NewDecoder(f).Decode(&allConf)
	if err != nil {
		return err
	}

	fillDefaults()

	return normalizeAndValidate()
}
