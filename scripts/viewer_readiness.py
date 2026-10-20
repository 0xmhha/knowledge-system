"""Verify owned viewer evidence without promoting DEV checks to acceptance."""
import gzip
import hashlib
import io
import json
import subprocess
import tarfile
from collections import Counter
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
PACKET = Path('docs/spec-driven/operational-preparation/source-review-supplement/followups/viewer-lifecycle-v1')
KIND = 'viewer_owned_UI_lifecycle_native_DEV_not_independent_acceptance'
BASE = '49662a5bc27dbae01e956c8c03a72a5b803a174e'
PATHS = [Path('docs/spec-driven/OPERATIONAL-ACCEPTANCE-'+n+'.json') for n in ('PROTOCOL','PROGRESS','DECISION-PROPOSAL')]+[PACKET.parent.parent/'GAPS.json']
TOOLS = ['scripts/viewer_readiness.py','scripts/test_viewer_readiness.py','scripts/check-operational-acceptance.py',
         'scripts/run-wi4-target-subscription.py','scripts/run-wi6-pair-pilot.py','scripts/verify-wi3-reasons-native.py']
def sha(b): return hashlib.sha256(b).hexdigest()
def read(p):
    b=p.read_bytes()
    return json.loads(gzip.decompress(b) if p.suffix=='.gz' else b)
def write(p,v):
    p.parent.mkdir(parents=True,exist_ok=True)
    p.write_text(json.dumps(v,ensure_ascii=False,indent=2)+'\n')
def require(ok,reason):
    if not ok: raise ValueError(reason)
def raw(p): return gzip.decompress(p.read_bytes())
def events(b): return [json.loads(x) for x in b.splitlines() if x.strip()]
def inventory(p): return {str(x.relative_to(p)):sha(x.read_bytes()) for x in sorted(p.rglob('*')) if x.is_file() and x.name!='manifest.json'}
def counts(rows): return Counter(r['Action'] for r in rows if r.get('Test') and r['Action'] in ('pass','fail','skip'))
def process_groups(receipts,groups):
    require([(r['label'],r['pid'])for r in receipts]==[(g['label'],g['pgid'])for g in groups]and all(g['remaining_live_group_members']==0 for g in groups),'native process group population/cleanup differs')
def default_profile(p,binding):
    q=p/'default-profile';receipt=read(q/'receipt.json.gz');export=read(q/'export-result.json.gz')
    require(receipt['optional_payload_restored']and receipt['generated_files_preserved']==46 and receipt['default_stub_export_only']and receipt['real_UI_success_claimed']is False and export['unbuilt_stub']and export['UI_success_claimed']is False,'default stub profile promoted to real UI')
    require(export['files']=={'index.html':binding['source_sha256']['internal/system/viewer/web_assets/index.html']},'default export is not exactly tracked fallback')
    expected={n.removeprefix('internal/system/viewer/web_assets/app/'):h for n,h in binding['generated_input_sha256'].items()}
    require(read(q/'payload-before.json.gz')==read(q/'payload-after.json.gz')==expected,'default smoke did not restore generated payload')
    for label in ('test','build','export-command'):
        r=read(q/label/'receipt.json.gz');require(r['exit_code']==0 and r['reaped']and not r['timeout']and not r['term_sent']and not r['kill_sent']and r['remaining_live_group_members']==0,'default profile command/cleanup failed')
        for stream in ('stdout','stderr'):require(sha(raw(q/label/(stream+'.gz')))==r[stream+'_sha256'],'default raw command log differs')
    require(counts(events(raw(q/'test/stdout.gz')))==Counter({'pass':20}),'default focused race population differs')
    return dict(default_focused_race_pass=20,build_and_stub_export=True,real_UI_success=False,generated_payload_restored=46)
