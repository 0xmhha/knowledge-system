"""Summarize the retrieval evaluation reports of every snapshot into one table.

Reads eval/<model>/<snapshot>/{cks.json,baselines.json} written by
retrieval_eval_run.py and pools every scenario of every snapshot. Errors stay
in the denominator as misses (RETRIEVAL-EVAL-SPEC.md section 4), and an
unknown or missing intent counts as wrong (section 5).

cks eval drops every citation whose commit differs from the scenario's
snapshot before it ranks. A reindexed index keeps the embedding commit on
chunks of files the reindex did not touch, so on every snapshot after the
first most CKS citations are dropped while the baselines keep theirs. The
`cks_any_commit` row re-ranks the recorded responses with the same overlap
rule but without that filter, after checking with git that each older-commit
citation's file is unchanged at the snapshot. `cks` stays the reported
cks eval result.

usage: retrieval_eval_summary.py --work <dir> --repo <clone> --scenarios <dir> --snapshots <json>
       --model <m> [--model <m>] --out <json>
"""
from __future__ import annotations

import argparse
import functools
import json
import statistics
import subprocess
from collections import Counter, defaultdict
from pathlib import Path

from retrieval_snapshot_chain import model_slug

BASELINES = ('ckv_vector', 'ckv_vector_bm25', 'ckg_keyword', 'grep')


def rank_metrics(ranks: list[int]) -> dict:
    """ranks holds each scenario's first hit rank; 0 is a miss or an error."""
    n = len(ranks)
    if n == 0:
        return {'scenarios': 0}
    return {
        'scenarios': n,
        'hit_at_1': sum(r == 1 for r in ranks) / n,
        'hit_at_3': sum(1 <= r <= 3 for r in ranks) / n,
        'mrr': sum(1 / r for r in ranks if r > 0) / n,
    }


def percentile(values: list[float], q: float) -> float:
    s = sorted(values)
    return s[min(len(s) - 1, int(q * len(s)))] if s else 0


def summarize_cks(results: list[dict]) -> dict:
    ranks, tokens, latency, recall = [], [], [], []
    errors = 0
    confusion: dict[str, Counter] = defaultdict(Counter)
    correct = 0
    for r in results:
        m = r['metrics']
        failed = 'error' in r or r['retrieval_state'] == 'error'
        errors += failed
        ranks.append(0 if failed else m['first_hit_rank'])
        recall.append(0 if failed else m['file_recall'])
        if not failed:
            tokens.append(m['used_tokens'])
            latency.append(m['latency_ms'])
        got = (r.get('classified_intents') or [''])[0] or '(unknown)'
        confusion[r['expected_intent']][got] += 1
        correct += bool(r.get('intent_match'))
    out = rank_metrics(ranks)
    out.update({
        'errors': errors,
        'intent_accuracy': correct / len(results) if results else 0,
        'intent_confusion': {k: dict(v) for k, v in sorted(confusion.items())},
        'file_recall_mean': statistics.fmean(recall) if recall else 0,
        'used_tokens_mean': statistics.fmean(tokens) if tokens else 0,
        'latency_ms_p50': percentile(latency, 0.5),
        'latency_ms_p95': percentile(latency, 0.95),
    })
    return out


def summarize_baseline(results: list[dict], method: str) -> dict:
    rows = [r[method] for r in results]
    out = rank_metrics([0 if 'error' in x else x['first_hit_rank'] for x in rows])
    tokens = [x['returned_tokens'] for x in rows if 'error' not in x]
    out.update({
        'errors': sum('error' in x for x in rows),
        'returned_tokens_mean': statistics.fmean(tokens) if tokens else 0,
    })
    return out


def first_hit_any_commit(expected: list[dict], citations: list[dict]) -> int:
    """cks eval firstHitRank in overlap mode, deduplicated by location only."""
    seen, ranked = set(), []
    for c in citations:
        key = (c['file'], c['start_line'], c['end_line'])
        if key not in seen:
            seen.add(key)
            ranked.append(c)
    for i, a in enumerate(ranked, 1):
        if any(e['file'] == a['file'] and e['start_line'] <= a['end_line'] and a['start_line'] <= e['end_line']
               for e in expected):
            return i
    return 0


