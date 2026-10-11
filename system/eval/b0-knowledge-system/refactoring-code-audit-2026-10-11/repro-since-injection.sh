#!/bin/sh
# Reproduce F1: a revision argument that starts with "-" becomes a git option.
# usage: repro-since-injection.sh <work-dir>   (run from the repository root)
set -e
W="$1"; mkdir -p "$W/repo"
go build -o "$W/ckv" ./cmd/vector
cd "$W/repo"; git init -q
printf 'package a\n\nfunc A() int { return 1 }\n' > a.go; git add .; git -c user.name=t -c user.email=t@x -c commit.gpgsign=false commit -qm one
printf 'package a\n\nfunc A() int { return 2 }\n' > a.go; git -c user.name=t -c user.email=t@x -c commit.gpgsign=false commit -qam two
cd "$W"
./ckv build --src "$W/repo" --out "$W/idx" --embedder mock >/dev/null
echo "important content" > victim.txt
printf 'package a\n\nfunc A() int { return 3 }\n' > repo/a.go
./ckv reindex --src "$W/repo" --out "$W/idx" --embedder mock --since "--output=$W/victim.txt"
echo "victim.txt now:"; cat victim.txt   # prints the git diff output instead of "important content"
