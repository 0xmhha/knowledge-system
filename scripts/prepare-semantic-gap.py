#!/usr/bin/env python3
"""Audit thirty exposed function proposals for missing semantic gold and matching."""
import argparse
from collections import Counter
import importlib.util
import json
from pathlib import Path

from source_packet import safe, sha, verify

ROOT = Path(__file__).resolve().parents[1]
BASE = Path('docs/spec-driven/operational-preparation/source-review-supplement')
INPUTS = {'pool': Path('docs/spec-driven/OPERATIONAL-ACCEPTANCE-INPUT-CANDIDATES.json'),
          'sample': BASE / 'function-sample-v1/sample-summary.json',
          'inventory': BASE / 'function-sample-v1/function-inventory.json',
          'scope': BASE / 'sample-locations-v1/scope-audit.json',
          'locations': BASE / 'sample-locations-v1/coverage-summary.json',
          'relations': BASE / 'dependency-review-v1/private-review.json',
          'matching': BASE / 'dependency-review-v1/matching-contract.json',
          'legacy': Path('docs/spec-driven/operational-preparation/review-drafts/gold-drafts.json'),
          'proposals': Path('scripts/fixtures/semantic-gap-proposals.json')}
MANIFESTS = [BASE / name / 'manifest.json' for name in
             ['lexical-coverage-v1', 'dependency-review-v1', 'function-sample-v1', 'sample-locations-v1']]
ENGINE = [Path('pkg/graph/valueflow') / name for name in ['cfg.go', 'dependence.go', 'query.go']]
REQUIREMENTS = ['independent_family_and_split', 'value_kind_and_role', 'complete_reaching_definition_sets',
                'complete_normal_exit_control_gold', 'memory_call_and_range_scope',
                'abnormal_exit_and_nontermination_scope', 'exact_occurrence_and_node_matching',
                'baseline_limits_and_unknown_denominators']


def load(name):
    spec = importlib.util.spec_from_file_location(name.replace('-', '_'), ROOT / 'scripts' / name)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def dump(path, data):
    path.write_text(json.dumps(data, ensure_ascii=False, indent=2, allow_nan=False) + '\n')


def unique(rows, key):
    result = {r[key]: r for r in rows}
    if len(result) != len(rows):
        raise ValueError('duplicate ' + key)
    return result


def span(raw, start, end):
    lines = raw.splitlines(keepends=True)
    if not 1 <= start <= end <= len(lines):
        raise ValueError('source line range')
    begin, finish = sum(map(len, lines[:start-1])), sum(map(len, lines[:end]))
    return {'start_line': start, 'end_line': end, 'start_byte': begin,
            'end_byte_exclusive': finish, 'span_sha256': sha(raw[begin:finish])}


def source_inputs():
    data = {key: json.loads((ROOT / path).read_text()) for key, path in INPUTS.items()}
    for path in MANIFESTS:
        manifest = json.loads((ROOT / path).read_text())
        for artifact, digest in manifest['artifacts'].items():
            if sha(safe(ROOT / path.parent, artifact).read_bytes()) != digest:
                raise ValueError('prior artifact binding: ' + artifact)
        for key, source in INPUTS.items():
            if source.is_relative_to(path.parent):
                if manifest['artifacts'][source.relative_to(path.parent).as_posix()] != sha((ROOT / source).read_bytes()):
                    raise ValueError('prior input provenance: ' + key)
    proof = verify(ROOT / BASE / 'lexical-coverage-v1', data['pool']['repository_commit'])
    source_files = {}
    for f in data['scope']['functions']:
        src = f['source']
        record = proof['files'][src['path']]
        raw = safe(ROOT / BASE / 'lexical-coverage-v1', record['artifact']).read_bytes()
        if sha(raw) != src['file_sha256'] or record['git_blob'] != src['git_blob'] or sha(raw[src['start_byte']:src['end_byte_exclusive']]) != src['function_sha256']:
            raise ValueError('selected function Git/source identity')
        source_files[src['path']] = raw
    data['source_files'] = source_files
    data['engine_files'] = {str(path): (ROOT / path).read_bytes() for path in ENGINE}
    return data


