"""Verified graph walk aliases, bounded verification and request telemetry."""
from collections import Counter
import copy,gzip,json,subprocess
from pathlib import Path
import wi3_resolver_fix as previous
ROOT,BASE,f=previous.ROOT,previous.BASE,previous.f
PACKET=BASE/'followups/wi3-walk-budget-v1'
REVISION='c18821b5f6b6319e5a7d97de79fd39f9270cacf0'
KIND='WI3_verified_walk_budget_and_logical_measurement_not_formal_execution'
CHANGED=['internal/system/backendmeasure/clients.go','internal/system/backendmeasure/recorder.go','internal/system/backendmeasure/recorder_test.go','internal/system/mcp/backend_measurement_test.go','internal/system/mcp/suspects.go','internal/system/suspects/compose.go','internal/system/suspects/compose_test.go']
PRIOR_NATIVE=previous.PACKET
TOOLS=['scripts/'+n for n in ('wi3_walk_budget.py','prepare-wi3-walk-budget.py','check-wi3-walk-budget.py','test_wi3_walk_budget.py','verify-wi3-walk-runtime.py','verify-wi3-resolver-native.py','verify-wi3-reasons-native.py','eval-value-dependencies.py','wi3_resolver_fix.py','check-operational-acceptance.py','test_operational_acceptance.py')]
LABELS=['build','test','race','lint','build-bins','build-runtime','build-info']
REGRESSIONS={'TestVerifiedWideAnchorPreservesRetrievalAndReachesCanonicalEdges','TestWideAnchorRejectsUnverifiedIdentityAndInterfaceBridge','TestWorkDeadlineReservesFinalVerificationInsideTotalBudget','TestWalkIdentityCacheKeepsSuccessAndFailureObservations','TestCanonicalAliasesDoNotMergeOriginalRetrievalRows','TestVerificationFailuresDistinguishBudgetCancelAndDrift','TestLabelSuspectsRequestMeasurementMatchesLogicalBudget','TestLabelSuspectsCancelledVerificationIsNotSnapshotDrift','TestLabelSuspectsEvidenceBoundsHaveTypedRefusal','TestSuspectVerificationRefusalsDoNotExposeOpaqueErrors','TestSuspectLogicalMeasurementsPreserveProviderFailures'}
REGRESSIONS|={'TestWideAnchorRejectsUnverifiedIdentityAndInterfaceBridge/'+v for v in ('foreign_file','foreign_commit','disjoint','overlap_only','empty_symbol','ambiguous','interface_bridge')}
REGRESSIONS|={'TestWorkDeadlineReservesFinalVerificationInsideTotalBudget/'+v for v in ('false','true')}
REGRESSIONS|={'TestWalkIdentityCacheKeepsSuccessAndFailureObservations/'+v for v in ('false','true')}
REGRESSIONS|={'TestLabelSuspectsRequestMeasurementMatchesLogicalBudget/'+v for v in ('symbol','text','invalid')}
REGRESSIONS|={'TestSuspectLogicalMeasurementsPreserveProviderFailures/'+v for v in ('get_node_prs','impact_analysis','concurrency_impact')}
REGRESSIONS|={'TestWorkBudgetDoesNotMasqueradeAsMissingSeed','TestIdentityDependencyDistinguishesProviderFailureAndUnsupported'}
REGRESSIONS|={'TestIdentityDependencyDistinguishesProviderFailureAndUnsupported/'+v for v in ('provider_error','unsupported')}
REGRESSIONS|={'TestSeedProviderFailuresExposeSafeClassificationOnly'}
REGRESSIONS|={'TestSeedProviderFailuresExposeSafeClassificationOnly/'+v for v in ('symbol','semantic')}


def packed(path):return json.loads(gzip.decompress(path.read_bytes()))
def events(raw):return [json.loads(line)for line in raw.decode().splitlines()if line.strip()]


