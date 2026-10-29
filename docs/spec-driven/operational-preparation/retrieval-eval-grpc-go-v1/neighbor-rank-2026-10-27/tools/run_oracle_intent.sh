#!/bin/bash
# DEV diagnostic: bin-final with each scenario's expected intent passed as `intent:` (scenarios-oracle-intent/),
# to see whether a correct intent alone puts more answers in the pack. Not an evaluation arm.
W=/Users/kevin/work/github/0xmhha/auto-coding/knowledge-data/grpc-go-retrieval-eval
K=/Users/kevin/work/github/0xmhha/auto-coding/knowledge/knowledge-system
cd $W
tag=oracle-intent
for m in qwen3-embedding:4b bge-m3; do
  python3 run_qual.py --model $m --tag $tag --bin final --commit-repo --scenarios-dir $W/scenarios-oracle-intent >> run-$tag.log 2>&1 || echo "grpc $tag $m FAILED"
done
python3 report_variant.py --tag $tag --base final > report-$tag.log 2>&1 || echo "report $tag FAILED"
for m in qwen3-embedding:4b bge-m3; do
  slug=${m/:/-}
  traces=()
  for t in $W/eval-$tag/eval/$slug/*/debug-trace.jsonl; do traces+=(--trace $t); done
  python3 $K/scripts/retrieval_found_diagnosis.py "${traces[@]}" --scenarios $K/docs/spec-driven/operational-preparation/retrieval-eval-grpc-go-v1/inputs/scenarios \
    --model $m --no-probe --out $W/eval-$tag/found-$slug.json >> run-$tag.log 2>&1 || echo "found $tag $m FAILED"
done
echo "oracle-intent done"
