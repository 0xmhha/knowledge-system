#!/usr/bin/env python3
"""Check all 28 roadmap rows and the KM-01 library evidence without public acceptance claims."""
import gzip
import hashlib
import json
from pathlib import Path
import re

ROOT = Path(__file__).resolve().parents[1]
PACKET = ROOT / 'system/eval/knowledge-roadmap-2026-10-15/km-01-foundation'


def require(value, message):
    if not value:
        raise ValueError(message)


def test_inventory(raw):
    rows = [json.loads(line) for line in raw.splitlines() if line]
    running, completed, failures, skipped = set(), set(), [], []
    packages, endings = set(), {}
    for row in rows:
        package, test, action = row.get('Package'), row.get('Test'), row.get('Action')
        if action == 'start':
            packages.add(package)
        if test:
            key = (package, test)
            if action == 'run':
                require(key not in running, 'duplicate test execution')
                running.add(key)
            if action in ('pass', 'fail', 'skip'):
                require(key not in completed, 'duplicate test completion')
                completed.add(key)
                if action == 'fail':
                    failures.append(key)
                if action == 'skip':
                    skipped.append(key)
        elif action in ('pass', 'fail', 'skip'):
            require(package not in endings, 'duplicate package completion')
            endings[package] = action
        if action == 'build-fail':
            failures.append((row.get('ImportPath'), 'build'))
    require(running == completed and packages == set(endings), 'unfinished tests/packages')
    return {'test_cases': len(running), 'passed_tests': len(completed) - len(failures) - len(skipped),
            'failed_tests': len(failures), 'skipped_tests': len(skipped),
            'packages': len(packages), 'failed_packages': sum(v == 'fail' for v in endings.values()),
            'skipped_packages': sum(v == 'skip' for v in endings.values()),
            'skipped': [{'package': p, 'test': t} for p, t in sorted(skipped)]}, running


def test_rows(raw):
    rows = []
    for line in raw.splitlines():
        row = json.loads(line)
        if row.get('Test') and row.get('Action') in ('pass', 'fail', 'skip'):
            rows.append({'package': row['Package'], 'test': row['Test'], 'state': row['Action']})
    return sorted(rows, key=lambda r: (r['package'], r['test']))


