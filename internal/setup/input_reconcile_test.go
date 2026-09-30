package setup

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEngineInputReconciliationRejectsUnknownAndAlteredSources(t *testing.T) {
	version := t.TempDir()
	graph := filepath.Join(version, "graph", "manifest.json")
	if err := os.MkdirAll(filepath.Dir(graph), 0o700); err != nil {
		t.Fatal(err)
	}
	vectorDir := filepath.Join(version, "vector")
	if err := os.MkdirAll(vectorDir, 0o700); err != nil {
		t.Fatal(err)
	}
	commit := strings.Repeat("a", 40)
	db, err := sql.Open("sqlite3", filepath.Join(vectorDir, "vector.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE chunks(file TEXT, commit_hash TEXT, chunk_kind TEXT);
		INSERT INTO chunks VALUES ('main.go', ?, 'symbol')`, commit); err != nil {
		t.Fatal(err)
	}
	captured := CapturedSource{Identity: SourceIdentity{SourceMode: "committed", SourceCommit: commit},
		Files: []CapturedFile{{Path: "main.go", SHA256: "correct"}}}
	write := func(path, sha string) {
		t.Helper()
		if err := writeJSONAtomic(graph, map[string]any{"files": []map[string]string{{"path": path, "sha256": sha}}}); err != nil {
			t.Fatal(err)
		}
	}
	write("main.go", "correct")
	vectorManifest := filepath.Join(vectorDir, "manifest.json")
	writeVector := func(sha string) {
		t.Helper()
		if err := writeJSONAtomic(vectorManifest, map[string]any{"input_files": []map[string]string{
			{"origin_id": "repo", "path": "main.go", "sha256": sha},
		}}); err != nil {
			t.Fatal(err)
		}
	}
	writeVector("correct")
	if err := VerifyEngineInputs(version, captured); err != nil {
		t.Fatal(err)
	}
	writeVector("different")
	if err := VerifyEngineInputs(version, captured); err == nil {
		t.Fatal("changed CKV input hash accepted")
	}
	writeVector("correct")
	write("main.go", "different")
	if err := VerifyEngineInputs(version, captured); err == nil {
		t.Fatal("changed CKG input hash accepted")
	}
	write("main.go", "correct")
	if _, err := db.Exec(`INSERT INTO chunks VALUES ('outside.go', ?, 'symbol')`, commit); err != nil {
		t.Fatal(err)
	}
	if err := VerifyEngineInputs(version, captured); err == nil {
		t.Fatal("uncaptured CKV input accepted")
	}
}
