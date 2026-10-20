"""Source-bound startup batching evidence; development checks, never independent gold."""
import argparse
import gzip
import hashlib
import json
import math
import subprocess
from collections import Counter
from pathlib import Path

import native_reliability as prior

ROOT = prior.ROOT
PACKET = prior.PACKET.parent/'intent-startup-batch-v1'
REVISION = '7d5a5537c24a73a2a191d2f94b6243fb98c0d059'
KIND = 'WI3_source_bound_intent_batch_DEV_not_independent_acceptance'
TOOLS = ['scripts/'+s for s in ('intent_startup_batch.py', 'test_intent_startup_batch.py',
    'run-intent-startup-batch.py', 'run-intent-startup-retained.py', 'run-wi6-pair-pilot.py', 'wi6_pair_contract.py',
    'verify-wi3-reasons-native.py', 'check-operational-acceptance.py',
    'test_remaining_13.py', 'check-remaining-13.py', 'test_package_notice_recipe.py',
    'test_operational_acceptance.py', 'test_knowledge_roadmap_current.py',
    'test_knowledge_roadmap.py', 'check-knowledge-roadmap.py', 'check-docs.py',
    'documentation_contract.py', 'test_documentation_contract.py')]
PATHS = prior.PATHS
sha, write = prior.sha, prior.write

def raw(path):
    b = path.read_bytes()
    return gzip.decompress(b) if path.suffix == '.gz' else b

def read(path): return json.loads(raw(path))
def events(data): return [json.loads(line) for line in data.splitlines() if line.strip()]
def compress(path, data):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_bytes(gzip.compress(data, mtime=0))

def require(condition, reason):
    if not condition: raise ValueError(reason)

def normalized(value):
    if isinstance(value, dict): return {k:normalized(v) for k,v in value.items() if k != 'elapsed_micros'}
    if isinstance(value, list): return [normalized(v) for v in value]
    return value

def vectors(rows, arm):
    result = {}
    for row in rows:
        if row['arm'] != arm or row['path'] != '/api/embed': continue
        inputs, outputs = row['request']['input'], row['response'].get('embeddings', [])
        require(row['status'] == 200 and row['forwarded'] and len(inputs) == len(outputs), 'successful upstream vectors incomplete')
        for text, vector in zip(inputs, outputs):
            require(text not in result and len(vector) == 1024 and all(math.isfinite(x) for x in vector), 'duplicate text or invalid model vector')
            result[text] = vector
    return result

def parity(baseline, candidate):
    require(set(baseline) == set(candidate) and len(baseline) == 66, 'exact-text vector population differs')
    max_abs, min_cos = 0.0, 1.0
    for text, a in baseline.items():
        b = candidate[text]
        require(len(a) == len(b) == 1024, 'vector dimensions differ')
        max_abs = max(max_abs, max(abs(x-y) for x,y in zip(a,b)))
        norm = math.sqrt(sum(x*x for x in a)*sum(y*y for y in b))
        require(norm > 0, 'zero model vector')
        min_cos = min(min_cos, sum(x*y for x,y in zip(a,b))/norm)
    require(max_abs <= .001 and min_cos >= .9999, 'predeclared numeric parity failed')
    return dict(text_matched_vectors=66, dimension=1024, max_absolute_difference=max_abs,
                minimum_cosine=min_cos, quality_acceptance=False)

