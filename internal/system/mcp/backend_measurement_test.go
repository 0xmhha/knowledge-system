package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/0xmhha/knowledge-system/internal/setup"
	"github.com/0xmhha/knowledge-system/internal/system/backendmeasure"
	"github.com/0xmhha/knowledge-system/internal/system/ckgclient"
	"github.com/0xmhha/knowledge-system/internal/system/ckvclient"
	"github.com/0xmhha/knowledge-system/internal/system/composer/sanitize"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

func TestLabelSuspectsRequestMeasurementMatchesLogicalBudget(t *testing.T) {
	for _, mode := range []string{"symbol", "text", "invalid"} {
		t.Run(mode, func(t *testing.T) {
			f := suspectsFixture(t, false)
			var reports []backendmeasure.Summary
			f.deps.BackendMeasurements = &backendmeasure.Recorder{Emit: func(_ context.Context, r backendmeasure.Summary) { reports = append(reports, r) }}
			f.deps.CKG, f.deps.CKV = backendmeasure.WrapCKG(f.ckg), backendmeasure.WrapCKV(f.ckv)
			c := f.ckg.SymbolCitations[0]
			f.ckv.HealthVal = ckvclient.Health{Reachable: true, ModelReachable: true, IndexedHead: c.CommitHash}
			f.ckv.SearchHits = []contract.Hit{{Citation: c, Rank: 1, Score: .9, Source: contract.HitSourceCKV}}
			f.ckg.BM25Hits = []contract.Hit{{Citation: c, Rank: 1, Score: 37, Source: contract.HitSourceCKG}}
			args := map[string]any{"symptom": "PRIVATE_SYMPTOM", "budget_ms": 5000}
			switch mode {
			case "symbol":
				args["symbol"] = "Alpha"
			case "invalid":
				delete(args, "symptom")
			}
			req := callToolReq(args)
			req.Params.Name = ToolNameLabelSuspects
			req.Params.Meta = &mcpgo.Meta{AdditionalFields: map[string]any{"cks.measurement_id": "0123456789abcdef0123456789abcdef"}}
			res, err := handleLabelSuspects(context.Background(), f.deps, req)
			if err != nil || len(reports) != 1 || reports[0].MeasurementID != "0123456789abcdef0123456789abcdef" || reports[0].Tool != ToolNameLabelSuspects || reports[0].PendingCalls != 0 {
				t.Fatal("request measurement lost correlation/seal", res, err, reports)
			}
			r := reports[0]
			if mode == "invalid" {
				if !res.IsError || r.Outcome != "tool_error" || len(r.Calls) != 0 {
					t.Fatal("early error invented backend attempts", res, r)
				}
				return
			}
			if res.IsError || r.Outcome != "returned" {
				t.Fatal("valid measured request failed", res, r)
			}
			out := res.StructuredContent.(contract.SuspectCandidates)
			if len(r.Calls) != out.Budget.Calls {
				t.Fatal("logical attempts differ from enforced call budget", r, out.Budget)
			}
			methods := map[string]bool{}
			for _, c := range r.Calls {
				methods[c.Method] = true
			}
			for _, method := range []string{"health", "neighbors", "resolve_citation", "get_node_prs", "impact_analysis", "concurrency_impact"} {
				if !methods[method] {
					t.Errorf("missing logical method %s", method)
				}
			}
			raw, _ := json.Marshal(r)
			if strings.Contains(string(raw), "PRIVATE_SYMPTOM") || strings.Contains(string(raw), "Alpha") || strings.Contains(string(raw), "a.go") {
				t.Fatal("measurement released raw query/symbol/source")
			}
		})
	}
}

func TestLabelSuspectsCancelledVerificationIsNotSnapshotDrift(t *testing.T) {
	f := suspectsFixture(t, true)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	res, err := handleLabelSuspects(ctx, f.deps, callToolReq(map[string]any{"symptom": "cancel", "symbol": "Alpha"}))
	raw, _ := json.Marshal(res)
	if err != nil || !res.IsError || res.StructuredContent != nil || !strings.Contains(string(raw), "request_cancelled") || strings.Contains(string(raw), "snapshot_mismatch") || f.ckg.Calls.Health != 0 {
		t.Fatal("cancellation was mislabeled as source drift", res, err)
	}
}

func TestLabelSuspectsEvidenceBoundsHaveTypedRefusal(t *testing.T) {
	f := newFixture(t, nil)
	f.deps.EvidenceVersionDir = newPinnedV2FilesFixture(t, map[string]string{"a.go": "package fixture\nfunc Alpha() {}\n// " + strings.Repeat("x", 33000) + "\n"})
	id, err := setup.InspectVersionIdentity(f.deps.EvidenceVersionDir)
	if err != nil {
		t.Fatal(err)
	}
	c := contract.Citation{File: "a.go", StartLine: 1, EndLine: 3, CommitHash: id.Source.SourceCommit}
	f.ckg.HealthVal = ckgclient.Health{Reachable: true, IndexedHead: id.Source.SourceCommit, ProjectID: id.Source.ProjectID, DatasetID: id.DatasetID, SnapshotID: id.Source.SnapshotID, SourceMode: id.Source.SourceMode}
	f.ckg.SymbolCitations = []contract.Citation{c}
	f.ckg.SymbolIdentities = map[string]ckgclient.SymbolIdentity{c.String(): {Symbol: "Alpha", Citation: c}}
	f.deps.EvidenceSanitizer, err = sanitize.New(f.ruleset)
	if err != nil {
		t.Fatal(err)
	}
	res, err := handleLabelSuspects(context.Background(), f.deps, callToolReq(map[string]any{"symptom": "bounded evidence", "symbol": "Alpha"}))
	raw, _ := json.Marshal(res)
	if err != nil || !res.IsError || res.StructuredContent != nil || !strings.Contains(string(raw), "evidence_budget_exceeded") || strings.Contains(string(raw), "snapshot_mismatch") {
		t.Fatal("evidence bound was mislabeled as source drift", res, err)
	}
}

func TestSuspectVerificationRefusalsDoNotExposeOpaqueErrors(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want string
	}{
		{context.Canceled, "request_cancelled"},
		{context.DeadlineExceeded, "verification_budget_exhausted"},
		{errSuspectEvidenceBudget, "evidence_budget_exceeded"},
		{errSuspectSourcePolicy, "source_policy_denied"},
		{errors.New("PRIVATE_BACKEND"), "snapshot_mismatch"},
	} {
		got := suspectVerificationRefusal(context.Background(), tc.err, "snapshot_mismatch: retained verification failed")
		if !strings.HasPrefix(got, tc.want+":") || strings.Contains(got, "PRIVATE_") {
			t.Fatal("verification cause classification changed", got)
		}
	}
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	if got := suspectVerificationRefusal(ctx, errors.New("opaque"), "snapshot_mismatch"); !strings.HasPrefix(got, "verification_budget_exhausted:") {
		t.Fatal("request deadline mislabeled as data drift", got)
	}
}

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
