package stage2

import "github.com/0xmhha/knowledge-system/pkg/system/contract"

// intentToKinds returns the SymbolKinds filter for ckg.FindSymbol given
// the user's Intent. Empty result means "any kind" — the safe default
// for Intents where filtering would over-constrain.
//
// The mapping is intentionally narrow: each Intent points at the symbol
// kinds the user is overwhelmingly likely to act on. BM25Search still
// runs unfiltered so conceptual matches survive even when the kind
// filter rules out the obvious symbol form. Phase E will measure
// whether tightening or loosening individual entries improves recall.
// Kind vocabulary note (2026-07-29): filter strings must match ckg's
// actual node-type taxonomy lowercased (Function, Method, Struct,
// Interface, TypeAlias, Constant, Variable, Field, ...). The original
// entries "type" and "const" matched NOTHING — ckg has no Type node
// (types are Struct/Interface/TypeAlias) and constants are Constant —
// so several intents ran on a silently narrower filter than designed.
// The groups below use the real vocabulary; a regression test pins
// every string against the taxonomy.
var (
	kindsCallable = []string{"function", "method"}
	kindsType     = []string{"struct", "interface", "typealias"}
	// kindsValue covers declaration-site lookups the callable/type
	// groups miss: option fields ("EnableBM25Rerank"), constants, and
	// top-level variables. Added after the field-level lookup gap —
	// FindSymbol resolved query.Options.EnableBM25Rerank but the
	// feature_add filter dropped the Field node on the floor.
	kindsValue = []string{"field", "constant", "variable"}
)

func intentToKinds(intent contract.Intent) []string {
	switch intent {
	case contract.IntentBugFix:
		// Bugs surface in callable code (the place the runtime executes).
		return kindsCallable
	case contract.IntentFeatureAdd:
		// New features add callable surface, new type boundaries, AND the
		// declaration sites they extend (option fields, constants, vars).
		return concatKinds(kindsCallable, kindsType, kindsValue)
	case contract.IntentArchExplain:
		// "How does X work" applies to whatever the user named — function,
		// method, type, interface, or const. FindSymbol returns LOCATIONS,
		// not bodies, so including callable kinds does not pollute results
		// with hot-path implementations; it just ensures the definition
		// receives the SymbolBonus when the user asks about a function or
		// method (a common case: "how does HandleRequest work").
		// Excluded: variable (locals are weak architecture signals);
		// field/constant stay in — option fields and const catalogs ARE
		// architecture surface.
		return concatKinds(kindsType, kindsCallable, []string{"constant", "field"})
	case contract.IntentTestAdd:
		// Tests target callable units.
		return kindsCallable
	case contract.IntentConcurrencySafety:
		// Concurrency issues live in callable code paths (synchronization
		// is enacted in functions, not in type declarations).
		return kindsCallable
	case contract.IntentSecurity:
		// Security audits trace input boundaries (handlers, validators)
		// and trust boundaries (interfaces).
		return []string{"function", "method", "interface"}
	case contract.IntentDocsUpdate:
		// Documentation describes the API surface: types, interfaces,
		// and callable signatures.
		return concatKinds(kindsType, kindsCallable)
	case contract.IntentRefactor, contract.IntentQAReview, contract.IntentUnknown:
		// No filter — refactor and review touch anything, and Unknown
		// must stay broad on purpose.
		return nil
	}
	return nil
}

// concatKinds joins kind groups into one slice (no dedup needed — the
// groups are disjoint by construction).
func concatKinds(groups ...[]string) []string {
	var out []string
	for _, g := range groups {
		out = append(out, g...)
	}
	return out
}
