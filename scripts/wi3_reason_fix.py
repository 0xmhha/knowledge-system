"""Fresh native and retained MCP proof for typed unresolved-seed failure metadata."""
from collections import Counter
import gzip,json,subprocess
from pathlib import Path
import acceptance_debt_diagnostic as previous
ROOT,BASE,f=previous.ROOT,previous.BASE,previous.f
PACKET=BASE/'followups/wi3-reason-fix-v1'
REVISION='3869a548b6d57dc7e3cdd1caf3c9f532399698f8'
KIND='WI3_typed_failure_reason_native_MCP_not_formal_execution'
CHANGED=['internal/system/suspects/compose.go','internal/system/suspects/compose_test.go']
TOOLS=['scripts/'+n for n in ('wi3_reason_fix.py','prepare-wi3-reason-fix.py','check-wi3-reason-fix.py','test_wi3_reason_fix.py','verify-wi3-reasons-native.py','verify-wi3-reasons-runtime.py','acceptance_debt_diagnostic.py','prepare-acceptance-debt-diagnostic.py','eval-value-dependencies.py','independent_acceptance.py','check-operational-acceptance.py','test_operational_acceptance.py')]
NATIVE_LABELS=['build','test','race','lint','build-bins','build-runtime','build-info']


def packed(p):return json.loads(gzip.decompress(p.read_bytes()))
def events(raw):
    return [json.loads(line) for line in raw.decode().splitlines() if line.strip()]


def native_errors(root,p):
    errors=[];binding=f.read(p/'source-binding.json');old=f.read(root/BASE/'followups/stablenet-comparison-v1/source-binding.json')
    if binding.get('base_revision')!=REVISION or binding.get('changed_native_files')!=CHANGED or set(binding['source_sha256'])!=set(old['source_sha256']) or binding['approved_input_sha256']!=old['approved_input_sha256']:errors.append('full native/approved input scope differs')
    inventory=subprocess.check_output(['git','ls-files','--cached','--others','--exclude-standard','*.go'],cwd=root,text=True)
    live={name for name in inventory.splitlines() if name.startswith(('cmd/','internal/','pkg/'))}
    if live!={name for name in binding['source_sha256'] if name.endswith('.go')}:errors.append('current native Go inventory differs')
    changed=sorted(name for name,h in old['source_sha256'].items() if binding['source_sha256'][name]!=h)
    if changed!=sorted(CHANGED):errors.append('unexpected native changes')
    for name,h in {**binding['source_sha256'],**binding['approved_input_sha256']}.items():
        if f.sha((root/name).read_bytes())!=h:errors.append('current source binding differs '+name)
    rs=f.read(p/'native/receipts.json')
    if [r['label'] for r in rs]!=NATIVE_LABELS:errors.append('fresh native required checks missing')
    for r in rs:
        if r['exit_code']!=0 or r['timeout'] or r['kill_sent'] or r['term_sent'] or not r['reaped'] or r['source_head']!=REVISION or r['source_binding_sha256']!=f.sha((p/'source-binding.json').read_bytes()):errors.append('native command failure/source mismatch '+r['label'])
        for suffix in ('stdout','stderr'):
            if f.sha(gzip.decompress((p/'native'/(r['label']+'.'+suffix+'.gz')).read_bytes()))!=r[suffix+'_sha256']:errors.append('native command raw log differs')
        args=r['argv']
        if r['label']=='build' and args!=['go','build','./...']:errors.append('build scope narrowed')
        if r['label']in('test','race'):
            expected=['go','test']+(['-race'] if r['label']=='race' else [])+['-json','-count=1','-timeout='+('20m' if r['label']=='race' else '18m'),'./...']
            if args!=expected:errors.append('native test scope narrowed')
            data=events(gzip.decompress((p/'native'/(r['label']+'.stdout.gz')).read_bytes()));tests=Counter(d['Action'] for d in data if d.get('Test') and d['Action']in('pass','fail','skip'));packages=Counter(d['Action'] for d in data if not d.get('Test') and d['Action']in('pass','fail','skip'))
            if any(r['tests'][k]!=tests[k] for k in ('pass','fail','skip')) or any(r['packages'][k]!=packages[k] for k in ('pass','fail','skip')) or tests['fail'] or packages['fail'] or tests['pass']<4097:errors.append('native raw test counts differ/fail')
            passed={d['Test'] for d in data if d.get('Test') and d['Action']=='pass'}
            historic=events(gzip.decompress((root/BASE/'followups/stablenet-comparison-v1/validation/native'/(r['label']+'.stdout.gz')).read_bytes()))
            old_pass={(d['Package'],d['Test']) for d in historic if d.get('Test') and d['Action']=='pass'}
            current_pass={(d['Package'],d['Test']) for d in data if d.get('Test') and d['Action']=='pass'}
            if not old_pass<=current_pass:errors.append('previous passing native test cases omitted')
            old_skip={(d['Package'],d['Test']) for d in historic if d.get('Test') and d['Action']=='skip'}
            current_skip={(d['Package'],d['Test']) for d in data if d.get('Test') and d['Action']=='skip'}
            if old_skip!=current_skip:errors.append('native skip scope changed without review')
            if not {'TestFailureReasonsDoNotConflateSeedBackendAndBudget','TestUnresolvedSeedSharedSignalsStayPartialBeforeOrAfterSuccess'}<=passed:errors.append('new native regressions not executed')
        if r['label']=='lint' and args[:2]!=['make','lint']:errors.append('full lint scope narrowed')
    installed=f.read(p/'native/installed-tools.json')
    if 'version 2.14.0 ' not in installed['lint_version'] or not installed['go_version'].startswith('go version go1.26.8 '):errors.append('installed native tool versions differ')
    lint_log=gzip.decompress((p/'native/lint.stdout.gz').read_bytes()).decode()
    if '0 issues.' not in lint_log or 'go vet ./...' not in lint_log or 'documentation: OK (134 live documents)' not in lint_log:errors.append('full native lint/vet/docs execution not observed')
    binary=f.read(p/'native/binary.json');runtime=f.read(p/'runtime/receipt.json')
    if runtime['binary_sha256']!=binary['sha256'] or binary['source_head']!=REVISION or binary['native_source_binding_sha256']!=f.sha((p/'source-binding.json').read_bytes()):errors.append('runtime binary not freshly native-bound')
    return errors


