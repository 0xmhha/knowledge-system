#!/usr/bin/env python3
"""Read-only audit of archived input boundaries; no model or MCP access."""
from pathlib import Path
import hashlib,json
ROOT=Path(__file__).resolve().parent
summary=json.loads((ROOT/'summary.json').read_text())
items={item['path']:item for item in summary['artifacts']}
actual={str(p.relative_to(ROOT)) for p in ROOT.rglob('*') if p.is_file() and p.name!='summary.json'}
assert set(items)==actual
for path,item in items.items():
    raw=(ROOT/path).read_bytes()
    assert len(raw)==item['size'] and hashlib.sha256(raw).hexdigest()==item['sha256'],path
report=json.loads((ROOT/'current-preflight.json').read_text())
assert report['status']=='pending' and report['pending_reasons']==['protocol_review_pending','dynamic_fixture_review_pending']
assert report['official_execution_ready'] is False and report['v2_matrix_ready'] is False and report['metrics'] is None
for key,name in [('question_set_sha256','questions.json'),('protocol_sha256','protocol-m2max-draft.json'),('fixture_manifest_sha256','dynamic-fixtures-m2max-draft.json'),('human_review_sha256','human-review-m2max-2026-10-03.json')]:
    assert hashlib.sha256((ROOT/'locked-inputs'/name).read_bytes()).hexdigest()==report[key]
book=json.loads((ROOT/'locked-inputs/questions.json').read_text())
manifest=json.loads((ROOT/'development/manifest.json').read_text())
assert manifest['selected_ids']==report['selected_ids'] and len(report['selected_ids'])==4
assert len(report['withheld_ids'])==8 and not set(report['selected_ids']) & set(report['withheld_ids'])
files=list((ROOT/'development').glob('*.yaml'))
assert len(files)==manifest['scenario_count']==4
scenarios=[json.loads(file.read_text()) for file in files]
assert {s['name'].upper() for s in scenarios}==set(report['selected_ids'])
for file in files:
    assert hashlib.sha256(file.read_bytes()).hexdigest()==manifest['scenario_sha256'][file.name]
for question in book['questions']:
    if question['id'] in report['selected_ids']:
        scenario=next(s for s in scenarios if s['name'].upper()==question['id'])
        assert scenario['prompt']==question['prompt'] and scenario['runs']==report['runs']['retrieval']
        assert scenario['expected_commit']==report['corpus_commit']
    assert all('candidate_answer' not in s for s in scenarios)
    assert all(question['candidate_answer'] not in file.read_text() for file in files)
    if question['id'] in report['withheld_ids']:
        assert all(question['prompt']!=s['prompt'] for s in scenarios)
commands=json.loads((ROOT/'commands.json').read_text())
assert [r['exit_code'] for r in commands['commands']]==[2,0,2,2]
assert commands['final_output_absent'] and commands['ready_guard_output_absent']
assert not commands['model_or_mcp_query_executed']
compat=json.loads((ROOT/'legacy-valid-export-compatibility.json').read_text())
assert compat['identical'] and compat['count']==13
corpus=json.loads((ROOT/'corpus-reaudit.json').read_text())
assert (corpus['commit'],corpus['tree'])==(report['corpus_commit'],report['corpus_tree'])
assert not corpus['dirty'] and not corpus['alternates_present'] and corpus['fsck_exit']==0
assert 'Ran 26 tests' in (ROOT/'tests-final.txt').read_text() and (ROOT/'tests-final.txt').read_text().rstrip().endswith('OK')
print(json.dumps({'artifacts':len(items),'development_scenarios':len(files),'status':'pending','metrics':None}))
