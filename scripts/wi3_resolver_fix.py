"""Source-bound optional resolver parity; no independent quality acceptance."""
from collections import Counter
import copy,gzip,json,subprocess
from pathlib import Path
import wi1_select_review as previous
ROOT,BASE,f=previous.ROOT,previous.BASE,previous.f
PACKET=BASE/'followups/wi3-resolver-fix-v1'
REVISION='30e43a34a5af63f1b3b4cc15df0df5316c359fb8'
KIND='WI3_optional_resolver_measurement_parity_not_formal_execution'
CHANGED=['internal/system/backendmeasure/clients.go','internal/system/backendmeasure/recorder.go','internal/system/backendmeasure/recorder_test.go','internal/system/mcp/backend_measurement_test.go']
PRIOR_NATIVE=BASE/'followups/wi3-reason-fix-v1'
TOOLS=['scripts/'+n for n in ('wi3_resolver_fix.py','prepare-wi3-resolver-fix.py','check-wi3-resolver-fix.py','test_wi3_resolver_fix.py','verify-wi3-resolver-native.py','verify-wi3-resolver-runtime.py','verify-wi3-reasons-native.py','eval-value-dependencies.py','wi1_select_review.py','check-operational-acceptance.py','test_operational_acceptance.py')]
LABELS=['build','test','race','lint','build-bins','build-runtime','build-info']
REGRESSIONS={'TestGraphMeasurementPreservesOptionalResolverCapability','TestGraphResolverMeasurementDelegatesWithoutPrivateData','TestGraphResolverDelegatesWithoutMeasurementScope','TestLabelSuspectsMeasurementPreservesResolverRoutes','TestLabelSuspectsMeasuredResolverFailureRemainsUnresolved'}
REGRESSIONS|={'TestGraphResolverMeasurementDelegatesWithoutPrivateData/'+name for name in ('success','unresolved','opaque','canceled','deadline')}
REGRESSIONS|={'TestLabelSuspectsMeasurementPreservesResolverRoutes/'+route+'/'+mode for route in ('site','symbol','expand')for mode in ('legacy','pinned')}


def packed(path):return json.loads(gzip.decompress(path.read_bytes()))
def events(raw):return [json.loads(line)for line in raw.decode().splitlines()if line.strip()]


def native_errors(root,p):
    errors=[];binding=f.read(p/'source-binding.json');old=f.read(root/PRIOR_NATIVE/'source-binding.json')
    if binding['base_revision']!=REVISION or binding['changed_native_files']!=CHANGED or binding['source_mode']!='working_tree_four_bound_Go_changes' or set(binding['source_sha256'])!=set(old['source_sha256']) or binding['approved_input_sha256']!=old['approved_input_sha256']:
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
            if any(r['tests'][k]!=counts[k]for k in ('pass','fail','skip'))or any(r['packages'][k]!=packages[k]for k in ('pass','fail','skip'))or counts['fail']or packages['fail']or counts['pass']<4107:errors.append('native raw counts/failures differ')
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
    return errors


def unpack(row):
    result=row['raw_response'].get('result',{})
    return result.get('structuredContent'),bool(result.get('isError')or'error'in row['raw_response'])


def normalized(value):
    v=copy.deepcopy(value)
    if v is not None:v['budget']['elapsed_ms']=0
    return v


