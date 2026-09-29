package semantic

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

func TestStoreImmutableVersionsActivationAndRollback(t *testing.T) {
	p, root := fixture(t)
	store, err := OpenStore(filepath.Join(t.TempDir(), "semantic.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	if err := store.Put(ctx, p, root); err != nil {
		t.Fatal(err)
	}
	if err := store.Put(ctx, p, root); err != nil {
		t.Fatalf("identical retry: %v", err)
	}
	if err := store.Activate(ctx, p.Snapshot.ProjectID, "missing"); err == nil {
		t.Fatal("activated missing dataset")
	}
	if err := store.Activate(ctx, p.Snapshot.ProjectID, "v1"); err != nil {
		t.Fatal(err)
	}
	current, err := store.Current(ctx, p.Snapshot.ProjectID)
	if err != nil || current.Snapshot.DatasetID != "v1" {
		t.Fatalf("current v1: projection=%+v err=%v", current.Snapshot, err)
	}
	changed := p
	changed.Claims = append([]Claim(nil), p.Claims...)
	changed.Claims[0].Statement = "changed claim"
	if err := store.Put(ctx, changed, root); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("mutable dataset ID accepted: %v", err)
	}
	changed.Snapshot.DatasetID = "v2"
	changed.Evidence = append([]EvidenceSpan(nil), p.Evidence...)
	changed.Evidence[0].Snapshot = changed.Snapshot
	if err := store.Put(ctx, changed, root); err != nil {
		t.Fatal(err)
	}
	if err := store.Activate(ctx, "other-project", "v2"); err == nil {
		t.Fatal("cross-project activation accepted")
	}
	if err := store.Activate(ctx, p.Snapshot.ProjectID, "v2"); err != nil {
		t.Fatal(err)
	}
	current, err = store.Current(ctx, p.Snapshot.ProjectID)
	if err != nil || current.Snapshot.DatasetID != "v2" {
		t.Fatalf("current v2: projection=%+v err=%v", current.Snapshot, err)
	}
	if err := store.Activate(ctx, p.Snapshot.ProjectID, "v1"); err != nil {
		t.Fatal(err)
	}
	current, err = store.Current(ctx, p.Snapshot.ProjectID)
	if err != nil || current.Snapshot.DatasetID != "v1" {
		t.Fatalf("rollback: projection=%+v err=%v", current.Snapshot, err)
	}
}

func TestStoreDetectsDocumentCorruptionAndFutureSchema(t *testing.T) {
	p, root := fixture(t)
	path := filepath.Join(t.TempDir(), "semantic.db")
	store, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(context.Background(), p, root); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`UPDATE semantic_projections SET document='{}'`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(context.Background(), "sample", "v1"); err == nil || !strings.Contains(err.Error(), "digest mismatch") {
		t.Fatalf("corrupt projection accepted: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("PRAGMA user_version=3"); err != nil {
		t.Fatal(err)
	}
	db.Close()
	if _, err := OpenStore(path); err == nil || !strings.Contains(err.Error(), "newer") {
		t.Fatalf("future semantic schema accepted: %v", err)
	}
}

func TestStoreMigratesV1WithoutRewritingHistoricalProjection(t *testing.T) {
	p, root := fixture(t)
	p.SchemaVersion = 1
	path := filepath.Join(t.TempDir(), "semantic.db")
	store, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(context.Background(), p, root); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec("PRAGMA user_version=1"); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	var version int
	if err := store.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil || version != SchemaVersion {
		t.Fatalf("migrated version %d: %v", version, err)
	}
	loaded, err := store.Load(context.Background(), p.Snapshot.ProjectID, p.Snapshot.DatasetID)
	if err != nil || loaded.SchemaVersion != 1 || len(loaded.Requirements) != 0 {
		t.Fatalf("historical projection changed: %+v, %v", loaded, err)
	}
	p.SchemaVersion = SchemaVersion
	p.Snapshot.DatasetID = "v2"
	p.Evidence[0].Snapshot = p.Snapshot
	if err := store.Put(context.Background(), p, root); err != nil {
		t.Fatal(err)
	}
}
