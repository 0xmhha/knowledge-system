# Graph expansion scoring

Current implementation: [expander](../../../internal/system/composer/stage3/expander.go), [weights](../../../internal/system/composer/stage3/relation_weights.go), [aggregation](../../../internal/system/composer/stage3/scoring.go), [intent relations](../../../internal/system/composer/stage3/intent_relations.go).

The neighbor score is seed.Score × relationWeight(intent, relation) / (1 + distance). For arch_explain, defines uses 0.5 and imports 0.7; other relations and other intents use 1.0. This demotes structural fan-out while retaining its evidence. It does not establish an independently optimal ranking.

Default bounds are 10 expanded seeds, 50 neighbors per seed and 50 final deduplicated neighbors. Expansion ignores duplicate edges and merges the same target with the maximum competing score. Sources retains the contributing seed/relation/distance paths. Summing every incoming score would let popular hubs dominate; max can also underreward a truly relevant multi-path target.

Relations/hops are selected per intent in intent_relations.go. Optional type-neighbor insertion is a separate request/config contract and defaults off. The graph-expansion score is not an answer-confidence or acceptance verdict.

A future scoring change needs approved inputs, a frozen comparison and per-intent recall/noise/cost measurements before choosing exponential/logarithmic decay, count bonuses or wider relation tuning. Old PR-specific plans are historical context, not an outstanding accepted implementation mandate. See [evaluation status](../../../docs/spec-driven/EVALUATION-STATUS.md) and [remaining work](../../../docs/REMAINING-WORK.md).
