package stage2

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/0xmhha/knowledge-system/internal/system/ckgclient"
	"github.com/0xmhha/knowledge-system/internal/system/ckvclient"
	"github.com/0xmhha/knowledge-system/internal/system/semantic"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

type textOntology struct {
	fakeOntology
	text          semantic.ConceptText
	relationCalls *int
}

func (f textOntology) VerifiedConceptText(id string) (semantic.ConceptText, bool) {
	return f.text, id == f.text.ConceptID
}
func (f textOntology) VerifiedImplementations(id string) []semantic.CodeImplementation {
	(*f.relationCalls)++
	return f.fakeOntology.VerifiedImplementations(id)
}

func TestTextArmsUseIndependentSearchPreserveCandidatesAndCapCombinedBoost(t *testing.T) {
	commit := strings.Repeat("a", 40)
	snapshot := semantic.Snapshot{ProjectID: "project", DatasetID: "dataset", Commit: commit}
	hits := []contract.Hit{
		{Citation: contract.Citation{File: "a.go", StartLine: 1, EndLine: 2, CommitHash: commit}},
		{Citation: contract.Citation{File: "b.go", StartLine: 1, EndLine: 2, CommitHash: commit}, CanonicalID: "pkg.Beta"},
		{Citation: contract.Citation{File: "outside.go", StartLine: 1, EndLine: 2, CommitHash: commit}},
	}
	cfg := DefaultConfig()
	cfg.MaxCitations = 2
	plain, _ := New(&ckgclient.Fake{}, WithConfig(cfg))
	baseline, _ := plain.Search(context.Background(), "Beta", nil, hits, contract.IntentBugFix)
	for _, combined := range []bool{false, true} {
		calls := 0
		resolver := textOntology{fakeOntology: fakeOntology{snapshot: snapshot,
			candidates:      []semantic.ConceptCandidate{{ConceptID: "beta", Status: semantic.StatusVerified}},
			implementations: map[string][]semantic.CodeImplementation{"beta": {{AssertionID: "r1", CanonicalID: "pkg.Beta", Evidence: semantic.EvidenceSpan{Snapshot: snapshot, Path: "b.go", StartLine: 1, EndLine: 2}}}}},
			text: semantic.ConceptText{ConceptID: "beta", Text: "Reviewed Beta definition and synonyms", ReviewedBy: "reviewer", Evidence: semantic.EvidenceSpan{ID: "definition", Snapshot: snapshot, Kind: semantic.SourceDocument, Path: "ontology.yaml", StartLine: 2, EndLine: 9, ContentSHA256: strings.Repeat("b", 64)}}, relationCalls: &calls}
		// An outside hit remains outside the original capped set, even ranked first.
		ckv := &ckvclient.Fake{SearchHits: []contract.Hit{hits[2], hits[1], hits[0]}}
		provider := providerFunc(func(context.Context) (OntologyResolver, error) { return resolver, nil })
		s, err := New(&ckgclient.Fake{}, WithConfig(cfg), WithOntologyProvider(provider, .2, time.Second), WithOntologyTextSearch(ckv, 20, combined))
		if err != nil {
			t.Fatal(err)
		}
		out, err := s.Search(context.Background(), "Beta", nil, hits, contract.IntentBugFix)
		if err != nil || out.Ontology.State != "active" || out.Ontology.TextSearchCalls != 1 || len(out.Citations) != 2 || out.Citations[0].Citation.File != "b.go" {
			t.Fatalf("text rerank failed: %+v %v", out, err)
		}
		if ckv.Calls.SemanticSearch[0].Query != resolver.text.Text || ckv.Calls.SemanticSearch[0].Opts.K != 20 || !ckv.Calls.SemanticSearch[0].Opts.BM25Rerank {
			t.Fatal("wrong text query or recall settings")
		}
		if (!combined && (calls != 0 || out.Ontology.AppliedRelations != 0)) || (combined && out.Ontology.AppliedRelations != 1) {
			t.Fatalf("text/relations signals mixed: calls=%d %+v", calls, out.Ontology)
		}
		if out.Ontology.TextSources[0].EvidenceID != "definition" || len(out.Ontology.TextSources[0].QuerySHA256) != 64 {
			t.Fatal("text provenance missing")
		}
		for _, c := range out.Citations {
			found := false
			for _, b := range baseline.Citations {
				if b.Citation.Key() == c.Citation.Key() {
					found = true
					if c.Score > b.Score*1.2+1e-12 {
						t.Fatal("combined boost exceeded cap")
					}
				}
			}
			if !found {
				t.Fatal("text expanded candidate set")
			}
		}
	}
}

