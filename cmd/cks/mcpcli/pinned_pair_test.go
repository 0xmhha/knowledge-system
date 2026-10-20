package mcpcli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/system/config"
)

func TestPinnedBackendPairCannotDowngradeToLegacy(t *testing.T) {
	root := t.TempDir()
	version := func(name string) string {
		dir := filepath.Join(root, name)
		for _, child := range []string{"graph", "vector"} {
			if err := os.MkdirAll(filepath.Join(dir, child), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		for _, child := range []string{"dataset-identity.json", "graph/graph.db"} {
			if err := os.WriteFile(filepath.Join(dir, child), []byte("{}"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		return dir
	}
	a, b := version("a"), version("b")
	alias := filepath.Join(root, "current")
	if err := os.Symlink(a, alias); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, graph, vector string
		refuse              bool
	}{
		{"same-version", filepath.Join(a, "graph/graph.db"), filepath.Join(a, "vector"), false},
		{"resolved-current", filepath.Join(alias, "graph/graph.db"), filepath.Join(a, "vector"), false},
		{"different-project-or-version", filepath.Join(a, "graph/graph.db"), filepath.Join(b, "vector"), true},
		{"pinned-graph-legacy-vector", filepath.Join(a, "graph/graph.db"), filepath.Join(root, "legacy-vector"), true},
		{"legacy-graph-pinned-vector", filepath.Join(root, "legacy-graph.db"), filepath.Join(b, "vector"), true},
		{"noncanonical-graph", filepath.Join(a, "graph/alternate.db"), filepath.Join(a, "vector"), true},
		{"noncanonical-vector", filepath.Join(a, "graph/graph.db"), filepath.Join(a, "vector/subdir"), true},
		{"legacy-independent", filepath.Join(root, "graph.db"), filepath.Join(root, "legacy-vector"), false},
		{"graph-only-diagnostics", filepath.Join(a, "graph/graph.db"), "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{}
			cfg.Backends.CKG.Path, cfg.Backends.CKV.Path = tc.graph, tc.vector
			if err := validatePinnedBackendPair(cfg); (err != nil) != tc.refuse {
				t.Fatalf("refuse=%v, error=%v", tc.refuse, err)
			}
			if tc.refuse {
				leases, err := pinDatasetReaders(cfg)
				if err == nil || len(leases) != 0 {
					t.Fatalf("invalid pair acquired leases: %v %v", leases, err)
				}
			}
		})
	}
	t.Run("symlink-identity", func(t *testing.T) {
		dir := version("forged")
		if err := os.Remove(filepath.Join(dir, "dataset-identity.json")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(a, "dataset-identity.json"), filepath.Join(dir, "dataset-identity.json")); err != nil {
			t.Fatal(err)
		}
		cfg := &config.Config{}
		cfg.Backends.CKG.Path, cfg.Backends.CKV.Path = filepath.Join(dir, "graph/graph.db"), filepath.Join(dir, "vector")
		if err := validatePinnedBackendPair(cfg); err == nil {
			t.Fatal("symlink identity admitted")
		}
	})
}
