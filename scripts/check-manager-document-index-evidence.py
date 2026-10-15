#!/usr/bin/env python3
"""Check actual document indexing/read proof without claiming full manager/host."""
import io
import json
from pathlib import Path, PurePosixPath
import tarfile

from knowledge_evidence import load,raw,require,sha,verify_packet,current_go

ROOT = Path(__file__).resolve().parents[1]
PACKET = ROOT/'system/eval/knowledge-roadmap-2026-10-15/km-03-document-index'


def check(folder=PACKET,root=ROOT):
    try:
        manifest,binding = verify_packet(folder,root)
        require(manifest['checkpoint']=='KM03_4_A_reviewed_document_index_and_retained_reads'and manifest['document_index_checkpoint_complete']is True
                and manifest['KM03_4_complete']is False and manifest['full_KM03_complete']is False and manifest['independent_acceptance']is False,'document index scope inflated')
        validation = load(folder,'validation.json')
        roadmap = current_go(root,folder,validation,binding)
        require(all(v==0 for v in validation['gates'].values())and 'documentation: OK'in raw(folder,'lint.txt').decode(),'build/lint/docs gate missing')
        summary = load(folder,'runtime/summary.json')
        require(summary['scope']=='KM03_4_A_actual_reviewed_documents_public_build_and_retained_reader_only'and summary['checks']==14
                and summary['MCP_sessions']==3 and summary['EOF_exit_codes']==[0,0,0]and summary['paid_model_calls']==0
                and summary['embedding']=='mock_lifecycle_not_quality'and summary['candidate_state']=='held_for_review'
                and all(summary[n]is False for n in ('DB_published','manager_reader_broker_verified','actual_Claude_host_workflow_verified','KM03_4_complete','full_KM03_complete','independent_acceptance')),'native reader proof promoted to broker/host/quality')
        checks = load(folder,'runtime/checks.json')
        require(len(checks)==len(set(checks))==14,'runtime case missing')
        commands = load(folder,'runtime/commands.json')
        require(len(commands)==4 and all(r['exit_code']==0 for r in commands),'actual fixture/review/adopt/config command missing')
        calls = load(folder,'runtime/calls.json')
        actual = []
        for label in ('maintenance-session','reader-session','maintenance-restart-session'):
            ins = [json.loads(line)for line in raw(folder,'runtime/'+label+'/mcp.stdin.jsonl').splitlines()]
            outs = [json.loads(line)for line in raw(folder,'runtime/'+label+'/mcp.stdout.jsonl').splitlines()]
            replies = {row['id']:row for row in outs if 'id'in row}
            require(len(replies)==len(outs)==len(ins)and all('error'not in row for row in outs),'actual MCP response missing/error')
            for row in ins:
                if row['method']=='tools/call':
                    response = replies[row['id']]['result']
                    require(not response.get('isError'),'actual tool failed')
                    actual.append({'session':label,'tool':row['params']['name'],'arguments':row['params']['arguments'],'response':response})
        require(actual==calls,'document observations are not from actual wire')
        job = load(folder,'runtime/job.json')
        inspection = load(folder,'runtime/inspection.json')
        require(job['state']=='held_for_review'and job['activated']is False and job['publication']is None and job['terminal']is True
                and all(job['gates'][gate]=='passed'for gate in inspection['profile']['required_gates']),'held candidate promoted/failed gates hidden')
        builds = [row for row in calls if row['tool']=='cks.dataset.build']
        require(len(builds)==2 and builds[0]['arguments']==builds[1]['arguments']==load(folder,'runtime/build-request.json')
                and builds[1]['response']['structuredContent']==job,'durable replay changed candidate')
        require(load(folder,'runtime/observations.json')[-1]==job,'terminal observation differs')
        pack = load(folder,'runtime/document-read.json')
        retained_read = load(folder,'runtime/document-read-after-live-deletion.json')
        require(pack['coordinates']==retained_read['coordinates']==job['candidate']['coordinates']and pack['citations']==retained_read['citations']and pack['bodies']==retained_read['bodies'],'reader fallback mixed changed/deleted source')
        require(pack['citations']and any(c['origin_id']=='docs:corpus.0'for c in pack['citations'])and all(c['project_id']==job['project_id']and c['dataset_id']==pack['coordinates']['dataset_id']for c in pack['citations'])
                and any('bounded queue'in json.dumps(body)for body in pack['bodies']),'reviewed body/docs origin/project citation missing')
        vector = load(folder,'runtime/vector-manifest.json')
        setup = load(folder,'runtime/setup.json')
        require(vector['docs_roots']==setup['document_corpora']and vector['languages']['markdown']>0,'declared notes not in actual index')
        retained = load(folder,'runtime/retained-manifest.json')
        notes = [r for r in retained['files']if r['origin_id']=='docs:corpus.0'and r['path']=='manager-reviewed.md']
        require(len(notes)==1,'retained docs origin missing')
        with tarfile.open(fileobj=io.BytesIO(raw(folder,'fixture.tar')))as archive:
            files = {}
            for item in archive:
                p = PurePosixPath(item.name)
                require(not p.is_absolute()and '..'not in p.parts and item.name not in files and (item.isfile()or item.isdir()),'unsafe document fixture')
                files[item.name] = archive.extractfile(item).read()if item.isfile()else None
        version = 'prepared/dataset-a/reviewed-doc-v1/'
        note = files[version+'sources/blobs/'+notes[0]['sha256']]
        require(sha(note)==notes[0]['sha256']and len(note)==notes[0]['size'],'retained note bytes changed')
        adoption = load(folder,'runtime/adoption.json')
        source_prefix = 'prepared/manager-a/adoptions/'+adoption['adoption_id']+'/project/'
        require(note==files[source_prefix+'.cks/knowledge/questions/manager-reviewed.md'],'indexed note differs from reviewed input')
        before = load(folder,'runtime/input-before.json')
        require(before=={n[len(source_prefix):]:sha(data)for n,data in files.items()if n.startswith(source_prefix)and data is not None},'filtered source restoration differs')
        for name,data in files.items():
            if name.startswith(version+'sources/blobs/')and data is not None:
                require(sha(data)==name.rsplit('/',1)[1]and b'PRIVATE_INPUT_SENTINEL'not in data and b'PRIVATE_REASON_SENTINEL'not in data,'private/tampered blob in built dataset')
        require(not any(n=='prepared/dataset-a/current'or n.startswith(source_prefix+prefix)for n in files for prefix in ('.env','private/')),'held publication or excluded source present')
        history = load(folder,'historical-design.json')
        require(history['revision'].startswith('3d39801c')and history['verification_exit_code']==0 and all(r['exit_code']==0 for r in history['checks']),'historical supplied review proof differs')
        progress = json.loads((root/'docs/spec-driven/KNOWLEDGE-ROADMAP-PROGRESS.json').read_text())
        item = next(i for i in progress['items']if i['id']=='KM-03')
        require(progress['completed_subtasks']==11 and progress['implementation_completed']==2 and item['status']=='in_progress'
                and item['subtasks'][3]['status']=='in_progress'and not roadmap.check(root=root),'partial document indexing counted as full manager')
    except (OSError,ValueError,KeyError,TypeError,IndexError,StopIteration,tarfile.TarError)as error:
        return [str(error)]
    return []


if __name__=='__main__':
    errors = check()
    for error in errors:
        print('FAIL:',error)
    print('manager document index evidence:','FAIL'if errors else 'PASS; KM03.4 A only; broker and actual host pending')
    raise SystemExit(bool(errors))
