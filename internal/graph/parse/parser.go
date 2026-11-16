package parse

import (
	"fmt"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// PendingRef is an unresolved cross-file reference produced in Pass 1
// and resolved (or marked AMBIGUOUS) in Pass 2.
//
// DispatchKind (schema 1.7): optional metadata for `invokes`
// edges. Empty for static `calls`. Carried through Pass 2 Resolve so the
// final edge row preserves the dispatch classification decided at AST time
// (when types.Info is still in scope).
type PendingRef struct {
	SrcID       string
	EdgeType    types.EdgeType
	TargetQName string
	// TargetCanonicalID is the import-path-qualified id of the target when
	// the parser resolved it with type information (Go typed builds). When
	// set, Resolve matches it exactly and never falls back to TargetQName: a
	// short-name match can belong to another package with the same name.
	TargetCanonicalID string
	HintFile          string
	Line              int
	// ByteOffset: byte position of the
	// referent in the source file. Used by the Solidity resolver to
	// disambiguate same-line shadow scopes — when the line-only
	// `declLine <= useSiteLine <= scopeEndLine` filter admits multiple
	// decls, the byte offset breaks the tie via containment. Zero =
	// unset (other parsers leave it default; Sol resolver falls back
	// to line-only behavior when bytes are missing).
	ByteOffset int
	// Order: source-order index. Carried to the
	// resolved Edge.Order field. Currently populated by Sol's
	// runHasModifier walker; ignored elsewhere.
	Order        int
	DispatchKind string
}

// ParseResult is the per-file output of Pass 1.
type ParseResult struct {
	Path    string
	Nodes   []types.Node
	Edges   []types.Edge
	Pending []PendingRef
}

// ResolvedGraph is the per-language Pass 2 output: in addition to the union of
// per-file results, edges that resolved or were marked AMBIGUOUS.
type ResolvedGraph struct {
	Nodes []types.Node
	Edges []types.Edge
	// Unresolved counts, by reason, the pending references Resolve could not
	// bind to a node and dropped; UnresolvedSamples keeps the first few per
	// reason as "target (file:line)". Nil when the parser does not count.
	Unresolved        map[string]int
	UnresolvedSamples map[string][]string
	// LoadErrors is the number of errors the language toolchain reported
	// while loading the sources (Go: go/packages list, parse and type
	// errors over the packages and their dependencies).
	LoadErrors int
}

// Reasons a reference is left unresolved.
const (
	// UnresolvedOutsideGraph: the target has a canonical id but no node, so it
	// lies outside the graph (standard library, outside modules, files the
	// build does not compile).
	UnresolvedOutsideGraph = "outside_graph"
	// UnresolvedNoTarget: the parser cleared the target because the call has
	// no declaration to bind (builtins, conversions, function values,
	// closures).
	UnresolvedNoTarget = "no_target"
	// UnresolvedBareName: a typed build left only a bare name, which go/types
	// could not tie to a declaration of the graph (a method of the builtin
	// error interface, a call through an expression).
	UnresolvedBareName = "bare_name"
	// UnresolvedNoMatch: no node has the target's qualified name.
	UnresolvedNoMatch = "no_match"
)

// UnresolvedSampleLimit caps the samples kept per reason.
const UnresolvedSampleLimit = 20

// CountUnresolved records one dropped reference on rg.
func (rg *ResolvedGraph) CountUnresolved(reason, target, file string, line int) {
	if rg.Unresolved == nil {
		rg.Unresolved = map[string]int{}
		rg.UnresolvedSamples = map[string][]string{}
	}
	rg.Unresolved[reason]++
	if len(rg.UnresolvedSamples[reason]) < UnresolvedSampleLimit {
		rg.UnresolvedSamples[reason] = append(rg.UnresolvedSamples[reason], fmt.Sprintf("%s (%s:%d)", target, file, line))
	}
}

// Parser is the contract every language parser implements.
type Parser interface {
	// ParseFile runs Pass 1 on a single file. Pure function — must be safe
	// to call concurrently from a worker pool.
	ParseFile(path string, src []byte) (*ParseResult, error)

	// Resolve runs Pass 2 over the union of ParseResults from the same language.
	Resolve(results []*ParseResult) (*ResolvedGraph, error)

	// Extensions reports the file extensions this parser handles (lowercase, with leading ".").
	Extensions() []string
}
