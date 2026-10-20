#!/usr/bin/env python3
"""Verify candidate identities, source proofs, bounded schedules and pending contracts."""
import argparse
import hashlib
import importlib.util
import itertools
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
BASE = Path('docs/spec-driven/operational-preparation/source-review-supplement')
IDENTITIES = {
    'SCHED-001': ('SRC-021', 'uninstall_status_handshake_cycle', 'existing_extracted_fixture_only'),
    'SCHED-002': ('SRC-032', 'removed_list_cursor_loses_successor', 'synthetic_deadline_source_branch_only'),
    'SCHED-003': ('SRC-027', 'cancellation_and_ready_send_selection', 'contract_question_only'),
    'SCHED-004': ('SRC-002', 'cache_mark_precedes_async_delivery', 'delivery_contract_question_only'),
    'SCHED-005': ('SRC-017', 'pool_lock_held_during_synchronous_fanout', 'controlled_lock_model_only'),
}


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def load(name):
    spec = importlib.util.spec_from_file_location(name.replace('-', '_'), ROOT / 'scripts' / name)
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


def check(packet, check_references=True):
    errors = []

    def require(condition, message):
        if not condition:
            errors.append(message)

    def safe(name):
        path = Path(name)
        if path.is_absolute() or '..' in path.parts or not (packet / path).resolve().is_relative_to(packet.resolve()) or (packet / path).is_symlink():
            raise ValueError('unsafe artifact path')
        return packet / path

    try:
        manifest = json.loads((packet / 'manifest.json').read_text())
        require(manifest['kind'] == 'source_schedule_candidates_not_independent_acceptance', 'packet kind')
        expected_prior = {str(BASE / n / 'manifest.json') for n in ['context-v1', 'subscription-probe-v1']}
        require(set(manifest['prior_packets']) == expected_prior, 'prior packet set')
        for path, digest in manifest['prior_packets'].items():
            require(sha((ROOT / path).read_bytes()) == digest, 'prior packet binding')
        files = {p.relative_to(packet).as_posix() for p in packet.rglob('*') if p.is_file() and p.name != 'manifest.json'}
        require(files == set(manifest['artifacts']), 'artifact inventory')
        for name, digest in manifest['artifacts'].items():
            require(sha(safe(name).read_bytes()) == digest, 'artifact hash: ' + name)
        proof = json.loads((packet / 'git-proof.json').read_text())
        revision = json.loads((ROOT / 'docs/spec-driven/OPERATIONAL-ACCEPTANCE-INPUT-CANDIDATES.json').read_text())['repository_commit']
        require(manifest['repository_commit'] == proof['commit'] == revision, 'fixed source commit')
        context = load('check-review-context.py')

        def object_raw(kind, identity):
            raw = safe(proof['objects'][kind + ':' + identity]).read_bytes()
            if context.oid(kind, raw) != identity:
                raise ValueError('Git object identity mismatch')
            return raw

        tree = object_raw('commit', revision).splitlines()[0].decode().removeprefix('tree ')
        for path, record in proof['files'].items():
            current = tree
            parts = Path(path).parts
            require(not Path(path).is_absolute() and '..' not in parts, 'source path')
            for i, name in enumerate(parts):
                mode, current = context.tree_entries(object_raw('tree', current))[name]
                require(mode in ('100644', '100755') if i == len(parts)-1 else mode == '40000', 'source mode')
            raw = safe(record['artifact']).read_bytes()
            require(current == record['git_blob'] == context.oid('blob', raw) and sha(raw) == record['sha256'], 'commit-to-file proof')
        public = json.loads((packet / 'public-inputs.json').read_text())
        private = json.loads((packet / 'private-schedules.json').read_text())
        require(public['status'] == 'schedule_input_candidates_only' and public['contains_gold'] is False, 'public boundary')
        require(private['status'] == 'ai_schedule_drafts_not_independent_gold'
                and private['accepted_independent_families'] == private['formal_runs'] == private['new_local_probe_runs'] == 0
                and private['reviewer'] is None and private['split'] is None, 'independent/runtime promotion')
        families = {f['case_id']: f for f in private['families']}
        inputs = {p['case_id']: p for p in public['cases']}
        require(len(private['families']) == len(public['cases']) == 5 and set(families) == set(inputs) == set(IDENTITIES), 'candidate identity/count')
        all_schedules, used = [], set()
        for identity, family in families.items():
            require((family['source_candidate_id'], family['mechanism'], family['local_probe_eligibility']) == IDENTITIES[identity], 'candidate mechanism/scope')
            require(family['origin'] == 'ai_manual_source_reading_and_schedule_design' and family['approved'] is False
                    and family['reviewer'] is None and family['review_receipt'] is None and family['independent_oracle'] is None
                    and family['accepted_family'] is False and family['verified_project_defect'] is False
                    and family['operational_reachability'] == 'not_verified', 'candidate approval/contract promotion')
            require(bool(family['unverified_assumptions']) and bool(family['contract_questions']), 'missing unresolved assumptions')
            p = inputs[identity]
            require(set(p) == {'case_id', 'work_item', 'source_commit', 'source_paths', 'prompt', 'split', 'approved', 'contains_gold'}, 'public private-field leak')
            require(p['work_item'] == 'WI-4' and p['source_commit'] == revision and p['split'] is None
                    and p['approved'] is False and p['contains_gold'] is False, 'public scope/promotion')
            require(set(p['source_paths']) == {a['path'] for a in family['source_anchors']}, 'public source binding')
            for anchor in family['source_anchors']:
                used.add(anchor['path'])
                raw = safe(proof['files'][anchor['path']]['artifact']).read_bytes()
                lines = raw.splitlines(keepends=True)
                start, end = anchor['start_line'], anchor['end_line']
                require(1 <= start <= end <= len(lines), 'anchor range')
                begin, finish = sum(map(len, lines[:start-1])), sum(map(len, lines[:end]))
                require(anchor['start_byte'] == begin and anchor['end_byte_exclusive'] == finish
                        and anchor['span_sha256'] == sha(raw[begin:finish]), 'anchor binding')
                require(anchor['source_fact_draft'] not in p['prompt'], 'source answer copied to public prompt')
            schedules = family['schedules']
            require(len(schedules) == (5 if identity == 'SCHED-001' else 4), 'correlated schedule count')
            require(any(s['kind'] == 'normal_control' for s in schedules), 'missing normal control')
            for s in schedules:
                all_schedules.append(s['id'])
                require(s['outcome_status'] == 'ai_draft_not_independent_oracle'
                        and isinstance(s['step_deadline_ms'], int) and 0 < s['step_deadline_ms'] <= 2000,
                        'unbounded or promoted schedule')
                require(all(bool(s[k]) for k in ['start_conditions', 'steps', 'observations', 'primary_expectation_draft'])
                        and len(s['cleanup']) >= 3, 'missing precondition/observation/cleanup')
        require(len(all_schedules) == len(set(all_schedules)) == 21 and used == set(proof['files']), 'schedule/source inventory')
        pairs = private['pairwise_overlap_review']
        require(len(pairs) == 10 and {(p['a'], p['b']) for p in pairs} == set(itertools.combinations(sorted(IDENTITIES), 2)), 'pairwise overlap inventory')
        require(all(p['independence_status'] == 'not_reviewed' and p['same_mechanism_proposed'] is False
                    and len(p['shared_risks']) >= 3 for p in pairs), 'independence claim from proposed difference')
        require(private['sampling_proposal'] == {'runs_per_arm': 100, 'minimum_baseline_symptom_events': 10,
                'arms': ['baseline', 'candidate_fix'], 'candidate_fix_revision': None, 'baseline_revision': revision,
                'sampling_approved': False, 'zero_or_insufficient_baseline': 'inconclusive',
                'statistical_independence': 'not_reviewed'}, 'sampling/gate promotion')
        require(private['runtime_boundary'] == {'external_model_calls': 0, 'nodes_started': 0, 'host_clock_changes': 0,
                'target_state_writes': 0, 'operational_execution_allowed': False}, 'runtime boundary')
        toolchain = json.loads((packet / 'toolchain-notes.json').read_text())
        require(toolchain['kind'] == 'local_fixture_semantics_not_target_runtime_adoption'
                and toolchain['go_version'] == 'go1.26.9' and toolchain['target_runtime_verified'] is False
                and toolchain['clock_rollback_reproduced'] is False and bool(toolchain['monotonic_limit']), 'clock/runtime overclaim')
        require(set(toolchain['sources']) == {'src/container/list/list.go', 'src/time/time.go'}, 'stdlib references')
        for path, record in toolchain['sources'].items():
            require(sha(safe(record['artifact']).read_bytes()) == record['sha256'], 'stdlib source binding')
        require(manifest['counts'] == {'candidate_families': 5, 'new_candidate_families': 4, 'schedule_proposals': 21,
                'source_anchors': 17, 'source_files': 7, 'pairwise_overlap_reviews': 10,
                'accepted_independent_families': 0}, 'inflated counts')
        require(json.loads((packet / 'formal-state.json').read_text()) == load('prepare-source-review-supplement.py').formal_state(ROOT), 'formal state changed')
        if check_references:
            for name in ['PROTOCOL', 'PROGRESS']:
                d = json.loads((ROOT / f'docs/spec-driven/OPERATIONAL-ACCEPTANCE-{name}.json').read_text())
                require(d.get('schedule_candidate_preparation') == {
                    'manifest': (packet / 'manifest.json').relative_to(ROOT).as_posix(),
                    'sha256': sha((packet / 'manifest.json').read_bytes()),
                    'status': 'source_schedule_proposals_not_independent_families'}, 'ledger reference: ' + name)
    except (OSError, ValueError, KeyError, TypeError, IndexError) as exc:
        errors.append('invalid schedule packet: ' + str(exc))
    return errors


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('packet', type=Path)
    args = parser.parse_args()
    errors = check(args.packet.resolve())
    if errors:
        print('\n'.join(errors))
        raise SystemExit(1)
    print('PASS: 5 candidate mechanisms, 21 correlated schedules, 17 source anchors; accepted independent families 0')
