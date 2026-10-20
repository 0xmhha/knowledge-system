"""Source-bound WI-V1/V2 AI DEV matrices and conservative event-trace draft."""
from collections import Counter
import copy
import difflib
import json
from pathlib import Path
import re
import subprocess
import wi4_dev_gold as previous
from source_packet import safe, verify

ROOT, BASE, f = previous.ROOT, previous.BASE, previous.f
PACKET = BASE / 'followups/ticket-dev-gold-v1'
REVISION = '20f470f2e1b8d9b0b9c6b6ed007b2a42b456befa'
KIND = 'WIV1_V2_bounded_ticket_AI_DEV_not_formal_execution'
SOURCE = BASE / 'ticket-gap-v1/source/project'
TICKET = BASE / 'subscription-ticket-v1'
GAPS = BASE / 'ticket-gap-v1'
AUTHOR = Path('scripts/fixtures/ticket-dev-authoring.json')
TOOLS = ['scripts/' + s for s in ('ticket_dev_gold.py', 'prepare-ticket-dev-gold.py',
         'check-ticket-dev-gold.py', 'test_ticket_dev_gold.py', 'fixtures/ticket-dev-authoring.json',
         'wi4_dev_gold.py', 'prepare-wi4-dev-gold.py', 'source_packet.py', 'independent_acceptance.py',
         'check-operational-acceptance.py', 'test_operational_acceptance.py')]
PREPARATIONS = ('wi4_dev_gold_preparation','wi3_consumer_matching_preparation', 'wi3_dev_gold_preparation',
                'wi2_consumer_matching_preparation','wi2_dev_gold_preparation','wi1_consumer_matching_preparation',
                'wi1_dev_gold_preparation','independent_freeze_preparation')


