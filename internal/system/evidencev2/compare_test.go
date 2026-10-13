package evidencev2

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

func comparisonPacks(t *testing.T) (contract.EvidencePackV2, contract.EvidencePackV2) {
	t.Helper()
	_, version := testVersion(t, "one\ntwo\n")
	refs := []contract.Citation{{File: "README.md", StartLine: 1, EndLine: 1}, {File: "README.md", StartLine: 2, EndLine: 2}}
	off, err := Build(context.Background(), version, "feature", refs, testCleaner(t))
	if err != nil {
		t.Fatal(err)
	}
	on, err := Build(context.Background(), version, "feature", []contract.Citation{refs[1], refs[0]}, testCleaner(t))
	if err != nil {
		t.Fatal(err)
	}
	on.Metadata.Ontology = &contract.OntologyDiagnostic{Mode: "relations", State: "active"}
	if err := Stamp(&on); err != nil {
		t.Fatal(err)
	}
	return off, on
}

func TestCompareResponsesReportsStructureWithoutQualityVerdict(t *testing.T) {
	off, on := comparisonPacks(t)
	r, err := CompareResponses(off, on)
	if err != nil {
		t.Fatal(err)
	}
	if r.CitationMembershipChanged || !r.CitationOrderChanged || !r.BodyOutputsChanged || r.QualityEvaluated || r.LatencyEvaluated || r.ControlConditionsVerified || len(r.Citations) != 2 {
		t.Fatalf("incorrect comparison: %+v", r)
	}
	if r.Citations[0].OffRank != 1 || r.Citations[0].OnRank != 2 || len(r.Citations[0].CitationKeySHA256) != 64 {
		t.Fatal(r)
	}
	data, _ := json.Marshal(r)
	if strings.Contains(string(data), "README.md") || strings.Contains(string(data), "feature") {
		t.Fatal("raw path/query leaked")
	}
	if err := Verify(off); err != nil {
		t.Fatal("comparison mutated inputs", err)
	}
	on.Metadata.Ontology.State = "no_candidates"
	if err := Stamp(&on); err != nil {
		t.Fatal(err)
	}
	if r, err := CompareResponses(off, on); err != nil || r.OnState != "no_candidates" {
		t.Fatal("valid empty-candidate fallback refused", r, err)
	}
	on.Metadata.Ontology.State = "active"
	// An active, unchanged arm is no evidence of benefit.
	on.Citations = off.Citations
	on.Bodies = off.Bodies
	if err := Stamp(&on); err != nil {
		t.Fatal(err)
	}
	r, err = CompareResponses(off, on)
	if err != nil || r.CitationOrderChanged || r.BodyOutputsChanged || r.QualityEvaluated {
		t.Fatal(r, err)
	}
	// Omitted body and lost citation are distinct from an order-only change.
	on.Citations = on.Citations[:1]
	on.Bodies = on.Bodies[:1]
	if err := Stamp(&on); err != nil {
		t.Fatal(err)
	}
	r, err = CompareResponses(off, on)
	if err != nil || !r.CitationMembershipChanged || r.Citations[1].OnRank != 0 {
		t.Fatal(r, err)
	}
}

func TestCompareResponsesRejectsIntegrityAndUnpairedInputs(t *testing.T) {
	for _, name := range []string{"tampered", "query", "coordinates", "off_is_on", "on_is_off", "unknown_state"} {
		t.Run(name, func(t *testing.T) {
			off, on := comparisonPacks(t)
			switch name {
			case "tampered":
				on.Bodies[0].Text = "tampered"
			case "query":
				on.Query = "different"
			case "coordinates":
				on.Coordinates.ProjectID = "different"
				for i := range on.Citations {
					on.Citations[i].ProjectID = "different"
				}
				for i := range on.Bodies {
					on.Bodies[i].Citation.ProjectID = "different"
				}
			case "off_is_on":
				off.Metadata.Ontology = on.Metadata.Ontology
				if err := Stamp(&off); err != nil {
					t.Fatal(err)
				}
			case "on_is_off":
				on.Metadata.Ontology.Mode = "off"
			case "unknown_state":
				on.Metadata.Ontology.State = "private/path"
			}
			if name != "tampered" {
				if err := Stamp(&on); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := CompareResponses(off, on); err == nil {
				t.Fatal("invalid comparison accepted")
			}
		})
	}
}
