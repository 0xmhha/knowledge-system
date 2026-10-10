package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/system/ckgclient"
	"github.com/0xmhha/knowledge-system/internal/system/composer/sanitize"
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
	got := withTypeNeighbors(context.Background(), g, []contract.Citation{a, b, c, d}, nil)
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
	if got := withTypeNeighbors(context.Background(), g, []contract.Citation{doc, a, b}, nil); got != nil {
		t.Fatalf("unexpected extension: %v", got)
	}
	if len(g.asked) != 2 {
		t.Fatalf("non-Go anchor was looked up: %d lookups", len(g.asked))
	}
	if got := withTypeNeighbors(context.Background(), nil, []contract.Citation{a}, nil); got != nil {
		t.Fatal("nil graph extended refs")
	}
	failing := &ckgclient.Fake{NeighborErr: errors.New("down")}
	if got := withTypeNeighbors(context.Background(), failing, []contract.Citation{a}, nil); got != nil {
		t.Fatal("failed lookup extended refs")
	}
}

func TestTypeDeclarationIncludesItsDocComment(t *testing.T) {
	src := "package p\n\n// unrelated\n\n// lock serializes writers.\n// Owned by the OS.\ntype lock struct{}\n"
	files := map[string][]byte{"p.go": []byte(src)}
	read := func(f string) ([]byte, error) {
		if b, ok := files[f]; ok {
			return b, nil
		}
		return nil, errors.New("missing")
	}
	if got := withDocComment(cite("p.go", 7, 7), read); got.StartLine != 5 || got.EndLine != 7 {
		t.Fatalf("doc comment not included: %+v", got)
	}
	for _, c := range []contract.Citation{cite("p.go", 3, 3), cite("missing.go", 7, 7), cite("p.go", 1, 1)} {
		if got := withDocComment(c, read); got != c {
			t.Fatalf("span changed without an adjacent comment: %+v -> %+v", c, got)
		}
	}
	if got := withDocComment(cite("p.go", 7, 7), nil); got.StartLine != 7 {
		t.Fatal("nil source changed the span")
	}
	long := "package p\n" + strings.Repeat("// x\n", 40) + "type t int\n"
	files["long.go"] = []byte(long)
	if got := withDocComment(cite("long.go", 42, 42), read); got.StartLine != 42-maxDocCommentLines {
		t.Fatalf("comment extension not capped: %+v", got)
	}
}

func TestTypeDeclarationCommentOverlappingACitedBodyIsSkipped(t *testing.T) {
	src := "package p\n// header\n// lock doc\ntype lock struct{}\ntype other int\n"
	read := func(string) ([]byte, error) { return []byte(src), nil }
	header, fn := cite("p.go", 1, 2), cite("f.go", 1, 3)
	g := &typeGraph{Fake: &ckgclient.Fake{}, edges: map[string][]contract.Neighbor{
		"f.go": {usesType(fn, cite("p.go", 4, 4)), usesType(fn, cite("p.go", 5, 5))},
	}}
	got := withTypeNeighbors(context.Background(), g, []contract.Citation{fn, header}, read)
	if len(got) != 3 || got[1] != cite("p.go", 5, 5) {
		t.Fatalf("declaration whose comment overlaps a cited body was added: %v", got)
	}
}

// The v2 handler adds used type declarations only when the option is on.
func TestV2ToolTypeNeighborsAreOptIn(t *testing.T) {
	src := "package p\n\n// limit bounds one request.\ntype limit struct{ n int }\n\nfunc Alpha(l limit) int {\n\treturn l.n\n}\n"
	version := newPinnedV2FilesFixture(t, map[string]string{"a.go": src})
	anchor, decl := cite("a.go", 6, 8), cite("a.go", 4, 4)
	for _, enabled := range []bool{false, true} {
		f := newFixture(t, func(f *fixture) {
			f.ckv.SearchHits = []contract.Hit{{Citation: anchor, Score: 1, Rank: 1, Symbol: "Alpha", Source: contract.HitSourceCKV}}
			f.ckg.SymbolCitations = []contract.Citation{anchor}
			f.ckg.NeighborEdges = []contract.Neighbor{usesType(anchor, decl)}
			f.fetcher.Bodies[anchor.Key()] = "func Alpha(l limit) int {\n\treturn l.n\n}\n"
		})
		cleaner, err := sanitize.New(f.ruleset)
		if err != nil {
			t.Fatal(err)
		}
		f.deps.EvidenceVersionDir, f.deps.EvidenceSanitizer, f.deps.TypeNeighbors = version, cleaner, enabled
		result, err := handleGetForTaskV2(context.Background(), f.deps, callToolReq(map[string]any{"prompt": "Alpha"}))
		if err != nil || result.IsError {
			t.Fatalf("enabled=%v: %v %+v", enabled, err, result)
		}
		pack := result.StructuredContent.(contract.EvidencePackV2)
		var files []string
		for _, c := range pack.Citations {
			files = append(files, fmt.Sprintf("%s:%d-%d", c.File, c.StartLine, c.EndLine))
		}
		asked := 0
		for _, call := range f.ckg.Calls.Neighbors {
			for _, r := range call.Opts.Relations {
				if r == contract.RelationUsesType {
					asked++
				}
			}
		}
		want := []string{"a.go:6-8"}
		if enabled {
			want = append(want, "a.go:3-4")
		}
		if strings.Join(files, ",") != strings.Join(want, ",") || (asked > 0) != enabled {
			t.Fatalf("enabled=%v: citations %v, uses_type lookups %d", enabled, files, asked)
		}
	}
}
