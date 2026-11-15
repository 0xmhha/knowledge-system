package buildscope

import (
	"context"
	"encoding/json"
	"os"
	"testing"
)

// files returns "role path" lines of the scope's files.
func files(s *Scope) map[string]bool {
	out := map[string]bool{}
	for _, f := range s.Files {
		out[f.Role+" "+f.Path] = true
	}
	return out
}

// TestExtractGo_BuildTagsTestsAndOutsideModules: testdata/mod has a main
// package that imports lib (with linux and non-linux variants of one file)
// and an outside module replaced by a local directory, plus a package no
// entry point reaches.
func TestExtractGo_BuildTagsTestsAndOutsideModules(t *testing.T) {
	cases := []struct {
		goos      string
		built     string
		ignored   string
		notBuiltf string
	}{
		{"linux", "lib/lib_linux.go", "lib/lib_other.go", "lib/lib_other.go"},
		{"darwin", "lib/lib_other.go", "lib/lib_linux.go", "lib/lib_linux.go"},
	}
	for _, c := range cases {
		t.Run(c.goos, func(t *testing.T) {
			s, err := ExtractGo(context.Background(), GoConfig{
				Root: "testdata/mod", ConfigID: "app-" + c.goos, GOOS: c.goos, GOARCH: "amd64",
				EntryPoints: []string{"./cmd/app"},
			})
			if err != nil {
				t.Fatalf("ExtractGo: %v", err)
			}
			got := files(s)
			for _, want := range []string{
				"production cmd/app/main.go",
				"production lib/lib.go",
				"production " + c.built,
				"test lib/lib_test.go",
				"test lib/external_test.go",
			} {
				if !got[want] {
					t.Errorf("missing %q in %v", want, got)
				}
			}
			for _, bad := range []string{"production " + c.notBuiltf, "production unused/unused.go"} {
				if got[bad] {
					t.Errorf("unexpected %q: a file outside the build entered the scope", bad)
				}
			}
			ignored := false
			for _, f := range s.Ignored {
				ignored = ignored || f.Path == c.ignored
			}
			if !ignored {
				t.Errorf("%s not reported as ignored: %v", c.ignored, s.Ignored)
			}
			if len(s.OutsideModules) != 1 || s.OutsideModules[0].Path != "example.com/ext" || s.OutsideModules[0].Replace != "./ext" {
				t.Errorf("outside modules = %+v, want example.com/ext replaced by ./ext", s.OutsideModules)
			}
			for _, f := range s.Files {
				if f.Path == "ext/ext.go" {
					t.Errorf("outside module file listed as a main-module file")
				}
			}
			if s.Target.GOOS != c.goos || s.Target.GOARCH != "amd64" || s.Target.CGOEnabled == "" {
				t.Errorf("target = %+v", s.Target)
			}
			if len(s.Toolchains) != 1 || s.Toolchains[0].Declared != "go1.22" || s.Toolchains[0].SHA256 == "" {
				t.Errorf("toolchain = %+v, want declared go1.22 and a checksum", s.Toolchains)
			}
		})
	}
}

// TestExtractGo_Deterministic: two runs give byte-identical JSON.
func TestExtractGo_Deterministic(t *testing.T) {
	cfg := GoConfig{Root: "testdata/mod", ConfigID: "app", GOOS: "linux", GOARCH: "arm64", EntryPoints: []string{"./cmd/app"}}
	a, err := ExtractGo(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	b, err := ExtractGo(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	if string(ja) != string(jb) {
		t.Errorf("two runs differ:\n%s\n%s", ja, jb)
	}
}

func TestExtractGo_NoEntryPoints(t *testing.T) {
	if _, err := ExtractGo(context.Background(), GoConfig{Root: "testdata/mod"}); err == nil {
		t.Fatal("want an error without entry points")
	}
}

func TestDeclaredGo_ToolchainWins(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/go.mod"
	if err := writeFile(path, "module m\n\ngo 1.22\n\ntoolchain go1.23.4\n"); err != nil {
		t.Fatal(err)
	}
	got, err := declaredGo(path)
	if err != nil || got != "go1.23.4" {
		t.Errorf("declaredGo = %q, %v; want go1.23.4", got, err)
	}
}

func writeFile(path, s string) error { return os.WriteFile(path, []byte(s), 0o644) }

// TestExtractGo_CGOEnabledIsRecorded: the effective cgo setting decides files
// (cgo files are dropped when it is 0), so the scope records it.
func TestExtractGo_CGOEnabledIsRecorded(t *testing.T) {
	for _, v := range []string{"0", "1"} {
		s, err := ExtractGo(context.Background(), GoConfig{
			Root: "testdata/mod", GOOS: "linux", GOARCH: "amd64", CGOEnabled: v, EntryPoints: []string{"./cmd/app"},
		})
		if err != nil {
			t.Fatal(err)
		}
		if s.Target.CGOEnabled != v {
			t.Errorf("CGOEnabled = %q, want %q", s.Target.CGOEnabled, v)
		}
	}
}
