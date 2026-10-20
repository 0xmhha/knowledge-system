#!/usr/bin/env python3
"""Reconstruct KM-01 final isolation results from raw owned protocol evidence."""
import copy
import gzip
import hashlib
import importlib.util
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
PACKET = ROOT / 'system/eval/knowledge-roadmap-2026-10-15/km-01-final-isolation'


def module(name, path):
    spec = importlib.util.spec_from_file_location(name, path)
    result = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(result)
    return result


inventory = module('isolation_inventory', ROOT / 'scripts/check-knowledge-roadmap.py')
runner = module('isolation_protocol', ROOT / 'scripts/verify-collection-isolation.py')


def require(value, message):
    if not value:
        raise ValueError(message)


def sha(data):
    return hashlib.sha256(data).hexdigest()


def check(folder=PACKET, root=ROOT):
    try:
        folder, root = folder.resolve(), root.resolve()
        def raw(name):
            p = folder / name
            return p.read_bytes() if p.exists() else gzip.decompress((folder / (name + '.gz')).read_bytes())
        def load(name):
            return json.loads(raw(name))
        m = load('manifest.json')
        require(m['scope'] == 'KM01_completed_local_isolation_not_independent_quality_or_versioned_writes'
                and m['KM01_local_isolation_complete'] is True and m['independent_acceptance'] is False
                and m['production_activated'] is False and m['versioned_DB_writes_added'] is False, 'acceptance scope promoted')
        actual = {p.relative_to(folder).as_posix() for p in folder.rglob('*') if p.is_file() and p.name != 'manifest.json'}
        names = [r['path'] for r in m['artifacts']]
        require(len(names) == len(set(names)) and set(names) == actual, 'artifact inventory differs')
        for row in m['artifacts']:
            path = (folder / row['path']).resolve()
            require(path.is_relative_to(folder), 'unsafe artifact path')
            data = path.read_bytes()
            require(sha(data) == row['sha256'] and len(data) == row['bytes'], 'artifact bytes differ')
            if 'uncompressed_sha256' in row:
                require(sha(gzip.decompress(data)) == row['uncompressed_sha256'], 'uncompressed raw differs')
        for name, digest in m['source_sha256'].items():
            path = (root / name).resolve()
            require(path.is_relative_to(root) and sha(path.read_bytes()) == digest, 'current script/source differs')
        binding = load('reused-source-binding.json')
        require(len(binding['source_sha256']) == 1652, 'Go source inventory differs')
        for name, digest in binding['source_sha256'].items():
            path = (root / name).resolve()
            require(path.is_relative_to(root) and sha(path.read_bytes()) == digest, 'reused Go validation no longer applies')
        for ref in m['reused_evidence']:
            require(sha((root / ref['manifest']).read_bytes()) == ref['sha256'], 'historical evidence changed')
        summary = load('summary.json')
        require(summary['checks_passed'] == len(summary['checks']) == len(set(summary['checks'])) == 52
                and summary['tool_calls'] == 46 and summary['datasets_built'] == 4 and summary['sessions_clean_EOF'] == 3
                and summary['embedding_model_quality_evaluated'] is False and summary['paid_model_calls'] == 0
                and summary['independent_acceptance'] is False and summary['production_activated'] is False
                and summary['versioned_DB_writes_added'] is False
                and summary['model_responder'] == 'loopback_deterministic_protocol_fixture', 'runtime denominator/scope differs')
        commands = load('commands.json')
        require(all(row['exit_code'] == 0 and sha(raw(row['label'] + '.stdout.txt')) == row['stdout_sha256']
                and sha(raw(row['label'] + '.stderr.txt')) == row['stderr_sha256'] for row in commands), 'command failed/receipt differs')
        rejections = load('startup-rejections.json')
        require(len(rejections) == 6 and {r['case'] for r in rejections} == {'duplicate-project', 'duplicate-version',
                'shared-version-path', 'fork-cycle', 'wrong-member-model', 'wrong-member-dimension'}, 'startup cases missing')
        for r in rejections:
            require(r['exit_code'] == 1 and r['stdout_empty'] is True and raw('rejected-' + r['case'] + '.stdout.txt') == b''
                    and b'"code":"config_invalid"' in raw('rejected-' + r['case'] + '.stderr.txt'), 'unsafe startup accepted')
        sessions = load('sessions.json')
        require({r['session'] for r in sessions} == {'public', 'restarted', 'membership-changed'} and len(sessions) == 3
                and all(r['exit_code'] == 0 and r['clean_EOF'] is True for r in sessions), 'shutdown failed')
        coords, files, vectors = {}, {}, {}
        for key in ('a1', 'a2', 'b1', 'b2'):
            identity = load('datasets/' + key + '/dataset-identity.json')
            coords[key] = {k: identity['source'][k] for k in ('project_id', 'snapshot_id', 'source_mode')}
            coords[key].update(dataset_id=identity['dataset_id'], base_commit=identity['source']['source_commit'])
            require(coords[key]['source_mode'] == 'committed', 'fixture source mode changed')
            source = load('datasets/' + key + '/source-manifest.json')
            require(source['identity'] == identity['source'], 'retained identity differs')
            files[key] = {(f['origin_id'], f['path']): f for f in source['files']}
            for kind in ('graph', 'vector'):
                manifest = load('datasets/' + key + '/' + kind + '-manifest.json')
                require(all(manifest[k] == coords[key][k] for k in ('project_id', 'dataset_id', 'snapshot_id', 'source_mode')), 'engine tuple differs')
                if kind == 'vector':
                    vectors[key] = manifest
            expected_dim = {'a1': 64, 'a2': 64, 'b1': 96, 'b2': 128}[key]
            require(vectors[key]['embedding_dim'] == expected_dim, 'member embedding dimensions mixed')
            if key.startswith('b'):
                model = 'collection-fixture-' + str(expected_dim) + ':latest'
                embedding = vectors[key]['embedding_identity_v2']
                require(vectors[key]['embedding_model'] == model and embedding['Provider'] == 'ollama'
                        and embedding['Dim'] == embedding['native_dim'] == expected_dim
                        and embedding['Model'] == model and embedding['model_digest'] == sha(('owned-test-model:' + model).encode()),
                        'embedding model/digest differs')
        same = {k: f['sha256'] for k, f in files['a1'].items()}
        require(same == {k: f['sha256'] for k, f in files['b1'].items()} == {k: f['sha256'] for k, f in files['b2'].items()}
                and coords['a1']['base_commit'] == coords['b1']['base_commit'] == coords['b2']['base_commit'] == summary['same_fork_commit']
                and coords['a1']['base_commit'] != coords['a2']['base_commit']
                and coords['a1']['project_id'] != coords['b1']['project_id']
                and len({json.dumps(c, sort_keys=True) for c in coords.values()}) == 4
                and coords['b1']['dataset_id'] != coords['b2']['dataset_id'], 'fork/code/version identity differs')
        # Git object IDs are reconstructed, including signed commit/tree/blob bytes.
        for path in (folder / 'git-objects').iterdir():
            obj = path.read_bytes()
            kind = load('git-object-types.json')[path.name]
            require(hashlib.sha1(kind.encode() + b' ' + str(len(obj)).encode() + b'\0' + obj).hexdigest() == path.name,
                    'Git object differs')
        for key in ('a1', 'a2', 'b1', 'b2'):
            commit = raw('git-objects/' + coords[key]['base_commit'])
            tree_id = commit.splitlines()[0].decode().removeprefix('tree ')
            tree = raw('git-objects/' + tree_id)
            entries = {}
            while tree:
                label, tree = tree.split(b'\0', 1)
                entries[label.split(b' ', 1)[1].decode()] = tree[:20].hex()
                tree = tree[20:]
            for (_, name), f in files[key].items():
                data = raw('source-blobs/' + f['sha256'] + '.txt')
                require(sha(data) == f['sha256'] and data == raw('git-objects/' + entries[name]), 'Git/retained bytes differ')
        http = [json.loads(x) for x in raw('embedding-http.jsonl').splitlines()]
        for n, h in enumerate(http):
            require(h['sequence'] == n and h['status'] == 200 and h['path'] in ('/api/tags', '/api/embed'), 'unexpected HTTP request')
            if h['path'] == '/api/embed':
                request, response = h['request'], h['response']
                require(request['model'] in runner.MODELS and request['truncate'] is False and response['model'] == request['model']
                        and response['embeddings'] == [runner.vector(t, runner.MODELS[request['model']]) for t in request['input']],
                        'embedding HTTP model/dimension/body differs')
        results = load('results.json')
        wire, count, errors, pack_count, writes = [], 0, 0, 0, 0
        def verify(pack, expected):
            nonlocal pack_count
            require(pack['coordinates'] == expected and pack['query'] == 'SharedConsensus', 'wrong evidence target')
            key = next(k for k, c in coords.items() if c == expected)
            value = copy.deepcopy(pack)
            integrity = value['metadata'].pop('integrity_hash')
            require(sha(json.dumps(value, sort_keys=True, ensure_ascii=False, separators=(',', ':')).encode()) == integrity, 'integrity differs')
            require(pack['citations'] and pack['bodies'], 'no actual cited body')
            for c in pack['citations']:
                require(all(c[k] == v for k, v in expected.items()), 'foreign citation')
                f = files[key][(c['origin_id'], c['file'])]
                data = raw('source-blobs/' + f['sha256'] + '.txt')
                require(sha(data) == f['sha256'] == c['file_sha256'] and
                        sha(b''.join(data.splitlines(keepends=True)[c['start_line']-1:c['end_line']])) == c['content_sha256'], 'span differs')
            require(all(b['citation'] in pack['citations'] and sha(b['text'].encode()) == b['citation']['content_sha256'] for b in pack['bodies']), 'body differs')
            require(str(303 if key == 'a2' else 101) in '\n'.join(b['text'] for b in pack['bodies']), 'version marker differs')
            pack_count += 1
        statuses = []
        for session in ('public', 'restarted', 'membership-changed'):
            requests = [json.loads(x) for x in raw(session + '/mcp.stdin.jsonl').splitlines()]
            replies = [json.loads(x) for x in raw(session + '/mcp.stdout.jsonl').splitlines()]
            by_id = {r['id']: r for r in replies}
            require(len(requests) == len(replies) == len(by_id), 'wire pair missing/duplicated')
            count += len(requests)
            for r in requests:
                reply = by_id[r['id']]
                require('error' not in reply, 'protocol error')
                if r['method'] == 'tools/list':
                    names = {t['name'] for t in reply['result']['tools']}
                    require(len(names) == 35 and {'cks.ops.index', 'cks.ops.setup', 'cks.ops.reindex'} <= names, 'tool inventory differs')
                if r['method'] != 'tools/call':
                    continue
                name, args, response = r['params']['name'], r['params']['arguments'], reply['result']
                wire.append({'session': session, 'tool': name, 'arguments': args, 'response': response})
                value = response.get('structuredContent', {})
                if response.get('isError'):
                    errors += 1
                    require(not value.get('groups') and not value.get('job_id'), 'failure published partial evidence/job')
                    if name in ('cks.ops.index', 'cks.ops.setup', 'cks.ops.reindex'):
                        writes += 1
                        require('versioned_setup_required' in json.dumps(response), 'writer gate bypassed')
                    elif name == 'cks.ops.setup_status':
                        require('unknown job_id' in json.dumps(response), 'unexpected status')
                    else:
                        require(value['code'] in ('scope_mismatch', 'stale_catalog', 'fetch_failed'), 'unplanned error')
                    continue
                if name == 'cks.collection.status':
                    require(value['write_enabled'] is False and value['member_count'] == 4 and value['state'] == 'configured'
                            and 'version_dir' not in json.dumps(value), 'status scope differs')
                    statuses.append(value['catalog_digest'])
                else:
                    require(name == 'cks.collection.get_context' and value['catalog_digest'] == args['catalog_digest']
                            and value['collection_id'] == args['collection_id'] and value['mode'] == args['mode'], 'response scope differs')
                    expected = [args['target']] if args['mode'] == 'implementation' else args['participants']
                    require(len(value['groups']) == len(expected), 'group lost/deduplicated')
                    for group, target in zip(value['groups'], expected, strict=True):
                        require(group['coordinates'] == target, 'group target differs')
                        verify(group['evidence'], target)
        require(count == 52 and len(wire) == len(results) == 46 and errors == 26 and writes == 18 and pack_count == 20,
                'wire/tool/error/write/group denominator differs')
        require(len(statuses) == 3 and statuses[0] == statuses[1] != statuses[2], 'catalog restart/membership pin differs')
        previous_http_end = 0
        for w, r in zip(wire, results, strict=True):
            require(w == {k: v for k, v in r.items() if k not in ('http_start', 'http_end')}, 'wire/report differs')
            require(previous_http_end <= r['http_start'] <= r['http_end'] <= len(http), 'HTTP boundary invalid')
            previous_http_end = r['http_end']
            if r['tool'] == 'cks.collection.get_context' and not r['response'].get('isError') and r['arguments']['mode'] == 'implementation':
                key = next(k for k, c in coords.items() if c == r['arguments']['target'])
                called = [h['request']['model'] for h in http[r['http_start']:r['http_end']] if h['path'] == '/api/embed']
                require((called and set(called) == {vectors[key]['embedding_model']}) if key.startswith('b') else not called, 'foreign embedder called')
        require(load('writer-tree-before.json') == load('writer-tree-after.json'), 'rejected writer mutated dataset')
        preservation = load('preservation.json')
        require(preservation['unchanged'] is True and len(preservation['files']) == 24
                and len(preservation['current_pointers']) == 2 and preservation['other_project_current_unchanged'] is True,
                'final DB/pointer preservation differs')
        post = load('post-run-state.json')
        require(post['files'] == preservation['files'] and post['current_pointers'] == preservation['current_pointers'],
                'post-run file hashes/pointers differ from runtime-before')
        for project in ('project-a', 'project-b'):
            plan = load('gc-' + project + '.stdout.txt')
            require(len(plan['versions']) == 2 and all(not v['delete'] and 'pinned_reader' in v['protected_reasons'] for v in plan['versions']), 'GC pin lost')
        validation = load('validation.json')
        module_summary, _ = inventory.test_inventory(raw('module-tests.jsonl'))
        race_summary, _ = inventory.test_inventory(raw('adapter-race.jsonl'))
        require(validation['go_validation'] == 'source_bound_reuse_no_Go_source_changes' and validation['module'] == module_summary
                and validation['race'] == race_summary and module_summary['failed_tests'] == module_summary['failed_packages'] == 0
                and race_summary['failed_tests'] == race_summary['failed_packages'] == 0, 'reused Go cases differ')
        require(load('test-inventory.json') == {'module': inventory.test_rows(raw('module-tests.jsonl')),
                'race': inventory.test_rows(raw('adapter-race.jsonl'))}, 'test inventory differs')
        require(set(validation['gates']) == {'runtime', 'documentation', 'current_roadmap_tests', 'historical_public_guard', 'source_binding'}
                and all(v == 0 for v in validation['gates'].values()), 'final gate failed/missing')
        progress = json.loads((root / 'docs/spec-driven/KNOWLEDGE-ROADMAP-PROGRESS.json').read_text())
        item = next(i for i in progress['items'] if i['id'] == 'KM-01')
        require(item['status'] == 'complete' and all(s['status'] == 'complete' for s in item['subtasks'])
                and progress['completed_subtasks'] == 4 and progress['implementation_completed'] == 1, 'local feature completion differs')
        require(not inventory.check(root=root), 'roadmap/library evidence mismatch')
    except (OSError, ValueError, KeyError, TypeError, IndexError, StopIteration) as exc:
        return [str(exc)]
    return []


if __name__ == '__main__':
    errors = check()
    for e in errors:
        print('FAIL:', e)
    print('final collection isolation:', 'FAIL' if errors else 'PASS; local KM-01 complete; quality and versioned writes separate')
    raise SystemExit(bool(errors))
