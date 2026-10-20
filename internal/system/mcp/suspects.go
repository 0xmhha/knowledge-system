package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"github.com/0xmhha/knowledge-system/internal/setup"
	"github.com/0xmhha/knowledge-system/internal/system/composer/sanitize"
	"github.com/0xmhha/knowledge-system/internal/system/suspects"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

var ToolNameLabelSuspects = toolName("context.label_suspects")

func registerLabelSuspects(s *mcpserver.MCPServer, d Deps) {
	tool := mcpgo.NewTool(ToolNameLabelSuspects,
		mcpgo.WithOutputSchema[contract.SuspectCandidates](),
		mcpgo.WithDescription("Bounded factual labels for an iterative symptom investigation. Choose symbol, file+line, expansion citations, or text-only seed discovery. Returns evidence and unknowns; never decides the root cause. Pinned datasets require retained expansion citations."),
		mcpgo.WithString("symptom", mcpgo.Required(), mcpgo.Description("Symptom used for seed discovery; judgments remain the agent's responsibility.")),
		mcpgo.WithString("symbol", mcpgo.Description("Optional unique symbol seed.")),
		mcpgo.WithString("file", mcpgo.Description("Optional repository-relative site file; requires line.")),
		mcpgo.WithNumber("line", mcpgo.Description("Positive integer site line.")),
		mcpgo.WithArray("expand", mcpgo.Items(expansionCitationSchema()), mcpgo.MaxItems(8), mcpgo.Description("Prior CitationV2 seeds for iterative expansion (legacy unpinned datasets use Citation).")),
		mcpgo.WithString("commit", mcpgo.Description("Optional exact indexed commit assertion.")),
		mcpgo.WithNumber("max_candidates", mcpgo.Description("Integer 1..20, default 10.")),
		mcpgo.WithNumber("max_seeds", mcpgo.Description("Integer 1..8, default 3.")),
		mcpgo.WithNumber("max_enriched", mcpgo.Description("Integer 1..8, default 3; expensive labels for first candidates only.")),
		mcpgo.WithNumber("max_calls", mcpgo.Description("Integer 4..64, default 32, includes final identity checks.")),
		mcpgo.WithNumber("depth", mcpgo.Description("Integer 1..3, default 2.")),
		mcpgo.WithNumber("budget_ms", mcpgo.Description("Integer 10..5000, default 1000; cooperative backend deadline, excludes retained-source verification.")))
	s.AddTool(tool, func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		return handleLabelSuspects(ctx, d, req)
	})
}

func expansionCitationSchema() map[string]any {
	properties := map[string]any{}
	for _, name := range []string{"file", "commit_hash", "project_id", "dataset_id", "snapshot_id", "source_mode", "base_commit", "origin_id", "file_sha256", "content_sha256"} {
		properties[name] = map[string]any{"type": "string"}
	}
	for _, name := range []string{"start_line", "end_line"} {
		properties[name] = map[string]any{"type": "integer", "minimum": 1}
	}
	return map[string]any{"type": "object", "properties": properties, "required": []string{"file", "start_line", "end_line", "commit_hash"}, "additionalProperties": false}
}