def native_errors(root,p):
    errors=[];binding=f.read(p/'source-binding.json');old=f.read(root/PRIOR_NATIVE/'source-binding.json')
    if binding['base_revision']!=REVISION or binding['changed_native_files']!=CHANGED or binding['source_mode']!='working_tree_7_bound_Go_changes' or set(binding['source_sha256'])!=set(old['source_sha256']) or binding['approved_input_sha256']!=old['approved_input_sha256']:
        errors.append('native/approved input scope differs')
    changed=sorted(n for n,h in old['source_sha256'].items()if binding['source_sha256'][n]!=h)
    if changed!=sorted(CHANGED):errors.append('unexpected native changes')
    live=subprocess.check_output(['git','ls-files','--cached','--others','--exclude-standard','*.go'],cwd=root,text=True).splitlines()
    if {n for n in live if n.startswith(('cmd/','pkg/','internal/'))}!={n for n in binding['source_sha256']if n.endswith('.go')}:errors.append('native Go inventory differs')
    for name,h in {**binding['source_sha256'],**binding['approved_input_sha256']}.items():
        if f.sha((root/name).read_bytes())!=h:errors.append('current native input differs '+name)
    patch=subprocess.check_output(['git','diff','--binary',REVISION,'--']+CHANGED,cwd=root)
    if patch!=gzip.decompress((p/'code-change.patch.gz').read_bytes()):errors.append('native patch differs')
    receipts=f.read(p/'native/receipts.json')
    if [r['label']for r in receipts]!=LABELS:errors.append('required whole native commands missing')
    for r in receipts:
        if r['exit_code']!=0 or r['timeout']or r['term_sent']or r['kill_sent']or not r['reaped']or r['source_head']!=REVISION or r['source_binding_sha256']!=f.sha((p/'source-binding.json').read_bytes()):errors.append('native command failed/source changed '+r['label'])
        for ext in ('stdout','stderr'):
            if f.sha(gzip.decompress((p/'native'/(r['label']+'.'+ext+'.gz')).read_bytes()))!=r[ext+'_sha256']:errors.append('native raw log differs')
        if r['label']=='build'and r['argv']!=['go','build','./...']:errors.append('whole build narrowed')
        if r['label']in ('test','race'):
            expected=['go','test']+(['-race']if r['label']=='race'else[])+['-json','-count=1','-timeout='+('20m'if r['label']=='race'else'18m'),'./...']
            if r['argv']!=expected:errors.append('whole native test scope narrowed')
            data=events(gzip.decompress((p/'native'/(r['label']+'.stdout.gz')).read_bytes()))
            counts=Counter(x['Action']for x in data if x.get('Test')and x['Action']in ('pass','fail','skip'))
            packages=Counter(x['Action']for x in data if not x.get('Test')and x['Action']in ('pass','fail','skip'))
            if any(r['tests'][k]!=counts[k]for k in ('pass','fail','skip'))or any(r['packages'][k]!=packages[k]for k in ('pass','fail','skip'))or counts['fail']or packages['fail']or counts['pass']<4158:errors.append('native raw counts/failures differ')
            historic=events(gzip.decompress((root/PRIOR_NATIVE/'native'/(r['label']+'.stdout.gz')).read_bytes()))
            ids=lambda rows,action:{(x['Package'],x['Test'])for x in rows if x.get('Test')and x['Action']==action}
            if not ids(historic,'pass')<=ids(data,'pass')or ids(historic,'skip')!=ids(data,'skip'):errors.append('prior passing/skip scope omitted or changed')
            if not REGRESSIONS<={x['Test']for x in data if x.get('Test')and x['Action']=='pass'}:errors.append('new resolver regressions not executed')
        if r['label']=='lint'and r['argv'][:2]!=['make','lint']:errors.append('full lint narrowed')
    lint=gzip.decompress((p/'native/lint.stdout.gz').read_bytes()).decode()
    if not all(x in lint for x in ('0 issues.','go vet ./...','documentation: OK (134 live documents)')):errors.append('lint/vet/docs result missing')
    configs=f.read(p/'native/lint-config-binding.json')
    if set(configs['configuration_sha256'])!={'.golangci.yml','docs/DOCUMENTATION-CATALOG.json'}or configs['captured_before_lint']is not True or configs['postrun_verified']is not True:errors.append('lint configuration scope differs')
    for name,h in configs['configuration_sha256'].items():
        if f.sha((root/name).read_bytes())!=h or f.sha(subprocess.check_output(['git','show',REVISION+':'+name],cwd=root))!=h:errors.append('lint configuration changed '+name)
    installed=f.read(p/'native/installed-tools.json')
    if not installed['go_version'].startswith('go version go1.26.8 ')or 'version 2.14.0 'not in installed['lint_version']:errors.append('installed native tools differ')
    binary=f.read(p/'native/binary.json')
    if binary['source_head']!=REVISION or binary['source_mode']!=binding['source_mode']or binary['native_source_binding_sha256']!=f.sha((p/'source-binding.json').read_bytes()):errors.append('binary provenance differs')
    candidate=f.read(p/'native/candidate-binary.json')
    if candidate['sha256']!=binary['sha256']or candidate['source_head']!=REVISION or candidate['source_mode']!=binding['source_mode']or candidate['native_source_binding_sha256']!=f.sha((p/'source-binding.json').read_bytes()):errors.append('actual runtime candidate differs from final validated binary')
    return errors


