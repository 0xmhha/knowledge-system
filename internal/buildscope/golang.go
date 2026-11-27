package buildscope

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GoConfig is one Go build configuration.
type GoConfig struct {
	// Root is the directory of the main module's go.mod.
	Root     string
	ConfigID string
	// GOOS and GOARCH default to the host's when empty.
	GOOS, GOARCH string
	// CGOEnabled is "1" or "0"; empty keeps the go command's default, which
	// turns cgo off when GOOS/GOARCH differ from the host. Set it to match the
	// real build: cgo decides files such as crypto/secp256k1 in go-ethereum.
	CGOEnabled string
	Tags       []string
	// EntryPoints are the package patterns the build command builds, for
	// example "./cmd/gstable". They must be inside the main module.
	EntryPoints []string
	// GoBinary is the go command to run; "go" from PATH when empty.
	GoBinary string
	// SkipTests leaves the test files of the configuration's packages out.
	SkipTests bool
	// PackageSets are main-module package patterns that belong to the scope
	// without being imported by an entry point, for example an integration
	// test package. Their own files are added, not their dependencies, and
	// each file records the set that added it.
	PackageSets []PackageSet
}

// PackageSet is a named group of package patterns.
type PackageSet struct {
	ID       string
	Patterns []string
}

// goListPkg is the part of `go list -json` output the extractor reads.
type goListPkg struct {
	ImportPath     string
	Dir            string
	Standard       bool
	GoFiles        []string
	CgoFiles       []string
	SFiles         []string
	IgnoredGoFiles []string
	EmbedFiles     []string
	TestGoFiles    []string
	XTestGoFiles   []string
	Module         *goListModule
	Error          *struct{ Err string }
	DepsErrors     []struct{ Err string }
}

type goListModule struct {
	Path    string
	Version string
	Main    bool
	Dir     string
	Replace *goListModule
}

// ExtractGo returns the scope of one Go build configuration: the files `go
// list` puts into the build of the entry points for the configuration's
// platform and tags, the test files of the main-module packages in that
// build, and the outside modules the build uses.
func ExtractGo(ctx context.Context, cfg GoConfig) (*Scope, error) {
	if len(cfg.EntryPoints) == 0 {
		return nil, errors.New("buildscope: no entry points")
	}
	goBin := cfg.GoBinary
	if goBin == "" {
		goBin = "go"
	}
	goPath, err := exec.LookPath(goBin)
	if err != nil {
		return nil, fmt.Errorf("buildscope: go command not found: %w", err)
	}
	root, err := filepath.Abs(cfg.Root)
	if err != nil {
		return nil, fmt.Errorf("buildscope: root: %w", err)
	}
	if real, err := filepath.EvalSymlinks(root); err == nil {
		root = real
	}
	env := goEnv(cfg)

	scope := &Scope{
		SchemaVersion: SchemaVersion,
		ConfigID:      cfg.ConfigID,
		Root:          root,
		EntryPoints:   append([]string(nil), cfg.EntryPoints...),
		Target:        Target{GOOS: cfg.GOOS, GOARCH: cfg.GOARCH, Tags: append([]string(nil), cfg.Tags...)},
	}
	tc, err := goToolchain(ctx, goPath, root, env)
	if err != nil {
		return nil, err
	}
	scope.Toolchains = []Toolchain{tc}
	// Record the effective platform and cgo setting, defaults included.
	goos, goarch, cgo, err := goTarget(ctx, goPath, root, env)
	if err != nil {
		return nil, err
	}
	scope.Target.GOOS, scope.Target.GOARCH, scope.Target.CGOEnabled = goos, goarch, cgo

	seenModule := map[string]bool{}
	seenFile := map[string]bool{}
	var stderrLines []string
	list := func(deps bool, patterns []string, set string) error {
		args := []string{"list", "-e", "-json"}
		if deps {
			args = append(args, "-deps")
		}
		if len(cfg.Tags) > 0 {
			args = append(args, "-tags", strings.Join(cfg.Tags, ","))
		}
		args = append(args, patterns...)
		out, warn, err := runStderr(ctx, goPath, root, env, args...)
		if err != nil {
			return err
		}
		if warn != "" {
			stderrLines = append(stderrLines, warn)
		}
		// Only the entry points' build embeds files into the outputs.
		return addPackages(scope, root, out, packageListing{skipTests: cfg.SkipTests, set: set, embeds: deps}, seenModule, seenFile)
	}
	if err := list(true, cfg.EntryPoints, ""); err != nil {
		return nil, err
	}
	for _, ps := range cfg.PackageSets {
		if err := list(false, ps.Patterns, ps.ID); err != nil {
			return nil, err
		}
	}
	scope.GoStderr = stderrLines
	if len(scope.Files) == 0 {
		return nil, noFilesError(cfg.EntryPoints, root, scope.Problems, stderrLines)
	}
	sortScope(scope)
	return scope, nil
}

