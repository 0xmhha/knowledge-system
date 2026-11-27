package validate

import (
	"context"

	"github.com/0xmhha/knowledge-system/internal/graph/graph"
	"github.com/0xmhha/knowledge-system/internal/graph/persist"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// SchemaValidator runs deterministic structural checks on a graph:
//
//   - Empty required fields (id, name, qualified_name, file_path where it
//     applies, confidence). Empty values on a node mean a parser branch
//     emitted a placeholder it never finished filling.
//   - Dangling edges (src or dst not present in node set). Reuses
//     graph.Inspect so the rule definition lives in one place.
//   - Unknown node/edge types. Schema bumps are catastrophic if a parser
//     emits a type the persist layer doesn't recognise.
//   - Edge-type semantic invariants (V1 baseline):
//   - implements src must be a Struct or TypeAlias, dst must be Interface
//   - listens_on src must be Function/Method, dst must be Endpoint
//   - calls/invokes src and dst must be Function/Method
//   - reverts_with src must be Function/Modifier, dst must be Error
//
// The validator is deterministic, fast, and dependency-free — safe to run
// on every build. Findings inform Citation Enforcement and the LLM
// validator (which uses these as priors).
type SchemaValidator struct{}

// NewSchemaValidator returns a stateless schema validator instance.
func NewSchemaValidator() *SchemaValidator { return &SchemaValidator{} }

// Name returns the validator identifier.
func (v *SchemaValidator) Name() string { return "schema" }

// fileScopedTypes are node types that MUST have a non-empty file_path. The
// remaining "global" types (Package, Commit, Endpoint, MessageType, Mutex,
// Channel) live above any single file and are exempt.
var fileScopedTypes = map[types.NodeType]struct{}{
	types.NodeFile:       {},
	types.NodeStruct:     {},
	types.NodeInterface:  {},
	types.NodeFunction:   {},
	types.NodeMethod:     {},
	types.NodeField:      {},
	types.NodeVariable:   {},
	types.NodeConstant:   {},
	types.NodeContract:   {},
	types.NodeImport:     {},
	types.NodeCallSite:   {},
	types.NodeIfStmt:     {},
	types.NodeLoopStmt:   {},
	types.NodeSwitchStmt: {},
	types.NodeReturnStmt: {},
	types.NodeError:      {},
}

// Validate executes all schema checks and returns one report aggregating
// every issue found. ctx and store are accepted for interface conformance;
// SchemaValidator does not block on either.
func (v *SchemaValidator) Validate(ctx context.Context, g *graph.Graph, store persist.StoreReader) (*Report, error) {
	report := &Report{Validator: v.Name()}
	if g == nil {
		return report, nil
	}

	// Build a node-id → node lookup once; downstream checks query it.
	byID := make(map[string]*types.Node, len(g.Nodes))
	for i := range g.Nodes {
		n := &g.Nodes[i]
		byID[n.ID] = n
	}

	// (1) Empty required fields per node.
	for i := range g.Nodes {
		n := &g.Nodes[i]
		if n.ID == "" {
			report.Issues = append(report.Issues, Issue{
				Severity: SeverityError, Code: "empty-id",
				Message: "node has empty ID",
			})
		}
		if n.Name == "" {
			report.Issues = append(report.Issues, Issue{
				Severity: SeverityError, Code: "empty-name",
				Message: "node has empty Name", NodeID: n.ID, FilePath: n.FilePath,
			})
		}
		if n.QualifiedName == "" {
			report.Issues = append(report.Issues, Issue{
				Severity: SeverityError, Code: "empty-qname",
				Message: "node has empty QualifiedName", NodeID: n.ID, FilePath: n.FilePath,
			})
		}
		if !n.Confidence.Valid() {
			report.Issues = append(report.Issues, Issue{
				Severity: SeverityError, Code: "invalid-confidence",
				Message: "node has invalid Confidence " + string(n.Confidence),
				NodeID:  n.ID, FilePath: n.FilePath,
			})
		}
		if _, scoped := fileScopedTypes[n.Type]; scoped && n.FilePath == "" {
			report.Issues = append(report.Issues, Issue{
				Severity: SeverityError, Code: "empty-file-path",
				Message: "file-scoped node has empty FilePath (type=" + string(n.Type) + ")",
				NodeID:  n.ID,
			})
		}
	}

	// (2) Dangling edges + unknown types — delegate to graph.Inspect so
	// the rule lives in one place. Convert each dangling case to an
	// Issue with code "dangling-src" / "dangling-dst".
	gReport := graph.Inspect(g)
	for _, schemaErr := range gReport.SchemaErrors {
		report.Issues = append(report.Issues, Issue{
			Severity: SeverityError, Code: "schema-error",
			Message: schemaErr.Error(),
		})
	}
	for _, d := range gReport.DanglingEdges {
		key := edgeKey(string(d.Edge.Type), d.Edge.Src, d.Edge.Dst, d.Edge.Line)
		if d.Src {
			report.Issues = append(report.Issues, Issue{
				Severity: SeverityError, Code: "dangling-src",
				Message: "edge src does not match any node",
				EdgeKey: key, FilePath: d.Edge.FilePath,
			})
		}
		if d.Dst {
			report.Issues = append(report.Issues, Issue{
				Severity: SeverityError, Code: "dangling-dst",
				Message: "edge dst does not match any node",
				EdgeKey: key, FilePath: d.Edge.FilePath,
			})
		}
	}

	// (3) Edge-type semantic invariants. Only fire when both endpoints are
	// resolvable (otherwise the dangling rules above already covered it).
	for _, e := range g.Edges {
		src, srcOK := byID[e.Src]
		dst, dstOK := byID[e.Dst]
		if !srcOK || !dstOK {
			continue
		}
		switch e.Type {
		case types.EdgeImplements:
			if dst.Type != types.NodeInterface {
				report.Issues = append(report.Issues, Issue{
					Severity: SeverityWarning, Code: "implements-bad-dst",
					Message:  "implements edge dst is not an Interface (got " + string(dst.Type) + ")",
					EdgeKey:  edgeKey(string(e.Type), e.Src, e.Dst, e.Line),
					FilePath: e.FilePath,
				})
			}
		case types.EdgeListensOn:
			if src.Type != types.NodeFunction && src.Type != types.NodeMethod {
				report.Issues = append(report.Issues, Issue{
					Severity: SeverityWarning, Code: "listens-on-bad-src",
					Message:  "listens_on src is not a Function/Method (got " + string(src.Type) + ")",
					EdgeKey:  edgeKey(string(e.Type), e.Src, e.Dst, e.Line),
					FilePath: e.FilePath,
				})
			}
			if dst.Type != types.NodeEndpoint {
				report.Issues = append(report.Issues, Issue{
					Severity: SeverityWarning, Code: "listens-on-bad-dst",
					Message:  "listens_on dst is not an Endpoint (got " + string(dst.Type) + ")",
					EdgeKey:  edgeKey(string(e.Type), e.Src, e.Dst, e.Line),
					FilePath: e.FilePath,
				})
			}
		case types.EdgeHTTPCalls:
			// Schema 1.9: http_calls always points Func/Method →
			// Endpoint (real or AMBIGUOUS placeholder). The matcher passes
			// through with the same shape so the validation is unchanged.
			if src.Type != types.NodeFunction && src.Type != types.NodeMethod {
				report.Issues = append(report.Issues, Issue{
					Severity: SeverityWarning, Code: "http-calls-bad-src",
					Message:  "http_calls src is not a Function/Method (got " + string(src.Type) + ")",
					EdgeKey:  edgeKey(string(e.Type), e.Src, e.Dst, e.Line),
					FilePath: e.FilePath,
				})
			}
			if dst.Type != types.NodeEndpoint {
				report.Issues = append(report.Issues, Issue{
					Severity: SeverityWarning, Code: "http-calls-bad-dst",
					Message:  "http_calls dst is not an Endpoint (got " + string(dst.Type) + ")",
					EdgeKey:  edgeKey(string(e.Type), e.Src, e.Dst, e.Line),
					FilePath: e.FilePath,
				})
			}
		case types.EdgeGRPCListensOn:
			// Schema 1.9: grpc_listens_on points server-impl Method →
			// Endpoint (`grpc:Service.Method`). Free functions cannot listen
			// on gRPC RPCs — a gRPC server impl is always a method on a
			// receiver registered via pb.RegisterXXXServer.
			if src.Type != types.NodeMethod {
				report.Issues = append(report.Issues, Issue{
					Severity: SeverityWarning, Code: "grpc-listens-on-bad-src",
					Message:  "grpc_listens_on src is not a Method (got " + string(src.Type) + ")",
					EdgeKey:  edgeKey(string(e.Type), e.Src, e.Dst, e.Line),
					FilePath: e.FilePath,
				})
			}
			if dst.Type != types.NodeEndpoint {
				report.Issues = append(report.Issues, Issue{
					Severity: SeverityWarning, Code: "grpc-listens-on-bad-dst",
					Message:  "grpc_listens_on dst is not an Endpoint (got " + string(dst.Type) + ")",
					EdgeKey:  edgeKey(string(e.Type), e.Src, e.Dst, e.Line),
					FilePath: e.FilePath,
				})
			}
		case types.EdgeGRPCCalls:
			// Schema 1.9: grpc_calls points caller Function/Method →
			// Endpoint (real grpc Endpoint or AMBIGUOUS placeholder for
			// unresolved external services). Mirror http_calls shape.
			if src.Type != types.NodeFunction && src.Type != types.NodeMethod {
				report.Issues = append(report.Issues, Issue{
					Severity: SeverityWarning, Code: "grpc-calls-bad-src",
					Message:  "grpc_calls src is not a Function/Method (got " + string(src.Type) + ")",
					EdgeKey:  edgeKey(string(e.Type), e.Src, e.Dst, e.Line),
					FilePath: e.FilePath,
				})
			}
			if dst.Type != types.NodeEndpoint {
				report.Issues = append(report.Issues, Issue{
					Severity: SeverityWarning, Code: "grpc-calls-bad-dst",
					Message:  "grpc_calls dst is not an Endpoint (got " + string(dst.Type) + ")",
					EdgeKey:  edgeKey(string(e.Type), e.Src, e.Dst, e.Line),
					FilePath: e.FilePath,
				})
			}
		case types.EdgeCalls, types.EdgeInvokes:
			if !isCaller(src.Type) {
				report.Issues = append(report.Issues, Issue{
					Severity: SeverityWarning, Code: "calls-bad-src",
					Message:  "calls/invokes src is not a caller kind (got " + string(src.Type) + ")",
					EdgeKey:  edgeKey(string(e.Type), e.Src, e.Dst, e.Line),
					FilePath: e.FilePath,
				})
			}
			// Schema 1.36: a call to a symbol outside the build ends at an
			// External node, which never starts one. Schema 1.39: a caller
			// that calls a closure or a function value carries a marker, an
			// invokes edge to itself with the dispatch kind; it names no
			// callee, so a Variable may carry one for its initialiser.
			if !isCallable(dst.Type) && dst.Type != types.NodeExternal && !isDispatchMarker(e) {
				report.Issues = append(report.Issues, Issue{
					Severity: SeverityWarning, Code: "calls-bad-dst",
					Message:  "calls/invokes dst is not a callable kind (got " + string(dst.Type) + ")",
					EdgeKey:  edgeKey(string(e.Type), e.Src, e.Dst, e.Line),
					FilePath: e.FilePath,
				})
			}
		case types.EdgeLiteralPassedTo:
			// Schema 1.42: a call site in a function literal -> the function
			// the literal is passed to.
			if src.Type != types.NodeCallSite || !isCallable(dst.Type) {
				report.Issues = append(report.Issues, Issue{
					Severity: SeverityWarning, Code: "literal-passed-to-bad-shape",
					Message:  "literal_passed_to must go from a CallSite to a callable (got " + string(src.Type) + " -> " + string(dst.Type) + ")",
					EdgeKey:  edgeKey(string(e.Type), e.Src, e.Dst, e.Line),
					FilePath: e.FilePath,
				})
			}
		case types.EdgeHandles, types.EdgeTriggers:
			// Schema 1.42: handles starts at a receive's CallSite and ends at
			// the handled code; triggers starts at a sender and ends at a
			// receive's CallSite.
			if bad := messageLinkShape(e.Type, src.Type, dst.Type); bad != "" {
				report.Issues = append(report.Issues, Issue{
					Severity: SeverityWarning, Code: string(e.Type) + "-bad-shape",
					Message:  string(e.Type) + " " + bad,
					EdgeKey:  edgeKey(string(e.Type), e.Src, e.Dst, e.Line),
					FilePath: e.FilePath,
				})
			}
		case types.EdgeRevertsWith:
			// Schema 1.38: a Solidity function, modifier or constructor
			// (a Function with sub kind constructor) reverts with a
			// custom error.
			if src.Type != types.NodeFunction && src.Type != types.NodeModifier {
				report.Issues = append(report.Issues, Issue{
					Severity: SeverityWarning, Code: "reverts-with-bad-src",
					Message:  "reverts_with src is not a Function/Modifier (got " + string(src.Type) + ")",
					EdgeKey:  edgeKey(string(e.Type), e.Src, e.Dst, e.Line),
					FilePath: e.FilePath,
				})
			}
			if dst.Type != types.NodeError {
				report.Issues = append(report.Issues, Issue{
					Severity: SeverityWarning, Code: "reverts-with-bad-dst",
					Message:  "reverts_with dst is not an Error (got " + string(dst.Type) + ")",
					EdgeKey:  edgeKey(string(e.Type), e.Src, e.Dst, e.Line),
					FilePath: e.FilePath,
				})
			}
		case types.EdgeTimeoutPath:
			if !isContextPathShape(src, dst, e) {
				report.Issues = append(report.Issues, Issue{
					Severity: SeverityWarning, Code: "timeout-path-bad-shape",
					Message: "timeout_path edge must be a Function/Method self-loop (got src=" +
						string(src.Type) + ", dst=" + string(dst.Type) + ", self=" + boolStr(e.Src == e.Dst) + ")",
					EdgeKey:  edgeKey(string(e.Type), e.Src, e.Dst, e.Line),
					FilePath: e.FilePath,
				})
			}
		case types.EdgeCancellationPath:
			if !isContextPathShape(src, dst, e) {
				report.Issues = append(report.Issues, Issue{
					Severity: SeverityWarning, Code: "cancellation-path-bad-shape",
					Message: "cancellation_path edge must be a Function/Method self-loop (got src=" +
						string(src.Type) + ", dst=" + string(dst.Type) + ", self=" + boolStr(e.Src == e.Dst) + ")",
					EdgeKey:  edgeKey(string(e.Type), e.Src, e.Dst, e.Line),
					FilePath: e.FilePath,
				})
			}
		}
	}

	_ = ctx
	_ = store
	return report, nil
}

// isCaller reports whether t is a node type that can start a calls/invokes
// edge: a callable kind, or a declaration whose own code makes calls without
// being called itself. Those are a Variable, whose package-level initialiser
// makes calls outside any function (`var Default = New().Inc()`; schema
// 1.38), a Solidity Modifier, whose body makes calls (`modifier onlyOwner()
// { _checkOwner(); _; }`), and a Solidity state variable or constant (Field,
// Constant), whose initialiser can (`uint256 public cap = _defaultCap();`).
// None of them ever ends a calls edge.
func isCaller(t types.NodeType) bool {
	switch t {
	case types.NodeVariable, types.NodeModifier, types.NodeField, types.NodeConstant:
		return true
	case types.NodeGoroutine:
		// Schema 1.42: a goroutine calls the function it runs and the
		// functions its literal calls (dispatch kind goroutine).
		return true
	}
	return isCallable(t)
}

// messageLinkShape returns what is wrong with a handles or triggers edge
// from a src node to a dst node, or "".
func messageLinkShape(t types.EdgeType, src, dst types.NodeType) string {
	if t == types.EdgeHandles {
		switch {
		case src != types.NodeCallSite:
			return "src is not a CallSite (got " + string(src) + ")"
		case dst != types.NodeCallSite && dst != types.NodeField && dst != types.NodeGoroutine:
			return "dst is not a CallSite, Field or Goroutine (got " + string(dst) + ")"
		}
		return ""
	}
	switch {
	case src != types.NodeFunction && src != types.NodeMethod && src != types.NodeGoroutine:
		return "src is not a Function, Method or Goroutine (got " + string(src) + ")"
	case dst != types.NodeCallSite:
		return "dst is not a CallSite (got " + string(dst) + ")"
	}
	return ""
}

// isDispatchMarker reports whether e is the marker of a dynamic call: an
// invokes edge from a node to itself that carries the dispatch kind.
func isDispatchMarker(e types.Edge) bool {
	return e.Type == types.EdgeInvokes && e.Src == e.Dst && e.DispatchKind != ""
}

// isCallable reports whether t is a node type that legitimately appears
// at the callee end of a calls/invokes edge, and, with isCaller, at its start.
func isCallable(t types.NodeType) bool {
	switch t {
	case types.NodeFunction, types.NodeMethod, types.NodeCallSite:
		return true
	}
	return false
}

// isContextPathShape enforces the timeout_path / cancellation_path
// invariants: both endpoints must be the same Function or Method
// node, i.e. a self-loop anchored on the enclosing function. Used by
// SchemaValidator to surface emitter regressions early.
func isContextPathShape(src, dst *types.Node, e types.Edge) bool {
	if src == nil || dst == nil {
		return false
	}
	if e.Src != e.Dst {
		return false
	}
	switch src.Type {
	case types.NodeFunction, types.NodeMethod:
		return true
	}
	return false
}

// boolStr returns "true"/"false" — a tiny helper so error messages don't
// import strconv just to format a single bool. Kept local to schema.go.
func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
