package debug

import (
	"net/http/pprof"

	"github.com/localvar/xuandb/pkg/config"
	"github.com/localvar/xuandb/pkg/httpserver"
	"github.com/localvar/xuandb/pkg/logger"
	"github.com/localvar/xuandb/pkg/meta"
)

// Init initializes the debug package.
func Init() {
	// add an http handler to expose configurations.
	httpserver.HandleFunc("/debug/config", meta.AuthForDebug(config.HandleList))

	httpserver.HandleFunc("GET /debug/logger/level", meta.AuthForDebug(logger.HandleGetLevel))
	httpserver.HandleFunc("POST /debug/logger/level", meta.AuthForDebug(logger.HandleSetLevel))

	// registers the pprof handlers.
	if config.CurrentNode().EnablePprof {
		httpserver.HandleFunc("GET /debug/pprof/", meta.AuthForDebug(pprof.Index))
		httpserver.HandleFunc("GET /debug/pprof/cmdline", meta.AuthForDebug(pprof.Cmdline))
		httpserver.HandleFunc("GET /debug/pprof/profile", meta.AuthForDebug(pprof.Profile))
		httpserver.HandleFunc("GET /debug/pprof/symbol", meta.AuthForDebug(pprof.Symbol))
		httpserver.HandleFunc("GET /debug/pprof/trace", meta.AuthForDebug(pprof.Trace))
	}
}
