package setup

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
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

	// Canonical_id — the vector<->graph join key (ADR-007) — exists only from
	// graph schema 1.19. Aligning a vector index against an older graph is
	// unsound even if the digest happens to match, so fail loud. An
	// unparseable/absent version (very old or hand-written manifest) can't be
	// judged and degrades to a warning.
	if maj, min, ok := parseSchemaVersion(gm.SchemaVersion); !ok {
		warn(fmt.Sprintf("graph schema_version %q unparseable — canonical_id (>=1.19) support not verifiable", gm.SchemaVersion))
	} else if maj < 1 || (maj == 1 && min < 19) {
		return fmt.Errorf("verify: graph schema %s < 1.19 — canonical_id (ADR-007) unavailable, vector join unreliable", gm.SchemaVersion)
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

// parseSchemaVersion splits a "major.minor[.patch]" schema string into its
// numeric major/minor. ok is false when either component is missing or
// non-numeric. Numeric (not lexical) so "1.9" < "1.19".
func parseSchemaVersion(s string) (maj, min int, ok bool) {
	parts := strings.SplitN(s, ".", 3)
	if len(parts) < 2 {
		return 0, 0, false
	}
	a, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	b, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return a, b, true
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

// contentManifest projects the vector manifest fields that record which
// non-code inputs an index actually absorbed.
type contentManifest struct {
	DocsRoots []string       `json:"docs_roots"`
	Languages map[string]int `json:"languages"`
}

// VerifyContent asserts that inputs the caller declared are present in the
// finished index. It exists because every one of these inputs used to fail
// open: a corpus flag that was never passed, an authoritative doc that could
// not be resolved, and a policy that had drifted from its entries all produced
// a build that exited zero and an index that was quietly smaller. The only
// symptom was a chunk count nobody compared against anything.
//
// The checks read the vector manifest rather than the database so this stays
// on the CLI-contract side of the engine boundary.
func VerifyContent(o Options, emit func(Event)) error {
	var cm contentManifest
	if err := readJSON(filepath.Join(o.VectorDir(), "manifest.json"), &cm); err != nil {
		return fmt.Errorf("verify: vector manifest: %w", err)
	}
	rooted := func(dir string) bool {
		want, err := filepath.Abs(dir)
		if err != nil {
			want = dir
		}
		for _, got := range cm.DocsRoots {
			if abs, err := filepath.Abs(got); err == nil {
				got = abs
			}
			if got == want {
				return true
			}
		}
		return false
	}

	var missing []string
	if o.DomainKnowledge != "" {
		if !rooted(o.DomainCorpusDir()) {
			missing = append(missing, fmt.Sprintf("the domain corpus %s is not among the index's docs roots", o.DomainCorpusDir()))
		} else if cm.Languages["markdown"] == 0 {
			missing = append(missing, "the domain corpus was passed but produced no markdown chunks (an empty corpus directory)")
		}
	}
	if o.FlowCorpus != "" && !rooted(filepath.Dir(o.FlowCorpus)) {
		missing = append(missing, fmt.Sprintf("the flow corpus %s is not among the index's docs roots", o.FlowCorpus))
	}
	if len(missing) > 0 {
		return fmt.Errorf("verify: declared inputs did not reach the index:\n  - %s", strings.Join(missing, "\n  - "))
	}
	if emit != nil {
		emit(Event{Time: time.Now().UTC(), Step: "verify-content", Type: "output",
			Message: fmt.Sprintf("docs roots %d, markdown chunks %d", len(cm.DocsRoots), cm.Languages["markdown"])})
	}
	return nil
}

// VerifyDerivedFresh regenerates a committed artifact into a temporary file
// and fails when it differs from the copy in the tree.
//
// Some artifacts are rendered from a project's domain-knowledge entries but
// stay committed, because a change to an entry should show up as a reviewable
// diff rather than only inside a rebuilt index. That only works if something
// notices when the two fall apart. Nothing did, and both the governance policy
// and the alias glossary of one pack sat months behind their entries — the
// policy silently building graphs with a quarter fewer governance edges than
// the entries called for.
//
// Checking instead of deriving keeps one copy of each artifact. Writing a
// second one into a build directory would remove the staleness but leave two
// files to reason about, and the build would quietly stop agreeing with what
// reviewers had approved.
func VerifyDerivedFresh(step, label string, argv []string, committed string, emit func(Event)) error {
	dir, err := os.MkdirTemp("", "knowledge-setup-fresh-")
	if err != nil {
		return fmt.Errorf("verify: %s: %w", label, err)
	}
	defer os.RemoveAll(dir)

	fresh := filepath.Join(dir, filepath.Base(committed))
	cmd := exec.Command(argv[0], append(append([]string{}, argv[1:]...), fresh)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("verify: %s: regenerating to compare failed: %w\n%s", label, err, out)
	}
	want, err := os.ReadFile(fresh)
	if err != nil {
		return fmt.Errorf("verify: %s: %w", label, err)
	}
	got, err := os.ReadFile(committed)
	if err != nil {
		return fmt.Errorf("verify: %s: reading the committed copy: %w", label, err)
	}
	if !bytes.Equal(got, want) {
		return fmt.Errorf("verify: %s is stale: %s no longer matches the domain entries "+
			"it is rendered from. Run 'make sync-domain-artifacts' and commit the result", label, committed)
	}
	if emit != nil {
		emit(Event{Time: time.Now().UTC(), Step: step, Type: "output",
			Message: fmt.Sprintf("%s matches its entries", label)})
	}
	return nil
}
