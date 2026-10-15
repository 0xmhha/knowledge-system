#!/usr/bin/env python3
"""Verify public collection routing from raw MCP and retained source, without quality promotion."""
import copy
import gzip
import hashlib
import importlib.util
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
PACKET = ROOT / 'system/eval/knowledge-roadmap-2026-10-15/km-01-public-read'
spec = importlib.util.spec_from_file_location('roadmap_inventory', Path(__file__).with_name('check-knowledge-roadmap.py'))
inventory = importlib.util.module_from_spec(spec)
spec.loader.exec_module(inventory)


def require(value, message):
    if not value:
        raise ValueError(message)


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def check(folder=PACKET, root=ROOT):
    errors = []
    try:
        folder = folder.resolve()
        def raw(name):
            p = folder / name
            return p.read_bytes() if p.exists() else gzip.decompress((folder / (name + '.gz')).read_bytes())
        def load(name):
            return json.loads(raw(name))
        m = load('manifest.json')
        require(m['scope'] == 'public_collection_read_with_owned_real_indexes_mock_embeddings'
                and m['public_MCP_connected'] is True and m['full_KM01_complete'] is False
                and m['independent_acceptance'] is False and m['versioned_DB_writes_added'] is False,
                'public read scope promoted to full feature/quality/write acceptance')
        names = [r['path'] for r in m['artifacts']]
        actual = {p.relative_to(folder).as_posix() for p in folder.rglob('*') if p.is_file() and p.name != 'manifest.json'}
        require(len(names) == len(set(names)) and set(names) == actual, 'artifact inventory differs')
        for row in m['artifacts']:
            p = (folder / row['path']).resolve()
            require(p.is_relative_to(folder), 'unsafe artifact path')
            data = p.read_bytes()
            require(sha(data) == row['sha256'] and len(data) == row['bytes'], 'artifact hash differs')
            if 'uncompressed_sha256' in row:
                require(sha(gzip.decompress(data)) == row['uncompressed_sha256'], 'uncompressed wire differs')
        for path, digest in m['source_sha256'].items():
            p = (root / path).resolve()
            require(p.is_relative_to(root.resolve()) and sha(p.read_bytes()) == digest, 'current source differs')
        binding = load('source-binding.json')
        require(len(binding['source_sha256']) == 1652 and len(binding['added_sources']) == 5
                and len(binding['changed_preexisting_sources']) == 3
                and binding['approval_or_completion_states_changed'] is False, 'source inventory/scope differs')
        for path, digest in binding['source_sha256'].items():
            p = (root / path).resolve()
            require(p.is_relative_to(root.resolve()) and sha(p.read_bytes()) == digest, 'bound Go source differs')
        summary = load('summary.json')
        require(summary['checks_passed'] == len(summary['checks']) == 24 and summary['public_tool_calls'] == 21
                and summary['MCP_sessions_closed'] == summary['datasets_built'] == 3
                and summary['production_activated'] is False and summary['independent_acceptance'] is False
                and summary['paid_model_calls'] == 0 and summary['mock_embedding_quality_evaluated'] is False
                and summary['full_KM01_complete'] is False and summary['versioned_DB_writes_added'] is False,
                'runtime observation count/scope differs')
        require(all(r['exit_code'] == 0 for r in load('commands.json')), 'build/config failed')
        rejections = load('startup-rejections.json')
        require({r['case'] for r in rejections} == {'cross-index', 'nested-collection', 'different-sanitizer',
                'unavailable-model', 'missing-profile', 'mutable-current'} and len(rejections) == 6
                and all(r['exit_code'] == 1 and r['stdout_empty'] is True and r['code'] == 'config_invalid' for r in rejections),
                'invalid member profile was not refused')
        catalog = load('catalog.json')
        coords = {}
        source_files = {}
        for key in ('a1', 'a2', 'b1'):
            identity = load('datasets/' + key + '/dataset-identity.json')
            c = {k: identity['source'][k] for k in ('project_id', 'snapshot_id', 'source_mode')}
            c.update(dataset_id=identity['dataset_id'], base_commit=identity['source']['source_commit'])
            coords[key] = c
            require(c['source_mode'] == 'snapshot-only' and c['base_commit'] == '', 'fixture source mode changed')
            manifest = load('datasets/' + key + '/source-manifest.json')
            require(manifest['identity'] == identity['source'], 'retained snapshot differs')
            source_files[key] = {(r['origin_id'], r['path']): r for r in manifest['files']}
            for engine in ('graph', 'vector'):
                engine_manifest = load('datasets/' + key + '/' + engine + '-manifest.json')
                require(all(engine_manifest[k] == c[k] for k in ('project_id', 'dataset_id', 'snapshot_id', 'source_mode')),
                        'engine/source tuple differs')
            vector = load('datasets/' + key + '/vector-manifest.json')
            require(vector['embedding_model'] == 'mock-feature-hash-v1' and vector['embedding_dim'] == 64,
                    'embedding identity differs')
        require(coords['a1']['project_id'] == coords['a2']['project_id'] != coords['b1']['project_id']
                and coords['a1']['dataset_id'] != coords['a2']['dataset_id'], 'fixture targets not distinct')
        status_digests, seen_results, valid_packs, citations = [], [], 0, 0
        tool_calls = wire_count = failures = 0
        optional_schema = None
        def verify(pack, expected):
            nonlocal valid_packs, citations
            require(pack['coordinates'] == expected and pack['query'] == 'SharedConsensus', 'wrong group target/query')
            key = next(k for k, c in coords.items() if c == expected)
            value = copy.deepcopy(pack)
            integrity = value['metadata'].pop('integrity_hash')
            require(sha(json.dumps(value, sort_keys=True, ensure_ascii=False, separators=(',', ':')).encode()) == integrity,
                    'pack integrity differs')
            require(pack['citations'] and pack['bodies'], 'real query produced no cited body')
            for c in pack['citations']:
                require(all(c[k] == v for k, v in expected.items()), 'foreign citation')
                f = source_files[key][(c['origin_id'], c['file'])]
                data = raw('source-blobs/' + f['sha256'] + '.txt')
                require(sha(data) == f['sha256'] == c['file_sha256'], 'retained file differs')
                require(sha(b''.join(data.splitlines(keepends=True)[c['start_line']-1:c['end_line']])) == c['content_sha256'],
                        'retained span differs')
                citations += 1
            for body in pack['bodies']:
                require(body['citation'] in pack['citations'] and sha(body['text'].encode()) == body['citation']['content_sha256'],
                        'wrong/foreign body bytes')
            require(str({'a1': 101, 'a2': 303, 'b1': 202}[key]) in '\n'.join(b['text'] for b in pack['bodies']),
                    'target marker not retrieved')
            valid_packs += 1
        for session in ('public', 'restarted', 'single-project'):
            requests = [json.loads(x) for x in raw(session + '/mcp.stdin.jsonl').splitlines()]
            replies = [json.loads(x) for x in raw(session + '/mcp.stdout.jsonl').splitlines()]
            by_id = {r['id']: r for r in replies}
            require(len(by_id) == len(replies) == len(requests), 'wire requests/replies missing/duplicated')
            wire_count += len(requests)
            for r in requests:
                reply = by_id[r['id']]
                require('error' not in reply, 'JSON-RPC error')
                if r['method'] == 'tools/list':
                    tools = reply['result']['tools']
                    require(len(tools) == (33 if session == 'single-project' else 35), 'default/optional tool set differs')
                    if session == 'single-project':
                        require(not any(t['name'].startswith('cks.collection.') for t in tools), 'default schema changed')
                    else:
                        optional_schema = {t['name']: t for t in tools if t['name'].startswith('cks.collection.')}
                        require(set(optional_schema) == {'cks.collection.status', 'cks.collection.get_context'}
                                and all(t['annotations']['readOnlyHint'] is True and t.get('outputSchema') for t in optional_schema.values()),
                                'collection schema/annotations differ')
                if r['method'] != 'tools/call':
                    continue
                tool_calls += 1
                name, arguments, response = r['params']['name'], r['params']['arguments'], reply['result']
                seen_results.append({'tool': name, 'arguments': arguments, 'response': response})
                value = response.get('structuredContent')
                if response.get('isError'):
                    failures += 1
                    require(not value or 'groups' not in value, 'failed comparison leaked partial groups')
                    if name == 'cks.ops.reindex':
                        require('versioned_setup_required' in json.dumps(response), 'pinned writer gate bypassed')
                        continue
                    if 'config_path' in arguments or arguments.get('collection_id') == 'unknown' or arguments.get('target', {}).get('project_id') == 'foreign' or arguments.get('target', {}).get('snapshot_id') == '0'*64:
                        code = 'scope_mismatch'
                    elif arguments.get('catalog_digest') == '0'*64:
                        code = 'stale_catalog'
                    elif arguments.get('mode') == 'auto':
                        code = 'unsupported_mode'
                    elif arguments.get('mode') == 'implementation' and 'target' not in arguments:
                        code = 'target_required'
                    else:
                        code = 'dataset_mismatch'
                    require(value['code'] == code and '/private/' not in json.dumps(response), 'public rejection differs/leaks details')
                    continue
                if name == 'cks.collection.status':
                    require(value['member_count'] == 3 and value['write_enabled'] is False and 'version_dir' not in json.dumps(value),
                            'status scope differs')
                    def membership(rows):
                        return {r['id']: sorted(json.dumps(c, sort_keys=True) for c in r['members']) for r in rows}
                    require(membership(value['collections']) == membership(catalog['collections']), 'status membership differs')
                    status_digests.append(value['catalog_digest'])
                elif name == 'cks.collection.get_context':
                    selected = [arguments['target']] if arguments['mode'] == 'implementation' else arguments['participants']
                    require(len(value['groups']) == len(selected) and value['catalog_digest'] == arguments['catalog_digest']
                            and value['collection_id'] == arguments['collection_id'] and value['mode'] == arguments['mode'],
                            'request/result scope differs')
                    for group, expected in zip(value['groups'], selected, strict=True):
                        require(group['coordinates'] == expected, 'group scope differs')
                        verify(group['evidence'], expected)
                else:
                    require(name == 'cks.context.get_for_task_v2', 'unplanned successful tool')
                    verify(value, coords['a2'])
        require(wire_count == 26 and tool_calls == 21 and failures == 9 and valid_packs == 14,
                'planned wire/tool/group denominator differs')
        require(len(status_digests) == 2 and status_digests[0] == status_digests[1], 'restart changed catalog pin')
        recorded = load('results.json') + [{'tool': 'cks.ops.reindex', 'arguments': next(r['arguments'] for r in seen_results if r['tool'] == 'cks.ops.reindex'), 'response': load('maintenance-rejection.json')}]
        require(sorted(json.dumps(r, sort_keys=True) for r in recorded) == sorted(json.dumps(r, sort_keys=True) for r in seen_results),
                'recorded responses differ from actual wire')
        preservation = load('preservation.json')
        require(preservation['unchanged'] is True and len(preservation['files']) == 18
                and len(preservation['current_pointers']) == 2, 'DB/current preservation failed')
        receipt = load('runtime-receipt.json')
        require(receipt['exit_code'] == 0 and receipt['sessions_clean_EOF'] == 3
                and receipt['no_paid_model_calls'] is True, 'runtime/shutdown did not pass')
        validation = load('validation.json')
        module, _ = inventory.test_inventory(raw('module-tests.jsonl'))
        race, _ = inventory.test_inventory(raw('adapter-race.jsonl'))
        require(module == validation['module'] and race == validation['race']
                and module['failed_tests'] == module['failed_packages'] == race['failed_tests'] == race['failed_packages'] == 0,
                'test counts/failures/skips differ')
        require(load('test-inventory.json') == {'module': inventory.test_rows(raw('module-tests.jsonl')),
                'race': inventory.test_rows(raw('adapter-race.jsonl'))}, 'full test list differs')
        require(set(validation['gates']) == {'module_test', 'adapter_race', 'build', 'adapter_lint', 'full_lint',
                'roadmap_guard_tests', 'ontology_guard'} and all(code == 0 for code in validation['gates'].values()),
                'final gate did not pass')
    except (OSError, ValueError, KeyError, TypeError, IndexError, StopIteration) as exc:
        errors.append(str(exc))
    return errors


if __name__ == '__main__':
    errors = check()
    for error in errors:
        print('FAIL:', error)
    print('public collection:', 'FAIL' if errors else 'PASS; scoped reads connected; full KM-01 and quality acceptance remain separate')
    raise SystemExit(bool(errors))
