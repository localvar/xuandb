package ast

import (
	"github.com/localvar/xuandb/pkg/meta"
)

type ResultSet interface {
	SetError(error)
	SetColumns(...string)
	AddRow(...any) error
}

type Statement interface {
	Auth(name, pwd string) error
	Execute(rs ResultSet) error
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

func (stmt *CreateUserStatement) Execute(rs ResultSet) error {
	return meta.CreateUser(&stmt.User)
}

// DropUserStatement represents a command for dropping a user.
type DropUserStatement struct {
	adminStatement
	Name string
}

func (stmt *DropUserStatement) Execute(rs ResultSet) error {
	return meta.DropUser(stmt.Name)
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

func (stmt *SetPasswordStatement) Execute(rs ResultSet) error {
	return meta.SetPassword(stmt.Name, stmt.Password)
}

// ShowUserStatement represents a command for showing all users.
type ShowUserStatement struct {
	readStatement
}

func (stmt *ShowUserStatement) Execute(rs ResultSet) error {
	rs.SetColumns("name", "isSystem", "privileges")
	for _, u := range meta.Users() {
		err := rs.AddRow(u.Name, u.System, u.Priv.String())
		if err != nil {
			return err
		}
	}
	return nil
}

// JoinNodeStatement represents a command for adding a new node to the cluster.
type JoinNodeStatement struct {
	adminStatement
	ID    string
	Addr  string
	Voter bool
}

func (stmt *JoinNodeStatement) Execute(rs ResultSet) error {
	return meta.AddNode(stmt.ID, stmt.Addr, stmt.Voter)
}

// DropNodeStatement represents a command for removing a node from the cluster.
type DropNodeStatement struct {
	adminStatement
	ID string
}

func (stmt *DropNodeStatement) Execute(rs ResultSet) error {
	return meta.DropNode(stmt.ID)
}

// ShowNodeStatement represents a command for showing all nodes in the cluster.
type ShowNodeStatement struct {
	readStatement
}

func (stmt *ShowNodeStatement) Execute(rs ResultSet) error {
	rs.SetColumns("id", "addr", "role", "heartbeatTime", "isLeader", "state")
	for _, n := range meta.NodeStatuses() {
		err := rs.AddRow(
			n.ID,
			n.Addr,
			n.Role.String(),
			n.LastHeartbeatTime,
			n.Leader,
			n.State,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

// CreateDatabaseStatement represents a command for creating a new database.
type CreateDatabaseStatement struct {
	adminStatement
	meta.Database
}

func (stmt *CreateDatabaseStatement) Execute(rs ResultSet) error {
	return meta.CreateDatabase(&stmt.Database)
}

// DropDatabaseStatement represents a command for dropping a database.
type DropDatabaseStatement struct {
	adminStatement
	Name string
}

func (stmt *DropDatabaseStatement) Execute(rs ResultSet) error {
	return meta.DropDatabase(stmt.Name)
}

// ShowDatabaseStatement represents a command for showing all databases.
type ShowDatabaseStatement struct {
	readStatement
}

func (stmt *ShowDatabaseStatement) Execute(rs ResultSet) error {
	rs.SetColumns("name", "duration")
	for _, db := range meta.Databases() {
		err := rs.AddRow(db.Name, db.Duration)
		if err != nil {
			return err
		}
	}
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
