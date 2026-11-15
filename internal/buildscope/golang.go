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

	args := []string{"list", "-e", "-deps", "-json"}
	if len(cfg.Tags) > 0 {
		args = append(args, "-tags", strings.Join(cfg.Tags, ","))
	}
	args = append(args, cfg.EntryPoints...)
	out, err := run(ctx, goPath, root, env, args...)
	if err != nil {
		return nil, err
	}
	seenModule := map[string]bool{}
	dec := json.NewDecoder(bytes.NewReader(out))
	for {
		var p goListPkg
		if err := dec.Decode(&p); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, fmt.Errorf("buildscope: parse go list output: %w", err)
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
				*list = append(*list, File{
					Path: filepath.ToSlash(filepath.Join(rel, n)), Language: languageOf(n), Role: role, Package: pkg,
				})
			}
		}
		add(&scope.Files, p.GoFiles, RoleProduction, p.ImportPath)
		add(&scope.Files, p.CgoFiles, RoleProduction, p.ImportPath)
		add(&scope.Files, p.SFiles, RoleProduction, p.ImportPath)
		add(&scope.Files, p.TestGoFiles, RoleTest, p.ImportPath)
		add(&scope.Files, p.XTestGoFiles, RoleTest, p.ImportPath+"_test")
		add(&scope.Ignored, p.IgnoredGoFiles, RoleProduction, p.ImportPath)
	}
	if len(scope.Files) == 0 {
		return nil, fmt.Errorf("buildscope: go list found no main-module files for %v in %s", cfg.EntryPoints, root)
	}
	sortScope(scope)
	return scope, nil
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
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir
	cmd.Env = env
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("buildscope: %s %s in %s: %w (stderr: %s)",
			filepath.Base(bin), strings.Join(args, " "), dir, err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}
