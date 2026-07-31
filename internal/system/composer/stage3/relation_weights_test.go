package stage3

import (
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

// TestRelationWeight_ArchExplainRanksCallsOverDefines pins the ordering
// this table exists for: on an "explain the flow" prompt a calls edge
// must beat a defines→field edge even when the defines edge comes from
// a better-scoring seed. Measured case (2026-07-31): the Composer struct
// seed outscored the Compose body seed, so its eight one-line field
// edges buried Compose→assemblePack at pack position 19.
func TestRelationWeight_ArchExplainRanksCallsOverDefines(t *testing.T) {
	t.Parallel()
	const (
		structSeed = 1.0 // the better-scoring seed, donating defines edges
		bodySeed   = 0.7 // the seed the user actually asked about
	)
	defines := structSeed * relationWeight(contract.IntentArchExplain, contract.RelationDefines)
	calls := bodySeed * relationWeight(contract.IntentArchExplain, contract.RelationCalls)
	if calls <= defines {
		t.Errorf("calls from the weaker seed = %.3f, defines from the stronger = %.3f; calls must win", calls, defines)
	}
}

func TestRelationWeight_ArchExplainOrdering(t *testing.T) {
	t.Parallel()
	arch := contract.IntentArchExplain
	calls := relationWeight(arch, contract.RelationCalls)
	imports := relationWeight(arch, contract.RelationImports)
	defines := relationWeight(arch, contract.RelationDefines)

	if !(calls > imports && imports > defines) {
		t.Errorf("want calls > imports > defines, got %.2f / %.2f / %.2f", calls, imports, defines)
	}
	for _, rel := range []contract.Relation{
		contract.RelationCalls, contract.RelationImplements, contract.RelationEmbeds,
	} {
		if got := relationWeight(arch, rel); got != 1.0 {
			t.Errorf("relationWeight(arch_explain, %s) = %.2f, want 1.0 (behavioral edges stay unscaled)", rel, got)
		}
	}
}

// TestRelationWeight_OtherIntentsUnchanged pins that this table is a
// no-op outside ArchExplain — every other intent keeps the previous
// relation-blind scoring until its own measurement says otherwise.
func TestRelationWeight_OtherIntentsUnchanged(t *testing.T) {
	t.Parallel()
	relations := []contract.Relation{
		contract.RelationCalls, contract.RelationCalledBy, contract.RelationDefines,
		contract.RelationImports, contract.RelationImplements, contract.RelationEmbeds,
		contract.RelationReferences,
	}
	for _, intent := range contract.AllIntents() {
		if intent == contract.IntentArchExplain {
			continue
		}
		for _, rel := range relations {
			if got := relationWeight(intent, rel); got != 1.0 {
				t.Errorf("relationWeight(%s, %s) = %.2f, want 1.0", intent, rel, got)
			}
		}
	}
}
