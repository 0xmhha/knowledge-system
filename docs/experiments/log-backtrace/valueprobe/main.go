// Command valueprobe is step S3 of the log back-trace study: it runs the
// repository's value-flow query on one function of a graph and prints, for the
// given line, the conditions that guard it and where the values on the line and
// in those conditions are defined.
//
//	valueprobe -db graph.db -node <id> -line <n>
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/0xmhha/knowledge-system/pkg/graph/store"
	"github.com/0xmhha/knowledge-system/pkg/graph/valueflow"
)

const (
	queryTimeout = 20 * time.Second
	textLimit    = 100
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		slog.Error("valueprobe failed", "err", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("valueprobe", flag.ContinueOnError)
	db := fs.String("db", "", "graph.db, opened read-only")
	node := fs.String("node", "", "function or method node id")
	line := fs.Int("line", 0, "source line of the message")
	if err := fs.Parse(args); err != nil {
		return err
	}
	r, err := store.OpenReadOnly(*db)
	if err != nil {
		return fmt.Errorf("open %s: %w", *db, err)
	}
	// Read-only: a close error loses nothing.
	defer func() { _ = r.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()
	res, err := valueflow.Query(ctx, r, valueflow.QueryInput{NodeID: *node})
	if err != nil {
		return fmt.Errorf("query: %w", err)
	}
	fmt.Println("status", res.Status, res.Reason)
	if res.Analysis == nil {
		return nil
	}
	report(res.Analysis, *line)
	return nil
}

func report(a *valueflow.Result, line int) {
	fmt.Println("analysis", a.Status, "nodes", len(a.Nodes), "def_use", len(a.DefUse), "diagnostics", len(a.Diagnostics), "capability", a.Capability)
	nodes := map[string]valueflow.Node{}
	var at []string
	for _, n := range a.Nodes {
		nodes[n.ID] = n
		if n.Location.StartLine <= line && line <= n.Location.EndLine {
			at = append(at, n.ID)
		}
	}
	kinds := map[string]int{}
	var guards []string
	for _, e := range a.Edges {
		kinds[e.Kind]++
		if strings.Contains(e.Kind, "control_dep") && slices.Contains(at, e.To) && !slices.Contains(guards, e.From) {
			guards = append(guards, e.From)
		}
	}
	fmt.Println("edge kinds", kinds)
	for _, id := range at {
		fmt.Println("  at line:", id, nodes[id].Kind, short(nodes[id].Text))
	}
	for _, id := range guards {
		fmt.Printf("  guarded by: %s line %d %s\n", id, nodes[id].Location.StartLine, short(nodes[id].Text))
	}
	occ := map[string]valueflow.Occurrence{}
	for _, o := range a.Occurrences {
		occ[o.ID] = o
	}
	vars := map[string]string{}
	for _, v := range a.Variables {
		vars[v.ID] = v.Kind + " " + v.Name
	}
	want := append(slices.Clone(at), guards...)
	for _, d := range a.DefUse {
		use, def := occ[d.Use], occ[d.Definition]
		if slices.Contains(want, use.Node) {
			fmt.Printf("  value %s used line %d, defined line %d: %s (%s)\n", vars[d.Variable], use.Location.StartLine, def.Location.StartLine, short(nodes[def.Node].Text), d.Certainty)
		}
	}
}

func short(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > textLimit {
		return s[:textLimit] + "..."
	}
	return s
}
