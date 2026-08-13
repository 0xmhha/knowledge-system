package mcp

import (
	"context"
	"encoding/json"
	"testing"

	mcpgo "github.com/mark3labs/mcp-go/mcp"

	"github.com/0xmhha/knowledge-system/internal/system/ckvclient"
)

func TestHandleFindInvariants_HappyPath(t *testing.T) {
	t.Parallel()
	f := newFixture(t, func(f *fixture) {
		f.ckv.InvariantHits = []ckvclient.InvariantHit{
			{ChunkID: "c1", File: "core/state.go", Tier: 1, Text: "no drop of valid next-seq", Category: "consensus"},
		}
	})
	res, err := handleFindInvariants(context.Background(), f.deps,
		callToolReq(map[string]any{"file": "core/state.go", "tier_min": 1}))
	if err != nil {
		t.Fatalf("handleFindInvariants: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected IsError: %s", resultText(res))
	}
	var out findInvariantsResponse
	if err := decodeStructured(res, &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out.Invariants) != 1 || out.Invariants[0].ChunkID != "c1" {
		t.Errorf("unexpected invariants: %+v", out.Invariants)
	}
}

func TestHandleGetConventions_HappyPath(t *testing.T) {
	t.Parallel()
	f := newFixture(t, func(f *fixture) {
		f.ckv.Conventions = []ckvclient.ConventionHit{
			{ChunkID: "c2", Package: "core/vm", Summary: "early-return idiom"},
		}
	})
	res, err := handleGetConventions(context.Background(), f.deps,
		callToolReq(map[string]any{"package_prefix": "core/"}))
	if err != nil {
		t.Fatalf("handleGetConventions: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected IsError: %s", resultText(res))
	}
	var out getConventionsResponse
	if err := decodeStructured(res, &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out.Conventions) != 1 || out.Conventions[0].Package != "core/vm" {
		t.Errorf("unexpected conventions: %+v", out.Conventions)
	}
}

// TestListReturningToolsEmitObjects pins the wire contract these two tools
// broke: both return a list, and both used to hand the bare slice to mcp-go.
// structuredContent then serialised as a JSON array, which a spec-conforming
// client rejects — the tools errored on every call while their unit tests,
// unmarshalling straight into a slice, stayed green. Assert the serialised
// shape, not just the decoded fields.
func TestListReturningToolsEmitObjects(t *testing.T) {
	t.Parallel()
	f := newFixture(t, func(f *fixture) {
		f.ckv.InvariantHits = []ckvclient.InvariantHit{{ChunkID: "c1"}}
		f.ckv.Conventions = []ckvclient.ConventionHit{{ChunkID: "c2"}}
	})
	cases := []struct {
		name    string
		call    func() (*mcpgo.CallToolResult, error)
		wantKey string
	}{
		{
			name: "find_invariants",
			call: func() (*mcpgo.CallToolResult, error) {
				return handleFindInvariants(context.Background(), f.deps, callToolReq(nil))
			},
			wantKey: "invariants",
		},
		{
			name: "get_conventions",
			call: func() (*mcpgo.CallToolResult, error) {
				return handleGetConventions(context.Background(), f.deps, callToolReq(nil))
			},
			wantKey: "conventions",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := tc.call()
			if err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			raw, err := json.Marshal(res.StructuredContent)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var obj map[string]json.RawMessage
			if err := json.Unmarshal(raw, &obj); err != nil {
				t.Fatalf("structuredContent is not a JSON object (%s): %v", raw, err)
			}
			if _, ok := obj[tc.wantKey]; !ok {
				t.Errorf("missing %q key in %s", tc.wantKey, raw)
			}
		})
	}
}
