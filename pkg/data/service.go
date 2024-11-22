package data

import (
	"log/slog"
	"sync"

	"github.com/localvar/xuandb/pkg/meta"
)

type Database struct {
	Name string
}

var databases = sync.Map{}

func handleCreateDatabase(db *meta.Database) {
	db1 := &Database{Name: db.Name}
	databases.LoadOrStore(db.Name, db1)
}

func handleDropDatabase(name string) {
	databases.Delete(name)
}

// StartService starts the data service.
func StartService() error {
	for _, db := range meta.Databases() {
		handleCreateDatabase(db)
	}

	meta.DatabaseInformer().AddCreateHandler(handleCreateDatabase)
	meta.DatabaseInformer().AddDropHandler(handleDropDatabase)
	slog.Info("data service started")
	return nil
}

// ShutdownService shuts down the data service.
func ShutdownService() {
	slog.Info("data service stopped")
}
