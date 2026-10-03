package composer

import (
	"github.com/0xmhha/knowledge-system/internal/system/composer/stage1"
	"github.com/0xmhha/knowledge-system/internal/system/composer/stage2"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
	"testing"
)

func TestTraceIncludesTextSearchAttemptsEvenWhenOptionalArmFallsBack(t *testing.T) {
	diagnostic := &contract.OntologyDiagnostic{Mode: "combined", State: "unavailable", Reason: "text_search_failed", TextSearchCalls: 2}
	trace := buildComposerTrace("raw", contract.IntentBugFix, stage1.Stage1Output{Rounds: 1}, stage2.Stage2Output{Ontology: diagnostic})
	if trace.CKVCalls != 3 || trace.Ontology != diagnostic {
		t.Fatalf("text cost disappeared on fallback: %+v", trace)
	}
}
