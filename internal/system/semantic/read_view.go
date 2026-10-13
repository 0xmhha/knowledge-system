package semantic

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Read views preserve proposed/rejected status and do not imply acceptance.
type TermReadView struct {
	Snapshot        Snapshot           `json:"snapshot"`
	Language        string             `json:"language"`
	Term            string             `json:"term"`
	Candidates      []ConceptCandidate `json:"candidates"`
	Evidence        []EvidenceSpan     `json:"evidence"`
	TotalCandidates int                `json:"total_candidates"`
	Ambiguous       bool               `json:"ambiguous"`
	Truncated       bool               `json:"truncated"`
}
type ConceptReadView struct {
	Snapshot       Snapshot       `json:"snapshot"`
	Concept        Concept        `json:"concept"`
	Relations      []Assertion    `json:"relations"`
	Evidence       []EvidenceSpan `json:"evidence"`
	TotalRelations int            `json:"total_relations"`
	Truncated      bool           `json:"truncated"`
}
type RequirementReadView struct {
	Snapshot            Snapshot         `json:"snapshot"`
	Requirement         Requirement      `json:"requirement"`
	Trace               RequirementTrace `json:"trace"`
	Evidence            []EvidenceSpan   `json:"evidence"`
	TotalPaths          int              `json:"total_paths"`
	Truncated           bool             `json:"truncated"`
	AcceptanceEvaluated bool             `json:"acceptance_evaluated"`
}

func detached[T any](v T) T {
	raw, _ := json.Marshal(v)
	var out T
	_ = json.Unmarshal(raw, &out)
	return out
}
func readLimit(limit int) int {
	if limit < 1 || limit > 100 {
		return 20
	}
	return limit
}
func (a ActiveProjection) readEvidence(ids map[string]bool) []EvidenceSpan {
	out := []EvidenceSpan{}
	for _, e := range a.projection.Evidence {
		if ids[e.ID] {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
func (a ActiveProjection) LookupTermView(lang, term string, limit int) TermReadView {
	lang = strings.ToLower(strings.TrimSpace(lang))
	term = strings.TrimSpace(term)
	limit = readLimit(limit)
	out := TermReadView{Snapshot: a.Snapshot(), Language: lang, Term: term, Candidates: []ConceptCandidate{}, Evidence: []EvidenceSpan{}}
	ids := map[string]bool{}
	matches := map[string]string{}
	for _, c := range a.projection.Concepts {
		for _, t := range c.Terms {
			if strings.ToLower(t.Lang) == lang && strings.EqualFold(strings.TrimSpace(t.Value), term) {
				out.Candidates = append(out.Candidates, ConceptCandidate{ConceptID: c.ID, Term: t.Value, Status: c.Status, Score: 1})
				matches[c.ID] = c.EvidenceID
				break
			}
		}
	}
	sort.Slice(out.Candidates, func(i, j int) bool { return out.Candidates[i].ConceptID < out.Candidates[j].ConceptID })
	out.TotalCandidates = len(out.Candidates)
	out.Ambiguous = out.TotalCandidates > 1
	out.Truncated = out.TotalCandidates > limit
	if out.Truncated {
		out.Candidates = out.Candidates[:limit]
	}
	for _, c := range out.Candidates {
		ids[matches[c.ConceptID]] = true
	}
	out.Evidence = a.readEvidence(ids)
	return detached(out)
}
func (a ActiveProjection) ConceptView(id string, limit int) (ConceptReadView, bool) {
	out := ConceptReadView{Snapshot: a.Snapshot(), Relations: []Assertion{}, Evidence: []EvidenceSpan{}}
	found := false
	for _, c := range a.projection.Concepts {
		if c.ID == id {
			out.Concept = c
			found = true
			break
		}
	}
	if !found {
		return out, false
	}
	ids := map[string]bool{out.Concept.EvidenceID: true}
	for _, r := range a.projection.Assertions {
		if r.SubjectID == id || r.ObjectID == id {
			out.Relations = append(out.Relations, r)
		}
	}
	sort.Slice(out.Relations, func(i, j int) bool { return out.Relations[i].ID < out.Relations[j].ID })
	out.TotalRelations = len(out.Relations)
	limit = readLimit(limit)
	out.Truncated = out.TotalRelations > limit
	if out.Truncated {
		out.Relations = out.Relations[:limit]
	}
	for _, r := range out.Relations {
		for _, id := range r.EvidenceIDs {
			ids[id] = true
		}
	}
	out.Evidence = a.readEvidence(ids)
	return detached(out), true
}
func (a ActiveProjection) RequirementView(id string, limit int) (RequirementReadView, bool, error) {
	out := RequirementReadView{Snapshot: a.Snapshot(), Evidence: []EvidenceSpan{}}
	found := false
	for _, r := range a.projection.Requirements {
		if r.ID == id {
			out.Requirement = r
			found = true
			break
		}
	}
	if !found {
		return out, false, nil
	}
	// Peer requirements own source spans and assertion endpoints. Validate
	// their complete projection before selecting a single public trace.
	report, err := a.Trace()
	if err != nil {
		return out, true, err
	}
	for _, trace := range report.Requirements {
		if trace.RequirementID == id {
			out.Trace = trace
			break
		}
	}
	if out.Trace.RequirementID == "" {
		return out, true, fmt.Errorf("requirement trace is missing")
	}
	out.TotalPaths = len(out.Trace.Paths)
	limit = readLimit(limit)
	out.Truncated = out.TotalPaths > limit
	if out.Truncated {
		out.Trace.Paths = out.Trace.Paths[:limit]
	}
	ids := map[string]bool{out.Requirement.EvidenceID: true}
	targets := map[string]bool{}
	assertions := map[string]bool{}
	for _, c := range out.Requirement.AcceptanceCriteria {
		ids[c.EvidenceID] = true
	}
	for _, p := range out.Trace.Paths {
		targets[p.CodeCanonicalID] = true
		targets[p.TestCanonicalID] = true
		assertions[p.ImplementationAssertion] = true
		assertions[p.TestedByAssertion] = true
	}
	for _, r := range a.projection.Assertions {
		if assertions[r.ID] || targets[r.ObjectID] {
			for _, e := range r.EvidenceIDs {
				ids[e] = true
			}
		}
	}
	out.Evidence = a.readEvidence(ids)
	return detached(out), true, nil
}
