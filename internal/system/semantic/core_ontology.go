package semantic

import "fmt"

// CoreConceptKinds is the D1 common vocabulary. Organization and industry
// concepts live in separately versioned packs and cannot redefine these IDs.
var CoreConceptKinds = map[string]string{
	"project": "entity", "dataset": "artifact", "source-snapshot": "artifact",
	"source-file": "artifact", "code-symbol": "entity", "vector-chunk": "artifact",
	"document-section": "artifact", "claim": "entity", "evidence-span": "artifact",
	"semantic-assertion": "entity", "concept": "entity", "term": "artifact",
	"requirement": "rule", "acceptance-criterion": "rule", "test-case": "artifact",
	"test-run": "process", "policy": "rule", "evidence-pack": "artifact",
	"patch-attempt": "process", "criterion-decision": "process",
}

// ValidateCoreOntology checks a projection intended to be the common pack.
// Generic ontology extraction does not call it: user packs may legitimately
// contain a different set of concepts, but their loader must reject core-ID
// redefinitions before merging with this pack.
func ValidateCoreOntology(p Projection) error {
	if err := p.Validate(); err != nil {
		return err
	}
	if len(p.Concepts) != len(CoreConceptKinds) {
		return fmt.Errorf("core ontology requires exactly %d concepts, got %d", len(CoreConceptKinds), len(p.Concepts))
	}
	for _, concept := range p.Concepts {
		kind, ok := CoreConceptKinds[concept.ID]
		if !ok || concept.Kind != kind {
			return fmt.Errorf("core ontology concept %q has an unapproved ID or kind", concept.ID)
		}
	}
	return nil
}