def derive(root=ROOT):
    proof = verify(root / SOURCE, f.SOURCE_COMMIT)
    notes = f.read(root / AUTHOR); old = f.read(root / TICKET / 'private-review.json')
    gap = f.read(root / GAPS / 'private-ticket-gaps.json')
    source_path = 'eth/downloader/api.go'
    original_file = safe(root / SOURCE, proof['files'][source_path]['artifact']).read_bytes()
    original_method = (root / TICKET / 'original.go.txt').read_bytes()
    if original_file.count(original_method) != 1: raise ValueError('original method extraction not unique')
    if [s['id'] for s in notes['scenarios']] != [s['id'] for s in old['scenarios']]: raise ValueError('scenario population differs')
    if [c['id'] for c in notes['candidates']] != [c['id'] for c in old['variants']]: raise ValueError('candidate population differs')
    anchors = {}
    for row in gap['source_anchors']:
        path = row['path']; raw = safe(root / SOURCE, proof['files'][path]['artifact']).read_bytes()
        a,b,span = previous.previous.line_span(raw,*row['lines'])
        if (a,b,f.sha(span)) != tuple(row['anchor'][k] for k in ('start_byte','end_byte_exclusive','span_sha256')):
            raise ValueError('source anchor coordinates differ')
        anchors[row['id']] = dict(path=path, file_sha256=f.sha(raw), start_byte=a, end_byte_exclusive=b,
                                  span_sha256=f.sha(span), text=span.decode(), source_fact=row['fact'])
    requirement_ids = [r['id'] for r in notes['requirements']]
    if len(requirement_ids) != 12 or len(set(requirement_ids)) != 12: raise ValueError('requirement population differs')
    candidates, matrix, overlays, diffs = [], [], {}, {}
    for note, variant in zip(notes['candidates'],old['variants']):
        name = note['id']; method = (root / TICKET / variant['artifact']).read_bytes()
        witnesses = []
        for text in note['witnesses']:
            start = method.find(text.encode())
            if start < 0: raise ValueError('candidate witness missing')
            witnesses.append(dict(text=text,start_byte=start,end_byte_exclusive=start+len(text.encode()),span_sha256=f.sha(text.encode())))
        overlay = original_file.replace(original_method,method,1)
        diff = ''.join(difflib.unified_diff(original_file.decode().splitlines(True),overlay.decode().splitlines(True),
                                           fromfile='original/'+source_path,tofile=name+'/'+source_path)).encode()
        overlays[name], diffs[name] = overlay, diff
        record = dict(id=name,artifact=str(TICKET / variant['artifact']),method_sha256=f.sha(method),
                      base_commit=f.SOURCE_COMMIT,base_file_sha256=f.sha(original_file),full_file_overlay_sha256=f.sha(overlay),
                      diff_sha256=f.sha(diff),applied_project_commit=None,applied_project_tree=None,
                      local_AI_membership=note['local_membership'], rationale=note['rationale'],witnesses=witnesses,
                      actual_API_membership='unknown',independent_acceptable_membership=None,
                      performance_or_fairness=None,original_bytes_alias='ORIGINAL' if name=='MUT-REINTRO' else None)
        candidates.append(record)
        for scenario in notes['scenarios']:
            units = []
            for req in notes['requirements']:
                if req['tier']=='actual_API' or req['id'] in ('SOURCE-BOUNDARY','OWNED-CLEANUP'):
                    label='runtime_unknown'; reason=req['unknown']
                elif req['id'] not in scenario['required']:
                    label='not_applicable'; reason='Outside this explicit finite scenario requirement set; not an absence/safety claim.'
                elif name=='MUT-EARLY' and req['id'] in ('METHOD-RETURN','METHOD-ONCE','METHOD-REMOVE','METHOD-ACK','LATE-DRAIN','BLOCKED-DELIVERY'):
                    label='source_conflict'; reason='Empty Once body performs no uninstall/ack handshake. Fast return does not satisfy '+req['assertion']
                elif name in ('ORIGINAL','MUT-REINTRO') and scenario['id']=='blocked_delivery_then_consumer_exit' and req['id'] in ('METHOD-RETURN','BLOCKED-DELIVERY'):
                    label='source_conflict'; reason='Verified conditional sole-receiver exit leaves two sends waiting before original drain loop is reachable.'
                else:
                    label='conditional_source_fit'; reason=note['rationale']+' Required finite predicate: '+req['assertion']
                units.append(dict(id=req['id'],label=label,rationale=reason,source_anchor_ids=req['anchors'],
                                  observed=False,independently_approved=False))
            sets={label:[u['id'] for u in units if u['label']==label] for label in
                  ('conditional_source_fit','source_conflict','runtime_unknown','not_applicable')}
            matrix.append(dict(candidate_id=name,scenario_id=scenario['id'],preconditions=scenario['preconditions'],
                               unit_ids=requirement_ids,units=units,
                               answer_sets=sets,
                               empty_set_rationales={label:'No '+label+' unit among these12 source/finite-scenario requirements; actual API/runtime/independent review remain unknown.' for label,ids in sets.items() if not ids},
                               bounded_method_expectation='fails_AI_method_predicates' if any(u['label']=='source_conflict' for u in units) else 'conditional_AI_method_fit',
                               actual_API_result=None,formal_quality_score=None))
    if overlays['ORIGINAL']!=overlays['MUT-REINTRO']: raise ValueError('reintroduction alias differs')
    public=f.read(root/TICKET/'public-ticket.json');public.update(split='DEV',formal_split=None,exposure='already_exposed_DEV_not_fresh_FINAL')
    requirements=[dict(r,source_refs=[anchors[k] for k in r['anchors']],independent_approved=False) for r in notes['requirements']]
    sets={k:[c['id'] for c in candidates if c['local_AI_membership']==k] for k in ('acceptable','unacceptable','unknown')}
    membership=dict(scope='finite5_method_schedules_only_not_actual_API_or_product_acceptance',
                    candidate_universe=[c['id'] for c in candidates],AI_bounded_answer_sets=sets,
                    empty_set_rationales={'unknown':'All5 method-level candidates classified under stated finite assumptions; actual_API membership of all5 remains unknown.'},
                    actual_API_unknown=[c['id'] for c in candidates],independent_acceptable_candidate_set=None,
                    approved=False,reviewer=None,review_receipt=None,canonical_selection=None,
                    semantics='CAND-A/B are conditional local fits, not independent acceptable set; ties/abstention allowed')
    summary=dict(kind=KIND,DEV_tickets=1,work_items=['WI-V1','WI-V2'],scenarios=5,candidate_roles=5,
                 unique_method_byte_variants=len({c['method_sha256'] for c in candidates}),
                 scenario_candidate_cells=len(matrix),requirement_units=sum(len(r['units']) for r in matrix),
                 labels=dict(Counter(u['label'] for r in matrix for u in r['units'])),
                 local_method_fits=sum(r['bounded_method_expectation']=='conditional_AI_method_fit' for r in matrix),
                 local_method_conflicts=sum(r['bounded_method_expectation']=='fails_AI_method_predicates' for r in matrix),
                 actual_API_unknown_candidates=5,independent_approved=0,independent_tickets=0,FINAL=0,
                 D04='미정',formal_completed=0,formal_total=9,freeze=None,execution_allowed=False,
                 actual_API_calls=0,new_model_calls=0,new_target_trials=0,current_scores=None)
    return dict(public=public,requirements=requirements,candidates=candidates,matrix=matrix,membership=membership,
                anchors=anchors,overlays=overlays,diffs=diffs,summary=summary,secondary=notes['secondary_derivation'])


