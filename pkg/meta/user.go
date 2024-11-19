package meta

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/raft"
	"github.com/localvar/xuandb/pkg/config"
	"github.com/localvar/xuandb/pkg/httpserver"
	"github.com/localvar/xuandb/pkg/xerrors"
)

// Errors for user operations.
var (
	ErrUserExists    = xerrors.New(http.StatusConflict, "user already exists")
	ErrUserNotExists = xerrors.New(http.StatusNotFound, "user does not exist")
	ErrSystemUser    = xerrors.New(http.StatusForbidden, "user is a system user")

	ErrAuthRequired           = xerrors.New(http.StatusUnauthorized, "authorization required")
	ErrPasswordMismatch       = xerrors.New(http.StatusUnauthorized, "password mismatch or user not exists")
	ErrInsufficientPrivileges = xerrors.New(http.StatusForbidden, "insufficient privileges")
)

// raft operation names for users.
const (
	opCreateUser  = "create-user"
	opDropUser    = "drop-user"
	opSetPassword = "set-password"
)

// userRegisterRaftApplyFuncs registers raft apply functions for user operations.
func userRegisterRaftApplyFuncs() {
	registerRaftApplyFunc(opCreateUser, applyCreateUser)
	registerRaftApplyFunc(opDropUser, applyDropUser)
	registerRaftApplyFunc(opSetPassword, applySetPassword)
}

// userRegisterAPIHandlers registers API handlers for user operations.
func userRegisterAPIHandlers() {
	// only voters need to register API handlers.
	if !config.CurrentNode().Meta.RaftVoter {
		return
	}
	httpserver.HandleFunc("POST /meta/users", handleCreateUser)
	httpserver.HandleFunc("PUT /meta/users", handleSetPassword)
	httpserver.HandleFunc("DELETE /meta/users", handleDropUser)
}

// Privilege represents the privilege of a user.
type Privilege uint

const (
	// PrivilegeNone means no privilege.
	PrivilegeNone Privilege = 0

	// PrivilegeDebug allows the user to perform debug operations, it can
	// only be a global privilege and has no effect on databases.
	PrivilegeDebug Privilege = 1
	// PrivilegeRead allows the user to read data from a database.
	PrivilegeRead Privilege = 2
	// PrivilegeWrite allows the user to write data to a database.
	PrivilegeWrite Privilege = 4
	// PrivilegeMask is a mask that used to check if a privilege is valid.
	PrivilegeMask Privilege = 7

	// PrivilegeAdmin is a special privilege that has all common privileges,
	// including the privileges we may add in the future.
	PrivilegeAdmin Privilege = math.MaxInt + 1
)

func (p *Privilege) parse(str string) error {
	v := PrivilegeNone

	for len(str) > 0 {
		s := ""
		if idx := strings.IndexByte(str, ','); idx >= 0 {
			s, str = str[:idx], str[idx+1:]
		} else {
			s, str = str, ""
		}

		s = strings.ToUpper(strings.Trim(s, " \t"))
		switch s {
		case "", "NONE":
			// nop
		case "DEBUG":
			v |= PrivilegeDebug
		case "READ":
			v |= PrivilegeRead
		case "WRITE":
			v |= PrivilegeWrite
		case "ADMIN":
			v |= PrivilegeAdmin
		default:
			return fmt.Errorf("invalid privilege: %s", s)
		}
	}

	*p = v
	return nil
}

// String returns the string representation of a privilege.
func (p Privilege) String() string {
	if p == PrivilegeNone {
		return ""
	}

	if p == PrivilegeAdmin {
		return "ADMIN"
	}

	s := ""
	if p&PrivilegeDebug == PrivilegeDebug {
		s = "DEBUG"
	}
	if p&PrivilegeRead == PrivilegeRead {
		if s != "" {
			s += ",READ"
		} else {
			s = "READ"
		}
	}
	if p&PrivilegeWrite == PrivilegeWrite {
		if s != "" {
			s += ",WRITE"
		} else {
			s = "WRITE"
		}
	}

	return s
}

// MarshalJSON implements [encoding/json.Marshaler]
func (p Privilege) MarshalJSON() ([]byte, error) {
	return strconv.AppendQuote(nil, p.String()), nil
}

// UnmarshalJSON implements [encoding/json.Unmarshaler]
func (p *Privilege) UnmarshalJSON(data []byte) error {
	s, err := strconv.Unquote(string(data))
	if err != nil {
		return err
	}
	return p.parse(s)
}

