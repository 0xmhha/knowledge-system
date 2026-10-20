"""Verify local CoreML transport evidence, keeping real-model acceptance open."""
import copy
import json
import subprocess
from pathlib import Path
import viewer_readiness as v
import public_v2_readiness as previous

ROOT=v.ROOT
PACKET=Path('docs/spec-driven/operational-preparation/source-review-supplement/followups/coreml-transport-v1')
BASE='75749926fdd5b1119881915fd6c7ff62015736c2'
KIND='CoreML_local_native_transport_CLI_default_regression_DEV_not_real_model_acceptance'
TOOLS=['scripts/coreml_readiness.py','scripts/test_coreml_readiness.py','scripts/check-operational-acceptance.py','scripts/viewer_readiness.py','scripts/public_v2_readiness.py','scripts/run-wi4-target-subscription.py','scripts/verify-wi3-reasons-native.py']

def boundary(d):
    v.require(all(d.get(k)==0 for k in ('actual_CoreML_model_inference_calls','actual_LLM_calls','paid_external_calls','formal_completed','independent_approved')),'fixture evidence promoted to model/formal evaluation')
    v.require(d.get('real_CoreML_model_accepted')is False and d.get('Qwen_CoreML_supported')is False and d.get('managed_CKS_CoreML_runtime_implemented')is False,'unimplemented model/runtime accepted')

def command(folder,expected=0):
    r=v.read(folder/'receipt.json.gz')
    v.require(r['exit_code']==expected and r['reaped']and not r['timeout']and not r['term_sent']and not r['kill_sent']and not r['remaining_live_group_members'],'owned command failed/was not reaped')
    for stream in ('stdout','stderr'):v.require(v.sha(v.raw(folder/(stream+'.gz')))==r[stream+'_sha256'],'command stream differs')
    return r