def test_population(rows,previous):
    ids=lambda data,action:{(r['Package'],r['Test'])for r in data if r.get('Test')and r['Action']==action}
    c=counts(rows)
    require(c['fail']==0 and c['skip']==18 and c['pass']>=4214,'native tests failed or denominator narrowed')
    require(not any(r.get('Action')=='fail'and not r.get('Test')for r in rows),'native package failed')
    require(ids(previous,'pass')<=ids(rows,'pass') and ids(previous,'skip')==ids(rows,'skip'),'prior passing/skip IDs omitted')
    return {k:c[k]for k in ('pass','fail','skip')}
def inspect_runtime(p,binary):
    scope=read(p/'SCOPE.json.gz');receipt=read(p/'process-receipt.json.gz');end=read(p/'receipt.json.gz')
    require(scope['base_revision']==BASE and scope['binaries']==binary,'actual binary/source identity differs')
    require(receipt['exit_code']==0 and receipt['reaped'] and receipt['normal_parent_SIGTERM_requested'] and not receipt['cleanup_fallback_group_TERM'] and not receipt['cleanup_fallback_group_KILL'] and receipt['remaining_group_members']==[] and receipt['failure'] is None,'actual viewer cleanup failed')
    maps=[read(p/(n+'.json.gz'))for n in ('original-before','original-after','owned-before','owned-after')]
    require(len(maps[0])==2248 and all(x==maps[0]for x in maps),'original/owned data changed')
    require(read(p/'cleanup-recorded-PIDs.json.gz')['remaining']==[],'recorded process remains')
    require(end['original_owned_payload_preserved'] and end['binary_hashes_preserved'] and not end['failed'] and end['actual_LLM_calls']==end['formal_completed']==end['independent_approved']==0,'runtime boundary promoted')
    return dict(exit_code=0,reaped=True,forced_cleanup=False,original_and_owned_files=2248,remaining_recorded_PIDs=0)
