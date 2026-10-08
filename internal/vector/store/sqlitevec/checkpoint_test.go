package sqlitevec

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestCheckpointRejectsBusyWAL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vector.db")
	s, err := Open(path, testDim)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.db.Exec(`PRAGMA busy_timeout=100; CREATE TABLE checkpoint_payload(value TEXT); INSERT INTO checkpoint_payload VALUES ('before')`); err != nil {
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
	if err := tx.QueryRow(`SELECT count(*) FROM checkpoint_payload`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`INSERT INTO checkpoint_payload VALUES ('after')`); err != nil {
		t.Fatal(err)
	}
	if err := s.Checkpoint(); err == nil {
		t.Fatal("busy PRAGMA row was treated as a successful checkpoint")
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err := s.Checkpoint(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}
