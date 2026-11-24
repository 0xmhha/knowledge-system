package main

import (
	"cmp"
	"slices"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// baselineTypes are the declarations the emit_file baseline lists after the
// emitting functions.
var baselineTypes = map[types.NodeType]bool{
	types.NodeFunction: true, types.NodeMethod: true, types.NodeStruct: true,
	types.NodeVariable: true, types.NodeConstant: true,
}

// emitFile is the baseline of CONFIRM.md: the emitting functions, inner message
// part first, then the other declarations of their files by line distance. It
// uses no graph edge except the references that lead from a package-level error
// value to its users, which rule E uses too.
func (t *tracer) emitFile(parts [][]position) []result {
	var out []result
	listed := map[[2]any]bool{}
	list := func(n types.Node) {
		key := [2]any{n.FilePath, n.StartLine}
		if !listed[key] {
			listed[key] = true
			out = append(out, result{Rank: len(out) + 1, ID: n.ID, QName: n.QualifiedName, File: n.FilePath, Line: n.StartLine})
		}
	}
	starts := map[string][]int{}
	for i := len(parts) - 1; i >= 0; i-- {
		var emit []types.Node
		for _, s := range t.sites(parts[i]) {
			emit = append(emit, t.ix.nodes[s.fn])
		}
		slices.SortFunc(emit, func(a, b types.Node) int {
			return cmp.Or(cmp.Compare(a.FilePath, b.FilePath), cmp.Compare(a.StartLine, b.StartLine))
		})
		for _, n := range emit {
			list(n)
			starts[n.FilePath] = append(starts[n.FilePath], n.StartLine)
		}
	}
	type near struct {
		n    types.Node
		dist int
	}
	var rest []near
	for file, lines := range starts {
		for _, n := range t.ix.byFile[file] {
			if !baselineTypes[n.Type] || (valueTypes[n.Type] && !t.ix.packageLevel(n)) {
				continue
			}
			d := -1
			for _, l := range lines {
				if x := max(n.StartLine-l, l-n.StartLine); d < 0 || x < d {
					d = x
				}
			}
			rest = append(rest, near{n, d})
		}
	}
	slices.SortFunc(rest, func(a, b near) int {
		return cmp.Or(cmp.Compare(a.dist, b.dist), cmp.Compare(a.n.FilePath, b.n.FilePath), cmp.Compare(a.n.StartLine, b.n.StartLine), cmp.Compare(a.n.ID, b.n.ID))
	})
	for _, r := range rest {
		list(r.n)
	}
	return out
}
