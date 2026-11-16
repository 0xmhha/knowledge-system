package detect

import (
	"fmt"
	"sort"

	"golang.org/x/tools/go/packages"
)

// ModuleUse is a module outside the repository as a build scope records it:
// the version the configuration uses and the directory its source was read
// from when the scope was made.
type ModuleUse struct {
	Path, Version, Dir string
}

// LoadProblems summarises what went wrong when the go command loaded and
// type-checked packages. A package that fails to load or type-check still
// yields a graph, but references into it lose their targets, so the build
// and the verifier report these instead of dropping them.
type LoadProblems struct {
	// Packages is the number of packages reached, roots and dependencies.
	Packages int `json:"packages"`
	// WithErrors is the number of those packages that carry errors.
	WithErrors int `json:"packages_with_errors"`
	// ByKind counts errors by kind: list (the go command could not list or
	// find the package), parse, type, unknown.
	ByKind map[string]int `json:"errors_by_kind,omitempty"`
	// Samples are the first errors, "package: message".
	Samples []string `json:"samples,omitempty"`
}

// Errors is the number of errors of every kind.
func (p LoadProblems) Errors() int {
	n := 0
	for _, c := range p.ByKind {
		n += c
	}
	return n
}

// SummarizeLoad walks the loaded packages and every dependency they reach
// once, and counts their errors by kind with up to limit samples.
func SummarizeLoad(pkgs []*packages.Package, limit int) LoadProblems {
	p := LoadProblems{ByKind: map[string]int{}}
	packages.Visit(pkgs, nil, func(pkg *packages.Package) {
		p.Packages++
		if len(pkg.Errors) == 0 {
			return
		}
		p.WithErrors++
		for _, e := range pkg.Errors {
			p.ByKind[errorKind(e.Kind)]++
			if len(p.Samples) < limit {
				p.Samples = append(p.Samples, pkg.PkgPath+": "+e.Msg)
			}
		}
	})
	return p
}

func errorKind(k packages.ErrorKind) string {
	switch k {
	case packages.ListError:
		return "list"
	case packages.ParseError:
		return "parse"
	case packages.TypeError:
		return "type"
	}
	return "unknown"
}

// ModuleCheck compares the outside modules a build loaded with the ones its
// scope records.
type ModuleCheck struct {
	// Matched modules were loaded at the scope's version from the scope's
	// directory.
	Matched int
	// OtherVersion modules were loaded at another version than the scope's;
	// the graph then describes other code than the scope.
	OtherVersion []string
	// OtherDir modules were loaded at the scope's version from another
	// directory (another module cache); the code is the same when both
	// caches are intact.
	OtherDir []string
	// NotLoaded modules of the scope were not reached by the load.
	NotLoaded []string
}

// CheckModules compares the modules reached from pkgs with want, by the
// version and directory the go command reports (for a replaced module, the
// directory is the replacement's, as in the scope).
func CheckModules(pkgs []*packages.Package, want []ModuleUse) ModuleCheck {
	loaded := map[string]*packages.Module{}
	packages.Visit(pkgs, nil, func(pkg *packages.Package) {
		if m := pkg.Module; m != nil && !m.Main {
			loaded[m.Path] = m
		}
	})
	var c ModuleCheck
	for _, w := range want {
		m, ok := loaded[w.Path]
		switch {
		case !ok:
			c.NotLoaded = append(c.NotLoaded, w.Path)
		case w.Version != "" && m.Version != w.Version:
			c.OtherVersion = append(c.OtherVersion, fmt.Sprintf("%s: scope %s, loaded %s", w.Path, w.Version, m.Version))
		case w.Dir != "" && m.Dir != w.Dir:
			c.OtherDir = append(c.OtherDir, fmt.Sprintf("%s: scope %s, loaded %s", w.Path, w.Dir, m.Dir))
		default:
			c.Matched++
		}
	}
	sort.Strings(c.NotLoaded)
	return c
}
