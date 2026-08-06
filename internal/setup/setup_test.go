package setup

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeScript drops an executable shell script into dir and returns its path.
func writeScript(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func writeManifest(t *testing.T, dir string, v any) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	buf, _ := json.Marshal(v)
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), buf, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestBuildPlan_Shape(t *testing.T) {
	p, err := BuildPlan(Options{
		Src: "/s", Out: "/o",
		PolicyFile: "pol.yaml", Embedder: "ollama", ModelName: "bge-m3", OllamaURL: "http://x",
	})
	if err != nil {
		t.Fatal(err)
	}
	// ollama backend inserts a preflight step: graph, preflight, vector, verify.
	if len(p.Steps) != 4 {
		t.Fatalf("steps = %d, want 4", len(p.Steps))
	}
	g := strings.Join(p.Steps[0].Cmd, " ")
	if !strings.Contains(g, "ckg build --src /s --out /o/graph") || !strings.Contains(g, "--policy-file pol.yaml") {
		t.Errorf("graph cmd = %q", g)
	}
	if p.Steps[1].ID != "vector-preflight" || p.Steps[1].Verify == nil {
		t.Errorf("step[1] should be the ollama preflight, got id=%q verify=%v", p.Steps[1].ID, p.Steps[1].Verify != nil)
	}
	v := strings.Join(p.Steps[2].Cmd, " ")
	if !strings.Contains(v, "--ckg /o/graph") || !strings.Contains(v, "--embedder=ollama") {
		t.Errorf("vector cmd = %q", v)
	}
	if len(p.Steps[2].Env) != 1 || p.Steps[2].Env[0] != "CKV_OLLAMA_ENDPOINT=http://x" {
		t.Errorf("vector env = %v", p.Steps[2].Env)
	}
	if p.Steps[3].Verify == nil {
		t.Error("verify step has no Verify func")
	}

	// SkipVector trims to the graph build only.
	p2, err := BuildPlan(Options{Src: "/s", Out: "/o", SkipVector: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(p2.Steps) != 1 {
		t.Errorf("SkipVector steps = %d, want 1", len(p2.Steps))
	}

	if _, err := BuildPlan(Options{Out: "/o"}); err == nil {
		t.Error("missing Src not rejected")
	}

	// FilelistConfig prepends the derivation step and scopes both builds.
	p3, err := BuildPlan(Options{Src: "/s", Out: "/o", FilelistConfig: "/p/filelist.yaml"})
	if err != nil {
		t.Fatal(err)
	}
	if p3.Steps[0].ID != "filelist-derive" {
		t.Fatalf("step[0] = %q, want filelist-derive", p3.Steps[0].ID)
	}
	d := strings.Join(p3.Steps[0].Cmd, " ")
	if !strings.Contains(d, "cks filelist --src /s --config /p/filelist.yaml --out /o/files-from.json") {
		t.Errorf("derive cmd = %q", d)
	}
	for _, i := range []int{1, 2} { // graph-build, vector-build
		c := strings.Join(p3.Steps[i].Cmd, " ")
		if !strings.Contains(c, "--files-from /o/files-from.json") {
			t.Errorf("step %s missing --files-from: %q", p3.Steps[i].ID, c)
		}
	}
}

func TestExecute_SubprocessOrderOutputAndFailure(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "order.txt")
	ok := writeScript(t, dir, "ok.sh", fmt.Sprintf("echo line-one\necho %s >> %s\n", "step-ran", marker))
	fail := writeScript(t, dir, "fail.sh", "echo about-to-fail\nexit 3\n")

	var events []Event
	emit := func(e Event) { events = append(events, e) }

	plan := Plan{Steps: []Step{
		{ID: "a", Cmd: []string{ok}},
		{ID: "b", Cmd: []string{fail}},
		{ID: "never", Cmd: []string{ok}},
	}}
	err := Execute(context.Background(), plan, SubprocessRunner{}, emit)
	if err == nil || !strings.Contains(err.Error(), "step b") {
		t.Fatalf("Execute err = %v, want step b failure", err)
	}

	var types []string
	for _, e := range events {
		types = append(types, e.Step+":"+e.Type)
	}
	joined := strings.Join(types, " ")
	// a completes, b starts and errors, "never" never starts.
	for _, want := range []string{"a:start", "a:output", "a:done", "b:start", "b:error"} {
		if !strings.Contains(joined, want) {
			t.Errorf("events missing %s in %v", want, types)
		}
	}
	if strings.Contains(joined, "never:") {
		t.Errorf("step after failure ran: %v", types)
	}
	// Output line captured verbatim.
	found := false
	for _, e := range events {
		if e.Step == "a" && e.Type == "output" && e.Message == "line-one" {
			found = true
		}
	}
	if !found {
		t.Error("stdout line not captured as output event")
	}
}

func TestVerifyAlignment(t *testing.T) {
	root := t.TempDir()
	g := filepath.Join(root, "graph")
	v := filepath.Join(root, "vector")

	// Aligned: same commit, matching pin.
	writeManifest(t, g, map[string]any{"src_commit": "abc", "graph_digest": "d1", "schema_version": "1.23"})
	writeManifest(t, v, map[string]any{
		"src_commit": "abc",
		"sources":    map[string]any{"ckg": map[string]any{"graph_digest": "d1", "src_commit": "abc"}},
	})
	if err := VerifyAlignment(g, v, nil); err != nil {
		t.Fatalf("aligned: %v", err)
	}

	// Pin mismatch fails.
	writeManifest(t, v, map[string]any{
		"src_commit": "abc",
		"sources":    map[string]any{"ckg": map[string]any{"graph_digest": "OTHER", "src_commit": "abc"}},
	})
	if err := VerifyAlignment(g, v, nil); err == nil || !strings.Contains(err.Error(), "pin mismatch") {
		t.Fatalf("pin mismatch: err = %v", err)
	}

	// Commit divergence fails.
	writeManifest(t, v, map[string]any{
		"src_commit": "zzz",
		"sources":    map[string]any{"ckg": map[string]any{"graph_digest": "d1", "src_commit": "zzz"}},
	})
	if err := VerifyAlignment(g, v, nil); err == nil || !strings.Contains(err.Error(), "different commits") {
		t.Fatalf("commit divergence: err = %v", err)
	}

	// Missing ledger degrades to warnings, not failure.
	var warns []Event
	writeManifest(t, v, map[string]any{"src_commit": "abc"})
	if err := VerifyAlignment(g, v, func(e Event) { warns = append(warns, e) }); err != nil {
		t.Fatalf("ledger-less: %v", err)
	}
	if len(warns) == 0 {
		t.Error("expected warnings for absent ledger")
	}
}

func TestJobs_AsyncLifecycle(t *testing.T) {
	dir := t.TempDir()
	ok := writeScript(t, dir, "ok.sh", "echo working\n")
	js := NewJobs(SubprocessRunner{})
	id := js.Start(Plan{Steps: []Step{{ID: "a", Cmd: []string{ok}}}})

	deadline := time.Now().Add(10 * time.Second)
	for {
		snap, found := js.Get(id, 0)
		if !found {
			t.Fatal("job not found")
		}
		if snap.State != JobRunning {
			if snap.State != JobDone {
				t.Fatalf("state = %s (err %s), want done", snap.State, snap.Error)
			}
			if len(snap.Events) == 0 {
				t.Error("no events recorded")
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("job did not finish")
		}
		time.Sleep(20 * time.Millisecond)
	}

	if _, found := js.Get("nope", 0); found {
		t.Error("unknown job id reported found")
	}
}

// waitJob polls until the job leaves the running state or the deadline passes.
func waitJob(t *testing.T, js *Jobs, id string) JobSnapshot {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		snap, found := js.Get(id, 0)
		if !found {
			t.Fatalf("job %s not found", id)
		}
		if snap.State != JobRunning {
			return snap
		}
		if time.Now().After(deadline) {
			t.Fatalf("job %s did not finish", id)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// TestStartFunc_GenericJob covers the generalized async core: an arbitrary
// function runs in the background, its kind labels the id, emitted events are
// captured, and a returned error becomes the failed state.
func TestStartFunc_GenericJob(t *testing.T) {
	js := NewJobs(SubprocessRunner{})

	okID := js.StartFunc("probe", func(ctx context.Context, emit func(Event)) error {
		emit(Event{Step: "s", Type: "output", Message: "hello"})
		return nil
	})
	if !strings.HasPrefix(okID, "probe-") {
		t.Errorf("job id %q should carry the kind prefix", okID)
	}
	snap := waitJob(t, js, okID)
	if snap.State != JobDone {
		t.Errorf("state = %s, want done", snap.State)
	}
	if len(snap.Events) != 1 || snap.Events[0].Message != "hello" {
		t.Errorf("events = %+v, want the emitted one captured", snap.Events)
	}

	failID := js.StartFunc("probe", func(ctx context.Context, emit func(Event)) error {
		return errors.New("boom")
	})
	fs := waitJob(t, js, failID)
	if fs.State != JobFailed || fs.Error != "boom" {
		t.Errorf("failed job = state %s err %q, want failed/boom", fs.State, fs.Error)
	}
}

// TestJobs_EventsCapped bounds per-job event retention so a chatty build does
// not grow memory without limit.
func TestJobs_EventsCapped(t *testing.T) {
	js := NewJobs(SubprocessRunner{})
	id := js.StartFunc("probe", func(ctx context.Context, emit func(Event)) error {
		for i := 0; i < maxJobEvents+50; i++ {
			emit(Event{Type: "output", Message: fmt.Sprintf("line-%d", i)})
		}
		return nil
	})
	snap := waitJob(t, js, id)
	if len(snap.Events) != maxJobEvents {
		t.Errorf("retained %d events, want the cap %d", len(snap.Events), maxJobEvents)
	}
	// The most recent event is kept (the tail, not the head).
	if last := snap.Events[len(snap.Events)-1].Message; last != fmt.Sprintf("line-%d", maxJobEvents+49) {
		t.Errorf("last event = %q, want the newest", last)
	}
}

// TestJobs_TerminalEviction bounds how many finished jobs are retained.
func TestJobs_TerminalEviction(t *testing.T) {
	js := NewJobs(SubprocessRunner{})
	for i := 0; i < maxRetainedJobs+20; i++ {
		id := js.StartFunc("probe", func(ctx context.Context, emit func(Event)) error { return nil })
		waitJob(t, js, id)
	}
	// One more triggers a final eviction pass.
	waitJob(t, js, js.StartFunc("probe", func(ctx context.Context, emit func(Event)) error { return nil }))
	js.mu.Lock()
	n := len(js.byID)
	js.mu.Unlock()
	if n > maxRetainedJobs+1 {
		t.Errorf("registry retained %d jobs, want <= %d", n, maxRetainedJobs+1)
	}
}

// TestJobs_CancelAndShutdown covers cooperative cancellation: a blocked job
// finishes failed when its context is cancelled, via Cancel(id) or Shutdown.
func TestJobs_CancelAndShutdown(t *testing.T) {
	block := func(ctx context.Context, emit func(Event)) error { <-ctx.Done(); return ctx.Err() }

	js := NewJobs(SubprocessRunner{})
	id := js.StartFunc("probe", block)
	if !js.Cancel(id) {
		t.Fatal("Cancel returned false for a live job")
	}
	if snap := waitJob(t, js, id); snap.State != JobFailed {
		t.Errorf("cancelled job state = %s, want failed", snap.State)
	}
	if js.Cancel("nope") {
		t.Error("Cancel on unknown id should be false")
	}

	js2 := NewJobs(SubprocessRunner{})
	id2 := js2.StartFunc("probe", block)
	js2.Shutdown()
	if snap := waitJob(t, js2, id2); snap.State != JobFailed {
		t.Errorf("job after Shutdown state = %s, want failed", snap.State)
	}
}

// TestValidateVersion guards the reindex version label against values that
// would escape the dataset root or build through the live `current` symlink.
func TestValidateVersion(t *testing.T) {
	bad := []string{"", "current", "..", ".", ".hidden", "a/b", "/abs", "../evil"}
	for _, v := range bad {
		if err := validateVersion(v); err == nil {
			t.Errorf("validateVersion(%q) = nil, want an error", v)
		}
	}
	good := []string{"v1785162032", "0bf2f4d1b-a1b2c3d4", "pr-77-2"}
	for _, v := range good {
		if err := validateVersion(v); err != nil {
			t.Errorf("validateVersion(%q) = %v, want nil", v, err)
		}
	}
}

// TestStartReindex_MissingVersion exercises the reindex job wrapper: it runs
// Reindex through the shared registry and surfaces its error as a failed job
// (empty version is rejected by Reindex).
func TestStartReindex_MissingVersion(t *testing.T) {
	js := NewJobs(SubprocessRunner{})
	id := js.StartReindex(Options{Src: t.TempDir(), Out: t.TempDir()}, "", GateOptions{})
	if !strings.HasPrefix(id, "reindex-") {
		t.Errorf("job id %q should carry the reindex prefix", id)
	}
	snap := waitJob(t, js, id)
	if snap.State != JobFailed || snap.Error == "" {
		t.Errorf("empty-version reindex = state %s err %q, want failed", snap.State, snap.Error)
	}
}

// TestVerifyAlignment_SchemaGate pins the ADR-007 canonical_id floor: the
// vector<->graph join exists only from graph schema 1.19, so alignment against
// an older graph must fail loud (not silently pass on a matching digest). An
// unparseable/absent version degrades to a warning.
func TestVerifyAlignment_SchemaGate(t *testing.T) {
	mk := func(t *testing.T, schema string) (string, string) {
		root := t.TempDir()
		g := filepath.Join(root, "graph")
		v := filepath.Join(root, "vector")
		writeManifest(t, g, map[string]any{"src_commit": "abc", "graph_digest": "d1", "schema_version": schema})
		writeManifest(t, v, map[string]any{"src_commit": "abc", "sources": map[string]any{"ckg": map[string]any{"graph_digest": "d1", "src_commit": "abc"}}})
		return g, v
	}
	for _, s := range []string{"1.18", "1.9"} { // below floor (1.9 also guards lexical-vs-numeric)
		g, v := mk(t, s)
		if err := VerifyAlignment(g, v, nil); err == nil || !strings.Contains(err.Error(), "schema") {
			t.Errorf("schema %s: want schema-gate error, got %v", s, err)
		}
	}
	for _, s := range []string{"1.19", "1.23", "2.0"} { // at/above floor
		g, v := mk(t, s)
		if err := VerifyAlignment(g, v, nil); err != nil {
			t.Errorf("schema %s: want ok, got %v", s, err)
		}
	}
	g, v := mk(t, "") // unparseable → warn, not error
	var warns []Event
	if err := VerifyAlignment(g, v, func(e Event) { warns = append(warns, e) }); err != nil {
		t.Errorf("empty schema: want ok+warn, got err %v", err)
	}
	if len(warns) == 0 {
		t.Error("empty schema: expected a warning")
	}
}

// TestBuildPlan_DomainArtifacts covers the three derivation steps and the two
// places their output is consumed. The regression this guards: the derivation
// used to live in a shell script that a refactor replaced with a thinner
// wrapper, dropping the corpus and the policy sync without any test noticing.
func TestBuildPlan_DomainArtifacts(t *testing.T) {
	p, err := BuildPlan(Options{
		Src: "/s", Out: "/o",
		DomainKnowledge: "/pack/domain-knowledge",
		CodeRoot:        "/checkout",
		FlowCorpus:      "/pack/domain-knowledge/flow-corpus/corpus.jsonl",
		PolicyFile:      "/pack/policies/graph.yaml",
		GlossaryFile:    "/pack/domain-knowledge/glossary.yaml",
	})
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, len(p.Steps))
	for i, s := range p.Steps {
		ids[i] = s.ID
	}
	want := []string{"domain-export", "policy-fresh", "glossary-fresh", "graph-build", "vector-build", "verify-align", "verify-content"}
	if strings.Join(ids, ",") != strings.Join(want, ",") {
		t.Fatalf("step ids = %v, want %v", ids, want)
	}

	// The corpus is the only artifact written; it defaults beside the pack.
	if e := strings.Join(p.Steps[0].Cmd, " "); !strings.Contains(e, "--out /pack/generated/domain-corpus") ||
		!strings.Contains(e, "--code-root /checkout") {
		t.Errorf("export cmd = %q", e)
	}
	// The policy and glossary checks are in-process; nothing is written for
	// them, so a second copy of either cannot drift from the committed one.
	for _, i := range []int{1, 2} {
		if p.Steps[i].Verify == nil || p.Steps[i].Cmd != nil {
			t.Errorf("step %q should be an in-process check, got cmd=%v", p.Steps[i].ID, p.Steps[i].Cmd)
		}
	}

	// The graph build consumes the committed policy — the one reviewers see.
	graph := strings.Join(p.Steps[3].Cmd, " ")
	if !strings.Contains(graph, "--policy-file /pack/policies/graph.yaml") {
		t.Errorf("graph cmd should use the committed policy, got %q", graph)
	}
	if strings.Contains(graph, "generated") {
		t.Errorf("graph cmd should not reference a derived policy: %q", graph)
	}

	vec := strings.Join(p.Steps[4].Cmd, " ")
	if !strings.Contains(vec, "--docs /pack/generated/domain-corpus") {
		t.Errorf("vector cmd missing --docs: %q", vec)
	}
	if !strings.Contains(vec, "--flow-corpus /pack/domain-knowledge/flow-corpus/corpus.jsonl") {
		t.Errorf("vector cmd missing --flow-corpus: %q", vec)
	}

	// Without domain knowledge the plan is unchanged.
	p2, err := BuildPlan(Options{Src: "/s", Out: "/o", PolicyFile: "/pack/policies/graph.yaml"})
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range p2.Steps {
		if s.ID == "verify-content" || s.ID == "domain-export" || s.ID == "policy-fresh" {
			t.Errorf("unexpected step %q when DomainKnowledge is unset", s.ID)
		}
	}
	if g := strings.Join(p2.Steps[0].Cmd, " "); !strings.Contains(g, "--policy-file /pack/policies/graph.yaml") {
		t.Errorf("configured policy should still be used: %q", g)
	}
}

// TestVerifyDerivedFresh covers the check that replaced writing a second copy
// of a committed artifact: a stale file must stop the build, and the error has
// to say which file and how to fix it.
func TestVerifyDerivedFresh(t *testing.T) {
	dir := t.TempDir()
	committed := filepath.Join(dir, "graph.yaml")
	gen := writeScript(t, dir, "gen.sh", `printf 'policies: [a, b]\n' > "$2"`+"\n")

	if err := os.WriteFile(committed, []byte("policies: [a, b]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := VerifyDerivedFresh("s", "the policy", []string{gen, "--out"}, committed, nil); err != nil {
		t.Errorf("matching copy should pass, got %v", err)
	}

	if err := os.WriteFile(committed, []byte("policies: [a]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := VerifyDerivedFresh("s", "the policy", []string{gen, "--out"}, committed, nil)
	if err == nil {
		t.Fatal("a stale committed copy must fail the build")
	}
	if !strings.Contains(err.Error(), committed) || !strings.Contains(err.Error(), "sync-domain-artifacts") {
		t.Errorf("error should name the file and the fix, got %v", err)
	}
}

// TestVerifyContent covers the gate that would have caught a corpus flag that
// was never passed, and an authoritative doc that silently failed to resolve.
func TestVerifyContent(t *testing.T) {
	newDataset := func(t *testing.T, manifest any) Options {
		t.Helper()
		dir := t.TempDir()
		writeManifest(t, filepath.Join(dir, "vector"), manifest)
		return Options{Out: dir, DomainKnowledge: filepath.Join(dir, "pack", "domain-knowledge")}
	}

	t.Run("corpus reached the index", func(t *testing.T) {
		o := newDataset(t, map[string]any{"languages": map[string]int{"markdown": 421}})
		o.DerivedDir = filepath.Join(t.TempDir(), "generated")
		writeManifest(t, filepath.Join(o.Out, "vector"), map[string]any{
			"docs_roots": []string{o.DomainCorpusDir()},
			"languages":  map[string]int{"markdown": 421},
		})
		if err := VerifyContent(o, nil); err != nil {
			t.Errorf("want pass, got %v", err)
		}
	})

	t.Run("corpus declared but never passed", func(t *testing.T) {
		o := newDataset(t, map[string]any{"docs_roots": []string{}, "languages": map[string]int{"go": 100}})
		err := VerifyContent(o, nil)
		if err == nil {
			t.Fatal("a corpus that never reached the index must fail the build")
		}
		if !strings.Contains(err.Error(), "docs roots") {
			t.Errorf("error should name the missing corpus, got %v", err)
		}
	})

	t.Run("corpus passed but empty", func(t *testing.T) {
		o := newDataset(t, nil)
		writeManifest(t, filepath.Join(o.Out, "vector"), map[string]any{
			"docs_roots": []string{o.DomainCorpusDir()},
			"languages":  map[string]int{"go": 100},
		})
		if err := VerifyContent(o, nil); err == nil {
			t.Error("an empty corpus directory must fail the build")
		}
	})

	t.Run("flow corpus declared but never passed", func(t *testing.T) {
		dir := t.TempDir()
		writeManifest(t, filepath.Join(dir, "vector"), map[string]any{"docs_roots": []string{}})
		o := Options{Out: dir, FlowCorpus: filepath.Join(dir, "flow", "corpus.jsonl")}
		if err := VerifyContent(o, nil); err == nil {
			t.Error("a flow corpus that never reached the index must fail the build")
		}
	})
}
