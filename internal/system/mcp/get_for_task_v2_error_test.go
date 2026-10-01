package mcp

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/system/composer/sanitize"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

func TestV2ToolClassifiesLegacyDatasetBeforeBackendHealth(t *testing.T) {
	f := newFixture(t, nil)
	cleaner, err := sanitize.New(f.ruleset)
	if err != nil {
		t.Fatal(err)
	}
	f.deps.EvidenceVersionDir = t.TempDir() // no pinned v2 identity
	f.deps.EvidenceSanitizer = cleaner
	f.ckv.HealthVal.ModelReachable = false
	result, err := handleGetForTaskV2(context.Background(), f.deps, callToolReq(map[string]any{"prompt": "explain Alpha"}))
	if err != nil || result == nil || !result.IsError {
		t.Fatalf("legacy v2 request lacked a public error: %+v %v", result, err)
	}
	structured := result.StructuredContent.(map[string]string)
	if structured["code"] != "reindex_required" {
		t.Fatalf("backend error hid the required v1 migration: %+v", structured)
	}
}

func TestV2ToolErrorHasMatchingStructuredCodeAndBoundedText(t *testing.T) {
	for _, code := range []string{"reindex_required", "snapshot_mismatch", "knowledge_context_failed"} {
		result := v2ToolError(code)
		structured, ok := result.StructuredContent.(map[string]string)
		if !ok || !result.IsError || len(result.Content) != 1 || structured["code"] == "" || structured["message"] == "" {
			t.Fatalf("missing structured v2 error for %q: %+v", code, result)
		}
		body, ok := result.Content[0].(mcpgo.TextContent)
		if !ok || !strings.HasPrefix(body.Text, "code="+structured["code"]+": ") ||
			strings.Contains(body.Text, "/tmp/") {
			t.Fatalf("unbounded v2 error text for %q: %+v", code, result.Content)
		}
	}
}

func TestV2ToolErrorForKeepsSpecificCodeWithoutSourceDetails(t *testing.T) {
	result := v2ToolErrorFor(fmt.Errorf("read /tmp/secret.txt: snapshot_mismatch: API_KEY=value"), "v2_evidence_failed")
	structured := result.StructuredContent.(map[string]string)
	text := result.Content[0].(mcpgo.TextContent).Text
	if structured["code"] != "snapshot_mismatch" || strings.Contains(text, "secret.txt") || strings.Contains(text, "API_KEY") {
		t.Fatalf("specific code or redaction failed: %+v", result)
	}
}
