package buildscope

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func writeProfile(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "filelist.yaml")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

const testProfile = `build_context:
  goos: linux
  goarch: amd64
  cgo: false
  tags: []
build_roots:
  - ./cmd/app
include_package_tests: false
include_embed_files: false
extra_packages:
  - ./unused
extra_globs:
  - "contracts/**/*.sol"
exclude_globs:
  - "contracts/mocks/**"
`

// TestExtractProject_Profile: the profile's options decide the Go files, its
// extra packages and globs add files outside the Go build, and its excludes
// remove files.
func TestExtractProject_Profile(t *testing.T) {
	prof := writeProfile(t, testProfile)
	s, err := ExtractProject(context.Background(), ProjectConfig{Root: "testdata/mod", ConfigID: "app", ProfilePath: prof})
	if err != nil {
		t.Fatal(err)
	}
	got := files(s)
	for _, want := range []string{
		"production cmd/app/main.go", "production lib/lib_linux.go",
		"production unused/unused.go", "production contracts/Token.sol",
	} {
		if !got[want] {
			t.Errorf("missing %q in %v", want, got)
		}
	}
	for _, bad := range []string{
		"production lib/lib_other.go", "production lib/lib_tag.go", "test lib/lib_test.go",
		"production contracts/mocks/Mock.sol",
	} {
		if got[bad] {
			t.Errorf("unexpected %q", bad)
		}
	}
	for _, f := range s.Files {
		if f.Path == "contracts/Token.sol" && f.Language != "sol" {
			t.Errorf("Token.sol language = %q", f.Language)
		}
	}
	if s.Target.GOOS != "linux" || s.Target.CGOEnabled != "0" {
		t.Errorf("target = %+v", s.Target)
	}
	for _, f := range s.Files {
		if f.Path == "unused/unused.go" && f.IncludedBy != "extra_packages" {
			t.Errorf("legacy extra_packages file marked %q", f.IncludedBy)
		}
	}
	if s.Profile == nil || s.Profile.SHA256 == "" || s.SchemaVersion != 2 {
		t.Errorf("profile = %+v, schema %d", s.Profile, s.SchemaVersion)
	}
}

