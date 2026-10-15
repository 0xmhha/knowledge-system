#!/usr/bin/env python3
"""Exercise opt-in collection MCP with real owned graph/vector DBs and a mock embedder."""
import argparse
import copy
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import time

import yaml

ROOT = Path(__file__).resolve().parents[1]


def require(value, message):
    if not value:
        raise ValueError(message)


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    out = args.out.resolve()
    require(not out.exists(), 'fresh owned output required')
    out.mkdir()
    binary = ROOT / 'bin/cks'
    receipts, checks, results = [], [], []
    def write(name, data):
        p = out / name
        p.parent.mkdir(parents=True, exist_ok=True)
        p.write_text(json.dumps(data, ensure_ascii=False, indent=2) + '\n')
    def run(label, command):
        start = time.monotonic()
        env = os.environ.copy()
        env.update(GOTOOLCHAIN='go1.26.9', GOCACHE='/private/tmp/ks-corpus-go-cache')
        r = subprocess.run([str(v) for v in command], cwd=ROOT, env=env, capture_output=True, timeout=240)
        (out / (label + '.stdout.txt')).write_bytes(r.stdout)
        (out / (label + '.stderr.txt')).write_bytes(r.stderr)
        receipts.append({'label': label, 'command': [str(v) for v in command], 'exit_code': r.returncode,
                         'elapsed_seconds': time.monotonic() - start})
        write('commands.json', receipts)
        r.check_returncode()
    specs = [('a1', 'project-a', 'v1', 101), ('a2', 'project-a', 'v2', 303), ('b1', 'project-b', 'v1', 202)]
    versions, coordinates, profiles = {}, {}, []
    for key, project, version, value in specs:
        src = out / project / 'source'
        src.mkdir(parents=True, exist_ok=True)
        (src / 'go.mod').write_text('module example.invalid/common\n\ngo 1.26.9\n')
        (src / 'consensus.go').write_text('package common\n\n// SharedConsensus returns the project-specific test marker.\nfunc SharedConsensus() int { return ' + str(value) + ' }\n')
        dataset = out / project / 'dataset'
        run('build-' + key, [binary, 'setup', '--src', src, '--out', dataset, '--version', version,
            '--project-id', project, '--source-mode', 'snapshot-only', '--embedder', 'mock', '--model-name', 'mock',
            '--graph-bin', ROOT / 'bin/ckg', '--vector-bin', ROOT / 'bin/ckv', '--build-timeout', '3m', '--progress', 'json'])
        vdir = dataset / version
        identity = json.loads((vdir / 'dataset-identity.json').read_text())
        c = {k: identity['source'][k] for k in ('project_id', 'snapshot_id', 'source_mode')}
        c.update(dataset_id=identity['dataset_id'], base_commit=identity['source']['source_commit'])
        coordinates[key], versions[key] = c, vdir
        cfg = out / (key + '.yaml')
        vector_manifest = json.loads((vdir / 'vector/manifest.json').read_text())
        run('config-' + key, [binary, 'mcp', 'gen-config', '--dataset-dir', vdir, '--source-root', src,
            '--embed-model', vector_manifest['embedding_model'], '--sanitize-rules', ROOT / 'system/policies/sanitization_rules.yaml', '--out', cfg])
        data = yaml.safe_load(cfg.read_text())
        data['backends']['ckv']['provider'] = 'mock'
        data['listen'].update(transport='stdio', mcp_stdio=True)
        data['logging']['level'] = 'error'
        cfg.write_text(yaml.safe_dump(data, sort_keys=False))
        profiles.append(str(cfg))
        print(key + ': actual graph/vector build and config prepared', flush=True)
    catalog = {'schema_version': 1, 'projects': [{'id': 'project-a', 'name': 'same-name'},
                {'id': 'project-b', 'name': 'same-name', 'upstream_project_id': 'project-a'}],
        'versions': [{'coordinates': coordinates[key], 'version_dir': str(v), 'origins': ['repo']} for key, v in versions.items()],
        'collections': [{'id': 'all', 'members': list(coordinates.values())}]}
    write('catalog.json', catalog)
    top = yaml.safe_load((out / 'a2.yaml').read_text())
    top['collection'] = {'catalog_path': str(out / 'catalog.json'), 'member_configs': profiles}
    (out / 'collection.yaml').write_text(yaml.safe_dump(top, sort_keys=False))
    preserved = {}
    for vdir in versions.values():
        for name in ('graph/graph.db', 'vector/vector.db', 'graph/manifest.json', 'vector/manifest.json', 'dataset-identity.json', 'sources/manifest.json'):
            p = vdir / name
            preserved[str(p)] = sha(p.read_bytes())
    pointers = {str(v.parent / 'current'): os.readlink(v.parent / 'current') for v in versions.values()}
    # Refuse operator profile mistakes before accepting any MCP request.
    rejected = []
    edits = [
        ('cross-index', lambda c: c['backends']['ckv'].update(path=str(versions['b1']/'vector'))),
        ('nested-collection', lambda c: c.update(collection=top['collection'])),
        ('different-sanitizer', lambda c: c['sanitize'].update(rules_path='/owned/different-rules.yaml')),
        ('unavailable-model', lambda c: c['backends']['ckv'].update(embed_model='unavailable-mock-model')),
    ]
    for name, edit in edits:
        member = yaml.safe_load((out/'a1.yaml').read_text())
        edit(member)
        bad_member = out / ('rejected-member-' + name + '.yaml')
        bad_member.write_text(yaml.safe_dump(member, sort_keys=False))
        parent = copy.deepcopy(top)
        parent['collection']['member_configs'][0] = str(bad_member)
        path = out / ('rejected-' + name + '.yaml')
        path.write_text(yaml.safe_dump(parent, sort_keys=False))
        rejected.append((name, path))
    parent = copy.deepcopy(top)
    parent['collection']['member_configs'] = profiles[:2]
    path = out / 'rejected-missing-profile.yaml'
    path.write_text(yaml.safe_dump(parent, sort_keys=False))
    rejected.append(('missing-profile', path))
    bad_catalog = copy.deepcopy(catalog)
    bad_catalog['versions'][0]['version_dir'] = str(versions['a1'].parent/'current')
    write('rejected-current-catalog.json', bad_catalog)
    parent = copy.deepcopy(top)
    parent['collection']['catalog_path'] = str(out/'rejected-current-catalog.json')
    path = out/'rejected-current.yaml'
    path.write_text(yaml.safe_dump(parent, sort_keys=False))
    rejected.append(('mutable-current', path))
    startup_results = []
    for name, path in rejected:
        r = subprocess.run([str(binary), 'mcp', '--config', str(path)], input=b'', capture_output=True, timeout=30)
        (out/('rejected-' + name + '.stdout.txt')).write_bytes(r.stdout)
        (out/('rejected-' + name + '.stderr.txt')).write_bytes(r.stderr)
        require(r.returncode == 1 and r.stdout == b'' and b'"code":"config_invalid"' in r.stderr,
                'bad profile served or failed without a safe config error: ' + name)
        startup_results.append({'case': name, 'exit_code': r.returncode, 'stdout_empty': True, 'code': 'config_invalid'})
        checks.append('startup-rejected:' + name)
    write('startup-rejections.json', startup_results)
    library = importlib.util.spec_from_file_location('mcp_probe', ROOT / 'scripts/eval-value-dependencies.py')
    lib = importlib.util.module_from_spec(library)
    library.loader.exec_module(lib)
    def connect(label, cfg):
        folder = out / label
        folder.mkdir()
        client = lib.Client(None, None, folder, command=[str(binary), 'mcp', '--config', str(cfg)], namespace='cks.collection.')
        r = client.request('initialize', {'protocolVersion': '2024-11-05', 'capabilities': {}, 'clientInfo': {'name': 'owned-collection-evaluator', 'version': '1'}})
        require('error' not in r, 'initialize failed')
        return client
    def tool(client, name, arguments, expected_error=None):
        r = client.request('tools/call', {'name': name, 'arguments': arguments})
        require('error' not in r and 'result' in r, 'tool protocol failed')
        result = r['result']
        results.append({'tool': name, 'arguments': arguments, 'response': result})
        write('results.json', results)
        if expected_error:
            require(result.get('isError') and result.get('structuredContent', {}).get('code') == expected_error, 'wrong rejection: ' + expected_error)
        else:
            require(not result.get('isError'), 'unexpected tool failure: ' + name)
        return result.get('structuredContent') or result
    def request(status, key):
        return {'catalog_digest': status['catalog_digest'], 'collection_id': 'all', 'mode': 'implementation',
                'query': 'SharedConsensus', 'target': copy.deepcopy(coordinates[key])}
    def verify(data, keys):
        require(len(data['groups']) == len(keys), 'wrong group count')
        for group, key in zip(data['groups'], keys, strict=True):
            pack = group['evidence']
            require(group['coordinates'] == pack['coordinates'] == coordinates[key], 'cross-project/version group')
            value = copy.deepcopy(pack)
            integrity = value['metadata'].pop('integrity_hash')
            require(sha(json.dumps(value, sort_keys=True, ensure_ascii=False, separators=(',', ':')).encode()) == integrity, 'integrity mismatch')
            require(pack['citations'] and pack['bodies'], 'no real evidence retrieved')
            manifest = json.loads((versions[key] / 'sources/manifest.json').read_text())
            files = {(r['origin_id'], r['path']): r for r in manifest['files']}
            for c in pack['citations']:
                require(all(c[k] == v for k, v in coordinates[key].items()), 'foreign citation')
                f = files[(c['origin_id'], c['file'])]
                raw = (versions[key] / 'sources/blobs' / f['sha256']).read_bytes()
                require(sha(raw) == f['sha256'] == c['file_sha256'], 'wrong retained file')
                require(sha(b''.join(raw.splitlines(keepends=True)[c['start_line']-1:c['end_line']])) == c['content_sha256'], 'wrong retained span')
            text = '\n'.join(b['text'] for b in pack['bodies'])
            require(str(dict((r[0], r[3]) for r in specs)[key]) in text, 'requested version marker not retrieved')
    client = connect('public', out / 'collection.yaml')
    try:
        tools = client.request('tools/list')['result']['tools']
        write('tools.json', tools)
        require(len(tools) == 35 and all(next(t for t in tools if t['name'] == n)['annotations']['readOnlyHint']
                    for n in ('cks.collection.status', 'cks.collection.get_context')), 'optional tools/schema mismatch')
        status = tool(client, 'cks.collection.status', {})
        require(status['member_count'] == 3 and status['write_enabled'] is False and str(out) not in json.dumps(status), 'status leaks paths/writes')
        for key in ('a1', 'b1', 'a2', 'b1', 'a1'):
            verify(tool(client, 'cks.collection.get_context', request(status, key)), [key])
            checks.append('implementation:' + key)
        comparison = {'catalog_digest': status['catalog_digest'], 'collection_id': 'all', 'mode': 'comparison',
                      'query': 'SharedConsensus', 'participants': list(coordinates.values())}
        verify(tool(client, 'cks.collection.get_context', comparison), list(coordinates))
        checks.append('explicit-three-version-comparison')
        cases = [
            ('missing-target', lambda r: r.pop('target'), 'target_required'),
            ('stale-catalog', lambda r: r.update(catalog_digest='0'*64), 'stale_catalog'),
            ('foreign-project', lambda r: r['target'].update(project_id='foreign'), 'scope_mismatch'),
            ('wrong-version', lambda r: r['target'].update(snapshot_id='0'*64), 'scope_mismatch'),
            ('unknown-collection', lambda r: r.update(collection_id='unknown'), 'scope_mismatch'),
            ('injected-path', lambda r: r.update(config_path='/private/unregistered'), 'scope_mismatch'),
            ('unknown-mode', lambda r: r.update(mode='auto'), 'unsupported_mode'),
        ]
        for name, edit, code in cases:
            q = request(status, 'a1')
            edit(q)
            tool(client, 'cks.collection.get_context', q, code)
            checks.append('rejected:' + name)
        blob_manifest = json.loads((versions['b1'] / 'sources/manifest.json').read_text())
        source = next(f for f in blob_manifest['files'] if f['path'] == 'consensus.go')
        blob = versions['b1'] / 'sources/blobs' / source['sha256']
        original, mode = blob.read_bytes(), blob.stat().st_mode & 0o777
        try:
            blob.chmod(0o600)
            blob.write_bytes(b'owned corruption fixture\n')
            tool(client, 'cks.collection.get_context', comparison, 'dataset_mismatch')
            verify(tool(client, 'cks.collection.get_context', request(status, 'a1')), ['a1'])
            checks.append('corrupt-comparison-fails-and-other-target-still-reads')
        finally:
            blob.write_bytes(original)
            blob.chmod(mode)
        verify(tool(client, 'cks.collection.get_context', comparison), list(coordinates))
        checks.append('restored-archive-recovers')
        negative = client.request('tools/call', {'name': 'cks.ops.reindex', 'arguments': {'src': str(out/'project-a/source'), 'out': str(versions['a1'].parent)}})['result']
        write('maintenance-rejection.json', negative)
        require(negative.get('isError') and 'versioned_setup_required' in json.dumps(negative), 'pinned writer guard bypassed')
        checks.append('existing-pinned-write-rejection')
    finally:
        client.close()
    restarted = connect('restarted', out / 'collection.yaml')
    try:
        again = tool(restarted, 'cks.collection.status', {})
        require(again['catalog_digest'] == status['catalog_digest'], 'restart changed catalog pin')
        verify(tool(restarted, 'cks.collection.get_context', request(again, 'a1')), ['a1'])
        checks.append('restart-old-version-with-current-and-source-newer')
    finally:
        restarted.close()
    single = connect('single-project', out / 'a2.yaml')
    try:
        tools = single.request('tools/list')['result']['tools']
        require(len(tools) == 33 and not any(t['name'].startswith('cks.collection.') for t in tools), 'default schema changed')
        pack = tool(single, 'cks.context.get_for_task_v2', {'prompt': 'SharedConsensus'})
        verify({'groups': [{'coordinates': pack['coordinates'], 'evidence': pack}]}, ['a2'])
        checks.append('legacy-single-project-v2-and-default-33-tools')
    finally:
        single.close()
    require(all(sha(Path(p).read_bytes()) == h for p, h in preserved.items()), 'database/source identities changed')
    require(all(os.readlink(p) == target for p, target in pointers.items()), 'current pointer changed')
    write('preservation.json', {'files': preserved, 'current_pointers': pointers, 'unchanged': True})
    write('summary.json', {'scope': 'owned_real_CKG_CKV_MCP_with_mock_embeddings_not_quality_acceptance',
        'checks': checks, 'checks_passed': len(checks), 'public_tool_calls': len(results)+1, 'MCP_sessions_closed': 3,
        'datasets_built': 3, 'optional_tools': 2, 'default_tools': 33, 'configured_tools': 35,
        'startup_rejections': len(startup_results),
        'database_and_pointer_preserved': True, 'independent_acceptance': False, 'production_activated': False,
        'KM01_3_public_read_connected': True, 'full_KM01_complete': False, 'versioned_DB_writes_added': False,
        'paid_model_calls': 0, 'mock_embedding_quality_evaluated': False})
    print('public collection verification complete:', len(checks), 'checks;', len(results)+1, 'tool calls; 3 clean EOF sessions', flush=True)


if __name__ == '__main__':
    main()
