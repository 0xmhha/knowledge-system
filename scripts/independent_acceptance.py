"""Read-only preparation checks. A structural pass is never semantic acceptance."""
import hashlib
import json
import math
import re
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
BASE = Path('docs/spec-driven')
SUPPLEMENT = BASE / 'operational-preparation/source-review-supplement'
PACKET = SUPPLEMENT / 'followups/independent-freeze-v1'
WORK_IDS = ('WI-1', 'WI-2', 'WI-3', 'WI-4', 'WI-V1', 'WI-V2', 'WI-6', 'WI-SP', 'WI-C')
INPUTS = (
    ('WI-1', 'semantic-gap-v1/private-function-gaps.json', 'functions'),
    ('WI-3', 'symptom-gap-v1/private-case-gaps.json', 'cases'),
    ('WI-4', 'schedule-gap-v1/private-family-gaps.json', 'families'),
    ('WI-V1/WI-V2', 'ticket-gap-v1/private-ticket-gaps.json', None),
    ('WI-2:controlled', 'rule-gap-v1/private-sample-gaps.json', 'controlled'),
    ('WI-2:project', 'rule-gap-v1/private-sample-gaps.json', 'project'),
    ('WI-6', 'expansion-gap-v1/private-family-pair-gaps.json', 'cases'),
    ('WI-SP', 'policy-security-gap-v1/private-symbol-gaps.json', 'cases'),
    ('WI-C', 'review-gap-v1/private-review-gaps.json', 'cases'),
)

# Exhaustive answer sets are required within an explicitly bounded universe.
# Null means unlabelled. An empty list needs a reviewed empty-set rationale.
GOLD_SETS = {
    'WI-1': ['data_edges', 'control_edges', 'excluded_or_unknown_occurrences'],
    'WI-2': ['rule_candidates', 'negative_locations', 'unsupported_or_unknown_flows'],
    'WI-3': ['relevant_locations', 'irrelevant_locations', 'unknown_locations'],
    'WI-4': ['legal_outcomes', 'symptom_outcomes', 'infra_or_unreached_outcomes'],
    'WI-V1': ['primary_oracle_assertions', 'derived_oracle_assertions', 'normal_controls', 'invalid_mutants'],
    'WI-V2': ['acceptable_candidates', 'unacceptable_candidates', 'unknown_candidates'],
    'WI-6': ['relevant_functions', 'noise_functions', 'unknown_functions'],
    'WI-SP': ['policy_annotations', 'security_annotations', 'untagged_symbols', 'refusal_controls'],
    'WI-C': ['required_items', 'optional_items', 'unsupported_claims', 'priorities', 'artifact_obligations'],
    'ONTOLOGY': ['relevant_locations', 'irrelevant_locations', 'unknown_locations', 'concept_links', 'negative_scope_controls'],
}
MATCHING = {
    'WI-1': 'project/version/file hash + UTF-8 half-open byte spans + lexical binding/definition/use + edge kind; explicit return/pseudo-node mapping; deduplicate before TP/FP/FN; may-reach, not feasibility',
    'WI-2': 'rule id + resolved caller/callee + source/sink/binding/condition spans + supported scope; candidate is not vulnerability or safety; controlled/project denominators separate',
    'WI-3': 'project/version/source mode/file hash + declared symbol/span equivalence; no substring or name-only match; enumerate bounded relevant universe and unknowns before retrieval',
    'WI-4': 'ticket/family + actual entry trace + legal schedule/outcome oracle + binary/driver/seed/order; planned/valid/symptom/infra/unreached all retained; zero baseline is inconclusive',
    'WI-V1': 'ticket requirement + actual API/transport + candidate diff hash + primary/independently derived oracle + normal/reintroduction/overfit controls; no pure-method replacement',
    'WI-V2': 'same ticket/candidate universe as V1 + independently reviewed acceptable membership; all-bad/tie/unknown explicit; missing set is not approved empty set',
    'WI-6': 'same retained seed/snapshot/caps/order; function-level bounded relevance/noise/unknown sets; seed policy explicit; no ground truth from returned hits; empty/undefined metric policy frozen',
    'WI-SP': 'project/version + canonical symbol + exact policy/security id + manual/automatic origin + YAML/reviewer provenance + ACL/private/spoof/stale context; policy-only scan cannot prove no security tags',
    'WI-C': 'case/source/requirement hash + required-item identity/evidence + independent severity/priority + typed sections/diagram/handoff/installed role receipts; section presence alone is not completeness',
    'ONTOLOGY': 'same project/version/source universe/model/preprocessing/top-k/budget; reviewed concept-link scope and applicability; independent semantic relevance, not concept-name substring or returned-hit gold',
}


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def read(path):
    return json.loads(path.read_text())


