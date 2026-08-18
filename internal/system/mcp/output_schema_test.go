package mcp

import (
	"sort"
	"strings"
	"testing"

	mcpserver "github.com/mark3labs/mcp-go/server"
)

// TestEveryToolDeclaresAnOutputSchema holds the line the wire contract needs
// and that no single tool's own test can hold.
//
// A tool without an outputSchema tells a client nothing about the shape it
// returns, so the only thing standing between a malformed structuredContent
// and the caller is this repo's own decode helper. That is what let a handler
// hand back a bare slice once: it serialises to an array, which a conforming
// client rejects before the caller sees the payload, and every test that
// unmarshalled into a slice still passed.
//
// The emptiness check is the part that matters. mcpgo.WithOutputSchema
// generates the schema by reflection and, when the type defeats it, prints to
// stderr and returns without setting anything — leaving the tool silently
// undeclared. Asserting the schema is present and populated turns that quiet
// failure into a build failure.
func TestEveryToolDeclaresAnOutputSchema(t *testing.T) {
	t.Parallel()

	srv := mcpserver.NewMCPServer("cks-test", "0.0.1")
	if err := Register(srv, newFixture(t, nil).deps); err != nil {
		t.Fatalf("Register: %v", err)
	}

	tools := srv.ListTools()
	if len(tools) == 0 {
		t.Fatal("no tools registered: this test would pass vacuously")
	}

	for name, st := range tools {
		t.Run(name, func(t *testing.T) {
			schema := st.Tool.OutputSchema
			switch {
			case schema.Type == "":
				t.Errorf("declares no outputSchema — add mcpgo.WithOutputSchema[T]() "+
					"naming the type %s hands to NewToolResultStructured", name)
			case schema.Type != "object":
				// MCP requires structuredContent to be a JSON object, so the
				// schema describing it can only be an object.
				t.Errorf("outputSchema type is %q, want \"object\"", schema.Type)
			case len(schema.Properties) == 0:
				t.Error("outputSchema has no properties: reflection produced nothing, " +
					"which is how WithOutputSchema reports a type it cannot describe")
			}
		})
	}
}

// TestOutputSchemaDescribesTheDeclaredResponse checks that the schema a tool
// declares is the shape it actually returns, not merely some object.
// Reflection keeps a schema in step with its type automatically; naming the
// WRONG type in WithOutputSchema is the mistake it cannot catch, and that
// mistake stays invisible until a client validates a response against it.
//
// Every registered tool is listed, deliberately. An earlier draft spot-checked
// eleven of them and did not notice a type swapped on get_subgraph, which was
// not on the list — a partial guard reads exactly like a whole one once it is
// green.
//
// What it still cannot catch: two tools whose responses carry the same
// top-level field names. impact_analysis and concurrency_impact are such a
// pair (both seed/result/instructions), as are find_callers and find_callees,
// which share one type on purpose. Their nested shapes differ; this test
// compares field names only.
func TestOutputSchemaDescribesTheDeclaredResponse(t *testing.T) {
	t.Parallel()

	srv := mcpserver.NewMCPServer("cks-test", "0.0.1")
	if err := Register(srv, newFixture(t, nil).deps); err != nil {
		t.Fatalf("Register: %v", err)
	}
	tools := srv.ListTools()

	// The properties each response is recognisable by: the json tags on the
	// type its handler constructs.
	want := map[string][]string{
		ToolNameChangeHistory:           {"seed", "hunks", "prs"},
		ToolNameConcurrencyImpact:       {"seed", "result"},
		ToolNameExpandFlow:              {"origin", "direction", "neighbors", "origin_branches"},
		ToolNameFindBranches:            {"matches"},
		ToolNameFindCallees:             {"seed", "direction", "neighbors"},
		ToolNameFindCallers:             {"seed", "direction", "neighbors"},
		ToolNameFindInvariants:          {"invariants"},
		ToolNameFindSymbol:              {"symbol", "citations"},
		ToolNameGetConventions:          {"conventions"},
		ToolNameGetFlow:                 {"flow_id", "entry_point", "steps"},
		ToolNameGetForTask:              {"query", "citations", "knowledge", "metadata"},
		ToolNameGetInvariantEnforcement: {"inv_id", "statement", "enforced_at"},
		ToolNameGetSubgraph:             {"seed", "nodes", "edges"},
		ToolNameImpactAnalysis:          {"seed", "result"},
		ToolNameSearchText:              {"query", "hits"},
		ToolNameSemanticSearch:          {"query", "hits"},
		ToolNameFreshness:               {"fresh", "indexed_head", "current_head"},
		ToolNameHealth:                  {"name", "status", "serviceable"},
		ToolNameOpsIndex:                {"mode", "graph", "vector", "alignment"},
		ToolNameOpsReindex:              {"job_id", "state", "version"},
		ToolNameOpsSetup:                {"job_id", "state"},
		ToolNameOpsSetupStatus:          {"id", "state", "events"},
	}

	// A tool added without a line here would otherwise be checked by nothing.
	for name := range tools {
		if _, ok := want[name]; !ok {
			t.Errorf("tool %q has no expected-properties entry: add one so a wrong "+
				"WithOutputSchema type on it cannot pass unnoticed", name)
		}
	}

	for name, props := range want {
		t.Run(name, func(t *testing.T) {
			st, ok := tools[name]
			if !ok {
				t.Fatalf("tool %q is not registered", name)
			}
			for _, p := range props {
				if _, ok := st.Tool.OutputSchema.Properties[p]; !ok {
					have := make([]string, 0, len(st.Tool.OutputSchema.Properties))
					for k := range st.Tool.OutputSchema.Properties {
						have = append(have, k)
					}
					sort.Strings(have)
					t.Errorf("outputSchema is missing %q — does WithOutputSchema name the wrong type? have: %s",
						p, strings.Join(have, ", "))
				}
			}
		})
	}
}
