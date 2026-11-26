// cmd/ckg/path.go — `ckg path A B` finds the shortest path between two
// nodes in the graph, printing the symbol chain + edge types along the
// way. Inspired by graphify's `graphify path "A" "B"` ergonomic
// (__main__.py:1622-1683).
//
// Each argument is resolved by persist.ResolveName, the resolver every tool
// shares (a canonical id, a qualified name, a site name
// "<qname>@<file>:<line>", or a bare name); a name that resolves to several
// declarations is an error that lists the candidates, never a silent pick.
// A 16-char node ID prefix (copied from /api/edges output) is accepted when
// the name resolves to nothing.
//
// BFS is undirected because "how are X and Y related" doesn't depend on
// edge direction (matches graphify's nx.shortest_path default behaviour).
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/0xmhha/knowledge-system/internal/graph/persist"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

func newPathCmd() *cobra.Command {
	var graph string
	var maxDepth int
	cmd := &cobra.Command{
		Use:   "path <from> <to>",
		Short: "Find the shortest path between two nodes (qualified_name / name / ID prefix)",
		Long: `Print the shortest undirected path between two nodes. Each argument
can be a canonical id, a fully-qualified name (e.g. ` + "`pkg.Foo`" + `), a site name
(` + "`pkg.F.x@file.go:12`" + `, the form candidates take), a bare name, or a 16-char
node ID prefix. A name that matches several declarations is an error that
lists them.

Output: the hop count, then a numbered chain of nodes with the relation
that links each pair (` + pathArrowDown + ` the node above has it to the node below, ` + pathArrowUp + ` the
node below has it to the node above).`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			db := filepath.Join(graph, "graph.db")
			store, err := persist.OpenReadOnly(db)
			if err != nil {
				return fmt.Errorf("open graph: %w", err)
			}
			defer func() { _ = store.Close() }()

			nodes, err := store.AllNodes()
			if err != nil {
				return fmt.Errorf("load nodes: %w", err)
			}
			edges, err := store.AllEdges()
			if err != nil {
				return fmt.Errorf("load edges: %w", err)
			}

			fromID, err := resolveNodeQuery(store, nodes, args[0])
			if err != nil {
				return err
			}
			toID, err := resolveNodeQuery(store, nodes, args[1])
			if err != nil {
				return err
			}
			if fromID == toID {
				fmt.Println("(both endpoints resolved to the same node — distance 0)")
				return nil
			}

			path, hops := bfsShortestPath(nodes, edges, fromID, toID, maxDepth)
			if len(path) == 0 {
				fmt.Printf("(no path of length ≤ %d between the two endpoints)\n", maxDepth)
				return nil
			}
			printPath(os.Stdout, nodes, path, hops)
			return nil
		},
	}
	cmd.Flags().StringVar(&graph, "graph", "", "graph directory (required)")
	cmd.Flags().IntVar(&maxDepth, "max-depth", 12,
		"BFS hop cap; raise for sparse graphs, lower for faster fail")
	_ = cmd.MarkFlagRequired("graph")
	return cmd
}

// resolveNodeQuery resolves one argument of ckg path to a node id (see the
// package comment): by persist.ResolveName, else by node id prefix.
func resolveNodeQuery(r persist.StoreReader, nodes []types.Node, q string) (string, error) {
	res, err := persist.ResolveName(r, q, "")
	if err != nil {
		return "", fmt.Errorf("resolve %q: %w", q, err)
	}
	switch {
	case res.Ambiguous():
		return "", fmt.Errorf("%q names several declarations; use one of: %s", q, strings.Join(res.Candidates, ", "))
	case res.Found():
		return res.Nodes[0].ID, nil
	}
	for _, n := range nodes {
		if strings.HasPrefix(n.ID, q) {
			return n.ID, nil
		}
	}
	return "", fmt.Errorf("could not resolve %q to any node", q)
}

// pathHop is the relation between two successive nodes of a path.
type pathHop struct {
	edgeType string
	// forward is true when the earlier node of the path has the relation to
	// the later one, false when the later node has it to the earlier one.
	forward bool
}

// pathArrowDown and pathArrowUp mark a hop's direction in the printed path:
// the node above has the relation to the node below, or the reverse.
const (
	pathArrowDown = "↓"
	pathArrowUp   = "↑"
)

// bfsShortestPath returns the shortest undirected node sequence from src
// to dst, plus the relation used to traverse to each non-root step and its
// direction. hops has length len(path)-1. Returns (nil, nil) when no path
// exists within maxDepth hops.
//
// Adjacency is built fresh per call (O(E)) — for repeated path queries
// the caller could cache it, but the typical interactive use is one
// query per invocation so the simplicity wins.
func bfsShortestPath(nodes []types.Node, edges []types.Edge,
	src, dst string, maxDepth int) ([]string, []pathHop) {
	type out struct {
		id  string
		hop pathHop
	}
	adj := make(map[string][]out, len(nodes))
	for _, e := range edges {
		adj[e.Src] = append(adj[e.Src], out{e.Dst, pathHop{string(e.Type), true}})
		adj[e.Dst] = append(adj[e.Dst], out{e.Src, pathHop{string(e.Type), false}})
	}

	parent := map[string]string{src: ""}
	parentEdge := map[string]pathHop{}
	queue := []string{src}
	depth := 0
	found := false
	for len(queue) > 0 && depth < maxDepth {
		next := make([]string, 0, len(queue))
		for _, v := range queue {
			for _, o := range adj[v] {
				if _, seen := parent[o.id]; seen {
					continue
				}
				parent[o.id] = v
				parentEdge[o.id] = o.hop
				if o.id == dst {
					found = true
					break
				}
				next = append(next, o.id)
			}
			if found {
				break
			}
		}
		if found {
			break
		}
		queue = next
		depth++
	}
	if !found {
		return nil, nil
	}

	var path []string
	var hops []pathHop
	cur := dst
	for {
		path = append([]string{cur}, path...)
		if cur == src {
			break
		}
		hops = append([]pathHop{parentEdge[cur]}, hops...)
		cur = parent[cur]
	}
	return path, hops
}

// printPath renders the path as a human-readable chain with the relation
// between successive nodes, drawn in the relation's own direction: the walk
// crosses edges both ways, so the order of the chain says nothing about who
// calls or writes whom. Format mirrors graphify's path output but uses CKG's
// richer schema (Type column + qualified_name).
func printPath(w io.Writer, nodes []types.Node, path []string, hops []pathHop) {
	idx := make(map[string]types.Node, len(path))
	for _, n := range nodes {
		for _, id := range path {
			if n.ID == id {
				idx[id] = n
				break
			}
		}
	}
	_, _ = fmt.Fprintf(w, "Path (%d hops):\n", len(path)-1)
	_, _ = fmt.Fprintf(w, "(%s the node above has the relation to the node below; %s the node below has it to the node above)\n\n",
		pathArrowDown, pathArrowUp)
	for i, id := range path {
		n := idx[id]
		_, _ = fmt.Fprintf(w, "  %2d. [%s] %s — `%s`\n", i+1, n.Type, n.Name, n.QualifiedName)
		if i < len(hops) {
			arrow := pathArrowDown
			if !hops[i].forward {
				arrow = pathArrowUp
			}
			_, _ = fmt.Fprintf(w, "       %s [%s]\n", arrow, hops[i].edgeType)
		}
	}
}
