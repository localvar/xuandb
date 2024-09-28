package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/localvar/xuandb/pkg/conf"
	"github.com/localvar/xuandb/pkg/logger"
	"github.com/localvar/xuandb/pkg/services/meta"
	"github.com/localvar/xuandb/pkg/services/query"
	"github.com/localvar/xuandb/pkg/version"
)

var shutdownHTTPServer func()

func startHTTPServer() {
	srv := &http.Server{Addr: conf.CurrentNode().HTTPAddr}

	shutdownHTTPServer = func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		srv.Shutdown(ctx)
		cancel()
		slog.Info("http server stopped")
	}

	go func(srv *http.Server) {
		err := srv.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			slog.Error(
				"http server stopped unexpectly",
				slog.String("error", err.Error()),
			)
			os.Exit(1)
		}
	}(srv)

	slog.Info("http server started", slog.String("address", srv.Addr))
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

	logger.Init(&conf.Common().Logger)

	startHTTPServer()
	defer func() {
		shutdownHTTPServer()
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
