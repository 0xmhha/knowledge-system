// knowledge-setup builds a complete knowledge dataset for a source tree in
// one command: graph index, vector index aligned to it, and an alignment
// verification gate. It is the typed replacement for the per-project shell
// scripts that used to encode this sequence.
//
//	knowledge-setup --src /path/to/repo --out /path/to/dataset \
//	    --embedder ollama --model-name bge-m3
//
// The graph build is incremental (the engine reuses its cache), so the same
// command serves both first-time setup and refresh runs.
//
// --progress selects the output contract: "text" (default) prints
// human-readable lines to stderr; "json" emits one JSON event object per
// line on stdout — the machine-readable stream orchestrators consume.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/0xmhha/knowledge-system/internal/setup"
)

func main() {
	var o setup.Options
	config := flag.String("config", "", "setup config file (e.g. projects/<name>/setup.yaml); explicit flags override its values")
	flag.StringVar(&o.Src, "src", "", "source tree to index (required)")
	flag.StringVar(&o.Out, "out", "", "dataset root; graph index in <out>/graph, vector index in <out>/vector (required)")
	flag.StringVar(&o.GraphBin, "graph-bin", "", "graph engine CLI (default: ckg on PATH)")
	flag.StringVar(&o.VectorBin, "vector-bin", "", "vector engine CLI (default: ckv on PATH)")
	flag.StringVar(&o.PolicyFile, "policy-file", "", "governance policy YAML for graph enrichment")
	flag.StringVar(&o.SecurityPatternFile, "security-pattern-file", "", "security-pattern YAML for graph enrichment")
	flag.StringVar(&o.Embedder, "embedder", "", "vector embedding backend (mock, bgeonnx, ollama)")
	flag.StringVar(&o.ModelName, "model-name", "", "vector embedding model name")
	flag.IntVar(&o.EmbedDim, "embed-dim", 0, "vector embedding dimension")
	flag.StringVar(&o.OllamaURL, "ollama-url", "", "ollama endpoint (exported as CKV_OLLAMA_ENDPOINT)")
	flag.StringVar(&o.VectorPolicy, "vector-policy", "", "vector chunk-categorization policy YAML")
	flag.StringVar(&o.FilelistConfig, "filelist", "", "filelist-gen config; derives <out>/files-from.json and scopes both engine builds")
	flag.StringVar(&o.FilelistBin, "filelist-bin", "", "filelist-gen CLI (default: filelist-gen on PATH)")
	flag.StringVar(&o.DomainKnowledge, "domain-knowledge", "", "project domain-knowledge dir; re-derives the corpus, governance policy and glossary before the builds")
	flag.StringVar(&o.DerivedDir, "derived-dir", "", "where the derived domain artifacts land (default: generated/ beside domain-knowledge)")
	flag.StringVar(&o.CodeRoot, "code-root", "", "working tree the project's authoritative_docs resolve against")
	flag.StringVar(&o.GlossaryFile, "glossary-file", "", "committed alias glossary; checked against the domain entries")
	flag.StringVar(&o.FlowCorpus, "flow-corpus", "", "curated flow-corpus JSONL embedded with the vector index")
	flag.StringVar(&o.DomainExportBin, "domain-export-bin", "", "cks-domain-export CLI (default: on PATH)")
	flag.StringVar(&o.DomainSyncBin, "domain-sync-bin", "", "cks-domain-sync CLI (default: on PATH)")
	flag.StringVar(&o.GlossaryGenBin, "glossary-gen-bin", "", "cks-glossary-gen CLI (default: on PATH)")
	flag.BoolVar(&o.SkipVector, "skip-vector", false, "build only the graph index")
	progress := flag.String("progress", "text", "progress output: text (stderr) or json (one event per line on stdout)")
	// Blue-green reindex (reindex-migration-design §4/§5). --out is the dataset
	// root holding version dirs + a `current` symlink.
	version := flag.String("version", "", "blue-green: build into <out>/<version>, gate it, then atomically promote <out>/current")
	rollback := flag.String("rollback", "", "blue-green: repoint <out>/current at an existing version and exit (no build)")
	gateMinCanonical := flag.Float64("gate-min-canonical", 0, "reindex gate: minimum canonical_id coverage (canonical/symbol chunks); 0 disables the check")
	flag.Parse()

	if *config != "" {
		base, err := setup.LoadConfig(*config)
		if err != nil {
			fmt.Fprintln(os.Stderr, "knowledge-setup:", err)
			os.Exit(2)
		}
		// Explicit flags win; unset flags take the config value.
		set := map[string]bool{}
		flag.Visit(func(f *flag.Flag) { set[f.Name] = true })
		merge := func(name string, dst *string, v string) {
			if !set[name] {
				*dst = v
			}
		}
		merge("src", &o.Src, base.Src)
		merge("out", &o.Out, base.Out)
		merge("graph-bin", &o.GraphBin, base.GraphBin)
		merge("vector-bin", &o.VectorBin, base.VectorBin)
		merge("policy-file", &o.PolicyFile, base.PolicyFile)
		merge("security-pattern-file", &o.SecurityPatternFile, base.SecurityPatternFile)
		merge("embedder", &o.Embedder, base.Embedder)
		merge("model-name", &o.ModelName, base.ModelName)
		merge("ollama-url", &o.OllamaURL, base.OllamaURL)
		merge("vector-policy", &o.VectorPolicy, base.VectorPolicy)
		merge("filelist", &o.FilelistConfig, base.FilelistConfig)
		merge("filelist-bin", &o.FilelistBin, base.FilelistBin)
		merge("domain-knowledge", &o.DomainKnowledge, base.DomainKnowledge)
		merge("derived-dir", &o.DerivedDir, base.DerivedDir)
		merge("code-root", &o.CodeRoot, base.CodeRoot)
		merge("glossary-file", &o.GlossaryFile, base.GlossaryFile)
		merge("flow-corpus", &o.FlowCorpus, base.FlowCorpus)
		merge("domain-export-bin", &o.DomainExportBin, base.DomainExportBin)
		merge("domain-sync-bin", &o.DomainSyncBin, base.DomainSyncBin)
		merge("glossary-gen-bin", &o.GlossaryGenBin, base.GlossaryGenBin)
		if !set["embed-dim"] {
			o.EmbedDim = base.EmbedDim
		}
		if !set["skip-vector"] {
			o.SkipVector = base.SkipVector
		}
	}

	emit, err := progressSink(*progress)
	if err != nil {
		fmt.Fprintln(os.Stderr, "knowledge-setup:", err)
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch {
	case *rollback != "":
		// Repoint current at an existing version — no build.
		if err := setup.Rollback(o.Out, *rollback); err != nil {
			fmt.Fprintln(os.Stderr, "knowledge-setup:", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "knowledge-setup: rolled back %s/current to %s\n", o.Out, *rollback)
	case *version != "":
		// Blue-green: build a new version, gate it, promote current on success.
		gopt := setup.GateOptions{GraphBin: o.GraphBin, Src: o.Src, MinCanonicalRatio: *gateMinCanonical}
		if err := setup.Reindex(ctx, o, *version, gopt, setup.SubprocessRunner{}, emit); err != nil {
			fmt.Fprintln(os.Stderr, "knowledge-setup:", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "knowledge-setup: promoted %s/%s to current\n", o.Out, *version)
	default:
		plan, err := setup.BuildPlan(o)
		if err != nil {
			fmt.Fprintln(os.Stderr, "knowledge-setup:", err)
			os.Exit(2)
		}
		if err := setup.Execute(ctx, plan, setup.SubprocessRunner{}, emit); err != nil {
			fmt.Fprintln(os.Stderr, "knowledge-setup:", err)
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, "knowledge-setup: dataset ready at", o.Out)
	}
}

func progressSink(mode string) (func(setup.Event), error) {
	switch mode {
	case "text":
		return func(e setup.Event) {
			switch e.Type {
			case "start":
				fmt.Fprintf(os.Stderr, "==> [%s] %s\n", e.Step, e.Message)
			case "output":
				fmt.Fprintf(os.Stderr, "    %s\n", e.Message)
			case "warning":
				fmt.Fprintf(os.Stderr, " !  [%s] %s\n", e.Step, e.Message)
			case "done":
				fmt.Fprintf(os.Stderr, "<== [%s] done\n", e.Step)
			case "error":
				fmt.Fprintf(os.Stderr, "ERR [%s] %s\n", e.Step, e.Message)
			}
		}, nil
	case "json":
		enc := json.NewEncoder(os.Stdout)
		return func(e setup.Event) { _ = enc.Encode(e) }, nil
	default:
		return nil, fmt.Errorf("unknown --progress %q (text|json)", mode)
	}
}