// packageListing says how one `go list` result enters the scope.
type packageListing struct {
	skipTests bool
	// set is recorded as IncludedBy of the files it adds; "" for the build.
	set string
	// embeds records the packages' go:embed files in scope.EmbedFiles.
	embeds bool
}

// addPackages adds the main-module files of a `go list -json` stream to
// scope and records the outside modules and load errors it reports. seen*
// keep a module or file that two listings return from being added twice.
func addPackages(scope *Scope, root string, out []byte, l packageListing, seenModule, seenFile map[string]bool) error {
	dec := json.NewDecoder(bytes.NewReader(out))
	for {
		var p goListPkg
		if err := dec.Decode(&p); errors.Is(err, io.EOF) {
			return nil
		} else if err != nil {
			return fmt.Errorf("buildscope: parse go list output: %w", err)
		}
		if p.Error != nil {
			scope.Problems = append(scope.Problems, p.ImportPath+": "+p.Error.Err)
		}
		if p.Standard || p.Module == nil {
			continue
		}
		if !p.Module.Main {
			if !seenModule[p.Module.Path] {
				seenModule[p.Module.Path] = true
				scope.OutsideModules = append(scope.OutsideModules, outsideModule(p.Module))
			}
			continue
		}
		rel, err := filepath.Rel(root, p.Dir)
		if err != nil || strings.HasPrefix(rel, "..") {
			continue
		}
		add := func(list *[]File, names []string, role, pkg string) {
			for _, n := range names {
				path := filepath.ToSlash(filepath.Join(rel, n))
				key := role + " " + path
				if list == &scope.Ignored {
					key = "ignored " + path
				}
				if seenFile[key] {
					continue
				}
				seenFile[key] = true
				f := File{Path: path, Language: languageOf(n), Role: role, Package: pkg}
				if list == &scope.Files {
					f.IncludedBy = l.set
				}
				*list = append(*list, f)
			}
		}
		add(&scope.Files, p.GoFiles, RoleProduction, p.ImportPath)
		add(&scope.Files, p.CgoFiles, RoleProduction, p.ImportPath)
		add(&scope.Files, p.SFiles, RoleProduction, p.ImportPath)
		if !l.skipTests {
			add(&scope.Files, p.TestGoFiles, RoleTest, p.ImportPath)
			add(&scope.Files, p.XTestGoFiles, RoleTest, p.ImportPath+"_test")
		}
		add(&scope.Ignored, p.IgnoredGoFiles, RoleProduction, p.ImportPath)
		if l.embeds {
			for _, e := range p.EmbedFiles {
				path := filepath.ToSlash(filepath.Join(rel, e))
				if !seenFile["embed "+path] {
					seenFile["embed "+path] = true
					scope.EmbedFiles = append(scope.EmbedFiles, path)
				}
			}
		}
	}
}

func languageOf(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".go":
		return "go"
	case ".s":
		return "asm"
	default:
		return strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
	}
}

func outsideModule(m *goListModule) Module {
	mod := Module{Path: m.Path, Version: m.Version, Dir: m.Dir}
	if r := m.Replace; r != nil {
		mod.Replace = r.Path
		if r.Version != "" {
			mod.Replace += "@" + r.Version
		}
		if r.Dir != "" {
			mod.Dir = r.Dir
		}
	}
	return mod
}

// goEnv is the environment of every go command: the caller's, with the
// configuration's platform, and no workspace file so only the main module's
// go.mod decides the build.
func goEnv(cfg GoConfig) []string {
	env := append(os.Environ(), "GOWORK=off")
	if cfg.GOOS != "" {
		env = append(env, "GOOS="+cfg.GOOS)
	}
	if cfg.GOARCH != "" {
		env = append(env, "GOARCH="+cfg.GOARCH)
	}
	if cfg.CGOEnabled != "" {
		env = append(env, "CGO_ENABLED="+cfg.CGOEnabled)
	}
	return env
}

func goTarget(ctx context.Context, goPath, root string, env []string) (string, string, string, error) {
	out, err := run(ctx, goPath, root, env, "env", "GOOS", "GOARCH", "CGO_ENABLED")
	if err != nil {
		return "", "", "", err
	}
	f := strings.Fields(string(out))
	if len(f) != 3 {
		return "", "", "", fmt.Errorf("buildscope: unexpected go env output %q", out)
	}
	return f[0], f[1], f[2], nil
}

