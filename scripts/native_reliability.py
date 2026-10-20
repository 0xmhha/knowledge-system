"""Current native source and real consumer evidence; never independent acceptance."""
import gzip
import json
import subprocess
from collections import Counter
from pathlib import Path

import wi1_consumer_matching as matching
import wi6_pair_results as prior
import wi6_version_readiness as history

ROOT = matching.ROOT
PACKET = matching.BASE/'followups/local-reliability-v1'
REVISION = '6a2272c9be7959ac64c20d40acea5becfb2043ce'
KIND = 'local_native_reliability_DEV_not_independent_acceptance'
TOOLS = ['scripts/'+n for n in ('native_reliability.py', 'test_native_reliability.py',
    'run-local-reliability.py', 'check-operational-acceptance.py',
    'verify-wi3-reasons-native.py', 'verify-wi3-resolver-native.py')]
PATHS = [Path('docs/spec-driven/OPERATIONAL-ACCEPTANCE-'+n+'.json')
         for n in ('PROTOCOL', 'PROGRESS', 'DECISION-PROPOSAL')]+[matching.BASE/'GAPS.json']
read, write, compressed = prior.read, prior.write, prior.compressed
sha = prior.c.sha


def runtime_scope(binary_sha):
    warm = read(ROOT/matching.BASE/'followups/wi6-warm-runtime-v1/SCOPE.json')
    pins = warm['cases'][0]['pins']
    plan=[dict(phase=phase,mode=mode,tool='cks.ops.health'if health else'cks.context.semantic_search',expire_health_TTL=expire)
        for phase,mode,health,expire in [
        ('inference_down_health','deny_embed',True,True),
        ('inference_down_cached_query','deny_embed',False,False),
        ('inference_down_health_after_cached_query','deny_embed',True,False),
        ('inference_recovered_health','normal',True,True),
        ('inference_recovered_cached_query','normal',False,False),
        ('identity_down_cached_query','deny_tags',False,False),
        ('identity_down_health','deny_tags',True,False),
        ('identity_recovered_health','normal',True,True),
        ('identity_recovered_cached_query','normal',False,False)]]
    return dict(kind='WI1_actual30_and_local_BGE_cache_off_on_DEV', binary_sha256=binary_sha,
                run='explicit_model_boundary_corrective1_after_owned_guard_failure',
                previous_attempt='model-boundary-initial-attempt',
                post_cached_query_health_probe_HTTP_calls_allowed=[0,1],
                TTL_note='graph health work can expire the five-second model-health TTL; state retention at cached success is separately unit-tested',
                pins={k:pins[k] for k in ('project_id','dataset_id','commit','snapshot_sha256')},
                limits=warm['cases'][0]['requests']['baseline']['limits'],
                semantic_search_arguments=dict(query='validator quorum finalize phase73 cache probe',k=3,language='go',exclude_tests=True,expand=False),
                sessions=['off','on'],cache_entries=[0,32],WI1_cases=30,semantic_searches=8,
                model_boundary_plan=plan,planned_tools=46,automatic_retries=0,formal_completed=0,independent_approved=0,
                tools_sha256={n:sha((ROOT/n).read_bytes()) for n in ('scripts/run-local-reliability.py','scripts/run-wi6-pair-pilot.py','scripts/wi6_pair_contract.py')})


def events(raw):
    return [json.loads(line) for line in raw.splitlines() if line.strip()]


