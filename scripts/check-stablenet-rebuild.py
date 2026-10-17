#!/usr/bin/env python3
"""Recompute held real-data rebuild and paired provenance/metadata evidence."""
import argparse
import copy
import gzip
import hashlib
import json
from pathlib import Path
import subprocess

ROOT = Path(__file__).resolve().parents[1]
REL = Path('docs/spec-driven/operational-preparation/source-review-supplement/followups/stablenet-rebuild-v1')


def require(ok, message):
    if not ok:
        raise ValueError(message)


def sha(data):
    return hashlib.sha256(data).hexdigest()


def read(path):
    return json.loads(path.read_text())


def zipped(path):
    return json.loads(gzip.decompress(path.read_bytes()))


def payload(response):
    data = response.get('result', {})
    if 'structuredContent' in data:
        return data['structuredContent']
    try:
        return json.loads(data['content'][0]['text'])
    except (KeyError, ValueError):
        return {'error_text': str(data.get('content', response))}


def failed(response):
    return 'error' in response or response.get('result', {}).get('isError') is True


def normalize(value):
    if isinstance(value, dict):
        return {k: normalize(v) for k, v in value.items() if k != 'elapsed_micros'}
    if isinstance(value, list):
        return [normalize(v) for v in value]
    return value


def source_lines(raw):
    parts = raw.split(b'\n')
    if raw.endswith(b'\n'):
        parts.pop()
        return [p + b'\n' for p in parts]
    return [p + b'\n' for p in parts[:-1]] + ([parts[-1]] if raw else [])


