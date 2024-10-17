package config

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/BurntSushi/toml"
)

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

// allCfg contains all of the configurations.
var allCfg = &Config{}

// curNodeCfg contains configuration for current node. It will be nil if
// current node is not specified.
var curNodeCfg *NodeConfig

// ClusterName returns the name of the cluster.
func ClusterName() string {
	return allCfg.ClusterName
}

// All returns all of the configurations. Note that nodes can be added or
// removed dynamically, but the return value of this function does not change
// accordingly.
func All() *Config {
	return allCfg
}

// Nodes returns configurations of all nodes, note that nodes can be added or
// removed dynamically, but the return value of this function does not change
// accordingly. To get a list of nodes at run time, call 'meta.Nodes' instead.
func Nodes() []*NodeConfig {
	return allCfg.Nodes
}

// NodeByID returns the server configuration by ID. It returns nil if not found.
// Note that nodes can be added or removed dynamically, but the return value of
// this function does not change accordingly. To get a node at run time, call
// 'meta.NodeByID' instead.
func NodeByID(id string) *NodeConfig {
	for _, n := range allCfg.Nodes {
		if n.ID == id {
			return n
		}
	}
	return nil
}

// NodeID returns the ID of current node. It panics if current node is not
// specified.
func NodeID() string {
	return curNodeCfg.ID
}

// CurrentNode returns configuration of the current node.
func CurrentNode() *NodeConfig {
	return curNodeCfg
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

// Load loads configurations from file, set missing items with default values,
// and makes necessary normalization and validation.
//
// If nodeID is specified (i.e. not empty), it set the corresponding node
// configuration as the current node configuration.
func Load(nodeID string) error {
	path := getConfigPath()
	if path == "" {
		return errors.New("no available configuration file")
	}

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	md, err := toml.NewDecoder(f).Decode(allCfg)
	if err != nil {
		return err
	}

	if err := allCfg.tidy(md.Keys()); err != nil {
		return err
	}

	if nodeID != "" {
		if curNodeCfg = NodeByID(nodeID); curNodeCfg == nil {
			return fmt.Errorf("missing configuration for node %q", nodeID)
		}
	}

	return nil
}

// HandleList is an http handler to expose configurations.
func HandleList(w http.ResponseWriter, r *http.Request) {
	if strings.EqualFold(r.Header.Get("Accept"), "application/json") {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(allCfg)
	} else {
		w.Header().Set("Content-Type", "application/toml")
		toml.NewEncoder(w).Encode(allCfg)
	}
}