def native(root, p):
    binding = read(p/'source-binding.json')
    old = read(root/matching.BASE/'followups/wi3-walk-budget-v1/source-binding.json')
    if binding['base_revision'] != REVISION or binding['approved_input_sha256'] != old['approved_input_sha256']:
        raise ValueError('native baseline/approved input changed')
    changed = sorted(n for n, h in binding['source_sha256'].items() if old['source_sha256'].get(n) != h)
    if changed != binding['changed_native_files'] or len(changed) != 20:
        raise ValueError('native corrective scope changed')
    live = subprocess.check_output(['git', 'ls-files', '--cached', '--others', '--exclude-standard', '*.go'], cwd=root, text=True).splitlines()
    if {n for n in live if n.startswith(('cmd/', 'pkg/', 'internal/'))} != {n for n in binding['source_sha256'] if n.endswith('.go')}:
        raise ValueError('native Go file population differs')
    for name, digest in {**binding['source_sha256'], **binding['approved_input_sha256']}.items():
        if sha((root/name).read_bytes()) != digest:
            raise ValueError('current native source/approved input changed: '+name)
    changes=read(p/'code-changes.json')
    if [r['path']for r in changes]!=changed:
        raise ValueError('corrective source archive population changed')
    for change in changes:
        previous=subprocess.run(['git','show',REVISION+':'+change['path']],cwd=root,capture_output=True)
        if change['current_sha256']!=binding['source_sha256'][change['path']] or change['previous_sha256']!=(sha(previous.stdout)if previous.returncode==0 else None) or sha(gzip.decompress((p/change['artifact']).read_bytes()))!=change['current_sha256']:
            raise ValueError('corrective source archive differs from native/Git inputs')
    receipts = read(p/'native/receipts.json')
    labels = ['build', 'test', 'race', 'lint', 'build-bins', 'build-runtime', 'build-info']
    if [r['label'] for r in receipts] != labels:
        raise ValueError('whole native command population incomplete')
    counts = {}
    for r in receipts:
        if r['exit_code'] != 0 or r['timeout'] or r['term_sent'] or r['kill_sent'] or not r['reaped'] or r['source_head'] != REVISION or r['source_binding_sha256'] != sha((p/'source-binding.json').read_bytes()):
            raise ValueError('native outcome/source binding differs')
        raw = {}
        for stream in ('stdout', 'stderr'):
            raw[stream] = gzip.decompress((p/'native'/(r['label']+'.'+stream+'.gz')).read_bytes())
            if sha(raw[stream]) != r[stream+'_sha256']:
                raise ValueError('native raw log differs')
        if r['label'] in ('test', 'race'):
            expected = ['go', 'test']+(['-race'] if r['label']=='race' else [])+['-json', '-count=1', '-timeout='+('20m' if r['label']=='race' else '18m'), './...']
            if r['argv'] != expected:
                raise ValueError('whole native test command narrowed')
            rows = events(raw['stdout'])
            tests = Counter(r['Action'] for r in rows if r.get('Test') and r['Action'] in ('pass','fail','skip'))
            packages = Counter(r['Action'] for r in rows if not r.get('Test') and r['Action'] in ('pass','fail','skip'))
            if tests['fail'] or packages['fail'] or tests['pass'] < 4172 or tests['skip'] != 18 or any(r['tests'][k] != tests[k] for k in ('pass','fail','skip')):
                raise ValueError('native pass/skip/failure denominator differs')
            previous = events(gzip.decompress((root/matching.BASE/'followups/wi3-walk-budget-v1/native'/(r['label']+'.stdout.gz')).read_bytes()))
            ids = lambda data, action: {(v['Package'],v['Test']) for v in data if v.get('Test') and v['Action']==action}
            if not ids(previous,'pass') <= ids(rows,'pass') or ids(previous,'skip') != ids(rows,'skip'):
                raise ValueError('prior passing/skip tests omitted')
            counts[r['label']] = {key:tests[key]for key in ('pass','fail','skip')}
        if r['label'] == 'build' and r['argv'] != ['go','build','./...']:
            raise ValueError('whole build narrowed')
        if r['label'] == 'lint':
            if r['argv'][:2] != ['make','lint'] or not all(x in raw['stdout'].decode() for x in ('0 issues.', 'go vet ./...', 'documentation: OK (134 live documents)')):
                raise ValueError('full fmt/vet/pinned lint/docs evidence missing')
    config = read(p/'native/lint-config-binding.json')
    if set(config) != {'.golangci.yml', 'docs/DOCUMENTATION-CATALOG.json'} or any(sha((root/n).read_bytes()) != h or sha(history.read_at(root, REVISION, n)) != h for n,h in config.items()):
        raise ValueError('lint configuration changed')
    versions=read(p/'native/installed-tools.json')
    if not versions['go'].startswith('go version go1.26.8 ') or 'version 2.14.0 ' not in versions['lint'] or versions['capture_before_lint'] is not True:
        raise ValueError('installed Go/linter proof differs')
    groups = read(p/'native/process-groups.json')
    if [(g['label'],g['pgid'])for g in groups]!=[(r['label'],r['pid'])for r in receipts] or any(r['remaining_live_group_members'] != 0 for r in groups):
        raise ValueError('native owned process group not cleaned')
    binary = read(p/'native/binary.json')
    if binary['source_head'] != REVISION or binary['source_mode'] != binding['source_mode'] or binary['native_source_binding_sha256'] != sha((p/'source-binding.json').read_bytes()):
        raise ValueError('runtime binary not from the fully validated source')
    runtime_binding=read(p/'runtime-source-binding.json')
    candidate=read(p/'candidate-binary.json')
    if sha((p/'runtime-source-binding.json').read_bytes())!=candidate['source_binding_sha256']:
        raise ValueError('actual runtime source binding changed')
    delta=[name for name,h in binding['source_sha256'].items()if runtime_binding['source_sha256'].get(name)!=h]
    if delta!=['internal/system/maintenance/source_check_test.go']:
        raise ValueError('undeclared runtime-to-final native source delta')
    return dict(source_files=len(binding['source_sha256']), approved_inputs=5, changes=20,
                fresh_general=counts['test'], fresh_race=counts['race'],
                full_build_fmt_vet_pinned_lint=True, process_groups_remaining=0,
                runtime_to_final_source_delta=delta,final_binary_must_equal_observed_runtime=True,
                binary_sha256=binary['sha256'])


