package buildscope

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Profile is a project's build profile: the options its build command uses
// and the exceptions, files outside the Go build that belong to the scope. It
// has the shape of projects/<pack>/filelist.yaml.
//
//	build_context: {goos: linux, goarch: amd64, cgo: true, tags: [ckzg]}
//	build_roots: [./cmd/gstable]
//	include_package_tests: true
//	inclusions:
//	  - id: system-contracts
//	    reason: gstable embeds their bytecode and calls them by address
//	    files: ["systemcontracts/**/*.sol"]
//	    linked_by: {embed: ["systemcontracts/artifacts/v1/*"]}
//	exclude_globs: []
type Profile struct {
	BuildContext struct {
		GOOS   string `yaml:"goos"`
		GOARCH string `yaml:"goarch"`
		// CGO is nil when the profile leaves cgo to the go command's default.
		CGO  *bool    `yaml:"cgo"`
		Tags []string `yaml:"tags"`
	} `yaml:"build_context"`
	BuildRoots          []string `yaml:"build_roots"`
	IncludePackageTests bool     `yaml:"include_package_tests"`
	// IncludeEmbedFiles is accepted for filelist.yaml compatibility; the
	// graph has no parser for embedded files, so true is refused.
	IncludeEmbedFiles bool        `yaml:"include_embed_files"`
	Inclusions        []Inclusion `yaml:"inclusions"`
	// ExtraPackages and ExtraGlobs are the older, unexplained form of an
	// inclusion. They still work, as the inclusions "extra_packages" and
	// "extra_globs", but a glob that matches nothing is not an error.
	ExtraPackages []string `yaml:"extra_packages"`
	ExtraGlobs    []string `yaml:"extra_globs"`
	// ExcludeGlobs remove files from the whole scope, inclusions included.
	ExcludeGlobs []string `yaml:"exclude_globs"`
}

// Inclusion is one exception: code the build does not compile that belongs
// to the scope because it is needed to understand the code it does compile.
type Inclusion struct {
	// ID names the inclusion in the scope (lower case, digits, hyphens).
	ID string `yaml:"id"`
	// Reason says why the code belongs to the scope; it is required.
	Reason string `yaml:"reason"`
	// Files are globs of files to add (`**` matches any number of
	// directories); every glob must match a file.
	Files []string `yaml:"files"`
	// GoPackages are main-module package patterns whose own files are added.
	GoPackages []string `yaml:"go_packages"`
	// Exclude removes files from this inclusion only.
	Exclude []string `yaml:"exclude"`
	// LinkedBy ties the inclusion to the build: extraction fails when the
	// build no longer produces what is named here, so an exception cannot
	// outlive its reason unnoticed. Without it the reason alone admits the
	// files.
	LinkedBy *struct {
		// Embed are globs of files the build must embed (go:embed), for
		// example the compiled contracts generated from the included sources.
		Embed []string `yaml:"embed"`
	} `yaml:"linked_by"`
}

// Options are the build options of one extraction. A set field overrides the
// profile, so the scope always follows the options the build actually uses.
type Options struct {
	GOOS, GOARCH string
	// CGOEnabled is "1", "0" or "" (keep the profile's, else the default).
	CGOEnabled string
	// Tags replaces the profile's tags when TagsSet is true, so an empty list
	// can clear them.
	Tags    []string
	TagsSet bool
	// EntryPoints replace the profile's build_roots when not empty.
	EntryPoints []string
	// ExtraGlobs are added as the inclusion "command-line"; each must match.
	ExtraGlobs []string
}

// ProjectConfig is one extraction request.
type ProjectConfig struct {
	Root     string
	ConfigID string
	// ProfilePath is the build profile; empty extracts from Options alone,
	// with the tests of every package included.
	ProfilePath string
	Options     Options
	// Binaries are built binaries to check the scope against: each must
	// record the scope's GOOS, GOARCH, CGO_ENABLED and tags and one of its
	// entry points as its main package.
	Binaries []string
	// AllowProblems keeps a scope whose go command reported load errors.
	AllowProblems bool
	GoBinary      string
}