def inspect_runtime(p, packet, measured, known_erased_source=False):
    scope = read(p/'SCOPE.json.gz'); outcomes = read(p/'outcomes.json.gz'); http = read(p/'http-events.json.gz')
    require(scope['spec_sha256'] == sha((packet/'SPEC.json').read_bytes()), 'runtime specification changed')
    if measured:
        require(scope['measurement_addendum_sha256'] == sha((packet/'MEASUREMENT-WRAPPER-ADDENDUM.json').read_bytes()), 'measurement addendum changed')
    for field in ('automatic_retries','LLM_calls','paid_external_calls','formal_completed','independent_approved'):
        require(scope[field] == 0, 'runtime scope promoted')
    require(scope['source_commit']=='0bf2f4d1bfeb6605006d556957ef8c045d8f8ed8' and scope['project_id']=='p-3fa6368dd3f6757c6b6a86121f0a9b7d' and scope['dataset_id']=='7d5355a19e255a7733eddc7615c9eb4e419531babde63f184c09992c2e041ab6' and scope['snapshot_id']=='5dee8b6b4582bb591ea41105c6e540b1a7eaa9ef2b302fd13677e87c20791ad4', 'project/dataset/source pins changed')
    require(scope['CKS_process'] == 'fresh_per_arm' and scope['resource_exclusivity'] is False, 'startup scope changed')
    for field in ('original-before','original-after','owned-before','owned-after'):
        require(read(p/(field+'.json.gz')) == read(p/'original-before.json.gz'), 'original/owned data changed')
    require(len(read(p/'original-before.json.gz')) == 2248 and read(p/'original-before.json.gz')==read(ROOT/prior.matching.BASE/'followups/wi6-warm-runtime-v1/SCOPE.json')['original_data_sha256'], 'data population or pinned payload changed')
    require(not read(p/'proxy-cleanup.json.gz')['thread_alive'], 'owned proxy thread remains')
    receipt = read(p/'receipt.json.gz')
    require(receipt['planned_arms'] == receipt['attempted_arms'] == len(scope['arms']) == (6 if measured else 4), 'runtime arms omitted')
    require(receipt['original_owned_data_preserved'] is True and receipt['global_model_modified'] is False and receipt['real_LLM_calls'] == receipt['paid_external_calls'] == 0, 'runtime preservation or cost changed')
    require([x['arm'] for x in outcomes] == [x['id'] for x in scope['arms']], 'runtime outcome identities differ')
    texts = read(p/'anchor-texts.json.gz')
    require(len(texts) == len(set(texts)) == 65 and sum(len(x.encode()) for x in texts) == 2445, 'anchor inputs changed')
    node = read(p/'selected-source.json.gz')
    require(node['function_sha256'] == sha(raw(p/'selected-function.go.txt.gz')) and node['bytes'] == 347 and node['node_id'] == 'ae0cec70160b1579', 'actual source identity changed')
    baseline = vectors(http,'baseline')
    require(set(texts) < set(baseline), 'anchor texts missing from upstream')
    reference = normalized(prior.prior.c.unpack(read(p/'baseline/source-response.json.gz')['raw_response'])[0])
    require(reference['status']=='partial' and reference['analysis']['source']['sha256']==node['function_sha256'] and reference['analysis']['source']['commit']==scope['source_commit'], 'baseline retained source not identified')
    facts = []
    for arm, outcome in zip(scope['arms'], outcomes):
        name = arm['id']; q = p/name; session = read(q/'session.json.gz')
        success = arm['expected_exit'] == 0
        require(session['exit_code'] == outcome['session_exit'] == arm['expected_exit'], 'undeclared runtime exit')
        require(session['reaped'] and session['reader_joined'] and session['owned_new_session'] and not session['term_sent'] and not session['kill_sent'] and session['remaining_live_group_members'] == 0, 'owned CKS cleanup incomplete')
        require(outcome['initialized'] == outcome['source_response_recorded'] == success and (outcome['error'] is None) == success, 'startup outcome differs')
        inputs, outputs = events(raw(q/'stdin.jsonl.gz')), events(raw(q/'stdout.jsonl.gz'))
        require(len(inputs) == session['requests'] == (3 if success else 1), 'wire request denominator differs')
        require([r['id'] for r in outputs] == ([1,2,3] if success else []), 'wire responses omitted or invented')
        embedding = [r for r in http if r['arm']==name and r['path']=='/api/embed']
        require(len(embedding) == outcome['startup_embedding_HTTP'] == arm['expected_startup_embedding_HTTP'], 'startup HTTP count differs')
        require([len(r['request']['input']) for r in embedding] == ([1]*66 if name == 'baseline' else [1,65]), 'unexpected batching/retry input population')
        for row in [r for r in http if r['arm']==name and r['path']=='/api/tags']:
            require(row['status']==200 and row['bge_model_digests']==['7907646426070047a77226ac3e684fbbe8410524f7b4a74d02837e43f2146bab'], 'model identity changed')
        diagnostic = None
        if success:
            payload=normalized(prior.prior.c.unpack(read(q/'source-response.json.gz')['raw_response'])[0])
            if known_erased_source and name=='batch_measured':
                require(payload==dict(schema_version=1,status='unavailable',reason='backend does not support retained value/rule analysis',analysis=None), 'known intermediate unavailable response changed')
            else:
                require(payload == reference, 'actual source response changed beyond elapsed_micros')
            require(outputs[-1] == read(q/'source-response.json.gz')['raw_response'] and inputs[-1] == read(q/'source-response.json.gz')['request'], 'source response not actual stdout')
            diagnostic = parity(baseline, vectors(http,name))
        else:
            fault = embedding[-1]
            require(not fault['forwarded'] and fault['status'] == (503 if arm['mode']=='unavailable' else 200), 'owned provider fault missing')
            require(outcome['error']['type']=='RuntimeError' and outcome['error']['message']=='actual MCP stream ended', 'startup refusal not observed')
        if arm.get('measure'):
            logs = list((q/'footprints').glob('*.gz'))
            entries = [v['summary'] for f in logs for v in events(raw(f)) if v.get('event')=='measurement.backend_calls']
            startup = [r for r in entries if r['tool']=='startup.intent_anchors']
            require(len(startup)==1 and startup[0]['pending_calls']==0, 'startup measurement incomplete')
            calls = startup[0]['calls']; logical = [r for r in calls if r['backend']=='intent']
            require(len(logical)==1 and logical[0]['method']=='embed_batch' and logical[0]['result_count']==(65 if success else 0), 'measurement wrapper erased batching')
            require('input' not in logical[0] and len(logical[0]['input_sha256'])==64, 'measurement raw input or fingerprint changed')
            require(sum(r['backend']=='ollama_http' and r['method']=='POST' for r in calls)==1, 'logical and HTTP units conflated')
        facts.append(dict(arm=name, exit=session['exit_code'], embedding_HTTP=len(embedding),
                          initialized=success, source_response_preserved=success and not(known_erased_source and name=='batch_measured'), no_forced_cleanup=True, vector_diagnostic=diagnostic))
    return dict(sessions=len(facts), arms=facts, embedding_HTTP=sum(r['embedding_HTTP'] for r in facts),
                successful_sessions=sum(r['initialized'] for r in facts),
                expected_provider_startup_refusals=sum(not r['initialized'] for r in facts),
                actual_LLM_calls=0, independent_approved=0, formal_completed=0)

