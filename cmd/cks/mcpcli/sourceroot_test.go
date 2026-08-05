package mcpcli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/system/config"
)

func TestDeriveSourceRoot(t *testing.T) {
	dir := t.TempDir()
	graphDir := filepath.Join(dir, "graph")
	if err := os.MkdirAll(graphDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(graphDir, "manifest.json"),
		[]byte(`{"src_root":"/repo/go-stablenet","src_commit":"abc"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	// empty source_root → derived from the manifest.
	cfg := &config.Config{}
	cfg.Backends.CKG.Path = filepath.Join(graphDir, "graph.db")
	if got := deriveSourceRoot(cfg); got != "/repo/go-stablenet" {
		t.Errorf("derived = %q, want /repo/go-stablenet", got)
	}
	if cfg.Backends.CKG.SourceRoot != "/repo/go-stablenet" {
		t.Errorf("cfg.SourceRoot = %q, want it set from the manifest", cfg.Backends.CKG.SourceRoot)
	}

	// already configured → left untouched, no derivation.
	cfg2 := &config.Config{}
	cfg2.Backends.CKG.Path = filepath.Join(graphDir, "graph.db")
	cfg2.Backends.CKG.SourceRoot = "/explicit/override"
	if got := deriveSourceRoot(cfg2); got != "" {
		t.Errorf("configured source_root should not be overridden; got derived=%q", got)
	}
	if cfg2.Backends.CKG.SourceRoot != "/explicit/override" {
		t.Errorf("explicit source_root changed to %q", cfg2.Backends.CKG.SourceRoot)
	}

	// missing manifest → no derivation, no error.
	cfg3 := &config.Config{}
	cfg3.Backends.CKG.Path = filepath.Join(dir, "nope", "graph.db")
	if got := deriveSourceRoot(cfg3); got != "" || cfg3.Backends.CKG.SourceRoot != "" {
		t.Errorf("missing manifest should derive nothing; got %q", cfg3.Backends.CKG.SourceRoot)
	}
}
