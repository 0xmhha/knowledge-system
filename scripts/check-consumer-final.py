#!/usr/bin/env python3
"""Validate final candidate readiness without inventing deployment or acceptance."""
import argparse
import importlib.util
from pathlib import Path
import subprocess
from source_packet import safe

ROOT = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location('final_prepare', ROOT / 'scripts/prepare-consumer-final.py')
p = importlib.util.module_from_spec(spec)
spec.loader.exec_module(p)


def check(packet, references=True):
    try:
        manifest = p.read(packet / 'manifest.json')
        p.require(manifest['kind'] == 'P08_T22_final_owned_readiness_not_adoption_or_acceptance', 'evidence scope')
        p.require(set(manifest['input_bindings']) == {str(f) for f in p.INPUTS}, 'input inventory')
        for name, value in manifest['input_bindings'].items():
            p.require(p.digest(safe(ROOT, name)) == value, 'input pin: ' + name)
        source = ROOT / 'scripts/prepare-consumer-final.py'
        p.require(manifest['generator_sha256'] == p.digest(source) and (packet / 'generator.py.txt').read_bytes() == source.read_bytes(), 'generator pin')
        expected = {*p.TOP, 'formal-state.json', 'preservation.json', 'historical-process-audit.json', 'commit-boundary.json', 'case-matrix.json', 'handoff.json', 'boundary.json', 'REPORT.txt', '.gitattributes', 'generator.py.txt', 'manifest.json'}
        expected.update('query/' + name for name in p.QUERY)
        expected.update('backup/' + name + ('.txt' if name.endswith('.go') else '') for name in p.read(ROOT / 'scripts/fixtures/consumer-final-plan.json')['owned_restore_files'])
        actual = {f.relative_to(packet).as_posix() for f in packet.rglob('*') if f.is_file()}
        p.require(actual == expected and set(manifest['artifacts']) == expected - {'manifest.json'}, 'artifact inventory')
        for name, value in manifest['artifacts'].items():
            p.require(p.digest(safe(packet, name)) == value, 'artifact hash: ' + name)
        p.require((packet / '.gitattributes').read_text() == '**/*.db binary\n', 'raw database attributes')
        p.require(p.read(packet / 'boundary.json') == p.derive(packet), 'derived T22 boundary/scope')
        report = (packet / 'REPORT.txt').read_text()
        p.require(all(s in report for s in ['22/22', '정식0/9', '과거 기준선', 'PID 현재 부재', '미확인 CKS exit1건', '온톨로지 효과', '후속12개']), 'report omitted limitations/remaining scope')
        if references:
            ref = {'manifest': (packet / 'manifest.json').relative_to(ROOT).as_posix(), 'sha256': p.digest(packet / 'manifest.json'), 'status': 'P08_T22_isolated_plan22_of22_not_adopted_or_accepted'}
            for name in ['PROTOCOL', 'PROGRESS']:
                p.require(p.read(ROOT / f'docs/spec-driven/OPERATIONAL-ACCEPTANCE-{name}.json').get('consumer_final') == ref, 'ledger ref: ' + name)
        return []
    except (OSError, ValueError, KeyError, TypeError, IndexError, StopIteration, SyntaxError, subprocess.SubprocessError) as error:
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
        print('T22 verified:22/22 isolated readiness; exact restore/query/cleanup; formal0/9/adoption0')
    raise SystemExit(bool(errors))
