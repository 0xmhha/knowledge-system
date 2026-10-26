# Factor ablation on HEAD 436ff2b0 (written before any ablation run)

Goal: find which pipeline factors drive CKS retrieval quality, one factor at a time.

Binary: bin-abl/cks = HEAD 436ff2b0 + ablation.diff. Each switch is an environment variable; with none
set the binary must behave as HEAD (checked by abl-base against eval-fix5152, same source as HEAD).

| tag | switch | factor it removes or changes |
|---|---|---|
| abl-base | none | control |
| abl-s1norr | CKS_ABL_S1_NORERANK=1 | Stage 1 ckv BM25 rerank |
| abl-nohitkw | CKS_ABL_NO_HIT_KW=1 | keywords mined from ckv hits (hit_symbol, hit_file) in Stage 2 |
| abl-nokw | CKS_ABL_NO_KW_LISTS=1 | every Stage 2 keyword list (ckv list only) |
| abl-nr0 | CKS_ABL_NEIGHBOR_RESERVE=0 | Stage 4 neighbor reserve (seed quota) |
| abl-mc20 | CKS_ABL_MAX_CITATIONS=20 | Stage 4 pack size 12 -> 20 |
| abl-it0 | CKS_ABL_INTENT_THRESHOLD=0 | intent Unknown threshold (always take the nearest intent) |
| abl-notd | CKS_ABL_NO_TEST_DEMOTION=1 | Stage 2 test-file demotion |

Data: grpc-go 100 scenarios x {qwen3-embedding:4b, bge-m3}, existing indexes; go-stablenet 12 PRs (qwen3, new arm).

Reading rules, fixed now:
1. Effect = metric(variant) - metric(abl-base), per model. grpc-go uses cks_any_commit hit@1 and MRR;
   go-stablenet uses file recall@10 and MRR.
2. A factor is key when |delta MRR| >= 0.02 on both grpc-go models in the same direction, or
   |delta MRR| >= 0.03 on go-stablenet. Smaller moves are reported as noise-level.
3. Per-scenario better/worse counts and the debug-trace report back each claim; a metric move without
   a matching trace mechanism is reported as unexplained.
4. abl-mc20 changes pack size, so it is also read against tokens used; it is a capacity factor, not a
   ranking factor.
5. abl-it0 is read for intent accuracy too; expected_intent is AI-labelled and not human-reviewed.
6. No default changes from this ablation alone; results feed a later decision.