def unpack(row):
    raw=row['raw_response']
    if 'error'in raw:return None,'protocol_error'
    res=raw.get('result',{})
    if res.get('isError'):return None,'tool_error'
    v=res.get('structuredContent')
    return (v,'partial' if v and v.get('partial') else 'structured_response') if isinstance(v,dict) else (None,'invalid_response')


def runtime_data(root,p):
    raw=packed(p/'runtime/primary-results.json.gz');planned=f.read(root/previous.OLD.parent/'wi3-dev-gold-v1/public/wire-requests.json')['requests']
    if [r['case_id'] for r in raw]!=[r['case_id'] for r in planned]:raise ValueError('original20 planned primary denominator differs')
    historical=f.read(root/previous.OLD/'runtime/primary-results.json')
    for a,b in zip(historical,raw):
        old_value,_=unpack(a);new_value,_=unpack(b)
        if new_value is not None:
            project=lambda v:[(c['citation'],c.get('retained_citation'),c['symbol']) for c in v['candidates']]
            if project(old_value)!=project(new_value):raise ValueError('original candidate source identities or order changed')
    cases=[];status=Counter();reasons=Counter()
    for r,plan in zip(raw,planned):
        if r['arguments']!=plan['arguments']:raise ValueError('primary input/caps changed')
        value,state=unpack(r);status[state]+=1
        if value is None:
            cases.append(dict(case_id=r['case_id'],status=state,candidates=None,failure_reasons=None,stop_reasons=None,raw_error_retained=True))
            continue
        current=Counter()
        for c in value['candidates']:
            for key in ('impact_category','concurrency'):
                s=c['signals'][key]
                if s['status']in('error','budget_exhausted'):current[key+':'+s.get('reason','')]+=1
        reasons.update(current);cases.append(dict(case_id=r['case_id'],status=state,candidates=len(value['candidates']),failure_reasons=dict(current),stop_reasons=value['budget']['stop_reasons']))
    controls=f.read(p/'runtime/controls.json');names=['missing_only','missing_then_known','known_then_missing','known_only','foreign_project','invalid_candidate_cap','wrong_commit']
    if [r['case_id'] for r in controls]!=names:raise ValueError('controlled input denominator/order differs')
    declared=f.read(p/'runtime/planned-controls.json')
    if [(r['case_id'],r['arguments']) for r in controls]!=[(r['case_id'],r['arguments']) for r in declared]:raise ValueError('controlled inputs changed after planning')
    for row in controls:
        value,state=unpack(row)
        if row['case_id']in names[:3]:
            if value is None or not value['partial']:raise ValueError('missing seed erased partial')
            selected=value['candidates'][:3]
            if not selected:raise ValueError('missing controlled source candidates')
            for c in selected:
                for key in ('impact_category','concurrency'):
                    s=c['signals'][key]
                    if s['status']!='error' or s['reason']!='seed_unresolved' or s['value']is not None:raise ValueError('missing seed reason/status/value wrong')
        elif row['case_id']=='known_only':
            if value is None or value['candidates'][0]['signals']['impact_category']['status']!='observed' or value['candidates'][0]['signals']['concurrency']['status']!='observed':raise ValueError('known-only graph response not observed')
        elif state!='tool_error':raise ValueError('foreign/invalid/stale controlled refusal failed')
    before=packed(p/'runtime/retained-before.json.gz');after=packed(p/'runtime/retained-after.json.gz')
    if before!=after or before!=f.read(root/previous.OLD/'runtime/retained-after.json'):raise ValueError('retained primary graph/source/vector changed')
    for phase in ('before','after'):
        model=f.read(p/'runtime'/('model-'+phase+'.json'))
        if model['model']['digest']!='7907646426070047a77226ac3e684fbbe8410524f7b4a74d02837e43f2146bab':raise ValueError('actual BGE-M3 digest changed')
    session=f.read(p/'runtime/session.json')
    if session['exit']!=0 or session['requests']!=29 or session['startup_timeout_seconds']!=120 or session['tool_transport_timeout_seconds']!=20:raise ValueError('actual MCP session/transport/cleanup differs')
    concurrent=packed(p/'concurrent-runtime/primary-results.json.gz')
    if [r['case_id'] for r in concurrent]!=[r['case_id'] for r in planned] or any(a['arguments']!=b['arguments'] for a,b in zip(concurrent,planned)):raise ValueError('concurrent error denominator or inputs omitted')
    concurrent_states=dict(Counter(unpack(r)[1] for r in concurrent))
    if concurrent_states!={'tool_error':11,'partial':9}:raise ValueError('recorded concurrent observations changed')
    if f.read(p/'concurrent-runtime/receipt.json')['binary_sha256']!=f.read(p/'runtime/receipt.json')['binary_sha256']:raise ValueError('concurrent/serial binaries differ')
    receipt=f.read(p/'runtime/receipt.json')
    for key,value in dict(planned_primary=20,attempted_primary=20,planned_controls=7,attempted_controls=7,primary_retries=0,unreached_primary=0,EOF_exit=0,paid_or_external_LLM_calls=0,installed_agent_role_runs=0,embedding_inference_call_count=None,formal_completed=0,independent_approved=0,current_scores=None).items():
        if receipt.get(key)!=value:raise ValueError('MCP execution/denominator/role scope differs '+key)
    initial=f.read(p/'initial-startup/attempt.json')
    if initial['initialize_timeout_seconds']!=20 or initial['tools_attempted']!=0 or initial['unreached_primary']!=20 or initial['EOF_exit']!=0:raise ValueError('initial timeout/unreached attempt omitted')
    return dict(cases=cases,statuses=dict(status),failure_reasons=dict(reasons),controls=names,initial=initial,session=session,concurrent_statuses=concurrent_states)


