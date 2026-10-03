package backendmeasure

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/system/ckgclient"
	"github.com/0xmhha/knowledge-system/internal/system/ckvclient"
	"github.com/0xmhha/knowledge-system/internal/system/composer/stage1"
	"github.com/0xmhha/knowledge-system/internal/system/composer/stage2"
	"github.com/0xmhha/knowledge-system/internal/system/composer/stage3"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

type failedKnowledge struct{ ckvclient.Client }

func (v failedKnowledge) SemanticSearch(ctx context.Context, query string, opts ckvclient.SearchOpts) ([]contract.Hit, error) {
	if len(opts.Filter.ChunkKinds) > 0 {
		return nil, errors.New("PRIVATE_BACKEND_ERROR")
	}
	return v.Client.SemanticSearch(ctx, query, opts)
}

func TestCompositionCountsNonfatalKnowledgeFailureAndStage3(t *testing.T) {
	citation := contract.Citation{File: "alpha.go", StartLine: 1, EndLine: 2}
	v := &ckvclient.Fake{SearchHits: []contract.Hit{{Citation: citation, Score: 1, Source: contract.HitSourceCKV}}}
	g := &ckgclient.Fake{BM25Hits: []contract.Hit{{Citation: citation, Score: 5, Source: contract.HitSourceCKG}}}
	ctx, scope := (&Recorder{}).Begin(context.Background(), "fixture", "composition")
	cfg := stage1.DefaultConfig()
	cfg.MaxRounds = 1
	extractor, err := stage1.New(WrapCKV(failedKnowledge{v}), WrapCKG(g), stage1.WithConfig(cfg))
	if err != nil {
		t.Fatal(err)
	}
	out, err := extractor.Extract(ctx, "PRIVATE_PROMPT Alpha", contract.IntentBugFix)
	if err != nil || out.Rounds != 1 || len(out.Hits) != 1 {
		t.Fatalf("nonfatal knowledge failure changed recall: %+v, %v", out, err)
	}
	expander, err := stage3.New(WrapCKG(g))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := expander.Expand(ctx, []stage2.ScoredCitation{{Citation: citation, Score: 5}}, contract.IntentBugFix); err != nil {
		t.Fatal(err)
	}
	summary := scope.Finish("returned")
	var searches []Call
	neighbors := 0
	for _, call := range summary.Calls {
		if call.Method == "semantic_search" {
			searches = append(searches, call)
		}
		if call.Method == "neighbors" {
			neighbors++
		}
	}
	if len(searches) != 2 || searches[1].Outcome != "backend_error" || neighbors != len(g.Calls.Neighbors) || neighbors == 0 {
		t.Fatalf("omitted logical calls: %+v", summary)
	}
	var opts ckvclient.SearchOpts
	if err := json.Unmarshal(searches[1].Options, &opts); err != nil {
		t.Fatal(err)
	}
	if opts.K != 6 || len(opts.Filter.ChunkKinds) != 2 {
		t.Fatalf("lost actual knowledge K/filter: %+v", opts)
	}
	raw, _ := json.Marshal(summary)
	if strings.Contains(string(raw), "PRIVATE_PROMPT") || strings.Contains(string(raw), "PRIVATE_BACKEND_ERROR") {
		t.Fatalf("raw text leaked: %s", raw)
	}
}

func TestScopesRemainIsolatedUnderConcurrentCancellation(t *testing.T) {
	var wg sync.WaitGroup
	summaries := make(chan Summary, 64)
	r := &Recorder{Emit: func(_ context.Context, s Summary) { summaries <- s }}
	for i := 0; i < cap(summaries); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, s := r.Begin(context.Background(), "", "cancelled-composition")
			finish := begin(ctx, "ckv", "semantic_search", "input", ckvclient.SearchOpts{K: 7})
			finish(0, context.Canceled)
			s.Finish("tool_error")
		}()
	}
	wg.Wait()
	close(summaries)
	ids := map[string]bool{}
	for s := range summaries {
		if ids[s.MeasurementID] || len(s.Calls) != 1 || s.Calls[0].Outcome != "cancelled" || s.Calls[0].Ordinal != 1 {
			t.Fatalf("cross-scope corruption: %+v", s)
		}
		ids[s.MeasurementID] = true
	}
	if len(ids) != 64 {
		t.Fatal("lost scopes")
	}
}

func TestInflightCallIsPreservedAndLateCompletionCannotRewriteEvidence(t *testing.T) {
	ctx, s := (&Recorder{}).Begin(context.Background(), "fixture", "composition")
	finish := begin(ctx, "ckv", "semantic_search", "input", nil)
	before := s.Finish("tool_error")
	if before.PendingCalls != 1 || before.Calls[0].Outcome != "in_flight" {
		t.Fatalf("unfinished call hidden: %+v", before)
	}
	finish(10, nil)
	after := s.Finish("returned")
	a, _ := json.Marshal(before)
	b, _ := json.Marshal(after)
	if string(a) != string(b) {
		t.Fatal("late completion changed sealed summary")
	}
}

func TestMeasurementPreservesOptionalFlowSurface(t *testing.T) {
	base := &ckvclient.Fake{FlowVal: ckvclient.Flow{FlowID: "fixture-flow"}}
	flow, ok := WrapCKV(base).(ckvclient.FlowClient)
	if !ok {
		t.Fatal("measurement erased direct flow tools")
	}
	result, err := flow.GetFlow(context.Background(), ckvclient.FlowQuery{FlowID: "fixture-flow"})
	if err != nil || result.FlowID != "fixture-flow" {
		t.Fatalf("flow forwarding changed: %+v, %v", result, err)
	}
}
