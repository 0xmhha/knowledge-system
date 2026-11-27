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

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
)

// SchemaVersion is the version of the Scope JSON document. Version 2 made the
// file list complete for every language: a graph built from a scope reads the
// listed files and nothing else, so non-Go files enter it through the
// profile's extra_globs instead of being discovered.
const SchemaVersion = 2

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
	// Profile identifies the build profile the scope was extracted with; nil
	// when it was extracted from command-line options alone.
	Profile *ProfileRef `json:"profile,omitempty"`
	// Files are compiled by the configuration (production), belong to one of
	// its packages as tests (test), or were added by the profile's
	// extra_globs (production). Paths are slash-separated, relative to Root.
	Files []File `json:"files"`
	// Ignored lists files in the configuration's packages that its build
	// constraints exclude, so a caller can tell "excluded" from "not seen".
	Ignored []File `json:"ignored,omitempty"`
	// EmbedFiles are the main-module files the build embeds (go:embed); the
	// graph does not read them, inclusions are checked against them.
	EmbedFiles []string `json:"embed_files,omitempty"`
	// Inclusions are the profile's exceptions: files outside the build that
	// belong to the scope, with the reason and the link to the build.
	Inclusions     []InclusionRecord `json:"inclusions,omitempty"`
	OutsideModules []Module          `json:"outside_modules,omitempty"`
	// Problems are load errors the build tool reported; a scope with problems
	// may be incomplete, so extraction fails on them unless told otherwise.
	Problems []string `json:"problems,omitempty"`
	// GoStderr is what the go command wrote to standard error while the
	// scope was extracted; it goes into error messages, not the file.
	GoStderr []string `json:"-"`
	// CheckedBinaries are built binaries whose recorded build settings were
	// compared with Target and EntryPoints and matched.
	CheckedBinaries []CheckedBinary `json:"checked_binaries,omitempty"`
}

// ProfileRef names a build profile by path and content.
type ProfileRef struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// CheckedBinary is a binary the scope was checked against.
type CheckedBinary struct {
	Path string `json:"path"`
	// Package is the main package the binary records.
	Package string `json:"package"`
	SHA256  string `json:"sha256"`
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
	GOOS   string `json:"goos,omitempty"`
	GOARCH string `json:"goarch,omitempty"`
	// CGOEnabled is the effective CGO_ENABLED value ("1" or "0").
	CGOEnabled string   `json:"cgo_enabled,omitempty"`
	Tags       []string `json:"tags,omitempty"`
}

// File is one source file of the scope.
type File struct {
	Path     string `json:"path"`
	Language string `json:"language"`
	Role     string `json:"role"`
	// Package is the language-level unit the file belongs to (the Go import
	// path; external test files carry the "_test" suffix).
	Package string `json:"package"`
	// IncludedBy is the id of the profile inclusion that added the file;
	// empty for a file the build itself compiles or tests.
	IncludedBy string `json:"included_by,omitempty"`
}

// InclusionRecord is what one profile inclusion added to the scope.
type InclusionRecord struct {
	ID     string `json:"id"`
	Reason string `json:"reason"`
	Files  int    `json:"files"`
	// LinkedBy names the build outputs that tie the inclusion to the build
	// (embedded files it was checked against); empty when it has none and
	// rests on its reason alone.
	LinkedBy []string `json:"linked_by,omitempty"`
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
	sort.Strings(s.EmbedFiles)
	sort.Slice(s.OutsideModules, func(i, j int) bool { return s.OutsideModules[i].Path < s.OutsideModules[j].Path })
	sort.Strings(s.EntryPoints)
	sort.Strings(s.Problems)
}

// Write stores s as indented JSON at path, through a temporary file and a
// rename so a reader never sees half a scope; "-" writes to standard output.
func Write(path string, s *Scope) error {
	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if path == "-" {
		_, err := os.Stdout.Write(raw)
		return err
	}
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