def native(root, p):
    binding = read(p/'source-binding.json'); previous = read(root/prior.PACKET/'source-binding.json')
    require(binding['base_revision']==REVISION and len(binding['changed_native_files'])==10 and len(binding['source_sha256'])==1746 and sum(n.endswith('.go')for n in binding['source_sha256'])==1290, 'native corrective scope changed')
    require(binding['approved_input_sha256']==previous['approved_input_sha256'], 'approved inputs changed')
    for name,digest in {**binding['source_sha256'],**binding['approved_input_sha256']}.items():
        require(sha((root/name).read_bytes())==digest, 'current source input changed: '+name)
    population = subprocess.check_output(['git','ls-files','--cached','--others','--exclude-standard','*.go'],cwd=root,text=True).splitlines()
    require({n for n in population if n.startswith(('cmd/','pkg/','internal/'))}=={n for n in binding['source_sha256'] if n.endswith('.go')}, 'native Go population changed')
    changes = read(p/'code-changes.json')
    require([r['path'] for r in changes]==binding['changed_native_files'], 'source archive population differs')
    for row in changes:
        before = subprocess.run(['git','show',REVISION+':'+row['path']],cwd=root,capture_output=True)
        require(row['previous_sha256']==(sha(before.stdout) if before.returncode==0 else None) and row['current_sha256']==binding['source_sha256'][row['path']]==sha(raw(p/row['artifact'])), 'source archive differs from Git/native input')
    receipts = read(p/'native/receipts.json'); labels=['build','test','race','lint','build-bins','build-runtime','build-info']; counts={}
    require([r['label'] for r in receipts]==labels, 'full native command population incomplete')
    for receipt in receipts:
        require(receipt['exit_code']==0 and not receipt['timeout'] and not receipt['term_sent'] and not receipt['kill_sent'] and receipt['reaped'] and receipt['source_head']==REVISION and receipt['source_binding_sha256']==sha((p/'source-binding.json').read_bytes()), 'native command failed or input changed')
        logs={stream:raw(p/'native'/(receipt['label']+'.'+stream+'.gz')) for stream in ('stdout','stderr')}
        require(all(sha(v)==receipt[k+'_sha256'] for k,v in logs.items()), 'native raw log changed')
        label=receipt['label']
        if label in ('test','race'):
            require(receipt['argv']==['go','test']+(['-race'] if label=='race' else [])+['-json','-count=1','-timeout='+('20m' if label=='race' else '18m'),'./...'], 'native tests narrowed')
            rows=events(logs['stdout']); count=Counter(r['Action'] for r in rows if r.get('Test') and r['Action'] in ('pass','fail','skip'))
            require(count['pass']>=4214 and count['fail']==0 and count['skip']==18 and not any(r['Action']=='fail' for r in rows), 'native pass/skip denominator failed')
            old=events(raw(root/prior.PACKET/'native'/(label+'.stdout.gz')))
            ids=lambda rs,a:{(r['Package'],r['Test']) for r in rs if r.get('Test') and r['Action']==a}
            require(ids(old,'pass')<=ids(rows,'pass') and ids(old,'skip')==ids(rows,'skip'), 'prior passing/skip tests omitted')
            counts[label]={k:count[k] for k in ('pass','fail','skip')}
        if label=='build': require(receipt['argv']==['go','build','./...'], 'whole build narrowed')
        if label=='lint': require(receipt['argv'][:2]==['make','lint'] and all(x in logs['stdout'].decode() for x in ('0 issues.','go vet ./...','documentation: OK (134 live documents)')), 'fmt/vet/lint/docs incomplete')
    groups=read(p/'native/process-groups.json')
    require([(r['label'],r['pgid']) for r in groups]==[(r['label'],r['pid']) for r in receipts] and all(r['remaining_live_group_members']==0 for r in groups), 'native process groups remain')
    binary=read(p/'native/binary.json'); candidate=read(p/'candidate/binary.json.gz')
    require(binary['sha256']==candidate['sha256'] and binary['source_head']==REVISION and binary['source_mode']==binding['source_mode'] and binary['native_source_binding_sha256']==sha((p/'source-binding.json').read_bytes()) and read(p/'candidate/source-binding.json.gz')==binding, 'runtime binary differs from fully tested source')
    config=read(p/'native/lint-config-binding.json'); versions=read(p/'native/installed-tools.json')
    require(set(config)=={'.golangci.yml','docs/DOCUMENTATION-CATALOG.json'} and all(sha((root/n).read_bytes())==h for n,h in config.items()), 'lint inputs changed')
    require(read(p/'native/environment-policy.json')['GIT_CONFIG_GLOBAL']=='/dev/null' and read(p/'native/environment-policy.json')['global_Git_files_modified'] is False, 'fixture Git configuration scope changed')
    require(versions['go'].startswith('go version go1.26.8 ') and 'version 2.14.0 ' in versions['lint'] and versions['capture_before_lint'] is True, 'pinned tool evidence differs')
    return dict(source_files=len(binding['source_sha256']),Go_files=sum(n.endswith('.go') for n in binding['source_sha256']),changed_Go_files=10,approved_inputs=5,tests=counts,full_build_fmt_vet_lint_docs=True,binary_sha256=binary['sha256'],process_groups_remaining=0)

