package meta

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/hashicorp/raft"
	"github.com/localvar/xuandb/pkg/httpserver"
	"github.com/localvar/xuandb/pkg/utils"
)

// registerUserAPIHandlers registers the user API handlers.
func registerUserAPIHandlers() {
	httpserver.HandleFunc("POST /meta/user", handleCreateUser)
	httpserver.HandleFunc("PUT /meta/user", handleSetPassword)
	httpserver.HandleFunc("DELETE /meta/user", handleDropUser)
}

// User represents a user.
type User struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

// handlers for the create user command.
type createUserCommand struct {
	baseCommand
	User
}

func leaderCreateUser(u *User) error {
	s := svcInst
	md := s.metadata

	// check if the user already exists for the 1st time.
	s.lockMetadata()
	u1 := md.Users[strings.ToLower(u.Name)]
	s.unlockMetadata()
	if u1 != nil {
		return &utils.StatusError{
			Code: http.StatusConflict,
			Msg:  fmt.Sprintf("user already exists: %s", u.Name),
		}
	}

	cmd := createUserCommand{
		baseCommand: baseCommand{Op: opCreateUser},
		User:        *u,
	}

	return s.raftApply(&cmd)
}

func handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var u User

	err := json.NewDecoder(r.Body).Decode(&u)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if u.Name == "" || u.Password == "" {
		http.Error(w, "name and password are required", http.StatusBadRequest)
		return
	}

	slog.Info("create user command received", slog.String("name", u.Name))
	if err = leaderCreateUser(&u); err == nil {
		return
	}

	slog.Error("create user failed", slog.String("error", err.Error()))
	if se, ok := err.(*utils.StatusError); ok {
		http.Error(w, se.Msg, se.Code)
	} else {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func applyCreateUser(l *raft.Log) any {
	var cmd createUserCommand
	if err := json.Unmarshal(l.Data, &cmd); err != nil {
		return err
	}

	s := svcInst
	md := s.metadata

	// though we have made a check before, it is still possible that the user
	// already exists because of the 2 phase commit, so check again here.
	key := strings.ToLower(cmd.Name)
	s.lockMetadata()
	u := md.Users[key]
	if u == nil {
		md.Users[key] = &cmd.User
	}
	s.unlockMetadata()

	if u == nil {
		slog.Info("user created", slog.String("name", cmd.Name))
		return nil
	}

	return &utils.StatusError{
		Code: http.StatusConflict,
		Msg:  fmt.Sprintf("user already exists: %s", cmd.Name),
	}
}

// handlers for the drop user command.
type dropUserCommand struct {
	baseCommand
	Name string `json:"name"`
}

func leaderDropUser(name string) error {
	s := svcInst
	md := s.metadata
	key := strings.ToLower(name)

	// check if the user already exists for the 1st time.
	s.lockMetadata()
	u := md.Users[key]
	s.unlockMetadata()

	if u == nil {
		slog.Info("user does not exist", slog.String("name", name))
		return nil
	}

	cmd := dropUserCommand{
		baseCommand: baseCommand{Op: opDropUser},
		Name:        name,
	}
	return s.raftApply(&cmd)
}

func handleDropUser(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("name")
	if name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	slog.Info("drop user command received", slog.String("name", name))
	err := leaderDropUser(name)

	slog.Error("drop user failed", slog.String("error", err.Error()))
	if se, ok := err.(*utils.StatusError); ok {
		http.Error(w, se.Msg, se.Code)
	} else {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func applyDropUser(l *raft.Log) any {
	var cmd dropUserCommand
	if err := json.Unmarshal(l.Data, &cmd); err != nil {
		return err
	}

	s := svcInst
	md := s.metadata

	key := strings.ToLower(cmd.Name)
	s.lockMetadata()
	u := md.Users[key]
	if u != nil {
		delete(md.Users, key)
	}
	s.unlockMetadata()

	if u != nil {
		slog.Info("user dropped", slog.String("name", cmd.Name))
	} else {
		slog.Info("user does not exist", slog.String("name", cmd.Name))
	}

	return nil
}

// handlers for the set password command.
type setPasswordCommand struct {
	baseCommand
	User
}

func leaderSetPassword(u *User) error {
	s := svcInst
	md := s.metadata

	// check if the user exists
	s.lockMetadata()
	u1 := md.Users[strings.ToLower(u.Name)]
	s.unlockMetadata()
	if u1 == nil {
		return &utils.StatusError{
			Code: http.StatusNotFound,
			Msg:  fmt.Sprintf("user not exists: %s", u.Name),
		}
	}

	cmd := setPasswordCommand{
		baseCommand: baseCommand{Op: opSetPassword},
		User:        *u,
	}

	return s.raftApply(&cmd)
}

func handleSetPassword(w http.ResponseWriter, r *http.Request) {
	var u User

	err := json.NewDecoder(r.Body).Decode(&u)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if u.Name == "" || u.Password == "" {
		http.Error(w, "name and password are required", http.StatusBadRequest)
		return
	}

	slog.Info("set password command received", slog.String("name", u.Name))
	if err = leaderSetPassword(&u); err == nil {
		return
	}

	slog.Error("set password failed", slog.String("error", err.Error()))
	if se, ok := err.(*utils.StatusError); ok {
		http.Error(w, se.Msg, se.Code)
	} else {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func applySetPassword(l *raft.Log) any {
	var cmd setPasswordCommand
	if err := json.Unmarshal(l.Data, &cmd); err != nil {
		return err
	}

	s := svcInst
	md := s.metadata

	// though we have made a check before, it is still possible that the user
	// already exists because of the 2 phase commit, so check again here.
	key := strings.ToLower(cmd.Name)
	s.lockMetadata()
	u := md.Users[key]
	if u != nil {
		u.Password = cmd.Password
	}
	s.unlockMetadata()

	if u == nil {
		slog.Info("user not exists", slog.String("name", cmd.Name))
		return nil
	}

	return nil
}

// Users returns all users.
func Users() []User {
	s := svcInst
	md := s.metadata

	s.lockMetadata()
	result := make([]User, 0, len(md.Users))
	for _, u := range md.Users {
		result = append(result, *u)
	}
	s.unlockMetadata()

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result
}

// CreateUser creates a user.
func CreateUser(u *User) error {
	if svcInst.isLeader() {
		return leaderCreateUser(u)
	}
	return sendPostRequestToLeader("/meta/user", u)
}

// SetPassword sets the password of a user.
func SetPassword(name, password string) error {
	u := &User{Name: name, Password: password}
	if svcInst.isLeader() {
		return leaderSetPassword(u)
	}
	return sendPutRequestToLeader("/meta/user", u)
}

// DropUser drops a user.
func DropUser(name string) error {
	if svcInst.isLeader() {
		return leaderDropUser(name)
	}
	return sendDeleteRequestToLeader("/meta/user?name=" + url.QueryEscape(name))
}
