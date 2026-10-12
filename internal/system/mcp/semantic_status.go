package mcp

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/0xmhha/knowledge-system/internal/setup"
	"github.com/0xmhha/knowledge-system/internal/system/semantic"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

var ToolNameSemanticStatus = toolName("semantic.status")

type semanticStatusResponse struct {
	State           string                     `json:"state"`
	Mode            string                     `json:"mode"`
	Availability    string                     `json:"availability"`
	Reason          string                     `json:"reason"`
	StoreConfigured bool                       `json:"store_configured"`
	ReadToolsReady  bool                       `json:"read_tools_ready"`
	RetrievalReady  bool                       `json:"retrieval_ready"`
	SupportedModes  []string                   `json:"supported_modes"`
	Snapshot        *semantic.Snapshot         `json:"snapshot,omitempty"`
	Counts          *semantic.ProjectionCounts `json:"counts,omitempty"`
}

func registerSemanticStatus(s *mcpserver.MCPServer, d Deps) {
	s.AddTool(mcpgo.NewTool(ToolNameSemanticStatus,
		mcpgo.WithDescription("Inspect reviewed ontology availability for this pinned dataset without changing it. Separates disabled retrieval from missing, stale, unreviewed or ready data; readiness is not a quality or test-acceptance verdict."),
		mcpgo.WithOutputSchema[semanticStatusResponse]()), func(ctx context.Context, _ mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		return mcpgo.NewToolResultStructured(inspectSemanticStatus(ctx, d), "ontology preparation status"), nil
	})
}

// loadSemanticProjection rechecks retained source and actual index anchors.
// Stable reasons never include SQLite errors, local paths or source bodies.
func loadSemanticProjection(ctx context.Context, d Deps) (semantic.ActiveProjection, string) {
	var empty semantic.ActiveProjection
	if ctx.Err() != nil {
		return empty, "budget_exceeded"
	}
	if d.SemanticStorePath == "" {
		return empty, "unconfigured"
	}
	if d.EvidenceVersionDir == "" {
		return empty, "dataset_missing"
	}
	if info, err := os.Stat(d.EvidenceVersionDir); err != nil || !info.IsDir() {
		return empty, "dataset_missing"
	}
	if info, err := os.Stat(d.SemanticStorePath); err != nil || !info.Mode().IsRegular() {
		return empty, "store_missing"
	}
	ctx, verification, err := setup.BeginRequestVerification(ctx, d.EvidenceVersionDir)
	if err != nil {
		if ctx.Err() != nil {
			return empty, "budget_exceeded"
		}
		return empty, "stale"
	}
	if verification == nil {
		return empty, "unsupported_dataset"
	}
	defer verification.Close()
	identity, err := setup.InspectVersionIdentityContext(ctx, d.EvidenceVersionDir)
	if err != nil || identity == nil {
		return empty, "stale"
	}
	store, err := semantic.OpenStoreReadOnly(ctx, d.SemanticStorePath)
	if err != nil {
		if ctx.Err() != nil {
			return empty, "budget_exceeded"
		}
		return empty, "store_unavailable"
	}
	defer func() { _ = store.Close() }()
	projection, err := store.LoadAlignedRetained(ctx, identity.Source.ProjectID, identity.DatasetID, d.EvidenceSourceRoot,
		filepath.Join(d.EvidenceVersionDir, "graph"), filepath.Join(d.EvidenceVersionDir, "vector"), d.EvidenceVersionDir)
	if ctx.Err() != nil {
		return empty, "budget_exceeded"
	}
	if errors.Is(err, sql.ErrNoRows) {
		return empty, "projection_missing"
	}
	if err != nil {
		return empty, "stale"
	}
	snapshot := projection.Snapshot()
	if snapshot.SnapshotID != identity.Source.SnapshotID || snapshot.Commit != identity.Source.SourceCommit {
		return empty, "stale"
	}
	if err := verification.Finish(ctx); err != nil {
		if ctx.Err() != nil {
			return empty, "budget_exceeded"
		}
		return empty, "stale"
	}
	return projection, "ready"
}

func semanticReadContext(ctx context.Context, d Deps) (context.Context, context.CancelFunc) {
	budget := d.SemanticBudgetMS
	if budget <= 0 || budget > 5000 {
		budget = 1000
	}
	return context.WithTimeout(ctx, time.Duration(budget)*time.Millisecond)
}

func inspectSemanticStatus(ctx context.Context, d Deps) semanticStatusResponse {
	mode := d.SemanticOntologyMode
	if mode == "" {
		mode = "off"
	}
	out := semanticStatusResponse{Mode: mode, State: "off", StoreConfigured: d.SemanticStorePath != "",
		SupportedModes: []string{"off", "baseline", "concept_text", "relations", "combined"}}
	enabled := mode == "relations" || mode == "concept_text" || mode == "combined"
	if !enabled && mode != "off" && mode != "baseline" {
		out.Mode = "unsupported"
		out.State = "unsupported"
		out.Availability = "unsupported"
		out.Reason = "unsupported_mode"
		return out
	}
	ctx, cancel := semanticReadContext(ctx, d)
	defer cancel()
	active, state := loadSemanticProjection(ctx, d)
	out.Availability, out.Reason = state, state
	if state != "ready" {
		if enabled {
			out.State = state
		}
		return out
	}
	snapshot, counts := active.Snapshot(), active.Counts()
	out.Snapshot, out.Counts = &snapshot, &counts
	out.ReadToolsReady = true
	out.Reason = "retrieval_disabled"
	if enabled {
		out.State = "unreviewed"
		out.Reason = "no_reviewed_concepts"
		if counts.VerifiedConcepts > 0 {
			out.Reason = "no_reviewed_implementations"
			if mode != "relations" || counts.VerifiedImplementations > 0 {
				out.State = "ready"
				out.Reason = "ready"
				out.RetrievalReady = true
			}
		}
	}
	return out
}
