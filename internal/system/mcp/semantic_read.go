package mcp

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/0xmhha/knowledge-system/internal/system/composer/sanitize"
	"github.com/0xmhha/knowledge-system/internal/system/semantic"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

var (
	ToolNameSemanticLookupTerm       = toolName("semantic.lookup_term")
	ToolNameSemanticGetConcept       = toolName("semantic.get_concept")
	ToolNameSemanticTraceRequirement = toolName("semantic.trace_requirement")
)

type semanticReadResponse[T any] struct {
	State  string `json:"state"`
	Reason string `json:"reason"`
	Result *T     `json:"result,omitempty"`
	Digest string `json:"digest,omitempty"`
}

func registerSemanticReads(s *mcpserver.MCPServer, d Deps) {
	limit := mcpgo.WithNumber("limit", mcpgo.Description("Maximum candidates, relations or trace paths (1..100, default 20); truncation and total are explicit."))
	s.AddTool(mcpgo.NewTool(ToolNameSemanticLookupTerm, mcpgo.WithDescription("Read exact ontology terms in this pinned dataset, preserving all reviewed/proposed/rejected interpretations and ambiguity. Does not rewrite search or activate ontology."), mcpgo.WithOutputSchema[semanticReadResponse[semantic.TermReadView]](), mcpgo.WithString("language", mcpgo.Required()), mcpgo.WithString("term", mcpgo.Required()), limit), func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		return handleSemanticLookupTerm(ctx, d, req), nil
	})
	s.AddTool(mcpgo.NewTool(ToolNameSemanticGetConcept, mcpgo.WithDescription("Read a source-backed ontology concept, definition, scope, terms, review status and typed incident relations. Relations are evidence-linked records, not inferred truth."), mcpgo.WithOutputSchema[semanticReadResponse[semantic.ConceptReadView]](), mcpgo.WithString("concept_id", mcpgo.Required()), limit), func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		return handleSemanticGetConcept(ctx, d, req), nil
	})
	s.AddTool(mcpgo.NewTool(ToolNameSemanticTraceRequirement, mcpgo.WithDescription("Read a requirement, Given/When/Then criteria and reviewed code/test paths without requiring policy context. Missing/unreviewed/conflicting links are explicit. Linked does not mean tests ran or criteria were accepted."), mcpgo.WithOutputSchema[semanticReadResponse[semantic.RequirementReadView]](), mcpgo.WithString("requirement_id", mcpgo.Required()), limit), func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		return handleSemanticTraceRequirement(ctx, d, req), nil
	})
}
func semanticResult[T any](state string, result *T) *mcpgo.CallToolResult {
	out := semanticReadResponse[T]{State: state, Reason: state, Result: result}
	if result != nil {
		raw, _ := json.Marshal(out)
		sum := sha256.Sum256(raw)
		out.Digest = fmt.Sprintf("%x", sum)
	}
	resultTool := mcpgo.NewToolResultStructured(out, "source-backed ontology read")
	resultTool.IsError = state != "ready" && state != "no_match" && state != "not_found"
	return resultTool
}
func readArgs(req mcpgo.CallToolRequest, names ...string) (int, bool) {
	for _, n := range names {
		v := strings.TrimSpace(req.GetString(n, ""))
		if v == "" || len(v) > 512 {
			return 0, false
		}
	}
	limit := float64(20)
	if value, present := req.GetArguments()["limit"]; present {
		switch v := value.(type) {
		case float64:
			limit = v
		case int:
			limit = float64(v)
		case json.Number:
			n, err := v.Float64()
			if err != nil {
				return 0, false
			}
			limit = n
		default:
			return 0, false
		}
	}
	if limit < 1 || limit > 100 || limit != float64(int(limit)) {
		return 0, false
	}
	return int(limit), true
}