def unpack(row):
    result=row['raw_response'].get('result',{})
    return result.get('structuredContent'),bool(result.get('isError')or'error'in row['raw_response'])


def normalized(value):
    v=copy.deepcopy(value)
    if v is not None:v['budget']['elapsed_ms']=0
    return v


def error_text(row):
    return json.dumps(row['raw_response'].get('result',{}).get('content',[]),ensure_ascii=False,sort_keys=True)


def runtime_data(root,p):
    result={};inventory=None;plans=None;config_normal=None;tools=None;measurements={}
    primary=f.read(root/BASE/'followups/wi3-dev-gold-v1/public/wire-requests.json')['requests']
    control_ids=['site','site_comment','symbol','wide_expand','canonical_expand','wrong_commit','foreign_project','foreign_dataset','forged_hash','unknown_site','invalid_call_cap']
    for phase in ('before','after'):
        q=p/(phase+'-runtime');plan=f.read(q/'planned.json');rows=packed(q/'responses.json.gz');receipt=f.read(q/'receipt.json');sessions=f.read(q/'sessions.json')
        if plans is None:plans=plan
        if plan!=plans or [r['case_id']for r in plan if r['group']=='control']!=control_ids:raise ValueError('old/new/control plan differs')
        declared=[r for r in plan if r['group']=='primary']
        if [(r['case_id'],r['arguments'])for r in declared]!=[(r['case_id'],r['arguments'])for r in primary]:raise ValueError('original20 input/caps changed')
        if len(plan)!=31 or [(r['measurement'],r['case_id'],r['arguments'])for r in rows]!=[(label,r['case_id'],r['arguments'])for label in ('off','on')for r in plan]:raise ValueError('actual62 planned attempts/order/pins differ')
        if receipt['planned']!=62 or receipt['attempted']!=62 or receipt['retries']!=0 or receipt['formal_completed']!=0 or receipt['quality_score']is not None or receipt['paid_or_external_LLM_calls']!=0 or receipt['installed_agent_role_runs']!=0:raise ValueError('runtime scope promoted')
        expected=f.read(root/PRIOR_NATIVE/'native/binary.json')['sha256']if phase=='before'else f.read(p/'native/binary.json')['sha256']
        if receipt['binary_sha256']!=expected:raise ValueError('runtime binary not source-bound')
        inv=packed(q/'before.json.gz')
        if inventory is None:inventory=inv
        if inv!=inventory or packed(q/'after.json.gz')!=inventory:raise ValueError('retained source/graph/vector changed between runs')
        if [s['measurement']for s in sessions]!=['off','on']or any(s['EOF_exit']!=0 or s['attempted']!=31 or s['requests']!=33 for s in sessions):raise ValueError('actual sessions/EOF/attempts differ')
        for when in ('before','after'):
            if f.read(q/('model-'+when+'.json'))['model']['digest']!='7907646426070047a77226ac3e684fbbe8410524f7b4a74d02837e43f2146bab':raise ValueError('local model identity differs')
        for label in ('off','on'):
            config=(q/label/'config.txt').read_text()
            normal='\n'.join(s for s in config.splitlines()if not s.strip().startswith('footprint_dir:')).replace('measure_backend_calls: true','measure_backend_calls: false')
            if config_normal is None:config_normal=normal
            if normal!=config_normal:raise ValueError('non-measurement config changed')
            actual_tools=f.read(q/label/'tools.json')
            if tools is None:tools=actual_tools
            if actual_tools!=tools:raise ValueError('exposed MCP tools changed')
            stdin=events(gzip.decompress((q/label/'mcp.stdin.jsonl.gz').read_bytes()));stdout=events(gzip.decompress((q/label/'mcp.stdout.jsonl.gz').read_bytes()));replies={r['id']:r for r in stdout if'id'in r}
            calls=[r for r in stdin if r['method']=='tools/call'];actual=[r for r in rows if r['measurement']==label]
            if len(calls)!=31:raise ValueError('actual stdin call denominator differs')
            for call,row in zip(calls,actual):
                mid=f.sha((label+':'+row['case_id']).encode())[:32]
                if row['measurement_id']!=mid or call['params']!=dict(name='cks.context.label_suspects',arguments=row['arguments'],_meta={'cks.measurement_id':mid})or replies.get(call['id'])!=row['raw_response']:raise ValueError('actual request/meta/response differs')
            summaries=f.read(q/label/'measurement-summaries.json')
            # Re-derive emitted summaries from the original footprint stream.
            raw_summaries=[]
            for log in sorted((q/label/'footprints').rglob('*.jsonl.gz')):
                raw_summaries.extend(r['summary']for r in events(gzip.decompress(log.read_bytes()))if r.get('event')=='measurement.backend_calls')
            if summaries!=raw_summaries:raise ValueError('measurement extraction differs from emitted footprint')
            measured=[s for s in summaries if s['tool']=='cks.context.label_suspects']
            if phase=='before' or label=='off':
                if measured:raise ValueError('baseline/off unexpectedly has automatic label scope')
            else:
                if len(measured)!=31 or len({s['measurement_id']for s in measured})!=31:raise ValueError('new label scope missing/duplicated')
                by={s['measurement_id']:s for s in measured}
                for row in actual:
                    summary=by[row['measurement_id']];value,is_error=unpack(row);logical=[c for c in summary['calls']if c['backend']!='ollama_http']
                    if summary['pending_calls']!=0 or summary['outcome']!=('tool_error'if is_error else'returned'):raise ValueError('request scope not sealed/attributed')
                    if len(logical)>row['arguments']['max_calls']:raise ValueError('actual logical call cap exceeded')
                    if value is not None and len(logical)!=value['budget']['calls']:raise ValueError('emitted logical count differs from enforced budget')
                    if [c['ordinal']for c in summary['calls']]!=list(range(1,len(summary['calls'])+1)):raise ValueError('measurement attempt order lost')
                    for call in summary['calls']:
                        forbidden={'input','query','symbol','citation','result','error','body','prompt'}
                        if forbidden&set(call):raise ValueError('raw payload in call measurement')
                    if row['case_id']=='invalid_call_cap'and logical:raise ValueError('invalid input contacted backend')
                measurements[phase]=dict(request_summaries=len(measured),logical_calls=sum(c['backend']!='ollama_http'for s in measured for c in s['calls']),physical_HTTP_attempts=sum(c['backend']=='ollama_http'for s in measured for c in s['calls']),pending_calls=0)
        result[phase]={(r['measurement'],r['case_id']):r for r in rows}
    before,after=result['before'],result['after'];comparisons=[]
    positives=['site','symbol','wide_expand','canonical_expand']
    for cid in positives:
        old,_=unpack(before['off',cid]);off,e1=unpack(after['off',cid]);on,e2=unpack(after['on',cid])
        if e1 or e2 or off is None or normalized(off)!=normalized(on):raise ValueError('controlled measured/unmeasured output differs')
        if old is None:raise ValueError('controlled baseline unavailable')
        if off['candidates'][0]['citation']!=old['candidates'][0]['citation']:raise ValueError('original seed citation was rewritten')
        if off['candidates'][0]['signals']['retrieval']!=old['candidates'][0]['signals']['retrieval']:raise ValueError('original retrieval observations changed')
        comparisons.append(dict(case_id=cid,old_candidates=len(old['candidates']),new_candidates=len(off['candidates']),strict_off_on_equal_except_elapsed=True,original_seed_citation_preserved=True,quality_score=None))
    for label in ('off','on'):
        wide,_=unpack(after[label,'wide_expand']);canonical,_=unpack(after[label,'canonical_expand'])
        def entities(value):return {(c['symbol'],c['citation']['file'],c['citation']['commit_hash'],c['retained_citation']['project_id'],c['retained_citation']['dataset_id'])for c in value['candidates']}
        if entities(wide)!=entities(canonical):raise ValueError('verified wide/canonical expansions reach different entities')
        oldwide,_=unpack(before[label,'wide_expand'])
        if len(wide['candidates'])<=len(oldwide['candidates']):raise ValueError('actual wide-source traversal regression not repaired')
        for cid in set(control_ids)-set(positives):
            value,error=unpack(after[label,cid])
            if not error or value is not None:raise ValueError('invalid/foreign/unknown input became a result')
    primary_rows=[]
    for plan in primary:
        cid=plan['case_id'];off,e1=unpack(after['off',cid]);on,e2=unpack(after['on',cid]);equal=e1==e2 and(normalized(off)==normalized(on)if off is not None or on is not None else error_text(after['off',cid])==error_text(after['on',cid]))
        timed=lambda value,row:(value is not None and'time_budget'in value['budget']['stop_reasons'])or'budget_exhausted'in error_text(row)
        time_limited=timed(off,after['off',cid])or timed(on,after['on',cid])
        if not equal and not time_limited:raise ValueError('non-time-limited original input differs under measurement '+cid)
        primary_rows.append(dict(case_id=cid,off_status='tool_error'if e1 else'partial'if off and off['partial']else'structured',on_status='tool_error'if e2 else'partial'if on and on['partial']else'structured',
            off_candidates=None if off is None else len(off['candidates']),on_candidates=None if on is None else len(on['candidates']),strict_equal_except_elapsed=equal,time_limited=time_limited,quality_score=None))
    return dict(controls=comparisons,primary=primary_rows,measurement=measurements['after'])