def derive(root=ROOT,packet=None):
    p=packet or root/PACKET;runtime=runtime_data(root,p);native=f.read(p/'native/receipts.json')
    return dict(cases=runtime['cases'],summary=dict(kind=KIND,native_checks=[r['label'] for r in native],fresh_native_test=next(r['tests'] for r in native if r['label']=='test'),fresh_native_race=next(r['tests'] for r in native if r['label']=='race'),
        primary_planned=20,primary_attempted=20,primary_statuses=runtime['statuses'],concurrent_primary_statuses=runtime['concurrent_statuses'],primary_failure_reasons=runtime['failure_reasons'],controlled_requests=7,actual_primary_retries=0,original_candidate_identity_order_equal_cases=sum(r['candidates']is not None for r in runtime['cases']),
        initialization_attempts=2,initial_tools_attempted=0,initial_unreached_primary=20,initial_EOF_exit=0,completed_EOF_exit=0,
        current_model='BGE-M3/native1024',model_runtime=f.read(p/'runtime/model-after.json')['runtime'],startup_elapsed_seconds=runtime['session']['startup_elapsed_seconds'],
        embedding_inference_call_count=None,paid_or_external_LLM_calls=0,installed_agent_role_runs=0,retained_preserved=True,
        primary_quality_scores=None,independent_approved=0,FINAL=0,formal_completed=0,formal_total=9,freeze=None,execution_allowed=False))


def contract():
    return dict(change='Typed/wrapped ErrSeedUnresolved -> status error/reason seed_unresolved; other opaque errors remain backend_error; budget precedence unchanged. No raw backend message leak.',
        aggregate='Partial/null/provenance and failure retention across later success preserved. A signal reason is aggregate metadata, not a complete per-call failure timeline or exclusive historical cause.',
        original='Earlier20 partial/error rows remain unchanged; current repeated exposed inputs are regression only, not fresh independent samples or primary quality improvements.',
        startup='Initial20second handshake timed out;0 tool calls/20 unreached, normal EOF0 preserved. Explicit new120second startup budget only; unchanged20second per-call transport and1000ms tool budget. Not a production startup SLA pass.',
        source='Fresh whole-module build/general/race/fmt/vet/pinned lint/boundaries/docs and matching native binary;1738 native input hashes/5 approved inputs,2 exact Go changes; historical native receipt never relabelled current.',
        privacy=previous.previous.previous.reviewer_identity_policy(),independent_review=None,formal_scoring_allowed=False)