var inclusionID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// LoadProfile reads a build profile; unknown keys are an error so that a
// misspelt option cannot silently fall back to a default, and so is an
// inclusion without an id, a reason or anything to include.
func LoadProfile(p string) (*Profile, []byte, error) {
	raw, err := os.ReadFile(p)
	if err != nil {
		return nil, nil, fmt.Errorf("buildscope: read profile: %w", err)
	}
	var prof Profile
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&prof); err != nil {
		return nil, nil, fmt.Errorf("buildscope: parse profile %s: %w", p, err)
	}
	seen := map[string]bool{}
	for i, in := range prof.Inclusions {
		switch {
		case !inclusionID.MatchString(in.ID):
			return nil, nil, fmt.Errorf("buildscope: profile %s: inclusion %d: id %q must be lower case letters, digits and hyphens", p, i+1, in.ID)
		case seen[in.ID]:
			return nil, nil, fmt.Errorf("buildscope: profile %s: inclusion id %q is used twice", p, in.ID)
		case strings.TrimSpace(in.Reason) == "":
			return nil, nil, fmt.Errorf("buildscope: profile %s: inclusion %s has no reason", p, in.ID)
		case len(in.Files) == 0 && len(in.GoPackages) == 0:
			return nil, nil, fmt.Errorf("buildscope: profile %s: inclusion %s includes nothing (give files or go_packages)", p, in.ID)
		case in.LinkedBy != nil && len(in.LinkedBy.Embed) == 0:
			return nil, nil, fmt.Errorf("buildscope: profile %s: inclusion %s: linked_by names no build output", p, in.ID)
		}
		seen[in.ID] = true
		for _, g := range append(append(append([]string(nil), in.Files...), in.Exclude...), embedGlobs(in)...) {
			if err := checkGlob(g); err != nil {
				return nil, nil, fmt.Errorf("buildscope: profile %s: inclusion %s: %w", p, in.ID, err)
			}
		}
	}
	return &prof, raw, nil
}

// ExtractProject extracts the scope of one build: the Go files the go command
// builds for the profile's options (overridden by cfg.Options), the files the
// profile's inclusions add, minus its excludes. It fails when the go command
// reports load errors (unless AllowProblems), when an inclusion adds nothing
// or its link to the build is gone, and when a binary in cfg.Binaries was
// built with other options.
func ExtractProject(ctx context.Context, cfg ProjectConfig) (*Scope, error) {
	prof := &Profile{IncludePackageTests: true}
	var ref *ProfileRef
	if cfg.ProfilePath != "" {
		p, raw, err := LoadProfile(cfg.ProfilePath)
		if err != nil {
			return nil, err
		}
		if p.IncludeEmbedFiles {
			return nil, errors.New("buildscope: include_embed_files is not supported: the graph does not parse embedded files")
		}
		sum := sha256.Sum256(raw)
		prof, ref = p, &ProfileRef{Path: cfg.ProfilePath, SHA256: hex.EncodeToString(sum[:])}
	}
	incs := inclusionsOf(prof, cfg.Options)
	o := cfg.Options
	gc := GoConfig{
		Root: cfg.Root, ConfigID: cfg.ConfigID, GoBinary: cfg.GoBinary,
		GOOS: prof.BuildContext.GOOS, GOARCH: prof.BuildContext.GOARCH, Tags: prof.BuildContext.Tags,
		EntryPoints: prof.BuildRoots, SkipTests: !prof.IncludePackageTests,
	}
	for _, in := range incs {
		if len(in.GoPackages) > 0 {
			gc.PackageSets = append(gc.PackageSets, PackageSet{ID: in.ID, Patterns: in.GoPackages})
		}
	}
	if c := prof.BuildContext.CGO; c != nil {
		gc.CGOEnabled = map[bool]string{true: "1", false: "0"}[*c]
	}
	if o.GOOS != "" {
		gc.GOOS = o.GOOS
	}
	if o.GOARCH != "" {
		gc.GOARCH = o.GOARCH
	}
	if o.CGOEnabled != "" {
		gc.CGOEnabled = o.CGOEnabled
	}
	if o.TagsSet {
		gc.Tags = o.Tags
	}
	if len(o.EntryPoints) > 0 {
		gc.EntryPoints = o.EntryPoints
	}
	s, err := ExtractGo(ctx, gc)
	if err != nil {
		return nil, err
	}
	s.Profile = ref
	if err := applyInclusions(s, incs); err != nil {
		return nil, err
	}
	s.Files = keep(s.Files, func(f File) bool { return !matchAny(prof.ExcludeGlobs, f.Path) })
	for i := range s.Inclusions {
		s.Inclusions[i].Files = 0
		for _, f := range s.Files {
			if f.IncludedBy == s.Inclusions[i].ID {
				s.Inclusions[i].Files++
			}
		}
	}
	if len(s.Problems) > 0 && !cfg.AllowProblems {
		sort.Strings(s.Problems)
		return nil, loadErrorsError(s.Problems, s.GoStderr)
	}
	for _, b := range cfg.Binaries {
		cb, err := CheckBinary(ctx, cfg.GoBinary, b, s)
		if err != nil {
			return nil, err
		}
		s.CheckedBinaries = append(s.CheckedBinaries, cb)
	}
	sortScope(s)
	return s, nil
}

