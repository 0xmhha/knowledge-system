from collections import Counter
import hashlib
import importlib.util
import json
from pathlib import Path

root = Path.cwd()
base = root / 'system/eval/b0-knowledge-system/backend-calls-m2max-2026-10-04'
spec = importlib.util.spec_from_file_location('call_smoke', root / 'scripts/wbs-ontology-relations-smoke.py')
harness = importlib.util.module_from_spec(spec); spec.loader.exec_module(harness)
responses = 0
measurements = 0
ids = set()
error_counts = {}
for kind in ['mock', 'real', 'compat']:
    output = base / kind
    summary = json.loads((output / 'summary.json').read_text())
    errors = Counter()
    for name, sha in summary['input_file_sha256'].items():
        assert hashlib.sha256((output / 'source' / name).read_bytes()).hexdigest() == sha
    for path, sha in summary['locked_files_sha256'].items():
        assert hashlib.sha256(Path(path).read_bytes()).hexdigest() == sha
    matrix = json.loads((output / 'matrix-manifest.json').read_text())
    for arm in matrix['arms']:
        path = output / arm['capture']
        assert hashlib.sha256(path.read_bytes()).hexdigest() == arm['capture_sha256']
        report = json.loads(path.read_text())
        assert report['state'] == 'captured'
        assert report['binary_sha256'] == report['binary_sha256_after'] == summary['binary_sha256']
        events = [json.loads(line) for line in (output / 'telemetry' / arm['mode'] / 'cks-mcp.jsonl').read_text().splitlines()]
        for row in report['rows']:
            identifier = row['call']['measurement_id']
            assert identifier not in ids; ids.add(identifier)
            found = [e for e in events if e.get('event') == 'measurement.backend_calls' and e['summary']['measurement_id'] == identifier]
            assert len(found) == 1 and found[0]['trace_id'] == identifier
            s = found[0]['summary']
            assert s['tool'] == row['call']['tool'] and s['outcome'] == 'returned' and s['pending_calls'] == 0
            assert [c['ordinal'] for c in s['calls']] == list(range(1, len(s['calls']) + 1))
            assert all(c['outcome'] in {'returned', 'backend_error'} for c in s['calls'])
            errors.update(c['backend'] + '.' + c['method'] for c in s['calls'] if c['outcome'] == 'backend_error')
            assert all(c['backend'] == 'ckg' for c in s['calls'] if c['outcome'] == 'backend_error')
            logical = [c for c in s['calls'] if c['backend'] != 'ollama_http']
            http = [c for c in s['calls'] if c['backend'] == 'ollama_http']
            assert any(c['method'] == 'neighbors' for c in logical)
            assert sum(c['method'] == 'semantic_search' and c['options']['K'] == 6 for c in logical) == 1
            if kind == 'real':
                assert http and all(c['outcome'] == 'returned' and c['http_status'] == 200 for c in http)
                assert any(c['method'] == 'POST' for c in http)
            else:
                assert not http
            if kind != 'compat':
                recorded = next(o['backend_measurement'] for o in arm['observations'] if o['request_id'] == row['request_id'])
                assert recorded == s
            measurements += 1
            response = row['call']['response']
            assert not response.get('isError')
            if kind == 'compat' and row['request_id'] == 'v1':
                continue
            pack = response['structuredContent']
            copy = json.loads(json.dumps(pack)); expected = copy['metadata'].pop('integrity_hash')
            assert hashlib.sha256(json.dumps(copy, ensure_ascii=False, sort_keys=True, separators=(',', ':')).encode()).hexdigest() == expected
            harness.verify_v2_sources(pack, output / 'source', summary['coordinates'])
            if kind != 'compat':
                harness.verify_knowledge(pack, arm['include_knowledge'], row['request_id'])
            responses += 1
    expected = {'mock': {'ckg.neighbors': 48}, 'real': {'ckg.bm25_search': 96, 'ckg.neighbors': 384}, 'compat': {'ckg.neighbors': 20}}[kind]
    assert dict(errors) == expected, (kind, errors)
    error_counts[kind] = dict(errors)
    print(kind, 'raw footprint IDs/options and original source/DB bytes verified; retained nonfatal errors:', dict(errors))
for arm in ['baseline_off', 'combined_on']:
    measured = json.loads((base / 'real' / (arm + '.json')).read_text())
    plain = json.loads((base / 'equivalence' / (arm + '.json')).read_text())
    assert len(measured['rows']) == len(plain['rows']) == 6
    for a, b in zip(measured['rows'], plain['rows']):
        assert a['request_id'] == b['request_id'] and a['call']['response'] == b['call']['response']
print('independent v2 source/integrity audit:', responses)
print('correlated request measurements:', measurements)
print('equivalence: 12 full SDK responses')
