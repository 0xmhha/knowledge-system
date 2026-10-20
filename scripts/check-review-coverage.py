#!/usr/bin/env python3
"""Rebuild WI-C material accounting and reject gold/priority/execution promotion."""
import argparse
import importlib.util
import json
from pathlib import Path

from source_packet import safe, sha

ROOT = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location('review_coverage_prepare', ROOT / 'scripts/prepare-review-coverage.py')
prepare = importlib.util.module_from_spec(spec)
spec.loader.exec_module(prepare)


def check(packet, check_references=True):
    errors = []

    def require(condition, message):
        if not condition:
            errors.append(message)

    try:
        manifest = json.loads((packet / 'manifest.json').read_text())
        require(manifest['kind'] == 'review_coverage_preparation_only', 'packet kind')
        data = prepare.source_inputs()
        require(set(manifest['input_bindings']) == {str(p) for p in prepare.bindings(data)}, 'input binding inventory')
        for path, digest in manifest['input_bindings'].items():
            require(sha(safe(ROOT, path).read_bytes()) == digest, 'input binding: ' + path)
        raw = (ROOT / 'scripts/prepare-review-coverage.py').read_bytes()
        require(manifest['generator_sha256'] == sha(raw) and (packet / 'generator.py.txt').read_bytes() == raw, 'generator binding')
        expected = prepare.generate(data)
        inventory = {p.relative_to(packet).as_posix() for p in packet.rglob('*') if p.is_file() and p.name != 'manifest.json'}
        require(inventory == set(manifest['artifacts']), 'artifact inventory')
        require(inventory == set(expected) | {'REVIEW-TABLE.txt', 'generator.py.txt', 'formal-state.json'}, 'unexpected artifact')
        for path, digest in manifest['artifacts'].items():
            require(sha(safe(packet, path).read_bytes()) == digest, 'artifact hash: ' + path)
        for path, value in expected.items():
            require(json.loads((packet / path).read_text()) == value, 'review coverage accounting: ' + path)
        require(manifest['counts'] == expected['coverage-summary.json']['counts'], 'manifest counts')
        require((packet / 'REVIEW-TABLE.txt').read_text() == prepare.table(expected), 'human review table')
        formal = json.loads((packet / 'formal-state.json').read_text())
        require(formal == prepare.load('prepare-source-review-supplement.py').formal_state(ROOT), 'formal state changed')
        require(formal['freeze'] is None and formal['execution_allowed'] is False, 'formal execution promotion')
        if check_references:
            ref = {'manifest': (packet / 'manifest.json').relative_to(ROOT).as_posix(),
                   'sha256': sha((packet / 'manifest.json').read_bytes()),
                   'status': 'review_material_mapping_not_independent_rubric_or_acceptance'}
            for name in ['PROTOCOL', 'PROGRESS']:
                doc = json.loads((ROOT / f'docs/spec-driven/OPERATIONAL-ACCEPTANCE-{name}.json').read_text())
                require(doc.get('review_coverage_preparation') == ref, 'ledger reference: ' + name)
    except (OSError, ValueError, KeyError, TypeError, IndexError) as exc:
        errors.append('invalid packet: ' + str(exc))
    return errors


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--packet', type=Path, required=True)
    parser.add_argument('--no-references', action='store_true')
    args = parser.parse_args()
    failures = check(args.packet.resolve(), not args.no_references)
    for failure in failures:
        print(failure)
    if not failures:
        print('review coverage preparation checks passed')
    raise SystemExit(bool(failures))
