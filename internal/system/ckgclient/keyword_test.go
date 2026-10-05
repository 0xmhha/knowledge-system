package ckgclient

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestKeywordFTSQueryWithRealSQLite(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE VIRTUAL TABLE terms USING fts5(text)`); err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"SOURCE SNAPSHOT ADR", "Alpha", "Beta", "OR", "api Server Handle", "quoted word"} {
		if _, err := db.Exec(`INSERT INTO terms(text) VALUES (?)`, text); err != nil {
			t.Fatal(err)
		}
	}
	// Confirm the original failure on the same SQLite engine before testing
	// literals. Boolean-looking input must not broaden to two separate rows.
	if _, err := db.Query(`SELECT rowid FROM terms WHERE terms MATCH ?`, "SOURCE-SNAPSHOT-ADR"); err == nil {
		t.Fatal("bare hyphen candidate did not reproduce the FTS error")
	}
	for _, tc := range []struct {
		keyword string
		want    int
	}{
		{"SOURCE-SNAPSHOT-ADR", 1}, {"Alpha", 1}, {"api.(*Server).Handle", 1},
		{"OR", 1}, {"Alpha OR Beta", 0}, {`quoted" word`, 1},
		{`Alpha" OR "Beta`, 0}, {"不存在", 0},
	} {
		t.Run(tc.keyword, func(t *testing.T) {
			var got int
			if err := db.QueryRow(`SELECT count(*) FROM terms WHERE terms MATCH ?`, KeywordFTSQuery(tc.keyword)).Scan(&got); err != nil {
				t.Fatalf("literal keyword query failed: %v", err)
			}
			if got != tc.want {
				t.Fatalf("literal query broadened or lost its match: got %d want %d", got, tc.want)
			}
		})
	}
}
