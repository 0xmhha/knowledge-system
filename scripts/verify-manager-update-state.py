#!/usr/bin/env python3
"""Actual CLI/disk/kill ledger recovery; no native build is forwarded."""
import argparse
import importlib.util
import json
import os
from pathlib import Path
import select
import subprocess
import sys
import time

ROOT=Path(__file__).resolve().parents[1]
sys.path.insert(0,str(ROOT/'plugins/knowledge-manager/scripts'))
import manager
import update_contract as contract
import update_inputs as inputs
import update_state as state
from broker import Native


def main():
    parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--out',type=Path,required=True)
    out=parser.parse_args().out.resolve();manager.require(not out.exists(),'fresh_output_required');out.mkdir()
    checks,commands=[],[]
    def write(name,value):(out/name).write_text(json.dumps(value,ensure_ascii=False,indent=2)+'\n')
    def check(label,value):
        manager.require(value,label);checks.append(label);write('checks.json',checks);print('PASS: '+label,flush=True)
    def run(operation,*extra):
        argv=[sys.executable,str(ROOT/'plugins/knowledge-manager/scripts/manager.py'),operation,'--registration',str(out/'manager/registration.json'),*extra]
        result=subprocess.run(argv,capture_output=True,timeout=30)
        commands.append({'argv':argv,'exit_code':result.returncode,'stdout':result.stdout.decode(),'stderr':result.stderr.decode()});write('commands.json',commands);return result
    source=out/'source';source.mkdir();code=source/'value.go';code.write_text('package fixture\nfunc Value()int{return 7}\n');(source/'go.mod').write_text('module example.invalid/manager-state\n\ngo 1.26.9\n')
    manager.register(argparse.Namespace(source=str(source),dataset=str(out/'dataset'),state_dir=str(out/'manager'),cks_bin=str(ROOT/'bin/cks'),graph_bin=str(ROOT/'bin/ckg'),vector_bin=str(ROOT/'bin/ckv'),embedder='mock',model_name='mock',source_mode='snapshot-only',project_id=None))
    registration=str(out/'manager/registration.json');r=manager.load_registration(registration)
    policy=contract.defaults(r);policy.update(enabled=True,triggers=['manual','periodic','change']);write('policy.json',policy)
    configured=run('configure-updates','--policy-file',str(out/'policy.json'));check('actual configure CLI stores policy without starting a scheduler',configured.returncode==0 and json.loads(configured.stdout)['scheduler_running']is False)
    prepared=inputs.prepare(policy,r);write('prepared.json',prepared)
    with state.Journal(registration) as journal:
        directory=journal.directory/'bundles';directory.mkdir();setup_path=directory/'setup.json';config_path=directory/'cks.json'
        setup=manager.strict_read(Path(r['state_root'])/'setup.json');setup.update(src=prepared['input_project'],source_mode='snapshot-only',document_corpora=[str(Path(prepared['input_project'])/'.cks/knowledge/questions')]);manager.write_new(setup_path,setup)
        config=manager.strict_read(Path(r['state_root'])/'cks.json');config['maintenance']['profiles'][0].update(setup_config=str(setup_path),source_check=inputs.source_check(r,Path(journal.state['policy']['path']),prepared['input_id']));manager.write_new(config_path,config)
        native=Native(r,config_path)
        try:
            response=native.tool('cks.dataset.inspect_profile',{'schema_version':1,'project_id':r['project_id'],'profile_id':r['profile_id']});write('actual-inspection-response.json',response)
            manager.require(not response.get('isError'),'actual_inspection_failed');profile=response['structuredContent']['profile']
        finally:write('native-close.json',{'EOF_exit_code':native.close()})
        bundle={'config':str(config_path),'files':{str(p):manager.file_hash(p) for p in (setup_path,config_path)}};write('intent-inputs.json',{'input_id':prepared['input_id'],'profile':profile,'bundle':bundle})
    script='''import json,signal,sys,time
from pathlib import Path
sys.path.insert(0,sys.argv[1])
from update_state import Journal
j=Journal(sys.argv[2]);v=json.loads(Path(sys.argv[3]).read_text())
j.checking('manual',10)
row=j.intend(v['input_id'],v['bundle'],v['profile'],10)
print(json.dumps(row),flush=True)
while True:time.sleep(1)
'''
    child=subprocess.Popen([sys.executable,'-c',script,str(ROOT/'plugins/knowledge-manager/scripts'),registration,str(out/'intent-inputs.json')],stdout=subprocess.PIPE,stderr=subprocess.PIPE)
    try:
        manager.require(select.select([child.stdout],[],[],30)[0],'intent_child_timeout')
        line=child.stdout.readline();row=json.loads(line);(out/'intent-child.stdout.jsonl').write_bytes(line)
        check('actual child fsyncs exact intent before any native admission',row['job']is None and row['request']['source_pin']==profile['source_pin'])
        try:state.Journal(registration)
        except manager.Failure as error:busy=str(error)
        else:raise ValueError('second writer entered')
        check('live child holds the same exclusive broker writer lease',busy=='collection_busy')
        before=(out/'manager/updates/worker/state.json').read_bytes();(out/'before-kill-state.json').write_bytes(before)
        child.kill();code_exit=child.wait(timeout=10);(out/'intent-child.stderr.txt').write_bytes(child.stderr.read())
        write('process-exit.json',{'signal':'SIGKILL','exit_code':code_exit,'native_build_was_forwarded':False})
        check('actual SIGKILL leaves durable bytes intact',code_exit==-9 and before==(out/'manager/updates/worker/state.json').read_bytes())
        code.write_text('package fixture\nfunc Value()int{return 8}\n')
        with state.Journal(registration) as recovered:
            write('recovered.json',recovered.describe())
            check('restart recovers original intent after later live source change',recovered.row()==row and recovered.row()['request_digest']==state.request_digest(row['request']))
            refusals=[]
            for action in (lambda:recovered.checking('manual',20),lambda:recovered.configure(policy)):
                try:action()
                except manager.Failure as error:refusals.append(str(error))
            write('pending-refusals.json',refusals)
            check('unknown admission cannot become a new check or policy',refusals==['pending_update_requires_reconciliation']*2)
        refused=run('serve');check('actual reviewed launcher refuses pending update before native startup',refused.returncode==2 and json.loads(refused.stderr)['code']=='pending_update_requires_reconciliation')
        observed=run('update-status');check('actual status reports preserved pending intent without claiming execution',observed.returncode==0 and json.loads(observed.stdout)['state']['pending']==row['id'] and json.loads(observed.stdout)['native_job_executed_by_this_module']is False)
        check('no dataset or reviewed semantic approval is manufactured',not(out/'dataset').exists() and not(out/'manager/workflows').exists())
        check('original source receives only deliberate fixture edits',sorted(p.name for p in source.iterdir())==['go.mod','value.go'] and 'return 8'in code.read_text())
    finally:
        if child.poll()is None:child.kill();child.wait(timeout=10)
        child.stdout.close();child.stderr.close()
    write('summary.json',{'schema_version':1,'scope':'KM04_3_B1_durable_ledger_process_recovery_not_native_worker',
          'checks':len(checks),'actual_CLI_commands':len(commands),'actual_native_inspections':1,'actual_native_builds':0,
          'actual_killed_ledger_processes':1,'native_EOF_exit_codes':[0],'full_KM04_3_complete':False,'scheduler_runtime_verified':False,
          'original_request_recovered':True,'native_admission_replay_verified':False,'paid_model_calls':0,'independent_acceptance':False})


if __name__=='__main__':main()
