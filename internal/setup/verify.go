package setup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// graphManifest / vectorManifest are minimal read-only projections of the two
// engines' manifest.json files — just the coordinate fields the alignment
// check needs. Reading the JSON files directly (instead of importing engine
// internals) keeps this package on the CLI-contract side of the boundary.
type graphManifest struct {
	SchemaVersion string `json:"schema_version"`
	SrcCommit     string `json:"src_commit"`
	GraphDigest   string `json:"graph_digest"`
}

type vectorManifest struct {
	SrcCommit string `json:"src_commit"`
	Sources   *struct {
		CKG *struct {
			GraphDigest string `json:"graph_digest"`
			SrcCommit   string `json:"src_commit"`
		} `json:"ckg"`
	} `json:"sources"`
}

// VerifyAlignment asserts that the graph and vector indexes under the two
// data directories describe the same source: same source commit, and the
// vector index's recorded coordinate pin matches the graph's digest. Absent
// optional coordinates (older indexes) degrade to warnings; a present-but-
// diverging coordinate fails.
//
// This is the setup-time gate. The fused server re-asserts the same
// invariant at startup and stays the runtime authority.
func VerifyAlignment(graphDir, vectorDir string, emit func(Event)) error {
	warn := func(msg string) {
		if emit != nil {
			emit(Event{Time: time.Now().UTC(), Step: "verify-align", Type: "warning", Message: msg})
		}
	}

	var gm graphManifest
	if err := readJSON(filepath.Join(graphDir, "manifest.json"), &gm); err != nil {
		return fmt.Errorf("verify: graph manifest: %w", err)
	}
	var vm vectorManifest
	if err := readJSON(filepath.Join(vectorDir, "manifest.json"), &vm); err != nil {
		return fmt.Errorf("verify: vector manifest: %w", err)
	}

	vecCommit := vm.SrcCommit
	var vecPin string
	if vm.Sources != nil && vm.Sources.CKG != nil {
		vecPin = vm.Sources.CKG.GraphDigest
		if vm.Sources.CKG.SrcCommit != "" {
			vecCommit = vm.Sources.CKG.SrcCommit
		}
	} else {
		warn("vector manifest has no sources.ckg ledger — using top-level src_commit")
	}

	switch {
	case gm.SrcCommit == "" || vecCommit == "":
		warn("source commit missing on one side — commit alignment not verifiable")
	case gm.SrcCommit != vecCommit:
		return fmt.Errorf("verify: graph and vector built from different commits (graph %.9s, vector %.9s)",
			gm.SrcCommit, vecCommit)
	}

	switch {
	case gm.GraphDigest == "":
		warn("graph manifest carries no digest (pre-digest build) — pin not verifiable")
	case vecPin == "":
		warn("vector index recorded no graph digest — pin not verifiable")
	case gm.GraphDigest != vecPin:
		return fmt.Errorf("verify: coordinate pin mismatch (graph %.12s, vector aligned to %.12s) — vector canonical_id join is stale",
			gm.GraphDigest, vecPin)
	}
	return nil
}

func readJSON(path string, v any) error {
	buf, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(buf, v); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}
