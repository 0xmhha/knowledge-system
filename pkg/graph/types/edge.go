package types

// Edge mirrors the SQLite edges row (spec §5.3).
//
// DispatchKind (schema 1.7): optional metadata column for the
// `invokes` edge type that disambiguates the dispatch mechanism. Valid values:
//
//	"interface_method" — callee is an interface method (virtual dispatch via
//	                     types.Selection on a *types.Interface receiver)
//	"func_value"       — callee is a function-typed variable / parameter
//	"method_value"     — callee is a struct field of function type
//	"closure"          — inline closure literal call: `func(){...}()`
//
// Empty string for every non-`invokes` edge AND for `invokes` edges that
// resolve as static (which shouldn't happen by construction — invokes is
// reserved for non-static dispatch). Old readers (schema ≤1.6) ignore the
// column gracefully because the SQLite ALTER ADD COLUMN keeps it nullable
// and SELECT projections in those readers don't reference it.
type Edge struct {
	ID           int64      `json:"id,omitempty"`
	Src          string     `json:"src"        validate:"required,len=16"`
	Dst          string     `json:"dst"        validate:"required,len=16"`
	Type         EdgeType   `json:"type"       validate:"required"`
	FilePath     string     `json:"file_path,omitempty"`
	Line         int        `json:"line,omitempty"`
	Count        int        `json:"count"      validate:"min=1"`
	Confidence   Confidence `json:"confidence" validate:"required"`
	DispatchKind string     `json:"dispatch_kind,omitempty"`
	// Order: source-order index for edges where
	// position is part of the relationship. Currently used by
	// EdgeHasModifier so multi-modifier functions preserve the
	// application sequence (Solidity applies modifiers outer-to-inner
	// in source order — `nonReentrant onlyOwner` wraps differently
	// from `onlyOwner nonReentrant`). Zero is omitted from JSON so
	// other edge types stay unaffected.
	Order int `json:"order,omitempty"`
}

// DispatchGoroutine is the dispatch kind of an edge from a Goroutine node
// to the code it runs (calls to a function, invokes of an interface method,
// dispatches_to through a function value or closure, reads_field and
// writes_field, references to a package-level variable),
// found in SSA (schema 1.42). A Goroutine is a caller; its edges are kept
// apart from the function-level ones (IsGoroutineCode).
const DispatchGoroutine = "goroutine"

// goroutineCodeTypes are the edge types a Goroutine's code edges have.
var goroutineCodeTypes = map[EdgeType]bool{EdgeCalls: true, EdgeInvokes: true, EdgeDispatchesTo: true,
	EdgeReadsField: true, EdgeWritesField: true, EdgeReferences: true}

// IsGoroutineCode reports whether e is an edge from a Goroutine to the code
// it runs (DispatchGoroutine). The graph walks and the readers of the query
// tools leave these out, so a function's callers and a field's users stay
// what the function-level edges say; the tools list goroutine callers
// under a key of their own.
func IsGoroutineCode(e Edge) bool {
	return e.DispatchKind == DispatchGoroutine && goroutineCodeTypes[e.Type] ||
		e.DispatchKind == DispatchGoroutineWrite && e.Type == EdgeReferences
}

// DispatchGoroutineWrite is the dispatch kind of a references edge from a
// Goroutine to a package-level variable its code assigns; a read has
// DispatchGoroutine. (Function-level references say read or write in the
// dispatch kind, which a goroutine's edge uses to say it is a goroutine's.)
const DispatchGoroutineWrite = "goroutine_write"

// DispatchPastCopy is the dispatch kind of a changed_in edge to a commit
// older than the copy that created the node's file: the line history goes
// on into the copied file, which git log -L does for some copies and not
// for others.
const DispatchPastCopy = "past_copy"

// DispatchCopy is the dispatch kind of a changed_in edge to the commit that
// created the node's file as a copy of another file: it brought the lines
// into the file (git log -L lists it unless it follows the copy and the
// copy left the lines as they were).
const DispatchCopy = "copy"

// WithoutGoroutineCode returns edges without the goroutine code edges
// (IsGoroutineCode), in order. It returns edges itself when there are none.
func WithoutGoroutineCode(edges []Edge) []Edge {
	for i, e := range edges {
		if !IsGoroutineCode(e) {
			continue
		}
		out := append(make([]Edge, 0, len(edges)-1), edges[:i]...)
		for _, e := range edges[i+1:] {
			if !IsGoroutineCode(e) {
				out = append(out, e)
			}
		}
		return out
	}
	return edges
}

// GoroutineCodeSQL is the SQL condition that leaves goroutine code edges
// out of a query over the edges table (IsGoroutineCode).
var GoroutineCodeSQL = NotGoroutineCodeSQL("")

// NotGoroutineCodeSQL is GoroutineCodeSQL for an edges table named by the
// alias prefix ("e." for "FROM edges e").
func NotGoroutineCodeSQL(prefix string) string {
	kind, typ := "COALESCE("+prefix+"dispatch_kind, '')", prefix+"type"
	return "NOT (" + kind + " = '" + DispatchGoroutine + "' AND " + typ +
		" IN ('calls', 'invokes', 'dispatches_to', 'reads_field', 'writes_field', 'references') OR " +
		kind + " = '" + DispatchGoroutineWrite + "' AND " + typ + " = 'references')"
}

// DispatchReceived is the dispatch kind of a flows_to edge from a receive
// operation's CallSite to a use of the value it received (schema 1.42).
const DispatchReceived = "received"

// ChannelLockSubKind is the sub kind of a Channel node used as a lock: a
// channel of empty structs with a buffer of one that is filled where it is
// made (schema 1.42). Receiving its token is acquires_lock, returning it
// releases_lock.
const ChannelLockSubKind = "lock"