EVENTS = ('call_start','uninstall_request','removed','ack','call_return','status_use','notification',
          'rpc_unsubscribe_true','cleanup_done')


def trace_oracle(trace):
    """Draft oracle over complete, controlled event traces. No transport/run claim."""
    if (not isinstance(trace,dict) or trace.get('kind')!='controlled_AI_DEV_trace'
            or trace.get('complete_event_types')!=list(EVENTS)
            or not isinstance(trace.get('expected_call_ids'),list) or not trace['expected_call_ids']
            or not all(isinstance(c,str) and c for c in trace['expected_call_ids'])
            or len(set(trace['expected_call_ids']))!=len(trace['expected_call_ids'])
            or not isinstance(trace.get('events'),list)):
        return dict(status='UNKNOWN',reason='incomplete controlled trace contract',independent_oracle=False)
    events=trace['events'];seq=-1;seen=set();ack=None;removed=None;returned={};requests=[];violations=[];starts={};unknown=[];cleanup=None
    for e in events:
        if (not isinstance(e,dict) or type(e.get('seq'))is not int or e['seq']<=seq
                or e.get('type') not in EVENTS or e.get('subscription_id')!=trace.get('subscription_id')
                or not isinstance(trace.get('subscription_id'),str) or not trace['subscription_id']):
            return dict(status='ERROR',reason='invalid order/event/subscription identity',independent_oracle=False)
        seq=e['seq'];kind=e['type'];seen.add(kind)
        if kind in ('call_start','call_return'):
            call=e.get('call_id')
            if call not in trace['expected_call_ids']: return dict(status='ERROR',reason='foreign call identity',independent_oracle=False)
            if kind=='call_start':
                if call in starts: return dict(status='ERROR',reason='duplicate call start',independent_oracle=False)
                starts[call]=seq
            else:
                if call not in starts or call in returned: return dict(status='ERROR',reason='missing start or duplicate return',independent_oracle=False)
                returned[call]=seq
                if ack is None: violations.append('return_before_ack')
        if kind=='uninstall_request':
            if not isinstance(e.get('request_id'),str) or not e['request_id']: return dict(status='ERROR',reason='missing request identity',independent_oracle=False)
            requests.append(e['request_id'])
            if len(requests)>1: violations.append('more_than_one_request')
        if kind in ('removed','ack'):
            if not requests or e.get('request_id') not in requests: return dict(status='ERROR',reason='foreign or unbound handshake request',independent_oracle=False)
            if kind=='removed':
                if removed is not None: return dict(status='ERROR',reason='duplicate removal',independent_oracle=False)
                removed=seq
            else:
                if ack is not None: return dict(status='ERROR',reason='duplicate acknowledgement',independent_oracle=False)
                ack=seq
                if removed is None: violations.append('ack_before_removal')
        if kind=='status_use' and returned: violations.append('status_use_after_return')
        if kind=='cleanup_done':
            if type(e.get('owned_remaining'))is not int or e['owned_remaining']!=0:
                return dict(status='ERROR',reason='owned cleanup failed or unproved',independent_oracle=False)
            cleanup=seq
        # Public RPC true does NOT set downstream ack or call-return.
    if violations: status='BOUNDED_FAIL'
    else:
        if len(requests)!=1: unknown.append('no_request_receipt')
        if set(returned)!=set(trace['expected_call_ids']): unknown.append('unreturned_calls')
        if ack is None or removed is None: unknown.append('missing_handshake')
        if 'cleanup_done' not in seen: unknown.append('owned_cleanup_unproved')
        elif returned and cleanup<max(returned.values()): unknown.append('cleanup_receipt_precedes_final_return')
        if trace.get('requires_notification')is True and 'notification' not in seen: unknown.append('normal_notification_unproved')
        status='UNKNOWN' if unknown else 'BOUNDED_PASS'
    return dict(status=status,violations=sorted(set(violations)),unknown=unknown,
                independent_oracle=False,actual_API_receipt=False,quality_score=None)