def derive(root=ROOT,packet=None):
    p=packet or root/PACKET;runtime=runtime_data(root,p);native=f.read(p/'native/receipts.json');before=[]
    for label in ('core','mcp','budget-reason','seed-error'):
        rows=events(gzip.decompress((p/'targeted'/('before-'+label+'.jsonl.gz')).read_bytes()))
        failures=[r['Test']for r in rows if r.get('Test')and r['Action']=='fail']
        if not failures:raise ValueError('before-fix regression evidence missing')
        before.extend(failures)
    targeted=events(gzip.decompress((p/'targeted/targeted-final.jsonl.gz').read_bytes()))
    if any(r['Action']=='fail'for r in targeted):raise ValueError('targeted post-fix failures')
    return dict(runtime=runtime,summary=dict(kind=KIND,changed_native_files=CHANGED,native_inputs=1738,approved_inputs=5,
        fresh_native_test=next(r['tests']for r in native if r['label']=='test'),fresh_native_race=next(r['tests']for r in native if r['label']=='race'),
        targeted_pass=sum(r['Action']=='pass'for r in targeted if r.get('Test')),targeted_skip=sum(r['Action']=='skip'for r in targeted if r.get('Test')),
        reproduced_failed_test_records=len(before),old_queries=62,new_queries=62,total_actual_queries=124,EOF_exit0_sessions=4,controlled_strict_parity=4,
        original20_attempted_each_mode=20,original20_strict_parity=sum(r['strict_equal_except_elapsed']for r in runtime['primary']),original20_time_limited=sum(r['time_limited']for r in runtime['primary']),
        new_request_measurement=runtime['measurement'],original_source_index_and_inputs_preserved=True,original_seed_citations_preserved=True,source_alias_matching='validated resolver identity plus exact citation equality; no geometric/name guessing',
        total_backend_budget_extended=False,retained_verification_required=True,new_independent_samples=0,independent_approved=0,FINAL=0,formal_completed=0,formal_total=9,freeze=None,execution_allowed=False,quality_score=None,paid_or_external_LLM_calls=0,installed_agent_role_runs=0))


