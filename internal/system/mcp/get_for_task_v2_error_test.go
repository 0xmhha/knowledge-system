package mcp

import (
	"fmt"
	"strings"
	"testing"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

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
