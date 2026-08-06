// Command filelist-gen derives the build-scope file list a knowledge dataset
// indexes: the files of the configured build roots' dependency closure, the
// tests of those packages, explicitly configured out-of-graph packages
// (test-only integration packages, tooling), and non-Go assets matched by
// globs. The output is the {include: [...]} shape the engine builds accept
// via --files-from, plus a _provenance block recording exactly how the scope
// was computed.
//
// Design: docs/design/filelist-gen.md. Key properties:
//   - the derivation runs `go list` under a PINNED build context from the
//     config (GOOS/GOARCH/CGO/tags) — never the invoking machine's;
//   - every file is resolved against the git-tracked tree, and a dirty
//     tracked tree is refused by default (-allow-dirty records the
//     exception) — the emitted list always corresponds to src_commit;
//   - unresolvable roots/packages fail closed.
//
// The tool imports no engine packages; `go list` and `git` subprocesses are
// the contract.
package filelistcli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/spf13/cobra"
)

const toolVersion = "filelist-gen 0.1.0"

// Config mirrors projects/<pack>/filelist.yaml (design §4.2).
type Config struct {
	BuildContext struct {
		GOOS   string   `yaml:"goos"`
		GOARCH string   `yaml:"goarch"`
		CGO    bool     `yaml:"cgo"`
		Tags   []string `yaml:"tags"`
	} `yaml:"build_context"`
	BuildRoots          []string `yaml:"build_roots"`
	IncludePackageTests bool     `yaml:"include_package_tests"`
	IncludeEmbedFiles   bool     `yaml:"include_embed_files"`
	ExtraPackages       []string `yaml:"extra_packages"`
	ExtraGlobs          []string `yaml:"extra_globs"`
	ExcludeGlobs        []string `yaml:"exclude_globs"`
}

// Provenance records how the scope was computed (design §4.3).
type Provenance struct {
	Tool         string         `json:"tool"`
	SrcCommit    string         `json:"src_commit"`
	Dirty        bool           `json:"dirty,omitempty"`
	BuildContext BuildContextPr `json:"build_context"`
	ConfigSHA256 string         `json:"config_sha256"`
	Roots        map[string]int `json:"roots"`
	Counts       Counts         `json:"counts"`
}

type BuildContextPr struct {
	GOOS   string   `json:"goos"`
	GOARCH string   `json:"goarch"`
	CGO    bool     `json:"cgo"`
	Tags   []string `json:"tags"`
}

type Counts struct {
	Build         int `json:"build"`
	Tests         int `json:"tests"`
	ExtraPackages int `json:"extra_packages"`
	ExtraGlobs    int `json:"extra_globs"`
}

// Output is the consumer file: engines read include; everything else is
// self-description they ignore.
type Output struct {
	Provenance Provenance `json:"_provenance"`
	Include    []string   `json:"include"`
}

// NewCmd builds the `cks filelist` command: derive the build-scope file
// list from a pack's filelist.yaml.
func NewCmd() *cobra.Command {
	var src, configPath, out string
	var check, allowDirty, strict bool
	cmd := &cobra.Command{
		Use:   "filelist",
		Short: "Derive the build-scope file list from a filelist.yaml",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if src == "" || configPath == "" || out == "" {
				return fmt.Errorf("--src, --config, and --out are required")
			}
			return run(src, configPath, out, check, allowDirty, strict)
		},
	}
	cmd.Flags().StringVar(&src, "src", "", "project root (single Go module, git repository) (required)")
	cmd.Flags().StringVar(&configPath, "config", "", "path to filelist.yaml (required)")
	cmd.Flags().StringVar(&out, "out", "", "output files-from.json path (required)")
	cmd.Flags().BoolVar(&check, "check", false, "compare a fresh derivation against the existing --out; fail on drift")
	cmd.Flags().BoolVar(&allowDirty, "allow-dirty", false, "permit a dirty tracked tree (recorded in provenance)")
	cmd.Flags().BoolVar(&strict, "strict", false, "zero-match extra_globs become errors")
	return cmd
}