def historical(root,p):
    old=prior.PACKET.parent/'four-role-current-v1'
    manifest=json.loads(subprocess.check_output(['git','show',REVISION+':'+str(old/'manifest.json')],cwd=root))
    require((root/old/'manifest.json').read_bytes()==subprocess.check_output(['git','show',REVISION+':'+str(old/'manifest.json')],cwd=root), 'historical role manifest changed')
    require(manifest['artifacts']=={str(x.relative_to(root/old)):sha(x.read_bytes()) for x in (root/old).rglob('*') if x.is_file() and x.name!='manifest.json'}, 'historical role artifact changed')
    for n,h in manifest['tools'].items(): require(sha(subprocess.check_output(['git','show',REVISION+':'+n],cwd=root))==h, 'historical tool differs at its own commit')
    for label in ('initial','intermediate','actual'):
        scope=read(p/label/'SCOPE.json.gz')
        for name,h in scope['tools_sha256'].items():
            content=raw(p/'initial-runner.py.gz') if label=='initial' and name=='scripts/run-intent-startup-batch.py' else (root/name).read_bytes()
            require(sha(content)==h, 'frozen runtime tool changed')
    return True

def derive(root=ROOT,p=None):
    p=p or root/PACKET
    historic=historical(root,p); n=native(root,p)
    initial=inspect_runtime(p/'initial',p,False)
    intermediate=inspect_runtime(p/'intermediate',p,True,known_erased_source=True)
    actual=inspect_runtime(p/'actual',p,True)
    require(read(p/'actual/SCOPE.json.gz')['retained_capability_addendum_sha256']==sha((p/'RETAINED-CAPABILITY-ADDENDUM.json').read_bytes()), 'corrective capability specification changed')
    failed=read(p/'native-failed-attempt/receipts.json.gz')
    require([r['label'] for r in failed]==['build','test'] and failed[-1]['exit_code']==1 and failed[-1]['tests']=={'pass':4182,'fail':12,'skip':18}, 'earlier general-test failure omitted')
    require(all(r['remaining_live_group_members']==0 for r in read(p/'native-failed-attempt/process-groups.json.gz')), 'earlier native process remains')
    require(read(p/'actual/SCOPE.json.gz')['binaries']['batch']['sha256']==n['binary_sha256']==read(p/'runtime-candidate/binary.json.gz')['sha256'], 'actual final candidate binary differs')
    runtime_binding=read(p/'runtime-candidate/source-binding.json.gz');final_binding=read(p/'source-binding.json')
    require([name for name,h in final_binding['source_sha256'].items() if runtime_binding['source_sha256'].get(name)!=h]==['internal/setup/command_guard_test.go','internal/system/backendmeasure/graph_capabilities_test.go'], 'runtime-to-final source delta not limited to the two corrected tests')
    old_test=raw(p/'code/internal/system/backendmeasure/graph_capabilities_test.go.gz').replace(b'p := &retainedProbe{Client: graphWithoutResolver{Client: &ckgclient.Fake{}}}',b'p := &retainedProbe{Client: &ckgclient.Fake{}}')
    require(sha(old_test)==runtime_binding['source_sha256']['internal/system/backendmeasure/graph_capabilities_test.go'], 'old test fixture byte reconstruction differs')
    require(sha(raw(p/'owner-observation/original-command-guard.go.txt.gz'))==runtime_binding['source_sha256']['internal/setup/command_guard_test.go'], 'original owner fixture not archived')
    require(read(p/'OWNER-OBSERVATION-ADDENDUM.json')['original_test_sha256']==runtime_binding['source_sha256']['internal/setup/command_guard_test.go'], 'owner observation addendum differs')
    owner_attempt=read(p/'native-owner-failed-attempt/receipts.json.gz')
    require([r['label']for r in owner_attempt]==['build','test','race'] and owner_attempt[1]['tests']=={'pass':4214,'fail':0,'skip':18} and owner_attempt[-1]['exit_code']==-15, 'whole-race assertion/intentional interruption missing')
    failed_rows=events(raw(p/'native-owner-failed-attempt/race.stdout.gz'))
    require([(r['Package'],r['Test'])for r in failed_rows if r.get('Test')and r['Action']=='fail']==[('github.com/0xmhha/knowledge-system/internal/setup','TestGuardianActualOwnerKillStopsWriterAndReleasesLease')], 'original owner failure identity changed')
    for directory,profiles,count in [('owner-original-reproduce',['race'],20),('owner-gated',['general','race'],10)]:
        for profile in profiles:
            q=p/directory/profile;receipt=read(q/'receipt.json.gz')
            require(receipt['exit_code']==0 and not receipt['timeout'] and receipt['remaining_live_group_members']==0 and receipt['reaped'], 'owner diagnostic process failed/unclean')
            records=events(raw(q/'stdout.gz'))
            require(sum(r.get('Test')=='TestGuardianActualOwnerKillStopsWriterAndReleasesLease' and r['Action']=='pass'for r in records)==count and not any(r['Action']=='fail'for r in records), 'owner repeat population differs')
    interrupted=read(p/'native-interrupted-attempt/INTERRUPTION.json.gz')
    require(all(r['remaining_live_group_members']==0 for r in read(p/'native-interrupted-attempt/process-groups.json.gz')), 'interrupted native group remains')
    require(interrupted['source_not_changed_before_interrupt'] and interrupted['not_runtime_server_failure'] and read(p/'native-interrupted-attempt/receipts.json.gz')[-1]['exit_code']==-15, 'interrupted fixture correction attempt omitted')
    return dict(kind=KIND,native=n,initial=initial,intermediate=intermediate,actual=actual,all_CKS_sessions=16,
                all_normal_process_exits=8,all_source_parity_successes=7,all_expected_provider_startup_refusals=8,
                intermediate_measured_source_parity_failures=1,earlier_general_test_environment_failures=12,earlier_owner_kill_race_assertion_failures=1,
                original_owner_failure_root_cause='unconfirmed_after_original20_passes',
                owner_test_post_kill_release_gate=True,production_guardian_changed=False,
                prior_role_evidence_verified_at_own_commit=historic,independent_approved=0,
                formal_completed=0,formal_total=9,FINAL=0,freeze=None,execution_allowed=False,
                D04='미정',quality_score=None,real_LLM_calls=0,paid_external_calls=0,
                request_counts_are_HTTP_not_inference_items=True,latency_SLA_accepted=False)

