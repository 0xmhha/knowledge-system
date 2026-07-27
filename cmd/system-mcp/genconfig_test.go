package main

import (
	"io"
	"path/filepath"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/system/config"
)

func TestRunGenConfig_WritesLoadableConfig(t *testing.T) {
	t.Parallel()
	out := filepath.Join(t.TempDir(), "cks.yaml")
	args := []string{
		"-out", out,
		"-name", "cks-stablenet",
		"-dataset-dir", "/data/pr-77",
		"-source-root", "/src/go-stablenet",
		"-sanitize-rules", "/policies/sanitization_rules.yaml",
	}
	if err := runGenConfig(args, io.Discard); err != nil {
		t.Fatalf("runGenConfig: %v", err)
	}

	cfg, err := config.Load(out)
	if err != nil {
		t.Fatalf("Load generated config: %v", err)
	}
	if cfg.Name != "cks-stablenet" {
		t.Errorf("Name = %q, want cks-stablenet", cfg.Name)
	}
	wantCKG := filepath.Join("/data/pr-77", "graph", "graph.db")
	if cfg.Backends.CKG.Path != wantCKG {
		t.Errorf("CKG.Path = %q, want %q", cfg.Backends.CKG.Path, wantCKG)
	}
	if cfg.Backends.CKV.Path != filepath.Join("/data/pr-77", "vector") {
		t.Errorf("CKV.Path = %q", cfg.Backends.CKV.Path)
	}
	if cfg.Listen.Transport != "http" {
		t.Errorf("Transport = %q, want http", cfg.Listen.Transport)
	}
}

func TestRunGenConfig_RequiresOut(t *testing.T) {
	t.Parallel()
	if err := runGenConfig([]string{"-dataset-dir", "/d"}, io.Discard); err == nil {
		t.Fatal("expected error when -out is omitted")
	}
}
