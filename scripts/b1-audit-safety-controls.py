#!/usr/bin/env python3
"""Audit declared synthetic policy controls; never approve facts or release gates."""
import argparse
from collections import Counter
import importlib.util
import json
import os
from pathlib import Path
import sys

SPEC = importlib.util.spec_from_file_location('paired_report', Path(__file__).with_name('b1-summarize-matrix.py'))
BASE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(BASE)


def canonical(value):
    return json.dumps(value, sort_keys=True, ensure_ascii=False, separators=(',', ':'))


def cited_references(value, path='semantic'):
    if isinstance(value, dict):
        if {'file', 'start_line', 'end_line', 'commit_hash'} <= set(value):
            yield path, value
        else:
            for key, child in value.items():
                yield from cited_references(child, path + '.' + key)
    elif isinstance(value, list):
        for index, child in enumerate(value):
            yield from cited_references(child, path + '[' + str(index) + ']')


def audit(report, requests, rows, controls, source, contract):
    if controls.get('schema_version') != 1 or controls.get('diagnostic_only') is not True:
        raise ValueError('explicit synthetic diagnostic controls required')
    if controls['coordinates'] != report['dataset_identity']['source'] or controls['dataset_id'] != report['dataset_identity']['dataset_id']:
        raise ValueError('control coordinates differ from captured dataset')
    cases = {c['request_id']: c for c in controls['cases']}
    if len(cases) != len(controls['cases']):
        raise ValueError('duplicate control ID')
    prepared = {qid: {'request_id': qid, 'group': 'synthetic-policy-control', 'independent_unit_id': 'alpha-common',
                     'expected_commit': controls['coordinates']['source_commit'], 'expected_citations': [],
                     'expect_no_citations': False} for qid in cases}
    # Reuse request/rotation/phase/invariant/source validation; no quality or CI
    # is taken from the composed-citation reader by this safety audit.
    checked = BASE.summarize(report, requests, rows, prepared, source, {}, resamples=1)
    if contract.get('schema_version') != 1 or len(contract['records']) != len(rows):
        raise ValueError('public verifier report does not cover these rows')
    for path, digest in controls['source_sha256'].items():
        if BASE.sha(BASE.source_bytes(source, path)) != digest:
            raise ValueError('declared control source bytes changed')
    lock = BASE.decode(BASE.source_bytes(source, '.cks/knowledge/knowledge.lock.json'))
    if lock['lock_digest'] != controls['knowledge_lock_digest'] or lock['project_id'] != controls['coordinates']['project_id']:
        raise ValueError('control lock differs from retained source declaration')
    diagnostics, packs, nested_count, states, locks = [], {}, 0, Counter(), set()
    for row, measurement, verified in zip(rows, checked['measurements'], contract['records']):
        binding = {'sequence': row['sequence'], 'arm': row['arm'], 'request_id': row['request_id'],
                   'phase': row['phase'], 'iteration': row['iteration'],
                   'measurement_id': row['call'].get('measurement_id', '')}
        if any(verified.get(field) != value for field, value in binding.items()):
            raise ValueError('public verifier record is not bound to captured SDK row')
        issues = list(measurement['issues'])
        if verified['status'] != 'valid_contract':
            issues.append('public_v2_contract_invalid_or_capture_error')
        if issues:
            diagnostics.append(dict(binding, issues=issues))
            continue
        pack = row['call']['response']['structuredContent']
        pack_key = (row['phase'], row['request_id'], row['iteration'], row['arm'])
        packs[pack_key] = pack
        case = cases[row['request_id']]
        semantic = pack.get('semantic') or {}
        for path, citation in cited_references(semantic):
            nested_count += 1
            if citation not in pack['citations']:
                issues.append('nested_reference_not_in_verified_registry: ' + path)
        response = row['call']['response']
        serialized = canonical(response)
        for marker in controls['forbidden_payload_markers']:
            if marker in serialized:
                issues.append('forbidden_synthetic_payload_marker')
        knowledge_files = {c['file'] for c in pack['citations'] if c['file'].startswith('.cks/knowledge/')}
        if not row['arm'].endswith('_on'):
            if 'knowledge_context' in semantic or 'coding_context' in semantic or knowledge_files:
                issues.append('pack_off_exposes_knowledge_overlay')
        else:
            knowledge, coding = semantic.get('knowledge_context') or {}, semantic.get('coding_context') or {}
            state = knowledge.get('state')
            states[state] += 1
            if state not in case['allowed_states']:
                issues.append('unexpected_policy_state')
            digest = knowledge.get('lock_digest')
            if digest != controls['knowledge_lock_digest']:
                issues.append('knowledge_lock_differs_from_frozen_control')
            if digest: locks.add(digest)
            policies = knowledge.get('applicable_policies', [])
            ids = [p['id'] for p in policies]
            if ids != case['applicable_policy_ids']:
                issues.append('unexpected_or_unreviewed_applicable_policy')
            if [p['id'] for p in coding.get('required_behavior', [])] != case['required_behavior_ids']:
                issues.append('incorrect_policy_to_required_behavior_projection')
            if any(p.get('state') != 'current' or p.get('reviewed_by') != controls['synthetic_reviewer'] for p in policies):
                issues.append('policy_review_state_mismatch')
            if knowledge_files != set(case['allowed_knowledge_files']):
                issues.append('unreviewed_expired_restricted_or_wrong_scope_source_exposed')
            for field in ['decisions', 'relations', 'related_requirements', 'test_links', 'trace_links', 'constraints']:
                if knowledge.get(field):
                    issues.append('control_invented_knowledge_claim: ' + field)
            for field in ['implemented_behavior', 'rationale', 'constraints']:
                if coding.get(field):
                    issues.append('control_invented_implementation_rationale_or_constraint: ' + field)
            conflicts = knowledge.get('conflicts', [])
            actual = {(c['left_id'], c['right_id'], c['reason']) for c in conflicts}
            expected = {tuple(c) for c in case['expected_conflicts']}
            if len(actual) != len(conflicts) or actual != expected:
                issues.append('conflict_endpoints_or_reason_differ')
            if 'implementation_link_unverified' not in coding.get('unknowns', []):
                issues.append('missing_implementation_uncertainty')
            if conflicts and 'conflicting_policies_require_review' not in coding.get('unknowns', []):
                issues.append('missing_conflict_uncertainty')
        if issues:
            diagnostics.append(dict(binding, issues=issues))
    comparisons = []
    groups = sorted({(r['phase'], r['request_id'], r['iteration']) for r in rows})
    for phase, qid, iteration in groups:
        for mode in ['baseline', 'concept_text', 'relations', 'combined']:
            before, after = (packs.get((phase, qid, iteration, mode + '_' + axis)) for axis in ['off', 'on'])
            issues = []
            if before is None or after is None:
                issues.append('paired_pack_response_missing_or_failed')
            else:
                for field in ['citations', 'bodies']:
                    if not {canonical(v) for v in before[field]} <= {canonical(v) for v in after[field]}:
                        issues.append('pack_on_dropped_base_' + field)
                for axis, pack in [('off', before), ('on', after)]:
                    baseline = packs.get((phase, qid, iteration, 'baseline_' + axis))
                    if baseline is None:
                        issues.append('baseline_response_missing_or_failed')
                    elif any({canonical(v) for v in pack[field]} != {canonical(v) for v in baseline[field]} for field in ['citations', 'bodies']):
                        issues.append('synthetic_empty_semantic_fixture_changed_candidate_set_' + axis)
            comparisons.append({'phase': phase, 'request_id': qid, 'iteration': iteration, 'mode': mode, 'issues': issues})
    violations = sum(bool(r['issues']) for r in diagnostics) + sum(bool(r['issues']) for r in comparisons)
    return {'schema_version': 1, 'scope': 'declared synthetic control state/payload/coordinate/preservation audit; not general secret detection or human policy truth',
            'diagnostic_only': True, 'status': 'control_audit_pass' if checked['status'] == 'descriptive' and not violations else 'control_audit_failed_or_incomplete',
            'observed_rows': len(rows), 'planned_rows': checked['planned_rows'], 'missing_slots': checked['missing_slots'],
            'capture_invariant_issues': checked['capture_invariant_issues'], 'capture_errors': checked['capture_errors'],
            'nested_semantic_citation_references_checked': nested_count, 'pack_on_states': dict(states),
            'observed_lock_digests': sorted(locks), 'violation_records': violations, 'diagnostics': diagnostics,
            'paired_preservation_comparisons': comparisons, 'public_contract_failed_records': contract['failed'],
            'quality_metrics': None, 'human_verdicts': None, 'official_B1_05_verdict': 'pending',
            'limitations': ['Synthetic fixture-reviewer is not an operating fact approval.',
                'Forbidden markers cover authored synthetic policy payloads, not every possible secret or inferred disclosure.',
                'Same-mode off/on base preservation and empty semantic-fixture candidate equality do not prove all production/authority cases.',
                'F-04 historical and F-05 distinct-project official oracles, raw CKV ranks, final fact review and human claim verdicts remain.']}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--capture-dir', type=Path, required=True)
    parser.add_argument('--controls', type=Path, required=True)
    parser.add_argument('--contract-audit', type=Path, required=True)
    parser.add_argument('--source-root', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    if args.output.exists() or args.output.is_symlink():
        raise ValueError('output already exists')
    paths = [args.capture_dir / n for n in ['report.json', 'requests.json', 'rows.jsonl']] + [args.controls, args.contract_audit]
    frozen = {str(p): BASE.sha(p.read_bytes()) for p in paths}
    report, requests = (BASE.decode(p.read_bytes()) for p in paths[:2])
    rows_raw = paths[2].read_bytes()
    controls, contract = (BASE.decode(p.read_bytes()) for p in paths[3:])
    if BASE.sha(rows_raw) != report['rows_sha256'] or frozen[str(paths[1])] != report['request_sha256'] or contract['input_sha256'] != BASE.sha(rows_raw):
        raise ValueError('capture/verifier raw input hash mismatch')
    configurations = BASE.verify_retained_configs(args.capture_dir, report)
    rows = [BASE.decode(line) for line in rows_raw.splitlines()]
    result = audit(report, requests, rows, controls, args.source_root, contract)
    if frozen != {str(p): BASE.sha(p.read_bytes()) for p in paths}:
        raise ValueError('input changed during replay')
    if configurations != BASE.verify_retained_configs(args.capture_dir, report):
        raise ValueError('configuration changed during replay')
    for path, digest in controls['source_sha256'].items():
        if BASE.sha(BASE.source_bytes(args.source_root, path)) != digest:
            raise ValueError('control source changed during replay')
    result.update(input_sha256=frozen, arm_configuration_sha256=configurations,
                  source_sha256=controls['source_sha256'], verifier_binary_sha256=contract['binary_sha256'],
                  auditor_sha256=BASE.sha(Path(__file__).read_bytes()))
    fd = os.open(args.output, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(fd, 'w', encoding='utf-8') as stream:
        stream.write(json.dumps(result, ensure_ascii=False, indent=2) + '\n')
    return 0 if result['status'] == 'control_audit_pass' else 2


if __name__ == '__main__':
    try:
        sys.exit(main())
    except (ValueError, OSError, KeyError, TypeError) as exc:
        print('b1-audit-safety-controls: ' + str(exc), file=sys.stderr)
        sys.exit(1)
