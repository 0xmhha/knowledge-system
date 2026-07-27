// Package setup turns the knowledge-setup sequence — build the graph index,
// build the vector index aligned to it, verify the two agree — into a typed,
// machine-runnable plan. It replaces the per-project shell scripts that used
// to encode this sequence with hardcoded paths.
//
// The engines are driven through their CLIs as subprocesses (the same
// execution model the fused server's index tool has always used): build
// failures stay isolated from the calling process, and cancellation is a
// process kill. The package deliberately imports no engine internals — the
// CLI flags used here are the compatibility contract, and the boundary check
// enforces the isolation.
package setup

import (
	"fmt"
	"path/filepath"
)

// Options parameterizes one knowledge-setup plan.
type Options struct {
	// Src is the source tree to index. Required.
	Src string
	// Out is the dataset root; the graph index lands in Out/graph and the
	// vector index in Out/vector. Required.
	Out string

	// GraphBin / VectorBin are the engine CLI binaries. Empty values fall
	// back to "ckg" and "ckv" resolved on PATH (the binary names the engine
	// makefiles produce).
	GraphBin  string
	VectorBin string

	// Graph enrichment inputs (optional): governance policy and
	// security-pattern YAML passed through to the graph build.
	PolicyFile          string
	SecurityPatternFile string

	// Vector build knobs (optional). Embedder "" lets the vector CLI pick
	// its default; OllamaURL is exported as CKV_OLLAMA_ENDPOINT.
	Embedder  string
	ModelName string
	EmbedDim  int
	OllamaURL string
	// VectorPolicy is the vector chunk-categorization policy YAML.
	VectorPolicy string

	// SkipVector builds only the graph index (and skips alignment
	// verification, which needs both).
	SkipVector bool
}

// GraphDir / VectorDir are the per-engine data directories under Out.
func (o Options) GraphDir() string  { return filepath.Join(o.Out, "graph") }
func (o Options) VectorDir() string { return filepath.Join(o.Out, "vector") }

// Step is one unit of the plan: either a subprocess (Cmd non-empty) or an
// internal verification (Verify non-nil).
type Step struct {
	ID    string
	Title string
	Cmd   []string // argv; nil for internal steps
	Env   []string // extra environment (KEY=VALUE), appended to the parent env
	// Verify runs an in-process check for internal steps. It may emit
	// non-fatal findings through the provided callback and returns an error
	// only when the step must fail the plan.
	Verify func(emit func(Event)) error
}

// Plan is the ordered step list for one setup run.
type Plan struct {
	Steps []Step
}

// BuildPlan assembles the canonical knowledge-setup sequence:
//
//  1. graph-build   — <graph-bin> build --src SRC --out OUT/graph [enrichment]
//  2. vector-build  — <vector-bin> build --src SRC --out OUT/vector --ckg OUT/graph [...]
//  3. verify-align  — read both manifests, assert same src commit and that
//     the vector index recorded the graph's coordinate pin
//
// The graph build is incremental by definition (the engine reuses its cache
// when OUT/graph already holds a usable manifest), so the same plan serves
// first-time setup and update runs.
func BuildPlan(o Options) (Plan, error) {
	if o.Src == "" {
		return Plan{}, fmt.Errorf("setup: Src is required")
	}
	if o.Out == "" {
		return Plan{}, fmt.Errorf("setup: Out is required")
	}
	graphBin := o.GraphBin
	if graphBin == "" {
		graphBin = "ckg"
	}
	vectorBin := o.VectorBin
	if vectorBin == "" {
		vectorBin = "ckv"
	}

	graphCmd := []string{graphBin, "build", "--src", o.Src, "--out", o.GraphDir()}
	if o.PolicyFile != "" {
		graphCmd = append(graphCmd, "--policy-file", o.PolicyFile)
	}
	if o.SecurityPatternFile != "" {
		graphCmd = append(graphCmd, "--security-pattern-file", o.SecurityPatternFile)
	}
	steps := []Step{{
		ID:    "graph-build",
		Title: "Build the graph index",
		Cmd:   graphCmd,
	}}

	if !o.SkipVector {
		vectorCmd := []string{vectorBin, "build", "--src", o.Src, "--out", o.VectorDir(), "--ckg", o.GraphDir()}
		if o.Embedder != "" {
			vectorCmd = append(vectorCmd, "--embedder="+o.Embedder)
		}
		if o.ModelName != "" {
			vectorCmd = append(vectorCmd, "--model-name="+o.ModelName)
		}
		if o.EmbedDim > 0 {
			vectorCmd = append(vectorCmd, fmt.Sprintf("--embed-dim=%d", o.EmbedDim))
		}
		if o.VectorPolicy != "" {
			vectorCmd = append(vectorCmd, "--policy", o.VectorPolicy)
		}
		var env []string
		if o.OllamaURL != "" {
			env = append(env, "CKV_OLLAMA_ENDPOINT="+o.OllamaURL)
		}
		// Fail fast on an unusable Ollama backend before the long build.
		if o.Embedder == "ollama" {
			url, model := o.OllamaURL, o.ModelName
			steps = append(steps, Step{
				ID:    "vector-preflight",
				Title: "Preflight the Ollama embedder",
				Verify: func(emit func(Event)) error {
					return PreflightOllama(url, model, emit)
				},
			})
		}
		steps = append(steps,
			Step{
				ID:    "vector-build",
				Title: "Build the vector index aligned to the graph",
				Cmd:   vectorCmd,
				Env:   env,
			},
			Step{
				ID:    "verify-align",
				Title: "Verify graph/vector alignment",
				Verify: func(emit func(Event)) error {
					return VerifyAlignment(o.GraphDir(), o.VectorDir(), emit)
				},
			},
		)
	}
	return Plan{Steps: steps}, nil
}
