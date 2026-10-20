#!/usr/bin/env python3
"""Owned Git forks + mixed local embedding protocols + public writer rejection.

The loopback responder implements a deterministic test protocol, not a real
embedding model. No production corpus, external model service or paid API is used.
"""
import argparse
import copy
import hashlib
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import importlib.util
import json
import math
import os
from pathlib import Path
import re
import subprocess
import threading

import yaml

ROOT = Path(__file__).resolve().parents[1]
MODELS = {'collection-fixture-96:latest': 96, 'collection-fixture-128:latest': 128}


def require(value, message):
    if not value:
        raise ValueError(message)


def sha(data):
    return hashlib.sha256(data).hexdigest()


def vector(text, dim):
    values = [0.0] * dim
    for word in re.findall(r'\w+', text.lower()):
        values[int(sha(word.encode())[:8], 16) % dim] += 1
    norm = math.sqrt(sum(v*v for v in values)) or 1
    return [v / norm for v in values]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--out', type=Path, required=True)
    out = parser.parse_args().out.resolve()
    require(not out.exists(), 'fresh owned fixture required')
    out.mkdir()
    def write(name, value):
        p = out / name
        p.parent.mkdir(parents=True, exist_ok=True)
        p.write_text(json.dumps(value, ensure_ascii=False, indent=2) + '\n')
    http_rows, receipts, results, checks, sessions = [], [], [], [], []
    digests = {name: sha(('owned-test-model:' + name).encode()) for name in MODELS}
    http_file = (out / 'embedding-http.jsonl').open('w')
    http_lock = threading.Lock()
    class Handler(BaseHTTPRequestHandler):
        def log_message(self, *_):
            pass
        def respond(self, request, response, status=200):
            body = json.dumps(response).encode()
            with http_lock:
                http_rows.append({'sequence': len(http_rows), 'method': self.command, 'path': self.path,
                                  'request': request, 'response': response, 'status': status})
                http_file.write(json.dumps(http_rows[-1]) + '\n')
                http_file.flush()
            self.send_response(status)
            self.send_header('Content-Type', 'application/json')
            self.send_header('Content-Length', str(len(body)))
            self.end_headers()
            self.wfile.write(body)
        def do_GET(self):
            if self.path != '/api/tags':
                self.respond(None, {'error': 'unsupported'}, 404)
                return
            self.respond(None, {'models': [{'name': name, 'digest': digests[name]} for name in MODELS]})
        def do_POST(self):
            request = json.loads(self.rfile.read(int(self.headers['Content-Length'])))
            model = request.get('model')
            if self.path != '/api/embed' or model not in MODELS:
                self.respond(request, {'error': 'unsupported'}, 400)
                return
            self.respond(request, {'model': model, 'embeddings': [vector(text, MODELS[model]) for text in request['input']]})
    server = ThreadingHTTPServer(('127.0.0.1', 0), Handler)
    worker = threading.Thread(target=server.serve_forever, daemon=True)
    worker.start()
    endpoint = 'http://127.0.0.1:' + str(server.server_port)
    env = os.environ.copy()
    env.update(GOTOOLCHAIN='go1.26.9', GOCACHE='/private/tmp/ks-corpus-go-cache')
    def run(label, command):
        r = subprocess.run([str(x) for x in command], cwd=ROOT, env=env, capture_output=True, timeout=240)
        (out / (label + '.stdout.txt')).write_bytes(r.stdout)
        (out / (label + '.stderr.txt')).write_bytes(r.stderr)
        receipts.append({'label': label, 'command': [str(x) for x in command], 'exit_code': r.returncode,
                         'stdout_sha256': sha(r.stdout), 'stderr_sha256': sha(r.stderr)})
        write('commands.json', receipts)
        require(r.returncode == 0, 'command failed: ' + label)
        return r.stdout
    def source(repo, marker):
        (repo / 'go.mod').write_text('module example.invalid/common\n\ngo 1.26.9\n')
        (repo / 'consensus.go').write_text('package common\n\n// SharedConsensus returns the retained test marker.\nfunc SharedConsensus() int { return ' + str(marker) + ' }\n')
    def commit(repo, label):
        run(label + '-add', ['git', '-C', repo, 'add', 'go.mod', 'consensus.go'])
        run(label + '-commit', ['git', '-C', repo, '-c', 'user.name=Collection Fixture', '-c',
            'user.email=collection-fixture@example.invalid', 'commit', '-m', label])
        return run(label + '-rev', ['git', '-C', repo, 'rev-parse', 'HEAD']).decode().strip()
    binary = ROOT / 'bin/cks'
    repo_a, repo_b = out / 'project-a/source', out / 'project-b/source'
    coords, versions, profiles = {}, {}, []
    markers = {'a1': 101, 'a2': 303, 'b1': 101, 'b2': 101}
    models = {'a1': None, 'a2': None, 'b1': 'collection-fixture-96:latest', 'b2': 'collection-fixture-128:latest'}
    try:
        repo_a.mkdir(parents=True)
        run('git-init', ['git', 'init', '--initial-branch=main', repo_a])
        source(repo_a, 101)
        original_commit = commit(repo_a, 'original')
        repo_b.parent.mkdir(parents=True)
        run('git-fork', ['git', 'clone', '--no-hardlinks', repo_a, repo_b])
        require(run('fork-rev', ['git', '-C', repo_b, 'rev-parse', 'HEAD']).decode().strip() == original_commit,
                'fork commit differs')
        for key, repo, version in [('a1', repo_a, 'v1'), ('b1', repo_b, 'v1'), ('b2', repo_b, 'v2'), ('a2', repo_a, 'v2')]:
            if key == 'a2':
                source(repo, 303)
                commit(repo, 'changed-a')
            dataset = repo.parent / 'dataset'
            command = [binary, 'setup', '--src', repo, '--out', dataset, '--version', version,
                '--project-id', repo.parent.name, '--source-mode', 'committed', '--embedder', 'ollama' if models[key] else 'mock',
                '--model-name', models[key] or 'mock', '--graph-bin', ROOT / 'bin/ckg', '--vector-bin', ROOT / 'bin/ckv',
                '--ollama-url', endpoint, '--build-timeout', '3m', '--progress', 'json']
            run('build-' + key, command)
            vdir = dataset / version
            versions[key] = vdir
            identity = json.loads((vdir / 'dataset-identity.json').read_text())
            c = {k: identity['source'][k] for k in ('project_id', 'snapshot_id', 'source_mode')}
            c.update(dataset_id=identity['dataset_id'], base_commit=identity['source']['source_commit'])
            coords[key] = c
            vector_manifest = json.loads((vdir / 'vector/manifest.json').read_text())
            cfg = out / (key + '.yaml')
            run('config-' + key, [binary, 'mcp', 'gen-config', '--dataset-dir', vdir, '--source-root', repo,
                '--embed-model', vector_manifest['embedding_model'], '--graph-binary', ROOT / 'bin/ckg',
                '--vector-binary', ROOT / 'bin/ckv', '--ollama-url', endpoint,
                '--sanitize-rules', ROOT / 'system/policies/sanitization_rules.yaml', '--out', cfg])
            data = yaml.safe_load(cfg.read_text())
            data['backends']['ckv']['provider'] = 'ollama' if models[key] else 'mock'
            data['listen'].update(transport='stdio', mcp_stdio=True)
            data['logging']['level'] = 'error'
            cfg.write_text(yaml.safe_dump(data, sort_keys=False))
            profiles.append(str(cfg))
            print(key + ': actual committed graph/vector; ' + str(vector_manifest['embedding_dim']) + ' dimensions', flush=True)
        require(coords['a1']['base_commit'] == coords['b1']['base_commit'] == coords['b2']['base_commit'] == original_commit,
                'identical fork commit lost')
        def retained_files(key):
            m = json.loads((versions[key] / 'sources/manifest.json').read_text())
            return {(f['origin_id'], f['path']): f for f in m['files']}
        require({k: f['sha256'] for k, f in retained_files('a1').items()} ==
                {k: f['sha256'] for k, f in retained_files('b1').items()} ==
                {k: f['sha256'] for k, f in retained_files('b2').items()}, 'fork retained bytes differ')
        checks.append('identical-fork-commit-and-all-retained-file-bytes')
        catalog = {'schema_version': 1, 'projects': [{'id': 'project-a', 'name': 'same-name'},
            {'id': 'project-b', 'name': 'same-name', 'upstream_project_id': 'project-a'}],
            'versions': [{'coordinates': coords[k], 'version_dir': str(v), 'origins': ['repo']} for k, v in versions.items()],
            'collections': [{'id': 'all', 'members': list(coords.values())},
                            {'id': 'a-only', 'members': [coords['a1'], coords['a2']]}]}
        write('catalog.json', catalog)
        top = yaml.safe_load((out / 'a2.yaml').read_text())
        top['collection'] = {'catalog_path': str(out / 'catalog.json'), 'member_configs': profiles}
        (out / 'collection.yaml').write_text(yaml.safe_dump(top, sort_keys=False))
        rejected = []
        for name in ('duplicate-project', 'duplicate-version', 'shared-version-path', 'fork-cycle', 'wrong-member-model', 'wrong-member-dimension'):
            bad, parent = copy.deepcopy(catalog), copy.deepcopy(top)
            if name == 'duplicate-project':
                bad['projects'].append(copy.deepcopy(bad['projects'][0]))
            elif name == 'duplicate-version':
                bad['versions'].append(copy.deepcopy(bad['versions'][0]))
            elif name == 'shared-version-path':
                bad['versions'][1]['version_dir'] = bad['versions'][0]['version_dir']
            elif name == 'fork-cycle':
                bad['projects'][0]['upstream_project_id'] = 'project-b'
            else:
                member = yaml.safe_load((out / 'b1.yaml').read_text())
                if name == 'wrong-member-model':
                    member['backends']['ckv']['embed_model'] = models['b2']
                else:
                    member['backends']['ckv']['embed_dim'] = 128
                path = out / ('rejected-member-' + name + '.yaml')
                path.write_text(yaml.safe_dump(member, sort_keys=False))
                parent['collection']['member_configs'][1] = str(path)
            write('rejected-catalog-' + name + '.json', bad)
            parent['collection']['catalog_path'] = str(out / ('rejected-catalog-' + name + '.json'))
            cfg = out / ('rejected-' + name + '.yaml')
            cfg.write_text(yaml.safe_dump(parent, sort_keys=False))
            r = subprocess.run([str(binary), 'mcp', '--config', str(cfg)], input=b'', env=env, capture_output=True, timeout=45)
            (out / ('rejected-' + name + '.stderr.txt')).write_bytes(r.stderr)
            (out / ('rejected-' + name + '.stdout.txt')).write_bytes(r.stdout)
            require(r.returncode == 1 and not r.stdout and b'"code":"config_invalid"' in r.stderr, 'bad catalog/profile served: ' + name)
            rejected.append({'case': name, 'exit_code': r.returncode, 'stdout_empty': not r.stdout})
            checks.append('startup-rejected:' + name)
        write('startup-rejections.json', rejected)
        libspec = importlib.util.spec_from_file_location('owned_mcp_client', ROOT / 'scripts/eval-value-dependencies.py')
        lib = importlib.util.module_from_spec(libspec)
        libspec.loader.exec_module(lib)
        def connect(label):
            folder = out / label
            folder.mkdir()
            client = lib.Client(None, None, folder, command=[str(binary), 'mcp', '--config', str(out / 'collection.yaml')])
            client.request('initialize', {'protocolVersion': '2024-11-05', 'capabilities': {},
                'clientInfo': {'name': 'owned-isolation-test', 'version': '1'}})
            tools = client.request('tools/list')['result']['tools']
            write(label + '/tools.json', tools)
            require(len(tools) == 35, 'public tool inventory changed')
            return client
        def close(label, client):
            client.close()
            sessions.append({'session': label, 'exit_code': client.proc.returncode, 'clean_EOF': True})
            write('sessions.json', sessions)
        def tool(client, name, arguments, error=None):
            start = len(http_rows)
            reply = client.request('tools/call', {'name': name, 'arguments': arguments})
            require('error' not in reply, 'protocol error')
            r = reply['result']
            results.append({'session': client.root.name, 'tool': name, 'arguments': arguments,
                'response': r, 'http_start': start, 'http_end': len(http_rows)})
            write('results.json', results)
            if error:
                require(r.get('isError') and error in json.dumps(r) and not r.get('structuredContent', {}).get('groups')
                        and not r.get('structuredContent', {}).get('job_id'), 'wrong rejection: ' + error)
            else:
                require(not r.get('isError'), 'unexpected tool error: ' + name)
            return r.get('structuredContent') or r
        def request(status, key):
            return {'catalog_digest': status['catalog_digest'], 'collection_id': 'all', 'mode': 'implementation',
                    'query': 'SharedConsensus', 'target': copy.deepcopy(coords[key])}
        def verify(value, keys):
            require(len(value['groups']) == len(keys), 'lost or extra groups')
            for group, key in zip(value['groups'], keys, strict=True):
                pack = group['evidence']
                require(group['coordinates'] == pack['coordinates'] == coords[key], 'cross-target result')
                copy_pack = copy.deepcopy(pack)
                integrity = copy_pack['metadata'].pop('integrity_hash')
                require(sha(json.dumps(copy_pack, sort_keys=True, ensure_ascii=False, separators=(',', ':')).encode()) == integrity,
                        'invalid integrity')
                require(pack['citations'] and pack['bodies'], 'no actual evidence')
                files = retained_files(key)
                for citation in pack['citations']:
                    require(all(citation[k] == v for k, v in coords[key].items()), 'foreign citation')
                    row = files[(citation['origin_id'], citation['file'])]
                    data = (versions[key] / 'sources/blobs' / row['sha256']).read_bytes()
                    require(sha(data) == citation['file_sha256'] == row['sha256'], 'wrong file')
                    span = b''.join(data.splitlines(keepends=True)[citation['start_line']-1:citation['end_line']])
                    require(sha(span) == citation['content_sha256'], 'wrong span')
                require(all(b['citation'] in pack['citations'] and sha(b['text'].encode()) == b['citation']['content_sha256'] for b in pack['bodies']),
                        'wrong body')
                require(str(markers[key]) in '\n'.join(b['text'] for b in pack['bodies']), 'wrong version marker')
        def read(client, status, key, label):
            verify(tool(client, 'cks.collection.get_context', request(status, key)), [key])
            row = results[-1]
            called = [h['request']['model'] for h in http_rows[row['http_start']:row['http_end']] if h['path'] == '/api/embed']
            require((called and set(called) == {models[key]}) if models[key] else not called, 'foreign embedder called')
            checks.append(label)
        preserved = {str(v / name): sha((v / name).read_bytes()) for v in versions.values()
                     for name in ('graph/graph.db', 'vector/vector.db', 'graph/manifest.json', 'vector/manifest.json',
                                  'dataset-identity.json', 'sources/manifest.json')}
        pointers = {str(v.parent / 'current'): os.readlink(v.parent / 'current') for v in versions.values()}
        client = connect('public')
        try:
            status = tool(client, 'cks.collection.status', {})
            require(status['member_count'] == 4 and status['write_enabled'] is False, 'wrong membership')
            for n, key in enumerate(('a1', 'b1', 'b2', 'a2', 'b1', 'a1', 'b2', 'a2')):
                read(client, status, key, 'warm-mixed-read:' + str(n) + ':' + key)
            comparison = {'catalog_digest': status['catalog_digest'], 'collection_id': 'all', 'mode': 'comparison',
                          'query': 'SharedConsensus', 'participants': [coords[k] for k in ('a1', 'b1', 'b2', 'a2')]}
            verify(tool(client, 'cks.collection.get_context', comparison), ['a1', 'b1', 'b2', 'a2'])
            checks.append('identical-forks-and-embedding-versions-not-deduplicated')
            for name, edit, error in [
                ('wrong-subcollection', lambda q: q.update(collection_id='a-only', target=coords['b1']), 'scope_mismatch'),
                ('other-project-same-commit', lambda q: q['target'].update(project_id='project-b'), 'scope_mismatch'),
                ('duplicate-participant', lambda q: q.update(mode='comparison', target=None, participants=[coords['a1'], coords['a1']]), 'scope_mismatch')]:
                q = request(status, 'a1')
                edit(q)
                tool(client, 'cks.collection.get_context', q, error)
                require(results[-1]['http_start'] == results[-1]['http_end'], 'invalid target dispatched')
                checks.append('request-rejected:' + name)
            original_digest = digests[models['b1']]
            try:
                digests[models['b1']] = 'e' * 64
                tool(client, 'cks.collection.get_context', request(status, 'b1'), 'fetch_failed')
                checks.append('runtime-model-digest-drift-refused')
                read(client, status, 'a1', 'other-project-works-during-model-drift')
                read(client, status, 'b2', 'same-project-other-model-works-during-model-drift')
            finally:
                digests[models['b1']] = original_digest
            # The existing Real CKV latches modelDown after a failed query.
            # Restoring the tag must not silently certify the old connection.
            tool(client, 'cks.collection.get_context', request(status, 'b1'), 'fetch_failed')
            checks.append('restored-model-stays-refused-until-reader-restart')
            def tree():
                rows = {}
                for dataset in (repo_a.parent/'dataset', repo_b.parent/'dataset'):
                    for p in sorted(dataset.rglob('*')):
                        name = p.relative_to(out).as_posix()
                        if p.is_symlink():
                            rows[name] = {'symlink': os.readlink(p)}
                        elif p.is_file():
                            rows[name] = {'sha256': sha(p.read_bytes()), 'mode': p.stat().st_mode & 0o777}
                        else:
                            rows[name] = {'directory': True}
                return rows
            before = tree()
            write('writer-tree-before.json', before)
            for mode in ('full', 'incremental'):
                tool(client, 'cks.ops.index', {'mode': mode}, 'versioned_setup_required')
                require(tree() == before, 'index rejection mutated dataset')
                checks.append('writer-rejected:index-' + mode)
            for project, repo in [('project-a', repo_a), ('project-b', repo_b)]:
                dataset = repo.parent / 'dataset'
                for target, path in [('root', dataset), ('version', dataset/'v1'), ('current', dataset/'current'),
                                     ('new-child', dataset/'current'/'unexpected-child')]:
                    for name in ('setup', 'reindex'):
                        tool(client, 'cks.ops.' + name, {'src': str(repo_a), 'out': str(path), 'version': 'unexpected-version',
                            'embedder': 'mock', 'model_name': 'mock'}, 'versioned_setup_required')
                        require(tree() == before, 'writer rejection mutated dataset')
                        checks.append('writer-rejected:' + name + ':' + project + ':' + target)
            write('writer-tree-after.json', tree())
            tool(client, 'cks.ops.setup_status', {'job_id': 'never-started', 'tail': 10}, 'unknown job_id')
            checks.append('no-async-job-created')
            for project in ('project-a', 'project-b'):
                plan = json.loads(run('gc-' + project, [binary, 'gc', '--out', out/project/'dataset', '--keep-recent', '2', '--min-age', '0s']))
                require(len(plan['versions']) == 2 and all(not v['delete'] and 'pinned_reader' in v['protected_reasons'] for v in plan['versions']),
                        'reader lease not protected in real GC plan')
                checks.append('GC-protects-all-pins:' + project)
            run('rollback-a-old', [binary, 'rollback', 'v1', '--out', repo_a.parent/'dataset'])
            require(os.readlink(repo_a.parent/'dataset/current') == 'v1', 'rollback did not switch current')
            read(client, status, 'a2', 'active-new-pin-ignores-rollback-to-old')
            read(client, status, 'a1', 'active-old-pin-after-rollback')
        finally:
            close('public', client)
        reordered = copy.deepcopy(catalog)
        reordered['projects'].reverse()
        reordered['versions'].reverse()
        for c in reordered['collections']:
            c['members'].reverse()
        write('catalog.json', reordered)
        client = connect('restarted')
        try:
            again = tool(client, 'cks.collection.status', {})
            require(again['catalog_digest'] == status['catalog_digest'], 'declaration order changed digest')
            read(client, again, 'a2', 'restart-reordered-catalog-keeps-new-pin-while-current-old')
            read(client, again, 'b1', 'restart-identical-fork-keeps-model96')
            read(client, again, 'b2', 'restart-same-source-keeps-model128')
        finally:
            close('restarted', client)
        changed = copy.deepcopy(catalog)
        changed['collections'][0]['members'].remove(coords['b1'])
        write('catalog.json', changed)
        client = connect('membership-changed')
        try:
            new_status = tool(client, 'cks.collection.status', {})
            require(new_status['catalog_digest'] != status['catalog_digest'], 'membership did not change pin')
            tool(client, 'cks.collection.get_context', request(status, 'a2'), 'stale_catalog')
            checks.append('membership-change-invalidates-old-catalog-pin')
            tool(client, 'cks.collection.get_context', request(new_status, 'b1'), 'scope_mismatch')
            checks.append('removed-member-refused-after-restart')
            read(client, new_status, 'a2', 'remaining-member-works-after-membership-change')
        finally:
            close('membership-changed', client)
        write('catalog.json', catalog)
        run('rollback-a-restore-new', [binary, 'rollback', 'v2', '--out', repo_a.parent/'dataset'])
        require(all(sha(Path(p).read_bytes()) == h for p, h in preserved.items()), 'database/manifest changed')
        require(all(os.readlink(p) == v for p, v in pointers.items()), 'final pointer not restored')
        write('preservation.json', {'files': preserved, 'current_pointers': pointers, 'unchanged': True,
            'intentional_rollback_project': 'project-a', 'other_project_current_unchanged': True})
        write('summary.json', {'scope': 'KM01_final_owned_real_indexes_structural_protocol_verification', 'checks': checks,
            'checks_passed': len(checks), 'tool_calls': len(results), 'datasets_built': 4, 'sessions_clean_EOF': len(sessions),
            'embedding_model_quality_evaluated': False, 'paid_model_calls': 0, 'independent_acceptance': False,
            'production_activated': False, 'versioned_DB_writes_added': False,
            'model_responder': 'loopback_deterministic_protocol_fixture', 'same_fork_commit': original_commit,
            'database_and_pointer_preserved': True})
        print('final isolation:', len(checks), 'checks;', len(results), 'tool calls;', len(sessions), 'clean EOF sessions', flush=True)
    finally:
        server.shutdown()
        server.server_close()
        worker.join(timeout=2)
        http_file.close()


if __name__ == '__main__':
    main()
