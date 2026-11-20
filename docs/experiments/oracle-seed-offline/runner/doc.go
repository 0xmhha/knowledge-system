// Command runner is the offline runner of the oracle-seed-offline experiment
// (docs/experiments/oracle-seed-offline/DESIGN.md).
//
// For every question it opens the cold graph of the question's base commit
// read-only, resolves the frozen blind seeds and the core oracle locations to
// graph nodes, and ranks candidates in two arms that start from the same
// seeds:
//
//   - grep: a word-boundary search for each seed name (the semantics of
//     rg -w) over the files in the graph's build scope. Each matching line is
//     mapped to the innermost declaration node that contains it. The score is
//     the number of matching lines summed over the seeds.
//   - graph: composer stage 3 (internal/system/composer/stage3) as it ships,
//     intent Unknown (every forward edge type plus callers), three hops. The
//     per-seed and final caps are off, so the list holds every node the walk
//     reaches; the order is stage 3's own (1/(1+distance), then span, file,
//     line).
//
// Neither arm lists the seeds themselves. A core location that is also a seed
// is reported as gold_is_seed and counted as found at rank 0 in both arms.
//
// The runner writes one JSON file per question with both full candidate
// lists, and summary.json with the per-question measures (reach, recall@10,
// gold ranks, candidate counts). It never writes to a graph directory.
package main
