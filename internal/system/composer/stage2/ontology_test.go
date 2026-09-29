package stage2

import (
	"context"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/system/ckgclient"
	"github.com/0xmhha/knowledge-system/internal/system/semantic"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

type fakeOntology struct {
	snapshot        semantic.Snapshot
	candidates      []semantic.ConceptCandidate
	implementations map[string][]semantic.CodeImplementation
	err             error
}

func (f fakeOntology) Snapshot() semantic.Snapshot { return f.snapshot }
func (f fakeOntology) MatchConcepts(string, int) ([]semantic.ConceptCandidate, error) {
	return f.candidates, f.err
}
func (f fakeOntology) VerifiedImplementations(id string) []semantic.CodeImplementation {
	return f.implementations[id]
}

func TestOntologyBoostOnlyExistingSameSnapshotCanonicalCitation(t *testing.T) {
	commit := strings.Repeat("a", 40)
	matching := contract.Citation{File: "main.go", StartLine: 10, EndLine: 13, CommitHash: commit}
	other := contract.Citation{File: "other.go", StartLine: 2, EndLine: 2, CommitHash: commit}
	hits := []contract.Hit{{Citation: other}, {Citation: matching, CanonicalID: "module.Alpha"}}
	a := newAggregator(60, 1, 1, 5)
	a.addCkvList(hits)
	beforeKeys := keys(a)
	before := a.byCitation[matching.Key()].Score
	f := fakeOntology{
		snapshot:   semantic.Snapshot{Commit: commit},
		candidates: []semantic.ConceptCandidate{{ConceptID: "alpha", Status: semantic.StatusVerified}},
		implementations: map[string][]semantic.CodeImplementation{"alpha": {{AssertionID: "r1", CanonicalID: "module.Alpha",
			Evidence: semantic.EvidenceSpan{Snapshot: semantic.Snapshot{Commit: commit}, Path: "main.go", StartLine: 11, EndLine: 12}}}},
	}
	f.candidates = append(f.candidates, f.candidates[0])
	candidates := applyOntologyBoost(a, f, "alpha", hits, 0.2, nil)
	if len(candidates) != 2 || !reflect.DeepEqual(keys(a), beforeKeys) {
		t.Fatalf("candidate set changed: %+v, %v", candidates, keys(a))
	}
	if got := a.byCitation[matching.Key()].Score; got != before*1.2 {
		t.Fatalf("boost %v, want %v", got, before*1.2)
	}
	if got := a.results(0, false, false)[0].Citation; got != matching {
		t.Fatalf("reviewed implementation did not rerank existing hit: %+v", got)
	}
	if got := a.byCitation[matching.Key()].Sources; len(got) != 2 || got[1] != "ontology:alpha@assertion=r1" {
		t.Fatalf("missing provenance: %v", got)
	}
}

func TestOntologyBoostFailsOpenOnUnverifiedOrMismatchedProof(t *testing.T) {
	commit := strings.Repeat("a", 40)
	c := contract.Citation{File: "main.go", StartLine: 10, EndLine: 13, CommitHash: commit}
	base := contract.Hit{Citation: c, CanonicalID: "module.Alpha"}
	impl := semantic.CodeImplementation{AssertionID: "r1", CanonicalID: base.CanonicalID,
		Evidence: semantic.EvidenceSpan{Snapshot: semantic.Snapshot{Commit: commit}, Path: c.File, StartLine: 11, EndLine: 12}}
	cases := []struct {
		name string
		f    fakeOntology
		hit  contract.Hit
	}{
		{"proposed meaning", fakeOntology{snapshot: semantic.Snapshot{Commit: commit}, candidates: []semantic.ConceptCandidate{{ConceptID: "alpha", Status: semantic.StatusProposed}}, implementations: map[string][]semantic.CodeImplementation{"alpha": {impl}}}, base},
		{"stale commit", fakeOntology{snapshot: semantic.Snapshot{Commit: strings.Repeat("b", 40)}, candidates: []semantic.ConceptCandidate{{ConceptID: "alpha", Status: semantic.StatusVerified}}, implementations: map[string][]semantic.CodeImplementation{"alpha": {impl}}}, base},
		{"different file", fakeOntology{snapshot: semantic.Snapshot{Commit: commit}, candidates: []semantic.ConceptCandidate{{ConceptID: "alpha", Status: semantic.StatusVerified}}, implementations: map[string][]semantic.CodeImplementation{"alpha": {{AssertionID: "r1", CanonicalID: base.CanonicalID, Evidence: semantic.EvidenceSpan{Snapshot: semantic.Snapshot{Commit: commit}, Path: "else.go", StartLine: 11, EndLine: 12}}}}}, base},
		{"different symbol", fakeOntology{snapshot: semantic.Snapshot{Commit: commit}, candidates: []semantic.ConceptCandidate{{ConceptID: "alpha", Status: semantic.StatusVerified}}, implementations: map[string][]semantic.CodeImplementation{"alpha": {impl}}}, contract.Hit{Citation: c, CanonicalID: "module.Beta"}},
		{"nonoverlapping lines", fakeOntology{snapshot: semantic.Snapshot{Commit: commit}, candidates: []semantic.ConceptCandidate{{ConceptID: "alpha", Status: semantic.StatusVerified}}, implementations: map[string][]semantic.CodeImplementation{"alpha": {{AssertionID: "r1", CanonicalID: base.CanonicalID, Evidence: semantic.EvidenceSpan{Snapshot: semantic.Snapshot{Commit: commit}, Path: c.File, StartLine: 20, EndLine: 25}}}}}, base},
		{"resolver error", fakeOntology{err: errors.New("unavailable")}, base},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := newAggregator(60, 1, 1, 5)
			a.addCkvList([]contract.Hit{tc.hit})
			before := *a.byCitation[c.Key()]
			applyOntologyBoost(a, tc.f, "alpha", []contract.Hit{tc.hit}, 0.2, nil)
			if got := a.byCitation[c.Key()]; got.Score != before.Score || !reflect.DeepEqual(got.Sources, before.Sources) {
				t.Fatalf("unsupported proof changed retrieval: %+v", got)
			}
		})
	}
}

