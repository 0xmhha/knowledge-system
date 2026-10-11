package persist

import "errors"

// ErrNoReviewIndex reports a graph built before schema 1.24, which has no
// review index; rebuilding the graph adds it.
var ErrNoReviewIndex = errors.New("graph has no review index (schema 1.24); rebuild the graph")

// ErrReviewSeedNotFound reports that no production function or method has
// the requested qualified name.
var ErrReviewSeedNotFound = errors.New("no production function or method with that qualified name")

// ReviewOptions bounds a review. Zero values take the defaults below.
type ReviewOptions struct {
	CallerDepth int // reverse call walk depth, 1..5 (default 5)
	MaxPaths    int // caller paths returned (default 12)
	MaxItems    int // entries per list (default 40)
}

func (o ReviewOptions) withDefaults() ReviewOptions {
	if o.CallerDepth <= 0 || o.CallerDepth > 5 {
		o.CallerDepth = 5
	}
	if o.MaxPaths <= 0 {
		o.MaxPaths = 12
	}
	if o.MaxItems <= 0 {
		o.MaxItems = 40
	}
	return o
}

// ReviewSymbol is a node reference in a review.
type ReviewSymbol struct {
	ID            string `json:"id"`
	QualifiedName string `json:"qualified_name"`
	Type          string `json:"type"`
	FilePath      string `json:"file_path"`
	StartLine     int    `json:"start_line"`
	EndLine       int    `json:"end_line"`
}

// ReviewCallSite is one call expression inside the function, in source order.
type ReviewCallSite struct {
	Line int    `json:"line"`
	Text string `json:"text"`
}

// ReviewCommit is a commit that changed the function.
type ReviewCommit struct {
	SHA     string `json:"sha"`
	Time    int64  `json:"time"`
	Subject string `json:"subject"`
}

// Review is the production-code, history and test view of one function.
// Lists are capped by ReviewOptions; the *Total fields carry full counts.
type Review struct {
	Seed      ReviewSymbol `json:"seed"`
	Signature string       `json:"signature"`

	// Inputs, outputs and state.
	UsesTypes        []ReviewSymbol `json:"uses_types"`
	Instantiates     []ReviewSymbol `json:"instantiates"`
	WritesFields     []ReviewSymbol `json:"writes_fields"`
	ReturnStatements int            `json:"return_statements"`

	// Internal logic, in source order.
	Control        map[string]int   `json:"control"`
	CallSites      []ReviewCallSite `json:"call_sites"`
	CallSitesTotal int              `json:"call_sites_total"`
	Callees        []ReviewSymbol   `json:"callees"`

	// Callers back to entry points (production code only).
	Callers       []ReviewSymbol `json:"callers"`
	ViaInterfaces []ReviewSymbol `json:"via_interfaces"`
	CallerPaths   [][]string     `json:"caller_paths"`
	EntryPoints   int            `json:"entry_points"`

	// Dynamic paths.
	Goroutines            []ReviewSymbol      `json:"goroutines"`
	SendsTo               []ReviewSymbol      `json:"sends_to"`
	RecvsFrom             []ReviewSymbol      `json:"recvs_from"`
	AcquiresLocks         []ReviewSymbol      `json:"acquires_locks"`
	ReleasesLocks         []ReviewSymbol      `json:"releases_locks"`
	TimeoutPaths          []ReviewSymbol      `json:"timeout_paths"`
	CancellationPaths     []ReviewSymbol      `json:"cancellation_paths"`
	InterfaceImplementers map[string][]string `json:"interface_implementers"`

	// History: depth-1 lookups, never walked onward.
	Commits      []ReviewCommit `json:"commits"`
	CommitsTotal int            `json:"commits_total"`

	// Tests: listed, not walked.
	TestCallers      []ReviewSymbol `json:"test_callers"`
	TestCallersTotal int            `json:"test_callers_total"`
}
