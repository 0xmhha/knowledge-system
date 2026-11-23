package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/0xmhha/knowledge-system/internal/system/ckgclient"
	"github.com/0xmhha/knowledge-system/pkg/graph/store"
)

// Arm names in the output files.
const armGrep = "grep"

// summaryFile is the per-run measure table.
const summaryFile = "summary.json"

// runMode names what the runner computes.
type runMode string

const (
	// modeCompare ranks grep and graph candidates (decision A).
	modeCompare runMode = "compare"
	// modeDistance finds the shortest seed-to-gold paths (experiment 1).
	modeDistance runMode = "distance"
	// modeCochange ranks the other declarations of a fix from each changed
	// declaration (DESIGN.md section 10).
	modeCochange runMode = "cochange"
	// modeReachDiag sorts the co-change pairs by the walk that reaches them
	// (DESIGN.md section 11); it reads a cochange run through -from.
	modeReachDiag runMode = "reachdiag"
)

// cochangeFile is the input of the cochange mode, inside the data directory.
const cochangeFile = "cochange-questions.json"

// graphsSegment marks a graph directory; the runner refuses to write there.
const graphsSegment = string(filepath.Separator) + "graphs" + string(filepath.Separator)

var graphVariants = []graphVariant{variantUncapped, variantDefault}

type config struct {
	dataDir string // the experiment's data/ directory
	root    string // the directory that holds knowledge-data
	outDir  string
	only    map[string]bool
	mode    runMode
	files   inputFiles // file names inside dataDir
	from    string     // cochange output directory the reachdiag mode reads
}

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		slog.Error("runner failed", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	cfg, err := parseFlags(args)
	if err != nil {
		return err
	}
	switch cfg.mode {
	case modeCochange:
		return runCochangeAll(ctx, cfg)
	case modeReachDiag:
		return runDiagAll(cfg)
	case modeCompare, modeDistance:
	}
	qs, err := loadQuestions(cfg.dataDir, cfg.files)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(cfg.outDir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", cfg.outDir, err)
	}
	var rows []summaryRow
	for _, q := range qs {
		if len(cfg.only) > 0 && !cfg.only[q.ID] {
			continue
		}
		if cfg.mode == modeDistance {
			dr, err := runDistance(cfg, q)
			if err != nil {
				return fmt.Errorf("question %s: %w", q.ID, err)
			}
			if err := writeJSON(filepath.Join(cfg.outDir, q.ID+".json"), dr); err != nil {
				return err
			}
			slog.Info("question done", "id", q.ID, "gold", len(dr.Gold))
			continue
		}
		res, err := runQuestion(ctx, cfg, q)
		if err != nil {
			return fmt.Errorf("question %s: %w", q.ID, err)
		}
		if err := writeJSON(filepath.Join(cfg.outDir, q.ID+".json"), res); err != nil {
			return err
		}
		rows = append(rows, res.summary())
		slog.Info("question done", "id", q.ID,
			"grep_candidates", res.Measures[armGrep].Candidates,
			"graph_candidates", res.Measures[string(variantUncapped)].Candidates)
	}
	if cfg.mode == modeDistance {
		return nil
	}
	return writeJSON(filepath.Join(cfg.outDir, summaryFile), rows)
}

// runCochangeAll runs the co-change experiment over every listed question.
func runCochangeAll(ctx context.Context, cfg config) error {
	qs, err := loadCochange(cfg.dataDir, cochangeFile)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(cfg.outDir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", cfg.outDir, err)
	}
	rows := []cochangeSummary{}
	for _, q := range qs {
		if (len(cfg.only) > 0 && !cfg.only[q.ID]) || q.GraphDB == "" {
			continue
		}
		res, err := runCochange(ctx, cfg, q)
		if err != nil {
			return fmt.Errorf("question %s: %w", q.ID, err)
		}
		if err := writeJSON(filepath.Join(cfg.outDir, q.ID+".json"), res); err != nil {
			return err
		}
		rows = append(rows, res.summary())
		slog.Info("question done", "id", q.ID, "changed", len(res.Changed))
	}
	return writeJSON(filepath.Join(cfg.outDir, summaryFile), rows)
}

