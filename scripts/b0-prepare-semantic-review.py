#!/usr/bin/env python3
"""Prepare source-bound DEVELOPMENT fixture proposals; never approve or score.

The frozen source/query book is preserved. Supplemental vocabulary/spec/pack
files get a separate deterministic Git commit and dataset identity. All facts
and edges remain proposed; final fixtures and retrieval scoring are excluded.
"""
import argparse
from contextlib import closing
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import sqlite3
import subprocess

ROOT = Path(__file__).resolve().parent.parent
SPEC = importlib.util.spec_from_file_location('b0_materialize', ROOT / 'scripts/b0-materialize-fixtures.py')
MAT = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MAT)


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def json_source(value):
    # JSON is a YAML subset; no optional Python YAML dependency or escaping.
    return json.dumps(value, ensure_ascii=False, indent=2) + '\n'


def proposed_inputs(fixture, state):
    """Only explicit synthetic proposals, with no code-policy inference."""
    family, project = fixture['family'], state['project_id']
    if family == 'F-03':
        return {}
    policy_path = 'README.md' if family == 'F-06' else 'project-' + state['name'] + '/docs/policy.md'
    policy_statement = fixture['sources'][policy_path].splitlines()[2]
    concept_id = 'refund-cap' if family == 'F-06' else 'alpha-function'
    symbol = 'RefundLimit' if family == 'F-06' else 'Alpha'
    concept = {'id': concept_id, 'kind': 'process',
               'definition': 'The fixture refund limit operation, whose required cap and observed return value are separate facts.' if family == 'F-06' else 'The Alpha function returns the fixture project marker; it does not enforce a refund policy.',
               'includes': ['Source-backed function identity'], 'excludes': ['Automatic policy compliance or acceptance'],
               'terms': [{'lang': 'en', 'value': symbol, 'preferred': True},
                         {'lang': 'ko', 'value': '환급 상한' if family == 'F-06' else '프로젝트 식별 함수', 'preferred': True}],
               'status': 'proposed'}
    ontology = {'version': 1, 'project_id': project, 'domain': 'synthetic B0 development fixture',
                'competency_questions': [q['prompt'] for q in fixture['queries']], 'concepts': [concept]}
    pack = {'pack_schema_version': 1, 'pack_id': 'fixture.refund', 'version': '1.0.0',
            'owner': 'B0 synthetic fixture proposal', 'scope': 'fixture-local', 'requires': [],
            'concepts': [{'id': 'business-policy', 'kind': 'rule', 'definition': 'A project-scoped synthetic refund requirement; the type does not approve any instance.'}],
            'relation_types': [], 'constraints': [],
            'competency_questions': ['Which fixture-local refund cap applies?']}
    manifest = {'schema_version': 1, 'project_id': project, 'selected_packs': [],
                'overlay_root': '.cks/knowledge', 'review_policy': {'min_approvals': 1}}
    policy = {'id': 'refund-cap-policy', 'type': {'pack_id': 'fixture.refund', 'local_id': 'business-policy'},
              'statement': policy_statement,
              'owner': 'B0 synthetic fixture proposal', 'scope': {'subsystem': 'refund'},
              'effective_from': '2026-10-03', 'status': 'proposed', 'visibility': 'public',
              'source_ref': {'origin_id': 'repo', 'path': '.cks/knowledge/policies/refund-cap.yaml'}}
    files = {'ontology.yaml': json_source(ontology), 'fixture-packs/refund/pack.yaml': json_source(pack),
             '.cks/knowledge/manifest.yaml': json_source(manifest),
             '.cks/knowledge/policies/refund-cap.yaml': json_source(policy)}
    if family == 'F-06':
        req = {'id': 'req-refund-cap', 'version': 1, 'title': 'Documented fixture refund cap',
               'statement': 'RefundLimit must return a value no greater than 10.', 'status': 'proposed',
               'concept_ids': [concept_id], 'acceptance_criteria': [{'id': 'ac-refund-cap',
               'given': 'The fixture RefundLimit implementation', 'when': 'RefundLimit is invoked',
               'then': 'Its returned value is no greater than 10.'}]}
        files['spec.yaml'] = json_source({'version': 1, 'project_id': project, 'requirements': [req]})
    return files


def verify_projection(projection, repo):
    snapshot = projection['snapshot']
    for span in projection['evidence']:
        if span['snapshot'] != snapshot:
            raise ValueError('mixed evidence tuple')
        raw = MAT.git(repo, 'show', snapshot['commit'] + ':' + span['path'], raw=True)
        lines = raw.splitlines(keepends=True)
        start, end = span['start_line'], span['end_line']
        if not 1 <= start <= end <= len(lines):
            raise ValueError('invalid physical evidence lines')
        if hashlib.sha256(b''.join(lines[start - 1:end])).hexdigest() != span['content_sha256']:
            raise ValueError('evidence source bytes changed')
    for group in ['concepts', 'requirements', 'claims', 'assertions']:
        for item in projection.get(group, []) or []:
            if item['status'] != 'proposed' or item.get('reviewed_by'):
                raise ValueError('preparation must not approve a fact')
    if any(a['predicate'] in ['CHECKED_BY', 'TESTED_BY', 'ACCEPTED_BY'] for a in projection.get('assertions', []) or []):
        raise ValueError('unrelated test cannot become an acceptance proof')


