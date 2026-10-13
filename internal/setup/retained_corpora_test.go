package setup

import (
	"context"
	"path/filepath"
	"testing"
)

type corpusRecorder struct{ calls *[]Step }

func (r corpusRecorder) Run(_ context.Context, s Step, _ func(Event)) error {
	*r.calls = append(*r.calls, s)
	return nil
}

func TestRetainedCorporaRenderBeforePinningAndUseVectorRootOrder(t *testing.T) {
	o := Options{DomainKnowledge: "/pack/domain", DerivedDir: "/derived", SemanticCorpus: "/semantic", FlowCorpus: "/flows/corpus.jsonl", CodeRoot: "/source", Src: "/source", Out: filepath.Join(t.TempDir(), "dataset")}
	var calls []Step
	if err := PrepareRetainedCorpora(context.Background(), &o, corpusRecorder{&calls}, nil); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 || calls[0].ID != "domain-export" || !o.RetainedCorporaPrepared {
		t.Fatal("domain must be rendered before pinning")
	}
	for i, root := range []string{"/derived/domain-corpus", "/semantic", "/flows"} {
		if o.ExternalOrigins[i].ID != []string{"docs:corpus.0", "docs:corpus.1", "docs:corpus.2"}[i] || o.ExternalOrigins[i].Root != root {
			t.Fatalf("wrong corpus order: %+v", o.ExternalOrigins)
		}
	}
	plan, err := BuildPlan(o)
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range plan.Steps {
		if step.ID == "domain-export" {
			t.Fatal("pinned corpus is regenerated")
		}
	}
	if err := PrepareRetainedCorpora(context.Background(), &o, corpusRecorder{&calls}, nil); err != nil || len(calls) != 1 {
		t.Fatal("preparation was not idempotent")
	}
}
