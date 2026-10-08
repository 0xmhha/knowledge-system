package setup

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func durableTestVersion(t *testing.T, dataset, version string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("retained evidence\n"), 0600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dataset, version)
	captured, err := CaptureSource(CaptureOptions{Root: root, Out: out, ProjectID: "durability-fixture", SourceMode: "snapshot-only"})
	if err != nil {
		t.Fatal(err)
	}
	writePinnedMockManifests(t, out, root, "", captured.Identity)
	if _, err := PublishCandidateIdentity(out, captured.Identity, "inputs"); err != nil {
		t.Fatal(err)
	}
	return out
}

func durabilityTree(t *testing.T, root string) map[string][32]byte {
	t.Helper()
	result := map[string][32]byte{}
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		result[strings.TrimPrefix(path, root)] = sha256.Sum256(data)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return result
}

func assertDurabilityCurrent(t *testing.T, dataset, want string) {
	t.Helper()
	current, err := os.Readlink(filepath.Join(dataset, "current"))
	if err != nil || current != want {
		t.Fatalf("current=%s want=%s: %v", current, want, err)
	}
	if id, err := InspectVersionIdentity(filepath.Join(dataset, current)); err != nil || id == nil {
		t.Fatalf("current is incomplete: %v", err)
	}
}

func addDurabilityDB(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA journal_mode=WAL; PRAGMA wal_autocheckpoint=0; CREATE TABLE payload(value TEXT); INSERT INTO payload VALUES ('durable evidence')`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestPromotionDurabilityFailureBoundaries(t *testing.T) {
	for _, stage := range []string{"checkpoint", "checkpoint-close", "candidate-file", "candidate-dir", "candidate-parent", "before-rename", "after-rename", "current-parent"} {
		t.Run(stage, func(t *testing.T) {
			dataset := t.TempDir()
			old := durableTestVersion(t, dataset, "old")
			candidate := durableTestVersion(t, dataset, "candidate")
			addDurabilityDB(t, filepath.Join(candidate, "graph", "graph.db"))
			if _, err := Promote(dataset, "old"); err != nil {
				t.Fatal(err)
			}
			before := durabilityTree(t, old)
			hit := false
			durabilityFault = func(got, _ string) error {
				if got == stage {
					hit = true
					return fmt.Errorf("injected %s", stage)
				}
				return nil
			}
			t.Cleanup(func() { durabilityFault = nil })
			prev, err := Promote(dataset, "candidate")
			durabilityFault = nil
			if !hit || err == nil || prev != "old" {
				t.Fatalf("failure not reported: prev=%s hit=%v err=%v", prev, hit, err)
			}
			post := stage == "after-rename" || stage == "current-parent"
			if errors.Is(err, ErrDurabilityUncertain) != post {
				t.Fatalf("incorrect uncertainty: %v", err)
			}
			want := "old"
			if post {
				want = "candidate"
			}
			assertDurabilityCurrent(t, dataset, want)
			after := durabilityTree(t, old)
			if fmt.Sprint(before) != fmt.Sprint(after) {
				t.Fatal("previous version bytes changed")
			}
			if err := Rollback(dataset, "old"); err != nil {
				t.Fatal(err)
			}
			assertDurabilityCurrent(t, dataset, "old")
			if _, err := Promote(dataset, "candidate"); err != nil {
				t.Fatal(err)
			}
			assertDurabilityCurrent(t, dataset, "candidate")
		})
	}
}

func holdDurabilityVersion(t *testing.T, dataset, version string) {
	t.Helper()
	id, err := InspectVersionIdentity(filepath.Join(dataset, version))
	if err != nil {
		t.Fatal(err)
	}
	if err := writeJSONAtomic(filepath.Join(dataset, version, "review-hold.json"), map[string]string{
		"project_id": id.Source.ProjectID, "dataset_id": id.DatasetID, "snapshot_id": id.Source.SnapshotID, "base_version": "old",
	}); err != nil {
		t.Fatal(err)
	}
}

func TestReviewedPromotionDurableIntentRecovery(t *testing.T) {
	for _, stage := range []string{"intent", "before-rename", "after-rename", "current-parent", "release"} {
		t.Run(stage, func(t *testing.T) {
			dataset := t.TempDir()
			durableTestVersion(t, dataset, "old")
			durableTestVersion(t, dataset, "candidate")
			if _, err := Promote(dataset, "old"); err != nil {
				t.Fatal(err)
			}
			holdDurabilityVersion(t, dataset, "candidate")
			hit := false
			durabilityFault = func(got, path string) error {
				match := got == stage || (stage == "intent" && got == "review-marker" && strings.HasSuffix(path, "review-intent.json")) ||
					(stage == "release" && got == "review-marker" && strings.HasSuffix(path, "review-release.json"))
				if match {
					hit = true
					return fmt.Errorf("injected %s", stage)
				}
				return nil
			}
			t.Cleanup(func() { durabilityFault = nil })
			_, err := PromoteReviewedCandidateIfBase(dataset, "candidate", "old", "patch-1", strings.Repeat("a", 64))
			durabilityFault = nil
			if !hit || err == nil {
				t.Fatalf("not interrupted: %v", err)
			}
			post := stage == "after-rename" || stage == "current-parent" || stage == "release"
			if errors.Is(err, ErrDurabilityUncertain) != post {
				t.Fatalf("incorrect uncertainty: %v", err)
			}
			want := "old"
			if post {
				want = "candidate"
			}
			assertDurabilityCurrent(t, dataset, want)
			if _, err := os.Stat(filepath.Join(dataset, "candidate", "review-release.json")); !os.IsNotExist(err) {
				t.Fatal("interrupted approval left release marker")
			}
			if err := Rollback(dataset, "candidate"); err == nil {
				t.Fatal("held candidate bypassed approval via rollback")
			}
			if stage != "intent" {
				if _, err := PromoteReviewedCandidateIfBase(dataset, "candidate", "old", "foreign", strings.Repeat("b", 64)); err == nil {
					t.Fatal("foreign approval recovered")
				}
			}
			prev, err := PromoteReviewedCandidateIfBase(dataset, "candidate", "old", "patch-1", strings.Repeat("a", 64))
			if err != nil || prev != "old" {
				t.Fatalf("same approval did not recover: prev=%s err=%v", prev, err)
			}
			assertDurabilityCurrent(t, dataset, "candidate")
			if err := Rollback(dataset, "old"); err != nil {
				t.Fatal(err)
			}
			if err := Rollback(dataset, "candidate"); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPromotionRejectsVectorDBChangedByCheckpoint(t *testing.T) {
	dataset := t.TempDir()
	version := durableTestVersion(t, dataset, "candidate")
	dbPath := filepath.Join(version, "vector", "vector.db")
	addDurabilityDB(t, dbPath)
	data, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(data)
	path := filepath.Join(version, "vector", "manifest.json")
	var manifest map[string]any
	if err := readJSON(path, &manifest); err != nil {
		t.Fatal(err)
	}
	manifest["db_sha256"] = fmt.Sprintf("%x", hash)
	if err := writeJSONAtomic(path, manifest); err != nil {
		t.Fatal(err)
	}
	if _, err := Promote(dataset, "candidate"); err == nil {
		t.Fatal("physical vector pin changed during checkpoint but candidate published")
	}
	if _, err := os.Lstat(filepath.Join(dataset, "current")); !os.IsNotExist(err) {
		t.Fatal("invalid DB changed current")
	}
	var saved map[string]any
	if err := readJSON(path, &saved); err != nil || saved["db_sha256"] != manifest["db_sha256"] {
		t.Fatal("promotion rewrote engine pin")
	}
}

func TestDurabilityProcessHelper(t *testing.T) {
	if os.Getenv("CKS_N04_DURABILITY_HELPER") != "1" {
		return
	}
	args := os.Args
	for len(args) > 0 && args[0] != "--" {
		args = args[1:]
	}
	if len(args) != 5 {
		os.Exit(25)
	}
	dataset, stage, ready, operation := args[1], args[2], args[3], args[4]
	durabilityFault = func(got, path string) error {
		if got == stage && (stage != "review-marker" || strings.HasSuffix(path, "review-release.json")) {
			if err := os.WriteFile(ready, []byte("ready"), 0600); err != nil {
				os.Exit(26)
			}
			for {
				time.Sleep(time.Second)
			}
		}
		return nil
	}
	var err error
	if operation == "reviewed" {
		_, err = PromoteReviewedCandidateIfBase(dataset, "candidate", "old", "patch-1", strings.Repeat("a", 64))
	} else {
		_, err = Promote(dataset, "candidate")
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(27)
	}
	os.Exit(28) // Parent must kill at the requested boundary, not observe success.
}

func TestPromotionProcessKillRecovery(t *testing.T) {
	for _, operation := range []string{"ordinary", "reviewed"} {
		stages := []string{"before-rename", "after-rename", "current-parent"}
		if operation == "reviewed" {
			stages = append(stages, "review-marker")
		}
		for _, stage := range stages {
			t.Run(operation+"/"+stage, func(t *testing.T) {
				dataset := t.TempDir()
				old := durableTestVersion(t, dataset, "old")
				durableTestVersion(t, dataset, "candidate")
				if _, err := Promote(dataset, "old"); err != nil {
					t.Fatal(err)
				}
				if operation == "reviewed" {
					holdDurabilityVersion(t, dataset, "candidate")
				}
				before := durabilityTree(t, old)
				ready := filepath.Join(t.TempDir(), "ready")
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestDurabilityProcessHelper$", "--", dataset, stage, ready, operation)
				cmd.Env = append(os.Environ(), "CKS_N04_DURABILITY_HELPER=1")
				if err := cmd.Start(); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if cmd.ProcessState == nil {
						cmd.Process.Kill()
						cmd.Wait()
					}
				})
				deadline := time.Now().Add(10 * time.Second)
				for {
					if _, err := os.Stat(ready); err == nil {
						break
					}
					if time.Now().After(deadline) {
						t.Fatal("helper did not reach crash boundary")
					}
					time.Sleep(10 * time.Millisecond)
				}
				if err := cmd.Process.Kill(); err != nil {
					t.Fatal(err)
				}
				if err := cmd.Wait(); err == nil {
					t.Fatal("helper was not killed")
				}
				want := "candidate"
				if stage == "before-rename" {
					want = "old"
				}
				assertDurabilityCurrent(t, dataset, want)
				if fmt.Sprint(before) != fmt.Sprint(durabilityTree(t, old)) {
					t.Fatal("crash changed previous immutable version")
				}
				if operation == "reviewed" {
					if _, err := os.Lstat(filepath.Join(dataset, "candidate", "review-release.json")); !os.IsNotExist(err) {
						t.Fatal("crash fabricated release")
					}
					if _, err := PromoteReviewedCandidateIfBase(dataset, "candidate", "old", "patch-1", strings.Repeat("a", 64)); err != nil {
						t.Fatal(err)
					}
				} else {
					if _, err := Promote(dataset, "candidate"); err != nil {
						t.Fatal(err)
					}
				}
				assertDurabilityCurrent(t, dataset, "candidate")
				// Explicitly verify retained source bytes after the writer is gone.
				id, _ := InspectVersionIdentity(filepath.Join(dataset, "candidate"))
				if err := VerifyRetainedSource(filepath.Join(dataset, "candidate"), id.Source); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestPromotionFlushesCandidateWALBeforePointer(t *testing.T) {
	dataset := t.TempDir()
	version := durableTestVersion(t, dataset, "v1")
	path := filepath.Join(version, "graph", "graph.db")
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA journal_mode=WAL; PRAGMA wal_autocheckpoint=0; CREATE TABLE payload(value TEXT); INSERT INTO payload VALUES ('durable evidence')`); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path + "-wal")
	if err != nil || info.Size() == 0 {
		t.Fatalf("fixture has no pending WAL: %v", err)
	}
	if _, err := Promote(dataset, "v1"); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(path + "-wal"); err == nil && info.Size() != 0 {
		t.Fatal("candidate was published with uncheckpointed WAL")
	} else if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