// TestExtractProject_OptionsOverrideProfile: the options of the build at hand
// win over the profile, so one profile serves builds with other options.
func TestExtractProject_OptionsOverrideProfile(t *testing.T) {
	prof := writeProfile(t, testProfile)
	s, err := ExtractProject(context.Background(), ProjectConfig{
		Root: "testdata/mod", ProfilePath: prof,
		Options: Options{GOOS: "darwin", Tags: []string{"bstag"}, TagsSet: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := files(s)
	if !got["production lib/lib_other.go"] || !got["production lib/lib_tag.go"] || got["production lib/lib_linux.go"] {
		t.Errorf("darwin + bstag files: %v", got)
	}
	if s.Target.GOOS != "darwin" || strings.Join(s.Target.Tags, ",") != "bstag" {
		t.Errorf("target = %+v", s.Target)
	}
}

// TestExtractProject_LoadErrorsFail: a package the go command cannot load
// makes the scope incomplete, so extraction fails unless told otherwise.
func TestExtractProject_LoadErrorsFail(t *testing.T) {
	prof := writeProfile(t, strings.Replace(testProfile, "./unused", "./missing", 1))
	cfg := ProjectConfig{Root: "testdata/mod", ProfilePath: prof}
	if _, err := ExtractProject(context.Background(), cfg); err == nil || !strings.Contains(err.Error(), "load errors") {
		t.Fatalf("want a load error, got %v", err)
	}
	cfg.AllowProblems = true
	s, err := ExtractProject(context.Background(), cfg)
	if err != nil || len(s.Problems) == 0 {
		t.Fatalf("allowed problems: %v, %+v", err, s)
	}
}

func TestLoadProfile_UnknownKeyFails(t *testing.T) {
	if _, _, err := LoadProfile(writeProfile(t, "build_roots: [./cmd/app]\nbuild_tags: [x]\n")); err == nil {
		t.Fatal("a misspelt key must fail")
	}
}

// TestCheckBinary: a binary built with the scope's options passes; one built
// with other tags, or for another main package, fails.
func TestCheckBinary(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "app")
	build := exec.Command("go", "build", "-tags", "bstag", "-o", bin, "./cmd/app")
	build.Dir = "testdata/mod"
	build.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	extract := func(tags []string) *Scope {
		s, err := ExtractProject(context.Background(), ProjectConfig{
			Root: "testdata/mod", Options: Options{EntryPoints: []string{"./cmd/app"}, Tags: tags, TagsSet: true},
		})
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	s := extract([]string{"bstag"})
	cb, err := CheckBinary(context.Background(), "", bin, s)
	if err != nil || cb.Package != "example.com/bs/cmd/app" || cb.SHA256 == "" {
		t.Fatalf("matching binary: %+v, %v", cb, err)
	}
	if _, err := CheckBinary(context.Background(), "", bin, extract(nil)); err == nil || !strings.Contains(err.Error(), "tags") {
		t.Errorf("tag mismatch not reported: %v", err)
	}
	s.EntryPoints = []string{"./lib"}
	if _, err := CheckBinary(context.Background(), "", bin, s); err == nil || !strings.Contains(err.Error(), "main package") {
		t.Errorf("main package mismatch not reported: %v", err)
	}
}

func TestMatchGlob(t *testing.T) {
	for _, c := range []struct {
		glob, path string
		want       bool
	}{
		{"contracts/**/*.sol", "contracts/Token.sol", true},
		{"contracts/**/*.sol", "contracts/a/b/Token.sol", true},
		{"contracts/**/*.sol", "other/Token.sol", false},
		{"**/testdata/**", "a/testdata/x.go", true},
		{"*.sol", "a/Token.sol", false},
	} {
		if got := matchAny([]string{c.glob}, c.path); got != c.want {
			t.Errorf("%s ~ %s = %v, want %v", c.glob, c.path, got, c.want)
		}
	}
}

const inclusionProfile = `build_context: {goos: linux, goarch: amd64, cgo: false}
build_roots: [./cmd/app]
include_package_tests: false
inclusions:
  - id: contracts
    reason: the build embeds the compiled Token contract
    files: ["contracts/**/*.sol"]
    exclude: ["contracts/mocks/**"]
    linked_by:
      embed: ["lib/artifacts/*.bin"]
  - id: tools
    reason: a package no entry point imports
    go_packages: [./unused]
`

// TestExtractProject_Inclusions: each inclusion adds its files, marked with
// its id, minus its own excludes, and the scope records why and how it is
// tied to the build.
func TestExtractProject_Inclusions(t *testing.T) {
	s, err := ExtractProject(context.Background(), ProjectConfig{Root: "testdata/mod", ProfilePath: writeProfile(t, inclusionProfile)})
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]string{}
	for _, f := range s.Files {
		by[f.Path] = f.IncludedBy
	}
	if by["contracts/Token.sol"] != "contracts" || by["unused/unused.go"] != "tools" {
		t.Errorf("included_by = %v", by)
	}
	if v, ok := by["lib/lib.go"]; !ok || v != "" {
		t.Errorf("a build file must carry no inclusion: %q %v", v, ok)
	}
	if _, ok := by["contracts/mocks/Mock.sol"]; ok {
		t.Error("the inclusion's exclude was not applied")
	}
	if len(s.EmbedFiles) != 1 || s.EmbedFiles[0] != "lib/artifacts/Token.bin" {
		t.Errorf("embed files = %v", s.EmbedFiles)
	}
	want := []InclusionRecord{
		{ID: "contracts", Reason: "the build embeds the compiled Token contract", Files: 1, LinkedBy: []string{"embed:lib/artifacts/*.bin"}},
		{ID: "tools", Reason: "a package no entry point imports", Files: 1},
	}
	if fmt.Sprint(s.Inclusions) != fmt.Sprint(want) {
		t.Errorf("inclusions = %+v, want %+v", s.Inclusions, want)
	}
}

// TestExtractProject_InclusionChecks: an inclusion whose link to the build is
// gone, that matches nothing, or that only repeats the build fails the
// extraction, so an exception cannot outlive its reason unnoticed.
func TestExtractProject_InclusionChecks(t *testing.T) {
	for name, c := range map[string]struct{ from, to, want string }{
		"link gone":     {`["lib/artifacts/*.bin"]`, `["lib/artifacts/*.abi"]`, "embeds no such file"},
		"no match":      {`["contracts/**/*.sol"]`, `["contracts/**/*.vy"]`, "matches no file"},
		"already built": {`[./unused]`, `[./lib]`, "adds no file"},
	} {
		t.Run(name, func(t *testing.T) {
			prof := writeProfile(t, strings.Replace(inclusionProfile, c.from, c.to, 1))
			_, err := ExtractProject(context.Background(), ProjectConfig{Root: "testdata/mod", ProfilePath: prof})
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("want %q, got %v", c.want, err)
			}
		})
	}
}

// TestLoadProfile_InclusionRules: every inclusion needs an id, a reason and
// something to include, and ids are unique.
func TestLoadProfile_InclusionRules(t *testing.T) {
	for name, body := range map[string]string{
		"no reason":  "inclusions: [{id: a, files: [x]}]\n",
		"bad id":     "inclusions: [{id: A_1, reason: r, files: [x]}]\n",
		"duplicate":  "inclusions: [{id: a, reason: r, files: [x]}, {id: a, reason: r, files: [y]}]\n",
		"empty":      "inclusions: [{id: a, reason: r}]\n",
		"empty link": "inclusions: [{id: a, reason: r, files: [x], linked_by: {embed: []}}]\n",
		"bad glob":   "inclusions: [{id: a, reason: r, files: ['[']}]\n",
	} {
		if _, _, err := LoadProfile(writeProfile(t, body)); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}

// TestStablenetProfileLoads: the committed stablenet profile follows the
// profile rules, so a broken edit fails here rather than at the next setup.
func TestStablenetProfileLoads(t *testing.T) {
	prof, _, err := LoadProfile("../../projects/stablenet/filelist.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if len(prof.Inclusions) == 0 || len(prof.ExtraGlobs)+len(prof.ExtraPackages) != 0 {
		t.Errorf("stablenet profile should list its exceptions as inclusions: %+v", prof)
	}
}
