package setup

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gateGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestCandidateTestGateRecordsPassFailureAndSourceIdentity(t *testing.T) {
	repo := t.TempDir()
	gateGit(t, repo, "init", "-q")
	if err := os.WriteFile(filepath.Join(repo, "main.go"), []byte("package sample\n"), 0600); err != nil {
		t.Fatal(err)
	}
	gateGit(t, repo, "add", ".")
	gateGit(t, repo, "-c", "user.name=Test", "-c", "user.email=test@example.com", "-c", "commit.gpgsign=false", "commit", "-qm", "fixture")
	commit := gateGit(t, repo, "rev-parse", "HEAD")
	candidate := func(name string) string {
		dir := filepath.Join(t.TempDir(), name)
		writeManifest(t, filepath.Join(dir, "graph"), map[string]any{"src_commit": commit, "graph_digest": strings.Repeat("a", 64)})
		return dir
	}
	passedDir := candidate("passed")
	if err := runTestGate(context.Background(), passedDir, repo, []string{"go", "version"}); err != nil {
		t.Fatal(err)
	}
	var passed testGateReport
	data, err := os.ReadFile(filepath.Join(passedDir, "test-gate.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &passed); err != nil {
		t.Fatal(err)
	}
	if !passed.CommandPassed || !passed.SnapshotConsistent || passed.SourceCommit != commit || passed.OutputBytes == 0 {
		t.Fatalf("pass report: %+v", passed)
	}
	failedDir := candidate("failed")
	if err := runTestGate(context.Background(), failedDir, repo, []string{"go", "invalid-command"}); err == nil {
		t.Fatal("failed command passed")
	}
	data, err = os.ReadFile(filepath.Join(failedDir, "test-gate.json"))
	if err != nil {
		t.Fatal(err)
	}
	var failed testGateReport
	if err := json.Unmarshal(data, &failed); err != nil {
		t.Fatal(err)
	}
	if failed.CommandPassed || !failed.SnapshotConsistent || failed.ExitCode == 0 {
		t.Fatalf("failure report: %+v", failed)
	}
	mutator := writeScript(t, t.TempDir(), "mutate.sh", "printf 'package sample\\n' > generated.go\n")
	mutatedDir := candidate("mutated")
	if err := runTestGate(context.Background(), mutatedDir, repo, []string{mutator}); err == nil {
		t.Fatal("source-mutating command passed")
	}
	data, err = os.ReadFile(filepath.Join(mutatedDir, "test-gate.json"))
	if err != nil {
		t.Fatal(err)
	}
	var mutated testGateReport
	if err := json.Unmarshal(data, &mutated); err != nil {
		t.Fatal(err)
	}
	if !mutated.CommandPassed || mutated.SnapshotConsistent {
		t.Fatalf("mutation not detected: %+v", mutated)
	}
	if err := os.Remove(filepath.Join(repo, "generated.go")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "untracked.go"), []byte("package sample\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := runTestGate(context.Background(), candidate("dirty"), repo, []string{"go", "version"}); err == nil {
		t.Fatal("dirty source passed")
	}
}
