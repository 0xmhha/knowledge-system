package detect

import (
	"testing"

	"golang.org/x/tools/go/packages"
)

// TestSummarizeLoad: errors of dependencies count, each package once even
// when two roots import it, with bounded samples.
func TestSummarizeLoad(t *testing.T) {
	dep := &packages.Package{PkgPath: "x/dep", Errors: []packages.Error{{Msg: "no required module", Kind: packages.ListError}}}
	a := &packages.Package{PkgPath: "x/a", Imports: map[string]*packages.Package{"x/dep": dep},
		Errors: []packages.Error{{Msg: "undefined: y", Kind: packages.TypeError}, {Msg: "mismatch", Kind: packages.TypeError}}}
	b := &packages.Package{PkgPath: "x/b", Imports: map[string]*packages.Package{"x/dep": dep}}
	p := SummarizeLoad([]*packages.Package{a, b}, 2)
	if p.Packages != 3 || p.WithErrors != 2 || p.Errors() != 3 || p.ByKind["type"] != 2 || p.ByKind["list"] != 1 {
		t.Errorf("summary %+v", p)
	}
	if len(p.Samples) != 2 {
		t.Errorf("samples %v, want 2", p.Samples)
	}
	if clean := SummarizeLoad([]*packages.Package{b}, 5); clean.Errors() != 1 || clean.Packages != 2 {
		t.Errorf("b reaches the failing dep: %+v", clean)
	}
}

func TestCheckModules(t *testing.T) {
	mod := func(path, version, dir string) *packages.Package {
		return &packages.Package{PkgPath: path, Module: &packages.Module{Path: path, Version: version, Dir: dir}}
	}
	main := &packages.Package{PkgPath: "me", Module: &packages.Module{Path: "me", Main: true},
		Imports: map[string]*packages.Package{
			"same":  mod("same", "v1.0.0", "/cache/same@v1.0.0"),
			"newer": mod("newer", "v1.2.0", "/cache/newer@v1.2.0"),
			"moved": mod("moved", "v1.0.0", "/other/moved@v1.0.0"),
		}}
	c := CheckModules([]*packages.Package{main}, []ModuleUse{
		{Path: "same", Version: "v1.0.0", Dir: "/cache/same@v1.0.0"},
		{Path: "newer", Version: "v1.1.0", Dir: "/cache/newer@v1.1.0"},
		{Path: "moved", Version: "v1.0.0", Dir: "/cache/moved@v1.0.0"},
		{Path: "gone", Version: "v1.0.0"},
	})
	if c.Matched != 1 || len(c.OtherVersion) != 1 || len(c.OtherDir) != 1 || len(c.NotLoaded) != 1 || c.NotLoaded[0] != "gone" {
		t.Errorf("check %+v", c)
	}
}