func handleLabelSuspects(ctx context.Context, d Deps, req mcpgo.CallToolRequest) (result *mcpgo.CallToolResult, callErr error) {
	ctx, finish := beginBackendMeasurement(ctx, d, req)
	defer func() { finish(result, callErr) }()
	r, rawExpand, err := parseSuspectRequest(req)
	if err != nil {
		return mcpgo.NewToolResultError("invalid_request: " + err.Error()), nil
	}
	if d.Alignment != nil && !d.Alignment.OK {
		return mcpgo.NewToolResultError("snapshot_mismatch: backend alignment unavailable"), nil
	}
	var verification *setup.RequestVerification
	var expected []contract.CitationV2
	if d.EvidenceVersionDir != "" {
		if d.EvidenceSanitizer == nil {
			return mcpgo.NewToolResultError("reindex_required: retained sanitizer unavailable"), nil
		}
		ctx, verification, err = setup.BeginRequestVerification(ctx, d.EvidenceVersionDir)
		if err != nil || verification == nil {
			return mcpgo.NewToolResultError(suspectVerificationRefusal(ctx, err, "snapshot_mismatch: retained identity unavailable")), nil
		}
		defer verification.Close()
		id := verification.Identity()
		r.Coordinates = &contract.V2Coordinates{ProjectID: id.Source.ProjectID, DatasetID: id.DatasetID, SnapshotID: id.Source.SnapshotID, SourceMode: id.Source.SourceMode, BaseCommit: id.Source.SourceCommit}
		if len(rawExpand) > 0 {
			if err = decodeStrict(rawExpand, &expected); err != nil {
				return mcpgo.NewToolResultError("invalid_request: invalid retained expansion citations"), nil
			}
			for _, c := range expected {
				if c.ProjectID != r.Coordinates.ProjectID || c.DatasetID != r.Coordinates.DatasetID || c.SnapshotID != r.Coordinates.SnapshotID || c.SourceMode != r.Coordinates.SourceMode || c.BaseCommit != r.Coordinates.BaseCommit || c.OriginID != "repo" || c.FileSHA256 == "" || c.ContentSHA256 == "" {
					return mcpgo.NewToolResultError("snapshot_mismatch: expansion belongs to a different retained source"), nil
				}
				r.Expand = append(r.Expand, contract.Citation{File: c.File, StartLine: c.StartLine, EndLine: c.EndLine, CommitHash: c.BaseCommit})
			}
		}
	} else if len(rawExpand) > 0 {
		if err = decodeStrict(rawExpand, &r.Expand); err != nil {
			return mcpgo.NewToolResultError("invalid_request: invalid expansion citations"), nil
		}
	}
	if err = r.Defaults(); err != nil {
		return mcpgo.NewToolResultError("invalid_request: " + err.Error()), nil
	}
	// Verify supplied hashes before querying. Composition uses archive-pinned
	// backend identity, not a live checkout or the caller's source assertion.
	if len(expected) > 0 {
		refs := append([]contract.Citation(nil), r.Expand...)
		anchors, e := retainedSuspectAnchors(ctx, d, refs)
		if e != nil {
			return mcpgo.NewToolResultError(suspectVerificationRefusal(ctx, e, "snapshot_mismatch: expansion source verification failed")), nil
		}
		for i, c := range expected {
			if *anchors[refs[i]] != c {
				return mcpgo.NewToolResultError("snapshot_mismatch: expansion hashes differ"), nil
			}
		}
	}
	out, err := suspects.Compose(ctx, d.CKG, d.CKV, r)
	if err != nil {
		return mcpgo.NewToolResultError("label_suspects: " + err.Error()), nil
	}
	if verification != nil {
		refs := []contract.Citation{}
		for _, c := range out.Candidates {
			refs = append(refs, c.Citation)
			for _, p := range signalProvenance(c.Signals) {
				refs = append(refs, p.Anchor)
			}
		}
		anchors, e := retainedSuspectAnchors(ctx, d, refs)
		if e != nil {
			return mcpgo.NewToolResultError(suspectVerificationRefusal(ctx, e, "snapshot_mismatch: retained signal verification failed")), nil
		}
		for i := range out.Candidates {
			c := &out.Candidates[i]
			c.Retained = anchors[c.Citation]
			attachSignalAnchors(&c.Signals, anchors)
		}
		if err = verification.Finish(ctx); err != nil {
			return mcpgo.NewToolResultError(suspectVerificationRefusal(ctx, err, "snapshot_mismatch: retained source changed")), nil
		}
	}
	return mcpgo.NewToolResultStructured(out, "Factual candidate signals; symptom judgments remain with the agent."), nil
}

var (
	errSuspectEvidenceBudget = errors.New("evidence_budget_exceeded")
	errSuspectSourcePolicy   = errors.New("source_policy_denied")
)

func suspectVerificationRefusal(ctx context.Context, err error, fallback string) string {
	switch {
	case errors.Is(ctx.Err(), context.Canceled), errors.Is(err, context.Canceled):
		return "request_cancelled: retained verification interrupted"
	case errors.Is(ctx.Err(), context.DeadlineExceeded), errors.Is(err, context.DeadlineExceeded):
		return "verification_budget_exhausted: retained verification interrupted"
	case errors.Is(err, errSuspectEvidenceBudget):
		return "evidence_budget_exceeded: retained evidence bounds exceeded"
	case errors.Is(err, errSuspectSourcePolicy):
		return "source_policy_denied: retained evidence rejected"
	default:
		return fallback
	}
}

