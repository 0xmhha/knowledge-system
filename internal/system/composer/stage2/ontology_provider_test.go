package stage2

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/0xmhha/knowledge-system/internal/system/ckgclient"
	"github.com/0xmhha/knowledge-system/internal/system/semantic"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

type providerFunc func(context.Context) (OntologyResolver, error)

func (f providerFunc) Resolve(ctx context.Context) (OntologyResolver, error) { return f(ctx) }

func TestProvidedOntologyRunsAfterRawSearchAndPreservesCappedSet(t *testing.T) {
	commit := strings.Repeat("a", 40)
	hits := []contract.Hit{
		{Citation: contract.Citation{File: "a.go", StartLine: 1, EndLine: 2, CommitHash: commit}},
		{Citation: contract.Citation{File: "b.go", StartLine: 1, EndLine: 2, CommitHash: commit}, CanonicalID: "pkg.Beta"},
		{Citation: contract.Citation{File: "c.go", StartLine: 1, EndLine: 2, CommitHash: commit}, CanonicalID: "pkg.Outside"},
	}
	f := fakeOntology{snapshot: semantic.Snapshot{Commit: commit}, candidates: []semantic.ConceptCandidate{{ConceptID: "beta", Status: semantic.StatusVerified}}, implementations: map[string][]semantic.CodeImplementation{"beta": {{AssertionID: "verified", CanonicalID: "pkg.Beta", Evidence: semantic.EvidenceSpan{Snapshot: semantic.Snapshot{Commit: commit}, Path: "b.go", StartLine: 1, EndLine: 2}}}}}
	backend := &ckgclient.Fake{}
	p := providerFunc(func(ctx context.Context) (OntologyResolver, error) {
		if len(backend.Calls.BM25Search) == 0 {
			t.Fatal("ontology evaluated before raw graph retrieval")
		}
		return f, nil
	})
	cfg := DefaultConfig()
	cfg.MaxCitations = 2
	s, err := New(backend, WithConfig(cfg), WithOntologyProvider(p, 0.2, time.Second))
	if err != nil {
		t.Fatal(err)
	}
	out, err := s.Search(context.Background(), "Beta", []string{"Beta"}, hits, contract.IntentBugFix)
	if err != nil {
		t.Fatal(err)
	}
	if out.Ontology == nil || out.Ontology.State != "active" || out.Ontology.AppliedRelations != 1 || len(out.Citations) != 2 || out.Citations[0].Citation.File != "b.go" || out.Citations[1].Citation.File != "a.go" {
		t.Fatalf("bad optional rerank: %+v", out)
	}
}

func TestProvidedOntologyFallbackAndAmbiguityDoNotReplaceBaseline(t *testing.T) {
	commit := strings.Repeat("a", 40)
	hits := []contract.Hit{{Citation: contract.Citation{File: "main.go", StartLine: 1, EndLine: 2, CommitHash: commit}, CanonicalID: "pkg.Alpha"}}
	plain, _ := New(&ckgclient.Fake{})
	base, _ := plain.Search(context.Background(), "Alpha", nil, hits, contract.IntentBugFix)
	for _, tc := range []struct {
		name, state string
		provider    providerFunc
	}{
		{"missing", "unavailable", func(context.Context) (OntologyResolver, error) { return nil, ErrOntologyUnavailable }},
		{"stale", "stale", func(context.Context) (OntologyResolver, error) { return nil, ErrOntologyStale }},
		{"timeout", "budget_exceeded", func(ctx context.Context) (OntologyResolver, error) { <-ctx.Done(); return nil, ctx.Err() }},
		{"unregistered", "no_match", func(context.Context) (OntologyResolver, error) {
			return fakeOntology{snapshot: semantic.Snapshot{Commit: commit}}, nil
		}},
		{"ambiguous proposed", "active", func(context.Context) (OntologyResolver, error) {
			return fakeOntology{snapshot: semantic.Snapshot{Commit: commit}, candidates: []semantic.ConceptCandidate{{ConceptID: "one", Status: semantic.StatusProposed}, {ConceptID: "two", Status: semantic.StatusProposed}}}, nil
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, err := New(&ckgclient.Fake{}, WithOntologyProvider(tc.provider, 0.2, 10*time.Millisecond))
			if err != nil {
				t.Fatal(err)
			}
			out, err := s.Search(context.Background(), "Alpha", nil, hits, contract.IntentBugFix)
			if err != nil || !reflect.DeepEqual(out.Citations, base.Citations) || out.Ontology == nil || out.Ontology.State != tc.state || out.Ontology.AppliedRelations != 0 {
				t.Fatalf("fallback changed raw baseline: %+v %v", out, err)
			}
			if tc.name == "ambiguous proposed" && len(out.ConceptCandidates) != 2 {
				t.Fatal("ambiguity was collapsed")
			}
		})
	}
}

