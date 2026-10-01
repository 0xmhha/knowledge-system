#!/usr/bin/env bash
# Model-independent A5.3 patch identity, exact test, human decision, promote.
set -euo pipefail
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
scratch="${KS_PATCH_SMOKE_DIR:-$(mktemp -d)}"
src="$scratch/src"
data="$scratch/data"
mkdir -p "$src"
cp "$repo_root/testdata/wbs-smoke/go.mod" "$repo_root/testdata/wbs-smoke/main.go" \
  "$repo_root/testdata/wbs-smoke/main_test.go" "$repo_root/testdata/wbs-smoke/README.md" \
  "$repo_root/testdata/wbs-smoke/ontology-reviewed.yaml" "$repo_root/testdata/wbs-smoke/spec-reviewed.yaml" "$src/"
git -C "$src" init -q
git -C "$src" add .
git -C "$src" -c commit.gpgsign=false -c user.name=Fixture -c user.email=fixture@example.org commit -qm base
"$repo_root/bin/cks" setup --src "$src" --out "$data" --project-id ks-fixture \
  --source-mode committed --embedder mock --version base > "$scratch/base.log" 2>&1
printf '\nPatch candidate keeps the Alpha contract.\n' >> "$src/README.md"
git -C "$src" add README.md
git -C "$src" -c commit.gpgsign=false -c user.name=Fixture -c user.email=fixture@example.org commit -qm patch
"$repo_root/bin/cks" setup --src "$src" --out "$data" --project-id ks-fixture \
  --source-mode committed --embedder mock --version held --hold-for-review \
  --gate-test-bin go --gate-test-arg test --gate-test-arg ./... > "$scratch/held.log" 2>&1
"$repo_root/bin/cks" patch --dataset "$data" --patch-id alpha-doc-change register --version held > "$scratch/patch.json"
dataset_id="$(python3 - "$data/held/dataset-identity.json" <<'PY'
import json,sys
print(json.load(open(sys.argv[1]))['dataset_id'])
PY
)"
"$repo_root/bin/cks" semantic build --repo "$src" --project-id ks-fixture --dataset-id "$dataset_id" \
  --graph "$data/held/graph" --vector "$data/held/vector" --store "$scratch/semantic.db" \
  --out "$scratch/projection.json" --docs README.md --ontology ontology-reviewed.yaml \
  --spec spec-reviewed.yaml --version-dir "$data/held" --extract-only > "$scratch/extract.json"
python3 - "$scratch/projection.json" "$data/held/graph/graph.db" "$src" <<'PY'
import hashlib,json,sqlite3,subprocess,sys
from pathlib import Path
path,graph_path,src=sys.argv[1:]
p=json.loads(Path(path).read_text())
graph=sqlite3.connect(graph_path)
anchors={}
for name,proof_id,kind in [('Alpha','code:alpha','code'),('TestAlpha','test:alpha','test')]:
    row=graph.execute("SELECT canonical_id,file_path,start_line,end_line FROM nodes WHERE name=? AND type='Function'",(name,)).fetchone()
    assert row and row[0],row
    canonical,file,start,end=row
    source=subprocess.check_output(['git','-C',src,'show',p['snapshot']['commit']+':'+file])
    span=b''.join(source.splitlines(keepends=True)[start-1:end])
    p['evidence'].append({'id':proof_id,'snapshot':p['snapshot'],'kind':kind,'path':file,
        'start_line':start,'end_line':end,'content_sha256':hashlib.sha256(span).hexdigest(),
        'canonical_id':canonical,'extractor':'patch-smoke-review-v1'})
    anchors[name]=canonical
concept=next(c for c in p['concepts'] if c['id']=='alpha-function')
criterion=p['requirements'][0]['acceptance_criteria'][0]
p['assertions']=[
 {'id':'reviewed:implementation','predicate':'IMPLEMENTED_BY','subject_id':'alpha-function',
  'object_id':anchors['Alpha'],'evidence_ids':[concept['evidence_id'],'code:alpha'],
  'status':'verified','reviewed_by':'fixture-reviewer'},
 {'id':'reviewed:tested','predicate':'TESTED_BY','subject_id':anchors['Alpha'],
  'object_id':anchors['TestAlpha'],'evidence_ids':['code:alpha','test:alpha'],
  'status':'verified','reviewed_by':'fixture-reviewer'},
 {'id':'reviewed:checked','predicate':'CHECKED_BY','subject_id':criterion['id'],
  'object_id':anchors['TestAlpha'],'evidence_ids':[criterion['evidence_id'],'test:alpha'],
  'status':'verified','reviewed_by':'fixture-reviewer'}]
Path(path).write_text(json.dumps(p,indent=2)+'\n')
PY
"$repo_root/bin/cks" semantic promote --input "$scratch/projection.json" --repo "$src" \
  --graph "$data/held/graph" --vector "$data/held/vector" --store "$scratch/semantic.db" \
  --version-dir "$data/held" > "$scratch/semantic-promote.json"
"$repo_root/bin/cks" semantic test --project-id ks-fixture --dataset-id "$dataset_id" \
  --version-dir "$data/held" --repo "$src" --graph "$data/held/graph" \
  --vector "$data/held/vector" --store "$scratch/semantic.db" \
  --criterion-id ac-alpha --go-test-exact --out "$scratch/test-run.json" > "$scratch/test.log"
"$repo_root/bin/cks" patch --dataset "$data" --patch-id alpha-doc-change record-run \
  --semantic-store "$scratch/semantic.db" --report "$scratch/test-run.json" > "$scratch/record-run.json"
if "$repo_root/bin/cks" patch --dataset "$data" --patch-id alpha-doc-change promote \
  --semantic-store "$scratch/semantic.db" > "$scratch/premature.log" 2>&1; then
  echo "test success was mistaken for human criterion approval" >&2; exit 1
fi
"$repo_root/bin/cks" patch --dataset "$data" --patch-id alpha-doc-change decide \
  --semantic-store "$scratch/semantic.db" --decision-id reviewer-1-ac-alpha \
  --criterion-id ac-alpha --reviewer fixture-owner --outcome approved \
  --reason 'The reviewed test and source criterion match the patch.' > "$scratch/decision.json"
"$repo_root/bin/cks" patch --dataset "$data" --patch-id alpha-doc-change promote \
  --semantic-store "$scratch/semantic.db" > "$scratch/promoted.json"
"$repo_root/bin/cks" setup --out "$data" --rollback base > "$scratch/rollback-base.log" 2>&1
"$repo_root/bin/cks" setup --out "$data" --rollback held > "$scratch/rollback-held.log" 2>&1
python3 - "$data" "$scratch" <<'PY'
import json,pathlib,sys
data,scratch=map(pathlib.Path,sys.argv[1:])
attempt=json.loads((scratch/'patch.json').read_text())
assert attempt['state']=='unconfirmed' and len(attempt['changed_files'])==1
assert attempt['changed_files'][0]['path']=='README.md'
assert (data/'current').resolve()==(data/'held').resolve()
assert json.loads((scratch/'promoted.json').read_text())['previous_version']=='base'
assert json.loads((scratch/'decision.json').read_text())['outcome']=='approved'
PY
echo "Patch review and promotion smoke passed: $scratch"