def pack_citations(result: dict) -> list[dict]:
    calls = [c for c in result.get('raw_calls', []) if c['tool'] == 'cks.context.get_for_task']
    if len(calls) != 1:
        raise SystemExit(f"{result['name']}: expected one recorded get_for_task call, found {len(calls)}")
    return calls[0]['response'].get('structuredContent', {}).get('citations') or []


@functools.lru_cache(maxsize=None)
def changed_files(repo: str, old: str, new: str) -> frozenset[str]:
    out = subprocess.run(['git', '-C', repo, 'diff', '--name-only', old, new], capture_output=True, text=True,
                         check=True, timeout=600).stdout
    return frozenset(out.split())


def rescore_any_commit(results: list[dict], scenarios: dict[str, dict], repo: str) -> dict:
    ranks, older, changed = [], 0, 0
    for r in results:
        if 'error' in r:
            ranks.append(0)
            continue
        sc = scenarios[r['name']]
        cites = pack_citations(r)
        for c in cites:
            if c.get('commit_hash') != sc['expected_commit']:
                older += 1
                if not c.get('commit_hash') or c['file'] in changed_files(repo, c['commit_hash'], sc['expected_commit']):
                    changed += 1
        ranks.append(first_hit_any_commit(sc['expected_citations'], cites))
    out = rank_metrics(ranks)
    out.update({'errors': sum('error' in r for r in results), 'older_commit_citations': older,
                'older_commit_citations_with_changed_file': changed})
    return out


def summarize(work: Path, repo: str, scenario_root: Path, snapshots: list[str], model: str) -> dict:
    import yaml

    slug = model_slug(model, 0)
    cks, base = [], []
    scenarios = {}
    exits = Counter()
    for snap in snapshots:
        d = work / 'eval' / slug / snap
        cks += json.loads((d / 'cks.json').read_text())['results']
        base += json.loads((d / 'baselines.json').read_text())['results']
        exits[(d / 'cks-exit.txt').read_text().strip()] += 1
        for f in sorted((scenario_root / snap).glob('*.yaml')):
            sc = yaml.safe_load(f.read_text())
            scenarios[sc['name']] = sc
    if sorted(r['name'] for r in cks) != sorted(r['name'] for r in base):
        raise SystemExit(f'{model}: CKS and baseline scenarios differ')
    return {
        'model': model,
        'snapshots': len(snapshots),
        'cks_exit_codes': dict(exits),
        'methods': {'cks': summarize_cks(cks), 'cks_any_commit': rescore_any_commit(cks, scenarios, repo),
                    **{m: summarize_baseline(base, m) for m in BASELINES}},
    }


def main() -> None:
    p = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    p.add_argument('--work', required=True)
    p.add_argument('--repo', required=True, help='grpc-go clone, for the file-unchanged check')
    p.add_argument('--scenarios', required=True, help='frozen inputs/scenarios directory')
    p.add_argument('--snapshots', required=True)
    p.add_argument('--model', action='append', required=True)
    p.add_argument('--out', required=True)
    a = p.parse_args()
    snaps = json.loads(Path(a.snapshots).read_text())
    report = [summarize(Path(a.work), a.repo, Path(a.scenarios), snaps, m) for m in a.model]
    Path(a.out).write_text(json.dumps(report, indent=2, ensure_ascii=False) + '\n')
    for r in report:
        for name, m in r['methods'].items():
            print(f"{r['model']:20} {name:16} n={m['scenarios']} err={m['errors']} "
                  f"hit@1={m['hit_at_1']:.3f} hit@3={m['hit_at_3']:.3f} mrr={m['mrr']:.3f}")


if __name__ == '__main__':
    main()