// MarshalText implements [encoding.TextMarshaler]
func (p Privilege) MarshalText() ([]byte, error) {
	return []byte(p.String()), nil
}

// UnmarshalText implements [encoding.TextUnmarshaler].
func (p *Privilege) UnmarshalText(data []byte) error {
	return p.parse(string(data))
}

// User represents a user.
type User struct {
	Name      string    `json:"name"`
	Password  string    `json:"password"`
	CreatedAt time.Time `json:"createdAt"`

	// System marks a system user, system users cannot be dropped, and their
	// privileges cannot be changed. The first user created is a system user.
	System bool `json:"system"`

	// Priv is the global privileges of a user, when checking if a user has
	// the privilege to perform an operation on a database, this field is
	// checked first, which means if the user has this privilege, he has this
	// privilege on all databases.
	Priv Privilege `json:"privilege"`

	// DbPriv is the database privileges of a user.
	DbPriv map[string]Privilege `json:"dbPriv"`
}

// handlers for the create user command.
type createUserCommand struct {
	baseCommand
	*User
}

func applyCreateUser(l *raft.Log) any {
	cmd := &createUserCommand{}
	if err := json.Unmarshal(l.Data, cmd); err != nil {
		return err
	}

	md := svcInst.md
	key := strings.ToLower(cmd.Name)

	md.lock()
	defer md.unlock()

	if u := md.Users[key]; u == nil {
		if len(md.Users) == 0 {
			cmd.User.Priv = PrivilegeAdmin
			cmd.User.System = true
			slog.Info("system admin created", slog.String("name", cmd.User.Name))
		}
		md.Users[key] = cmd.User
		return nil
	}

	return ErrUserExists
}

func leaderCreateUser(u *User) error {
	if UserByName(u.Name) != nil {
		slog.Debug("user already exists", slog.String("name", u.Name))
		return ErrUserExists
	}

	u.CreatedAt = time.Now()
	cmd := createUserCommand{
		baseCommand: baseCommand{Op: opCreateUser},
		User:        u,
	}
	err := svcInst.raftApply(&cmd)
	if err == nil {
		slog.Info("user created", slog.String("name", u.Name))
		return nil
	}

	slog.Debug("create user failed", slog.String("error", err.Error()))
	return err
}