def prepare(out, manifest, binary, embedder, model, ollama_url, partition='development'):
    if partition not in ['development', 'final']:
        raise ValueError('semantic source preparation partition must be development or final')
    if out.exists() or out.is_symlink():
        raise ValueError('output exists; preparation never overwrites inputs')
    out.mkdir(parents=True)
    binary_hash = sha(binary)
    input_hash = sha(manifest)
    records = MAT.materialize(manifest, out / 'fixtures', partition=partition, allow_draft=True)
    book = json.loads(manifest.read_bytes())
    proposals = {f['id']: f for f in book['fixtures'] if f['evaluation_partition'] == partition}
    checks, cases = [], []

    def run(label, args):
        env = {k: v for k, v in os.environ.items() if not k.startswith('GIT_')}
        if embedder == 'ollama':
            env['CKV_REQUIRE_COMPLETE_EMBEDDINGS'] = '1'
        result = subprocess.run([str(a) for a in args], cwd=ROOT, capture_output=True, timeout=300, env=env)
        path = out / (label + '.txt')
        path.write_bytes(result.stdout + result.stderr)
        checks.append({'id': label, 'command': [str(a) for a in args], 'exit_code': result.returncode,
                       'raw_output': path.name, 'sha256': sha(path)})
        (out / 'execution-checks.json').write_text(json_source({'state': 'preparing' if result.returncode == 0 else 'failed', 'checks': checks, 'quality_metrics': None}))
        if result.returncode:
            raise RuntimeError(f'{label} failed; original log: {path}')
        return result.stdout

    for fixture in records['fixtures']:
        if fixture['family'] not in ['F-03', 'F-05', 'F-06']:
            continue
        for state in fixture['states']:
            label = fixture['id'] + '-' + state['name']
            case = out / label
            case.mkdir()
            repo = out / 'fixtures' / state['repository']
            source = proposals[fixture['id']]
            supplement = proposed_inputs(source, state)
            if supplement:
                MAT.write_sources(repo, supplement)
                pack_digest = json.loads(run(label + '-pack-digest', [binary, 'knowledge', 'digest', '--project-root', repo, '--source', 'fixture-packs/refund']))['sha256']
                m = json.loads((repo / '.cks/knowledge/manifest.yaml').read_text())
                m['selected_packs'] = [{'pack_id': 'fixture.refund', 'version': '1.0.0', 'source': 'fixture-packs/refund', 'sha256': pack_digest}]
                (repo / '.cks/knowledge/manifest.yaml').write_text(json_source(m))
                run(label + '-knowledge-lock', [binary, 'knowledge', 'lock', '--project-root', repo])
                run(label + '-knowledge-validate', [binary, 'knowledge', 'validate', '--project-root', repo])
                run(label + '-knowledge-review', [binary, 'knowledge', 'review', '--project-root', repo])
                # Original frozen file bytes are checked before committing new
                # proposals; supplements never edit the frozen base sources.
                for path, expected in state['source_sha256'].items():
                    if sha(repo / path) != expected:
                        raise ValueError('supplement changed frozen source')
                MAT.git(repo, 'add', '--all')
                MAT.git(repo, 'commit', '--quiet', '-m', label + ' proposed source-bound supplement')
            augmented = MAT.git(repo, 'rev-parse', 'HEAD')
            if MAT.git(repo, 'status', '--porcelain'):
                raise ValueError('source is dirty before build')
            args = [binary, 'setup', '--src', repo, '--out', case / 'dataset', '--project-id', state['project_id'],
                    '--source-mode', 'committed', '--version', 'review', '--embedder', embedder]
            if embedder == 'ollama':
                args.extend(['--model-name', model, '--ollama-url', ollama_url])
            run(label + '-setup', args)
            version = (case / 'dataset/current').resolve()
            identity = json.loads((version / 'dataset-identity.json').read_text())
            assert identity['source']['source_commit'] == augmented
            (case / 'dataset-identity.json').write_text(json_source(identity))
            projection_path = case / 'projection-proposed.json'
            args = [binary, 'semantic', 'build', '--repo', repo, '--project-id', state['project_id'],
                    '--dataset-id', identity['dataset_id'], '--graph', version / 'graph', '--vector', version / 'vector',
                    '--version-dir', version, '--store', case / 'semantic-proposed.db', '--out', projection_path,
                    '--ontology', 'ontology.yaml', '--extract-only']
            if fixture['family'] == 'F-06':
                args.extend(['--spec', 'spec.yaml', '--docs', 'README.md'])
            if supplement:
                args.append('--include-packs')
            run(label + '-extract', args)
            projection = json.loads(projection_path.read_text())
            symbol = {'F-03': 'ProjectIdentity', 'F-05': 'Alpha', 'F-06': 'RefundLimit'}[fixture['family']]
            with closing(sqlite3.connect(f'file:{version}/graph/graph.db?mode=ro', uri=True)) as graph:
                anchors = graph.execute("SELECT canonical_id,file_path,start_line,end_line,type,name FROM nodes WHERE type='Function' AND name=?", (symbol,)).fetchall()
                test = graph.execute("SELECT canonical_id,file_path,start_line,end_line,type,name FROM nodes WHERE type='Function' AND name='TestHealth'").fetchall()
            if len(anchors) != 1:
                raise ValueError('expected unique native CKG function anchor')
            canonical, file, start, end, node_type, name = anchors[0]
            raw = MAT.git(repo, 'show', augmented + ':' + file, raw=True)
            span = {'id': 'fixture:code:' + symbol, 'snapshot': projection['snapshot'], 'kind': 'code',
                    'path': file, 'start_line': start, 'end_line': end,
                    'content_sha256': hashlib.sha256(b''.join(raw.splitlines(keepends=True)[start - 1:end])).hexdigest(),
                    'canonical_id': canonical, 'extractor': 'b0-semantic-review-candidate-v1'}
            projection['evidence'].append(span)
            if fixture['family'] != 'F-03':
                concept = projection['concepts'][0]
                projection['assertions'] = [{'id': 'fixture:implementation:' + symbol, 'predicate': 'IMPLEMENTED_BY',
                     'subject_id': concept['id'], 'object_id': canonical, 'evidence_ids': [concept['evidence_id'], span['id']], 'status': 'proposed'}]
            verify_projection(projection, repo)
            projection_path.write_text(json_source(projection))
            # Store proposed records solely to exercise the native alignment,
            # AST and retained-source validators. Never activate a pointer.
            run(label + '-validate-proposed', [binary, 'semantic', 'promote', '--input', projection_path, '--repo', repo,
                '--graph', version / 'graph', '--vector', version / 'vector', '--version-dir', version,
                '--store', case / 'semantic-proposed.db'])
            with closing(sqlite3.connect(case / 'semantic-proposed.db')) as store:
                if store.execute('SELECT count(*) FROM semantic_current').fetchone()[0] != 0:
                    raise ValueError('preparation activated a semantic version')
            MAT.git(repo, 'bundle', 'create', str(case / 'source.bundle'), '--all')
            inventory = {p: sha(repo / p) for p in MAT.git(repo, 'ls-files').splitlines()}
            if fixture['family'] == 'F-06' and len(test) != 1:
                raise ValueError('unrelated test anchor missing')
            cases.append({'id': label, 'fixture_id': fixture['id'], 'family': fixture['family'], 'state': state['name'],
                          'project_id': state['project_id'], 'base_commit': state['commit'], 'base_tree': state['tree'],
                          'supplement_commit': augmented, 'supplement_tree': MAT.git(repo, 'rev-parse', 'HEAD^{tree}'),
                          'dataset': identity, 'source_sha256': inventory,
                          'projection': str(projection_path.relative_to(out)), 'projection_sha256': sha(projection_path),
                          'review_state': 'draft', 'reviewer': None, 'reviewed_at': None,
                          'native_code_anchor': {'canonical_id': canonical, 'file': file, 'start_line': start, 'end_line': end},
                          'unrelated_test_anchors': test,
                          'forbidden_claims': ['TestHealth proves refund acceptance', 'An Alpha marker proves refund policy enforcement']})
    if sha(binary) != binary_hash or sha(manifest) != input_hash:
        raise ValueError('binary or frozen source book changed during preparation')
    result = {'schema_version': 1, 'state': 'prepared_pending_human_review', 'diagnostic_only': True,
              'partition': partition, 'quality_metrics': None, 'binary_sha256': binary_hash,
              'input_manifest_sha256': input_hash, 'embedder': embedder, 'cases': cases, 'checks': checks}
    (out / 'review-manifest.json').write_text(json_source(result))
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--out', required=True, type=Path)
    parser.add_argument('--manifest', type=Path, default=MAT.DEFAULT)
    parser.add_argument('--binary', type=Path, default=ROOT / 'bin/cks')
    parser.add_argument('--embedder', choices=['mock', 'ollama'], default='mock')
    parser.add_argument('--partition', choices=['development', 'final'], default='development')
    parser.add_argument('--model-name', default='bge-m3:latest')
    parser.add_argument('--ollama-url', default='http://127.0.0.1:11434')
    args = parser.parse_args()
    result = prepare(args.out.resolve(), args.manifest.resolve(), args.binary.resolve(), args.embedder, args.model_name, args.ollama_url, args.partition)
    print(json.dumps({'state': result['state'], 'cases': len(result['cases']), 'quality_metrics': None}))


if __name__ == '__main__':
    main()