def check(folder=PACKET, root=ROOT):
    errors = []
    try:
        progress = json.loads((root / 'docs/spec-driven/KNOWLEDGE-ROADMAP-PROGRESS.json').read_text())
        states = {s['id']: s for item in progress['items'] for s in item['subtasks']}
        expected = {f'KM-{i:02}.{j}' for i in range(1, 7) for j in range(1, 5)} | {f'OE-01.{j}' for j in range(1, 5)}
        require(set(states) == expected and len(states) == progress['total_subtasks'] == 28, 'roadmap inventory differs')
        completed = sum(s['status'] == 'complete' for s in states.values())
        require(completed == progress['completed_subtasks'], 'completed subtask count differs')
        feature_done = sum(i['status'] == 'complete' for i in progress['items'])
        require(feature_done == progress['implementation_completed'] and len(progress['items']) == progress['total_work_items'] == 7,
                'completed feature count differs')
        spec = (root / progress['spec']).read_text()
        boxes = dict((qid, mark) for mark, qid in re.findall(r'^- \[([ x])\] ((?:KM|OE)-\d\d\.\d):', spec, re.M))
        require(set(boxes) == expected, 'spec checkbox inventory differs')
        matrix = (root / 'docs/spec-driven/KNOWLEDGE-ROADMAP-TEST-MATRIX.md').read_text()
        rows = {parts[1].strip(): parts for line in matrix.splitlines() if line.startswith('| KM-') or line.startswith('| OE-')
                if len(parts := line.split('|')) >= 6}
        labels = {'complete': '완료', 'in_progress': '진행 중', 'not_started': '미착수', 'blocked': '차단'}
        require(set(rows) == expected, 'test matrix inventory differs')
        for qid, state in states.items():
            require(state['status'] in labels and (boxes[qid] == 'x') == (state['status'] == 'complete')
                    and rows[qid][3].strip() == labels[state['status']], 'checkbox/matrix/ledger status mismatch: ' + qid)
            if state['status'] == 'complete':
                require(bool(state['evidence']) and all((root / p).is_file() for p in state['evidence']), 'completed task lacks evidence')
        for item in progress['items']:
            if item['status'] == 'complete':
                require(all(s['status'] == 'complete' for s in item['subtasks']) and item['verification'] != 'not_run',
                        'partial feature promoted to complete')
        require(f'전체 완료는{feature_done}/7' in matrix, 'matrix total differs')
        folder = folder.resolve()
        manifest = json.loads((folder / 'manifest.json').read_text())
        require(manifest['scope'] == 'KM01_library_fixture_verification_not_public_adapters'
                and manifest['public_MCP_connected'] is False and manifest['versioned_DB_write_verified'] is False
                and manifest['full_KM01_complete'] is False and manifest['independent_acceptance'] is False,
                'library verification promoted to public/full acceptance')
        names = [r['path'] for r in manifest['artifacts']]
        actual = {p.relative_to(folder).as_posix() for p in folder.rglob('*') if p.is_file() and p.name != 'manifest.json'}
        require(len(names) == len(set(names)) and set(names) == actual, 'artifact inventory differs')
        for row in manifest['artifacts']:
            p = (folder / row['path']).resolve()
            require(p.is_relative_to(folder), 'unsafe evidence path')
            raw = p.read_bytes()
            require(hashlib.sha256(raw).hexdigest() == row['sha256'] and len(raw) == row['bytes'], 'evidence hash differs')
            if 'uncompressed_sha256' in row:
                require(hashlib.sha256(gzip.decompress(raw)).hexdigest() == row['uncompressed_sha256'], 'raw log bytes differ')
        for name, digest in manifest['source_sha256'].items():
            p = (root / name).resolve()
            require(p.is_relative_to(root.resolve()) and hashlib.sha256(p.read_bytes()).hexdigest() == digest, 'source binding differs')
        validation = json.loads((folder / 'validation.json').read_text())
        collection, cases = test_inventory(gzip.decompress((folder / 'collection-race.jsonl.gz').read_bytes()))
        require(collection == validation['collection'] and collection['test_cases'] == collection['passed_tests'] == 40
                and collection['failed_tests'] == collection['failed_packages'] == collection['skipped_tests'] == 0,
                'collection test denominator/result differs')
        code = (root / 'internal/system/collection/collection_test.go').read_text()
        test_names = set(re.findall(r'^func (Test\w+)\(', code, re.M))
        top = {name for _, name in cases if '/' not in name}
        require(top == test_names and len(top) == 17 and all('| ' + name + ' |' in matrix for name in top),
                'top-level test inventory missing from code/log/matrix')
        module_raw = gzip.decompress((folder / 'module-tests.jsonl.gz').read_bytes())
        module, _ = test_inventory(module_raw)
        require(module == validation['module'] and module['failed_tests'] == module['failed_packages'] == 0,
                'whole-module failures/skips hidden')
        inventory = json.loads((folder / 'test-inventory.json').read_text())
        require(inventory['module'] == test_rows(module_raw)
                and inventory['collection'] == test_rows(gzip.decompress((folder / 'collection-race.jsonl.gz').read_bytes())),
                'full test list differs from raw completions')
        require(all(validation['gates'][name] == 0 for name in ('build', 'module_test', 'fmt', 'lint', 'docs', 'old_source_binding')),
                'gate did not pass')
    except (OSError, ValueError, KeyError, TypeError, IndexError) as exc:
        errors.append(str(exc))
    return errors


if __name__ == '__main__':
    errors = check()
    for error in errors:
        print('FAIL:', error)
    print('knowledge roadmap:', 'FAIL' if errors else 'PASS; all 28 rows reconciled; library scope only')
    raise SystemExit(bool(errors))
