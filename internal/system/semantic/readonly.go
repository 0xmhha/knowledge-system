package semantic

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"path/filepath"
)

// OpenStoreReadOnly never creates or migrates an evaluation/query input.
// Operators migrate old stores separately through OpenStore.
func OpenStoreReadOnly(ctx context.Context, path string) (*Store, error) {
	if path == "" {
		return nil, fmt.Errorf("semantic: empty store path")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	uri := url.URL{Scheme: "file", Path: abs, RawQuery: "mode=ro"}
	db, err := sql.Open("sqlite3", uri.String())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	var version int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		db.Close()
		return nil, err
	}
	if version != StoreSchemaVersion {
		db.Close()
		return nil, fmt.Errorf("semantic: read-only store schema %d requires supported schema %d", version, StoreSchemaVersion)
	}
	return &Store{db: db}, nil
}
