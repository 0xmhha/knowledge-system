package semantic

// ProjectionCounts describes validated records, not quality or acceptance.
type ProjectionCounts struct {
	Concepts                int `json:"concepts"`
	VerifiedConcepts        int `json:"verified_concepts"`
	Requirements            int `json:"requirements"`
	VerifiedRequirements    int `json:"verified_requirements"`
	Assertions              int `json:"assertions"`
	VerifiedAssertions      int `json:"verified_assertions"`
	VerifiedImplementations int `json:"verified_implementations"`
}

func (a ActiveProjection) Counts() ProjectionCounts {
	out := ProjectionCounts{Concepts: len(a.projection.Concepts), Requirements: len(a.projection.Requirements), Assertions: len(a.projection.Assertions)}
	implementations := map[string]bool{}
	for _, c := range a.projection.Concepts {
		if c.Status == StatusVerified {
			out.VerifiedConcepts++
		}
		for _, i := range a.VerifiedImplementations(c.ID) {
			implementations[i.AssertionID] = true
		}
	}
	out.VerifiedImplementations = len(implementations)
	for _, r := range a.projection.Requirements {
		if r.Status == StatusVerified {
			out.VerifiedRequirements++
		}
	}
	for _, r := range a.projection.Assertions {
		if r.Status == StatusVerified {
			out.VerifiedAssertions++
		}
	}
	return out
}
