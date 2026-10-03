package stage2

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/0xmhha/knowledge-system/internal/system/semantic"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

var (
	ErrOntologyUnavailable = errors.New("ontology unavailable")
	ErrOntologyStale       = errors.New("ontology stale")
)

// OntologyProvider resolves the exact dataset after raw CKV/CKG retrieval.
// Implementations must honor ctx and never activate another semantic version.
type OntologyProvider interface {
	Resolve(context.Context) (OntologyResolver, error)
}

// WithOntologyProvider enables the relations arm with cooperative deadlines.
// Expired work is discarded, including partially calculated score changes.
func WithOntologyProvider(provider OntologyProvider, boost float64, budget time.Duration) Option {
	return func(s *Searcher) {
		s.ontologyProvider, s.ontologyBoost, s.ontologyBudget = provider, boost, budget
	}
}

type preparedOntology struct {
	snapshot        semantic.Snapshot
	candidates      []semantic.ConceptCandidate
	implementations map[string][]semantic.CodeImplementation
}

func (p preparedOntology) Snapshot() semantic.Snapshot { return p.snapshot }
func (p preparedOntology) MatchConcepts(string, int) ([]semantic.ConceptCandidate, error) {
	return p.candidates, nil
}
func (p preparedOntology) VerifiedImplementations(id string) []semantic.CodeImplementation {
	return p.implementations[id]
}

func (s *Searcher) applyProvidedOntology(ctx context.Context, agg *aggregator, prompt string, hits []contract.Hit, baseline []ScoredCitation, demoteTests, demoteDocs bool) ([]ScoredCitation, []semantic.ConceptCandidate, *contract.OntologyDiagnostic) {
	diagnostic := &contract.OntologyDiagnostic{Mode: "relations", State: "unavailable", BaselineCitations: len(baseline)}
	if s.ontologyProvider == nil {
		return baseline, nil, diagnostic
	}
	queryCtx, cancel := context.WithTimeout(ctx, s.ontologyBudget)
	defer cancel()
	fallback := func(err error) ([]ScoredCitation, []semantic.ConceptCandidate, *contract.OntologyDiagnostic) {
		switch {
		case queryCtx.Err() != nil || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled):
			diagnostic.State, diagnostic.Reason = "budget_exceeded", "deadline"
		case errors.Is(err, ErrOntologyUnavailable):
			diagnostic.State, diagnostic.Reason = "unavailable", "store_or_dataset_missing"
		default:
			diagnostic.State, diagnostic.Reason = "stale", "alignment_or_evidence_invalid"
		}
		diagnostic.AppliedRelations = 0
		return baseline, nil, diagnostic
	}
	resolver, err := s.ontologyProvider.Resolve(queryCtx)
	if err != nil || queryCtx.Err() != nil {
		return fallback(err)
	}
	if resolver == nil {
		return fallback(ErrOntologyUnavailable)
	}
	candidates, err := resolver.MatchConcepts(prompt, 8)
	if err != nil || queryCtx.Err() != nil {
		return fallback(err)
	}
	diagnostic.MatchedConcepts = len(candidates)
	// Ties stay ambiguous. Refuse over-budget work instead of choosing one
	// interpretation or silently dropping tied candidates.
	if len(candidates) > 8 {
		diagnostic.State, diagnostic.Reason = "budget_exceeded", "concept_limit"
		return baseline, nil, diagnostic
	}
	if len(candidates) == 0 {
		diagnostic.State = "no_match"
		return baseline, candidates, diagnostic
	}
	prepared := preparedOntology{snapshot: resolver.Snapshot(), candidates: candidates, implementations: map[string][]semantic.CodeImplementation{}}
	count := 0
	for _, candidate := range candidates {
		if candidate.Status != semantic.StatusVerified {
			continue
		}
		implementations := resolver.VerifiedImplementations(candidate.ConceptID)
		count += len(implementations)
		if count > 128 {
			diagnostic.State, diagnostic.Reason = "budget_exceeded", "relation_limit"
			return baseline, nil, diagnostic
		}
		prepared.implementations[candidate.ConceptID] = implementations
		if queryCtx.Err() != nil {
			return fallback(queryCtx.Err())
		}
	}
	allowed := make(map[string]bool, len(baseline))
	for _, citation := range baseline {
		allowed[citation.Citation.Key()] = true
	}
	applyOntologyBoost(agg, prepared, prompt, hits, s.ontologyBoost, allowed)
	if queryCtx.Err() != nil {
		return fallback(queryCtx.Err())
	}
	// Select from the original capped set, even at exact-score ties. Merely
	// rescoring and recapping the full aggregator could displace a tied hit.
	var ranked []ScoredCitation
	for _, citation := range agg.results(0, demoteTests, demoteDocs) {
		if allowed[citation.Citation.Key()] {
			ranked = append(ranked, citation)
		}
	}
	proofs := map[string]bool{}
	for _, citation := range ranked {
		boosted := false
		for _, source := range citation.Sources {
			if strings.HasPrefix(source, "ontology:") {
				proofs[source] = true
				boosted = true
			}
		}
		if boosted {
			diagnostic.BoostedCitations++
		}
	}
	diagnostic.AppliedRelations = len(proofs)
	diagnostic.State = "active"
	return ranked, candidates, diagnostic
}
