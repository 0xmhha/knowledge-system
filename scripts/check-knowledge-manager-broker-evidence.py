#!/usr/bin/env python3
"""Validate the real scoped broker, not an actual Claude host/model claim."""
import io
import json
import subprocess
from pathlib import Path, PurePosixPath
import tarfile

from knowledge_evidence import load,raw,require,sha,verify_packet,unchanged_document_index_go

ROOT = Path(__file__).resolve().parents[1]
PACKET = ROOT/'system/eval/knowledge-roadmap-2026-10-15/km-03-broker'


def digest(domain,value):
    return sha(domain.encode()+b'\0'+json.dumps(value,sort_keys=True,separators=(',', ':'),ensure_ascii=False).encode())


def check(folder=PACKET,root=ROOT):
    try:
        manifest,_ = verify_packet(folder,root)
        require(manifest['checkpoint']=='KM03_4_B_adoption_bound_manager_tools_and_actual_reader'and manifest['broker_checkpoint_complete']is True
                and all(manifest[n]is False for n in ('KM03_4_complete','full_KM03_complete','independent_acceptance')),'broker scope inflated')
        validation = load(folder,'validation.json')
        roadmap = unchanged_document_index_go(root,validation)
        require(validation['broker_tests_passed']==29 and validation['collection_tests_passed']==23 and validation['registration_tests_passed']==14
                and all(v==0 for v in validation['gates'].values())and 'documentation: OK'in raw(folder,'lint.txt').decode(),'broker gates missing')
        for name,count in [('broker',29),('collection',23),('registration',14)]:
            text = raw(folder,name+'-tests.txt').decode()
            require('Ran '+str(count)+' tests'in text and '\nOK\n'in text,'unit proof differs')
        summary = load(folder,'runtime/summary.json')
        require(summary['scope']=='KM03_4_B_actual_plugin_launcher_and_adoption_bound_maintenance_reader_only'and summary['checks']==26
                and summary['MCP_sessions']==3 and summary['EOF_exit_codes']==[0,0,0]and summary['paid_model_calls']==0
                and summary['manager_reader_broker_verified']is True and summary['embedding']=='mock_lifecycle_not_quality'and summary['candidate_state']=='held_for_review'
                and all(summary[n]is False for n in ('DB_published','actual_Claude_host_workflow_verified','KM03_4_complete','full_KM03_complete','independent_acceptance')),'broker inflated to publication/Claude/model quality')
        checks = load(folder,'runtime/checks.json')
        require(len(checks)==len(set(checks))==26,'broker runtime case missing')
        commands = load(folder,'runtime/commands.json')
        require(len(commands)==1 and commands[0]['exit_code']==0,'actual source preparation missing')
        calls = load(folder,'runtime/calls.json')
        wire = []
        for label in ('manager-session','manager-restart-session','manager-retired-restart-session'):
            ins = [json.loads(line)for line in raw(folder,'runtime/'+label+'/mcp.stdin.jsonl').splitlines()]
            outs = [json.loads(line)for line in raw(folder,'runtime/'+label+'/mcp.stdout.jsonl').splitlines()]
            replies = {r['id']:r for r in outs}
            require(len(ins)==len(outs)==len(replies)and all('error'not in r for r in outs),'outer MCP response missing/error')
            for row in ins:
                if row['method']=='tools/call':
                    wire.append({'session':label,'tool':row['params']['name'],'arguments':row['params']['arguments'],'response':replies[row['id']]['result']})
        require(wire==[{k:v for k,v in r.items()if k!='expected_error'}for r in calls],'broker observations not actual wire')
        for call in calls:
            if call['expected_error']:
                require(call['response'].get('isError')is True,'broker refusal hidden')
                if call['expected_error']is not True:
                    error = json.loads(call['response']['content'][0]['text'])
                    require(error['code']==call['expected_error'],'different refusal reason')
            else:
                require(not call['response'].get('isError'),'positive broker call failed')
        tools = load(folder,'runtime/tools.json')
        require(len(tools)==len({r['name']for r in tools})==18 and not any(t['name'].startswith('cks.ops.')for t in tools)
                and {'cks.dataset.build','cks.manager.select_input','cks.manager.resume_build','cks.context.get_for_task_v2'}<={t['name']for t in tools},'legacy/dummy/unscoped tool exposed')
        for label in ('manifest','skills','agents'):
            report = load(folder,'native/'+label+'.stdout.json')
            require(report['success']is True and report['strict']is True,'native package validation differs')
        job = load(folder,'runtime/job.json')
        profile = load(folder,'runtime/inspection.json')['profile']
        require(job['terminal']is True and job['state']=='held_for_review'and job['activated']is False and job['publication']is None
                and profile['allowed_activation']==['hold_for_review']and all(job['gates'][g]=='passed'for g in profile['required_gates']),'held job/gates or activation policy changed')
        request = load(folder,'runtime/request.json')
        require(job['request_digest']==digest('cks.dataset-build.v1',request),'actual request digest differs')
        replay = [r for r in calls if r['tool']=='cks.manager.resume_build'and not r['expected_error']]
        require(len(replay)==2 and all(r['arguments']['original_request']==request and r['arguments']['job_id']==job['job_id']and r['response']['structuredContent']==job for r in replay),'known-job replay admitted changed intent')
        first = load(folder,'runtime/read-first.json')
        restart = load(folder,'runtime/read-restart.json')
        require(first['coordinates']==restart['coordinates']==job['candidate']['coordinates']and first['bodies']==restart['bodies']and first['citations']==restart['citations'],'restart changed dataset/retained evidence')
        successful_reads = [r['response']['structuredContent']for r in calls if r['tool']=='cks.context.get_for_task_v2'and not r['expected_error']]
        require(len(successful_reads)==3 and successful_reads[0]==first and successful_reads[1]==restart,'actual read result substituted')
        require(first['citations']and any(c['origin_id']=='docs:corpus.0'for c in first['citations'])and all(c['project_id']==job['project_id']and c['dataset_id']==first['coordinates']['dataset_id']for c in first['citations'])
                and any('bounded queue'in json.dumps(body)for body in first['bodies']),'actual reviewed body/docs citation missing or foreign')
        observed = load(folder,'runtime/state-after-read.json')
        require(observed['last_validated_read']['coordinates']==first['coordinates']and observed['last_validated_read']['candidate_publication_inferred']is False
                and observed['selection_rechecked_on_use']is True and observed['actual_Claude_host_workflow_verified']is False,'past read promoted to publication/host/current freshness')
        require(load(folder,'runtime/corruption-refusal.json').get('isError')is True,'open reader ignored dataset corruption')
        with tarfile.open(fileobj=io.BytesIO(raw(folder,'fixture.tar')))as archive:
            files = {}
            for item in archive:
                p = PurePosixPath(item.name)
                require(not p.is_absolute()and '..'not in p.parts and item.name not in files and (item.isfile()or item.isdir()),'unsafe broker fixture')
                files[item.name] = archive.extractfile(item).read()if item.isfile()else None
        r = json.loads(files['prepared/manager-a/registration.json'])['registration']
        require(r['project_id']==job['project_id']==profile['project_id']and r['profile_id']==job['profile_id']==profile['profile_id'],'registration/job/profile identity mismatch')
        adoption = load(folder,'runtime/adoption.json')
        aid = adoption['adoption_id']
        envelope = json.loads(files['prepared/manager-a/broker/state.json'])
        state = envelope['state']
        require(envelope['checksum']==digest('cks.manager-broker-state.v1',state)and state['project_id']==r['project_id']and state['registration_token']==r['token']
                and state['selected_input']==aid and state['reader_selection']['expected_coordinates']==job['candidate']['coordinates']and state['pending_build']is None,'durable selection/intent changed')
        setup = json.loads(files['prepared/manager-a/broker/inputs/'+aid+'/setup.json'])
        require(setup['src']==adoption['input_project']and setup['document_corpora']==[setup['src']+'/.cks/knowledge/questions']and setup['source_mode']=='snapshot-only'
                and setup['src']!=r['source_root'],'original unfiltered source was used')
        roots = 'prepared/dataset-a/'+job['target_version']+'/'
        require(not any(n=='prepared/dataset-a/current'for n in files),'held dataset was published')
        retained = json.loads(files[roots+'sources/manifest.json'])
        for item in retained['files']:
            blob = files[roots+'sources/blobs/'+item['sha256']]
            require(sha(blob)==item['sha256']and b'PRIVATE_INPUT_SENTINEL'not in blob and b'PRIVATE_REASON_SENTINEL'not in blob,'private/tampered retained input')
        events = [json.loads(data)for name,data in files.items()if name.startswith('prepared/manager-a/workflows/')and data is not None and name.rsplit('/',1)[1][:1].isdigit()]
        require(max(events,key=lambda row:row['sequence'])['kind']=='retracted','withdrawal not persisted')
        history = load(folder,'historical-design.json')
        require(history['revision'].startswith('bb71aae3')and history['verification_exit_code']==0 and all(r['exit_code']==0 for r in history['checks']),'A historical proof differs')
        progress = json.loads((root/'docs/spec-driven/KNOWLEDGE-ROADMAP-PROGRESS.json').read_text())
        item = next(i for i in progress['items']if i['id']=='KM-03')
        require(progress['completed_subtasks']==11 and progress['implementation_completed']==2 and item['status']=='in_progress'
                and item['subtasks'][3]['status']=='in_progress'and not roadmap.check(root=root),'B checkpoint counted as full manager')
    except (OSError,ValueError,KeyError,TypeError,IndexError,StopIteration,tarfile.TarError,subprocess.CalledProcessError)as error:
        return [str(error)]
    return []


if __name__=='__main__':
    errors = check()
    for error in errors:
        print('FAIL:',error)
    print('manager broker evidence:','FAIL'if errors else 'PASS; B only; actual Claude host/quality pending')
    raise SystemExit(bool(errors))
