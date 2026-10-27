package eval

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	mcpgo "github.com/mark3labs/mcp-go/mcp"

	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

// gitRepo makes a two-commit repository: a.go is the same in both, b.go
// changes in the second. It returns the repository and both commit SHAs.
func gitRepo(t *testing.T) (dir, first, second string) {
	t.Helper()
	dir = t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false"}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	git("init", "-q")
	write("a.go", "package a\n")
	write("b.go", "package b\n")
	git("add", ".")
	git("commit", "-q", "-m", "one")
	first = git("rev-parse", "HEAD")
	write("b.go", "package b\n\nvar X = 1\n")
	git("commit", "-q", "-am", "two")
	second = git("rev-parse", "HEAD")
	return dir, first, second
}

// A reindexed vector index keeps the commit a file was first embedded at, so
// a citation can carry an older commit than expected_commit while pointing at
// identical content. With a commit repository, such a citation is scored when
// its file did not change between the two commits, and only then.
func TestRunner_Execute_AcceptsOlderCommitOnlyWhenTheFileIsUnchanged(t *testing.T) {
	t.Parallel()
	repo, first, second := gitRepo(t)
	for _, tc := range []struct {
		name          string
		file          string
		commitRepo    string
		wantRetrieval string
		wantSnapshot  string
		wantAccepted  int
	}{
		{name: "unchanged file is scored", file: "a.go", commitRepo: repo, wantRetrieval: "pass", wantSnapshot: "current", wantAccepted: 1},
		{name: "changed file stays a conflict", file: "b.go", commitRepo: repo, wantRetrieval: "miss", wantSnapshot: "conflict"},
		{name: "no commit repository keeps the strict match", file: "a.go", wantRetrieval: "miss", wantSnapshot: "conflict"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			pack := contract.EvidencePack{Citations: []contract.Citation{{File: tc.file, StartLine: 1, EndLine: 1, CommitHash: first}}}
			m := &mockMCPClient{callOut: map[string]*mcpgo.CallToolResult{
				toolGetForTask: packResult(pack),
				toolFreshness:  mcpgo.NewToolResultStructured(map[string]string{"indexed_head": second}, "freshness"),
			}}
			r := &Runner{client: m, commitRepo: tc.commitRepo}
			s := &Scenario{Version: 1, Name: "pinned", Prompt: "p", MatchMode: MatchOverlap, Runs: 1, ExpectedCommit: second,
				ExpectedCitations: []contract.Citation{cit(tc.file, 1, 1)}}
			got, err := r.Execute(context.Background(), s)
			if err != nil {
				t.Fatal(err)
			}
			if got.RetrievalState != tc.wantRetrieval || got.SnapshotState != tc.wantSnapshot || got.UnchangedCommitCitations != tc.wantAccepted {
				t.Fatalf("retrieval=%s snapshot=%s accepted=%d, want %s %s %d", got.RetrievalState, got.SnapshotState,
					got.UnchangedCommitCitations, tc.wantRetrieval, tc.wantSnapshot, tc.wantAccepted)
			}
		})
	}
}
