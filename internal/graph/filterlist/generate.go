// Generation of a FilterList from a Go main-package closure.
//
// This is the Go port of the `go list -deps` shell logic in
// graph/scripts/index-project.sh: given one or more main packages, resolve
// their transitive dependency closure, keep only the packages that live inside
// the module, and emit an include glob `<module-relative-dir>/*.go` per package
// (the module root itself → `*.go`). Solidity include globs are appended
// verbatim and the exclude list is set through. The result feeds `graph build`
// exactly like a loaded --files-from JSON would (see FilterList / Load).
//
// Mirrors internal/vector/discover.ResolveGoBuildRoots' subprocess + JSON-stream
// parsing, but produces module-relative globs instead of an absolute file set.

package filterlist

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// goListPkg is the subset of `go list -json` output GenerateFromMain needs.
// The full JSON is large; we decode only the load-bearing fields.
type goListPkg struct {
	Dir        string        `json:"Dir"`
	ImportPath string        `json:"ImportPath"`
	Standard   bool          `json:"Standard"`
	Module     *goListModule `json:"Module"`
}

type goListModule struct {
	Path string `json:"Path"`
	Main bool   `json:"Main"`
	Dir  string `json:"Dir"`
}

// GenerateFromMain builds a FilterList from the dependency closure of mainPkgs.
//
// moduleRoot is the directory `go list` runs in — it must be inside the module
// that owns mainPkgs. For every package in the closure that lives inside the
// module, GenerateFromMain emits an include glob `<module-relative-dir>/*.go`
// (forward slashes; the module root maps to the bare `*.go`). solIncludes are
// appended to Include verbatim (e.g. "contracts/**/*.sol") and excludes becomes
// Exclude. Include is deduped and sorted for run-to-run determinism.
//
// Returns an error when mainPkgs is empty, `go` is not on PATH, `go list`
// fails, or the closure contains no in-module packages.
func GenerateFromMain(ctx context.Context, moduleRoot string, mainPkgs, solIncludes, excludes []string) (*FilterList, error) {
	if len(mainPkgs) == 0 {
		return nil, errors.New("filterlist: GenerateFromMain called with no main packages")
	}
	if _, err := exec.LookPath("go"); err != nil {
		return nil, fmt.Errorf("filterlist: `go` not in PATH — required for --files-from-main")
	}

	args := append([]string{"list", "-json", "-deps"}, mainPkgs...)
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = moduleRoot
	stdout, err := cmd.Output()
	if err != nil {
		stderr := ""
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			stderr = strings.TrimSpace(string(ee.Stderr))
		}
		return nil, fmt.Errorf("filterlist: go list -deps failed in %s: %w (stderr: %s)", moduleRoot, err, stderr)
	}

	// Resolve the module root once so pkg.Dir (which `go list` returns with
	// symlinks evaluated, e.g. /private/var on macOS) can be made relative.
	rootAbs := resolveDir(moduleRoot)

	includeSet := make(map[string]struct{})
	// `go list -json` emits a stream of concatenated JSON objects, not an
	// array; loop over Decode() calls.
	dec := json.NewDecoder(strings.NewReader(string(stdout)))
	for dec.More() {
		var pkg goListPkg
		if err := dec.Decode(&pkg); err != nil {
			return nil, fmt.Errorf("filterlist: parse go list output: %w", err)
		}
		if pkg.Standard || pkg.Dir == "" {
			continue // stdlib and synthetic packages live outside the module
		}
		if !inModule(pkg, rootAbs) {
			continue // third-party dependency (module cache) — drop it
		}
		glob, ok := pkgGlob(rootAbs, pkg.Dir)
		if !ok {
			continue
		}
		includeSet[glob] = struct{}{}
	}
	if len(includeSet) == 0 {
		return nil, fmt.Errorf("filterlist: go list returned no in-module packages for %v — check that the paths are valid Go packages inside %s", mainPkgs, moduleRoot)
	}

	// Append the Solidity includes verbatim, then dedupe + sort the union so
	// the output is byte-identical across runs.
	for _, g := range solIncludes {
		if g = strings.TrimSpace(g); g != "" {
			includeSet[g] = struct{}{}
		}
	}
	include := make([]string, 0, len(includeSet))
	for g := range includeSet {
		include = append(include, g)
	}
	sort.Strings(include)

	return &FilterList{Include: include, Exclude: excludes}, nil
}

// inModule reports whether pkg belongs to the main module. Primary signal is
// Module.Main from `go list`; the Dir-under-root check is a fallback for the
// rare cases where the Module block is absent.
func inModule(pkg goListPkg, rootAbs string) bool {
	if pkg.Module != nil && pkg.Module.Main {
		return true
	}
	rel, err := filepath.Rel(rootAbs, resolveDir(pkg.Dir))
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// pkgGlob converts a package directory to a module-relative `<dir>/*.go` glob.
// The module root itself maps to the bare "*.go". Returns ok=false when dir
// resolves outside rootAbs.
func pkgGlob(rootAbs, dir string) (string, bool) {
	rel, err := filepath.Rel(rootAbs, resolveDir(dir))
	if err != nil {
		return "", false
	}
	rel = filepath.ToSlash(rel)
	if rel == "." || rel == "" {
		return "*.go", true
	}
	if strings.HasPrefix(rel, "../") {
		return "", false
	}
	return rel + "/*.go", true
}

// resolveDir returns the symlink-evaluated absolute form of dir, falling back
// to the absolute path (then dir itself) when resolution fails.
func resolveDir(dir string) string {
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		return resolved
	}
	if abs, err := filepath.Abs(dir); err == nil {
		return abs
	}
	return dir
}