// runDiagAll sorts the pairs of a cochange run by the walk that reaches them.
func runDiagAll(cfg config) error {
	qs, err := loadCochange(cfg.dataDir, cochangeFile)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(cfg.outDir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", cfg.outDir, err)
	}
	var all []diagResult
	for _, q := range qs {
		if (len(cfg.only) > 0 && !cfg.only[q.ID]) || q.GraphDB == "" {
			continue
		}
		var c cochangeResult
		if err := readJSON(filepath.Join(cfg.from, q.ID+".json"), &c); err != nil {
			return err
		}
		res, err := runDiag(cfg, q, shippedPairs(c))
		if err != nil {
			return fmt.Errorf("question %s: %w", q.ID, err)
		}
		if err := writeJSON(filepath.Join(cfg.outDir, q.ID+".json"), res); err != nil {
			return err
		}
		all = append(all, res)
		slog.Info("question done", "id", q.ID, "pairs", len(res.Pairs))
	}
	isNew := func(r diagResult) bool { return !r.Stage1 }
	every := func(diagResult) bool { return true }
	rows := []diagSummary{
		summariseDiag("new, other file", all, func(r diagResult, p diagPair) bool { return !r.Stage1 && p.OtherFile }, isNew),
		summariseDiag("new, same file", all, func(r diagResult, p diagPair) bool { return !r.Stage1 && !p.OtherFile }, isNew),
		summariseDiag("all, other file", all, func(_ diagResult, p diagPair) bool { return p.OtherFile }, every),
	}
	return writeJSON(filepath.Join(cfg.outDir, summaryFile), rows)
}

func parseFlags(args []string) (config, error) {
	fs := flag.NewFlagSet("runner", flag.ContinueOnError)
	data := fs.String("data", "", "experiment data directory (seeds-blind.json, oracle-accepted.json, base-commits.json)")
	root := fs.String("root", "", "directory that holds knowledge-data (graph paths in base-commits.json are relative to it)")
	out := fs.String("out", "", "output directory; must not be inside a graph directory")
	only := fs.String("only", "", "comma-separated question ids to run (default: all)")
	mode := fs.String("mode", string(modeCompare), "compare (decision A), distance (experiment 1), cochange (section 10) or reachdiag (section 11)")
	from := fs.String("from", "", "cochange output directory; required with -mode reachdiag")
	seeds := fs.String("seeds", defaultSeedsFile, "seed file name inside -data (seeds-blind.json or seeds-near.json)")
	oracle := fs.String("oracle", defaultOracleFile, "oracle label file name inside -data")
	bases := fs.String("bases", defaultBasesFile, "base commit file name inside -data")
	if err := fs.Parse(args); err != nil {
		return config{}, err
	}
	if *data == "" || *root == "" || *out == "" {
		return config{}, errors.New("-data, -root and -out are required")
	}
	absOut, err := filepath.Abs(*out)
	if err != nil {
		return config{}, fmt.Errorf("resolve -out: %w", err)
	}
	if strings.Contains(absOut+string(filepath.Separator), graphsSegment) {
		return config{}, fmt.Errorf("-out %s is inside a graph directory", absOut)
	}
	m := runMode(*mode)
	for _, f := range []struct{ flag, v string }{{"-seeds", *seeds}, {"-oracle", *oracle}, {"-bases", *bases}} {
		if f.v == "" || filepath.Base(f.v) != f.v {
			return config{}, fmt.Errorf("%s %q: want a file name inside -data", f.flag, f.v)
		}
	}
	if m != modeCompare && m != modeDistance && m != modeCochange && m != modeReachDiag {
		return config{}, fmt.Errorf("-mode %q: want %s, %s, %s or %s", *mode, modeCompare, modeDistance, modeCochange, modeReachDiag)
	}
	if m == modeReachDiag && *from == "" {
		return config{}, errors.New("-mode reachdiag needs -from")
	}
	cfg := config{dataDir: *data, root: *root, outDir: absOut, only: map[string]bool{}, mode: m, from: *from, files: inputFiles{Seeds: *seeds, Oracle: *oracle, Bases: *bases}}
	for _, id := range strings.Split(*only, ",") {
		if id = strings.TrimSpace(id); id != "" {
			cfg.only[id] = true
		}
	}
	return cfg, nil
}

// questionResult is the per-question output file.
type questionResult struct {
	ID         string                 `json:"id"`
	Base       string                 `json:"base"`
	Blind      bool                   `json:"blind"`
	Seeds      []resolvedSeed         `json:"seeds"`
	Gold       []resolvedGold         `json:"gold"`
	GrepStats  grepStats              `json:"grep_stats"`
	FailedSeed map[string][]loc       `json:"failed_seeds"`
	Measures   map[string]armMeasure  `json:"measures"`
	Candidates map[string][]candidate `json:"candidates"`
}