// goToolchain records the go command's version and checksum and the version
// go.mod declares (its toolchain line, else its go line).
func goToolchain(ctx context.Context, goPath, root string, env []string) (Toolchain, error) {
	out, err := run(ctx, goPath, root, env, "env", "GOVERSION")
	if err != nil {
		return Toolchain{}, err
	}
	sum, err := fileSHA256(goPath)
	if err != nil {
		return Toolchain{}, err
	}
	declared, err := declaredGo(filepath.Join(root, "go.mod"))
	if err != nil {
		return Toolchain{}, err
	}
	return Toolchain{Language: "go", Declared: declared, Installed: strings.TrimSpace(string(out)), Path: goPath, SHA256: sum}, nil
}

// declaredGo reads the toolchain line of go.mod, or the go line when there is
// no toolchain line, as "go<version>".
func declaredGo(goMod string) (string, error) {
	f, err := os.Open(goMod)
	if err != nil {
		return "", fmt.Errorf("buildscope: %w", err)
	}
	defer func() { _ = f.Close() }()
	var goLine, toolchain string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) != 2 {
			continue
		}
		switch fields[0] {
		case "go":
			goLine = "go" + fields[1]
		case "toolchain":
			toolchain = fields[1]
		}
	}
	if err := sc.Err(); err != nil {
		return "", fmt.Errorf("buildscope: read %s: %w", goMod, err)
	}
	if toolchain != "" {
		return toolchain, nil
	}
	return goLine, nil
}

func fileSHA256(path string) (string, error) {
	if real, err := filepath.EvalSymlinks(path); err == nil {
		path = real
	}
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("buildscope: %w", err)
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("buildscope: hash %s: %w", path, err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func run(ctx context.Context, bin, dir string, env []string, args ...string) ([]byte, error) {
	out, _, err := runStderr(ctx, bin, dir, env, args...)
	return out, err
}

// runStderr is run that also returns what the command wrote to standard
// error when it succeeded: go list -e exits 0 after load errors and after
// warnings (a build or module cache it cannot write), and says so only there.
func runStderr(ctx context.Context, bin, dir string, env []string, args ...string) ([]byte, string, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir
	cmd.Env = env
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, "", fmt.Errorf("buildscope: %s %s in %s: %w (stderr: %s)",
			filepath.Base(bin), strings.Join(args, " "), dir, err, strings.TrimSpace(stderr.String()))
	}
	return out, strings.TrimSpace(stderr.String()), nil
}

// maxReportedProblems bounds how many go command errors an error message
// quotes.
const maxReportedProblems = 3

// noFilesError says that go list put no main-module file into the scope,
// with what the go command reported (goReport).
func noFilesError(entries []string, root string, problems, stderr []string) error {
	msg := fmt.Sprintf("buildscope: go list found no main-module files for %v in %s", entries, root)
	msg += "; " + goReport(problems, stderr)
	if len(problems) == 0 && len(stderr) == 0 {
		msg += "; check that the entry points name main-module packages"
	}
	return errors.New(msg)
}

// loadErrorsError says that the go command reported load errors, so the
// scope may be incomplete, with what it reported (goReport).
func loadErrorsError(problems, stderr []string) error {
	return errors.New("buildscope: the scope may be incomplete: " + goReport(problems, stderr))
}

// goReport is the one wording of what the go command reported: how many
// load errors and the first maxReportedProblems of them, and what it wrote
// to standard error, or that it wrote nothing there.
func goReport(problems, stderr []string) string {
	var msg string
	switch n := len(problems); {
	case n > maxReportedProblems:
		msg = fmt.Sprintf("the go command reported %d load errors, the first %d: %s", n, maxReportedProblems,
			strings.Join(problems[:maxReportedProblems], "; "))
	case n > 0:
		msg = fmt.Sprintf("the go command reported %d load errors: %s", n, strings.Join(problems, "; "))
	default:
		msg = "the go command reported no load errors"
	}
	var lines []string
	for _, out := range stderr {
		for _, l := range strings.Split(out, "\n") {
			if l = strings.TrimSpace(l); l != "" {
				lines = append(lines, l)
			}
		}
	}
	if len(lines) == 0 {
		return msg + "; the go command wrote nothing to standard error"
	}
	msg += "; go command standard error: " + strings.Join(lines[:min(len(lines), maxStderrLines)], " | ")
	if left := len(lines) - maxStderrLines; left > 0 {
		msg += fmt.Sprintf(" (%d more lines left out)", left)
	}
	return msg
}

// maxStderrLines bounds how many lines of the go command's standard error
// an error message quotes.
const maxStderrLines = 3
