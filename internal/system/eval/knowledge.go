package eval

import "github.com/0xmhha/knowledge-system/pkg/system/contract"

// missingKnowledgeScopes returns the expected scopes absent from the
// pack's knowledge section.
//
// Recall and MRR are computed over citations, so they cannot see this
// section at all — a scenario can score a perfect 1.00 while the pack
// delivers no knowledge whatsoever. That blind spot is how convention
// chunks went missing from every pack until #72 turned one up by
// accident, and it is the same shape as the bm25-rerank-option collapse:
// nothing measured it, so nothing complained.
func missingKnowledgeScopes(expected []string, got []contract.KnowledgeChunk) []string {
	if len(expected) == 0 {
		return nil
	}
	have := make(map[string]struct{}, len(got))
	for _, k := range got {
		have[k.Scope] = struct{}{}
	}
	var missing []string
	for _, scope := range expected {
		if _, ok := have[scope]; !ok {
			missing = append(missing, scope)
		}
	}
	return missing
}
