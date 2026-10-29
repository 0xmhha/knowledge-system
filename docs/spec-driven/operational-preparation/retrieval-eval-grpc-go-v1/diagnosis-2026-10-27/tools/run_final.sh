#!/bin/bash
# Final measurement of the committed tree: grpc-go both models (cks eval --commit-repo), go-stablenet PR body and Korean questions.
W=/Users/kevin/work/github/0xmhha/auto-coding/knowledge-data/grpc-go-retrieval-eval
E=/Users/kevin/work/github/0xmhha/auto-coding/knowledge-data/go-stablenet-pr-eval-20261025
(
  cd $E/scripts
  for arm in final ko-final; do
    mkdir -p $E/requery/$arm
    extra=""; [ $arm = ko-final ] && extra="--prompts $E/prompts-ko.json"
    CKS_DEBUG_TRACE=$E/requery/$arm/debug-trace.jsonl python3 requery.py --bin bin-final --index-arm new --tag $arm $extra > $E/requery-$arm.log 2>&1 || echo "stablenet $arm FAILED"
    echo "stablenet $arm done"
  done
) &
cd $W
for m in qwen3-embedding:4b bge-m3; do
  python3 run_qual.py --model $m --tag final --commit-repo >> run-final.log 2>&1 || echo "grpc final $m FAILED"
done
echo "grpc final done"
wait
echo "all final done"
