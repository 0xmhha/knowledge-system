package knowledgepack

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPolicyAndDecisionRequireReviewAndExposeExplicitConflict(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{"policies", "decisions"} {
		if err := os.Mkdir(filepath.Join(root, dir), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	pack := validPack("engineering.decisions")
	pack.Concepts = append(pack.Concepts, ConceptType{ID: "design-decision", Kind: "entity", Definition: "A reviewed design choice."})
	loaded := []LoadedPack{{Pack: pack, Digest: strings.Repeat("a", 64)}}
	write := func(rel, text string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, rel), []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	policy := func(id, statement, other string) string {
		return `id: ` + id + `
type: {pack_id: engineering.decisions, local_id: business-policy}
statement: ` + statement + `
owner: payments-team
scope: {subsystem: transfers}
effective_from: "2026-01-01"
status: verified
reviewed_by: reviewer-1
review_reason: Approved against the governing source.
visibility: public
conflicts_with: [` + other + `]
source_ref: {origin_id: repo, path: .cks/knowledge/policies/` + id + `.yaml}
`
	}
	write("policies/BR-17.yaml", policy("BR-17", "Separate approval is required.", "BR-18"))
	write("policies/BR-18.yaml", policy("BR-18", "Separate approval is forbidden.", "BR-17"))
	write("decisions/ADR-1.md", `---
id: ADR-1
type: {pack_id: engineering.decisions, local_id: design-decision}
problem: Which approval path should transfers use?
decision: Require an independent approval.
rationale: Preserve separation of duties.
alternatives: [Single approver]
assumptions: [Approval service is available]
scope: {subsystem: transfers}
date: "2026-01-01"
status: verified
reviewed_by: reviewer-1
review_reason: Compared with the approval policy.
visibility: public
source_ref: {origin_id: repo, path: .cks/knowledge/decisions/ADR-1.md}
---
# Decision
The team selected separate approval.
`)
	instances, err := LoadInstances(root, loaded)
	if err != nil {
		t.Fatal(err)
	}
	if len(instances.Policies) != 2 || len(instances.Decisions) != 1 || len(instances.Conflicts) != 1 ||
		instances.Conflicts[0].Reason != "explicit_verified_policy_conflict" {
		t.Fatalf("missing conflict: %+v", instances)
	}
	context, err := instances.SelectPolicies("2026-06-01", map[string]string{"subsystem": "transfers"}, true)
	if err != nil || context.State != "conflict" || len(context.Conflicts) != 1 {
		t.Fatalf("conflict query: %+v, %v", context, err)
	}
	instances.Policies[0].Visibility = "restricted"
	context, err = instances.SelectPolicies("2026-06-01", map[string]string{"subsystem": "transfers"}, false)
	if err != nil || context.State != "restricted" || len(context.Applicable) != 0 || len(context.Conflicts) != 0 {
		t.Fatalf("restricted policy leaked through query: %+v, %v", context, err)
	}
	instances.Policies[0].Visibility = "public"
	instances.Policies[0].EffectiveTo = "2026-06-30"
	context, err = instances.SelectPolicies("2027-01-01", map[string]string{"subsystem": "transfers"}, true)
	if err != nil || context.State != "needs_citation" || len(context.Applicable) != 1 || len(context.Conflicts) != 0 {
		t.Fatalf("expired policy treated as current/conflict: %+v, %v", context, err)
	}
	write("policies/BR-17.yaml", strings.Replace(policy("BR-17", "Separate approval is required.", "BR-18"),
		"reviewed_by: reviewer-1\nreview_reason: Approved against the governing source.\n", "", 1))
	if _, err := LoadInstances(root, loaded); err == nil || !strings.Contains(err.Error(), "evidence_unverified") {
		t.Fatalf("unreviewed verified policy accepted: %v", err)
	}
}
