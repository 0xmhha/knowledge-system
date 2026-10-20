#!/usr/bin/env python3
"""Keep supplied review and input adoption separate from DB/independent acceptance."""
import io
import json
from pathlib import Path, PurePosixPath
import tarfile

from knowledge_evidence import load,raw,require,sha,verify_packet,unchanged_go

ROOT = Path(__file__).resolve().parents[1]
PACKET = ROOT/'system/eval/knowledge-roadmap-2026-10-15/km-03-review'


def digest(domain,value):
    return sha(domain.encode()+b'\0'+json.dumps(value,sort_keys=True,separators=(',', ':'),ensure_ascii=False).encode())


def check(folder=PACKET,root=ROOT):
    try:
        manifest,_ = verify_packet(folder,root)
        require(manifest['checkpoint']=='KM03_3_supplied_review_filtered_input_adoption_retraction'and manifest['KM03_3_complete']is True
                and manifest['full_KM03_complete']is False and manifest['independent_acceptance']is False,'review completion inflated')
        validation = load(folder,'validation.json')
        roadmap = unchanged_go(root,validation)
        require(validation['review_tests_passed']==35 and validation['collection_tests_passed']==23 and validation['registration_tests_passed']==14
                and all(v==0 for v in validation['gates'].values()),'validation gate missing')
        for label,count in [('review',35),('collection',23),('registration',14)]:
            text = raw(folder,label+'-tests.txt').decode()
            require('Ran '+str(count)+' tests'in text and '\nOK\n'in text,'unit proof differs')
        require('documentation: OK'in raw(folder,'lint.txt').decode(),'lint/doc proof missing')
        summary = load(folder,'runtime/summary.json')
        require(summary['scope']=='KM03_3_supplied_review_filtered_input_adoption_retraction_only'and summary['checks']==24 and summary['commands']==25
                and summary['paid_model_calls']==0 and summary['actual_native_post_rename_validations']==1 and summary['controlled_response_records']is True
                and all(summary[n]is False for n in ('external_human_identity_authenticated','independent_human_acceptance','original_source_adopted','db_published','managed_reader_verified','full_KM03_complete','independent_acceptance')),'controlled input review promoted to authenticated/DB acceptance')
        cases = load(folder,'runtime/checks.json')
        require(len(cases)==len(set(cases))==24,'runtime case missing/duplicated')
        commands = load(folder,'runtime/commands.json')
        require(len(commands)==len({r['label']for r in commands})==25,'command receipt missing')
        for row in commands:
            if row['expected_error']:
                error = load(folder,'runtime/'+row['label']+'.stderr.txt')
                require(row['exit_code']==2 and error['code']==row['expected_error'],'refusal hidden')
            else:
                require(row['exit_code']==0,'positive command failed')
        require(load(folder,'runtime/source-before.json')==load(folder,'runtime/source-after.json'),'original sources changed')
        with tarfile.open(fileobj=io.BytesIO(raw(folder,'fixture.tar')))as archive:
            files = {}
            for item in archive:
                p = PurePosixPath(item.name)
                require(not p.is_absolute()and '..'not in p.parts and item.name not in files and (item.isfile()or item.isdir()),'unsafe review fixture')
                files[item.name] = archive.extractfile(item).read()if item.isfile()else None
        r = json.loads(files['manager-a/registration.json'])['registration']
        project = r['project_id']
        other = json.loads(files['manager-b/registration.json'])['registration']['project_id']
        require(project!=other,'projects merged')
        plan = load(folder,'runtime/review-plan.stdout.txt')
        require(plan['project_id']==project and len(plan['questions'])==7 and all(q['answer_status']=='not_provided'for q in plan['questions'])
                and plan['human_approval']=='not_inferred'and len(plan['possible_duplicates'])==1
                and all('not_proven_contradiction'in row['classification']for row in plan['shared_evidence_interpretations']),'questions/conflicts were fabricated or merged')
        did = plan['draft_id']
        prefix = 'manager-a/workflows/'+did+'/'
        names = sorted(n for n in files if n.startswith(prefix)and n[len(prefix):][:1].isdigit())
        require(len(names)==3,'workflow event missing')
        chain = [json.loads(files[n])for n in names]
        require([r['kind']for r in chain]==['reviewed','adopted','retracted'],'workflow order differs')
        previous = None
        for index,row in enumerate(chain,1):
            require(row['project_id']==project and row['draft_id']==did and row['registration_token']==r['token']and row['sequence']==index
                    and row['previous_revision']==previous and row['revision']==digest('cks.manager-review-event.v1',{k:v for k,v in row.items()if k!='revision'})
                    and row['request_id']==digest('cks.manager-review-request.v1',{'kind':row['kind'],'payload':row['payload']}),'event chain differs')
            previous = row['revision']
        response = chain[0]['payload']
        require(response==load(folder,'runtime/review-input.json')and response['reviewer']=='explicit-fixture-reviewer'and response['response_source']['kind']=='controlled_fixture'
                and all(row['reason']for row in response['decisions']),'supplied fixture reviewer/reasons differ')
        require(sorted(row['disposition']for row in response['decisions'])==['accepted']*4+['deferred']*2+['excluded'],'disposition changed')
        adoption = load(folder,'runtime/adoption-result.json')
        aid = adoption['adoption_id']
        require(aid==digest('cks.manager-adoption.v1',{'project_id':project,'draft_id':did,'review_revision':chain[0]['revision']})
                and adoption['revision']==chain[1]['revision']and chain[1]['payload']['adoption_id']==chain[2]['payload']['adoption_id']==aid,'adoption identity differs')
        base = 'manager-a/adoptions/'+aid+'/'
        envelope = json.loads(files[base+'adoption-manifest.json'])
        subject = envelope['adoption']
        require(envelope['checksum']==digest('cks.manager-adoption-manifest.v1',subject)and subject['project_id']==project and subject['registration_token']==r['token'],'adoption seal differs')
        actual = {n[len(base):]for n,b in files.items()if n.startswith(base)and b is not None and n!=base+'adoption-manifest.json'}
        require(actual==set(subject['files'])and all(sha(files[base+n])==h for n,h in subject['files'].items()),'adopted byte inventory differs')
        receipt = json.loads(files[base+'receipt.json'])
        accepted = sorted(row['claim_id']for row in response['decisions']if row['disposition']=='accepted')
        require(receipt['project_id']==project and receipt['adoption_id']==aid and receipt['review_revision']==chain[0]['revision']and receipt['accepted_claim_ids']==accepted
                and len(receipt['omitted_claim_ids'])==3 and receipt['response_source']==response['response_source']and receipt['review_authentication']=='not_performed'
                and receipt['retrieval_verified']is False and receipt['db_publication']=='not_run','adoption includes unreviewed/false verified knowledge')
        require([r['operation']for r in receipt['native_commands']]==['init','validate','lock','validate']and all(r['exit_code']==0 for r in receipt['native_commands']),'native input validation missing')
        body = files[base+'project/.cks/knowledge/questions/manager-reviewed.md']
        for claim in plan['claims']:
            require((claim['claim_id'].encode()in body)==(claim['claim_id']in accepted),'excluded/deferred claim entered knowledge note')
        for name,blob in files.items():
            if name.startswith(base)and blob is not None:
                require(b'PRIVATE_INPUT_SENTINEL'not in blob and b'PRIVATE_REASON_SENTINEL'not in blob,'private source recaptured')
        require(files[base+'project/value.go']==files['source-a/value.go'],'source projection changed')
        native = load(folder,'runtime/native-validate.stdout.txt')
        require(native['status']=='locked'and native['project_id']==project and native['policy_count']==native['decision_count']==0,'native structure lock promoted to false policy/ADR')
        for label,key in [('review-replay','review_revision'),('adopt-replay','adoption_id'),('retract-replay','revision')]:
            row = load(folder,'runtime/'+label+'.stdout.txt')
            expected = chain[0]['revision']if key=='review_revision'else aid if key=='adoption_id'else chain[2]['revision']
            require(row[key]==expected,'idempotent replay differs')
        state = load(folder,'runtime/review-status.stdout.txt')
        require(state['project_id']==project and state['revision']==chain[2]['revision']and state['stage']=='retracted'and state['events']==3 and state['current_adoption_id']is None,'withdrawn adoption remains current')
        native = load(folder,'runtime/native-plugin.stdout.txt')
        require(native['success']is True and native['strict']is True,'native plugin validation differs')
        history = load(folder,'historical-design.json')
        require(history['revision'].startswith('322daf39')and history['verification_exit_code']==0 and all(r['exit_code']==0 for r in history['checks']),'collection historical proof differs')
        progress = json.loads((root/'docs/spec-driven/KNOWLEDGE-ROADMAP-PROGRESS.json').read_text())
        item = next(i for i in progress['items']if i['id']=='KM-03')
        require(progress['completed_subtasks']==11 and progress['implementation_completed']==2 and all(s['status']=='complete'for s in item['subtasks'][:3])
                and item['subtasks'][3]['status']!='complete'and item['status']=='in_progress'and not roadmap.check(root=root),'review counted as full manager')
    except (OSError,ValueError,KeyError,TypeError,IndexError,StopIteration,tarfile.TarError)as error:
        return [str(error)]
    return []


if __name__=='__main__':
    errors = check()
    for error in errors:
        print('FAIL:',error)
    print('manager review evidence:','FAIL'if errors else 'PASS; supplied review/input adoption only; DB reader/host pending')
    raise SystemExit(bool(errors))
