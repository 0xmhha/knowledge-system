#!/usr/bin/env python3
"""Audit WI-3 exposure, family/gold gaps and scoped retrieval evidence."""
import argparse
from collections import Counter
import importlib.util
import itertools
import json
from pathlib import Path
import re

from source_packet import SourceCapture, safe, sha, verify

ROOT = Path(__file__).resolve().parents[1]
BASE = Path('docs/spec-driven/operational-preparation/source-review-supplement')
SYMPTOMS = BASE / 'symptom-inputs-v1'
WIRE = BASE / 'symptom-wire-preflight-v1'
HOST = Path('system/eval/operational-preparation-2026-10-15/isolated-host')
IMPLEMENTATION = Path('system/eval/upper-work-items-implementation-2026-10-14/wi-3')
CONSUMER = ['plugin/agents/analyzer.md', 'plugin/skills/suspect-labeling/SKILL.md', 'plugin/scripts/suspect_labels.py']
SESSIONS = ['valid-03', 'tools-03', 'incomplete-02', 'tampered-02', 'cancel-01']
INPUTS = {'pool': Path('docs/spec-driven/OPERATIONAL-ACCEPTANCE-INPUT-CANDIDATES.json'),
          'public': SYMPTOMS / 'public-inputs.json', 'private': SYMPTOMS / 'private-source-review.json',
          'wire': WIRE / 'public-bundle/wire-requests.json', 'parser': WIRE / 'summary.json',
          'prior': Path('system/eval/stablenet-corpus-repair-2026-10-14/questions-frozen.json'),
          'review': Path('docs/spec-driven/operational-preparation/review-drafts/public-inputs.json'),
          'expansion': BASE / 'expansion-v1/public-inputs.json',
          'sample': BASE / 'function-sample-v1/function-inventory.json',
          'semantic': BASE / 'semantic-gap-v1/private-function-gaps.json',
          'host': HOST / 'manifest.json', 'consumer_inventory': HOST / 'coding_agent-source-files.json',
          'implementation': IMPLEMENTATION / 'manifest.json',
          'smoke': IMPLEMENTATION / 'execution/summary.json', 'smoke_wire': IMPLEMENTATION / 'execution/wire.json',
          'smoke_host': IMPLEMENTATION / 'host/summary.json',
          'proposals': Path('scripts/fixtures/symptom-gap-proposals.json')}
MANIFESTS = [BASE / p / 'manifest.json' for p in
             ['symptom-inputs-v1', 'symptom-wire-preflight-v1', 'expansion-v1', 'function-sample-v1', 'semantic-gap-v1']]
SOURCE = [Path(p) for p in ['scripts/export-symptom-wire.py', 'internal/system/mcp/suspects.go',
          'internal/system/suspects/compose.go', 'internal/system/ckvclient/real.go',
          'scripts/fixtures/symptom_wire_preflight_test.go.txt', 'bench/suspects/smoke.py', 'pkg/system/contract/suspects.go']]
RECEIPTS = ['handler', 'semantic_search', 'keyword_search', 'graph_walk', 'retained_source',
            'installed_role', 'typed_candidate_artifact', 'analysis_handoff', 'independent_gold', 'recall_at_10']
OBJECT_ATTRIBUTES = b'consumer-source/objects/*.commit binary\nconsumer-source/objects/*.tree binary\n'


def load(name):
    spec = importlib.util.spec_from_file_location(name.replace('-', '_'), ROOT / 'scripts' / name)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


helpers = load('prepare-semantic-gap.py')
dump, unique, span = helpers.dump, helpers.unique, helpers.span


def bindings():
    return sorted(set(INPUTS.values()) | set(MANIFESTS) | set(SOURCE) |
                  {HOST / 'final' / s / 'mcp-frames.jsonl' for s in SESSIONS} |
                  {Path('scripts/prepare-semantic-gap.py')})


