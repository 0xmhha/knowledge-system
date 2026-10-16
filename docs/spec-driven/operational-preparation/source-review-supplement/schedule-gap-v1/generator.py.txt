#!/usr/bin/env python3
"""Audit WI-4 source reachability, contracts, families and repetition denominators."""
import argparse
from collections import Counter
import importlib.util
import itertools
import json
from pathlib import Path

from source_packet import SourceCapture, safe, sha, verify

ROOT = Path(__file__).resolve().parents[1]
BASE = Path('docs/spec-driven/operational-preparation/source-review-supplement')
SCHEDULE = BASE / 'schedule-candidates-v1'
HOST = Path('system/eval/operational-preparation-2026-10-15/isolated-host')
OLD = Path('system/eval/upper-work-items-implementation-2026-10-14/wi-4')
MODE9 = Path('system/eval/upper-work-items-implementation-2026-10-14/wi-4-mode9')
INPUTS = {'public': SCHEDULE / 'public-inputs.json', 'private': SCHEDULE / 'private-schedules.json',
          'toolchain': SCHEDULE / 'toolchain-notes.json', 'subscription': BASE / 'subscription-probe-v1/summary.json',
          'udp': BASE / 'udp-cursor-probe-v1/summary.json', 'old_manifest': OLD / 'manifest.json',
          'old_summary': OLD / 'summary.json', 'old_comparison': OLD / 'timing/1/timing-evaluation-report.json',
          'mode9_manifest': MODE9 / 'manifest.json', 'mode9_summary': MODE9 / 'execution/summary.json',
          'window1': MODE9 / 'execution/window-1/log-report.json', 'window2': MODE9 / 'execution/window-2/log-report.json',
          'mode9_controls': MODE9 / 'execution/.coding-agent/tickets/read-probe/timing-instrumentation-controls/report.json',
          'host': HOST / 'manifest.json', 'chainbench_inventory': HOST / 'chainbench-source-files.json',
          'coding_agent_inventory': HOST / 'coding_agent-source-files.json',
          'proposals': Path('scripts/fixtures/schedule-gap-proposals.json')}
MANIFESTS = [BASE / n / 'manifest.json' for n in ['schedule-candidates-v1', 'subscription-probe-v1', 'udp-cursor-probe-v1']]
ADOPTED = {'chainbench': ['internal/experiment/plan.go', 'internal/experiment/run.go',
                        'internal/experiment/log_monitor.go', 'internal/mcp/experiment_tool.go', 'internal/mcp/log_observation.go'],
           'coding_agent': ['plugin/scripts/timing_instrumentation.py']}
ATTRIBUTES = b'adopted-source/*/objects/*.commit binary\nadopted-source/*/objects/*.tree binary\n'
OBLIGATIONS = ['target_entry_and_reachability', 'schedule_preconditions_and_legal_outcomes',
               'independent_primary_and_derived_oracles', 'independent_family_and_dev_final',
               'frozen_baseline_fix_binary_driver_environment', 'paired_schedule_and_order_policy',
               'valid_repetition_denominator_and_error_policy', 'ten_minute_observation_mapping',
               'relative_rate_and_family_uncertainty_mapping', 'owned_cancel_cleanup_receipts',
               'instrumentation_timing_effect_assessment', 'installed_host_role_and_reentry_receipts']


def load(name):
    spec = importlib.util.spec_from_file_location(name.replace('-', '_'), ROOT / 'scripts' / name)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


helpers = load('prepare-semantic-gap.py')
dump, unique, span = helpers.dump, helpers.unique, helpers.span


def bindings():
    return sorted(set(INPUTS.values()) | set(MANIFESTS) | {Path('scripts/prepare-semantic-gap.py')})