def generate(data):
    selected = sorted(data['sample']['selected_proposal'])
    audits = unique(data['scope']['functions'], 'candidate_id')
    inventory = unique(data['inventory']['functions'], 'candidate_id')
    pool = unique(data['pool']['functions'], 'candidate_id')
    proposals = unique(data['proposals']['functions'], 'candidate_id')
    if len(selected) != 30 or len(set(selected)) != 30 or set(selected) != set(audits) or set(selected) != set(proposals):
        raise ValueError('selected thirty function inventory')
    relations = data['relations']
    if any(relations[k] != 0 for k in ['approved_independent_functions', 'independent_gold_records', 'formal_runs', 'dependency_backend_calls']):
        raise ValueError('relation/gold promotion')
    contexts = unique(relations['contexts'], 'candidate_id')
    legacy = unique(data['legacy']['value_control_anchor_drafts'], 'candidate_id')
    if set(contexts) != {'SRC-002', 'SRC-006', 'SRC-011', 'SRC-036'} or set(legacy) != {'SRC-007', 'SRC-031', 'SRC-036'}:
        raise ValueError('prior manual review inventory')
    rows = []
    for cid in selected:
        audit, inv, src, proposal = audits[cid], inventory[cid], pool[cid], proposals[cid]
        if audit['source'] != src or inv['source'] != src or inv['scope'] != 'production_source_candidate':
            raise ValueError('selected source/scope identity')
        if audit['gold'] is not False or audit['independence_approved'] is not False or audit['exhaustive_data_edges'] is not None or audit['exhaustive_control_edges'] is not None:
            raise ValueError('audit/gold promotion')
        raw = data['source_files'][src['path']]
        anchor = span(raw, *proposal['lines'])
        if not src['start_line'] <= anchor['start_line'] <= anchor['end_line'] <= src['end_line']:
            raise ValueError('proposal outside selected function')
        if not proposal['fact'] or not proposal['question']:
            raise ValueError('missing semantic note/question')
        local, quarantine = [], []
        bindings = {}
        roles = Counter()
        for i, occurrence in enumerate(audit['occurrence_audit']):
            o = occurrence['occurrence']
            start, end = o['site']['start_byte'], o['site']['end_byte_exclusive']
            if not src['start_byte'] <= start < end <= src['end_byte_exclusive'] or raw[start:end].decode() != o['name']:
                raise ValueError('occurrence source identity')
            if occurrence['value_kind'] is not None or occurrence['gold'] is not False:
                raise ValueError('occurrence value kind/gold promotion')
            if occurrence['status'] == 'corroborated_own_function_variable':
                if len(occurrence['possible_lexical_bindings']) != 1:
                    raise ValueError('ambiguous local binding')
                b = occurrence['possible_lexical_bindings'][0]
                if b['site']['start_byte'] != o['declaration']['start_byte']:
                    raise ValueError('declaration identity mismatch')
                bindings[b['id']] = b
                local.append(i)
                roles[o['role']] += 1
            elif occurrence['status'] == 'quarantined_non_body_signature_scope':
                quarantine.append(i)
            else:
                raise ValueError('unresolved scope cannot silently become local')
        markers = audit['syntax_markers']
        marker_counts = dict(sorted(Counter(m['kind'] for m in markers).items()))
        previous = {key: [r['id'] for r in relations[key] if r['candidate_id'] == cid]
                    for key in ['data_reviews', 'exclusion_reviews', 'boundaries']}
        for r in [r for group in ['data_reviews', 'exclusion_reviews', 'boundaries'] for r in relations[group] if r['candidate_id'] == cid]:
            if r['approved'] is not False or r['gold'] is not False:
                raise ValueError('prior proposal approval')
        old = legacy.get(cid)
        rows.append({'candidate_id': cid, 'module': inv['module'], 'source': src, 'constructs': inv['constructs'],
                     'source_observation_refs': {'packet': str(BASE / 'sample-locations-v1'),
                         'local_occurrence_indices': local, 'quarantined_signature_indices': quarantine},
                     'local_occurrence_role_counts': dict(sorted(roles.items())),
                     'binding_inventory': [{'binding': b, 'value_kind': None, 'independent_review': 'not_assigned'}
                                           for _, b in sorted(bindings.items())],
                     'syntax_profile': {'markers': marker_counts,
                         'range_bindings': audit['limitations']['range_bindings'],
                         'excluded_closures': audit['limitations']['excluded_closures'],
                         'zero_explicit_result_returns': [m for m in markers if m['kind'] == 'return' and m['explicit_results'] == 0],
                         'named_result_implicit_uses': 'not_determined_from_zero_result_marker',
                         'panic_syntax': audit['limitations']['panic_syntax'],
                         'syntax_proves_runtime_or_postdominators': False},
                     'prior_ai_material': {'dependency_review': cid in contexts, **previous,
                         'legacy_statement_anchors': {'available': old is not None,
                             'data_count': len(old['data_anchors']) if old else 0,
                             'control_count': len(old['control_anchors']) if old else 0,
                             'occurrence_or_node_gold': False}, 'all_material_is_non_exhaustive': True},
                     'semantic_review_draft': {'origin': 'ai_manual_source_reading_not_gold', 'anchor': anchor,
                         'fact_draft': proposal['fact'], 'review_question': proposal['question'],
                         'approved': False, 'reviewer': None},
                     'requirements': {key: 'not_satisfied' for key in REQUIREMENTS},
                     'data_gold': None, 'control_gold': None, 'excluded_or_unknown_gold': None,
                     'empty_gold_is_not_missing_gold': True, 'canonical_backend_mapping': None,
                     'reviewer': None, 'review_receipt': None, 'independence_approved': False,
                     'split': None, 'scoring_allowed': False})
    engine = []
    for r in data['proposals']['engine_reviews']:
        if r['path'] not in data['engine_files'] or not r['proposal'] or not r['unresolved']:
            raise ValueError('engine review scope')
        engine.append({**r, 'anchor': span(data['engine_files'][r['path']], *r['lines']),
                       'status': 'source_inspected_proposal_not_frozen', 'approved': False,
                       'reviewer': None, 'receipt': None})
    if len(unique(engine, 'id')) != 14 or {r['id'] for r in engine} != {f'M{i:02d}' for i in range(1, 15)}:
        raise ValueError('matching review inventory')
    counts = {'selected_functions': len(rows), 'modules': len({r['module'] for r in rows}),
              'source_files': len({r['source']['path'] for r in rows}),
              'local_occurrences': sum(len(r['source_observation_refs']['local_occurrence_indices']) for r in rows),
              'quarantined_signature_occurrences': sum(len(r['source_observation_refs']['quarantined_signature_indices']) for r in rows),
              'local_bindings_needing_value_kind_review': sum(len(r['binding_inventory']) for r in rows),
              'prior_dependency_review_functions': len(contexts), 'without_prior_dependency_relations': len(rows)-len(contexts),
              'prior_data_proposals': len(relations['data_reviews']), 'prior_exclusion_proposals': len(relations['exclusion_reviews']),
              'prior_exit_owner_proposals': len(relations['boundaries']), 'legacy_statement_anchor_functions': len(legacy),
              'unique_functions_with_any_prior_relation_material': len(set(contexts) | set(legacy)),
              'without_any_prior_relation_material': len(rows)-len(set(contexts) | set(legacy)),
              'new_semantic_questions': len(rows), 'matching_contract_questions': len(engine),
              'functions_with_range_bindings': sum(r['syntax_profile']['range_bindings'] > 0 for r in rows),
              'functions_with_excluded_closures': sum(bool(r['syntax_profile']['excluded_closures']) for r in rows),
              'functions_with_select_markers': sum(r['syntax_profile']['markers'].get('select', 0) > 0 for r in rows),
              'functions_with_panic_syntax': sum(bool(r['syntax_profile']['panic_syntax']) for r in rows),
              'functions_with_zero_result_return_markers': sum(bool(r['syntax_profile']['zero_explicit_result_returns']) for r in rows),
              'functions_missing_complete_data_gold': len(rows), 'functions_missing_complete_control_gold': len(rows),
              'functions_without_frozen_matching': len(rows), 'approved_independent_functions': 0,
              'independent_gold_records': 0, 'new_dependency_backend_calls': 0, 'mcp_calls': 0,
              'model_calls': 0, 'nodes_started': 0, 'formal_runs': 0}
    if counts['local_occurrences'] != 1721 or counts['quarantined_signature_occurrences'] != 4:
        raise ValueError('source occurrence denominator changed')
    summary = {'kind': 'semantic_gap_inventory_not_complete_gold_or_acceptance', 'counts': counts,
               'construct_function_counts': {key: sum(r['constructs'][key] for r in rows) for key in rows[0]['constructs']},
               'material_scope': 'selected source proposals, unapproved manual questions and implementation contract inspection',
               'independence_or_freshness_established': False, 'quality_scores': None,
               'missing_gold_encoded_as_null_not_empty_set': True,
               'comment_implementation_mismatch': {'implementation_review': 'M04', 'comment_review': 'M05',
                   'implementation_coordinate': 'return_statement', 'comment_coordinate': 'declaration',
                   'gold_mapping_frozen': False, 'runtime_defect_proven': False, 'maintained_go_changed': False}}
    contract = {'status': 'source_grounded_matching_questions_not_frozen_or_executed', 'approved': False,
                'reviews': engine, 'legacy_matching_proposal': data['matching'],
                'evaluation_units_must_remain_separate': ['occurrence_def_use', 'node_dependence_edges', 'normal_exit_control_edges'],
                'gold_manifest': None, 'canonical_matching_frozen': False, 'baseline': None,
                'dev_final_allocation': None, 'resource_budget': None, 'unknown_denominator_policy': None,
                'scoring_allowed': False}
    return {'private-function-gaps.json': {'status': 'ai_source_review_not_independent_gold', 'functions': rows},
            'matching-gap-review.json': contract, 'gap-summary.json': summary}


