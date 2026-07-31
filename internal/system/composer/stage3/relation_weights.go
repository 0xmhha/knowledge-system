package stage3

import "github.com/0xmhha/knowledge-system/pkg/system/contract"

// relationWeight scales a neighbor's score by how much the edge's
// relation answers the active Intent.
//
// Why this exists: the neighbor score was seed.Score/(1+distance) —
// relation-blind. Whichever seed scored highest donated its whole
// fan-out at the top, regardless of what the edges meant. Measured on
// the composer-pipeline-flow scenario (2026-07-31): the Composer struct
// citation outranked the Compose body citation, so its eight
// defines→field edges (one per struct field, each a one-line target)
// took the top of the neighbor list and pushed the
// Compose→assemblePack calls edge to position 19 in the pack. Two of
// those one-line fields even won body slots that assemblePack — the
// second half of the answer — did not get.
//
// Relation is not interchangeable evidence. For "how does X work",
// calls is the flow the user asked about while defines→field is the
// containment of a struct. Both are real signals (the field neighbors
// were kept deliberately — they name the pipeline's stage components),
// so this demotes rather than filters.
//
// Only ArchExplain is tuned. Every other intent keeps 1.0 across the
// board, which reproduces the previous relation-blind behavior exactly;
// widening the table needs its own measurement.
func relationWeight(intent contract.Intent, rel contract.Relation) float64 {
	if intent != contract.IntentArchExplain {
		return 1.0
	}
	switch rel {
	case contract.RelationDefines:
		// Struct→field containment. Architecture signal, but it fans out
		// once per field and drowns the behavioral edges when a type
		// declaration outscores the function the user asked about.
		return 0.5
	case contract.RelationImports:
		// File-level wiring — coarser than the symbol relations below.
		return 0.7
	default:
		// calls / implements / embeds: the behavioral and structural
		// edges an "explain the flow" prompt is actually asking for.
		return 1.0
	}
}