def inputs(packet):
    data = {k: json.loads((ROOT / p).read_text()) for k, p in INPUTS.items()}
    for path in MANIFESTS:
        manifest = json.loads((ROOT / path).read_text())
        for name, digest in manifest['artifacts'].items():
            if sha(safe(ROOT / path.parent, name).read_bytes()) != digest:
                raise ValueError('prior packet artifact binding: ' + name)
    for directory, key in [(OLD, 'old_manifest'), (MODE9, 'mode9_manifest')]:
        for path in INPUTS.values():
            if path.is_relative_to(directory) and path != INPUTS[key]:
                if data[key]['files_sha256'][path.relative_to(directory).as_posix()] != sha((ROOT / path).read_bytes()):
                    raise ValueError('historical implementation evidence binding')
    revision = json.loads((ROOT / SCHEDULE / 'manifest.json').read_text())['repository_commit']
    proof = verify(ROOT / SCHEDULE, revision)
    data['project_sources'] = {name: safe(ROOT / SCHEDULE, r['artifact']).read_bytes() for name, r in proof['files'].items()}
    for family in data['private']['families']:
        for anchor in family['source_anchors']:
            expected = span(data['project_sources'][anchor['path']], anchor['start_line'], anchor['end_line'])
            if expected != {k: v for k, v in anchor.items() if k not in ['path', 'source_fact_draft']}:
                raise ValueError('project anchor span')
    data['adopted_sources'] = {}
    for owner, paths in ADOPTED.items():
        revision = data['host']['source_adoptions'][owner]['source_commit']
        proof = verify(packet / 'adopted-source' / owner, revision)
        expected = {r['path']: r['sha256'] for r in data[owner+'_inventory']}
        if set(proof['files']) != set(paths):
            raise ValueError('adopted source inventory')
        data['adopted_sources'][owner] = {}
        for name, r in proof['files'].items():
            raw = safe(packet / 'adopted-source' / owner, r['artifact']).read_bytes()
            if sha(raw) != expected[name]:
                raise ValueError('adopted source hash')
            data['adopted_sources'][owner][name] = raw
    protocol = json.loads((ROOT / 'docs/spec-driven/OPERATIONAL-ACCEPTANCE-PROTOCOL.json').read_text())
    progress = json.loads((ROOT / 'docs/spec-driven/OPERATIONAL-ACCEPTANCE-PROGRESS.json').read_text())
    data['criteria'] = {'protocol_work_item': next(r for r in protocol['input_groups'] if r['id']=='WI-4'),
                        'progress_work_item': next(r for r in progress['items'] if r['id']=='WI-4'),
                        'statistics_proposal': protocol['statistics_proposal']}
    return data


def probe_matrix(rows, scenario_key):
    counter = Counter((r['variant'], r['mode']) for r in rows)
    return [{'variant': variant, 'mode': mode, 'observations': count,
             'scenarios': sorted({r[scenario_key] for r in rows if r['variant']==variant and r['mode']==mode}),
             'operational_trials': 0, 'independent_units': 0} for (variant, mode), count in sorted(counter.items())]


