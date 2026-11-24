package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/0xmhha/knowledge-system/pkg/graph/store"
)

const defaultTop = 20

// output is what the command prints.
type output struct {
	Fragments  []string `json:"fragments"`
	Lines      []int    `json:"matched_lines"`
	Analysed   int      `json:"functions_analysed"`
	Total      int      `json:"candidates"`
	Candidates []result `json:"top"`
	Baseline   []result `json:"baseline"`
}

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		slog.Error("tracer failed", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("tracer", flag.ContinueOnError)
	db := fs.String("db", "", "graph.db, opened read-only")
	src := fs.String("src", "", "source tree of the graph's commit")
	message := fs.String("message", "", "failure message")
	top := fs.Int("top", defaultTop, "candidates to print, 0 for all")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *db == "" || *src == "" || *message == "" {
		return errors.New("-db, -src and -message are required")
	}
	r, err := store.OpenReadOnly(*db)
	if err != nil {
		return fmt.Errorf("open %s: %w", *db, err)
	}
	// Read-only: a close error loses nothing.
	defer func() { _ = r.Close() }()
	ix, err := buildIndex(r)
	if err != nil {
		return err
	}
	out := output{Fragments: fragments(*message)}
	var parts [][]position
	for _, f := range out.Fragments {
		ps, err := literalLines(*src, f)
		if err != nil {
			return err
		}
		parts = append(parts, ps)
		out.Lines = append(out.Lines, len(ps))
	}
	t := newTracer(ix, newGraphFlow(r).lines)
	ranked := t.run(ctx, parts)
	out.Analysed, out.Total, out.Candidates = t.analysed, len(ranked), ranked
	out.Baseline = t.emitFile(parts)
	if *top > 0 {
		out.Candidates = ranked[:min(*top, len(ranked))]
		out.Baseline = out.Baseline[:min(*top, len(out.Baseline))]
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", " ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(out); err != nil {
		return fmt.Errorf("write result: %w", err)
	}
	return nil
}
