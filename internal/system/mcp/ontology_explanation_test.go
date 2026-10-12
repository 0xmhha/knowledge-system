package mcp

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/0xmhha/knowledge-system/internal/system/composer/sanitize"
	"github.com/0xmhha/knowledge-system/internal/system/composer/stage2"
	"github.com/0xmhha/knowledge-system/internal/system/config"
	"github.com/0xmhha/knowledge-system/internal/system/evidencev2"
	"github.com/0xmhha/knowledge-system/internal/system/semantic"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

func TestOntologyExplanationIsMCPOptInInput(t *testing.T) {
	s := mcpserver.NewMCPServer("explain-test", "test")
	if err := Register(s, newFixture(t, nil).deps); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.GetTool(ToolNameGetForTaskV2).Tool.InputSchema.Properties["explain_ontology"]; !ok {
		t.Fatal("MCP cannot request ontology effect explanation")
	}
}

// This controlled provider is a ranking-mechanism fixture, not a source-
// validated real ontology projection or an independent quality evaluation.
type explanationProvider struct{}

func (explanationProvider) Resolve(context.Context) (stage2.OntologyResolver, error) {
	return explanationResolver{}, nil
}

type explanationResolver struct{}

func (explanationResolver) Snapshot() semantic.Snapshot { return semantic.Snapshot{} }
func (explanationResolver) MatchConcepts(string, int) ([]semantic.ConceptCandidate, error) {
	return []semantic.ConceptCandidate{{ConceptID: "feature", Status: semantic.StatusVerified}}, nil
}
func (explanationResolver) VerifiedImplementations(string) []semantic.CodeImplementation {
	return []semantic.CodeImplementation{{AssertionID: "owned-relation", CanonicalID: "pkg.B", Evidence: semantic.EvidenceSpan{Snapshot: semantic.Snapshot{}, Path: "README.md", StartLine: 2, EndLine: 2}}}
}
func TestV2OntologyExplanationShowsControlledRankChangeAndOff(t *testing.T) {
	version := newPinnedV2ContentFixture(t, strings.Repeat("fixture source\n", 20))
	makeDeps := func(on bool) Deps {
		options := []stage2.Option{}
		if on {
			options = append(options, stage2.WithOntologyProvider(explanationProvider{}, .2, time.Second))
		}
		f := newFixture(t, func(f *fixture) {
			a := contract.Citation{File: "README.md", StartLine: 1, EndLine: 1}
			b := contract.Citation{File: "README.md", StartLine: 2, EndLine: 2}
			f.ckv.SearchHits = []contract.Hit{{Citation: a, CanonicalID: "pkg.A", Rank: 1, Source: contract.HitSourceCKV}, {Citation: b, CanonicalID: "pkg.B", Rank: 2, Source: contract.HitSourceCKV}}
			f.fetcher.Bodies[a.Key()] = "fixture source\n"
			f.fetcher.Bodies[b.Key()] = "fixture source\n"
		}, options...)
		cleaner, err := sanitize.New(f.ruleset)
		if err != nil {
			t.Fatal(err)
		}
		f.deps.EvidenceVersionDir = version
		f.deps.EvidenceSanitizer = cleaner
		return f.deps
	}
	for _, on := range []bool{false, true} {
		d := makeDeps(on)
		result, err := handleGetForTaskV2(context.Background(), d, callToolReq(map[string]any{"prompt": "feature", "explain_ontology": true}))
		if err != nil || result.IsError {
			t.Fatalf("explain failed: %+v %v", result, err)
		}
		p := result.StructuredContent.(contract.EvidencePackV2)
		e := p.Metadata.Ontology.Explanation
		if e == nil || e.QualityEvaluated {
			t.Fatalf("missing/false quality explanation: %+v", e)
		}
		if !on {
			if e.State != "off" {
				t.Fatal(e)
			}
		} else {
			if e.State != "applied" || e.TotalChanged != 2 || len(e.Matches) != 1 {
				t.Fatalf("rank mechanism unexplained: %+v", e)
			}
			gain := false
			for _, c := range e.Changes {
				before, _ := strconv.ParseFloat(c.ScoreBefore, 64)
				after, _ := strconv.ParseFloat(c.ScoreAfter, 64)
				if len(c.RelationIDs) == 1 && c.RelationIDs[0] == "owned-relation" && c.RankBefore == 2 && c.RankAfter == 1 && c.SelectedByComposer && after > before {
					gain = true
				}
			}
			if !gain {
				t.Fatal(e)
			}
		}
		if err := evidencev2.Verify(p); err != nil {
			t.Fatal(err)
		}
	}
	plain, err := handleGetForTaskV2(context.Background(), makeDeps(false), callToolReq(map[string]any{"prompt": "feature"}))
	if err != nil || plain.IsError {
		t.Fatal(err)
	}
	if plain.StructuredContent.(contract.EvidencePackV2).Metadata.Ontology != nil {
		t.Fatal("default response changed without an explanation request")
	}
	// Redaction of an explanation ID must not publish a partial reason or
	// drop the already sanitized retained evidence pack.
	d := makeDeps(true)
	rules, err := config.LoadSanitizeRulesetBytes([]byte("version: 1\nrules:\n  - id: hide_relation\n    description: controlled ID policy\n    pattern: owned-relation\n    action: drop\n    severity: high\n"))
	if err != nil {
		t.Fatal(err)
	}
	d.EvidenceSanitizer, err = sanitize.New(rules)
	if err != nil {
		t.Fatal(err)
	}
	result, err := handleGetForTaskV2(context.Background(), d, callToolReq(map[string]any{"prompt": "feature", "explain_ontology": true}))
	if err != nil || result.IsError {
		t.Fatal(result, err)
	}
	p := result.StructuredContent.(contract.EvidencePackV2)
	e := p.Metadata.Ontology.Explanation
	if len(e.Changes) != 0 || len(e.Matches) != 0 || e.State != "restricted" || len(p.Bodies) == 0 {
		t.Fatalf("unsafe or lost explanation fallback: %+v", e)
	}
	if err := evidencev2.Verify(p); err != nil {
		t.Fatal(err)
	}
}