def derive(p):
    b=v.read(p/'source-binding.json');v.require(b['base_revision']==BASE,'native base differs')
    parent=p.parent/'public-v2-surface-v1';stored=subprocess.check_output(['git','show',BASE+':'+str((parent/'manifest.json').relative_to(ROOT))],cwd=ROOT)
    v.require(v.sha(stored)==v.sha((parent/'manifest.json').read_bytes()),'parent packet manifest changed')
    pm=v.read(parent/'manifest.json');v.require(pm['artifacts']==v.inventory(parent),'parent evidence changed')
    for n,h in pm['tools'].items():v.require(v.sha(subprocess.check_output(['git','show',BASE+':'+n],cwd=ROOT))==h,'parent verifier at committed source differs')
    v.require(v.read(parent/'SUMMARY.json')==previous.derive(parent),'historical parent result changed')
    receipts=v.read(p/'native/receipts.json.gz');v.require([r['label']for r in receipts]==['build','test','race','lint','build-bins','build-runtime','build-info'],'whole native command population incomplete')
    result={}
    for r in receipts:
        v.require(r['exit_code']==0 and r['reaped']and not r['timeout']and not r['term_sent']and not r['kill_sent']and r['source_head']==BASE and r['source_binding_sha256']==v.sha((p/'source-binding.json').read_bytes()),'whole native command/input identity failed')
        streams={s:v.raw(p/'native'/(r['label']+'.'+s+'.gz'))for s in ('stdout','stderr')}
        v.require(all(v.sha(streams[s])==r[s+'_sha256']for s in streams),'native log differs')
        if r['label']in ('test','race'):
            expected=['go','test']+(['-race']if r['label']=='race'else[])+['-json','-count=1','-timeout='+('20m'if r['label']=='race'else'18m'),'./...']
            v.require(r['argv']==expected,'native test population narrowed')
            result[r['label']]=previous.native_population(v.events(streams['stdout']),v.events(v.raw(parent/'native'/(r['label']+'.stdout.gz'))))
            v.require(r['tests']==result[r['label']],'native counts differ')
        if r['label']=='lint':v.require(all(x in streams['stdout'].decode()for x in ('0 issues.','go vet ./...','documentation: OK')),'full fmt/vet/lint/docs missing')
    v.process_groups(receipts,v.read(p/'native/process-groups.json.gz'))
    reused=v.read(p/'native/REUSED-PASSED-COMMANDS.json.gz');v.require(reused['source_binding_unchanged']and reused['reused_labels']==['build','test'],'passed command reuse scope changed')
    initial=v.read(p/'failed-native/receipts.json.gz');v.require([(r['label'],r['exit_code'])for r in initial]==[('build',0),('test',0),('race',1)],'initial race failure hidden')
    v.require(receipts[:2]==initial[:2] and v.read(p/'native/source-binding.json.gz')==v.read(p/'failed-native/source-binding.json.gz')==b,'passed command reuse changed receipts or source inputs')
    failed_rows=v.events(v.raw(p/'failed-native/race.stdout.gz'));failed=[r for r in failed_rows if r.get('Action')=='fail'and r.get('Test')]
    v.require([(r['Package'],r['Test'])for r in failed]==[('github.com/0xmhha/knowledge-system/internal/setup','TestGuardianAbsoluteDeadlineWorksWhileOwnerSocketStaysOpen')],'unattributed additional race failure')
    for r in initial:
        for stream in ('stdout','stderr'):v.require(v.sha(v.raw(p/'failed-native'/(r['label']+'.'+stream+'.gz')))==r[stream+'_sha256'],'initial native raw failure changed')
    command(p/'guardian-focus')
    focus=v.counts(v.events(v.raw(p/'guardian-focus/stdout.gz')));v.require(focus['pass']==20 and focus['fail']==focus['skip']==0,'existing guardian repetition differs')
    command(p/'guardian-diagnostic/run')
    diagnostic=v.counts(v.events(v.raw(p/'guardian-diagnostic/run/stdout.gz')));v.require(diagnostic['pass']==80 and diagnostic['fail']==diagnostic['skip']==0,'guardian diagnostic repetition differs')

    tools=v.read(p/'native/TOOLS-BEFORE-LINT.json.gz');after=v.read(p/'native/installed-tools.json.gz')
    v.require(tools['capture_before_lint']and tools['go'].startswith('go version go1.26.8 ')and 'version 2.14.0 'in tools['lint']and all(tools[n]==after[n]for n in ('go','lint')),'native tools changed')
    v.require(tools['lint_configuration_sha256']==v.sha((ROOT/'.golangci.yml').read_bytes()),'lint configuration changed')
    packages=v.read(p/'native/package-boundary-proof.json.gz');v.require(packages['previous']==packages['current']and not packages['removed_external_only'],'maintained package population changed')
    binaries={'cks':v.read(p/'native/binary.json.gz')['sha256']}
    for name,exe in [('graph','ckg'),('vector','ckv')]:
        for label in ('build-'+name+'-runtime',name+'-build-info'):
            r=v.read(p/'native'/(label+'-receipt.json.gz'));v.require(r['exit_code']==0 and r['reaped']and not r['remaining_live_group_members']and r['source_head']==BASE,'extra native binary command failed')
        binaries[exe]=v.read(p/'native'/(name+'-binary.json.gz'))['sha256']
    v.require(v.read(p/'candidate/source-binding.json.gz')==b,'candidate/default native inputs differ')
    v.require(binaries=={n:v.read(p/'candidate'/(n+'-binary.json.gz'))['sha256']for n in binaries},'frozen candidate/default binaries differ')
    component=p/'native-component';scope=v.read(component/'SCOPE.json.gz');v.require(all(b['source_sha256'][n]==h for n,h in scope['source_sha256'].items()),'native C component source differs')
    cresults={}
    for mode in ('MRC','ARC'):
        for label in ('compile','run'):command(component/(mode+'-'+label))
        d=v.read(component/(mode+'-run/stdout.gz'));v.require(d['tokens']==[101,102,103]and d['mask']==[1,1,1]and d['float32_double_float16_and_strided']and d['invalid_output_refused']and d['models_released']==d['fixture_predictions']==2 and d['actual_model_inference_calls']==0,'native C fixture failed/promoted')
        cresults[mode]=d
    tagged=p/'tagged';tagScope=v.read(tagged/'SCOPE.json.gz');tagResult={}
    v.require(scope['SDK']==tagScope['SDK'],'native/Go profile SDK differs')
    for label in ('general','race','build','lint','cgo-off','CLI-qwen-refusal'):
        command(tagged/label,1 if label=='CLI-qwen-refusal'else 0)
        if label in ('general','race','cgo-off'):
            rows=v.events(v.raw(tagged/label/'stdout.gz'));counts=v.counts(rows);v.require(counts['fail']==0 and counts['pass']==(3 if label=='cgo-off'else 28)and counts['skip']==(0 if label=='cgo-off'else 1),'tagged/disabled test population differs');v.require(not any(x.get('Action')=='fail'for x in rows),'tagged package failed');tagResult[label]={k:counts[k]for k in ('pass','fail','skip')}
        if label=='lint':v.require('0 issues.'in v.raw(tagged/label/'stdout.gz').decode(),'tagged lint did not pass')
    v.require('not supported by the hidden-state/CLS profile'in v.raw(tagged/'CLI-qwen-refusal/stderr.gz').decode(),'native CLI accepted Ollama-only Qwen')
    dep=v.read(p/'dependency/RECEIPT.json.gz');v.require(dep['tag']=='v1.27.0'and dep['GitHub_declared_digest_matches']and not dep['global_install']and dep['model_downloads']==0,'optional native dependency scope changed')
    compare=p/'BGE-compat';cleanup=v.read(compare/'cleanup.json.gz');compat=v.read(p/'BGE-analysis/RESULT.json.gz')
    v.require(cleanup['failure']['type']=='TypeError'and cleanup['failure']['message']=="unsupported operand type(s) for -: 'dict' and 'dict'"and not cleanup['proxy_thread_live']and cleanup['payload_preserved']and v.read(compare/'payload-before.json.gz')==v.read(compare/'payload-after.json.gz'),'BGE comparison cleanup/data failed')
    for label in ('old-first','old-repeat','new'):command(compare/label)
    v.require(compat['old_repeat_same']and compat['new_same_order_citations_snippets']and compat['hits']>0 and compat['model']=='bge-m3:latest'and compat['embedding_dim']==1024,'default BGE compatibility failed')
    clarification=v.read(p/'BGE-analysis/CLARIFICATION.json.gz');v.require(clarification['new_model_calls']==0 and clarification['raw_runtime_not_modified'],'BGE failure hidden by extra calls')
    v.require(clarification['original_runtime_artifact_sha256']=={n.removesuffix('.gz'):v.sha(v.raw(compare/n))for n in v.inventory(compare)},'BGE runtime bytes changed after offline analysis')
    cs=v.read(compare/'SCOPE.json.gz');v.require(cs['new_binary_sha256']==binaries['ckv']and cs['new_source_binding_sha256']==v.sha((p/'source-binding.json').read_bytes()),'actual default CKV candidate differs')
    http=v.read(compare/'HTTP.json.gz');v.require(all(x['status']==200 and x['path']in ('/api/tags','/api/embed')for x in http),'unplanned model/provider path')
    emb=[x for x in http if x['path']=='/api/embed']
    d=dict(kind=KIND,native=result,native_source_inputs=len(b['source_sha256']),Go_files=sum(n.endswith('.go')for n in b['source_sha256']),changed_Go_files=sum(n.endswith('.go')for n in b['changed_native_files']),component=cresults,tagged=tagResult,SDK=scope['SDK'],BGE_compat=compat,local_BGE_HTTP_requests=len(emb),local_BGE_input_items=sum(x['inputs']for x in emb),actual_CoreML_model_inference_calls=0,actual_LLM_calls=0,paid_external_calls=0,formal_completed=0,independent_approved=0,real_CoreML_model_accepted=False,Qwen_CoreML_supported=False,managed_CKS_CoreML_runtime_implemented=False,compiled_package_positive_cleanup_verified=False,model_downloads=0,global_install=False,initial_default_race_failures=1,guardian_original_repetitions_passed=20,guardian_overlay_diagnostic_repetitions_passed=80,guardian_intermittence_resolved=False,default_passed_build_general_reused_same_source=True)
    boundary(d);return d

