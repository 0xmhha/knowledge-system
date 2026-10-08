#!/usr/bin/env bash
# Structural determinism probe for a reviewed 200-concept ontology. Mock
# embeddings compare rebuild identity, not real-model retrieval quality.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
scratch="${KS_LARGE_CORPUS_SMOKE_DIR:-$(mktemp -d)}"
src="$scratch/src"
dataset="$scratch/dataset"
mkdir -p "$src"
printf 'module example.com/ontologyfixture\n\ngo 1.25\n' > "$src/go.mod"
printf '# Corpus fixture\nReviewed vocabulary below.\n' > "$src/README.md"
python3 - "$src/ontology.yaml" <<'PY'
from pathlib import Path
import sys
lines = ['version: 1', 'project_id: corpus-fixture', 'domain: deterministic index fixture',
         'competency_questions:', '  - Which concept matches a reviewed term?', 'concepts:']
for i in range(200):
    lines += [f'  - id: concept-{i:03d}', '    kind: entity',
              f'    definition: Reviewed fixture concept number {i:03d}.',
              f'    includes: [Term {i:03d}]', '    excludes: [Unrelated term]',
              f'    terms: [{{lang: en, value: Term {i:03d}, preferred: true}}]',
              '    status: verified', '    reviewed_by: fixture-reviewer']
Path(sys.argv[1]).write_text('\n'.join(lines) + '\n')
PY
git -C "$src" init -q
git -C "$src" add .
git -C "$src" -c commit.gpgsign=false -c user.name=Codex \
  -c user.email=codex@example.com commit -qm fixture
"$repo_root/bin/cks" setup --src "$src" --out "$dataset" --embedder mock \
  --version base --progress text > "$scratch/base.log" 2>&1
"$repo_root/bin/cks" semantic build --repo "$src" --project-id corpus-fixture \
  --dataset-id semantic-base --graph "$dataset/current/graph" \
  --vector "$dataset/current/vector" --store "$scratch/semantic.db" \
  --out "$scratch/projection.json" --docs README.md --ontology ontology.yaml \
  --extract-only > "$scratch/build.json"
"$repo_root/bin/cks" semantic promote --input "$scratch/projection.json" \
  --repo "$src" --graph "$dataset/current/graph" --vector "$dataset/current/vector" \
  --store "$scratch/semantic.db" --activate > "$scratch/promote.json"
for number in 000 099 199; do
  "$repo_root/bin/cks" semantic lookup-term --project-id corpus-fixture \
    --repo "$src" --graph "$dataset/current/graph" \
    --vector "$dataset/current/vector" --store "$scratch/semantic.db" \
    --lang en --term "Term $number" > "$scratch/lookup-$number.json"
done
for name in one two; do
  "$repo_root/bin/cks" semantic export-text --project-id corpus-fixture \
    --repo "$src" --graph "$dataset/current/graph" \
    --vector "$dataset/current/vector" --store "$scratch/semantic.db" \
    --out "$scratch/corpus-$name" > "$scratch/export-$name.json"
done
python3 - "$scratch" <<'PY'
import json
from pathlib import Path
import sys
root = Path(sys.argv[1])
first, second = root / 'corpus-one', root / 'corpus-two'
files = sorted(p.name for p in first.iterdir())
assert len(files) == 201 and files == sorted(p.name for p in second.iterdir()), files
for name in files:
    assert (first / name).read_bytes() == (second / name).read_bytes(), name
manifest = json.loads((first / 'manifest.json').read_text())
assert len(manifest['files']) == 200, manifest
for number in ('000', '099', '199'):
    lookup = json.loads((root / f'lookup-{number}.json').read_text())
    assert [c['concept_id'] for c in lookup['candidates']] == [f'concept-{number}'], lookup
PY
for version in large-a large-b; do
  "$repo_root/bin/cks" setup --src "$src" --out "$dataset" --embedder mock \
    --version "$version" --semantic-corpus "$scratch/corpus-one" \
    --progress text > "$scratch/$version.log" 2>&1
  for term in 'Term 000' 'Term 099' 'Term 199'; do
    query_name="${term// /-}"
    "$repo_root/bin/ckv" --embedder mock query "$term" \
      --out "$dataset/$version/vector" --threshold -1 --json \
      > "$scratch/$version-$query_name.json" 2> "$scratch/$version-$query_name.log"
  done
done
python3 - "$scratch" <<'PY'
import hashlib
import json
from pathlib import Path
import sqlite3
import sys
root = Path(sys.argv[1])
def logical(version):
    base = root / 'dataset' / version
    graph = json.loads((base / 'graph' / 'manifest.json').read_text())
    vector = json.loads((base / 'vector' / 'manifest.json').read_text())
    db = sqlite3.connect(base / 'vector' / 'vector.db')
    rows = db.execute('SELECT id,file,start_line,end_line,commit_hash,text FROM chunks ORDER BY id').fetchall()
    db.close()
    digest = hashlib.sha256(json.dumps(rows, ensure_ascii=False).encode()).hexdigest()
    return graph['graph_digest'], vector['chunk_count'], digest, rows
a, b = logical('large-a'), logical('large-b')
assert a[:3] == b[:3], (a[:3], b[:3])
assert a[1] >= 200 and any('concept-' in row[1] for row in a[3]), a[1]
for term in ('Term-000', 'Term-099', 'Term-199'):
    queries = [json.loads((root / f'{version}-{term}.json').read_text()) for version in ('large-a', 'large-b')]
    citations = [[h['citation'] for h in q['hits']] for q in queries]
    assert citations[0] and citations[0] == citations[1], (term, citations)
print(f'Large corpus structural determinism: 200 concepts, {a[1]} CKV chunks, graph={a[0][:12]}, rows={a[2][:12]}')
print(f'Reports: {root}')
PY
