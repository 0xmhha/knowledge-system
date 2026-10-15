#!/usr/bin/env python3
"""Check KM-03.1 packaging/metadata proof without claiming a full manager."""
import io
import json
from pathlib import Path, PurePosixPath
import tarfile

from knowledge_evidence import load,raw,require,sha,verify_packet,unchanged_go

ROOT = Path(__file__).resolve().parents[1]
PACKET = ROOT/'system/eval/knowledge-roadmap-2026-10-15/km-03-bootstrap'


def check(folder=PACKET,root=ROOT):
    try:
        m,binding = verify_packet(folder,root)
        require(m['checkpoint']=='KM03_1_owned_registration_native_package_metadata' and m['KM03_1_complete']is True
                and m['full_KM03_complete']is False and m['independent_acceptance']is False,'bootstrap completion inflated')
        validation = load(folder,'validation.json')
        roadmap = unchanged_go(root,validation)
        require(validation['registration_tests_passed']==14 and all(v==0 for v in validation['gates'].values())
                and 'Ran 14 tests' in raw(folder,'registration-tests.txt').decode() and '\nOK\n'in raw(folder,'registration-tests.txt').decode()
                and 'documentation: OK'in raw(folder,'lint.txt').decode(),'bootstrap validation missing')
        summary = load(folder,'runtime/summary.json')
        require(summary['scope']=='KM03_1_native_package_registration_metadata_only' and summary['checks']==9 and summary['native_validations']==3
                and summary['actual_metadata_connections']==2 and summary['launcher_EOF_exit_code']==0 and summary['paid_model_calls']==0
                and summary['model_behavior_verified']is False and summary['full_KM03_complete']is False and summary['independent_acceptance']is False,'metadata discovery inflated to full/model proof')
        expected = ['native strict manifest validation passes without model call','native strict skills validation passes without model call',
                    'native strict agents validation passes without model call','document-free project registration writes only owned state',
                    'offline diagnosis preserves explicit project and unimplemented boundaries','actual metadata doctor from unrelated cwd discovers four maintenance tools',
                    'plugin launcher execs actual CKS and preserves public tool schemas','launcher actual MCP EOF exits zero',
                    'metadata-only connections do not create datasets or source documents']
        require(load(folder,'runtime/checks.json')==expected,'bootstrap case missing')
        commands = load(folder,'runtime/commands.json')
        require(len(commands)==7 and len({r['label']for r in commands})==7 and all(r['exit_code']==0 for r in commands),'native/registration/doctor receipt missing')
        for label in ('manifest','skills','agents'):
            report = json.loads(raw(folder,'runtime/native-'+label+'.stdout.txt'))
            require(report['success']is True and report['strict']is True,'native validation failed/not strict')
        offline = json.loads(raw(folder,'runtime/doctor-offline.stdout.txt'))
        actual = json.loads(raw(folder,'runtime/doctor-connect.stdout.txt'))
        require(all(offline[n]is False and actual[n]is False for n in ('draft_collection','review_workflow','managed_reader_broker','scheduling'))
                and offline['model_readiness']==actual['model_readiness']=='not_measured','unimplemented capability promoted')
        required = {'cks.dataset.'+n for n in ('inspect_profile','build','status','cancel')}
        require(set(actual['connection']['maintenance_tools'])==required and actual['connection']['EOF_exit_code']==0
                and actual['connection']['model_inspection']=='not_run' and actual['connection']['paid_model_calls']==0,'actual doctor differs')
        ins = [json.loads(line)for line in raw(folder,'runtime/launcher-session/mcp.stdin.jsonl').splitlines()]
        outs = [json.loads(line)for line in raw(folder,'runtime/launcher-session/mcp.stdout.jsonl').splitlines()]
        require([r['method']for r in ins]==['initialize','notifications/initialized','tools/list']
                and [r.get('id')for r in ins]==[1,None,2] and [r['id']for r in outs]==[1,2] and all('error'not in r for r in outs),'launcher raw metadata handshake differs')
        tools = load(folder,'runtime/tools.json')
        require(tools==outs[1]['result']['tools']and len(tools)==37,'launcher schemas not from actual wire')
        selected = {t['name']:t for t in tools if t['name']in required}
        require(set(selected)==required and all(t.get('outputSchema')for t in selected.values()),'maintenance schema missing')
        with tarfile.open(fileobj=io.BytesIO(raw(folder,'fixture.tar')))as archive:
            files = {}
            for item in archive:
                p = PurePosixPath(item.name)
                require(not p.is_absolute()and '..'not in p.parts and item.name not in files and (item.isfile()or item.isdir()),'unsafe bootstrap fixture')
                files[item.name] = archive.extractfile(item).read()if item.isfile()else None
        require(not any(n=='dataset'or n.startswith('dataset/')or(n.startswith('source/')and n.endswith('.md'))for n in files),'metadata probe wrote dataset/source docs')
        r = json.loads(files['manager/registration.json'])
        payload = r['registration']
        canonical = json.dumps(payload,sort_keys=True,separators=(',', ':'),ensure_ascii=False).encode()
        require(r['checksum']==sha(b'cks.manager-registration.v1\0'+canonical),'registration checksum differs')
        require(json.loads(files['manager/owner.json'])=={'schema_version':1,'project_id':payload['project_id'],'token':payload['token']},'owned registration marker differs')
        for name,digest in payload['files'].items():
            require(sha(files['manager/'+name])==digest,'owned registration config differs')
        result = load(folder,'runtime/registration-result.json')
        require(result['project_id']==offline['project_id']==actual['project_id']==payload['project_id'] and result['profile_id']==payload['profile_id']
                and result['stage']=='registered_not_built'and result['dataset_created']is False and result['source_modified']is False,'registration scope/identity differs')
        require('source/go.mod'in files and 'source/consensus.go'in files and b'return 1'in files['source/consensus.go'],'document-free source fixture missing')
        history = load(folder,'historical-design.json')
        require(history['revision'].startswith('6705ed57')and history['verification_exit_code']==0 and all(r['exit_code']==0 for r in history['checks']),'historical public failure proof differs')
        progress = json.loads((root/'docs/spec-driven/KNOWLEDGE-ROADMAP-PROGRESS.json').read_text())
        item = next(i for i in progress['items']if i['id']=='KM-03')
        require(progress['completed_subtasks']==9 and progress['implementation_completed']==2 and item['subtasks'][0]['status']=='complete'
                and all(r['status']!='complete'for r in item['subtasks'][1:])and item['status']=='in_progress'and not roadmap.check(root=root),'bootstrap counted as full manager')
    except (OSError,ValueError,KeyError,TypeError,IndexError,StopIteration,tarfile.TarError)as error:
        return [str(error)]
    return []


if __name__=='__main__':
    errors = check()
    for error in errors:
        print('FAIL:',error)
    print('manager bootstrap evidence:','FAIL'if errors else 'PASS; KM-03.1 only; collection/review/reader pending')
    raise SystemExit(bool(errors))
