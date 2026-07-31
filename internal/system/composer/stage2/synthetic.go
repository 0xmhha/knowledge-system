package stage2

import (
	"strings"

	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

// syntheticMarker is the filename ckv gives a chunk that summarises a
// directory rather than quoting a file.
const syntheticMarker = "<convention>"

// isSyntheticLocation reports whether a citation points at synthesized
// knowledge rather than a span of a real file.
//
// ckv writes a per-package conventions summary ("errors:
// fmt.Errorf_wrap=1, constructors: 1 New*, tests: 120 files") under
// <package-dir>/<convention> with no line range — the 0-0 range is the
// index stating there is nothing to point at. Measured 2026-07-31: all
// 145 convention chunks look like this, while all 44 invariant chunks
// and all 3,679 doc chunks cite real files at real lines and keep
// sourcing their bodies from the tree.
func isSyntheticLocation(c contract.Citation) bool {
	return c.StartLine == 0 && c.EndLine == 0
}

// knowledgeFromHit converts a synthetic-location hit into the pack's
// knowledge shape. Reports false when the hit carries no text, which
// would produce a KnowledgeChunk the contract rejects.
func knowledgeFromHit(h contract.Hit) (contract.KnowledgeChunk, bool) {
	if h.Text == "" {
		return contract.KnowledgeChunk{}, false
	}
	kind := h.ChunkKind
	if kind == "" {
		kind = "convention"
	}
	scope := h.Citation.File
	if i := strings.LastIndex(scope, "/"+syntheticMarker); i >= 0 {
		scope = scope[:i]
	}
	if scope == "" {
		return contract.KnowledgeChunk{}, false
	}
	return contract.KnowledgeChunk{Scope: scope, Kind: kind, Text: h.Text}, true
}
