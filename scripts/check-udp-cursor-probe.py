#!/usr/bin/env python3
"""Validate raw synthetic list-cursor observations without claiming UDP reachability."""
import argparse
import hashlib
import importlib.util
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
BASE = Path('docs/spec-driven/operational-preparation/source-review-supplement')


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def load(name):
    spec = importlib.util.spec_from_file_location(name.replace('-', '_'), ROOT / 'scripts' / name)
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


def check(packet, check_references=True):
    errors = []

    def require(condition, message):
        if not condition:
            errors.append(message)

    try:
        manifest = json.loads((packet / 'manifest.json').read_text())
        require(manifest['kind'] == 'udp_cursor_synthetic_local_probe', 'probe kind')
        candidate = BASE / 'schedule-candidates-v1/manifest.json'
        require(manifest['candidate_manifest'] == str(candidate)
                and manifest['candidate_manifest_sha256'] == sha((ROOT / candidate).read_bytes()), 'candidate binding')
        errors.extend(load('check-schedule-candidates.py').check((ROOT / candidate).parent))
        inventory = {p.relative_to(packet).as_posix() for p in packet.rglob('*') if p.is_file() and p.name != 'manifest.json'}
        require(inventory == set(manifest['artifacts']), 'artifact inventory')
        for name, digest in manifest['artifacts'].items():
            path = Path(name)
            if path.is_absolute() or '..' in path.parts or not (packet / path).resolve().is_relative_to(packet.resolve()) or (packet / path).is_symlink():
                raise ValueError('unsafe artifact')
            require(sha((packet / path).read_bytes()) == digest, 'artifact hash: ' + name)
        require(manifest['template'] == 'scripts/fixtures/udp_cursor_probe_test.go.txt', 'template identity')
        template = (ROOT / manifest['template']).read_bytes()
        require(sha(template) == manifest['template_sha256'], 'template binding')
        require((packet / 'runner.py.txt').read_bytes() == (ROOT / 'scripts/run-udp-cursor-probe.py').read_bytes(), 'runner binding')
        source = (ROOT / candidate.parent / 'sources/p2p/discover/v4_udp.go.txt').read_bytes()
        original = b''.join(source.splitlines(keepends=True)[419:440])
        draft = load('run-udp-cursor-probe.py').cursor_draft(original)
        require((packet / 'original-closure.go.txt').read_bytes() == original, 'exact original closure')
        require((packet / 'cursor-draft-closure.go.txt').read_bytes() == draft, 'declared draft patch')
        summary = json.loads((packet / 'summary.json').read_text())
        require(summary['kind'] == 'synthetic_source_branch_probe_not_udp_or_clock_rollback', 'synthetic scope')
        require(summary['go_version'].startswith('go version go1.26.9 ') and bool(summary['lint_version']), 'toolchain')
        require(summary['timer_scope'] == 'Reset/Stop call recorder; real timer firing is not tested', 'timer overclaim')
        require(summary['network_calls'] == summary['nodes_started'] == summary['host_clock_changes'] == summary['formal_runs'] == summary['accepted_independent_families'] == 0
                and summary['verified_project_defect'] is False and summary['operational_reachability'] == 'not_verified'
                and summary['independent_oracle'] is None and summary['production_patch_applied'] is False, 'probe promotion')
        require(manifest['counts'] == {'arms': 2, 'synthetic_cases': 5, 'modes': 2, 'observations': 20, 'quality_checks': 8}, 'count inflation')
        variants = {'ORIGINAL': original, 'CURSOR-DRAFT': draft}
        for variant, fragment in variants.items():
            expected = {'go.mod': b'module udpcursorfixture\n\ngo 1.26.9\n',
                        'cursor_test.go': template.replace(b'\t// RESET_TIMEOUT_SOURCE\n', fragment)}
            require(set(summary['compiled_inputs_sha256'][variant]) == set(expected), 'compiled inventory')
            for name, raw in expected.items():
                require((packet / 'compiled-inputs' / variant / (name + '.txt')).read_bytes() == raw
                        and summary['compiled_inputs_sha256'][variant][name] == sha(raw), 'compiled source mismatch')
        checks = summary['quality_checks']
        require(len(checks) == 8 and {(c['variant'], c['kind']) for c in checks} ==
                {(v, k) for v in variants for k in ['build', 'fmt', 'vet', 'lint']}, 'quality matrix')
        for c in checks:
            stem = c['variant'] + '-' + c['kind']
            require(c['exit_code'] == 0 and not (packet / 'logs' / (stem + '.stderr.txt')).read_bytes(), 'quality check failure')
            stdout = (packet / 'logs' / (stem + '.stdout.txt')).read_text().strip()
            require(stdout in ('', '0 issues.') if c['kind'] == 'lint' else not stdout, 'quality findings')
        cases = {'one_far', 'two_far', 'far_then_near', 'two_near', 'near_then_far'}
        rows = summary['observations']
        require(len(rows) == 20 and {(r['variant'], r['mode'], r['case']) for r in rows} ==
                {(v, m, c) for v in variants for m in ['normal', 'race'] for c in cases}, 'complete matrix')
        runs = summary['runs']
        require(len(runs) == 4 and {(r['variant'], r['mode']) for r in runs} ==
                {(v, m) for v in variants for m in ['normal', 'race']}, 'run matrix')
        for run in runs:
            variant, mode = run['variant'], run['mode']
            command = ['test', '-json', '-count=1', '-timeout=10s'] + (['-race'] if mode == 'race' else []) + ['./...']
            require(run['command'] == command and run['exit_code'] == 0 and run['observations'] == 5, 'run command/result')
            stem = variant + '-' + mode
            events = [json.loads(line) for line in (packet / 'logs' / (stem + '.jsonl')).read_text().splitlines() if line]
            text = ''.join(e.get('Output', '') for e in events)
            require(not (packet / 'logs' / (stem + '.stderr.txt')).read_bytes()
                    and not any(e.get('Action') in ['fail', 'skip'] for e in events) and 'WARNING: DATA RACE' not in text, 'failed/skipped/racing run')
            require({e.get('Test') for e in events if e.get('Action') == 'pass' and e.get('Test')} ==
                    {'TestSyntheticCursor'} | {'TestSyntheticCursor/' + c for c in cases}
                    and any(e.get('Action') == 'pass' and not e.get('Test') for e in events), 'raw passing tests')
            observed = [dict(json.loads(line.split('CURSOR_OBSERVATION ', 1)[1]), mode=mode)
                        for line in text.splitlines() if 'CURSOR_OBSERVATION ' in line]
            require(observed == [r for r in rows if r['variant'] == variant and r['mode'] == mode], 'summary/raw mismatch')
        # These are AI example expectations, not independent quality gold.
        for row in rows:
            variant, case = row['variant'], row['case']
            if case == 'one_far':
                expected = (0, 1, 0, 1, False)
            elif case == 'two_far':
                expected = (1, 1, 0, 1, False) if variant == 'ORIGINAL' else (0, 2, 0, 1, False)
            elif case == 'far_then_near':
                expected = (1, 1, 0, 1, False) if variant == 'ORIGINAL' else (1, 1, 1, 0, True)
            else:
                expected = (2, 0, 1, 0, True)
            require(tuple(row[k] for k in ['remaining_elements', 'clockwarp_notifications', 'timer_reset_requests',
                                          'timer_stop_requests', 'next_timeout_recorded']) == expected, 'AI example observation mismatch')
        if check_references:
            for name in ['PROTOCOL', 'PROGRESS']:
                d = json.loads((ROOT / f'docs/spec-driven/OPERATIONAL-ACCEPTANCE-{name}.json').read_text())
                require(d.get('udp_cursor_local_probe') == {'manifest': (packet / 'manifest.json').relative_to(ROOT).as_posix(),
                        'sha256': sha((packet / 'manifest.json').read_bytes()),
                        'status': 'synthetic_source_branch_only_not_operational_reachability'}, 'ledger reference: ' + name)
    except (OSError, ValueError, KeyError, TypeError, IndexError) as exc:
        errors.append('invalid UDP cursor packet: ' + str(exc))
    return errors


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('packet', type=Path)
    args = parser.parse_args()
    errors = check(args.packet.resolve())
    if errors:
        print('\n'.join(errors))
        raise SystemExit(1)
    print('PASS: 20 synthetic cursor observations; 8 quality checks; real UDP/clock/timer firing not tested')