def normalize_search(value):
    if isinstance(value, list):
        return [normalize_search(r) for r in value]
    if isinstance(value, dict):
        return {k: normalize_search(v) for k,v in value.items() if k not in ('elapsed_micros','elapsed_ms','latency_ms')}
    return value


def runtime(root, p, binary_sha):
    q = p/'actual'
    scope = read(p/'RUNTIME-SCOPE.json')
    if scope != runtime_scope(binary_sha) or read(q/'SCOPE.json.gz') != scope:
        raise ValueError('runtime scope binary differs')
    expected = read(root/matching.BASE/'followups/wi6-warm-runtime-v1/SCOPE.json')['original_data_sha256']
    for phase in ('original-before','original-after','owned-before','owned-after'):
        if read(q/(phase+'.json.gz')) != expected:
            raise ValueError('original/owned data changed')
    calls = read(q/'calls.json.gz')
    receipt = read(q/'receipt.json.gz')
    if receipt != dict(binary_sha256=binary_sha, sessions=2, tool_calls=46, WI1_cases=30,
                       semantic_searches=8, controlled_model_boundary_calls=9, automatic_retries=0, original_owned_payload_preserved=True,
                       proxy_thread_joined=True, external_or_paid_LLM_calls=0,
                       formal_completed=0, independent_approved=0, quality_score=None):
        raise ValueError('runtime denominator or scope promoted')
    driver=read(p/'driver/receipt.json.gz')
    if driver['exit_code']!=0 or driver['timeout'] or not driver['reaped'] or driver['remaining_live_group_members']:
        raise ValueError('actual runtime driver did not cleanly terminate')
    for stream in ('stdout','stderr'):
        if sha(gzip.decompress((p/'driver'/(stream+'.gz')).read_bytes()))!=driver[stream+'_sha256']:
            raise ValueError('actual runtime driver stream changed')
    for arm, count in [('off',5),('on',45)]:
        session = read(q/arm/'session.json.gz')
        if session['exit_code'] != 0 or session['requests'] != count or not session['reaped'] or not session['reader_joined'] or session['term_sent'] or session['kill_sent'] or session['remaining_live_group_members']:
            raise ValueError('actual consumer did not terminate cleanly')
        inputs, outputs = [events(gzip.decompress((q/arm/('std'+s+'.jsonl.gz')).read_bytes())) for s in ('in','out')]
        if [r['id'] for r in inputs] != list(range(1,count+1)) or [r['id'] for r in outputs] != list(range(1,count+1)):
            raise ValueError('actual consumer raw wire denominator differs')
        wire = {r['id']:r for r in outputs}
        rows = [r for r in calls if r['arm']==arm]
        if [r for r in inputs if r['method']=='tools/call'] != [r['request'] for r in rows] or any(wire[r['request']['id']] != r['raw_response'] for r in rows):
            raise ValueError('consumer receipt not actual wire')
    searches = read(q/'search-counts.json.gz')
    if [(r['arm'],r['repeat'],r['embedding_HTTP_calls']) for r in searches] != [('off',0,1),('off',1,1),('on',0,1),('on',1,0)]:
        raise ValueError('actual repeated embedding-cache effect not established')
    search_rows = [r for r in calls if r['phase'].startswith('semantic_search_')]
    values = [prior.c.unpack(r['raw_response'])[0] for r in search_rows]
    if any(not v for v in values) or any(normalize_search(v) != normalize_search(values[0]) for v in values[1:]):
        raise ValueError('off/on search response/source changed')
    health = [prior.c.unpack(r['raw_response'])[0] for r in calls if r['phase']=='health']
    if any(h is None for h in health) or health[1]['backends']['vector'].get('embedding_cache_entries') != 32 or health[0]['backends']['vector'].get('embedding_cache_entries',0) != 0:
        raise ValueError('cache option not exposed through actual health')
    boundaries=read(q/'controlled-model-boundaries.json.gz')
    expected_http=[{1},{0},{0,1},{1},{0},{0},{0},{1},{0}]
    if [(r['phase'],r['mode'])for r in boundaries]!=[(r['phase'],r['mode'])for r in scope['model_boundary_plan']] or any(r['embedding_HTTP_calls']not in allowed for r,allowed in zip(boundaries,expected_http)):
        raise ValueError('model boundary requests/probe effect differs')
    actual_boundaries={r['phase']:r for r in calls if r['phase']in{e['phase']for e in scope['model_boundary_plan']}}
    for row in boundaries:
        response=actual_boundaries[row['phase']]
        if response['request']['id']!=row['request_id']:raise ValueError('boundary not tied to actual wire')
        value,encoding=prior.c.unpack(response['raw_response'])
        if row['phase']=='identity_down_cached_query':
            if encoding!='tool_error':raise ValueError('cached query did not reject model identity unavailability')
        elif response['request']['params']['name']=='cks.context.semantic_search':
            if normalize_search(value)!=normalize_search(values[0]):raise ValueError('cached/recovered query source changed')
        else:
            expected_up='recovered'in row['phase']
            if value is None or bool(value['backends']['vector'].get('model_reachable',False))!=expected_up:
                raise ValueError('cached search masked inference failure or prevented probe recovery')
    observed_rows = matching.old.packed(root/matching.BASE/'followups/wi1-select-review-v1/observations.json.gz')
    observed = {(r['path'],r['span']['start']):r for r in observed_rows}
    candidates = matching.old.read(root/matching.old.FUNCTIONS)['functions']
    rows = [r for r in calls if r['phase'].startswith('WI1_')]
    selected = read(q/'selected.json.gz')
    if [r['case_id'] for r in rows] != [r['candidate_id'] for r in candidates]:
        raise ValueError('original WI1 thirty cases changed')
    comparisons=[]
    if len(observed)!=30 or len(selected)!=30:
        raise ValueError('source reference/selected seed population differs')
    for case, row, seed in zip(candidates, rows, selected):
        observation=observed[(case['source']['path'],case['source']['start_byte'])]
        value, _ = prior.c.unpack(row['raw_response'])
        analysis = (value or {}).get('analysis')
        if not analysis or analysis['truncated'] or analysis['source']['commit'] != scope['pins']['commit'] or row['request']['params']['arguments']['limits'] != scope['limits']:
            raise ValueError('actual WI1 analysis missing/truncated or limits changed')
        args=row['request']['params']['arguments']
        if seed['case_id'] != case['candidate_id'] or args != dict(node_id=seed['node']['node_id'],expected_commit=scope['pins']['commit'],expected_source_sha256=case['source']['function_sha256'],expected_snapshot_sha256=scope['pins']['snapshot_sha256'],limits=scope['limits']) or analysis['source']['node_id']!=seed['node']['node_id'] or value['snapshot_sha256']!=scope['pins']['snapshot_sha256']:
            raise ValueError('actual WI1 source/snapshot/node pin differs')
        mapped = matching.match(observation, analysis)
        comparisons.append(dict(case_id=case['candidate_id'], matching=mapped, status=value['status']))
    if sum(r['matching']['unmatched_occurrence_phases'] for r in comparisons) != 37 or sum(r['matching']['unmatched_blocks'] for r in comparisons) != 3:
        raise ValueError('selected receive correction or residual uncertainty differs')
    return dict(actual_WI1_cases=30, selected_receive_owner_corrections=16,
                remaining_range_unknown_definitions=37, ambiguous_unreachable_empty_blocks=3,
                actual_semantic_searches=8, cache_off_query_embedding_HTTP_calls=2,
                cache_on_query_embedding_HTTP_calls=1, repeat_on_embedding_HTTP_calls=0,
                search_output_and_source_preserved=True, health_capacity_on=32,
                actual_model_boundary_calls=9,identity_failure_cached_query_rejected=True,
                model_down_after_cached_query_observed=True,
                post_cached_query_health_probe_HTTP_calls=boundaries[2]['embedding_HTTP_calls'],
                exact_health_TTL_state_at_cached_success='native_regression_only_not_inferred_from_later_MCP_health',fresh_probe_recovery_verified=True,
                raw_tool_calls=46, protocol_requests=50, sessions=2,
                owned_process_groups_remaining=0, original_owned_payload_preserved=True,
                quality_score=None), comparisons


