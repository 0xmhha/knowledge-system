package setup

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A version is fully verified once per process; later inspections compare a
// fingerprint of every file and verify again only when it changed.
func TestVersionVerificationIsReusedUntilFilesChange(t *testing.T) {
	version, captured := retainedVersionForRequest(t)
	before := fullVerifications.Load()
	for i := 0; i < 3; i++ {
		if id, err := InspectVersionIdentity(version); err != nil || id == nil {
			t.Fatalf("inspect %d: %v %v", i, id, err)
		}
	}
	if got := fullVerifications.Load() - before; got != 1 {
		t.Fatalf("full verifications = %d, want 1", got)
	}
	// SQLite sidecars appear and vanish while a read-only store is open; they
	// are not verified content and must not force a full verification.
	side := filepath.Join(version, "vector", "vector.db-shm")
	if err := os.WriteFile(side, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := InspectVersionIdentity(version); err != nil {
		t.Fatal(err)
	}
	if got := fullVerifications.Load() - before; got != 1 {
		t.Fatalf("a sidecar forced a full verification (%d)", got)
	}

	// Same size, original modification time restored: the change time still
	// moves, so the damage is found.
	var blob string
	for _, f := range captured.Files {
		if f.Path == "unused.md" {
			blob = filepath.Join(captured.BlobDir, f.SHA256)
		}
	}
	info, err := os.Stat(blob)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	if err := os.Chmod(blob, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(blob, []byte("UNUSED!\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(blob, info.Mode().Perm()); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(blob, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	if _, err := InspectVersionIdentity(version); err == nil {
		t.Fatal("damage hidden behind a restored size and modification time was not found")
	}
}

// A failed verification is never cached.
func TestFailedVerificationIsNotCached(t *testing.T) {
	version, captured := retainedVersionForRequest(t)
	alterRetainedBlob(t, captured, "unused.md")
	for i := 0; i < 2; i++ {
		if _, err := InspectVersionIdentity(version); err == nil {
			t.Fatalf("inspect %d accepted a damaged archive", i)
		}
	}
}
