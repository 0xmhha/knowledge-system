# Symbol summary chunks (V1): state and how to continue

State as of 2026-11-09 (updated after the check fixes and the switch to gemma4:12b). Results, known defects and the order of work are in
[the accuracy roadmap, section 5.1](../ACCURACY-ROADMAP.md). This page lists what a new session needs to resume the
experiment: where the code, indexes and scripts are, what is waiting for a user decision, and the steps once it is
made.

## 1. Code

- Package `internal/vector/summary`: prompt (`BuildPrompt`, `PromptVersion` v1), eligibility (`Eligible`: symbol
  chunk with a span, not a test, not markdown, at least the minimum lines), validation (`Validate`) and the disk
  cache (key: prompt version, model, symbol name, body; an empty file is a cached rejection; generation errors are
  not cached).
- Wiring: `internal/vector/build/pipeline.go` (`resolveSummarizer`, `reportSummaryStats`),
  `internal/vector/build/builder.go` and `internal/vector/build/reindex.go` (summaries are added per file before
  embedding).
- Flags on `ckv build` and `ckv reindex`: `--llm-summary-model` (empty disables) and `--llm-summary-min-lines`
  (default 3). The cache lives in `.ckv-summary-cache` under the output directory.
- The query path does not call the summary model; only the embedding model embeds the question.

## 2. Evaluation workspace

Everything below is in knowledge-data/retrieval-eval-v2, outside this repository. Raw evaluation records stay
there and are not committed.

| what | where (relative to the workspace) |
|---|---|
| pre-registered setup, decision rule, run log, results | EXP-symbol-summary.md |
| binaries used | bin-sum/ (ckg, cks, ckv built from 7fd035b8 on a clean tree; BUILD.txt, bin-sum-sha256.txt) |
| index chain | tools/chain_sum.py (gemma3:12b, MIN_LINES = 10; arms write vec/ARM-sum; one shared cache per project) |
| retrieval run | tools/run_sum.py (`--built-only` runs only commits recorded for both arms in PROJECT/chain-sum.json) |
| judge | tools/judge_d6.py (owning function among the final pack bodies of the last get_for_task call) |
| summary cache | PROJECT/summary-cache-gemma3-12b/ |
| go-stablenet | all 11 base commits built; judged against tag tc-2026-11 (judge-d6-2026-11-sum.json) |
| indexer-go | 15 of 26 base commits built (indexer-go/chain-sum.json); judged against gf-2026-11 (judge-d6-2026-11-sum-indexer-go-partial.json) |

The indexer-go vector directory of the 16th commit (2c0daafa) is partial because the chain was stopped during it.
The chain script deletes and rebuilds it on resume, since that commit is not in chain-sum.json.

## 3. Decided on 2026-11-09

1. Done: `AppendForFile` skips files with the Go generated-code header (`// Code generated ... DO NOT EDIT.`),
   counted as `generated_files_skipped` in the build log.
2. Done: `Validate` matches identifiers case-insensitively and accepts an unlabelled answer of exactly two lines,
   one English and one Korean.
3. The summary model is gemma4:12b and the Ollama generator sends `think: false` (user decision on the published
   benchmarks). With thinking on, a summary took about 2 minutes and exceeded the 60 s request timeout.
4. Next: build the 11 remaining indexer-go commits and judge those 11 PRs as an unseen sample, reported apart from
   the 15 already seen.

The defects were found from build counts and an 11-symbol probe, not from judged results, so fixing them is not
tuning on the holdout. The 15 seen indexer-go PRs and go-stablenet were summarized by gemma3:12b with the old check;
record that difference, or rebuild them with gemma4:12b, before calling V1 one change.

## 4. Steps to resume

1. Done in this repository (check fixes and `think: false`).
2. Rebuild the workspace binaries from the new commit on a clean tree and update BUILD.txt and the sha256 file.
   `PromptVersion` stays v1 because the prompt did not change. The cache is keyed by model, so gemma4:12b starts
   with an empty cache; gemma3:12b rejections need not be deleted.
3. Write the change, the model and the decision rule into EXP-symbol-summary.md before running, and pass
   gemma4:12b to `--llm-summary-model` (tools/chain_sum.py hard-codes gemma3:12b).
4. Resume the chain from the workspace (it skips recorded commits; about 3 to 5 hours for 11 commits):

   ```
   caffeinate -i python3 tools/chain_sum.py --project indexer-go --repo "$PWD/clones/indexer-go" \
     --commits indexer-go-commits.json --arms bge-m3,qwen3-cb > indexer-go-sum-chain.log 2>&1
   ```

5. Run retrieval on the new commits (the run script skips commits that already have results), then judge:

   ```
   python3 tools/run_sum.py --project indexer-go --bin bin-sum --set scenarios-c-ko --tag sum-2026-11
   python3 tools/judge_d6.py --base gf-2026-11 --new sum-2026-11 --projects indexer-go
   ```

## 5. Machine and power

From 2026-11-09 simulation runs happen on the Apple M4 machine with 24 GB (user decision). Its evaluation
workspace has not been set up yet: section 2 describes the M2 Max layout. The indexer-go clone is at
~/Work/github/indexer-go. Ollama holds bge-m3, qwen3-embedding:4b and gemma4:12b; gemma4:12b loads at about 12 GB,
so it and one embedding model fit, but not a second large model.

Under load the M2 Max drew about 98 W, the adapter's rating, so its battery drained even while charging. Earlier
runs paused the workspace binaries with SIGSTOP at 20% battery and resumed at 40%. The power draw of the M4
machine has not been measured.

## 6. Other open items

- The planned gemma3:12b and gemma4:12b comparison on 200 symbols was replaced by the user's benchmark-based
  choice and the 11-symbol probe in the roadmap 5.1.
- Record the summary model in the vector manifest; today only the cache key carries it.
- Measure how many kept summaries are wrong in meaning by reading a sample of 50 against the code.
- Build cost options from the roadmap: skip generated files, parallel requests, a shared cache built once on a
  powered machine, prompt and output caps, a smaller model after the comparison.