def superseded_errors(root,p):
    errors=[];q=p/'superseded-native';binding=f.read(q/'source-binding.json');current=f.read(p/'source-binding.json');control=f.read(p/'supersession.json');rs=f.read(q/'receipts.json')
    if binding['base_revision']!=REVISION or set(binding['source_sha256'])!=set(current['source_sha256'])or binding['approved_input_sha256']!=current['approved_input_sha256']:errors.append('superseded input scope differs')
    differences={n for n,h in binding['source_sha256'].items()if current['source_sha256'][n]!=h}
    if differences!={'internal/system/suspects/compose.go','internal/system/suspects/compose_test.go','internal/system/mcp/backend_measurement_test.go'}:errors.append('superseded/final native difference scope changed')
    for name in CHANGED:
        if f.sha(gzip.decompress((q/'source'/(name+'.txt.gz')).read_bytes()))!=binding['source_sha256'][name]:errors.append('superseded source snapshot differs '+name)
    if [r['label']for r in rs]!=['build','test','race']or any(r['exit_code']!=0 for r in rs[:2])or rs[-1]['exit_code']!=-15 or rs[-1]['pid']!=control['command_pid']or rs[-1]['argv']!=control['command_argv']or not rs[-1]['reaped']:errors.append('superseded terminal receipt differs')
    for r in rs:
        if r['source_binding_sha256']!=f.sha((q/'source-binding.json').read_bytes()):errors.append('superseded native binding differs')
        for ext in ('stdout','stderr'):
            if f.sha(gzip.decompress((q/(r['label']+'.'+ext+'.gz')).read_bytes()))!=r[ext+'_sha256']:errors.append('superseded raw log differs')
    if control['remaining_live_group_members']!=0 or control['runner_terminal_exit']!=1 or control['production_or_model_server_terminated']is not False or control['signal']!='SIGTERM':errors.append('superseded owned process cleanup not verified')
    if 'reproduced'not in control['reason']:errors.append('supersession must preserve concrete reason rather than observation timeout')
    return errors