def inputs(packet):
    data = {key: json.loads((ROOT / path).read_text()) for key, path in INPUTS.items()}
    for path in MANIFESTS:
        manifest = json.loads((ROOT / path).read_text())
        for artifact, digest in manifest['artifacts'].items():
            if sha(safe(ROOT / path.parent, artifact).read_bytes()) != digest:
                raise ValueError('prior artifact binding: ' + artifact)
    for path in INPUTS.values():
        if path.is_relative_to(IMPLEMENTATION) and path != INPUTS['implementation']:
            if data['implementation']['files_sha256'][path.relative_to(IMPLEMENTATION).as_posix()] != sha((ROOT / path).read_bytes()):
                raise ValueError('historical implementation artifact binding')
    proof = verify(ROOT / SYMPTOMS, data['pool']['repository_commit'])
    pool = unique(data['pool']['functions'], 'candidate_id')
    for r in data['private']['reviews']:
        src = r['source']
        raw = safe(ROOT / SYMPTOMS, proof['files'][src['path']]['artifact']).read_bytes()
        if src != pool[r['candidate_id']] or sha(raw) != src['file_sha256'] or sha(raw[src['start_byte']:src['end_byte_exclusive']]) != src['function_sha256']:
            raise ValueError('symptom source identity')
        for anchor in r['source_anchors']:
            if span(raw, anchor['start_line'], anchor['end_line']) != {k: v for k, v in anchor.items() if k not in ['path', 'source_fact_draft']} or anchor['path'] != src['path']:
                raise ValueError('symptom source anchor')
    revision = data['host']['source_adoptions']['coding_agent']['source_commit']
    proof = verify(packet / 'consumer-source', revision)
    expected = {r['path']: r['sha256'] for r in data['consumer_inventory']}
    if set(proof['files']) != set(CONSUMER):
        raise ValueError('consumer source inventory')
    data['consumer_sources'] = {}
    for path, record in proof['files'].items():
        raw = safe(packet / 'consumer-source', record['artifact']).read_bytes()
        if sha(raw) != expected[path]:
            raise ValueError('adopted consumer source binding')
        data['consumer_sources'][path] = raw
    data['current_sources'] = {str(p): (ROOT / p).read_bytes() for p in SOURCE}
    data['frames'] = {s: [json.loads(line) for line in (ROOT / HOST / 'final' / s / 'mcp-frames.jsonl').read_text().splitlines()] for s in SESSIONS}
    return data


def tool_calls(frames):
    calls = []
    for frame in frames:
        if frame.get('kind') == 'request' and 'raw' in frame:
            message = json.loads(frame['raw'])
            if message.get('method') == 'tools/call':
                calls.append(message['params']['name'])
    return calls


