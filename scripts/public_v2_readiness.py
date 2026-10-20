"""Check source-bound public v2 DEV evidence without promoting acceptance."""
import gzip
import json
import subprocess
from pathlib import Path
import viewer_readiness as v

ROOT=v.ROOT
PACKET=Path('docs/spec-driven/operational-preparation/source-review-supplement/followups/public-v2-surface-v1')
BASE='deb61f8af2b3cdb1cca6134d2c76b8d938260ea3'
KIND='public_v2_CLI_HTTP_graph_native_DEV_not_independent_acceptance'
TOOLS=['scripts/public_v2_readiness.py','scripts/test_public_v2_readiness.py','scripts/check-operational-acceptance.py','scripts/viewer_readiness.py','scripts/run-wi4-target-subscription.py','scripts/verify-wi3-reasons-native.py']

def native_population(rows,previous):
    return v.test_population(rows,previous)

def boundary(d):
    v.require(all(d.get(k)==0 for k in ('formal_completed','independent_approved','actual_LLM_calls','paid_external_calls')),'DEV evidence promoted to formal/paid execution')
    v.require(d.get('direct_CKV_v2_implemented')is False and d.get('static_chunked_v2_implemented')is False,'pending direct engine surface promoted')

def derive(p):
    b=v.read(p/'source-binding.json')
    v.require(b['base_revision']==BASE,'native base differs')
    receipts=v.read(p/'native/receipts.json.gz')
    v.require([r['label']for r in receipts]==['build','test','race','lint','build-bins','build-runtime','build-info'],'native population incomplete')
    previous=p.parent/'viewer-lifecycle-v1/native';result={}
    old_manifest=p.parent/'viewer-lifecycle-v1/manifest.json'
    old_bytes=subprocess.check_output(['git','show',BASE+':'+str(old_manifest.relative_to(ROOT))],cwd=ROOT)
    v.require(v.sha(old_manifest.read_bytes())==v.sha(old_bytes),'prior native packet manifest changed')
    old=v.read(old_manifest)['artifacts']
    for label in ('test','race'):
        name='native/'+label+'.stdout.gz';v.require(v.sha((old_manifest.parent/name).read_bytes())==old[name],'prior native population changed')
    for r in receipts:
        v.require(r['exit_code']==0 and not r['timeout']and not r['term_sent']and not r['kill_sent']and r['reaped']and r['source_head']==BASE and r['source_binding_sha256']==v.sha((p/'source-binding.json').read_bytes()),'native command or input identity failed')
        streams={s:v.raw(p/'native'/(r['label']+'.'+s+'.gz'))for s in ('stdout','stderr')}
        v.require(all(v.sha(streams[s])==r[s+'_sha256']for s in streams),'native raw log differs')
        if r['label']in ('test','race'):
            expected=['go','test']+(['-race']if r['label']=='race'else[])+['-json','-count=1','-timeout='+('20m'if r['label']=='race'else'18m'),'./...']
            v.require(r['argv']==expected,'whole-module population narrowed')
            result[r['label']]=native_population(v.events(streams['stdout']),v.events(v.raw(previous/(r['label']+'.stdout.gz'))))
            v.require(r['tests']==result[r['label']],'native counts differ')
        if r['label']=='lint':
            v.require(all(s in streams['stdout'].decode() for s in ('0 issues.','go vet ./...','documentation: OK')),'full fmt/vet/lint/docs missing')
    v.process_groups(receipts,v.read(p/'native/process-groups.json.gz'))
    binaries={n:v.read(p/'native'/('binary.json.gz'if n=='cks'else'graph-binary.json.gz'))['sha256']for n in ('cks','ckg')}
    for label in ('build-graph-runtime','graph-build-info'):
        r=v.read(p/'native'/(label+'-receipt.json.gz'));v.require(r['exit_code']==0 and r['reaped']and not r['remaining_live_group_members']and r['source_head']==BASE,'graph native binary command failed')
    tools=v.read(p/'native/TOOLS-BEFORE-LINT.json.gz');after=v.read(p/'native/installed-tools.json.gz')
    v.require(tools['capture_before_lint']and tools['go'].startswith('go version go1.26.8 ')and 'version 2.14.0 'in tools['lint']and all(tools[n]==after[n]for n in ('go','lint')),'native tools changed')
    v.require(tools['lint_configuration_sha256']==v.sha((ROOT/'.golangci.yml').read_bytes()),'lint configuration changed')
    packages=v.read(p/'native/package-boundary-proof.json.gz');v.require(packages['previous']==packages['current']and not packages['removed_external_only'],'native package population changed')
    v.require(v.read(p/'candidate/source-binding.json.gz')==b,'candidate/native source binding differs')
    v.require(binaries=={n:v.read(p/'candidate'/('binary.json.gz'if n=='cks'else'graph-binary.json.gz'))['sha256']for n in binaries},'native binary differs from actual runtime')
    actual=p/'actual';scope=v.read(actual/'SCOPE.json.gz');end=v.read(actual/'receipt.json.gz');parity=v.read(actual/'transport-result.json.gz')
    v.require(scope['binary_sha256']==binaries['cks']and scope['source_binding_sha256']==v.sha((p/'source-binding.json').read_bytes()),'actual CKS identity differs')
    v.require(all(end.get(k)==0 for k in ('formal_completed','independent_approved','actual_LLM_calls','paid_external_calls')),'actual DEV promoted')
    v.require(end['failure']is None and not end['proxy_thread_alive']and end['payload_preserved'],'actual CLI/HTTP cleanup or payload failed')
    for field in ('same_coordinates','same_query','same_integrity_hash','HTTP_invalid_request','legacy_markdown_retained'):v.require(parity[field],'actual CLI/HTTP parity failed: '+field)
    cmds=v.read(actual/'CLI-receipts.json.gz');v.require([(x['name'],x['receipt']['exit_code'])for x in cmds]==[('v2',0),('existing-output',1),('legacy',0)],'actual CLI cases differ')
    for x in cmds:
        r=x['receipt'];v.require(r['reaped']and not r['timeout']and not r['remaining_live_group_members'],'actual CLI process remains')
        server=v.read(actual/(x['name']+'-server')/'exit.json.gz');v.require(server['exit_code']==0,'owned actual MCP did not exit successfully')
    h=v.read(actual/'HTTP-server-exit.json.gz');v.require(h['exit_code']==0 and h['reaped']and not h['forced']and not h['remaining_group'],'HTTP server cleanup failed')
    for name in ('context-v2.json','HTTP-v2.json'):
        pack=v.read(actual/(name+'.gz'));digest=pack['metadata'].pop('integrity_hash');canonical=json.dumps(pack,ensure_ascii=False,sort_keys=True,separators=(',',':')).encode();v.require(v.sha(canonical)==digest,'actual independent v2 hash differs')
    http=v.read(actual/'HTTP-model-events.json.gz');embedding=[x for x in http if x['path']in ('/api/embed','/api/embeddings')];v.require(all(x['status']==200 for x in http)and not any(x['path']in ('/api/chat','/api/generate')for x in http),'unexpected provider path/failure')
    graphdir=p/'graph'
    normal=v.read(graphdir/'success/receipt.json.gz');cancel=v.read(graphdir/'cancel-exit.json.gz')
    v.require(normal['exit_code']==0 and normal['reaped']and not normal['timeout']and not normal['remaining_live_group_members'],'normal graph export cleanup failed')
    v.require(cancel['exit_code']==1 and cancel['reaped']and cancel['term_sent']and not cancel['forced']and not cancel['remaining_group'],'actual canceled graph cleanup failed')
    v.require(v.read(graphdir/'cancel.stderr.gz')['code']=='request_canceled','actual graph cancellation code differs')
    v.require(v.read(graphdir/'payload-before.json.gz')==v.read(graphdir/'payload-after.json.gz'),'graph payload changed')
    graph=v.read(p/'graph/RESULT.json.gz');v.require(graph['success']and graph['counts']=={'nodes':244989,'edges':1964193}and graph['all_minimal_rows_match_SQLite']and graph['canceled_output_absent']and graph['temporary_files_remaining']==0 and graph['payload_preserved']and graph['actual_LLM_calls']==0,'actual graph export/recovery failed')
    v.require(graph['binary_sha256']==binaries['ckg']and graph['source_binding_sha256']==v.sha((p/'source-binding.json').read_bytes()),'actual CKG identity differs')
    d=dict(kind=KIND,native=result,native_source_inputs=len(b['source_sha256']),Go_files=sum(n.endswith('.go')for n in b['source_sha256']),changed_Go_files=sum(n.endswith('.go')for n in b['changed_native_files']),CLI_HTTP=parity,local_embedding_HTTP_requests=len(embedding),local_embedding_input_items=sum(x['inputs']for x in embedding),graph=graph,owned_process_groups_remaining=0,direct_CKV_v2_implemented=False,static_chunked_v2_implemented=False,formal_completed=0,independent_approved=0,actual_LLM_calls=0,paid_external_calls=0)
    docs=v.read(p/'post-native-docs/receipt.json.gz')
    v.require(docs['exit_code']==0 and docs['reaped']and not docs['timeout']and not docs['remaining_live_group_members'],'post-native documentation check failed')
    for stream in ('stdout','stderr'):v.require(v.sha(v.raw(p/'post-native-docs'/(stream+'.gz')))==docs[stream+'_sha256'],'post-native docs log differs')
    v.require('documentation: OK'in v.raw(p/'post-native-docs/stdout.gz').decode(),'documentation was not rechecked')
    after=v.read(p/'post-native-docs/binary-equality.json.gz');v.require(after['cks']==binaries['cks']and after['ckg']==binaries['ckg']and after['Go_inputs_preserved'],'documentation change affected compiled input')
    d['post_native_documentation_only_recheck']=True
    boundary(d)
    return d

