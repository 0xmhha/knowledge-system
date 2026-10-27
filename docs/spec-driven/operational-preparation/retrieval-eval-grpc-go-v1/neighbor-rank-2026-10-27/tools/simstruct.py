"""Offline: structural neighbor-order variants (test demotion like Stage 2, defines / one-line demotion), slot count fixed."""
import json, re, sys
from pathlib import Path
sys.argv = [sys.argv[0]]
exec(open('simrank.py').read().split("for slug in")[0])
TESTW = re.compile(r'(?i)\b(tests?|flaky|e2e|fuzz|benchmark)\b')
def asks_tests(p): return bool(TESTW.search(p)) or any(w in p for w in ['시험', '테스트', '퍼저', '벤치마크'])
def is_test(f):
    b = f.split('/')[-1]
    return b.endswith('_test.go') or (b.endswith('.go') and (b.startswith('testutil') or b.startswith('testhelper'))) \
        or any(s in ('test', 'tests', 'testdata', 'testutil', 'testutils', 'testhelpers') for s in f.split('/'))
def mk(test=False, defines=1.0, oneline=1.0):
    def w(n, demote):
        s = n['score']
        if test and demote and is_test(n['file']): s *= 0.25
        if n['kind'] == 'defines': s *= defines
        if n['end'] == n['start']: s *= oneline
        return s
    return w
VARS = {'stage4_now': None, 'd4': mk(), 'd4+test': mk(test=True), 'd4+test+def.5': mk(test=True, defines=0.5),
        'd4+test+1line.5': mk(test=True, oneline=0.5), 'd4+def.5': mk(defines=0.5)}
for slug in ['qwen3-embedding-4b', 'bge-m3']:
    rp = json.load(open(W / 'eval-final' / f'relprobe-{slug}.json'))
    found = {r['name']: r for r in json.load(open(W / 'eval-final' / f'found-{slug}.json'))['scenarios']}
    rep = {r['name']: r for r in json.load(open(W / 'eval-final' / f'debug-report-{slug}.json'))['scenarios']}
    res = {v: [0, [], []] for v in VARS}
    for name, d in rp.items():
        f = found[name]; nbs = d['neighbors']; k = f['selected_by_origin'].get('neighbor', 0)
        ans = yaml.safe_load((SCEN / f['snapshot'] / f'{name}.yaml').read_text())['expected_citations']
        row = rep[name]
        r = next(json.loads(l) for l in open(row['trace']) if json.loads(l)['seq'] == row['seq'])
        dec = [e for e in r['events'] if e['stage'] == 'stage4' and e['step'] == 'allocate' and e['phase'] == 'after'][0]['data']['decisions']
        seed_found = any(x['origin'] == 'seed' and x['decision'].startswith('selected') and ov(a, x['file'], x['start'], x['end']) for a in ans for x in dec)
        demote = f['intent'] != 'test_add' and not asks_tests(f['prompt'])
        for v, w in VARS.items():
            key = (lambda n: (-n['score'], n['file'], n['start'])) if w is None else (lambda n: (-w(n, demote), -span(n), n['file'], n['start']))
            top = sorted(nbs, key=key)[:k]
            hit = seed_found or any(ov(a, n['file'], n['start'], n['end']) for a in ans for n in top)
            res[v][0] += hit
            if hit and f['loss'] != 'found': res[v][1].append(name)
            if not hit and f['loss'] == 'found': res[v][2].append(name)
    print('==', slug)
    for v, (h, g, l) in res.items():
        print(f"  {v:16s} found {h}  gain {g}  lose {l}")
