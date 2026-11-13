# Retrieval accuracy roadmap

This page is the starting point for retrieval accuracy work: what the system builds and reads today, why the
remaining misses happen, which algorithms could fix them, and in what order they are tried. Keep it current. When
an item is tried, update its status and link the measurement; when a decision changes, add a row to the decision
log instead of rewriting history.

Related: `docs/RETRIEVAL-PIPELINE.md` draws the query pipeline alone and compares it with the Transformer.
This page adds the index-building side, the join between the two stores, and the improvement plan.

Last reviewed: 2026-11-13 (main at #142, plus `ckg verify`).

## 1. Figure: the whole system

The Transformer figure has two towers: an encoder that turns the input into representations once, and a decoder
that reads those representations to produce the output. CKS has the same split. The offline build is the encoder:
it turns source code into two stores. The online query is the decoder: it reads both stores to select the evidence
for one question. Unlike the paper, nothing is learned end to end; each box is a hand-written rule, and the two
towers meet through one join key.

```
 ENCODER SIDE (offline, once per commit)                       DECODER SIDE (online, once per question)

 source tree @ commit                                           question (Korean or English)
      |                                                              |
      +-------------------------+                                    v
      |                         |                           +-------------------+
      v                         v                           | 0 query rewrite   |  calling agent writes
 +-----------+            +-------------+                   |   -> code_query   |  code vocabulary (English)
 | ckg parse |            | ckv chunk   |                   +-------------------+
 | AST per   |            | one chunk   |                   | 1 intent          |  bug_fix / arch_explain / ...
 | language  |            | per symbol, |                   +-------------------+
 +-----------+            | split long  |                   | 2 Stage 1 recall  |  vector k=20 + BM25 rerank,
      |                   | functions,  |                   |                   |  2 rounds, keywords (cap 8)
      v                   | file header,|      vector       |                   |<------ ckv
 +-----------+            | docs, PRs,  |      search       +-------------------+
 | resolve   |            | invariants  |  ---------------> | 3 Stage 2 rank    |  RRF over vector, BM25 and
 | names,    |            +-------------+                   |                   |  symbol lists:
 | qualify   |                  |                           |                   |  sum w / (60 + rank)
 | ids       |                  v                           +-------------------+
 +-----------+            +-------------+                   | 4 Stage 3 expand  |  1-hop edges per seed:
      |                   | embed text  |                   |                   |  seed x w_rel / (1 + dist)
      v                   | = rule      |      edges        |                   |<------ ckg
 +-----------+            |  prefix     |  ---------------> +-------------------+
 | edges     |            | (+ optional |                   | 5 Stage 4 select  |  one candidate per function,
 | calls,    |            |  LLM line)  |                   |                   |  seed quota 10, reserves,
 | implements|            | + body      |                   |                   |  7200-token budget, cap 12
 | reads/    |            +-------------+                   +-------------------+
 | writes_   |                  |                           | 6 sanitize, pack  |  bodies + edges +
 | field,    |                  v                           |                   |  cut_candidates (no body)
 | changed_in|            +-------------+                   +-------------------+
 | ...       |            | embed       |                            |
 +-----------+            | bge-m3 or   |                            v
      |                   | qwen3       |                     diagnosing agent
      v                   +-------------+
 +-----------+                  |
 | code graph|  <--- join ----   +--> vector index (chunks + vectors)
 | nodes,    |  canonical_id:
 | edges     |  one ckg node per chunk, chosen by line span
 +-----------+

 Curated knowledge (domain packs, glossaries, invariants) enters both towers as extra chunks and is
 placed by the Stage 4 knowledge reserve.
```

How to read it:

- **Encoder left, ckg.** Each language parser reads the AST, resolves names to import-path-qualified ids and
  writes nodes and typed edges. The graph already has many edge kinds (for example `implements`,
  `reads_field`, `writes_field`, `changed_in`), but Stage 3 follows only the relations in the cks contract
  (`calls`, `called_by`, `implements`, `imports`, `references`, `tested_by`, `embeds`, `defines`,
  `uses_type`).
- **Encoder right, ckv.** Code is cut into chunks: one per function or type, several for a long function, plus a
  file header. PR descriptions, commit messages, docs and invariants become chunks too. Each chunk is embedded as a
  rule-based prefix (language, file, symbol, kind) plus the raw body. An LLM-written one-line prefix exists but is
  off by default; it did not beat the rule prefix on the small corpus it was measured on.
- **Join.** While building ckv, each chunk with a line span is matched to the ckg node whose span fits best,
  and that node's `canonical_id` is copied into one string field on the chunk. Chunks without a span (PR,
  commit, doc, curated invariant) get no join key.
