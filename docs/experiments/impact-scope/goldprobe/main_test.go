package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

const fixture = `package p

type T struct {
	A int
	B int
}

type D T

func readsDerived(d D) int { return d.A }

func readsA(t T) int { return t.A }

func writesA(t *T) { t.A = 1 }

func readsB(t T) int { return t.B }

func makes() T { return T{B: 1} }

func makesBoth() T { return T{A: 1, B: 2} }

func helper() {}

func usesHelper() { helper() }

func storesHelper() func() { return helper }
`

func TestGold(t *testing.T) {
	dir := t.TempDir()
	write := func(name, text string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.com/p\n\ngo 1.23\n")
	write("p.go", fixture)
	write("scope.json", `{"files":[{"path":"p.go"}]}`)
	out := filepath.Join(dir, "gold.json")
	if err := run([]string{"-src", dir, "-scope", filepath.Join(dir, "scope.json"), "-out", out}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var res Output
	if err := json.Unmarshal(b, &res); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		kind  Kind
		qname string
		want  []string
	}{
		{KindField, "p.T.A", []string{"makesBoth", "readsA", "readsDerived", "writesA"}},
		{KindField, "p.D.A", nil},
		{KindOmit, "p.T.A", []string{"makes"}},
		{KindFunc, "p.helper", []string{"storesHelper", "usesHelper"}},
	}
	for _, tt := range tests {
		t.Run(string(tt.kind), func(t *testing.T) {
			var got []string
			for _, x := range res.Targets {
				if x.Kind == tt.kind && x.QName == tt.qname {
					for _, d := range x.Gold {
						got = append(got, d.Name)
					}
				}
			}
			slices.Sort(got)
			if !slices.Equal(got, tt.want) {
				t.Fatalf("gold = %v, want %v", got, tt.want)
			}
		})
	}
}
