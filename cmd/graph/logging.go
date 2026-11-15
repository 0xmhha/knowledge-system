package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/0xmhha/knowledge-system/internal/logx"
)

// multiHandler fans out log records to two slog.Handler instances.
// Used to write JSON to a log file while also writing text to stderr.
type multiHandler struct {
	a, b slog.Handler
}

func (m *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return m.a.Enabled(ctx, level) || m.b.Enabled(ctx, level)
}

func (m *multiHandler) Handle(ctx context.Context, r slog.Record) error {
	// Both handlers must be attempted even if the first returns an error
	// (e.g. file handler errors on disk-full should not suppress stderr output).
	var firstErr error
	if m.a.Enabled(ctx, r.Level) {
		firstErr = m.a.Handle(ctx, r.Clone())
	}
	if m.b.Enabled(ctx, r.Level) {
		if err := m.b.Handle(ctx, r.Clone()); firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (m *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &multiHandler{a: m.a.WithAttrs(attrs), b: m.b.WithAttrs(attrs)}
}

func (m *multiHandler) WithGroup(name string) slog.Handler {
	return &multiHandler{a: m.a.WithGroup(name), b: m.b.WithGroup(name)}
}

// newLogger builds an slog.Logger from CLI flags and the CKG_LOG_LEVEL env var.
//
// CKG_LOG_LEVEL takes a default level and per-area overrides, for example
// "info,resolve=debug,mcp=trace" (internal/logx); --verbose sets the default
// to debug. Areas are set by components with log.With(logx.AreaKey, name).
//
// When logFile is non-empty the logger tees output:
//   - log file: JSON (machine-readable, one record per line), every level
//     the areas enable, trace included
//   - stderr:   text (human-readable), debug at most, never trace
//
// When logFile is empty only the text handler on stderr is used.
//
// The returned cleanup func closes the log file if one was opened.
// Callers must defer the cleanup call.
func newLogger(verbose bool, logFile string) (*slog.Logger, func(), error) {
	spec := os.Getenv("CKG_LOG_LEVEL")
	if verbose {
		spec = "debug," + spec
	}
	levels, err := logx.ParseLevels(spec)
	if err != nil {
		return nil, func() {}, err
	}
	if verbose {
		levels.Default = slog.LevelDebug
	}
	stderrLevel := levels.Min()
	if stderrLevel < slog.LevelDebug {
		stderrLevel = slog.LevelDebug
	}
	stderrHandler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: stderrLevel})

	if logFile == "" {
		return slog.New(logx.NewHandler(stderrHandler, levels)), func() {}, nil
	}

	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, func() {}, err
	}
	fileHandler := slog.NewJSONHandler(f, &slog.HandlerOptions{Level: levels.Min()})
	h := logx.NewHandler(&multiHandler{a: fileHandler, b: stderrHandler}, levels)
	cleanup := func() { _ = f.Close() }
	return slog.New(h), cleanup, nil
}

// buildLogFile is the default log file of a build: a new JSON file per run in
// <out>/logs, the newest 20 kept. An explicit --log-file wins.
func buildLogFile(explicit, outDir string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	return logx.RunLogFile(filepath.Join(outDir, "logs"), "build", 20, time.Now())
}

// serverLogFile is the default log file of a long-running server (ckg mcp):
// one JSON file per day in the user cache directory, not in the graph
// directory, because servers open graphs read-only and a graph directory can
// be a published dataset. An explicit --log-file wins; without a cache
// directory the server logs to stderr only.
func serverLogFile(explicit, prefix string) string {
	if explicit != "" {
		return explicit
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	p, err := logx.DailyLogFile(filepath.Join(cache, "knowledge-system", "logs"), prefix, time.Now())
	if err != nil {
		return ""
	}
	return p
}