// Scan raw string values, not escaped JSON, so multi-line secret rules work.
// Curated meaning is never published without the server's sanitizer.
func semanticReadSafe(ctx context.Context, d Deps, result any, evidence []semantic.EvidenceSpan) string {
	if ctx.Err() != nil {
		return "budget_exceeded"
	}
	if d.EvidenceSanitizer == nil {
		return "sanitizer_unavailable"
	}
	raw, err := json.Marshal(result)
	if err != nil || len(raw) > 32000 {
		return "budget_exceeded"
	}
	var text strings.Builder
	var walk func(reflect.Value)
	walk = func(v reflect.Value) {
		if !v.IsValid() {
			return
		}
		switch v.Kind() {
		case reflect.Pointer, reflect.Interface:
			if !v.IsNil() {
				walk(v.Elem())
			}
		case reflect.Struct:
			for i := 0; i < v.NumField(); i++ {
				walk(v.Field(i))
			}
		case reflect.Slice, reflect.Array:
			for i := 0; i < v.Len(); i++ {
				walk(v.Index(i))
			}
		case reflect.String:
			text.WriteString(v.String())
			text.WriteByte('\n')
		}
	}
	walk(reflect.ValueOf(result))
	if len(evidence) == 0 {
		return "source_unavailable"
	}
	e := evidence[0]
	cleaned, err := d.EvidenceSanitizer.Sanitize(ctx, []sanitize.Sanitizable{{Citation: contract.Citation{File: e.Path, StartLine: e.StartLine, EndLine: e.EndLine, CommitHash: e.Snapshot.Commit}, Body: text.String()}})
	if ctx.Err() != nil {
		return "budget_exceeded"
	}
	if err != nil || cleaned.FailClosed || len(cleaned.Items) != 1 || cleaned.Items[0].Dropped || len(cleaned.Redactions) > 0 {
		return "restricted"
	}
	return "ready"
}
func handleSemanticLookupTerm(ctx context.Context, d Deps, req mcpgo.CallToolRequest) *mcpgo.CallToolResult {
	limit, ok := readArgs(req, "language", "term")
	if !ok {
		return semanticResult[semantic.TermReadView]("invalid_request", nil)
	}
	ctx, cancel := semanticReadContext(ctx, d)
	defer cancel()
	a, state := loadSemanticProjection(ctx, d)
	if state != "ready" {
		return semanticResult[semantic.TermReadView](state, nil)
	}
	view := a.LookupTermView(req.GetString("language", ""), req.GetString("term", ""), limit)
	if view.TotalCandidates == 0 {
		return semanticResult[semantic.TermReadView]("no_match", nil)
	}
	if state := semanticReadSafe(ctx, d, view, view.Evidence); state != "ready" {
		return semanticResult[semantic.TermReadView](state, nil)
	}
	return semanticResult("ready", &view)
}
func handleSemanticGetConcept(ctx context.Context, d Deps, req mcpgo.CallToolRequest) *mcpgo.CallToolResult {
	limit, ok := readArgs(req, "concept_id")
	if !ok {
		return semanticResult[semantic.ConceptReadView]("invalid_request", nil)
	}
	ctx, cancel := semanticReadContext(ctx, d)
	defer cancel()
	a, state := loadSemanticProjection(ctx, d)
	if state != "ready" {
		return semanticResult[semantic.ConceptReadView](state, nil)
	}
	view, found := a.ConceptView(strings.TrimSpace(req.GetString("concept_id", "")), limit)
	if !found {
		return semanticResult[semantic.ConceptReadView]("not_found", nil)
	}
	if state := semanticReadSafe(ctx, d, view, view.Evidence); state != "ready" {
		return semanticResult[semantic.ConceptReadView](state, nil)
	}
	return semanticResult("ready", &view)
}
func handleSemanticTraceRequirement(ctx context.Context, d Deps, req mcpgo.CallToolRequest) *mcpgo.CallToolResult {
	limit, ok := readArgs(req, "requirement_id")
	if !ok {
		return semanticResult[semantic.RequirementReadView]("invalid_request", nil)
	}
	ctx, cancel := semanticReadContext(ctx, d)
	defer cancel()
	a, state := loadSemanticProjection(ctx, d)
	if state != "ready" {
		return semanticResult[semantic.RequirementReadView](state, nil)
	}
	view, found, err := a.RequirementView(strings.TrimSpace(req.GetString("requirement_id", "")), limit)
	if err != nil {
		return semanticResult[semantic.RequirementReadView]("stale", nil)
	}
	if !found {
		return semanticResult[semantic.RequirementReadView]("not_found", nil)
	}
	if state := semanticReadSafe(ctx, d, view, view.Evidence); state != "ready" {
		return semanticResult[semantic.RequirementReadView](state, nil)
	}
	return semanticResult("ready", &view)
}