def derive(root=ROOT, packet=None):
    p = packet or root/PACKET
    historic = history.snapshot(root, matching.BASE/'followups/wi3-walk-budget-v1', 'ad944ac6cc15a2c4e2ebd2100fb1807bef911f28')
    n = native(root,p)
    r, comparisons = runtime(root,p,n['binary_sha256'])
    for name in ('superseded-native-attempt','superseded-model-contract-attempt'):
        interrupted=read(p/name/'INTERRUPTION.json')
        interrupted_receipts=read(p/name/'receipts.json.gz')
        if interrupted['signal']!='SIGTERM' or interrupted['source_not_changed_before_interrupt']is not True or interrupted['not_runtime_server_failure']is not True or interrupted_receipts[-1]['exit_code']!=-15:
            raise ValueError('superseded validation attempt lost or misclassified')
        if any(row['remaining_live_group_members']!=0 for row in read(p/name/'CLEANUP.json')):
            raise ValueError('superseded owned process group cleanup not established')
    archived=[]
    for name,expected_exit,tools,requests in [('intermediate-query-aware-candidate',0,37,[5,36]),('model-boundary-initial-attempt',1,46,[5,45])]:
        q=p/name
        driver=read(q/'driver/receipt.json.gz')
        if driver['exit_code']!=expected_exit or driver['timeout'] or not driver['reaped'] or driver['remaining_live_group_members']:
            raise ValueError('earlier runtime driver lost or promoted')
        if len(read(q/'actual/calls.json.gz'))!=tools:
            raise ValueError('earlier runtime call denominator changed')
        for arm,count in zip(('off','on'),requests):
            session=read(q/'actual'/arm/'session.json.gz')
            if session['requests']!=count or session['exit_code']!=0 or session['term_sent'] or session['kill_sent'] or not session['reaped'] or session['remaining_live_group_members']:
                raise ValueError('earlier runtime session cleanup changed')
        for which in ('original','owned'):
            if read(q/'actual'/(which+'-before.json.gz'))!=read(q/'actual'/(which+'-after.json.gz')):
                raise ValueError('earlier runtime payload was modified')
        archived.append(dict(scope=name,driver_exit=expected_exit,tool_calls=tools,protocol_requests=sum(requests),server_exits=[0,0]))
    failed=read(p/'native-race-failed-attempt/receipts.json.gz')
    if [row['label']for row in failed]!=['build','test','race']or failed[-1]['exit_code']!=1 or failed[-1]['tests']!={'pass':4171,'fail':1,'skip':18}:
        raise ValueError('earlier full-race failure was omitted or promoted')
    failure_rows=events(gzip.decompress((p/'native-race-failed-attempt/race.stdout.gz').read_bytes()))
    if [(row['Package'],row['Test'])for row in failure_rows if row.get('Test')and row['Action']=='fail']!=[('github.com/0xmhha/knowledge-system/internal/system/maintenance','TestSourceCheckHasDeadlineAndStopsLateDescendant')]:
        raise ValueError('earlier deadline failure identity differs')
    for name,count in [('phase73-deadline-reproduce.jsonl',20),('phase73-deadline-trace.jsonl',100)]:
        rows=events(gzip.decompress((p/'native-race-failed-attempt/diagnostics'/(name+'.gz')).read_bytes()))
        observed=Counter(row['Action']for row in rows if row.get('Test')and row['Action']in('pass','fail','skip'))
        if observed['pass']!=count or observed['fail']or observed['skip']:
            raise ValueError('earlier deadline diagnostic repeat population changed')
    return dict(summary=dict(kind=KIND, native=n, runtime=r,
                intentional_superseded_native_attempts=2, superseded_native_test_exits=[-15,-15],
                earlier_runtime_attempts=archived,explicit_model_boundary_corrective_attempts=1,
                all_runtime_tool_calls=129,all_runtime_protocol_requests=141,all_runtime_server_sessions=6,
                earlier_full_race_assertion_failures=1,
                original_deadline_failure_root_cause='not_confirmed_after20_original_and100_traced_repetitions',
                deadline_test_now_requires_entered_descendant_and_post_return_release_gate=True,
                production_guardian_implementation_changed=False,
                previous_evidence_preserved=historic['historical_bytes_verified'],
                independent_approved=0, formal_completed=0, formal_total=9, FINAL=0,
                D04='미정', freeze=None, execution_allowed=False, quality_score=None,
                personal_names_required=False), comparisons=comparisons)


