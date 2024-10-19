package meta

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/hashicorp/raft"
	"github.com/localvar/xuandb/pkg/config"
	"github.com/localvar/xuandb/pkg/httpserver"
	"github.com/localvar/xuandb/pkg/xerrors"
)

// Errors for user operations.
var (
	ErrUserExists      = xerrors.New(http.StatusConflict, "user already exists")
	ErrUserNotExists   = xerrors.New(http.StatusNotFound, "user does not exist")
	ErrCannotDropAdmin = xerrors.New(http.StatusForbidden, "cannot drop admin user")
)

// raft operation names for users.
const (
	opCreateUser  = "CreateUser"
	opDropUser    = "DropUser"
	opSetPassword = "SetPassword"
)

// registerUserHandlers registers handlers for user operations.
func registerUserHandlers() {
	registerDataApplyFunc(opCreateUser, applyCreateUser)
	registerDataApplyFunc(opDropUser, applyDropUser)
	registerDataApplyFunc(opSetPassword, applySetPassword)

	// only voters need to register HTTP handlers.
	if !config.CurrentNode().Meta.RaftVoter {
		return
	}
	httpserver.HandleFunc("POST /meta/users", handleCreateUser)
	httpserver.HandleFunc("PUT /meta/users", handleSetPassword)
	httpserver.HandleFunc("DELETE /meta/users", handleDropUser)
}

// User represents a user.
type User struct {
	Name     string `json:"name"`
	Password string `json:"password"`
	// the first user will be the administrator, and the 'admin' field is
	// read-only for clients.
	Admin bool `json:"admin"`
}

// handlers for the create user command.
type createUserCommand struct {
	baseCommand
	*User
}

func applyCreateUser(l *raft.Log) any {
	var cmd createUserCommand
	if err := json.Unmarshal(l.Data, &cmd); err != nil {
		return err
	}

	md := svcInst.md
	key := strings.ToLower(cmd.Name)

	md.lock()
	defer md.unlock()

	if u := md.Users[key]; u == nil {
		cmd.User.Admin = len(md.Users) == 0
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
	cmd := dropUserCommand{}
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
	} else if u.Admin {
		slog.Debug("cannot drop admin user", slog.String("name", name))
		return ErrCannotDropAdmin
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
	var cmd setPasswordCommand
	if err := json.Unmarshal(l.Data, &cmd); err != nil {
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

	cmd := setPasswordCommand{
		baseCommand: baseCommand{Op: opSetPassword},
		Name:        u.Name,
		Password:    u.Password,
	}
	err := svcInst.raftApply(&cmd)
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

// HashUser returns true if there is any user.
func HashUser() bool {
	md := svcInst.md
	md.lock()
	defer md.unlock()
	return len(md.Users) > 0
}

// UserByName returns a user by name. It returns nil if the user does not exist.
func UserByName(name string) *User {
	md := svcInst.md
	key := strings.ToLower(name)

	md.lock()
	defer md.unlock()
	return md.Users[key]
}
