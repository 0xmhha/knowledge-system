#!/usr/bin/env python3
"""N08 DEV-only diagnosis. Reads the first DEV capture, the frozen DEV questions,
the eval ontology and a scratch copy of the frozen CKV index (never the dataset
itself). FINAL questions are not read.
Usage: analyze.py <run-DEV dir> <dataset version dir> <vector copy> <ckv binary> <source root>"""
import gzip, json, subprocess, sys, yaml
from pathlib import Path
run, version, vec, ckv, src = map(Path, sys.argv[1:6])
ROOT = Path(__file__).resolve().parents[4]
qs = {x['id']: x for x in json.load(open(run / 'questions.json'))['questions'] if x['split'] == 'DEV'}
onto = yaml.safe_load(open(ROOT / 'docs/spec-driven/ontology-pilot-functional-v2.yaml'))
terms = [(c['id'], t['value']) for c in onto['concepts'] for t in c['terms']]
out = {'ontology_concepts': len(onto['concepts']), 'ontology_terms': len(terms), 'questions': {}}
packs = {}
for line in gzip.open(ROOT / 'system/eval/b0-knowledge-system/refactoring-n11-evaluation-run-2026-10-09/dev-capture/rows.jsonl.gz', 'rt'):
    r = json.loads(line)
    if r['arm'] == 'baseline_off' and r['phase'] == 'retrieval':
        packs.setdefault(r['request_id'], r['call']['response']['structuredContent']['citations'])
ov = lambda e, c: c['file'] == e['path'] and c['start_line'] <= e['last'] and e['first'] <= c['end_line']
for qid, q in qs.items():
    raw = json.loads(subprocess.run([str(ckv), 'query', q['prompt'], '--out', str(vec), '--src', str(src), '--json', '-k', '60',
        '--threshold', '-1', '--no-footprint', '--embed-dim', '1024', '--model-name', 'bge-m3:latest'],
        capture_output=True, text=True, check=True).stdout)['hits']
    gold = []
    for e in q['evidence']:
        exact = [i + 1 for i, h in enumerate(raw) if h['citation']['file'] == e['path'] and h['chunk_kind'] != 'file_header' and ov(e, h['citation'])]
        gold.append({'span': f"{e['path']}:{e['first']}-{e['last']}",
                     'pack_rank': next((i + 1 for i, c in enumerate(packs[qid]) if ov(e, c)), None),
                     'raw_ckv_rank_non_header': exact[0] if exact else None,
                     'raw_ckv_rank_any': next((i + 1 for i, h in enumerate(raw) if ov(e, h['citation'])), None)})
    out['questions'][qid] = {'group': q['group'], 'matched_terms': [t for t in terms if t[1].lower() in q['prompt'].lower()], 'gold': gold}
print(json.dumps(out, ensure_ascii=False, indent=1))
