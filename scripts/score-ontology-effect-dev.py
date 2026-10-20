#!/usr/bin/env python3
"""Verify and describe the frozen four-arm local observation, never quality approval."""
import argparse
import collections
import copy
import gzip
import hashlib
import json
from pathlib import Path
import statistics

import yaml

ARMS = ('off', 'concept_text', 'relations', 'combined')
COUNTS = {'warmup': 1, 'retrieval': 2, 'warm_latency': 5, 'cold_process': 0}
SOURCE_MANIFEST_SHA = 'e587383dab29257e85b23bb053eff22c3cbbdd2e28f78af6b039e7e95fafa373'
COORDINATES = {
    'project_id': 'p-3fa6368dd3f6757c6b6a86121f0a9b7d',
    'dataset_id': '4894eaf2f1f14b049d8a262f04327f92e7eacf73a5b4cd0401f9f9df30eb9ff4',
    'snapshot_id': '99ad6ae09c82fb2cfa2c70b405bde55f5caf112116562249463903aadff43bb4',
    'source_mode': 'committed', 'base_commit': 'baca2869118774c5f6219932dc76e197a47e6ecd',
}
FAMILIES = {f'OE-D{i:02}': ('quorum', 'round-change', 'message-handling', 'current-lock')[(i - 1) // 2]
            for i in range(1, 9)}


def require(ok, message):
    if not ok:
        raise ValueError(message)


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def canonical(value):
    return json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(',', ':'), allow_nan=False).encode()


def percentile(values, q):
    if not values:
        return None
    values = sorted(values)
    index = (len(values) - 1) * q
    lo = int(index)
    return values[lo] + (values[min(lo + 1, len(values) - 1)] - values[lo]) * (index - lo)


def verify_pack(pack, query, arm, source):
    """Every published citation is verified against original retained bytes."""
    require(pack['format_version'] == 2 and pack['query'] == query, 'format/query mismatch')
    require(pack['coordinates'] == COORDINATES, 'foreign project/version coordinates')
    require(pack['evidence_state'] in ('complete', 'partial'), 'invalid evidence state')
    value = copy.deepcopy(pack)
    integrity = value['metadata'].pop('integrity_hash')
    require(value['metadata']['integrity_hash_algo'] == 'sha256-v2' and sha(canonical(value)) == integrity,
            'response integrity hash mismatch')
    ontology = pack['metadata']['ontology']
    require(ontology['mode'] == arm and ontology['explanation']['quality_evaluated'] is False,
            'mode/quality scope mismatch')
    require(pack['graph_neighbors'] == [] and pack['semantic'] is None, 'unexpected unverified overlay')
    citations = pack['citations']
    for c in citations:
        require(all(c[k] == v for k, v in COORDINATES.items()), 'foreign citation coordinates')
        require(c['commit_hash'] == COORDINATES['base_commit'], 'citation commit mismatch')
        raw = source(c['origin_id'], c['file'], c['file_sha256'])
        lines = raw.splitlines(keepends=True)
        a, b = c['start_line'], c['end_line']
        require(type(a) is int and type(b) is int and 1 <= a <= b <= len(lines), 'invalid citation range')
        require(sha(b''.join(lines[a - 1:b])) == c['content_sha256'], 'citation span hash mismatch')
    for body in pack['bodies']:
        require(body['citation'] in citations, 'body citation not published')
        require(sha(body['text'].encode()) == body['citation']['content_sha256'], 'body text differs from retained span')
    for text_source in ontology.get('text_sources', []):
        require(all(text_source[k] == COORDINATES[k] for k in ('project_id', 'dataset_id', 'snapshot_id'))
                and text_source['commit'] == COORDINATES['base_commit'], 'foreign ontology text source')
        data = source('repo', text_source['file'], None)
        require(sha(b''.join(data.splitlines(keepends=True)[text_source['start_line'] - 1:text_source['end_line']]))
                == text_source['content_sha256'], 'ontology text source span differs')
    return ontology


def location_metric(citations, refs):
    ranks = [rank for rank, c in enumerate(citations[:10], 1)
             if any(c['file'] == r['file'] and c['start_line'] <= r['end_line']
                    and r['start_line'] <= c['end_line'] for r in refs)]
    return {'source_location_hit_at_10': float(bool(ranks)),
            'source_location_reciprocal_rank_at_10': 1 / min(ranks) if ranks else 0.0}


