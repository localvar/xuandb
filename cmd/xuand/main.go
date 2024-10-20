package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/localvar/xuandb/pkg/config"
	"github.com/localvar/xuandb/pkg/debug"
	"github.com/localvar/xuandb/pkg/httpserver"
	"github.com/localvar/xuandb/pkg/logger"
	"github.com/localvar/xuandb/pkg/meta"
	"github.com/localvar/xuandb/pkg/query"
	"github.com/localvar/xuandb/pkg/version"
)

func main() {
	var nodeID string
	flag.StringVar(&nodeID, "node-id", "", "id of this node.")
	flag.Parse()

	if config.ShowVersion() {
		fmt.Println("xuandb server version:", version.Version())
		fmt.Println("Built with:", version.GoVersion())
		fmt.Println("Git commit:", version.Revision())
		if version.LocalModified() {
			fmt.Println("Warning: this build contains uncommitted changes.")
		}
		return
	}

	if nodeID == "" {
		if nodeID = os.Getenv("XUANDB_NODE_ID"); nodeID == "" {
			fmt.Println("command line argument 'node-id' is required")
		}
	}

	if err := config.Load(nodeID); err != nil {
		fmt.Fprintln(os.Stderr, "failed to load configuration:", err.Error())
		return
	}

	logger.Init()
	debug.Init()

	httpserver.Start()
	defer func() {
		httpserver.Shutdown()
		slog.Info("xuandb stopped.")
	}()

	if err := meta.StartService(); err != nil {
		slog.Error(
			"failed to start meta service.",
			slog.String("error", err.Error()),
		)
		return
	}
	defer meta.ShutdownService()

	if err := query.StartService(); err != nil {
		slog.Error(
			"failed to start query service.",
			slog.String("error", err.Error()),
		)
		return
	}
	defer query.ShutdownService()

	slog.Info("xuandb started.")

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	<-signals
	slog.Info("xuandb stopping...")
}