def generate(data):
    families = unique(data['private']['families'], 'case_id')
    public = unique(data['public']['cases'], 'case_id')
    proposals = unique(data['proposals']['families'], 'case_id')
    if set(families) != {f'SCHED-{i:03d}' for i in range(1,6)} or set(families) != set(public) or set(families) != set(proposals):
        raise ValueError('five candidate inventory')
    if data['private']['accepted_independent_families'] != 0 or data['private']['formal_runs'] != 0 or data['private']['sampling_proposal']['sampling_approved'] is not False:
        raise ValueError('family/sampling promotion')
    rows, schedules = [], []
    for key, family in sorted(families.items()):
        if family['operational_reachability'] != 'not_verified' or family['accepted_family'] is not False or family['independent_oracle'] is not None or family['verified_project_defect'] is not False:
            raise ValueError('reachability/oracle approval')
        proposal = proposals[key]
        path, start, end = proposal['entry_anchor']
        anchor = {'path': path, **span(data['project_sources'][path], start, end)}
        scope = {'SCHED-001': 'existing_extracted_subscription_method_only',
                 'SCHED-002': 'existing_synthetic_deadline_timer_request_recorder_only'}.get(key, 'no_local_probe_execution_in_this_preparation')
        rows.append({'case_id': key, 'source_candidate_id': family['source_candidate_id'],
                     'mechanism_proposal': family['mechanism'], 'existing_source_anchors': family['source_anchors'],
                     'entry_review_draft': {'anchor': anchor, 'fact': proposal['entry_fact'],
                         'reachability_question': proposal['reachability_question'], 'contract_question': proposal['contract_question'],
                         'origin': 'ai_source_inspection_not_runtime_or_gold', 'approved': False},
                     'existing_unknowns': family['unverified_assumptions'], 'existing_contract_questions': family['contract_questions'],
                     'local_probe_evidence_scope': scope, 'schedule_ids': [s['id'] for s in family['schedules']],
                     'obligations': {k: 'not_satisfied' for k in OBLIGATIONS},
                     'target_reachability_receipt': None, 'contract_receipt': None, 'independent_oracles': None,
                     'baseline_plan': None, 'fix_plan': None, 'baseline_valid_trials': None, 'fix_valid_trials': None,
                     'baseline_symptom_events': None, 'fix_symptom_events': None, 'rate_ratio': None,
                     'cluster_interval': None, 'mode9_600_second_receipt': None, 'timing_neutrality': 'not_established',
                     'independent_family': None, 'reviewer': None, 'review_receipt': None, 'split': None,
                     'operational_gate': 'not_run', 'quality_gate': 'not_run', 'independent_acceptance': 'not_recorded'})
        for s in family['schedules']:
            schedules.append({'family_proposal': key, 'schedule': s,
                              'operational_reachability': 'not_verified', 'independent_oracle': None,
                              'frozen_arm_assignment': None, 'operational_runs': 0})
    if len(schedules) != 21 or len({r['schedule']['id'] for r in schedules}) != 21:
        raise ValueError('correlated schedule denominator')
    pairs = [{'a': a['case_id'], 'b': b['case_id'], 'same_mechanism_label': a['mechanism_proposal']==b['mechanism_proposal'],
              'shared_source_files': sorted({x['path'] for x in a['existing_source_anchors']} & {x['path'] for x in b['existing_source_anchors']}),
              'shared_ai_author_and_project_snapshot': True, 'semantic_independence': 'not_reviewed'}
             for a, b in itertools.combinations(rows,2)]
    sub, udp = data['subscription'], data['udp']
    if sub['independent_families'] != 0 or sub['verified_project_defect'] is not False or udp['accepted_independent_families'] != 0 or udp['operational_reachability'] != 'not_verified':
        raise ValueError('local probe promotion')
    if len(sub['observations']) != 50 or len(udp['observations']) != 20:
        raise ValueError('local observation matrix')
    comparison = data['old_comparison']
    if comparison['plan']['repetitions'] != 60 or comparison['before']['completed'] != 60 or comparison['after']['completed'] != 60 or comparison['acceptance'] != 'not_recorded':
        raise ValueError('historical controlled comparison scope')
    windows = [data['window1'],data['window2']]
    if [w['plan']['window_seconds'] for w in windows] != [2,2] or [w['verdict'] for w in windows] != ['AWAITING_OBSERVATION','RECURRED']:
        raise ValueError('historical two short windows')
    contracts = []
    for r in data['proposals']['contracts']:
        contracts.append({**r, 'anchor': span(data['adopted_sources'][r['owner']][r['path']], *r['lines']),
                          'observation_kind': 'adopted_source_inspection_not_new_execution', 'approved': False})
    if len(unique(contracts,'id')) != 11:
        raise ValueError('contract review inventory')
    runtime = {'status': 'scoped_prior_evidence_and_source_inspection_only', 'contracts': contracts,
               'adopted_revisions': {owner: data['host']['source_adoptions'][owner]['source_commit'] for owner in ADOPTED},
               'subscription_extracted_matrix': probe_matrix(sub['observations'],'scenario'),
               'udp_synthetic_matrix': probe_matrix(udp['observations'],'case'),
               'udp_timer_scope': udp['timer_scope'], 'target_toolchain_adoption': data['toolchain']['target_runtime_verified'],
               'wall_clock_rollback_reproduced': data['toolchain']['clock_rollback_reproduced'],
               'historical_comparison': {'owner_revisions': data['old_manifest']['owner_commits'],
                   'scope': data['old_summary']['scope'], 'plan': comparison['plan'],
                   'before': comparison['before'], 'after': comparison['after'], 'verdict': comparison['verdict'],
                   'acceptance': comparison['acceptance'], 'current_five_families_covered': 0},
               'historical_mode9': {'owner_revisions': data['mode9_manifest']['owner_commits'], 'summary': data['mode9_summary'],
                   'windows': [{'planned_seconds': w['plan']['window_seconds'], 'verdict': w['verdict'],
                       'polls': w['polls'], 'events': len(w['events']), 'resume_requested': w['resume_requested'],
                       'acceptance': w['acceptance']} for w in windows],
                   'instrumentation_controls_status': data['mode9_controls']['status'],
                   'timing_neutrality': data['mode9_controls']['timing_neutrality'], 'current_five_families_covered': 0},
               'metric_boundary': {'generic_runtime': 'after_one_sided_95_percent_absolute_binomial_upper_bound',
                   'generic_minimum_inference_samples': 30,
                   'work_item_proposal': 'fix_to_baseline_rate_ratio_at_most_0.50_with_family_uncertainty',
                   'criteria_snapshot': data['criteria'], 'equivalence_established': False,
                   'work_item_adapter_or_scoring_receipt': None,
                   'no_recurrence_scope_mapping': None, 'timing_neutrality_effect_estimate': None},
               'new_backend_calls':0, 'new_model_calls':0, 'new_nodes_started':0, 'host_clock_changes':0,
               'target_state_writes':0, 'formal_runs':0}
    counts = {'candidate_families':5, 'schedule_proposals':21, 'prior_project_anchors':17, 'new_project_context_anchors':5,
              'pairwise_family_reviews':len(pairs), 'approved_independent_families':0,
              'pending_independent_family_minimum':5, 'subscription_local_observations':50,
              'udp_synthetic_observations':20, 'historical_fixture_trials_per_comparison_arm':60,
              'historical_mode9_windows':2, 'adopted_source_files':sum(len(v) for v in ADOPTED.values()),
              'contract_review_anchors':len(contracts), 'families_missing_reachability_receipts':5,
              'families_without_frozen_baseline_fix_plan':5, 'approved_operational_trials':0,
              'independent_oracle_records':0, 'formal_runs':0}
    summary = {'kind':'schedule_reachability_contract_family_and_sampling_gap_preparation_only','counts':counts,
               'local_modes_are_not_baseline_fix_arms':True, 'repeated_schedules_are_not_independent_families':True,
               'missing_trial_counts_are_null_not_observed_zero':True,'work_item_rate_scores':None,
               'zero_or_insufficient_baseline_proposal':'inconclusive', 'protocol_or_criteria_changed':False}
    return {'private-family-gaps.json':{'status':'ai_source_and_evidence_gap_review_not_independent_acceptance','families':rows},
            'schedule-obligations.json':schedules,'pairwise-family-review.json':pairs,
            'execution-contract-review.json':runtime,'gap-summary.json':summary,'public-inputs.json':data['public']}