def canonical(value):
    return json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(',', ':')).encode()


def registry(root=ROOT):
    records = []
    for group, relative, key in INPUTS:
        path = SUPPLEMENT / relative
        data = read(root / path)
        rows = data[key] if key else [data]
        for index, row in enumerate(rows):
            identifier = row.get('case_id') or row.get('candidate_id') or row.get('id') or row.get('call_id')
            # Project rule call ids distinguish two rules applied to one function.
            if group == 'WI-2:project':
                identifier = row['call_id']
            records.append({
                'record_id': group + ':' + identifier,
                'work_item_ids': group.split(':')[0].split('/') if group != 'WI-V1/WI-V2' else ['WI-V1', 'WI-V2'],
                'stratum': group,
                'origin': {'path': str(path), 'sha256': digest(root / path), 'row_key': key, 'row_index': index},
                'row_sha256': hashlib.sha256(canonical(row)).hexdigest(),
                'source': row.get('source'),
                'family_proposal': row.get('family_proposal') or row.get('mechanism_proposal'),
                'development_use': 'exposed_DEV_authoring_material_only',
                'formal_split': None, 'eligible_fresh_FINAL': False,
                'independent_family': None, 'complete_gold': False,
                'independent_review_receipt': None,
            })
    return records


def binding_paths():
    sources = {str(SUPPLEMENT / relative) for _, relative, _ in INPUTS}
    sources.update(str(SUPPLEMENT / relative.split('/')[0] / 'gap-summary.json') for _, relative, _ in INPUTS)
    sources.update(str(SUPPLEMENT / 'followups/stablenet-comparison-v1' / name) for name in ('manifest.json', 'source-binding.json', 'result.json'))
    sources.add('system/eval/operational-preparation-2026-10-16/readiness-refresh/manifest.json')
    tools = ['scripts/' + name for name in ('independent_acceptance.py', 'prepare-independent-freeze.py',
                                           'check-independent-freeze.py', 'test_independent_acceptance.py',
                                           'check-operational-acceptance.py', 'test_operational_acceptance.py')]
    return sorted(sources), tools


def native_reuse_record(root=ROOT):
    previous = root / SUPPLEMENT / 'followups/stablenet-comparison-v1'
    binding = read(previous / 'source-binding.json')
    result = read(previous / 'result.json')
    changed = [relative for relative, expected in {**binding['source_sha256'], **binding['approved_input_sha256']}.items()
               if digest(root / relative) != expected]
    return {'source_commit': '38dd5c5449b1e57759e2532090dce101b3ad9124',
            'bound_source_files': len(binding['source_sha256']),
            'bound_Go_files': sum(v.endswith('.go') for v in binding['source_sha256']),
            'approved_inputs': len(binding['approved_input_sha256']), 'changed_bound_files': changed,
            'fresh_Go_full_tests_this_stage': False, 'prior_general_pass': result['native_test']['pass'],
            'prior_race_pass': result['native_race']['pass'], 'prior_skips_each': result['native_test']['skip'],
            'evidence_manifest': str(SUPPLEMENT / 'followups/stablenet-comparison-v1/manifest.json')}


def authoring_index(records, root=ROOT):
    refs = []
    for record in records:
        origin = record['origin']
        data = read(root / origin['path'])
        row = data[origin['row_key']][origin['row_index']] if origin['row_key'] else data
        refs.append({
            'record_id': record['record_id'], 'private_draft_row': origin,
            'source_grounded_ai_draft_fields': [key for key in row if any(
                hint in key for hint in ('anchor', 'gold', 'question', 'prior_ai', 'oracle', 'obligation', 'semantic_review', 'matching'))],
            'gold_form_ids': record['work_item_ids'],
            'complete_gold': False, 'semantic_authoring_and_review_remaining': True,
        })
    return refs


def gold_form(work_id):
    return {
        'work_item_id': work_id, 'status': 'authoring_form_not_gold',
        'case_id': None, 'family_id': None, 'source_pins': None,
        'universe': None, 'completeness_review': None,
        'answer_sets': dict.fromkeys(GOLD_SETS[work_id]),
        'empty_set_rationales': {}, 'matching_policy': None,
        'matching_contract_proposal': MATCHING[work_id],
        'unknown_and_abstention_policy': None, 'denominator_policy': None,
        'author': None, 'independent_review_receipt': None,
        'public_input_sha256': None,
        'host_access_policy': 'public_input_only_no_gold_or_private_review_paths',
    }


