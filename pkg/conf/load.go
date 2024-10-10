package conf

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
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

// allConf contains all of the configuration.
var allConf struct {
	CommonConf
	Nodes []*NodeConf `toml:"node" json:"nodes"`
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

// curNodeConf contains configuration for current node. It will be nil if
// current node is not specified.
var curNodeConf *NodeConf

// NodeID returns the ID of current node. It panics if current node is not
// specified.
func NodeID() string {
	return curNodeConf.ID
}

// CurrentNode returns configuration for current node.
func CurrentNode() *NodeConf {
	return curNodeConf
}

// extracts the keys for each node.
func extractNodeKeys(keys []string) [][]string {
	var result [][]string
	var node []string

	for _, k := range keys {
		if k == "node" {
			// we need to add all nodes, even if it has no keys. so don't use
			// 'len(nodeKeys) == 0' here.
			if node != nil {
				result = append(result, node)
			}
			node = make([]string, 0, 8)
		} else if strings.HasPrefix(k, "node.") {
			node = append(node, k[5:])
		}
	}
	if node != nil {
		result = append(result, node)
	}

	return result
}

// tidy fills missing configuration items with default values, normalizes all
// values and validates the configuration.
func tidy(definedKeys []string) error {
	// tidy common configuration.
	if err := allConf.CommonConf.tidy(); err != nil {
		return err
	}

	// keys contains defined keys of the current node, allNodeKeys contains
	// defined keys of all nodes.
	var keys []string
	hasKey := func(key string) bool {
		return slices.Contains(keys, "node."+key)
	}
	allNodeKeys := extractNodeKeys(definedKeys)

	// find the default node configuration and remove it from the slice.
	dflt := defaultNodeConf
	for i, nc := range allConf.Nodes {
		if nc.ID == "#default#" {
			if dflt != defaultNodeConf {
				return errors.New("duplicated default node")
			}
			dflt = nc
			keys = allNodeKeys[i]
			continue
		}
		// left shift nodes after the default node to remove the default node.
		if dflt != defaultNodeConf {
			allConf.Nodes[i-1] = nc
			allNodeKeys[i-1] = allNodeKeys[i]
		}
	}

	// the default node is defined.
	if dflt != defaultNodeConf {
		// shrink the slice as we have removed the default node.
		allConf.Nodes = allConf.Nodes[:len(allConf.Nodes)-1]
		allNodeKeys = allNodeKeys[:len(allNodeKeys)-1]

		if err := dflt.tidy(defaultNodeConf, hasKey, true); err != nil {
			return err
		}
	}

	// tidy all other nodes.
	numVoter := 0
	ids := make(map[string]struct{})
	for i, nc := range allConf.Nodes {
		if _, ok := ids[nc.ID]; ok {
			return fmt.Errorf("duplicated node id: %s", nc.ID)
		}
		ids[nc.ID] = struct{}{}

		keys = allNodeKeys[i]
		if err := nc.tidy(dflt, hasKey, false); err != nil {
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

	md, err := toml.NewDecoder(f).Decode(&allConf)
	if err != nil {
		return err
	}

	keys := make([]string, 0, len(md.Keys()))
	for _, k := range md.Keys() {
		keys = append(keys, k.String())
	}

	if err = tidy(keys); err != nil {
		return err
	}

	if nodeID != "" {
		if curNodeConf = NodeByID(nodeID); curNodeConf == nil {
			return fmt.Errorf("configuration for node '%s' is missing", nodeID)
		}
	}

	return nil
}

// HandleListConf is an http handler to expose configurations.
func HandleListConf(w http.ResponseWriter, r *http.Request) {
	if strings.EqualFold(r.Header.Get("Accept"), "application/json") {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(&allConf)
	} else {
		w.Header().Set("Content-Type", "application/toml")
		toml.NewEncoder(w).Encode(&allConf)
	}
}