def pre_lint_errors(p):
    errors=[];q=p/'pre-lint-native';binding=f.read(q/'source-binding.json');current=f.read(p/'source-binding.json');rs=f.read(q/'receipts.json')
    if binding['base_revision']!=REVISION or set(binding['source_sha256'])!=set(current['source_sha256'])or binding['approved_input_sha256']!=current['approved_input_sha256']:errors.append('pre-lint input scope differs')
    if {n for n,h in binding['source_sha256'].items()if current['source_sha256'][n]!=h}!={'internal/system/mcp/backend_measurement_test.go'}:errors.append('pre-lint fix changed more than the test syntax')
    for name in CHANGED:
        if f.sha(gzip.decompress((q/'source'/(name+'.txt.gz')).read_bytes()))!=binding['source_sha256'][name]:errors.append('pre-lint source snapshot differs '+name)
    if [r['label']for r in rs]!=['build','test','race','lint']or any(r['exit_code']!=0 for r in rs[:3])or rs[-1]['exit_code']==0:errors.append('pre-lint failed receipt lost')
    for r in rs:
        if r['source_binding_sha256']!=f.sha((q/'source-binding.json').read_bytes())or not r['reaped']or r['timeout']:errors.append('pre-lint source/terminal binding differs')
        for ext in ('stdout','stderr'):
            if f.sha(gzip.decompress((q/(r['label']+'.'+ext+'.gz')).read_bytes()))!=r[ext+'_sha256']:errors.append('pre-lint raw log differs')
        if r['label']in ('test','race')and r['tests']!={'pass':4158,'fail':0,'skip':18}:errors.append('pre-lint whole tests changed')
    if b'QF1003'not in gzip.decompress((q/'lint.stdout.gz').read_bytes())+gzip.decompress((q/'lint.stderr.gz').read_bytes()):errors.append('lint counterexample missing')
    return errors


