#!/usr/bin/env python3
"""Validate T10 scope, actual partial/error receipts and completion refusals."""
import argparse
import importlib.util
from pathlib import Path
import subprocess
import sqlite3
from source_packet import safe

ROOT = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location('partial_prepare', ROOT / 'scripts/prepare-consumer-partial.py')
p = importlib.util.module_from_spec(spec)
spec.loader.exec_module(p)


def expected_files(packet):
    names = {'manifest.json', '.gitattributes', 'execution.json', 'matrix-cases.json', 'legacy-index-control.json', 'formal-state.json', 'preservation.json', 'integrity.json', 'postrun-process-audit.json', 'partial.json', 'REPORT.txt', 'generator.py.txt'}
    for case in p.CASES:
        prefix = 'hosts/' + case + '/'
        names.update(prefix + name for name in p.old.old.TOP)
        if case in ['T10-build-seed', 'T10-build-field', 'T10-controls', 'T10-field-delivery']:
            names.add(prefix + 'action-resolutions.jsonl')
        state = p.read(packet / prefix / 'workspace-after.json')
        p.require(set(state) <= set(p.old.old.ARTIFACTS) | {'state.json', 'input-bundle.json', 'scripted-judgments.json', 'adapter-commands.json'}, 'unexpected workspace inventory')
        names.update(prefix + 'workspace-after/' + (name + '.txt' if name.endswith('.md') else name) for name in state)
    for case in p.CONTROLS:
        prefix = 'controls/' + case + '/'
        names.update(prefix + name for name in ['before.json', 'after-attempt.json', 'mutated.json', 'restore.json'])
        names.update(prefix + 'original/' + (name + '.txt' if name.endswith('.md') else name) for name in p.driver.FILES)
    names.update('indexes/' + kind + '/' + name for kind in ['normal', 'legacy'] for name in ['graph.db', 'manifest.json'])
    return names


def check(packet, references=True):
    try:
        manifest = p.read(packet / 'manifest.json')
        p.require(manifest['kind'] == 'T10_partial_unknown_not_safety_or_completeness', 'evidence scope')
        p.require(set(manifest['input_bindings']) == {str(f) for f in p.INPUTS}, 'input inventory')
        for name, value in manifest['input_bindings'].items():
            p.require(p.digest(safe(ROOT, name)) == value, 'input pin: ' + name)
        source = ROOT / 'scripts/prepare-consumer-partial.py'
        p.require(manifest['generator_sha256'] == p.digest(source) and (packet / 'generator.py.txt').read_bytes() == source.read_bytes(), 'generator pin')
        expected = expected_files(packet)
        actual = {f.relative_to(packet).as_posix() for f in packet.rglob('*') if f.is_file()}
        p.require(actual == expected and set(manifest['artifacts']) == expected - {'manifest.json'}, 'artifact inventory')
        for name, value in manifest['artifacts'].items():
            p.require(p.digest(safe(packet, name)) == value, 'artifact hash: ' + name)
        p.require((packet / '.gitattributes').read_text() == '**/*.db binary\n', 'raw database attributes')
        p.require(p.read(packet / 'partial.json') == p.derive(packet), 'derived T10 accounting/scope')
        report = (packet / 'REPORT.txt').read_text()
        p.require(all(s in report for s in ['빈 후보는 안전·누락0 증명이 아님', 'synthetic', 'requires_reindex', '정식0/9', '20/22']), 'report omitted partial/unknown scope')
        if references:
            ref = {'manifest': (packet / 'manifest.json').relative_to(ROOT).as_posix(), 'sha256': p.digest(packet / 'manifest.json'), 'status': 'T10_supported_partial_pass_plan20_of22_not_adopted_or_accepted'}
            for name in ['PROTOCOL', 'PROGRESS']:
                p.require(p.read(ROOT / f'docs/spec-driven/OPERATIONAL-ACCEPTANCE-{name}.json').get('consumer_partial') == ref, 'ledger ref: ' + name)
        return []
    except (OSError, ValueError, KeyError, TypeError, IndexError, StopIteration, SyntaxError, sqlite3.Error, subprocess.SubprocessError) as error:
        return [str(error)]


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--packet', type=Path, required=True)
    parser.add_argument('--no-references', action='store_true')
    args = parser.parse_args()
    errors = check(args.packet.resolve(), not args.no_references)
    for error in errors:
        print(error)
    if not errors:
        print('T10 verified:2 partial deliveries/10 CLI+hook refusals;6 hosts/9 CKS; plan20/22 formal0/9')
    raise SystemExit(bool(errors))