func run(src, configPath, outPath string, check, allowDirty, strict bool) error {
	srcAbs, err := filepath.Abs(src)
	if err != nil {
		return err
	}
	// go list reports symlink-resolved Dirs (e.g. /var -> /private/var on
	// macOS); resolve the root the same way so prefix checks agree.
	if resolved, err := filepath.EvalSymlinks(srcAbs); err == nil {
		srcAbs = resolved
	}
	// Assumptions, fail closed (design §4.1): git repo + single-module root.
	if _, err := os.Stat(filepath.Join(srcAbs, "go.mod")); err != nil {
		return fmt.Errorf("--src %s is not a Go module root (no go.mod)", srcAbs)
	}
	if _, err := gitOut(srcAbs, "rev-parse", "--git-dir"); err != nil {
		return fmt.Errorf("--src %s is not a git repository: %v", srcAbs, err)
	}

	cfgBytes, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}
	var cfg Config
	if err := yaml.Unmarshal(cfgBytes, &cfg); err != nil {
		return fmt.Errorf("%s: %w", configPath, err)
	}
	if cfg.BuildContext.GOOS == "" || cfg.BuildContext.GOARCH == "" {
		return fmt.Errorf("%s: build_context.goos and build_context.goarch are required (design R9 — the derivation must not inherit the invoking machine's context)", configPath)
	}
	if len(cfg.BuildRoots) == 0 {
		return fmt.Errorf("%s: build_roots must not be empty", configPath)
	}

	// Source-state discipline (design R10).
	dirty, err := dirtyTracked(srcAbs)
	if err != nil {
		return err
	}
	if dirty && !allowDirty {
		return fmt.Errorf("tracked working tree is dirty — the derived list would not correspond to the recorded commit; commit/stash first or pass -allow-dirty (recorded in provenance)")
	}
	commit, err := gitOut(srcAbs, "rev-parse", "HEAD")
	if err != nil {
		return err
	}

	env := buildEnv(cfg)

	include := map[string]struct{}{}
	counts := Counts{}
	rootContrib := map[string]int{}

	// Build roots: one `go list -deps` per root, union with ordered
	// per-root contribution deltas (design §4.3).
	for _, root := range cfg.BuildRoots {
		pkgs, err := goList(srcAbs, env, cfg.tagsArgs(), true, root)
		if err != nil {
			return fmt.Errorf("build root %s: %w", root, err)
		}
		before := len(include)
		b, t := collect(srcAbs, pkgs, cfg, include)
		counts.Build += b
		counts.Tests += t
		rootContrib[root] = len(include) - before
	}
	if counts.Build == 0 {
		return fmt.Errorf("build roots resolved zero in-module build files — refusing to emit an empty scope (R7)")
	}

	// Extra packages (outside the dep graph): resolved via go list so they
	// fail closed and stay glob-free.
	for _, pat := range cfg.ExtraPackages {
		pkgs, err := goList(srcAbs, env, cfg.tagsArgs(), false, pat)
		if err != nil {
			return fmt.Errorf("extra package %s: %w", pat, err)
		}
		if len(pkgs) == 0 {
			return fmt.Errorf("extra package %s matched no packages", pat)
		}
		before := len(include)
		collect(srcAbs, pkgs, cfg, include)
		counts.ExtraPackages += len(include) - before
	}

	// Extra globs: matched against git-tracked files only (design R10).
	if len(cfg.ExtraGlobs) > 0 {
		tracked, err := gitLines(srcAbs, "ls-files")
		if err != nil {
			return err
		}
		for _, g := range cfg.ExtraGlobs {
			re, err := globToRegexp(g)
			if err != nil {
				return fmt.Errorf("extra glob %q: %w", g, err)
			}
			n := 0
			for _, f := range tracked {
				if re.MatchString(f) {
					if _, ok := include[f]; !ok {
						include[f] = struct{}{}
						counts.ExtraGlobs++
					}
					n++
				}
			}
			if n == 0 {
				msg := fmt.Sprintf("extra glob %q matched no tracked files", g)
				if strict {
					return fmt.Errorf("%s (-strict)", msg)
				}
				fmt.Fprintln(os.Stderr, "filelist-gen: warning:", msg)
			}
		}
	}

	// Excludes last.
	for _, g := range cfg.ExcludeGlobs {
		re, err := globToRegexp(g)
		if err != nil {
			return fmt.Errorf("exclude glob %q: %w", g, err)
		}
		for f := range include {
			if re.MatchString(f) {
				delete(include, f)
			}
		}
	}

	list := make([]string, 0, len(include))
	for f := range include {
		list = append(list, f)
	}
	sort.Strings(list)

	sum := sha256.Sum256(cfgBytes)
	outObj := Output{
		Provenance: Provenance{
			Tool:      toolVersion,
			SrcCommit: commit,
			Dirty:     dirty,
			BuildContext: BuildContextPr{
				GOOS: cfg.BuildContext.GOOS, GOARCH: cfg.BuildContext.GOARCH,
				CGO: cfg.BuildContext.CGO, Tags: append([]string{}, cfg.BuildContext.Tags...),
			},
			ConfigSHA256: hex.EncodeToString(sum[:]),
			Roots:        rootContrib,
			Counts:       counts,
		},
		Include: list,
	}

	if check {
		return checkAgainst(outPath, outObj)
	}
	return writeAtomic(outPath, outObj)
}