def analyze(folder):
    def raw(name):
        path = folder / name
        return path.read_bytes() if path.exists() else gzip.decompress((folder / (name + '.gz')).read_bytes())
    def load(name):
        return json.loads(raw(name))
    protocol = load('protocol.json')
    require(protocol['quality_acceptance'] is False and protocol['production_activation'] is False
            and protocol['mode_defaults_changed'] is False and protocol['new_independent_FINAL'] == 0
            and protocol['independent_units_approved'] == 0 and protocol['LLM_answer_quality'] is None,
            'development observation promoted to acceptance')
    require(protocol['arms'] == list(ARMS) and protocol['counts'] == COUNTS
            and protocol['planned_calls'] == 384, 'frozen design differs')
    manifest = raw('source-manifest.json')
    require(sha(manifest) == SOURCE_MANIFEST_SHA, 'retained source manifest differs')
    source_rows = {(r['origin_id'], r['path']): r for r in json.loads(manifest)['files']}
    blob_cache = {}
    def source(origin, path, digest):
        require((origin, path) in source_rows and (digest is None or source_rows[(origin, path)]['sha256'] == digest),
                'source absent from pinned manifest')
        digest = source_rows[(origin, path)]['sha256']
        if digest not in blob_cache:
            data = (folder / 'source-blobs' / (digest + '.txt')).read_bytes()
            require(sha(data) == digest, 'retained blob hash mismatch')
            blob_cache[digest] = data
        return blob_cache[digest]
    identity = load('dataset-identity.json')
    require(identity['dataset_id'] == COORDINATES['dataset_id']
            and all(identity['source'][k] == COORDINATES[k] for k in ('project_id', 'snapshot_id', 'source_mode'))
            and identity['source']['source_commit'] == COORDINATES['base_commit'], 'dataset tuple differs')
    ontology_sha = '65c53b587f0833c5b3031752014a826f130d6884f65f2ddfee4b6d7240b162c3'
    source('repo', 'docs/approved-meaning/ontology.yaml', ontology_sha)
    require(sha(raw('semantic.sqlite')) == protocol['semantic_sha256'], 'semantic database changed')
    projection = load('projection.json')
    require(len(projection['concepts']) == 4 and not projection.get('assertions'), 'semantic projection differs')
    refs = load('private-location-references.json')
    require(refs['independent_approval'] is False, 'AI location proxies promoted to human gold')
    refs = {r['id']: r['references'] for r in refs['cases']}
    require(set(refs) == set(FAMILIES), 'positive location proxy inventory differs')
    for spans in refs.values():
        for r in spans:
            data = source('repo', r['file'], r['file_sha256'])
            require(sha(b''.join(data.splitlines(keepends=True)[r['start_line'] - 1:r['end_line']]))
                    == r['span_sha256'], 'proxy source span differs')
    inventory = load('capture-inventory.json')
    questions = {}
    configs = []
    for arm in ARMS:
        config = yaml.safe_load(raw(arm + '.yaml'))
        config['semantic'].pop('ontology_mode', None)
        configs.append(config)
    require(all(c == configs[0] for c in configs), 'non-mode configuration differences')
    actual = {}
    citations_verified = bodies_verified = 0
    seen_captures = set()
    for entry in inventory:
        qid, arm = entry['question'], entry['arm']
        require(qid in protocol['question_ids'] and arm in ARMS and (qid, arm) not in seen_captures,
                'duplicate or unplanned capture')
        seen_captures.add((qid, arm))
        req = load(qid + '-request.json')
        require(len(req['requests']) == 1, 'request inventory differs')
        question = req['requests'][0]
        require(question['id'] == qid and question['tool'] == 'cks.context.get_for_task_v2', 'wrong tool/question')
        questions[qid] = question
        report = load(entry['file'])
        require(report['counts'] == COUNTS and report['state'] == 'captured'
                and report['quality_metrics'] is None and len(report['rows']) == entry['rows'] == 8,
                'incomplete capture')
        require(report['request_sha256'] == sha(raw(qid + '-request.json'))
                and report['config_sha256'] == report['config_sha256_after'] == sha(raw(arm + '.yaml'))
                and report['binary_sha256'] == report['binary_sha256_after'] == protocol['binary_sha256'],
                'capture input pin differs')
        require(len(report['session_closures']) == 1 and not report['session_closures'][0].get('error'),
                'MCP session did not close cleanly')
        for row in report['rows']:
            key = (arm, qid, row['phase'], row['iteration'])
            require(row['request_id'] == qid and row['phase'] in COUNTS and type(row['iteration']) is int
                    and 1 <= row['iteration'] <= COUNTS[row['phase']] and key not in actual,
                    'duplicate/unplanned call slot')
            call = row.get('call') or {}
            require(call.get('tool') == question['tool'] and call.get('arguments') == question['arguments'],
                    'request arguments differ')
            require(type(call.get('elapsed_ns')) is int and call['elapsed_ns'] >= 0, 'invalid elapsed time')
            response = call.get('response') or {}
            failed = bool(row.get('error') or call.get('transport_error') or response.get('isError')
                          or not response.get('structuredContent'))
            pack = response.get('structuredContent') if not failed else None
            if pack:
                verify_pack(pack, question['arguments']['prompt'], arm, source)
                citations_verified += len(pack['citations'])
                bodies_verified += len(pack['bodies'])
            actual[key] = (row, pack)
    slots = {(arm, qid, phase, i) for arm in ARMS for qid in protocol['question_ids']
             for phase, count in COUNTS.items() for i in range(1, count + 1)}
    require(set(actual) == slots and len(inventory) == 48, 'missing capture/call slots')
    ordered_questions = {'requests': [questions[qid] for qid in protocol['question_ids']]}
    # The runner pins the full public request document; preserve its metadata too.
    public = load('public-requests.json')
    require(public['requests'] == ordered_questions['requests']
            and sha(json.dumps(public, ensure_ascii=False, sort_keys=True).encode()) == protocol['requests_sha256'],
            'frozen public questions differ')
    require(load('model-before.json') == load('model-after.json'), 'model digest changed')
    require(load('preservation.json')['unchanged'] is True, 'dataset preservation failed')
    readiness = load('readiness.json')
    require(set(readiness) == set(ARMS), 'four-arm readiness missing')
    for arm, status in readiness.items():
        require(status['availability'] == 'ready' and status['read_tools_ready'] is True
                and status['mode'] == arm and status['retrieval_ready'] == (arm in ('concept_text', 'combined'))
                and status['counts']['verified_concepts'] == 4 and status['counts']['verified_implementations'] == 0,
                'semantic readiness differs')
    require(all(c['exit_code'] == 0 for c in load('commands.json')), 'failed command omitted')
    model = load('model-before.json')
    require(model == {'name': 'bge-m3:latest', 'digest': '7907646426070047a77226ac3e684fbbe8410524f7b4a74d02837e43f2146bab'},
            'unapproved embedding model')
    arms = {}
    per_question = {}
    metrics = ('source_location_hit_at_10', 'source_location_reciprocal_rank_at_10')
    for arm in ARMS:
        rows = [v for k, v in actual.items() if k[0] == arm]
        states = collections.Counter(p['metadata']['ontology']['state'] if p else 'failed' for _, p in rows)
        warm = [r['call']['elapsed_ns'] / 1e6 for k, (r, p) in actual.items()
                if k[0] == arm and k[2] == 'warm_latency' and p]
        per_question[arm] = {}
        families = collections.defaultdict(list)
        for qid, family in FAMILIES.items():
            observations = [location_metric(actual[(arm, qid, 'retrieval', i)][1]['citations'], refs[qid])
                            if actual[(arm, qid, 'retrieval', i)][1] else dict.fromkeys(metrics, 0.0)
                            for i in (1, 2)]
            mean = {m: statistics.mean(r[m] for r in observations) for m in metrics}
            per_question[arm][qid] = {'family': family, **mean}
            families[family].append(mean)
        family_means = {f: {m: statistics.mean(r[m] for r in values) for m in metrics}
                        for f, values in families.items()}
        arms[arm] = {'planned_calls': 96, 'observed_calls': len(rows), 'failed_calls': sum(p is None for _, p in rows),
                     'ontology_states': dict(states), 'warm_n': len(warm), 'warm_median_ms': statistics.median(warm),
                     'warm_p95_ms': percentile(warm, .95), 'per_family': family_means,
                     'family_macro': {m: statistics.mean(r[m] for r in family_means.values()) for m in metrics},
                     'matched_concept_calls': sum(bool(p and p['metadata']['ontology']['matched_concepts']) for _, p in rows),
                     'boosted_citation_calls': sum(bool(p and p['metadata']['ontology']['boosted_citations']) for _, p in rows)}
    changes = {}
    for arm in ARMS[1:]:
        paired = []
        for qid in protocol['question_ids']:
            for i in (1, 2):
                off, on = [actual[(a, qid, 'retrieval', i)][1] for a in ('off', arm)]
                paired.append({'question': qid, 'iteration': i, 'available': bool(off and on),
                               'citation_order_changed': off['citations'] != on['citations'] if off and on else None})
        changes[arm] = {'paired_retrieval_calls': 24,
                        'changed_citation_order_calls': sum(r['citation_order_changed'] is True for r in paired),
                        'pairs': paired,
                        'family_macro_minus_off': {m: arms[arm]['family_macro'][m] - arms['off']['family_macro'][m] for m in metrics},
                        'warm_p95_ratio_to_off': arms[arm]['warm_p95_ms'] / arms['off']['warm_p95_ms']}
    probe = None
    if (folder / 'mechanism-probe').exists():
        probe_protocol = load('mechanism-probe/protocol.json')
        require(probe_protocol['chosen_after_main_results'] is True and probe_protocol['part_of_frozen_384'] is False
                and probe_protocol['quality_evaluated'] is False and probe_protocol['independent_acceptance'] is False,
                'post-observation probe promoted to frozen evaluation')
        request = load('mechanism-probe/requests.json')['requests'][0]
        require(request == {'id': 'OE-X01', 'tool': 'cks.context.get_for_task_v2',
                            'arguments': {'prompt': '쿼럼', 'explain_ontology': True}}, 'mechanism probe differs')
        probe = {'scope': probe_protocol['scope'], 'part_of_frozen_384': False, 'observed_calls': 0,
                 'quality_evaluated': False, 'arms': {}}
        for arm in ARMS:
            report = load('mechanism-probe/' + arm + '.json')
            require(report['state'] == 'captured' and len(report['rows']) == 2
                    and report['counts'] == probe_protocol['counts'] and report['quality_metrics'] is None,
                    'incomplete mechanism probe')
            require(report['request_sha256'] == sha(raw('mechanism-probe/requests.json'))
                    and report['config_sha256'] == report['config_sha256_after'] == sha(raw(arm + '.yaml'))
                    and report['binary_sha256'] == report['binary_sha256_after'] == protocol['binary_sha256'],
                    'probe input pin differs')
            require(len(report['session_closures']) == 1 and not report['session_closures'][0].get('error'),
                    'probe session did not close')
            values = []
            for i, row in enumerate(report['rows']):
                call = row['call']
                require(row['request_id'] == 'OE-X01' and row['phase'] == ('warmup', 'retrieval')[i]
                        and row['iteration'] == 1 and call['tool'] == request['tool']
                        and call['arguments'] == request['arguments'] and not row.get('error')
                        and not call.get('transport_error') and not call['response'].get('isError'), 'invalid probe call')
                ontology = verify_pack(call['response']['structuredContent'], '쿼럼', arm, source)
                values.append(ontology)
                probe['observed_calls'] += 1
            require(all(v['matched_concepts'] == (0 if arm == 'off' else 1) for v in values), 'exact term not matched')
            probe['arms'][arm] = {k: values[1].get(k, 0) for k in ('state', 'matched_concepts', 'applied_relations',
                'boosted_citations', 'text_search_calls', 'text_boosted_citations')}
        require(all(c['exit_code'] == 0 for c in load('mechanism-probe/commands.json')), 'probe command failed')
    return {'scope': protocol['kind'], 'planned_calls': 384, 'observed_calls': len(actual),
            'failed_calls': sum(p is None for _, p in actual.values()), 'missing_calls': 0,
            'citations_verified': citations_verified, 'bodies_verified': bodies_verified,
            'retained_files_verified': len(blob_cache), 'source_coordinates': COORDINATES,
            'arms': arms, 'per_question': per_question, 'comparisons_to_off': changes,
            'positive_questions': 8, 'overlapping_families': 4, 'unscored_controls': 4,
            'quality_verdict': 'inconclusive: AI location proxies, known families, no independent gold or answer scoring',
            'answer_quality_evaluated': False, 'answer_abstention': None, 'precision_recall_evaluated': False,
            'independent_units_approved': 0, 'new_independent_FINAL': 0, 'production_activation': False,
            'default_mode_changed': False, 'multi_project_evaluated': False, 'full_OE01_complete': False,
            'post_observation_mechanism_probe': probe,
            'limitations': protocol['limitations'] + ['model embedding request count/cost not instrumented',
                'missing or failed observations must remain in denominators; no model answers were generated']}


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('folder', type=Path)
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    summary = analyze(args.folder)
    args.out.write_text(json.dumps(summary, ensure_ascii=False, indent=2) + '\n')
    print('verified:', summary['observed_calls'], 'calls;', summary['citations_verified'], 'citations; quality inconclusive')