func decodeStrict(raw []byte, out any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	return dec.Decode(out)
}
func parseSuspectRequest(req mcpgo.CallToolRequest) (suspects.Request, json.RawMessage, error) {
	r := suspects.Request{}
	var expand json.RawMessage
	for k, v := range req.GetArguments() {
		switch k {
		case "symptom", "symbol", "file", "commit":
			text, ok := v.(string)
			if !ok {
				return r, nil, fmt.Errorf("%s must be string", k)
			}
			switch k {
			case "symptom":
				r.Symptom = text
			case "symbol":
				r.Symbol = text
			case "file":
				r.File = text
			case "commit":
				r.Commit = text
			}
		case "line", "max_candidates", "max_seeds", "max_enriched", "max_calls", "depth", "budget_ms":
			raw, e := json.Marshal(v)
			if e != nil {
				return r, nil, fmt.Errorf("invalid %s", k)
			}
			var n float64
			if _, bad := v.(string); bad {
				return r, nil, fmt.Errorf("%s must be integer number", k)
			}
			if e = json.Unmarshal(raw, &n); e != nil || math.IsNaN(n) || math.IsInf(n, 0) || n != math.Trunc(n) || n < 1 || n > 1_000_000 {
				return r, nil, fmt.Errorf("%s must be positive integer number", k)
			}
			value := int(n)
			switch k {
			case "line":
				r.Line = value
			case "max_candidates":
				r.MaxCandidates = value
			case "max_seeds":
				r.MaxSeeds = value
			case "max_enriched":
				r.MaxEnriched = value
			case "max_calls":
				r.MaxCalls = value
			case "depth":
				r.Depth = value
			case "budget_ms":
				r.BudgetMS = value
			}
		case "expand":
			raw, e := json.Marshal(v)
			if e != nil || len(raw) > 32_000 || len(raw) < 2 || raw[0] != '[' {
				return r, nil, fmt.Errorf("expand must be bounded array")
			}
			expand = raw
		default:
			return r, nil, fmt.Errorf("unknown input %s", k)
		}
	}
	return r, expand, nil
}

func signalProvenance(s contract.SuspectSignals) []contract.SignalFrom {
	out := []contract.SignalFrom{}
	for _, p := range [][]contract.SignalFrom{s.Distance.Provenance, s.Relations.Provenance, s.Impact.Provenance, s.Recency.Provenance, s.Concurrency.Provenance, s.Retrieval.Provenance, s.Security.Provenance, s.Dataflow.Provenance} {
		out = append(out, p...)
	}
	return out
}
func attach[T any](s *contract.Signal[T], anchors map[contract.Citation]*contract.CitationV2) {
	for i := range s.Provenance {
		s.Provenance[i].Retained = anchors[s.Provenance[i].Anchor]
	}
}
func attachSignalAnchors(s *contract.SuspectSignals, a map[contract.Citation]*contract.CitationV2) {
	attach(&s.Distance, a)
	attach(&s.Relations, a)
	attach(&s.Impact, a)
	attach(&s.Recency, a)
	attach(&s.Concurrency, a)
	attach(&s.Retrieval, a)
	attach(&s.Security, a)
	attach(&s.Dataflow, a)
}

func retainedSuspectAnchors(ctx context.Context, d Deps, refs []contract.Citation) (map[contract.Citation]*contract.CitationV2, error) {
	out := map[contract.Citation]*contract.CitationV2{}
	unique := []contract.Citation{}
	spans := []setup.RetainedLineSpan{}
	seen := map[contract.Citation]bool{}
	for _, ref := range refs {
		if seen[ref] {
			continue
		}
		seen[ref] = true
		unique = append(unique, ref)
		spans = append(spans, setup.RetainedLineSpan{OriginID: "repo", Path: ref.File, First: ref.StartLine, Last: ref.EndLine})
	}
	if len(spans) > 128 {
		return nil, errSuspectEvidenceBudget
	}
	total := 0
	err := setup.VisitRetainedLineSpans(ctx, d.EvidenceVersionDir, spans, func(i int, e setup.RetainedEvidence) error {
		ref := unique[i]
		if ref.CommitHash != e.BaseCommit {
			return fmt.Errorf("snapshot_mismatch")
		}
		total += len(e.Text)
		if len(e.Text) > 32_000 || total > 256_000 {
			return errSuspectEvidenceBudget
		}
		clean, err := d.EvidenceSanitizer.Sanitize(ctx, []sanitize.Sanitizable{{Citation: ref, Body: e.Text}})
		if err != nil {
			return err
		}
		if clean.FailClosed || len(clean.Items) != 1 || clean.Items[0].Dropped || clean.Items[0].Citation != ref {
			return errSuspectSourcePolicy
		}
		out[ref] = &contract.CitationV2{File: e.File, StartLine: e.StartLine, EndLine: e.EndLine, CommitHash: e.CommitHash, ProjectID: e.ProjectID, DatasetID: e.DatasetID, SnapshotID: e.SnapshotID, SourceMode: e.SourceMode, BaseCommit: e.BaseCommit, OriginID: e.OriginID, FileSHA256: e.FileSHA256, ContentSHA256: e.ContentSHA256}
		return nil
	})
	return out, err
}
