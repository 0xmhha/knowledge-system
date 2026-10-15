#!/usr/bin/env python3
"""KM03.4 A: actual public build and retained reviewed-document reads, no broker yet."""
import argparse
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys
import time

import yaml

ROOT = Path(__file__).resolve().parents[1]
MANAGER = ROOT/'plugins/knowledge-manager/scripts/manager.py'


def require(value,message):
    if not value:
        raise ValueError(message)


def sha(data):
    return hashlib.sha256(data).hexdigest()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--out',required=True,type=Path)
    out = parser.parse_args().out.resolve()
    require(not out.exists(),'fresh owned output required')
    out.mkdir()
    checks,commands,calls,observations,sessions = [],[],[],[],[]
    def write(name,value):
        (out/name).write_text(json.dumps(value,ensure_ascii=False,indent=2)+'\n')
    def checked(label,value):
        require(value,label)
        checks.append(label)
        write('checks.json',checks)
        print('PASS: '+label,flush=True)
    def run(label,args):
        p = subprocess.run([str(a)for a in args],cwd=ROOT,capture_output=True,timeout=120,env=dict(os.environ,PYTHONDONTWRITEBYTECODE='1'))
        (out/(label+'.stdout.txt')).write_bytes(p.stdout)
        (out/(label+'.stderr.txt')).write_bytes(p.stderr)
        commands.append({'label':label,'command':[str(a)for a in args],'exit_code':p.returncode})
        write('commands.json',commands)
        require(p.returncode==0,'actual command failed: '+label)
        return p.stdout
    prepared = out/'prepared'
    run('prepare-reviewed-input',[sys.executable,ROOT/'scripts/verify-knowledge-manager-review.py','--out',prepared])
    initial = json.loads((prepared/'adoption-result.json').read_text())
    state = json.loads((prepared/'review-status.stdout.txt').read_text())
    response = json.loads((prepared/'review-input.json').read_text())
    response['expected_revision']=state['revision']
    response['decisions'][0]['reason']='New controlled document-index test response after the prior withdrawal.'
    write('review-input.json',response)
    registration = prepared/'manager-a/registration.json'
    reviewed = json.loads(run('review-current',[sys.executable,MANAGER,'review','--registration',registration,'--review-file',out/'review-input.json']))
    adopted = json.loads(run('adopt-current',[sys.executable,MANAGER,'adopt','--registration',registration,'--draft-id',response['draft_id'],'--review-revision',reviewed['review_revision']]))
    write('adoption.json',adopted)
    source = Path(adopted['input_project'])
    checked('start from actual manager-generated reviewed filtered input',source.is_dir()and not(source/'.env').exists()and not(source/'private').exists()and adopted['adoption_id']!=initial['adoption_id'])
    source_before = {p.relative_to(source).as_posix():sha(p.read_bytes())for p in source.rglob('*')if p.is_file()}
    write('input-before.json',source_before)
    setup = json.loads((prepared/'manager-a/setup.json').read_text())
    setup.update(src=str(source),source_mode='snapshot-only',document_corpora=[str(source/'.cks/knowledge/questions')],build_timeout='120s')
    setup_path = out/'setup.json'
    write('setup.json',setup)
    config = json.loads((prepared/'manager-a/cks.json').read_text())
    profile = config['maintenance']['profiles'][0]
    profile.update(setup_config=str(setup_path),allowed_activation=['hold_for_review'],timeout_seconds=120)
    write('maintenance.json',config)
    spec = importlib.util.spec_from_file_location('document_index_client',ROOT/'scripts/eval-value-dependencies.py')
    lib = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(lib)
    dataset = Path(setup['out'])
    binary = ROOT/'bin/cks'
    def connect(label,path):
        folder = out/label
        folder.mkdir()
        client = lib.Client(None,None,folder,command=[str(binary),'mcp','--config',str(path)])
        sessions.append(client)
        reply = client.request('initialize',{'protocolVersion':'2024-11-05','capabilities':{},'clientInfo':{'name':'owned-reviewed-doc-index','version':'1'}})
        require('error'not in reply,'actual MCP initialization failed')
        return client
    def tool(client,name,args):
        response = client.request('tools/call',{'name':name,'arguments':args})
        require('error'not in response and not response['result'].get('isError'),'actual tool failed: '+str(response))
        calls.append({'session':client.root.name,'tool':name,'arguments':args,'response':response['result']})
        write('calls.json',calls)
        return response['result']['structuredContent']
    try:
        maintenance = connect('maintenance-session',out/'maintenance.json')
        inspected = tool(maintenance,'cks.dataset.inspect_profile',{'schema_version':1,'project_id':profile['project_id'],'profile_id':profile['id']})
        write('inspection.json',inspected)
        p = inspected['profile']
        checked('actual registered profile pins source and reviewed knowledge inputs','knowledge'in p['required_gates']and inspected['capabilities']['job_persistence']=='durable_journal')
        request = {'schema_version':1,'project_id':profile['project_id'],'profile_id':profile['id'],
                   **{key:p[key]for key in ('profile_digest','source_pin','input_digest','embedding_digest')},'base':p['observed_base'],
                   'new_version':'reviewed-doc-v1','activation':'hold_for_review','idempotency_key':'owned-reviewed-doc-v1','deadline_seconds':120}
        write('build-request.json',request)
        job = tool(maintenance,'cks.dataset.build',request)
        ref = {'schema_version':1,**{key:job[key]for key in ('project_id','profile_id','job_id')}}
        deadline = time.monotonic()+120
        while True:
            observations.append(job)
            write('observations.json',observations)
            if job['terminal']:
                break
            require(time.monotonic()<deadline,'actual document build timed out')
            time.sleep(.05)
            job = tool(maintenance,'cks.dataset.status',ref)
        write('job.json',job)
        checked('actual public build holds candidate with every required gate passed',job['state']=='held_for_review'and job['activated']is False and all(job['gates'][gate]=='passed'for gate in p['required_gates']))
        checked('held document candidate never creates a current publication',not(dataset/'current').exists()and not(dataset/'current').is_symlink())
        version = dataset/request['new_version']
        vector = json.loads((version/'vector/manifest.json').read_text())
        write('vector-manifest.json',vector)
        checked('actual CKV index contains selected notes and Markdown chunks',vector['docs_roots']==setup['document_corpora']and vector['languages']['markdown']>0)
        retained = json.loads((version/'sources/manifest.json').read_text())
        write('retained-manifest.json',retained)
        note = source/'.cks/knowledge/questions/manager-reviewed.md'
        note_bytes = note.read_bytes()
        files = retained['files']
        matches = [row for row in files if row['origin_id']=='docs:corpus.0'and row['path']=='manager-reviewed.md']
        checked('reviewed note is retained under a distinct docs origin with exact bytes',len(matches)==1 and matches[0]['sha256']==sha(note_bytes)and (version/'sources/blobs'/matches[0]['sha256']).read_bytes()==note_bytes)
        retained_bytes = b''.join(file.read_bytes()for file in(version/'sources/blobs').iterdir()if file.is_file())
        checked('private original inputs never reenter retained dataset',b'PRIVATE_INPUT_SENTINEL'not in retained_bytes and b'PRIVATE_REASON_SENTINEL'not in retained_bytes)
        reader_path = out/'reader.yaml'
        run('gen-reader',[binary,'mcp','gen-config','--dataset-dir',version,'--source-root',source,'--embed-model',vector['embedding_model'],
                         '--sanitize-rules',prepared/'manager-a/sanitization_rules.yaml','--out',reader_path])
        reader_config = yaml.safe_load(reader_path.read_text())
        reader_config['backends']['ckv']['provider']='mock'
        reader_config['listen'].update(transport='stdio',mcp_stdio=True)
        reader_config['logging']['level']='error'
        reader_path.write_text(yaml.safe_dump(reader_config))
        reader = connect('reader-session',reader_path)
        pack = tool(reader,'cks.context.get_for_task_v2',{'prompt':'bounded queue fixture owner reports'})
        write('document-read.json',pack)
        checked('actual reader returns reviewed document body and docs-origin citation',any('bounded queue'in json.dumps(body)for body in pack['bodies'])and any(c['origin_id']=='docs:corpus.0'for c in pack['citations']))
        checked('read coordinates match exact held job and project',pack['coordinates']==job['candidate']['coordinates']and all(c['project_id']==profile['project_id']and c['dataset_id']==pack['coordinates']['dataset_id']for c in pack['citations']))
        code = source/'value.go'
        code_bytes = code.read_bytes()
        code.write_text('package fixture\nfunc Value()int{return 999}\n')
        note.unlink()
        retained_read = tool(reader,'cks.context.get_for_task_v2',{'prompt':'bounded queue fixture owner reports'})
        write('document-read-after-live-deletion.json',retained_read)
        checked('same reader preserves archived review after live input deletion',retained_read['coordinates']==pack['coordinates']and retained_read['bodies']==pack['bodies']and retained_read['citations']==pack['citations'])
        note.write_bytes(note_bytes)
        code.write_bytes(code_bytes)
        checked('original filtered input bytes are restored for repeatable evidence',source_before=={p.relative_to(source).as_posix():sha(p.read_bytes())for p in source.rglob('*')if p.is_file()})
        reader.close()
        maintenance.close()
        restarted = connect('maintenance-restart-session',out/'maintenance.json')
        repeated = tool(restarted,'cks.dataset.build',request)
        checked('durable replay preserves held candidate without rebuilding',repeated==job)
        restarted.close()
        checked('all actual MCP sessions terminate on EOF without error',len(sessions)==3 and all(c.proc.returncode==0 for c in sessions))
        original = {str(file.relative_to(prepared)):sha(file.read_bytes())for name in ('source-a','source-b')for file in(prepared/name).rglob('*')if file.is_file()}
        checked('original project preserves existing files without a manager overlay',original==json.loads((prepared/'source-before.json').read_text())and not(prepared/'source-a/.cks').exists())
    finally:
        for client in sessions:
            if client.proc.poll()is None:
                try:
                    client.close()
                except Exception:
                    client.proc.kill()
                    client.proc.wait(timeout=10)
                    raise
    summary = {'schema_version':1,'scope':'KM03_4_A_actual_reviewed_documents_public_build_and_retained_reader_only','checks':len(checks),
               'MCP_sessions':len(sessions),'EOF_exit_codes':[c.proc.returncode for c in sessions],'paid_model_calls':0,'embedding':'mock_lifecycle_not_quality',
               'candidate_state':'held_for_review','DB_published':False,'manager_reader_broker_verified':False,'actual_Claude_host_workflow_verified':False,
               'KM03_4_complete':False,'full_KM03_complete':False,'independent_acceptance':False}
    write('summary.json',summary)
    print(json.dumps(summary))


if __name__=='__main__':
    main()
