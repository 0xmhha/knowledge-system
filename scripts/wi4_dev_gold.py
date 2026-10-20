"""Bounded WI-4 source/schedule AI drafts; no trial or acceptance fabrication."""
from collections import Counter
import copy
import json
from pathlib import Path
import subprocess

import wi3_consumer_matching as previous
from source_packet import safe, verify

ROOT, BASE, f = previous.ROOT, previous.BASE, previous.f
PACKET = BASE / 'followups/wi4-dev-gold-v1'
REVISION = 'd95b3e91ac2ef0241f3386618c1f483e36cbf80e'
KIND = 'WI4_bounded_schedule_AI_DEV_not_formal_execution'
SOURCE = BASE / 'schedule-candidates-v1'
GAPS = BASE / 'schedule-gap-v1'
AUTHOR = Path('scripts/fixtures/wi4-dev-authoring.json')
TOOLS = ['scripts/' + n for n in (
    'wi4_dev_gold.py', 'prepare-wi4-dev-gold.py', 'check-wi4-dev-gold.py',
    'test_wi4_dev_gold.py', 'fixtures/wi4-dev-authoring.json', 'source_packet.py',
    'independent_acceptance.py', 'wi3_consumer_matching.py',
    'check-operational-acceptance.py', 'test_operational_acceptance.py')]
LABELS = ('legal', 'symptom_candidate', 'unknown', 'infra_or_unreached')
INFRA = (
    ('entry_unreached', 'The specified source branch, object or trigger was not reached. No symptom success or valid trial.'),
    ('precondition_unproved', 'Readiness, identity, buffer or ordering precondition was not proved. Do not infer target behavior from timeout.'),
    ('driver_error', 'Driver, transport, instrumentation, resource or decode failure. Preserve planned slot and original error.'),
    ('cancel_or_cleanup_error', 'Cancellation or failure to join all owned resources. Preserve original observation and cleanup failure separately.'),
)


def source_anchor(root, proof, path, first, last):
    raw = safe(root / SOURCE, proof['files'][path]['artifact']).read_bytes()
    a, b, span = previous.line_span(raw, first, last)
    return dict(path=path, file_sha256=f.sha(raw), start_line=first, end_line=last,
                start_byte=a, end_byte_exclusive=b, span_sha256=f.sha(span), text=span.decode())


