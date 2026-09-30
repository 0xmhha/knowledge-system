package semantic

import (
	"fmt"

	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

// AnnotatePack adds reviewed trace IDs only for citations already present in
// a valid EvidencePack. It never adds a citation or source body. Callers must
// obtain ActiveProjection through CurrentAligned for live coordinate checks.
func (a ActiveProjection) AnnotatePack(pack *contract.EvidencePack) error {
	if pack == nil || !pack.IsValid() {
		return fmt.Errorf("invalid evidence pack")
	}
	valid, err := contract.VerifyIntegrity(*pack)
	if err != nil || !valid {
		return fmt.Errorf("base evidence pack integrity failed: %v", err)
	}
	trace, err := a.Trace()
	if err != nil {
		return err
	}
	for _, citation := range pack.Citations {
		if citation.CommitHash != "" && citation.CommitHash != trace.Snapshot.Commit {
			return fmt.Errorf("citation belongs to another commit: %s", citation.File)
		}
	}
	evidence := map[string]EvidenceSpan{}
	for _, span := range a.projection.Evidence {
		evidence[span.ID] = span
	}
	assertions := map[string]Assertion{}
	for _, assertion := range a.projection.Assertions {
		assertions[assertion.ID] = assertion
	}
	overlay := contract.SemanticOverlay{ProjectID: trace.Snapshot.ProjectID,
		DatasetID: trace.Snapshot.DatasetID, Commit: trace.Snapshot.Commit,
		Links: []contract.SemanticLink{}}
	for _, requirement := range trace.Requirements {
		if requirement.State != TraceLinked {
			continue
		}
		for _, path := range requirement.Paths {
			code, okCode := assertionAnchor(assertions[path.ImplementationAssertion], evidence, path.CodeCanonicalID, SourceCode)
			test, okTest := assertionAnchor(assertions[path.TestedByAssertion], evidence, path.TestCanonicalID, SourceTest)
			if !okCode || !okTest || len(path.CriterionIDs()) == 0 {
				continue
			}
			for _, citation := range pack.Citations {
				if citation.CommitHash != trace.Snapshot.Commit ||
					!citationOverlaps(citation, code) && !citationOverlaps(citation, test) {
					continue
				}
				overlay.Links = append(overlay.Links, contract.SemanticLink{
					RequirementID: requirement.RequirementID, CriterionIDs: append([]string(nil), path.CriterionIDs()...),
					ConceptID: path.ConceptID, CodeSymbolID: path.CodeCanonicalID,
					TestSymbolID:   path.TestCanonicalID,
					AssertionIDs:   []string{path.ImplementationAssertion, path.TestedByAssertion},
					CodeEvidenceID: code.ID, TestEvidenceID: test.ID,
					Citation: citation, Unconfirmed: true,
				})
			}
		}
	}
	if len(overlay.Links) == 0 {
		pack.Semantic = nil
		return nil
	}
	if err := contract.StampSemanticOverlay(&overlay); err != nil {
		return err
	}
	pack.Semantic = &overlay
	if !pack.IsValid() {
		return fmt.Errorf("generated semantic overlay failed validation")
	}
	return nil
}

func citationOverlaps(c contract.Citation, e EvidenceSpan) bool {
	return c.File == e.Path && c.StartLine <= e.EndLine && c.EndLine >= e.StartLine
}

func assertionAnchor(assertion Assertion, evidence map[string]EvidenceSpan, canonicalID string, kind SourceKind) (EvidenceSpan, bool) {
	for _, id := range assertion.EvidenceIDs {
		span := evidence[id]
		if span.CanonicalID == canonicalID && span.Kind == kind {
			return span, true
		}
	}
	return EvidenceSpan{}, false
}
