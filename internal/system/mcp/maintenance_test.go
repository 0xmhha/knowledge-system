package mcp

import (
	"testing"

	"github.com/0xmhha/knowledge-system/internal/system/maintenance"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

func TestMaintenanceToolsAreOptInAndDoNotExposePaths(t *testing.T) {
	f := newFixture(t, nil)
	server := mcpserver.NewMCPServer("maintenance-test", "test")
	if err := Register(server, f.deps); err != nil {
		t.Fatal(err)
	}
	if server.GetTool(ToolNameDatasetBuild) != nil {
		t.Fatal("default surface changed")
	}
	service, err := maintenance.New(nil, "unused")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(service.Close)
	f.deps.Maintenance = service
	server = mcpserver.NewMCPServer("maintenance-test", "test")
	if err := Register(server, f.deps); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{ToolNameDatasetInspect, ToolNameDatasetBuild, ToolNameDatasetStatus, ToolNameDatasetCancel} {
		tool := server.GetTool(name)
		if tool == nil || tool.Tool.OutputSchema.Type != "object" {
			t.Fatalf("missing schema: %s", name)
		}
	}
	result := publicMaintenanceCall(t, f.deps, ToolNameDatasetInspect, map[string]any{"schema_version": 1, "project_id": "a", "profile_id": "p"})
	if result["isError"] != true {
		t.Fatal("unknown profile accepted")
	}
	result = publicMaintenanceCall(t, f.deps, ToolNameDatasetInspect, map[string]any{"schema_version": 1, "project_id": "a", "profile_id": "p", "path": "/private/other"})
	if result["isError"] != true {
		t.Fatal("caller path accepted")
	}
}
