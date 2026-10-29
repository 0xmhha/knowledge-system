"""Never-retrieved scenarios with a non-test answer: rank of the answer among non-test chunks in pure vector order."""
import json, subprocess, sys, collections
from pathlib import Path
sys.path.insert(0, '/Users/kevin/work/github/0xmhha/auto-coding/knowledge/knowledge-system/scripts')
import yaml
from retrieval_eval_run import checkout
from retrieval_found_diagnosis import first_answer_rank, rank_bucket
W = Path('/Users/kevin/work/github/0xmhha/auto-coding/knowledge-data/grpc-go-retrieval-eval')
SC = Path('/Users/kevin/work/github/0xmhha/auto-coding/knowledge/knowledge-system/docs/spec-driven/operational-preparation/retrieval-eval-grpc-go-v1/inputs/scenarios')
scen = {}
for p in SC.rglob('*.yaml'):
    s = yaml.safe_load(p.read_text()); scen[s['name']] = s
out = {}
for model, slug in (('qwen3-embedding:4b', 'qwen3-embedding-4b'), ('bge-m3', 'bge-m3')):
    rows = [r for r in json.load(open(W / 'found-head9e4a' / f'found-{slug}.json'))['scenarios'] if r['loss'] == 'never_retrieved']
    res = []
    for r in sorted(rows, key=lambda r: r['snapshot']):
        answers = [a for a in scen[r['name']]['expected_citations'] if not a['file'].endswith('_test.go')]
        if not answers:
            continue
        checkout(W / 'grpc-go', W / 'src-probe', r['snapshot'])
        args = [str(W / 'bin-head9e4a' / 'ckv'), 'query', '--out', str(W / 'vec' / slug / r['snapshot']), '--embedder=ollama',
                f'--model-name={model}', '--src', str(W / 'src-probe'), '-k', '300', '--threshold', '-1', '--examples', '1',
                '--json', '--no-footprint', r['prompt']]
        data = json.loads(subprocess.run(args, capture_output=True, text=True, check=True).stdout)
        cites = [h['citation'] for h in data.get('hits') or []]
        nonmd = [c for c in cites if not c['file'].endswith('.md') and c['start_line'] > 0]
        res.append({'name': r['name'], 'all_rank': r['vector_rank'], 'nontest_rank': first_answer_rank(answers, cites),
                    'nontest_nondoc_rank': first_answer_rank(answers, nonmd)})
    out[slug] = res
    for key in ('nontest_rank', 'nontest_nondoc_rank'):
        print(slug, key, len(res), dict(collections.Counter(rank_bucket(x[key]) for x in res)))
Path(sys.argv[1]).write_text(json.dumps(out, indent=1))
