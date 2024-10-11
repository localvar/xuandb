package httpserver

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/localvar/xuandb/pkg/config"
)

var (
	mux = http.NewServeMux()
	svr = &http.Server{}
)

// Start starts the http server.
func Start() {
	go func() {
		svr.Addr = config.CurrentNode().HTTPAddr
		svr.Handler = mux

		err := svr.ListenAndServe()
		if err == nil || err == http.ErrServerClosed {
			return
		}

		slog.Error(
			"http server stopped unexpectly",
			slog.String("error", err.Error()),
		)
		os.Exit(1)
	}()

	slog.Info("http server started", slog.String("address", svr.Addr))
}

// Shutdown stops the http server.
func Shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	svr.Shutdown(ctx)
	cancel()
	slog.Info("http server stopped")
}

// Handle registers the handler for the given pattern in [mux].
func Handle(pattern string, handler http.Handler) {
	mux.Handle(pattern, handler)
}

// HandleFunc registers the handler function for the given pattern in [mux].
func HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	mux.HandleFunc(pattern, handler)
}
