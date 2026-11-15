// Package buildscope extracts, from a project's build settings, the files one
// build configuration compiles, the test files that belong to them, the
// toolchain it uses and the modules it pulls from outside the repository.
//
// It is the build-scope extractor of docs/graph/SYMBOL-CONTRACT-DESIGN.md
// (section 3). The graph only reads the files a scope lists, so declarations
// from other build tags or platforms never enter it. The package imports
// nothing from the graph, vector or system engines so that it can move to its
// own repository; its output is a versioned JSON document.
package buildscope

import "sort"

// SchemaVersion is the version of the Scope JSON document.
const SchemaVersion = 1

// File roles.
const (
	RoleProduction = "production" // compiled into the configuration's outputs
	RoleTest       = "test"       // test file of a package in the configuration
)

// Scope is everything one build configuration compiles.
type Scope struct {
	SchemaVersion int    `json:"schema_version"`
	ConfigID      string `json:"config_id"`
	// Root is the module or project root the paths below are relative to.
	Root        string      `json:"root"`
	Toolchains  []Toolchain `json:"toolchains"`
	Target      Target      `json:"target"`
	EntryPoints []string    `json:"entry_points"`
	// Files are compiled by the configuration (production) or belong to one of
	// its packages as tests (test). Paths are slash-separated, relative to Root.
	Files []File `json:"files"`
	// Ignored lists files in the configuration's packages that its build
	// constraints exclude, so a caller can tell "excluded" from "not seen".
	Ignored        []File   `json:"ignored,omitempty"`
	OutsideModules []Module `json:"outside_modules,omitempty"`
	// Problems are load errors the build tool reported; a scope with problems
	// may be incomplete.
	Problems []string `json:"problems,omitempty"`
}

// Toolchain records the tool a language is built with.
type Toolchain struct {
	Language string `json:"language"`
	// Declared is the version the project asks for (go.mod go and toolchain
	// lines for Go), empty when the project declares none.
	Declared  string `json:"declared,omitempty"`
	Installed string `json:"installed"`
	// Path and SHA256 identify the tool binary that produced the scope.
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// Target is the platform and options of a configuration.
type Target struct {
	GOOS   string   `json:"goos,omitempty"`
	GOARCH string   `json:"goarch,omitempty"`
	Tags   []string `json:"tags,omitempty"`
}

// File is one source file of the scope.
type File struct {
	Path     string `json:"path"`
	Language string `json:"language"`
	Role     string `json:"role"`
	// Package is the language-level unit the file belongs to (the Go import
	// path; external test files carry the "_test" suffix).
	Package string `json:"package"`
}

// Module is a module the configuration uses from outside the repository.
type Module struct {
	Path    string `json:"path"`
	Version string `json:"version,omitempty"`
	// Dir is where its source was found (module cache or a replace target).
	Dir string `json:"dir,omitempty"`
	// Replace is the replacement path or module when go.mod replaces it.
	Replace string `json:"replace,omitempty"`
}

// sortScope orders every list so the same build gives byte-identical output.
func sortScope(s *Scope) {
	byPath := func(fs []File) {
		sort.Slice(fs, func(i, j int) bool {
			if fs[i].Path != fs[j].Path {
				return fs[i].Path < fs[j].Path
			}
			return fs[i].Role < fs[j].Role
		})
	}
	byPath(s.Files)
	byPath(s.Ignored)
	sort.Slice(s.OutsideModules, func(i, j int) bool { return s.OutsideModules[i].Path < s.OutsideModules[j].Path })
	sort.Strings(s.EntryPoints)
	sort.Strings(s.Problems)
}