// inclusion is an Inclusion as applied; lenient is set for the older
// extra_* keys, whose globs may match nothing.
type inclusion struct {
	Inclusion
	lenient bool
}

// inclusionsOf lists the profile's inclusions, the older extra_* keys and the
// command-line globs as one list.
func inclusionsOf(prof *Profile, o Options) []inclusion {
	var out []inclusion
	for _, in := range prof.Inclusions {
		out = append(out, inclusion{Inclusion: in})
	}
	if len(prof.ExtraPackages) > 0 {
		out = append(out, inclusion{Inclusion: Inclusion{ID: "extra_packages", Reason: "extra_packages of the profile (no reason recorded)", GoPackages: prof.ExtraPackages}, lenient: true})
	}
	if len(prof.ExtraGlobs) > 0 {
		out = append(out, inclusion{Inclusion: Inclusion{ID: "extra_globs", Reason: "extra_globs of the profile (no reason recorded)", Files: prof.ExtraGlobs}, lenient: true})
	}
	if len(o.ExtraGlobs) > 0 {
		out = append(out, inclusion{Inclusion: Inclusion{ID: "command-line", Reason: "added with --include for this extraction", Files: o.ExtraGlobs}})
	}
	return out
}

// applyInclusions adds each inclusion's files (its go_packages files are
// already in s, marked with its id), drops the ones its exclude names, checks
// that it added something and that the build still embeds what links it,
// and records it in s.Inclusions.
func applyInclusions(s *Scope, incs []inclusion) error {
	if len(incs) == 0 {
		return nil
	}
	tree, err := treeFiles(s.Root)
	if err != nil {
		return err
	}
	have := map[string]bool{}
	for _, f := range s.Files {
		have[f.Path] = true
	}
	for _, in := range incs {
		s.Files = keep(s.Files, func(f File) bool { return f.IncludedBy != in.ID || !matchAny(in.Exclude, f.Path) })
		for _, g := range in.Files {
			matched := 0
			for _, rel := range tree {
				if !matchAny([]string{g}, rel) || matchAny(in.Exclude, rel) {
					continue
				}
				matched++
				if !have[rel] {
					have[rel] = true
					s.Files = append(s.Files, File{Path: rel, Language: languageOf(rel), Role: RoleProduction, IncludedBy: in.ID})
				}
			}
			if matched == 0 && !in.lenient {
				return fmt.Errorf("buildscope: inclusion %s: %q matches no file", in.ID, g)
			}
		}
		added := 0
		for _, f := range s.Files {
			if f.IncludedBy == in.ID {
				added++
			}
		}
		if added == 0 && !in.lenient {
			return fmt.Errorf("buildscope: inclusion %s adds no file: everything it names is already in the scope (the build or an earlier inclusion) or excluded", in.ID)
		}
		rec := InclusionRecord{ID: in.ID, Reason: strings.TrimSpace(in.Reason)}
		for _, g := range embedGlobs(in.Inclusion) {
			n := 0
			for _, e := range s.EmbedFiles {
				if matchAny([]string{g}, e) {
					n++
				}
			}
			if n == 0 {
				return fmt.Errorf("buildscope: inclusion %s is linked by embedded %q, but the build embeds no such file", in.ID, g)
			}
			rec.LinkedBy = append(rec.LinkedBy, "embed:"+g)
		}
		s.Inclusions = append(s.Inclusions, rec)
	}
	return nil
}

func embedGlobs(in Inclusion) []string {
	if in.LinkedBy == nil {
		return nil
	}
	return in.LinkedBy.Embed
}

// treeFiles lists the files under root, slash-separated and relative;
// version-control and dependency directories are not searched.
func treeFiles(root string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	return out, err
}

func checkGlob(g string) error {
	if _, err := path.Match(strings.ReplaceAll(g, "**", "x"), ""); err != nil {
		return fmt.Errorf("bad glob %q: %w", g, err)
	}
	return nil
}

func keep(fs []File, ok func(File) bool) []File {
	out := fs[:0]
	for _, f := range fs {
		if ok(f) {
			out = append(out, f)
		}
	}
	return out
}

func matchAny(globs []string, rel string) bool {
	for _, g := range globs {
		if matchGlob(strings.Split(g, "/"), strings.Split(rel, "/")) {
			return true
		}
	}
	return false
}

