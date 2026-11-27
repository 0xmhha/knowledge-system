package persist

import (
	"fmt"
	"time"

	"github.com/0xmhha/knowledge-system/internal/graph/cluster"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// ClusterEdge mirrors cluster.Edge to avoid making persist's exported surface
// reach across packages. cluster.PersistClusterEdge is a structurally identical
// type defined in the cluster package; InsertPkgTreeFromCluster bridges them.
type ClusterEdge struct {
	ParentID, ChildID string
	Level             int
}

// TopicTreeInput abstracts the per-resolution view of a topic tree so persist
// can consume it without importing cluster types directly. *cluster.TopicTree
// satisfies this interface (see internal/cluster/persist_adapter.go).
type TopicTreeInput interface {
	ResolutionsCount() int
	ResolutionGamma(i int) float64
	ResolutionMembers(i int) map[string][]string // label -> []nodeID
}

// InsertNodes bulk-inserts nodes (transactional). The trailing
// attrs column carries the marker JSON blob (SlotIndex,
// HasAssembly, IsFunctionTyped, HasFunctionPointerCall,
// HasExternalCall, YulBuiltins, HasInheritanceMROFallback, …).
func (s *sqliteStore) InsertNodes(nodes []types.Node) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO nodes
		(id, type, name, qualified_name, canonical_id, file_path, start_line, end_line,
		 start_byte, end_byte, language, visibility, signature, doc_comment,
		 complexity, in_degree, out_degree, pagerank, usage_score, confidence, sub_kind, attrs, search_tokens, simple_name)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer func() { _ = stmt.Close() }()
	for _, n := range nodes {
		attrs := marshalNodeAttrs(&n)
		tokens := buildSearchTokens(n.Name, n.QualifiedName)
		if _, err := stmt.Exec(n.ID, n.Type, n.Name, n.QualifiedName, n.CanonicalID, n.FilePath,
			n.StartLine, n.EndLine, n.StartByte, n.EndByte, n.Language,
			n.Visibility, n.Signature, n.DocComment, n.Complexity,
			n.InDegree, n.OutDegree, n.PageRank, n.UsageScore,
			string(n.Confidence), n.SubKind, attrs, tokens, simpleName(n.QualifiedName)); err != nil {
			return fmt.Errorf("insert node %s: %w", n.ID, err)
		}
	}
	return tx.Commit()
}

// InsertEdges bulk-inserts edges (transactional). dispatch_kind (schema 1.7)
// is written as the empty string for non-`invokes` edges; SQLite
// stores it as a regular TEXT value either way.
func (s *sqliteStore) InsertEdges(edges []types.Edge) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	stmt, err := tx.Prepare(`INSERT INTO edges
		(src, dst, type, file_path, line, count, confidence, dispatch_kind)
		VALUES (?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer func() { _ = stmt.Close() }()
	for _, e := range edges {
		if _, err := stmt.Exec(e.Src, e.Dst, string(e.Type), e.FilePath, e.Line,
			e.Count, string(e.Confidence), e.DispatchKind); err != nil {
			return fmt.Errorf("insert edge %s->%s: %w", e.Src, e.Dst, err)
		}
	}
	return tx.Commit()
}

// InsertPkgTree bulk-inserts package-tree edges.
func (s *sqliteStore) InsertPkgTree(edges []ClusterEdge) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO pkg_tree (parent_id, child_id, level) VALUES (?,?,?)`)
	if err != nil {
		return err
	}
	defer func() { _ = stmt.Close() }()
	for _, e := range edges {
		if _, err := stmt.Exec(e.ParentID, e.ChildID, e.Level); err != nil {
			return fmt.Errorf("insert pkg_tree %s->%s: %w", e.ParentID, e.ChildID, err)
		}
	}
	return tx.Commit()
}

// InsertPkgTreeFromCluster adapts cluster.PersistClusterEdge slices to the
// internal ClusterEdge type and delegates to InsertPkgTree.
func (s *sqliteStore) InsertPkgTreeFromCluster(edges []cluster.PersistClusterEdge) error {
	out := make([]ClusterEdge, len(edges))
	for i, e := range edges {
		out[i] = ClusterEdge(e)
	}
	return s.InsertPkgTree(out)
}

// InsertTopicTree persists multi-resolution Leiden communities. Existing rows
// are dropped first so a full rebuild matches V0 expectations.
func (s *sqliteStore) InsertTopicTree(t TopicTreeInput) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`DELETE FROM topic_tree`); err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT INTO topic_tree (parent_id, child_id, resolution, topic_label) VALUES (?,?,?,?)`)
	if err != nil {
		return err
	}
	defer func() { _ = stmt.Close() }()
	for i := 0; i < t.ResolutionsCount(); i++ {
		members := t.ResolutionMembers(i)
		for label, ids := range members {
			for _, id := range ids {
				if _, err := stmt.Exec(nil, id, i, label); err != nil {
					return fmt.Errorf("insert topic_tree %s@%d: %w", id, i, err)
				}
			}
		}
	}
	return tx.Commit()
}

// InsertBlobs stores per-node source slices keyed by node ID.
func (s *sqliteStore) InsertBlobs(blobs map[string][]byte) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO blobs (node_id, source) VALUES (?, ?)`)
	if err != nil {
		return err
	}
	defer func() { _ = stmt.Close() }()
	for id, b := range blobs {
		if _, err := stmt.Exec(id, b); err != nil {
			return fmt.Errorf("insert blob %s: %w", id, err)
		}
	}
	return tx.Commit()
}

// InsertNodePRs writes PR breadcrumbs. INSERT OR REPLACE
// (rather than OR IGNORE) because the rebuild path frequently has new
// title/summary text for the same (node_id, number) — keeping the latest
// commit-message-derived metadata is more useful than rejecting the
// update. PRIMARY KEY (node_id, number) bounds duplicates per node.
func (s *sqliteStore) InsertNodePRs(byNode map[string][]types.PRRef) error {
	if len(byNode) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO node_prs
		(node_id, number, title, summary, base_sha, head_sha, merged_at, repo)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer func() { _ = stmt.Close() }()
	for nodeID, refs := range byNode {
		for _, r := range refs {
			if _, err := stmt.Exec(nodeID, r.Number, r.Title, r.Summary,
				r.BaseSHA, r.HeadSHA, r.MergedAtUTC.UTC().Format(time.RFC3339), r.Repo); err != nil {
				return fmt.Errorf("insert node_pr %s #%d: %w", nodeID, r.Number, err)
			}
		}
	}
	return tx.Commit()
}

// RebuildFTS reloads the FTS5 virtual table from the nodes content table.
func (s *sqliteStore) RebuildFTS() error {
	_, err := s.db.Exec(`INSERT INTO nodes_fts(nodes_fts) VALUES('rebuild')`)
	return err
}
