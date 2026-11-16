// Package contract holds the records a language frontend emits
// (docs/graph/SYMBOL-CONTRACT-DESIGN.md section 4): declarations, references
// between them, references the frontend could not resolve, and the layer and
// confidence values they carry.
//
// The package depends only on pkg/graph/types, so frontends and the core can
// both import it. Increment 1 of the migration (design section 9.1) fills the
// records from the Go pipeline's resolved graph and turns them back; the
// stored graph does not change. Later increments make the records the
// frontend's own output.
package contract

import (
	"strings"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// Layer separates the code a query is about from code that only tests it,
// code of outside modules and history (decision D6).
type Layer string

const (
	LayerProduction Layer = "production"
	LayerTest       Layer = "test"
	LayerOutside    Layer = "outside"
	LayerHistory    Layer = "history"
)

// Confidence says how a reference is known.
type Confidence string

const (
	// Certain: the compiler or the linker resolved it.
	Certain Confidence = "certain"
	// StaticMay: an analysis estimates that it can happen (VTA, pattern
	// matches on concurrency and topology).
	StaticMay Confidence = "static_may"
	// Observed: seen in a trace of a test run.
	Observed Confidence = "observed"
	// Unbound: a placeholder for a target that a later pass binds, such as
	// the endpoint a Go HTTP or gRPC client calls before the cross-language
	// linker finds the server (stored as AMBIGUOUS). Increment 2 of design
	// section 9.1 decides whether placeholders become Unresolved records.
	Unbound Confidence = "unbound"
)

// ConfidenceOf maps the stored labels to contract values.
func ConfidenceOf(c types.Confidence) (Confidence, bool) {
	switch c {
	case types.ConfExtracted:
		return Certain, true
	case types.ConfInferred:
		return StaticMay, true
	case types.ConfAmbiguous:
		return Unbound, true
	}
	return "", false
}

// Stored maps a contract value back to the stored label. Observed has no
// stored label yet and is stored as INFERRED.
func (c Confidence) Stored() types.Confidence {
	switch c {
	case Certain:
		return types.ConfExtracted
	case Unbound:
		return types.ConfAmbiguous
	}
	return types.ConfInferred
}

// Kind is the common declaration kind the core and the query engine use; the
// language's own kind stays in Decl.LangKind.
type Kind string

const (
	KindModule      Kind = "module"
	KindFile        Kind = "file"
	KindType        Kind = "type"
	KindInterface   Kind = "interface"
	KindCallable    Kind = "callable"
	KindField       Kind = "field"
	KindVariable    Kind = "variable"
	KindConstant    Kind = "constant"
	KindMessage     Kind = "message"
	KindService     Kind = "service"
	KindStatement   Kind = "statement"
	KindConcurrency Kind = "concurrency"
	KindOther       Kind = "other"
)

// KindOf returns the common kind of a stored node type.
func KindOf(t types.NodeType) Kind {
	switch t {
	case types.NodePackage:
		return KindModule
	case types.NodeFile:
		return KindFile
	case types.NodeStruct, types.NodeClass, types.NodeTypeAlias, types.NodeEnum, types.NodeContract:
		return KindType
	case types.NodeInterface:
		return KindInterface
	case types.NodeFunction, types.NodeMethod, types.NodeModifier, types.NodeConstructor:
		return KindCallable
	case types.NodeField:
		return KindField
	case types.NodeVariable, types.NodeLocalVariable, types.NodeParameter, types.NodeMapping:
		return KindVariable
	case types.NodeConstant:
		return KindConstant
	case types.NodeMessageType, types.NodeEvent:
		return KindMessage
	case types.NodeEndpoint:
		return KindService
	case types.NodeIfStmt, types.NodeLoopStmt, types.NodeCallSite, types.NodeReturnStmt, types.NodeSwitchStmt:
		return KindStatement
	case types.NodeGoroutine, types.NodeChannel, types.NodeMutex:
		return KindConcurrency
	}
	return KindOther
}

// Span is a declaration's position in its file.
type Span struct {
	StartLine, EndLine int
	StartByte, EndByte int
}

// Decl is one declaration.
type Decl struct {
	// NodeID is the store key. Frontends take it from parse.NodeID today; the
	// core makes it from the symbol id once frontends emit records (increment
	// 4 of design section 9.1).
	NodeID string
	// SymbolID is "<language>:<compiler identity>", the cross-snapshot
	// identity (design section 4.1); empty when the frontend has no compiler
	// identity for the declaration (statements, AST-only builds).
	SymbolID      string
	Language      string
	Kind          Kind
	LangKind      types.NodeType
	SubKind       string
	Name          string
	QualifiedName string
	File          string
	Span          Span
	Layer         Layer
	Signature     string
	Doc           string
	Visibility    string
	Complexity    int
	Confidence    Confidence
}

// Ref is one reference from a declaration to another.
type Ref struct {
	// From and To are node ids until the core owns identity (increment 4).
	From, To   string
	Kind       types.EdgeType
	File       string
	Line       int
	Count      int
	Confidence Confidence
	// DispatchKind and Order keep the stored edge attributes of the same
	// names (an invokes edge's dispatch mechanism, a modifier's position).
	DispatchKind string
	Order        int
}

// Unresolved is a reference the frontend could not bind to a declaration of
// the graph.
type Unresolved struct {
	From   string
	Target string
	File   string
	Line   int
	Reason string
}

// Unit is what a frontend returns for one language and one configuration.
type Unit struct {
	Language   string
	Decls      []Decl
	Refs       []Ref
	Unresolved []Unresolved
	// UnresolvedCounts and UnresolvedSamples carry the per-reason counts the
	// Go Resolve keeps today; increment 2 replaces them with Unresolved
	// records.
	UnresolvedCounts  map[string]int
	UnresolvedSamples map[string][]string
	// LoadErrors is the number of errors the toolchain reported while
	// loading the sources; records of such a unit can miss references.
	LoadErrors int
}

// SymbolID joins a language and a compiler identity; empty identity gives an
// empty id.
func SymbolID(language, identity string) string {
	if identity == "" {
		return ""
	}
	return language + ":" + identity
}

// Identity is the language-specific part of a symbol id, the value stored as
// canonical_id.
func Identity(language, symbolID string) string {
	return strings.TrimPrefix(symbolID, language+":")
}