def validate(root=ROOT,p=None):
    p=p or root/PACKET
    try:
        manifest=v.read(p/'manifest.json');v.require(manifest['artifacts']==v.inventory(p),'CoreML artifact population/bytes changed');v.require(manifest['tools']=={n:v.sha((root/n).read_bytes())for n in TOOLS},'CoreML verifier tools changed')
        b=v.read(p/'source-binding.json');current=v.read(p/'current-source-binding.json');expected=copy.deepcopy(b)
        tagged=v.read(p/'tagged/SCOPE.json.gz')['source_sha256'];v.require(all(b['source_sha256'].get(n,h)==h for n,h in tagged.items()),'native/default profile source overlap differs');expected['source_sha256'].update(tagged);expected['source_sha256'].update({n:v.sha((root/n).read_bytes())for n in TOOLS});expected['native_source_binding_sha256']=v.sha((p/'source-binding.json').read_bytes());v.require(expected==current,'current CoreML binding differs')
        for n,h in{**current['source_sha256'],**current['approved_input_sha256']}.items():v.require(v.sha((root/n).read_bytes())==h,'current input changed: '+n)
        names=subprocess.check_output(['git','ls-files','--cached','--others','--exclude-standard','*.go'],cwd=root,text=True).splitlines();v.require({n for n in names if n.startswith(('cmd/','internal/','pkg/'))}=={n for n in b['source_sha256']if n.endswith('.go')and n.startswith(('cmd/','internal/','pkg/'))},'Go inventory changed')
        app=root/'internal/system/viewer/web_assets/app'
        if app.exists():v.require({str(f.relative_to(root)):v.sha(f.read_bytes())for f in app.rglob('*')if f.is_file()}==b['generated_input_sha256'],'optional UI payload changed')
        v.require(v.read(p/'SUMMARY.json')==derive(p),'CoreML summary changed/promoted');return []
    except(ValueError,OSError,KeyError,TypeError)as e:return[str(e)]

