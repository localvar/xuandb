package ast

import (
	"encoding/json"
	"net/http"

	"github.com/localvar/xuandb/pkg/meta"
)

type Statement interface {
	Execute(w http.ResponseWriter, r *http.Request) error
}

// CreateUserStatement represents a command for creating a new user.
type CreateUserStatement struct {
	meta.User
}

func (stmt *CreateUserStatement) Execute(w http.ResponseWriter, r *http.Request) error {
	if err := meta.CreateUser(&stmt.User); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

type DropUserStatement struct {
	Name string
}

func (stmt *DropUserStatement) Execute(w http.ResponseWriter, r *http.Request) error {
	if err := meta.DropUser(stmt.Name); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

type SetPasswordStatement struct {
	Name     string
	Password string
}

func (stmt *SetPasswordStatement) Execute(w http.ResponseWriter, r *http.Request) error {
	if err := meta.SetPassword(stmt.Name, stmt.Password); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

type ShowUserStatement struct {
}

func (stmt *ShowUserStatement) Execute(w http.ResponseWriter, r *http.Request) error {
	users := meta.Users()
	json.NewEncoder(w).Encode(users)
	return nil
}

type JoinNodeStatement struct {
	ID    string
	Addr  string
	Voter bool
}

func (stmt *JoinNodeStatement) Execute(w http.ResponseWriter, r *http.Request) error {
	if err := meta.AddNode(stmt.ID, stmt.Addr, stmt.Voter); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

type DropNodeStatement struct {
	ID string
}

func (stmt *DropNodeStatement) Execute(w http.ResponseWriter, r *http.Request) error {
	if err := meta.DropNode(stmt.ID); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

type ShowNodeStatement struct {
}

func (stmt *ShowNodeStatement) Execute(w http.ResponseWriter, r *http.Request) error {
	nodes := meta.NodeStatuses()
	json.NewEncoder(w).Encode(nodes)
	return nil
}

type CreateDatabaseStatement struct {
	meta.Database
}

func (stmt *CreateDatabaseStatement) Execute(w http.ResponseWriter, r *http.Request) error {
	if err := meta.CreateDatabase(&stmt.Database); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

type DropDatabaseStatement struct {
	Name string
}

func (stmt *DropDatabaseStatement) Execute(w http.ResponseWriter, r *http.Request) error {
	if err := meta.DropDatabase(stmt.Name); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

type ShowDatabaseStatement struct {
}

func (stmt *ShowDatabaseStatement) Execute(w http.ResponseWriter, r *http.Request) error {
	dbs := meta.Databases()
	json.NewEncoder(w).Encode(dbs)
	return nil
}

type Expr interface {
}

type IntExpr struct {
	Value uint64
}

type FloatExpr struct {
	Value float64
}

type StringExpr struct {
	Value string
}

type AddExpr struct {
	Left  Expr
	Right Expr
}
