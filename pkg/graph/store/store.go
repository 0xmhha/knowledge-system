// Package store is the public, read-only graph access surface for external
// callers (eval harness, sister repos like code-knowledge-system). It
// re-exports the minimum useful subset of internal/persist as type aliases
// so callers don't have to depend on internal/persist directly. Write
// access stays internal — there is no Writer here by design.
//
// # Stability
//
// This surface follows semantic versioning once the sister-repo extraction
// lands. Until then, treat it as the single throat to choke when changing
// internal/persist — anything that breaks the alias here will break
// external consumers, even if in-repo callers compile fine.
//
// # What to import from where
//
// External consumers (anything outside this module) should import only
// from pkg/store and pkg/types. They cannot reach internal/persist by
// the Go `internal/` rule, and that's intentional: pkg/store decides what
// to promote to the public surface.
//
// Reader covers the read API; SearchHit / SearchFTSOptions /
// FindSymbolOptions are the value types you'll touch when calling
// Reader.SearchFTS or Reader.FindSymbol. Manifest is the minimal
// mirror of build-time metadata — use the GetManifest helper rather
// than Reader.GetManifest directly so the projection (which drops
// incremental-cache fields) stays the single source of truth.
//
// # Do NOT
//
// External code MUST NOT type-alias persist.StoreReader on its own
// (the "self-shim" pattern observed in the cks consumer). That duplicates
// the public surface and silently drifts the moment we change
// internal/persist. If you find yourself wanting one, it means a type
// you need isn't re-exported here yet — open a PR to add the alias
// instead.
package store

