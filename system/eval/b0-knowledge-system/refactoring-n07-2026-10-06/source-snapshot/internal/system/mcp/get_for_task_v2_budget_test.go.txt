package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/system/composer/sanitize"
	"github.com/0xmhha/knowledge-system/internal/system/evidencev2"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

func TestV2ToolBudgetPartialAndTypedErrorPreserveDenominators(t *testing.T) {
	for _, fitting := range []bool{false, true} {
		t.Run(map[bool]string{false: "budget-error", true: "partial"}[fitting], func(t *testing.T) {
			content := strings.Repeat("x", 33000) + "\nsafe\n"
			version := newPinnedV2ContentFixture(t, content)
			f := newFixture(t, func(f *fixture) {
				ref := contract.Citation{File: "README.md", StartLine: 1, EndLine: 1}
				f.ckv.SearchHits = []contract.Hit{{Citation: ref, Score: 1, Rank: 1, Symbol: "Alpha", Source: contract.HitSourceCKV}}
				f.ckg.SymbolCitations = []contract.Citation{ref}
				f.fetcher.Bodies[ref.Key()] = "Alpha metadata excerpt\n"
				if fitting {
					small := contract.Citation{File: "README.md", StartLine: 2, EndLine: 2}
					f.ckv.SearchHits = append(f.ckv.SearchHits, contract.Hit{Citation: small, Score: .9, Rank: 2, Symbol: "Alpha", Source: contract.HitSourceCKV})
					f.fetcher.Bodies[small.Key()] = "Alpha safe\n"
				}
			})
			cleaner, err := sanitize.New(f.ruleset)
			if err != nil {
				t.Fatal(err)
			}
			f.deps.EvidenceVersionDir, f.deps.EvidenceSanitizer = version, cleaner
			result, err := handleGetForTaskV2(context.Background(), f.deps, callToolReq(map[string]any{"prompt": "Alpha"}))
			if err != nil {
				t.Fatal(err)
			}
			public := publicMaintenanceCall(t, f.deps, ToolNameGetForTaskV2, map[string]any{"prompt": "Alpha"})
			publicBody, _ := json.Marshal(public)
			if !fitting {
				if public["isError"] != true || public["structuredContent"].(map[string]any)["code"] != "budget_exceeded" {
					t.Fatalf("public budget error missing: %s", publicBody)
				}
			} else {
				if public["isError"] == true || public["structuredContent"].(map[string]any)["evidence_state"] != "partial" {
					t.Fatalf("public partial missing: %s", publicBody)
				}
			}
			t.Logf("registered public response fitting=%v: %s", fitting, publicBody)
			wire, _ := json.Marshal(result)
			if strings.Contains(string(wire), strings.Repeat("x", 100)) || strings.Contains(string(wire), version) {
				t.Fatal("raw oversized/private data exposed")
			}
			if !fitting {
				if !result.IsError || result.StructuredContent.(map[string]string)["code"] != "budget_exceeded" {
					t.Fatalf("lost error denominator: %+v", result)
				}
				return
			}
			if result.IsError {
				t.Fatalf("fitting candidate lost: %+v", result)
			}
			pack := result.StructuredContent.(contract.EvidencePackV2)
			if pack.EvidenceState != "partial" || pack.Metadata.EvidenceBudget == nil || len(pack.Bodies) != 1 || pack.Bodies[0].Text != "safe\n" || pack.Citations[0].StartLine != 2 {
				t.Fatalf("lost partial denominator: %+v", pack)
			}
			if err := evidencev2.Verify(pack); err != nil {
				t.Fatal(err)
			}
		})
	}
}
