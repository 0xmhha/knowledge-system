package mcphandlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// TestLogToolCalls: every call leaves one record with a request id, the tool,
// its arguments and how it ended; ids differ between calls.
func TestLogToolCalls(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	mw := LogToolCalls(log)

	ok := mw(func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText("done"), nil
	})
	fail := mw(func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return nil, errors.New("store closed")
	})
	req := mcp.CallToolRequest{}
	req.Params.Name = "find_callers"
	req.Params.Arguments = map[string]any{"qname": "core.NewBlockChain", "depth": 2}
	if _, err := ok(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	req.Params.Arguments = map[string]any{"task": strings.Repeat("x", 2*maxLoggedArgs)}
	if _, err := fail(context.Background(), req); err == nil {
		t.Fatal("the handler error must pass through")
	}

	var recs []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		var r map[string]any
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatalf("record %q: %v", line, err)
		}
		recs = append(recs, r)
	}
	if len(recs) != 2 {
		t.Fatalf("want 2 records, got %d: %s", len(recs), buf.String())
	}
	first, second := recs[0], recs[1]
	if first["area"] != "mcp" || first["tool"] != "find_callers" || first["outcome"] != "ok" ||
		!strings.Contains(first["arguments"].(string), "core.NewBlockChain") {
		t.Errorf("first record: %v", first)
	}
	if second["outcome"] != "error" || second["err"] != "store closed" || second["level"] != "WARN" ||
		!strings.HasSuffix(second["arguments"].(string), "...(cut)") {
		t.Errorf("second record: %v", second)
	}
	if first["request_id"] == "" || first["request_id"] == second["request_id"] {
		t.Errorf("request ids %v and %v", first["request_id"], second["request_id"])
	}
}
