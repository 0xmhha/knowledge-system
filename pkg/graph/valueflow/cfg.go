// Package valueflow owns bounded on-demand Go value/control analysis over
// retained function source. Computation is shared by graph MCP and consumers.
package valueflow

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"time"

	"golang.org/x/tools/go/cfg"
)

type Source struct {
	NodeID      string `json:"node_id"`
	CanonicalID string `json:"canonical_id,omitempty"`
	File        string `json:"file"`
	Commit      string `json:"commit,omitempty"`
	SHA256      string `json:"sha256"`
	Language    string `json:"language"`
	StartLine   int    `json:"start_line"`
	StartByte   int    `json:"start_byte"`
	Bytes       int    `json:"bytes"`
}
type Options struct {
	MaxSourceBytes int `json:"max_source_bytes"`
	MaxASTNodes    int `json:"max_ast_nodes"`
	MaxBlocks      int `json:"max_blocks"`
	MaxNodes       int `json:"max_nodes"`
	MaxEdges       int `json:"max_edges"`
	MaxIterations  int `json:"max_iterations"`
	TimeoutMS      int `json:"timeout_ms"`
}
type Location struct {
	StartByte int `json:"start_byte"`
	EndByte   int `json:"end_byte"`
	StartLine int `json:"start_line"`
	EndLine   int `json:"end_line"`
}
type Node struct {
	ID        string   `json:"id"`
	Block     int      `json:"block"`
	Kind      string   `json:"kind"`
	Location  Location `json:"location"`
	Text      string   `json:"text"`
	Synthetic bool     `json:"synthetic"`
}
type Block struct {
	ID        int      `json:"id"`
	Kind      string   `json:"kind"`
	Reachable bool     `json:"reachable"`
	Nodes     []string `json:"nodes"`
}
type Edge struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Kind      string `json:"kind"`
	Condition string `json:"condition,omitempty"`
	Certainty string `json:"certainty"`
}
type Diagnostic struct {
	Code     string    `json:"code"`
	Message  string    `json:"message"`
	Location *Location `json:"location,omitempty"`
}
type Result struct {
	SchemaVersion int               `json:"schema_version"`
	Source        Source            `json:"source"`
	Limits        Options           `json:"limits"`
	Status        string            `json:"status"`
	Capability    map[string]string `json:"capability"`
	Blocks        []Block           `json:"blocks"`
	Nodes         []Node            `json:"nodes"`
	Edges         []Edge            `json:"edges"`
	Diagnostics   []Diagnostic      `json:"diagnostics"`
	Truncated     bool              `json:"truncated"`
	ElapsedMicros int64             `json:"elapsed_micros"`
}

type analysis struct {
	result   Result
	fs       *token.FileSet
	fn       *ast.FuncDecl
	graph    *cfg.CFG
	astNodes map[string]ast.Node
	prefix   int
	blob     []byte
	context  context.Context
}

func limits(o Options) (Options, error) {
	defaults := Options{256 * 1024, 4096, 256, 1024, 4096, 128, 1000}
	requested := []*int{&o.MaxSourceBytes, &o.MaxASTNodes, &o.MaxBlocks, &o.MaxNodes, &o.MaxEdges, &o.MaxIterations, &o.TimeoutMS}
	caps := []int{defaults.MaxSourceBytes, defaults.MaxASTNodes, defaults.MaxBlocks, defaults.MaxNodes, defaults.MaxEdges, defaults.MaxIterations, defaults.TimeoutMS}
	for i, value := range requested {
		if *value == 0 {
			*value = caps[i]
		}
		if *value < 1 || *value > caps[i] {
			return o, fmt.Errorf("limit outside supported bounds")
		}
	}
	return o, nil
}