- **Decoder.** Stage 1 reads ckv, Stage 3 reads ckg, Stage 2 and Stage 4 combine scores. Each stage keeps a
  fixed number of candidates, so a function lost early is never seen again. This is where the 2026-11 misses
  happen (section 2).

## 2. Why the remaining answers are missed

Measured on go-stablenet (11 PRs, Korean questions, two embedding arms) after #135:

| Arm | Owning function in pack bodies | Misses | Cause |
|---|---|---|---|
| bge-m3 | 8 of 11 | pr-63, pr-67, pr-73 | pr-63: not retrieved at all. pr-67 and pr-73: retrieved but cut at seed ranks 19 and 16 |
| qwen3 | 10 of 11 | pr-63 | not retrieved at all |

The misses fall into three classes:

1. **Vocabulary gap.** A Korean question has no identifiers of its own, so the keyword channel is empty and only
   the vector search can find the code. Code chunks are embedded as English identifiers and comments.
2. **Structure, not words.** pr-63 asks where a new hardfork registers its system contract upgrade. The owning code
   is an extension point; no word in the question names it, and no edge leads to it from what is found.
3. **Embedding model.** pr-67 and pr-73 are found by qwen3 on the same data and lost by bge-m3.

A common belief is that the index favours words that appear often in the code. BM25 does the opposite (IDF lowers
frequent terms). The real limit is that the stores hold only what the code literally says: identifiers, comments
and a location prefix. Nothing states what a function is for, in the words a person asking about it would use.

## 3. Correction to the Transformer analogy

The paper's six layers are not six repetitions that push a probability up. They are six different layers, each
with its own weights, and all of those weights are learned from labelled pairs. The gain comes from the learning
signal. Adding hand-written stages without such a signal adds knobs; two keyword experiments in 2026-11 lost PRs
for that reason.

CKS does have a learning signal, larger than the evaluation labels: git history pairs each PR description with
the functions it changed. Section 4 uses it, under the leakage rule in section 6.

## 4. Proposals

Status values: `proposed`, `in progress`, `measured`, `adopted`, `rejected`. Each row names the miss class
(section 2) it targets.

### 4.0 Data correctness first

Every proposal below assumes the built stores are right. A read-only check of one go-stablenet graph on
2026-11-13 found call and type edges that point to the wrong declaration, so the stores are verified and fixed
first. For Go the reference is the compiler front end: `ckg verify` compares the graph with go/types
([Go graph verification](./graph/GO-GRAPH-VERIFICATION.md)). On the 11 go-stablenet base commits it gives static
call recall 0.977, interface call recall 0.979 and type-reference recall 0.544, with 7 misresolved calls, 357
misresolved type references and 931 call edges go/types does not support.

| ID | Proposal | Status | Cost and risk |
|---|---|---|---|
| D1 | `ckg verify`: measure the Go graph against go/types | adopted (2026-11-13) | none on retrieval; Go only |
| D2 | Resolve calls and type references by the go/types object (canonical id) instead of the short `pkg.Name`; drop the suffix fallback | proposed | changes many edges; every index must be rebuilt |
| D3 | Keep function-local declarations out of the package-level id space | proposed | canonical ids of some nodes change; ckv joins follow on rebuild |
| D4 | Record calls inside function literals and calls to methods of generic types | proposed | more edges compete for Stage 3 slots |
| D5 | Emit type references from bodies, interface signatures and aliases; emit `reads_field` and `references` | proposed | larger graph; Stage 3 relation weights may need retuning |
| D6 | ckv join: do not bind file-header chunks to the first declaration; split grouped `var (...)` blocks | proposed | small |
| D7 | Composer and data agree: Stage 3 asks for relations that no graph contains (`references`) | proposed | needs D5 or a change to the relation sets |

Each fix is judged first by `ckg verify` (or the matching store check), then by whether evidence packs change on
the go-stablenet cases, using the stage trace (`CKS_DEBUG_TRACE`) to find where an answer is lost.

### 4.1 Join between graph and vector (canonical_id)

Today one chunk carries at most one `canonical_id`. Real relations are not one to one:

| Relation | Example | Today |
|---|---|---|
| chunk 1 : symbol N | a PR changes several functions; a type chunk holds several methods; a doc section covers several functions | no link, or the smallest node only |
| chunk N : symbol 1 | the parts of a split function; code, summary and test of one function | same string repeated; scores are not combined by symbol |
| id 1 : node N | the same function under two build tags; an interface method and its implementations | one picked, or left ambiguous |
| id 1 : commit N | a function renamed or moved | different ids, not connected |

