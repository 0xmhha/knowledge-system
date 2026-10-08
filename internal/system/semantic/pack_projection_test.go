package semantic

import (
	"strings"
	"testing"
)

func TestPackProjectionKeepsTupleNamespaceAndRejectsForgedLinks(t *testing.T) {
	source := []byte(`pack_schema_version: 1
pack_id: industry.payments
version: 1.0.0
owner: project
scope: payments
requires: []
concepts:
  - {id: transfer, kind: entity, definition: A transfer.}
relation_types:
  - predicate: governed-by
    subject_type: {pack_id: industry.payments, local_id: transfer}
    object_type: {pack_id: core, local_id: policy}
    direction: forward
    cardinality: many-to-one
    required_evidence: true
    review_rule: human
constraints: []
competency_questions: ["What governs this transfer?"]
`)
	p, err := ProjectPackSource(source, "industry.payments", "1.0.0", strings.Repeat("a", 64), "knowledge:industry.payments")
	if err != nil {
		t.Fatal(err)
	}
	k := &KnowledgeProjection{LockDigest: strings.Repeat("b", 64), Packs: []PackProjection{p}}
	if err := k.validate(); err != nil {
		t.Fatal(err)
	}
	k.Packs[0].Relations[0].ObjectType = PackTypeRef{PackID: "other.pack", LocalID: "policy"}
	if err := k.validate(); err == nil {
		t.Fatal("cross-namespace type accepted without a matching tuple")
	}
	k.Packs[0].Relations[0].ObjectType = PackTypeRef{PackID: "core", LocalID: "policy"}
	k.Packs[0].Relations[0].RequiredEvidence = false
	if err := k.validate(); err == nil {
		t.Fatal("evidence-free relation accepted")
	}
}
