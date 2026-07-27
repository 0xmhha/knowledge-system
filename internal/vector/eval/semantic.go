package eval

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

// semanticFixture is the JSON semantic-validation format consumed by
// vector/scripts/build-knowledge.sh. Each query PASSes when any of the
// top-k cited files' paths CONTAIN the expect substring. It is mapped
// onto the shared Fixture with MatchMode == MatchSubstring so the same
// eval.Run / Score path scores it.
//
//	{
//	  "k": 10,
//	  "queries": [
//	    {"query": "<natural-language paraphrase>",
//	     "expect": "<expected file path substring>",
//	     "note": "..."}
//	  ]
//	}
type semanticFixture struct {
	K       int             `json:"k"`
	Queries []semanticQuery `json:"queries"`
}

type semanticQuery struct {
	Query  string `json:"query"`
	Expect string `json:"expect"`
	Note   string `json:"note"`
}

// isSemanticJSON reports whether path/data is the JSON semantic-validation
// format. Detection is by .json extension or a leading '{' — a YAML
// fixture always starts with the "schema_version:" key, never a brace, so
// the sniff cannot misclassify the existing YAML fixtures.
func isSemanticJSON(path string, data []byte) bool {
	if strings.EqualFold(filepath.Ext(path), ".json") {
		return true
	}
	return bytes.HasPrefix(bytes.TrimSpace(data), []byte("{"))
}

// loadSemanticFixture parses the JSON semantic-validation format and maps
// it onto a Fixture with substring match mode. Query IDs are synthesized
// as q1, q2, … in file order. Validation requires a non-empty query set
// and, per entry, both a query and an expect substring.
func loadSemanticFixture(data []byte) (*Fixture, error) {
	var sf semanticFixture
	// Unknown top-level keys (e.g. the real fixture's "_comment") are
	// tolerated — only the k/queries shape matters here.
	if err := json.Unmarshal(data, &sf); err != nil {
		return nil, fmt.Errorf("parse semantic fixture: %w", err)
	}
	if len(sf.Queries) == 0 {
		return nil, fmt.Errorf("semantic fixture: no queries")
	}
	f := &Fixture{
		SchemaVersion: FixtureSchemaVersion,
		MatchMode:     MatchSubstring,
		K:             sf.K,
		Queries:       make([]Query, 0, len(sf.Queries)),
	}
	for i, q := range sf.Queries {
		if q.Query == "" {
			return nil, fmt.Errorf("semantic fixture: query %d missing query text", i)
		}
		if q.Expect == "" {
			return nil, fmt.Errorf("semantic fixture: query %d (%q) missing expect", i, q.Query)
		}
		f.Queries = append(f.Queries, Query{
			ID:       fmt.Sprintf("q%d", i+1),
			Intent:   q.Query,
			Notes:    q.Note,
			Expected: Expected{Substring: q.Expect},
		})
	}
	return f, nil
}
