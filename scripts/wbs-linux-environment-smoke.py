#!/usr/bin/env python3
"""Actual minimal-Linux/mock matrix diagnostic; no official quality verdict."""
import argparse
import copy
import hashlib
import json
import os
from pathlib import Path
import platform
import shutil
import subprocess


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def load(path):
    return json.loads(path.read_text())


def verify(base, matrix):
    report = load(matrix / 'report.json')
    rows = [json.loads(line) for line in (matrix / 'rows.jsonl').read_text().splitlines()]
    assert report['state'] == 'captured' and report['rows'] == len(rows) == 24
    assert report['quality_metrics'] is None
    assert report['binary_sha256'] == report['binary_sha256_after']
    assert report['locked_files_sha256'] == report['locked_files_sha256_after']
    assert report['source_head'] == report['source_head_after']
    assert report['live_executable_bits'] == report['live_executable_bits_after']
    assert report['source_head'] != report['dataset_identity']['source']['source_commit'], 'rollback must retain old evidence'
    for side in ('before', 'after'):
        env = report['environment_' + side]
        assert env['valid'] and env['model']['valid']
        assert env['model']['provider'] == 'mock' and env['model']['dimension'] == 64
        hw = env['hardware']
        assert hw['os'] == 'linux' and hw['cpu_brand'] and hw['process_count'] > 0
        assert hw['process_pressure_method'].startswith('Linux /proc/')
        assert hw['resource_limits']['v2.cpu.max'] == '100000 100000'
        assert hw['resource_limits']['v2.memory.max'] == '536870912'
        assert all(set(p) == {'pid', 'parent_pid', 'cpu_percent', 'rss_bytes'} for p in hw['top_cpu_processes'])
    version = (base / 'dataset/current').resolve()
    manifest = load(version / 'sources/manifest.json')
    source = {entry['path']: (version / 'sources/blobs' / entry['sha256']).read_bytes()
              for entry in manifest['files'] if entry['kind'] == 'regular'}
    identity = report['dataset_identity']
    comparisons, ids, citations = {}, set(), 0
    scopes, startups, backend_errors = {}, 0, []
    for file in (matrix / 'footprints').rglob('*.jsonl'):
        for line in file.read_text().splitlines():
            event = json.loads(line)
            if event.get('event') != 'measurement.backend_calls':
                continue
            summary = event['summary']
            if summary['tool'] == 'startup.intent_anchors':
                startups += 1
                continue
            assert summary['measurement_id'] not in scopes
            scopes[summary['measurement_id']] = summary
    for original, copied in report['locked_input_copies'].items():
        raw = (matrix / copied['path']).read_bytes()
        assert sha(raw) == copied['sha256'] == copied['sha256_after'] == report['locked_files_sha256'][original]
        assert (matrix / copied['path']).stat().st_mode & 0o777 == 0o600
    for arm in report['arms']:
        assert arm['config_sha256'] == arm['config_sha256_after']
    arm_ids = [arm['id'] for arm in report['arms']]
    for row in rows:
        assert row['arm'] == arm_ids[(row['group'] + row['position'] - 1) % 8]
        call = row['call']
        assert not call.get('error') and call['elapsed_ns'] > 0
        assert call['measurement_id'] not in ids
        ids.add(call['measurement_id'])
        summary = scopes[call['measurement_id']]
        assert summary['tool'] == call['tool'] and summary['outcome'] == 'returned'
        assert not summary.get('http')
        search = [c for c in summary['calls'] if c['backend'] == 'ckv' and c['method'] == 'semantic_search']
        assert search and search[0]['options']['K'] == 20
        backend_errors.extend({'measurement_id': call['measurement_id'], 'call': c}
                              for c in summary['calls'] if c['outcome'] != 'returned')
        response = call['response']
        assert not response.get('isError')
        pack = response.get('structuredContent') or json.loads(response['content'][0]['text'])
        clone = copy.deepcopy(pack)
        expected = clone['metadata'].pop('integrity_hash')
        assert sha(json.dumps(clone, sort_keys=True, ensure_ascii=False, separators=(',', ':')).encode()) == expected
        assert pack['coordinates']['dataset_id'] == identity['dataset_id']
        assert pack['citations'], 'synthetic guide must return retained evidence'
        for citation in pack['citations']:
            for key, expected in {'project_id': identity['source']['project_id'],
                                  'dataset_id': identity['dataset_id'],
                                  'snapshot_id': identity['source']['snapshot_id'],
                                  'commit_hash': identity['source']['source_commit']}.items():
                assert citation[key] == expected
            raw = source[citation['file']]
            lines = raw.splitlines(keepends=True)
            assert 1 <= citation['start_line'] <= citation['end_line'] <= len(lines)
            span = b''.join(lines[citation['start_line'] - 1:citation['end_line']])
            assert sha(raw) == citation['file_sha256'] and sha(span) == citation['content_sha256']
            citations += 1
        for body in pack['bodies']:
            assert body['citation'] in pack['citations']
            assert sha(body['text'].encode()) == body['citation']['content_sha256']
        value = {'citations': pack['citations'], 'bodies': pack['bodies']}
        mode = row['arm'].rsplit('_', 1)[0]
        ontology = pack['metadata'].get('ontology')
        if mode == 'baseline':
            assert ontology is None
        else:
            assert ontology['state'] == 'unavailable' and ontology['mode'] == mode
        key = call['arguments']['include_knowledge']
        if key in comparisons:
            assert value == comparisons[key], 'fallback arm/phase changed retained evidence'
        comparisons[key] = value
    assert len(ids) == len(scopes) == 24 and startups == 16
    assert (matrix / 'report.json').stat().st_mode & 0o777 == 0o600
    assert (matrix / 'rows.jsonl').stat().st_mode & 0o777 == 0o600
    return {'rows': len(rows), 'citations_verified': citations, 'unique_measurement_ids': len(ids),
            'state': report['state'], 'quality_metrics': None, 'source_head': report['source_head'],
            'indexed_commit': identity['source']['source_commit'], 'dataset_id': identity['dataset_id'],
            'startup_scopes': startups, 'backend_errors': backend_errors, 'source_snapshot': manifest}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--bin-dir', type=Path, required=True)
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    assert platform.system() == 'Linux' and not shutil.which('go') and not shutil.which('ps')
    args.out.mkdir(mode=0o700)  # refuse existing output
    repo = Path(__file__).resolve().parent.parent
    env = dict(os.environ, KS_BIN_DIR=str(args.bin_dir),
               KS_SANITIZE_RULES=str(repo / 'system/policies/sanitization_rules.yaml'),
               KS_INSTALL_SMOKE_DIR=str(args.out / 'install'))
    with (args.out / 'install.log').open('w') as log:
        subprocess.run(['bash', str(repo / 'scripts/wbs-install-smoke.sh')], env=env, stdout=log, stderr=subprocess.STDOUT, check=True)
    result = {'scope': 'Linux minimal runtime/mock synthetic integration, not B0/B1 quality or release approval',
              'go_available': False, 'ps_available': False, 'cases': {}}
    for kind in ('empty-go', 'typescript', 'unsupported-python'):
        base = args.out / 'install' / kind
        request = base / 'requests.json'
        request.write_text(json.dumps({'schema_version': 1, 'requests': [{
            'id': 'guide', 'tool': 'cks.context.get_for_task_v2',
            'arguments': {'prompt': f'Where is the committed guide for {kind}?',
                          'knowledge_as_of': '2026-10-04', 'knowledge_subsystem': kind}}]}) + '\n')
        descriptor = base / 'diagnostic-input.json'
        descriptor.write_text(json.dumps({'scope': result['scope'], 'kind': kind,
                                         'official_protocol_approval': False}) + '\n')
        matrix = args.out / ('matrix-' + kind)
        with (args.out / ('matrix-' + kind + '.log')).open('w') as log:
            subprocess.run([str(args.bin_dir / 'cks'), 'eval', 'matrix', '--requests', str(request),
                            '--config', str(base / 'mcp.yaml'), '--output', str(matrix),
                            '--warmup', '0', '--retrieval-runs', '1', '--warm-runs', '1', '--cold-runs', '1',
                            '--environment-ledger', '--environment-note', result['scope'] + '; competing host work not excluded',
                            '--lock-file', str(descriptor)], stdout=log, stderr=subprocess.STDOUT, check=True)
        result['cases'][kind] = verify(base, matrix)
    (args.out / 'verification.json').write_text(json.dumps(result, indent=2) + '\n')
    print(json.dumps({'state': 'verified', 'cases': len(result['cases']), 'rows': 72, 'output': str(args.out)}))


if __name__ == '__main__':
    main()
