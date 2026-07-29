package eval

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

// scenarioVersion is the only supported top-level version. Bumping
// requires a migration path documented in the changelog.
const scenarioVersion = 1

// Scenario is one evaluation case: a vibe prompt plus the ground-truth
// citations the harness expects cks to surface.
//
// YAML shape (v1):
//
//	version: 1
//	name: short-stable-id            # required, kebab-case recommended
//	description: free-form details   # optional, not used by the runner
//	prompt: "find the X handler"     # required, non-empty
//	expected_citations:              # optional but typical
//	  - file: path/to/file.go
//	    start_line: 10
//	    end_line: 30
//	  - file: other.go
//	    start_line: 1
//	    end_line: 5
//	match_mode: overlap              # default "overlap"; or "strict"
//	runs: 1                          # default 1; >1 takes median
type Scenario struct {
	Version           int
	Name              string
	Description       string
	Prompt            string
	Intent            contract.Intent
	ExpectedCitations []contract.Citation
	// Anchors runs parallel to ExpectedCitations: an optional substring
	// that must occur INSIDE the citation's line span in the source tree.
	// Guards against expected-span drift — the chronic failure where code
	// moves and a scenario silently scores 0 against stale line numbers
	// (see docs/dev/archive/2026-07-28-dogfood-zero-recall-diagnosis.md, three
	// separate re-anchoring rounds). Verified by VerifyAnchors when the
	// runner is given a source root; empty string = no anchor declared.
	Anchors   []string
	MatchMode MatchMode
	Runs      int
}

// scenarioWire is the YAML-tagged wire mirror. We don't tag
// contract.Citation directly (it's a public cks type whose JSON tags
// must stay authoritative); the eval package owns its own wire shape
// and translates into contract.Citation at parse time.
type scenarioWire struct {
	Version           int                    `yaml:"version"`
	Name              string                 `yaml:"name"`
	Description       string                 `yaml:"description,omitempty"`
	Prompt            string                 `yaml:"prompt"`
	Intent            string                 `yaml:"intent,omitempty"`
	ExpectedCitations []scenarioCitationWire `yaml:"expected_citations,omitempty"`
	MatchMode         MatchMode              `yaml:"match_mode,omitempty"`
	Runs              int                    `yaml:"runs,omitempty"`
}

type scenarioCitationWire struct {
	File       string `yaml:"file"`
	StartLine  int    `yaml:"start_line"`
	EndLine    int    `yaml:"end_line"`
	CommitHash string `yaml:"commit_hash,omitempty"`
	// Anchor is a substring that must appear within [start_line, end_line]
	// of File — typically the symbol declaration the span is about
	// (e.g. "func Register("). See Scenario.Anchors.
	Anchor string `yaml:"anchor,omitempty"`
}

// ParseScenario loads YAML bytes into a validated Scenario, applying
// defaults (MatchMode=overlap, Runs=1) when omitted.
func ParseScenario(data []byte) (*Scenario, error) {
	var w scenarioWire
	dec := yaml.NewDecoder(strings.NewReader(string(data)))
	dec.KnownFields(true) // reject typos / unknown keys
	if err := dec.Decode(&w); err != nil {
		return nil, fmt.Errorf("scenario: decode: %w", err)
	}
	s := &Scenario{
		Version:     w.Version,
		Name:        w.Name,
		Description: w.Description,
		Prompt:      w.Prompt,
		Intent:      contract.Intent(w.Intent),
		MatchMode:   w.MatchMode,
		Runs:        w.Runs,
	}
	if len(w.ExpectedCitations) > 0 {
		s.ExpectedCitations = make([]contract.Citation, len(w.ExpectedCitations))
		s.Anchors = make([]string, len(w.ExpectedCitations))
		for i, c := range w.ExpectedCitations {
			s.ExpectedCitations[i] = contract.Citation{
				File:       c.File,
				StartLine:  c.StartLine,
				EndLine:    c.EndLine,
				CommitHash: c.CommitHash,
			}
			s.Anchors[i] = c.Anchor
		}
	}
	applyDefaults(s)
	if err := s.Validate(); err != nil {
		return nil, err
	}
	return s, nil
}

// LoadScenario reads a scenario YAML from path.
func LoadScenario(path string) (*Scenario, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("scenario: read %q: %w", path, err)
	}
	return ParseScenario(data)
}

func applyDefaults(s *Scenario) {
	if s.MatchMode == "" {
		s.MatchMode = MatchOverlap
	}
	if s.Runs == 0 {
		s.Runs = 1
	}
}

// Validate reports the first structural problem in s.
func (s *Scenario) Validate() error {
	if s == nil {
		return errors.New("scenario: nil")
	}
	if s.Version != scenarioVersion {
		return fmt.Errorf("scenario: version=%d, want %d", s.Version, scenarioVersion)
	}
	if strings.TrimSpace(s.Name) == "" {
		return errors.New("scenario: name is required")
	}
	if strings.TrimSpace(s.Prompt) == "" {
		return errors.New("scenario: prompt is required")
	}
	if !s.MatchMode.IsValid() {
		return fmt.Errorf("scenario: match_mode=%q invalid (overlap|strict)", s.MatchMode)
	}
	// Intent is optional, but if set must be a known contract.Intent.
	// Empty string is contract.IntentUnknown which is also a valid
	// value — but for scenario authoring we treat "" as "intent omitted"
	// rather than "explicitly unknown". The distinction does not affect
	// the runner; it only matters for per-intent breakdown grouping.
	if s.Intent != "" && !s.Intent.IsValid() {
		return fmt.Errorf("scenario: intent=%q invalid", s.Intent)
	}
	if s.Runs < 1 {
		return fmt.Errorf("scenario: runs=%d invalid (must be >= 1)", s.Runs)
	}
	for i, c := range s.ExpectedCitations {
		if !c.IsValid() {
			return fmt.Errorf("scenario: expected_citations[%d] invalid: %+v", i, c)
		}
	}
	return nil
}