def derive(p):
    binding=read(p/'source-binding.json');require(binding['base_revision']==BASE,'native base differs')
    changes=binding['changed_native_files'];require(sum(n.endswith('.go')for n in changes)==6,'declared Go scope differs')
    receipts=read(p/'native/receipts.json.gz');require([r['label']for r in receipts]==['build','test','race','lint','build-bins','build-runtime','build-info'],'native command population incomplete')
    result={}
    previous=p.parent/'intent-startup-batch-v1/native'
    for r in receipts:
        require(r['exit_code']==0 and not r['timeout'] and not r['term_sent'] and not r['kill_sent'] and r['reaped'] and r['source_head']==BASE and r['source_binding_sha256']==sha((p/'source-binding.json').read_bytes()),'native command failed or input identity changed')
        streams={s:raw(p/'native'/(r['label']+'.'+s+'.gz'))for s in ('stdout','stderr')}
        require(all(sha(streams[s])==r[s+'_sha256']for s in streams),'native raw log differs')
        if r['label']in ('test','race'):
            expected=['go','test']+(['-race']if r['label']=='race'else[])+['-json','-count=1','-timeout='+('20m'if r['label']=='race'else'18m'),'./...']
            require(r['argv']==expected,'whole-module test narrowed')
            result[r['label']]=test_population(events(streams['stdout']),events(raw(previous/(r['label']+'.stdout.gz'))))
            require(r['tests']==result[r['label']],'native receipt counts differ from events')
        if r['label']=='lint':require(all(x in streams['stdout'].decode()for x in ('0 issues.','go vet ./...','documentation: OK (134 live documents)')),'full lint/fmt/vet/docs result missing')
    process_groups(receipts,read(p/'native/process-groups.json.gz'))
    tools=read(p/'native/TOOLS-BEFORE-LINT.json.gz');after=read(p/'native/installed-tools.json.gz')
    require(tools['capture_before_lint'] and tools['go'].startswith('go version go1.26.8 ') and 'version 2.14.0 'in tools['lint'] and all(tools[n]==after[n]for n in ('go','lint')),'pinned native tools changed')
    require(tools['lint_configuration_sha256']==sha((ROOT/'.golangci.yml').read_bytes())==sha(subprocess.check_output(['git','show',BASE+':.golangci.yml'],cwd=ROOT)),'lint configuration changed')
    binary={'cks':read(p/'native/binary.json.gz')['sha256'],'ckg':read(p/'native/graph-binary.json.gz')['sha256']}
    require(read(p/'candidate/source-binding.json.gz')==read(p/'native/source-binding.json.gz')==binding,'candidate/native compile inputs differ')
    runtime_binding=read(p/'runtime-candidate/source-binding.json.gz')
    require(read(p/'actual/SCOPE.json.gz')['source_binding_sha256']==sha(raw(p/'runtime-candidate/source-binding.json.gz')),'actual runtime source binding differs')
    source_delta=sorted(n for n in set(binding['source_sha256'])|set(runtime_binding['source_sha256'])if binding['source_sha256'].get(n)!=runtime_binding['source_sha256'].get(n))
    require(source_delta==['tools/viewer/README.md','tools/viewer/go.mod']and binding['generated_input_sha256']==runtime_binding['generated_input_sha256']and binding['approved_input_sha256']==runtime_binding['approved_input_sha256'],'undeclared runtime-to-final source change')
    require(binary=={n:read(p/'runtime-candidate'/('binary.json.gz'if n=='cks'else'graph-binary.json.gz'))['sha256']for n in binary},'module boundary changed observed runtime binaries')
    packages=read(p/'native/package-boundary-proof.json.gz')
    removed={'github.com/0xmhha/knowledge-system/tools/viewer/node_modules/flatted/golang/pkg/flatted'}
    require(set(packages['previous'])-set(packages['current'])==removed and not(set(packages['current'])-set(packages['previous']))and packages['maintained_packages_preserved']and not packages['lint_rule_or_config_changed'],'frontend boundary omitted maintained packages')
    failed=read(p/'failed-native/receipts.json.gz')
    require([(r['label'],r['exit_code'])for r in failed]==[('build',0),('test',0),('race',0),('lint',2)],'initial full native lint failure hidden')
    for r in failed:
        require(not r['timeout']and not r['term_sent']and not r['kill_sent']and r['reaped'],'initial failed native process not reaped')
        for stream in ('stdout','stderr'):require(sha(raw(p/'failed-native'/(r['label']+'.'+stream+'.gz')))==r[stream+'_sha256'],'initial failed native log changed')
    require(binary=={n:read(p/'candidate'/('binary.json.gz'if n=='cks'else'graph-binary.json.gz'))['sha256']for n in binary},'runtime and full native binaries differ')
    for label in ('build-graph-runtime','graph-build-info'):
        r=read(p/'native'/(label+'-receipt.json.gz'))
        require(r['exit_code']==0 and r['reaped'] and not r['remaining_live_group_members'] and r['source_head']==BASE,'actual graph binary build failed')
    built=inspect_runtime(p/'actual',binary)
    navigation=inspect_runtime(p/'navigation',binary)
    require('not built'in raw(p/'initial-browser/initial-snapshot.txt.gz').decode(),'initial stub failure observation omitted')
    initial=read(p/'initial/SCOPE.json.gz');first=inspect_runtime(p/'initial',initial['binaries'])
    visible=read(p/'browser/visible-result.json.gz');keyboard=read(p/'browser/keyboard-result.json.gz')
    require(visible['source'] and visible['help'] and not visible['stub'] and len(visible['canvases'])==1 and all(x['width']>0 and x['height']>0 and x['visible']for x in visible['canvases']),'built UI was not actually rendered')
    require(keyboard['activeTag']=='INPUT' and keyboard['helpClosed'] and keyboard['nodeLimit']=='500' and keyboard['sourceStillPresent'],'actual keyboard/help/redraw controls failed')
    visual=read(p/'VISUAL-REVIEW.json');require(visual['inspected'] and visual['independent_approved']==0,'visual inspection promoted')
    require(read(p/'browser/closing-command-exits.json.gz')['close']==0,'owned browser was not closed')
    close=read(p/'browser-cleanup.json');require(not close['browser_PID_live']and len(close['all_owned_browsers'])==3 and not any(x['live']for x in close['all_owned_browsers']),'owned browser process remains')
    for name in ('back','home','graph-return'):
        nav=read(p/'navigation-browser'/(name+'-result.json.gz'))
        require(nav['rootDepth']and nav['canvases']==1,'actual graph navigation failed')
    atlas=read(p/'navigation-browser/atlas-result.json.gz')
    require(atlas['atlasHeading']and atlas['unsupportedDataVisible']and atlas['queryEmpty'],'Atlas unsupported scope hidden or query injected')
    require('closed'in raw(p/'navigation-browser/close.txt.gz').decode().lower(),'navigation browser not closed')
    generated={}
    with tarfile.open(fileobj=io.BytesIO((p/'generated-payload.tar.gz').read_bytes()),mode='r:gz')as t:
        for item in t.getmembers():
            require(item.isfile()and not item.name.startswith('/')and '..'not in Path(item.name).parts,'invalid generated asset archive entry')
            generated[item.name]=sha(t.extractfile(item).read())
    require(generated==binding['generated_input_sha256'],'compiled generated payload differs')
    export=read(p/'navigation/export-asset-receipt.json.gz')
    require(export['exit_code']==0 and export['CKS_sha256']==binary['cks']and export['asset_sha256']=={n.removeprefix('internal/system/viewer/web_assets/app/'):h for n,h in generated.items()},'actual static export differs from embedded payload')
    default=default_profile(p,binding)
    return dict(kind=KIND,Go_files_changed=6,source_inputs=len(binding['source_sha256']),generated_UI_files=len(generated),native=result,binaries=binary,actual=built,navigation=navigation,default_profile=default,actual_CKS_CKG_runs=3,graph_navigation_and_static_export=True,initial_stub_observation=first,initial_stub_UI_success=False,initial_native_lint_exit=2,external_npm_Go_package_removed=1,maintained_Go_packages_omitted=0,runtime_to_final_source_delta=source_delta,actual_search='downloader.SyncStatusSubscription.Unsubscribe',source='eth/downloader/api.go:183',keyboard_search=True,help_escape=True,node_limit_redraw=500,visual_inspected=True,mode_detection_404s_expected=2,full_accessibility_audit=False,Atlas_vector_backend_verified=False,actual_LLM_calls=0,paid_external_calls=0,independent_approved=0,formal_completed=0,formal_total=9,FINAL=0,freeze=None,execution_allowed=False)