func TestPromotionRejectsLinkedCandidateArtifact(t *testing.T) {
	dataset := t.TempDir()
	version := durableTestVersion(t, dataset, "v1")
	if err := os.Symlink(t.TempDir(), filepath.Join(version, "external")); err != nil {
		t.Fatal(err)
	}
	if _, err := Promote(dataset, "v1"); err == nil {
		t.Fatal("candidate with symlink artifact published")
	}
	if _, err := os.Lstat(filepath.Join(dataset, "current")); !os.IsNotExist(err) {
		t.Fatal("rejected candidate changed current")
	}
}

func TestPromotionRejectsHardlinkedDatabaseBeforeCheckpoint(t *testing.T) {
	dataset := t.TempDir()
	version := durableTestVersion(t, dataset, "candidate")
	external := filepath.Join(t.TempDir(), "external.db")
	db := addDurabilityDB(t, external)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(external)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Link(external, filepath.Join(version, "graph", "graph.db")); err != nil {
		t.Fatal(err)
	}
	if _, err := Promote(dataset, "candidate"); err == nil {
		t.Fatal("hardlinked database published")
	}
	after, err := os.ReadFile(external)
	if err != nil || sha256.Sum256(before) != sha256.Sum256(after) {
		t.Fatal("external DB changed before hardlink rejection")
	}
}

