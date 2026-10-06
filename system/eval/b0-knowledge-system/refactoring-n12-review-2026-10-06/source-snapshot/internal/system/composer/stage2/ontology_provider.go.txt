package stage2

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/0xmhha/knowledge-system/internal/system/ckvclient"
	"github.com/0xmhha/knowledge-system/internal/system/semantic"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

var (
	ErrOntologyUnavailable = errors.New("ontology unavailable")
	ErrOntologyStale       = errors.New("ontology stale")
	ErrOntologyTextSearch  = errors.New("ontology text search unavailable")
)

// OntologyProvider resolves the exact dataset after raw CKV/CKG retrieval.
// Implementations must honor ctx and never activate another semantic version.
type OntologyProvider interface {
	Resolve(context.Context) (OntologyResolver, error)
}

// OntologyTextResolver provides reviewed definitions, independently of edges.
type OntologyTextResolver interface {
	VerifiedConceptText(string) (semantic.ConceptText, bool)
}

// WithOntologyTextSearch reuses the raw-query CKV client, embedding identity
// and recall K. Combined mode also applies reviewed relations. No new index,
// vector space, candidate set or inferred implementation edge is introduced.
func WithOntologyTextSearch(client ckvclient.Client, k int, combined bool) Option {
	return func(s *Searcher) {
		s.ontologyMode = "concept_text"
		if combined {
			s.ontologyMode = "combined"
		}
		s.ontologyText, s.ontologyTextK = client, k
	}
}

func (s *Searcher) providedOntologyMode() string {
	if s.ontologyMode != "" {
		return s.ontologyMode
	}
	return "relations"
}

// WithOntologyProvider enables a pinned optional arm with cooperative deadlines.
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
	diagnostic := &contract.OntologyDiagnostic{Mode: s.providedOntologyMode(), State: "unavailable", BaselineCitations: len(baseline)}
	if s.ontologyProvider == nil {
		return baseline, nil, diagnostic
	}
	queryCtx, cancel := context.WithTimeout(ctx, s.ontologyBudget)
	defer cancel()
	fallback := func(err error) ([]ScoredCitation, []semantic.ConceptCandidate, *contract.OntologyDiagnostic) {
		switch {
		case queryCtx.Err() != nil || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled):
			diagnostic.State, diagnostic.Reason = "budget_exceeded", "deadline"
		case errors.Is(err, ErrOntologyTextSearch):
			diagnostic.State, diagnostic.Reason = "unavailable", "text_search_failed"
		case errors.Is(err, ErrOntologyUnavailable):
			diagnostic.State, diagnostic.Reason = "unavailable", "store_or_dataset_missing"
		default:
			diagnostic.State, diagnostic.Reason = "stale", "alignment_or_evidence_invalid"
		}
		diagnostic.AppliedRelations = 0
		diagnostic.BoostedCitations = 0
		diagnostic.TextBoostedCitations = 0
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
		if diagnostic.Mode == "concept_text" {
			break
		}
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
	original := make(map[string]float64, len(baseline))
	for _, citation := range baseline {
		allowed[citation.Citation.Key()] = true
		original[citation.Citation.Key()] = agg.byCitation[citation.Citation.Key()].Score
	}
	// Finish all fallible text work before changing scores. Failures discard
	// the whole optional arm, including relations in combined mode.
	type textSignal struct {
		fraction float64
		proof    string
	}
	signals := map[string]textSignal{}
	if s.ontologyText != nil {
		textResolver, ok := resolver.(OntologyTextResolver)
		if !ok {
			return fallback(ErrOntologyUnavailable)
		}
		for _, candidate := range candidates {
			if candidate.Status != semantic.StatusVerified {
				continue
			}
			text, ok := textResolver.VerifiedConceptText(candidate.ConceptID)
			e := text.Evidence
			if !ok || text.ConceptID != candidate.ConceptID || text.ReviewedBy == "" ||
				e.Kind != semantic.SourceDocument || e.Snapshot != resolver.Snapshot() || text.Text == "" {
				return fallback(ErrOntologyStale)
			}
			if len(text.Text) > 6144 {
				diagnostic.State, diagnostic.Reason = "budget_exceeded", "text_byte_limit"
				return baseline, nil, diagnostic
			}
			sum := sha256.Sum256([]byte(text.Text))
			diagnostic.TextSources = append(diagnostic.TextSources, contract.OntologyTextSource{
				ConceptID: text.ConceptID, EvidenceID: e.ID, QuerySHA256: fmt.Sprintf("%x", sum),
				ProjectID: e.Snapshot.ProjectID, DatasetID: e.Snapshot.DatasetID, SnapshotID: e.Snapshot.SnapshotID,
				Commit: e.Snapshot.Commit, File: e.Path, StartLine: e.StartLine, EndLine: e.EndLine, ContentSHA256: e.ContentSHA256})
			diagnostic.TextSearchCalls++
			textHits, err := s.ontologyText.SemanticSearch(queryCtx, text.Text, ckvclient.SearchOpts{K: s.ontologyTextK, BM25Rerank: true})
			if err != nil {
				return fallback(ErrOntologyTextSearch)
			}
			if queryCtx.Err() != nil {
				return fallback(queryCtx.Err())
			}
			diagnostic.TextSources[len(diagnostic.TextSources)-1].ReturnedHits = len(textHits)
			for i, hit := range textHits {
				key := hit.Citation.Key()
				if !allowed[key] || hit.Citation.CommitHash != resolver.Snapshot().Commit || ontologyHeaderShadowed(hit, hits, allowed) {
					continue
				}
				fraction := s.ontologyBoost / float64(i+1)
				if fraction > signals[key].fraction {
					signals[key] = textSignal{fraction, fmt.Sprintf("ontology_text:%s@source=%s@rank=%d", text.ConceptID, e.ID, i+1)}
				}
			}
		}
	}
	if diagnostic.Mode != "concept_text" {
		applyOntologyBoost(agg, prepared, prompt, hits, s.ontologyBoost, allowed)
	}
	for key, signal := range signals {
		sc := agg.byCitation[key]
		score := math.Min(original[key]*(1+s.ontologyBoost), sc.Score+original[key]*signal.fraction)
		if score > sc.Score {
			sc.Score = score
			sc.Sources = append(sc.Sources, signal.proof)
		}
	}
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
		textBoosted := false
		for _, source := range citation.Sources {
			if strings.HasPrefix(source, "ontology:") {
				proofs[source] = true
				boosted = true
			}
			if strings.HasPrefix(source, "ontology_text:") {
				textBoosted, boosted = true, true
			}
		}
		if boosted {
			diagnostic.BoostedCitations++
		}
		if textBoosted {
			diagnostic.TextBoostedCitations++
		}
	}
	diagnostic.AppliedRelations = len(proofs)
	diagnostic.State = "active"
	return ranked, candidates, diagnostic
}
