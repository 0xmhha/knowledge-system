"""Run the comparison baselines of the retrieval evaluation on the same prompts and indexes.

Each baseline returns an ordered citation list that is scored with the same
rule `cks eval` uses: a citation hits when it is in the answer file and its
line range overlaps an answer range; ranks are counted after collapsing
duplicate locations.
"""
from __future__ import annotations

import argparse
import json
import re
import subprocess
from collections.abc import Callable, Mapping, Sequence
from dataclasses import dataclass
from pathlib import Path

from retrieval_snapshot_chain import model_slug

QUERY_TIMEOUT_SECONDS = 300
TOP_K = 10
CHARS_PER_TOKEN = 4
WHOLE_FILE_END = 10**9
GREP_MIN_LEN = 3
CKG_CITATION = re.compile(r'· ((?:[\w.-]+/)*[\w.-]+\.go):(\d+)\s*$')
ASCII_WORD = re.compile(r'[A-Za-z][A-Za-z0-9_]*')
# Common English words in Korean developer prompts that say nothing about
# code location.
GREP_STOPWORDS = {'and', 'the', 'for', 'api', 'rpc'}


@dataclass(frozen=True, slots=True)
class Cite:
    file: str
    start: int
    end: int


@dataclass(frozen=True, slots=True)
class Answer:
    cites: list[Cite]
    returned_chars: int


def first_hit_rank(expected: Sequence[Cite], actual: Sequence[Cite]) -> int:
    """Return the 1-based rank of the first actual citation overlapping an expected one, or 0."""
    seen: set[Cite] = set()
    rank = 0
    for a in actual:
        if a in seen:
            continue
        seen.add(a)
        rank += 1
        if any(e.file == a.file and e.start <= a.end and a.start <= e.end for e in expected):
            return rank
    return 0


def parse_ckg(markdown: str) -> list[Cite]:
    """Seeds first, then neighbours, in printed order; each citation is one line."""
    out = []
    for line in markdown.splitlines():
        if m := CKG_CITATION.search(line):
            n = int(m.group(2))
            out.append(Cite(m.group(1), n, n))
    return out


def grep_terms(prompt: str) -> list[str]:
    """ASCII words a grep user would search for; Korean words cannot match English code."""
    seen = []
    for w in ASCII_WORD.findall(prompt):
        w = w.lower()
        if len(w) >= GREP_MIN_LEN and w not in GREP_STOPWORDS and w not in seen:
            seen.append(w)
    return seen


def run(args: Sequence[str], cwd: Path | None = None) -> str:
    r = subprocess.run(list(args), cwd=cwd, capture_output=True, text=True, timeout=QUERY_TIMEOUT_SECONDS, check=False)
    # git grep exits 1 when nothing matches; that is an empty result, not a failure.
    if r.returncode not in (0, 1) or (r.returncode == 1 and args[1:2] != ['grep']):
        raise RuntimeError(f'{args[0]} failed ({r.returncode}): {r.stderr.strip()[:300]}')
    return r.stdout


@dataclass(frozen=True, slots=True)
class Index:
    vector: Path
    graph: Path
    embed: list[str]


def index_for(datasets: Path, snapshot: str, model: str, embed_dim: int) -> Index:
    """The indexes retrieval_snapshot_chain.py builds: one graph per snapshot, one vector index per model."""
    embed = ['--embedder=ollama', f'--model-name={model}'] + ([f'--embed-dim={embed_dim}'] if embed_dim else [])
    return Index(datasets / 'vec' / model_slug(model, embed_dim) / snapshot, datasets / f'chain-{snapshot}' / 'graph', embed)


def ckv(bindir: Path, ix: Index, prompt: str, rerank: bool) -> Answer:
    args = [str(bindir / 'ckv'), 'query', '--out', str(ix.vector), *ix.embed, '--json', '-k', str(TOP_K)]
    if rerank:
        args.append('--bm25-rerank')
    data = json.loads(run([*args, prompt]))
    hits = data.get('hits') or []
    cites = [Cite(h['citation']['file'], h['citation']['start_line'], h['citation']['end_line']) for h in hits]
    return Answer(cites, sum(len(h.get('snippet', '')) for h in hits))