def reference(root=ROOT): return dict(kind=KIND,manifest=str(PACKET/'manifest.json'),manifest_sha256=sha((root/PACKET/'manifest.json').read_bytes()))
def seal(p): write(p/'manifest.json',dict(kind=KIND,tools={n:sha((ROOT/n).read_bytes()) for n in TOOLS},artifacts={str(x.relative_to(p)):sha(x.read_bytes()) for x in sorted(p.rglob('*')) if x.is_file() and x.name!='manifest.json'}))
def validate(root=ROOT,packet=None):
    p=packet or root/PACKET
    try:
        m=read(p/'manifest.json')
        require(m['kind']==KIND and m['tools']=={n:sha((root/n).read_bytes()) for n in TOOLS} and m['artifacts']=={str(x.relative_to(p)):sha(x.read_bytes()) for x in p.rglob('*') if x.is_file() and x.name!='manifest.json'}, 'batch evidence/tools integrity differs')
        require(read(p/'SUMMARY.json')==derive(root,p), 'batch evidence summary promoted or changed')
        return []
    except (OSError,ValueError,KeyError,TypeError,IndexError) as exc: return [str(exc)]

def update_indices():
    docs=[read(ROOT/n) for n in PATHS]; state=read(ROOT/PACKET/'WORKLIST.json'); ref=reference()
    worklist=Path('docs/spec-driven/REMAINING-13-EXECUTION.json')
    for data in docs:
        data['current_readiness_preparation']=ref;data['current_execution_state']=state
        data['latest_intent_batch_followup']=ref
        data['active_remaining_worklist']=dict(path=str(worklist),sha256=sha((ROOT/worklist).read_bytes()))
    write(ROOT/PATHS[2],docs[2]);link=dict(path=str(PATHS[2]),sha256=sha((ROOT/PATHS[2]).read_bytes()),status='D04_review_pending_local_native_DEV_not_independent_acceptance')
    for i in (0,1,3):docs[i]['adoption_acceptance_decision_proposal']=link;write(ROOT/PATHS[i],docs[i])

