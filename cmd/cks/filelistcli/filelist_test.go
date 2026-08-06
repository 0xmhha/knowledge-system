package filelistcli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// fixtureRepo copies testdata/fixturemod into a temp dir and turns it into a
// committed git repository — the tool requires git-tracked state (R10). The
// fixture imports stdlib only, so `go list` needs no network (design §7).
func fixtureRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("`go` not in PATH")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("`git` not in PATH")
	}
	src, err := filepath.Abs("testdata/fixturemod")
	if err != nil {
		t.Fatal(err)
	}
	dst := t.TempDir()
	if out, err := exec.Command("cp", "-R", src+"/.", dst).CombinedOutput(); err != nil {
		t.Fatalf("copy fixture: %v (%s)", err, out)
	}
	git(t, dst, "init", "-q")
	git(t, dst, "add", "-A")
	git(t, dst, "-c", "user.email=fixture@test", "-c", "user.name=fixture", "commit", "-q", "-m", "fixture")
	return dst
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v (%s)", args, err, out)
	}
}

func writeConfig(t *testing.T, dir, goos string, embed bool) string {
	t.Helper()
	cfg := `build_context:
  goos: ` + goos + `
  goarch: amd64
  cgo: false
  tags: []
build_roots:
  - ./cmd/app
include_package_tests: true
include_embed_files: ` + boolStr(embed) + `
extra_packages:
  - ./integtest
  - ./tool/...
extra_globs:
  - "assets/**/*.sol"
exclude_globs: []
`
	p := filepath.Join(dir, "filelist.yaml")
	if err := os.WriteFile(p, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func derive(t *testing.T, repo, cfgPath string, allowDirty bool) Output {
	t.Helper()
	out := filepath.Join(t.TempDir(), "files-from.json")
	if err := run(repo, cfgPath, out, false, allowDirty, false); err != nil {
		t.Fatalf("run: %v", err)
	}
	buf, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var o Output
	if err := json.Unmarshal(buf, &o); err != nil {
		t.Fatal(err)
	}
	return o
}

func has(o Output, f string) bool {
	for _, x := range o.Include {
		if x == f {
			return true
		}
	}
	return false
}

// TestDerive_BuildTestsExtrasGlobs locks the full composition: build closure,
// closure tests, out-of-graph extra packages (incl. /... expansion), and
// tracked-glob assets, under the pinned linux context.
func TestDerive_BuildTestsExtrasGlobs(t *testing.T) {
	repo := fixtureRepo(t)
	o := derive(t, repo, writeConfig(t, t.TempDir(), "linux", false), false)

	for _, want := range []string{
		"cmd/app/main.go",      // build root
		"lib/lib.go",           // closure
		"lib/only_linux.go",    // present under GOOS=linux (R9)
		"lib/lib_test.go",      // TestGoFiles (R2)
		"lib/answer_x_test.go", // XTestGoFiles (R2)
		"integtest/doc.go",     // extra package (R3)
		"integtest/integ_test.go",
		"tool/tool.go", // extra package /... expansion
		"tool/sub/sub.go",
		"assets/contract.sol", // extra glob (R3)
	} {
		if !has(o, want) {
			t.Errorf("missing %s in %v", want, o.Include)
		}
	}
	if has(o, "cmd/app/asset.txt") {
		t.Error("embed file included despite include_embed_files: false")
	}
	p := o.Provenance
	if p.SrcCommit == "" || p.ConfigSHA256 == "" || p.BuildContext.GOOS != "linux" {
		t.Errorf("provenance incomplete: %+v", p)
	}
	if p.Dirty {
		t.Error("clean derivation must not record dirty")
	}
	if p.Counts.Tests == 0 || p.Counts.ExtraPackages == 0 || p.Counts.ExtraGlobs != 1 {
		t.Errorf("counts wrong: %+v", p.Counts)
	}
	if p.Roots["./cmd/app"] == 0 {
		t.Errorf("root contribution missing: %+v", p.Roots)
	}
}

// TestBuildContext locks R9: the pinned GOOS decides constraint-guarded
// files, regardless of the invoking machine.
func TestBuildContext(t *testing.T) {
	repo := fixtureRepo(t)
	darwin := derive(t, repo, writeConfig(t, t.TempDir(), "darwin", false), false)
	if has(darwin, "lib/only_linux.go") {
		t.Error("GOOS=darwin derivation must exclude only_linux.go")
	}
}

// TestEmbedSwitch locks the include_embed_files behavior (design §4.2).
func TestEmbedSwitch(t *testing.T) {
	repo := fixtureRepo(t)
	o := derive(t, repo, writeConfig(t, t.TempDir(), "linux", true), false)
	if !has(o, "cmd/app/asset.txt") {
		t.Error("include_embed_files: true must include the embedded asset")
	}
}

// TestSourceState locks R10: dirty tracked tree refused by default,
// -allow-dirty records the exception, untracked files never enter via globs.
func TestSourceState(t *testing.T) {
	repo := fixtureRepo(t)
	cfg := writeConfig(t, t.TempDir(), "linux", false)

	// Untracked .sol must NOT match (git-tracked resolution).
	if err := os.WriteFile(filepath.Join(repo, "assets", "untracked.sol"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	o := derive(t, repo, cfg, false)
	if has(o, "assets/untracked.sol") {
		t.Error("untracked file entered the scope through a glob")
	}

	// Dirty tracked file → refused; -allow-dirty → recorded.
	if err := os.WriteFile(filepath.Join(repo, "lib", "lib.go"), []byte("package lib\n\nfunc Answer() int { return 43 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "f.json")
	if err := run(repo, cfg, out, false, false, false); err == nil || !strings.Contains(err.Error(), "dirty") {
		t.Fatalf("dirty tree not refused: %v", err)
	}
	o2 := derive(t, repo, cfg, true)
	if !o2.Provenance.Dirty {
		t.Error("-allow-dirty derivation must record dirty: true")
	}
}

// TestFailClosed locks R7: unresolvable inputs are errors, never omissions.
func TestFailClosed(t *testing.T) {
	repo := fixtureRepo(t)
	dir := t.TempDir()

	bad := filepath.Join(dir, "bad-root.yaml")
	os.WriteFile(bad, []byte("build_context: {goos: linux, goarch: amd64}\nbuild_roots: [./cmd/nonexistent]\n"), 0o644)
	if err := run(repo, bad, filepath.Join(dir, "o.json"), false, false, false); err == nil {
		t.Error("unknown build root must fail")
	}

	badPkg := filepath.Join(dir, "bad-pkg.yaml")
	os.WriteFile(badPkg, []byte("build_context: {goos: linux, goarch: amd64}\nbuild_roots: [./cmd/app]\nextra_packages: [./no/such/pkg]\n"), 0o644)
	if err := run(repo, badPkg, filepath.Join(dir, "o.json"), false, false, false); err == nil {
		t.Error("unknown extra package must fail")
	}

	noCtx := filepath.Join(dir, "no-ctx.yaml")
	os.WriteFile(noCtx, []byte("build_roots: [./cmd/app]\n"), 0o644)
	if err := run(repo, noCtx, filepath.Join(dir, "o.json"), false, false, false); err == nil || !strings.Contains(err.Error(), "build_context") {
		t.Errorf("missing build_context must fail: %v", err)
	}

	strictGlob := filepath.Join(dir, "strict.yaml")
	os.WriteFile(strictGlob, []byte("build_context: {goos: linux, goarch: amd64}\nbuild_roots: [./cmd/app]\nextra_globs: [\"nomatch/**/*.xyz\"]\n"), 0o644)
	if err := run(repo, strictGlob, filepath.Join(dir, "o.json"), false, false, true); err == nil {
		t.Error("zero-match glob with -strict must fail")
	}
	if err := run(repo, strictGlob, filepath.Join(dir, "o.json"), false, false, false); err != nil {
		t.Errorf("zero-match glob without -strict must warn, not fail: %v", err)
	}

	if err := run(t.TempDir(), filepath.Join(dir, "strict.yaml"), filepath.Join(dir, "o.json"), false, false, false); err == nil {
		t.Error("non-module, non-git -src must fail")
	}
}

// TestCheck locks the -check semantics: OK on identity, drift on list
// change, config error on cross-context comparison.
func TestCheck(t *testing.T) {
	repo := fixtureRepo(t)
	cfg := writeConfig(t, t.TempDir(), "linux", false)
	out := filepath.Join(t.TempDir(), "files-from.json")
	if err := run(repo, cfg, out, false, false, false); err != nil {
		t.Fatal(err)
	}
	if err := run(repo, cfg, out, true, false, false); err != nil {
		t.Errorf("check against fresh output must pass: %v", err)
	}

	// Mutate the list → drift.
	var o Output
	buf, _ := os.ReadFile(out)
	json.Unmarshal(buf, &o)
	o.Include = append(o.Include, "zzz/injected.go")
	mut, _ := json.Marshal(o)
	os.WriteFile(out, mut, 0o644)
	if err := run(repo, cfg, out, true, false, false); err == nil || !strings.Contains(err.Error(), "drift") {
		t.Errorf("mutated list must be drift: %v", err)
	}

	// Cross-context comparison → config error, not drift.
	json.Unmarshal(buf, &o)
	o.Provenance.BuildContext.GOOS = "windows"
	mut, _ = json.Marshal(o)
	os.WriteFile(out, mut, 0o644)
	if err := run(repo, cfg, out, true, false, false); err == nil || !strings.Contains(err.Error(), "build_context") {
		t.Errorf("cross-context check must be a config error: %v", err)
	}
}

// TestGlobToRegexp pins the documented doublestar semantics.
func TestGlobToRegexp(t *testing.T) {
	cases := []struct {
		glob, path string
		want       bool
	}{
		{"systemcontracts/**/*.sol", "systemcontracts/solidity/v1/GovValidator.sol", true},
		{"systemcontracts/**/*.sol", "systemcontracts/top.sol", true}, // **/ matches empty
		{"systemcontracts/**/*.sol", "other/solidity/x.sol", false},
		{"*.go", "main.go", true},
		{"*.go", "cmd/main.go", false}, // * stays within a segment
		{"cmd/?pp/*.go", "cmd/app/main.go", true},
	}
	for _, c := range cases {
		re, err := globToRegexp(c.glob)
		if err != nil {
			t.Fatalf("%s: %v", c.glob, err)
		}
		if got := re.MatchString(c.path); got != c.want {
			t.Errorf("glob %q vs %q = %v, want %v", c.glob, c.path, got, c.want)
		}
	}
}