def gold_errors(gold):
    """Structural authoring check; cannot prove semantic truth or independence."""
    errors = []
    work_id = gold.get('work_item_id')
    if work_id not in GOLD_SETS:
        return ['unknown work item']
    for name in ('case_id', 'family_id', 'source_pins', 'universe', 'completeness_review',
                 'matching_policy', 'unknown_and_abstention_policy', 'denominator_policy', 'author',
                 'independent_review_receipt', 'public_input_sha256'):
        if not gold.get(name):
            errors.append('missing ' + name)
    sets = gold.get('answer_sets', {})
    if not isinstance(sets, dict) or set(sets) != set(GOLD_SETS[work_id]):
        return errors + ['answer-set schema differs']
    universe = gold.get('universe')
    units = universe.get('unit_ids') if isinstance(universe, dict) else None
    if not isinstance(units, list) or not units or len(set(units)) != len(units):
        errors.append('bounded universe missing or duplicated')
        units = []
    covered = []
    for name, values in sets.items():
        if not isinstance(values, list):
            errors.append('unlabelled answer set ' + name)
            continue
        if not values and not gold.get('empty_set_rationales', {}).get(name):
            errors.append('empty set lacks reviewed rationale ' + name)
        ids = []
        for row in values:
            if not isinstance(row, dict) or not all(row.get(k) for k in ('id', 'source_ref', 'rationale')):
                errors.append('answer lacks identity/source/rationale ' + name)
                continue
            ids.append(row['id'])
            if row['id'] not in units:
                errors.append('answer outside declared universe ' + name)
        if len(set(ids)) != len(ids):
            errors.append('duplicate answer ' + name)
        covered.extend(ids)
    if set(covered) != set(units) or len(set(covered)) != len(covered):
        errors.append('universe has unlabelled or multiply labelled units')
    receipt = gold.get('independent_review_receipt')
    if not isinstance(receipt, dict) or not all(receipt.get(k) for k in ('name', 'role', 'artifact_sha256', 'independence_declaration', 'evidence_path')) or receipt.get('kind') != 'human' or receipt.get('decision') != 'approved':
        errors.append('human review receipt incomplete')
    elif receipt['artifact_sha256'] != gold_content_hash(gold):
        errors.append('review receipt is not bound to current gold content')
    if not re.fullmatch('[0-9a-f]{64}', gold.get('public_input_sha256') or ''):
        errors.append('public input digest invalid')
    if gold.get('host_access_policy') != 'public_input_only_no_gold_or_private_review_paths':
        errors.append('private gold exposure policy differs')
    return errors


def gold_content_hash(gold):
    return hashlib.sha256(canonical({k: v for k, v in gold.items() if k != 'independent_review_receipt'})).hexdigest()


def location_key(location):
    """Names or node ids alone cannot identify a location across projects."""
    fields = ('project_id', 'dataset_version', 'source_mode', 'path', 'file_sha256', 'start_byte', 'end_byte_exclusive', 'kind')
    values = tuple(location[k] for k in fields)
    if (not all(isinstance(v, str) and v for v in values[:5]) or
            any(isinstance(v, bool) or not isinstance(v, int) for v in values[5:7]) or
            not 0 <= values[5] < values[6] or not isinstance(values[7], str) or not values[7] or
            not re.fullmatch('[0-9a-f]{64}', values[4])):
        raise ValueError('invalid scoped location')
    return values


def split_errors(cases):
    errors, family_splits, fingerprints = [], {}, {}
    for case in cases:
        split, family = case.get('split'), case.get('family_id')
        if split not in ('DEV', 'FINAL') or not family:
            errors.append('unassigned split/family')
            continue
        family_splits.setdefault(family, set()).add(split)
        if split == 'FINAL' and case.get('exposed_before_freeze') is not False:
            errors.append('exposed or unknown FINAL exposure')
        # Exact-byte duplicates supplement, never establish, semantic-family review.
        for field in ('source_function_sha256', 'public_input_sha256'):
            value = case.get(field)
            if value:
                fingerprints.setdefault((field, value), set()).add(split)
        if not case.get('semantic_family_review_receipt'):
            errors.append('semantic family independence unreviewed')
    if any(len(values) > 1 for values in family_splits.values()):
        errors.append('family leaks across DEV/FINAL')
    if any(len(values) > 1 for values in fingerprints.values()):
        errors.append('exact input/source leaks across DEV/FINAL')
    return errors