// BuildCFG reports executable successors, not an AST ordering masquerading as
// control flow. Value/def-use/control dependence is deliberately not yet
// advertised by this first kernel; later analysis stages must compute it.
func BuildCFG(ctx context.Context, source Source, blob []byte, options Options) (out Result, err error) {
	started := time.Now()
	defer func() { out.ElapsedMicros = time.Since(started).Microseconds() }()
	options, err = limits(options)
	if err != nil {
		return out, err
	}
	hash := sha256.Sum256(blob)
	source.SHA256 = hex.EncodeToString(hash[:])
	source.Bytes = len(blob)
	out = Result{SchemaVersion: 1, Source: source, Limits: options, Status: "partial", Capability: map[string]string{"cfg": "bounded_go", "def_use": "not_computed", "data_dependence": "not_computed", "control_dependence": "not_computed", "package_types": "unavailable_function_blob", "memory_alias": "unsupported", "exceptions": "unsupported", "short_circuit": "unsupported"}, Blocks: []Block{}, Nodes: []Node{}, Edges: []Edge{}, Diagnostics: []Diagnostic{}}
	diagnostic := func(code, message string) {
		out.Diagnostics = append(out.Diagnostics, Diagnostic{Code: code, Message: message})
	}
	if source.Language != "go" {
		out.Status = "unsupported"
		out.Capability["cfg"] = "unsupported_language"
		diagnostic("unsupported_language", "Only retained Go function source is supported")
		return out, nil
	}
	if source.NodeID == "" || source.File == "" || source.StartLine < 1 || source.StartByte < 0 {
		return out, fmt.Errorf("retained source identity/location required")
	}
	if source.Commit == "" {
		diagnostic("revision_unavailable", "Snapshot source commit is unavailable; no live checkout revision is inferred")
	}
	exhausted := func(message string) (Result, error) {
		out.Status = "budget_exhausted"
		out.Truncated = true
		diagnostic("budget_exhausted", message)
		return out, nil
	}
	if len(blob) > options.MaxSourceBytes {
		return exhausted("Retained source byte limit exceeded")
	}
	if ctx == nil {
		return out, fmt.Errorf("context required")
	}
	bounded, cancel := context.WithTimeout(ctx, time.Duration(options.TimeoutMS)*time.Millisecond)
	defer cancel()
	if bounded.Err() != nil {
		return exhausted("Analysis context already ended")
	}
	prefix := "package __valueflow\n"
	fs := token.NewFileSet()
	file, parseError := parser.ParseFile(fs, source.File, prefix+string(blob), 0)
	if parseError != nil {
		out.Status = "unsupported"
		diagnostic("invalid_function_source", parseError.Error())
		return out, nil
	}
	if len(file.Decls) != 1 {
		out.Status = "unsupported"
		diagnostic("function_blob_required", "Exactly one retained function declaration is required")
		return out, nil
	}
	fn, ok := file.Decls[0].(*ast.FuncDecl)
	if !ok || fn.Body == nil {
		out.Status = "unsupported"
		diagnostic("function_body_required", "A function/method body is required")
		return out, nil
	}
	count := 0
	ast.Inspect(fn, func(node ast.Node) bool {
		if node != nil {
			count++
		}
		return count <= options.MaxASTNodes
	})
	if count > options.MaxASTNodes {
		return exhausted("AST size limit exceeded before CFG construction")
	}
	if bounded.Err() != nil {
		return exhausted("Analysis deadline ended after parsing")
	}
	// Unknown callees may return. Never infer os.Exit/log.Fatal or an unresolved
	// identifier named panic as a proven no-return call from an isolated blob.
	graph := cfg.New(fn.Body, func(*ast.CallExpr) bool { return true })
	if len(graph.Blocks) > options.MaxBlocks {
		return exhausted("CFG block limit exceeded")
	}
	a := analysis{result: out, fs: fs, fn: fn, graph: graph, prefix: len(prefix), blob: blob, context: bounded, astNodes: map[string]ast.Node{}}
	conditions := map[ast.Expr]bool{}
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		switch value := node.(type) {
		case *ast.IfStmt:
			conditions[value.Cond] = true
		case *ast.ForStmt:
			if value.Cond != nil {
				conditions[value.Cond] = true
			}
		}
		return true
	})
	for _, block := range graph.Blocks {
		if bounded.Err() != nil {
			out = a.result
			return exhausted("CFG extraction deadline ended")
		}
		b := Block{ID: int(block.Index), Kind: block.Kind.String(), Reachable: block.Live, Nodes: []string{}}
		for index, node := range block.Nodes {
			if len(a.result.Nodes) >= options.MaxNodes {
				out = a.result
				return exhausted("CFG node limit exceeded")
			}
			id := fmt.Sprintf("b%d/n%d", block.Index, index)
			location := a.loc(node)
			begin, end := location.StartByte-source.StartByte, location.EndByte-source.StartByte
			kind := strings.TrimPrefix(fmt.Sprintf("%T", node), "*ast.")
			synthetic := false
			if ret, ok := node.(*ast.ReturnStmt); ok && ret.Pos() == fn.Body.Rbrace {
				synthetic = true
			}
			a.result.Nodes = append(a.result.Nodes, Node{ID: id, Block: b.ID, Kind: kind, Location: location, Text: string(blob[begin:end]), Synthetic: synthetic})
			a.astNodes[id] = node
			b.Nodes = append(b.Nodes, id)
		}
		a.result.Blocks = append(a.result.Blocks, b)
		for index, successor := range block.Succs {
			if len(a.result.Edges) >= options.MaxEdges {
				out = a.result
				return exhausted("CFG edge limit exceeded")
			}
			condition := ""
			certainty := "structural"
			if len(block.Succs) == 2 {
				condition = "branch_" + fmt.Sprint(index)
				certainty = "conservative"
				if len(block.Nodes) > 0 {
					if expression, ok := block.Nodes[len(block.Nodes)-1].(ast.Expr); ok && conditions[expression] {
						condition = []string{"true", "false"}[index]
					}
				}
			}
			a.result.Edges = append(a.result.Edges, Edge{From: fmt.Sprintf("b%d", block.Index), To: fmt.Sprintf("b%d", successor.Index), Kind: "cfg", Condition: condition, Certainty: certainty})
		}
	}
	out = a.result
	// An explicit partial result prevents absent dependence edges being read as
	// safe. CFG cannot represent short-circuit/exception semantics; next stages
	// retain diagnostics rather than promising whole-program precision.
	diagnostic("dependence_not_computed", "This CFG kernel has not computed def-use or data/control dependence")
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		var code, message string
		switch value := node.(type) {
		case *ast.FuncLit:
			code = "closure_unsupported"
			message = "Captured variables and closure calls require conservative analysis"
		case *ast.GoStmt, *ast.DeferStmt:
			code = "async_or_deferred_effect"
			message = "Deferred/concurrent effects are not ordinary sequential CFG semantics"
		case *ast.SelectStmt:
			code = "select_unsupported"
			message = "Scheduling/select alternatives are not proven control dependence"
		case *ast.BinaryExpr:
			if value.Op == token.LAND || value.Op == token.LOR {
				code = "short_circuit_unsupported"
				message = "CFG does not model short-circuit expression edges"
			}
		case *ast.CallExpr:
			code = "opaque_call"
			message = "Isolated function source has no interprocedural effects or abnormal-exit proof"
		}
		if code != "" {
			location := a.loc(node)
			out.Diagnostics = append(out.Diagnostics, Diagnostic{Code: code, Message: message, Location: &location})
		}
		return true
	})
	return out, nil
}
func (a *analysis) loc(node ast.Node) Location {
	endPos := node.End()
	if file := a.fs.File(node.Pos()); file != nil && endPos > file.Pos(file.Size()) {
		endPos = file.Pos(file.Size())
	}
	begin, end := a.fs.Position(node.Pos()), a.fs.Position(endPos)
	start, finish := begin.Offset-a.prefix, end.Offset-a.prefix
	start = max(0, min(start, len(a.blob)))
	finish = max(start, min(finish, len(a.blob)))
	return Location{StartByte: a.result.Source.StartByte + start, EndByte: a.result.Source.StartByte + finish, StartLine: a.result.Source.StartLine + begin.Line - 2, EndLine: a.result.Source.StartLine + end.Line - 2}
}
