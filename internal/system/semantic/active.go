package semantic

import (
	"context"
)

// ActiveProjection can only be obtained after the current semantic dataset
// has been checked against source bytes and the live CKG/CKV coordinates.
// Query rerankers receive this read-only view rather than an arbitrary JSON
// projection, so unreviewed or stale relations cannot supply a score boost.
type ActiveProjection struct{ projection Projection }

func (s *Store) CurrentAligned(ctx context.Context, projectID, repoRoot, graphDir, vectorDir string) (ActiveProjection, error) {
	p, err := s.Current(ctx, projectID)
	if err != nil {
		return ActiveProjection{}, err
	}
	if err := validateActiveProjection(ctx, p, repoRoot, graphDir, vectorDir); err != nil {
		return ActiveProjection{}, err
	}
	return ActiveProjection{projection: p}, nil
}

func validateActiveProjection(ctx context.Context, p Projection, repoRoot, graphDir, vectorDir string) error {
	if err := ValidateDatasetAlignment(p, repoRoot, graphDir, vectorDir); err != nil {
		return err
	}
	if err := ValidateCodeAnchors(p, graphDir); err != nil {
		return err
	}
	if err := ValidateCKVChunkLinks(ctx, p, vectorDir); err != nil {
		return err
	}
	if err := p.ValidateSources(ctx, repoRoot); err != nil {
		return err
	}
	return nil
}

func (a ActiveProjection) Snapshot() Snapshot { return a.projection.Snapshot }

func (a ActiveProjection) MatchConcepts(query string, limit int) ([]ConceptCandidate, error) {
	return a.projection.MatchConcepts(query, limit)
}

// CodeImplementation is one reviewed concept→code link plus the source span
// that must overlap a candidate citation for a soft rerank boost.
type CodeImplementation struct {
	AssertionID string
	CanonicalID string
	Evidence    EvidenceSpan
}

func (a ActiveProjection) VerifiedImplementations(conceptID string) []CodeImplementation {
	var out []CodeImplementation
	conceptVerified := false
	for _, concept := range a.projection.Concepts {
		if concept.ID == conceptID && concept.Status == StatusVerified {
			conceptVerified = true
			break
		}
	}
	if !conceptVerified {
		return nil
	}
	evidence := map[string]EvidenceSpan{}
	for _, e := range a.projection.Evidence {
		evidence[e.ID] = e
	}
	for _, assertion := range a.projection.Assertions {
		if assertion.Predicate != PredicateImplementedBy || assertion.Status != StatusVerified || assertion.SubjectID != conceptID {
			continue
		}
		for _, id := range assertion.EvidenceIDs {
			e := evidence[id]
			if e.CanonicalID == assertion.ObjectID && (e.Kind == SourceCode || e.Kind == SourceTest) {
				out = append(out, CodeImplementation{AssertionID: assertion.ID,
					CanonicalID: e.CanonicalID, Evidence: e})
			}
		}
	}
	return out
}
