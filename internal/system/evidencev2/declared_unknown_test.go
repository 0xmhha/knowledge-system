package evidencev2

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

const opsDoc = "# Operations\nThe recovery objective is not approved.\nNo owner is assigned.\n"

func unknownRecord(fields map[string]string) string {
	values := map[string]string{"status": "verified", "reviewed_by": "reviewer", "review_reason": "Compared with docs/ops.md.",
		"visibility": "public", "recorded_on": `"2026-03-01"`, "resolved_on": `""`, "start": "2", "end": "3"}
	for k, v := range fields {
		values[k] = v
	}
	review := "reviewed_by: " + values["reviewed_by"] + "\nreview_reason: " + values["review_reason"] + "\n"
	if values["status"] == "proposed" {
		review = ""
	}
	return `id: UNK-RECOVERY
statement: The operating recovery objective and its owner are not decided.
scope: {subsystem: transfers}
recorded_on: ` + values["recorded_on"] + `
resolved_on: ` + values["resolved_on"] + `
match_any: [[recovery, objective], [RTO]]
basis:
  - {path: docs/ops.md, start_line: ` + values["start"] + `, end_line: ` + values["end"] + `}
status: ` + values["status"] + `
` + review + `visibility: ` + values["visibility"] + `
source_ref: {origin_id: repo, path: .cks/knowledge/unknowns/UNK-RECOVERY.yaml}
`
}

func declaredUnknownVersion(t *testing.T, fields map[string]string) string {
	t.Helper()
	_, version := buildKnowledgeVersionWithMutation(t, "public", "Separate approval is required.", false, func(root string) error {
		dir := filepath.Join(root, ".cks", "knowledge", "unknowns")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Join(root, "docs"), 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(root, "docs", "ops.md"), []byte(opsDoc), 0o600); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dir, "UNK-RECOVERY.yaml"), []byte(unknownRecord(fields)), 0o600)
	})
	return version
}

func readmeBase(t *testing.T, version, query string) contract.EvidencePackV2 {
	t.Helper()
	base, err := Build(context.Background(), version, query, []contract.Citation{{File: "README.md", StartLine: 1, EndLine: 1}}, testCleaner(t))
	if err != nil {
		t.Fatal(err)
	}
	return base
}

func knowledgeState(t *testing.T, p contract.EvidencePackV2) contract.KnowledgeContextV2 {
	t.Helper()
	return p.Semantic.(contract.KnowledgeSemanticV2).KnowledgeContext
}

func TestDeclaredUnknownCitesOnlyItsBasis(t *testing.T) {
	version := declaredUnknownVersion(t, nil)
	got, err := AttachKnowledge(context.Background(), readmeBase(t, version, "What RTO is approved?"), version, "2026-06-01", "transfers", testCleaner(t))
	if err != nil {
		t.Fatal(err)
	}
	k := knowledgeState(t, got)
	if k.State != "declared_unknown" || len(k.DeclaredUnknowns) != 1 || k.DeclaredUnknowns[0].ID != "UNK-RECOVERY" ||
		len(k.ApplicablePolicies) != 0 || len(got.Citations) != 1 || got.Citations[0].File != "docs/ops.md" ||
		got.Citations[0].StartLine != 2 || got.Citations[0].EndLine != 3 || len(got.Bodies) != 1 ||
		!strings.Contains(got.Bodies[0].Text, "not approved") || got.EvidenceState != "complete" {
		t.Fatalf("declared unknown did not replace evidence with its basis: %+v", got)
	}
	if err := Verify(got); err != nil {
		t.Fatal(err)
	}
	// The multi-term group needs every term; matching folds case.
	got, err = AttachKnowledge(context.Background(), readmeBase(t, version, "Which Recovery OBJECTIVE applies?"), version, "2026-06-01", "transfers", testCleaner(t))
	if err != nil || knowledgeState(t, got).State != "declared_unknown" {
		t.Fatalf("case-folded group did not match: %v", err)
	}
}