def derive(packet, live=True):
    execution = read(packet / 'execution.json')
    require(execution['formal_completed'] == 0 and execution['formal_total'] == 9 and execution['reviewer_assignment'] == '미정' and execution['paid_LLM_calls'] == execution['production_writes'] == execution['external_publication'] == 0, 'scope or formal promotion')
    binding = read(packet / 'source-binding.json')
    committed = subprocess.check_output(['git', 'show', execution['binding_commit'] + ':' + str(REL / 'source-binding.json')], cwd=ROOT)
    require(sha(committed) == sha((packet / 'source-binding.json').read_bytes()), 'committed source binding')
    if live:
        for name, digest in {**binding['source_sha256'], **binding['approved_input_sha256']}.items():
            require(sha((ROOT / name).read_bytes()) == digest, 'current source/input changed')
    native = read(packet / 'validation/native/receipts.json')
    require([r['label'] for r in native] == ['build', 'test', 'race', 'lint', 'build-bins'] and all(r['exit_code'] == 0 and r['reaped'] and not r['timeout'] and not r['term_sent'] and not r['kill_sent'] and r['source_head'] == execution['native_source_commit'] for r in native), 'fresh native checks incomplete')
    counts = {}
    for r in native:
        for stream in ['stdout', 'stderr']:
            raw = gzip.decompress((packet / 'validation/native' / (r['label'] + '.' + stream + '.gz')).read_bytes())
            require(sha(raw) == r[stream + '_sha256'], 'native raw log association')
            if stream == 'stdout' and r['label'] in ['test', 'race']:
                require('-count=1' in r['argv'] and r['argv'][-1] == './...' and '-run' not in r['argv'], 'filtered/cached test run')
                rows = [json.loads(line) for line in raw.decode().splitlines() if line.startswith('{')]
                observed = {action: sum(e.get('Action') == action and bool(e.get('Test')) for e in rows) for action in ['pass', 'skip', 'fail']}
                require(observed == r['tests'] and observed['pass'] > 4000 and not any(e.get('Action') == 'fail' for e in rows), 'native test accounting')
                for name in ['TestHit_MetadataFlagSurvivesJSON', 'TestHandleSemanticSearch_PreservesNonCitableMetadata']:
                    require(any(e.get('Action') == 'pass' and e.get('Test') == name for e in rows), 'new metadata contract test absent')
                counts[r['label']] = observed
    build = read(packet / 'build-exit.json')
    require(build['exit_code'] == 0 and build['reaped'] and not build['forced_termination'] and not build['owned_current_exists'], 'held build/promotion boundary')
    start = read(packet / 'build-start.json')
    require('--hold-for-review' in start['command'] and not start['before_current_exists'], 'supported held build not used')
    for name in ['cks', 'ckg', 'ckv']:
        require('vcs.revision=' + execution['build_source_commit'] in (packet / (name + '-build-info.txt')).read_text(), 'builder source pin')
    require('vcs.revision=' + execution['native_source_commit'] in (packet / 'read-bin-build-info.txt').read_text() and 'vcs.modified=false' in (packet / 'read-bin-build-info.txt').read_text(), 'clean reader binary source pin')
    old, new = read(packet / 'old-identity.json'), read(packet / 'new-identity.json')
    require(old['source']['project_id'] == new['source']['project_id'] and old['source']['source_commit'] == new['source']['source_commit'] and old['source']['snapshot_id'] != new['source']['snapshot_id'] and old['dataset_id'] != new['dataset_id'] and old['embedding_identity'] == new['embedding_identity'], 'dataset/source/model identities')
    require(new['embedding_identity']['model_digest'] == read(packet / 'model-before.json')['model']['digest'] == read(packet / 'preservation.json')['model_digest_after'] and new['embedding_identity']['Dim'] == 1024, 'model digest/dimension')
    sources = {arm: zipped(packet / (arm + '-source-manifest.json.gz')) for arm in ['old', 'new']}
    code = lambda arm: {r['path']: r['sha256'] for r in sources[arm]['files'] if r['origin_id'] == 'repo'}
    require(code('old') == code('new') and len(code('new')) == 2224, 'retained repository bytes changed')
    inventory = {(r['origin_id'], r['path']): r for r in sources['new']['files']}
    for origin in read(packet / 'corpus-inputs.json')['origins']:
        for path, digest in origin['file_sha256'].items():
            require(inventory[(origin['id'], path)]['sha256'] == digest, 'external original capture mismatch')
    ledger = zipped(packet / 'chunk-source-ledger.json.gz')
    require(len(ledger) > 20000, 'corpus denominator narrowed')
    kinds = {}
    for row in ledger:
        state = row['citation_state']
        kinds[state] = kinds.get(state, 0) + 1
        if state == 'non_citable_metadata':
            require(row['kind'] in ['convention', 'flow_spine', 'invariant', 'pr_background', 'pr_solution', 'commit_message'], 'uncited code hidden as metadata')
        else:
            require(state == 'retained_source_range' and inventory[(row['origin_id'], row['path'])]['sha256'] == row['file_sha256'] and row['start_line'] > 0 and row['end_line'] >= row['start_line'], 'unbound source path')
            if row['origin_id'] != 'repo':
                require(row['file'] == '@cks/docs/' + row['origin_id'].split('.')[-1] + '/' + row['path'], 'external namespace collision')
    witness = {(r['origin_id'], r['path']): r for r in read(packet / 'witness-index.json')}
    sessions, records, packs = [], {}, []
    cases = ['paired-old', 'paired-new', 'boundaries-new', 'foreign-project-new', 'foreign-snapshot-new', 'boundary-restored-new']
    for case in cases:
        folder = packet / 'sessions' / case
        lifecycle = read(folder / 'lifecycle.json')
        expected = 1 if case in ['foreign-project-new', 'foreign-snapshot-new'] else 0
        require(lifecycle['exit_code'] == expected and lifecycle['eof_sent'] and lifecycle['child_reaped'] and lifecycle['reader_joined'] and not lifecycle['terminate_sent'] and not lifecycle['kill_sent'], 'stdio shutdown')
        rows = read(folder / 'messages.json')
        wire = [json.loads(s) for s in (folder / 'stdout.jsonl').read_text().splitlines()]
        require(all(row['response'] in wire for row in rows), 'raw wire response omitted')
        require(sum(r['request']['method'] == 'tools/call' for r in rows) == lifecycle['tool_requests'], 'tool denominator')
        require(len(read(folder / 'attempts.json')) == lifecycle['rpc_requests'], 'request attempts hidden')
        records[case] = {r['label']: r for r in rows}
        for row in rows:
            p = payload(row['response'])
            if not failed(row['response']) and p.get('format_version') == 2:
                packs.append((old if case == 'paired-old' else new, p))
        sessions.append(lifecycle)
    inputs = read(packet / 'paired-inputs.json')
    require(len(inputs['inputs']) == 6 and inputs['k'] == 10 and inputs['ontology'] == 'off' and inputs['read_source_commit'] == execution['native_source_commit'], 'paired input scope changed')
    for item in inputs['inputs']:
        for suffix in ['keyword', 'vector-go', 'vector-all', 'pack-v2']:
            label = item['id'] + '-' + suffix
            old_request, new_request = (records['paired-' + arm][label]['request'] for arm in ['old', 'new'])
            require(old_request['params'] == new_request['params'], 'paired old/new requests differ')
            args = new_request['params']['arguments']
            if suffix == 'pack-v2':
                require(args['prompt'] == item['prompt'] and args['intent'] == item['intent'], 'v2 frozen input changed')
            else:
                require(args['query'] == (item['keyword'] if suffix == 'keyword' else item['prompt']) and args['k'] == 10, 'search frozen input changed')
    paired = {}
    new_external_raw_hits = 0
    for arm in ['old', 'new']:
        observed = {key: 0 for key in ['keyword', 'vector-go', 'vector-all', 'pack-v2']}
        rows = records['paired-' + arm]
        for i in range(1, 7):
            for suffix in observed:
                observed[suffix] += not failed(rows['Q' + str(i) + '-' + suffix]['response'])
        conv = payload(rows['convention-contract']['response'])['hits']
        require(len(conv) > 0 and all(h['chunk_kind'] == 'convention' and h['non_citable_metadata'] is True for h in conv), 'actual convention metadata contract')
        for row in rows.values():
            if failed(row['response']):
                continue
            p = payload(row['response'])
            for h in p.get('hits', []):
                if h.get('chunk_kind') == 'convention':
                    require(h.get('non_citable_metadata') is True, 'convention promoted to citable source')
                if arm == 'new' and not h.get('non_citable_metadata', False):
                    c = h['citation']
                    file, origin = c['file'], 'repo'
                    if file.startswith('@cks/docs/'):
                        number, file = file[len('@cks/docs/'):].split('/', 1)
                        require(str(int(number)) == number, 'malformed raw docs namespace')
                        origin = 'docs:corpus.' + number
                    item = witness[(origin, file)]
                    raw = (packet / 'source-witness' / (item['sha256'] + '.txt')).read_bytes()
                    require(sha(raw) == item['sha256'] == inventory[(origin, file)]['sha256'] and 1 <= c['start_line'] <= c['end_line'] <= len(source_lines(raw)), 'raw hit retained source/range')
                    new_external_raw_hits += origin != 'repo'
        paired[arm] = observed
    legacy_row = records['paired-new']['legacy-knowledge-separation']['response']
    require(not failed(legacy_row), 'new legacy composer remains incomplete')
    legacy = payload(legacy_row)
    require(any(k['kind'] == 'convention' and k.get('text') for k in legacy['knowledge']), 'convention knowledge lost')
    require(all(c['start_line'] > 0 and c['end_line'] >= c['start_line'] and '<convention>' not in c['file'] for c in legacy['citations']), 'legacy metadata promoted to source citation')
    require(all(n == 6 for n in paired['new'].values()), 'new default retrieval remains incomplete')
    require(new_external_raw_hits > 0, 'actual external retained path never exercised')
    citations, external_citations = 0, 0
    for identity, pack in packs:
        coordinates = {k: identity['source'][k] for k in ['project_id', 'snapshot_id', 'source_mode']}
        coordinates.update(dataset_id=identity['dataset_id'], base_commit=identity['source']['source_commit'])
        require(pack['coordinates'] == coordinates, 'pack project/dataset mixing')
        unsigned = copy.deepcopy(pack)
        want = unsigned['metadata'].pop('integrity_hash')
        require(sha(json.dumps(unsigned, ensure_ascii=False, sort_keys=True, separators=(',', ':')).encode()) == want, 'v2 integrity')
        require([b['citation'] for b in pack['bodies']] == pack['citations'], 'body/citation order')
        for body in pack['bodies']:
            c = body['citation']
            require(all(c[k] == v for k, v in coordinates.items()) and c['start_line'] > 0 and '<convention>' not in c['file'], 'foreign/non-citable v2 citation')
            item = witness[(c['origin_id'], c['file'])]
            raw = (packet / 'source-witness' / (item['sha256'] + '.txt')).read_bytes()
            excerpt = b''.join(source_lines(raw)[c['start_line'] - 1:c['end_line']])
            require(sha(raw) == item['sha256'] == c['file_sha256'] and sha(excerpt) == c['content_sha256'] and body['text'] == excerpt.decode(), 'retained body/hash')
            citations += 1
            external_citations += c['origin_id'] != 'repo'
    rows = records['boundaries-new']
    for label in ['wrong-commit', 'wrong-source', 'wrong-snapshot']:
        p = payload(rows[label]['response'])
        require(p['status'] == 'source_mismatch' and p['analysis'] is None, 'wrong pin not refused')
    require(payload(rows['foreign-node']['response'])['status'] == 'not_found' and payload(rows['external-retained-tamper']['response'])['code'] == 'snapshot_mismatch', 'foreign/blob boundary')
    require(normalize(payload(rows['external-live-before']['response'])) == normalize(payload(rows['external-live-tamper']['response'])), 'live external origin substituted')
    require(read(packet / 'lease-release.json')['released'], 'reader lease not released')
    scope = read(packet / 'preservation.json')
    require(scope['original_selected_unchanged'] and scope['global_configuration_equal'] and scope['operational_metadata_pointers_equal'] and scope['prior_source_checkouts_equal'] and scope['original_unowned_engines_preserved'] and scope['no_current_created_in_owned_dataset'] and scope['original_external_inputs_unchanged'], 'preservation boundary')
    audit = read(packet / 'process-audit.json')
    require(audit['self']['exit_code'] == 0 and all(r['identity_gone'] for r in audit['probes']) and {s['pid'] for s in sessions} <= {r['pid'] for r in audit['probes']}, 'owned process cleanup')
    return {'status': 'SN-P01_SN-P02_scoped_technical_pass', 'native_source_commit': execution['native_source_commit'], 'native_test': counts['test'], 'native_race': counts['race'], 'source_files_unchanged': 2224, 'retained_origin_files': {origin: sum(r['origin_id'] == origin for r in sources['new']['files']) for origin in ['repo', 'docs:corpus.0', 'docs:corpus.1']}, 'indexed_chunks': len(ledger), 'chunk_citation_states': kinds, 'paired_inputs': 6, 'old': paired['old'], 'new': paired['new'], 'all_actual_tool_attempts': sum(s['tool_requests'] for s in sessions), 'actual_metadata_contract_arms': 2, 'legacy_convention_knowledge_separated': True, 'successful_v2_packs_including_restore': len(packs), 'v2_citations_verified': citations, 'external_v2_citations_verified': external_citations, 'external_raw_hits_with_retained_source_verified': new_external_raw_hits, 'normal_stdio_EOF_exit0_reaped': 4, 'intentional_manifest_startup_exit1_reaped': 2, 'owned_process_identities_audited': len(audit['probes']), 'formal_completed': 0, 'formal_total': 9, 'reviewer_assignment': '미정', 'independent_quality_evaluated': False, 'ontology_effect_evaluated': False, 'paid_LLM_calls': 0, 'production_activation': False, 'SN-P03_error_classification_complete': False, 'SN-P04_go_wemix_comparison_complete': False}


def check(packet, live=True):
    try:
        manifest = read(packet / 'manifest.json')
        require(sha((ROOT / manifest['checker']).read_bytes()) == manifest['checker_sha256'], 'checker source binding')
        paths = {str(p.relative_to(packet)) for p in packet.rglob('*') if p.is_file() and p.name != 'manifest.json'}
        require(paths == set(manifest['artifacts']), 'packet inventory')
        for name, digest in manifest['artifacts'].items():
            require(sha((packet / name).read_bytes()) == digest, 'artifact hash')
        require(derive(packet, live) == read(packet / 'result.json'), 'derived result/completion promotion')
        return []
    except (OSError, ValueError, KeyError, TypeError, IndexError, subprocess.SubprocessError) as e:
        return [str(e)]


if __name__ == '__main__':
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--packet', type=Path, default=ROOT / REL)
    p.add_argument('--no-live-source', action='store_true')
    args = p.parse_args()
    errors = check(args.packet, not args.no_live_source)
    print('\n'.join(errors) if errors else 'held rebuild/paired provenance verified; formal 0/9; no production activation')
    raise SystemExit(bool(errors))