def historical_errors(root=ROOT):
    errors=previous.historical_errors(root);p=root/previous.PACKET;m=f.read(p/'manifest.json')
    if (p/'manifest.json').read_bytes()!=subprocess.check_output(['git','show',REVISION+':'+str(previous.PACKET/'manifest.json')],cwd=root):errors.append('previous resolver manifest changed')
    for name,h in m['artifacts'].items():
        if f.sha((p/name).read_bytes())!=h:errors.append('previous resolver artifact changed '+name)
    for r in m['tools']:
        if f.sha(subprocess.check_output(['git','show',REVISION+':'+r['path']],cwd=root))!=r['sha256']:errors.append('previous resolver tool changed '+r['path'])
    return errors


def reference(root=ROOT):return dict(manifest=str(PACKET/'manifest.json'),sha256=f.sha((root/PACKET/'manifest.json').read_bytes()),status='WI3_verified_walk_budget_measurement_fresh_native_stdio_not_independent_acceptance',report=str(PACKET/'REPORT.txt'),worklist=str(PACKET/'WORKLIST.json'))


def current_indices(root=ROOT,protocol_override=None,ledger_override=None):
    paths=[Path('docs/spec-driven/OPERATIONAL-ACCEPTANCE-'+n+'.json')for n in ('PROTOCOL','PROGRESS','DECISION-PROPOSAL')]+[BASE/'GAPS.json'];docs=[f.read(root/x)for x in paths]
    if protocol_override is not None:docs[0]=protocol_override
    if ledger_override is not None:docs[1]=ledger_override
    ref=reference(root);state=f.read(root/PACKET/'WORKLIST.json');current=dict(kind=KIND,manifest=ref['manifest'],manifest_sha256=ref['sha256']);errors=[]
    for d,path in zip(docs,paths):
        prior=json.loads(subprocess.check_output(['git','show',REVISION+':'+str(path)],cwd=root))
        if d.get('wi3_walk_budget')!=ref or d.get('current_execution_state')!=state or d.get('current_readiness_preparation')!=current:errors.append('WI3 walk current reference/state differs')
        if d.get('reviewer_identity_policy')!=prior.get('reviewer_identity_policy'):errors.append('reviewer privacy policy changed')
        for key,value in prior.items():
            if key=='current_readiness_preparation'or not isinstance(value,dict)or'manifest'not in value:continue
            expected=dict(value)
            if key=='wi3_resolver_fix':expected['historical_source_revision']=REVISION
            if d.get(key)!=expected:errors.append('previous preparation changed '+key)
    if docs[0]['freeze']is not None or docs[0]['execution_allowed']is not False or any(docs[0]['roles'].values()):errors.append('technical graph/budget fix cannot assign/freeze/approve')
    link=docs[0]['adoption_acceptance_decision_proposal']
    if link['sha256']!=f.sha((root/paths[2]).read_bytes())or any(d.get('adoption_acceptance_decision_proposal')!=link for d in (docs[1],docs[3])):errors.append('proposal link differs')
    return errors


