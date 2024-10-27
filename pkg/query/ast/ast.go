package ast

import (
	"github.com/localvar/xuandb/pkg/meta"
)

type Statement interface {
	Auth(name, pwd string) error
	Execute() (any, error)
}

// adminStatement represents a statement which requires the global admin
// privilege.
type adminStatement struct {
}

func (stmt *adminStatement) Auth(name, pwd string) error {
	rp := meta.RequiredPrivileges{Global: meta.PrivilegeAdmin}
	return meta.Auth(name, pwd, rp)
}

// readStatement represents a statement which requires the global read
// privilege.
type readStatement struct {
}

func (stmt *readStatement) Auth(name, pwd string) error {
	rp := meta.RequiredPrivileges{Global: meta.PrivilegeRead}
	return meta.Auth(name, pwd, rp)
}

// CreateUserStatement represents a command for creating a new user.
type CreateUserStatement struct {
	adminStatement
	meta.User
}

func (stmt *CreateUserStatement) Execute() (any, error) {
	if err := meta.CreateUser(&stmt.User); err != nil {
		return nil, err
	}
	return nil, nil
}

// DropUserStatement represents a command for dropping a user.
type DropUserStatement struct {
	adminStatement
	Name string
}

func (stmt *DropUserStatement) Execute() (any, error) {
	if err := meta.DropUser(stmt.Name); err != nil {
		return nil, err
	}
	return nil, nil
}

// SetPasswordStatement represents a command for setting a user's password.
type SetPasswordStatement struct {
	Name     string
	Password string
}

func (stmt *SetPasswordStatement) Auth(name, pwd string) error {
	rp := meta.RequiredPrivileges{Global: meta.PrivilegeAdmin}
	// A user can change his own password, in this case, we only need to call
	// 'meta.Auth' to check the password.
	if name == stmt.Name {
		rp.Global = meta.PrivilegeNone
	}
	return meta.Auth(name, pwd, rp)
}

func (stmt *SetPasswordStatement) Execute() (any, error) {
	if err := meta.SetPassword(stmt.Name, stmt.Password); err != nil {
		return nil, err
	}
	return nil, nil
}

// ShowUserStatement represents a command for showing all users.
type ShowUserStatement struct {
	readStatement
}

func (stmt *ShowUserStatement) Execute() (any, error) {
	return meta.Users(), nil
}

// JoinNodeStatement represents a command for adding a new node to the cluster.
type JoinNodeStatement struct {
	adminStatement
	ID    string
	Addr  string
	Voter bool
}

func (stmt *JoinNodeStatement) Execute() (any, error) {
	if err := meta.AddNode(stmt.ID, stmt.Addr, stmt.Voter); err != nil {
		return nil, err
	}
	return nil, nil
}

// DropNodeStatement represents a command for removing a node from the cluster.
type DropNodeStatement struct {
	adminStatement
	ID string
}

func (stmt *DropNodeStatement) Execute() (any, error) {
	if err := meta.DropNode(stmt.ID); err != nil {
		return nil, err
	}
	return nil, nil
}

// ShowNodeStatement represents a command for showing all nodes in the cluster.
type ShowNodeStatement struct {
	readStatement
}

func (stmt *ShowNodeStatement) Execute() (any, error) {
	return meta.NodeStatuses(), nil
}

// CreateDatabaseStatement represents a command for creating a new database.
type CreateDatabaseStatement struct {
	adminStatement
	meta.Database
}

func (stmt *CreateDatabaseStatement) Execute() (any, error) {
	if err := meta.CreateDatabase(&stmt.Database); err != nil {
		return nil, err
	}
	return nil, nil
}

// DropDatabaseStatement represents a command for dropping a database.
type DropDatabaseStatement struct {
	adminStatement
	Name string
}

func (stmt *DropDatabaseStatement) Execute() (any, error) {
	if err := meta.DropDatabase(stmt.Name); err != nil {
		return nil, err
	}
	return nil, nil
}

// ShowDatabaseStatement represents a command for showing all databases.
type ShowDatabaseStatement struct {
	readStatement
}

func (stmt *ShowDatabaseStatement) Execute() (any, error) {
	return meta.Databases(), nil
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
