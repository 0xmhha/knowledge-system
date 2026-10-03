package sqlitevec

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"path/filepath"
)

// OpenReadOnly opens an existing index without schema changes or migrations.
// Evaluation probes use this to avoid changing their frozen input database.
func OpenReadOnly(path string, dim int) (*Store, error) {
	if dim <= 0 {
		return nil, fmt.Errorf("sqlitevec: invalid dim %d", dim)
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
	s := &Store{db: db, dim: dim}
	fail := func(err error) (*Store, error) { _ = db.Close(); return nil, err }
	stored, err := s.getStoredDim()
	if err != nil {
		return fail(err)
	}
	if stored != dim {
		return fail(fmt.Errorf("sqlitevec: embedding dim mismatch: db=%d, caller=%d", stored, dim))
	}
	status, err := NewMigrationRunner(db, path, WithBackup(false)).Status(context.Background())
	if err != nil {
		return fail(err)
	}
	if len(status.Tampered) > 0 {
		return fail(ErrMigrationTampered)
	}
	if len(status.Pending) > 0 {
		return fail(ErrMigrationRequired)
	}
	return s, nil
}
