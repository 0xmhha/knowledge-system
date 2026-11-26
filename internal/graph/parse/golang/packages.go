package golang

import (
	"path"
	"path/filepath"
	"strings"

	"github.com/0xmhha/knowledge-system/internal/graph/parse"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// packageNodeID is the id of the Package node of the package called pkgName
// that the file relPath belongs to. One node per package directory: files of
// one package share it, two packages that declare the same name do not.
func packageNodeID(relPath, pkgName string) string {
	return parse.NodeID(nodeLanguage, path.Dir(filepath.ToSlash(relPath)), pkgName, 0)
}

// CanonicalPackagePaths gives every Go Package node of nodes the file that
// shows the package: its first file, in path order, that is not test code, or
// its first file when all are. isTest says whether a file is test code.
//
// Each file of a package emits the package's node with its own path, and the
// store keeps one node per id, so the path a package ended up with was that
// of whichever file came last. A node is test code when its file is: 58 of
// the 140 packages of go-stablenet were flagged as tests through a _test.go
// path although they have production files (p2p, with 17 files, showed
// p2p/util_test.go). With the rule here a package is test code only when all
// its files are, and it shows the same file on every build.
func CanonicalPackagePaths(nodes []types.Node, isTest func(file string) bool) {
	shown := map[string]string{} // package node id -> file
	better := func(file, than string) bool {
		if than == "" {
			return true
		}
		if a, b := isTest(file), isTest(than); a != b {
			return !a
		}
		return file < than
	}
	for i := range nodes {
		n := &nodes[i]
		if n.Type != types.NodeFile || n.Language != nodeLanguage {
			continue
		}
		// A File node is named "<package>/<path>" (newDeclVisitor).
		pkgName, ok := strings.CutSuffix(n.QualifiedName, "/"+n.Name)
		if !ok {
			continue
		}
		id := packageNodeID(n.FilePath, pkgName)
		if better(n.FilePath, shown[id]) {
			shown[id] = n.FilePath
		}
	}
	for i := range nodes {
		n := &nodes[i]
		if n.Type != types.NodePackage || n.Language != nodeLanguage {
			continue
		}
		if file, ok := shown[n.ID]; ok {
			n.FilePath = file
		}
	}
}

// isGoTestFile is the test-file rule of the go command, for callers without
// a build scope.
func isGoTestFile(file string) bool { return strings.HasSuffix(file, "_test.go") }
