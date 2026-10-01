package mcp

import (
	"context"
	"fmt"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/0xmhha/knowledge-system/internal/system/evidencev2"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

func registerGetForTaskV2(s *mcpserver.MCPServer, d Deps) {
	tool := mcpgo.NewTool(ToolNameGetForTaskV2,
		mcpgo.WithOutputSchema[contract.EvidencePackV2](),
		mcpgo.WithDescription("Compose archive-backed v2 evidence with immutable dataset citations and sha256-v2 integrity."),
		mcpgo.WithString("prompt", mcpgo.Required(), mcpgo.Description("Natural-language development task.")),
		mcpgo.WithString("intent", mcpgo.Description("Optional task intent override.")),
		mcpgo.WithBoolean("include_knowledge", mcpgo.DefaultBool(false), mcpgo.Description("Add reviewed local policy context from this retained dataset.")),
		mcpgo.WithString("knowledge_as_of", mcpgo.Description("Policy date (YYYY-MM-DD) when include_knowledge is true.")),
		mcpgo.WithString("knowledge_subsystem", mcpgo.Description("Explicit subsystem scope when include_knowledge is true.")))
	s.AddTool(tool, func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		return handleGetForTaskV2(ctx, d, req)
	})
}

func handleGetForTaskV2(ctx context.Context, d Deps, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	if d.EvidenceVersionDir == "" || d.EvidenceSanitizer == nil {
		return mcpgo.NewToolResultError("reindex_required: v2 evidence requires a pinned, retained source dataset"), nil
	}
	prompt := req.GetString("prompt", "")
	if prompt == "" {
		return mcpgo.NewToolResultError("cks.context.get_for_task_v2: missing required argument \"prompt\""), nil
	}
	callerIntent := contract.IntentUnknown
	if raw := req.GetString("intent", ""); raw != "" {
		parsed, ok := contract.ParseIntent(raw)
		if !ok {
			return mcpgo.NewToolResultError(fmt.Sprintf("cks.context.get_for_task_v2: invalid intent %q", raw)), nil
		}
		callerIntent = parsed
	}
	if ok, reason := serviceable(ctx, d); !ok {
		return mcpgo.NewToolResultError(fmt.Sprintf("service_unavailable: %s", reason)), nil
	}
	legacy, err := d.Composer.ComposeWithIntent(ctx, prompt, callerIntent)
	if err != nil {
		return mcpgo.NewToolResultErrorf("compose_failed: %v", err), nil
	}
	pack, err := evidencev2.Build(ctx, d.EvidenceVersionDir, prompt, legacy.Citations, d.EvidenceSanitizer)
	if err != nil {
		return mcpgo.NewToolResultErrorf("v2_evidence_failed: %v", err), nil
	}
	if req.GetBool("include_knowledge", false) {
		asOf, subsystem := req.GetString("knowledge_as_of", ""), req.GetString("knowledge_subsystem", "")
		if asOf == "" || subsystem == "" {
			return mcpgo.NewToolResultError("invalid_knowledge_scope: knowledge_as_of and knowledge_subsystem are required"), nil
		}
		pack, err = evidencev2.AttachKnowledge(ctx, pack, d.EvidenceVersionDir, asOf, subsystem, d.EvidenceSanitizer)
		if err != nil {
			return mcpgo.NewToolResultErrorf("knowledge_context_failed: %v", err), nil
		}
	}
	return mcpgo.NewToolResultStructured(pack, "v2 evidence pack"), nil
}