func TestProvidedOntologyLimitsTiesWithoutAssertingOneMeaning(t *testing.T) {
	f := fakeOntology{candidates: make([]semantic.ConceptCandidate, 9)}
	p := providerFunc(func(context.Context) (OntologyResolver, error) { return f, nil })
	s, _ := New(&ckgclient.Fake{}, WithOntologyProvider(p, 0.2, time.Second))
	hits := []contract.Hit{{Citation: contract.Citation{File: "main.go", StartLine: 1, EndLine: 2}}}
	out, err := s.Search(context.Background(), "ambiguous", nil, hits, contract.IntentBugFix)
	if err != nil || out.Ontology.State != "budget_exceeded" || out.Ontology.Reason != "concept_limit" || len(out.ConceptCandidates) != 0 || len(out.Citations) != 1 {
		t.Fatalf("unbounded tie handling: %+v %v", out, err)
	}
	for _, budget := range []time.Duration{0, -time.Millisecond, 6 * time.Second} {
		if _, err := New(&ckgclient.Fake{}, WithOntologyProvider(p, 0.2, budget)); err == nil {
			t.Fatal("invalid budget accepted")
		}
	}
}

// A verified concept whose IMPLEMENTED_BY relation is not verified exposes no
// implementations (semantic.ActiveProjection.VerifiedImplementations, covered
// by TestActiveProjectionExposesOnlyVerifiedImplementations). Here the
// composed search keeps the exact baseline, including a CKG-only citation,
// while the verified-relation positive control proves the arm can reorder.
func TestProvidedOntologyUnverifiedRelationPreservesCKGAndCKVBaseline(t *testing.T) {
	commit := strings.Repeat("a", 40)
	ckvHits := []contract.Hit{
		{Citation: contract.Citation{File: "a.go", StartLine: 1, EndLine: 2, CommitHash: commit}},
		{Citation: contract.Citation{File: "b.go", StartLine: 1, EndLine: 2, CommitHash: commit}, CanonicalID: "pkg.Beta"},
	}
	graphOnly := contract.Citation{File: "graph.go", StartLine: 5, EndLine: 9, CommitHash: commit}
	backend := func() *ckgclient.Fake {
		return &ckgclient.Fake{BM25Hits: []contract.Hit{{Citation: graphOnly, CanonicalID: "pkg.Graph"}}}
	}
	verifiedConcept := []semantic.ConceptCandidate{{ConceptID: "beta", Status: semantic.StatusVerified}}
	plain, err := New(backend())
	if err != nil {
		t.Fatal(err)
	}
	base, err := plain.Search(context.Background(), "Beta", []string{"Beta"}, ckvHits, contract.IntentBugFix)
	if err != nil {
		t.Fatal(err)
	}
	hasGraph := false
	for _, c := range base.Citations {
		hasGraph = hasGraph || c.Citation.Key() == graphOnly.Key()
	}
	if !hasGraph || len(base.Citations) != 3 || base.Citations[0].Citation.File == "b.go" {
		t.Fatalf("fixture baseline must hold the CKG-only citation and not already rank b.go first: %+v", base.Citations)
	}
	run := func(implementations map[string][]semantic.CodeImplementation) Stage2Output {
		t.Helper()
		f := fakeOntology{snapshot: semantic.Snapshot{Commit: commit}, candidates: verifiedConcept, implementations: implementations}
		s, err := New(backend(), WithOntologyProvider(providerFunc(func(context.Context) (OntologyResolver, error) { return f, nil }), 0.2, time.Second))
		if err != nil {
			t.Fatal(err)
		}
		out, err := s.Search(context.Background(), "Beta", []string{"Beta"}, ckvHits, contract.IntentBugFix)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	unverified := run(nil)
	if !reflect.DeepEqual(unverified.Citations, base.Citations) || unverified.Ontology == nil ||
		unverified.Ontology.State != "active" || unverified.Ontology.AppliedRelations != 0 || unverified.Ontology.BoostedCitations != 0 {
		t.Fatalf("unverified relation changed the CKV+CKG baseline: %+v", unverified)
	}
	if len(unverified.ConceptCandidates) != 1 || unverified.ConceptCandidates[0].Status != semantic.StatusVerified {
		t.Fatalf("verified concept interpretation was lost: %+v", unverified.ConceptCandidates)
	}
	verified := run(map[string][]semantic.CodeImplementation{"beta": {{AssertionID: "verified", CanonicalID: "pkg.Beta",
		Evidence: semantic.EvidenceSpan{Snapshot: semantic.Snapshot{Commit: commit}, Path: "b.go", StartLine: 1, EndLine: 2}}}})
	if verified.Ontology == nil || verified.Ontology.AppliedRelations != 1 || reflect.DeepEqual(verified.Citations, base.Citations) {
		t.Fatalf("positive control: a verified relation must be able to reorder: %+v", verified)
	}
}
