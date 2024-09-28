package logger

import (
	"context"
	"io"
	"log"
	"log/slog"

	"github.com/hashicorp/go-hclog"
)

// hcLevel2slog maps hclog levels to slog levels
var hclLevel2slog = map[hclog.Level]slog.Level{
	hclog.NoLevel: slog.LevelDebug - 8,
	hclog.Trace:   slog.LevelDebug - 4,
	hclog.Debug:   slog.LevelDebug,
	hclog.Info:    slog.LevelInfo,
	hclog.Warn:    slog.LevelWarn,
	hclog.Error:   slog.LevelError,
	hclog.Off:     slog.LevelError + 4,
}

// hashiCorpLogger is an adapter to use log/slog.Logger as hclog.Logger.
// Some methods of hclog.Logger are not implemented, calling them result in a
// panic.
type hashiCorpLogger struct {
	*slog.Logger
}

// HashCorp wraps 'logger' and returns a hclog.Logger, it wraps 'slog.Default()'
// if 'logger' is nil.
func HashiCorp(logger *slog.Logger) hclog.Logger {
	if logger == nil {
		logger = slog.Default()
	}
	return hashiCorpLogger{Logger: logger}
}

func (hcl hashiCorpLogger) Log(level hclog.Level, msg string, args ...any) {
	lvl, ok := hclLevel2slog[level]
	if !ok {
		panic("unknown hclog level: " + level.String())
	}
	lowLevelLog(hcl.Logger, lvl, msg, args...)
}

func (hcl hashiCorpLogger) Trace(msg string, args ...any) {
	lowLevelLog(hcl.Logger, slog.LevelDebug-4, msg, args...)
}

func (hcl hashiCorpLogger) IsTrace() bool {
	return hcl.Enabled(context.Background(), slog.LevelDebug-4)
}

func (hcl hashiCorpLogger) IsDebug() bool {
	return hcl.Enabled(context.Background(), slog.LevelDebug)
}

func (hcl hashiCorpLogger) IsInfo() bool {
	return hcl.Enabled(context.Background(), slog.LevelInfo)
}

func (hcl hashiCorpLogger) IsWarn() bool {
	return hcl.Enabled(context.Background(), slog.LevelWarn)
}

func (hcl hashiCorpLogger) IsError() bool {
	return hcl.Enabled(context.Background(), slog.LevelError)
}

func (hcl hashiCorpLogger) ImpliedArgs() []any {
	panic("not implemented")
}

func (hcl hashiCorpLogger) With(args ...any) hclog.Logger {
	return &hashiCorpLogger{Logger: hcl.Logger.With(args...)}
}

func (hcl hashiCorpLogger) Name() string {
	panic("not implemented")
}

func (hcl hashiCorpLogger) Named(name string) hclog.Logger {
	panic("not implemented")
}

func (hcl hashiCorpLogger) ResetNamed(name string) hclog.Logger {
	panic("not implemented")
}

func (hcl hashiCorpLogger) SetLevel(level hclog.Level) {
	// no-op as we don't support set level
}

func (hcl hashiCorpLogger) GetLevel() hclog.Level {
	for lvl := hclog.NoLevel; lvl < hclog.Off; lvl++ {
		l := hclLevel2slog[lvl]
		if hcl.Enabled(context.Background(), l) {
			return lvl
		}
	}
	return hclog.Off
}

func (hcl hashiCorpLogger) StandardLogger(opts *hclog.StandardLoggerOptions) *log.Logger {
	panic("not implemented")
}

func (hcl hashiCorpLogger) StandardWriter(opts *hclog.StandardLoggerOptions) io.Writer {
	panic("not implemented")
}
