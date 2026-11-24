package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// wrapSeparator joins a wrapped error to the error inside it.
const wrapSeparator = ": "

var (
	formatVerb = regexp.MustCompile(`%[-+# 0-9.*\[\]]*[a-zA-Z]`)
	goLiteral  = regexp.MustCompile("\"(?:[^\"\\\\]|\\\\.)*\"|`[^`]*`")
)

// position is one source line.
type position struct {
	file string
	line int
}

// fragments returns the fixed text of each wrapped part of a message, outermost
// first: the longest piece between format verbs.
func fragments(message string) []string {
	var out []string
	for _, part := range strings.Split(message, wrapSeparator) {
		best := ""
		for _, piece := range formatVerb.Split(part, -1) {
			if piece = strings.TrimSpace(piece); len(piece) > len(best) {
				best = piece
			}
		}
		if best != "" {
			out = append(out, best)
		}
	}
	return out
}

// literalLines returns the lines of non-test Go files under root with a string
// literal that holds the fragment, in path order.
func literalLines(root, fragment string) ([]position, error) {
	var out []position
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, testSuffix) {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return fmt.Errorf("relative path of %s: %w", path, err)
		}
		for i, line := range strings.Split(string(src), "\n") {
			if strings.Contains(line, fragment) && inLiteral(line, fragment) {
				out = append(out, position{filepath.ToSlash(rel), i + 1})
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("search %s: %w", root, err)
	}
	return out, nil
}

func inLiteral(line, fragment string) bool {
	for _, lit := range goLiteral.FindAllString(line, -1) {
		if strings.Contains(lit, fragment) {
			return true
		}
	}
	return false
}
