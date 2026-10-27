#!/bin/bash
# Run the FOUND-FIX-PLAN.md variants on grpc-go (sequential, both models) and go-stablenet (parallel stream).
W=/Users/kevin/work/github/0xmhha/auto-coding/knowledge-data/grpc-go-retrieval-eval
E=/Users/kevin/work/github/0xmhha/auto-coding/knowledge-data/go-stablenet-pr-eval-20261025
VARIANTS="fix-d23"
(
  cd $E/scripts
  for tag in $VARIANTS; do
    mkdir -p $E/requery/$tag
    CKS_DEBUG_TRACE=$E/requery/$tag/debug-trace.jsonl python3 requery.py --bin bin-$tag --index-arm new --tag $tag > $E/requery-$tag.log 2>&1 || echo "stablenet $tag FAILED"
    echo "stablenet $tag done"
  done
) &
cd $W
for tag in $VARIANTS; do
  for m in qwen3-embedding:4b bge-m3; do
    python3 run_qual.py --model $m --tag $tag >> run-$tag.log 2>&1 || echo "grpc $tag $m FAILED"
  done
  echo "grpc $tag done"
done
wait
echo "all d23 variants done"