// matchGlob matches slash-separated segments; a "**" segment matches zero or
// more segments, other segments use path.Match.
func matchGlob(pat, name []string) bool {
	for len(pat) > 0 {
		if pat[0] == "**" {
			for i := 0; i <= len(name); i++ {
				if matchGlob(pat[1:], name[i:]) {
					return true
				}
			}
			return false
		}
		if len(name) == 0 {
			return false
		}
		if ok, _ := path.Match(pat[0], name[0]); !ok {
			return false
		}
		pat, name = pat[1:], name[1:]
	}
	return len(name) == 0
}

// CheckBinary compares the build settings a Go binary records (`go version
// -m`) with the scope: GOOS, GOARCH, CGO_ENABLED, the set of tags, and its
// main package against the scope's entry points. A mismatch means the scope
// was extracted with options other than the ones the binary was built with.
func CheckBinary(ctx context.Context, goBin, binPath string, s *Scope) (CheckedBinary, error) {
	goPath, err := lookGo(goBin)
	if err != nil {
		return CheckedBinary{}, err
	}
	out, err := run(ctx, goPath, s.Root, os.Environ(), "version", "-m", binPath)
	if err != nil {
		return CheckedBinary{}, err
	}
	settings, mainPkg := map[string]string{}, ""
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		f := strings.SplitN(strings.TrimSpace(sc.Text()), "\t", 2)
		if len(f) != 2 {
			continue
		}
		switch f[0] {
		case "path":
			mainPkg = f[1]
		case "build":
			if k, v, ok := strings.Cut(f[1], "="); ok {
				settings[k] = v
			}
		}
	}
	var diffs []string
	want := func(key, got, scope string) {
		if got != scope {
			diffs = append(diffs, fmt.Sprintf("%s: binary %q, scope %q", key, got, scope))
		}
	}
	want("GOOS", settings["GOOS"], s.Target.GOOS)
	want("GOARCH", settings["GOARCH"], s.Target.GOARCH)
	want("CGO_ENABLED", settings["CGO_ENABLED"], s.Target.CGOEnabled)
	want("tags", sortedTags(strings.Split(settings["-tags"], ",")), sortedTags(s.Target.Tags))
	if pkgs, err := entryPackages(s); err != nil {
		return CheckedBinary{}, err
	} else if pkgs != nil && !pkgs[mainPkg] {
		diffs = append(diffs, fmt.Sprintf("main package: binary %q is not an entry point of the scope %v", mainPkg, s.EntryPoints))
	}
	if len(diffs) > 0 {
		return CheckedBinary{}, fmt.Errorf("buildscope: %s was built with other options than the scope: %s", binPath, strings.Join(diffs, "; "))
	}
	sum, err := fileSHA256(binPath)
	if err != nil {
		return CheckedBinary{}, err
	}
	return CheckedBinary{Path: binPath, Package: mainPkg, SHA256: sum}, nil
}

func sortedTags(tags []string) string {
	var t []string
	for _, x := range tags {
		if x = strings.TrimSpace(x); x != "" {
			t = append(t, x)
		}
	}
	sort.Strings(t)
	return strings.Join(t, ",")
}

// entryPackages maps relative entry points ("./cmd/app") to import paths with
// the module path of go.mod. It returns nil, skipping the check, when an
// entry point is a pattern ("./cmd/...") or already an import path.
func entryPackages(s *Scope) (map[string]bool, error) {
	raw, err := os.ReadFile(filepath.Join(s.Root, "go.mod"))
	if err != nil {
		return nil, fmt.Errorf("buildscope: %w", err)
	}
	mod := ""
	for _, line := range strings.Split(string(raw), "\n") {
		if f := strings.Fields(line); len(f) == 2 && f[0] == "module" {
			mod = strings.Trim(f[1], `"`)
		}
	}
	out := map[string]bool{}
	for _, ep := range s.EntryPoints {
		if strings.Contains(ep, "...") || (ep != "." && !strings.HasPrefix(ep, "./")) {
			return nil, nil
		}
		rel := strings.TrimPrefix(strings.TrimPrefix(ep, "."), "/")
		if rel == "" {
			out[mod] = true
		} else {
			out[mod+"/"+rel] = true
		}
	}
	return out, nil
}

func lookGo(goBin string) (string, error) {
	if goBin == "" {
		goBin = "go"
	}
	p, err := exec.LookPath(goBin)
	if err != nil {
		return "", fmt.Errorf("buildscope: go command not found: %w", err)
	}
	return p, nil
}
