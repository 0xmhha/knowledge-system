package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Input file names inside the experiment's data directory.
const (
	seedsFile  = "seeds-blind.json"
	oracleFile = "oracle-relayed.json"
	basesFile  = "base-commits.json"
)

// seedRef is one frozen start symbol.
type seedRef struct {
	Symbol string `json:"symbol"`
	Kind   string `json:"kind"`
	File   string `json:"file"`
}

type seedsDoc struct {
	Seeds []struct {
		ID    string    `json:"id"`
		Seeds []seedRef `json:"seeds"`
		Blind bool      `json:"blind"`
	} `json:"seeds"`
}

// oracleRef is one oracle location at the base commit.
type oracleRef struct {
	Core      bool   `json:"core"`
	File      string `json:"file"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Symbol    string `json:"symbol"`
	Kind      string `json:"kind"`
}

type oracleDoc struct {
	Questions []struct {
		ID        string      `json:"id"`
		Citations []oracleRef `json:"citations"`
	} `json:"questions"`
}

type basesDoc struct {
	Bases []struct {
		ID    string `json:"id"`
		Base  string `json:"base"`
		Graph string `json:"graph"`
	} `json:"bases"`
}

// question joins the three input files for one question id.
type question struct {
	ID    string
	Blind bool
	Base  string
	// GraphDB is the graph.db path relative to the directory that holds
	// knowledge-data (the form base-commits.json records).
	GraphDB string
	Seeds   []seedRef
	Core    []oracleRef
}

// loadQuestions reads the frozen inputs and returns the questions in the
// order of seeds-blind.json. A question missing from the oracle or base file
// is an error: the runner must not silently skip one.
func loadQuestions(dataDir string) ([]question, error) {
	var sd seedsDoc
	if err := readJSON(filepath.Join(dataDir, seedsFile), &sd); err != nil {
		return nil, err
	}
	var od oracleDoc
	if err := readJSON(filepath.Join(dataDir, oracleFile), &od); err != nil {
		return nil, err
	}
	var bd basesDoc
	if err := readJSON(filepath.Join(dataDir, basesFile), &bd); err != nil {
		return nil, err
	}

	core := make(map[string][]oracleRef, len(od.Questions))
	for _, q := range od.Questions {
		for _, c := range q.Citations {
			if c.Core {
				core[q.ID] = append(core[q.ID], c)
			}
		}
	}
	type baseInfo struct{ base, graph string }
	bases := make(map[string]baseInfo, len(bd.Bases))
	for _, b := range bd.Bases {
		bases[b.ID] = baseInfo{b.Base, b.Graph}
	}

	out := make([]question, 0, len(sd.Seeds))
	for _, s := range sd.Seeds {
		b, ok := bases[s.ID]
		if !ok || b.base == "" || b.graph == "" {
			return nil, fmt.Errorf("question %s: no base commit or graph in %s", s.ID, basesFile)
		}
		c, ok := core[s.ID]
		if !ok {
			return nil, fmt.Errorf("question %s: no core location in %s", s.ID, oracleFile)
		}
		out = append(out, question{
			ID: s.ID, Blind: s.Blind, Base: b.base, GraphDB: b.graph,
			Seeds: s.Seeds, Core: c,
		})
	}
	return out, nil
}

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	return nil
}

// scopeFiles returns the repo-relative paths in a graph's build-scope.json,
// sorted as the file lists them.
func scopeFiles(graphDir string) ([]string, error) {
	var sc struct {
		Files []struct {
			Path string `json:"path"`
		} `json:"files"`
	}
	if err := readJSON(filepath.Join(graphDir, "build-scope.json"), &sc); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(sc.Files))
	for _, f := range sc.Files {
		out = append(out, f.Path)
	}
	return out, nil
}
