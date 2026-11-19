package verify

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/0xmhha/knowledge-system/internal/graph/detect"
	"github.com/0xmhha/knowledge-system/internal/graph/verify/observe"
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
	// Statements compares the statement links (schema 1.32) by edge type;
	// nil for a graph without them.
	Statements map[string]StmtRelStats `json:"statement_links,omitempty"`
	// Observed compares a test run's records with the runtime links; nil
	// without --observed.
	Observed       *observe.Report `json:"observed,omitempty"`
	LocalDeclNodes LocalDeclStats  `json:"unmatched_decl_nodes"`
	// Load is what the go command reported while loading the truth. Graph
	// and truth come from the same go command, so a package it cannot load
	// is missing from both and the counts above cannot show it.
	Load LoadStats `json:"load"`
}

// LoadStats reports the truth's package load.
type LoadStats struct {
	detect.LoadProblems
	// Module counts against the build scope; zero without a scope.
	ModulesMatched      int      `json:"modules_matched,omitempty"`
	ModulesOtherVersion []string `json:"modules_other_version,omitempty"`
	ModulesOtherDir     []string `json:"modules_other_dir,omitempty"`
	ModulesNotLoaded    []string `json:"modules_not_loaded,omitempty"`
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
	// the same name and kind, but not to the declaration go/types resolves,
	// and go/types does not support that other edge either.
	Misresolved int `json:"misresolved"`
	// Missing: the edge to the expected node is absent and no unsupported
	// same-name edge stands in for it.
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
	if r.Statements == nil {
		p("statement links: none in this graph\n")
	}
	for _, rel := range stmtRels {
		s, ok := r.Statements[string(rel)]
		if !ok {
			continue
		}
		p("statement links %s: truth %d, matched %d, missing %d, no statement node %d, endpoint missing %d, unexpected %d, given to an outer statement %d, recall %.4f, precision %.4f\n",
			rel, s.Truth, s.Matched, s.Missing, s.NoStatementNode, s.EndpointMissing, s.Unexpected, s.OuterStatement, s.Recall(), s.Precision())
	}
	p("go load: %d packages, %d with errors, %d errors %v\n", r.Load.Packages, r.Load.WithErrors, r.Load.Errors(), r.Load.ByKind)
	if r.Load.ModulesMatched+len(r.Load.ModulesOtherVersion)+len(r.Load.ModulesOtherDir)+len(r.Load.ModulesNotLoaded) > 0 {
		p("scope modules: matched %d, other version %d, other directory %d, not loaded %d\n", r.Load.ModulesMatched,
			len(r.Load.ModulesOtherVersion), len(r.Load.ModulesOtherDir), len(r.Load.ModulesNotLoaded))
	}
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
	for _, rel := range stmtRels {
		if s, ok := r.Statements[string(rel)]; ok {
			section("missing "+string(rel), s.MissingSamples)
			section("unexpected "+string(rel), s.UnexpectedSamples)
		}
	}
	if o := r.Observed; o != nil {
		p("\nobserved run: %d records\n", o.Records)
		for _, x := range []struct {
			name string
			s    observe.PairStats
		}{{"interface calls (method, implementation)", o.Interface}, {"function-value calls (caller, line, callee)", o.FuncValue},
			{"channel operations linked to a channel", o.ChannelOps}, {"sender-receiver pairs joined by a channel", o.ChannelPairs}} {
			p("  %s: observed %d, in graph %d, recall %.4f\n", x.name, x.s.Observed, x.s.InGraph, x.s.Recall())
		}
		p("  interface calls linked only by the type hierarchy: %d\n", o.InterfaceByTypeOnly)
		p("  sender-receiver pairs not joined: both linked to different channel nodes %d, one not linked %d\n", o.ChannelPairsDisjoint, o.ChannelPairsUnlinked)
		p("  dispatches_to edges that ran: %d of %d\n", o.DispatchObserved, o.DispatchEdges)
		section("observed interface calls the graph lacks", o.Interface.MissingSamples)
		section("observed function-value calls the graph lacks", o.FuncValue.MissingSamples)
		section("observed channel operations without a channel edge", o.ChannelOps.MissingSamples)
		section("observed sender-receiver pairs the graph does not join", o.ChannelPairs.MissingSamples)
	}
	section("go load errors", r.Load.Samples)
	section("scope modules loaded at another version", r.Load.ModulesOtherVersion)
	section("scope modules loaded from another directory", firstN(r.Load.ModulesOtherDir, 20))
	return nil
}

func firstN(s []string, n int) []string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
