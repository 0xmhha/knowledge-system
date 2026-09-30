package knowledgepack

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/0xmhha/knowledge-system/internal/setup"
	"gopkg.in/yaml.v3"
)

type SourceRef struct {
	OriginID string `yaml:"origin_id" json:"origin_id"`
	Path     string `yaml:"path" json:"path"`
}

type Policy struct {
	ID            string            `yaml:"id" json:"id"`
	Type          TypeRef           `yaml:"type" json:"type"`
	Statement     string            `yaml:"statement" json:"statement"`
	Owner         string            `yaml:"owner" json:"owner"`
	Scope         map[string]string `yaml:"scope" json:"scope"`
	EffectiveFrom string            `yaml:"effective_from" json:"effective_from"`
	EffectiveTo   string            `yaml:"effective_to" json:"effective_to"`
	Status        string            `yaml:"status" json:"status"`
	ReviewedBy    string            `yaml:"reviewed_by" json:"reviewed_by"`
	ReviewReason  string            `yaml:"review_reason" json:"review_reason"`
	Visibility    string            `yaml:"visibility" json:"visibility"`
	ConflictsWith []string          `yaml:"conflicts_with" json:"conflicts_with"`
	SourceRef     SourceRef         `yaml:"source_ref" json:"source_ref"`
}

type Decision struct {
	ID             string            `yaml:"id" json:"id"`
	Type           TypeRef           `yaml:"type" json:"type"`
	Problem        string            `yaml:"problem" json:"problem"`
	Decision       string            `yaml:"decision" json:"decision"`
	Rationale      string            `yaml:"rationale" json:"rationale"`
	Alternatives   []string          `yaml:"alternatives" json:"alternatives"`
	Assumptions    []string          `yaml:"assumptions" json:"assumptions"`
	Scope          map[string]string `yaml:"scope" json:"scope"`
	Date           string            `yaml:"date" json:"date"`
	Status         string            `yaml:"status" json:"status"`
	ReviewedBy     string            `yaml:"reviewed_by" json:"reviewed_by"`
	ReviewReason   string            `yaml:"review_reason" json:"review_reason"`
	Visibility     string            `yaml:"visibility" json:"visibility"`
	Supersedes     string            `yaml:"supersedes" json:"supersedes"`
	RequirementIDs []string          `yaml:"requirement_ids" json:"requirement_ids"`
	SourceRef      SourceRef         `yaml:"source_ref" json:"source_ref"`
	Body           string            `yaml:"-" json:"body"`
}

type Instances struct {
	Policies  []Policy   `json:"policies"`
	Decisions []Decision `json:"decisions"`
	Conflicts []Conflict `json:"conflicts"`
}

type Conflict struct {
	LeftID  string `json:"left_id"`
	RightID string `json:"right_id"`
	Reason  string `json:"reason"`
}

type PolicySummary struct {
	ID            string    `json:"id"`
	State         string    `json:"state"`
	SourceRef     SourceRef `json:"source_ref"`
	ReviewedBy    string    `json:"reviewed_by"`
	EffectiveFrom string    `json:"effective_from"`
	EffectiveTo   string    `json:"effective_to,omitempty"`
}

type PolicyContext struct {
	State      string          `json:"state"`
	Applicable []PolicySummary `json:"applicable"`
	Conflicts  []Conflict      `json:"conflicts"`
	Unknowns   []string        `json:"unknowns"`
}

// SelectPolicies applies only reviewed, in-scope, in-time policy records.
// It never returns policy statements: public bodies must be supplied through
// separately authorized, sanitized source citations.
func (i Instances) SelectPolicies(asOf string, queryScope map[string]string, allowRestricted bool) (PolicyContext, error) {
	if !validDate(asOf) {
		return PolicyContext{}, fmt.Errorf("invalid policy query date")
	}
	result := PolicyContext{State: "unknown", Applicable: []PolicySummary{}, Conflicts: []Conflict{}, Unknowns: []string{}}
	current := map[string]bool{}
	restricted := false
	stale := false
	for _, p := range i.Policies {
		if !policyScopeMatches(p.Scope, queryScope) {
			continue
		}
		if p.Status != "verified" {
			continue
		}
		if asOf < p.EffectiveFrom || p.EffectiveTo != "" && asOf > p.EffectiveTo {
			stale = true
			continue
		}
		if p.Visibility == "restricted" && !allowRestricted {
			restricted = true
			continue
		}
		current[p.ID] = true
		result.Applicable = append(result.Applicable, PolicySummary{ID: p.ID, State: "current", SourceRef: p.SourceRef,
			ReviewedBy: p.ReviewedBy, EffectiveFrom: p.EffectiveFrom, EffectiveTo: p.EffectiveTo})
	}
	for _, conflict := range i.Conflicts {
		if current[conflict.LeftID] && current[conflict.RightID] {
			result.Conflicts = append(result.Conflicts, conflict)
		}
	}
	switch {
	case len(result.Conflicts) > 0:
		result.State = "conflict"
	case restricted:
		result.State = "restricted"
		result.Applicable = nil
		result.Conflicts = nil
	case len(result.Applicable) > 0:
		result.State = "needs_citation"
	case stale:
		result.State = "stale"
		result.Unknowns = append(result.Unknowns, "no_current_policy")
	default:
		result.Unknowns = append(result.Unknowns, "no_reviewed_applicable_policy")
	}
	return result, nil
}

