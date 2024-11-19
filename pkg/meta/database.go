package meta

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/raft"
	"github.com/localvar/xuandb/pkg/config"
	"github.com/localvar/xuandb/pkg/httpserver"
	"github.com/localvar/xuandb/pkg/xerrors"
)

// Errors for database operations.
var (
	ErrDatabaseExists    = xerrors.New(http.StatusConflict, "database already exists")
	ErrDatabaseNotExists = xerrors.New(http.StatusNotFound, "database does not exist")
)

// raft operation names for databases.
const (
	opCreateDatabase = "create-database"
	opDropDatabase   = "drop-database"
)

type dbInformer struct {
	wg             sync.WaitGroup
	ch             chan any
	createHandlers []func(*Database)
	dropHandlers   []func(string)
}

func (di *dbInformer) AddCreateHandler(handler func(*Database)) {
	di.createHandlers = append(di.createHandlers, handler)
}

func (di *dbInformer) AddDropHandler(handler func(string)) {
	di.dropHandlers = append(di.dropHandlers, handler)
}

func (di *dbInformer) inform(evt any) {
	di.ch <- evt
}

func (di *dbInformer) run() {
	di.wg.Add(1)
	go func() {
		defer di.wg.Done()
		for evt := range di.ch {
			switch cmd := evt.(type) {
			case *createDatabaseCommand:
				for _, handler := range di.createHandlers {
					handler(cmd.Database)
				}

			case *dropDatabaseCommand:
				for _, handler := range di.dropHandlers {
					handler(cmd.Name)
				}

			default:
				panic("unknown database event")
			}
		}
	}()
}

func (di *dbInformer) close() {
	close(di.ch)
	di.wg.Wait()
}

var databaseInformer *dbInformer

func dbInit() {
	// registers raft apply functions for database operations.
	registerRaftApplyFunc(opCreateDatabase, applyCreateDatabase)
	registerRaftApplyFunc(opDropDatabase, applyDropDatabase)

	databaseInformer = &dbInformer{ch: make(chan any, 10)}
	databaseInformer.run()
}

// databaseRegisterAPIHandlers registers API handlers for database operations.
func databaseRegisterAPIHandlers() {
	// only voters need to register API handlers.
	if !config.CurrentNode().Meta.RaftVoter {
		return
	}
	httpserver.HandleFunc("POST /meta/databases", handleCreateDatabase)
	httpserver.HandleFunc("DELETE /meta/databases", handleDropDatabase)
}

func dbUninit() {
	databaseInformer.close()
}

// Database represents a database.
type Database struct {
	Name     string        `json:"name"`
	Duration time.Duration `json:"duration"`
}

// handlers for the create database command.
type createDatabaseCommand struct {
	baseCommand
	*Database
}

// applyCreateDatabase applies the create database command.
func applyCreateDatabase(log *raft.Log) any {
	cmd := &createDatabaseCommand{}
	if err := json.Unmarshal(log.Data, cmd); err != nil {
		return err
	}

	md := svcInst.md
	key := strings.ToLower(cmd.Name)

	md.lock()
	defer md.unlock()

	if u := md.Databases[key]; u == nil {
		md.Databases[key] = cmd.Database
		return nil
	}

	databaseInformer.inform(cmd)
	return ErrDatabaseExists
}

func leaderCreateDatabase(db *Database) error {
	if DatabaseByName(db.Name) != nil {
		slog.Debug("database already exists", slog.String("name", db.Name))
		return ErrDatabaseExists
	}

	cmd := createDatabaseCommand{
		baseCommand: baseCommand{Op: opCreateDatabase},
		Database:    db,
	}
	err := svcInst.raftApply(cmd)
	if err == nil {
		slog.Info("database created", slog.String("name", db.Name))
		return nil
	}

	slog.Debug("create database failed", slog.String("error", err.Error()))
	return err
}

func handleCreateDatabase(w http.ResponseWriter, r *http.Request) {
	db := &Database{}

	if err := json.NewDecoder(r.Body).Decode(db); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if db.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	slog.Debug("create database command received", slog.String("name", db.Name))
	if err := leaderCreateDatabase(db); err != nil {
		se := err.(*xerrors.StatusError)
		http.Error(w, se.Msg, se.StatusCode)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func CreateDatabase(db *Database) error {
	if svcInst.isLeader() {
		return leaderCreateDatabase(db)
	}
	return sendPostRequestToLeader("/meta/databases", db)
}

// handlers for the drop database command.
type dropDatabaseCommand struct {
	baseCommand
	Name string `json:"name"`
}

func applyDropDatabase(log *raft.Log) any {
	cmd := &dropDatabaseCommand{}
	if err := json.Unmarshal(log.Data, cmd); err != nil {
		return err
	}

	md := svcInst.md
	key := strings.ToLower(cmd.Name)

	md.lock()
	delete(md.Databases, key)
	md.unlock()

	databaseInformer.inform(cmd)
	return nil
}

func leaderDropDatabase(name string) error {
	if DatabaseByName(name) == nil {
		slog.Debug("database does not exist", slog.String("name", name))
		return ErrDatabaseNotExists
	}

	cmd := dropDatabaseCommand{
		baseCommand: baseCommand{Op: opDropDatabase},
		Name:        name,
	}
	err := svcInst.raftApply(cmd)
	if err == nil {
		slog.Info("database dropped", slog.String("name", name))
		return nil
	}

	slog.Debug("drop database failed", slog.String("error", err.Error()))
	return err
}

func handleDropDatabase(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("name")
	if name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	slog.Debug("drop database command received", slog.String("name", name))
	if err := leaderDropDatabase(name); err != nil {
		se := err.(*xerrors.StatusError)
		http.Error(w, se.Msg, se.StatusCode)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DropDatabase drops a database.
func DropDatabase(name string) error {
	if svcInst.isLeader() {
		return leaderDropDatabase(name)
	}
	return sendDeleteRequestToLeader("/meta/databases?name=" + url.QueryEscape(name))
}

// Databases returns all databases. The result is sorted by name.
func Databases() []*Database {
	md := svcInst.md

	md.lock()
	result := make([]*Database, 0, len(md.Databases))
	for _, db := range md.Databases {
		result = append(result, db)
	}
	defer md.unlock()

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result
}

// DatabaseByName returns a database by name.
func DatabaseByName(name string) *Database {
	md := svcInst.md
	key := strings.ToLower(name)

	md.lock()
	defer md.unlock()
	return md.Databases[key]
}
