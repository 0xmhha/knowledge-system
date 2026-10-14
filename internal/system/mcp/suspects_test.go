package mcp

import (
	"context"
	"encoding/json"
	"github.com/0xmhha/knowledge-system/internal/setup"
	"github.com/0xmhha/knowledge-system/internal/system/ckgclient"
	"github.com/0xmhha/knowledge-system/internal/system/composer/sanitize"
	"github.com/0xmhha/knowledge-system/internal/system/config"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func suspectsFixture(t *testing.T, pinned bool) *fixture {
	t.Helper()
	f := newFixture(t, nil)
	c := contract.Citation{File: "a.go", StartLine: 1, EndLine: 3, CommitHash: strings.Repeat("1", 40)}
	f.ckg.HealthVal.IndexedHead = c.CommitHash
	if pinned {
		f.deps.EvidenceVersionDir = newPinnedV2FilesFixture(t, map[string]string{"a.go": "package fixture\nfunc Alpha() {}\n// source anchor\n"})
		id, e := setup.InspectVersionIdentity(f.deps.EvidenceVersionDir)
		if e != nil {
			t.Fatal(e)
		}
		h := &f.ckg.HealthVal
		h.ProjectID = id.Source.ProjectID
		h.DatasetID = id.DatasetID
		h.SnapshotID = id.Source.SnapshotID
		h.SourceMode = id.Source.SourceMode
		h.IndexedHead = id.Source.SourceCommit
		c.CommitHash = id.Source.SourceCommit
		f.deps.EvidenceSanitizer, e = sanitize.New(f.ruleset)
		if e != nil {
			t.Fatal(e)
		}
	}
	f.ckg.SymbolCitations = []contract.Citation{c}
	f.ckg.SymbolIdentities = map[string]ckgclient.SymbolIdentity{c.String(): {Symbol: "Alpha", Citation: c}}
	return f
}
func TestLabelSuspectsPublicToolKeepsJudgmentsAndUnknownsSeparate(t *testing.T) {
	f := suspectsFixture(t, false)
	res, e := handleLabelSuspects(context.Background(), f.deps, callToolReq(map[string]any{"symptom": "hang", "symbol": "Alpha"}))
	if e != nil || res.IsError {
		t.Fatal(res, e)
	}
	out := res.StructuredContent.(contract.SuspectCandidates)
	if len(out.Candidates) != 1 || out.Candidates[0].Retained != nil || out.Candidates[0].Signals.Security.Value != nil || out.Candidates[0].Signals.Dataflow.Value != nil {
		t.Fatal(out)
	}
}
func TestLabelSuspectsPinnedSnapshotAndExpansionRoundTrip(t *testing.T) {
	f := suspectsFixture(t, true)
	req := callToolReq(map[string]any{"symptom": "hang", "symbol": "Alpha"})
	res, e := handleLabelSuspects(context.Background(), f.deps, req)
	if e != nil || res.IsError {
		t.Fatal(res, e)
	}
	out := res.StructuredContent.(contract.SuspectCandidates)
	c := out.Candidates[0]
	if out.Coordinates == nil || out.Coordinates.SourceMode != "snapshot-only" || c.Retained == nil || c.Retained.CommitHash != "" || len(c.Retained.ContentSHA256) != 64 || c.Signals.Distance.Provenance[0].Retained == nil {
		t.Fatal(out)
	}
	raw, _ := json.Marshal(res)
	if strings.Contains(string(raw), "package fixture") {
		t.Fatal("raw source escaped labels")
	}
	expRaw, _ := json.Marshal([]contract.CitationV2{*c.Retained})
	var exp []any
	if e = json.Unmarshal(expRaw, &exp); e != nil {
		t.Fatal(e)
	}
	res, e = handleLabelSuspects(context.Background(), f.deps, callToolReq(map[string]any{"symptom": "hang", "expand": exp}))
	if e != nil || res.IsError {
		t.Fatal("valid retained expansion rejected", res, e)
	}
	bad := *c.Retained
	bad.ContentSHA256 = strings.Repeat("0", 64)
	res, _ = handleLabelSuspects(context.Background(), f.deps, callToolReq(map[string]any{"symptom": "hang", "expand": []contract.CitationV2{bad}}))
	if !res.IsError {
		t.Fatal("forged content hash accepted")
	}
	bad = *c.Retained
	bad.DatasetID = "foreign"
	res, _ = handleLabelSuspects(context.Background(), f.deps, callToolReq(map[string]any{"symptom": "hang", "expand": []contract.CitationV2{bad}}))
	if !res.IsError {
		t.Fatal("foreign dataset accepted")
	}
	res, _ = handleLabelSuspects(context.Background(), f.deps, callToolReq(map[string]any{"symptom": "hang", "expand": []contract.Citation{c.Citation}}))
	if !res.IsError {
		t.Fatal("legacy expansion bypassed retained coordinates")
	}
}
func TestLabelSuspectsRejectsBackendIdentityAndArchiveDrift(t *testing.T) {
	for _, mode := range []string{"dataset", "source"} {
		t.Run(mode, func(t *testing.T) {
			f := suspectsFixture(t, true)
			if mode == "dataset" {
				f.ckg.HealthVal.DatasetID = "foreign"
			} else {
				raw, e := os.ReadFile(filepath.Join(f.deps.EvidenceVersionDir, "sources", "manifest.json"))
				if e != nil {
					t.Fatal(e)
				}
				// Manifest corruption must fail before any backend observation.
				if e = os.WriteFile(filepath.Join(f.deps.EvidenceVersionDir, "sources", "manifest.json"), append(raw, 'x'), 0600); e != nil {
					t.Fatal(e)
				}
			}
			res, e := handleLabelSuspects(context.Background(), f.deps, callToolReq(map[string]any{"symptom": "hang", "symbol": "Alpha"}))
			if e != nil || !res.IsError {
				t.Fatal("changed evidence published", res, e)
			}
		})
	}
}
func TestLabelSuspectsHonorsSourcePolicyBeforePublishingAnchors(t *testing.T) {
	f := suspectsFixture(t, true)
	rules := &config.SanitizeRuleset{Version: 1, Rules: []config.SanitizeRule{{ID: "DENY", Pattern: "package", Action: contract.RedactionDrop, Severity: config.SeverityHigh}}}
	if e := rules.Validate(); e != nil {
		t.Fatal(e)
	}
	cleaner, e := sanitize.New(rules)
	if e != nil {
		t.Fatal(e)
	}
	f.deps.EvidenceSanitizer = cleaner
	res, e := handleLabelSuspects(context.Background(), f.deps, callToolReq(map[string]any{"symptom": "hang", "symbol": "Alpha"}))
	if e != nil || !res.IsError || res.StructuredContent != nil {
		t.Fatal("policy-denied source published", res, e)
	}
}
func TestLabelSuspectsStrictInputs(t *testing.T) {
	for _, args := range []map[string]any{
		{"symptom": "hang", "depth": 1.5}, {"symptom": "hang", "line": "3"}, {"symptom": "hang", "max_calls": true}, {"symptom": "hang", "max_candidates": 0}, {"symptom": "hang", "expand": "bad"}, {"symptom": "hang", "unknown": 1}, {"symptom": "hang", "expand": []map[string]any{{"file": "a.go", "start_line": 1, "end_line": 1, "extra": "bad"}}},
	} {
		f := suspectsFixture(t, false)
		res, e := handleLabelSuspects(context.Background(), f.deps, callToolReq(args))
		if e != nil || !res.IsError || f.ckg.Calls.Health != 0 {
			t.Fatalf("invalid input touched backend: %v %v", args, e)
		}
	}
}

func TestLabelSuspectsRefusesUnresolvedSeed(t *testing.T) {
	f := suspectsFixture(t, false)
	f.ckg.SymbolCitations = nil
	res, e := handleLabelSuspects(context.Background(), f.deps, callToolReq(map[string]any{"symptom": "hang", "symbol": "missing"}))
	if e != nil || !res.IsError || res.StructuredContent != nil {
		t.Fatal("unresolved seed accepted", res, e)
	}
}
