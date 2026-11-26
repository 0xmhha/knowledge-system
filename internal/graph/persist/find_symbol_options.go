package persist

import "github.com/0xmhha/knowledge-system/pkg/graph/types"

// DefaultFindSymbolLimit is the number of rows FindSymbol returns when
// FindSymbolOptions.Limit is zero. It bounds a response for callers that did
// not ask for the whole match set.
const DefaultFindSymbolLimit = 100

// FindSymbolNoLimit as FindSymbolOptions.Limit returns every match. A caller
// that ranks or counts the matches itself needs all of them: on go-stablenet
// the first 100 rows for "Hash" held 16 nodes of that name out of 63
// declarations in the source.
const FindSymbolNoLimit = -1

// FindSymbolOptions configures filter push-down for StoreReader.FindSymbol.
// Zero value means "no filter" — every match on the name (and exactness mode)
// passes through, up to DefaultFindSymbolLimit rows.
//
// Downstream motivation: cks Stage 2's `arch_explain` intent fetches
// Function / Type / Interface symbols separately, paying N round-trips
// for what should be one. With Kinds set, the SQL layer returns a kind-
// tagged result in a single query so cks can dedupe by Citation key on
// the way back.
type FindSymbolOptions struct {
	// Language pushes a `language = ?` predicate when non-empty.
	// Empty string disables the predicate.
	Language string

	// Kinds restricts results to the named NodeTypes (SQL `type IN (...)`).
	// Empty slice (or nil) disables the predicate — all kinds are returned.
	// Duplicates are tolerated (the SQL planner dedupes); callers don't need
	// to deduplicate.
	Kinds []types.NodeType

	// Limit caps the rows returned: zero means DefaultFindSymbolLimit, a
	// positive value replaces it and FindSymbolNoLimit returns every match.
	Limit int

	// IgnoreCase makes a suffix match (exact=false) compare names without
	// regard to case. The default keeps the case the caller wrote. It has no
	// effect on an exact match.
	IgnoreCase bool
}

// rowLimit is the SQL LIMIT for o: the default cap, the caller's cap, or -1
// (no limit in SQLite) for FindSymbolNoLimit.
func (o FindSymbolOptions) rowLimit() int {
	switch {
	case o.Limit == 0:
		return DefaultFindSymbolLimit
	case o.Limit < 0:
		return FindSymbolNoLimit
	default:
		return o.Limit
	}
}
