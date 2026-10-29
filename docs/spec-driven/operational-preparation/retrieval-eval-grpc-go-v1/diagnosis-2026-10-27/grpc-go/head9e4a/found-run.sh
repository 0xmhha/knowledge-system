#!/bin/zsh
# Found/loss diagnosis of the 9e4a0f0b packs (eval-head9e4a), both models, deep vector probe K=1000.
W=/Users/kevin/work/github/0xmhha/auto-coding/knowledge-data/grpc-go-retrieval-eval
K=/Users/kevin/work/github/0xmhha/auto-coding/knowledge/knowledge-system
for m in qwen3-embedding:4b bge-m3; do
  slug=${m/:/-}
  traces=()
  for t in $W/eval-head9e4a/eval/$slug/*/debug-trace.jsonl; do traces+=(--trace $t); done
  python3 $K/scripts/retrieval_found_diagnosis.py $traces --scenarios $K/docs/spec-driven/operational-preparation/retrieval-eval-grpc-go-v1/inputs/scenarios \
    --model $m --work $W --bin $W/bin-head9e4a --repo $W/grpc-go --src $W/src-eval --deep-k 1000 \
    --out $W/found-head9e4a/found-$slug.json || exit 1
done
echo ALL_DONE