def reference(root=ROOT):
    return dict(kind=KIND, manifest=str(PACKET/'manifest.json'),
                manifest_sha256=sha((root/PACKET/'manifest.json').read_bytes()))


def current_indices(root, protocol, ledger):
    errors=[]
    docs=[read(root/n) for n in PATHS]
    docs[0],docs[1]=protocol,ledger
    for path,data in zip(PATHS,docs):
        previous=json.loads(history.read_at(root,REVISION,path))
        if data.get('current_readiness_preparation')!=reference(root) or data.get('current_execution_state')!=read(root/PACKET/'WORKLIST.json'):
            errors.append('current local native readiness/state differs')
        for k,v in previous.items():
            if k not in ('current_readiness_preparation','current_execution_state','adoption_acceptance_decision_proposal') and data.get(k)!=v:
                errors.append('previous acceptance decision/role/gold changed: '+k)
    proposal_sha=sha((root/PATHS[2]).read_bytes())
    expected_link=dict(path=str(PATHS[2]),sha256=proposal_sha,status='D04_review_pending_local_native_DEV_not_independent_acceptance')
    if any(d.get('adoption_acceptance_decision_proposal')!=expected_link for d in (docs[0],docs[1],docs[3])):
        errors.append('current proposal hash/reference differs')
    return errors


