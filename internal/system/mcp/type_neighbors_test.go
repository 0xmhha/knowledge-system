package mcp

import (
	"context"
	"errors"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/system/ckgclient"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

// typeGraph answers uses_type lookups per anchor file and records them.
type typeGraph struct {
	*ckgclient.Fake
	edges map[string][]contract.Neighbor
	asked []ckgclient.NeighborsOpts
	fail  map[string]bool
}

func (g *typeGraph) Neighbors(_ context.Context, src contract.Citation, opts ckgclient.NeighborsOpts) ([]contract.Neighbor, error) {
	g.asked = append(g.asked, opts)
	if g.fail[src.File] {
		return nil, ckgclient.ErrSeedUnresolved
	}
	return g.edges[src.File], nil
}

func cite(file string, first, last int) contract.Citation {
	return contract.Citation{File: file, StartLine: first, EndLine: last}
}

func usesType(src, dst contract.Citation) contract.Neighbor {
	return contract.Neighbor{Source: src, Target: dst, Relation: contract.RelationUsesType, Distance: 1}
}

func TestTypeNeighborsInsertAfterAnchorWithinCaps(t *testing.T) {
	a, b, c, d := cite("a.go", 10, 20), cite("b.go", 5, 9), cite("c.go", 1, 4), cite("d.go", 1, 4)
	lockType, optsType, extraType, lateType := cite("a.go", 60, 63), cite("types.go", 3, 8), cite("types.go", 20, 22), cite("late.go", 1, 2)
	g := &typeGraph{Fake: &ckgclient.Fake{}, edges: map[string][]contract.Neighbor{
		"a.go": {usesType(a, cite("a.go", 12, 14)), usesType(a, lockType), usesType(a, extraType)}, // first overlaps a cited body
		"b.go": {usesType(b, optsType)},
		"c.go": {usesType(c, extraType)},
		"d.go": {usesType(d, lateType)},
	}}
	got := withTypeNeighbors(context.Background(), g, []contract.Citation{a, b, c, d})
	want := []contract.Citation{a, lockType, b, optsType, c, d}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("position %d: got %v want %v", i, got[i], want[i])
		}
	}
	for _, opts := range g.asked {
		if len(opts.Relations) != 1 || opts.Relations[0] != contract.RelationUsesType || opts.Hops != 1 || !opts.ExcludeTests {
			t.Fatalf("lookup was not a single-hop uses_type walk: %+v", opts)
		}
	}
	if len(g.asked) != 2 {
		t.Fatalf("lookups continued after the cap: %d", len(g.asked))
	}
}

func TestTypeNeighborsSkipNonGoFailuresAndOtherRelations(t *testing.T) {
	doc, a, b := cite("README.md", 1, 5), cite("a.go", 1, 9), cite("b.go", 1, 9)
	g := &typeGraph{Fake: &ckgclient.Fake{}, fail: map[string]bool{"a.go": true}, edges: map[string][]contract.Neighbor{
		"README.md": {usesType(doc, cite("x.go", 1, 2))},
		"b.go":      {{Source: b, Target: cite("y.go", 1, 2), Relation: contract.RelationCalls, Distance: 1}},
	}}
	if got := withTypeNeighbors(context.Background(), g, []contract.Citation{doc, a, b}); got != nil {
		t.Fatalf("unexpected extension: %v", got)
	}
	if len(g.asked) != 2 {
		t.Fatalf("non-Go anchor was looked up: %d lookups", len(g.asked))
	}
	if got := withTypeNeighbors(context.Background(), nil, []contract.Citation{a}); got != nil {
		t.Fatal("nil graph extended refs")
	}
	failing := &ckgclient.Fake{NeighborErr: errors.New("down")}
	if got := withTypeNeighbors(context.Background(), failing, []contract.Citation{a}); got != nil {
		t.Fatal("failed lookup extended refs")
	}
}