def table(artifacts):
    lines = ['WI-1 함수 의미·정답·매칭 공백 — 비공개 AI 검토 초안',
             '30개 위치 관측은 완전한 의미 검토/정답이 아니다. data/control gold 30개 모두 미작성.',
             '기존 관계 초안 4함수·statement anchor 3함수, 합집합 6함수. 독립 gold 0개.',
             '새 원문 질문 30개와 구현 계약 질문 14개. missing gold=null; unknown/미관측을 음성으로 세지 않는다.']
    for r in artifacts['private-function-gaps.json']['functions']:
        src, profile, prior, note = r['source'], r['syntax_profile'], r['prior_ai_material'], r['semantic_review_draft']
        lines += ['', f"{r['candidate_id']} {r['module']} {src['name']} {src['path']}:{src['start_line']}",
                  f"기존 관계 초안: {len(prior['data_reviews'])} / statement anchor: {prior['legacy_statement_anchors']['available']} / range bindings: {profile['range_bindings']} / 제외 closure: {len(profile['excluded_closures'])}",
                  note['fact_draft'], '미충족 판단: ' + note['review_question']]
    lines += ['', '매칭·지원 범위 질문:']
    for r in artifacts['matching-gap-review.json']['reviews']:
        lines += [r['id'] + ' ' + r['topic'] + ': ' + r['unresolved']]
    return '\n'.join(lines) + '\n'