def contract():
    return dict(status='AI_DEV_not_frozen',unit='one exposed ticket,5 correlated schedules,5 roles/4 unique method bytes',
                source='commit/file/method/witness and proposed full-file/diff SHA; proposed overlay is not applied commit/tree or binary',
                V1='12 explicit requirements x25 candidate-scenario cells; conditional source fit, conflict, runtime unknown and not-applicable all preserved',
                V2='A/B local conditional fits only; actualAPI unknown5; independent acceptable set null, never inferred from old fixture agreement',
                primary='controlled event trace predicates; complete event capture contract required; RPC true is not downstream ack/return',
                secondary='separate wait-dependency representation with same AI/source bias, NOT independently derived executable oracle',
                API='subscription-capable eth_subscribe syncing and eth_unsubscribe path required; unsupported/error/missing tier not RED or method pass',
                mutation='ORIGINAL and REINTRO are identical alias controls; EARLY rejects return-only overfit; each stage/source binding retained',
                candidate_decision='all-bad->none, equal sufficient candidates->TIED, any mandatory gate/membership unknown->DEFERRED; no default A/B choice',
                comparison='independent correctness/regression/mutation/API/cleanup gates precede performance/memory/fairness/maintenance axes; no invented scores',
                denominator='one ticket not2;5 schedules not5 tickets; errors/unknown/abstention retain planned slots; no success-only filter',
                exposure='all DEV previously exposed; new independent DEV/FINAL families and2-ticket sufficiency pending',
                execution='installed four-role handoff/source finalHEAD and exact publicAPI transport not performed',
                independent_review=None,formal_scoring_allowed=False)


def review_ledger(data):
    return dict(complete=False,independent_approved=0,independent_acceptable_set=None,
                actual_API_unknown_candidates=data['membership']['actual_API_unknown'],
                unresolved_requirements=[dict(id=r['id'],unknown=r['unknown']) for r in data['requirements']],
                missing=['actual_bound_API_subscription_and_unsubscribe_trace','independent_ticket_and_primary_derived_oracles',
                         'independently_reviewed_candidate_membership','applied_candidate_commit_tree_binary_and_drivers',
                         'timing_fairness_loop_shutdown_and_product_contract','actual_mutant_and_normal_tiers',
                         'owned_cleanup_and_installed_four_role_finalHEAD','fresh_DEV_FINAL_sufficient_families',
                         'comparison_axes_and_tie_abstention_review','D04_and_F01_F08'])


def historical_errors(root=ROOT):
    errors=previous.historical_errors(root)
    for relative in (previous.PACKET,TICKET,GAPS):
        p=root/relative;m=f.read(p/'manifest.json')
        if (p/'manifest.json').read_bytes()!=subprocess.check_output(['git','show',REVISION+':'+str(relative/'manifest.json')],cwd=root): errors.append('historical manifest differs '+str(relative))
        for name,h in m['artifacts'].items():
            if f.sha(safe(p,name).read_bytes())!=h:errors.append('historical artifact differs '+str(relative/name))
        for t in m.get('tools',[]):
            if f.sha(subprocess.check_output(['git','show',REVISION+':'+t['path']],cwd=root))!=t['sha256']:errors.append('historical tool differs '+t['path'])
    return errors