def validate(root=ROOT,p=None):
    p=p or root/PACKET
    try:
        m=read(p/'manifest.json');require(m['artifacts']==inventory(p),'viewer evidence bytes changed')
        require(m['tools']=={n:sha((root/n).read_bytes())for n in TOOLS},'viewer verifier tools changed')
        binding=read(p/'source-binding.json');delta=read(p/'POST-NATIVE-AUXILIARY-DELTA.json')
        require(set(delta)=={'scripts/check-operational-acceptance.py'},'undeclared post-native auxiliary delta')
        current=read(p/'current-source-binding.json')
        expected=json.loads(json.dumps(binding))
        expected['source_sha256']['scripts/check-operational-acceptance.py']=delta['scripts/check-operational-acceptance.py']['current_sha256']
        expected['native_source_binding_sha256']=sha((p/'source-binding.json').read_bytes())
        require(current==expected,'current source ledger binding differs from declared guard-only delta')
        guard='scripts/check-operational-acceptance.py'
        require(guard not in binding['source_sha256']and delta[guard]['not_a_native_compilation_input'] is True and delta[guard]['baseline_sha256']==sha(subprocess.check_output(['git','show',BASE+':'+guard],cwd=root)) and delta[guard]['current_sha256']==sha((root/guard).read_bytes()),'post-native auxiliary guard binding differs')
        for n,h in {**binding['source_sha256'],**binding['approved_input_sha256']}.items():
            require(sha((root/n).read_bytes())==h,'current bound input changed: '+n)
        live=subprocess.check_output(['git','ls-files','--cached','--others','--exclude-standard','*.go'],cwd=root,text=True).splitlines()
        require({n for n in live if n.startswith(('cmd/','internal/','pkg/'))}=={n for n in binding['source_sha256']if n.endswith('.go')and n.startswith(('cmd/','internal/','pkg/'))},'Go inventory changed')
        generated=root/'internal/system/viewer/web_assets/app'
        if generated.exists():require({str(x.relative_to(root)):sha(x.read_bytes())for x in generated.rglob('*')if x.is_file()}==binding['generated_input_sha256'],'live optional build payload changed')
        require(read(p/'SUMMARY.json')==derive(p),'viewer result changed/promoted')
        return []
    except (ValueError,OSError,KeyError,TypeError,tarfile.TarError)as e:return[str(e)]