def historical_errors(root=ROOT):
    errors=previous.historical_errors(root);relative=previous.PACKET;p=root/relative;m=f.read(p/'manifest.json')
    if (p/'manifest.json').read_bytes()!=subprocess.check_output(['git','show',REVISION+':'+str(relative/'manifest.json')],cwd=root):errors.append('historical diagnostic manifest differs')
    for n,h in m['artifacts'].items():
        if f.sha((p/n).read_bytes())!=h:errors.append('historical diagnostic artifact differs '+n)
    for t in m['tools']:
        if f.sha(subprocess.check_output(['git','show',REVISION+':'+t['path']],cwd=root))!=t['sha256']:errors.append('historical diagnostic tool differs '+t['path'])
    return errors


def reference(root=ROOT):
    return dict(manifest=str(PACKET/'manifest.json'),sha256=f.sha((root/PACKET/'manifest.json').read_bytes()),status='WI3_typed_reason_fresh_native_MCP_regression_not_independent_acceptance',report=str(PACKET/'REPORT.txt'),worklist=str(PACKET/'WORKLIST.json'))


def current_indices(root=ROOT,protocol_override=None,ledger_override=None):
    paths=[Path('docs/spec-driven/OPERATIONAL-ACCEPTANCE-'+n+'.json') for n in ('PROTOCOL','PROGRESS','DECISION-PROPOSAL')]+[BASE/'GAPS.json'];docs=[f.read(root/p) for p in paths]
    if protocol_override is not None:docs[0]=protocol_override
    if ledger_override is not None:docs[1]=ledger_override
    ref=reference(root);state=f.read(root/PACKET/'WORKLIST.json');current=dict(kind=KIND,manifest=ref['manifest'],manifest_sha256=ref['sha256']);errors=[]
    for d,path in zip(docs,paths):
        old=json.loads(subprocess.check_output(['git','show',REVISION+':'+str(path)],cwd=root))
        if d.get('reviewer_identity_policy')!=contract()['privacy']:errors.append('reviewer privacy policy differs')
        if d.get('wi3_reason_fix')!=ref or d.get('current_execution_state')!=state or d.get('current_readiness_preparation')!=current:errors.append('typed reason current reference differs')
        for key,value in old.items():
            if key=='current_readiness_preparation' or not isinstance(value,dict) or 'manifest'not in value:continue
            if not key.endswith('_preparation') and key!='wi123_debt_diagnostic':continue
            expected=dict(value)
            if key=='wi123_debt_diagnostic':expected['historical_source_revision']=REVISION
            if d.get(key)!=expected:errors.append('previous preparation promoted '+key)
    if docs[0]['freeze']is not None or docs[0]['execution_allowed']is not False or any(docs[0]['roles'].values()):errors.append('native regression cannot freeze or assign reviewers')
    link=docs[0]['adoption_acceptance_decision_proposal']
    if link['sha256']!=f.sha((root/paths[2]).read_bytes()) or any(d.get('adoption_acceptance_decision_proposal')!=link for d in (docs[1],docs[3])):errors.append('proposal link differs')
    return errors


def validate(root=ROOT,packet=None):
    p=packet or root/PACKET;errors=[]
    try:
        m=f.read(p/'manifest.json');actual={x.relative_to(p).as_posix():f.sha(x.read_bytes()) for x in p.rglob('*') if x.is_file() and x!=p/'manifest.json'}
        if m['kind']!=KIND or m['previous_source_revision']!=REVISION or [t['path'] for t in m['tools']]!=TOOLS or actual!=m['artifacts']:errors.append('fresh reason manifest scope/hash differs')
        for t in m['tools']:
            if f.sha((root/t['path']).read_bytes())!=t['sha256']:errors.append('tool binding differs '+t['path'])
        errors.extend(historical_errors(root));errors.extend(native_errors(root,p));data=derive(root,p)
        if f.read(p/'SUMMARY.json')!=data['summary'] or f.read(p/'private/current-cases.json')!=data['cases']:errors.append('derived reason regression differs')
        if f.read(p/'matching-contract.json')!=contract():errors.append('reason regression scope promoted')
        state=f.read(p/'WORKLIST.json')
        if state['formal_completed']!=0 or state['formal_total']!=9 or state['reviewer_assignment']!='미정' or state['independent_quality_or_ontology_acceptance']is not False or state['production_activation']is not False or state['publication']is not False:errors.append('worklist falsely completed')
    except (ValueError,KeyError,TypeError,OSError,IndexError,StopIteration) as exc:errors.append(str(exc))
    return errors
