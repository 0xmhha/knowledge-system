package persist

import "fmt"

// reviewCodeEdgeTypes are the code relations kept in code_edges. Structural
// (contains, defines from files), temporal (changed_in, blame, has_hunk,
// modifies) and cross-file plumbing edges are left out: a review walk over
// them reaches commits and, through them, most of the graph.
var reviewCodeEdgeTypes = []string{
	"calls", "invokes", "implements", "extends", "references",
	"uses_type", "instantiates", "writes_field",
	"spawns", "sends_to", "recvs_from", "acquires_lock", "releases_lock",
	"accessed_under_lock", "timeout_path", "cancellation_path",
}

// RebuildReviewIndex implements StoreWriter. It runs in one transaction so a
// reader never sees a partial index.
func (s *sqliteStore) RebuildReviewIndex(testFiles []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("review index: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`UPDATE nodes SET is_test = 0 WHERE is_test <> 0`); err != nil {
		return fmt.Errorf("review index: reset is_test: %w", err)
	}
	for start := 0; start < len(testFiles); start += queryEdgesChunk {
		chunk := testFiles[start:min(start+queryEdgesChunk, len(testFiles))]
		if _, err := tx.Exec(`UPDATE nodes SET is_test = 1 WHERE file_path IN (`+placeholders(len(chunk))+`)`, anys(chunk)...); err != nil {
			return fmt.Errorf("review index: mark tests: %w", err)
		}
	}
	if _, err := tx.Exec(`DELETE FROM code_edges`); err != nil {
		return fmt.Errorf("review index: clear code_edges: %w", err)
	}
	// defines is kept only for type membership (Struct/Interface -> Method,
	// Struct -> Field); File -> symbol defines are file plumbing.
	if _, err := tx.Exec(`INSERT INTO code_edges (src, dst, type, line)
		SELECT e.src, e.dst, e.type, COALESCE(e.line, 0)
		FROM edges e JOIN nodes s ON s.id = e.src JOIN nodes d ON d.id = e.dst
		WHERE s.is_test = 0 AND d.is_test = 0
		  AND (e.type IN (`+placeholders(len(reviewCodeEdgeTypes))+`)
		       OR (e.type = 'defines' AND s.type IN ('Struct', 'Interface')))`, anys(reviewCodeEdgeTypes)...); err != nil {
		return fmt.Errorf("review index: fill code_edges: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM node_commits`); err != nil {
		return fmt.Errorf("review index: clear node_commits: %w", err)
	}
	if _, err := tx.Exec(`INSERT OR IGNORE INTO node_commits (node_id, commit_id)
		SELECT m.dst, h.src FROM edges m
		JOIN edges h ON h.dst = m.src AND h.type = 'has_hunk'
		WHERE m.type = 'modifies'`); err != nil {
		return fmt.Errorf("review index: fill node_commits: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("review index: commit: %w", err)
	}
	return nil
}
