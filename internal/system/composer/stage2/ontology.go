package stage2

import (
	"fmt"

	"github.com/0xmhha/knowledge-system/internal/system/semantic"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

// OntologyResolver exposes an aligned, read-only semantic view. Production
// callers obtain one from semantic.Store.CurrentAligned before opting in.
type OntologyResolver interface {
	Snapshot() semantic.Snapshot
	MatchConcepts(query string, limit int) ([]semantic.ConceptCandidate, error)
	VerifiedImplementations(conceptID string) []semantic.CodeImplementation
}

// applyOntologyBoost can only change the order of citations already found by
// CKV/CKG. A reviewed code edge must agree with the exact CKV canonical ID,
// source commit, file, and overlapping source lines before it contributes.
func applyOntologyBoost(a *aggregator, resolver OntologyResolver, query string, ckvHits []contract.Hit, boost float64, allowed map[string]bool) []semantic.ConceptCandidate {
	candidates, err := resolver.MatchConcepts(query, 8)
	if err != nil {
		return nil
	}
	snapshot := resolver.Snapshot()
	boosted := make(map[string]bool)
	for _, candidate := range candidates {
		if candidate.Status != semantic.StatusVerified {
			continue
		}
		for _, implementation := range resolver.VerifiedImplementations(candidate.ConceptID) {
			if implementation.CanonicalID == "" {
				continue
			}
			for _, hit := range ckvHits {
				c := hit.Citation
				e := implementation.Evidence
				if e.Snapshot != snapshot || hit.CanonicalID != implementation.CanonicalID || c.CommitHash != snapshot.Commit ||
					c.File != e.Path || c.StartLine > e.EndLine || c.EndLine < e.StartLine {
					continue
				}
				sc, ok := a.byCitation[c.Key()]
				if !ok || sc.Citation.CommitHash != snapshot.Commit || boosted[c.Key()] || (allowed != nil && !allowed[c.Key()]) {
					continue
				}
				sc.Score *= 1 + boost
				sc.Sources = append(sc.Sources, fmt.Sprintf("ontology:%s@assertion=%s", candidate.ConceptID, implementation.AssertionID))
				boosted[c.Key()] = true
			}
		}
	}
	return candidates
}
