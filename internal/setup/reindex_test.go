package setup

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// aligned graph+vector manifests for a version dir (commit abc, digest d1,
// schema 1.23, full canonical coverage) so the gate passes.
func writeAlignedVersion(t *testing.T, verDir string) {
	t.Helper()
	writeManifest(t, filepath.Join(verDir, "graph"),
		map[string]any{"src_commit": "abc", "graph_digest": "d1", "schema_version": "1.23"})
	writeManifest(t, filepath.Join(verDir, "vector"), map[string]any{
		"src_commit": "abc", "chunk_count": 10, "symbol_count": 8, "canonical_count": 8,
		"sources": map[string]any{"ckg": map[string]any{"graph_digest": "d1", "src_commit": "abc"}},
	})
}

func TestPromoteAndRollback(t *testing.T) {
	ds := t.TempDir()
	for _, v := range []string{"v1", "v2"} {
		if err := os.MkdirAll(filepath.Join(ds, v), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	prev, err := Promote(ds, "v1")
	if err != nil || prev != "" {
		t.Fatalf("promote v1: prev=%q err=%v", prev, err)
	}
	if got, _ := os.Readlink(filepath.Join(ds, "current")); got != "v1" {
		t.Errorf("current -> %q, want v1", got)
	}
	prev, err = Promote(ds, "v2")
	if err != nil || prev != "v1" {
		t.Fatalf("promote v2: prev=%q (want v1) err=%v", prev, err)
	}
	if got, _ := os.Readlink(filepath.Join(ds, "current")); got != "v2" {
		t.Errorf("current -> %q, want v2", got)
	}
	if err := Rollback(ds, "v1"); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if got, _ := os.Readlink(filepath.Join(ds, "current")); got != "v1" {
		t.Errorf("after rollback current -> %q, want v1", got)
	}
	if _, err := Promote(ds, "missing"); err == nil {
		t.Error("promote of a missing version should fail")
	}
}

func TestReindexLock(t *testing.T) {
	ds := t.TempDir()
	l, err := acquireReindexLock(ds)
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	if _, err := acquireReindexLock(ds); err == nil {
		t.Error("second acquire should fail while held")
	}
	l.release()
	if l2, err := acquireReindexLock(ds); err != nil {
		t.Errorf("acquire after release: %v", err)
	} else {
		l2.release()
	}
}

// gateRunner passes ckg validate/audit unless a step ID/cmd is in fail.
type gateRunner struct{ failValidate bool }

func (g gateRunner) Run(_ context.Context, s Step, emit func(Event)) error {
	if s.Verify != nil {
		return s.Verify(emit)
	}
	if g.failValidate && len(s.Cmd) > 1 && s.Cmd[1] == "validate" {
		return validateFail{}
	}
	return nil
}

type validateFail struct{}

func (validateFail) Error() string { return "validate exit 1" }

func TestGate(t *testing.T) {
	mk := func(t *testing.T, mutate func(v map[string]any)) string {
		ds := t.TempDir()
		ver := filepath.Join(ds, "v1")
		writeAlignedVersion(t, ver)
		if mutate != nil {
			var m map[string]any
			readJSON(filepath.Join(ver, "vector", "manifest.json"), &m)
			mutate(m)
			writeManifest(t, filepath.Join(ver, "vector"), m)
		}
		return ds
	}
	opt := GateOptions{GraphBin: "ckg", Src: "/src", MinCanonicalRatio: 0.9}

	if err := Gate(context.Background(), mk(t, nil), "v1", opt, gateRunner{}, nil); err != nil {
		t.Errorf("aligned version should pass the gate, got %v", err)
	}
	// empty index
	ds := mk(t, func(v map[string]any) { v["chunk_count"] = 0 })
	if err := Gate(context.Background(), ds, "v1", opt, gateRunner{}, nil); err == nil || !strings.Contains(err.Error(), "chunk") {
		t.Errorf("chunk_count=0 should fail, got %v", err)
	}
	// sparse canonical coverage (1/8 = 12.5% < 90%)
	ds = mk(t, func(v map[string]any) { v["canonical_count"] = 1 })
	if err := Gate(context.Background(), ds, "v1", opt, gateRunner{}, nil); err == nil || !strings.Contains(err.Error(), "canonical") {
		t.Errorf("sparse canonical coverage should fail, got %v", err)
	}
	// ckg validate failure (hard)
	if err := Gate(context.Background(), mk(t, nil), "v1", opt, gateRunner{failValidate: true}, nil); err == nil || !strings.Contains(err.Error(), "validate") {
		t.Errorf("validate failure should fail the gate, got %v", err)
	}
}

// buildRunner simulates a build by materializing aligned manifests under each
// build step's --out, and passes the gate's validate/audit + verify steps.
type buildRunner struct {
	t        *testing.T
	badChunk bool // write chunk_count=0 to force a gate failure
}

func (b buildRunner) Run(_ context.Context, s Step, emit func(Event)) error {
	if s.Verify != nil {
		return s.Verify(emit)
	}
	out := argAfter(s.Cmd, "--out")
	switch {
	case strings.HasSuffix(out, "graph"):
		writeManifest(b.t, out, map[string]any{"src_commit": "abc", "graph_digest": "d1", "schema_version": "1.23"})
	case strings.HasSuffix(out, "vector"):
		cc := 10
		if b.badChunk {
			cc = 0
		}
		writeManifest(b.t, out, map[string]any{
			"src_commit": "abc", "chunk_count": cc, "symbol_count": 8, "canonical_count": 8,
			"sources": map[string]any{"ckg": map[string]any{"graph_digest": "d1", "src_commit": "abc"}},
		})
	}
	return nil
}

func argAfter(cmd []string, flag string) string {
	for i, a := range cmd {
		if a == flag && i+1 < len(cmd) {
			return cmd[i+1]
		}
	}
	return ""
}

func TestReindex(t *testing.T) {
	ds := t.TempDir()
	o := Options{Src: "/src", Out: ds, GraphBin: "ckg", VectorBin: "ckv"}
	gopt := GateOptions{MinCanonicalRatio: 0.9}

	// happy path: build v1 → gate → promote.
	if err := Reindex(context.Background(), o, "v1", gopt, buildRunner{t: t}, nil); err != nil {
		t.Fatalf("reindex v1: %v", err)
	}
	if got, _ := os.Readlink(filepath.Join(ds, "current")); got != "v1" {
		t.Fatalf("current -> %q, want v1", got)
	}

	// gate failure: current must stay on v1, and v2 kept for diagnosis.
	err := Reindex(context.Background(), o, "v2", gopt, buildRunner{t: t, badChunk: true}, nil)
	if err == nil {
		t.Fatal("reindex with empty vector index should fail the gate")
	}
	if got, _ := os.Readlink(filepath.Join(ds, "current")); got != "v1" {
		t.Errorf("after gate failure current -> %q, want v1 (unchanged)", got)
	}
	if _, statErr := os.Stat(filepath.Join(ds, "v2")); statErr != nil {
		t.Errorf("failed version v2 should be kept for diagnosis: %v", statErr)
	}
	// lock released after each run
	if l, err := acquireReindexLock(ds); err != nil {
		t.Errorf("lock not released after reindex: %v", err)
	} else {
		l.release()
	}
}
