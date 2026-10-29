"""Offline: intent accuracy from the recorded per-intent anchor similarities (eval-final traces)."""
import json, collections, statistics
from pathlib import Path
W = Path('/Users/kevin/work/github/0xmhha/auto-coding/knowledge-data/grpc-go-retrieval-eval')
for slug in ['qwen3-embedding-4b', 'bge-m3']:
    found = {r['name']: r for r in json.load(open(W/'eval-final'/f'found-{slug}.json'))['scenarios']}
    rep = {r['name']: r for r in json.load(open(W/'eval-final'/f'debug-report-{slug}.json'))['scenarios']}
    n = top1 = top2 = assigned = assigned_ok = 0
    best = []; margin_ok = []; margin_bad = []
    conf = collections.Counter(); exp_c = collections.Counter()
    for name, f in found.items():
        row = rep[name]
        r = next(json.loads(l) for l in open(row['trace']) if json.loads(l)['seq'] == row['seq'])
        d = [e for e in r['events'] if e['stage'] == 'intent' and e['step'] == 'classify'][0]['data']
        sc = d['scores']; exp = f['expected_intent']; exp_c[exp] += 1
        n += 1; best.append(sc[0]['similarity'])
        ok = sc[0]['intent'] == exp
        top1 += ok; top2 += exp in (sc[0]['intent'], sc[1]['intent'])
        (margin_ok if ok else margin_bad).append(sc[0]['similarity'] - sc[1]['similarity'])
        if d['intent']:
            assigned += 1; assigned_ok += d['intent'] == exp
        conf[(exp, sc[0]['intent'])] += 1
    print('==', slug, 'n', n, 'expected', dict(exp_c))
    print(f"  argmax acc {top1/n:.2f}  top2 {top2/n:.2f}  assigned (>=0.55) {assigned} correct {assigned_ok}  best sim median {statistics.median(best):.3f} range {min(best):.3f}-{max(best):.3f}")
    print(f"  margin top1-top2 when right: median {statistics.median(margin_ok) if margin_ok else 0:.4f}; when wrong {statistics.median(margin_bad):.4f}")
    print('  confusions (expected -> argmax):', conf.most_common(12))
