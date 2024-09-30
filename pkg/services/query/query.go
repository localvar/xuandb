package query

import (
	"log/slog"
	"net/http"

	"github.com/localvar/xuandb/pkg/httpserver"
	"github.com/localvar/xuandb/pkg/parser"
	"github.com/localvar/xuandb/pkg/services/metaapi"
	"github.com/localvar/xuandb/pkg/utils"
)

func handleCreateUser(stmt *parser.CreateUserStatement) error {
	u := metaapi.User{Name: stmt.Name, Password: stmt.Password}
	return metaapi.AddUser(u)
}

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

	slog.Info("query request received")

	switch s := stmt.(type) {
	case *parser.CreateUserStatement:
		err = handleCreateUser(s)
	}

	if err == nil {
		return
	}

	if se, ok := err.(*utils.StatusError); ok {
		http.Error(w, se.Msg, se.Code)
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
