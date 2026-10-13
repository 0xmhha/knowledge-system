package stage2

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strconv"
	"strings"

	"github.com/0xmhha/knowledge-system/internal/system/semantic"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

type ontologyExplanationKey struct{}

// WithOntologyExplanation is request-scoped and does not change retrieval settings.
func WithOntologyExplanation(ctx context.Context) context.Context {
	return context.WithValue(ctx, ontologyExplanationKey{}, true)
}
func ontologyExplanationRequested(ctx context.Context) bool {
	return ctx.Value(ontologyExplanationKey{}) == true
}
func explainOntology(d *contract.OntologyDiagnostic, before, after []ScoredCitation, candidates []semantic.ConceptCandidate) {
	if d.Explanation == nil {
		return
	}
	e := d.Explanation
	for _, c := range candidates {
		e.Matches = append(e.Matches, contract.OntologyConceptMatch{ConceptID: c.ConceptID, Status: string(c.Status)})
	}
	positions := map[string]int{}
	scores := map[string]float64{}
	for i, c := range before {
		positions[c.Citation.Key()] = i + 1
		scores[c.Citation.Key()] = c.Score
	}
	for i, c := range after {
		key := c.Citation.Key()
		if positions[key] == i+1 && scores[key] == c.Score {
			continue
		}
		e.TotalChanged++
		if len(e.Changes) >= 20 {
			e.Truncated = true
			continue
		}
		sum := sha256.Sum256([]byte(key))
		change := contract.OntologyRankChange{CitationKeySHA256: fmt.Sprintf("%x", sum), RankBefore: positions[key], RankAfter: i + 1, ScoreBefore: strconv.FormatFloat(scores[key], 'g', -1, 64), ScoreAfter: strconv.FormatFloat(c.Score, 'g', -1, 64)}
		for _, source := range c.Sources {
			if prefix, ok := strings.CutPrefix(source, "ontology:"); ok {
				if _, id, found := strings.Cut(prefix, "@assertion="); found {
					change.RelationIDs = append(change.RelationIDs, id)
				}
			}
			if prefix, ok := strings.CutPrefix(source, "ontology_text:"); ok {
				if _, tail, found := strings.Cut(prefix, "@source="); found {
					if at := strings.LastIndex(tail, "@rank="); at >= 0 {
						change.TextEvidenceIDs = append(change.TextEvidenceIDs, tail[:at])
					}
				}
			}
		}
		e.Changes = append(e.Changes, change)
	}
	e.State = "no_effect"
	if e.TotalChanged > 0 {
		e.State = "applied"
	}
}
