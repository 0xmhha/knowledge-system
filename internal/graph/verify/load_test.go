package verify

import (
	"bytes"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/graph/detect"
	"golang.org/x/tools/go/packages"
)

// TestRunGo_ReportsLoadErrors: graph and truth come from the same go command,
// so a package it cannot load is missing from both and the comparison agrees.
// The report must carry the load errors so that agreement is not read as a
// clean graph. The fixture loads cleanly; one error is injected.
func TestRunGo_ReportsLoadErrors(t *testing.T) {
	r, err := RunGo(fixtureRoot, fixtureGraph(t), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if r.Load.Errors() != 0 || r.Load.Packages == 0 {
		t.Fatalf("clean fixture: load %+v", r.Load)
	}

	orig := loadPackages
	t.Cleanup(func() { loadPackages = orig })
	loadPackages = func(root string, b *detect.GoBuild) ([]*packages.Package, error) {
		pkgs, err := orig(root, b)
		if err == nil && len(pkgs) > 0 {
			pkgs[0].Errors = append(pkgs[0].Errors, packages.Error{Msg: "could not import example.com/gone", Kind: packages.TypeError})
		}
		return pkgs, err
	}
	r, err = RunGo(fixtureRoot, fixtureGraph(t), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if r.Load.Errors() != 1 || r.Load.ByKind["type"] != 1 {
		t.Errorf("load %+v, want one type error", r.Load)
	}
	var out bytes.Buffer
	if err := r.WriteText(&out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "go load errors") || !strings.Contains(out.String(), "example.com/gone") {
		t.Errorf("text report does not show the load error:\n%s", out.String())
	}
}