import (
	"github.com/0xmhha/knowledge-system/internal/graph/persist"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// Reader is the read-only graph surface — the canonical entry point for
// external consumers. Aliased from persist.StoreReader; any change to
// the upstream interface is a breaking change here.
type Reader = persist.StoreReader

// SearchHit pairs a node with its full-text search relevance score.
// Returned by Reader.SearchFTS. See persist.SearchHit doc for the
// Score vs RawScore semantics.
type SearchHit = persist.SearchHit

// SearchFTSOptions configures filter push-down for Reader.SearchFTS.
// Zero value means "no filter".
type SearchFTSOptions = persist.SearchFTSOptions

// FindSymbolOptions configures filter push-down for Reader.FindSymbol
// (Language, Kinds). Zero value means "no filter".
type FindSymbolOptions = persist.FindSymbolOptions

// DefaultFindSymbolLimit is the number of rows Reader.FindSymbol returns when
// FindSymbolOptions.Limit is zero; FindSymbolNoLimit as Limit returns every
// match.
const (
	DefaultFindSymbolLimit = persist.DefaultFindSymbolLimit
	FindSymbolNoLimit      = persist.FindSymbolNoLimit
)

// NameResolution is what ResolveName returns: the qualified name and nodes a
// caller-supplied name resolved to, or the candidates of an ambiguous bare
// name. See persist.NameResolution.
type NameResolution = persist.NameResolution

// NameClass says what a node matched by name is (a declaration of the build,
// a promoted copy, a function-local name, an External stub). See
// persist.NameClass.
type NameClass = persist.NameClass

// The classes of a node matched by name, in the order a bare name prefers.
const (
	NameDeclared = persist.NameDeclared
	NamePromoted = persist.NamePromoted
	NameLocal    = persist.NameLocal
	NameExternal = persist.NameExternal
	NameOther    = persist.NameOther
)

// ResolveName maps a canonical id, a qualified name or a bare name to nodes.
// It is the one resolver behind every tool that takes a symbol name; see
// persist.ResolveName for the order.
func ResolveName(r Reader, name, language string) (NameResolution, error) {
	return persist.ResolveName(r, name, language)
}

// ClassifyNames returns the class of each of nodes by id. See
// persist.ClassifyNames.
func ClassifyNames(r Reader, nodes []types.Node) (map[string]NameClass, error) {
	return persist.ClassifyNames(r, nodes)
}

// InFunctionBody reports whether n is declared inside a function body, given
// the enclosers of its file (IsEncloser): the one rule for "known only inside
// one function". See persist.InFunctionBody.
func InFunctionBody(n types.Node, enclosers []types.Node) bool {
	return persist.InFunctionBody(n, enclosers)
}

// TestFiles returns the files the build recorded as test code
// (nodes.is_test: the build scope's file role, the path rule without a
// scope) and whether r holds that record. See persist.TestFiles.
func TestFiles(r Reader) (files []string, recorded bool, err error) { return persist.TestFiles(r) }

// MutexSuffix ends the seed name of a mutex: "<field or variable>#mutex".
// See persist.MutexSuffix.
const MutexSuffix = persist.MutexSuffix

// NamedNodes leaves the Mutex node of a variable out of nodes found by name:
// the name names the variable. See persist.NamedNodes.
func NamedNodes(nodes []types.Node) []types.Node { return persist.NamedNodes(nodes) }

// NameOf returns the name that names n alone: its qualified name, or its
// site name when several declarations carry it. See persist.NameOf.
func NameOf(r Reader, n types.Node) (string, error) { return persist.NameOf(r, n) }

// PrintedQName is the qualified name a tool prints for n ("<name>#mutex" for
// the Mutex node of a variable). See persist.PrintedQName.
func PrintedQName(n types.Node) string { return persist.PrintedQName(n) }

// ParseSiteName splits a site name, "<qname>@<file>:<line>", into its parts;
// ok is false for any other name. See persist.ParseSiteName.
func ParseSiteName(name string) (qname, file string, line int, ok bool) {
	return persist.ParseSiteName(name)
}

// NodesInFunctions returns, by node id, which of nodes are declared inside a
// function body (InFunctionBody). See persist.NodesInFunctions.
func NodesInFunctions(r Reader, nodes []types.Node) (map[string]bool, error) {
	return persist.NodesInFunctions(r, nodes)
}

// IsCallable reports whether n is a function, method, modifier or constructor
// declaration.
func IsCallable(n types.Node) bool { return persist.IsCallable(n) }

// IsEncloser reports whether other declarations can be declared inside n, the
// nodes InFunctionBody takes as enclosers: a callable or a variable. See
// persist.IsEncloser.
func IsEncloser(n types.Node) bool { return persist.IsEncloser(n) }

// PromotedFrom reports whether n is the copy of a promoted method and, if so,
// the qualified name of the method it copies.
func PromotedFrom(n types.Node) (string, bool) { return persist.PromotedFrom(n) }

// DeclSite is where a method is declared; a promoted copy has the site of
// the method it copies. See persist.DeclSite.
type DeclSite = persist.DeclSite

// SiteOf returns the declaration site of n.
func SiteOf(n types.Node) DeclSite { return persist.SiteOf(n) }

// EmbeddingType returns the qualified name of the type a promoted copy is
// listed under.
func EmbeddingType(promotedCopy types.Node) string { return persist.EmbeddingType(promotedCopy) }

// PromotedTo returns, by method id, the types that get each of methods
// through an embedded field. See persist.PromotedTo.
func PromotedTo(r Reader, methods []types.Node) (map[string][]string, error) {
	return persist.PromotedTo(r, methods)
}

// BlankName is the name of a declaration that binds nothing (`var _ = x`).
const BlankName = persist.BlankName

// The keys a tool adds to its answer when the caller named the copy of a
// promoted method; NameResolution.SeedNote is the text of the second.
const (
	SeedKeyPromotedFrom = persist.SeedKeyPromotedFrom
	SeedKeyNote         = persist.SeedKeyNote
)

// PRRef is the public alias for the build-time-derived PR breadcrumb
// stored per node. External consumers (cks, ckv) read this via
// Reader.GetNodePRs; see pkg/types.PRRef for the field semantics and
// the temporal-slicing contract.
type PRRef = types.PRRef

// Review is the production-code, history and test view of one function
// returned by Reader.Review (schema 1.24). ReviewOptions bounds it; see
// persist.Review for the field semantics.
type (
	Review         = persist.Review
	ReviewOptions  = persist.ReviewOptions
	ReviewSymbol   = persist.ReviewSymbol
	ReviewCallSite = persist.ReviewCallSite
	ReviewCommit   = persist.ReviewCommit
)

// ErrNoReviewIndex is returned by Reader.Review for graphs built before
// schema 1.24; ErrReviewSeedNotFound when no production function or method
// has the requested qualified name.
var (
	ErrNoReviewIndex      = persist.ErrNoReviewIndex
	ErrReviewSeedNotFound = persist.ErrReviewSeedNotFound
)

// ErrInvalidMetric is returned by Reader.TopNodes when the metric
// argument is not one of the supported column names. HTTP layers
// typically map this to 400.
var ErrInvalidMetric = persist.ErrInvalidMetric

// Manifest is the public, minimal snapshot of build-time metadata an
// external consumer needs. Compared to the internal
// persist.Manifest, this type deliberately drops incremental-cache
// fields (SrcRoot, Files, StalenessFiles, StalenessMTimeSum, …) — those
// rotate with build-pipeline changes and external consumers must not
// depend on them.
//
// Use cases (mirroring what the cks consumer needs):
//   - CommitHash drives Citation.CommitHash drift detection
//   - SchemaVersion gates compatibility in cks.ops.health
//   - IndexTimestamp shows freshness in user-facing status output
//
// Adding a field here is a breaking change for external consumers —
// it forces them to widen their struct in lockstep. If a new field is
// truly needed by every consumer, add it; if only one consumer needs
// it, prefer exposing it through a dedicated Reader method instead.
type Manifest struct {
	CommitHash     string // source commit at index time; empty when build did not record one
	SchemaVersion  string // ckg schema version, e.g. "1.9"
	IndexTimestamp string // RFC3339 timestamp of the build
}

// OpenReadOnly opens a graph DB at path for read-only access. The
// returned Reader must be closed by the caller via Reader.Close().
func OpenReadOnly(path string) (Reader, error) {
	return persist.OpenReadOnly(path)
}

// GetManifest reads the build-time manifest from r and projects it
// onto the minimal public Manifest. External consumers should use
// this in preference to Reader.GetManifest, which returns the full
// internal struct (whose field set is not part of the public API).
//
// The projection is intentionally one-way: there is no inverse
// operation. If a consumer needs richer metadata, the right move is
// to widen Manifest here (a breaking change with deliberate review)
// rather than to read internal fields by other means.
func GetManifest(r Reader) (Manifest, error) {
	m, err := r.GetManifest()
	if err != nil {
		return Manifest{}, err
	}
	return projectManifest(m), nil
}

// projectManifest is the single source of truth for the internal →
// public Manifest mapping. Extracted so the field mapping is
// unit-testable without standing up a real graph fixture.
func projectManifest(m persist.Manifest) Manifest {
	return Manifest{
		CommitHash:     m.SrcCommit,
		SchemaVersion:  m.SchemaVersion,
		IndexTimestamp: m.BuildTimestamp,
	}
}
