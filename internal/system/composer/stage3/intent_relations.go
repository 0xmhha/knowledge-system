package stage3

import "github.com/0xmhha/knowledge-system/pkg/system/contract"

// intentToRelations returns the graph relations Stage 3 should traverse
// for the given Intent.
//
// Mapping rationale (baseline). Specific suspicions to verify
// against PR #70 data are documented in
// docs/composer/stage3-scoring.md §3.
//
//   - BugFix:            called_by + calls — trace bug origin and downstream
//   - FeatureAdd:        imports + implements + references — integration
//     points and existing patterns
//   - Refactor:          called_by + calls + references + implements —
//     impact analysis across all reach
//   - ArchExplain:       imports + defines + calls + implements + embeds —
//     structure / flow / composition
//   - TestAdd:           tested_by + calls + called_by — coverage gaps
//   - dependency / scenario
//   - ConcurrencySafety: called_by + references + calls — concurrent
//     reachability + shared state
//   - Security:          called_by + calls + references — trust boundary
//     (taint flow)
//   - DocsUpdate:        defines + implements — API surface only
//   - QAReview/Unknown:  nil (all relations) — broad coverage
//
// Returning nil means "no relation filter" — ckg.Neighbors will traverse
// every known edge type. This is the safe broad default for Intents
// where narrowing would miss the user's actual interest.
func intentToRelations(intent contract.Intent) []contract.Relation {
	switch intent {
	case contract.IntentBugFix:
		return []contract.Relation{
			contract.RelationCalledBy,
			contract.RelationCalls,
		}
	case contract.IntentFeatureAdd:
		return []contract.Relation{
			contract.RelationImports,
			contract.RelationImplements,
			contract.RelationReferences,
		}
	case contract.IntentRefactor:
		return []contract.Relation{
			contract.RelationCalledBy,
			contract.RelationCalls,
			contract.RelationReferences,
			contract.RelationImplements,
		}
	case contract.IntentArchExplain:
		return []contract.Relation{
			contract.RelationImports,
			contract.RelationDefines,
			contract.RelationCalls,
			contract.RelationImplements,
			contract.RelationEmbeds,
		}
	case contract.IntentTestAdd:
		return []contract.Relation{
			contract.RelationTestedBy,
			contract.RelationCalls,
			contract.RelationCalledBy,
		}
	case contract.IntentConcurrencySafety:
		return []contract.Relation{
			contract.RelationCalledBy,
			contract.RelationReferences,
			contract.RelationCalls,
		}
	case contract.IntentSecurity:
		return []contract.Relation{
			contract.RelationCalledBy,
			contract.RelationCalls,
			contract.RelationReferences,
		}
	case contract.IntentDocsUpdate:
		return []contract.Relation{
			contract.RelationDefines,
			contract.RelationImplements,
		}
	case contract.IntentQAReview, contract.IntentUnknown:
		return nil
	}
	return nil
}

// intentToHops returns the graph traversal depth for the given Intent.
//
// Every intent walks three hops (2026-11-16, user decision: one hop says
// almost nothing about where a problem comes from, and at least three hops
// are needed). The cost that kept depth at one or two is now bounded in the
// walk itself: ckgclient.Neighbors expands at most neighborFrontierCap nodes
// per hop, records each neighbour's hop so the expander's
// score / (1 + distance) discounts far neighbours, and shares
// NeighborsPerSeed between the hops so direct neighbours cannot crowd out
// the farther ones.
func intentToHops(contract.Intent) int {
	return 3
}
