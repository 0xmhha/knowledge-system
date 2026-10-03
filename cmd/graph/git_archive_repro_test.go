package main

import (
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/graph/persist"
	"github.com/0xmhha/knowledge-system/internal/setup"
)

func TestCapturedGitHistoryReplaysSameRecoveryGraphAfterSourceRemoval(t *testing.T) {
	root := t.TempDir()
	git := func(dir string, args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git(root, "init", "-q")
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/fixture\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "main.go")
	if err := os.WriteFile(file, []byte("package main\n\nfunc Kept() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(root, "add", ".")
	commit := []string{"-c", "commit.gpgsign=false", "-c", "user.email=test@example.org", "-c", "user.name=Test", "commit", "-qm"}
	git(root, append(slices.Clone(commit), "base")...)
	base := git(root, "rev-parse", "HEAD")
	if err := os.WriteFile(file, []byte("package main\n\nfunc Kept() {}\nfunc Lost() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(root, "add", ".")
	git(root, append(slices.Clone(commit), "abandoned")...)
	lost := git(root, "rev-parse", "HEAD")
	git(root, "reset", "--hard", base)
	version := filepath.Join(t.TempDir(), "version")
	captured, err := setup.CaptureSource(setup.CaptureOptions{Root: root, Out: version,
		ProjectID: "fixture", SourceMode: "committed", SourceCommit: base})
	if err != nil {
		t.Fatal(err)
	}
	type graphEvidence struct {
		Digest string
		Nodes  int
		Edges  int
		Meta   []string
	}
	build := func(name string) graphEvidence {
		t.Helper()
		stage := filepath.Join(t.TempDir(), name+"-src")
		cleanup, err := captured.MaterializeBuildTree(stage)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = cleanup() }()
		out := filepath.Join(t.TempDir(), name+"-graph")
		cmd := newBuildCmd()
		cmd.SetArgs([]string{"--src", stage, "--out", out, "--no-cache", "--fail-on-parse-errors"})
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("CKG graph build: %v", err)
		}
		buf, err := os.ReadFile(filepath.Join(out, "manifest.json"))
		if err != nil {
			t.Fatal(err)
		}
		var manifest struct {
			GraphDigest string         `json:"graph_digest"`
			Stats       map[string]int `json:"stats"`
		}
		if err := json.Unmarshal(buf, &manifest); err != nil {
			t.Fatal(err)
		}
		store, err := persist.OpenReadOnly(filepath.Join(out, "graph.db"))
		if err != nil {
			t.Fatal(err)
		}
		defer store.Close()
		meta, err := store.AmbiguousMetaNodes()
		if err != nil {
			t.Fatal(err)
		}
		result := graphEvidence{Digest: manifest.GraphDigest,
			Nodes: manifest.Stats["nodes"], Edges: manifest.Stats["edges"]}
		foundLost := false
		for _, node := range meta {
			result.Meta = append(result.Meta, node.ID)
			if strings.Contains(node.QualifiedName, lost) {
				foundLost = true
			}
		}
		slices.Sort(result.Meta)
		if !foundLost || len(result.Meta) == 0 || result.Digest == "" {
			t.Fatal("abandoned commit, recovery graph or code graph digest missing")
		}
		return result
	}
	first := build("before")
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	if err := setup.VerifyRetainedSource(version, captured.Identity); err != nil {
		t.Fatal(err)
	}
	second := build("after")
	if first.Digest != second.Digest || first.Nodes != second.Nodes || first.Edges != second.Edges ||
		!slices.Equal(first.Meta, second.Meta) {
		t.Fatalf("source removal changed CKG graph: before=%+v after=%+v", first, second)
	}
}