def derive(root=ROOT):
    proof = verify(root / SOURCE, f.SOURCE_COMMIT)
    original = f.read(root / SOURCE / 'private-schedules.json')['families']
    gaps = {r['case_id']: r for r in f.read(root / GAPS / 'private-family-gaps.json')['families']}
    authored = f.read(root / AUTHOR)
    public = f.read(root / GAPS / 'public-inputs.json')
    ordered = [s['id'] for family in original for s in family['schedules']]
    if len(ordered) != 21 or [s['id'] for s in authored['schedules']] != ordered:
        raise ValueError('original schedule population/order differs')
    by_id = {s['id']: s for s in authored['schedules']}
    gold, inputs = [], []
    for family in original:
        gap = gaps[family['case_id']]
        for schedule in family['schedules']:
            note = by_id[schedule['id']]
            anchors = [source_anchor(root, proof, *w) for w in note['witnesses']]
            # Recheck prior byte spans independently, including entry route evidence.
            for anchor in gap['existing_source_anchors'] + [gap['entry_review_draft']['anchor']]:
                a = source_anchor(root, proof, anchor['path'], anchor['start_line'], anchor['end_line'])
                if any(a[k] != anchor[k] for k in ('start_byte', 'end_byte_exclusive', 'span_sha256')):
                    raise ValueError('prior family source anchor differs')
            units = [dict(id=key, label=label, rationale=text, source_refs=anchors,
                          source_expected_not_observed=True)
                     for key, label, text in note['outcomes']]
            units += [dict(id='unmodelled_trace', label='unknown', rationale=note['unknown'],
                           source_refs=anchors, source_expected_not_observed=True)]
            units += [dict(id=key, label='infra_or_unreached', rationale=text,
                           source_refs=anchors, source_expected_not_observed=True) for key, text in INFRA]
            if (len({u['id'] for u in units}) != len(units)
                    or any(u['label'] not in LABELS for u in units)):
                raise ValueError('overlapping or unlabelled outcome vocabulary')
            sets = {label: [u['id'] for u in units if u['label'] == label] for label in LABELS}
            row = dict(case_id=schedule['id'], family_proposal=family['case_id'],
                       mechanism_proposal=family['mechanism'], work_item_id='WI-4', split='DEV',
                       formal_split=None, independent_family=None, reviewer=None, independent_review_receipt=None,
                       source_commit=f.SOURCE_COMMIT, source_mode='committed', source_project='go-stablenet',
                       schedule=copy.deepcopy(schedule), strengthened_preconditions=note['preconditions'],
                       source_entry=gap['entry_review_draft'], source_unknowns=gap['existing_unknowns'],
                       product_contract_questions=gap['existing_contract_questions'],
                       source_refs=anchors, universe=units, answer_sets=sets,
                       empty_set_rationales={k: 'No authored ' + k + ' outcome in this finite conditional vocabulary; unmodelled traces stay unknown.' for k, v in sets.items() if not v},
                       complete_within_declared_vocabulary=True, complete_trace_or_product_gold=False,
                       observation_axis='language_permitted_source_behavior_and_product_symptom_are_distinct',
                       symptom_meaning='conditional_AI_observation_candidate_not_independent_product_violation',
                       unknown_policy='unmodelled_or_product_contract_unresolved_never_negative_or_automatic_pass',
                       formal_gold_schema_gap='unknown_outcomes must remain explicit; old three-set authoring form cannot silently merge unknown with infra',
                       current_scores=None, actual_target_trial_receipt=None, formal_scoring_allowed=False,
                       exposure='previously_exposed_DEV_no_fresh_FINAL', independent_approved=False)
            gold.append(row)
            inputs.append(dict(case_id=schedule['id'], family_proposal=family['case_id'], split='DEV',
                               formal_split=None, source_commit=f.SOURCE_COMMIT,
                               start_conditions=schedule['start_conditions'], steps=schedule['steps'],
                               observations=schedule['observations'], step_deadline_ms=schedule['step_deadline_ms'],
                               cleanup=schedule['cleanup'],
                               strengthened_preconditions=note['preconditions'],
                               timing_scope='2000ms_driver_watchdog_not_API_contract',
                               contains_gold=False, approved=False, actual_runs=0))
    notes = f.read(root / SOURCE / 'toolchain-notes.json')
    toolchain = []
    for name, binding in notes['sources'].items():
        raw = safe(root / SOURCE, binding['artifact']).read_bytes()
        if f.sha(raw) != binding['sha256']:
            raise ValueError('historical toolchain source differs')
        toolchain.append(dict(path=name, artifact=str(SOURCE / binding['artifact']), sha256=f.sha(raw)))
    return dict(public=inputs, original_public=public, gold=gold,
                toolchain=dict(notes=notes, bindings=toolchain, target_runtime_adopted=False),
                summary=dict(kind=KIND, DEV_schedules=len(gold), family_proposals=5,
                             outcome_units=sum(len(r['universe']) for r in gold),
                             labels=dict(Counter(u['label'] for r in gold for u in r['universe'])),
                             independent_families=0, independent_approved=0, FINAL=0,
                             formal_completed=0, formal_total=9, D04='미정', freeze=None,
                             execution_allowed=False, current_scores=None, new_target_trials=0,
                             new_MCP_calls=0, new_nodes=0, new_model_calls=0))


def contract():
    return dict(status='AI_DEV_proposal_not_frozen',
                scope='21 pre-existing schedules in5 proposed mechanisms; finite conditional outcome vocabulary, unmodelled trace unknown',
                axes='source/language-permitted choice is not product legality; pending product contracts never become symptom failures',
                source_identity='committed go-stablenet0bf2f4d1 plus original Git chain/file/span hashes and explicit entry route',
                observed_identity='family/schedule/source commit+binary/driver/environment SHA+seed+arm+order+actual entry/precondition trace required',
                outcome_matching='schedule-qualified exact outcome ids only after source/schedule/entry/driver evidence; declared id alone is not an independent oracle',
                select='both ready permits send OR exit; no fixed branch/frequency/fairness/termination bound',
                udp='synthetic deadline separate from actual Now.Add monotonic path; Reset/Stop request is not real timer firing',
                gossip='cache-before-async-attempt, nil return is not delivery; retry/best-effort contract pending',
                pool='reorgs=true, nonempty reinjected adds, StateAt success, actual buffer/read/lock order must be proved; backpressure not automatically defect',
                subscription='actual RPC notifier/error-close route separate from extracted method; ack and return distinct; rescue is post-observation',
                denominator='retain every planned arm/seed slot: valid no symptom, valid symptom, unknown, infra, unreached, missing and cleanup failure; no successful-only denominator',
                sampling='5 independently reviewed families minimum;100 planned per arm and baseline10 symptoms are proposals, repeated21 schedules add no independent families',
                comparison='same reviewed schedule/seed per arm, preregistered alternating order, independent product oracle and candidate/control identities required',
                statistics='ratio<=0.50 proposal; baseline zero/missing/unknown insufficient -> inconclusive; family paired uncertainty not runtime absolute binomial bound',
                observation='600second owned mode9 window, no recurrence inconclusive; actual signed source/log/cleanup receipts required',
                timing='instrumentation neutrality unknown until measured; normal/race/build pass alone insufficient',
                security='public DEV inputs only at host; no private labels/source-review paths; no existing/external node control',
                independent_review=None, formal_scoring_allowed=False)