def runtime_data(root,p):
    result={};plans=None;inventory=None;config_normal=None;tool_list=None
    for phase in ('before','after'):
        q=p/(phase+'-runtime');plan=f.read(q/'planned.json');rows=packed(q/'responses.json.gz');receipt=f.read(q/'receipt.json');sessions=f.read(q/'sessions.json')
        if plans is None:plans=plan
        if plans!=plan or [r['case_id']for r in plan]!=['site','site_comment','symbol','expand','wrong_commit','foreign_dataset','forged_hash','unknown_site']:
            raise ValueError('declared old/new plan differs')
        if [(r['measurement'],r['case_id'],r['arguments'])for r in rows]!=[(label,r['case_id'],r['arguments'])for label in ('off','on')for r in plan]:raise ValueError('runtime denominator/order/pins changed')
        if receipt['planned']!=16 or receipt['attempted']!=16 or receipt['retries']!=0 or receipt['formal_completed']!=0 or receipt['paid_or_external_LLM_calls']!=0 or receipt['auto_label_measurement_scope_added']is not False or receipt['quality_score']is not None:raise ValueError('runtime scope promoted')
        expected_binary=f.read(root/PRIOR_NATIVE/'native/binary.json')['sha256']if phase=='before'else f.read(p/'native/binary.json')['sha256']
        if receipt['binary_sha256']!=expected_binary:raise ValueError('runtime binary not bound to source')
        if packed(q/'before.json.gz')!=packed(q/'after.json.gz'):raise ValueError('existing source/index changed')
        current_inventory=packed(q/'before.json.gz')
        if inventory is None:inventory=current_inventory
        if inventory!=current_inventory:raise ValueError('old/new retained data versions differ')
        for label in ('off','on'):
            config=(q/label/'config.txt').read_text()
            normal='\n'.join(line for line in config.splitlines()if not line.strip().startswith('footprint_dir:')).replace('measure_backend_calls: true','measure_backend_calls: false')
            if config_normal is None:config_normal=normal
            if normal!=config_normal:raise ValueError('measurement changes non-telemetry config')
            tools=f.read(q/label/'tools.json')
            if tool_list is None:tool_list=tools
            if tools!=tool_list:raise ValueError('old/new exposed tools differ')
            stdin=events(gzip.decompress((q/label/'mcp.stdin.jsonl.gz').read_bytes()))
            stdout=events(gzip.decompress((q/label/'mcp.stdout.jsonl.gz').read_bytes()))
            calls=[r for r in stdin if r['method']=='tools/call']
            actual=[r for r in rows if r['measurement']==label]
            replies={r['id']:r for r in stdout if'id'in r}
            if len(calls)!=8:raise ValueError('actual stdin call denominator differs')
            for call,row in zip(calls,actual):
                if call['params']!=dict(name='cks.context.label_suspects',arguments=row['arguments'])or replies.get(call['id'])!=row['raw_response']:raise ValueError('actual wire request/response differs')

        if len(sessions)!=2 or any(r['EOF_exit']!=0 or r['attempted']!=8 or r['requests']!=10 for r in sessions):raise ValueError('runtime session/EOF count differs')
        for end in ('before','after'):
            if f.read(q/('model-'+end+'.json'))['model']['digest']!='7907646426070047a77226ac3e684fbbe8410524f7b4a74d02837e43f2146bab':raise ValueError('local model identity differs')
        result[phase]={(r['measurement'],r['case_id']):r for r in rows}
    initial=p/'initial-runtime';initialrows=packed(initial/'responses.json.gz')
    initial_sessions=f.read(initial/'sessions.json');initial_plan=f.read(initial/'planned.json');initial_receipt=f.read(initial/'receipt.json')
    if len(initialrows)!=14 or len(initial_sessions)!=2 or [s['measurement']for s in initial_sessions]!=['off','on']or any(s['EOF_exit']!=0 or s['attempted']!=7 or s['requests']!=9 for s in initial_sessions):raise ValueError('initial comment-line attempt omitted')
    if len(initial_plan)!=7 or [(r['measurement'],r['case_id'],r['arguments'])for r in initialrows]!=[(label,r['case_id'],r['arguments'])for label in ('off','on')for r in initial_plan]:raise ValueError('initial planned denominator differs')
    if initial_receipt['planned']!=14 or initial_receipt['attempted']!=14 or initial_receipt['retries']!=0 or initial_receipt['binary_sha256']!=f.read(root/PRIOR_NATIVE/'native/binary.json')['sha256']:raise ValueError('initial binary/attempt scope differs')
    for label in ('off','on'):
        stdin=events(gzip.decompress((initial/label/'mcp.stdin.jsonl.gz').read_bytes()));stdout=events(gzip.decompress((initial/label/'mcp.stdout.jsonl.gz').read_bytes()));replies={r['id']:r for r in stdout if'id'in r}
        calls=[r for r in stdin if r['method']=='tools/call'];actual=[r for r in initialrows if r['measurement']==label]
        if len(calls)!=7:raise ValueError('initial actual call denominator differs')
        for call,row in zip(calls,actual):
            if call['params']!=dict(name='cks.context.label_suspects',arguments=row['arguments'])or replies.get(call['id'])!=row['raw_response']:raise ValueError('initial actual wire differs')
    if packed(initial/'before.json.gz')!=inventory or packed(initial/'after.json.gz')!=inventory:raise ValueError('initial/baseline data changed')
    initialoff=next(r for r in initialrows if r['measurement']=='off'and r['case_id']=='site')
    if not unpack(initialoff)[1]or initialoff['arguments']['line']!=257:raise ValueError('initial wrong site premise erased')
    before=result['before'];after=result['after'];comparisons=[]
    for cid in ('site','symbol','expand'):
        off,off_error=unpack(before['off',cid]);on,on_error=unpack(before['on',cid])
        if off_error or off is None:raise ValueError('baseline unmeasured positive unavailable')
        if cid=='site':
            if not on_error:raise ValueError('baseline measured site regression not reproduced')
        elif on_error or not on['candidates']or any(c['symbol']is not None for c in on['candidates']):raise ValueError('baseline measured identity loss not reproduced')
        newoff,e1=unpack(after['off',cid]);newon,e2=unpack(after['on',cid])
        if e1 or e2 or normalized(newoff)!=normalized(newon)or normalized(off)!=normalized(newoff):raise ValueError('actual measurement parity/off behavior changed')
        if not newon['candidates']or not all(c['symbol']for c in newon['candidates']):raise ValueError('post-fix identity missing')
        comparisons.append(dict(case_id=cid,before_off_candidates=len(off['candidates']),before_on_tool_error=on_error,
            before_on_identified=0 if on is None else sum(c['symbol']is not None for c in on['candidates']),after_candidates=len(newon['candidates']),
            measured_unmeasured_equal_except_elapsed=True,unmeasured_before_after_equal_except_elapsed=True,quality_score=None))
    for cid in ('site_comment','wrong_commit','foreign_dataset','forged_hash','unknown_site'):
        for label in ('off','on'):
            value,error=unpack(after[label,cid])
            if not error or value is not None:raise ValueError('invalid/unresolved input acquired a result')
        if cid in ('site_comment','unknown_site'):
            raw=json.dumps(after['on',cid]['raw_response'])
            if 'site_resolver_unavailable'in raw or'seed_unresolved'not in raw:raise ValueError('supported resolver failure conflated with absent capability')
    return comparisons


