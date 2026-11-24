package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

const fixture = `package a

type Store interface{ Get(k string) int }

type Reader interface{ Get(k string) int }

type mem struct{}

func (m *mem) Get(k string) int { return leaf(k) }

type disk struct{}

func (d disk) Get(k string) int { return 0 }

type unused struct{}

func (unused) Get(k string) int { return 1 }

func leaf(k string) int { return len(k) }

func NewStore(fast bool) Store {
	if fast {
		return &mem{}
	}
	return disk{}
}

func AsReader() Reader { return &mem{} }

func one(s Store) int { return s.Get("x") }

func two() int { return one(NewStore(true)) }

func three() int { return two() }

func four() int { return three() }

var table = map[string]func() int{"three": three}
`

func loadFixture(t *testing.T) []Target {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"go.mod": "module a\n\ngo 1.22\n", "a.go": fixture} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), outMode); err != nil {
			t.Fatal(err)
		}
	}
	p := newProbe(dir, map[string]bool{"a.go": true})
	if _, err := p.load(""); err != nil {
		t.Fatal(err)
	}
	return p.targets()
}

func TestGold(t *testing.T) {
	targets := loadFixture(t)
	cases := []struct {
		kind  Kind
		qname string
		want  []string
	}{
		{KindSignature, "a.Store.Get", []string{"Reader", "Get", "Get", "one"}},
		{KindReach, "a.leaf", []string{"Get", "one", "two"}},
		{KindReach, "a.one", []string{"two", "three", "four", "table"}},
	}
	for _, c := range cases {
		t.Run(string(c.kind)+" "+c.qname, func(t *testing.T) {
			i := slices.IndexFunc(targets, func(x Target) bool { return x.Kind == c.kind && x.QName == c.qname })
			if i < 0 {
				t.Fatalf("no target %s", c.qname)
			}
			var got []string
			for _, d := range targets[i].Gold {
				got = append(got, d.Name)
			}
			slices.Sort(got)
			want := slices.Sorted(slices.Values(c.want))
			if !slices.Equal(got, want) {
				t.Errorf("gold = %v, want %v", got, want)
			}
		})
	}
}

// The method of a type that is never converted to the interface is tied to it
// by method sets only, so the target must report the gap.
func TestTypeGap(t *testing.T) {
	for _, x := range loadFixture(t) {
		if x.Kind == KindSignature && x.QName == "a.Store.Get" && x.TypeGap == 0 {
			t.Error("TypeGap = 0, want the unused implementation to count")
		}
	}
}

func TestScopeFilesMissing(t *testing.T) {
	if _, err := scopeFiles(filepath.Join(t.TempDir(), "none.json")); err == nil {
		t.Error("want an error for a missing scope file")
	}
}