def match_observation(observation, draft):
    """Conservative offline identity adapter, not a semantic/runtime oracle."""
    if not isinstance(observation, dict):
        return dict(status='unknown_missing_scope', label=None)
    for key, value in [('case_id', draft['case_id']), ('family_proposal', draft['family_proposal']),
                       ('source_commit', draft['source_commit']), ('source_project', 'go-stablenet'),
                       ('source_mode', 'committed')]:
        if key not in observation:
            return dict(status='unknown_missing_scope', label=None)
        if observation[key] != value:
            return dict(status='refused_wrong_scope', label=None)
    import re
    for key in ('binary_sha256', 'driver_sha256', 'environment_sha256'):
        if not isinstance(observation.get(key), str) or not re.fullmatch('[0-9a-f]{64}', observation[key]):
            return dict(status='unknown_unbound_runtime', label=None)
    if (observation.get('entry_reached') is not True
            or observation.get('preconditions_verified') is not True
            or not isinstance(observation.get('seed'), str) or not observation['seed']
            or observation.get('arm') not in ('baseline', 'fix')
            or type(observation.get('order')) is not int or observation['order'] < 0
            or not isinstance(observation.get('trace_sha256'), str)
            or not re.fullmatch('[0-9a-f]{64}', observation['trace_sha256'])):
        return dict(status='unknown_unverified_trial', label=None)
    unit = next((u for u in draft['universe'] if u['id'] == observation.get('outcome_id')), None)
    return dict(status='matched_AI_vocabulary_only' if unit else 'unknown_unmodelled_trace',
                label=unit['label'] if unit else 'unknown', independent_oracle=False,
                product_violation_established=False, quality_score=None)


def review_ledger(data):
    return dict(complete=False, independent_approved=0, current_scores=None,
                unresolved_product_contracts=[dict(case_id=r['case_id'], questions=r['product_contract_questions'],
                                                  unknown=r['source_unknowns']) for r in data['gold']],
                unknown_outcomes=[dict(case_id=r['case_id'], outcome_id=u['id'], rationale=u['rationale'])
                                  for r in data['gold'] for u in r['universe'] if u['label'] == 'unknown'],
                missing=['actual_API_entry_reachability', 'primary_and_independently_derived_oracles',
                         'reviewed_source_and_target_toolchain', 'reviewed_independent_families_DEV_FINAL',
                         'frozen_baseline_fix_and_counterexample_arms', 'planned_seed_order_and_valid_trial_receipts',
                         'family_paired_uncertainty', 'owned_cancel_cleanup_and_600sec_observation',
                         'instrumentation_effect_and_installed_host_role', 'explicit_unknown_formal_schema',
                         'D04_reviewers_and_F01_F08'])


def historical_errors(root=ROOT):
    errors = previous.historical_errors(root)
    for relative in (previous.PACKET, SOURCE, GAPS):
        p = root / relative
        manifest = f.read(p / 'manifest.json')
        original = subprocess.check_output(['git', 'show', REVISION + ':' + str(relative / 'manifest.json')], cwd=root)
        if original != (p / 'manifest.json').read_bytes():
            errors.append('historical manifest differs ' + str(relative))
        for name, digest in manifest['artifacts'].items():
            if f.sha(safe(p, name).read_bytes()) != digest:
                errors.append('historical artifact differs ' + str(relative / name))
        for tool in manifest.get('tools', []):
            if f.sha(subprocess.check_output(['git', 'show', REVISION + ':' + tool['path']], cwd=root)) != tool['sha256']:
                errors.append('historical source tool differs ' + tool['path'])
    return errors