| ID | Proposal | Status | Cost and risk |
|---|---|---|---|
| J1 | Link table `chunk_symbol_links(chunk_id, canonical_id, role, weight, source)` with roles `defines`, `part_of`, `contains`, `changes`, `describes`, `constrains`, `tests`. Keep the scalar `canonical_id` as the `defines` row so current readers keep working. Stage 2 then combines scores per symbol, not per span | proposed | ckv schema change, manifest digest change; PR `changes` rows need diff-to-function mapping |
| J2 | Alias table `symbol_aliases(old_id, new_id, commit)` from git rename detection and body similarity | proposed | false merges when two functions look alike |
| J3 | Alternative to J1: put PR, doc and summary chunks into ckg as nodes with `describes` / `changes` edges so Stage 3 follows them | proposed (alternative) | mixes code structure and documents in one graph; larger graph; ckg schema change |

### 4.2 Vector knowledge (ckv)

| ID | Proposal | Targets | Status | Cost and risk |
|---|---|---|---|---|
| V1 | Symbol summary: an LLM writes, for each function or type, what it is for and when one would change it, in English and Korean. The summary is a separate chunk with the function's own span, so a hit on the summary cites the code | 1, 2 | tried, not adopted as built (section 5.1) | LLM time at build (about 5 s per summary; about 7 h for the first go-stablenet commit at 10+ lines); a wrong summary pulls retrieval to the wrong code |
| V2 | Expected questions (doc2query): a few questions per function that the function answers | 1 | proposed | generated questions may resemble evaluation questions and inflate scores |
| V3 | Strip boilerplate (repeated error handling) from the embedding input so names and comments weigh more | 1 | proposed | small effect likely |
| V4 | Fine-tune the embedder contrastively on (PR description, changed function) pairs with same-file negatives | 1, 3 | proposed | needs GPU and a training pipeline; strict time split required |

V1 differs from the existing LLM prefix: the prefix is one English sentence added in front of the code body, so
the body still dominates the vector. A separate summary chunk is a vector of the purpose alone, in the language of
the question.

### 4.3 Graph knowledge (ckg)

| ID | Proposal | Targets | Status | Cost and risk |
|---|---|---|---|---|
| G1 | Registration and extension-point edges: map-literal registration, function values passed as arguments, switch over an enum; mark nodes that "a new X" must touch | 2 | proposed | parser work per language; which of these ckg already emits is not yet checked |
| G2 | Co-change edges from git history, weighted by frequency | 2 | proposed | old history adds noise; evaluation PRs must be excluded |
| G3 | Expose ckg edges Stage 3 does not follow today (`reads_field`, `writes_field`, `changed_in`) through the cks contract | 2 | proposed | more neighbors compete for the same slots |
| G4 | Community detection over the graph plus one summary per community (GraphRAG style) for broad questions | 1, 2 | proposed | summary cost; must be rebuilt as code changes |

### 4.4 Iterative scoring at query time

| ID | Proposal | Targets | Status | Cost and risk |
|---|---|---|---|---|
| Q1 | Stage 3 as personalized PageRank from the seeds instead of one 1-hop pass: a function pointed to by several seeds rises | 3 | proposed | hub functions (logging, errors) absorb score unless degree-normalised; useless when no seed is near the answer |
| Q2 | Pseudo-relevance feedback: mix the vectors of the first round's top bodies into a second vector query | 1 | proposed | drifts further when round one is wrong |
| Q3 | Cross-encoder rerank of the final candidates, reading question and body together | 1, 3 | proposed | one model call per candidate; needs a model good at Korean and code |

### 4.5 Evidence precision and marked gaps

The proposals above all raise recall: they try to bring the owning function into the pack. This group asks the
opposite question. A diagnosing agent may do better with three right bodies and an explicit "not covered here"
than with twelve bodies of which most are unrelated: a wrong body pulls the reasoning toward the wrong code,
while a marked gap can be filled with grep or a file read. The pack today always fills up to 12 bodies, has no
score floor, and gives no sign of how confident it is; an unknown intent widens the fan-out instead of saying it
is unknown. The evaluation counts only whether the owning function is in the pack, so a change that adds bodies
always looks no worse. The V1 indexer-go loss is a measured case of the failure this group targets: bodies whose
rank the summaries raised pushed the owning function out.

Nothing in this repository has measured whether the agent is in fact misled by extra bodies, so P1 comes before
any change to body selection.

