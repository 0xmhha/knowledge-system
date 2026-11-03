#!/usr/bin/env bash
# Build the per-case source clone (baseline ancestors only), graph and vector, and the per-arm working copies.
# usage: setup.sh <case> <base-sha>
set -euo pipefail
case=$1 base=$2
A=${AUTO_CODING:?set AUTO_CODING to the auto-coding checkout root}
M=$(cd "$(dirname "$0")" && pwd)
B=$A/knowledge-data/retrieval-eval-v2/bin-main2
mk_clone() { git init -q "$1"; git -C "$1" fetch -q --no-tags "$A/test/go-stablenet" "$base"; git -C "$1" -c advice.detachedHead=false checkout -q FETCH_HEAD; git -C "$1" switch -q -c baseline; }
mkdir -p "$M/$case/logs"
[ -d "$M/$case/src" ] || mk_clone "$M/$case/src"
for arm in A0 A2; do [ -d "$M/$case/work/$arm" ] || mk_clone "$M/$case/work/$arm"; done
cd "$M/$case/src"
go list -deps -f '{{if not .Standard}}{{if .Module}}{{if .Module.Main}}{{$d := .Dir}}{{range .GoFiles}}{{$d}}/{{.}}{{"\n"}}{{end}}{{range .CgoFiles}}{{$d}}/{{.}}{{"\n"}}{{end}}{{end}}{{end}}{{end}}' ./cmd/gstable \
  | sed "s#^$PWD/##" | sort -u | python3 -c 'import sys,json; print(json.dumps({"include":[l.strip() for l in sys.stdin if l.strip()]}, indent=0))' > "$M/$case/scope.files-from.json"
( time "$B/ckg" build --src "$M/$case/src" --files-from-main ./cmd/gstable --no-cache --out "$M/$case/graph" ) > "$M/$case/logs/graph.log" 2>&1
( time "$B/ckv" build --src "$M/$case/src" --files-from "$M/$case/scope.files-from.json" --ckg "$M/$case/graph" --embedder ollama --model-name bge-m3 --out "$M/$case/vector" ) > "$M/$case/logs/vector.log" 2>&1
echo "$case done"
