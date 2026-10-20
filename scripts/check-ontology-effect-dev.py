#!/usr/bin/env python3
"""Reject omitted/tampered observations and unsupported ontology quality claims."""
import gzip
import hashlib
import importlib.util
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
PACKET = ROOT / 'system/eval/operational-preparation-2026-10-15/ontology-effect-dev'
spec = importlib.util.spec_from_file_location('ontology_effect_score', Path(__file__).with_name('score-ontology-effect-dev.py'))
score = importlib.util.module_from_spec(spec)
spec.loader.exec_module(score)


def check(folder=PACKET, root=ROOT):
    errors = []
    try:
        folder = folder.resolve()
        manifest = json.loads((folder / 'manifest.json').read_text())
        score.require(manifest['kind'] == 'source_bound_four_arm_development_observation'
                      and manifest['independent_acceptance'] is False
                      and manifest['production_activation'] is False, 'observation promoted to acceptance')
        names = [r['path'] for r in manifest['artifacts']]
        actual = {p.relative_to(folder).as_posix() for p in folder.rglob('*') if p.is_file() and p.name != 'manifest.json'}
        score.require(len(names) == len(set(names)) and set(names) == actual, 'artifact inventory differs')
        for row in manifest['artifacts']:
            p = (folder / row['path']).resolve()
            score.require(p.is_relative_to(folder) and p.is_file(), 'unsafe artifact path')
            raw = p.read_bytes()
            score.require(hashlib.sha256(raw).hexdigest() == row['sha256'] and len(raw) == row['bytes'], 'artifact hash differs')
            if 'uncompressed_sha256' in row:
                score.require(score.sha(gzip.decompress(raw)) == row['uncompressed_sha256'], 'raw capture bytes differ')
        for path, digest in manifest['scripts'].items():
            p = (root / path).resolve()
            score.require(p.is_relative_to(root.resolve()) and score.sha(p.read_bytes()) == digest, 'verification script differs')
        summary = score.analyze(folder)
        score.require(summary == json.loads((folder / 'summary.json').read_text()), 'recorded summary differs from raw calls')
        score.require(score.sha((root / manifest['public_requests']).read_bytes()) == manifest['public_requests_sha256'],
                      'prepared questions differ')
    except (OSError, ValueError, KeyError, TypeError, IndexError, ZeroDivisionError) as exc:
        errors.append(str(exc))
    return errors


if __name__ == '__main__':
    errors = check()
    for error in errors:
        print('FAIL:', error)
    print('ontology effect observation:', 'FAIL' if errors else 'PASS; local descriptive results; independent quality inconclusive')
    raise SystemExit(bool(errors))
