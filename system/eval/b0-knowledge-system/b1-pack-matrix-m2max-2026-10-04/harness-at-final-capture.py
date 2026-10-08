#!/usr/bin/env python3
"""Exercise all four opt-in ontology paths on synthetic, explicitly scoped facts.

No official B0/B1 gold, claim approval or real-model quality score is produced.
"""
import argparse
from contextlib import closing
import hashlib
import json
import os
import re
from pathlib import Path
import shutil
import sqlite3
import subprocess


def prepare_pack(source, cks, run, out):
    """Synthetic review statuses exercise runtime branches, not human approval."""
    run('knowledge-init', [cks, 'knowledge', 'init', '--project-root', source, '--project-id', 'ks-fixture'])
    pack = source / 'vendor/fixture-policy'
    pack.mkdir(parents=True)
    (pack / 'pack.yaml').write_text('''pack_schema_version: 1
pack_id: fixture.policy
version: 1.0.0
owner: project
scope: synthetic-test
requires: []
concepts:
  - {id: business-policy, kind: rule, definition: A synthetic test policy.}
relation_types: []
constraints: []
competency_questions: ["Which synthetic policy applies?"]
''')
    run('knowledge-digest', [cks, 'knowledge', 'digest', '--project-root', source, '--source', 'vendor/fixture-policy'])
    digest = json.loads((out / 'knowledge-digest.txt').read_text())['sha256']
    manifest = source / '.cks/knowledge/manifest.yaml'
    text = manifest.read_text()
    assert 'selected_packs: []' in text
    manifest.write_text(text.replace('selected_packs: []', 'selected_packs:\n  - pack_id: fixture.policy\n    version: 1.0.0\n    source: ./vendor/fixture-policy\n    sha256: ' + digest))
    for name, scope, status, visibility, expired, conflicts in [
        ('public', 'alpha', 'verified', 'public', False, []),
        ('proposed', 'proposed', 'proposed', 'public', False, []),
        ('expired', 'expired', 'verified', 'public', True, []),
        ('restricted', 'restricted', 'verified', 'restricted', False, []),
        ('conflict-a', 'conflict', 'verified', 'public', False, ['conflict-b']),
        ('conflict-b', 'conflict', 'verified', 'public', False, ['conflict-a']),
    ]:
        policy = source / '.cks/knowledge/policies' / (name + '.yaml')
        policy.write_text(f'''id: {name}
type: {{pack_id: fixture.policy, local_id: business-policy}}
statement: Synthetic {name} policy for runtime integration only.
owner: fixture-owner
scope: {{subsystem: {scope}}}
effective_from: "2026-01-01"
''' + ('effective_to: "2026-02-01"\n' if expired else '') + f'''status: {status}
''' + ('''reviewed_by: fixture-reviewer
review_reason: Synthetic branch coverage; not an operating fact approval.
''' if status == 'verified' else '') + f'''visibility: {visibility}
conflicts_with: {json.dumps(conflicts)}
source_ref: {{origin_id: repo, path: .cks/knowledge/policies/{name}.yaml}}
''')
    run('knowledge-lock', [cks, 'knowledge', 'lock', '--project-root', source])
    run('knowledge-validate', [cks, 'knowledge', 'validate', '--project-root', source])
    run('knowledge-review', [cks, 'knowledge', 'review', '--project-root', source])


def matrix_requests(enabled):
    return {'schema_version': 1, 'requests': [
        {'id': scope, 'tool': 'cks.context.get_for_task_v2', 'arguments': {
            'prompt': 'Alpha function implementation', 'include_knowledge': enabled,
            'knowledge_as_of': '2026-10-03', 'knowledge_subsystem': scope}}
        for scope in ['alpha', 'wrong-scope', 'proposed', 'expired', 'restricted', 'conflict']]}


