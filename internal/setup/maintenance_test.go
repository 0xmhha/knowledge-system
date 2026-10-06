package setup

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestMaintenanceTargetChecksPartialPinsAndAliases(t *testing.T) {
	for _, marker := range []string{"dataset-identity.json", "sources/manifest.json", "sources/blobs/retained", "graph/manifest.json", "vector/manifest.json"} {
		t.Run(marker, func(t *testing.T) {
			root := t.TempDir()
			version := filepath.Join(root, "v1")
			path := filepath.Join(version, marker)
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(`{"snapshot_id":"partial-pin"}`), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("v1", filepath.Join(root, "current")); err != nil {
				t.Fatal(err)
			}
			for _, target := range []string{version, filepath.Join(version, "vector", "new.db"), filepath.Join(root, "current", "new-child")} {
				if err := CheckMutableTarget(target); !errors.Is(err, ErrVersionedSetupRequired) {
					t.Fatalf("partial pin/alias accepted %s: %v", target, err)
				}
			}
			if err := CheckLegacyReindex(root); !errors.Is(err, ErrVersionedSetupRequired) {
				t.Fatalf("unpinned downgrade accepted: %v", err)
			}
		})
	}
}

func TestMaintenanceTargetAllowsExistingFlatLegacyDB(t *testing.T) {
	root := t.TempDir()
	writeAlignedVersion(t, root)
	path := filepath.Join(root, "graph", "graph.db")
	if err := os.WriteFile(path, []byte("legacy db"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{root, path, filepath.Join(root, "vector")} {
		if err := CheckLegacyMaintenanceTarget(target); err != nil {
			t.Fatalf("flat legacy refused: %v", err)
		}
	}
}

func TestPlanRechecksImmutableTargetBeforeFirstWrite(t *testing.T) {
	root := t.TempDir()
	plan, err := BuildPlan(Options{Src: t.TempDir(), Out: root})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "dataset-identity.json"), []byte("damaged"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := Execute(context.Background(), plan, buildRunner{t: t}, nil); !errors.Is(err, ErrVersionedSetupRequired) {
		t.Fatalf("plan executed after pin appeared: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "graph")); !os.IsNotExist(err) {
		t.Fatal("guard wrote graph output")
	}
}

func TestReindexRefusesExistingLegacyRollbackVersion(t *testing.T) {
	root := t.TempDir()
	writeAlignedVersion(t, filepath.Join(root, "v1"))
	if err := os.Symlink("v1", filepath.Join(root, "current")); err != nil {
		t.Fatal(err)
	}
	old, err := os.ReadFile(filepath.Join(root, "v1", "graph", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := Reindex(context.Background(), Options{Src: t.TempDir(), Out: root}, "v1", GateOptions{}, buildRunner{t: t, badChunk: true}, nil); err == nil {
		t.Fatal("existing rollback version overwritten")
	}
	after, err := os.ReadFile(filepath.Join(root, "v1", "graph", "manifest.json"))
	if err != nil || string(after) != string(old) {
		t.Fatal("existing manifest changed")
	}
	if target, err := os.Readlink(filepath.Join(root, "current")); err != nil || target != "v1" {
		t.Fatal("existing current changed")
	}
}
