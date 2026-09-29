package semantic

import (
	"fmt"

	graphstore "github.com/0xmhha/knowledge-system/pkg/graph/store"
	graphtypes "github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// AttachCodeEvidence accepts a code/test span only when CKG resolves its
// canonical symbol in the same committed snapshot and the span lies inside
// that AST node. It does not promote any Claim to verified; Store.Put still
// checks the source bytes at the commit before activation.
func AttachCodeEvidence(p *Projection, graph graphstore.Reader, e EvidenceSpan) error {
	if p == nil || graph == nil {
		return fmt.Errorf("semantic: projection and graph reader are required")
	}
	if err := p.Validate(); err != nil {
		return err
	}
	if e.CanonicalID == "" {
		return fmt.Errorf("semantic: code evidence %q needs canonical_id", e.ID)
	}
	manifest, err := graphstore.GetManifest(graph)
	if err != nil {
		return fmt.Errorf("semantic: read CKG manifest: %w", err)
	}
	node, found, err := graph.FindByCanonicalID(e.CanonicalID)
	if err != nil {
		return fmt.Errorf("semantic: resolve CKG canonical_id %q: %w", e.CanonicalID, err)
	}
	if !found {
		return fmt.Errorf("semantic: CKG canonical_id %q not found", e.CanonicalID)
	}
	if err := checkCodeAnchor(p.Snapshot, manifest, node, e); err != nil {
		return err
	}
	for _, existing := range p.Evidence {
		if existing.ID == e.ID {
			return fmt.Errorf("semantic: duplicate evidence ID %q", e.ID)
		}
	}
	next := *p
	next.Evidence = append(append([]EvidenceSpan(nil), p.Evidence...), e)
	if err := next.Validate(); err != nil {
		return err
	}
	*p = next
	return nil
}

func checkCodeAnchor(snapshot Snapshot, manifest graphstore.Manifest, node graphtypes.Node, e EvidenceSpan) error {
	if manifest.CommitHash != snapshot.Commit || e.Snapshot != snapshot {
		return fmt.Errorf("semantic: code anchor crosses project or snapshot")
	}
	if e.Kind != SourceCode && e.Kind != SourceTest {
		return fmt.Errorf("semantic: anchor %q is not code or test evidence", e.ID)
	}
	if e.CanonicalID == "" || node.CanonicalID != e.CanonicalID ||
		node.FilePath != e.Path || e.StartLine < node.StartLine || e.EndLine > node.EndLine || e.StartLine < 1 || e.EndLine < e.StartLine {
		return fmt.Errorf("semantic: anchor %q does not fit CKG symbol %q", e.ID, e.CanonicalID)
	}
	return nil
}
