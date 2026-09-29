package semantic

import (
	"context"
	"database/sql"
	"os"
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

func TestStoreIndexesTermsAndMigratesV2WithoutRewritingProjection(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "spec-driven", "ontology-pilot.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	// Two concepts intentionally share one English term. Lookup must keep
	// both meanings for review instead of asserting a single interpretation.
	source = []byte(strings.Replace(string(source), "value: dataset, preferred: true", "value: project, preferred: true", 1))
	repo := t.TempDir()
	gitOutput(t, repo, "init", "-q")
	if err := os.WriteFile(filepath.Join(repo, "ontology.yaml"), source, 0600); err != nil {
		t.Fatal(err)
	}
	gitOutput(t, repo, "add", "ontology.yaml")
	gitOutput(t, repo, "-c", "user.name=Test", "-c", "user.email=test@example.com", "-c", "commit.gpgsign=false", "commit", "-qm", "ontology")
	p, _, err := ExtractOntology(Snapshot{ProjectID: "knowledge-system", DatasetID: "v1",
		Commit: gitOutput(t, repo, "rev-parse", "HEAD")}, "ontology.yaml", source)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "semantic.db")
	store, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := store.Put(ctx, p, repo); err != nil {
		t.Fatal(err)
	}
	if err := store.Put(ctx, p, repo); err != nil {
		t.Fatalf("identical indexed retry: %v", err)
	}
	assertTerms := func() {
		t.Helper()
		hits, err := store.LookupTerm(ctx, "knowledge-system", "v1", "EN", " PROJECT ")
		if err != nil || len(hits) != 2 || hits[0].ConceptID != "dataset" || hits[1].ConceptID != "project" || hits[0].Score != 0.5 {
			t.Fatalf("ambiguous indexed terms: %+v, %v", hits, err)
		}
		if hits, err := store.LookupTerm(ctx, "knowledge-system", "v1", "en", "unknown"); err != nil || len(hits) != 0 {
			t.Fatalf("unknown term: %+v, %v", hits, err)
		}
	}
	assertTerms()
	p2, _, err := ExtractOntology(Snapshot{ProjectID: "knowledge-system", DatasetID: "v2", Commit: p.Snapshot.Commit},
		"ontology.yaml", source)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(ctx, p2, repo); err != nil {
		t.Fatal(err)
	}
	if hits, err := store.LookupTerm(ctx, "knowledge-system", "v2", "en", "project"); err != nil || len(hits) != 2 {
		t.Fatalf("second dataset term isolation: %+v, %v", hits, err)
	}
	var before []byte
	if err := store.db.QueryRow(`SELECT document FROM semantic_projections WHERE project_id=? AND dataset_id=?`,
		"knowledge-system", "v1").Scan(&before); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`DROP TABLE semantic_concept_terms; PRAGMA user_version=2;`); err != nil {
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
	assertTerms()
	var after []byte
	if err := store.db.QueryRow(`SELECT document FROM semantic_projections WHERE project_id=? AND dataset_id=?`,
		"knowledge-system", "v1").Scan(&after); err != nil || string(after) != string(before) {
		t.Fatalf("migration rewrote historical projection: %v", err)
	}
	if _, err := store.db.Exec(`DELETE FROM semantic_concept_terms WHERE dataset_id='v1' AND concept_id='dataset'`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.LookupTerm(ctx, "knowledge-system", "v1", "en", "project"); err == nil || !strings.Contains(err.Error(), "index count mismatch") {
		t.Fatalf("missing normalized term silently accepted: %v", err)
	}
	if hits, err := store.LookupTerm(ctx, "knowledge-system", "v2", "en", "project"); err != nil || len(hits) != 2 {
		t.Fatalf("v1 index damage crossed into v2: %+v, %v", hits, err)
	}
}

func TestTermMigrationRejectsCorruptLegacyDocumentAtomically(t *testing.T) {
	p, repo := fixture(t)
	path := filepath.Join(t.TempDir(), "semantic.db")
	store, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(context.Background(), p, repo); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`UPDATE semantic_projections SET document='{}'; PRAGMA user_version=2;`); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenStore(path); err == nil || !strings.Contains(err.Error(), "digest mismatch") {
		t.Fatalf("corrupt legacy document migrated: %v", err)
	}
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil || version != 2 {
		t.Fatalf("failed migration changed schema version to %d: %v", version, err)
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
	if _, err := db.Exec("PRAGMA user_version=4"); err != nil {
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
	if err := store.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil || version != StoreSchemaVersion {
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