def sufficiency(cases, minimum):
    """Repeated runs, translation variants and multiple edges never add families."""
    units = {c.get('family_id') for c in cases if c.get('split') == 'FINAL'
             and c.get('independent_reviewed') is True and c.get('family_id')
             and c.get('exposed_before_freeze') is False}
    enough = isinstance(minimum, int) and not isinstance(minimum, bool) and minimum > 0 and len(units) >= minimum
    return {'independent_FINAL_families': len(units), 'declared_minimum': minimum,
            'count_floor_met': enough, 'verdict': 'requires_precision_and_coverage_review' if enough else 'inconclusive'}


def nearest_rank(samples, fraction=0.95):
    if (not samples or not 0 < fraction <= 1 or
            any(isinstance(v, bool) or not isinstance(v, (float, int)) or not math.isfinite(v) or v < 0 for v in samples)):
        raise ValueError('finite nonnegative raw times required')
    return sorted(samples)[math.ceil(fraction * len(samples)) - 1]


def ontology_verdict(delta_recall, ci_lower, delta_precision, latency_ratio,
                     contamination, citation_fraction, sufficient, reviewed_links,
                     paired_pins_match, final_predeclared, final_mode='combined'):
    values = [delta_recall, ci_lower, delta_precision, latency_ratio, contamination, citation_fraction]
    if any(isinstance(v, bool) or not isinstance(v, (int, float)) or not math.isfinite(v) for v in values):
        return 'inconclusive'
    if not (-1 <= delta_recall <= 1 and -1 <= ci_lower <= 1 and -1 <= delta_precision <= 1
            and latency_ratio > 0 and contamination >= 0 and 0 <= citation_fraction <= 1):
        return 'inconclusive'
    if contamination != 0 or citation_fraction != 1 or not paired_pins_match:
        return 'boundary_failure_keep_off'
    if final_mode not in ('concept_text', 'relations', 'combined'):
        return 'inconclusive'
    if not all(v is True for v in (sufficient, final_predeclared)) or (final_mode in ('relations', 'combined') and reviewed_links is not True):
        return 'inconclusive'
    if delta_recall >= .05 and ci_lower > 0 and delta_precision >= -.02 and latency_ratio <= 1.25:
        return 'proposed_retrieval_gate_met_requires_human_acceptance'
    return 'no_demonstrated_benefit_keep_off'


def index_errors(documents, reference, state):
    errors = []
    for document in documents:
        if document.get('independent_freeze_preparation') != reference:
            errors.append('current preparation reference missing, changed or promoted')
        if document.get('current_execution_state') != state:
            errors.append('current progress differs from prepared worklist')
    return errors


def current_index_errors(root=ROOT):
    packet = root / PACKET
    reference = {'manifest': str(PACKET / 'manifest.json'), 'sha256': digest(packet / 'manifest.json'),
                 'status': 'authoring_prepared_not_frozen_not_accepted',
                 'report': str(PACKET / 'REPORT.txt'), 'worklist': str(PACKET / 'WORKLIST.json')}
    paths = [BASE / ('OPERATIONAL-ACCEPTANCE-' + name + '.json') for name in ('PROTOCOL', 'PROGRESS', 'DECISION-PROPOSAL')]
    paths.append(SUPPLEMENT / 'GAPS.json')
    documents = [read(root / p) for p in paths]
    errors = index_errors(documents, reference, read(packet / 'WORKLIST.json'))
    proposal_ref = documents[0].get('adoption_acceptance_decision_proposal')
    if any(d.get('adoption_acceptance_decision_proposal') != proposal_ref for d in (documents[1], documents[3])) or not proposal_ref or proposal_ref['sha256'] != digest(root / paths[2]):
        errors.append('proposal linkage differs')
    if documents[0]['freeze'] is not None or documents[0]['execution_allowed'] is not False:
        errors.append('unassigned preparation cannot enable formal execution')
    return errors


