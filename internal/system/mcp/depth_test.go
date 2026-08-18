package mcp

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

// traversalDepthDoc is the standing reference for the default. It is read
// rather than restated so the constant cannot quietly drift away from the
// document that justifies it — which is how the default came to ship as 1
// while the doc, still in the tree and still authoritative, said 2.
const traversalDepthDoc = "../../../docs/graph/TRAVERSAL-DEPTH.md"

// TestTraversalDepthDefaultMatchesItsDocumentedValue reads the depth the
// design doc declares and requires the code to use it.
func TestTraversalDepthDefaultMatchesItsDocumentedValue(t *testing.T) {
	t.Parallel()

	body, err := os.ReadFile(filepath.Clean(traversalDepthDoc))
	if err != nil {
		t.Fatalf("read %s: %v — the default has a written justification; "+
			"if the doc moved, point this test at it rather than dropping the check",
			traversalDepthDoc, err)
	}
	// "**Default `depth=2`.**" is the decision line.
	m := regexp.MustCompile(`\*\*Default ` + "`" + `depth=(\d+)` + "`").FindSubmatch(body)
	if m == nil {
		t.Fatalf("%s no longer states a decision in the form **Default `depth=N`**", traversalDepthDoc)
	}
	want, err := strconv.Atoi(string(m[1]))
	if err != nil {
		t.Fatalf("parse documented depth: %v", err)
	}
	if defaultTraversalDepth != want {
		t.Errorf("defaultTraversalDepth = %d, but %s decides %d",
			defaultTraversalDepth, traversalDepthDoc, want)
	}
}

// TestTraversalDepthDefaultReachesTheBackend pins that the constant is what
// the handlers actually pass. A constant nothing reads would satisfy the test
// above while every call still walked one hop.
func TestTraversalDepthDefaultReachesTheBackend(t *testing.T) {
	t.Parallel()

	t.Run("find_callers and find_callees", func(t *testing.T) {
		t.Parallel()
		for _, tc := range []struct {
			tool string
			rel  contract.Relation
			dir  string
		}{
			{ToolNameFindCallers, contract.RelationCalledBy, "callers"},
			{ToolNameFindCallees, contract.RelationCalls, "callees"},
		} {
			f := newFixture(t, func(f *fixture) {
				f.ckg.SymbolCitations = []contract.Citation{cit("pkg/a.go", 1, 9)}
			})
			req := callToolReq(map[string]any{"symbol": "pkg.A"}) // no depth given
			if _, err := handleFindRelatives(context.Background(), f.deps, req, tc.tool, tc.dir,
				[]contract.Relation{tc.rel}); err != nil {
				t.Fatalf("%s: %v", tc.tool, err)
			}
			if len(f.ckg.Calls.Neighbors) != 1 {
				t.Fatalf("%s: Neighbors calls = %d, want 1", tc.tool, len(f.ckg.Calls.Neighbors))
			}
			if got := f.ckg.Calls.Neighbors[0].Opts.Hops; got != defaultTraversalDepth {
				t.Errorf("%s: Hops = %d, want the %d default", tc.tool, got, defaultTraversalDepth)
			}
		}
	})

	t.Run("get_subgraph", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t, nil)
		req := callToolReq(map[string]any{"symbol": "pkg.A"}) // no depth given
		if _, err := handleGetSubgraph(context.Background(), f.deps, req); err != nil {
			t.Fatalf("handleGetSubgraph: %v", err)
		}
		if len(f.ckg.Calls.GetSubgraph) != 1 {
			t.Fatalf("GetSubgraph calls = %d, want 1", len(f.ckg.Calls.GetSubgraph))
		}
		if got := f.ckg.Calls.GetSubgraph[0].Opts.Depth; got != defaultTraversalDepth {
			t.Errorf("Depth = %d, want the %d default", got, defaultTraversalDepth)
		}
	})
}

// TestTraversalDepthDescriptionsAgreeWithTheDefault keeps the number an agent
// reads in the tool description from disagreeing with the number the tool
// uses. The descriptions said "default 1" for as long as the code did.
func TestTraversalDepthDescriptionsAgreeWithTheDefault(t *testing.T) {
	t.Parallel()
	want := "default " + strconv.Itoa(defaultTraversalDepth)
	srv := mcpserver.NewMCPServer("cks-test", "0.0.1")
	if err := Register(srv, newFixture(t, nil).deps); err != nil {
		t.Fatalf("Register: %v", err)
	}
	tools := srv.ListTools()
	for _, name := range []string{ToolNameFindCallers, ToolNameFindCallees, ToolNameGetSubgraph} {
		st, ok := tools[name]
		if !ok {
			t.Fatalf("tool %q is not registered", name)
		}
		prop, ok := st.Tool.InputSchema.Properties["depth"]
		if !ok {
			t.Fatalf("%s declares no depth argument", name)
		}
		desc, _ := prop.(map[string]any)["description"].(string)
		if !strings.Contains(desc, want) {
			t.Errorf("%s depth description %q does not say %q", name, desc, want)
		}
	}
}
