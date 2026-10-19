package persist

import (
	"context"
	"database/sql"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// VisitGraph reads a finished graph in one read transaction. Callbacks are
// synchronous, ordered by node ID and edge ID, and must not retain mutable row
// state. Only the current row is buffered; callback/cancellation failures stop
// before commit. This does not support rewriting immutable graph files in place.
func (s *sqliteStore) VisitGraph(ctx context.Context, header func(Manifest) error, node func(types.Node) error, edge func(types.Edge) error) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	m, err := readManifest(ctx, tx)
	if err != nil {
		return err
	}
	if err := header(m); err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, `SELECT `+nodeColumns+` FROM nodes ORDER BY id`)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, err := scanNode(rows)
		if err != nil {
			return err
		}
		if err := node(n); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	rows, err = tx.QueryContext(ctx, `SELECT id, src, dst, type, COALESCE(file_path,''), COALESCE(line,0), count, confidence, COALESCE(dispatch_kind,'') FROM edges ORDER BY id`)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return err
		}
		var e types.Edge
		if err := rows.Scan(&e.ID, &e.Src, &e.Dst, &e.Type, &e.FilePath, &e.Line, &e.Count, &e.Confidence, &e.DispatchKind); err != nil {
			return err
		}
		if err := edge(e); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	return tx.Commit()
}
