package stage2

import (
	"strings"

	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

// docDemotionFactor is the multiplier applied to a documentation
// citation's fused score when the active intent seeks code (see
// demoteDocsFor). Mirrors testDemotionFactor's rationale: docs stay in
// the evidence pack as background, but they must not outrank the code
// a code-seeking query is actually about. 0.5 (vs the test factor's
// 0.25) because markdown can still carry the best conceptual answer —
// halving the score drops it below comparable code hits without
// burying it.
const docDemotionFactor = 0.5

// archiveDemotionFactor is the multiplier for citations under an
// archive/ directory, applied for EVERY intent. The docs discipline
// moves superseded material to archive/ ("supersede, don't delete"),
// so an archived section is by definition not the current answer;
// citing it above live docs or code actively misleads. Stronger than
// docDemotionFactor, and not stacked with it — archive classification
// wins.
const archiveDemotionFactor = 0.2

// isDocCitation reports whether a scored citation is documentation
// rather than code. Primary signal is the ckv chunk-strategy label
// ("doc" covers markdown DocSection/ADRSection chunks); the path
// suffix is the fallback for citations that lost the label (e.g.
// ckg-only citations never carry one, but ckg does not index
// markdown, so the suffix check is effectively a safety net).
func isDocCitation(chunkKind, file string) bool {
	if chunkKind == "doc" {
		return true
	}
	lower := strings.ToLower(file)
	return strings.HasSuffix(lower, ".md") || strings.HasSuffix(lower, ".markdown")
}

// isArchivePath reports whether file lives under an archive/ path
// segment (any depth) — the home of superseded documents per the
// documentation discipline.
func isArchivePath(file string) bool {
	lower := strings.ToLower(file)
	return strings.HasPrefix(lower, "archive/") || strings.Contains(lower, "/archive/")
}

// demoteDocsFor reports whether documentation citations should be
// demoted for the given intent. Code-seeking intents want the
// implementation above the prose; DocsUpdate explicitly targets docs,
// ArchExplain legitimately draws on ADRs/design sections ("왜 X
// 결정했나" is what markdown indexing exists for), and Unknown stays
// broad on purpose — mirroring intentToKinds' philosophy.
func demoteDocsFor(intent contract.Intent) bool {
	switch intent {
	case contract.IntentDocsUpdate, contract.IntentArchExplain, contract.IntentUnknown:
		return false
	}
	return true
}
