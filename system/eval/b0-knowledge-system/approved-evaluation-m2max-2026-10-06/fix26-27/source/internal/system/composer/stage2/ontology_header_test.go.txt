package stage2

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/0xmhha/knowledge-system/internal/system/ckgclient"
	"github.com/0xmhha/knowledge-system/internal/system/ckvclient"
	"github.com/0xmhha/knowledge-system/internal/system/semantic"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

// A file header can include a short function and inherit its canonical ID.
// Giving both spans an ontology contribution can move a normative document
// below a second copy of the same implementation, even without new candidates.
func TestOntologyDoesNotDoubleBoostHeaderOverPolicy(t *testing.T) {
	commit := strings.Repeat("a", 40)
	snapshot := semantic.Snapshot{Commit: commit}
	symbol := contract.Hit{Citation: contract.Citation{File: "main.go", StartLine: 3, EndLine: 3, CommitHash: commit}, CanonicalID: "pkg.Limit", ChunkKind: "symbol"}
	policy := contract.Hit{Citation: contract.Citation{File: "README.md", StartLine: 1, EndLine: 3, CommitHash: commit}, ChunkKind: "doc"}
	header := symbol
	header.Citation.StartLine, header.ChunkKind = 1, "file_header"
	hits := []contract.Hit{symbol, policy, header}
	for _, mode := range []string{"relations", "concept_text", "combined"} {
		t.Run(mode, func(t *testing.T) {
			relationCalls := 0
			resolver := textOntology{fakeOntology: fakeOntology{snapshot: snapshot,
				candidates:      []semantic.ConceptCandidate{{ConceptID: "limit", Status: semantic.StatusVerified}},
				implementations: map[string][]semantic.CodeImplementation{"limit": {{AssertionID: "r1", CanonicalID: symbol.CanonicalID, Evidence: semantic.EvidenceSpan{Snapshot: snapshot, Path: "main.go", StartLine: 3, EndLine: 3}}}}},
				text: semantic.ConceptText{ConceptID: "limit", Text: "Reviewed limit definition", ReviewedBy: "reviewer", Evidence: semantic.EvidenceSpan{Snapshot: snapshot, Kind: semantic.SourceDocument}}, relationCalls: &relationCalls}
			opts := []Option{WithOntologyProvider(providerFunc(func(context.Context) (OntologyResolver, error) { return resolver, nil }), .2, time.Second)}
			if mode != "relations" {
				opts = append(opts, WithOntologyTextSearch(&ckvclient.Fake{SearchHits: []contract.Hit{header, symbol}}, 10, mode == "combined"))
			}
			searcher, err := New(&ckgclient.Fake{}, opts...)
			if err != nil {
				t.Fatal(err)
			}
			agg := newAggregator(60, 1, 1, 5)
			agg.addCkvList(hits)
			for i, h := range hits {
				agg.byCitation[h.Citation.Key()].Score = []float64{1, .95, .90}[i]
			}
			beforeKeys := keys(agg)
			baseline := agg.results(0, false, false)
			out, _, diag := searcher.applyProvidedOntology(context.Background(), agg, "limit", hits, baseline, false, false)
			if diag.State != "active" || len(out) != 3 || out[1].Citation != policy.Citation || agg.byCitation[header.Citation.Key()].Score != .90 {
				t.Fatalf("duplicate implementation displaced policy: %+v; %+v", out, diag)
			}
			if !reflect.DeepEqual(keys(agg), beforeKeys) || out[0].Score > 1.2 || diag.BoostedCitations != 1 {
				t.Fatalf("candidate set, boost cap or attribution changed: %+v; %+v", out, diag)
			}
			if mode == "concept_text" && (relationCalls != 0 || diag.AppliedRelations != 0) {
				t.Fatal("text arm queried implementation relations")
			}
		})
	}
}

func TestOntologyHeaderFallbackKeepsOnlyAvailableOrDistinctEvidence(t *testing.T) {
	header := contract.Hit{Citation: contract.Citation{File: "main.go", StartLine: 1, EndLine: 50, CommitHash: strings.Repeat("a", 40)}, CanonicalID: "pkg.Limit", ChunkKind: "file_header"}
	peer := header
	peer.Citation.StartLine, peer.Citation.EndLine, peer.ChunkKind = 3, 100, "symbol"
	for _, tc := range []struct {
		name     string
		peer     contract.Hit
		allowed  bool
		shadowed bool
	}{
		{"overlapping symbol", peer, true, true},
		{"outside cap", peer, false, false},
		{"different canonical", func() contract.Hit { h := peer; h.CanonicalID = "pkg.Other"; return h }(), true, false},
		{"different commit", func() contract.Hit { h := peer; h.Citation.CommitHash = strings.Repeat("b", 40); return h }(), true, false},
		{"different file", func() contract.Hit { h := peer; h.Citation.File = "other.go"; return h }(), true, false},
		{"nonoverlapping", func() contract.Hit { h := peer; h.Citation.StartLine = 51; return h }(), true, false},
		{"split function", func() contract.Hit { h := peer; h.ChunkKind = "function_split"; return h }(), true, true},
		{"legacy untyped", func() contract.Hit { h := peer; h.ChunkKind = ""; return h }(), true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			allowed := map[string]bool{header.Citation.Key(): true, tc.peer.Citation.Key(): tc.allowed}
			if got := ontologyHeaderShadowed(header, []contract.Hit{header, tc.peer}, allowed); got != tc.shadowed {
				t.Fatalf("shadowed=%v want %v", got, tc.shadowed)
			}
			if ontologyHeaderShadowed(tc.peer, []contract.Hit{header, tc.peer}, allowed) {
				t.Fatal("symbol or split was suppressed")
			}
		})
	}
	if ontologyHeaderShadowed(header, []contract.Hit{header}, nil) {
		t.Fatal("header-only recall suppressed")
	}
	header.CanonicalID = ""
	if ontologyHeaderShadowed(header, []contract.Hit{header, peer}, nil) {
		t.Fatal("unidentified header suppressed")
	}
}
