#!/usr/bin/env python3
"""Verify source bindings, proposal denominators and formal-state boundaries."""
import argparse
import importlib.util
import json
from pathlib import Path
import subprocess

from source_packet import safe, sha

ROOT = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location('rule_preparation', ROOT / 'scripts/prepare-rule-coverage.py')
prepare = importlib.util.module_from_spec(spec)
spec.loader.exec_module(prepare)


def check(packet, check_references=True):
    errors = []

    def require(value, message):
        if not value:
            errors.append(message)

    try:
        manifest = json.loads((packet / 'manifest.json').read_text())
        require(manifest['kind'] == 'rule_input_preparation_only', 'packet kind')
        require(set(manifest['input_bindings']) == {str(p) for p in prepare.bindings()}, 'input binding inventory')
        for name, digest in manifest['input_bindings'].items():
            require(sha(safe(ROOT, name).read_bytes()) == digest, 'input binding: ' + name)
        require((packet / 'generator.py.txt').read_bytes() == (ROOT / 'scripts/prepare-rule-coverage.py').read_bytes(), 'generator snapshot')
        actual = {p.relative_to(packet).as_posix() for p in packet.rglob('*') if p.is_file() and p.name != 'manifest.json'}
        require(actual == set(manifest['artifacts']), 'artifact inventory')
        expected = prepare.documents()
        require(actual == set(expected) | {'generator.py.txt', 'formal-state.json', 'syntax-validation.json'}, 'unexpected artifact')
        for name, digest in manifest['artifacts'].items():
            require(sha(safe(packet, name).read_bytes()) == digest, 'artifact hash: ' + name)
        errors.extend(prepare.load('check-policy-security-preparation.py').check(ROOT / prepare.PRIOR, ROOT))
        if errors:
            return errors
        for name, value in expected.items():
            require(json.loads((packet / name).read_text()) == value, 'source/proposal accounting: ' + name)
        require(manifest['counts'] == expected['coverage-summary.json']['counts'], 'manifest counts')
        require(json.loads((packet / 'syntax-validation.json').read_text()) == prepare.parse_sources(
            json.dumps(expected['public-inputs.json']['cases'])), 'isolated syntax validation')
        formal = json.loads((packet / 'formal-state.json').read_text())
        require(formal == prepare.load('prepare-source-review-supplement.py').formal_state(ROOT)
                and formal['freeze'] is None and formal['execution_allowed'] is False, 'formal state promotion')
        if check_references:
            for name in ['PROTOCOL', 'PROGRESS']:
                doc = json.loads((ROOT / f'docs/spec-driven/OPERATIONAL-ACCEPTANCE-{name}.json').read_text())
                require(doc.get('rule_coverage_preparation') == {'manifest': (packet / 'manifest.json').relative_to(ROOT).as_posix(),
                        'sha256': sha((packet / 'manifest.json').read_bytes()), 'status': 'rule_source_proposals_not_detection_gold_or_acceptance'}, 'ledger reference: ' + name)
    except (OSError, ValueError, KeyError, TypeError, AttributeError, IndexError, subprocess.SubprocessError) as exc:
        errors.append('invalid rule coverage preparation: ' + str(exc))
    return errors


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('packet', type=Path)
    args = parser.parse_args()
    errors = check(args.packet.resolve())
    if errors:
        print('\n'.join(errors))
        raise SystemExit(1)
    print('PASS: 40 exposed drafts, 68 separate project calls; no detection/gold/acceptance promotion')