func runQuestion(ctx context.Context, cfg config, q question) (questionResult, error) {
	dbPath := filepath.Join(cfg.root, q.GraphDB)
	graphDir := filepath.Dir(dbPath)
	srcRoot := filepath.Join(cfg.root, "knowledge-data", "agent-eval", "sources", q.Base)

	r, err := store.OpenReadOnly(dbPath)
	if err != nil {
		return questionResult{}, fmt.Errorf("open %s: %w", dbPath, err)
	}
	// Both handles are read-only (immutable=1): a close error loses nothing.
	defer func() { _ = r.Close() }()
	ckg, err := ckgclient.NewReal(dbPath)
	if err != nil {
		return questionResult{}, err
	}
	defer func() { _ = ckg.Close() }()

	fn := newFileNodes(r)
	seeds, seedNodes, err := resolveSeeds(fn, q.Seeds)
	if err != nil {
		return questionResult{}, err
	}
	gold, err := resolveGold(fn, q.Core, seedNodes)
	if err != nil {
		return questionResult{}, err
	}
	res := questionResult{
		ID: q.ID, Base: q.Base, Blind: q.Blind, Seeds: seeds, Gold: gold,
		FailedSeed: map[string][]loc{},
		Measures:   map[string]armMeasure{},
		Candidates: map[string][]candidate{},
	}
	if len(seedNodes) == 0 {
		// No seed exists at the base: both arms are empty by construction.
		for _, a := range append([]string{armGrep}, variantNames()...) {
			res.Measures[a] = measure(nil, gold)
			res.Candidates[a] = []candidate{}
		}
		return res, nil
	}

	files, err := scopeFiles(graphDir)
	if err != nil {
		return questionResult{}, err
	}
	gc, st, err := grepArm(srcRoot, files, fn, seeds)
	if err != nil {
		return questionResult{}, err
	}
	res.GrepStats = st
	res.Candidates[armGrep] = gc
	res.Measures[armGrep] = measure(gc, gold)

	for _, v := range graphVariants {
		cands, failed, err := graphArm(ctx, ckg, fn, seedNodes, v)
		if err != nil {
			return questionResult{}, fmt.Errorf("%s: %w", v, err)
		}
		res.Candidates[string(v)] = cands
		res.Measures[string(v)] = measure(cands, gold)
		if len(failed) > 0 {
			res.FailedSeed[string(v)] = failed
		}
	}
	return res, nil
}

func variantNames() []string {
	out := make([]string, 0, len(graphVariants))
	for _, v := range graphVariants {
		out = append(out, string(v))
	}
	return out
}

// runDistance opens the question's graph and runs experiment 1 on it.
func runDistance(cfg config, q question) (distanceResult, error) {
	dbPath := filepath.Join(cfg.root, q.GraphDB)
	r, err := store.OpenReadOnly(dbPath)
	if err != nil {
		return distanceResult{}, fmt.Errorf("open %s: %w", dbPath, err)
	}
	// Read-only (immutable=1): a close error loses nothing.
	defer func() { _ = r.Close() }()
	fn := newFileNodes(r)
	_, seedNodes, err := resolveSeeds(fn, q.Seeds)
	if err != nil {
		return distanceResult{}, err
	}
	gold, err := resolveGold(fn, q.Core, seedNodes)
	if err != nil {
		return distanceResult{}, err
	}
	return distanceQuestion(r, fn, q, seedNodes, gold)
}

// summaryRow is one line of summary.json.
type summaryRow struct {
	ID           string                `json:"id"`
	Blind        bool                  `json:"blind"`
	SeedsGiven   int                   `json:"seeds_given"`
	SeedNodes    int                   `json:"seed_nodes"`
	CoreTotal    int                   `json:"core_total"`
	CoreResolved int                   `json:"core_resolved"`
	CoreIsSeed   int                   `json:"core_is_seed"`
	Measures     map[string]armMeasure `json:"measures"`
}

func (r questionResult) summary() summaryRow {
	row := summaryRow{ID: r.ID, Blind: r.Blind, SeedsGiven: len(r.Seeds), CoreTotal: len(r.Gold), Measures: r.Measures}
	for _, s := range r.Seeds {
		row.SeedNodes += len(s.Nodes)
	}
	for _, g := range r.Gold {
		if g.Resolved {
			row.CoreResolved++
		}
		if g.IsSeed {
			row.CoreIsSeed++
		}
	}
	return row
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", " ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
