#!/usr/bin/env python3
"""Validate T04 raw requests, reused projections and honest duplicate controls."""
import argparse
import importlib.util
from pathlib import Path
import subprocess
from source_packet import safe

ROOT = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location('opt_in_prepare', ROOT / 'scripts/prepare-consumer-opt-in.py')
p = importlib.util.module_from_spec(spec)
spec.loader.exec_module(p)


def expected_files(packet):
    expected = {'manifest.json', '.gitattributes', 'generator.py.txt', 'execution.json', 'formal-state.json', 'preservation.json', 'source-integrity.json', 'postrun-process-audit.json', 'opt-in.json', 'REPORT.txt'}
    allowed = set(p.ARTIFACTS) | {'state.json', 'input-bundle.json', 'scripted-judgments.json', 'adapter-commands.json'}
    for case in p.CASES:
        prefix = 'hosts/' + case + '/'
        expected.update(prefix + name for name in p.TOP)
        if case in ['review-build', 'planner-delivery']:
            expected.add(prefix + 'action-resolutions.jsonl')
        workspace = p.read(packet / prefix / 'workspace-after.json')
        p.require(set(workspace) <= allowed, 'unexpected saved workspace file')
        for name in workspace:
            expected.add(prefix + 'workspace-after/' + (name + '.txt' if name.endswith('.md') else name))
    expected.update('native/' + name for name in ['stdin.jsonl', 'stdout.jsonl', 'stderr.txt', 'messages.json', 'lifecycle.json'])
    proof = p.read(packet / 'native-source/git-proof.json')
    expected.add('native-source/git-proof.json')
    expected.update('native-source/' + name for name in proof['objects'].values())
    expected.update('native-source/' + row['artifact'] for row in proof['files'].values())
    return expected


def check(packet, references=True):
    try:
        manifest = p.read(packet / 'manifest.json')
        p.require(manifest['kind'] == 'T04_owned_opt_in_not_model_or_formal_acceptance', 'scope changed')
        p.require(set(manifest['input_bindings']) == {str(f) for f in p.INPUTS}, 'input inventory')
        for name, value in manifest['input_bindings'].items():
            p.require(p.digest(safe(ROOT, name)) == value, 'input pin: ' + name)
        source = ROOT / 'scripts/prepare-consumer-opt-in.py'
        p.require(manifest['generator_sha256'] == p.digest(source) and (packet / 'generator.py.txt').read_bytes() == source.read_bytes(), 'generator pin')
        expected = expected_files(packet)
        actual = {f.relative_to(packet).as_posix() for f in packet.rglob('*') if f.is_file()}
        p.require(actual == expected and set(manifest['artifacts']) == expected - {'manifest.json'}, 'artifact inventory')
        for name, value in manifest['artifacts'].items():
            p.require(p.digest(safe(packet, name)) == value, 'artifact hash: ' + name)
        p.require((packet / '.gitattributes').read_text() == 'native-source/objects/*.commit binary\nnative-source/objects/*.tree binary\n', 'raw Git object attributes differ')
        _, source_text = p.consumer()
        for role in ['analyzer', 'planner']:
            body = source_text('plugin/agents/' + role + '.md').split('---', 2)[2]
            before, review = body.split('\n## 7.', 1)
            review, after = review.split('\n## 8.', 1)
            p.require('go-local-review@1.0.0' in review and 'rule_candidates' in review and 'mode: code_review' in review, 'review opt-in section missing')
            p.require('go-local-review@1.0.0' not in before + after and 'rule_candidates' not in before + after, 'rule execution contract leaked to other modes')
            p.require(any('code_review' in line and '§7' in line and line.startswith('|') for line in before.splitlines()), 'mode dispatch does not select structured review')
        native = p.read(packet / 'native/messages.json')
        query = p.read(packet / 'execution.json')['query']
        invalid = [
            {'function': query},
            {'rule_pack': 'go-local-review', 'function': query},
            {'rule_pack': 'unknown@1.0.0', 'function': query},
            {'rule_pack': 'go-local-review@1.0.0'},
        ]
        p.require([r['request']['params'] for r in native[1:5]] == [{'name': 'cks.context.rule_candidates', 'arguments': value} for value in invalid], 'native invalid controls not actual missing/unknown inputs')
        p.require(p.read(packet / 'opt-in.json') == p.derive(packet), 'derived T04 accounting/scope')
        report = (packet / 'REPORT.txt').read_text()
        p.require(all(t in report for t in ['시험용 어댑터', '중복 요청은 native/host에서 허용', 'exit1', '정식0/9', '18/22']), 'report omits limits/failure')
        if references:
            ref = {'manifest': (packet / 'manifest.json').relative_to(ROOT).as_posix(), 'sha256': p.digest(packet / 'manifest.json'), 'status': 'T04_isolated_workflow_pass_plan18_of22_not_adopted_or_accepted'}
            for name in ['PROTOCOL', 'PROGRESS']:
                p.require(p.read(ROOT / f'docs/spec-driven/OPERATIONAL-ACCEPTANCE-{name}.json').get('consumer_opt_in') == ref, 'ledger ref: ' + name)
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
        print('T04 verified:7 hosts/8 CKS reaped/13 calls; reuse3 with0 CKS; plan18/22, formal0/9')
    raise SystemExit(bool(errors))
