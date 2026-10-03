import hashlib
import importlib.util
import json
from pathlib import Path

root = Path.cwd()
folder = root / 'system/eval/b0-knowledge-system/b1-pack-matrix-m2max-2026-10-04'
spec = importlib.util.spec_from_file_location('capture_harness', folder / 'harness-at-final-capture.py')
harness = importlib.util.module_from_spec(spec)
spec.loader.exec_module(harness)
verified = 0
for kind in ['mock', 'real', 'compat']:
    output = folder / kind
    summary = json.loads((output / 'summary.json').read_text())
    assert summary['state'] == 'verified' and summary['quality_metrics'] is None
    for name, sha in summary['input_file_sha256'].items():
        assert hashlib.sha256((output / 'source' / name).read_bytes()).hexdigest() == sha
    for path, sha in summary['locked_files_sha256'].items():
        assert hashlib.sha256(Path(path).read_bytes()).hexdigest() == sha
    matrix = json.loads((output / 'matrix-manifest.json').read_text())
    for arm in matrix['arms']:
        capture = output / arm['capture']
        assert hashlib.sha256(capture.read_bytes()).hexdigest() == arm['capture_sha256']
        report = json.loads(capture.read_text())
        assert report['state'] == 'captured'
        assert report['binary_sha256'] == report['binary_sha256_after'] == summary['binary_sha256']
        assert report['config_sha256'] == report['config_sha256_after'] == arm['config_sha256']
        assert report['request_sha256'] == arm['requests_sha256']
        for row in report['rows']:
            response = row['call']['response']
            assert not response.get('isError')
            if kind == 'compat' and row['request_id'] != 'v2':
                continue
            pack = response['structuredContent']
            clone = json.loads(json.dumps(pack))
            expected = clone['metadata'].pop('integrity_hash')
            canonical = json.dumps(clone, ensure_ascii=False, sort_keys=True, separators=(',', ':')).encode()
            assert hashlib.sha256(canonical).hexdigest() == expected
            harness.verify_v2_sources(pack, output / 'source', summary['coordinates'])
            if kind != 'compat':
                harness.verify_knowledge(pack, arm['include_knowledge'], row['request_id'])
            verified += 1
    print(kind, 'captured responses, source SHA and original DB SHA verified')

pack = json.loads((folder / 'mock/baseline_on.json').read_text())['rows'][1]['call']['response']['structuredContent']
pack['semantic']['coding_context']['required_behavior'] = [{'id': 'FORGED'}]
try:
    harness.verify_knowledge(pack, True, 'wrong-scope')
except AssertionError:
    print('forged normative instruction rejected')
else:
    raise AssertionError('forged normative instruction accepted')
print('v2 responses independently rechecked:', verified)
