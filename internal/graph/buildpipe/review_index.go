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
)

// reviewTestFiles classifies the graph's source files as test code.
//
// A file the build scope lists has the role the scope gives it (roles).
// Any other file is judged by the testpath rule, plus Go packages of this
// module that only test code imports, when they are internal or test-named
// (internal/testlog, p/testrand). Public packages are left alone because
// callers outside the repository may import them. The import rule is applied
// until no package changes, so a helper used only by another test helper is
// caught too; it never overrides a role the scope gave.
func reviewTestFiles(srcRoot string, nodes []types.Node, roles FileRoles) []string {
	files := map[string]bool{}
	scoped := map[string]bool{} // directories with a file whose role the scope gives
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
		if _, seen := files[n.FilePath]; !seen {
			test, fromScope := roles.isTest(n.FilePath)
			files[n.FilePath] = test
			if strings.HasSuffix(n.FilePath, ".go") {
				d := path.Dir(filepath.ToSlash(n.FilePath))
				dirFiles[d] = append(dirFiles[d], n.FilePath)
				scoped[d] = scoped[d] || fromScope
			}
		}
	}
	if module := goModulePath(srcRoot); module != "" {
		for changed := true; changed; {
			changed = false
			for imp, from := range importers {
				dir, ok := strings.CutPrefix(imp, module+"/")
				if !ok || len(dirFiles[dir]) == 0 || scoped[dir] || !testOnlyCandidate(dir) || files[dirFiles[dir][0]] {
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
func rebuildReviewIndex(store persist.StoreWriter, srcRoot string, nodes []types.Node, roles FileRoles, log *slog.Logger) error {
	testFiles := reviewTestFiles(srcRoot, nodes, roles)
	if err := store.RebuildReviewIndex(testFiles); err != nil {
		return err
	}
	log.Info("review index rebuilt", "test_files", len(testFiles))
	return nil
}