func policyScopeMatches(policy, query map[string]string) bool {
	for key, value := range policy {
		if query[key] != value {
			return false
		}
	}
	return true
}

func LoadInstances(overlayRoot string, packs []LoadedPack) (Instances, error) {
	result := Instances{Policies: []Policy{}, Decisions: []Decision{}, Conflicts: []Conflict{}}
	types := map[TypeRef]bool{}
	for _, loaded := range packs {
		for _, concept := range loaded.Pack.Concepts {
			types[TypeRef{PackID: loaded.Pack.PackID, LocalID: concept.ID}] = true
		}
	}
	seen := map[string]bool{}
	readDir := func(name string, handle func(string, []byte) error) error {
		entries, err := os.ReadDir(filepath.Join(overlayRoot, name))
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("evidence_unverified: nonregular %s entry", name)
			}
			if name == "policies" && filepath.Ext(entry.Name()) != ".yaml" ||
				name == "decisions" && filepath.Ext(entry.Name()) != ".md" {
				return fmt.Errorf("evidence_unverified: unsupported %s file %q", name, entry.Name())
			}
			rel := name + "/" + entry.Name()
			buf, err := setup.ReadSourceFileNoFollow(overlayRoot, rel, 4<<20)
			if err != nil {
				return fmt.Errorf("evidence_unverified: read %q: %w", rel, err)
			}
			if err := handle(rel, buf); err != nil {
				return err
			}
		}
		return nil
	}
	if err := readDir("policies", func(rel string, buf []byte) error {
		var p Policy
		if err := decodeYAML(buf, &p); err != nil {
			return fmt.Errorf("evidence_unverified: policy %q: %w", rel, err)
		}
		if err := validatePolicy(p, rel, types); err != nil {
			return err
		}
		if seen[p.ID] {
			return fmt.Errorf("evidence_unverified: duplicate instance ID %q", p.ID)
		}
		seen[p.ID] = true
		result.Policies = append(result.Policies, p)
		return nil
	}); err != nil {
		return Instances{}, err
	}
	if err := readDir("decisions", func(rel string, buf []byte) error {
		var d Decision
		front, body, err := markdownFrontMatter(buf)
		if err != nil {
			return fmt.Errorf("evidence_unverified: decision %q: %w", rel, err)
		}
		if err := decodeYAML(front, &d); err != nil {
			return fmt.Errorf("evidence_unverified: decision %q: %w", rel, err)
		}
		d.Body = body
		if err := validateDecision(d, rel, types); err != nil {
			return err
		}
		if seen[d.ID] {
			return fmt.Errorf("evidence_unverified: duplicate instance ID %q", d.ID)
		}
		seen[d.ID] = true
		result.Decisions = append(result.Decisions, d)
		return nil
	}); err != nil {
		return Instances{}, err
	}
	byID := map[string]Policy{}
	for _, p := range result.Policies {
		byID[p.ID] = p
	}
	conflicts := map[string]bool{}
	for _, p := range result.Policies {
		for _, otherID := range p.ConflictsWith {
			other, ok := byID[otherID]
			if !ok || otherID == p.ID {
				return Instances{}, fmt.Errorf("evidence_unverified: invalid policy conflict reference")
			}
			if p.Status != "verified" || other.Status != "verified" {
				continue
			}
			if !scopeOverlaps(p.Scope, other.Scope) || !timeOverlaps(p, other) {
				continue
			}
			ids := []string{p.ID, otherID}
			sort.Strings(ids)
			key := ids[0] + "\x00" + ids[1]
			if !conflicts[key] {
				conflicts[key] = true
				result.Conflicts = append(result.Conflicts, Conflict{ids[0], ids[1], "explicit_verified_policy_conflict"})
			}
		}
	}
	sort.Slice(result.Conflicts, func(i, j int) bool {
		if result.Conflicts[i].LeftID == result.Conflicts[j].LeftID {
			return result.Conflicts[i].RightID < result.Conflicts[j].RightID
		}
		return result.Conflicts[i].LeftID < result.Conflicts[j].LeftID
	})
	return result, nil
}

