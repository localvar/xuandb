package httpserver

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/localvar/xuandb/pkg/conf"
)

var (
	// Shutdown is a function to stop the http server.
	Shutdown func()
	serveMux http.ServeMux
)

// Start starts the http server.
func Start() {
	srv := &http.Server{Addr: conf.CurrentNode().HTTPAddr, Handler: &serveMux}

	Shutdown = func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		srv.Shutdown(ctx)
		cancel()
		slog.Info("http server stopped")
	}

	go func() {
		err := srv.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			slog.Error(
				"http server stopped unexpectly",
				slog.String("error", err.Error()),
			)
			os.Exit(1)
		}
	}()

	slog.Info("http server started", slog.String("address", srv.Addr))
}

// Handle registers the handler for the given pattern in [serveMux].
func Handle(pattern string, handler http.Handler) {
	serveMux.Handle(pattern, handler)
}

// HandleFunc registers the handler function for the given pattern in [serveMux].
func HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	serveMux.HandleFunc(pattern, handler)
}
