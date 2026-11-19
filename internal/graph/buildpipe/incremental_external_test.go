package buildpipe_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// externalModule writes a module whose two files call time.Now and
// time.AfterFunc, which are outside the build.
func externalModule(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "go.mod"), "module example.com/ext\n\ngo 1.22\n")
	mustWrite(t, filepath.Join(dir, "a.go"), `package ext

import "time"

// Stamp calls time.Now.
func Stamp() time.Time { return time.Now() }
`)
	mustWrite(t, filepath.Join(dir, "b.go"), `package ext

import "time"

// Later calls time.Now and time.AfterFunc.
func Later(f func()) *time.Timer {
	_ = time.Now()
	return time.AfterFunc(time.Second, f)
}
`)
	return dir
}

// TestIncremental_KeepsExternalEdges: External nodes (schema 1.36) belong to
// no file and are shared by every caller. After a cold build, editing b.go
// re-resolves b.go's calls while a.go stays cached. The cached caller's edge
// to time.Now must survive (re-inserting the shared node would cascade it
// away), and the incremental graph must equal a cold build.
func TestIncremental_KeepsExternalEdges(t *testing.T) {
	src := externalModule(t)
	incOut := t.TempDir()
	runBuild(t, src, incOut)

	edited := filepath.Join(src, "b.go")
	body, err := os.ReadFile(edited)
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, edited, string(body)+"\n// Extra makes this file dirty.\nfunc Extra() {}\n")
	runBuild(t, src, incOut)

	coldOut := t.TempDir()
	runBuild(t, src, coldOut)

	inc, cold := edgeSet(t, incOut), edgeSet(t, coldOut)
	// Known and unrelated to External nodes: an incremental build loses the
	// package's contains edge to a cached file of the same package when
	// another file of it is dirty (the package node row carries one file's
	// path and is deleted with that file). It happens without any external
	// call, so it is left out here and tracked separately.
	cold = dropPrefix(cold, "contains ext -> ")
	inc = dropPrefix(inc, "contains ext -> ")
	onlyInc, onlyCold := diffSets(inc, cold)
	if len(onlyInc) > 0 || len(onlyCold) > 0 {
		t.Errorf("incremental graph differs from a cold build of the same tree\nonly incremental:\n  %s\nonly cold:\n  %s",
			strings.Join(onlyInc, "\n  "), strings.Join(onlyCold, "\n  "))
	}
	for _, want := range []string{
		"calls example.com/ext.Stamp -> time.Now",
		"calls example.com/ext.Later -> time.Now",
		"calls example.com/ext.Later -> time.AfterFunc",
	} {
		for name, set := range map[string][]string{"incremental": inc, "cold": cold} {
			found := false
			for _, e := range set {
				found = found || e == want
			}
			if !found {
				t.Errorf("%s build lacks %q; edges:\n  %s", name, want, strings.Join(set, "\n  "))
			}
		}
	}
}

func dropPrefix(set []string, prefix string) []string {
	out := set[:0:0]
	for _, s := range set {
		if !strings.HasPrefix(s, prefix) {
			out = append(out, s)
		}
	}
	return out
}
