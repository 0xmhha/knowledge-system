// Command hopgold lists, for synthetic multi-hop change requests, every
// declaration the Go type checker says the change reaches
// (docs/experiments/issue-relations/HOPGATE.md). It does not read the knowledge
// graph.
//
//	hopgold -src <tree> -scope build-scope.json -out gold.json
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// Kind names the change request.
type Kind string

const (
	// KindSignature changes the signature of an interface method: everything
	// that must change with it for the build to compile.
	KindSignature Kind = "signature"
	// KindReach changes the behaviour of a function or method: everything that
	// reaches it within reachSteps uses, through interfaces too.
	KindReach Kind = "reach"
)

const (
	testSuffix = "_test.go"
	outMode    = 0o644
	reachSteps = 3
)

// Decl is one top-level declaration range.
type Decl struct {
	File  string `json:"file"`
	Start int    `json:"start_line"`
	End   int    `json:"end_line"`
	Name  string `json:"name"`
	Kind  string `json:"kind"`
}

// Target is one change request with its gold declarations.
type Target struct {
	Kind    Kind   `json:"kind"`
	QName   string `json:"qname"`
	File    string `json:"file"`
	Line    int    `json:"line"`
	Own     Decl   `json:"own"`
	Gold    []Decl `json:"gold"`
	Plain   int    `json:"gold_without_interfaces"`
	TypeGap int    `json:"gold_by_type_difference"`
	Members int    `json:"members"`
	Outside bool   `json:"outside_member"`
}

// Output is the probe's result file.
type Output struct {
	Decls   []Decl   `json:"decls"`
	Targets []Target `json:"targets"`
	Errors  int      `json:"package_errors"`
	Loaded  int      `json:"packages"`
	// Untyped lists the packages of the build whose conversions were not read.
	Untyped []string `json:"untyped_packages"`
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		slog.Error("hopgold failed", "err", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("hopgold", flag.ContinueOnError)
	src := fs.String("src", "", "source tree of the base commit")
	scope := fs.String("scope", "", "build-scope.json of the graph")
	out := fs.String("out", "", "result file")
	tags := fs.String("tags", "urfave_cli_no_docs,ckzg", "build tags")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *src == "" || *scope == "" || *out == "" {
		return errors.New("-src, -scope and -out are required")
	}
	abs, err := filepath.EvalSymlinks(*src)
	if err != nil {
		return fmt.Errorf("resolve -src: %w", err)
	}
	files, err := scopeFiles(*scope)
	if err != nil {
		return err
	}
	p := newProbe(abs, files)
	res, err := p.load(*tags)
	if err != nil {
		return err
	}
	res.Targets = p.targets()
	res.Decls = p.allDecls()
	b, err := json.Marshal(res)
	if err != nil {
		return fmt.Errorf("encode: %w", err)
	}
	if err := os.WriteFile(*out, b, outMode); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	slog.Info("done", "packages", res.Loaded, "errors", res.Errors, "targets", len(res.Targets), "decls", len(res.Decls))
	return nil
}

func scopeFiles(path string) (map[string]bool, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read scope: %w", err)
	}
	var doc struct {
		Files []struct {
			Path string `json:"path"`
		} `json:"files"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, fmt.Errorf("parse scope: %w", err)
	}
	out := map[string]bool{}
	for _, f := range doc.Files {
		if strings.HasSuffix(f.Path, ".go") && !strings.HasSuffix(f.Path, testSuffix) {
			out[f.Path] = true
		}
	}
	return out, nil
}
