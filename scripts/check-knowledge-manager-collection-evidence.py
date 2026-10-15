#!/usr/bin/env python3
"""Check source collection/proposed drafts without promoting review or DB readiness."""
import io
import json
from pathlib import Path, PurePosixPath
import tarfile

from knowledge_evidence import load,raw,require,sha,verify_packet,unchanged_go

ROOT = Path(__file__).resolve().parents[1]
PACKET = ROOT/'system/eval/knowledge-roadmap-2026-10-15/km-03-collection'


def digest(domain,value):
    return sha(domain.encode()+b'\0'+json.dumps(value,sort_keys=True,separators=(',', ':'),ensure_ascii=False).encode())


def check(folder=PACKET,root=ROOT):
    try:
        manifest,_ = verify_packet(folder,root)
        require(manifest['checkpoint']=='KM03_2_grounded_collection_proposed_native_overlay'and manifest['KM03_2_complete']is True
                and manifest['full_KM03_complete']is False and manifest['independent_acceptance']is False,'collection completion inflated')
        validation = load(folder,'validation.json')
        roadmap = unchanged_go(root,validation)
        require(validation['collection_tests_passed']==23 and validation['registration_tests_passed']==14 and all(v==0 for v in validation['gates'].values()),'validation gate missing')
        for label,count in [('collection',23),('registration',14)]:
            text = raw(folder,label+'-tests.txt').decode()
            require('Ran '+str(count)+' tests'in text and '\nOK\n'in text,'unit test proof differs')
        require('documentation: OK'in raw(folder,'lint.txt').decode(),'lint/doc check missing')
        summary = load(folder,'runtime/summary.json')
        require(summary['scope']=='KM03_2_source_collection_and_proposed_native_overlay_only'and summary['checks']==24
                and summary['paid_model_calls']==0 and summary['native_knowledge_post_rename_validations']==2 and summary['actual_Git_change_evidence']is True
                and all(summary[name]is False for name in ('source_adopted','human_review_verified','db_built','managed_reader_verified','full_KM03_complete','independent_acceptance')),'proposed input promoted to adopted/retrievable')
        checks = load(folder,'runtime/checks.json')
        require(len(checks)==len(set(checks))==24,'runtime case missing/duplicated')
        commands = load(folder,'runtime/commands.json')
        require(len(commands)==len({r['label']for r in commands})==19,'actual command receipt missing')
        for row in commands:
            if row['expected_error']:
                error = json.loads(raw(folder,'runtime/'+row['label']+'.stderr.txt'))
                require(row['exit_code']==2 and error['code']==row['expected_error'],'negative command did not refuse')
            else:
                require(row['exit_code']==0,'positive command failed')
        require(load(folder,'runtime/native-plugin-validation.stdout.txt')['success']is True
                and load(folder,'runtime/native-plugin-validation.stdout.txt')['strict']is True,'actual native validation differs')
        with tarfile.open(fileobj=io.BytesIO(raw(folder,'fixture.tar')))as archive:
            files = {}
            for item in archive:
                p = PurePosixPath(item.name)
                require(not p.is_absolute()and '..'not in p.parts and item.name not in files and (item.isfile()or item.isdir()),'unsafe collection fixture')
                files[item.name] = archive.extractfile(item).read()if item.isfile()else None
        require(not any(n.startswith('dataset-')or n.startswith(('source-a/.cks','source-b/.cks'))or n=='reported-command-was-not-run'for n in files),'original source/DB was adopted')
        ids = {}
        for project in ('a','b'):
            r = json.loads(files['manager-'+project+'/registration.json'])
            require(r['checksum']==digest('cks.manager-registration.v1',r['registration']),'registration checksum differs')
            ids[project] = r['registration']['project_id']
            for name,data in files.items():
                prefix = 'manager-'+project+'/collections/'
                if not name.startswith(prefix)or not name.endswith('/manifest.json'):
                    continue
                c = json.loads(data)
                require(c['checksum']==digest('cks.manager-collection-manifest.v1',c['collection']),'collection seal differs')
                content = c['collection']
                require(content['project_id']==ids[project] and content['collection_id']==digest('cks.manager-collection.v1',content['recipe'])
                        and content['files']==content['recipe']['source_files']and content['cks_snapshot_id']is None
                        and content['coverage']['whole_project_semantics_verified']is False,'collection scope/identity differs')
                base = name.rsplit('/',1)[0]
                for item in content['files']:
                    blob = files[base+'/blobs/'+item['sha256']]
                    require(sha(blob)==item['sha256']and len(blob)==item['bytes'],'source blob differs')
                    require(b'PRIVATE_INPUT_MUST_NOT_ENTER_COLLECTION'not in blob and b'EXCLUDED_DECISION_MUST_NOT_ENTER_COLLECTION'not in blob,'excluded source leaked')
        require(ids['a']!=ids['b'],'project identities merged')
        result = load(folder,'runtime/draft-results.json')
        for value in result.values():
            prefix = 'manager-a/drafts/'+value['draft_id']+'/'
            envelope = json.loads(files[prefix+'draft-manifest.json'])
            require(envelope['checksum']==digest('cks.manager-draft-manifest.v1',envelope['draft']),'draft seal differs')
            for name,hash in envelope['draft']['files'].items():
                require(sha(files[prefix+name])==hash,'draft artifact differs')
            draft = json.loads(files[prefix+'draft.json'])
            require(draft['project_id']==ids['a']and draft['human_approval']=='not_recorded'and draft['db_adoption']=='not_run'
                    and all(c['status']=='proposed'and c['reviewed_by']is None and c['review_reason']is None and c['semantic_verification']=='not_run'for c in draft['claims']),'review fabricated')
            require(draft['draft_id']==digest('cks.manager-draft.v1',{'project_id':ids['a'],'collection_id':draft['collection_id'],'claims':draft['claims']}),'draft identity differs')
            native = json.loads(files[prefix+'native-validation.json'])
            require([c['operation']for c in native['commands']]==['init','validate','lock','validate']and all(c['exit_code']==0 for c in native['commands'])
                    and all(native[n]is False for n in ('human_review_verified','source_adopted','retrieval_verified')),'native structure check inflated')
        for label in ('default','custom'):
            native = load(folder,'runtime/native-validate-'+label+'.stdout.txt')
            require(native['status']=='locked'and native['project_id']==ids['a']and native['policy_count']==native['decision_count']==0,'native draft not locked or false normative decision')
        evidence = load(folder,'runtime/evidence-a.stdout.txt')
        require(evidence==load(folder,'runtime/evidence-old-after-change.stdout.txt')and evidence['evidence']['project_id']==ids['a']
                and evidence['evidence']['file_sha256']==sha(files['source-b/consensus.go'])and evidence['evidence']['content_sha256']==sha(evidence['text'].encode())and 'return 101'in evidence['text'],'original evidence changed')
        other = load(folder,'runtime/evidence-b.stdout.txt')
        require(other['evidence']['project_id']==ids['b']and other['evidence']['evidence_id']!=evidence['evidence']['evidence_id'],'foreign project evidence merged')
        change = load(folder,'runtime/evidence-change.stdout.txt')
        require(change['evidence']['origin_id']=='change'and change['evidence']['project_id']==ids['a']and change['evidence']['content_sha256']==sha(change['text'].encode())
                and '-func SharedConsensus() int { return 101 }'in change['text']and '+func SharedConsensus() int { return 303 }'in change['text'],'Git removed/added evidence missing')
        history = load(folder,'historical-design.json')
        require(history['revision'].startswith('2c2b7ed1')and history['verification_exit_code']==0 and all(r['exit_code']==0 for r in history['checks']),'bootstrap historical proof differs')
        progress = json.loads((root/'docs/spec-driven/KNOWLEDGE-ROADMAP-PROGRESS.json').read_text())
        item = next(i for i in progress['items']if i['id']=='KM-03')
        require(progress['completed_subtasks']==10 and progress['implementation_completed']==2 and item['subtasks'][1]['status']=='complete'
                and all(s['status']!='complete'for s in item['subtasks'][2:])and item['status']=='in_progress'and not roadmap.check(root=root),'collection counted as full manager')
    except (OSError,ValueError,KeyError,TypeError,IndexError,StopIteration,tarfile.TarError)as error:
        return [str(error)]
    return []


if __name__=='__main__':
    errors = check()
    for error in errors:
        print('FAIL:',error)
    print('manager collection evidence:','FAIL'if errors else 'PASS; proposed input only; review/adoption/reader pending')
    raise SystemExit(bool(errors))