def table(artifacts):
    lines = ['WI-4 도달성·계약·독립 가족·반복 실행 공백 — 비공개 준비 자료',
             '가설 5개/스케줄 21개/독립 가족 0개. 로컬 관측 50+20건은 운영 arm별 100회가 아니다.',
             '과거 대역 60회/arm 및 모드9 2초 계획 창 2개는 현재 5가족 수용/600초 증거가 아니다.']
    for row in artifacts['private-family-gaps.json']['families']:
        d=row['entry_review_draft'];lines += ['',row['case_id']+' '+row['mechanism_proposal'],d['fact'],
                                           '도달성: '+d['reachability_question'],'계약: '+d['contract_question']]
    lines += ['','실행·판정 연결 질문:']
    for r in artifacts['execution-contract-review.json']['contracts']:
        lines += [r['id']+' '+r['fact'],'미충족: '+r['gap']]
    return '\n'.join(lines)+'\n'


def prepare(output,repositories):
    if output.exists():
        raise ValueError('use a new output; preserve prior evidence')
    for checker, name in [('check-schedule-candidates.py','schedule-candidates-v1'),
                          ('check-subscription-probe.py','subscription-probe-v1'),('check-udp-cursor-probe.py','udp-cursor-probe-v1')]:
        failures=load(checker).check(ROOT/BASE/name)
        if failures:
            raise ValueError('\n'.join(failures))
    output.mkdir(parents=True)
    host=json.loads((ROOT/INPUTS['host']).read_text())
    for owner,paths in ADOPTED.items():
        capture=SourceCapture(repositories[owner],output/'adopted-source'/owner,host['source_adoptions'][owner]['source_commit'])
        for path in paths:
            capture.source(path)
        dump(output/'adopted-source'/owner/'git-proof.json',capture.proof())
    data=inputs(output);artifacts=generate(data)
    for name,value in artifacts.items():
        dump(output/name,value)
    (output/'REVIEW-TABLE.txt').write_text(table(artifacts))
    (output/'.gitattributes').write_bytes(ATTRIBUTES)
    (output/'generator.py.txt').write_bytes(Path(__file__).read_bytes())
    dump(output/'formal-state.json',load('prepare-source-review-supplement.py').formal_state(ROOT))
    dump(output/'manifest.json',{'schema_version':1,'kind':'schedule_gap_preparation_only',
         'input_bindings':{str(p):sha((ROOT/p).read_bytes()) for p in bindings()},
         'generator_sha256':sha(Path(__file__).read_bytes()),'counts':artifacts['gap-summary.json']['counts'],
         'artifacts':{p.relative_to(output).as_posix():sha(p.read_bytes()) for p in sorted(output.rglob('*')) if p.is_file()}})
    print(json.dumps(artifacts['gap-summary.json']['counts']),flush=True)


if __name__=='__main__':
    parser=argparse.ArgumentParser(description=__doc__)
    for name in ['output','chainbench-repo','consumer-repo']:
        parser.add_argument('--'+name,type=Path,required=True)
    args=parser.parse_args()
    prepare(args.output.resolve(),{'chainbench':args.chainbench_repo.resolve(),'coding_agent':args.consumer_repo.resolve()})
