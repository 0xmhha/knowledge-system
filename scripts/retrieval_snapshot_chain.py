"""Build one CKS dataset per snapshot: a full build at the earliest snapshot,
then each later snapshot advances a copy of the previous vector index with
`ckv reindex`, re-embedding only changed files. The graph is rebuilt at
every snapshot. Records per-step timing in chain.json.
"""
from __future__ import annotations

import argparse
import json
import shutil
import subprocess
import time
from pathlib import Path

STEP_TIMEOUT_SECONDS = 4 * 3600
EMBED = ['--embedder', 'ollama', '--model-name', 'bge-m3']


def run(args: list[str], log: Path) -> float:
    start = time.monotonic()
    with log.open('a') as out:
        out.write('$ ' + ' '.join(args) + '\n')
        out.flush()
        subprocess.run(args, stdout=out, stderr=subprocess.STDOUT, check=True, timeout=STEP_TIMEOUT_SECONDS)
    return time.monotonic() - start


def main() -> None:
    p = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    p.add_argument('--repo', required=True)
    p.add_argument('--snapshots', required=True, help='JSON list of commits in time order')
    p.add_argument('--work', required=True)
    p.add_argument('--bin', required=True)
    a = p.parse_args()
    work, bindir = Path(a.work), Path(a.bin)
    snaps: list[str] = json.loads(Path(a.snapshots).read_text())
    src = work / 'src-chain'
    if not src.exists():
        subprocess.run(['git', '-C', a.repo, 'worktree', 'add', '-q', '--detach', str(src), snaps[0]], check=True, timeout=600)
    record_path = work / 'chain.json'
    record = json.loads(record_path.read_text()) if record_path.exists() else []
    done = {r['snapshot'] for r in record}
    prev = None
    for snap in snaps:
        ds = work / f'chain-{snap}'
        if snap in done:
            prev = ds
            continue
        subprocess.run(['git', '-C', str(src), 'checkout', '-q', '--detach', snap], check=True, timeout=600)
        log = work / f'chain-{snap}.log'
        row = {'snapshot': snap}
        if prev is None:
            row['mode'] = 'full'
            row['seconds'] = run([str(bindir / 'cks'), 'setup', '--src', str(src), '--out', str(ds), *EMBED], log)
        else:
            row['mode'] = 'reindex'
            row['graph_seconds'] = run([str(bindir / 'cks'), 'setup', '--src', str(src), '--out', str(ds), '--skip-vector'], log)
            shutil.copytree(prev / 'vector', ds / 'vector')
            row['vector_seconds'] = run([str(bindir / 'ckv'), 'reindex', '--src', str(src), '--out', str(ds / 'vector'), *EMBED, '--json'], log)
        record.append(row)
        record_path.write_text(json.dumps(record, indent=2) + '\n')
        prev = ds


if __name__ == '__main__':
    main()