def generate(data):
    cards = unique(data['public']['cases'], 'case_id')
    reviews = unique(data['private']['reviews'], 'case_id')
    requests = unique(data['wire']['requests'], 'case_id')
    if set(cards) != {f'RQ-{i:03d}' for i in range(1, 21)} or set(cards) != set(reviews) or set(cards) != set(requests):
        raise ValueError('twenty paired case inventory')
    if data['wire'] != load('export-symptom-wire.py').render(data['public']):
        raise ValueError('public-only wire mismatch')
    if any(data['private'][k] != 0 for k in ['approved_gold_records', 'approved_independent_families', 'formal_runs', 'retrieval_runs']):
        raise ValueError('private gold/retrieval promotion')
    if any(data['parser'][k] != 0 for k in ['backend_calls', 'retrieval_runs', 'model_calls', 'agent_role_runs', 'formal_runs']):
        raise ValueError('parser promoted to retrieval')
    groups = {}
    rows = []
    review_sources = {r['candidate_id']: r['source'] for r in data['review']['cases']}
    expansion_ids = {r['candidate_id'] for r in data['expansion']['cases']}
    selected_ids = {r['candidate_id'] for r in data['sample']['functions'] if r['selection_role'] == 'selected_proposal'}
    semantic_ids = {r['candidate_id'] for r in data['semantic']['functions']}
    for case_id, card in sorted(cards.items()):
        r = reviews[case_id]
        if r['independent_family_approved'] is not False or r['approved'] is not False or r['gold_locations'] is not None or r['independent_root_cause'] is not None or r['exhaustive_relevance_labels'] is not None or r['canonical_match_rule'] is not None:
            raise ValueError('case gold/independence promotion')
        group = r['family_group_proposal']
        groups.setdefault(group, []).append(case_id)
        observations = [o for o in data['parser']['observations'] if o['case_id'] == case_id]
        if len(observations) != 2 or {o['mode'] for o in observations} != {'normal', 'race'} or any(o['backend_calls'] != 0 or o['seed_mode'] != 'text_only' or o['parser_accepted'] is not True for o in observations):
            raise ValueError('parser observation matrix')
        cid = r['candidate_id']
        rows.append({'case_id': case_id, 'candidate_id': cid, 'symptom_type': card['symptom_type'],
                     'family_proposal': group, 'source': r['source'], 'source_anchors': r['source_anchors'],
                     'source_unknowns': r['unverified_assumptions'],
                     'exposure': {'public_source_pool': True, 'private_ai_source_notes': True,
                         'exact_public_review_case': cid in review_sources,
                         'public_review_same_file_cases': sorted(k for k, s in review_sources.items() if s['path'] == r['source']['path']),
                         'expansion_card': cid in expansion_ids, 'selected_function_sample': cid in selected_ids,
                         'semantic_gap_questions': cid in semantic_ids, 'prior_topic_flags': r['prior_question_topic_overlap_proposal'],
                         'semantic_decontamination': 'not_reviewed', 'fresh_holdout': False},
                     'parser_modes_observed': sorted(o['mode'] for o in observations),
                     'current_case_receipts': {key: None for key in RECEIPTS},
                     'independent_family': None, 'exhaustive_relevance_labels': None,
                     'canonical_location_matching': None, 'gold_locations': None,
                     'reviewer': None, 'review_receipt': None, 'split': None,
                     'quality_scoring_allowed': False, 'root_cause_established': False})
    if groups != data['private']['conservative_family_groups_proposal'] or len(groups) != 15:
        raise ValueError('unreviewed family proposal inventory')
    questions = unique(data['proposals']['families'], 'group')
    if set(questions) != set(groups):
        raise ValueError('family question coverage')
    families = [{'group_proposal': key, 'cases': cases, 'question_draft': questions[key]['question'],
                 'grouping_decision': 'not_reviewed', 'independent_family_count': None,
                 'root_cause_gold': None, 'reviewer': None, 'approved': False} for key, cases in groups.items()]
    pairs = []
    for a, b in itertools.combinations(rows, 2):
        pairs.append({'a': a['case_id'], 'b': b['case_id'], 'same_proposed_group': a['family_proposal'] == b['family_proposal'],
                      'same_module': a['source']['module'] == b['source']['module'],
                      'same_source_file': a['source']['path'] == b['source']['path'],
                      'same_function_bytes': a['source']['function_sha256'] == b['source']['function_sha256'],
                      'exact_prompt_match': cards[a['case_id']]['prompt'] == cards[b['case_id']]['prompt'],
                      'semantic_independence': 'not_reviewed'})
    prior = []
    for row in rows:
        for q in data['prior']['questions']:
            prior.append({'case_id': row['case_id'], 'prior_question_id': q['id'],
                          'candidate_name_mentioned': bool(re.search(r'\b' + re.escape(row['source']['name']) + r'\b', q['prompt'])),
                          'exact_prompt_match': cards[row['case_id']]['prompt'] == q['prompt'],
                          'ai_topic_overlap_flag': q['id'] in row['exposure']['prior_topic_flags'],
                          'semantic_independence': 'not_reviewed'})
    if prior != data['private']['prior_question_comparison']:
        raise ValueError('prior exposure comparison mismatch')
    routes = []
    for r in data['proposals']['routes']:
        source = data['current_sources'] if r['scope'] == 'current_repository' else data['consumer_sources']
        if not r['fact'] or not r['gap']:
            raise ValueError('missing route fact/gap')
        routes.append({**r, 'anchor': span(source[r['path']], *r['lines']),
                       'observation_kind': 'source_inspection_not_new_runtime', 'approved': False})
    if len(unique(routes, 'id')) != 12:
        raise ValueError('route question inventory')
    old_calls = [r['request']['params']['arguments'] for r in data['smoke_wire'] if r['request'].get('method') == 'tools/call']
    if len(old_calls) != 3 or not all(r.get('symbol') or r.get('expand') for r in old_calls):
        raise ValueError('historical symbol/expand smoke scope')
    host_calls = {s: tool_calls(data['frames'][s]) for s in SESSIONS}
    if [t for calls in host_calls.values() for t in calls] != ['chainbench_chains']:
        raise ValueError('historical isolated-host call scope')
    runtime = {'status': 'source_and_prior_evidence_scope_review_not_new_search_run', 'routes': routes,
               'parser_only': {'requests': 20, 'normal_race_observations': len(data['parser']['observations']), 'backend_calls': 0},
               'historical_real_graph_smoke': {'summary': data['smoke'],
                   'coding_agent_revision': data['implementation']['coding_agent_commit'],
                   'core_reference_revision': data['implementation']['source_commit'], 'tool_call_count': len(old_calls),
                   'seed_modes': ['symbol' if r.get('symbol') else 'expand' for r in old_calls],
                   'current_symptom_text_requests_covered': 0, 'historical_http_host': data['smoke_host']},
               'later_isolated_host': {'session_tool_calls': host_calls, 'current_symptom_text_requests_covered': 0,
                   'actual_agent_pipeline_acceptance': data['host']['actual_agent_pipeline_acceptance']},
               'adopted_consumer': {'revision': data['host']['source_adoptions']['coding_agent']['source_commit'],
                   'verified_source_paths': CONSUMER, 'installed_role_execution_established': False},
               'top_10_evaluation_target': None, 'cross_backend_cause_ranking': 'not_established',
               'canonical_matching': None, 'baseline': None, 'case_vs_family_aggregation': None,
               'unknown_and_abstention_denominator_policy': None, 'dev_final_allocation': None,
               'new_backend_calls': 0, 'new_model_calls': 0, 'new_host_sessions': 0, 'new_nodes_started': 0,
               'formal_runs': 0, 'macro_recall_at_10': None}
    counts = {'input_cards': len(rows), 'symptom_type_labels': len({r['symptom_type'] for r in rows}),
              'source_modules': len({r['source']['module'] for r in rows}), 'source_files': len({r['source']['path'] for r in rows}),
              'source_anchors': sum(len(r['source_anchors']) for r in rows), 'family_groups_proposed': len(families),
              'two_card_groups': sum(len(f['cases']) == 2 for f in families), 'proposal_group_gap_to_20_if_retained': 20-len(families),
              'approved_independent_families': 0, 'pending_independent_family_minimum': 20,
              'pairwise_comparisons': len(pairs), 'same_group_pairs': sum(p['same_proposed_group'] for p in pairs),
              'same_file_pairs': sum(p['same_source_file'] for p in pairs), 'same_function_byte_pairs': sum(p['same_function_bytes'] for p in pairs),
              'prior_question_comparisons': len(prior), 'prior_literal_name_flags': sum(p['candidate_name_mentioned'] for p in prior),
              'prior_exact_prompt_flags': sum(p['exact_prompt_match'] for p in prior),
              'prior_ai_topic_flags': sum(p['ai_topic_overlap_flag'] for p in prior),
              'cards_with_prior_ai_topic_flags': sum(bool(r['exposure']['prior_topic_flags']) for r in rows),
              'cards_without_prior_ai_topic_flags': sum(not r['exposure']['prior_topic_flags'] for r in rows),
              'selected_function_sample_overlaps': sum(r['exposure']['selected_function_sample'] for r in rows),
              'exact_public_review_case_overlaps': sum(r['exposure']['exact_public_review_case'] for r in rows),
              'public_review_same_file_card_overlaps': sum(bool(r['exposure']['public_review_same_file_cases']) for r in rows),
              'route_review_anchors': len(routes), 'case_gold_location_sets_missing': len(rows),
              'current_cases_with_text_retrieval_receipts': 0, 'independent_gold_records': 0, 'formal_runs': 0}
    summary = {'kind': 'symptom_family_exposure_gold_and_retrieval_gap_preparation_only', 'counts': counts,
               'symptom_type_distribution': dict(sorted(Counter(r['symptom_type'] for r in rows).items())),
               'family_deficit_is_not_a_split_authorization': True, 'all_inputs_already_exposed': True,
               'independent_family_sufficiency': 'not_reviewed', 'quality_scores': None,
               'missing_gold_is_null_not_empty': True}
    return {'private-case-gaps.json': {'status': 'ai_gap_review_not_independent_gold', 'cases': rows},
            'family-review.json': families, 'pairwise-review.json': pairs, 'prior-question-review.json': prior,
            'retrieval-path-review.json': runtime, 'gap-summary.json': summary,
            'public-bundle/wire-requests.json': data['wire']}


