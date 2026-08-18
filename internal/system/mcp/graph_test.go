package mcp

import (
	"context"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/system/ckgclient"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

// --- find_symbol ---

func TestHandleFindSymbol_MissingName_IsError(t *testing.T) {
	t.Parallel()
	f := newFixture(t, nil)
	res, err := handleFindSymbol(context.Background(), f.deps, callToolReq(nil))
	if err != nil {
		t.Fatalf("handleFindSymbol: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatalf("expected IsError for missing name; got %+v", res)
	}
}

func TestHandleFindSymbol_HappyPath(t *testing.T) {
	t.Parallel()
	wantCit := cit("login.go", 10, 30)
	f := newFixture(t, func(f *fixture) {
		f.ckg.SymbolCitations = []contract.Citation{wantCit}
	})

	req := callToolReq(map[string]any{
		"name":  "Login",
		"kinds": "function,method",
	})
	res, err := handleFindSymbol(context.Background(), f.deps, req)
	if err != nil {
		t.Fatalf("handleFindSymbol: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected IsError: %s", resultText(res))
	}

	var out findSymbolResponse
	if err := decodeStructured(res, &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Symbol != "Login" {
		t.Errorf("Symbol = %q, want Login", out.Symbol)
	}
	if len(out.Citations) != 1 || out.Citations[0].Key() != wantCit.Key() {
		t.Errorf("Citations = %+v", out.Citations)
	}
	// Verify CKG fake recorded the kinds filter.
	if len(f.ckg.Calls.FindSymbol) != 1 {
		t.Fatalf("expected 1 FindSymbol call, got %d", len(f.ckg.Calls.FindSymbol))
	}
	gotKinds := f.ckg.Calls.FindSymbol[0].Opts.Kinds
	if len(gotKinds) != 2 || gotKinds[0] != "function" || gotKinds[1] != "method" {
		t.Errorf("Kinds = %v", gotKinds)
	}
}

func TestHandleFindSymbol_DummyEmitsInstructions(t *testing.T) {
	t.Parallel()
	// Wire a dummy ckg directly into Deps so handleFindSymbol invokes it.
	f := newFixture(t, nil)
	dummy := ckgclient.NewDummy()
	f.deps.CKG = dummy

	req := callToolReq(map[string]any{"name": "Finalize"})
	res, err := handleFindSymbol(context.Background(), f.deps, req)
	if err != nil {
		t.Fatalf("handleFindSymbol: %v", err)
	}
	var out findSymbolResponse
	if err := decodeStructured(res, &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out.Instructions) != 1 {
		t.Fatalf("Instructions: got %d, want 1", len(out.Instructions))
	}
	if out.Instructions[0].Operation != "FindSymbol" || out.Instructions[0].Backend != "graph" {
		t.Errorf("instruction = %+v", out.Instructions[0])
	}
}

// --- find_callers / find_callees ---

func TestHandleFindCallers_HappyPath(t *testing.T) {
	t.Parallel()
	seed := cit("consensus/wbft/finalize.go", 100, 150)
	f := newFixture(t, func(f *fixture) {
		f.ckg.SymbolCitations = []contract.Citation{seed}
		f.ckg.NeighborEdges = []contract.Neighbor{{
			Source:   cit("eth/handler.go", 50, 60),
			Target:   seed,
			Relation: contract.RelationCalledBy,
			Distance: 1,
		}}
	})
	req := callToolReq(map[string]any{
		"symbol": "consensus.wbft.Finalize",
		"depth":  float64(2),
	})
	res, err := handleFindRelatives(context.Background(), f.deps, req, ToolNameFindCallers, "callers", []contract.Relation{contract.RelationCalledBy})
	if err != nil {
		t.Fatalf("handleFindRelatives: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected IsError: %s", resultText(res))
	}
	var out graphNeighborsResponse
	if err := decodeStructured(res, &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Direction != "callers" {
		t.Errorf("Direction = %q", out.Direction)
	}
	if len(out.Neighbors) != 1 || out.Neighbors[0].Relation != contract.RelationCalledBy {
		t.Errorf("Neighbors = %+v", out.Neighbors)
	}
	// Verify the depth=2 propagated through to Neighbors.
	if len(f.ckg.Calls.Neighbors) != 1 || f.ckg.Calls.Neighbors[0].Opts.Hops != 2 {
		t.Errorf("Neighbors call Hops = %v", f.ckg.Calls.Neighbors)
	}
}

// TestHandleFindRelatives_UnresolvedSeedIsRefused replaces a test that asserted
// the opposite: that an unknown symbol comes back as an ordinary result with an
// empty neighbour list. That is the behaviour to prevent, not to pin — "no such
// symbol" and "nothing calls this symbol" are different answers, and the caller
// cannot tell them apart once both arrive as an empty list. The old assertion
// is why the gap survived the fixes around it.
func TestHandleFindRelatives_UnresolvedSeedIsRefused(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		cits    []contract.Citation
		symbol  string
		wantMsg string
	}{
		{
			name: "unknown symbol", cits: nil, symbol: "missing.Symbol",
			wantMsg: "matches no indexed symbol",
		},
		{
			name: "ambiguous symbol",
			cits: []contract.Citation{
				cit("consensus/wbft/backend/engine.go", 174, 176),
				cit("crypto/bn256/cloudflare/bn256.go", 383, 387),
			},
			symbol: "Finalize", wantMsg: "is ambiguous across 2 definitions",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, tool := range []struct {
				name string
				rel  contract.Relation
				dir  string
			}{
				{ToolNameFindCallers, contract.RelationCalledBy, "callers"},
				{ToolNameFindCallees, contract.RelationCalls, "callees"},
			} {
				f := newFixture(t, func(f *fixture) { f.ckg.SymbolCitations = tc.cits })
				req := callToolReq(map[string]any{"symbol": tc.symbol})
				res, err := handleFindRelatives(context.Background(), f.deps, req, tool.name, tool.dir,
					[]contract.Relation{tool.rel})
				if err != nil {
					t.Fatalf("%s: handleFindRelatives: %v", tool.name, err)
				}
				if !res.IsError {
					t.Fatalf("%s: want a refusal, got a result: %s", tool.name, resultText(res))
				}
				if got := resultText(res); !strings.Contains(got, tc.wantMsg) {
					t.Errorf("%s: message %q does not say %q", tool.name, got, tc.wantMsg)
				}
				// The traversal must not have run: answering about whichever
				// definition came back first is the failure being prevented.
				if len(f.ckg.Calls.Neighbors) != 0 {
					t.Errorf("%s: Neighbors was called with an unresolved seed: %+v",
						tool.name, f.ckg.Calls.Neighbors)
				}
			}
		})
	}
}

// --- get_subgraph ---

func TestHandleGetSubgraph_HappyPath(t *testing.T) {
	t.Parallel()
	f := newFixture(t, func(f *fixture) {
		f.ckg.SubgraphCitations = []contract.Citation{cit("a.go", 1, 10), cit("b.go", 1, 10)}
		f.ckg.SubgraphNeighbors = []contract.Neighbor{{
			Source: cit("a.go", 1, 10), Target: cit("b.go", 1, 10),
			Relation: contract.RelationImports, Distance: 1,
		}}
	})

	req := callToolReq(map[string]any{
		"symbol": "pkg.Foo",
		"depth":  float64(2),
	})
	res, err := handleGetSubgraph(context.Background(), f.deps, req)
	if err != nil {
		t.Fatalf("handleGetSubgraph: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected IsError: %s", resultText(res))
	}
	var out subgraphResponse
	if err := decodeStructured(res, &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Seed != "pkg.Foo" {
		t.Errorf("Seed = %q", out.Seed)
	}
	if len(out.Nodes) != 2 {
		t.Errorf("Nodes = %d, want 2", len(out.Nodes))
	}
	if len(out.Edges) != 1 || out.Edges[0].Relation != contract.RelationImports {
		t.Errorf("Edges = %+v", out.Edges)
	}
}

func TestHandleGetSubgraph_MissingSymbol_IsError(t *testing.T) {
	t.Parallel()
	f := newFixture(t, nil)
	res, err := handleGetSubgraph(context.Background(), f.deps, callToolReq(nil))
	if err != nil {
		t.Fatalf("handleGetSubgraph: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatalf("expected IsError for missing symbol; got %+v", res)
	}
}