def storage_errors(root=ROOT):
    errors=[]
    try:
        path=root/PACKET.parent/'wi3-walk-budget-v1-finalization.json';record=f.read(path);code=record['code_revision']
        if record.get('completed_requirement_ids')!=['W09']or record.get('code_signature_verified')is not True or record.get('completion_guard_exit')!=0:errors.append('signed commit/source completion evidence not finalized')
        for relative in (PACKET/'manifest.json',PACKET/'source-binding.json'):
            if subprocess.check_output(['git','show',code+':'+str(relative)],cwd=root)!=(root/relative).read_bytes():errors.append('stored code/evidence revision differs')
        ledger=f.read(root/'docs/spec-driven/REFACTORING-PROGRESS.json')
        prior=json.loads(subprocess.check_output(['git','show',REVISION+':docs/spec-driven/REFACTORING-PROGRESS.json'],cwd=root))
        if {k:v for k,v in ledger.items()if k!='active_source_binding'}!={k:v for k,v in prior.items()if k!='active_source_binding'}:errors.append('unrelated task states changed during storage')
        expected=dict(revision=code,manifest=str(PACKET/'source-binding.json'),manifest_sha256=f.sha((root/PACKET/'source-binding.json').read_bytes()))
        if ledger['active_source_binding']!=expected or record.get('active_source_binding')!=expected:errors.append('new committed source binding not active')
        if record.get('formal_completed')!=0 or record.get('formal_total')!=9 or record.get('independent_approved')!=0 or record.get('execution_allowed')is not False:errors.append('technical storage promoted to independent acceptance')
    except (ValueError,KeyError,TypeError,OSError,subprocess.CalledProcessError):errors.append('signed commit/source binding finalization pending or invalid')
    return errors


def validate(root=ROOT,packet=None):
    p=packet or root/PACKET;errors=[]
    try:
        m=f.read(p/'manifest.json');actual={x.relative_to(p).as_posix():f.sha(x.read_bytes())for x in p.rglob('*')if x.is_file()and x!=p/'manifest.json'}
        if m['kind']!=KIND or m['previous_source_revision']!=REVISION or m['artifacts']!=actual or [r['path']for r in m['tools']]!=TOOLS:errors.append('walk packet scope/hash differs')
        for r in m['tools']:
            if f.sha((root/r['path']).read_bytes())!=r['sha256']:errors.append('walk tool changed '+r['path'])
        errors.extend(historical_errors(root));errors.extend(native_errors(root,p));errors.extend(superseded_errors(root,p));errors.extend(pre_lint_errors(p));d=derive(root,p)
        if f.read(p/'SUMMARY.json')!=d['summary']or f.read(p/'runtime-comparison.json')!=d['runtime']:errors.append('derived walk/measurement evidence differs')
        state=f.read(p/'WORKLIST.json')
        if state['formal_completed']!=0 or state['formal_total']!=9 or state['reviewer_assignment']!='미정'or state['production_activation']is not False or state['publication']is not False or state['independent_quality_or_ontology_acceptance']is not False:errors.append('worklist falsely completed')
        review=f.read(p/'acceptance-review.json')
        if [r['id']for r in review]!=['W'+str(i).zfill(2)for i in range(1,11)]or review[8]['status']!='pending_signed_commit_and_binding_finalization':errors.append('storage completion prematurely asserted in frozen technical preparation')
    except (ValueError,KeyError,TypeError,OSError,IndexError)as exc:errors.append(str(exc))
    return errors
