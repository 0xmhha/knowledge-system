package composer

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/system/debugtrace"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

func debugTraceFixture(t *testing.T) *fixture {
	t.Helper()
	return newFixture(t, func(f *fixture) {
		f.ckv.SearchHits = []contract.Hit{hit("login.go", 10, 30, 0.9, contract.HitSourceCKV)}
		f.ckg.BM25Hits = []contract.Hit{hit("login.go", 10, 30, 8.0, contract.HitSourceCKG)}
		f.ckg.SymbolCitations = []contract.Citation{cit("login.go", 10, 30)}
		f.ckg.NeighborEdges = []contract.Neighbor{
			{Source: cit("login.go", 10, 30), Target: cit("auth.go", 5, 25), Relation: contract.RelationCalls, Distance: 1},
		}
		f.fetcher.Bodies = map[string]string{
			cit("login.go", 10, 30).Key(): "func Login() { return validate() }",
			cit("auth.go", 5, 25).Key():   "func validate() bool { return true }",
		}
	})
}

// The trace must record every algorithm on both sides, so a reader can find
// the first step where an answer moved, and must not change the pack.
func TestCompose_DebugTraceRecordsEachAlgorithmWithoutChangingPack(t *testing.T) {
	t.Parallel()
	const prompt = "find the Login handler"
	plain, err := debugTraceFixture(t).composer.Compose(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Compose without trace: %v", err)
	}

	f := debugTraceFixture(t)
	path := filepath.Join(t.TempDir(), "trace.jsonl")
	f.composer.debugSink = debugtrace.NewFileSink(path)
	traced, err := f.composer.Compose(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Compose with trace: %v", err)
	}
	if !reflect.DeepEqual(plain.Citations, traced.Citations) || !reflect.DeepEqual(plain.GraphNeighbors, traced.GraphNeighbors) || !reflect.DeepEqual(plain.Bodies, traced.Bodies) {
		t.Fatalf("tracing changed the pack:\nplain  %+v\ntraced %+v", plain.Citations, traced.Citations)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 1 {
		t.Fatalf("trace lines = %d, want 1 per compose call", len(lines))
	}
	var rec debugtrace.Record
	if err := json.Unmarshal([]byte(lines[0]), &rec); err != nil {
		t.Fatal(err)
	}
	if rec.Prompt != prompt || rec.Error != "" {
		t.Errorf("record prompt/error = %q/%q", rec.Prompt, rec.Error)
	}
	got := map[string]bool{}
	for _, e := range rec.Events {
		got[e.Stage+"/"+e.Step+"/"+e.Phase] = true
	}
	for _, want := range []string{
		"intent/classify/after",
		"stage1/ckv_search/before", "stage1/ckv_search/after",
		"stage1/merge_hits/before", "stage1/merge_hits/after",
		"stage1/extract_keywords/after",
		"stage1/rerank_keywords/before", "stage1/rerank_keywords/after",
		"stage1/confidence_gate/note",
		"stage1/keyword_cap/before", "stage1/keyword_cap/after",
		"stage2/ckv_list/after",
		"stage2/keyword_search/before",
		"stage2/bm25/after", "stage2/find_symbol/after",
		"stage2/rank_citations/before", "stage2/rank_citations/after",
		"stage3/expand/before", "stage3/seed_neighbors/after", "stage3/expand/after",
		"stage4/allocate/before", "stage4/allocate/after",
	} {
		if !got[want] {
			t.Errorf("trace is missing %s", want)
		}
	}
}

func TestCompose_DebugTraceRecordsCallerIntentOverride(t *testing.T) {
	t.Parallel()
	f := debugTraceFixture(t)
	path := filepath.Join(t.TempDir(), "trace.jsonl")
	f.composer.debugSink = debugtrace.NewFileSink(path)
	if _, err := f.composer.ComposeWithIntent(context.Background(), "find the Login handler", contract.IntentBugFix); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var rec debugtrace.Record
	if err := json.Unmarshal(raw, &rec); err != nil {
		t.Fatal(err)
	}
	if rec.CallerIntent != string(contract.IntentBugFix) {
		t.Errorf("caller intent = %q", rec.CallerIntent)
	}
	var override, classified bool
	for _, e := range rec.Events {
		override = override || (e.Stage == "intent" && e.Step == "caller_override")
		classified = classified || (e.Stage == "intent" && e.Step == "classify")
	}
	if !override || classified {
		t.Errorf("override=%v classified=%v, want the override recorded and no classification", override, classified)
	}
}