def validate(root=ROOT,p=None):
    p=p or root/PACKET
    try:
        m=v.read(p/'manifest.json');v.require(m['artifacts']==v.inventory(p),'public v2 artifact population/bytes changed');v.require(m['tools']=={n:v.sha((root/n).read_bytes())for n in TOOLS},'public v2 verifier tools changed')
        b=v.read(p/'source-binding.json');current=v.read(p/'current-source-binding.json');expected=json.loads(json.dumps(b));expected['source_sha256'].update({n:v.sha((root/n).read_bytes())for n in TOOLS})
        delta=v.read(p/'POST-NATIVE-DOCUMENTATION-DELTA.json');doc='docs/spec-driven/PUBLIC-CONTRACT-V2.md'
        v.require(set(delta)=={doc}and delta[doc]['not_a_native_compilation_input']is True and delta[doc]['baseline_sha256']==b['source_sha256'][doc]and delta[doc]['baseline_sha256']==v.sha(v.raw(p/'native-public-contract.md.gz')),'undeclared documentation delta')
        expected['source_sha256'][doc]=delta[doc]['current_sha256']
        expected['native_source_binding_sha256']=v.sha((p/'source-binding.json').read_bytes());v.require(current==expected,'current binding differs')
        for n,h in {**current['source_sha256'],**current['approved_input_sha256']}.items():v.require(v.sha((root/n).read_bytes())==h,'current input changed: '+n)
        names=subprocess.check_output(['git','ls-files','--cached','--others','--exclude-standard','*.go'],cwd=root,text=True).splitlines();v.require({n for n in names if n.startswith(('cmd/','internal/','pkg/'))}=={n for n in b['source_sha256']if n.endswith('.go')and n.startswith(('cmd/','internal/','pkg/'))},'Go inventory differs')
        app=root/'internal/system/viewer/web_assets/app'
        if app.exists():v.require({str(f.relative_to(root)):v.sha(f.read_bytes())for f in app.rglob('*')if f.is_file()}==b['generated_input_sha256'],'optional generated UI differs')
        spec=v.read(p/'SPEC.json');v.require(v.sha((root/spec['requirements']['path']).read_bytes())==spec['requirements']['sha256'],'public contract changed')
        v.require(v.read(p/'SUMMARY.json')==derive(p),'public v2 summary changed/promoted')
        return []
    except (ValueError,OSError,KeyError,TypeError)as e:return[str(e)]

