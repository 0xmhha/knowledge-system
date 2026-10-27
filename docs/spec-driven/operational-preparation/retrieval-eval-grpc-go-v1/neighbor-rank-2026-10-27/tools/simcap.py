"""Offline: extra pack slots spent on the next Stage 2 seeds (Stage 2 order), neighbors fixed. Tokens estimated
from the mean tokens per selected body in the same packs."""
import json, sys, yaml
from pathlib import Path
SCEN = Path('/Users/kevin/work/github/0xmhha/auto-coding/knowledge/knowledge-system/docs/spec-driven/operational-preparation/retrieval-eval-grpc-go-v1/inputs/scenarios')
tag = sys.argv[1]
def ov(a, f, s, e): return a['file'] == f and a['start_line'] <= e and s <= a['end_line']
for slug in ['qwen3-embedding-4b', 'bge-m3']:
    found = {r['name']: r for r in json.load(open(f'eval-{tag}/found-{slug}.json'))['scenarios']}
    rep = {r['name']: r for r in json.load(open(f'eval-{tag}/debug-report-{slug}.json'))['scenarios']}
    rows = []
    for name, f in found.items():
        row = rep[name]
        r = next(json.loads(l) for l in open(row['trace']) if json.loads(l)['seq'] == row['seq'])
        s2 = [e for e in r['events'] if e['stage'] == 'stage2' and e['step'] == 'rank_citations' and e['phase'] == 'after'][0]['data']['citations']
        s4 = [e for e in r['events'] if e['stage'] == 'stage4' and e['step'] == 'allocate' and e['phase'] == 'after'][0]['data']
        ans = yaml.safe_load((SCEN / f['snapshot'] / f'{name}.yaml').read_text())['expected_citations']
        sel = [d for d in s4['decisions'] if d['decision'].startswith('selected')]
        selkeys = {(d['file'], d['start']) for d in sel}
        rest = [c for c in s2 if (c['file'], c['start']) not in selkeys]
        rows.append((f['loss'] == 'found', ans, rest, s4['used_tokens'] / max(1, len(sel))))
    per = sum(x[3] for x in rows) / len(rows)
    out = []
    for extra in (0, 4, 8, 18):
        n = sum(fd or any(ov(a, c['file'], c['start'], c['end']) for a in ans for c in rest[:extra]) for fd, ans, rest, _ in rows)
        out.append(f'+{extra} slots: found {n} (~+{extra * per:.0f} tokens)')
    print(slug, f'(mean {per:.0f} tokens per body)', '; '.join(out))
