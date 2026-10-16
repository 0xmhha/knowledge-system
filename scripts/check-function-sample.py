#!/usr/bin/env python3
"""Check deterministic sample/exposure accounting without independent acceptance."""
import argparse
import importlib.util
import json
from pathlib import Path
import subprocess

from source_packet import safe, sha

ROOT = Path(__file__).resolve().parents[1]
BASE = Path('docs/spec-driven/operational-preparation/source-review-supplement')


def load(name):
    spec = importlib.util.spec_from_file_location(name.replace('-', '_'), ROOT / 'scripts' / name)
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


prepare = load('prepare-function-sample.py')
lexical = load('check-lexical-coverage.py')


def check(packet, check_references=True):
    errors = []

    def require(condition, message):
        if not condition:
            errors.append(message)

    try:
        manifest = json.loads((packet / 'manifest.json').read_text())
        require(manifest['kind'] == 'function_sample_preparation_only', 'packet kind')
        bindings = set(prepare.INPUTS.values()) | set(prepare.MANIFESTS)
        require(set(manifest['input_bindings']) == {str(p) for p in bindings}, 'input binding inventory')
        for path, digest in manifest['input_bindings'].items():
            require(sha(safe(ROOT, path).read_bytes()) == digest, 'input binding: ' + path)
        raw = (ROOT / 'scripts/prepare-function-sample.py').read_bytes()
        require(manifest['generator_sha256'] == sha(raw) and (packet / 'generator.py.txt').read_bytes() == raw, 'generator source binding')
        errors.extend(lexical.check(ROOT / BASE / 'lexical-coverage-v1'))
        inputs = {key: json.loads((ROOT / path).read_text()) for key, path in prepare.INPUTS.items()}
        for parent in prepare.MANIFESTS:
            original = json.loads((ROOT / parent).read_text())
            for path in prepare.INPUTS.values():
                if path.is_relative_to(parent.parent):
                    require(original['artifacts'][path.relative_to(parent.parent).as_posix()] == sha((ROOT / path).read_bytes()), 'prior artifact provenance: ' + str(path))
        if errors:
            return errors
        expected = prepare.generate(inputs)
        inventory = {p.relative_to(packet).as_posix() for p in packet.rglob('*') if p.is_file() and p.name != 'manifest.json'}
        require(inventory == set(manifest['artifacts']), 'artifact inventory')
        require(inventory == set(expected) | {'REVIEW-TABLE.txt', 'formal-state.json', 'generator.py.txt'}, 'unexpected sample artifact')
        for name, digest in manifest['artifacts'].items():
            require(sha(safe(packet, name).read_bytes()) == digest, 'artifact hash: ' + name)
        for name, data in expected.items():
            require(json.loads((packet / name).read_text()) == data, 'sample/exposure accounting: ' + name)
        summary = expected['sample-summary.json']
        require(manifest['counts'] == summary['counts'], 'manifest counts')
        require(summary['counts']['selected_functions'] == 30 and summary['counts']['reserve_functions'] == 9
                and summary['counts']['modules'] == 8 and summary['counts']['approved_independent_functions'] == 0, 'proposal function/module counts')
        require(all(c >= 4 for c in summary['construct_function_counts'].values()), 'construct proposal floor')
        require((packet / 'REVIEW-TABLE.txt').read_text() == prepare.table(expected['function-inventory.json']), 'human review table')
        formal = json.loads((packet / 'formal-state.json').read_text())
        require(formal == prepare.load('prepare-source-review-supplement.py').formal_state(ROOT), 'formal state changed')
        require(formal['freeze'] is None and formal['execution_allowed'] is False, 'formal execution promotion')
        if check_references:
            for name in ['PROTOCOL', 'PROGRESS']:
                doc = json.loads((ROOT / f'docs/spec-driven/OPERATIONAL-ACCEPTANCE-{name}.json').read_text())
                require(doc.get('function_sample_preparation') == {'manifest': (packet / 'manifest.json').relative_to(ROOT).as_posix(),
                        'sha256': sha((packet / 'manifest.json').read_bytes()), 'status': 'exposed_review_sample_not_independent_holdout_or_gold'}, 'ledger reference: ' + name)
    except (OSError, ValueError, KeyError, TypeError, IndexError, subprocess.SubprocessError) as exc:
        errors.append('invalid function sample: ' + str(exc))
    return errors


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('packet', type=Path)
    args = parser.parse_args()
    errors = check(args.packet.resolve())
    if errors:
        print('\n'.join(errors))
        raise SystemExit(1)
    print('PASS: 30 exposed function proposals, 9 reserves, 8 modules; independent functions and gold remain 0')