def current_indices(root,protocol,ledger):
    errors=[]; docs=[read(root/n) for n in PATHS];docs[0],docs[1]=protocol,ledger
    allowed={'current_readiness_preparation','current_execution_state','latest_intent_batch_followup','active_remaining_worklist','adoption_acceptance_decision_proposal'}
    for path,data in zip(PATHS,docs):
        before=json.loads(subprocess.check_output(['git','show',REVISION+':'+str(path)],cwd=root))
        if any(data.get(k)!=v for k,v in before.items() if k not in allowed):errors.append('prior decision/role/gold changed')
        if data.get('current_readiness_preparation')!=reference(root) or data.get('current_execution_state')!=read(root/PACKET/'WORKLIST.json'):errors.append('batch readiness/state differs')
        target=Path('docs/spec-driven/REMAINING-13-EXECUTION.json')
        if data.get('active_remaining_worklist')!=dict(path=str(target),sha256=sha((root/target).read_bytes())):errors.append('active remaining worklist differs')
    link=dict(path=str(PATHS[2]),sha256=sha((root/PATHS[2]).read_bytes()),status='D04_review_pending_local_native_DEV_not_independent_acceptance')
    if any(d.get('adoption_acceptance_decision_proposal')!=link for d in (docs[0],docs[1],docs[3])):errors.append('proposal hash reference differs')
    return errors

