#!/usr/bin/env python3
"""Propose an exposed, module-balanced function sample; never approve holdout/gold."""
import argparse
import importlib.util
import itertools
import json
from pathlib import Path
import re

from source_packet import sha

ROOT = Path(__file__).resolve().parents[1]
BASE = Path('docs/spec-driven/operational-preparation/source-review-supplement')
POOL = Path('docs/spec-driven/OPERATIONAL-ACCEPTANCE-INPUT-CANDIDATES.json')
PRIOR = Path('system/eval/stablenet-corpus-repair-2026-10-14/questions-frozen.json')
INPUTS = {'pool': POOL, 'lexical': BASE / 'lexical-coverage-v1/coverage.json',
          'review_public': Path('docs/spec-driven/operational-preparation/review-drafts/public-inputs.json'),
          'review_private': Path('docs/spec-driven/operational-preparation/review-drafts/gold-drafts.json'),
          'expansion': BASE / 'expansion-v1/public-inputs.json',
          'expansion_private': BASE / 'expansion-v1/private-ai-review.json',
          'context': BASE / 'context-v1/private-context-review.json',
          'symptoms': BASE / 'symptom-inputs-v1/private-source-review.json',
          'schedules': BASE / 'schedule-candidates-v1/private-schedules.json',
          'ticket': BASE / 'subscription-ticket-v1/public-ticket.json',
          'dependency': BASE / 'dependency-review-v1/private-review.json', 'prior': PRIOR}
MANIFESTS = [BASE / (name + '/manifest.json') for name in ['lexical-coverage-v1', 'expansion-v1', 'context-v1',
              'symptom-inputs-v1', 'schedule-candidates-v1', 'subscription-ticket-v1', 'dependency-review-v1']]
CONSTRUCTS = ['declaration_redefinition', 'branch', 'loop', 'shadow', 'return']


def load(name):
    spec = importlib.util.spec_from_file_location(name.replace('-', '_'), ROOT / 'scripts' / name)
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


def dump(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2, allow_nan=False) + '\n')


def constructs(row):
    f = row['features']
    return {'declaration_redefinition': row['declaration_and_reassignment'],
            'branch': bool(f.get('branch')), 'loop': bool(f.get('loop')),
            'shadow': bool(f.get('local_variable_shadow')), 'return': bool(f.get('return'))}


def policy():
    return {'kind': 'deterministic_purposive_review_proposal_not_random_or_independent_sampling',
            'target_functions': 30, 'construct_function_floor_proposal': 4,
            'module_quota_rule': 'three per module, then one extra to each of the first six lexically sorted modules',
            'selection_order': ['lowest current selected count for that module',
                                'most constructs still below the proposed floor of four',
                                'prefer a source file not yet selected', 'lowest candidate_id'],
            'excluded_scope': 'development_stress_source_only',
            'performance_or_gold_used_for_selection': False, 'exposure_used_as_freshness_score': False,
            'representativeness': 'not_established', 'sample_sufficiency': 'not_reviewed',
            'independence_approved': False, 'dev_final_allocation': None}


def select(rows):
    eligible = [r for r in rows if r['scope'] == 'production_source_candidate']
    modules = sorted({r['module'] for r in eligible})
    if len(eligible) != 39 or len(modules) != 8:
        raise ValueError('unexpected audited source pool; review selection policy')
    quotas = {module: 3 + int(i < 6) for i, module in enumerate(modules)}
    if any(sum(r['module'] == module for r in eligible) < quota for module, quota in quotas.items()):
        raise ValueError('not enough production functions for module quota')
    picked, trace = [], []
    counts, files, module_counts = {k: 0 for k in CONSTRUCTS}, set(), {m: 0 for m in modules}
    remaining = {r['candidate_id']: r for r in eligible}
    while len(picked) < 30:
        choices = [r for r in remaining.values() if module_counts[r['module']] < quotas[r['module']]]

        def rank(row):
            gain = sum(constructs(row)[k] and counts[k] < 4 for k in CONSTRUCTS)
            return (module_counts[row['module']], -gain, -int(row['source']['path'] not in files), row['candidate_id'])

        chosen = min(choices, key=rank)
        trace.append({'slot': len(picked) + 1, 'candidate_id': chosen['candidate_id'],
                      'eligible_count': len(choices), 'rank': list(rank(chosen)),
                      'construct_counts_before': dict(counts), 'new_source_file': chosen['source']['path'] not in files})
        picked.append(chosen['candidate_id'])
        del remaining[chosen['candidate_id']]
        files.add(chosen['source']['path'])
        module_counts[chosen['module']] += 1
        for key in CONSTRUCTS:
            counts[key] += int(constructs(chosen)[key])
    if any(v < 4 for v in counts.values()) or module_counts != quotas:
        raise ValueError('proposed construct/module coverage not achieved')
    return picked, quotas, counts, trace


