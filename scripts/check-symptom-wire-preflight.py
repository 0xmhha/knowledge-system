#!/usr/bin/env python3
"""Verify public-only request packaging and real parser observations, without recall claims."""
import argparse
import importlib.util
import json
from pathlib import Path
import subprocess

from source_packet import safe, sha

ROOT = Path(__file__).resolve().parents[1]
BASE = Path('docs/spec-driven/operational-preparation/source-review-supplement')
CONTROLS = {'minimal_defaults': ('parser_defaults', True), 'unknown_gold_field': ('parser_defaults', False),
            'fractional_candidates': ('parser_defaults', False), 'insufficient_call_budget': ('parser_defaults', False),
            'site_without_line': ('parser_defaults', False), 'public_seed_hint': ('public_input_policy', False)}


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
        require(manifest['kind'] == 'symptom_wire_parser_preparation_only', 'packet kind')
        inputs = BASE / 'symptom-inputs-v1'
        require(manifest['input_manifest'] == str(inputs / 'manifest.json')
                and manifest['input_manifest_sha256'] == sha((ROOT / inputs / 'manifest.json').read_bytes()), 'input packet binding')
        errors.extend(load('check-symptom-inputs.py').check(ROOT / inputs))
        inventory = {p.relative_to(packet).as_posix() for p in packet.rglob('*') if p.is_file() and p.name != 'manifest.json'}
        require(inventory == set(manifest['artifacts']), 'artifact inventory')
        for name, digest in manifest['artifacts'].items():
            require(sha(safe(packet, name).read_bytes()) == digest, 'artifact hash: ' + name)
        for name, digest in manifest['source_sha256'].items():
            require(sha(safe(ROOT, name).read_bytes()) == digest, 'actual parser/source binding: ' + name)
        expected_sources = {'go.mod', 'go.sum', '.golangci.yml', 'internal/system/mcp/suspects.go',
                            'internal/system/mcp/server_test.go', 'internal/system/mcp/suspects_fixture_test.go',
                            'internal/system/suspects/compose.go', 'scripts/fixtures/symptom_wire_preflight_test.go.txt',
                            'scripts/export-symptom-wire.py'}
        require(set(manifest['source_sha256']) == expected_sources, 'parser/source binding inventory')
        require((packet / 'runner.py.txt').read_bytes() == (ROOT / 'scripts/run-symptom-wire-preflight.py').read_bytes()
                and (packet / 'exporter.py.txt').read_bytes() == (ROOT / 'scripts/export-symptom-wire.py').read_bytes(), 'preflight implementation binding')
        public_raw = (ROOT / inputs / 'public-inputs.json').read_bytes()
        require(sha(public_raw) == manifest['public_input_sha256'], 'public input binding')
        expected_wire = load('export-symptom-wire.py').render(json.loads(public_raw))
        wire = json.loads((packet / 'public-bundle/wire-requests.json').read_text())
        require(wire == expected_wire, 'public request payload/seed mismatch')
        require({p.relative_to(packet / 'public-bundle').as_posix() for p in (packet / 'public-bundle').rglob('*') if p.is_file()} == {'wire-requests.json'}, 'private data in public bundle')
        summary = json.loads((packet / 'summary.json').read_text())
        require(summary['kind'] == 'actual_request_parser_preflight_not_search_or_host_evaluation', 'preflight scope')
        require(summary['go_version'].startswith('go version go1.26.9 ') and bool(summary['lint_version']), 'tool identity')
        require(summary['maintained_go_worktree_unchanged'] is True and summary['existing_test_file_byte_unchanged'] is True
                and summary['physical_test_file_created_in_repo'] is False, 'Go source modification claim')
        require(summary['backend_calls'] == summary['retrieval_runs'] == summary['model_calls'] == summary['agent_role_runs'] == summary['formal_runs'] == summary['independent_gold_records'] == 0
                and summary['macro_recall_at_10'] is None, 'search/gold/host promotion')
        require(summary['public_bundle_scope'] == 'request packaging separation; not an OS filesystem access boundary', 'input access overclaim')
        require(not subprocess.check_output(['git', '-C', str(ROOT), 'diff', summary['go_source_reference_commit'], '--', '*.go']), 'Go sources differ from tested revision')
        template = (ROOT / 'scripts/fixtures/symptom_wire_preflight_test.go.txt').read_bytes()
        base = (ROOT / 'internal/system/mcp/suspects_fixture_test.go').read_bytes()
        backing = load('run-symptom-wire-preflight.py').overlay_source(base, template)
        require((packet / 'template.go.txt').read_bytes() == template and (packet / 'overlay-base.go.txt').read_bytes() == base
                and (packet / 'overlay-backing.go.txt').read_bytes() == backing, 'overlay changes existing tests or preparation code')
        overlay = json.loads((packet / 'overlay.json.txt').read_text())
        require(len(overlay['Replace']) == 1 and set(overlay['Replace']) == {str(ROOT / 'internal/system/mcp/suspects_fixture_test.go')}
                and summary['virtual_test_file'] == str(ROOT / 'internal/system/mcp/suspects_fixture_test.go'), 'overlay scope')
        require(manifest['counts'] == {'public_requests': 20, 'modes': 2, 'request_observations': 40, 'controls': 12, 'quality_checks': 4}, 'count inflation')
        quality = summary['quality_checks']
        require(len(quality) == 4 and {q['kind'] for q in quality} == {'build', 'fmt', 'vet', 'lint'}, 'quality inventory')
        for q in quality:
            require(q['exit_code'] == 0 and q['go_overlay_enabled'] == (q['kind'] != 'fmt'), 'quality scope/result')
            require(not (packet / 'logs' / (q['kind'] + '.stderr.txt')).read_bytes(), 'quality stderr')
            stdout = (packet / 'logs' / (q['kind'] + '.stdout.txt')).read_text().strip()
            require(stdout in ('', '0 issues.') if q['kind'] == 'lint' else not stdout, 'quality findings')
        observations, controls = summary['observations'], summary['controls']
        cards = {r['case_id']: r for r in wire['requests']}
        require(len(observations) == 40 and {(r['case_id'], r['mode']) for r in observations} ==
                {(c, m) for c in cards for m in ['normal', 'race']}, 'request observation matrix')
        require(len(controls) == 12 and {(r['control_id'], r['mode']) for r in controls} ==
                {(c, m) for c in CONTROLS for m in ['normal', 'race']}, 'control matrix')
        for row in observations:
            expected = {'case_id': row['case_id'], 'name': 'cks.context.label_suspects',
                        'symptom_bytes': len(cards[row['case_id']]['arguments']['symptom'].encode()),
                        'commit': wire['source_commit'], 'seed_mode': 'text_only',
                        'parser_accepted': True, 'defaults_accepted': True, 'backend_calls': 0, 'mode': row['mode']}
            require(row == expected, 'public parser observation claim')
        for row in controls:
            stage, valid = CONTROLS[row['control_id']]
            require(row == {'control_id': row['control_id'], 'stage': stage, 'expected_valid': valid,
                            'observed_valid': valid, 'backend_calls': 0, 'mode': row['mode']}, 'control validity claim')
        require(len(summary['runs']) == 2 and {r['mode'] for r in summary['runs']} == {'normal', 'race'}, 'run inventory')
        for run in summary['runs']:
            mode = run['mode']
            require(run['exit_code'] == 0 and run['public_requests'] == 20 and run['controls'] == 6
                    and ('-race' in run['command']) == (mode == 'race') and '-run=^TestSymptomWirePreflight' in run['command'], 'run scope/command')
            events = [json.loads(line) for line in (packet / 'logs' / (mode + '.jsonl')).read_text().splitlines() if line]
            text = ''.join(e.get('Output', '') for e in events)
            require(not (packet / 'logs' / (mode + '.stderr.txt')).read_bytes()
                    and not any(e.get('Action') in ['fail', 'skip'] for e in events) and 'WARNING: DATA RACE' not in text, 'failed/skipped/racing preflight')
            expected_passes = {'TestSymptomWirePreflight', 'TestSymptomWirePreflightControls'} | {'TestSymptomWirePreflight/' + c for c in cards} | {'TestSymptomWirePreflightControls/' + c for c in CONTROLS}
            require({e.get('Test') for e in events if e.get('Action') == 'pass' and e.get('Test')} == expected_passes
                    and any(e.get('Action') == 'pass' and not e.get('Test') for e in events), 'raw passed test set')
            got_rows, got_controls = [], []
            for line in text.splitlines():
                if 'WIRE_OBSERVATION ' in line:
                    got_rows.append(dict(json.loads(line.split('WIRE_OBSERVATION ', 1)[1]), mode=mode))
                if 'WIRE_CONTROL ' in line:
                    got_controls.append(dict(json.loads(line.split('WIRE_CONTROL ', 1)[1]), mode=mode))
            require(got_rows == [r for r in observations if r['mode'] == mode]
                    and got_controls == [r for r in controls if r['mode'] == mode], 'summary/raw mismatch')
        diagnostics = json.loads((packet / 'diagnostics.json').read_text())
        require(len(diagnostics) <= 1, 'diagnostic count')
        if diagnostics:
            require(diagnostics[0]['kind'] == 'new_virtual_file_unreadable_by_lint' and diagnostics[0]['backend_calls'] == 0
                    and diagnostics[0]['go_files_changed_in_repo'] is False, 'prior failure scope')
            require('no such file or directory' in (packet / 'prior-overlay-diagnostic/lint.stdout.txt').read_text(), 'missing prior lint failure')
        if check_references:
            for name in ['PROTOCOL', 'PROGRESS']:
                d = json.loads((ROOT / f'docs/spec-driven/OPERATIONAL-ACCEPTANCE-{name}.json').read_text())
                require(d.get('symptom_wire_preflight') == {'manifest': (packet / 'manifest.json').relative_to(ROOT).as_posix(),
                        'sha256': sha((packet / 'manifest.json').read_bytes()), 'status': 'parser_preflight_not_retrieval_or_independent_acceptance'}, 'ledger reference: ' + name)
    except (OSError, ValueError, KeyError, TypeError, IndexError, subprocess.CalledProcessError) as exc:
        errors.append('invalid wire preflight: ' + str(exc))
    return errors


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('packet', type=Path)
    args = parser.parse_args()
    errors = check(args.packet.resolve())
    if errors:
        print('\n'.join(errors))
        raise SystemExit(1)
    print('PASS: 40 real parser input observations, 12 controls, public-only requests; search/host/recall not evaluated')
