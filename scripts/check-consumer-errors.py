#!/usr/bin/env python3
"""Validate T16 raw error receipts and fixed attempted-case denominators."""
import argparse
import importlib.util
from pathlib import Path
import sqlite3
import subprocess
from source_packet import safe

ROOT = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location('errors_prepare', ROOT / 'scripts/prepare-consumer-errors.py')
p = importlib.util.module_from_spec(spec)
spec.loader.exec_module(p)


def check(packet, references=True):
    try:
        manifest = p.read(packet / 'manifest.json')
        p.require(manifest['kind'] == 'T16_owned_errors_not_quality_acceptance', 'evidence scope')
        p.require(set(manifest['input_bindings']) == {str(f) for f in p.INPUTS}, 'input inventory')
        for name, value in manifest['input_bindings'].items():
            p.require(p.digest(safe(ROOT, name)) == value, 'input pin: ' + name)
        source = ROOT / 'scripts/prepare-consumer-errors.py'
        p.require(manifest['generator_sha256'] == p.digest(source) and (packet / 'generator.py.txt').read_bytes() == source.read_bytes(), 'generator pin')
        expected = {'manifest.json', 'execution.json', 'fault-setup.json', 'formal-state.json', 'preservation.json', 'integrity.json', 'postrun-process-audit.json', 'errors.json', 'REPORT.txt', 'generator.py.txt', '.gitattributes'}
        expected.update('sessions/' + name + '/' + file for name in p.SESSIONS for file in p.FILES)
        expected.update('configs/' + name + '.yaml' for name in p.SESSIONS)
        expected.update('indexes/' + name + '.db' for name in ['normal', 'legacy', 'broken'])
        actual = {f.relative_to(packet).as_posix() for f in packet.rglob('*') if f.is_file()}
        p.require(actual == expected and set(manifest['artifacts']) == expected - {'manifest.json'}, 'artifact inventory')
        for name, value in manifest['artifacts'].items():
            p.require(p.digest(safe(packet, name)) == value, 'artifact hash: ' + name)
        p.require((packet / '.gitattributes').read_text() == '**/*.db binary\n', 'raw database attributes')
        p.require(p.read(packet / 'errors.json') == p.derive(packet), 'derived T16 accounting/scope')
        report = (packet / 'REPORT.txt').read_text()
        p.require(all(s in report for s in ['평가7', '지원partial2+비사용6', 'late response', 'manifest 누락', '정식0/9', '21/22']), 'report omitted fault/denominator scope')
        if references:
            ref = {'manifest': (packet / 'manifest.json').relative_to(ROOT).as_posix(), 'sha256': p.digest(packet / 'manifest.json'), 'status': 'T16_owned_errors_pass_plan21_of22_not_adopted_or_accepted'}
            for name in ['PROTOCOL', 'PROGRESS']:
                p.require(p.read(ROOT / f'docs/spec-driven/OPERATIONAL-ACCEPTANCE-{name}.json').get('consumer_errors') == ref, 'ledger ref: ' + name)
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
        print('T16 verified:8 scenarios=2 partial+6 unusable; late reply excluded; plan21/22 formal0/9')
    raise SystemExit(bool(errors))
