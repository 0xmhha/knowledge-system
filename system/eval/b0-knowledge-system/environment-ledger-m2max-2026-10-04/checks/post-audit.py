import hashlib
import importlib.util
import json
from collections import Counter
from pathlib import Path

ROOT = Path.cwd()
OUT = ROOT / 'system/eval/b0-knowledge-system/environment-ledger-m2max-2026-10-04'
spec = importlib.util.spec_from_file_location('fixture_audit', ROOT / 'scripts/wbs-ontology-relations-smoke.py')
helper = importlib.util.module_from_spec(spec)
spec.loader.exec_module(helper)
digest = lambda path: hashlib.sha256(path.read_bytes()).hexdigest()

for kind, prior, k in [('real', 'recall-k-m2max-2026-10-04', 10), ('mock', 'backend-calls-m2max-2026-10-04', 20)]:
    directory = OUT / kind
    old = ROOT / 'system/eval/b0-knowledge-system' / prior / kind
    report = json.loads((directory / 'report.json').read_text())
    rows = [json.loads(line) for line in (directory / 'rows.jsonl').read_text().splitlines()]
    assert report['state'] == 'captured' and report['rows'] == len(rows) == 24
    assert report['quality_metrics'] is None
    assert report['locked_files_sha256'] == report['locked_files_sha256_after']
    assert report['binary_sha256'] == report['binary_sha256_after']
    assert report['source_head'] == report['source_head_after']
    assert report['live_executable_bits'] == report['live_executable_bits_after']
    assert digest(directory / 'rows.jsonl') == report['rows_sha256']
    # This check uses live native files during this recorded audit. Rebuilding
    # binaries later requires the recorded hash/result, not a retroactive claim.
    for path, expected in report['locked_files_sha256'].items():
        assert (not Path(path).exists()) if expected == 'missing' else digest(Path(path)) == expected, path
    assert len(report['locked_input_copies']) == 3
    for source, copy in report['locked_input_copies'].items():
        assert digest(directory / copy['path']) == copy['sha256'] == copy['sha256_after'] == report['locked_files_sha256'][source]
        # Git records only the executable bit. Check private permissions on
        # original runtime output, not on a future checkout of the archive.
        runtime_output = Path(report['arms'][0]['config']).parent
        assert (runtime_output / copy['path']).stat().st_mode & 0o777 == 0o600
    protocol_copy = next(copy for source, copy in report['locked_input_copies'].items() if source.endswith('/protocol-m2max-draft.json'))
    assert json.loads((directory / protocol_copy['path']).read_text())['status'] == 'draft'
    before, after = report['environment_before'], report['environment_after']
    assert before['valid'] and after['valid']
    for snapshot in (before, after):
        hardware, model = snapshot['hardware'], snapshot['model']
        assert hardware['cpu_brand'] == 'Apple M2 Max' and hardware['memory_bytes'] == 68719476736 and hardware['logical_cpus'] == 12
        assert hardware['load'] and hardware['swap'] and hardware['process_count'] > 0
        assert all(set(process) == {'pid', 'parent_pid', 'cpu_percent', 'rss_bytes'} for process in hardware['top_cpu_processes'])
        assert model['valid'] and model['dimension'] == (1024 if kind == 'real' else 64)
        if kind == 'real':
            assert model['digest'] == report['dataset_identity']['embedding_identity']['model_digest']
            assert model['server_version'] == '0.35.1' and model['runtime_options'] == {'num_ctx': 8192, 'num_batch': 8192}
            assert model['residency']['selected_models'][0]['digest'] == model['digest']
    assert before['model'] == after['model'] if kind == 'mock' else all(before['model'][key] == after['model'][key] for key in ('model', 'digest', 'dimension', 'server_version', 'runtime_options'))
    old_summary = json.loads((old / 'summary.json').read_text())
    responses, events = {}, {}
    for arm in report['arms']:
        assert digest(directory / Path(arm['config']).name) == arm['config_sha256'] == arm['config_sha256_after']
        captured = json.loads((old / (arm['id'] + '.json')).read_text())
        responses[arm['id']] = {row['request_id']: row['call']['response'] for row in captured['rows']}
        events[arm['id']] = [json.loads(line) for line in (directory / 'footprints' / arm['id'] / 'cks-mcp.jsonl').read_text().splitlines()]
    seen, phases, errors = set(), Counter(), Counter()
    for row in rows:
        index = row['sequence'] - 1
        assert row['group'] == index // 8 and row['position'] == index % 8 + 1
        assert row['arm'] == report['arms'][(row['group'] + row['position'] - 1) % 8]['id']
        assert not row.get('error') and row['request_id'] == 'alpha'
        assert row['call']['response'] == responses[row['arm']]['alpha']
        assert set(row['call']['arguments']) <= {'prompt', 'knowledge_as_of', 'knowledge_subsystem', 'include_knowledge', '_meta'}
        assert row['call']['arguments']['prompt'] == 'Alpha function implementation'
        pack = row['call']['response']['structuredContent']
        clone = json.loads(json.dumps(pack))
        expected = clone['metadata'].pop('integrity_hash')
        assert hashlib.sha256(json.dumps(clone, sort_keys=True, ensure_ascii=False, separators=(',', ':')).encode()).hexdigest() == expected
        helper.verify_v2_sources(pack, old / 'source', old_summary['coordinates'])
        helper.verify_knowledge(pack, row['arm'].endswith('_on'), 'alpha')
        ident = row['call']['measurement_id']
        assert ident not in seen
        seen.add(ident)
        found = [event for event in events[row['arm']] if event.get('event') == 'measurement.backend_calls' and event['summary']['measurement_id'] == ident]
        assert len(found) == 1 and found[0]['trace_id'] == ident
        calls = found[0]['summary']['calls']
        assert found[0]['summary']['pending_calls'] == 0
        searches = [call for call in calls if call['method'] == 'semantic_search']
        assert all(call['options']['K'] == k for call in searches if not call['options']['Filter']['ChunkKinds'])
        knowledge = [call for call in searches if call['options']['Filter']['ChunkKinds']]
        assert len(knowledge) == 1 and knowledge[0]['options']['K'] == 6
        assert all(call['outcome'] == 'returned' for call in calls if call['method'] == 'bm25_search')
        http = [call for call in calls if call['backend'] == 'ollama_http']
        assert bool(http) == (kind == 'real')
        assert all(call['outcome'] == 'returned' and call['http_status'] == 200 for call in http)
        errors.update(call['backend'] + '.' + call['method'] for call in calls if call['outcome'] != 'returned')
        assert (row.get('process_start_through_response_ns', 0) > 0) == (row['phase'] == 'cold_process')
        phases[row['phase']] += 1
    assert phases == {'retrieval': 8, 'warm_latency': 8, 'cold_process': 8}
    startup = [event for arm_events in events.values() for event in arm_events if event.get('event') == 'measurement.backend_calls' and event['summary']['tool'] == 'startup.intent_anchors']
    assert len(startup) == 16
    print(kind, ': 24 unchanged SDK responses; source/integrity/policy, rotation, model/host/input copies and actual K/measurement links verified; startup scopes 16; internal errors', dict(errors))

for name, reason in [('initial-mock-pin-failure', 'configured_provider_differs_from_dataset'), ('wrong-model', 'configured_model_or_dataset_digest_invalid')]:
    directory = OUT / name
    report = json.loads((directory / 'report.json').read_text())
    assert report['state'] == 'partial' and report['rows'] == 0
    assert reason in report['environment_before']['model']['errors']
    assert not (directory / 'rows.jsonl').exists() and not (directory / 'footprints').exists()
    assert report['environment_after'] and report['quality_metrics'] is None
    print(name, ': partial preflight, zero query rows/startup footprints; preserved failure and after snapshot')