def table(artifacts):
    lines = ['WI-3 가족·노출·정답·검색 경로 공백 — 비공개 준비 자료',
             '20카드/7증상 label/15가족 제안; 독립 가족·gold 0개. 제안 5개 부족은 임의 분리 승인이 아니다.',
             '190쌍 및 과거 질문 840건 비교. literal/exact 불일치·topic flag 부재는 fresh holdout 증거가 아니다.']
    for f in artifacts['family-review.json']:
        lines += ['', f['group_proposal'] + ' ' + ', '.join(f['cases']), f['question_draft']]
    lines += ['', '실제 검색 경로:']
    for r in artifacts['retrieval-path-review.json']['routes']:
        lines += [r['id'] + ' ' + r['fact'], '미충족: ' + r['gap']]
    return '\n'.join(lines) + '\n'


def prepare(output, consumer_repo):
    if output.exists():
        raise ValueError('use a new output; preserve prior evidence')
    for checker, packet in [('check-symptom-inputs.py', SYMPTOMS), ('check-symptom-wire-preflight.py', WIRE)]:
        failures = load(checker).check(ROOT / packet)
        if failures:
            raise ValueError('\n'.join(failures))
    output.mkdir(parents=True)
    host = json.loads((ROOT / INPUTS['host']).read_text())
    capture = SourceCapture(consumer_repo, output / 'consumer-source', host['source_adoptions']['coding_agent']['source_commit'])
    for path in CONSUMER:
        capture.source(path)
    dump(output / 'consumer-source/git-proof.json', capture.proof())
    data = inputs(output)
    artifacts = generate(data)
    for name, value in artifacts.items():
        path = output / name
        path.parent.mkdir(parents=True, exist_ok=True)
        dump(path, value)
    for path, raw in data['current_sources'].items():
        dest = output / 'current-source' / (path + '.txt')
        dest.parent.mkdir(parents=True, exist_ok=True)
        dest.write_bytes(raw)
    (output / 'REVIEW-TABLE.txt').write_text(table(artifacts))
    # Git object payloads are byte-bound evidence, including signed-header spaces.
    (output / '.gitattributes').write_bytes(OBJECT_ATTRIBUTES)
    (output / 'generator.py.txt').write_bytes(Path(__file__).read_bytes())
    dump(output / 'formal-state.json', load('prepare-source-review-supplement.py').formal_state(ROOT))
    dump(output / 'manifest.json', {'schema_version': 1, 'kind': 'symptom_gap_preparation_only',
         'input_bindings': {str(p): sha((ROOT / p).read_bytes()) for p in bindings()},
         'generator_sha256': sha(Path(__file__).read_bytes()), 'counts': artifacts['gap-summary.json']['counts'],
         'artifacts': {p.relative_to(output).as_posix(): sha(p.read_bytes()) for p in sorted(output.rglob('*')) if p.is_file()}})
    print(json.dumps(artifacts['gap-summary.json']['counts']), flush=True)


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--consumer-repo', type=Path, required=True)
    args = parser.parse_args()
    prepare(args.output.resolve(), args.consumer_repo.resolve())
