"""Run CKS and the comparison baselines over the snapshot indexes of the retrieval evaluation.

For each snapshot, in the order of --snapshots, a separate worktree is checked
out to that snapshot (the chain builder owns src-chain), a cks.yaml is written
that points at this snapshot's graph and this model's vector index, and then
`cks eval` (with anchor verification) and retrieval_baselines.py run on the
frozen scenarios of that snapshot. A snapshot whose two reports already exist
is skipped, so an interrupted run resumes. Inputs are only read.
"""
from __future__ import annotations

import argparse
import json
import subprocess
import sys
from pathlib import Path

from retrieval_baselines import index_for
from retrieval_snapshot_chain import model_slug

EVAL_TIMEOUT_SECONDS = 3 * 3600
GIT_TIMEOUT_SECONDS = 600
REPO = Path(__file__).resolve().parent.parent


def cks_yaml(graph: Path, src: Path, vector: Path, bindir: Path, model: str, embed_dim: int, telemetry: Path) -> str:
    dim = f'\n    embed_dim: {embed_dim}' if embed_dim else ''
    return f"""version: 1
name: retrieval-eval
backends:
  ckg:
    path: {graph / 'graph.db'}
    source_root: {src}
    binary_path: {bindir / 'ckg'}
  ckv:
    path: {vector}
    provider: ollama
    binary_path: {bindir / 'ckv'}
    embed_model: {model}{dim}
listen:
  transport: stdio
logging:
  level: error
  footprint_dir: {telemetry}
sanitize:
  rules_path: {REPO / 'system/policies/sanitization_rules.yaml'}
  default_action: drop
  fail_closed_on_unknown_rule: true
"""


def checkout(repo: Path, src: Path, commit: str) -> None:
    if not src.exists():
        subprocess.run(['git', '-C', str(repo), 'worktree', 'add', '-q', '--detach', str(src), commit], check=True,
                       timeout=GIT_TIMEOUT_SECONDS)
    subprocess.run(['git', '-C', str(src), 'checkout', '-q', '--detach', commit], check=True, timeout=GIT_TIMEOUT_SECONDS)


def main() -> None:
    p = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    p.add_argument('--repo', required=True, help='grpc-go clone')
    p.add_argument('--snapshots', required=True, help='JSON list of commits')
    p.add_argument('--work', required=True, help='work directory of retrieval_snapshot_chain.py')
    p.add_argument('--inputs', required=True, help='frozen inputs directory (sample.json, prompts.json, scenarios/)')
    p.add_argument('--bin', required=True)
    p.add_argument('--model', required=True)
    p.add_argument('--embed-dim', type=int, default=0)
    a = p.parse_args()
    work, inputs, bindir = Path(a.work).resolve(), Path(a.inputs).resolve(), Path(a.bin).resolve()
    src = work / 'src-eval'
    slug = model_slug(a.model, a.embed_dim)
    out_root = work / 'eval' / slug
    record = work / f'chain-{slug}.json'
    # Only snapshots the chain builder recorded are finished: a reindex in
    # progress already holds the copied manifest of the previous snapshot.
    built = {r['snapshot'] for r in json.loads(record.read_text())} if record.exists() else set()
    for snap in json.loads(Path(a.snapshots).read_text()):
        if snap not in built:
            print(f'stopping at {snap}: not yet recorded in {record.name}')
            break
        out = out_root / snap
        cks_report, base_report = out / 'cks.json', out / 'baselines.json'
        if cks_report.exists() and base_report.exists():
            continue
        ix = index_for(work, snap, a.model, a.embed_dim)
        if not (ix.vector / 'manifest.json').exists() or not (ix.graph / 'graph.db').exists():
            sys.exit(f'index missing for {snap}: {ix.vector} or {ix.graph}')
        checkout(Path(a.repo), src, snap)
        out.mkdir(parents=True, exist_ok=True)
        config = out / 'cks.yaml'
        config.write_text(cks_yaml(ix.graph, src, ix.vector, bindir, a.model, a.embed_dim, out / 'telemetry'))
        with (out / 'run.log').open('a') as log:
            # cks eval writes its report and then exits non-zero for scenario-level
            # failures (snapshot conflicts, abstentions); keep the code with the
            # report. No report means the run never measured (for example an
            # anchor violation), so stop.
            r = subprocess.run([str(bindir / 'cks'), 'eval', '--config', str(config), '--scenarios',
                                str(inputs / 'scenarios' / snap), '--verify-anchors', str(src), '--output', str(cks_report)],
                               stdout=log, stderr=subprocess.STDOUT, check=False, timeout=EVAL_TIMEOUT_SECONDS)
            (out / 'cks-exit.txt').write_text(f'{r.returncode}\n')
            if not cks_report.exists() or cks_report.stat().st_size == 0:
                cks_report.unlink(missing_ok=True)
                sys.exit(f'cks eval produced no report for {snap} (exit {r.returncode}); see {out / "run.log"}')
            subprocess.run([sys.executable, str(REPO / 'scripts/retrieval_baselines.py'), '--sample', str(inputs / 'sample.json'),
                            '--prompts', str(inputs / 'prompts.json'), '--datasets', str(work), '--src', str(src),
                            '--snapshot', snap, '--bin', str(bindir), '--model', a.model, '--embed-dim', str(a.embed_dim),
                            '--out', str(base_report)],
                           stdout=log, stderr=subprocess.STDOUT, check=True, timeout=EVAL_TIMEOUT_SECONDS)


if __name__ == '__main__':
    main()
