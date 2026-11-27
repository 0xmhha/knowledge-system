package mcphandlers

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/graph/buildpipe"
	"github.com/0xmhha/knowledge-system/internal/graph/persist"
	"github.com/0xmhha/knowledge-system/internal/graph/temporal"
)

// historyFixture is a git repository whose function F was changed by three
// commits: an upstream one with a pull request number, one of the fork with
// none, and one of the fork with its own number; a fourth commit changes
// only G. It returns the source root and the commit ids, oldest first.
func historyFixture(t *testing.T) (string, []string) {
	t.Helper()
	dir := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Ada", "GIT_AUTHOR_EMAIL=ada@example.com",
			"GIT_COMMITTER_NAME=Ada", "GIT_COMMITTER_EMAIL=ada@example.com")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	write := func(body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	commit := func(date, msg string) string {
		t.Helper()
		git("add", "-A")
		cmd := exec.Command("git", "-C", dir, "commit", "-q", "-m", msg)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Ada", "GIT_AUTHOR_EMAIL=ada@example.com",
			"GIT_COMMITTER_NAME=Ada", "GIT_COMMITTER_EMAIL=ada@example.com",
			"GIT_AUTHOR_DATE="+date, "GIT_COMMITTER_DATE="+date)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("commit: %v\n%s", err, out)
		}
		return git("rev-parse", "HEAD")
	}
	git("init", "-q", "-b", "work")
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/a\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	const tail = "\nfunc G() int {\n\treturn 1\n}\n"
	write("package a\n\nfunc F() int {\n\treturn 1\n}\n" + tail)
	c1 := commit("2026-01-01T00:00:00Z", "upstream: add F (#5)")
	write("package a\n\nfunc F() int {\n\treturn 2\n}\n" + tail)
	c2 := commit("2026-02-01T00:00:00Z", "fork: change F without a pull request")
	write("package a\n\nfunc F() int {\n\treturn 3\n}\n" + tail)
	c3 := commit("2026-03-01T00:00:00Z", "fork: change F again (#9)")
	write("package a\n\nfunc F() int {\n\treturn 3\n}\n\nfunc G() int {\n\treturn 4\n}\n")
	c4 := commit("2026-04-01T00:00:00Z", "fork: change G (#10)")
	return dir, []string{c1, c2, c3, c4}
}

