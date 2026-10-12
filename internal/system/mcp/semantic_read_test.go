package mcp

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/0xmhha/knowledge-system/internal/system/composer/sanitize"
	"github.com/0xmhha/knowledge-system/internal/system/config"
	"github.com/0xmhha/knowledge-system/internal/system/semantic"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"os"
	"strings"
	"testing"
)

func TestSemanticReadToolsAreDiscoverable(t *testing.T) {
	s := mcpserver.NewMCPServer("semantic-read", "test")
	if err := Register(s, newFixture(t, nil).deps); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"semantic.lookup_term", "semantic.get_concept", "semantic.trace_requirement"} {
		if s.GetTool(toolName(name)) == nil {
			t.Errorf("missing read-only MCP tool: %s", name)
		}
	}
}

func checkSemanticDigest[T any](t *testing.T, out semanticReadResponse[T]) {
	t.Helper()
	digest := out.Digest
	out.Digest = ""
	raw, _ := json.Marshal(out)
	sum := sha256.Sum256(raw)
	if digest != fmt.Sprintf("%x", sum) {
		t.Fatal("read payload digest differs")
	}
}
func TestSemanticReadsPreserveMeaningAndUnapprovedTrace(t *testing.T) {
	d := semanticReadFixtureN(t, true, true)
	before, _ := os.ReadFile(d.SemanticStorePath)
	lookup := handleSemanticLookupTerm(context.Background(), d, callToolReq(map[string]any{"language": "EN", "term": " FEATURE ", "limit": 1})).StructuredContent.(semanticReadResponse[semantic.TermReadView])
	if lookup.State != "ready" || lookup.Result == nil || !lookup.Result.Ambiguous || !lookup.Result.Truncated || lookup.Result.TotalCandidates != 2 || len(lookup.Result.Candidates) != 1 || len(lookup.Result.Evidence) != 1 {
		t.Fatalf("ambiguity/cutoff hidden: %+v", lookup)
	}
	checkSemanticDigest(t, lookup)
	all := handleSemanticLookupTerm(context.Background(), d, callToolReq(map[string]any{"language": "en", "term": "feature"})).StructuredContent.(semanticReadResponse[semantic.TermReadView])
	if len(all.Result.Candidates) != 2 || all.Result.Candidates[1].Status != semantic.StatusProposed {
		t.Fatalf("proposed meaning lost: %+v", all)
	}
	concept := handleSemanticGetConcept(context.Background(), d, callToolReq(map[string]any{"concept_id": "feature"})).StructuredContent.(semanticReadResponse[semantic.ConceptReadView])
	if concept.Result == nil || concept.Result.Concept.ReviewedBy != "owned-fixture-label" || len(concept.Result.Concept.Includes) != 1 || len(concept.Result.Evidence) != 1 {
		t.Fatalf("meaning source lost: %+v", concept)
	}
	checkSemanticDigest(t, concept)
	trace := handleSemanticTraceRequirement(context.Background(), d, callToolReq(map[string]any{"requirement_id": "req-feature"})).StructuredContent.(semanticReadResponse[semantic.RequirementReadView])
	if trace.Result == nil || trace.Result.Trace.State != semantic.TraceSpecUnapproved || trace.Result.AcceptanceEvaluated || len(trace.Result.Requirement.AcceptanceCriteria) != 1 || len(trace.Result.Evidence) != 2 {
		t.Fatalf("unapproved/criterion state lost: %+v", trace)
	}
	checkSemanticDigest(t, trace)
	concept.Result.Concept.Terms[0].Value = "mutated client copy"
	again := handleSemanticGetConcept(context.Background(), d, callToolReq(map[string]any{"concept_id": "feature"})).StructuredContent.(semanticReadResponse[semantic.ConceptReadView])
	if again.Result.Concept.Terms[0].Value != "feature" {
		t.Fatal("client view mutated source")
	}
	after, _ := os.ReadFile(d.SemanticStorePath)
	if sha256.Sum256(before) != sha256.Sum256(after) {
		t.Fatal("reads changed store bytes")
	}
}
func TestSemanticReadsRejectInvalidUnavailableAndRestricted(t *testing.T) {
	d := semanticReadFixture(t, false)
	for _, limit := range []any{0, 101, 1.5, "twenty"} {
		result := handleSemanticGetConcept(context.Background(), d, callToolReq(map[string]any{"concept_id": "feature", "limit": limit}))
		if !result.IsError {
			t.Fatal("invalid read is not a MCP tool error")
		}
		out := result.StructuredContent.(semanticReadResponse[semantic.ConceptReadView])
		if out.State != "invalid_request" || out.Result != nil {
			t.Fatalf("bad limit accepted: %+v", out)
		}
	}
	if out := handleSemanticGetConcept(context.Background(), d, callToolReq(map[string]any{})).StructuredContent.(semanticReadResponse[semantic.ConceptReadView]); out.State != "invalid_request" {
		t.Fatal(out)
	}
	if out := handleSemanticLookupTerm(context.Background(), d, callToolReq(map[string]any{"language": "en", "term": "unregistered"})).StructuredContent.(semanticReadResponse[semantic.TermReadView]); out.State != "no_match" || out.Result != nil {
		t.Fatal(out)
	}
	if out := handleSemanticTraceRequirement(context.Background(), d, callToolReq(map[string]any{"requirement_id": "missing"})).StructuredContent.(semanticReadResponse[semantic.RequirementReadView]); out.State != "not_found" {
		t.Fatal(out)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if out := handleSemanticGetConcept(ctx, d, callToolReq(map[string]any{"concept_id": "feature"})).StructuredContent.(semanticReadResponse[semantic.ConceptReadView]); out.State != "budget_exceeded" || out.Result != nil {
		t.Fatal(out)
	}
	for _, kind := range []string{"missing", "sanitizer", "restricted", "stale"} {
		t.Run(kind, func(t *testing.T) {
			d := semanticReadFixture(t, false)
			switch kind {
			case "missing":
				d.SemanticStorePath = ""
			case "sanitizer":
				d.EvidenceSanitizer = nil
			case "restricted":
				rules := &config.SanitizeRuleset{Version: 1, Rules: []config.SanitizeRule{{ID: "HIDE", Pattern: "Fixture feature", Action: contract.RedactionDrop, Severity: config.SeverityLow}}}
				if err := rules.Validate(); err != nil {
					t.Fatal(err)
				}
				cleaner, err := sanitize.New(rules)
				if err != nil {
					t.Fatal(err)
				}
				d.EvidenceSanitizer = cleaner
			case "stale":
				d.EvidenceSourceRoot = "/private-sensitive/source"
			}
			out := handleSemanticGetConcept(context.Background(), d, callToolReq(map[string]any{"concept_id": "feature"})).StructuredContent.(semanticReadResponse[semantic.ConceptReadView])
			want := map[string]string{"missing": "unconfigured", "sanitizer": "sanitizer_unavailable", "restricted": "restricted", "stale": "stale"}[kind]
			raw, _ := json.Marshal(out)
			if out.State != want || out.Result != nil || strings.Contains(string(raw), "private-sensitive") {
				t.Fatalf("unsafe result: %s", raw)
			}
		})
	}
}
