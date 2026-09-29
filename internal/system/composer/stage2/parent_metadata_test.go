package stage2

import (
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

func TestAggregatorCarriesOnlyValidDocumentParent(t *testing.T) {
	child := contract.Citation{File: "guide.md", StartLine: 20, EndLine: 22, CommitHash: "abc"}
	valid := contract.Citation{File: child.File, StartLine: 1, EndLine: 100, CommitHash: child.CommitHash}
	agg := newAggregator(60, 1, 1, 1)
	agg.addCkvList([]contract.Hit{{Citation: child, ParentCitation: &valid, ChunkKind: "doc"}})
	if got := agg.byCitation[child.Key()].ParentCitation; got == nil || *got != valid {
		t.Fatalf("valid parent lost: %+v", got)
	}
	wrongSnapshot := valid
	wrongSnapshot.CommitHash = "def"
	agg = newAggregator(60, 1, 1, 1)
	agg.addCkvList([]contract.Hit{{Citation: child, ParentCitation: &wrongSnapshot, ChunkKind: "doc"}})
	if got := agg.byCitation[child.Key()].ParentCitation; got != nil {
		t.Fatalf("cross-snapshot parent accepted: %+v", got)
	}
}