def verify_v2_sources(pack, source, identity):
    assert pack['coordinates']['dataset_id'] == identity['dataset_id']
    for citation in pack['citations']:
        for field, expected in [('project_id', identity['source']['project_id']),
                                ('dataset_id', identity['dataset_id']),
                                ('snapshot_id', identity['source']['snapshot_id']),
                                ('commit_hash', identity['source']['source_commit'])]:
            assert citation[field] == expected
        assert citation['origin_id'] == 'repo'
        raw = (source / citation['file']).read_bytes()
        lines = raw.splitlines(keepends=True)
        assert 1 <= citation['start_line'] <= citation['end_line'] <= len(lines)
        span = b''.join(lines[citation['start_line'] - 1:citation['end_line']])
        assert hashlib.sha256(raw).hexdigest() == citation['file_sha256']
        assert hashlib.sha256(span).hexdigest() == citation['content_sha256']
    for body in pack['bodies']:
        assert body['citation'] in pack['citations']
        assert hashlib.sha256(body['text'].encode()).hexdigest() == body['citation']['content_sha256']


def verify_knowledge(pack, enabled, scope):
    semantic = pack.get('semantic') or {}
    if not enabled:
        assert 'knowledge_context' not in semantic
        assert all(not c['file'].startswith('.cks/knowledge/') for c in pack['citations'])
        return None
    context = semantic['knowledge_context']
    coding = semantic['coding_context']
    expected = {'alpha': ['complete', 'partial'], 'wrong-scope': ['unknown'], 'proposed': ['unknown'],
                'expired': ['stale'], 'restricted': ['restricted'], 'conflict': ['conflict']}[scope]
    assert context['state'] in expected, context
    ids = [p['id'] for p in context['applicable_policies']]
    assert ids == ({'alpha': ['public'], 'conflict': ['conflict-a', 'conflict-b']}.get(scope, [])), context
    required = [p['id'] for p in coding['required_behavior']]
    assert required == (['public'] if scope == 'alpha' else []), coding
    for field in ['decisions', 'relations', 'related_requirements', 'test_links', 'trace_links']:
        assert not context.get(field, []), context
    for field in ['implemented_behavior', 'rationale', 'constraints']:
        assert not coding.get(field, []), coding
    assert bool(context['conflicts']) == (scope == 'conflict')
    expected_sources = {'.cks/knowledge/policies/' + name + '.yaml' for name in ids}
    assert {c['file'] for c in pack['citations'] if c['file'].startswith('.cks/knowledge/')} == expected_sources
    return {'state': context['state'], 'policy_ids': ids, 'required_behavior_ids': required,
            'lock_digest': context['lock_digest']}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--out', required=True, help='new directory; existing input is refused')
    parser.add_argument('--embedder', choices=['mock', 'ollama'], default='mock')
    parser.add_argument('--model-name', default='bge-m3:latest')
    parser.add_argument('--ollama-url', default='http://127.0.0.1:11434')
    parser.add_argument('--pack-matrix', action='store_true', help='eight v2 arms with synthetic policy safety controls')
    args = parser.parse_args()
    root = Path(__file__).resolve().parent.parent
    # These are compiled defaults, not telemetry. Pin their source contract
    # so a changed runtime cannot silently reuse a mislabeled experiment.
    settings_sources = {}
    for file, pattern in [
        ('internal/system/composer/stage1/extractor.go', r'DefaultInitialK\s*=\s*20\b'),
        ('internal/system/composer/stage2/searcher.go', r'DefaultMaxCitations\s*=\s*30\b'),
        ('cmd/cks/mcpcli/ontology.go', r'WithOntologyTextSearch\(ckv, stage1.DefaultInitialK,'),
        ('cmd/cks/mcpcli/serve.go', r'stage1.New\(ckv, ckg, stage1Opts\.\.\.\)'),
    ]:
        raw = (root / file).read_bytes()
        assert re.search(pattern, raw.decode()), 'runtime settings contract changed: ' + file
        if file.endswith('stage1/extractor.go'):
            assert re.search(r'KnowledgeK:\s*6,', raw.decode()), 'knowledge pass default changed'
        settings_sources[file] = hashlib.sha256(raw).hexdigest()
    out = Path(args.out).resolve()
    out.mkdir(parents=True, exist_ok=False)
    source = out / 'src'
    source.mkdir()
    for name in ['go.mod', 'main.go', 'main_test.go', 'README.md', 'ontology-reviewed.yaml']:
        shutil.copy2(root / 'testdata/wbs-smoke' / name, source / name)
    records = []
    cks = root / 'bin/cks'
    binary_sha = hashlib.sha256(cks.read_bytes()).hexdigest()

    def run(label, command, expected=0):
        env = {k: v for k, v in os.environ.items() if not k.startswith('GIT_')}
        env.update(GIT_AUTHOR_DATE='2026-10-03T00:00:00Z', GIT_COMMITTER_DATE='2026-10-03T00:00:00Z',
                   CKV_REQUIRE_COMPLETE_EMBEDDINGS='1')
        result = subprocess.run([str(v) for v in command], cwd=root, env=env, capture_output=True, timeout=120)
        log = out / (label + '.txt')
        log.write_bytes(result.stdout + result.stderr)
        records.append({'id': label, 'command': [str(v) for v in command], 'exit_code': result.returncode,
                        'raw_output': log.name, 'sha256': hashlib.sha256(log.read_bytes()).hexdigest()})
        (out / 'checks.json').write_text(json.dumps(records, indent=2) + '\n')
        if result.returncode != expected:
            raise RuntimeError(f'{label} failed: {result.returncode}; see {log}')

    if args.pack_matrix:
        prepare_pack(source, cks, run, out)
    run('git-init', ['git', '-C', source, 'init', '-q'])
    run('git-add', ['git', '-C', source, 'add', '.'])
    run('git-commit', ['git', '-C', source, '-c', 'commit.gpgsign=false', '-c', 'user.name=Fixture',
                       '-c', 'user.email=fixture@example.invalid', 'commit', '-qm', 'synthetic relations input'])
    run('git-bundle', ['git', '-C', source, 'bundle', 'create', out / 'source.bundle', '--all'])
    run('git-bundle-verify', ['git', '-C', source, 'bundle', 'verify', out / 'source.bundle'])
    input_hashes = {str(f.relative_to(source)): hashlib.sha256(f.read_bytes()).hexdigest()
                    for f in source.rglob('*') if f.is_file() and '.git' not in f.relative_to(source).parts}
    dataset = out / 'dataset'
    setup_args = [cks, 'setup', '--src', source, '--out', dataset, '--project-id', 'ks-fixture',
                  '--source-mode', 'committed', '--version', 'smoke', '--embedder', args.embedder]
    if args.embedder == 'ollama':
        setup_args.extend(['--model-name', args.model_name, '--ollama-url', args.ollama_url])
    run('setup', setup_args)
    version = (dataset / 'current').resolve()
    identity = json.loads((version / 'dataset-identity.json').read_text())
    vector_manifest = json.loads((version / 'vector/manifest.json').read_text())
    store = out / 'semantic.db'
    projection = out / 'projection.json'
    run('semantic-extract', [cks, 'semantic', 'build', '--repo', source, '--project-id', 'ks-fixture',
                            '--dataset-id', identity['dataset_id'], '--graph', version / 'graph',
                            '--vector', version / 'vector', '--version-dir', version, '--store', store,
                            '--out', projection, '--docs', 'README.md', '--ontology', 'ontology-reviewed.yaml', '--extract-only']
                            + (['--include-packs'] if args.pack_matrix else []))
    p = json.loads(projection.read_text())
    with closing(sqlite3.connect(version / 'graph/graph.db')) as db:
        canonical, file, start, end = db.execute("SELECT canonical_id,file_path,start_line,end_line FROM nodes WHERE type='Function' AND name='Alpha'").fetchone()
    raw = (source / file).read_bytes()
    span = b''.join(raw.splitlines(keepends=True)[start - 1:end])
    p['evidence'].append({'id': 'fixture:code-alpha', 'snapshot': p['snapshot'], 'kind': 'code',
                          'path': file, 'start_line': start, 'end_line': end,
                          'content_sha256': hashlib.sha256(span).hexdigest(), 'canonical_id': canonical,
                          'extractor': 'synthetic-relations-smoke-v1'})
    concept = next(c for c in p['concepts'] if c['id'] == 'alpha-function')
    p['assertions'] = [{'id': 'fixture:alpha-implementation', 'predicate': 'IMPLEMENTED_BY',
                        'subject_id': concept['id'], 'object_id': canonical,
                        'evidence_ids': [concept['evidence_id'], 'fixture:code-alpha'],
                        'status': 'verified', 'reviewed_by': 'fixture-reviewer'}]
    projection.write_text(json.dumps(p, indent=2) + '\n')
    run('semantic-promote', [cks, 'semantic', 'promote', '--input', projection, '--repo', source,
                            '--graph', version / 'graph', '--vector', version / 'vector',
                            '--version-dir', version, '--store', store])
    # No current pointer is activated: retrieval must select the pinned tuple.
    with closing(sqlite3.connect(store)) as db:
        assert db.execute('SELECT count(*) FROM semantic_current').fetchone()[0] == 0
    before_store = hashlib.sha256(store.read_bytes()).hexdigest()
    stale = out / 'stale-semantic.db'
    shutil.copy2(store, stale)
    with closing(sqlite3.connect(stale)) as db:
        db.execute("UPDATE semantic_projections SET digest='damaged'")
        db.commit()
    requests = out / 'requests.json'
    requests.write_text(json.dumps({'schema_version': 1, 'requests': [
        {'id': 'v1', 'tool': 'cks.context.get_for_task', 'arguments': {'prompt': 'Alpha function implementation'}},
        {'id': 'v2', 'tool': 'cks.context.get_for_task_v2', 'arguments': {'prompt': 'Alpha function implementation'}}]}, indent=2) + '\n')
    packs = {}
    states = {}
    arms = [('baseline', 'baseline', store), ('relations', 'relations', store),
                                 ('concept_text', 'concept_text', store), ('combined', 'combined', store),
                                 ('missing', 'relations', out / 'missing.db'), ('stale', 'relations', stale),
                                 ('text_missing', 'concept_text', out / 'missing.db'), ('text_stale', 'concept_text', stale),
                                 ('combined_missing', 'combined', out / 'missing.db'), ('combined_stale', 'combined', stale)]
    if args.pack_matrix:
        arms = [(mode + '_' + axis, mode, store) for mode, axis in [
            ('baseline', 'off'), ('concept_text', 'on'), ('relations', 'off'), ('combined', 'on'),
            ('baseline', 'on'), ('concept_text', 'off'), ('relations', 'on'), ('combined', 'off')]]
    locked_files = [version / 'dataset-identity.json', version / 'vector/manifest.json',
                    version / 'vector/vectors.db', version / 'graph/graph.db', store]
    # CKV's SQLite filename is backend-owned, not assumed by the manifest.
    locked_files = [f for f in locked_files if f.exists()]
    locked_files.extend(f for f in (version / 'vector').glob('*.db') if f not in locked_files)
    locked_sha = {str(f): hashlib.sha256(f.read_bytes()).hexdigest() for f in locked_files}
    matrix = {'schema_version': 1, 'diagnostic_only': True, 'quality_metrics': None,
              'binary_sha256': binary_sha, 'coordinates': identity, 'embedding_identity': identity['embedding_identity'],
              'settings': {'raw_recall_k': 20, 'stage2_candidate_cap': 30, 'knowledge_pass_k': 6,
                           'raw_search_filter': {}, 'bm25_rerank': True, 'ontology_budget_ms': 5000},
              'settings_evidence': 'compiled default source contract; not per-call backend telemetry',
              'settings_sources_sha256': settings_sources, 'input_file_sha256': input_hashes,
              'sizes': {'input_files': len(input_hashes),
                        'input_bytes': sum((source / name).stat().st_size for name in input_hashes),
                        'graph_db_bytes': (version / 'graph/graph.db').stat().st_size,
                        'vector_db_bytes': sum(f.stat().st_size for f in (version / 'vector').glob('*.db')),
                        'semantic_db_bytes': store.stat().st_size,
                        'projection_json_bytes': projection.stat().st_size},
              'pack_axis': 'same retained pack and index; only include_knowledge changes',
              'order': [a[0] for a in arms], 'arm_rotation': False,
              'latency_gate': 'not measured; sequential structural single captures', 'arms': []}
    (out / 'matrix-manifest.json').write_text(json.dumps(matrix, indent=2) + '\n')
    for arm, mode, selected in arms:
        enabled = args.pack_matrix and arm.endswith('_on')
        if args.pack_matrix:
            requests = out / (arm + '-requests.json')
            requests.write_text(json.dumps(matrix_requests(enabled), indent=2) + '\n')
        config = out / (arm + '.yaml')
        run('config-' + arm, [cks, 'mcp', 'gen-config', '--dataset-dir', version, '--source-root', source,
                             '--embed-model', vector_manifest['embedding_model'], '--ollama-url', args.ollama_url,
                             '--semantic-store', selected, '--out', config])
        lines = []
        for line in config.read_text().splitlines():
            line = line.replace('provider: ""', 'provider: ' + args.embedder).replace('provider: ollama', 'provider: ' + args.embedder)
            line = line.replace('mcp_stdio: false', 'mcp_stdio: true').replace('transport: http', 'transport: stdio')
            lines.append(line)
            if line.lstrip().startswith('store_path:'):
                lines.extend(['    ontology_mode: ' + mode, '    ontology_budget_ms: 5000'])
        config.write_text('\n'.join(lines) + '\n')
        capture = out / (arm + '.json')
        run('capture-' + arm, [cks, 'eval', 'capture', '--requests', requests, '--config', config,
                              '--output', capture, '--warmup', '0', '--retrieval-runs', '1', '--warm-runs', '0', '--cold-runs', '0'])
        report = json.loads(capture.read_text())
        assert report['state'] == 'captured' and len(report['rows']) == (6 if args.pack_matrix else 2)
        assert report['binary_sha256'] == report['binary_sha256_after'] == binary_sha
        assert report['config_sha256'] == report['config_sha256_after']
        packs[arm] = {}
        states[arm] = {}
        observations = []
        for row in report['rows']:
            response = row['call']['response']
            assert not response.get('isError', False)
            pack = response.get('structuredContent')
            if pack is None:
                pack = json.loads(response['content'][0]['text'])
            packs[arm][row['request_id']] = pack
            diagnostic = pack['metadata'].get('ontology')
            states[arm][row['request_id']] = diagnostic
            if mode == 'baseline':
                assert diagnostic is None
            else:
                want = 'unavailable' if arm.endswith('missing') else 'stale' if arm.endswith('stale') else 'active'
                assert diagnostic['mode'] == mode and diagnostic['state'] == want, diagnostic
                if mode in ['relations', 'combined'] and not arm.endswith(('missing', 'stale')):
                    assert diagnostic['applied_relations'] == 1 and diagnostic['boosted_citations'] >= 1, diagnostic
                elif mode == 'concept_text' and not arm.endswith(('missing', 'stale')):
                    assert diagnostic['applied_relations'] == 0 and diagnostic['text_boosted_citations'] >= 1, diagnostic
                else:
                    assert diagnostic['applied_relations'] == 0 and diagnostic['boosted_citations'] == 0, diagnostic
            if mode in ['concept_text', 'combined'] and not arm.endswith(('missing', 'stale')):
                assert diagnostic['text_search_calls'] == 1, diagnostic
                source_proof = diagnostic['text_sources'][0]
                assert source_proof['concept_id'] == 'alpha-function'
                assert source_proof['dataset_id'] == identity['dataset_id']
                assert source_proof['snapshot_id'] == identity['source']['snapshot_id']
                src = (source / source_proof['file']).read_bytes()
                selected_lines = b''.join(src.splitlines(keepends=True)[source_proof['start_line'] - 1:source_proof['end_line']])
                assert hashlib.sha256(selected_lines).hexdigest() == source_proof['content_sha256']
                assert source_proof['returned_hits'] > 0
            if row['request_id'] == 'v2' or args.pack_matrix:
                assert pack['coordinates']['dataset_id'] == identity['dataset_id']
                clone = json.loads(json.dumps(pack))
                want_hash = clone['metadata'].pop('integrity_hash')
                canonical_json = json.dumps(clone, sort_keys=True, ensure_ascii=False, separators=(',', ':')).encode()
                assert hashlib.sha256(canonical_json).hexdigest() == want_hash
                verify_v2_sources(pack, source, identity)
            if args.pack_matrix:
                knowledge = verify_knowledge(pack, enabled, row['request_id'])
                observations.append({'request_id': row['request_id'], 'ontology': diagnostic, 'knowledge': knowledge,
                                     'elapsed_ns': row['call']['elapsed_ns'],
                                     'response_json_bytes': len(json.dumps(response, ensure_ascii=False, separators=(',', ':')).encode()),
                                     'citation_count': len(pack['citations']),
                                     'body_utf8_bytes': sum(len(b['text'].encode()) for b in pack['bodies'])})
        matrix['arms'].append({'id': arm, 'mode': mode, 'include_knowledge': enabled,
                               'config_sha256': report['config_sha256'], 'requests_sha256': report['request_sha256'],
                               'capture': capture.name, 'capture_sha256': hashlib.sha256(capture.read_bytes()).hexdigest(),
                               'observations': observations})
        (out / 'matrix-manifest.json').write_text(json.dumps(matrix, indent=2) + '\n')
    if args.pack_matrix:
        key = lambda c: json.dumps(c, sort_keys=True)
        configs = [(out / (a[0] + '.yaml')).read_text() for a in arms]
        assert len({re.sub(r'ontology_mode: \w+', 'ontology_mode: BASE', c) for c in configs}) == 1
        assert len({a['config_sha256'] for a in matrix['arms']}) == 4
        for scope in matrix_requests(False)['requests']:
            api = scope['id']
            for axis in ['off', 'on']:
                base = packs['baseline_' + axis][api]
                for mode in ['relations', 'concept_text', 'combined']:
                    pack = packs[mode + '_' + axis][api]
                    assert {key(c) for c in pack['citations']} == {key(c) for c in base['citations']}
                    assert {key(b) for b in pack['bodies']} == {key(b) for b in base['bodies']}
            for mode in ['baseline', 'relations', 'concept_text', 'combined']:
                off = packs[mode + '_off'][api]
                on = packs[mode + '_on'][api]
                assert {key(c) for c in off['citations']} <= {key(c) for c in on['citations']}
                assert {key(b) for b in off['bodies']} <= {key(b) for b in on['bodies']}
                if api in ['wrong-scope', 'proposed', 'expired', 'restricted']:
                    assert on['citations'] == off['citations'] and on['bodies'] == off['bodies']
        lock = json.loads((out / 'knowledge-lock.txt').read_text())['lock_digest']
        assert all(r['knowledge']['lock_digest'] == lock for a in matrix['arms'] if a['include_knowledge']
                   for r in a['observations'])
    for api in ([] if args.pack_matrix else ['v1', 'v2']):
        for arm in ['missing', 'stale', 'text_missing', 'text_stale', 'combined_missing', 'combined_stale']:
            assert packs[arm][api]['citations'] == packs['baseline'][api]['citations']
            assert packs[arm][api]['bodies'] == packs['baseline'][api]['bodies']
        for arm in ['relations', 'concept_text', 'combined']:
            assert {json.dumps(c, sort_keys=True) for c in packs[arm][api]['citations']} == {json.dumps(c, sort_keys=True) for c in packs['baseline'][api]['citations']}
    assert hashlib.sha256(store.read_bytes()).hexdigest() == before_store
    assert not (out / 'missing.db').exists()
    assert hashlib.sha256(cks.read_bytes()).hexdigest() == binary_sha
    assert all(hashlib.sha256(Path(f).read_bytes()).hexdigest() == sha for f, sha in locked_sha.items())
    assert input_hashes == {str(f.relative_to(source)): hashlib.sha256(f.read_bytes()).hexdigest()
                           for f in source.rglob('*') if f.is_file() and '.git' not in f.relative_to(source).parts}
    run('git-status-after', ['git', '-C', source, 'status', '--porcelain'])
    assert (out / 'git-status-after.txt').read_bytes() == b''
    summary = {'state': 'verified', 'quality_metrics': None, 'scope': 'synthetic structural integration; not official B1',
               'embedder': args.embedder, 'embedding_identity': identity['embedding_identity'],
               'coordinates': identity, 'semantic_db_unchanged': True, 'semantic_current_not_required': True,
               'states': states, 'checks': records, 'pack_matrix': args.pack_matrix,
               'requests': sum(len(p) for p in packs.values()), 'binary_sha256': binary_sha,
               'locked_files_sha256': locked_sha, 'input_file_sha256': input_hashes}
    (out / 'summary.json').write_text(json.dumps(summary, indent=2) + '\n')
    print(('eight-arm pack' if args.pack_matrix else 'four-arm ontology') + ' runtime smoke: verified; ' + str(out))


if __name__ == '__main__':
    main()
