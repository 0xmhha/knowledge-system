package persist

import (
	"context"
	"fmt"
	"strings"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// EdgePageInput bounds rows in SQL before materializing them. Direction is
// explicit and edge kinds are parameters, never interpolated SQL identifiers.
type EdgePageInput struct {
	NodeID    string
	Direction string
	Types     []string
	Limit     int
}
type BoundedEdgeReader interface {
	IncidentEdges(context.Context, EdgePageInput) ([]types.Edge, bool, error)
}

func edgePageSQL(q EdgePageInput, postgres bool) (string, []any, error) {
	if q.NodeID == "" || len(q.NodeID) > 128 || q.Limit < 1 || q.Limit > 256 || len(q.Types) > 8 {
		return "", nil, fmt.Errorf("invalid bounded edge input")
	}
	args := []any{}
	parameter := func(v any) string {
		args = append(args, v)
		if postgres {
			return fmt.Sprintf("$%d", len(args))
		}
		return "?"
	}
	where := ""
	switch q.Direction {
	case "outgoing":
		where = "src = " + parameter(q.NodeID)
	case "incoming":
		where = "dst = " + parameter(q.NodeID)
	case "both":
		where = "(src = " + parameter(q.NodeID) + " OR dst = " + parameter(q.NodeID) + ")"
	default:
		return "", nil, fmt.Errorf("explicit edge direction required")
	}
	if len(q.Types) > 0 {
		marks := []string{}
		for _, kind := range q.Types {
			if kind == "" || len(kind) > 64 {
				return "", nil, fmt.Errorf("invalid edge kind")
			}
			marks = append(marks, parameter(kind))
		}
		where += " AND type IN (" + strings.Join(marks, ",") + ")"
	}
	sql := `SELECT id, src, dst, type, COALESCE(file_path,''), COALESCE(line,0), count, confidence, COALESCE(dispatch_kind,'') FROM edges WHERE ` + where + ` ORDER BY type, src, dst, id LIMIT ` + parameter(q.Limit+1)
	return sql, args, nil
}
func (s *sqliteStore) IncidentEdges(ctx context.Context, q EdgePageInput) ([]types.Edge, bool, error) {
	sql, args, e := edgePageSQL(q, false)
	if e != nil {
		return nil, false, e
	}
	if ctx == nil {
		return nil, false, fmt.Errorf("context required")
	}
	rows, e := s.db.QueryContext(ctx, sql, args...)
	if e != nil {
		return nil, false, e
	}
	defer func() { _ = rows.Close() }()
	edges, e := scanEdges(rows)
	if e != nil {
		return nil, false, e
	}
	more := len(edges) > q.Limit
	if more {
		edges = edges[:q.Limit]
	}
	return edges, more, nil
}
func (s *pgStore) IncidentEdges(ctx context.Context, q EdgePageInput) ([]types.Edge, bool, error) {
	sql, args, e := edgePageSQL(q, true)
	if e != nil {
		return nil, false, e
	}
	if ctx == nil {
		return nil, false, fmt.Errorf("context required")
	}
	rows, e := s.pool.Query(ctx, sql, args...)
	if e != nil {
		return nil, false, e
	}
	defer rows.Close()
	edges, e := scanPGEdges(rows)
	if e != nil {
		return nil, false, e
	}
	more := len(edges) > q.Limit
	if more {
		edges = edges[:q.Limit]
	}
	return edges, more, nil
}