def reference(root=ROOT):return dict(kind=KIND,manifest=str(PACKET/'manifest.json'),manifest_sha256=sha((root/PACKET/'manifest.json').read_bytes()))
def seal(p):write(p/'manifest.json',dict(kind=KIND,tools={n:sha((ROOT/n).read_bytes())for n in TOOLS},artifacts=inventory(p)))
def update_indices():
    docs=[read(ROOT/n)for n in PATHS];ref=reference();target=Path('docs/spec-driven/REMAINING-13-EXECUTION.json')
    for d in docs:
        d['current_readiness_preparation']=ref;d['latest_viewer_followup']=ref
        d['current_execution_state']=read(ROOT/PACKET/'WORKLIST.json')
        d['active_remaining_worklist']=dict(path=str(target),sha256=sha((ROOT/target).read_bytes()))
    write(ROOT/PATHS[2],docs[2]);link=dict(path=str(PATHS[2]),sha256=sha((ROOT/PATHS[2]).read_bytes()),status='D04_review_pending_local_native_DEV_not_independent_acceptance')
    for i in (0,1,3):docs[i]['adoption_acceptance_decision_proposal']=link;write(ROOT/PATHS[i],docs[i])
def current_indices(root,protocol,ledger):
    try:
        docs=[read(root/n)for n in PATHS];docs[:2]=[protocol,ledger]
        allowed={'current_readiness_preparation','latest_viewer_followup','current_execution_state','active_remaining_worklist','adoption_acceptance_decision_proposal'}
        for path,d in zip(PATHS,docs):
            old=json.loads(subprocess.check_output(['git','show',BASE+':'+str(path)],cwd=root))
            require(all(d.get(k)==v for k,v in old.items()if k not in allowed),'prior decision/gold changed')
            require(d['current_readiness_preparation']==reference(root)and d['latest_viewer_followup']==reference(root),'current viewer reference differs')
            target=Path('docs/spec-driven/REMAINING-13-EXECUTION.json');require(d['active_remaining_worklist']==dict(path=str(target),sha256=sha((root/target).read_bytes())),'active worklist differs')
            require(d['current_execution_state']==read(root/PACKET/'WORKLIST.json'),'current execution state differs')
        link=dict(path=str(PATHS[2]),sha256=sha((root/PATHS[2]).read_bytes()),status='D04_review_pending_local_native_DEV_not_independent_acceptance')
        require(all(d.get('adoption_acceptance_decision_proposal')==link for d in (docs[0],docs[1],docs[3])),'proposal hash link differs')
        return []
    except(ValueError,OSError,KeyError)as e:return[str(e)]
if __name__=='__main__':
    errors=validate()+current_indices(ROOT,read(ROOT/PATHS[0]),read(ROOT/PATHS[1]))
    if errors:raise SystemExit('\n'.join(errors))
    print(json.dumps(read(ROOT/PACKET/'SUMMARY.json'),ensure_ascii=False))
