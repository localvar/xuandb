package query

import (
	"log/slog"
	"net/http"

	"github.com/localvar/xuandb/pkg/parser"
	"github.com/localvar/xuandb/pkg/services/metaapi"
)

func handleCreateUser(stmt *parser.CreateUserStatement) error {
	u := metaapi.User{Name: stmt.Name, Password: stmt.Password}
	metaapi.AddUser(u)
	return nil
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

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// StartService starts the query service.
func StartService() error {
	http.Handle("/query", http.HandlerFunc(queryHandler))
	return nil
}

// ShutdownService shuts down the query service.
func ShutdownService() {
	slog.Info("query service stopped")
}
