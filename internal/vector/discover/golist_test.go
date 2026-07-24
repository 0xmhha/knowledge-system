package discover

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// findRepoRoot walks upward until it finds the repository root — the
// directory containing go.mod and the relocated cmd/vector entry package —
// and returns it. Used to anchor `go list` in the repo that owns this test.
func findRepoRoot(t *testing.T) string {
	t.Helper()
	cwd, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skipf("`go` not in PATH — skipping golist test")
	}
	dir := cwd
	for {
		if isDir(filepath.Join(dir, "cmd", "vector")) && isDir(filepath.Join(dir, "internal", "vector", "projectcfg")) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find repository root (cmd/vector + internal/vector/projectcfg) above %s", cwd)
		}
		dir = parent
	}
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func TestResolveGoBuildRoots_SelfTest_CmdCkv(t *testing.T) {
	// Self-test: this very repo. `./cmd/vector` is a real Go main package
	// in our tree, so resolving it must yield (at minimum) cmd/vector/*.go
	// files plus the packages they import (internal/vector/build,
	// internal/vector/embed, etc.). Concrete file names we assert on are stable load-bearing
	// entries — if someone moves them, the test should catch the drift.
	root := findRepoRoot(t)
	files, err := ResolveGoBuildRoots(context.Background(), root, []string{"./cmd/vector"}, DefaultGoListOptions())
	if err != nil {
		t.Fatalf("ResolveGoBuildRoots: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("empty file set — go list returned nothing")
	}

	// Must include something from cmd/vector (the entry package itself).
	mustContain := []string{
		filepath.Join(root, "cmd", "vector", "root.go"),
		filepath.Join(root, "cmd", "vector", "build.go"),
		// Internal package reachable from cmd/vector.
		filepath.Join(root, "internal", "vector", "build", "builder.go"),
	}
	for _, want := range mustContain {
		if _, ok := files[want]; !ok {
			t.Errorf("expected file in set: %s", want)
		}
	}
}

func TestResolveGoBuildRoots_SkipsStandardLib(t *testing.T) {
	// stdlib paths live under GOROOT, outside our srcRoot. The default
	// option set SkipStandardLib=true must drop them.
	root := findRepoRoot(t)
	files, err := ResolveGoBuildRoots(context.Background(), root, []string{"./cmd/vector"}, DefaultGoListOptions())
	if err != nil {
		t.Fatalf("ResolveGoBuildRoots: %v", err)
	}
	for f := range files {
		if strings.Contains(f, "/usr/local/go/src/") || strings.Contains(f, "GOROOT") {
			t.Errorf("stdlib file leaked into set: %s", f)
		}
	}
}

func TestResolveGoBuildRoots_TestsIncludedByDefault(t *testing.T) {
	// IncludeTests=true (default) → *_test.go files appear in the set.
	root := findRepoRoot(t)
	files, err := ResolveGoBuildRoots(context.Background(), root, []string{"./internal/vector/projectcfg"}, DefaultGoListOptions())
	if err != nil {
		t.Fatalf("ResolveGoBuildRoots: %v", err)
	}
	wantTest := filepath.Join(root, "internal", "vector", "projectcfg", "config_test.go")
	if _, ok := files[wantTest]; !ok {
		t.Errorf("default options must include *_test.go (missing %s)", wantTest)
	}
}

func TestResolveGoBuildRoots_TestsExcludedWhenOptedOut(t *testing.T) {
	// IncludeTests=false → no *_test.go files at all.
	root := findRepoRoot(t)
	opts := DefaultGoListOptions()
	opts.IncludeTests = false
	files, err := ResolveGoBuildRoots(context.Background(), root, []string{"./internal/vector/projectcfg"}, opts)
	if err != nil {
		t.Fatalf("ResolveGoBuildRoots: %v", err)
	}
	for f := range files {
		if strings.HasSuffix(f, "_test.go") {
			t.Errorf("IncludeTests=false leaked test file: %s", f)
		}
	}
}

func TestResolveGoBuildRoots_EmptyEntryRejected(t *testing.T) {
	_, err := ResolveGoBuildRoots(context.Background(), ".", nil, DefaultGoListOptions())
	if err == nil {
		t.Error("expected error for empty entry list")
	}
}

func TestResolveGoBuildRoots_BadEntryReturnsError(t *testing.T) {
	// Non-existent package path → go list exits non-zero.
	root := findRepoRoot(t)
	_, err := ResolveGoBuildRoots(context.Background(), root,
		[]string{"./this/path/does/not/exist/anywhere"},
		DefaultGoListOptions())
	if err == nil {
		t.Error("expected error for invalid entry path")
	}
}
