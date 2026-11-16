package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/0xmhha/knowledge-system/internal/buildscope"
	"github.com/0xmhha/knowledge-system/internal/graph/detect"
	"github.com/0xmhha/knowledge-system/internal/graph/filterlist"
)

// loadScope reads a build scope written by `ckg scope` (or cmd/buildscope)
// and returns the Go build configuration and the file filter it implies:
// exactly the files the scope lists, of every language. Files the scope does
// not list are not read; a build without a scope reads every file.
func loadScope(path string) (*buildscope.Scope, *detect.GoBuild, *filterlist.FilterList, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("read scope: %w", err)
	}
	var s buildscope.Scope
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, nil, nil, fmt.Errorf("parse scope %s: %w", path, err)
	}
	if s.SchemaVersion != buildscope.SchemaVersion {
		return nil, nil, nil, fmt.Errorf("scope %s: schema version %d, want %d; extract it again with ckg scope", path, s.SchemaVersion, buildscope.SchemaVersion)
	}
	b := &detect.GoBuild{GOOS: s.Target.GOOS, GOARCH: s.Target.GOARCH, CGOEnabled: s.Target.CGOEnabled, Tags: s.Target.Tags}
	for _, m := range s.OutsideModules {
		b.Modules = append(b.Modules, detect.ModuleUse{Path: m.Path, Version: m.Version, Dir: m.Dir})
	}
	fl := &filterlist.FilterList{}
	n := 0
	for _, f := range s.Files {
		fl.Include = append(fl.Include, f.Path)
		if f.Language == "go" {
			n++
		}
	}
	if n == 0 {
		return nil, nil, nil, fmt.Errorf("scope %s lists no Go files", path)
	}
	return &s, b, fl, nil
}
