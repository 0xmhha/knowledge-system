package mcp

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/graph/store"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

func TestHandleReviewFunctionSeparatesTheViews(t *testing.T) {
	f := newFixture(t, func(f *fixture) {
		f.ckg.ReviewResult = store.Review{
			Seed:             store.ReviewSymbol{QualifiedName: "p.Lookup", Type: "Function", FilePath: "p/p.go", StartLine: 10},
			Signature:        "func Lookup(s Store, k string) string",
			Callers:          []store.ReviewSymbol{{QualifiedName: "p.Handler"}},
			CallerPaths:      [][]string{{"p.Main", "p.Handler", "p.Lookup"}},
			Commits:          []store.ReviewCommit{{SHA: "abc", Subject: "reject the dash key"}},
			CommitsTotal:     3,
			TestCallers:      []store.ReviewSymbol{{QualifiedName: "p.TestLookup"}},
			TestCallersTotal: 1,
		}
		f.ckg.PRRefs = []contract.PRRef{{Number: 7}}
	})
	res, err := handleReviewFunction(context.Background(), f.deps, callToolReq(map[string]any{"symbol": "Lookup", "caller_depth": float64(99), "max_items": float64(-3)}))
	if err != nil || res.IsError {
		t.Fatalf("review: %v %s", err, resultText(res))
	}
	var out reviewFunctionResponse
	if err := decodeStructured(res, &out); err != nil {
		t.Fatal(err)
	}
	if out.Code.Signature == "" || len(out.Code.Callers) != 1 || len(out.Code.CallerPaths) != 1 {
		t.Errorf("code view = %+v", out.Code)
	}
	if out.History.CommitsTotal != 3 || len(out.History.PRs) != 1 || len(out.Tests.Callers) != 1 {
		t.Errorf("history %+v tests %+v", out.History, out.Tests)
	}
	joined := strings.Join(out.Available, " | ")
	for _, want := range []string{"3 commits", ToolNameChangeHistory, "1 test functions", ToolNameFindCallers} {
		if !strings.Contains(joined, want) {
			t.Errorf("available %q lacks %q", joined, want)
		}
	}
	if len(out.Limits) == 0 {
		t.Error("limits missing")
	}
	call := f.ckg.Calls.ReviewFunction[0]
	if call.Qname != "Lookup" || call.Opts.CallerDepth != maxTraversalDepth || call.Opts.MaxItems != 1 {
		t.Errorf("client call = %+v", call)
	}
}

func TestHandleReviewFunctionErrors(t *testing.T) {
	f := newFixture(t, func(f *fixture) { f.ckg.ReviewErr = fmt.Errorf("ckgclient: review p.X: %w", store.ErrNoReviewIndex) })
	res, _ := handleReviewFunction(context.Background(), f.deps, callToolReq(map[string]any{"symbol": "p.X"}))
	if !res.IsError || !strings.Contains(resultText(res), "requires_reindex") {
		t.Fatalf("old graph: %s", resultText(res))
	}
	res, _ = handleReviewFunction(context.Background(), f.deps, callToolReq(map[string]any{}))
	if !res.IsError || !strings.Contains(resultText(res), "symbol") {
		t.Fatalf("missing symbol: %s", resultText(res))
	}
}
