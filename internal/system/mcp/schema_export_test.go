package mcp

import (
	"encoding/json"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"os"
	"testing"
)

// An explicit developer export captures actual registered input schemas;
// normal verification never repairs the checked-in contract on a mismatch.
func TestExportToolContract(t *testing.T) {
	path := os.Getenv("KS_EXPORT_TOOL_CONTRACT")
	if path == "" {
		t.Skip("explicit contract export only")
	}
	f := newFixture(t, nil)
	srv := mcpserver.NewMCPServer("contract-export", "1")
	if e := Register(srv, f.deps); e != nil {
		t.Fatal(e)
	}
	tools := map[string]any{}
	for name, registered := range srv.ListTools() {
		wire, e := json.Marshal(registered.Tool)
		if e != nil {
			t.Fatal(e)
		}
		var definition struct {
			Input json.RawMessage `json:"inputSchema"`
		}
		if e = json.Unmarshal(wire, &definition); e != nil {
			t.Fatal(e)
		}
		tools[name] = map[string]any{"description": registered.Tool.Description, "input": definition.Input}
	}
	raw, e := json.MarshalIndent(map[string]any{"tools": tools}, "", "  ")
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(path, append(raw, '\n'), 0600); e != nil {
		t.Fatal(e)
	}
}
