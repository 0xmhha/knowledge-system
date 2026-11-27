package buildpipe

import (
	"path/filepath"

	"github.com/0xmhha/knowledge-system/internal/graph/contract"
	"github.com/0xmhha/knowledge-system/pkg/system/testpath"
)

// FileRole is the role of a file in the build scope a graph is built for.
// The values are the role names of a scope document (internal/buildscope).
type FileRole string

const (
	// FileRoleProduction is a file compiled into the build's outputs, or one
	// the build profile includes.
	FileRoleProduction FileRole = "production"
	// FileRoleTest is a test file of a package of the build.
	FileRoleTest FileRole = "test"
)

// FileRoles maps the files of a build scope (slash-separated paths relative
// to the source root) to their roles. It decides which files are test code:
// the is_test flag of their nodes and the layer of their contract records.
//
// Without a scope a file is test code by its path (testpath.IsTest): the
// _test.go suffix, and also helper names and test directories (testutil.go,
// a path segment "test"), because nothing else says what such a file is for.
// A scope does say: it lists the file as compiled into the build or as a test
// file of one of its packages. By the path rule alone, 2,410 nodes in 6 files
// of go-stablenet that the scope lists as production (systemcontracts/test/*.go,
// triedb/pathdb/testutils.go, consensus/wbft/testutil.go) were flagged as
// tests and left out of every query that excludes tests.
//
// Queries read the role back from the nodes (store.TestFileRule); only the
// vector index, which holds no graph node, still judges a result by its path
// (testpath.IsTest).
type FileRoles map[string]FileRole

// isTest reports whether file is test code, and whether the scope said so
// (scoped) or the path rule did.
func (r FileRoles) isTest(file string) (test, scoped bool) {
	switch r[filepath.ToSlash(file)] {
	case FileRoleTest:
		return true, true
	case FileRoleProduction:
		return false, true
	}
	return testpath.IsTest(file), false
}

// testFile reports whether file is test code (isTest without its source).
func (r FileRoles) testFile(file string) bool {
	test, _ := r.isTest(file)
	return test
}

// layer is the contract layer of file's declarations.
func (r FileRoles) layer(file string) contract.Layer {
	if test, _ := r.isTest(file); test {
		return contract.LayerTest
	}
	return contract.LayerProduction
}