func TestOntologyBoostKeepsBaselineCap(t *testing.T) {
	commit := strings.Repeat("a", 40)
	first := contract.Hit{Citation: contract.Citation{File: "a.go", StartLine: 1, EndLine: 2, CommitHash: commit}}
	second := contract.Hit{Citation: contract.Citation{File: "b.go", StartLine: 3, EndLine: 4, CommitHash: commit}, CanonicalID: "module.Second"}
	a := newAggregator(60, 1, 1, 5)
	a.addCkvList([]contract.Hit{first, second})
	f := fakeOntology{snapshot: semantic.Snapshot{Commit: commit},
		candidates: []semantic.ConceptCandidate{{ConceptID: "second", Status: semantic.StatusVerified}},
		implementations: map[string][]semantic.CodeImplementation{"second": {{AssertionID: "r2", CanonicalID: "module.Second",
			Evidence: semantic.EvidenceSpan{Snapshot: semantic.Snapshot{Commit: commit}, Path: "b.go", StartLine: 3, EndLine: 4}}}},
	}
	baseline := a.results(1, false, false)
	allowed := map[string]bool{baseline[0].Citation.Key(): true}
	applyOntologyBoost(a, f, "second", []contract.Hit{first, second}, 0.2, allowed)
	if got := a.results(1, false, false); !reflect.DeepEqual(got, baseline) {
		t.Fatalf("ontology displaced baseline citation: %+v, want %+v", got, baseline)
	}
}

func TestSearchOntologyIsOptionalAndPreservesCappedSet(t *testing.T) {
	commit := strings.Repeat("a", 40)
	hits := []contract.Hit{
		{Citation: contract.Citation{File: "a.go", StartLine: 1, EndLine: 2, CommitHash: commit}},
		{Citation: contract.Citation{File: "b.go", StartLine: 3, EndLine: 4, CommitHash: commit}, CanonicalID: "module.Second"},
		{Citation: contract.Citation{File: "c.go", StartLine: 5, EndLine: 6, CommitHash: commit}},
	}
	f := fakeOntology{snapshot: semantic.Snapshot{Commit: commit},
		candidates: []semantic.ConceptCandidate{{ConceptID: "second", Status: semantic.StatusVerified}},
		implementations: map[string][]semantic.CodeImplementation{"second": {{AssertionID: "r2", CanonicalID: "module.Second",
			Evidence: semantic.EvidenceSpan{Snapshot: semantic.Snapshot{Commit: commit}, Path: "b.go", StartLine: 3, EndLine: 4}}}},
	}
	cfg := DefaultConfig()
	cfg.MaxCitations = 2
	baseline, err := New(&ckgclient.Fake{}, WithConfig(cfg))
	if err != nil {
		t.Fatal(err)
	}
	opted, err := New(&ckgclient.Fake{}, WithConfig(cfg), WithOntologyResolver(f, 0.2))
	if err != nil {
		t.Fatal(err)
	}
	plain, err := baseline.Search(context.Background(), "second", nil, hits, contract.IntentBugFix)
	if err != nil {
		t.Fatal(err)
	}
	boosted, err := opted.Search(context.Background(), "second", nil, hits, contract.IntentBugFix)
	if err != nil {
		t.Fatal(err)
	}
	if len(plain.ConceptCandidates) != 0 || len(boosted.ConceptCandidates) != 1 || len(boosted.Citations) != 2 {
		t.Fatalf("optional candidates/citations: plain=%+v boosted=%+v", plain, boosted)
	}
	if plain.Citations[0].Citation != hits[0].Citation || boosted.Citations[0].Citation != hits[1].Citation ||
		boosted.Citations[1].Citation != hits[0].Citation {
		t.Fatalf("unexpected capped reorder: plain=%+v boosted=%+v", plain.Citations, boosted.Citations)
	}
	for _, invalid := range []float64{-0.01, 0.21, math.NaN(), math.Inf(1)} {
		if _, err := New(&ckgclient.Fake{}, WithOntologyResolver(f, invalid)); err == nil {
			t.Fatalf("accepted boost %v", invalid)
		}
	}
}

func keys(a *aggregator) map[string]bool {
	out := make(map[string]bool, len(a.byCitation))
	for k := range a.byCitation {
		out[k] = true
	}
	return out
}
