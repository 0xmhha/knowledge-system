package main

import (
	"reflect"
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

func curveLoc(file string, start, end int) loc {
	return loc{File: file, StartLine: start, EndLine: end}
}

func TestFold(t *testing.T) {
	fn := newFileNodes(nil)
	fn.cache["a.go"] = []types.Node{
		node("f", types.NodeFunction, "f", 10, 20),
		node("v", types.NodeVariable, "v", 12, 12),
		node("s", types.NodeStruct, "S", 30, 40),
		node("fd", types.NodeField, "fd", 32, 32),
		node("c", types.NodeConstant, "C", 50, 50),
	}
	tests := []struct {
		name string
		in   loc
		want loc
		ok   bool
	}{
		{"local folds to its function", curveLoc("a.go", 12, 12), curveLoc("a.go", 10, 20), true},
		{"function stays", curveLoc("a.go", 10, 20), curveLoc("a.go", 10, 20), true},
		{"field folds to its struct", curveLoc("a.go", 32, 32), curveLoc("a.go", 30, 40), true},
		{"constant stays", curveLoc("a.go", 50, 50), curveLoc("a.go", 50, 50), true},
		{"outside any declaration", curveLoc("a.go", 60, 60), loc{}, false},
		{"test file", curveLoc("a_test.go", 1, 2), loc{}, false},
		{"no file", loc{}, loc{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok, err := folder{fn}.fold(tt.in)
			if err != nil || ok != tt.ok || (ok && got != tt.want) {
				t.Fatalf("fold(%v) = %v, %v, %v; want %v, %v", tt.in, got, ok, err, tt.want, tt.ok)
			}
		})
	}
}

func TestAlternate(t *testing.T) {
	a, b, c := curveLoc("a", 1, 1), curveLoc("b", 1, 1), curveLoc("c", 1, 1)
	got := alternate([]loc{a, b}, []loc{b, c}, nil)
	if want := []loc{a, b, c}; !reflect.DeepEqual(got, want) {
		t.Fatalf("alternate = %v, want %v", got, want)
	}
	if got := alternate(); len(got) != 0 {
		t.Fatalf("alternate() = %v, want empty", got)
	}
}

func TestCurveMeasure(t *testing.T) {
	s, a, b, x := curveLoc("a", 1, 10), curveLoc("a", 11, 15), curveLoc("b", 1, 1), curveLoc("x", 1, 1)
	list := readingList([]loc{s}, []loc{a, s, b})
	got := curveMeasure(list, []loc{b, s, x})
	want := []curveGold{{Rank: 3, Lines: 16}, {Rank: 1, Lines: 10}, {Rank: rankNotReached}}
	if got.Candidates != 3 || !reflect.DeepEqual(got.Gold, want) {
		t.Fatalf("curveMeasure = %+v, want candidates 3 gold %v", got, want)
	}
}

func TestCurveLinkNames(t *testing.T) {
	g, errG := curveGroup()
	w, errW := curveWeight()
	if errG != nil || errW != nil || linkArmName(g, w) != curveLink {
		t.Fatalf("got %s (%v, %v), want %s", linkArmName(g, w), errG, errW, curveLink)
	}
}