def seal(p):
    write(p/'manifest.json', dict(kind=KIND, previous_revision=REVISION,
        tools=[dict(path=n,sha256=sha((ROOT/n).read_bytes())) for n in TOOLS],
        artifacts={str(x.relative_to(p)):sha(x.read_bytes()) for x in sorted(p.rglob('*')) if x.is_file() and x!=p/'manifest.json'}))


def validate(root=ROOT, packet=None):
    errors=[]
    p=packet or root/PACKET
    try:
        m=read(p/'manifest.json')
        if m!=dict(kind=KIND,previous_revision=REVISION,tools=[dict(path=n,sha256=sha((root/n).read_bytes())) for n in TOOLS],artifacts={str(x.relative_to(p)):sha(x.read_bytes()) for x in p.rglob('*') if x.is_file() and x!=p/'manifest.json'}):
            raise ValueError('native reliability artifacts/tools changed')
        data=derive(root,p)
        if read(p/'SUMMARY.json')!=data['summary'] or read(p/'comparisons.json')!=data['comparisons']:
            raise ValueError('native/consumer replay changed or falsely promoted')
    except (OSError,ValueError,KeyError,TypeError,IndexError) as error:
        errors.append(str(error))
    return errors


def prepare(native_dir):
    p=ROOT/PACKET
    if (p/'native/receipts.json').exists():
        raise ValueError('refusing final native capture overwrite')
    if read(native_dir/'source-binding.json')!=read(p/'source-binding.json'):
        raise ValueError('final native inputs differ')
    if read(native_dir/'binary.json')['sha256']!=read(p/'candidate-binary.json')['sha256']:
        raise ValueError('observed runtime candidate differs from final validated binary')
    for name in ('receipts.json','binary.json'):
        write(p/'native'/name,read(native_dir/name))
    for x in native_dir.iterdir():
        if x.suffix in ('.stdout','.stderr'):
            compressed(p/'native'/(x.name+'.gz'),x.read_bytes())
    rows=[line.split() for line in subprocess.check_output(['ps','-axo','pid=,pgid=,stat='],text=True).splitlines()]
    groups=[]
    for receipt in read(native_dir/'receipts.json'):
        live=[r for r in rows if len(r)==3 and int(r[1])==receipt['pid'] and not r[2].startswith('Z')]
        groups.append(dict(label=receipt['label'],pgid=receipt['pid'],remaining_live_group_members=len(live)))
    write(p/'native/process-groups.json',groups)
    for attempt in ('superseded-native-attempt','superseded-model-contract-attempt'):
        interruption=read(p/attempt/'INTERRUPTION.json')
        cleanup=[]
        for name in ('owned_driver_pid','owned_pgid'):
            pgid=interruption[name]
            cleanup.append(dict(scope=name,pgid=pgid,remaining_live_group_members=sum(len(r)==3 and int(r[1])==pgid and not r[2].startswith('Z')for r in rows)))
        write(p/attempt/'CLEANUP.json',cleanup)
    binding=read(p/'source-binding.json');changes=[]
    for name in binding['changed_native_files']:
        blob=subprocess.run(['git','show',REVISION+':'+name],cwd=ROOT,capture_output=True)
        artifact='code/'+name+'.gz'
        compressed(p/artifact,(ROOT/name).read_bytes())
        changes.append(dict(path=name,previous_sha256=sha(blob.stdout)if blob.returncode==0 else None,
                            current_sha256=sha((ROOT/name).read_bytes()),artifact=artifact))
    write(p/'code-changes.json',changes)
    finalize(p)