func handleCreateUser(w http.ResponseWriter, r *http.Request) {
	u := &User{}

	if err := json.NewDecoder(r.Body).Decode(u); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if u.Name == "" || u.Password == "" {
		http.Error(w, "name and password are required", http.StatusBadRequest)
		return
	}

	if (u.Priv != PrivilegeAdmin) && (u.Priv&^PrivilegeMask != 0) {
		http.Error(w, "invalid privilege", http.StatusBadRequest)
		return
	}

	slog.Debug("create user command received", slog.String("name", u.Name))
	if err := leaderCreateUser(u); err != nil {
		se := err.(*xerrors.StatusError)
		http.Error(w, se.Msg, se.StatusCode)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// CreateUser creates a user.
func CreateUser(u *User) error {
	u.System = false // clear the system flag
	if svcInst.isLeader() {
		return leaderCreateUser(u)
	}
	return sendPostRequestToLeader("/meta/users", u)
}

// handlers for the drop user command.
type dropUserCommand struct {
	baseCommand
	Name string `json:"name"`
}

func applyDropUser(l *raft.Log) any {
	cmd := &dropUserCommand{}
	if err := json.Unmarshal(l.Data, cmd); err != nil {
		return err
	}

	md := svcInst.md
	key := strings.ToLower(cmd.Name)

	// we have checked that the user is not the admin in leaderDropUser, and we
	// don't care if the user exists or not, so simply delete the user here.
	md.lock()
	delete(md.Users, key)
	md.unlock()

	return nil
}

func leaderDropUser(name string) error {
	if u := UserByName(name); u == nil {
		// treat the user does not exist as a successful operation.
		slog.Debug("user does not exist", slog.String("name", name))
		return nil
	} else if u.System {
		slog.Debug("cannot drop system user", slog.String("name", name))
		return ErrSystemUser
	}

	cmd := dropUserCommand{
		baseCommand: baseCommand{Op: opDropUser},
		Name:        name,
	}
	err := svcInst.raftApply(&cmd)
	if err == nil {
		slog.Info("user dropped", slog.String("name", name))
		return nil
	}

	if err == ErrUserNotExists {
		slog.Debug("user does not exist", slog.String("name", name))
		return nil
	}

	slog.Debug("drop user failed", slog.String("error", err.Error()))
	return err
}

func handleDropUser(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("name")
	if name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	slog.Debug("drop user command received", slog.String("name", name))
	if err := leaderDropUser(name); err != nil {
		se := err.(*xerrors.StatusError)
		http.Error(w, se.Msg, se.StatusCode)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DropUser drops a user.
func DropUser(name string) error {
	if svcInst.isLeader() {
		return leaderDropUser(name)
	}
	return sendDeleteRequestToLeader("/meta/users?name=" + url.QueryEscape(name))
}

// handlers for the set password command.
type setPasswordCommand struct {
	baseCommand
	Name     string `json:"name"`
	Password string `json:"password"`
}

func applySetPassword(l *raft.Log) any {
	cmd := &setPasswordCommand{}
	if err := json.Unmarshal(l.Data, cmd); err != nil {
		return err
	}

	md := svcInst.md
	key := strings.ToLower(cmd.Name)

	md.lock()
	defer md.unlock()

	if u := md.Users[key]; u != nil {
		u1 := *u
		u1.Password = cmd.Password
		md.Users[key] = &u1
		return nil
	}

	return ErrUserNotExists
}

func leaderSetPassword(u *User) error {
	// check if the user exists
	if UserByName(u.Name) == nil {
		slog.Debug("user not exists", slog.String("name", u.Name))
		return ErrUserNotExists
	}

	cmd := &setPasswordCommand{
		baseCommand: baseCommand{Op: opSetPassword},
		Name:        u.Name,
		Password:    u.Password,
	}
	err := svcInst.raftApply(cmd)
	if err == nil {
		slog.Info("set password succeeded", slog.String("name", u.Name))
		return nil
	}

	slog.Debug("set password failed", slog.String("error", err.Error()))
	return err
}

func handleSetPassword(w http.ResponseWriter, r *http.Request) {
	u := &User{}

	err := json.NewDecoder(r.Body).Decode(u)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if u.Name == "" || u.Password == "" {
		http.Error(w, "name and password are required", http.StatusBadRequest)
		return
	}

	slog.Debug("set password command received", slog.String("name", u.Name))
	if err = leaderSetPassword(u); err != nil {
		se := err.(*xerrors.StatusError)
		http.Error(w, se.Msg, se.StatusCode)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// SetPassword sets the password of a user.
func SetPassword(name, password string) error {
	u := &User{Name: name, Password: password}
	if svcInst.isLeader() {
		return leaderSetPassword(u)
	}
	return sendPutRequestToLeader("/meta/users", u)
}

// Users returns all users. The result is sorted by name.
func Users() []*User {
	md := svcInst.md

	md.lock()
	result := make([]*User, 0, len(md.Users))
	for _, u := range md.Users {
		result = append(result, u)
	}
	md.unlock()

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result
}

// UserByName returns a user by name. It returns nil if the user does not exist.
func UserByName(name string) *User {
	md := svcInst.md
	key := strings.ToLower(name)

	md.lock()
	defer md.unlock()
	return md.Users[key]
}

// getUser returns a user by name, noUser is true if no user has been created.
func getUser(name string) (u *User, noUser bool) {
	name = strings.ToLower(name)
	md := svcInst.md
	md.lock()
	u = md.Users[name]
	noUser = len(md.Users) == 0
	md.unlock()
	return
}

// RequiredPrivileges represents the required privileges of an operation.
type RequiredPrivileges struct {
	Global    Privilege
	Databases map[string]Privilege
}

// Auth does authentication and authorization.
func Auth(name, pwd string, rp RequiredPrivileges) error {
	u, noUser := getUser(name)
	if noUser {
		return nil
	}

	if len(name) == 0 {
		return ErrAuthRequired
	}

	if u == nil {
		return ErrPasswordMismatch
	}

	if subtle.ConstantTimeCompare([]byte(pwd), []byte(u.Password)) == 0 {
		return ErrPasswordMismatch
	}

	if u.Priv == PrivilegeAdmin {
		return nil
	}

	// don't use 'u.Priv&rp.Global != 0' because 'rp.Global' may be 0.
	if u.Priv&rp.Global != rp.Global {
		return ErrInsufficientPrivileges
	}

	for db, priv := range rp.Databases {
		if (u.Priv|u.DbPriv[db])&priv != priv {
			return ErrInsufficientPrivileges
		}
	}

	return nil
}