func TestPromotionHandlesSidecarsRemovedByCheckpointClose(t *testing.T) {
	dataset := t.TempDir()
	version := durableTestVersion(t, dataset, "candidate")
	path := filepath.Join(version, "vector", "vector.db")
	db := addDurabilityDB(t, path)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	// Input reconciliation opens a read-only connection after the builder exits.
	// Its empty sidecars can disappear when promotion checkpoints/closes the DB.
	reader, err := sql.Open("sqlite3", "file:"+path+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	var count int
	if err := reader.QueryRow(`SELECT count(*) FROM payload`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path + "-wal"); err != nil {
		t.Fatalf("fixture lacks read-only sidecar: %v", err)
	}
	if _, err := Promote(dataset, "candidate"); err != nil {
		t.Fatal(err)
	}
	assertDurabilityCurrent(t, dataset, "candidate")
}

func TestPromotionRejectsBusyCheckpointAndFlushesSemanticStore(t *testing.T) {
	dataset := t.TempDir()
	durableTestVersion(t, dataset, "old")
	version := durableTestVersion(t, dataset, "candidate")
	// Optional semantic store names are configurable, so do not depend on .db.
	path := filepath.Join(version, "semantic-state")
	db := addDurabilityDB(t, path)
	if _, err := Promote(dataset, "old"); err != nil {
		t.Fatal(err)
	}
	reader, err := sql.Open("sqlite3", "file:"+path+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	tx, err := reader.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	var count int
	if err := tx.QueryRow(`SELECT count(*) FROM payload`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO payload VALUES ('after reader')`); err != nil {
		t.Fatal(err)
	}
	if _, err := Promote(dataset, "candidate"); err == nil {
		t.Fatal("busy semantic WAL published")
	}
	assertDurabilityCurrent(t, dataset, "old")
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if _, err := Promote(dataset, "candidate"); err != nil {
		t.Fatal(err)
	}
	assertDurabilityCurrent(t, dataset, "candidate")
	if info, err := os.Stat(path + "-wal"); err == nil && info.Size() != 0 {
		t.Fatal("semantic WAL not flushed")
	}
}

func TestReindexHoldDoesNotReportDoneAfterPersistenceFailure(t *testing.T) {
	dataset := t.TempDir()
	o := Options{Src: "/src", Out: dataset, GraphBin: "ckg", VectorBin: "ckv"}
	done := false
	durabilityFault = func(stage, _ string) error {
		if stage == "candidate-file" {
			return fmt.Errorf("injected hold flush")
		}
		return nil
	}
	t.Cleanup(func() { durabilityFault = nil })
	err := Reindex(context.Background(), o, "held", GateOptions{HoldForReview: true}, buildRunner{t: t}, func(event Event) {
		if event.Step == "reindex-review" && event.Type == "done" {
			done = true
		}
	})
	durabilityFault = nil
	if err == nil || done {
		t.Fatalf("unsynced held candidate reported complete: err=%v done=%v", err, done)
	}
	if _, err := os.Lstat(filepath.Join(dataset, "current")); !os.IsNotExist(err) {
		t.Fatal("hold changed current")
	}
	if _, err := os.Lstat(filepath.Join(dataset, "held", "review-release.json")); !os.IsNotExist(err) {
		t.Fatal("failed hold created release")
	}
}
