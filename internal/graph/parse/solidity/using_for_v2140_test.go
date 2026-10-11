package solidity_test

import (
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// Interface-body using-for, three variants in one test.
//
// The using-for query in queries.go nests `using_directive` under
// `interface_declaration`, so interface bodies are walked. This test
// probes the empirical behavior for each of the three `using` forms
// when the directive appears inside an interface body — semantically
// nonsensical (interfaces have no state), but the question is what
// the parser+resolver actually emits.
//
// Three interfaces in one fixture:
//   IBare → `using SafeMath for uint256;`         (legacy type-alias)
//   IFree → `using {Math.add} for uint256;`       (0.8.13+ free-func)
//   IOp   → `using {Math.add as +} for uint256;`  (0.8.19+ operator)
//
// Initial hypothesis (assertion values may be updated post-run to
// lock the observed empirical behavior):
//   IBare → 1 EdgeUsesFor : happy path on type_alias.
//   IFree → 1 EdgeUsesFor : incidental @lib capture (as at contract scope).
//   IOp   → 0 EdgeUsesFor : operator-form AST shape mismatch.
//
// Contrast table extension (cf. the contract- and file-scope tests):
//   scope:interface × variant:type-alias    → (IBare)
//   scope:interface × variant:free-func     → (IFree)
//   scope:interface × variant:operator-form → (IOp)

// TestUsingForV2140_InterfaceBodyVariants — locks edge counts and
// surround-safety for all three `using` variants nested inside an
// interface body.
func TestUsingForV2140_InterfaceBodyVariants(t *testing.T) {
	nodes, edges := parseResolveOneSol(t, "testdata/using_for_v2140", "interface_body_variants.sol")

	// Build id→node lookup for source resolution.
	byID := map[string]types.Node{}
	for _, n := range nodes {
		byID[n.ID] = n
	}

	// Tally EdgeUsesFor by source-interface name.
	counts := map[string]int{}
	for _, e := range edges {
		if e.Type != types.EdgeUsesFor {
			continue
		}
		counts[byID[e.Src].Name]++
	}

	// (a) IBare — legacy type-alias form. Predicted: 1.
	if got := counts["IBare"]; got != 1 {
		t.Errorf("V2.14 IBare (bare type-alias): expected 1 EdgeUsesFor, got %d", got)
	}

	// (b) IFree — free-function form. Predicted: 1 (mirrors contract scope).
	if got := counts["IFree"]; got != 1 {
		t.Errorf("V2.14 IFree (free-function): expected 1 EdgeUsesFor, got %d", got)
	}

	// (c) IOp — operator-form. 1 EdgeUsesFor: the
	// operator-form recovery walker pattern-matches the
	// misparsed `state_variable_declaration` shape and emits the
	// binding pair, flipping this lock from 0 → 1.
	if got := counts["IOp"]; got != 1 {
		t.Errorf("V2.14 IOp (operator-form, post-V2.20 recovery): expected 1 EdgeUsesFor, got %d", got)
		for _, e := range edges {
			if e.Type == types.EdgeUsesFor && byID[e.Src].Name == "IOp" {
				t.Logf("  IOp edge: %+v", e)
			}
		}
	}

	// (d) Surround-safety: every declaration in the fixture must index,
	// regardless of edge count outcome.
	want := map[string]bool{
		"SafeMath": false,
		"Math":     false,
		"IBare":    false,
		"IFree":    false,
		"IOp":      false,
	}
	for _, n := range nodes {
		if _, ok := want[n.QualifiedName]; ok {
			want[n.QualifiedName] = true
		}
	}
	for qn, seen := range want {
		if !seen {
			t.Errorf("V2.14 surround-safety: declaration %q not indexed", qn)
		}
	}
}
