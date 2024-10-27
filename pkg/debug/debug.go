package debug

import (
	"net/http"
	"net/http/pprof"

	"github.com/localvar/xuandb/pkg/config"
	"github.com/localvar/xuandb/pkg/httpserver"
	"github.com/localvar/xuandb/pkg/logger"
	"github.com/localvar/xuandb/pkg/meta"
	"github.com/localvar/xuandb/pkg/xerrors"
)

// auth wraps the input http.HandlerFunc to a new http.HandlerFunc which
// authenticates and authorizes the request for debug operations.
func auth(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name, pwd, _ := r.BasicAuth()

		rp := meta.RequiredPrivileges{Global: meta.PrivilegeDebug}
		if err := meta.Auth(name, pwd, rp); err != nil {
			se := err.(*xerrors.StatusError)
			http.Error(w, se.Msg, se.StatusCode)
			return
		}

		handler(w, r)
	}
}

// Init initializes the debug package.
func Init() {
	// add an http handler to expose configurations.
	httpserver.HandleFunc("GET /debug/config", auth(config.HandleList))

	httpserver.HandleFunc("GET /debug/logger/level", auth(logger.HandleGetLevel))
	httpserver.HandleFunc("POST /debug/logger/level", auth(logger.HandleSetLevel))

	// registers the pprof handlers.
	if config.CurrentNode().EnablePprof {
		httpserver.HandleFunc("GET /debug/pprof/", auth(pprof.Index))
		httpserver.HandleFunc("GET /debug/pprof/cmdline", auth(pprof.Cmdline))
		httpserver.HandleFunc("GET /debug/pprof/profile", auth(pprof.Profile))
		httpserver.HandleFunc("GET /debug/pprof/symbol", auth(pprof.Symbol))
		httpserver.HandleFunc("GET /debug/pprof/trace", auth(pprof.Trace))
	}
}
