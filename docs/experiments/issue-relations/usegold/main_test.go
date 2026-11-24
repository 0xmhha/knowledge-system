package main

import (
	"os"
	"path/filepath"
	"testing"
)

const fixture = `package a

type box struct{ size int }

const limit = 3

func (b *box) grow() { b.size++ }

func use(b *box) int {
	b.grow()
	// limit in a comment is not a use
	return b.size + limit
}
`

func TestCollect(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"go.mod": "module a\n\ngo 1.22\n", "a.go": fixture, "a_test.go": "package a\n\nvar _ = limit\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), outMode); err != nil {
			t.Fatal(err)
		}
	}
	syms, err := collect(dir, map[string]bool{"a.go": true}, "")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int{"field box size": 2, "const  limit": 1, "method box grow": 1, "type  box": 2}
	got := map[string]int{}
	for _, s := range syms {
		got[s.Kind+" "+s.Recv+" "+s.Name] = len(s.Uses)
	}
	for k, n := range want {
		if got[k] != n {
			t.Errorf("%s: %d use lines, want %d (all: %v)", k, got[k], n, got)
		}
	}
}

func TestScopeFilesMissing(t *testing.T) {
	if _, err := scopeFiles(filepath.Join(t.TempDir(), "none.json")); err == nil {
		t.Error("want an error for a missing scope file")
	}
}
