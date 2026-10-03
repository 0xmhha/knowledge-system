package mcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/0xmhha/knowledge-system/internal/setup"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/0xmhha/knowledge-system/internal/system/evidencev2"
	"github.com/0xmhha/knowledge-system/internal/system/semantic"
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

func handleGetForTaskV2(ctx context.Context, d Deps, req mcpgo.CallToolRequest) (result *mcpgo.CallToolResult, callErr error) {
	ctx, finish := beginBackendMeasurement(ctx, d, req)
	defer func() { finish(result, callErr) }()
	if d.EvidenceVersionDir == "" || d.EvidenceSanitizer == nil {
		return v2ToolError("reindex_required"), nil
	}
	prompt := req.GetString("prompt", "")
	if prompt == "" {
		return v2ToolError("invalid_request"), nil
	}
	callerIntent := contract.IntentUnknown
	if raw := req.GetString("intent", ""); raw != "" {
		parsed, ok := contract.ParseIntent(raw)
		if !ok {
			return v2ToolError("invalid_request"), nil
		}
		callerIntent = parsed
	}
	identity, identityErr := setup.InspectVersionIdentity(d.EvidenceVersionDir)
	if identityErr != nil || identity == nil {
		return v2ToolError("reindex_required"), nil
	}
	if ok, _ := serviceable(ctx, d); !ok {
		return v2ToolError("service_unavailable"), nil
	}
	legacy, err := d.Composer.ComposeWithIntent(ctx, prompt, callerIntent)
	if err != nil {
		return v2ToolErrorFor(err, "compose_failed"), nil
	}
	pack, err := evidencev2.Build(ctx, d.EvidenceVersionDir, prompt, legacy.Citations, d.EvidenceSanitizer)
	if err != nil {
		return v2ToolErrorFor(err, "v2_evidence_failed"), nil
	}
	pack.Metadata.Ontology = legacy.Metadata.Ontology
	// Knowledge/traces verify their input before extending it. Stamp the
	// optional diagnostic at this boundary, not only after those layers.
	if pack.Metadata.Ontology != nil {
		if err := evidencev2.Stamp(&pack); err != nil {
			return v2ToolErrorFor(err, "v2_evidence_failed"), nil
		}
	}
	if req.GetBool("include_knowledge", false) {
		asOf, subsystem := req.GetString("knowledge_as_of", ""), req.GetString("knowledge_subsystem", "")
		if asOf == "" || subsystem == "" {
			return v2ToolError("invalid_knowledge_scope"), nil
		}
		pack, err = evidencev2.AttachKnowledge(ctx, pack, d.EvidenceVersionDir, asOf, subsystem, d.EvidenceSanitizer)
		if err != nil {
			return v2ToolErrorFor(err, "knowledge_context_failed"), nil
		}
		if d.SemanticStorePath != "" {
			// The store is optional. A missing or stale projection keeps the
			// already cited CKV/CKG and local knowledge response intact.
			if info, statErr := os.Stat(d.SemanticStorePath); statErr == nil && info.Mode().IsRegular() {
				store, openErr := semantic.OpenStoreReadOnly(ctx, d.SemanticStorePath)
				if openErr == nil {
					projection, loadErr := store.LoadAlignedRetained(ctx, pack.Coordinates.ProjectID,
						pack.Coordinates.DatasetID, d.EvidenceSourceRoot,
						filepath.Join(d.EvidenceVersionDir, "graph"), filepath.Join(d.EvidenceVersionDir, "vector"), d.EvidenceVersionDir)
					_ = store.Close()
					if loadErr == nil {
						pack, err = evidencev2.AttachVerifiedTraces(ctx, pack, d.EvidenceVersionDir, asOf, subsystem, projection, d.EvidenceSanitizer)
						if err != nil {
							return v2ToolErrorFor(err, "knowledge_context_failed"), nil
						}
					}
				}
			}
		}
	}
	return mcpgo.NewToolResultStructured(pack, "v2 evidence pack"), nil
}

func v2ToolErrorFor(err error, fallback string) *mcpgo.CallToolResult {
	if err != nil {
		for _, code := range []string{"requires_v2", "reindex_required", "snapshot_mismatch", "source_missing"} {
			if strings.Contains(err.Error(), code) {
				return v2ToolError(code)
			}
		}
	}
	return v2ToolError(fallback)
}

func v2ToolError(code string) *mcpgo.CallToolResult {
	messages := map[string]string{
		"requires_v2":              "Use the v2 interface for this source mode.",
		"reindex_required":         "A pinned retained dataset is required.",
		"snapshot_mismatch":        "The indexed source identity does not match.",
		"source_missing":           "A pinned source file is missing.",
		"invalid_request":          "The request is missing or has an invalid argument.",
		"service_unavailable":      "The indexed service is unavailable.",
		"compose_failed":           "The base evidence could not be composed.",
		"v2_evidence_failed":       "Retained source evidence could not be verified.",
		"invalid_knowledge_scope":  "A date and subsystem are required for knowledge context.",
		"knowledge_context_failed": "The knowledge context could not be verified.",
	}
	message, ok := messages[code]
	if !ok {
		code, message = "operation_failed", "The operation failed."
	}
	result := mcpgo.NewToolResultError(fmt.Sprintf("code=%s: %s", code, message))
	result.StructuredContent = map[string]string{"code": code, "message": message}
	return result
}
