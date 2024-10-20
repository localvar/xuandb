package query

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/localvar/xuandb/pkg/httpserver"
	"github.com/localvar/xuandb/pkg/query/parser"
	"github.com/localvar/xuandb/pkg/xerrors"
)

func queryHandler(w http.ResponseWriter, r *http.Request) {
	q := r.FormValue("q")
	if q == "" {
		http.Error(w, "query statement is required", http.StatusBadRequest)
		return
	}

	stmt, err := parser.Parse(q)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	slog.Debug("query received", slog.String("query", q))
	/*
		if db := r.FormValue("db"); db != "" {
			stmt.BindDatabase(db)
		}
	*/

	name, pwd, _ := r.BasicAuth()
	if err := stmt.CheckPrivilege(name, pwd); err != nil {
		se := err.(*xerrors.StatusError)
		http.Error(w, se.Msg, se.StatusCode)
		return
	}

	res, err := stmt.Execute()
	if err == nil {
		if res == nil {
			w.WriteHeader(http.StatusNoContent)
		} else {
			json.NewEncoder(w).Encode(res)
		}
		return
	}

	slog.Error(
		"failed to execute query",
		slog.String("query", q),
		slog.String("error", err.Error()),
	)
	if se, ok := err.(*xerrors.StatusError); ok {
		http.Error(w, se.Msg, se.StatusCode)
	} else {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// StartService starts the query service.
func StartService() error {
	httpserver.HandleFunc("/query", queryHandler)
	return nil
}

// ShutdownService shuts down the query service.
func ShutdownService() {
	slog.Info("query service stopped")
}
