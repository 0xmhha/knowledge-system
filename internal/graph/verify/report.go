package verify

import (
	"encoding/json"
	"fmt"
	"io"
)

// Report is the result of one verification run.
type Report struct {
	Files          int           `json:"files"`
	Decls          DeclStats     `json:"declarations"`
	StaticCalls    RelationStats `json:"static_calls"`
	InterfaceCalls RelationStats `json:"interface_calls"`
	TypeRefs       RelationStats `json:"type_refs"`
	// DynamicCallSites are calls through function values, method values and
	// closures. go/types gives them no callee, so they are counted, not checked.
	DynamicCallSites  int             `json:"dynamic_call_sites"`
	CallsOutsideFuncs int             `json:"calls_outside_funcs"`
	Unexpected        UnexpectedStats `json:"unexpected_edges"`
	LocalDeclNodes    LocalDeclStats  `json:"unmatched_decl_nodes"`
}

// DeclStats compares package-level declarations.
type DeclStats struct {
	Truth          int      `json:"truth"`
	Matched        int      `json:"matched"`
	Missing        int      `json:"missing"`
	MissingSamples []string `json:"missing_samples,omitempty"`
}

// RelationStats compares one relation family.
type RelationStats struct {
	Truth   int `json:"truth"`
	Matched int `json:"matched"`
	// Misresolved: the graph has an edge from the caller to another node of
	// the same name and kind, but not to the declaration go/types resolves.
	Misresolved int `json:"misresolved"`
	// Missing: no edge to the expected node and no same-name substitute.
	Missing int `json:"missing"`
	// MissingInClosure counts the Missing pairs whose first site is inside a
	// function literal.
	MissingInClosure int `json:"missing_in_closure"`
	// EndpointMissing: a declaration of the pair has no graph node.
	EndpointMissing    int      `json:"endpoint_missing"`
	MisresolvedSamples []string `json:"misresolved_samples,omitempty"`
	MissingSamples     []string `json:"missing_samples,omitempty"`
}

// Recall is Matched over the pairs whose endpoints both exist in the graph.
func (s RelationStats) Recall() float64 {
	d := s.Truth - s.EndpointMissing
	if d == 0 {
		return 0
	}
	return float64(s.Matched) / float64(d)
}

// UnexpectedStats counts graph edges between matched declarations that
// go/types does not support. The edges of misresolved pairs are included.
type UnexpectedStats struct {
	Calls          int      `json:"calls"`
	TypeRefs       int      `json:"type_refs"`
	CallSamples    []string `json:"call_samples,omitempty"`
	TypeRefSamples []string `json:"type_ref_samples,omitempty"`
}

// LocalDeclStats counts declaration-kind graph nodes that no package-level
// declaration maps to.
type LocalDeclStats struct {
	Count   int      `json:"count"`
	Samples []string `json:"samples,omitempty"`
}

// WriteJSON writes the report as indented JSON.
func (r Report) WriteJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(r); err != nil {
		return fmt.Errorf("verify: encode report: %w", err)
	}
	return nil
}

// WriteText writes a short human-readable summary with samples.
func (r Report) WriteText(w io.Writer) error {
	p := func(format string, a ...any) { _, _ = fmt.Fprintf(w, format, a...) }
	p("ckg verify (go): %d files\n", r.Files)
	p("declarations: truth %d, matched %d, missing %d\n", r.Decls.Truth, r.Decls.Matched, r.Decls.Missing)
	for _, rel := range []struct {
		name string
		s    RelationStats
	}{{"static calls", r.StaticCalls}, {"interface calls", r.InterfaceCalls}, {"type refs", r.TypeRefs}} {
		p("%s: truth %d, matched %d, misresolved %d, missing %d (in closures %d), endpoint missing %d, recall %.4f\n",
			rel.name, rel.s.Truth, rel.s.Matched, rel.s.Misresolved, rel.s.Missing, rel.s.MissingInClosure,
			rel.s.EndpointMissing, rel.s.Recall())
	}
	p("dynamic call sites (not checked): %d\n", r.DynamicCallSites)
	p("calls in package-level initialisers (not checked): %d\n", r.CallsOutsideFuncs)
	p("unexpected edges: calls %d, type refs %d\n", r.Unexpected.Calls, r.Unexpected.TypeRefs)
	p("declaration nodes with no package-level declaration: %d\n", r.LocalDeclNodes.Count)
	section := func(head string, items []string) {
		if len(items) == 0 {
			return
		}
		p("\n%s:\n", head)
		for _, it := range items {
			p("  %s\n", it)
		}
	}
	section("missing declarations", r.Decls.MissingSamples)
	section("misresolved static calls", r.StaticCalls.MisresolvedSamples)
	section("missing static calls", r.StaticCalls.MissingSamples)
	section("misresolved interface calls", r.InterfaceCalls.MisresolvedSamples)
	section("missing interface calls", r.InterfaceCalls.MissingSamples)
	section("misresolved type refs", r.TypeRefs.MisresolvedSamples)
	section("missing type refs", r.TypeRefs.MissingSamples)
	section("unexpected call edges", r.Unexpected.CallSamples)
	section("unexpected type-ref edges", r.Unexpected.TypeRefSamples)
	section("declaration nodes with no package-level declaration", r.LocalDeclNodes.Samples)
	return nil
}
