package setup

import (
	"context"
	"encoding/json"
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
	if len(p.Steps) != 3 {
		t.Fatalf("steps = %d, want 3", len(p.Steps))
	}
	g := strings.Join(p.Steps[0].Cmd, " ")
	if !strings.Contains(g, "ckg build --src /s --out /o/graph") || !strings.Contains(g, "--policy-file pol.yaml") {
		t.Errorf("graph cmd = %q", g)
	}
	v := strings.Join(p.Steps[1].Cmd, " ")
	if !strings.Contains(v, "--ckg /o/graph") || !strings.Contains(v, "--embedder=ollama") {
		t.Errorf("vector cmd = %q", v)
	}
	if len(p.Steps[1].Env) != 1 || p.Steps[1].Env[0] != "CKV_OLLAMA_ENDPOINT=http://x" {
		t.Errorf("vector env = %v", p.Steps[1].Env)
	}
	if p.Steps[2].Verify == nil {
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
