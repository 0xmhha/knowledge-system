#!/bin/bash
# NEIGHBOR-RANK-PLAN.md variants: grpc-go both models (cks eval --commit-repo), go-stablenet PR body and Korean arms,
# then report_variant.py (base final) and retrieval_found_diagnosis.py --no-probe per model.
W=/Users/kevin/work/github/0xmhha/auto-coding/knowledge-data/grpc-go-retrieval-eval
E=/Users/kevin/work/github/0xmhha/auto-coding/knowledge-data/go-stablenet-pr-eval-20261025
K=/Users/kevin/work/github/0xmhha/auto-coding/knowledge/knowledge-system
VARIANTS="${VARIANTS:-fix-d4 fix-d45 fix-d456}"
(
  cd $E/scripts
  for tag in $VARIANTS; do
    for arm in $tag ko-$tag; do
      mkdir -p $E/requery/$arm
      extra=""; [ $arm = ko-$tag ] && extra="--prompts $E/prompts-ko.json"
      CKS_DEBUG_TRACE=$E/requery/$arm/debug-trace.jsonl python3 requery.py --bin bin-$tag --index-arm new --tag $arm $extra > $E/requery-$arm.log 2>&1 || echo "stablenet $arm FAILED"
      echo "stablenet $arm done"
    done
  done
) &
cd $W
for tag in $VARIANTS; do
  for m in qwen3-embedding:4b bge-m3; do
    python3 run_qual.py --model $m --tag $tag --commit-repo >> run-$tag.log 2>&1 || echo "grpc $tag $m FAILED"
  done
  python3 report_variant.py --tag $tag --base final > report-$tag.log 2>&1 || echo "report $tag FAILED"
  for m in qwen3-embedding:4b bge-m3; do
    slug=${m/:/-}
    traces=()
    for t in $W/eval-$tag/eval/$slug/*/debug-trace.jsonl; do traces+=(--trace $t); done
    python3 $K/scripts/retrieval_found_diagnosis.py "${traces[@]}" --scenarios $K/docs/spec-driven/operational-preparation/retrieval-eval-grpc-go-v1/inputs/scenarios \
      --model $m --no-probe --out $W/eval-$tag/found-$slug.json >> run-$tag.log 2>&1 || echo "found $tag $m FAILED"
  done
  echo "grpc $tag done"
done
wait
echo "all neighbor-rank variants done"
