package main

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/0xmhha/knowledge-system/pkg/graph/store"
	"github.com/0xmhha/knowledge-system/pkg/graph/valueflow"
)

const (
	flowTimeout = 5 * time.Second
	controlEdge = "control_dependence"
	returnKind  = "ReturnStmt"
	nilReturn   = "nil"
)

// graphFlow answers flowLines from the repository's value-flow query. Results
// are kept per function: rule R asks for the same function again.
type graphFlow struct {
	r     store.Reader
	cache map[string]*valueflow.Result
}

func newGraphFlow(r store.Reader) *graphFlow {
	return &graphFlow{r: r, cache: map[string]*valueflow.Result{}}
}

func (g *graphFlow) analysis(ctx context.Context, node string) *valueflow.Result {
	if a, ok := g.cache[node]; ok {
		return a
	}
	qctx, cancel := context.WithTimeout(ctx, flowTimeout)
	defer cancel()
	res, err := valueflow.Query(qctx, g.r, valueflow.QueryInput{NodeID: node})
	if err != nil {
		// RULES.md: a function the value flow cannot answer keeps its focus lines.
		g.cache[node] = nil
		return nil
	}
	g.cache[node] = res.Analysis
	return res.Analysis
}

func (g *graphFlow) lines(ctx context.Context, node string, focus []int) flowLines {
	a := g.analysis(ctx, node)
	if a == nil {
		return flowLines{failed: true}
	}
	return linesOf(a, focus)
}

func linesOf(a *valueflow.Result, focus []int) flowLines {
	byID := make(map[string]valueflow.Node, len(a.Nodes))
	var at []string
	var out flowLines
	for _, n := range a.Nodes {
		byID[n.ID] = n
		for _, l := range focus {
			if n.Location.StartLine <= l && l <= n.Location.EndLine {
				at = append(at, n.ID)
				break
			}
		}
		if n.Kind == returnKind && !strings.HasSuffix(strings.TrimSpace(n.Text), nilReturn) {
			out.returns = append(out.returns, n.Location.StartLine)
		}
	}
	guards := follow(at, maxGuardSteps, func(id string) []string {
		var from []string
		for _, e := range a.Edges {
			if e.Kind == controlEdge && e.To == id {
				from = append(from, e.From)
			}
		}
		return from
	})
	occ := make(map[string]valueflow.Occurrence, len(a.Occurrences))
	for _, o := range a.Occurrences {
		occ[o.ID] = o
	}
	defs := follow(append(slices.Clone(at), guards...), maxDefSteps, func(id string) []string {
		var from []string
		for _, d := range a.DefUse {
			if occ[d.Use].Node == id && occ[d.Definition].Node != "" {
				from = append(from, occ[d.Definition].Node)
			}
		}
		return from
	})
	out.guards, out.defs = startLines(byID, guards), startLines(byID, defs)
	slices.Sort(out.returns)
	out.returns = slices.Compact(out.returns)
	return out
}

// follow returns the nodes within steps of the start nodes, starts excluded.
func follow(start []string, steps int, next func(string) []string) []string {
	seen := map[string]bool{}
	for _, s := range start {
		seen[s] = true
	}
	var out []string
	level := start
	for range steps {
		var found []string
		for _, id := range level {
			for _, n := range next(id) {
				if !seen[n] {
					seen[n] = true
					found = append(found, n)
				}
			}
		}
		out = append(out, found...)
		level = found
	}
	return out
}

func startLines(byID map[string]valueflow.Node, ids []string) []int {
	var out []int
	for _, id := range ids {
		if n, ok := byID[id]; ok && n.Location.StartLine > 0 {
			out = append(out, n.Location.StartLine)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}
