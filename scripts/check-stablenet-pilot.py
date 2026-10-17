#!/usr/bin/env python3
"""Verify bounded real-data pilot evidence, including observed readiness failures."""
import argparse
import copy
import gzip
import hashlib
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
REL = Path('docs/spec-driven/operational-preparation/source-review-supplement/followups/stablenet-pilot-v1')
CASES = ['initial-probe', 'baseline-probe', 'retrieval', 'boundary-resume', 'foreign-project-manifest', 'wrong-snapshot-manifest', 'restored']


def sha(data):
    return hashlib.sha256(data).hexdigest()


def read(path):
    return json.loads(path.read_text())


def require(ok, reason):
    if not ok:
        raise ValueError(reason)


def payload(response):
    r = response.get('result', {})
    if 'structuredContent' in r:
        return r['structuredContent']
    try:
        return json.loads(r['content'][0]['text'])
    except (KeyError, ValueError):
        return {'error_text': str(r.get('content', response))}


def failed(response):
    return 'error' in response or response.get('result', {}).get('isError') is True


def derive(packet):
    identity = read(packet / 'dataset-identity.json')
    model = read(packet / 'model-identity.json')
    require(model['selected_model']['digest'] == identity['embedding_identity']['model_digest'] and model['embedding_probe']['dimensions'] == identity['embedding_identity']['Dim'] == 1024 and model['embedding_probe']['finite'], 'model identity/dimension')
    protocol, pin = read(packet / 'protocol.json'), read(packet / 'binary-pin.json')
    require(protocol['runtime_binary_sha256'] == pin['sha256'] and protocol['native_source_commit'] == pin['source_commit'] and protocol['ontology_mode'] == 'off' and not protocol['type_neighbors'], 'runtime source/config pin')
    before, after = read(packet / 'original-before.json'), read(packet / 'original-after.json')
    require(before == after and not before['source_status'] and before['source_commit'] == identity['source']['source_commit'], 'original source/data changed')
    preservation = read(packet / 'preservation.json')
    require(all(preservation[k] for k in ['selected_original_all_file_bytes_unchanged', 'owned_source_clean', 'global_configuration_equal', 'operational_metadata_pointers_equal', 'prior_source_checkouts_equal', 'original_unowned_engines_preserved', 'original_current_unchanged']) and preservation['owned_copy_changed_original_files'] == [] and preservation['production_writes'] == preservation['external_publication'] == preservation['paid_LLM_calls'] == 0, 'preservation scope')
    sessions, records, pids = [], {}, set()
    for case in CASES:
        folder = packet / 'sessions' / case
        life, attempts = read(folder / 'lifecycle.json'), read(folder / 'attempts.json') if (folder / 'attempts.json').exists() else []
        require(life['command'][:3] == [pin['path'], 'mcp', '--config'], 'executed binary command')
        expected = 1 if case in ['initial-probe', 'foreign-project-manifest', 'wrong-snapshot-manifest'] else 0
        require(life['exit_code'] == expected and life['child_reaped'] and life['reader_joined'] and life['eof_sent'] and not life['terminate_sent'] and not life['kill_sent'], 'session cleanup/expected failure')
        if expected == 1:
            require(json.loads((folder / 'stderr.txt').read_text())['code'] == 'operation_failed', 'public startup diagnostic changed')
        rows = read(folder / 'messages.json')
        wire = [json.loads(line) for line in (folder / 'stdout.jsonl').read_text().splitlines()]
        for row in rows:
            require(row['response'] in wire, 'response absent from raw wire')
        calls = [row for row in rows if row['request']['method'] == 'tools/call']
        require(len(calls) == life['tool_requests'], 'tool attempt denominator')
        if attempts:
            require(len(attempts) == life['rpc_requests'], 'attempt inventory')
        records[case] = {row['label']: row for row in rows}
        sessions.append(life)
        pids.add(life['pid'])
    require(read(packet / 'process-audit.json')['self']['exit_code'] == 0, 'process audit positive control')
    audit = read(packet / 'process-audit.json')['probes']
    require({r['pid'] for r in audit} == pids and all(r['exit_code'] == 1 and not r['stdout'].strip() for r in audit), 'owned process remains')
    primary = records['retrieval']
    healthy = payload(primary['health-start']['response'])
    require(healthy['serviceable'] and healthy['alignment']['ok'] and healthy['alignment']['project_id'] == identity['source']['project_id'] and healthy['alignment']['dataset_id'] == identity['dataset_id'], 'actual alignment')
    coords = {k: identity['source'][k] for k in ['project_id', 'snapshot_id', 'source_mode']}
    coords.update(dataset_id=identity['dataset_id'], base_commit=identity['source']['source_commit'])
    summaries, raw_hits, packs = [], 0, []
    non_citable_conventions = 0
    for i in range(1, 7):
        states = {}
        for suffix in ['keyword', 'vector-go', 'vector-all', 'pack-v2']:
            row = primary['Q' + str(i) + '-' + suffix]
            states[suffix] = not failed(row['response'])
            if states[suffix] and suffix != 'pack-v2':
                hits = payload(row['response'])['hits']
                ranks = [h['rank'] for h in hits]
                require(0 < len(hits) <= 10 and all(type(n) is int and n > 0 for n in ranks) and ranks == sorted(set(ranks)), 'rank/cap contract')
                if suffix != 'keyword':
                    require(ranks == list(range(1, len(hits) + 1)), 'vector rank contract')
                require(all(hits[j]['score'] >= hits[j + 1]['score'] for j in range(len(hits) - 1)), 'score order')
                for hit in hits:
                    c = hit['citation']
                    require(c['commit_hash'] == coords['base_commit'], 'legacy hit commit pin')
                    if c['file'] not in read(packet / 'witness-index.json'):
                        require(hit.get('chunk_kind') == 'convention' and c['start_line'] == c['end_line'] == 0 and c['file'].endswith('/<convention>'), 'unverified raw hit')
                        non_citable_conventions += 1
                raw_hits += len(hits)
            if suffix == 'pack-v2' and states[suffix]:
                packs.append(payload(row['response']))
        summaries.append(states)
    for row in records['restored'].values():
        if row['label'] == 'known-success-v2':
            require(not failed(row['response']), 'restore did not recover known successful v2')
            packs.append(payload(row['response']))
    witness = read(packet / 'witness-index.json')
    git_equality = {r['path']: r for r in read(packet / 'retained-source-git-equality.json')}
    require(len(git_equality) == 2224 and all(r['commit_equal'] for r in git_equality.values()), 'retained source Git inventory')
    for file, info in witness.items():
        raw = (packet / 'source-witness' / (info['sha256'] + '.txt')).read_bytes()
        blob = hashlib.sha1(b'blob ' + str(len(raw)).encode() + b'\0' + raw).hexdigest()
        require(sha(raw) == info['sha256'] == git_equality[file]['sha256'] and blob == git_equality[file]['git_blob'], 'source witness Git/hash')
    citation_count = 0
    for pack in packs:
        require(pack['coordinates'] == coords and pack['metadata']['ontology']['mode'] == 'off' and pack['metadata']['ontology']['explanation']['quality_evaluated'] is False, 'v2 project coordinates/ontology scope')
        unsigned = copy.deepcopy(pack)
        want = unsigned['metadata'].pop('integrity_hash')
        require(sha(json.dumps(unsigned, ensure_ascii=False, sort_keys=True, separators=(',', ':')).encode()) == want, 'v2 integrity')
        require([b['citation'] for b in pack['bodies']] == pack['citations'], 'citation/body order')
        for body in pack['bodies']:
            c = body['citation']
            require(all(c[k] == v for k, v in coords.items()) and c['commit_hash'] == coords['base_commit'] and c['origin_id'] == 'repo', 'foreign v2 citation')
            info = witness[c['file']]
            raw = (packet / 'source-witness' / (info['sha256'] + '.txt')).read_bytes()
            excerpt = b''.join(raw.splitlines(keepends=True)[c['start_line'] - 1:c['end_line']])
            require(sha(raw) == c['file_sha256'] == info['sha256'] and sha(excerpt) == c['content_sha256'] and body['text'] == excerpt.decode(), 'retained citation hash/body')
            citation_count += 1
    for key in ['wrong-commit', 'wrong-source', 'wrong-snapshot', 'foreign-fixture-node']:
        response = payload(primary[key]['response'])
        require(response['status'] == ('not_found' if key == 'foreign-fixture-node' else 'source_mismatch') and response['analysis'] is None, 'typed pin/foreign refusal')
    require(payload(records['boundary-resume']['retained-blob-tamper']['response'])['code'] == 'snapshot_mismatch', 'retained mutation accepted')
    def normalize(v):
        if isinstance(v, dict):
            return {k: normalize(x) for k, x in v.items() if k != 'elapsed_micros'}
        if isinstance(v, list):
            return [normalize(x) for x in v]
        return v
    require(normalize(payload(primary['value-baseline']['response'])) == normalize(payload(records['boundary-resume']['live-checkout-tamper']['response'])), 'live source replaced retained graph evidence')
    require(payload(records['boundary-resume']['exact-pin-restored']['response'])['status'] == 'partial', 'restored exact pins failed')
    require([primary['Q1-vector-go']['response']['result']['structuredContent']['hits'][j]['citation'] for j in range(10)] == [primary['repeat-vector-go-0']['response']['result']['structuredContent']['hits'][j]['citation'] for j in range(10)] == [primary['repeat-vector-go-1']['response']['result']['structuredContent']['hits'][j]['citation'] for j in range(10)], 'repeat citation order')
    total = lambda key: sum(s[key] for s in summaries)
    return {'status': 'verification_complete_default_retrieval_not_ready', 'source_commit': coords['base_commit'], 'project_id': coords['project_id'], 'dataset_id': coords['dataset_id'], 'model_digest': model['selected_model']['digest'], 'model_dimensions': 1024, 'ollama_version': model['version'], 'primary_inputs': 6, 'keyword_success': total('keyword'), 'Go_filtered_vector_success': total('vector-go'), 'unfiltered_vector_success': total('vector-all'), 'v2_success': total('pack-v2'), 'primary_raw_hits_checked': raw_hits, 'non_citable_convention_hits': non_citable_conventions, 'raw_hits_with_retained_file_witness': raw_hits - non_citable_conventions, 'v2_packs_including_restore': len(packs), 'v2_citations_hash_body_coordinates_checked': citation_count, 'ranking_repeats': 2, 'typed_pin_foreign_refusals': 4, 'retained_tamper_refusals': 1, 'controlled_manifest_startup_refusals': 2, 'live_source_override_ignored': True, 'all_tool_attempts': sum(s['tool_requests'] for s in sessions), 'normal_EOF_exit0_reaped': 4, 'observed_exit1_reaped': 3, 'owned_PID_count': len(pids), 'forced_termination': 0, 'default_retrieval_ready': False, 'independent_quality_evaluated': False, 'ontology_effect_evaluated': False, 'formal_completed': 0, 'formal_total': 9, 'reviewer_assignment': '미정', 'paid_LLM_calls': 0, 'production_writes': 0}


def check(packet):
    try:
        manifest = read(packet / 'manifest.json')
        require(sha((ROOT / manifest['checker']).read_bytes()) == manifest['checker_sha256'], 'checker binding')
        files = {str(p.relative_to(packet)) for p in packet.rglob('*') if p.is_file() and p.name != 'manifest.json'}
        require(files == set(manifest['artifacts']), 'packet inventory')
        for path, digest in manifest['artifacts'].items():
            require(sha((packet / path).read_bytes()) == digest, 'artifact hash')
        require(read(packet / 'result.json') == derive(packet), 'derived result/readiness promotion')
        return []
    except (OSError, ValueError, KeyError, TypeError, IndexError) as e:
        return [str(e)]


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--packet', type=Path, default=ROOT / REL)
    args = parser.parse_args()
    errors = check(args.packet)
    print('\n'.join(errors) if errors else 'pilot evidence verified; default retrieval NOT ready; formal 0/9')
    raise SystemExit(bool(errors))