def reference(root=ROOT):
    return dict(manifest=str(PACKET/'manifest.json'),sha256=f.sha((root/PACKET/'manifest.json').read_bytes()),
                status='WIV1_V2_DEV1_bounded_AI_oracle_membership_not_independent_gold',report=str(PACKET/'REPORT.txt'),worklist=str(PACKET/'WORKLIST.json'))


def current_indices(root=ROOT,protocol_override=None,ledger_override=None):
    paths=[Path('docs/spec-driven/OPERATIONAL-ACCEPTANCE-'+n+'.json') for n in ('PROTOCOL','PROGRESS','DECISION-PROPOSAL')]+[BASE/'GAPS.json']
    docs=[f.read(root/p) for p in paths]
    if protocol_override is not None:docs[0]=protocol_override
    if ledger_override is not None:docs[1]=ledger_override
    ref=reference(root);state=f.read(root/PACKET/'WORKLIST.json');current=dict(kind=KIND,manifest=ref['manifest'],manifest_sha256=ref['sha256']);errors=[]
    old=json.loads(subprocess.check_output(['git','show',REVISION+':'+str(paths[0])],cwd=root))
    for d in docs:
        if d.get('ticket_dev_gold_preparation')!=ref or d.get('current_execution_state')!=state or d.get('current_readiness_preparation')!=current:errors.append('ticket current reference differs')
        for key in PREPARATIONS:
            expected=dict(old[key])
            if key=='wi4_dev_gold_preparation':expected['historical_source_revision']=REVISION
            if d.get(key)!=expected:errors.append('previous preparation promoted '+key)
    if docs[0]['freeze']is not None or docs[0]['execution_allowed']is not False or any(docs[0]['roles'].values()):errors.append('AI draft cannot freeze or assign independent reviewers')
    link=docs[0]['adoption_acceptance_decision_proposal']
    if link['sha256']!=f.sha((root/paths[2]).read_bytes()) or any(d.get('adoption_acceptance_decision_proposal')!=link for d in (docs[1],docs[3])):errors.append('proposal link differs')
    return errors


def validate(root=ROOT,packet=None):
    p=packet or root/PACKET;errors=[]
    try:
        m=f.read(p/'manifest.json');inventory={x.relative_to(p).as_posix():f.sha(x.read_bytes()) for x in p.rglob('*') if x.is_file() and x!=p/'manifest.json'}
        if m['kind']!=KIND or m['previous_source_revision']!=REVISION or [t['path'] for t in m['tools']]!=TOOLS or m['artifacts']!=inventory:errors.append('ticket manifest scope/hash differs')
        for t in m['tools']:
            if f.sha((root/t['path']).read_bytes())!=t['sha256']:errors.append('source tool differs '+t['path'])
        errors.extend(historical_errors(root));data=derive(root)
        for name,key in [('public/DEV-ticket.json','public'),('private/requirements.json','requirements'),('private/candidates.json','candidates'),('private/AI-matrix.json','matrix'),('private/AI-membership.json','membership'),('private/source-anchors.json','anchors'),('SUMMARY.json','summary'),('private/secondary-derivation.json','secondary')]:
            if f.read(p/name)!=data[key]:errors.append('ticket derived data differs '+name)
        for key in data['overlays']:
            if (p/'private/overlays'/(key+'.go.txt')).read_bytes()!=data['overlays'][key] or (p/'private/diffs'/(key+'.diff.txt')).read_bytes()!=data['diffs'][key]:errors.append('candidate overlay/diff differs '+key)
        if f.read(p/'matching-contract.json')!=contract():errors.append('matching contract changed or promoted')
        if f.read(p/'REVIEW-LEDGER.json')!=review_ledger(data):errors.append('review debt erased or promoted')
        state=f.read(p/'WORKLIST.json')
        if state['formal_completed']!=0 or state['formal_total']!=9 or state['reviewer_assignment']!='미정' or state['independent_quality_or_ontology_acceptance']is not False or state['production_activation']is not False or state['publication']is not False:errors.append('worklist falsely completed')
        from independent_acceptance import native_reuse_record
        if native_reuse_record(root)['changed_bound_files']:errors.append('native source changed')
    except (ValueError,TypeError,KeyError,OSError,IndexError,StopIteration) as exc:errors.append(str(exc))
    return errors