def generate(inputs):
    lexical = inputs['lexical']['functions']
    pool = {r['candidate_id']: r for r in inputs['pool']['functions']}
    if len(lexical) != 40 or {r['candidate_id'] for r in lexical} != set(pool) or any(r['source'] != pool[r['candidate_id']] for r in lexical):
        raise ValueError('audited function identities must equal original pool')
    selected, quotas, counts, trace = select(lexical)
    selected_set = set(selected)
    by_id = {r['candidate_id']: r for r in lexical}
    symptoms = {r['candidate_id']: r for r in inputs['symptoms']['reviews']}
    questions = inputs['prior']['questions']
    question_ids = {q['id'] for q in questions}
    if len(questions) != 42 or len(question_ids) != 42:
        raise ValueError('unexpected prior question inventory')
    public_review = {r['candidate_id']: r['case_id'] for r in inputs['review_public']['cases']}
    private_review = {r['candidate_id'] for r in inputs['review_private']['review_drafts']}
    legacy_value_control = {r['candidate_id'] for r in inputs['review_private']['value_control_anchor_drafts']}
    expansion = {r['candidate_id']: r['case_id'] for r in inputs['expansion']['cases']}
    expansion_private = {r['case_id'] for r in inputs['expansion_private']['review_drafts']}
    context = {r['candidate_id'] for r in inputs['context']['reviews']}
    dependency = {r['candidate_id'] for r in inputs['dependency']['contexts']}
    schedule = {}
    for row in inputs['schedules']['families']:
        schedule.setdefault(row['source_candidate_id'], []).append(row['case_id'])
    rows, comparisons = [], []
    for cid, row in sorted(by_id.items()):
        source = row['source']
        symptom = symptoms.get(cid)
        topics = symptom['prior_question_topic_overlap_proposal'] if symptom else None
        if topics is not None and not set(topics) <= question_ids:
            raise ValueError('unknown prior question in topic proposal')
        role = 'selected_proposal' if cid in selected_set else ('reserve_proposal' if row['scope'] == 'production_source_candidate' else 'excluded_development_source_only')
        exposure = {'candidate_pool_material': True, 'source_file_and_lexical_census': True,
                    'legacy_public_review_case': public_review.get(cid), 'legacy_private_ai_review': cid in private_review,
                    'legacy_value_control_anchor_draft': cid in legacy_value_control,
                    'expansion_public_case': expansion.get(cid), 'private_caller_callee_context': cid in context,
                    'expansion_private_ai_facts': expansion.get(cid) in expansion_private,
                    'symptom_public_case': symptom['case_id'] if symptom else None,
                    'symptom_private_source_facts': symptom is not None,
                    'private_dependency_review': cid in dependency, 'schedule_source_proposals': schedule.get(cid, []),
                    'subscription_ticket_same_file': source['path'] == inputs['ticket']['source']['path'],
                    'subscription_ticket_scope': 'file_context_only_not_independent_defect_evidence',
                    'prior_ai_topic_flags': topics,
                    'semantic_decontamination': 'not_reviewed', 'unexposed_holdout': False}
        rows.append({'candidate_id': cid, 'selection_role': role, 'module': row['module'], 'scope': row['scope'],
                     'source': source, 'constructs': constructs(row), 'raw_repeated_name_hint': row['old_name_hint'],
                     'exposure': exposure, 'approved': False, 'reviewer': None, 'independence_status': 'not_reviewed',
                     'split': None, 'final_eligibility': 'not_established', 'exhaustive_gold': None,
                     'quality_scoring_allowed': False})
        if role == 'excluded_development_source_only':
            continue
        for q in questions:
            comparisons.append({'candidate_id': cid, 'prior_question_id': q['id'],
                                'selected_proposal': cid in selected_set,
                                'literal_name_mentioned': bool(re.search(r'\b' + re.escape(source['name']) + r'\b', q['prompt'])),
                                'exact_path_mentioned': source['path'] in q['prompt'],
                                'inherited_ai_topic_overlap_flag': (q['id'] in topics) if topics is not None else None,
                                'topic_review_origin': 'unapproved_symptom_source_note' if topics is not None else 'not_reviewed',
                                'semantic_independence': 'not_reviewed'})
    production = [r for r in rows if r['scope'] == 'production_source_candidate']
    pairs = []
    for a, b in itertools.combinations(production, 2):
        sa, sb = symptoms.get(a['candidate_id']), symptoms.get(b['candidate_id'])
        same_group = bool(sa and sb and sa['family_group_proposal'] == sb['family_group_proposal'])
        pairs.append({'a': a['candidate_id'], 'b': b['candidate_id'],
                      'both_selected': a['candidate_id'] in selected_set and b['candidate_id'] in selected_set,
                      'same_module': a['module'] == b['module'], 'same_source_file': a['source']['path'] == b['source']['path'],
                      'identical_function_bytes': a['source']['function_sha256'] == b['source']['function_sha256'],
                      'shared_unapproved_symptom_group': same_group,
                      'metadata_flags_prove_independence': False, 'semantic_family_review': 'not_reviewed'})
    selected_rows = [r for r in rows if r['candidate_id'] in selected_set]
    groups = {}
    for row in selected_rows:
        groups.setdefault(row['source']['path'], []).append(row['candidate_id'])
    selected_comparisons = [r for r in comparisons if r['selected_proposal']]
    summary = {'kind': 'exposed_function_sample_preparation_not_independent_acceptance',
               'source_commit': inputs['pool']['repository_commit'], 'selected_proposal': selected,
               'reserve_proposal': [r['candidate_id'] for r in rows if r['selection_role'] == 'reserve_proposal'],
               'excluded_development_source_only': [r['candidate_id'] for r in rows if r['selection_role'] == 'excluded_development_source_only'],
               'module_quotas': quotas, 'construct_function_counts': counts,
               'selected_source_file_groups': groups, 'file_groups_are_independent_families': False,
               'counts': {'production_pool': len(production), 'selected_functions': len(selected),
                          'reserve_functions': len(production) - len(selected), 'modules': len(quotas),
                          'selected_source_files': len(groups), 'selected_pairs': sum(p['both_selected'] for p in pairs),
                          'production_pairs': len(pairs), 'selected_same_file_pairs': sum(p['both_selected'] and p['same_source_file'] for p in pairs),
                          'selected_shared_ai_group_pairs': sum(p['both_selected'] and p['shared_unapproved_symptom_group'] for p in pairs),
                          'prior_question_comparisons': len(comparisons), 'selected_prior_question_comparisons': len(selected_comparisons),
                          'selected_literal_name_flags': sum(r['literal_name_mentioned'] for r in selected_comparisons),
                          'selected_inherited_ai_topic_flags': sum(r['inherited_ai_topic_overlap_flag'] is True for r in selected_comparisons),
                          'selected_functions_with_ai_topic_flags': len({r['candidate_id'] for r in selected_comparisons if r['inherited_ai_topic_overlap_flag'] is True}),
                          'selected_functions_without_topic_review': sum(r['exposure']['prior_ai_topic_flags'] is None for r in selected_rows),
                          'selected_private_dependency_reviews': sum(r['exposure']['private_dependency_review'] for r in selected_rows),
                          'selected_without_private_dependency_review': sum(not r['exposure']['private_dependency_review'] for r in selected_rows),
                          'approved_independent_functions': 0, 'independent_gold_records': 0},
               'reviewer': None, 'split_allocation': None, 'formal_sample_sufficiency': 'not_reviewed',
               'formal_runs': 0, 'dependency_backend_calls': 0, 'quality_scores': None,
               'limitations': ['all candidates already appear in the exposed source pool and census',
                              'purposive deterministic quotas do not establish random sampling or representativeness',
                              'same-file groups and AI topic/group flags are review prompts, not family verdicts',
                              'absence of literal/path/topic flags does not establish freshness or independence',
                              'four dependency source reviews are non-exhaustive; the other selected functions still need review',
                              'selected syntax counts do not supply approved independent functions, edge gold, DEV/FINAL or frozen matching']}
    checklist = [{'candidate_id': row['candidate_id'], 'module': row['module'],
                  'new_dependency_source_locations': 'non_exhaustive_prepared' if row['exposure']['private_dependency_review'] else 'not_prepared',
                  'legacy_value_control_anchors': 'non_exhaustive_only' if row['exposure']['legacy_value_control_anchor_draft'] else 'not_prepared',
                  'exhaustive_data_gold': 'not_authored', 'exhaustive_control_gold': 'not_authored',
                  'independent_review': 'not_assigned', 'exact_backend_matching': 'not_frozen',
                  'split': None, 'formal_execution': 'not_run', 'acceptance': 'not_recorded'} for row in selected_rows]
    return {'selection-policy.json': policy(), 'sample-summary.json': summary,
            'function-inventory.json': {'kind': 'private_review_inventory_not_model_input', 'functions': rows},
            'selection-trace.json': trace, 'pairwise-review.json': pairs, 'prior-question-review.json': comparisons,
            'review-checklist.json': checklist}


