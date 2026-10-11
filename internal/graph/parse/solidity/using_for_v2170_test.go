package solidity_test

import (
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// Operator-form grammar-block lock + reclassification of the
// operator-form gap.
//
// Original plan: extend queryUsingFor
// with a `using_alias` arm so operator-form `using {f as +} for T;`
// (Sol 0.8.19+) emits an EdgeUsesFor like the type-alias variant.
// The gap had been classified as "category B (query gap)" because
// the earlier operator-form tests suggested the grammar parsed cleanly
// and only the query missed the `using_alias` child.
//
// Empirical AST dump on the vendored tree-sitter-solidity grammar
// (v1.2.11, internal/parse/solidity/binding/) against
// fixtures using_for_v260/probe_using_alias.sol +
// using_for_v270/probe_operator_form.sol invalidates that claim:
//
//   (a) `using_alias` is NOT a valid node type in the vendored
//       grammar. Tree-sitter rejects any query referencing it with
//       "Invalid node type using_alias". Earlier spec comments
//       referenced it speculatively but the parser
//       doesn't expose it.
//
//   (b) Free-function form `using {Math.add, Math.sub} for T;`
//       parses to a degraded using_directive containing:
//         using_directive
//           ERROR
//             user_defined_type
//           type_alias                    ← partial recovery
//           ERROR "}"
//       The recovered `type_alias` slot is exactly why the query's
//       `(type_alias (identifier) @lib)` incidentally hits the
//       library name — the contract-scope "rediscovery" was a fortuitous
//       partial-parse artifact, not first-class grammar support.
//
//   (c) Operator-form `using {Math.add as +} for T;` parses with NO
//       using_directive node at all. The braced content is
//       misclassified as a state_variable_declaration:
//         ERROR "{ using"
//         contract_body
//           state_variable_declaration
//             type_name (user_defined_type)
//             identifier "as"
//             ERROR "+} for uint256"
//       No type_alias, no library identifier in a queryable position,
//       no incidental query hit. Hence the 0-edge result at every scope.
//
// Conclusion: operator-form is **category A (grammar reject)**, NOT
// category B (query gap). The file / contract / interface 0-edge locks were
// correct as-is — the gap is upstream of the query, not addressable
// by extending it. Only a grammar bump (or an ERROR-tolerant custom
// walker) can fix this.
//
// This test locks the behavior at library
// scope (complementing the file / contract / interface
// cells), asserts surround-safety (the malformed using
// directive doesn't break declarations on either side), and backs
// the operator-form classification in the design doc.
//
// Not covered here:
//   - Upstream grammar bump tracking — when tree-sitter-solidity
//     ships a version that parses operator-form, revisit the
//     operator-form locks at every scope as a coordinated lock-flip.
//   - ERROR-tolerant walker for both file-level `using ... global;`
//     AND operator-form. Both share the
//     "extract identifier from malformed using_directive context"
//     pattern.

// TestUsingForV2170_LibraryScopeOperatorFormGrammarBlock — library-
// scope operator-form using directive currently emits 0 EdgeUsesFor
// because the vendored grammar misparses the whole construct. Locks
// the empirical behavior so a future grammar bump produces a clearly
// failing test (forcing reclassification + lock-flip across the
// file / contract / interface scopes simultaneously).
func TestUsingForV2170_LibraryScopeOperatorFormGrammarBlock(t *testing.T) {
	nodes, edges := parseResolveOneSol(t, "testdata/using_for_v2170", "library_scope_operator_form.sol")

	// (a) Grammar-block lock: 0 EdgeUsesFor scoped to OpHelpers (no
	// using_directive recognised, so no library identifier captured).
	byID := map[string]types.Node{}
	for _, n := range nodes {
		byID[n.ID] = n
	}
	got := 0
	for _, e := range edges {
		if e.Type != types.EdgeUsesFor {
			continue
		}
		if byID[e.Src].Name == "OpHelpers" {
			got++
		}
	}
	// The operator-form recovery walker emits
	// the binding pair after pattern-matching the misparsed
	// state_variable_declaration shape. Library-scope flips 0 → 1
	// alongside the contract and interface scopes. The file-level
	// case is handled differently — the file-level misparse
	// surfaces at source_file scope where the recovery walker can't
	// extract a clean identifier.
	if got != 1 {
		t.Errorf("V2.17 library-scope operator-form (post-V2.20 recovery): expected 1 EdgeUsesFor for OpHelpers, got %d", got)
		for _, e := range edges {
			if e.Type == types.EdgeUsesFor {
				t.Logf("  edge: src=%s dst=%s", byID[e.Src].QualifiedName, byID[e.Dst].QualifiedName)
			}
		}
	}

	// (b) Surround-safety: the Math library + OpHelpers library +
	// OpHelpers.combine function must all index — the grammar-block
	// on the using directive shouldn't cascade and break surrounding
	// declarations.
	want := map[string]bool{
		"Math":              false,
		"Math.add":          false,
		"OpHelpers":         false,
		"OpHelpers.combine": false,
	}
	for _, n := range nodes {
		if _, ok := want[n.QualifiedName]; ok {
			want[n.QualifiedName] = true
		}
	}
	for qn, seen := range want {
		if !seen {
			t.Errorf("V2.17 surround-safety: declaration %q not indexed (grammar-block on using directive cascaded?)", qn)
		}
	}
}