def reference(root=ROOT, packet=None):
    p = packet or root / PACKET
    return dict(manifest=str(PACKET / 'manifest.json'), sha256=f.sha((p / 'manifest.json').read_bytes()),
                status='WI4_DEV21_conditional_outcomes_AI_not_independent_gold',
                report=str(PACKET / 'REPORT.txt'), worklist=str(PACKET / 'WORKLIST.json'))


def current_indices(root=ROOT, protocol_override=None, ledger_override=None):
    paths = [Path('docs/spec-driven/OPERATIONAL-ACCEPTANCE-' + n + '.json') for n in ('PROTOCOL', 'PROGRESS', 'DECISION-PROPOSAL')] + [BASE / 'GAPS.json']
    docs = [f.read(root / p) for p in paths]
    if protocol_override is not None: docs[0] = protocol_override
    if ledger_override is not None: docs[1] = ledger_override
    ref = reference(root); state = f.read(root / PACKET / 'WORKLIST.json')
    current = dict(kind=KIND, manifest=ref['manifest'], manifest_sha256=ref['sha256'])
    old = json.loads(subprocess.check_output(['git', 'show', REVISION + ':' + str(paths[0])], cwd=root))
    errors = []
    for d in docs:
        if (d.get('wi4_dev_gold_preparation') != ref or d.get('current_execution_state') != state
                or d.get('current_readiness_preparation') != current): errors.append('WI4 current reference differs')
        for key in ('wi3_consumer_matching_preparation', 'wi3_dev_gold_preparation', 'wi2_consumer_matching_preparation',
                    'wi2_dev_gold_preparation', 'wi1_consumer_matching_preparation', 'wi1_dev_gold_preparation', 'independent_freeze_preparation'):
            expected = dict(old[key])
            if key == 'wi3_consumer_matching_preparation': expected['historical_source_revision'] = REVISION
            if d.get(key) != expected: errors.append('previous preparation promoted ' + key)
    if docs[0]['freeze'] is not None or docs[0]['execution_allowed'] is not False or any(docs[0]['roles'].values()):
        errors.append('AI drafts cannot assign independent reviewers or freeze execution')
    link = docs[0]['adoption_acceptance_decision_proposal']
    if link['sha256'] != f.sha((root / paths[2]).read_bytes()) or any(d.get('adoption_acceptance_decision_proposal') != link for d in (docs[1], docs[3])):
        errors.append('decision proposal link differs')
    return errors


def render_review(row):
    return ('%s — WI-4 bounded conditional AI DEV, not independent oracle\n' % row['case_id']
            + json.dumps(row, ensure_ascii=False, indent=2) + '\n')


def validate(root=ROOT, packet=None):
    p = packet or root / PACKET; errors = []
    try:
        manifest = f.read(p / 'manifest.json')
        inventory = {x.relative_to(p).as_posix(): f.sha(x.read_bytes()) for x in p.rglob('*') if x.is_file() and x != p / 'manifest.json'}
        if (manifest['kind'] != KIND or manifest['previous_source_revision'] != REVISION
                or [t['path'] for t in manifest['tools']] != TOOLS or inventory != manifest['artifacts']):
            errors.append('WI4 manifest scope/hash differs')
        for t in manifest['tools']:
            if f.sha((root / t['path']).read_bytes()) != t['sha256']: errors.append('current source binding differs ' + t['path'])
        errors.extend(historical_errors(root)); data = derive(root)
        for name, key in [('public/DEV-inputs.json', 'public'), ('private/AI-gold.json', 'gold'),
                          ('SUMMARY.json', 'summary'), ('toolchain-scope.json', 'toolchain')]:
            if f.read(p / name) != data[key]: errors.append('WI4 derived draft differs ' + name)
        if f.read(p / 'matching-contract.json') != contract(): errors.append('matching contract changed or promoted')
        if f.read(p / 'REVIEW-LEDGER.json') != review_ledger(data): errors.append('review debt erased or promoted')
        for row in data['gold']:
            if (p / 'private' / (row['case_id'] + '-REVIEW.txt')).read_text() != render_review(row): errors.append('readable review differs')
        state = f.read(p / 'WORKLIST.json')
        if (state['formal_completed'] != 0 or state['formal_total'] != 9 or state['reviewer_assignment'] != '미정'
                or state['independent_quality_or_ontology_acceptance'] is not False
                or state['production_activation'] is not False or state['publication'] is not False): errors.append('worklist falsely completed')
        from independent_acceptance import native_reuse_record
        if native_reuse_record(root)['changed_bound_files']: errors.append('native source changed')
    except (KeyError, ValueError, TypeError, OSError, IndexError, StopIteration) as exc:
        errors.append(str(exc))
    return errors
