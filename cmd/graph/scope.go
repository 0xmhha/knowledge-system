package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/0xmhha/knowledge-system/internal/buildscope"
	"github.com/0xmhha/knowledge-system/internal/graph/detect"
	"github.com/0xmhha/knowledge-system/internal/graph/filterlist"
)

// nonGoGlobs keep the languages that have no build-scope extractor yet: until
// they do, their files are discovered as before.
var nonGoGlobs = []string{
	"**/*.sol", "**/*.ts", "**/*.tsx", "**/*.mts", "**/*.cts",
	"**/*.js", "**/*.jsx", "**/*.mjs", "**/*.cjs", "**/*.proto",
}

// loadScope reads a build scope written by cmd/buildscope and returns the Go
// build configuration and the file filter it implies: the Go files the scope
// lists (production and test), plus every file of the other languages.
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
		return nil, nil, nil, fmt.Errorf("scope %s: schema version %d, want %d", path, s.SchemaVersion, buildscope.SchemaVersion)
	}
	b := &detect.GoBuild{GOOS: s.Target.GOOS, GOARCH: s.Target.GOARCH, CGOEnabled: s.Target.CGOEnabled, Tags: s.Target.Tags}
	fl := &filterlist.FilterList{Include: append([]string(nil), nonGoGlobs...)}
	n := 0
	for _, f := range s.Files {
		if f.Language == "go" {
			fl.Include = append(fl.Include, f.Path)
			n++
		}
	}
	if n == 0 {
		return nil, nil, nil, fmt.Errorf("scope %s lists no Go files", path)
	}
	return &s, b, fl, nil
}