| ID | Proposal | Targets | Status | Cost and risk |
|---|---|---|---|---|
| P0 | Measure pack precision next to recall: `cks eval` reports `answer_body_count`, `body_precision` and `other_body_tokens` per scenario (EVAL-CONTRACT.md). Report them for the go-stablenet and indexer-go arms, and the curve over pack sizes 3, 6 and 12 with `CKS_PACK_MAX_BODIES` | measurement | in progress (fields and the pack-size override added 2026-11-09) | needs the frozen v2 / indexer-go inputs, which live in the evaluation workspace |
| P1 | Agent-level A/B: the same diagnosis tasks with a 12-body pack and a 3-body pack plus `cut_candidates`, judged by fixed criteria (the C1-C4 frame of projects/stablenet/eval/diagnosis-procedure) | the principle itself | proposed; design open, see below | paid agent runs (about 1 to 1.5 USD each on the recorded runs); run-to-run variation needs repeats |
| P2 | Variable body count: stop where the score drops sharply (relative gap, not an absolute threshold) and list the rest as `cut_candidates` | precision | proposed; only if P1 shows an effect | can lose owning functions that sit below the gap; judged by D6 |
| P3 | Say what is not known: a pack-level confidence signal (for example the top-1 / top-2 score gap), and an unknown intent reported as unknown instead of widening the fan-out | precision | proposed; only if P1 shows an effect | absolute scores differ by embedding model (the 0.55 intent threshold problem), so the signal must be relative; the intent part showed little pack effect (+1 / -1) |

P1 design, to be fixed before any run:

- **Cases.** The four recorded diagnosis cases (PR-56, 70, 75, 77) passed C1 in every run with the 12-body pack,
  so they leave little room to show harm from extra bodies. The design should add cases where the pack is weak:
  the go-stablenet PRs whose owning function was not in the pack or was cut (pr-63, pr-67 and pr-73 with
  bge-m3), with symptom-only problem texts and criteria written before the run.
- **Arms.** A: default pack (12 bodies). B: `CKS_PACK_MAX_BODIES=3` (neighbor reserve scales to 0), which keeps
  `cut_candidates` as the marked gap. Same index, same cks binary, same agent and procedure.
- **Measures.** C1 root cause and C2 sites as in the recorded cases, plus the number of tool calls and tokens the
  agent spends, and whether a wrong site from the pack appears in the diagnosis.
- **Repeats and budget.** At least three runs per case and arm, and a total cost cap; both are user decisions.
- **Leakage.** Problem texts state only what an operator observes; the index is built at the PR base commit.

## 5. Order of work

0. **Data correctness (4.0) first** (user decision 2026-11-13). Verify and fix the built data, then debug the
   logic that combines it with the stage trace, then continue with the items below. Reason: a wrong edge or join
   makes every later measurement unreliable.
1. **V1 symbol summaries** (started 2026-11-05). Smallest change that targets miss classes 1 and 2. It needs no
   schema change: the summary chunk carries the function's span, so Stage 2 merges it with the code hit by citation.
2. **J1 link table** once V1 shows a gain, so summaries, PR chunks and code combine per symbol rather than per span.
3. **Q1 personalized PageRank** as a cheap query-only experiment, any time.
4. G1 to G3, then V4 when a time-split training set exists.
5. **P0 pack precision** alongside every measurement from now on; **P1 agent A/B** once its cases, repeats and
   budget are decided. P2 and P3 wait for the P1 result.

### 5.1 V1 state (2026-11-09)

Code: `ckv build` / `ckv reindex` flags `--llm-summary-model` and `--llm-summary-min-lines` (default 3), package
`internal/vector/summary`. Off unless the model flag is set. Measured with gemma3:12b, prompt v1, symbols of 10 or
more lines, both embedding arms reading the same summaries.

| set | arm | before | with summaries | gains | losses |
|---|---|---|---|---|---|
| go-stablenet (11 PRs) | bge-m3-eq | 8 | 9 | 1 | 0 |
| go-stablenet (11 PRs) | qwen3-eq | 10 | 10 | 0 | 0 |
| indexer-go (15 of 26 PRs) | bge-m3-eq | 7 | 6 | 0 | 1 |
| indexer-go (15 of 26 PRs) | qwen3-eq | 8 | 9 | 1 | 0 |

The indexer-go run stopped after 15 of 26 base commits by user decision. bge-m3 fails the D6 rule on those 15
(one loss, no gain). In the lost PR all three owning functions had summaries; the owning function was displaced
from the pack by other bodies whose rank the summaries raised. The 11 remaining indexer-go PRs have not been run
with summaries and are still unseen for V1.

