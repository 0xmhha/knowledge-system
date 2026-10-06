package backendmeasure

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
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

func TestNeighborMeasurementBindsSourceWithoutReleasingPrivatePath(t *testing.T) {
	backend := &ckgclient.Fake{}
	client := WrapCKG(backend)
	ctx, scope := (&Recorder{}).Begin(context.Background(), "source-bound", "neighbors")
	citations := []contract.Citation{
		{File: "PRIVATE_SOURCE/alpha.go", StartLine: 3, EndLine: 8, CommitHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		{File: "PRIVATE_SOURCE/alpha.go", StartLine: 4, EndLine: 8, CommitHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		{File: "PRIVATE_SOURCE/alpha.go", StartLine: 3, EndLine: 8, CommitHash: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
	}
	opts := ckgclient.NeighborsOpts{Hops: 2, MaxTotal: 17}
	for _, citation := range citations {
		if _, err := client.Neighbors(ctx, citation, opts); err != nil {
			t.Fatal(err)
		}
	}
	report := scope.Finish("returned")
	if len(report.Calls) != len(citations) || len(backend.Calls.Neighbors) != len(citations) {
		t.Fatal("lost neighbor attempts")
	}
	hashes := map[string]bool{}
	for i, call := range report.Calls {
		raw, err := json.Marshal(citations[i])
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(raw)
		if call.InputSHA256 != hex.EncodeToString(digest[:]) || call.InputBytes != len(raw) {
			t.Errorf("neighbor source not bound at attempt %d: %+v", i, call)
		}
		if hashes[call.InputSHA256] {
			t.Errorf("different range/commit collapsed to the same binding")
		}
		hashes[call.InputSHA256] = true
		if backend.Calls.Neighbors[i].Src != citations[i] || backend.Calls.Neighbors[i].Opts.Hops != opts.Hops || backend.Calls.Neighbors[i].Opts.MaxTotal != opts.MaxTotal {
			t.Fatal("source measurement changed backend arguments")
		}
	}
	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "PRIVATE_SOURCE") || strings.Contains(string(raw), citations[0].CommitHash) {
		t.Fatalf("neighbor argument was released instead of hashed: %s", raw)
	}
}

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

func TestNeighborSeedDiagnosticPreservesErrorOutcomeAndAllAttempts(t *testing.T) {
	cases := []struct {
		name, backend, method string
		err                   error
		outcome, kind         string
	}{
		{"unresolved", "ckg", "neighbors", fmt.Errorf("PRIVATE_PATH: %w", ckgclient.ErrSeedUnresolved), "backend_error", "seed_unresolved"},
		{"storage", "ckg", "neighbors", errors.New("PRIVATE_STORE_ERROR"), "backend_error", ""},
		{"cancelled-unresolved", "ckg", "neighbors", errors.Join(ckgclient.ErrSeedUnresolved, context.Canceled), "cancelled", ""},
		{"deadline-unresolved", "ckg", "neighbors", errors.Join(ckgclient.ErrSeedUnresolved, context.DeadlineExceeded), "deadline_exceeded", ""},
		{"other-method", "ckg", "bm25_search", ckgclient.ErrSeedUnresolved, "backend_error", ""},
		{"returned-empty", "ckg", "neighbors", nil, "returned", ""},
	}
	ctx, scope := (&Recorder{}).Begin(context.Background(), "new-structural-dev", "cks.context.get_for_task_v2")
	for _, c := range cases {
		finish := begin(ctx, c.backend, c.method, "PRIVATE_CITATION", nil)
		finish(0, c.err)
	}
	report := scope.Finish("returned")
	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	var wire struct {
		Calls []map[string]any `json:"calls"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatal(err)
	}
	if len(wire.Calls) != len(cases) || report.PendingCalls != 0 {
		t.Fatal("error attempts lost")
	}
	nonreturned := 0
	for i, c := range cases {
		call := wire.Calls[i]
		if call["outcome"] != c.outcome {
			t.Fatalf("%s: outcome=%v want=%s", c.name, call["outcome"], c.outcome)
		}
		if c.kind != "" && call["error_kind"] != c.kind {
			t.Fatalf("%s: error_kind=%v want=%s", c.name, call["error_kind"], c.kind)
		}
		if c.kind == "" {
			if _, exists := call["error_kind"]; exists {
				t.Fatalf("%s: inferred diagnostic on unrelated event", c.name)
			}
		}
		if call["outcome"] != "returned" {
			nonreturned++
		}
	}
	if nonreturned != 5 || strings.Contains(string(raw), "PRIVATE_") {
		t.Fatalf("denominators or privacy changed: %s", raw)
	}
}

type neighborDiagnosticClient struct {
	ckgclient.Client
	err error
}

func (g neighborDiagnosticClient) Neighbors(ctx context.Context, c contract.Citation, opts ckgclient.NeighborsOpts) ([]contract.Neighbor, error) {
	_, _ = g.Client.Neighbors(ctx, c, opts)
	return nil, g.err
}

func TestTypedNeighborFailureDoesNotChangeComposerSeedsOrAttempts(t *testing.T) {
	seed := stage2.ScoredCitation{Citation: contract.Citation{File: "PRIVATE_SEED.md", StartLine: 2, EndLine: 3}, Score: 7}
	var outputs []stage3.Stage3Output
	for _, typed := range []bool{false, true} {
		cause := errors.New("PRIVATE_SOURCE no node")
		if typed {
			cause = fmt.Errorf("PRIVATE_SOURCE: %w", ckgclient.ErrSeedUnresolved)
		}
		backend := &ckgclient.Fake{}
		ctx, scope := (&Recorder{}).Begin(context.Background(), "fresh-structural-diagnostic", "cks.context.get_for_task_v2")
		expander, err := stage3.New(WrapCKG(neighborDiagnosticClient{Client: backend, err: cause}))
		if err != nil {
			t.Fatal(err)
		}
		out, err := expander.Expand(ctx, []stage2.ScoredCitation{seed}, contract.IntentBugFix)
		if err != nil {
			t.Fatal(err)
		}
		outputs = append(outputs, out)
		summary := scope.Finish("returned")
		if len(summary.Calls) != 2 || len(backend.Calls.Neighbors) != 2 || len(out.FailedSeeds) != 1 || len(out.Seeds) != 1 || out.Seeds[0].Score != 7 || len(out.Neighbors) != 0 {
			t.Fatalf("optional failures altered seed/attempt denominators: %+v", out)
		}
		for _, call := range summary.Calls {
			if call.Outcome != "backend_error" || (call.ErrorKind == "seed_unresolved") != typed {
				t.Fatalf("changed failure outcome: %+v", call)
			}
		}
		raw, _ := json.Marshal(summary)
		if strings.Contains(string(raw), "PRIVATE_") {
			t.Fatalf("raw error/source released: %s", raw)
		}
	}
	if !reflect.DeepEqual(outputs[0], outputs[1]) {
		t.Fatalf("classification changed expansion: before=%+v after=%+v", outputs[0], outputs[1])
	}
}
