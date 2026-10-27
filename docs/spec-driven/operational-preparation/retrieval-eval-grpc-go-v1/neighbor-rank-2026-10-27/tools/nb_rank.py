import json, sys, yaml, collections
from pathlib import Path
W = Path('/Users/kevin/work/github/0xmhha/auto-coding/knowledge-data/grpc-go-retrieval-eval')
K = Path('/Users/kevin/work/github/0xmhha/auto-coding/knowledge/knowledge-system')
SCEN = K / 'docs/spec-driven/operational-preparation/retrieval-eval-grpc-go-v1/inputs/scenarios'
tag = sys.argv[1] if len(sys.argv) > 1 else 'final'
def rec(path, seq):
    for l in open(path):
        r = json.loads(l)
        if r['seq'] == seq: return r
def ov(a, f, s, e): return a['file'] == f and a['start_line'] <= e and s <= a['end_line']
for m in ['qwen3-embedding-4b', 'bge-m3']:
    found = {r['name']: r for r in json.load(open(W / f'eval-{tag}' / f'found-{m}.json'))['scenarios']}
    rep = {r['name']: r for r in json.load(open(W / f'eval-{tag}' / f'debug-report-{m}.json'))['scenarios']}
    print('=====', m)
    for name, f in found.items():
        if f['ranks']['stage2'] or not f['ranks']['stage3']: continue
        row = rep[name]
        r = rec(row['trace'], row['seq'])
        sc = yaml.safe_load((SCEN / f['snapshot'] / f'{name}.yaml').read_text())
        ans = sc['expected_citations']
        ev = [e for e in r['events'] if e['stage'] == 'stage3' and e['step'] == 'expand' and e['phase'] == 'after'][0]
        nbs = ev['data']['neighbors']
        idx = next(i for i, n in enumerate(nbs) if any(ov(a, n['file'], n['start'], n['end']) for a in ans))
        print(f"-- {name} [{f['loss']}] intent={f['intent']!r} answer nb#{idx+1}/{len(nbs)}: {nbs[idx]['file']}:{nbs[idx]['start']}-{nbs[idx]['end']} {nbs[idx]['kind']} {nbs[idx]['score']:.4f}")
        print('   prompt:', sc['prompt'][:110])
        c = collections.Counter(); spans = []
        for n in nbs[:idx]:
            c[n['kind']] += 1; spans.append(n['end'] - n['start'] + 1)
        print('   above by relation:', dict(c), 'spans<=3:', sum(s <= 3 for s in spans), 'of', len(spans))
        for n in nbs[:6]:
            print(f"     {n['file']}:{n['start']}-{n['end']} {n['kind']} {n['score']:.4f} {n['sources'][0]}")