def prepare(native_dir):
    p=ROOT/PACKET
    require(not (p/'SUMMARY.json').exists(), 'refusing final evidence overwrite')
    for label,source in [('initial',Path('/private/tmp/ks-phase77-startup-runtime-20261019')),('intermediate',Path('/private/tmp/ks-phase77-startup-measured-runtime-20261019')),('actual',Path('/private/tmp/ks-phase77-startup-retained-runtime-20261019'))]:
        for x in sorted(source.rglob('*')):
            parts=x.relative_to(source).parts
            if not x.is_file() or x.is_symlink() or parts[0] in ('data','empty-live-source') or x.suffix in ('.lease','.lock'):continue
            compress(p/label/(str(x.relative_to(source))+'.gz'),x.read_bytes())
    compress(p/'owner-observation/original-command-guard.go.txt.gz',Path('/private/tmp/phase77-command-guard-original.go.txt').read_bytes())
    compress(p/'initial-runner.py.gz',Path('/private/tmp/phase77-initial-runtime-driver-used.py').read_bytes())
    for label,source in [('candidate',Path('/private/tmp/ks-phase77-candidate-gated-20261019')),('pre-gate-candidate',Path('/private/tmp/ks-phase77-candidate-final-20261019')),('runtime-candidate',Path('/private/tmp/ks-phase77-candidate-retained-20261019')),('intermediate-candidate',Path('/private/tmp/ks-phase77-candidate-measured-20261019')),('initial-candidate',Path('/private/tmp/ks-phase77-candidate-20261019'))]:
        for x in source.rglob('*'):
            if x.is_file() and x.name!='cks':compress(p/label/(str(x.relative_to(source))+'.gz'),x.read_bytes())
    for label,source in [('native-failed-attempt',Path('/private/tmp/ks-phase77-native-20261019')),('native-owner-failed-attempt',Path('/private/tmp/ks-phase77-native-corrective2-20261019')),('owner-original-reproduce',Path('/private/tmp/ks-phase77-owner-original-reproduce-20261019')),('owner-gated',Path('/private/tmp/ks-phase77-owner-gated-20261019')),('focused',Path('/private/tmp/ks-phase77-focused-retained-corrective1-20261019')),('focused-fixture-failed',Path('/private/tmp/ks-phase77-focused-retained-20261019')),('native-interrupted-attempt',Path('/private/tmp/ks-phase77-native-corrective1-20261019')),('candidate-metadata-denied',Path('/private/tmp/ks-phase77-candidate-retained-metadata-denied-20261019')),('focused-metadata-denied',Path('/private/tmp/ks-phase77-focused-retained-metadata-denied-20261019'))]:
        for x in source.rglob('*'):
            if x.is_file() and x.name not in ('cks','read-cks'):
                compress(p/label/(str(x.relative_to(source))+'.gz'),x.read_bytes())
    for name in ('run-phase77-native.py','run-phase77-native-corrective1.py','run-phase77-native-corrective2.py','run-phase77-native-corrective3.py','run-phase77-owner-original-reproduce.py','run-phase77-owner-gated.py','run-phase77-focused-retained.py','run-phase77-focused-retained-corrective1.py','build-phase77-candidate-final.py'):
        compress(p/'drivers'/(name+'.gz'),Path('/private/tmp',name).read_bytes())
    for name in ('phase77-evidence-tests.log','phase77-build-retained-driver.log','phase77-focused-retained-driver.log','phase77-native-driver.log'):
        compress(p/'packaging-attempts'/(name+'.gz'),Path('/private/tmp',name).read_bytes())
    write(p/'packaging-attempts/NOTE.json',dict(evidence_guard_failure='first checker compared timed request wrapper to raw stdout, corrected to compare actual payload/wire; then correctly found measured source unavailable',metadata_denied='build/focused commands ran but process cleanup metadata ps was denied by sandbox; evidence not adopted, explicit elevated corrective rerun',original_general_failed='12 fixture Git commit signing failures; global Git config isolated per test process, never edited',source_parity_guard_failure_is_retained=True))
    for name in ('source-binding.json','receipts.json','binary.json'):
        target=p/('source-binding.json' if name=='source-binding.json' else 'native/'+name);write(target,read(native_dir/name))
    for x in native_dir.iterdir():
        if x.suffix in ('.stdout','.stderr'):compress(p/'native'/(x.name+'.gz'),x.read_bytes())
    rows=[r.split() for r in subprocess.check_output(['ps','-axo','pid=,pgid=,stat='],text=True).splitlines()]
    for label,directory in [('native-failed-attempt',Path('/private/tmp/ks-phase77-native-20261019')),('native-interrupted-attempt',Path('/private/tmp/ks-phase77-native-corrective1-20261019')),('native-owner-failed-attempt',Path('/private/tmp/ks-phase77-native-corrective2-20261019'))]:
        prior_groups=[dict(label=r['label'],pgid=r['pid'],remaining_live_group_members=sum(len(x)==3 and int(x[1])==r['pid'] and not x[2].startswith('Z') for x in rows)) for r in read(directory/'receipts.json')]
        compress(p/label/'process-groups.json.gz',json.dumps(prior_groups,indent=2).encode())
    write(p/'native/process-groups.json',[dict(label=r['label'],pgid=r['pid'],remaining_live_group_members=sum(len(x)==3 and int(x[1])==r['pid'] and not x[2].startswith('Z') for x in rows)) for r in read(native_dir/'receipts.json')])
    write(p/'native/lint-config-binding.json',{n:sha((ROOT/n).read_bytes()) for n in ('.golangci.yml','docs/DOCUMENTATION-CATALOG.json')})
    versions=read(native_dir/'installed-tools.json')
    require(all(sha(Path(n).read_bytes())==h for n,h in versions['executable_sha256'].items()), 'installed tools changed after pre-lint capture')
    write(p/'native/installed-tools.json',versions)
    write(p/'native/environment-policy.json',dict(inherited_environment='positive_allowlist_only',GIT_CONFIG_GLOBAL='/dev/null',GIT_CONFIG_NOSYSTEM='1',scope='native_tests_and_build_process_only',global_Git_files_modified=False,GOTOOLCHAIN='local',GOPROXY='off',GOSUMDB='off',source_head_unchanged_during_all_commands=True))
    changes=[]
    for name in read(p/'source-binding.json')['changed_native_files']:
        previous=subprocess.run(['git','show',REVISION+':'+name],cwd=ROOT,capture_output=True);artifact='code/'+name+'.gz';compress(p/artifact,(ROOT/name).read_bytes())
        changes.append(dict(path=name,previous_sha256=sha(previous.stdout) if previous.returncode==0 else None,current_sha256=sha((ROOT/name).read_bytes()),artifact=artifact))
    write(p/'code-changes.json',changes)
    write(p/'SUMMARY.json',derive())
    state=read(ROOT/prior.PACKET/'WORKLIST.json');state.update(current_stage='77_source_bound_intent_batch_native_and_real_runtime_verified_DEV',next_task='WI4_real_peer_sync_and_remaining13',completed_in_this_scope=['batch_anchor65_mapping_copy_bounds_error_and_cancellation','measurement_intent_batch_and_exact_CKG_optional_method_sets','actual16_CKS_sessions_all_failures_preserved_and_final_source_parity','owner_test_post_kill_release_gate_without_guardian_change','fresh_source10_whole_build_general_race_fmt_vet_lint_docs'],current_intent_batch=read(p/'SUMMARY.json'))
    write(p/'WORKLIST.json',state);seal(p)

