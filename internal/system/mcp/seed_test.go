package mcp

import (
	"sort"
	"strings"
	"testing"

	mcpserver "github.com/mark3labs/mcp-go/server"
)

// seedParamNames are the argument names a caller-supplied symbol arrives under.
var seedParamNames = map[string]bool{"symbol": true}

// seedCoverage records, for every tool that takes a symbol seed, where the
// guarantee "an unresolved seed is refused, not silently answered" is tested.
// A tool resolves its seed in exactly one of two places, and the two are
// tested in different packages:
//
//   - mcp: the traversal needs a Citation, so seedCitation resolves it here.
//     Covered by TestHandleFindRelatives_UnresolvedSeedIsRefused and
//     TestHandleChangeHistory_UnresolvedSeedIsRefused.
//   - ckgclient: the traversal takes a qname, so Real.resolveSeedOrErr
//     resolves it there. Covered by
//     ckgclient.TestReal_SeedResolutionFailureIsAnError.
//
// The split is why this defect kept coming back one surface at a time: a fix
// applied in one place looks complete from that side. Listing every seeded
// tool against its owner makes a new tool declare which half it belongs to
// instead of quietly belonging to neither.
var seedCoverage = map[string]string{
	ToolNameFindCallers:       "mcp",
	ToolNameFindCallees:       "mcp",
	ToolNameChangeHistory:     "mcp",
	ToolNameGetSubgraph:       "ckgclient",
	ToolNameImpactAnalysis:    "ckgclient",
	ToolNameConcurrencyImpact: "ckgclient",
}

// TestEverySeededToolHasSeedRefusalCoverage fails when a tool takes a symbol
// seed and no one has said where its refusal is tested. It cannot check the
// behaviour of a tool it has never seen, so it checks that a human decided.
func TestEverySeededToolHasSeedRefusalCoverage(t *testing.T) {
	t.Parallel()

	srv := mcpserver.NewMCPServer("cks-test", "0.0.1")
	if err := Register(srv, newFixture(t, nil).deps); err != nil {
		t.Fatalf("Register: %v", err)
	}

	var seeded []string
	for name, st := range srv.ListTools() {
		for prop := range st.Tool.InputSchema.Properties {
			if seedParamNames[prop] {
				seeded = append(seeded, name)
				break
			}
		}
	}
	sort.Strings(seeded)
	if len(seeded) == 0 {
		t.Fatal("no tool declares a symbol seed: this test would pass vacuously")
	}

	for _, name := range seeded {
		if owner, ok := seedCoverage[name]; !ok {
			t.Errorf("tool %q takes a symbol seed but seedCoverage does not say where its "+
				"refusal is tested; add it (owner \"mcp\" or \"ckgclient\") and write the test",
				name)
		} else if owner != "mcp" && owner != "ckgclient" {
			t.Errorf("tool %q: unknown seed-resolution owner %q", name, owner)
		}
	}
	for name := range seedCoverage {
		found := false
		for _, s := range seeded {
			if s == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("seedCoverage lists %q, which no longer takes a symbol seed: "+
				"remove the entry, or the list stops describing the surface", name)
		}
	}
	t.Logf("seeded tools: %s", strings.Join(seeded, ", "))
}
