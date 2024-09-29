package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net/http/pprof"
	"os"
	"os/signal"
	"syscall"

	"github.com/localvar/xuandb/pkg/conf"
	"github.com/localvar/xuandb/pkg/httpserver"
	"github.com/localvar/xuandb/pkg/logger"
	"github.com/localvar/xuandb/pkg/services/meta"
	"github.com/localvar/xuandb/pkg/services/query"
	"github.com/localvar/xuandb/pkg/version"
)

// registerPprofHandlers registers the pprof handlers.
func registerPprofHandlers() {
	httpserver.HandleFunc("GET /debug/pprof/", pprof.Index)
	httpserver.HandleFunc("GET /debug/pprof/cmdline", pprof.Cmdline)
	httpserver.HandleFunc("GET /debug/pprof/profile", pprof.Profile)
	httpserver.HandleFunc("GET /debug/pprof/symbol", pprof.Symbol)
	httpserver.HandleFunc("GET /debug/pprof/trace", pprof.Trace)
}

func main() {
	flag.Parse()

	if conf.ShowVersion() {
		fmt.Println("xuandb server version:", version.Version())
		fmt.Println("Built with:", version.GoVersion())
		fmt.Println("Git commit:", version.Revision())
		if version.LocalModified() {
			fmt.Println("Warning: this build contains uncommitted changes.")
		}
		return
	}

	if err := conf.LoadServer(); err != nil {
		fmt.Fprintln(os.Stderr, "failed to load configuration:", err.Error())
		return
	}

	logger.Init()

	// add an http handler to expose configurations, we cannot do this in the
	// conf package.
	httpserver.HandleFunc("/debug/config", conf.HandleListConf)

	if conf.CurrentNode().EnablePprof == "true" {
		registerPprofHandlers()
	}

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