def finalize(p):
    data=derive()
    write(p/'SUMMARY.json',data['summary']);write(p/'comparisons.json',data['comparisons'])
    state=read(ROOT/prior.PACKET/'WORKLIST.json')
    state.update(current_stage='73_selected_receive_actual30_and_opt_in_cache_native_verified_DEV',
                 completed_in_this_scope=['selected_receive_owner16_corrected_actual30_replayed',
                    'public_query_aware_defensive_LRU_CKV_CKS_opt_in_and_health',
                    'actual_local_BGE_cache_off2_on1_calls_same_search_source',
                    'fresh_whole_native_build_general_race_fmt_vet_lint'],
                 next_task='WI_V1_full_host_factory_sync_indexer_contract_then_remaining13',
                 current_native_reliability=data['summary'])
    state['remaining_all'][5]['task']='WI-1: actual30 definitions16 corrected; range_unknown37 and indistinguishable empty blocks3 remain explicit uncertainty; independent value/type/memory gold and installed-role acceptance remain'
    state['remaining_all'][-1]['task']='Optional active11: embedding cache locally implemented and actual model/error paths verified; remaining10 require declared surface/inputs/evidence; organization auth user-deferred'
    state['AI_DEV_draft_progress']['WI-1']='actual30_selected_receive16_corrected_range_unknown37_empty_block3; independent0'
    state['AI_DEV_draft_progress']['WI-6']='actual_warm627_pairs_baseline_partial570_expansion_budget570_complete0; independent0_quality_null'
    write(p/'WORKLIST.json',state)
    seal(p)


