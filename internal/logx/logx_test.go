package logx

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseLevels(t *testing.T) {
	l, err := ParseLevels("info, resolve=debug ,mcp=trace")
	if err != nil {
		t.Fatal(err)
	}
	if l.Default != slog.LevelInfo || l.Areas["resolve"] != slog.LevelDebug || l.Areas["mcp"] != LevelTrace {
		t.Errorf("levels = %+v", l)
	}
	if l.Min() != LevelTrace {
		t.Errorf("Min = %v, want trace", l.Min())
	}
	if d, _ := ParseLevels(""); d.Default != slog.LevelInfo {
		t.Errorf("empty spec default = %v", d.Default)
	}
	if _, err := ParseLevels("loud"); err == nil {
		t.Error("want an error for an unknown level")
	}
}

// TestNewHandler_AreaLevels: a logger tagged with an area uses that area's
// level; other loggers use the default.
func TestNewHandler_AreaLevels(t *testing.T) {
	var buf bytes.Buffer
	l, _ := ParseLevels("info,resolve=debug")
	base := slog.New(NewHandler(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: l.Min()}), l))
	resolve := base.With(AreaKey, "resolve")
	parse := base.With(AreaKey, "parse")
	resolve.Debug("resolve-debug")
	parse.Debug("parse-debug")
	base.Debug("base-debug")
	parse.Info("parse-info")
	out := buf.String()
	if !strings.Contains(out, "resolve-debug") || !strings.Contains(out, "parse-info") {
		t.Errorf("missing records:\n%s", out)
	}
	if strings.Contains(out, "parse-debug") || strings.Contains(out, "base-debug") {
		t.Errorf("debug records outside the debug area were kept:\n%s", out)
	}
	if resolve.Enabled(context.Background(), LevelTrace) {
		t.Error("trace enabled for an area set to debug")
	}
}

func TestRunLogFile_KeepsNewest(t *testing.T) {
	dir := t.TempDir()
	start := time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC)
	var paths []string
	for i := 0; i < 5; i++ {
		p, err := RunLogFile(dir, "build", 3, start.Add(time.Duration(i)*time.Second))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("{}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, p)
	}
	left, _ := filepath.Glob(filepath.Join(dir, "build-*.jsonl"))
	if len(left) != 3 || left[len(left)-1] != paths[4] || left[0] != paths[2] {
		t.Errorf("kept %v, want the newest three", left)
	}
}