func TestTextFailureDiscardsCombinedChangesAndProposedDoesNotSearch(t *testing.T) {
	commit := strings.Repeat("a", 40)
	hit := contract.Hit{Citation: contract.Citation{File: "a.go", StartLine: 1, EndLine: 2, CommitHash: commit}, CanonicalID: "pkg.Alpha"}
	plain, _ := New(&ckgclient.Fake{})
	base, _ := plain.Search(context.Background(), "Alpha", nil, []contract.Hit{hit}, contract.IntentBugFix)
	for _, tc := range []struct {
		name   string
		status semantic.Status
		text   string
		err    error
		state  string
		calls  int
	}{
		{"backend error", semantic.StatusVerified, "definition", errors.New("model identity changed"), "unavailable", 1},
		{"oversize", semantic.StatusVerified, strings.Repeat("x", 6145), nil, "budget_exceeded", 0},
		{"proposed", semantic.StatusProposed, "definition", nil, "active", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			snapshot := semantic.Snapshot{Commit: commit}
			relations := 0
			f := textOntology{fakeOntology: fakeOntology{snapshot: snapshot, candidates: []semantic.ConceptCandidate{{ConceptID: "alpha", Status: tc.status}}, implementations: map[string][]semantic.CodeImplementation{"alpha": {{AssertionID: "r1", CanonicalID: "pkg.Alpha", Evidence: semantic.EvidenceSpan{Snapshot: snapshot, Path: "a.go", StartLine: 1, EndLine: 2}}}}}, text: semantic.ConceptText{ConceptID: "alpha", Text: tc.text, ReviewedBy: "reviewer", Evidence: semantic.EvidenceSpan{Snapshot: snapshot, Kind: semantic.SourceDocument}}, relationCalls: &relations}
			ckv := &ckvclient.Fake{SearchErr: tc.err}
			s, err := New(&ckgclient.Fake{}, WithOntologyProvider(providerFunc(func(context.Context) (OntologyResolver, error) { return f, nil }), .2, time.Second), WithOntologyTextSearch(ckv, 20, true))
			if err != nil {
				t.Fatal(err)
			}
			out, err := s.Search(context.Background(), "Alpha", nil, []contract.Hit{hit}, contract.IntentBugFix)
			if err != nil || !reflect.DeepEqual(out.Citations, base.Citations) || out.Ontology.State != tc.state || out.Ontology.TextSearchCalls != tc.calls || out.Ontology.AppliedRelations != 0 || out.Ontology.BoostedCitations != 0 {
				t.Fatalf("unsafe fallback %+v %v", out, err)
			}
		})
	}
	if _, err := New(&ckgclient.Fake{}, WithOntologyTextSearch(nil, 20, false)); err == nil {
		t.Fatal("unwired text arm accepted")
	}
}

type deadlineTextClient struct{ ckvclient.Fake }

func (f *deadlineTextClient) SemanticSearch(ctx context.Context, _ string, _ ckvclient.SearchOpts) ([]contract.Hit, error) {
	<-ctx.Done()
	// A backend returning success after its deadline must not affect ranking.
	return f.SearchHits, nil
}
func TestTextDeadlineDiscardsLateSuccessfulResults(t *testing.T) {
	commit := strings.Repeat("a", 40)
	snapshot := semantic.Snapshot{Commit: commit}
	relations := 0
	hit := contract.Hit{Citation: contract.Citation{File: "main.go", StartLine: 1, EndLine: 2, CommitHash: commit}}
	f := textOntology{fakeOntology: fakeOntology{snapshot: snapshot, candidates: []semantic.ConceptCandidate{{ConceptID: "alpha", Status: semantic.StatusVerified}}}, text: semantic.ConceptText{ConceptID: "alpha", Text: "verified definition", ReviewedBy: "reviewer", Evidence: semantic.EvidenceSpan{Snapshot: snapshot, Kind: semantic.SourceDocument}}, relationCalls: &relations}
	client := &deadlineTextClient{Fake: ckvclient.Fake{SearchHits: []contract.Hit{hit}}}
	plain, _ := New(&ckgclient.Fake{})
	base, _ := plain.Search(context.Background(), "alpha", nil, []contract.Hit{hit}, contract.IntentBugFix)
	s, _ := New(&ckgclient.Fake{}, WithOntologyProvider(providerFunc(func(context.Context) (OntologyResolver, error) { return f, nil }), .2, 5*time.Millisecond), WithOntologyTextSearch(client, 20, true))
	out, err := s.Search(context.Background(), "alpha", nil, []contract.Hit{hit}, contract.IntentBugFix)
	if err != nil || out.Ontology.State != "budget_exceeded" || out.Ontology.TextSearchCalls != 1 || out.Ontology.BoostedCitations != 0 || !reflect.DeepEqual(out.Citations, base.Citations) {
		t.Fatalf("late successful results escaped fallback: %+v %v", out, err)
	}
}
