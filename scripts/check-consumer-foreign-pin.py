#!/usr/bin/env python3
"""Reject T09 false completion, local-ID overclaims and hidden initial failures."""
import argparse
import importlib.util
from pathlib import Path
import subprocess
from source_packet import safe

ROOT = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location('foreign_pin_prepare', ROOT / 'scripts/prepare-consumer-foreign-pin.py')
p = importlib.util.module_from_spec(spec)
spec.loader.exec_module(p)


def expected_files(packet):
    names = {'manifest.json', '.gitattributes', 'execution.json', 'commands.json', 'matrix-cases.json', 'formal-state.json', 'preservation.json', 'integrity.json', 'postrun-process-audit.json', 'foreign-pin.json', 'REPORT.txt', 'generator.py.txt'}
    for parent in ['', 'diagnostics/integration/']:
        for case in p.CASES:
            prefix = parent + 'hosts/' + case + '/'
            names.update(prefix + name for name in p.old.TOP)
            if case != 'T09-matrix':
                names.add(prefix + 'action-resolutions.jsonl')
            state = p.read(packet / prefix / 'workspace-after.json')
            p.require(set(state) <= set(p.old.ARTIFACTS) | {'state.json', 'input-bundle.json', 'scripted-judgments.json', 'adapter-commands.json'}, 'unexpected workspace inventory')
            names.update(prefix + 'workspace-after/' + (name + '.txt' if name.endswith('.md') else name) for name in state)
        for case in p.CONTROLS:
            prefix = parent + 'controls/' + case + '/'
            names.update(prefix + name for name in ['before.json', 'after-attempt.json', 'mutated.json', 'restore.json'])
            names.update(prefix + 'original/' + (name + '.txt' if name.endswith('.md') else name) for name in p.driver.FILES)
        for folder in ['native-B', 'discovery-B']:
            names.update(parent + folder + '/' + name for name in p.SESSION)
    names.update('diagnostics/discovery-initial/' + name for name in p.SESSION)
    names.update('diagnostics/' + name for name in ['summary.json', 'driver-initial.py.txt', 'discovery-controller.stderr.txt', 'driver-before-collision.py.txt', 'plan-before-collision.json', 'integration/execution.json'])
    names.update('indexes/' + project + '/' + name for project in ['A', 'B'] for name in ['graph.db', 'manifest.json'])
    proof = p.read(packet / 'project-B-source/git-proof.json')
    names.add('project-B-source/git-proof.json')
    names.update('project-B-source/' + name for name in proof['objects'].values())
    names.update('project-B-source/' + row['artifact'] for row in proof['files'].values())
    return names


def check(packet, references=True):
    try:
        m = p.read(packet / 'manifest.json')
        p.require(m['kind'] == 'T09_two_project_pins_not_tenant_authentication', 'evidence scope')
        p.require(set(m['input_bindings']) == {str(f) for f in p.INPUTS}, 'input inventory')
        for name, value in m['input_bindings'].items():
            p.require(p.digest(safe(ROOT, name)) == value, 'input pin: ' + name)
        source = ROOT / 'scripts/prepare-consumer-foreign-pin.py'
        p.require(m['generator_sha256'] == p.digest(source) and (packet / 'generator.py.txt').read_bytes() == source.read_bytes(), 'generator pin')
        expected = expected_files(packet)
        actual = {f.relative_to(packet).as_posix() for f in packet.rglob('*') if f.is_file()}
        p.require(actual == expected and set(m['artifacts']) == expected - {'manifest.json'}, 'artifact inventory')
        for name, value in m['artifacts'].items():
            p.require(p.digest(safe(packet, name)) == value, 'artifact hash: ' + name)
        p.require((packet / '.gitattributes').read_text() == '**/*.commit binary\n**/*.tree binary\n**/*.db binary\n', 'raw object/database attributes')
        schema = p.read(ROOT / 'internal/system/mcp/testdata/agent-mcp.schema.json')['tools']['cks.context.review_function']['input']['properties']
        p.require(set(schema) == {'symbol', 'caller_depth', 'max_items'}, 'structural supported input contract changed')
        p.require(p.read(packet / 'foreign-pin.json') == p.derive(packet), 'derived T09 accounting/scope')
        report = (packet / 'REPORT.txt').read_text()
        p.require(all(s in report for s in ['node ID가 충돌', '초기', 'stale-read', '미지원', '정식0/9', '19/22']), 'report hides collision/failures/scope')
        if references:
            ref = {'manifest': (packet / 'manifest.json').relative_to(ROOT).as_posix(), 'sha256': p.digest(packet / 'manifest.json'), 'status': 'T09_two_project_pins_pass_plan19_of22_not_adopted_or_accepted'}
            for name in ['PROTOCOL', 'PROGRESS']:
                p.require(p.read(ROOT / f'docs/spec-driven/OPERATIONAL-ACCEPTANCE-{name}.json').get('consumer_foreign_pin') == ref, 'ledger ref: ' + name)
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
        print('T09 verified:final4 hosts/42 CKS calls;24 matrix+9 hook refusals; initial failures retained; plan19/22 formal0/9')
    raise SystemExit(bool(errors))
