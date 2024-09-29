// 'xuandb_editor' is to satisfy code editors, not used in the build process.
// e.g. in vscode, add the following to settings.json to make the editor happy.
//        "gopls": {
//                "build.buildFlags": ["-tags=xuandb_editor"]
//        }
//
//go:build xuandb_server || xuandb_editor

package conf

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
)

// nodeID is the ID of current node.
var nodeID string

// curNodeConf contains configuration for current node.
var curNodeConf *NodeConf

func init() {
	flag.StringVar(&nodeID, "node-id", "", "id of this node.")
}

// NodeID returns the ID of current node.
func NodeID() string {
	return nodeID
}

// CurrentNode returns configuration for current node.
func CurrentNode() *NodeConf {
	return curNodeConf
}

// LoadServer loads server configurations from file, environment variables.
// It also fills the default values and normalizes the configurations.
func LoadServer() error {
	if nodeID == "" {
		nodeID = os.Getenv("XUANDB_NODE_ID")
	}
	if nodeID == "" {
		return errors.New("node id is missing from both command line and environment variable")
	}

	if err := load(); err != nil {
		return err
	}

	curNodeConf = NodeByID(nodeID)
	if curNodeConf == nil {
		return fmt.Errorf("missing configuration for current node: %s", nodeID)
	}

	// add an http handler to expose configurations.
	http.HandleFunc("/debug/config", handleListConf)

	return nil
}

func handleListConf(w http.ResponseWriter, r *http.Request) {
	if strings.EqualFold(r.Header.Get("Accept"), "application/json") {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(&allConf)
	} else {
		w.Header().Set("Content-Type", "application/toml")
		toml.NewEncoder(w).Encode(&allConf)
	}
}
