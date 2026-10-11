package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestHandleIndex_RequiresMode(t *testing.T) {
	eng := buildSample(t)
	s := NewServer(eng)
	res, err := s.handleIndex(context.Background(),
		callRequest("cks.ops.index", map[string]any{"project_root": "/tmp"}))
	if err != nil {
		t.Fatalf("handleIndex: %v", err)
	}
	if !res.IsError {
		t.Error("expected error when mode is missing")
	}
}

func TestHandleIndex_RequiresProjectRoot(t *testing.T) {
	eng := buildSample(t)
	s := NewServer(eng)
	res, err := s.handleIndex(context.Background(),
		callRequest("cks.ops.index", map[string]any{"mode": "full"}))
	if err != nil {
		t.Fatalf("handleIndex: %v", err)
	}
	if !res.IsError {
		t.Error("expected error when project_root is missing")
	}
}

func TestHandleIndex_UnknownMode(t *testing.T) {
	eng := buildSample(t)
	s := NewServer(eng)
	res, err := s.handleIndex(context.Background(),
		callRequest("cks.ops.index", map[string]any{
			"mode":         "magic",
			"project_root": "/tmp",
		}))
	if err != nil {
		t.Fatalf("handleIndex: %v", err)
	}
	if !res.IsError {
		t.Error("expected error for unknown mode")
	}
}

func TestHandleIndex_FullModeRebuildsSample(t *testing.T) {
	eng := buildSample(t)
	s := NewServer(eng)
	srcAbs, _ := filepath.Abs(filepath.Join("..", "..", "..", "vector", "testdata", "sample"))

	res, err := s.handleIndex(context.Background(),
		callRequest("cks.ops.index", map[string]any{
			"mode":         "full",
			"project_root": srcAbs,
		}))
	if err != nil {
		t.Fatalf("handleIndex: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error: %s", textContent(t, res))
	}

	var m map[string]any
	if err := json.Unmarshal([]byte(textContent(t, res)), &m); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if m["mode"] != "full" {
		t.Errorf("mode = %v, want full", m["mode"])
	}
	if files, _ := m["files_processed"].(float64); files <= 0 {
		t.Errorf("files_processed = %v, want > 0", m["files_processed"])
	}
	if chunks, _ := m["chunks_created"].(float64); chunks <= 0 {
		t.Errorf("chunks_created = %v, want > 0", m["chunks_created"])
	}
	if _, ok := m["duration_ms"]; !ok {
		t.Error("expected duration_ms field")
	}
}

// ops.index may only rebuild the index from the source tree it serves; any
// other directory would replace the served index with that directory's
// contents, readable through search.
func TestHandleIndex_RefusesForeignProjectRoot(t *testing.T) {
	eng := buildSample(t)
	s := NewServer(eng)
	foreign := t.TempDir()
	if err := os.WriteFile(filepath.Join(foreign, "secret.go"), []byte("package secret\n\nconst Token = \"s3cr3t\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, root := range []string{foreign, "/", eng.Manifest().SrcRoot + "/.."} {
		res, err := s.handleIndex(context.Background(), callRequest("cks.ops.index", map[string]any{"mode": "full", "project_root": root}))
		if err != nil {
			t.Fatal(err)
		}
		if !res.IsError {
			t.Fatalf("project_root %q accepted", root)
		}
	}
	res, _ := s.handleIndex(context.Background(), callRequest("cks.ops.index", map[string]any{"mode": "full", "project_root": eng.Manifest().SrcRoot + "/."}))
	if res.IsError {
		t.Fatalf("served source root refused: %s", textContent(t, res))
	}
}
