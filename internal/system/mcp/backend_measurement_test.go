package mcp

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/system/backendmeasure"
	"github.com/0xmhha/knowledge-system/internal/system/ckgclient"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

func TestLabelSuspectsMeasurementPreservesResolverRoutes(t *testing.T) {
	for _, pinned := range []bool{false, true} {
		for _, route := range []string{"site", "symbol", "expand"} {
			name := route + "/legacy"
			if pinned {
				name = route + "/pinned"
			}
			t.Run(name, func(t *testing.T) {
				f := suspectsFixture(t, pinned)
				c := f.ckg.SymbolCitations[0]
				site := c
				site.EndLine = site.StartLine
				f.ckg.SymbolIdentities[site.String()] = f.ckg.SymbolIdentities[c.String()]
				args := map[string]any{"symptom": "resolver parity", "budget_ms": 5000}
				switch route {
				case "site":
					args["file"], args["line"] = c.File, c.StartLine
				case "symbol":
					args["symbol"] = "Alpha"
				case "expand":
					res, err := handleLabelSuspects(context.Background(), f.deps, callToolReq(map[string]any{"symptom": "seed", "symbol": "Alpha"}))
					if err != nil || res.IsError {
						t.Fatal("seed fixture failed", res, err)
					}
					seed := res.StructuredContent.(contract.SuspectCandidates).Candidates[0]
					if pinned {
						args["expand"] = []contract.CitationV2{*seed.Retained}
					} else {
						args["expand"] = []contract.Citation{seed.Citation}
					}
				}
				direct, err := handleLabelSuspects(context.Background(), f.deps, callToolReq(args))
				if err != nil || direct.IsError {
					t.Fatal("unmeasured route failed", direct, err)
				}
				f.deps.CKG = backendmeasure.WrapCKG(f.ckg)
				measured, err := handleLabelSuspects(context.Background(), f.deps, callToolReq(args))
				if err != nil || measured.IsError {
					t.Fatal("measurement changed route availability", measured, err)
				}
				a := direct.StructuredContent.(contract.SuspectCandidates)
				b := measured.StructuredContent.(contract.SuspectCandidates)
				a.Budget.ElapsedMS, b.Budget.ElapsedMS = 0, 0
				if !reflect.DeepEqual(a, b) {
					t.Fatal("measurement changed source, candidates, labels, budgets or provenance")
				}
				if len(b.Candidates) != 1 || b.Candidates[0].Symbol == nil || *b.Candidates[0].Symbol != "Alpha" {
					t.Fatal("resolver identity lost")
				}
				if pinned {
					bad := *b.Candidates[0].Retained
					bad.ContentSHA256 = strings.Repeat("0", 64)
					args["expand"] = []contract.CitationV2{bad}
					delete(args, "file")
					delete(args, "line")
					delete(args, "symbol")
					res, err := handleLabelSuspects(context.Background(), f.deps, callToolReq(args))
					if err != nil || !res.IsError || res.StructuredContent != nil {
						t.Fatal("measured route accepted forged retained hash", res, err)
					}
				}
			})
		}
	}
}

func TestLabelSuspectsMeasuredResolverFailureRemainsUnresolved(t *testing.T) {
	f := suspectsFixture(t, false)
	f.ckg.SymbolIdentityErr = ckgclient.ErrSeedUnresolved
	f.deps.CKG = backendmeasure.WrapCKG(f.ckg)
	c := f.ckg.SymbolCitations[0]
	res, err := handleLabelSuspects(context.Background(), f.deps, callToolReq(map[string]any{"symptom": "unresolved site", "file": c.File, "line": c.StartLine}))
	if err != nil || !res.IsError || res.StructuredContent != nil {
		t.Fatal("unresolved site acquired a fallback identity", res, err)
	}
	raw, _ := json.Marshal(res)
	if !strings.Contains(string(raw), "seed_unresolved") || strings.Contains(string(raw), "site_resolver_unavailable") {
		t.Fatal("supported failing resolver was confused with unsupported capability")
	}
}

func TestBothContextHandlersSealEarlyErrorsWithCaptureID(t *testing.T) {
	for _, tool := range []string{ToolNameGetForTask, ToolNameGetForTaskV2} {
		var records []backendmeasure.Summary
		f := newFixture(t, nil)
		f.deps.BackendMeasurements = &backendmeasure.Recorder{Emit: func(_ context.Context, s backendmeasure.Summary) { records = append(records, s) }}
		req := mcpgo.CallToolRequest{}
		req.Params.Name = tool
		req.Params.Meta = &mcpgo.Meta{AdditionalFields: map[string]any{"cks.measurement_id": "0123456789abcdef0123456789abcdef"}}
		handler := handleGetForTask
		if tool == ToolNameGetForTaskV2 {
			handler = handleGetForTaskV2
		}
		result, err := handler(context.Background(), f.deps, req)
		if err != nil || result == nil || !result.IsError {
			t.Fatalf("early error changed: %+v, %v", result, err)
		}
		if len(records) != 1 || records[0].MeasurementID != "0123456789abcdef0123456789abcdef" || records[0].Outcome != "tool_error" || len(records[0].Calls) != 0 {
			t.Fatalf("lost error correlation for %s: %+v", tool, records)
		}
	}
}
