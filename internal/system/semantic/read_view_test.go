package semantic

import "testing"

// Pure selector controls: loading and source proof are tested separately by
// the MCP retained-source fixture. These model records are not real reviews.
func TestReadViewsBoundRelationsAndDetachMutableFields(t *testing.T) {
	p := Projection{Concepts: []Concept{{ID: "feature", Definition: "A feature", Terms: []Term{{Lang: "en", Value: "feature", Preferred: true}}, EvidenceID: "e", Status: StatusProposed}}, Evidence: []EvidenceSpan{{ID: "e", Kind: SourceDocument}}, Assertions: []Assertion{
		{ID: "z", SubjectID: "feature", ObjectID: "code", Predicate: PredicateImplementedBy, EvidenceIDs: []string{"e"}, Status: StatusProposed},
		{ID: "a", SubjectID: "claim", ObjectID: "feature", Predicate: PredicateAbout, EvidenceIDs: []string{"e"}, Status: StatusRejected},
		{ID: "unrelated", SubjectID: "other", ObjectID: "code", Predicate: PredicateImplementedBy},
	}}
	a := ActiveProjection{projection: p}
	view, found := a.ConceptView("feature", 1)
	if !found || view.TotalRelations != 2 || !view.Truncated || len(view.Relations) != 1 || view.Relations[0].ID != "a" || view.Relations[0].Status != StatusRejected || len(view.Evidence) != 1 {
		t.Fatalf("bounded relations: %+v", view)
	}
	view.Concept.Terms[0].Value = "changed"
	view.Relations[0].EvidenceIDs[0] = "changed"
	if p.Concepts[0].Terms[0].Value != "feature" || p.Assertions[1].EvidenceIDs[0] != "e" {
		t.Fatal("view aliases mutable projection fields")
	}
	if _, ok := a.ConceptView("missing", 20); ok {
		t.Fatal("missing concept invented")
	}
}
