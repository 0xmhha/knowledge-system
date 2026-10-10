package knowledgepack

import (
	"fmt"
	"sort"
	"strings"
)

// DeclaredUnknown records that a project has explicitly not decided or not
// recorded a fact. It lets a retrieval answer "not decided" from reviewed
// project evidence instead of citing unrelated material as if it answered.
//
// Match terms are reviewed project data, not evaluation gold: a request
// matches when every term of at least one group appears in the query. The
// Basis spans are the archived lines that state the fact is undecided.
type DeclaredUnknown struct {
	ID           string            `yaml:"id" json:"id"`
	Statement    string            `yaml:"statement" json:"statement"`
	Scope        map[string]string `yaml:"scope" json:"scope"`
	RecordedOn   string            `yaml:"recorded_on" json:"recorded_on"`
	ResolvedOn   string            `yaml:"resolved_on" json:"resolved_on"`
	MatchAny     [][]string        `yaml:"match_any" json:"match_any"`
	Basis        []BasisSpan       `yaml:"basis" json:"basis"`
	Status       string            `yaml:"status" json:"status"`
	ReviewedBy   string            `yaml:"reviewed_by" json:"reviewed_by"`
	ReviewReason string            `yaml:"review_reason" json:"review_reason"`
	Visibility   string            `yaml:"visibility" json:"visibility"`
	SourceRef    SourceRef         `yaml:"source_ref" json:"source_ref"`
}

// BasisSpan is an inclusive 1-based line range in an archived repository file.
type BasisSpan struct {
	Path      string `yaml:"path" json:"path"`
	StartLine int    `yaml:"start_line" json:"start_line"`
	EndLine   int    `yaml:"end_line" json:"end_line"`
}

// UnknownSummary is one declared unknown selected for a request.
type UnknownSummary struct {
	ID         string      `json:"id"`
	ReviewedBy string      `json:"reviewed_by"`
	RecordedOn string      `json:"recorded_on"`
	Basis      []BasisSpan `json:"basis"`
}

const (
	maxUnknownGroups     = 16
	maxUnknownGroupTerms = 6
	maxUnknownBasis      = 4
)

func validateDeclaredUnknown(u DeclaredUnknown, path string) error {
	if !instanceIDPattern.MatchString(u.ID) || strings.TrimSpace(u.Statement) == "" || len(u.Scope) == 0 ||
		!validSource(u.SourceRef, path) || !validReview(u.Status, u.ReviewedBy, u.ReviewReason) ||
		(u.Visibility != "public" && u.Visibility != "restricted") {
		return fmt.Errorf("evidence_unverified: invalid declared unknown %q", path)
	}
	for key, value := range u.Scope {
		if !instanceIDPattern.MatchString(key) || strings.TrimSpace(value) == "" || len(value) > 128 {
			return fmt.Errorf("evidence_unverified: invalid declared unknown scope")
		}
	}
	if !validDate(u.RecordedOn) || u.ResolvedOn != "" && (!validDate(u.ResolvedOn) || u.ResolvedOn <= u.RecordedOn) {
		return fmt.Errorf("evidence_unverified: invalid declared unknown dates")
	}
	if len(u.MatchAny) == 0 || len(u.MatchAny) > maxUnknownGroups {
		return fmt.Errorf("evidence_unverified: declared unknown needs 1..%d match groups", maxUnknownGroups)
	}
	for _, group := range u.MatchAny {
		if len(group) == 0 || len(group) > maxUnknownGroupTerms {
			return fmt.Errorf("evidence_unverified: declared unknown match group needs 1..%d terms", maxUnknownGroupTerms)
		}
		for _, term := range group {
			// Single-character terms would match almost any query.
			if t := strings.TrimSpace(term); t != term || len([]rune(t)) < 2 || len(t) > 64 {
				return fmt.Errorf("evidence_unverified: invalid declared unknown match term")
			}
		}
	}
	if len(u.Basis) == 0 || len(u.Basis) > maxUnknownBasis {
		return fmt.Errorf("evidence_unverified: declared unknown needs 1..%d basis spans", maxUnknownBasis)
	}
	for _, b := range u.Basis {
		if b.Path == "" || strings.HasPrefix(b.Path, "/") || strings.Contains(b.Path, "..") || strings.Contains(b.Path, "\\") ||
			b.StartLine < 1 || b.EndLine < b.StartLine {
			return fmt.Errorf("evidence_unverified: invalid declared unknown basis span")
		}
	}
	return nil
}

// SelectDeclaredUnknowns returns reviewed, public, in-scope unknowns that were
// recorded on or before asOf, not yet resolved, and whose match terms occur in
// the query. Restricted matches are withheld and reported as restricted.
func (i Instances) SelectDeclaredUnknowns(asOf string, queryScope map[string]string, query string) ([]UnknownSummary, bool, error) {
	if !validDate(asOf) {
		return nil, false, fmt.Errorf("invalid declared unknown query date")
	}
	folded := strings.ToLower(query)
	out := []UnknownSummary{}
	restricted := false
	for _, u := range i.Unknowns {
		if u.Status != "verified" || !policyScopeMatches(u.Scope, queryScope) ||
			asOf < u.RecordedOn || u.ResolvedOn != "" && asOf >= u.ResolvedOn || !unknownMatches(u.MatchAny, folded) {
			continue
		}
		if u.Visibility != "public" {
			restricted = true
			continue
		}
		out = append(out, UnknownSummary{ID: u.ID, ReviewedBy: u.ReviewedBy, RecordedOn: u.RecordedOn,
			Basis: append([]BasisSpan{}, u.Basis...)})
	}
	sort.Slice(out, func(a, b int) bool { return out[a].ID < out[b].ID })
	if restricted {
		return nil, true, nil
	}
	return out, false, nil
}

func unknownMatches(groups [][]string, foldedQuery string) bool {
	for _, group := range groups {
		all := true
		for _, term := range group {
			if !strings.Contains(foldedQuery, strings.ToLower(term)) {
				all = false
				break
			}
		}
		if all {
			return true
		}
	}
	return false
}
