package setup

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func stagingFixture(t *testing.T) (string, SourceIdentity, string) {
	t.Helper()
	candidate := filepath.Join(t.TempDir(), "candidate")
	if err := os.Mkdir(candidate, 0o700); err != nil {
		t.Fatal(err)
	}
	return candidate, SourceIdentity{ProjectID: "project-a", SnapshotID: strings.Repeat("a", 64)}, strings.Repeat("b", 64)
}
func stagingCleanup(ctx context.Context, candidate string, source SourceIdentity, dataset string) error {
	return CleanupAbandonedBuildStaging(ctx, filepath.Dir(candidate), filepath.Base(candidate), source.ProjectID, source.SnapshotID, dataset, "job-a")
}
func TestOwnedStagingRemovesOnlyExactCandidateScratch(t *testing.T) {
	candidate, source, dataset := stagingFixture(t)
	parent, cleanup, err := newBuildStaging(context.Background(), candidate, source, dataset, "job-a")
	if err != nil {
		t.Fatal(err)
	}
	retained := filepath.Join(candidate, "retained")
	if os.WriteFile(retained, []byte("keep"), 0o600) != nil || os.WriteFile(filepath.Join(parent, "partial"), []byte("scratch"), 0o600) != nil {
		t.Fatal("write")
	}
	if err := cleanup(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(parent); !os.IsNotExist(err) {
		t.Fatal("scratch survived", err)
	}
	if data, err := os.ReadFile(retained); err != nil || string(data) != "keep" {
		t.Fatal("retained changed")
	}
	if err := cleanup(); err != nil {
		t.Fatal("cleanup not idempotent", err)
	}
}
func TestOwnedStagingRefusesForeignOrLinkedPaths(t *testing.T) {
	for _, kind := range []string{"project", "snapshot", "dataset", "operation", "marker", "missing-marker", "directory-link", "candidate-link", "record-link", "traversal"} {
		t.Run(kind, func(t *testing.T) {
			candidate, source, dataset := stagingFixture(t)
			parent, _, err := newBuildStaging(context.Background(), candidate, source, dataset, "job-a")
			if err != nil {
				t.Fatal(err)
			}
			witness := filepath.Join(parent, "witness")
			if os.WriteFile(witness, []byte("keep"), 0o600) != nil {
				t.Fatal("write")
			}
			operation := "job-a"
			switch kind {
			case "project":
				source.ProjectID = "other"
			case "snapshot":
				source.SnapshotID = strings.Repeat("c", 64)
			case "dataset":
				dataset = strings.Repeat("c", 64)
			case "operation":
				operation = "other"
			case "marker":
				if os.WriteFile(filepath.Join(parent, ".owner.json"), []byte(`{}`), 0o600) != nil {
					t.Fatal("marker")
				}
			case "missing-marker":
				if os.Remove(filepath.Join(parent, ".owner.json")) != nil {
					t.Fatal("marker")
				}
			case "directory-link":
				moved := parent + "-real"
				if os.Rename(parent, moved) != nil || os.Symlink(moved, parent) != nil {
					t.Fatal("link")
				}
				witness = filepath.Join(moved, "witness")
			case "candidate-link":
				moved := candidate + "-real"
				if os.Rename(candidate, moved) != nil || os.Symlink(moved, candidate) != nil {
					t.Fatal("link")
				}
				witness = filepath.Join(moved, filepath.Base(parent), "witness")
			case "record-link":
				record := filepath.Join(candidate, buildStagingFile)
				moved := record + "-real"
				if os.Rename(record, moved) != nil || os.Symlink(moved, record) != nil {
					t.Fatal("link")
				}
			case "traversal":
				var record buildStagingRecord
				if readManagedMetadata(filepath.Join(candidate, buildStagingFile), &record) != nil {
					t.Fatal("record")
				}
				record.Directory = "../outside"
				if writeJSONAtomic(filepath.Join(candidate, buildStagingFile), record) != nil {
					t.Fatal("record")
				}
			}
			if err := CleanupAbandonedBuildStaging(context.Background(), filepath.Dir(candidate), filepath.Base(candidate), source.ProjectID, source.SnapshotID, dataset, operation); err == nil {
				t.Fatal("unsafe cleanup accepted")
			}
			if raw, err := os.ReadFile(witness); err != nil || string(raw) != "keep" {
				t.Fatal("foreign path changed", err)
			}
		})
	}
}
func TestBuildStagingCrashHelper(t *testing.T) {
	stage := os.Getenv("KS_STAGING_KILL_STAGE")
	if stage == "" {
		return
	}
	root := os.Getenv("KS_STAGING_KILL_ROOT")
	ready := os.Getenv("KS_STAGING_KILL_READY")
	source := SourceIdentity{ProjectID: "project-a", SnapshotID: strings.Repeat("a", 64)}
	stop := func() {
		if os.WriteFile(ready, []byte("ready"), 0o600) != nil {
			t.Fatal("ready")
		}
		for {
			time.Sleep(time.Hour)
		}
	}
	durabilityFault = func(at, _ string) error {
		if at == stage {
			stop()
		}
		return nil
	}
	parent, _, err := newBuildStaging(context.Background(), root, source, strings.Repeat("b", 64), "job-a")
	if err != nil {
		t.Fatal(err)
	}
	if stage == "populated" {
		if os.WriteFile(filepath.Join(parent, "partial"), []byte("partial"), 0o600) != nil {
			t.Fatal("partial")
		}
		stop()
	}
	t.Fatal("boundary not reached")
}
func TestOwnedStagingRecoversRealProcessKillBoundaries(t *testing.T) {
	for _, stage := range []string{"staging-intent", "staging-directory", "staging-owner", "populated"} {
		t.Run(stage, func(t *testing.T) {
			candidate, source, dataset := stagingFixture(t)
			ready := filepath.Join(t.TempDir(), "ready")
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestBuildStagingCrashHelper$")
			cmd.Env = append(os.Environ(), "KS_STAGING_KILL_STAGE="+stage, "KS_STAGING_KILL_ROOT="+candidate, "KS_STAGING_KILL_READY="+ready)
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if cmd.ProcessState == nil {
					_ = cmd.Process.Kill()
					_ = cmd.Wait()
				}
			})
			waitOwnedMarker(t, ready)
			if err := cmd.Process.Kill(); err != nil {
				t.Fatal(err)
			}
			if cmd.Wait() == nil {
				t.Fatal("helper survived")
			}
			if err := stagingCleanup(ctx, candidate, source, dataset); err != nil {
				t.Fatal(err)
			}
			entries, err := os.ReadDir(candidate)
			if err != nil || len(entries) != 0 {
				t.Fatal("staging remains", err)
			}
		})
	}
}
func TestOwnedStagingCanceledCleanupRetainsRecoveryRecord(t *testing.T) {
	candidate, source, dataset := stagingFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	parent, cleanup, err := newBuildStaging(ctx, candidate, source, dataset, "job-a")
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	if cleanup() == nil {
		t.Fatal("canceled cleanup succeeded")
	}
	if _, err := os.Stat(parent); err != nil {
		t.Fatal("lost staging")
	}
	if err := stagingCleanup(context.Background(), candidate, source, dataset); err != nil {
		t.Fatal(err)
	}
}
