package persist_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/graph/persist"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// TestFindByCanonicalID covers the exact, unambiguous lookup added for
// symbol-identity Phase 1 (ADR-0001): a hit returns the one node, a miss and an
// empty input both return found=false with no error.
func TestFindByCanonicalID(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "canonical.db")
	s, err := persist.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := s.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	nodes := []types.Node{
		{
			ID: "cid0000000000001", Type: types.NodeMethod,
			Name: "Size", QualifiedName: "core.Set.Size",
			CanonicalID: "example.com/core.(*Set).Size",
			FilePath:    "core/set.go", Language: "go",
			Confidence: types.ConfExtracted,
		},
		{
			ID: "cid0000000000002", Type: types.NodeMethod,
			Name: "Size", QualifiedName: "wbft.Other.Size",
			CanonicalID: "example.com/consensus/wbft.(*Other).Size",
			FilePath:    "wbft/other.go", Language: "go",
			Confidence: types.ConfExtracted,
		},
		{
			ID: "cid0000000000003", Type: types.NodeFunction,
			Name: "builtin", QualifiedName: "p.builtin",
			// no canonical id (mirrors builtins / AST-only mode)
			FilePath: "p/p.go", Language: "go", Confidence: types.ConfExtracted,
		},
	}
	if err := s.InsertNodes(nodes); err != nil {
		t.Fatalf("InsertNodes: %v", err)
	}

	// exact hit on a colliding short name resolves to exactly one node.
	n, found, err := s.FindByCanonicalID("example.com/consensus/wbft.(*Other).Size")
	if err != nil {
		t.Fatalf("FindByCanonicalID: %v", err)
	}
	if !found {
		t.Fatalf("expected to find wbft Other.Size")
	}
	if n.ID != "cid0000000000002" || n.QualifiedName != "wbft.Other.Size" {
		t.Errorf("resolved wrong node: id=%q qname=%q", n.ID, n.QualifiedName)
	}

	// miss → not found, no error.
	if _, found, err := s.FindByCanonicalID("example.com/does/not.Exist"); err != nil || found {
		t.Errorf("miss: found=%v err=%v, want found=false err=nil", found, err)
	}

	// empty input → not found, no error (never matches the empty-canonical node).
	if _, found, err := s.FindByCanonicalID(""); err != nil || found {
		t.Errorf("empty input: found=%v err=%v, want found=false err=nil", found, err)
	}
}

func TestCanonicalIDsSurviveSharedReadPathsAndExport(t *testing.T) {
	s, err := persist.Open(filepath.Join(t.TempDir(), "canonical.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := s.Migrate(); err != nil {
		t.Fatal(err)
	}
	nodes := []types.Node{
		{ID: "retained00000001", Type: types.NodeFunction, Name: "Retained", QualifiedName: "pkg.보존.Retained",
			CanonicalID: "example.com/pkg.Retained", FilePath: "pkg/retained.go", StartLine: 2, EndLine: 4,
			Language: "go", DocComment: "보존", Confidence: types.ConfExtracted},
		{ID: "legacy0000000001", Type: types.NodeFunction, Name: "Legacy", QualifiedName: "pkg.Legacy",
			FilePath: "pkg/retained.go", StartLine: 6, EndLine: 8, Language: "go", Confidence: types.ConfExtracted},
	}
	if err := s.InsertNodes(nodes); err != nil {
		t.Fatal(err)
	}
	if err := s.RebuildFTS(); err != nil {
		t.Fatal(err)
	}
	// Establish that the identity is stored before exercising the shared scanners.
	if n, found, err := s.FindByCanonicalID(nodes[0].CanonicalID); err != nil || !found || n.ID != nodes[0].ID {
		t.Fatalf("stored canonical lookup: %+v, %v, %v", n, found, err)
	}
	check := func(t *testing.T, got []types.Node, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, n := range got {
			if n.ID == nodes[0].ID {
				found = true
				if n.CanonicalID != nodes[0].CanonicalID {
					t.Fatalf("stored identity lost on read: %q", n.CanonicalID)
				}
			}
			if n.ID == nodes[1].ID && n.CanonicalID != "" {
				t.Fatalf("legacy node acquired canonical identity: %q", n.CanonicalID)
			}
		}
		if !found {
			t.Fatal("expected canonical symbol missing")
		}
	}
	for name, read := range map[string]func() ([]types.Node, error){
		"file-cache": func() ([]types.Node, error) { return s.NodesByFilePath(nodes[0].FilePath) },
		"IDs":        func() ([]types.Node, error) { return s.NodesByIDs([]string{nodes[0].ID, nodes[1].ID}) },
		"symbol": func() ([]types.Node, error) {
			return s.FindSymbol(nodes[0].QualifiedName, true, persist.FindSymbolOptions{})
		},
		"ranked": func() ([]types.Node, error) { return s.TopNodes("pagerank", 10) },
		"search": func() ([]types.Node, error) { return s.Search("Retained", 10) },
		"CJK":    func() ([]types.Node, error) { return s.Search("보존", 10) },
	} {
		t.Run(name, func(t *testing.T) { got, err := read(); check(t, got, err) })
	}
	t.Run("FTS", func(t *testing.T) {
		hits, err := s.SearchFTS("Retained", 10, persist.SearchFTSOptions{})
		got := make([]types.Node, len(hits))
		for i, hit := range hits {
			got[i] = hit.Node
		}
		check(t, got, err)
	})
	t.Run("export", func(t *testing.T) {
		out := t.TempDir()
		if err := s.ExportChunked(out, 10, 10); err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(filepath.Join(out, "nodes", "chunk_0000.json"))
		if err != nil {
			t.Fatal(err)
		}
		var got []types.Node
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatal(err)
		}
		check(t, got, nil)
	})
}