def reference(root=ROOT):return dict(kind=KIND,manifest=str(PACKET/'manifest.json'),manifest_sha256=v.sha((root/PACKET/'manifest.json').read_bytes()))

def current_indices(root,protocol,ledger):
    errors=[]
    try:
        ref=reference(root);target=Path('docs/spec-driven/REMAINING-13-EXECUTION.json');work=dict(path=str(target),sha256=v.sha((root/target).read_bytes()));state=v.read(root/PACKET/'WORKLIST.json')
        allowed={'current_readiness_preparation','latest_public_v2_followup','current_execution_state','active_remaining_worklist','adoption_acceptance_decision_proposal'}
        docs=[v.read(root/n)for n in v.PATHS]
        docs[:2]=[protocol,ledger]
        for n,d in zip(v.PATHS,docs):
            old=json.loads(subprocess.check_output(['git','show',BASE+':'+str(n)],cwd=root))
            v.require(all(d.get(k)==value for k,value in old.items()if k not in allowed),'prior decision/gold changed')
            v.require(d['current_readiness_preparation']==d['latest_public_v2_followup']==ref and d['active_remaining_worklist']==work and d['current_execution_state']==state,'current public v2 references differ')
        v.require(protocol['current_readiness_preparation']==ledger['current_readiness_preparation']==ref,'protocol/ledger reference differs')
        link=dict(path=str(v.PATHS[2]),sha256=v.sha((root/v.PATHS[2]).read_bytes()),status='D04_review_pending_local_native_DEV_not_independent_acceptance')
        v.require(all(v.read(root/v.PATHS[i]).get('adoption_acceptance_decision_proposal')==link for i in (0,1,3)),'proposal hash link differs')
    except(ValueError,OSError,KeyError,TypeError)as e:errors.append(str(e))
    return errors
