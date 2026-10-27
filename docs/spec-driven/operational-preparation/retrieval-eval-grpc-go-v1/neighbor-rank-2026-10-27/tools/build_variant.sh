#!/bin/bash
# Build cks from the knowledge-system working tree (HEAD + uncommitted diff) into bin-<tag> for grpc-go and go-stablenet.
set -e
tag=$1
K=/Users/kevin/work/github/0xmhha/auto-coding/knowledge/knowledge-system
W=/Users/kevin/work/github/0xmhha/auto-coding/knowledge-data/grpc-go-retrieval-eval
E=/Users/kevin/work/github/0xmhha/auto-coding/knowledge-data/go-stablenet-pr-eval-20261025
d=$W/bin-$tag
mkdir -p $d
(cd $K && go build -o $d/cks ./cmd/cks && git diff HEAD > $d/fix.diff && git rev-parse HEAD > $d/head.txt)
cp $W/bin-final/ckg $W/bin-final/ckv $d/
(cd $d && shasum -a 256 cks ckg ckv > sha256.txt && echo "cks built from $(cat head.txt) + fix.diff; ckg/ckv copied from bin-final" >> sha256.txt)
rm -rf $E/bin-$tag && cp -R $d $E/bin-$tag
echo built $tag
