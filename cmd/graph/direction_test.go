package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// directionGraph is run -calls-> newCallTracer -writes_field-> callstack.
func directionGraph() ([]types.Node, []types.Edge) {
	nodes := []types.Node{
		{ID: "n1", Type: types.NodeField, Name: "callstack", QualifiedName: "native.callTracer.callstack", FilePath: "native/call.go", StartLine: 10},
		{ID: "n2", Type: types.NodeFunction, Name: "newCallTracer", QualifiedName: "native.newCallTracer", FilePath: "native/call.go", StartLine: 20},
		{ID: "n3", Type: types.NodeFunction, Name: "run", QualifiedName: "native.run", FilePath: "native/run.go", StartLine: 5},
	}
	edges := []types.Edge{
		{Src: "n2", Dst: "n1", Type: types.EdgeWritesField},
		{Src: "n3", Dst: "n2", Type: types.EdgeCalls},
	}
	return nodes, edges
}

// TestPathArrowFollowsRelation: the path search walks edges both ways, and
// each hop is drawn in the direction of its relation. Every arrow used to
// point from the first argument onward, so 143 of 358 hops on go-stablenet
// read backwards ("callstack —[writes_field]→ newCallTracer").
func TestPathArrowFollowsRelation(t *testing.T) {
	nodes, edges := directionGraph()
	for _, tc := range []struct {
		from, to string
		want     []string
	}{
		{"n1", "n3", []string{
			"1. [Field] callstack",
			pathArrowUp + " [writes_field]",
			"2. [Function] newCallTracer",
			pathArrowUp + " [calls]",
			"3. [Function] run",
		}},
		{"n3", "n1", []string{
			"1. [Function] run",
			pathArrowDown + " [calls]",
			"2. [Function] newCallTracer",
			pathArrowDown + " [writes_field]",
			"3. [Field] callstack",
		}},
	} {
		path, hops := bfsShortestPath(nodes, edges, tc.from, tc.to, 12)
		var out bytes.Buffer
		printPath(&out, nodes, path, hops)
		text := out.String()
		if !strings.Contains(text, "Path (2 hops)") {
			t.Fatalf("%s to %s: no 2-hop path:\n%s", tc.from, tc.to, text)
		}
		at := 0
		for _, want := range tc.want {
			i := strings.Index(text[at:], want)
			if i < 0 {
				t.Fatalf("%s to %s: %q missing or out of order in:\n%s", tc.from, tc.to, want, text)
			}
			at += i + len(want)
		}
	}
}

// TestQueryArrowFollowsRelation: a neighbourhood line names the relation in
// its own direction, whichever end the walk came from (1,481 lines on
// go-stablenet were printed seed —[kind]→ neighbour although the neighbour
// has the relation to the seed).
func TestQueryArrowFollowsRelation(t *testing.T) {
	nodes, edges := directionGraph()
	for _, tc := range []struct {
		seed int
		want []string
	}{
		{0, []string{"- `callstack` ←[writes_field]— `newCallTracer`", "- `newCallTracer` ←[calls]— `run`"}},
		{2, []string{"- `run` —[calls]→ `newCallTracer`", "- `newCallTracer` —[writes_field]→ `callstack`"}},
	} {
		var out bytes.Buffer
		renderQueryAnswer(&out, "q", nodes[tc.seed:tc.seed+1], nodes, buildAdjacency(edges), buildEdgeIndex(edges), 2, 2000)
		for _, want := range tc.want {
			if !strings.Contains(out.String(), want) {
				t.Errorf("seed %s: %q missing in:\n%s", nodes[tc.seed].Name, want, out.String())
			}
		}
	}
}
