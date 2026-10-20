#!/usr/bin/env python3
"""Check actual raw Go events, source-bound fixtures and bounded observation claims."""
import argparse
import hashlib
import importlib.util
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
BASE = Path('docs/spec-driven/operational-preparation/source-review-supplement')


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def check(packet, check_references=True):
    errors = []

    def require(condition, message):
        if not condition:
            errors.append(message)

    def safe(name):
        path = Path(name)
        if path.is_absolute() or '..' in path.parts or not (packet / path).resolve().is_relative_to(packet.resolve()) or (packet / path).is_symlink():
            raise ValueError('unsafe artifact')
        return packet / path

    try:
        manifest = json.loads((packet / 'manifest.json').read_text())
        require(manifest['kind'] == 'subscription_extracted_method_local_probe', 'probe kind')
        ticket_manifest = BASE / 'subscription-ticket-v1/manifest.json'
        ticket = (ROOT / ticket_manifest).parent
        require(manifest['ticket_manifest'] == str(ticket_manifest)
                and manifest['ticket_manifest_sha256'] == sha((ROOT / ticket_manifest).read_bytes()), 'ticket binding')
        spec = importlib.util.spec_from_file_location('subscription_ticket_guard', ROOT / 'scripts/check-subscription-ticket.py')
        guard = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(guard)
        errors.extend(guard.check(ticket))
        actual = {p.relative_to(packet).as_posix() for p in packet.rglob('*') if p.is_file() and p.name != 'manifest.json'}
        require(actual == set(manifest['artifacts']), 'artifact inventory')
        for name, digest in manifest['artifacts'].items():
            require(sha(safe(name).read_bytes()) == digest, 'artifact hash: ' + name)
        require(manifest['harness'] == 'scripts/fixtures/subscription_probe_test.go.txt', 'harness identity')
        harness = (ROOT / manifest['harness']).read_bytes()
        require(sha(harness) == manifest['harness_sha256'], 'harness binding')
        require((packet / 'runner.py.txt').read_bytes() == (ROOT / 'scripts/run-subscription-probe.py').read_bytes(), 'runner binding')
        quality = json.loads((packet / 'quality.json').read_text())
        require(quality['kind'] == 'quality_checks_of_recorded_extracted_fixture_not_production', 'quality check scope')
        require((packet / 'quality-validator.py.txt').read_bytes() == (ROOT / 'scripts/validate-subscription-fixture.py').read_bytes(), 'quality validator binding')
        private = json.loads((ticket / 'private-review.json').read_text())
        expected = private['expected_draft_matrix']
        variants = {v['id']: v['artifact'] for v in private['variants']}
        scenarios = set(expected['ORIGINAL'])
        summary = json.loads((packet / 'summary.json').read_text())
        require(summary['kind'] == 'local_extracted_method_probe_not_full_project_or_independent_acceptance', 'observation scope')
        require(summary['local_model_calls'] == summary['nodes_started'] == summary['formal_runtime_executions'] == summary['independent_families'] == 0
                and summary['independent_acceptable_candidate_set'] is None and summary['independent_oracle'] is None
                and summary['verified_project_defect'] is False, 'probe promotion')
        require(summary['toolchain'].startswith('go version go1.26.9 '), 'toolchain identity')
        require(quality['go_version'] == summary['toolchain'] and bool(quality['lint_version']), 'quality tool identity')
        require(quality['compiled_inputs_sha256'] == summary['compiled_inputs_sha256'], 'quality compiled-input binding')
        checks = quality['checks']
        require(len(checks) == 20 and {(c['variant'], c['check']) for c in checks} ==
                {(v, c) for v in variants for c in ['build', 'fmt', 'vet', 'lint']}, 'quality check matrix')
        commands = {'build': ['build', './...'], 'fmt': ['-l', 'types.go', 'variant.go', 'subscription_test.go'],
                    'vet': ['vet', './...'], 'lint': ['run', '--no-config', '--timeout=60s']}
        for receipt in checks:
            require(receipt['exit_code'] == 0 and receipt['command'] == commands[receipt['check']], 'quality check command/result')
            stem = 'quality-' + receipt['variant'] + '-' + receipt['check']
            require(receipt['log_stem'] == stem, 'quality log binding')
            require(not (packet / 'logs' / (stem + '.stderr.txt')).read_bytes(), 'quality stderr')
            stdout = (packet / 'logs' / (stem + '.stdout.txt')).read_text().strip()
            require(not stdout if receipt['check'] != 'lint' else stdout in ('', '0 issues.'), 'quality findings')
        require(summary['candidate_result_scope'] == 'AI fixture criteria only; no production patch adopted', 'candidate scope')
        require(summary['limitations'] == private['limitations'], 'preserved limitations')
        require(manifest['counts'] == {'variants': 5, 'correlated_schedules': 5, 'modes': 2, 'observations': 50}, 'probe counts')
        require(set(summary['compiled_inputs_sha256']) == set(variants), 'compiled variants')
        for variant, artifact in variants.items():
            inputs = summary['compiled_inputs_sha256'][variant]
            require(set(inputs) == {'go.mod', 'types.go', 'variant.go', 'subscription_test.go', 'expectations.json'}, 'compiled file inventory')
            folder = packet / 'compiled-inputs' / variant
            for name, digest in inputs.items():
                require(sha((folder / (name + '.txt')).read_bytes()) == digest, 'compiled input hash')
            require((folder / 'variant.go.txt').read_bytes() == b'package subscriptionfixture\n\n' + (ticket / artifact).read_bytes(), 'compiled method differs from ticket')
            require((folder / 'subscription_test.go.txt').read_bytes() == harness, 'compiled harness mismatch')
            require((folder / 'go.mod.txt').read_bytes() == b'module subscriptionfixture\n\ngo 1.26.9\n', 'module dependencies')
            types = b'package subscriptionfixture\n\nimport "sync"\n\n' + (ticket / 'source-types.go.txt').read_bytes()
            types += b'\ntype DownloaderAPI struct {\n\tuninstallSyncSubscription chan *uninstallSyncSubscriptionRequest\n}\n'
            require((folder / 'types.go.txt').read_bytes() == types, 'compiled types/stub boundary')
            require(json.loads((folder / 'expectations.json.txt').read_text()) == expected, 'compiled draft expectations')
        rows = summary['observations']
        keys = {(r['variant'], r['mode'], r['scenario']) for r in rows}
        require(len(rows) == len(keys) == 50 and keys == {(v, m, s) for v in variants for m in ['normal', 'race'] for s in scenarios}, 'complete observation matrix')
        runs = {(r['variant'], r['mode']): r for r in summary['runs']}
        require(len(summary['runs']) == len(runs) == 10, 'run count')
        for (variant, mode), run in runs.items():
            require(variant in variants and mode in ['normal', 'race'], 'run identity')
            command = ['test', '-json', '-count=1', '-timeout=20s'] + (['-race'] if mode == 'race' else []) + ['./...']
            require(run['exit_code'] == 0 and run['observations'] == 5 and run['command'] == command, 'run command/result')
            stem = variant + '-' + mode
            events = [json.loads(line) for line in (packet / 'logs' / (stem + '.jsonl')).read_text().splitlines() if line]
            output = ''.join(e.get('Output', '') for e in events)
            require(not (packet / 'logs' / (stem + '.stderr.txt')).read_bytes(), 'unexpected Go stderr')
            require(not any(e.get('Action') in ['fail', 'skip'] for e in events)
                    and 'WARNING: DATA RACE' not in output, 'failed/skipped/racing fixture')
            passed = {e.get('Test') for e in events if e.get('Action') == 'pass' and e.get('Test')}
            require(passed == {'TestExtractedMethod'} | {'TestExtractedMethod/' + s for s in scenarios}
                    and any(e.get('Action') == 'pass' and not e.get('Test') for e in events), 'raw passed test set')
            observed = []
            for line in output.splitlines():
                if 'DRAFT_OBSERVATION ' in line:
                    record = json.loads(line.split('DRAFT_OBSERVATION ', 1)[1])
                    record['mode'] = mode
                    observed.append(record)
            selected = [r for r in rows if r['variant'] == variant and r['mode'] == mode]
            require(observed == selected and len(observed) == 5, 'summary/raw observation mismatch')
        for row in rows:
            variant, scenario = row['variant'], row['scenario']
            accepted = (row['completed_before_rescue'] and row['confirmation_issued_before_rescue']
                        and row['subscription_removed_before_rescue'] and row['requests_before_rescue'] == 1
                        and not row['returned_before_confirmation'])
            if scenario == 'confirmation_delayed':
                accepted = accepted and row['confirmation_delay_observed']
            require(row['satisfies_ai_fixture_criteria'] == accepted == expected[variant][scenario], 'recomputed criteria mismatch')
            require(row['all_owned_goroutines_stopped'] is True, 'owned goroutine cleanup')
            blocked = variant in {'ORIGINAL', 'MUT-REINTRO'} and scenario == 'blocked_delivery_then_consumer_exit'
            require(row['two_sender_wait_witness'] == blocked and row['completed_before_rescue'] == (not blocked), 'blocked/completion distinction')
            if blocked:
                blocks = [b.strip() for b in row['two_sender_wait_stacks'].split('\n\n') if b.strip()]
                require(any('(*pilot).sendStatus' in b and '[select]' in b.split('\n')[0] for b in blocks)
                        and any('(*SyncStatusSubscription).Unsubscribe' in b and '[chan send]' in b.split('\n')[0]
                                and '/variant.go:' in b for b in blocks), 'missing concrete sender wait stacks')
            else:
                require(row['two_sender_wait_stacks'] == '', 'unexpected wait witness')
        diagnostics = json.loads((packet / 'observer-diagnostics.json').read_text())
        require(diagnostics['kind'] == 'retained_observer_failures_not_suppressed'
                and len(diagnostics['attempts']) == 2, 'observer failure accounting')
        for attempt in diagnostics['attempts']:
            require(attempt['id'] in {'initial', 'diagnostic'} and attempt['data_race_reported'] is False, 'observer diagnostic identity')
            folder = packet / 'observer-diagnostics' / attempt['id']
            events = [json.loads(line) for line in (folder / 'ORIGINAL-race.jsonl').read_text().splitlines() if line]
            failed = {e.get('Test') for e in events if e.get('Action') == 'fail' and '/' in e.get('Test', '')}
            require(failed == {'TestExtractedMethod/' + s for s in attempt['failed_subtests']}, 'retained failed observer cases')
            output = ''.join(e.get('Output', '') for e in events)
            require('inconclusive: did not observe' in output and 'WARNING: DATA RACE' not in output, 'observer failure classification')
            require('Unsubscribe.func1' in (folder / 'harness.go.txt').read_text(), 'prior observer predicate')
        if check_references:
            for name in ['PROTOCOL', 'PROGRESS']:
                d = json.loads((ROOT / f'docs/spec-driven/OPERATIONAL-ACCEPTANCE-{name}.json').read_text())
                require(d.get('subscription_local_probe') == {
                    'manifest': (packet / 'manifest.json').relative_to(ROOT).as_posix(),
                    'sha256': sha((packet / 'manifest.json').read_bytes()),
                    'status': 'extracted_method_fixture_only_not_independent_acceptance'}, 'probe ledger reference: ' + name)
    except (OSError, ValueError, KeyError, TypeError, IndexError) as exc:
        errors.append('invalid probe packet: ' + str(exc))
    return errors


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('packet', type=Path)
    args = parser.parse_args()
    errors = check(args.packet.resolve())
    if errors:
        print('\n'.join(errors))
        raise SystemExit(1)
    print('PASS: 50 extracted-method observations, normal/race, raw Go events and owned cleanup; independent acceptance unchanged')
