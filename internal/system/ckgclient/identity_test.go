package ckgclient

import (
	"context"
	"errors"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
	"testing"
)

func TestResolveCitationUsesDeclaredSymbolNotSmallASTStatement(t *testing.T) {
	fn := node("fn", "pkg.F", "a.go", 2, 20, types.NodeFunction, "go")
	fn.CanonicalID = "go:pkg.F"
	stmt := node("ret", "pkg.F.return", "a.go", 7, 7, types.NodeReturnStmt, "go")
	m := &mockStoreReader{manifest: ManifestSnapshot{SrcCommit: "head"}, pathNodes: []types.Node{stmt, fn}}
	r := &Real{s: m}
	got, e := r.ResolveCitation(context.Background(), contract.Citation{File: "a.go", StartLine: 7, EndLine: 7, CommitHash: "head"})
	if e != nil || got.Symbol != "go:pkg.F" || got.Citation.StartLine != 2 || got.Citation.EndLine != 20 {
		t.Fatalf("identity=%+v error=%v", got, e)
	}
}
func TestResolveCitationRejectsAmbiguousWideOrForeignLocations(t *testing.T) {
	one := node("one", "pkg.One", "a.go", 2, 5, types.NodeFunction, "go")
	two := node("two", "pkg.Two", "a.go", 8, 12, types.NodeFunction, "go")
	for _, tc := range []struct {
		name     string
		nodes    []types.Node
		citation contract.Citation
	}{
		{"wide", []types.Node{one, two}, contract.Citation{File: "a.go", StartLine: 1, EndLine: 15, CommitHash: "head"}},
		{"same_span", []types.Node{one, node("other", "other.One", "a.go", 2, 5, types.NodeFunction, "go")}, contract.Citation{File: "a.go", StartLine: 3, EndLine: 3, CommitHash: "head"}},
		{"foreign_file", []types.Node{one}, contract.Citation{File: "other.go", StartLine: 3, EndLine: 3, CommitHash: "head"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &Real{s: &mockStoreReader{manifest: ManifestSnapshot{SrcCommit: "head"}, pathNodes: tc.nodes}}
			_, e := r.ResolveCitation(context.Background(), tc.citation)
			if !errors.Is(e, ErrSeedUnresolved) {
				t.Fatalf("expected unresolved: %v", e)
			}
		})
	}
	r := &Real{s: &mockStoreReader{manifest: ManifestSnapshot{SrcCommit: "head"}, pathNodes: []types.Node{one}}}
	if _, e := r.ResolveCitation(context.Background(), contract.Citation{File: "a.go", StartLine: 3, EndLine: 3, CommitHash: "other"}); e == nil {
		t.Fatal("foreign revision accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := r.ResolveCitation(ctx, contract.Citation{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
func TestHealthPreservesFullGraphIdentity(t *testing.T) {
	r := &Real{s: &mockStoreReader{manifest: ManifestSnapshot{ProjectID: "p", DatasetID: "d", SnapshotID: "s", SourceMode: "working-tree", SrcCommit: "head", GraphDigest: "digest"}}}
	h, e := r.Health(context.Background())
	if e != nil || h.ProjectID != "p" || h.DatasetID != "d" || h.SnapshotID != "s" || h.SourceMode != "working-tree" || h.IndexedHead != "head" {
		t.Fatal(h, e)
	}
}

func TestNeighborsOneLineDeclarationWinsOverExactReturnStatement(t *testing.T) {
	fn := node("fn", "pkg.F", "a.go", 3, 3, types.NodeFunction, "go")
	stmt := node("ret", "pkg.F#ReturnStmt@35", "a.go", 3, 3, types.NodeReturnStmt, "go")
	if got := matchQname([]types.Node{stmt, fn}, contract.Citation{File: "a.go", StartLine: 3, EndLine: 3}); got != "pkg.F" {
		t.Fatal("AST storage order selected wrong seed", got)
	}
}
