// Package logx holds the logging conventions shared by the graph, vector and
// system engines (docs/graph/BUILD-AND-QUERY-FLOW.md): a TRACE level below
// DEBUG, log levels set per area from one environment value, and per-run JSON
// log files next to the data a run produces.
//
// A component tags its logger with an area once:
//
//	log := base.With(logx.AreaKey, "resolve")
//
// and the level that applies to its records is the one configured for that
// area, falling back to the default level.
package logx

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// LevelTrace is for per-item records (every reference, every visited node);
// it is meant for development runs and log files, not for stderr.
const LevelTrace = slog.Level(-8)

// AreaKey is the attribute that names the area of a logger.
const AreaKey = "area"

// Levels is a default level plus per-area overrides.
type Levels struct {
	Default slog.Level
	Areas   map[string]slog.Level
}

// ParseLevels reads "info", "debug" or "info,resolve=debug,mcp=trace". An
// empty spec is "info".
func ParseLevels(spec string) (Levels, error) {
	l := Levels{Default: slog.LevelInfo, Areas: map[string]slog.Level{}}
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		area, name, hasArea := strings.Cut(part, "=")
		if !hasArea {
			name, area = area, ""
		}
		lv, err := parseLevel(name)
		if err != nil {
			return Levels{}, fmt.Errorf("log level %q: %w", part, err)
		}
		if area == "" {
			l.Default = lv
		} else {
			l.Areas[strings.TrimSpace(area)] = lv
		}
	}
	return l, nil
}

func parseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "trace":
		return LevelTrace, nil
	case "debug":
		return slog.LevelDebug, nil
	case "info", "":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	}
	return 0, fmt.Errorf("unknown level %q (want trace, debug, info, warn or error)", s)
}

// Min returns the lowest level any area enables, for handlers that need one
// threshold.
func (l Levels) Min() slog.Level {
	m := l.Default
	for _, lv := range l.Areas {
		if lv < m {
			m = lv
		}
	}
	return m
}

// NewHandler wraps next so that records are kept when they reach the level of
// the logger's area (set with With(AreaKey, ...)), or the default level for a
// logger without an area. next should accept every level down to l.Min().
func NewHandler(next slog.Handler, l Levels) slog.Handler {
	return &areaHandler{next: next, levels: l, min: l.Default}
}

type areaHandler struct {
	next   slog.Handler
	levels Levels
	min    slog.Level
}

func (h *areaHandler) Enabled(ctx context.Context, lv slog.Level) bool {
	return lv >= h.min && h.next.Enabled(ctx, lv)
}

func (h *areaHandler) Handle(ctx context.Context, r slog.Record) error {
	if r.Level < h.min {
		return nil
	}
	return h.next.Handle(ctx, r)
}

func (h *areaHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	out := &areaHandler{next: h.next.WithAttrs(attrs), levels: h.levels, min: h.min}
	for _, a := range attrs {
		if a.Key != AreaKey {
			continue
		}
		if lv, ok := h.levels.Areas[a.Value.String()]; ok {
			out.min = lv
		} else {
			out.min = h.levels.Default
		}
	}
	return out
}

func (h *areaHandler) WithGroup(name string) slog.Handler {
	return &areaHandler{next: h.next.WithGroup(name), levels: h.levels, min: h.min}
}

// RunLogFile returns the path of a new per-run log file in dir,
// "<prefix>-<UTC time>.jsonl", creating dir, and removes the oldest files of
// the same prefix so that at most keep remain (keep <= 0 keeps all).
func RunLogFile(dir, prefix string, keep int, now time.Time) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("log dir: %w", err)
	}
	name := fmt.Sprintf("%s-%s.jsonl", prefix, now.UTC().Format("20060102T150405.000Z"))
	path := filepath.Join(dir, name)
	if keep > 0 {
		old, err := filepath.Glob(filepath.Join(dir, prefix+"-*.jsonl"))
		if err != nil {
			return "", err
		}
		sort.Strings(old) // the timestamp format sorts by time
		// keep-1 old files plus the new one.
		for len(old) > keep-1 {
			_ = os.Remove(old[0])
			old = old[1:]
		}
	}
	return path, nil
}

// DailyLogFile returns "<dir>/<prefix>-<UTC date>.jsonl" for servers that
// append to one file per day, creating dir.
func DailyLogFile(dir, prefix string, now time.Time) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("log dir: %w", err)
	}
	return filepath.Join(dir, fmt.Sprintf("%s-%s.jsonl", prefix, now.UTC().Format("20060102"))), nil
}