def ckg(bindir: Path, ix: Index, prompt: str) -> Answer:
    out = run([str(bindir / 'ckg'), 'query', '--graph', str(ix.graph), prompt])
    return Answer(parse_ckg(out), len(out))


def grep(src: Path, prompt: str) -> Answer:
    counts: dict[str, int] = {}
    for term in grep_terms(prompt):
        out = run(['git', 'grep', '-c', '-i', '-w', '-e', term, 'HEAD', '--', '*.go', ':!*_test.go'], cwd=src)
        for line in out.splitlines():
            path, _, n = line.removeprefix('HEAD:').rpartition(':')
            counts[path] = counts.get(path, 0) + int(n)
    ranked = sorted(counts, key=lambda p: (-counts[p], p))[:TOP_K]
    return Answer([Cite(p, 1, WHOLE_FILE_END) for p in ranked], 0)


def summarize(ranks: Sequence[int]) -> dict:
    n = len(ranks)
    if n == 0:
        return {'scenarios': 0}
    return {'scenarios': n, 'hit_at_1': sum(r == 1 for r in ranks) / n, 'hit_at_3': sum(0 < r <= 3 for r in ranks) / n,
            'mrr': sum(1 / r for r in ranks if r) / n}


def main() -> None:
    p = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    p.add_argument('--sample', required=True)
    p.add_argument('--prompts', required=True)
    p.add_argument('--datasets', required=True, help='work directory of retrieval_snapshot_chain.py')
    p.add_argument('--src', required=True, help='worktree checked out to --snapshot')
    p.add_argument('--snapshot', required=True)
    p.add_argument('--bin', required=True)
    p.add_argument('--model', required=True)
    p.add_argument('--embed-dim', type=int, default=0)
    p.add_argument('--out', required=True)
    a = p.parse_args()
    sample = json.loads(Path(a.sample).read_text())
    prompts: Mapping[str, str] = json.loads(Path(a.prompts).read_text())
    bindir, src = Path(a.bin), Path(a.src)
    ix = index_for(Path(a.datasets), a.snapshot, a.model, a.embed_dim)
    head = run(['git', '-C', str(src), 'rev-parse', 'HEAD']).strip()
    if head != a.snapshot:
        raise SystemExit(f'source worktree is at {head}, not {a.snapshot}')
    baselines: dict[str, Callable[[str], Answer]] = {
        'ckv_vector': lambda q: ckv(bindir, ix, q, False),
        'ckv_vector_bm25': lambda q: ckv(bindir, ix, q, True),
        'ckg_keyword': lambda q: ckg(bindir, ix, q),
        'grep': lambda q: grep(src, q),
    }
    rows = []
    for c in sample['sample']:
        if c['snapshot'] != a.snapshot:
            continue
        prompt = prompts[str(c['number'])]
        expected = [Cite(t['file'], t['start_line'], t['end_line']) for t in c['truth']]
        row = {'name': f'pr-{c["number"]}', 'intent': c['intent']}
        for name, fn in baselines.items():
            try:
                ans = fn(prompt)
                row[name] = {'first_hit_rank': first_hit_rank(expected, ans.cites), 'returned': len(ans.cites),
                             'returned_tokens': ans.returned_chars // CHARS_PER_TOKEN}
            except (RuntimeError, subprocess.TimeoutExpired, json.JSONDecodeError, KeyError) as error:
                row[name] = {'first_hit_rank': 0, 'error': str(error)[:300]}
        rows.append(row)
    summary = {name: summarize([r[name]['first_hit_rank'] for r in rows]) for name in baselines}
    Path(a.out).write_text(json.dumps({'snapshot': a.snapshot, 'model': a.model, 'embed_dim': a.embed_dim, 'results': rows, 'summary': summary}, indent=2, ensure_ascii=False) + '\n')


if __name__ == '__main__':
    main()
