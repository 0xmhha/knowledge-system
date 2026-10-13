package stage2

import (
	"context"
	"github.com/0xmhha/knowledge-system/internal/system/ckgclient"
	"github.com/0xmhha/knowledge-system/internal/system/semantic"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
	"testing"
	"time"
)

func TestOntologyExplanationSeparatesNoEffectFromUnavailable(t *testing.T) {
	hits := []contract.Hit{{Citation: contract.Citation{File: "main.go", StartLine: 1, EndLine: 1}, CanonicalID: "pkg.Alpha"}}
	for _, unavailable := range []bool{false, true} {
		p := providerFunc(func(context.Context) (OntologyResolver, error) {
			if unavailable {
				return nil, ErrOntologyUnavailable
			}
			return fakeOntology{candidates: []semantic.ConceptCandidate{{ConceptID: "alpha", Status: semantic.StatusProposed}}}, nil
		})
		search, err := New(&ckgclient.Fake{}, WithOntologyProvider(p, .2, time.Second))
		if err != nil {
			t.Fatal(err)
		}
		out, err := search.Search(WithOntologyExplanation(context.Background()), "Alpha", nil, hits, contract.IntentBugFix)
		if err != nil {
			t.Fatal(err)
		}
		want := "no_effect"
		if unavailable {
			want = "unavailable"
		}
		if out.Ontology.Explanation == nil || out.Ontology.Explanation.State != want || out.Ontology.Explanation.TotalChanged != 0 || len(out.Ontology.Explanation.Changes) != 0 {
			t.Fatalf("false effect: %+v", out.Ontology)
		}
	}
}

func TestOntologyExplanationPreservesNoCandidates(t *testing.T) {
	search, err := New(&ckgclient.Fake{}, WithOntologyProvider(providerFunc(func(context.Context) (OntologyResolver, error) { return fakeOntology{}, nil }), .2, time.Second))
	if err != nil {
		t.Fatal(err)
	}
	out, err := search.Search(WithOntologyExplanation(context.Background()), "feature", nil, nil, contract.IntentBugFix)
	if err != nil {
		t.Fatal(err)
	}
	if out.Ontology == nil || out.Ontology.Explanation == nil || out.Ontology.Explanation.State != "no_candidates" || out.Ontology.Explanation.QualityEvaluated {
		t.Fatalf("empty-candidate explanation missing: %+v", out.Ontology)
	}
}
