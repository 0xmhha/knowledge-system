package buildpipe_test

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/graph/persist"
)

// sameNameModule writes a module with two packages that share the short
// name x. Package use calls one/x.Do directly and inside a goroutine literal,
// and refers to one/x.T, so a resolver that falls back to the short name
// x.Do can bind those references to two/x instead.
func sameNameModule(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, d := range []string{"one/x", "two/x", "use"} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite(t, filepath.Join(dir, "go.mod"), "module example.com/samename\n\ngo 1.22\n")
	mustWrite(t, filepath.Join(dir, "one", "x", "x.go"), `package x

// Do is the function use.Call calls.
func Do() {}

// T is the type use.Take refers to.
type T struct{}

// A and B have methods with the same short name.
type A struct{}

// Name is A's name.
func (A) Name() string { return "a" }

// B is the second type with a Name method.
type B struct{}

// Name is B's name.
func (B) Name() string { return "b" }
`)
	mustWrite(t, filepath.Join(dir, "two", "x", "x.go"), `package x

// Do has the same short name as one/x.Do.
func Do() {}

// T has the same short name as one/x.T.
type T struct{}
`)
	mustWrite(t, filepath.Join(dir, "use", "use.go"), `package use

import (
	"example.com/samename/one/x"
	y "example.com/samename/two/x"
)

// Call calls one/x.Do.
func Call() { x.Do() }

// Spawn calls one/x.Do inside a goroutine literal.
func Spawn() {
	go func() {
		x.Do()
	}()
}

// Take refers to one/x.T.
func Take(t x.T) x.T { return t }

// Both calls two methods named Name on one line.
func Both(a x.A, b x.B) string { return a.Name() + b.Name() }

// Pair calls both Do functions on one line; both refs store the target
// name x.Do and differ only in their canonical target.
func Pair() { x.Do(); y.Do() }
`)
	return dir
}

// edgeSet renders every edge as "type src -> dst" using canonical ids (or
// qualified names when a node has none), sorted, so two builds of the same
// source compare as text.
func edgeSet(t *testing.T, out string) []string {
	t.Helper()
	st, err := persist.OpenReadOnly(filepath.Join(out, "graph.db"))
	if err != nil {
		t.Fatalf("open graph.db: %v", err)
	}
	defer func() { _ = st.Close() }()
	nodes, err := st.AllNodes()
	if err != nil {
		t.Fatalf("AllNodes: %v", err)
	}
	name := make(map[string]string, len(nodes))
	for _, n := range nodes {
		id := n.CanonicalID
		if id == "" {
			id = n.QualifiedName
		}
		name[n.ID] = id
	}
	edges, err := st.AllEdges()
	if err != nil {
		t.Fatalf("AllEdges: %v", err)
	}
	out2 := make([]string, 0, len(edges))
	for _, e := range edges {
		out2 = append(out2, fmt.Sprintf("%s %s -> %s", e.Type, name[e.Src], name[e.Dst]))
	}
	sort.Strings(out2)
	return out2
}

// diffSets returns the lines only in a and only in b.
func diffSets(a, b []string) (onlyA, onlyB []string) {
	count := map[string]int{}
	for _, s := range a {
		count[s]++
	}
	for _, s := range b {
		count[s]--
	}
	for s, c := range count {
		for ; c > 0; c-- {
			onlyA = append(onlyA, s)
		}
		for ; c < 0; c++ {
			onlyB = append(onlyB, s)
		}
	}
	sort.Strings(onlyA)
	sort.Strings(onlyB)
	return onlyA, onlyB
}

// TestIncremental_KeepsCanonicalTargets: after a cold build, editing one/x
// makes use.go reverse-dirty, so its pending refs are reloaded from the
// pending_refs table and resolved again. The incremental graph must equal a
// cold build of the edited tree. If the reloaded refs lose their canonical
// target, the calls to one/x.Do resolve by short name or are dropped.
func TestIncremental_KeepsCanonicalTargets(t *testing.T) {
	src := sameNameModule(t)
	incOut := t.TempDir()
	runBuild(t, src, incOut)

	edited := filepath.Join(src, "one", "x", "x.go")
	body, err := os.ReadFile(edited)
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, edited, string(body)+"\n// Extra makes this file dirty.\nfunc Extra() {}\n")
	runBuild(t, src, incOut)

	coldOut := t.TempDir()
	runBuild(t, src, coldOut)

	inc, cold := edgeSet(t, incOut), edgeSet(t, coldOut)
	onlyInc, onlyCold := diffSets(inc, cold)
	if len(onlyInc) > 0 || len(onlyCold) > 0 {
		t.Errorf("incremental graph differs from a cold build of the same tree\nonly incremental:\n  %s\nonly cold:\n  %s",
			strings.Join(onlyInc, "\n  "), strings.Join(onlyCold, "\n  "))
	}
	// Guard the fixture itself: the cold build must hold the edges the
	// incremental build used to lose.
	for _, want := range []string{
		"calls example.com/samename/use.Call -> example.com/samename/one/x.Do",
		"calls example.com/samename/use.Spawn -> example.com/samename/one/x.Do",
		"calls example.com/samename/use.Both -> example.com/samename/one/x.(A).Name",
		"calls example.com/samename/use.Both -> example.com/samename/one/x.(B).Name",
		"calls example.com/samename/use.Pair -> example.com/samename/one/x.Do",
		"calls example.com/samename/use.Pair -> example.com/samename/two/x.Do",
	} {
		found := false
		for _, e := range cold {
			found = found || e == want
		}
		if !found {
			t.Errorf("fixture: cold build lacks %q; edges:\n  %s", want, strings.Join(cold, "\n  "))
		}
	}
}
