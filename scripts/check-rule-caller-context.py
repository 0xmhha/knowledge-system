#!/usr/bin/env python3
"""Replay caller syntax; reject runtime, safety, independence and denominator claims."""
import argparse
import importlib.util
import json
from pathlib import Path
import subprocess

from source_packet import safe, sha

ROOT = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location('caller_context', ROOT / 'scripts/prepare-rule-caller-context.py')
prepare = importlib.util.module_from_spec(spec)
spec.loader.exec_module(prepare)


def check(packet, check_references=True):
    errors = []

    def require(value, message):
        if not value:
            errors.append(message)

    try:
        manifest = json.loads((packet / 'manifest.json').read_text())
        revision, proof, payload = prepare.source_data()
        require(manifest['schema_version'] == 1 and manifest['kind'] == 'rule_caller_context_preparation_only'
                and manifest['repository_commit'] == revision, 'caller packet identity')
        require(manifest['input_bindings'] == prepare.bindings(), 'caller live input bindings')
        expected = {'main.go.txt', 'main_test.go.txt', 'go.mod.txt', 'generator.py.txt', 'curation.json',
                    'source-inputs.json', 'logs/source-observations.json', 'logs/source-observations.stderr.txt',
                    'coverage-summary.json', 'caller-review.json', 'context-anchors.json', 'private-ai-context.json',
                    'source-proof-reference.json', 'formal-state.json', 'validation.json'}
        expected.update(f'logs/{kind}.{stream}.txt' for kind in ['build', 'fmt', 'vet', 'lint'] for stream in ['stdout', 'stderr'])
        expected.update(f'logs/{mode}.{suffix}' for mode in ['normal', 'race'] for suffix in ['jsonl', 'stderr.txt'])
        inventory = {p.relative_to(packet).as_posix() for p in packet.rglob('*') if p.is_file() and p.name != 'manifest.json'}
        require(inventory == set(manifest['artifacts']) == expected, 'caller artifact inventory')
        for name, digest in manifest['artifacts'].items():
            require(sha(safe(packet, name).read_bytes()) == digest, 'caller artifact hash: ' + name)
        for source, artifact in [(prepare.HELPER, 'main.go.txt'), (prepare.CONTROLS, 'main_test.go.txt'),
                                 (prepare.CURATION, 'curation.json'), (Path('scripts/prepare-rule-caller-context.py'), 'generator.py.txt')]:
            require((ROOT / source).read_bytes() == (packet / artifact).read_bytes(), 'caller source snapshot: ' + artifact)
        require((packet / 'go.mod.txt').read_bytes() == b'module rulecallercontext\n\ngo 1.26.9\n', 'caller isolated module')
        require(json.loads((packet / 'source-inputs.json').read_text()) == payload, 'caller source payload/private data boundary')
        require(json.loads((packet / 'source-proof-reference.json').read_text()) == {
            'manifest': str(prepare.PRIOR / 'manifest.json'), 'manifest_sha256': sha((ROOT / prepare.PRIOR / 'manifest.json').read_bytes()),
            'git_proof': str(prepare.PRIOR / 'git-proof.json'), 'git_proof_sha256': sha((ROOT / prepare.PRIOR / 'git-proof.json').read_bytes()),
            'repository_commit': revision, 'selected_files': {p: proof['files'][p] for p in prepare.PATHS},
            'scope': 'reuse_immutable_commit_to_source_proof'}, 'caller source proof reference')
        if errors:
            return errors
        observations = json.loads((packet / 'logs/source-observations.json').read_text())
        trusted = prepare.replay(json.dumps(payload), (ROOT / prepare.HELPER).read_bytes())
        require(observations == trusted, 'actual caller source replay mismatch')
        require(not (packet / 'logs/source-observations.stderr.txt').read_bytes(), 'caller source execution diagnostics')
        derived = prepare.derive(trusted)
        for name, value in derived.items():
            require(json.loads((packet / name).read_text()) == value, 'caller source/claim accounting: ' + name)
        require(manifest['counts'] == derived['coverage-summary.json']['counts'], 'caller count inflation')
        validation = json.loads((packet / 'validation.json').read_text())
        require(validation['kind'] == 'caller_syntax_and_development_controls_not_project_execution', 'caller validation scope')
        require(validation['go_version'].startswith('go version go1.26.9 ') and bool(validation['lint_version']), 'caller native tools')
        require(validation['deterministic_source_repetition'] is True and validation['target_source_parsed_not_executed'] is True, 'caller execution/determinism boundary')
        require(all(validation[k] == 0 for k in ['project_packages_built', 'context_scanner_kernel_calls', 'model_calls', 'mcp_calls', 'nodes_started', 'formal_runs']), 'caller runtime promotion')
        require(validation['quality_checks'] == [{'kind': k, 'exit_code': 0} for k in ['build', 'fmt', 'vet', 'lint']], 'caller quality accounting')
        for kind in ['build', 'fmt', 'vet', 'lint']:
            require(not (packet / f'logs/{kind}.stderr.txt').read_bytes(), 'caller quality diagnostics')
            stdout = (packet / f'logs/{kind}.stdout.txt').read_text().strip()
            require(stdout in ('', '0 issues.') if kind == 'lint' else not stdout, 'caller quality findings')
        require(validation['runs'] == [{'mode': m, 'exit_code': 0, 'syntax_controls': 9} for m in ['normal', 'race']], 'caller control matrix')
        for mode in ['normal', 'race']:
            events = [json.loads(line) for line in (packet / f'logs/{mode}.jsonl').read_text().splitlines() if line]
            text = ''.join(e.get('Output', '') for e in events)
            expected_passes = {'', 'TestCallerScanner'} | {'TestCallerScanner/' + name for name in prepare.CONTROL_NAMES}
            passes = [e.get('Test', '') for e in events if e['Action'] == 'pass']
            require(set(passes) == expected_passes and len(passes) == len(expected_passes)
                    and not any(e['Action'] in ['fail', 'skip'] for e in events) and 'WARNING: DATA RACE' not in text
                    and not (packet / f'logs/{mode}.stderr.txt').read_bytes(), 'caller raw native completion')
            controls = [line.split('CALLER_SYNTAX_CONTROL ', 1)[1] for line in text.splitlines() if 'CALLER_SYNTAX_CONTROL ' in line]
            require(set(controls) == {name + ' pass=true' for name in prepare.CONTROL_NAMES} and len(controls) == 9, 'caller raw control observations')
        formal = json.loads((packet / 'formal-state.json').read_text())
        require(formal == prepare.load('prepare-source-review-supplement.py').formal_state(ROOT)
                and formal['freeze'] is None and formal['execution_allowed'] is False, 'caller formal state promotion')
        if check_references:
            for name in ['PROTOCOL', 'PROGRESS']:
                doc = json.loads((ROOT / f'docs/spec-driven/OPERATIONAL-ACCEPTANCE-{name}.json').read_text())
                require(doc.get('rule_caller_context') == {'manifest': (packet / 'manifest.json').relative_to(ROOT).as_posix(),
                        'sha256': sha((packet / 'manifest.json').read_bytes()), 'status': 'caller_syntax_and_ai_context_not_runtime_or_independent_gold'}, 'caller ledger reference: ' + name)
    except (OSError, ValueError, KeyError, TypeError, AttributeError, IndexError, subprocess.SubprocessError) as exc:
        errors.append('invalid caller source context: ' + str(exc))
    return errors


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('packet', type=Path)
    args = parser.parse_args()
    errors = check(args.packet.resolve())
    if errors:
        print('\n'.join(errors))
        raise SystemExit(1)
    print('PASS: 14 syntax caller references / 5 unapproved context notes; runtime and independent labels unknown')
