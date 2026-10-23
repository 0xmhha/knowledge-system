"""Build the per-snapshot indexes for the retrieval evaluation.

The graph is model-independent, so every snapshot gets one graph build
(`cks setup --skip-vector`), reused by every embedding model. Each model then
gets a full vector build at the earliest snapshot and, for every later
snapshot, a copy of the previous vector index advanced with `ckv reindex`.

`ckv reindex` re-aligns canonical IDs against the graph recorded in the
index manifest. A copied index still records the previous snapshot's graph,
so the manifest is pointed at this snapshot's graph first; reindex then sees
a regenerated graph and re-aligns every affected chunk. Timing per step goes
to chain-<model>.json.
"""
from __future__ import annotations

import argparse
import json
import shutil
import subprocess
import time
from pathlib import Path

STEP_TIMEOUT_SECONDS = 8 * 3600
GIT_TIMEOUT_SECONDS = 600


def run(args: list[str], log: Path) -> float:
    start = time.monotonic()
    with log.open('a') as out:
        out.write('$ ' + ' '.join(args) + '\n')
        out.flush()
        subprocess.run(args, stdout=out, stderr=subprocess.STDOUT, check=True, timeout=STEP_TIMEOUT_SECONDS)
    return time.monotonic() - start


def checkout(src: Path, commit: str) -> None:
    subprocess.run(['git', '-C', str(src), 'checkout', '-q', '--detach', commit], check=True, timeout=GIT_TIMEOUT_SECONDS)


def graph_ready(graph: Path, commit: str) -> bool:
    manifest = graph / 'manifest.json'
    if not (graph / 'graph.db').exists() or not manifest.exists():
        return False
    return commit in manifest.read_text()


def point_at_graph(vector: Path, graph: Path) -> None:
    manifest = vector / 'manifest.json'
    data = json.loads(manifest.read_text())
    data['sources']['ckg']['path'] = str(graph)
    manifest.write_text(json.dumps(data, indent=2) + '\n')


def model_slug(model: str, dim: int) -> str:
    return model.replace(':', '-').replace('/', '-') + (f'-d{dim}' if dim else '')


def main() -> None:
    p = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    p.add_argument('--repo', required=True)
    p.add_argument('--snapshots', required=True, help='JSON list of commits in time order')
    p.add_argument('--work', required=True)
    p.add_argument('--bin', required=True)
    p.add_argument('--model', required=True)
    p.add_argument('--embed-dim', type=int, default=0)
    a = p.parse_args()
    work, bindir = Path(a.work), Path(a.bin)
    snaps: list[str] = json.loads(Path(a.snapshots).read_text())
    src = work / 'src-chain'
    if not src.exists():
        subprocess.run(['git', '-C', a.repo, 'worktree', 'add', '-q', '--detach', str(src), snaps[0]], check=True, timeout=GIT_TIMEOUT_SECONDS)
    embed = ['--embedder=ollama', f'--model-name={a.model}'] + ([f'--embed-dim={a.embed_dim}'] if a.embed_dim else [])
    slug = model_slug(a.model, a.embed_dim)
    record_path = work / f'chain-{slug}.json'
    record = json.loads(record_path.read_text()) if record_path.exists() else []
    done = {r['snapshot'] for r in record}
    prev = None
    for snap in snaps:
        graph = work / f'chain-{snap}' / 'graph'
        vector = work / 'vec' / slug / snap
        if snap in done:
            prev = vector
            continue
        checkout(src, snap)
        log = work / f'chain-{slug}-{snap}.log'
        row: dict = {'snapshot': snap}
        if not graph_ready(graph, snap):
            if graph.exists():
                shutil.rmtree(graph)
            row['graph_seconds'] = run([str(bindir / 'cks'), 'setup', '--src', str(src), '--out', str(graph.parent), '--skip-vector'], log)
        if vector.exists():
            shutil.rmtree(vector)
        if prev is None:
            row['mode'] = 'full'
            row['vector_seconds'] = run([str(bindir / 'ckv'), 'build', '--src', str(src), '--out', str(vector), '--ckg', str(graph), *embed], log)
        else:
            row['mode'] = 'reindex'
            shutil.copytree(prev, vector)
            point_at_graph(vector, graph)
            row['vector_seconds'] = run([str(bindir / 'ckv'), 'reindex', '--src', str(src), '--out', str(vector), *embed, '--json'], log)
        record.append(row)
        record_path.write_text(json.dumps(record, indent=2) + '\n')
        prev = vector


if __name__ == '__main__':
    main()