// TestChangeHistory_ListsEveryCommitWithItsNumberSpace builds a graph of the
// fixture and asks for F: every commit that changed F comes back, with or
// without a pull request number, the numbers placed in their repository,
// G's commit left out, and the k cut reported with the total.
func TestChangeHistory_ListsEveryCommitWithItsNumberSpace(t *testing.T) {
	src, shas := historyFixture(t)
	out := t.TempDir()
	if _, err := buildpipe.Run(buildpipe.Options{
		SrcRoot: src, OutDir: out, Languages: []string{"auto"}, CKGVersion: "test",
		NumberSpaces: temporal.NumberSpaces{Repo: "fork/repo", UpstreamRepo: "up/repo", UpstreamBase: shas[0]},
	}); err != nil {
		t.Fatalf("build: %v", err)
	}
	reader, err := persist.OpenReadOnly(filepath.Join(out, "graph.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = reader.Close() }()

	got := callTool(t, reader, "change_history", map[string]any{"qname": "a.F", "k": 2})
	raw, _ := json.Marshal(got)
	var res struct {
		Commits []struct {
			SHA, Author, Subject string
			AuthorTime           string `json:"author_time"`
			CommitterTime        string `json:"committer_time"`
			PR                   *struct {
				Number   int
				Repo     string
				Upstream bool
			}
		}
		Total  int `json:"commits_total"`
		WithPR int `json:"commits_with_pr"`
		Cut    []struct {
			By          string
			Kept, Total int
		}
		Coverage string
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		t.Fatal(err)
	}
	if res.Total != 3 || res.WithPR != 2 {
		t.Fatalf("commits_total = %d, commits_with_pr = %d; want 3 and 2 (G's commit is not F's): %s", res.Total, res.WithPR, raw)
	}
	if len(res.Commits) != 2 || res.Commits[0].SHA != shas[2] || res.Commits[1].SHA != shas[1] {
		t.Fatalf("commits = %+v, want the newest two of F: %s, %s", res.Commits, shas[2], shas[1])
	}
	if c := res.Commits[0]; c.Author != "Ada" || c.Subject != "fork: change F again (#9)" ||
		c.AuthorTime != "2026-03-01T00:00:00Z" || c.CommitterTime != "2026-03-01T00:00:00Z" ||
		c.PR == nil || c.PR.Number != 9 || c.PR.Repo != "fork/repo" || c.PR.Upstream {
		t.Errorf("newest commit = %+v", c)
	}
	if res.Commits[1].PR != nil {
		t.Errorf("a commit without a number is listed without a pr: %+v", res.Commits[1])
	}
	if len(res.Cut) != 1 || res.Cut[0].By != "k" || res.Cut[0].Kept != 2 || res.Cut[0].Total != 3 {
		t.Errorf("cut = %+v, want k cutting 3 to 2", res.Cut)
	}
	if !strings.Contains(res.Coverage, "up/repo") {
		t.Errorf("coverage does not state the number-space rule: %q", res.Coverage)
	}

	all := callTool(t, reader, "change_history", map[string]any{"qname": "a.F", "k": 3})
	raw, _ = json.Marshal(all)
	if !strings.Contains(string(raw), `"repo":"up/repo","reverts":[],"reverts_repo":"","upstream":true`) {
		t.Errorf("the upstream commit's pull request is not placed upstream: %s", raw)
	}

	// The input forms: one file in any spelling, a line of a file, a
	// commit, a path the graph does not hold.
	for _, tc := range []struct {
		qname string
		total int
		input string
	}{
		{"a.go", 4, ""},
		{"./a.go", 4, ""},
		{filepath.Join(src, "a.go"), 4, ""},
		{"a.go:4", 3, ""},
		{"a.go@4", 3, ""},
		{shas[0], 0, inputCommit},
		{"commit:" + shas[0], 0, inputCommit},
		{"gone/old.go", 0, inputNoFile},
		{"x/../a.go", 4, ""},
		{".//a.go", 4, ""},
		{`.\a.go`, 4, ""},
		{"A.go", 0, inputWrongCase},
		{"go.mod", 0, inputNotSource},
		{"", 0, inputEmpty},
		{" ", 0, inputEmpty},
		{"/elsewhere/a.go", 0, inputOutsideSrc},
		{"a.NoSuch", 0, inputSymbol},
		{"a.go:abc", 0, inputBadLine},
		{"a.go:-5", 0, inputBadLine},
		{"a.go:0", 0, inputBadLine},
		{"x/../../etc/passwd", 0, inputOutsideSrc},
		{"hunk:0123:a.go:0", 0, inputHunk},
	} {
		out := callTool(t, reader, "change_history", map[string]any{"qname": tc.qname})
		total, _ := out[keyCommitsTotal].(float64)
		if int(total) != tc.total || (tc.input != "" && out["input"] != tc.input) {
			t.Errorf("change_history(%q): commits_total %v, input %v; want %d, %q", tc.qname, out[keyCommitsTotal], out["input"], tc.total, tc.input)
		}
		if tc.input != "" && !strings.HasPrefix(fmt.Sprint(out[keyNote]), "taken for") && tc.input != inputEmpty {
			t.Errorf("change_history(%q): the note does not say what the input was taken for: %v", tc.qname, out[keyNote])
		}
		if tc.total > 0 && out[keyCheck] == nil {
			t.Errorf("change_history(%q): no check command", tc.qname)
		}
	}
}
