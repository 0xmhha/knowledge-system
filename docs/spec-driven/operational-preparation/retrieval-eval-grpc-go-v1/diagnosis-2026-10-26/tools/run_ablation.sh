#!/bin/bash
# Run every ablation variant of ABLATION-PLAN.md on grpc-go (sequential) and go-stablenet (parallel stream).
W=/Users/kevin/work/github/0xmhha/auto-coding/knowledge-data/grpc-go-retrieval-eval
E=/Users/kevin/work/github/0xmhha/auto-coding/knowledge-data/go-stablenet-pr-eval-20261025
VARIANTS="abl-base: abl-s1norr:CKS_ABL_S1_NORERANK=1 abl-nohitkw:CKS_ABL_NO_HIT_KW=1 abl-nokw:CKS_ABL_NO_KW_LISTS=1 abl-nr0:CKS_ABL_NEIGHBOR_RESERVE=0 abl-mc20:CKS_ABL_MAX_CITATIONS=20 abl-it0:CKS_ABL_INTENT_THRESHOLD=0 abl-notd:CKS_ABL_NO_TEST_DEMOTION=1"
(
  cd $E/scripts
  for v in $VARIANTS; do
    tag=${v%%:*}; env=${v#*:}
    mkdir -p $E/requery/$tag
    env $env CKS_DEBUG_TRACE=$E/requery/$tag/debug-trace.jsonl python3 requery.py --bin bin-abl --index-arm new --tag $tag > $E/requery-$tag.log 2>&1 || echo "stablenet $tag FAILED"
    echo "stablenet $tag done"
  done
) &
cd $W
for v in $VARIANTS; do
  tag=${v%%:*}; env=${v#*:}
  arg=""; [ -n "$env" ] && arg="--env $env"
  for m in qwen3-embedding:4b bge-m3; do
    python3 run_qual.py --model $m --tag $tag --bin abl $arg >> run-$tag.log 2>&1 || echo "grpc $tag $m FAILED"
  done
  echo "grpc $tag done"
done
wait
echo "all ablations done"