def bindings():
    return sorted(set(INPUTS.values()) | set(MANIFESTS) | set(ENGINE))


def prepare(output):
    if output.exists():
        raise ValueError('use a new output; preserve prior evidence')
    # Replay the existing trusted source observer; never execute project function bodies.
    failures = load('check-sample-locations.py').check(ROOT / BASE / 'sample-locations-v1')
    if failures:
        raise ValueError('\n'.join(failures))
    data = source_inputs()
    artifacts = generate(data)
    output.mkdir(parents=True)
    for name, value in artifacts.items():
        dump(output / name, value)
    (output / 'REVIEW-TABLE.txt').write_text(table(artifacts))
    for name, raw in data['engine_files'].items():
        path = output / 'engine-source' / (name + '.txt')
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(raw)
    raw = Path(__file__).read_bytes()
    (output / 'generator.py.txt').write_bytes(raw)
    dump(output / 'formal-state.json', load('prepare-source-review-supplement.py').formal_state(ROOT))
    dump(output / 'manifest.json', {'schema_version': 1, 'kind': 'semantic_gap_preparation_only',
         'input_bindings': {str(path): sha((ROOT / path).read_bytes()) for path in bindings()},
         'generator_sha256': sha(raw), 'counts': artifacts['gap-summary.json']['counts'],
         'artifacts': {p.relative_to(output).as_posix(): sha(p.read_bytes()) for p in sorted(output.rglob('*')) if p.is_file()}})
    print(json.dumps(artifacts['gap-summary.json']['counts']), flush=True)


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output', type=Path, required=True)
    prepare(parser.parse_args().output.resolve())
