package mcp

import (
	mcpgo "github.com/mark3labs/mcp-go/mcp"

	"github.com/0xmhha/knowledge-system/internal/setup"
)

// Legacy context tools cannot express snapshot-only or working-tree evidence
// provenance. Keep their historical wire contract for committed datasets only.
func legacyContextGuard(d Deps) *mcpgo.CallToolResult {
	if d.EvidenceVersionDir == "" {
		return nil
	}
	identity, err := setup.InspectVersionIdentity(d.EvidenceVersionDir)
	if err != nil {
		return mcpgo.NewToolResultErrorf("snapshot_mismatch: %v", err)
	}
	if identity != nil && identity.Source.SourceMode != "committed" {
		return mcpgo.NewToolResultError("requires_v2: this source snapshot requires cks.context.get_for_task_v2")
	}
	return nil
}