def derive(root=ROOT,packet=None):
    p=packet or root/PACKET;comparisons=runtime_data(root,p);native=f.read(p/'native/receipts.json')
    beforetests=[]
    for label in ('unit','handler'):
        data=events(gzip.decompress((p/'targeted'/('before-'+label+'.jsonl.gz')).read_bytes()))
        if not any(x['Action']=='fail'for x in data if x.get('Test')):raise ValueError('before-fix regressions missing')
        beforetests.extend(x['Test']for x in data if x.get('Test')and x['Action']=='fail')
    targeted=events(gzip.decompress((p/'targeted/after-targeted.jsonl.gz').read_bytes()))
    if any(x['Action']=='fail'for x in targeted):raise ValueError('post-fix targeted failures')
    return dict(comparisons=comparisons,summary=dict(kind=KIND,changed_native_files=CHANGED,native_inputs=1738,approved_inputs=5,
        fresh_native_test=next(r['tests']for r in native if r['label']=='test'),fresh_native_race=next(r['tests']for r in native if r['label']=='race'),
        targeted_pass=sum(x['Action']=='pass'for x in targeted if x.get('Test')),targeted_skip=sum(x['Action']=='skip'for x in targeted if x.get('Test')),
        reproduced_failed_test_records=len(beforetests),initial_runtime_queries=14,baseline_runtime_queries=16,postfix_runtime_queries=16,actual_queries_total=46,EOF_exit0_sessions=6,
        source_index_preserved=True,actual_measured_parity_cases=3,invalid_or_unresolved_cases_each_mode=5,auto_label_measurement_scope_added=False,
        unanchored_traversal_changed=False,installed_agent_role_runs=0,paid_or_external_LLM_calls=0,new_independent_samples=0,quality_score=None,
        independent_approved=0,FINAL=0,formal_completed=0,formal_total=9,freeze=None,execution_allowed=False))


