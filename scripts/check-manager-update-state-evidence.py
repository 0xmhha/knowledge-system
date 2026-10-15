"""Ledger state/kill proof cannot be promoted to native worker execution."""
import ast
import io
import json
from pathlib import Path, PurePosixPath
import re
import sys
import tarfile

from knowledge_evidence import load,raw,require,sha,verify_packet,unchanged_checkpoint_go
ROOT=Path(__file__).resolve().parents[1]
PACKET=ROOT/'system/eval/knowledge-roadmap-2026-10-15/km-04-state'
SUITES={'registration':14,'collection':23,'review':35,'broker':31,'update_contract':27,'update_inputs':32,'source_check':12,'update_state':50}


def check(folder=PACKET,root=ROOT):
    try:
        manifest,binding=verify_packet(folder,root)
        require(manifest['checkpoint']=='KM04_3_B1_durable_state_not_native_worker'
                and manifest['state_foundation_complete']is True and manifest['full_KM04_3_complete']is False
                and manifest['scheduler_runtime_verified']is False and manifest['native_worker_recovery_verified']is False
                and manifest['paid_model_calls']==0 and manifest['independent_acceptance']is False,'state promoted to worker/full/quality acceptance')
        validation=load(folder,'validation.json');roadmap=unchanged_checkpoint_go(root,validation,'8bfe40e8')
        require(validation['manager_python_tests']==sum(SUITES.values())==224 and all(v==0 for v in validation['gates'].values()),'failed/reduced validation')
        log=raw(folder,'python-tests.txt').decode();require([int(n)for n in re.findall(r'Ran (\d+) tests',log)]==list(SUITES.values())
                and log.count('\nOK\n')==8 and 'FAILED'not in log and 'ResourceWarning'not in log,'Python test completion differs')
        inventory=load(folder,'python-test-inventory.json')
        for label,count in SUITES.items():
            path='scripts/test_knowledge_manager_'+label+'.py';tree=ast.parse((root/path).read_text())
            names=sorted(n.name for n in ast.walk(tree)if isinstance(n,ast.FunctionDef)and n.name.startswith('test_'))
            require(inventory[path]==names and len(names)==count and all(n+' ('in log for n in names),'Python test names omitted')
        require('0 issues.'in raw(folder,'build-lint.txt').decode() and 'documentation: OK (132 live documents)'in raw(folder,'docs-final.txt').decode(),'lint/docs failure')
        summary=load(folder,'runtime/summary.json');checks=load(folder,'runtime/checks.json');commands=load(folder,'runtime/commands.json')
        require(summary['scope']=='KM04_3_B1_durable_ledger_process_recovery_not_native_worker'
                and summary['checks']==len(checks)==len(set(checks))==10 and summary['actual_CLI_commands']==len(commands)==3
                and summary['actual_native_inspections']==1 and summary['actual_native_builds']==0
                and summary['actual_killed_ledger_processes']==1 and summary['native_EOF_exit_codes']==[0]
                and summary['full_KM04_3_complete']is False and summary['scheduler_runtime_verified']is False
                and summary['original_request_recovered']is True and summary['native_admission_replay_verified']is False
                and summary['paid_model_calls']==0 and summary['independent_acceptance']is False,'ledger proof promoted to native build/replay')
        require([c['exit_code']for c in commands]==[0,2,0]
                and [Path(c['argv'][1]).name for c in commands]==['manager.py']*3
                and [c['argv'][2]for c in commands]==['configure-updates','serve','update-status'],'actual CLI inventory/results differ')
        configured=json.loads(commands[0]['stdout']);refused=json.loads(commands[1]['stderr']);observed=json.loads(commands[2]['stdout'])
        require(configured['scheduler_running']is False and refused['code']=='pending_update_requires_reconciliation'
                and 'Traceback'not in commands[1]['stderr'] and observed['scheduler_running']is False
                and observed['native_job_executed_by_this_module']is False,'CLI refusal/execution claim differs')
        before=load(folder,'runtime/before-kill-state.json');recovered=load(folder,'runtime/recovered.json');row=json.loads(raw(folder,'runtime/intent-child.stdout.jsonl'))
        require(before['state']==recovered['state']==observed['state'] and before['state']['phase']=='admitting'
                and before['state']['pending']==row['id'] and before['state']['attempts']==[row]
                and row['job']is None and row['outcome']is None and before['state']['last_success']is None,'intent lost/mutated/claimed as success')
        require(load(folder,'runtime/process-exit.json')=={'signal':'SIGKILL','exit_code':-9,'native_build_was_forwarded':False}
                and raw(folder,'runtime/intent-child.stderr.txt')==b'' and load(folder,'runtime/native-close.json')=={'EOF_exit_code':0}
                and load(folder,'runtime/pending-refusals.json')==['pending_update_requires_reconciliation']*2,'kill/refusal/EOF differs')
        response=load(folder,'runtime/actual-inspection-response.json');profile=response['structuredContent']['profile'];args=load(folder,'runtime/intent-inputs.json')
        require(not response.get('isError') and row['profile']==profile==args['profile'] and row['bundle']==args['bundle']
                and row['input_id']==args['input_id']==load(folder,'runtime/prepared.json')['input_id']
                and all(row['request'][k]==profile[k]for k in ('source_pin','profile_digest','input_digest','embedding_digest'))
                and row['request']['base']==profile['observed_base'],'actual inspected pins relabeled')
        scripts=root/'plugins/knowledge-manager/scripts';sys.path.insert(0,str(scripts))
        from update_state import request_digest,validate_state
        with tarfile.open(fileobj=io.BytesIO(raw(folder,'fixture.tar')))as archive:
            files={}
            for member in archive:
                path=PurePosixPath(member.name);require(not path.is_absolute()and '..'not in path.parts and member.name not in files and(member.isfile()or member.isdir()),'unsafe fixture')
                files[member.name]=archive.extractfile(member).read()if member.isfile()else None
        registration=json.loads(files['fixture/manager/registration.json'])['registration'];validate_state(before['state'],registration)
        require(before['checksum']==__import__('collection').domain_hash('cks.manager-update-state.v1',before['state'])
                and row['request_digest']==request_digest(row['request']),'state/request seal differs')
        require(not any('/dataset/'in n or '/workflows/'in n for n in files)
                and b'return 8'in files['fixture/source/value.go'],'unobserved dataset/semantic review or missing source change')
        copied='fixture/manager/updates/inputs/'+row['input_id']+'/project/value.go'
        require(b'return 7'in files[copied] and files[copied]!=files['fixture/source/value.go'],'original intent followed changed live bytes')
        history=load(folder,'historical-design.json');require(history['revision'].startswith('8bfe40e8') and history['verification_exit_code']==0 and all(r['exit_code']==0 for r in history['checks']),'A proof changed')
        progress=json.loads((root/'docs/spec-driven/KNOWLEDGE-ROADMAP-PROGRESS.json').read_text());item=next(i for i in progress['items']if i['id']=='KM-04')
        require(progress['completed_subtasks']==14 and progress['implementation_completed']==3
                and [s['status']for s in item['subtasks']]==['complete','complete','in_progress','not_started']
                and item['independent_acceptance']=='not_recorded' and not roadmap.check(root=root),'B1 marked full worker complete')
    except (OSError,ValueError,KeyError,TypeError,IndexError,StopIteration,tarfile.TarError)as error:return [str(error)]
    return []


if __name__=='__main__':
    errors=check()
    for error in errors:print('FAIL:',error)
    print('manager update state:','FAIL'if errors else 'PASS; durable ledger only; native worker pending')
    raise SystemExit(bool(errors))
