package persist

import "fmt"

// RowCounts returns how many rows the nodes and edges tables hold: the
// graph's counts as written, whatever the manifest recorded during the
// build.
func (s *sqliteStore) RowCounts() (nodes, edges int, err error) {
	if err := s.db.QueryRow(`SELECT (SELECT count(*) FROM nodes), (SELECT count(*) FROM edges)`).Scan(&nodes, &edges); err != nil {
		return 0, 0, fmt.Errorf("count rows: %w", err)
	}
	return nodes, edges, nil
}