def table(inventory):
    lines = ['검토용 함수 표본 — 제안이며 승인/DEV/FINAL/gold 아님',
             'ID       역할       모듈             구문(D/B/L/S/R)  함수 및 원문',
             'D=선언+재대입 B=분기 L=루프 S=실제 로컬 shadow R=return']
    for r in inventory['functions']:
        flags = ''.join('Y' if r['constructs'][k] else '-' for k in CONSTRUCTS)
        role = {'selected_proposal': '선정제안', 'reserve_proposal': '예비제안', 'excluded_development_source_only': '개발전용'}[r['selection_role']]
        src = r['source']
        name = (src['receiver'] + '.' if src['receiver'] else '') + src['name']
        lines.append(f"{r['candidate_id']} {role} {r['module']} {flags} {name} {src['path']}:{src['start_line']}")
    return '\n'.join(lines) + '\n'


def prepare(output):
    if output.exists():
        raise ValueError('use a new output; preserve prior evidence')
    errors = load('check-lexical-coverage.py').check(ROOT / BASE / 'lexical-coverage-v1')
    if errors:
        raise ValueError('\n'.join(errors))
    inputs = {key: json.loads((ROOT / path).read_text()) for key, path in INPUTS.items()}
    artifacts = generate(inputs)
    output.mkdir(parents=True)
    for name, data in artifacts.items():
        dump(output / name, data)
    (output / 'REVIEW-TABLE.txt').write_text(table(artifacts['function-inventory.json']))
    dump(output / 'formal-state.json', load('prepare-source-review-supplement.py').formal_state(ROOT))
    (output / 'generator.py.txt').write_bytes(Path(__file__).read_bytes())
    bindings = sorted(set(INPUTS.values()) | set(MANIFESTS))
    dump(output / 'manifest.json', {'schema_version': 1, 'kind': 'function_sample_preparation_only',
         'input_bindings': {str(path): sha((ROOT / path).read_bytes()) for path in bindings},
         'generator_sha256': sha(Path(__file__).read_bytes()), 'counts': artifacts['sample-summary.json']['counts'],
         'artifacts': {p.relative_to(output).as_posix(): sha(p.read_bytes()) for p in sorted(output.iterdir()) if p.is_file()}})
    print(json.dumps(artifacts['sample-summary.json']['counts']), flush=True)


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    prepare(args.output.resolve())