def historical_errors(root=ROOT):
    errors=previous.historical_errors(root);p=root/previous.PACKET;m=f.read(p/'manifest.json')
    if (p/'manifest.json').read_bytes()!=subprocess.check_output(['git','show',REVISION+':'+str(previous.PACKET/'manifest.json')],cwd=root):errors.append('prior reference manifest changed')
    for name,h in m['artifacts'].items():
        if f.sha((p/name).read_bytes())!=h:errors.append('prior reference artifact changed '+name)
    for r in m['tools']:
        if f.sha(subprocess.check_output(['git','show',REVISION+':'+r['path']],cwd=root))!=r['sha256']:errors.append('prior reference tool changed '+r['path'])
    return errors


def reference(root=ROOT):return dict(manifest=str(PACKET/'manifest.json'),sha256=f.sha((root/PACKET/'manifest.json').read_bytes()),status='WI3_optional_resolver_fresh_native_actual_stdio_not_independent_acceptance',report=str(PACKET/'REPORT.txt'),worklist=str(PACKET/'WORKLIST.json'))


def current_indices(root=ROOT,protocol_override=None,ledger_override=None):
    paths=[Path('docs/spec-driven/OPERATIONAL-ACCEPTANCE-'+n+'.json')for n in ('PROTOCOL','PROGRESS','DECISION-PROPOSAL')]+[BASE/'GAPS.json'];docs=[f.read(root/x)for x in paths]
    if protocol_override is not None:docs[0]=protocol_override
    if ledger_override is not None:docs[1]=ledger_override
    ref=reference(root);state=f.read(root/PACKET/'WORKLIST.json');current=dict(kind=KIND,manifest=ref['manifest'],manifest_sha256=ref['sha256']);errors=[]
    for d,path in zip(docs,paths):
        prior=json.loads(subprocess.check_output(['git','show',REVISION+':'+str(path)],cwd=root))
        if d.get('wi3_resolver_fix')!=ref or d.get('current_execution_state')!=state or d.get('current_readiness_preparation')!=current:errors.append('WI3 resolver current reference/state differs')
        if d.get('reviewer_identity_policy')!=previous.previous.contract()['privacy']:errors.append('reviewer privacy policy changed')
        for key,value in prior.items():
            if key=='current_readiness_preparation'or not isinstance(value,dict)or'manifest'not in value:continue
            expected=dict(value)
            if key=='wi1_select_review':expected['historical_source_revision']=REVISION
            if d.get(key)!=expected:errors.append('previous preparation changed '+key)
    if docs[0]['freeze']is not None or docs[0]['execution_allowed']is not False or any(docs[0]['roles'].values()):errors.append('native patch cannot approve/freeze/assign roles')
    link=docs[0]['adoption_acceptance_decision_proposal']
    if link['sha256']!=f.sha((root/paths[2]).read_bytes())or any(d.get('adoption_acceptance_decision_proposal')!=link for d in (docs[1],docs[3])):errors.append('proposal link differs')
    return errors


def validate(root=ROOT,packet=None):
    p=packet or root/PACKET;errors=[]
    try:
        m=f.read(p/'manifest.json');actual={x.relative_to(p).as_posix():f.sha(x.read_bytes())for x in p.rglob('*')if x.is_file()and x!=p/'manifest.json'}
        if m['kind']!=KIND or m['previous_source_revision']!=REVISION or m['artifacts']!=actual or [r['path']for r in m['tools']]!=TOOLS:errors.append('resolver packet scope/hash differs')
        for r in m['tools']:
            if f.sha((root/r['path']).read_bytes())!=r['sha256']:errors.append('resolver tool changed '+r['path'])
        errors.extend(historical_errors(root));errors.extend(native_errors(root,p));d=derive(root,p)
        if f.read(p/'SUMMARY.json')!=d['summary']or f.read(p/'runtime-comparison.json')!=d['comparisons']:errors.append('derived resolver evidence differs')
        state=f.read(p/'WORKLIST.json')
        if state['formal_completed']!=0 or state['formal_total']!=9 or state['reviewer_assignment']!='미정'or state['production_activation']is not False or state['publication']is not False or state['independent_quality_or_ontology_acceptance']is not False:errors.append('worklist falsely completed')
    except (ValueError,KeyError,TypeError,OSError,IndexError)as exc:errors.append(str(exc))
    return errors