def update_indices():
    ref=reference();state=read(ROOT/PACKET/'WORKLIST.json');docs=[read(ROOT/n)for n in PATHS]
    for data in docs:
        data['current_readiness_preparation']=ref
        data['current_execution_state']=state
        data['local_native_reliability']=ref
    write(ROOT/PATHS[2],docs[2])
    link=dict(path=str(PATHS[2]),sha256=sha((ROOT/PATHS[2]).read_bytes()),status='D04_review_pending_local_native_DEV_not_independent_acceptance')
    for i in (0,1,3):
        docs[i]['adoption_acceptance_decision_proposal']=link
        write(ROOT/PATHS[i],docs[i])


def stored():
    revision=subprocess.check_output(['git','log','-1','--format=%H','--',str(PACKET/'manifest.json')],cwd=ROOT,text=True).strip()
    if not revision:
        raise ValueError('native reliability packet not committed')
    m=read(ROOT/PACKET/'manifest.json')
    binding=read(ROOT/PACKET/'source-binding.json')
    paths=[PACKET/'manifest.json',*[PACKET/n for n in m['artifacts']],*[Path(n)for n in TOOLS],*PATHS,
           *[Path(n)for n in binding['changed_native_files']],Path('docs/spec-driven/REMAINING-13-EXECUTION.json'),
           *[Path('docs/spec-driven')/n for n in ('CURRENT-HANDOFF.md','KNOWLEDGE-ROADMAP-TEST-MATRIX.md','OPERATIONAL-ACCEPTANCE-PROTOCOL.md','KNOWLEDGE-MANAGER-AND-COMPARISON-SPEC.md')],
           Path('docs/REMAINING-WORK.md'),Path('docs/vector/embedder-integration.md')]
    if any(subprocess.check_output(['git','show',revision+':'+str(path)],cwd=ROOT)!=(ROOT/path).read_bytes()for path in paths):
        raise ValueError('native evidence/tools/current indices differ from committed Git blobs')
    result=subprocess.run(['git','verify-commit',revision],cwd=ROOT,capture_output=True)
    if result.returncode:
        raise ValueError('native reliability commit signature not verified')
    return revision


if __name__=='__main__':
    import argparse
    parser=argparse.ArgumentParser();parser.add_argument('command',choices=['prepare','check','update-indices'])
    parser.add_argument('--native',type=Path);parser.add_argument('--require-formal',action='store_true');parser.add_argument('--require-stored',action='store_true')
    args=parser.parse_args()
    if args.command=='prepare':prepare(args.native.resolve())
    elif args.command=='update-indices':update_indices()
    else:
        errors=validate()
        if errors:raise SystemExit('\n'.join(errors))
        if args.require_stored:stored()
        if args.require_formal:raise SystemExit('formal independent acceptance unavailable: review/freeze unassigned')
        print(json.dumps(read(ROOT/PACKET/'SUMMARY.json'),ensure_ascii=False,indent=2))