Known defects in the summary check, found from build counts and not from judged results:

- Rejection rate is 1.9% on go-stablenet and 12% on indexer-go. Most indexer-go rejections come from gqlgen output
  (`// Code generated ... DO NOT EDIT.`), which should not be summarized at all.
- Identifiers are matched case-sensitively, so prose terms such as "GraphQL" are rejected when the code spells
  them `graphql`. Valid resolver summaries were dropped for this reason.
- The check catches invented names only; how many kept summaries are wrong in meaning is not measured.

Build cost options, none implemented: skip generated files; send summary requests in parallel; build the
content-keyed cache once on a powered machine and share it; cap prompt and output length; a smaller model after a
quality comparison.

How to resume, with paths and commands: [symbol summary handoff](./vector/SYMBOL-SUMMARY-HANDOFF.md).

The check fixes were approved on 2026-11-09 and are in the code: files with the Go generated-code header get no
summaries, identifiers are matched case-insensitively, and an unlabelled two-line answer (one English, one Korean
line) is accepted. The summary model is now gemma4:12b, chosen by the user on the published benchmarks (MMLU Pro
77.2%, LiveCodeBench v6 72.0%, MMMLU 83.4% on the Gemma 4 model card; no Gemma 3 12B column there). Requests send
`think: false`.

Probe on this machine (Apple M4, 24 GB), 11 indexer-go functions of 10-80 lines, prompt v1:

| model | mean / max seconds | kept by the old check | kept by the fixed check |
|---|---|---|---|
| gemma3:12b | 11.8 / 15 | 11 | not rerun |
| gemma4:12b, thinking off | 7.9 / 11 | 7 (4 without EN:/KO: labels) | 11 |
| gemma4:12b, Ollama default | 119.3 / 168 | 7 (4 without labels) | not rerun |

Eleven symbols show the format and speed, not summary quality. Next: build the 11 remaining indexer-go commits
with gemma4:12b and the fixed check and judge them as an unseen sample. Since the model changed, the 15 seen
commits and go-stablenet need summaries from gemma4:12b as well before V1 can be judged as one change.

## 6. Measurement rules

- Judge each change with the D6 rule ([decision record](./decisions/20261103-indexer-go-eval/decision.md)): report the gain on go-stablenet, require no loss on indexer-go (losses do
  not exceed gains in either arm), and report the risk subset.
- indexer-go has been looked at 6 times (the sixth on 15 of 26 PRs, V1). A change that alters body selection should be confirmed on a fresh
  holdout before it is called general.
- **Leakage.** Anything built from history or by an LLM (V1, V2, V4, G2) must not see information after the
  evaluation PR's base commit. Summaries are generated from the code at the base commit only, never from PR text.
- **Generated knowledge must be checked.** A summary that names an identifier absent from the body is dropped.
- A change that leaves pack bodies unchanged (for example `cut_candidates`) needs no indexer-go run.

## 7. Decision log

| Date | Decision | Why |
|---|---|---|
| 2026-11-04 | Knowledge reserve placed outside the citation cap (#134) | it took the neighbor slots |
| 2026-11-04 | Vector and graph spans of one function are one candidate (#135) | the same function used two slots |
| 2026-11-04 | Keyword seeds from `code_query` not adopted | lost PRs on go-stablenet twice |
| 2026-11-05 | Pack lists cut seeds as `cut_candidates` (#137) | two bge misses become visible by name; bodies unchanged |
| 2026-11-05 | Start with V1 symbol summaries, then J1 | targets the two miss classes the other proposals cannot reach |
| 2026-11-05 | V1 summarizes symbols of 10 or more lines | the 3-line run was projected at about 16 h of generation on go-stablenet |
| 2026-11-06 | indexer-go V1 run stopped at 15 of 26 commits and judged there | user decision; V1 not adopted as built (bge-m3 lost one PR) |
| 2026-11-09 | Summary model gemma4:12b, thinking off; check fixes (generated files, case-insensitive identifiers, unlabelled two-line answers) | user decision on the benchmarks; thinking made each summary take about 2 minutes; the label fix recovered the 4 of 11 answers gemma4 wrote without labels |
| 2026-11-13 | Verify and fix the built data before any new proposal; start with `ckg verify` (D1) | user decision; a read-only check found wrong call and type edges, and the compared code-graph papers do not measure their graphs at all |
| 2026-11-09 | Add 4.5 evidence precision and marked gaps; start with P0, then P1 | user decision; the principle "few right bodies plus a marked gap beat many wrong ones" is not measured here, and every metric so far rewards adding bodies |