func TestDeclaredUnknownLeavesOtherRequestsUnchanged(t *testing.T) {
	cases := []struct {
		name, query, asOf string
		fields            map[string]string
	}{
		{"no term", "Which approval policy applies?", "2026-06-01", nil},
		{"partial group", "What recovery steps exist?", "2026-06-01", nil},
		{"before recorded", "What RTO is approved?", "2026-02-01", nil},
		{"after resolved", "What RTO is approved?", "2026-06-01", map[string]string{"resolved_on": `"2026-05-01"`}},
		{"proposed", "What RTO is approved?", "2026-06-01", map[string]string{"status": "proposed"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			version := declaredUnknownVersion(t, tc.fields)
			got, err := AttachKnowledge(context.Background(), readmeBase(t, version, tc.query), version, tc.asOf, "transfers", testCleaner(t))
			if err != nil {
				t.Fatal(err)
			}
			k := knowledgeState(t, got)
			if k.State == "declared_unknown" || len(k.DeclaredUnknowns) != 0 || got.Citations[0].File != "README.md" {
				t.Fatalf("unrelated request abstained: %+v", k)
			}
			if err := Verify(got); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestDeclaredUnknownRestrictedOrUnprovenKeepsBase(t *testing.T) {
	for name, fields := range map[string]map[string]string{
		"restricted":        {"visibility": "restricted"},
		"basis out of file": {"start": "9", "end": "12"},
	} {
		t.Run(name, func(t *testing.T) {
			version := declaredUnknownVersion(t, fields)
			got, err := AttachKnowledge(context.Background(), readmeBase(t, version, "What RTO is approved?"), version, "2026-06-01", "transfers", testCleaner(t))
			if err != nil {
				t.Fatal(err)
			}
			k := knowledgeState(t, got)
			if k.State == "declared_unknown" || len(k.DeclaredUnknowns) != 0 || len(got.Citations) != 1 || got.Citations[0].File != "README.md" {
				t.Fatalf("unproven or restricted unknown changed the base: %+v", got)
			}
			if name == "restricted" && (k.State != "restricted" || strings.Contains(strings.Join(k.Unknowns, ","), "UNK-")) {
				t.Fatalf("restricted unknown leaked its identity: %+v", k)
			}
			if err := Verify(got); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestDeclaredUnknownVerifyRejectsMixedEvidence(t *testing.T) {
	version := declaredUnknownVersion(t, nil)
	got, err := AttachKnowledge(context.Background(), readmeBase(t, version, "What RTO is approved?"), version, "2026-06-01", "transfers", testCleaner(t))
	if err != nil {
		t.Fatal(err)
	}
	readme := readmeBase(t, version, "What RTO is approved?")
	mixed := got
	mixed.Citations = append(append([]contract.CitationV2{}, got.Citations...), readme.Citations[0])
	mixed.Bodies = append(append([]contract.BodyV2{}, got.Bodies...), readme.Bodies[0])
	s := mixed.Semantic.(contract.KnowledgeSemanticV2)
	s.CodingContext.Evidence = append([]contract.CitationV2{}, mixed.Citations...)
	mixed.Semantic = s
	if err := Stamp(&mixed); err == nil {
		if err := Verify(mixed); err == nil {
			t.Fatal("declared_unknown pack accepted a non-basis citation")
		}
	}
	other := readme
	k := emptyKnowledgeContext("unknown", knowledgeState(t, got).LockDigest)
	k.DeclaredUnknowns = knowledgeState(t, got).DeclaredUnknowns
	other.Semantic = contract.KnowledgeSemanticV2{KnowledgeContext: k, CodingContext: buildCodingContext(k, other.Citations)}
	if err := Stamp(&other); err == nil {
		if err := Verify(other); err == nil {
			t.Fatal("declared unknowns accepted outside the declared_unknown state")
		}
	}
}