// tagsArgs returns the -tags argument slice for go list, empty when unset.
func (c Config) tagsArgs() []string {
	if len(c.BuildContext.Tags) == 0 {
		return nil
	}
	return []string{"-tags", strings.Join(c.BuildContext.Tags, ",")}
}

// buildEnv is the pinned build context for every go list subprocess
// (design R9): explicit, never inherited.
func buildEnv(cfg Config) []string {
	cgo := "0"
	if cfg.BuildContext.CGO {
		cgo = "1"
	}
	return append(os.Environ(),
		"GOOS="+cfg.BuildContext.GOOS,
		"GOARCH="+cfg.BuildContext.GOARCH,
		"CGO_ENABLED="+cgo,
	)
}

// pkg is the subset of `go list -json` output the derivation reads.
type pkg struct {
	Dir          string
	Standard     bool
	GoFiles      []string
	CgoFiles     []string
	TestGoFiles  []string
	XTestGoFiles []string
	EmbedFiles   []string
}

// goList runs `go list [-deps] -json patterns...` under the pinned context
// and returns the in-module packages.
func goList(src string, env, tagArgs []string, deps bool, patterns ...string) ([]pkg, error) {
	args := []string{"list"}
	if deps {
		args = append(args, "-deps")
	}
	args = append(args, tagArgs...)
	args = append(args, "-json")
	args = append(args, patterns...)
	cmd := exec.Command("go", args...)
	cmd.Dir = src
	cmd.Env = env
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("go %s: %v (%s)", strings.Join(args, " "), err, firstLine(stderr.String()))
	}
	var pkgs []pkg
	dec := json.NewDecoder(&stdout)
	for dec.More() {
		var p pkg
		if err := dec.Decode(&p); err != nil {
			return nil, fmt.Errorf("parse go list output: %w", err)
		}
		if p.Standard || (p.Dir != src && !strings.HasPrefix(p.Dir, src+string(filepath.Separator))) {
			continue
		}
		pkgs = append(pkgs, p)
	}
	return pkgs, nil
}

// collect adds a package set's files to include, returning (build, test)
// counts of newly added files.
func collect(src string, pkgs []pkg, cfg Config, include map[string]struct{}) (build, tests int) {
	add := func(dir, f string, counter *int) {
		rel, err := filepath.Rel(src, filepath.Join(dir, f))
		if err != nil {
			return
		}
		rel = filepath.ToSlash(rel)
		if _, ok := include[rel]; !ok {
			include[rel] = struct{}{}
			*counter++
		}
	}
	for _, p := range pkgs {
		for _, f := range p.GoFiles {
			add(p.Dir, f, &build)
		}
		for _, f := range p.CgoFiles {
			add(p.Dir, f, &build)
		}
		if cfg.IncludePackageTests {
			for _, f := range p.TestGoFiles {
				add(p.Dir, f, &tests)
			}
			for _, f := range p.XTestGoFiles {
				add(p.Dir, f, &tests)
			}
		}
		if cfg.IncludeEmbedFiles {
			for _, f := range p.EmbedFiles {
				add(p.Dir, f, &build)
			}
		}
	}
	return build, tests
}