def packet_errors(root=ROOT, packet=None):
    packet = packet or root / PACKET
    errors = []
    try:
        manifest = read(packet / 'manifest.json')
        sources, tools = binding_paths()
        if (manifest.get('kind') != 'authoring_preparation_only_not_acceptance'
                or [r['path'] for r in manifest['source_refs']] != sources
                or [r['path'] for r in manifest['tool_refs']] != tools):
            errors.append('manifest scope or required bindings differ')
        if any(p.is_symlink() for p in packet.rglob('*')):
            errors.append('symlink in evidence packet')
        artifacts = {p.relative_to(packet).as_posix(): digest(p) for p in packet.rglob('*') if p.is_file() and p != packet / 'manifest.json'}
        if artifacts != manifest['artifacts']:
            errors.append('packet inventory/hash differs')
        for ref in manifest['source_refs']:
            p = (root / ref['path']).resolve()
            if not p.is_relative_to(root.resolve()) or digest(p) != ref['sha256']:
                errors.append('source reference differs ' + ref['path'])
        for ref in manifest['tool_refs']:
            p = (root / ref['path']).resolve()
            if not p.is_relative_to(root.resolve()) or digest(p) != ref['sha256']:
                errors.append('preparation tool differs ' + ref['path'])
        if read(packet / 'candidate-ledger.json')['records'] != registry(root):
            errors.append('candidate ledger differs from pinned draft rows')
        if read(packet / 'private-authoring-index.json') != authoring_index(registry(root), root):
            errors.append('private authoring coverage differs')
        forms = read(packet / 'gold-authoring-forms.json')
        if forms != [gold_form(i) for i in (*WORK_IDS, 'ONTOLOGY')]:
            errors.append('incomplete draft forms promoted or changed')
        state = read(packet / 'formal-state.json')
        if state != {'D04': '미정', 'roles': None, 'freeze': None, 'execution_allowed': False,
                     'approved_gold': 0, 'frozen_FINAL_cases': 0, 'formal_completed': 0, 'formal_total': 9}:
            errors.append('preparation promoted to acceptance')
        freezes = read(packet / 'freeze-readiness.json')
        if len(freezes) != 8 or {r['id'] for r in freezes} != {f'F-{n:02d}' for n in range(1, 9)}:
            errors.append('freeze topics missing or duplicated')
        if any(r.get('decision_status') != 'pending' or r.get('receipt') is not None or not r.get('remaining') for r in freezes):
            errors.append('freeze decision falsely filled')
        contract = read(packet / 'ontology-contract.json')
        if contract['gate'] != read(root / BASE / 'OPERATIONAL-ACCEPTANCE-DECISION-PROPOSAL.json')['ontology_new_gate_proposal']:
            errors.append('ontology thresholds silently changed')
        if contract['FINAL_mode'] is not None or contract['reviewed_link_count'] is not None or contract['execution_allowed'] is not False:
            errors.append('ontology review/FINAL mode invented')
        ledger = read(packet / 'candidate-ledger.json')
        if ledger['formal_FINAL_cases'] != [] or ledger['reviewed_independent_units'] != 0:
            errors.append('exposed draft promoted to independent sample')
        from collections import Counter
        if ledger['counts_by_stratum'] != dict(Counter(r['stratum'] for r in ledger['records'])):
            errors.append('candidate counts inflated')
        samples = read(packet / 'sample-sufficiency.json')
        protocol = read(root / BASE / 'OPERATIONAL-ACCEPTANCE-PROTOCOL.json')
        groups = samples['groups']
        if len(groups) != 10 or {g['work_item_id'] for g in groups} != {*WORK_IDS, 'ONTOLOGY'}:
            errors.append('sufficiency group identities differ')
        for group in groups:
            if group['reviewed_independent_FINAL_units'] != 0 or group['sufficiency_verdict'] != 'inconclusive_not_reviewed' or group['precision_or_power_evidence'] is not None:
                errors.append('draft count promoted to sufficiency')
            if group['work_item_id'] != 'ONTOLOGY':
                original = next(g for g in protocol['input_groups'] if g['id'] == group['work_item_id'])
                for new, old in [('minimum_requirements_scope_approved_not_run_frozen', 'requirements_proposal'), ('metrics_unchanged', 'metrics_proposal'), ('FINAL_minimum_independent_units', 'final_minimum_independent_units'), ('DEV_allocation', 'dev_sample_allocation')]:
                    if group[new] != original[old]:
                        errors.append('sampling or scoring changed ' + group['work_item_id'])
        if samples['method'] != protocol['statistics_proposal'] or samples['repeat_plan'] != protocol['measurement_proposal']:
            errors.append('statistics/repeat proposals changed')
        holdout = read(packet / 'holdout-procedure.json')
        if any(holdout[k] is not None for k in ('formal_DEV_manifest', 'formal_FINAL_manifest', 'assignment_seed')):
            errors.append('formal split invented')
        worklist = read(packet / 'WORKLIST.json')
        if worklist['formal_completed'] != 0 or worklist['independent_quality_or_ontology_acceptance'] is not False or worklist['reviewer_assignment'] != '미정':
            errors.append('worklist falsely completes independent acceptance')
        reuse = read(packet / 'native-reuse.json')
        if reuse != native_reuse_record(root) or reuse['changed_bound_files'] != []:
            errors.append('native reuse misrepresented as fresh or unchanged')
    except (KeyError, TypeError, ValueError, OSError) as exc:
        errors.append(str(exc))
    return errors