func decodeYAML(buf []byte, out any) error {
	decoder := yaml.NewDecoder(bytes.NewReader(buf))
	decoder.KnownFields(true)
	if err := decoder.Decode(out); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("multiple YAML documents")
	}
	return nil
}

func markdownFrontMatter(data []byte) ([]byte, string, error) {
	if !bytes.HasPrefix(data, []byte("---\n")) {
		return nil, "", fmt.Errorf("missing YAML front matter")
	}
	end := bytes.Index(data[4:], []byte("\n---\n"))
	if end < 0 {
		return nil, "", fmt.Errorf("unterminated YAML front matter")
	}
	start := 4 + end
	return data[4:start], string(data[start+5:]), nil
}

func validReview(status, reviewer, reason string) bool {
	switch status {
	case "proposed":
		return reviewer == "" && reason == ""
	case "verified", "rejected":
		return strings.TrimSpace(reviewer) != "" && strings.TrimSpace(reason) != ""
	}
	return false
}

func validDate(value string) bool { _, err := time.Parse("2006-01-02", value); return err == nil }

func validSource(ref SourceRef, path string) bool {
	return ref.OriginID == "repo" && ref.Path == ".cks/knowledge/"+path
}

func validatePolicy(p Policy, path string, types map[TypeRef]bool) error {
	if p.ID == "" || !types[p.Type] || strings.TrimSpace(p.Statement) == "" ||
		strings.TrimSpace(p.Owner) == "" || len(p.Scope) == 0 || !validSource(p.SourceRef, path) ||
		!validReview(p.Status, p.ReviewedBy, p.ReviewReason) ||
		(p.Visibility != "public" && p.Visibility != "restricted") {
		return fmt.Errorf("evidence_unverified: invalid policy %q", path)
	}
	for key, value := range p.Scope {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			return fmt.Errorf("evidence_unverified: empty policy scope")
		}
	}
	if p.EffectiveFrom != "" && !validDate(p.EffectiveFrom) || p.EffectiveTo != "" && !validDate(p.EffectiveTo) ||
		p.EffectiveTo != "" && (p.EffectiveFrom == "" || p.EffectiveTo < p.EffectiveFrom) ||
		p.Status == "verified" && p.EffectiveFrom == "" {
		return fmt.Errorf("evidence_unverified: invalid policy effective dates")
	}
	return nil
}

func validateDecision(d Decision, path string, types map[TypeRef]bool) error {
	if d.ID == "" || !types[d.Type] || strings.TrimSpace(d.Problem) == "" ||
		strings.TrimSpace(d.Decision) == "" || strings.TrimSpace(d.Rationale) == "" ||
		len(d.Alternatives) == 0 || len(d.Scope) == 0 || !validSource(d.SourceRef, path) ||
		!validReview(d.Status, d.ReviewedBy, d.ReviewReason) ||
		(d.Visibility != "public" && d.Visibility != "restricted") || strings.TrimSpace(d.Body) == "" {
		return fmt.Errorf("evidence_unverified: invalid decision %q", path)
	}
	if d.Date != "" && !validDate(d.Date) || d.Status == "verified" && d.Date == "" {
		return fmt.Errorf("evidence_unverified: invalid decision date")
	}
	return nil
}

func scopeOverlaps(a, b map[string]string) bool {
	for key, value := range a {
		if other, ok := b[key]; ok && other != value {
			return false
		}
	}
	return true
}

func timeOverlaps(a, b Policy) bool {
	if a.EffectiveTo != "" && b.EffectiveFrom != "" && a.EffectiveTo < b.EffectiveFrom {
		return false
	}
	if b.EffectiveTo != "" && a.EffectiveFrom != "" && b.EffectiveTo < a.EffectiveFrom {
		return false
	}
	return true
}
