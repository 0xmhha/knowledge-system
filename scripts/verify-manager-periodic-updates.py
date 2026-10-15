#!/usr/bin/env python3
"""Final owned two-project periodic/change/deletion/failure recovery lifecycle."""
import argparse
import copy
import hashlib
import importlib.util
import json
from pathlib import Path
import subprocess
import sys
import time

ROOT=Path(__file__).resolve().parents[1]
sys.path.insert(0,str(ROOT/'plugins/knowledge-manager/scripts'))
import manager
import update_state as state


def main():
    parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--out',type=Path,required=True)
    out=parser.parse_args().out.resolve();manager.require(not out.exists(),'fresh_output_required');out.mkdir()
    checks,calls,sessions,events=[],[],[],[]
    def write(n,v):(out/n).write_text(json.dumps(v,ensure_ascii=False,indent=2)+'\n')
    def check(label,value):
        manager.require(value,label);checks.append(label);write('checks.json',checks);print('PASS: '+label,flush=True)
    spec=importlib.util.spec_from_file_location('periodic_update_wire',ROOT/'scripts/eval-value-dependencies.py');wire=importlib.util.module_from_spec(spec);spec.loader.exec_module(wire)
    def fixture(label,value):
        base=out/label;base.mkdir();source=base/'source';source.mkdir();(source/'go.mod').write_text('module example.invalid/managed-project\n\ngo 1.26.9\n')
        (source/'value.go').write_text('package fixture\nfunc Value()int{return '+str(value)+'}\n');(source/'.env').write_text('PRIVATE_PERIODIC_SENTINEL_'+label+'\n')
        graph=base/'graph-wrapper.py'
        graph.write_text('#!'+str(Path(sys.executable).resolve())+'\nimport os,sys\nfrom pathlib import Path\n'
          'if len(sys.argv)>1 and sys.argv[1]=="build" and Path('+repr(str(base/'fail-graph'))+').exists():sys.exit(23)\n'
          'os.execv('+repr(str(ROOT/'bin/ckg'))+',['+repr(str(ROOT/'bin/ckg'))+']+sys.argv[1:])\n');graph.chmod(0o700)
        manager.register(argparse.Namespace(source=str(source),dataset=str(base/'dataset'),state_dir=str(base/'manager'),cks_bin=str(ROOT/'bin/cks'),graph_bin=str(graph),vector_bin=str(ROOT/'bin/ckv'),embedder='mock',model_name='mock',project_id=None,source_mode='snapshot-only'))
        r=manager.load_registration(str(base/'manager/registration.json'))
        return {'root':base,'source':source,'r':r}
    a,b=fixture('project-a',7),fixture('project-b',101)
    (a['source']/'old.md').write_text('OLD_PROJECT_DOCUMENT_MARKER\n')
    ontology=a['source']/'.cks/ontology';ontology.mkdir(parents=True);(ontology/'terms.yaml').write_text('term: OLD_STAKING_POLICY_MARKER\nstatus: proposed\n')
    context=a['root']/'provided.json'
    def provided(text):context.write_text(json.dumps({'schema_version':1,'project_id':a['r']['project_id'],'records':[{'kind':'note','actor':'controlled-source-record','text':text}]}))
    provided('OLD_SUPPLIED_PROJECT_NOTE')
    def connect(subject,label):
        folder=out/label;folder.mkdir();client=wire.Client(None,None,folder,command=[sys.executable,str(ROOT/'plugins/knowledge-manager/scripts/manager.py'),'serve-updates','--registration',str(subject['root']/'manager/registration.json')],namespace='');sessions.append(client)
        manager.require(client.request('initialize',{'protocolVersion':'2024-11-05','capabilities':{},'clientInfo':{'name':'owned-final-periodic','version':'1'}})['result']['serverInfo']['name']=='knowledge-manager-updates','wrong mode');return client
    def tool(client,operation,args=None,error=None):
        response=client.request('tools/call',{'name':'cks.manager.update_'+operation,'arguments':args or{}})['result']
        calls.append({'session':client.root.name,'operation':operation,'arguments':copy.deepcopy(args or{}),'response':copy.deepcopy(response),'expected_error':error});write('calls.json',calls)
        if error:
            manager.require(response.get('isError')and json.loads(response['content'][0]['text'])['code']==error,'wrong actual refusal');return response
        manager.require(not response.get('isError'),'actual MCP failure: '+str(response));return response['structuredContent']
    def configure(client,subject,extra=None):
        policy=copy.deepcopy(tool(client,'policy_template'));policy.update(enabled=True,triggers=['manual','periodic','change'],interval_seconds=1,activation='activate_after_gate',deadline_seconds=120)
        policy['retry']={'max_attempts':2,'initial_seconds':2,'max_seconds':2}
        if extra:
            policy['collection']['materials']=[{'id':'terms','role':'ontology','root':str(ontology)}];policy['collection']['context_file']=str(context)
        path=subject['root']/'policy.json';path.write_text(json.dumps(policy));tool(client,'configure',{'policy_file':str(path)});return policy
    def start(client,trigger='manual',continuous=False):
        value=tool(client,'start',{'trigger':trigger,'max_iterations':10000 if continuous else 1,'max_seconds':120,'poll_seconds':.05})
        events.append({'at':time.time(),'event':'start','session':client.root.name,'trigger':trigger,'process':value['process']});write('events.json',events);return value
    def wait(client,predicate,label):
        end=time.monotonic()+70
        while True:
            status=tool(client,'status')
            if predicate(status):write(label+'.json',status);return status
            manager.require(status['process']['exit_code']is None,'worker stopped before '+label+': '+str(status['last_error']))
            manager.require(time.monotonic()<end,'periodic timeout '+label);time.sleep(.07)
    def success(client,label,prior=None):return wait(client,lambda s:s['last_success']is not None and s['last_success']['intent_id']!=prior,label)['last_success']
    def read(client,row,label,prompt='Value observed source'):
        value=tool(client,'read',{'job_id':row['job']['job_id'],'expected_coordinates':row['job']['candidate']['coordinates'],'prompt':prompt});write(label+'.json',value);return value
    def tree(path):
        manager.require(not any(p.is_symlink()for p in path.rglob('*')),'unexpected version link')
        return {p.relative_to(path).as_posix():manager.file_hash(p)for p in path.rglob('*')if p.is_file()}
    def actual_row(subject,identifier):
        value=state.read_state(subject['root']/'manager/updates/worker/state.json',subject['r']);return next(r for r in value['attempts']if r['id']==identifier)
    ca,cb=None,None
    try:
        ca,cb=connect(a,'project-a-session'),connect(b,'project-b-session')
        configure(ca,a,True);configure(cb,b);start(cb);b1=success(cb,'b1-status');bpack=read(cb,b1,'b1-read')
        wait(cb,lambda s:s['process']['running']is False,'b1-stopped-status')
        bversion=b['root']/'dataset'/b1['job']['candidate']['version'];before=tree(bversion);write('b-version-before.json',before)
        check('separate project has an actual published same-name Value implementation101',b1['outcome']=='succeeded'and any('return 101'in json.dumps(x)for x in bpack['pack']['bodies']))
        start(ca,'periodic',True);a1=success(ca,'a1-status');pack1=read(ca,a1,'a1-read');terms1=read(ca,a1,'a1-terms-read','Show the staking policy from observed material')
        row1=actual_row(a,a1['intent_id']);write('a1-attempt.json',row1)
        check('actual periodic start publishes code7 and distinct explicit material evidence',a1['outcome']=='succeeded'and any('return 7'in json.dumps(x)for x in pack1['pack']['bodies'])and any('OLD_STAKING_POLICY_MARKER'in json.dumps(x)for x in terms1['pack']['bodies']))
        check('project and dataset coordinates are distinct despite same symbol/module names',pack1['pack']['coordinates']['project_id']!=bpack['pack']['coordinates']['project_id']and pack1['pack']['coordinates']['dataset_id']!=bpack['pack']['coordinates']['dataset_id'])
        tool(ca,'read',{'job_id':b1['job']['job_id'],'expected_coordinates':b1['job']['candidate']['coordinates'],'prompt':'Value'},error='update_read_selection_required')
        check('one project cannot select another project job',True)
        (a['source']/'value.go').write_text('package fixture\nfunc Value()int{return 8}\n');(a['source']/'old.md').unlink()
        a2=success(ca,'a2-status',a1['intent_id']);pack2=read(ca,a2,'a2-read');row2=actual_row(a,a2['intent_id']);write('a2-attempt.json',row2)
        check('live periodic loop observes code8 and deletes the old source document',a2['input_id']!=a1['input_id']and any('return 8'in json.dumps(x)for x in pack2['pack']['bodies'])and not(Path(a['r']['state_root'])/'updates/inputs'/a2['input_id']/'project/old.md').exists())
        noop=wait(ca,lambda s:s['phase']=='no_op','a2-noop-status')
        check('actual timer no-op retains exactly two admitted jobs',noop['attempts_recorded']==2 and noop['last_success']['intent_id']==a2['intent_id'])
        stopped=tool(ca,'stop');write('periodic-stop.json',stopped);check('periodic worker ends on explicit owned stop',stopped['status']['process']['running']is False and stopped['status']['process']['exit_code']==0)
        (a['source']/'value.go').write_text('package fixture\nfunc Value()int{return 9}\n');(ontology/'terms.yaml').unlink();ontology.rmdir();provided('NEW_SUPPLIED_PROJECT_NOTE')
        start(ca,'change',True);a3=success(ca,'a3-status',a2['intent_id']);pack3=read(ca,a3,'a3-read');row3=actual_row(a,a3['intent_id']);write('a3-attempt.json',row3)
        check('explicit change trigger observes code9 after a second content edit',any('return 9'in json.dumps(x)for x in pack3['pack']['bodies'])and a3['input_id']!=a2['input_id'])
        capture=manager.strict_read(Path(a['r']['state_root'])/'updates/inputs'/a3['input_id']/'capture.json',16*1024*1024);write('a3-capture.json',capture)
        check('previously applied ontology root disappearance is captured as missing not re-approved',any(s['origin_id']=='material:terms'and s.get('missing')is True for s in capture['scopes'])and capture['semantic_review']=='not_performed'and capture['human_approval']=='not_inferred')
        deleted=read(ca,a3,'a3-deleted-query','OLD_STAKING_POLICY_MARKER OLD_SUPPLIED_PROJECT_NOTE OLD_PROJECT_DOCUMENT_MARKER')
        blobs=list((a['root']/'dataset'/a3['job']['candidate']['version']/'sources/blobs').glob('*'));new_data=b''.join(p.read_bytes()for p in blobs if p.is_file())
        check('latest retained blobs and actual query omit deleted old material and note',all(word not in new_data for word in(b'OLD_STAKING_POLICY_MARKER',b'OLD_SUPPLIED_PROJECT_NOTE',b'OLD_PROJECT_DOCUMENT_MARKER'))and all(word not in json.dumps(deleted['pack']['bodies'])for word in('OLD_STAKING_POLICY_MARKER','OLD_SUPPLIED_PROJECT_NOTE','OLD_PROJECT_DOCUMENT_MARKER')))
        newnote=read(ca,a3,'a3-provided-query','Value observed source staking policy')
        check('changed supplied note is actual distinct evidence in the new DB',b'NEW_SUPPLIED_PROJECT_NOTE'in new_data and any('NEW_SUPPLIED_PROJECT_NOTE'in json.dumps(x)for x in newnote['pack']['bodies']))
        old=read(ca,a1,'a1-retained-after-delete','Show the staking policy from observed material')
        check('exact v1 read preserves its removed ontology evidence',old['pack']['coordinates']==terms1['pack']['coordinates']and old['pack']['bodies']==terms1['pack']['bodies']and old['pack']['citations']==terms1['pack']['citations'])
        (a['root']/'fail-graph').write_text('controlled engine failure\n');(a['source']/'value.go').write_text('package fixture\nfunc Value()int{return 10}\n')
        failed=wait(ca,lambda s:s['last_outcome']is not None and s['last_outcome']['outcome']=='failed','failed-status');failed_row=actual_row(a,failed['last_outcome']['intent_id']);write('failed-attempt.json',failed_row)
        pointer=str((a['root']/'dataset/current').readlink());write('failed-current.json',{'version':pointer,'last_success':failed['last_success']})
        check('actual engine failure keeps current and last successful v3',failed_row['job']['activated']is False and pointer==a3['job']['candidate']['version']and failed['last_success']['intent_id']==a3['intent_id'])
        (a['root']/'fail-graph').unlink();old3=read(ca,a3,'a3-read-after-failure')
        check('old code9 snapshot remains queryable across the failure',any('return 9'in json.dumps(x)for x in old3['pack']['bodies']))
        a4=success(ca,'a4-status',a3['intent_id']);pack4=read(ca,a4,'a4-read');row4=actual_row(a,a4['intent_id']);write('a4-attempt.json',row4)
        check('same change worker recovers with linked bounded retry and actual code10 publication',row4['previous_attempt']==failed_row['id']and row4['input_id']==failed_row['input_id']and row4['attempt']==2 and row4['outcome']=='succeeded'and any('return 10'in json.dumps(x)for x in pack4['pack']['bodies'])and pack4['current_selected_at_pointer_checks']is True)
        final_noop=wait(ca,lambda s:s['phase']=='no_op','final-noop-status');check('no-op after recovery adds no duplicate sixth job',final_noop['attempts_recorded']==5 and final_noop['last_success']['intent_id']==a4['intent_id'])
        after=tree(bversion);write('b-version-after.json',after);bafter=read(cb,b1,'b1-read-after-a-updates')
        check('other project immutable version bytes current and actual bodies citations are unchanged',before==after and str((b['root']/'dataset/current').readlink())==b1['job']['candidate']['version']and bafter['pack']['bodies']==bpack['pack']['bodies']and bafter['pack']['citations']==bpack['pack']['citations'])
        oldcode=read(ca,a1,'a1-code-after-all');check('old code7 remains distinct from current code10 and project-b code101',any('return 7'in json.dumps(x)for x in oldcode['pack']['bodies'])and oldcode['pack']['coordinates']!=pack4['pack']['coordinates']and oldcode['pack']['coordinates']!=bafter['pack']['coordinates'])
        allblobs=b''.join(p.read_bytes()for subject in(a,b)for p in(subject['root']/'dataset').glob('*/sources/blobs/*')if p.is_file())
        check('private source is excluded and no semantic review or source documentation is manufactured',b'PRIVATE_PERIODIC_SENTINEL'not in allblobs and not(a['root']/'manager/workflows').exists()and not(b['root']/'manager/workflows').exists()and not(a['source']/'.cks/knowledge').exists()and not(b['source']/'.cks').exists())
        write('final-a-status.json',tool(ca,'stop'));write('final-b-status.json',tool(cb,'stop'))
        for client in sessions:client.close();client.proc.stdout.close()
        check('both actual MCP sessions close with EOF0',all(c.proc.returncode==0 for c in sessions))
    finally:
        for client in sessions:
            if client.proc.poll()is None:client.close();client.proc.stdout.close()
    write('summary.json',{'schema_version':1,'scope':'KM04_4_final_actual_two_project_periodic_change_lifecycle_not_quality',
        'checks':len(checks),'actual_tool_calls':len(calls),'actual_MCP_sessions':2,'EOF_exit_codes':[c.proc.returncode for c in sessions],
        'projects':2,'project_a_build_jobs':5,'project_a_publications':4,'intentional_engine_failures':1,'project_b_build_jobs':1,
        'triggers_observed':['manual','periodic','change'],'live_content_changes':3,'explicit_material_root_deleted':True,'supplied_note_changed':True,
        'old_snapshot_and_other_project_preserved':True,'no_op_verified':True,'full_KM04_complete':True,
        'paid_model_calls':0,'mock_embedding_not_quality':True,'independent_acceptance':False})


if __name__=='__main__':main()