// globToRegexp converts a doublestar-style glob to a regexp:
// `**/` matches any (possibly empty) directory prefix, `*` matches within a
// path segment, `?` matches one non-separator character. Matching is
// against slash-separated repo-relative paths, anchored at both ends.
func globToRegexp(glob string) (*regexp.Regexp, error) {
	var b strings.Builder
	b.WriteString("^")
	i := 0
	for i < len(glob) {
		switch {
		case strings.HasPrefix(glob[i:], "**/"):
			b.WriteString(`(?:[^/]+/)*`)
			i += 3
		case strings.HasPrefix(glob[i:], "**"):
			b.WriteString(`.*`)
			i += 2
		case glob[i] == '*':
			b.WriteString(`[^/]*`)
			i++
		case glob[i] == '?':
			b.WriteString(`[^/]`)
			i++
		default:
			b.WriteString(regexp.QuoteMeta(string(glob[i])))
			i++
		}
	}
	b.WriteString("$")
	return regexp.Compile(b.String())
}

// dirtyTracked reports whether any TRACKED file is modified/staged/deleted.
// Untracked files are ignorable noise (they can never enter the list — R10
// resolves against git-tracked state).
func dirtyTracked(src string) (bool, error) {
	lines, err := gitLines(src, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	for _, l := range lines {
		if !strings.HasPrefix(l, "??") && strings.TrimSpace(l) != "" {
			return true, nil
		}
	}
	return false, nil
}

func gitOut(src string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = src
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %v", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}

func gitLines(src string, args ...string) ([]string, error) {
	out, err := gitOut(src, args...)
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

// checkAgainst compares a fresh derivation with the existing output file.
// Include-list drift exits 1; a build-context mismatch is a config error
// (cross-context comparison is meaningless — design §4.1), not drift.
func checkAgainst(outPath string, fresh Output) error {
	buf, err := os.ReadFile(outPath)
	if err != nil {
		return fmt.Errorf("-check: read %s: %w", outPath, err)
	}
	var existing Output
	if err := json.Unmarshal(buf, &existing); err != nil {
		return fmt.Errorf("-check: parse %s: %w", outPath, err)
	}
	ec, fc := existing.Provenance.BuildContext, fresh.Provenance.BuildContext
	if ec.GOOS != "" && (ec.GOOS != fc.GOOS || ec.GOARCH != fc.GOARCH || ec.CGO != fc.CGO ||
		strings.Join(ec.Tags, ",") != strings.Join(fc.Tags, ",")) {
		return fmt.Errorf("-check: %s was derived under a different build_context (%+v vs %+v) — cross-context comparison is a config error, not drift", outPath, ec, fc)
	}
	if len(existing.Include) != len(fresh.Include) {
		return driftErr(existing.Include, fresh.Include)
	}
	for i := range fresh.Include {
		if existing.Include[i] != fresh.Include[i] {
			return driftErr(existing.Include, fresh.Include)
		}
	}
	fmt.Printf("filelist-gen: check OK (%d files, src %s)\n", len(fresh.Include), short(fresh.Provenance.SrcCommit))
	return nil
}

func driftErr(old, new []string) error {
	oldSet := map[string]struct{}{}
	for _, f := range old {
		oldSet[f] = struct{}{}
	}
	newSet := map[string]struct{}{}
	for _, f := range new {
		newSet[f] = struct{}{}
	}
	var added, removed []string
	for f := range newSet {
		if _, ok := oldSet[f]; !ok {
			added = append(added, f)
		}
	}
	for f := range oldSet {
		if _, ok := newSet[f]; !ok {
			removed = append(removed, f)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	return fmt.Errorf("drift: +%d -%d files (added: %s; removed: %s)",
		len(added), len(removed), preview(added), preview(removed))
}

func preview(s []string) string {
	if len(s) == 0 {
		return "-"
	}
	if len(s) > 3 {
		return strings.Join(s[:3], ", ") + ", ..."
	}
	return strings.Join(s, ", ")
}

func writeAtomic(path string, v Output) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	buf, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(buf, '\n'), 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	fmt.Printf("filelist-gen: %d files -> %s (src %s)\n", len(v.Include), path, short(v.Provenance.SrcCommit))
	return nil
}

func short(s string) string {
	if len(s) > 9 {
		return s[:9]
	}
	return s
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
