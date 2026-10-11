package buildpipe

import (
	"bufio"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/0xmhha/knowledge-system/internal/graph/persist"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
	"github.com/0xmhha/knowledge-system/pkg/system/testpath"
)

// reviewTestFiles classifies the graph's source files as test code: the
// testpath rule, plus Go packages of this module that only test code
// imports, when they are internal or test-named (internal/testlog,
// p/testrand). Public packages are left alone because callers outside the
// repository may import them. The import rule is applied until no package
// changes, so a helper used only by another test helper is caught too.
func reviewTestFiles(srcRoot string, nodes []types.Node) []string {
	files := map[string]bool{}
	dirFiles := map[string][]string{}
	importers := map[string][]string{} // import path -> importing files
	for _, n := range nodes {
		if n.FilePath == "" {
			continue
		}
		if n.Type == types.NodeImport {
			importers[n.Name] = append(importers[n.Name], n.FilePath)
			continue
		}
		if !files[n.FilePath] {
			files[n.FilePath] = testpath.IsTest(n.FilePath)
			if strings.HasSuffix(n.FilePath, ".go") {
				d := path.Dir(filepath.ToSlash(n.FilePath))
				dirFiles[d] = append(dirFiles[d], n.FilePath)
			}
		}
	}
	if module := goModulePath(srcRoot); module != "" {
		for changed := true; changed; {
			changed = false
			for imp, from := range importers {
				dir, ok := strings.CutPrefix(imp, module+"/")
				if !ok || len(dirFiles[dir]) == 0 || !testOnlyCandidate(dir) || files[dirFiles[dir][0]] {
					continue
				}
				onlyTests := true
				for _, f := range from {
					if !files[f] {
						onlyTests = false
						break
					}
				}
				if onlyTests {
					for _, f := range dirFiles[dir] {
						files[f] = true
					}
					changed = true
				}
			}
		}
	}
	var out []string
	for f, isTest := range files {
		if isTest {
			out = append(out, f)
		}
	}
	sort.Strings(out)
	return out
}

// testOnlyCandidate admits internal packages and packages whose name says
// they are for tests.
func testOnlyCandidate(dir string) bool {
	segs := strings.Split(dir, "/")
	for _, s := range segs {
		if s == "internal" {
			return true
		}
	}
	return strings.Contains(strings.ToLower(segs[len(segs)-1]), "test")
}

// goModulePath returns the module path declared in srcRoot/go.mod, or "".
func goModulePath(srcRoot string) string {
	f, err := os.Open(filepath.Join(srcRoot, "go.mod"))
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(sc.Text()), "module "); ok {
			return strings.Trim(strings.TrimSpace(rest), `"`)
		}
	}
	return ""
}

// rebuildReviewIndex classifies test files and recomputes the review index
// (schema 1.24) once every node and edge of the build is stored.
func rebuildReviewIndex(store persist.StoreWriter, srcRoot string, nodes []types.Node, log *slog.Logger) error {
	testFiles := reviewTestFiles(srcRoot, nodes)
	if err := store.RebuildReviewIndex(testFiles); err != nil {
		return err
	}
	log.Info("review index rebuilt", "test_files", len(testFiles))
	return nil
}