def reference(root=ROOT):return dict(kind=KIND,manifest=str(PACKET/'manifest.json'),manifest_sha256=v.sha((root/PACKET/'manifest.json').read_bytes()))

def current_indices(root,protocol,ledger):
    try:
        ref=reference(root);path=Path('docs/spec-driven/REMAINING-13-EXECUTION.json');work=dict(path=str(path),sha256=v.sha((root/path).read_bytes()));state=v.read(root/PACKET/'WORKLIST.json');docs=[v.read(root/n)for n in v.PATHS];docs[:2]=[protocol,ledger]
        allowed={'current_readiness_preparation','latest_coreml_followup','current_execution_state','active_remaining_worklist','adoption_acceptance_decision_proposal'}
        for name,d in zip(v.PATHS,docs):
            old=json.loads(subprocess.check_output(['git','show',BASE+':'+str(name)],cwd=root));v.require(all(d.get(k)==value for k,value in old.items()if k not in allowed),'prior decision/gold changed')
            v.require(d['current_readiness_preparation']==d['latest_coreml_followup']==ref and d['active_remaining_worklist']==work and d['current_execution_state']==state,'current CoreML reference/state differs')
        link=dict(path=str(v.PATHS[2]),sha256=v.sha((root/v.PATHS[2]).read_bytes()),status='D04_review_pending_local_native_DEV_not_independent_acceptance');v.require(all(d.get('adoption_acceptance_decision_proposal')==link for d in (docs[0],docs[1],docs[3])),'proposal hash link differs')
        return []
    except(ValueError,OSError,KeyError,TypeError)as e:return[str(e)]
