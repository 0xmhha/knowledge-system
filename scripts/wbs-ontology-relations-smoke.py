#!/usr/bin/env python3
"""Exercise all four opt-in ontology paths on synthetic, explicitly scoped facts.

No official B0/B1 gold, claim approval or real-model quality score is produced.
"""
import argparse
from contextlib import closing
import hashlib
import json
from pathlib import Path
import shutil
import sqlite3
import subprocess


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--out', required=True, help='new directory; existing input is refused')
    parser.add_argument('--embedder', choices=['mock', 'ollama'], default='mock')
    parser.add_argument('--model-name', default='bge-m3:latest')
    parser.add_argument('--ollama-url', default='http://127.0.0.1:11434')
    args = parser.parse_args()
    root = Path(__file__).resolve().parent.parent
    out = Path(args.out).resolve()
    out.mkdir(parents=True, exist_ok=False)
    source = out / 'src'
    source.mkdir()
    for name in ['go.mod', 'main.go', 'main_test.go', 'README.md', 'ontology-reviewed.yaml']:
        shutil.copy2(root / 'testdata/wbs-smoke' / name, source / name)
    records = []

    def run(label, command, expected=0):
        result = subprocess.run([str(v) for v in command], cwd=root, capture_output=True, timeout=120)
        log = out / (label + '.txt')
        log.write_bytes(result.stdout + result.stderr)
        records.append({'id': label, 'command': [str(v) for v in command], 'exit_code': result.returncode,
                        'raw_output': log.name, 'sha256': hashlib.sha256(log.read_bytes()).hexdigest()})
        if result.returncode != expected:
            raise RuntimeError(f'{label} failed: {result.returncode}; see {log}')

    run('git-init', ['git', '-C', source, 'init', '-q'])
    run('git-add', ['git', '-C', source, 'add', '.'])
    run('git-commit', ['git', '-C', source, '-c', 'commit.gpgsign=false', '-c', 'user.name=Fixture',
                       '-c', 'user.email=fixture@example.invalid', 'commit', '-qm', 'synthetic relations input'])
    run('git-bundle', ['git', '-C', source, 'bundle', 'create', out / 'source.bundle', '--all'])
    cks = root / 'bin/cks'
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
                            '--out', projection, '--docs', 'README.md', '--ontology', 'ontology-reviewed.yaml', '--extract-only'])
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
    for arm, mode, selected in [('baseline', 'baseline', store), ('relations', 'relations', store),
                                 ('concept_text', 'concept_text', store), ('combined', 'combined', store),
                                 ('missing', 'relations', out / 'missing.db'), ('stale', 'relations', stale),
                                 ('text_missing', 'concept_text', out / 'missing.db'), ('text_stale', 'concept_text', stale),
                                 ('combined_missing', 'combined', out / 'missing.db'), ('combined_stale', 'combined', stale)]:
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
        assert report['state'] == 'captured' and len(report['rows']) == 2
        assert report['binary_sha256'] == report['binary_sha256_after']
        assert report['config_sha256'] == report['config_sha256_after']
        packs[arm] = {}
        states[arm] = {}
        for row in report['rows']:
            response = row['call']['response']
            assert not response.get('isError', False)
            pack = response.get('structuredContent')
            if pack is None:
                pack = json.loads(response['content'][0]['text'])
            packs[arm][row['request_id']] = pack
            diagnostic = pack['metadata'].get('ontology')
            states[arm][row['request_id']] = diagnostic
            if arm == 'baseline':
                assert diagnostic is None
            else:
                want = 'unavailable' if arm.endswith('missing') else 'stale' if arm.endswith('stale') else 'active'
                assert diagnostic['mode'] == mode and diagnostic['state'] == want, diagnostic
                if arm in ['relations', 'combined']:
                    assert diagnostic['applied_relations'] == 1 and diagnostic['boosted_citations'] >= 1, diagnostic
                elif arm == 'concept_text':
                    assert diagnostic['applied_relations'] == 0 and diagnostic['text_boosted_citations'] >= 1, diagnostic
                else:
                    assert diagnostic['applied_relations'] == 0 and diagnostic['boosted_citations'] == 0, diagnostic
            if arm in ['concept_text', 'combined']:
                assert diagnostic['text_search_calls'] == 1, diagnostic
                source_proof = diagnostic['text_sources'][0]
                assert source_proof['concept_id'] == 'alpha-function'
                assert source_proof['dataset_id'] == identity['dataset_id']
                assert source_proof['snapshot_id'] == identity['source']['snapshot_id']
                src = (source / source_proof['file']).read_bytes()
                selected_lines = b''.join(src.splitlines(keepends=True)[source_proof['start_line'] - 1:source_proof['end_line']])
                assert hashlib.sha256(selected_lines).hexdigest() == source_proof['content_sha256']
                assert source_proof['returned_hits'] > 0
            if row['request_id'] == 'v2':
                assert pack['coordinates']['dataset_id'] == identity['dataset_id']
                clone = json.loads(json.dumps(pack))
                want_hash = clone['metadata'].pop('integrity_hash')
                canonical_json = json.dumps(clone, sort_keys=True, ensure_ascii=False, separators=(',', ':')).encode()
                assert hashlib.sha256(canonical_json).hexdigest() == want_hash
    for api in ['v1', 'v2']:
        for arm in ['missing', 'stale', 'text_missing', 'text_stale', 'combined_missing', 'combined_stale']:
            assert packs[arm][api]['citations'] == packs['baseline'][api]['citations']
            assert packs[arm][api]['bodies'] == packs['baseline'][api]['bodies']
        for arm in ['relations', 'concept_text', 'combined']:
            assert {json.dumps(c, sort_keys=True) for c in packs[arm][api]['citations']} == {json.dumps(c, sort_keys=True) for c in packs['baseline'][api]['citations']}
    assert hashlib.sha256(store.read_bytes()).hexdigest() == before_store
    assert not (out / 'missing.db').exists()
    summary = {'state': 'verified', 'quality_metrics': None, 'scope': 'synthetic structural integration; not official B1',
               'embedder': args.embedder, 'embedding_identity': identity['embedding_identity'],
               'coordinates': identity, 'semantic_db_unchanged': True, 'semantic_current_not_required': True,
               'states': states, 'checks': records}
    (out / 'summary.json').write_text(json.dumps(summary, indent=2) + '\n')
    print('four-arm ontology runtime smoke: verified; ' + str(out))


if __name__ == '__main__':
    main()
