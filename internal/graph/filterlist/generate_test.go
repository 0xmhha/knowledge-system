package filterlist

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

// writeModule lays down a tiny throwaway Go module under root:
//
//	go.mod                         module example.com/tm
//	cmd/app/main.go                package main, imports internal/sub + fmt
//	internal/sub/sub.go            package sub (in-module dep of main)
//	internal/unused/unused.go      package unused (NOT imported anywhere)
//
// The unused package proves GenerateFromMain follows the import closure and
// does not just glob every dir; fmt proves stdlib is dropped.
func writeModule(t *testing.T, root string) {
	t.Helper()
	files := map[string]string{
		"go.mod": "module example.com/tm\n\ngo 1.25\n",
		"cmd/app/main.go": `package main

import (
	"fmt"

	"example.com/tm/internal/sub"
)

func main() { fmt.Println(sub.Hello()) }
`,
		"internal/sub/sub.go": `package sub

func Hello() string { return "hi" }
`,
		"internal/unused/unused.go": `package unused

func Unused() string { return "nope" }
`,
	}
	for rel, body := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func requireGo(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not on PATH")
	}
}

func TestGenerateFromMain_ClosureGlobs(t *testing.T) {
	requireGo(t)
	root := t.TempDir()
	writeModule(t, root)

	sol := []string{"contracts/**/*.sol"}
	exc := []string{"**/*_test.go"}
	fl, err := GenerateFromMain(context.Background(), root, []string{"./cmd/app"}, sol, exc)
	if err != nil {
		t.Fatalf("GenerateFromMain: %v", err)
	}

	// Include must cover the main pkg dir and its in-module dep dir as
	// <dir>/*.go, plus the Solidity glob appended verbatim.
	want := []string{
		"cmd/app/*.go",
		"contracts/**/*.sol",
		"internal/sub/*.go",
	}
	if !reflect.DeepEqual(fl.Include, want) {
		t.Errorf("Include = %v, want %v", fl.Include, want)
	}

	// Determinism: the result is already sorted.
	if !sort.StringsAreSorted(fl.Include) {
		t.Errorf("Include not sorted: %v", fl.Include)
	}

	// The unrelated (unimported) package must not appear.
	for _, g := range fl.Include {
		if g == "internal/unused/*.go" {
			t.Errorf("unimported package leaked into Include: %v", fl.Include)
		}
	}

	// stdlib deps live outside the module and must be dropped — no glob
	// should reference a stdlib path.
	for _, g := range fl.Include {
		if filepath.IsAbs(g) {
			t.Errorf("absolute (out-of-module) glob leaked: %q", g)
		}
	}

	if !reflect.DeepEqual(fl.Exclude, exc) {
		t.Errorf("Exclude = %v, want %v", fl.Exclude, exc)
	}
}

// TestGenerateFromMain_RelativeModuleRoot is the regression test for the
// relative --src case (`ckg build --src=.. --files-from-main ...`): a
// relative moduleRoot used to stay relative through resolveDir, breaking
// every filepath.Rel against go list's absolute Dir values and yielding
// "no in-module packages".
func TestGenerateFromMain_RelativeModuleRoot(t *testing.T) {
	requireGo(t)
	root := t.TempDir()
	writeModule(t, root)

	// Run from a subdirectory of the module and pass ".." as the root,
	// mirroring the graph/Makefile eval invocation.
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(filepath.Join(root, "cmd")); err != nil {
		t.Fatal(err)
	}

	fl, err := GenerateFromMain(context.Background(), "..", []string{"./cmd/app"}, nil, nil)
	if err != nil {
		t.Fatalf("GenerateFromMain with relative root: %v", err)
	}
	want := []string{"cmd/app/*.go", "internal/sub/*.go"}
	if !reflect.DeepEqual(fl.Include, want) {
		t.Errorf("Include = %v, want %v", fl.Include, want)
	}
}

// TestGenerateFromMain_RootPackage covers the module-root special case: a main
// package that lives at the module root maps to the bare "*.go" glob.
func TestGenerateFromMain_RootPackage(t *testing.T) {
	requireGo(t)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"),
		[]byte("module example.com/rootmod\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"),
		[]byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	fl, err := GenerateFromMain(context.Background(), root, []string{"."}, nil, nil)
	if err != nil {
		t.Fatalf("GenerateFromMain: %v", err)
	}
	if !reflect.DeepEqual(fl.Include, []string{"*.go"}) {
		t.Errorf("Include = %v, want [*.go]", fl.Include)
	}
	if len(fl.Exclude) != 0 {
		t.Errorf("Exclude = %v, want empty", fl.Exclude)
	}
}

func TestGenerateFromMain_NoMainPkgs(t *testing.T) {
	if _, err := GenerateFromMain(context.Background(), t.TempDir(), nil, nil, nil); err == nil {
		t.Error("expected error when no main packages are given")
	}
}

func TestGenerateFromMain_GoListFails(t *testing.T) {
	requireGo(t)
	root := t.TempDir()
	writeModule(t, root)
	// A package path that does not exist makes `go list` exit non-zero.
	if _, err := GenerateFromMain(context.Background(), root, []string{"./does/not/exist"}, nil, nil); err == nil {
		t.Error("expected error when go list fails")
	}
}
