package discover

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
)

func validBuildSource(rel string) bool {
	return rel != "" && !strings.ContainsAny(rel, "\\*?[\x00") &&
		!strings.HasPrefix(rel, "/") && !strings.HasPrefix(rel, "../") &&
		path.Clean(rel) == rel && path.Ext(rel) == ".go" &&
		isIgnored(rel, []string{"build/"})
}

// LoadBuildSources reads {schema_version:1, paths:[exact relative Go paths]}.
// These paths bypass only the build/ default, never explicit or safety rules.
func LoadBuildSources(file string) ([]string, error) {
	if file == "" {
		return nil, nil
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var value struct {
		SchemaVersion int      `json:"schema_version"`
		Paths         []string `json:"paths"`
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&value); err != nil {
		return nil, err
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("build sources: trailing JSON")
	}
	if value.SchemaVersion != 1 || len(value.Paths) == 0 {
		return nil, fmt.Errorf("build sources: schema 1 and nonempty paths required")
	}
	seen := make(map[string]bool)
	for _, rel := range value.Paths {
		if !validBuildSource(rel) || seen[rel] {
			return nil, fmt.Errorf("invalid or duplicate build source %q", rel)
		}
		seen[rel] = true
	}
	return value.Paths, nil
}