def stored():
    revision=subprocess.check_output(['git','log','-1','--format=%H','--',str(PACKET/'manifest.json')],cwd=ROOT,text=True).strip()
    require(bool(revision),'batch evidence not committed')
    paths=subprocess.check_output(['git','diff-tree','--root','--no-commit-id','--name-only','-r',revision],cwd=ROOT,text=True).splitlines()
    for name in paths:
        committed=subprocess.check_output(['git','show',revision+':'+name],cwd=ROOT)
        require(committed==(ROOT/name).read_bytes(),'stored changed blob differs: '+name)
    require(subprocess.run(['git','verify-commit',revision],cwd=ROOT,capture_output=True).returncode==0,'batch commit signature not verified')
    return dict(revision=revision,changed_blobs_verified=len(paths),signature_verified=True)

if __name__=='__main__':
    parser=argparse.ArgumentParser();parser.add_argument('command',choices=['prepare','check','update-indices']);parser.add_argument('--native',type=Path);parser.add_argument('--require-stored',action='store_true');args=parser.parse_args()
    if args.command=='prepare':prepare(args.native)
    elif args.command=='update-indices':update_indices()
    else:
        errors=validate()
        if errors:raise SystemExit('\n'.join(errors))
        if args.require_stored:print(json.dumps(stored()))
        print(json.dumps(read(ROOT/PACKET/'SUMMARY.json'),ensure_ascii=False,indent=2))
