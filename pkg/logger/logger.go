package logger

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/localvar/xuandb/pkg/config"
)

// lvlVar is a slog.LevelVar that can be used to get/set the minimal log level.
var lvlVar = &slog.LevelVar{}

// Init initialize a logger according to the configuration and set it as the
// slog.Default().
func Init() {
	lc := config.CurrentNode().Logger

	lvlVar.Set(lc.Level)
	opts := &slog.HandlerOptions{
		AddSource: lc.AddSource,
		Level:     lvlVar,
	}

	var w io.Writer
	switch strings.ToLower(lc.OutputTo) {
	case "stderr":
		w = os.Stderr
	case "stdout":
		w = os.Stdout
	case "discard":
		w = io.Discard
	default:
		// TODO: add file handler
		panic("not implemented")
	}

	var handler slog.Handler
	if lc.Format == "json" {
		handler = slog.NewJSONHandler(w, opts)
	} else {
		handler = slog.NewTextHandler(w, opts)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)

}

// HandleGetLevel is an http handler that returns the current minimal log level.
func HandleGetLevel(w http.ResponseWriter, r *http.Request) {
	if text, err := lvlVar.MarshalText(); err == nil {
		w.Write(text)
	} else {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// HandleSetLevel is an http handler that sets the minimal log level.
func HandleSetLevel(w http.ResponseWriter, r *http.Request) {
	val := r.FormValue("value")
	if err := lvlVar.UnmarshalText([]byte(val)); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
}

// lowLevelLog is the low-level logging function that wraps a slog logger, it
// is used to build adapters of slog for other logger packages like hclog.
func lowLevelLog(l *slog.Logger, level slog.Level, msg string, args ...any) {
	handler := l.Handler()
	if !handler.Enabled(context.Background(), level) {
		return
	}

	var pcs [1]uintptr
	// skip [runtime.Callers, this function, this function's caller]
	runtime.Callers(3, pcs[:])
	r := slog.NewRecord(time.Now(), level, msg, pcs[0])
	r.Add(args...)

	_ = handler.Handle(context.Background(), r)
}
