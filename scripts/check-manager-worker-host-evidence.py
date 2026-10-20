"""Actual CLI process lifecycle proof is separate from MCP/sustained execution."""
import ast
import io
import re
import json
from pathlib import Path,PurePosixPath
import tarfile

from knowledge_evidence import load,raw,require,sha,verify_packet,unchanged_checkpoint_go
ROOT=Path(__file__).resolve().parents[1]
PACKET=ROOT/'system/eval/knowledge-roadmap-2026-10-16/km-04-worker-host'


def check(folder=PACKET,root=ROOT):
    try:
        manifest,binding=verify_packet(folder,root)
        require(manifest['checkpoint']=='KM04_3_C1_actual_worker_process_lifecycle' and manifest['actual_worker_process_lifecycle_verified']is True
                and manifest['manager_MCP_worker_operation_verified']is False and manifest['sustained_schedule_verified']is False
                and manifest['full_KM04_3_complete']is False and manifest['full_KM04_4_complete']is False
                and manifest['independent_acceptance']is False,'CLI host proof promoted to MCP/sustained/full/quality')
        validation=load(folder,'validation.json');roadmap=unchanged_checkpoint_go(root,validation,'a801f551')
        require(all(v==0 for v in validation['gates'].values()) and validation['manager_python_tests']==248
                and validation['current_manager_Python_rerun']is True,'failed validation or Python results reused')
        suites={'registration':14,'collection':23,'review':35,'broker':31,'update_contract':27,'update_inputs':32,'source_check':12,'update_state':50,'update_worker':24}
        log=raw(folder,'python-tests.txt').decode();inventory=load(folder,'python-test-inventory.json')
        require([int(n) for n in re.findall(r'Ran (\d+) tests',log)]==list(suites.values()) and log.count('\nOK\n')==9
                and 'FAILED'not in log and 'ResourceWarning'not in log,'Python test completion differs')
        for label,count in suites.items():
            path='scripts/test_knowledge_manager_'+label+'.py';names=sorted(n.name for n in ast.walk(ast.parse((root/path).read_text())) if isinstance(n,ast.FunctionDef)and n.name.startswith('test_'))
            require(inventory[path]==names and len(names)==count and all(n+' ('in log for n in names),'Python inventory omitted')
        require('0 issues.'in raw(folder,'build-lint.txt').decode() and 'documentation: OK (132 live documents)'in raw(folder,'docs-final.txt').decode(),'lint/docs failure')
        summary=load(folder,'runtime/summary.json');checks=load(folder,'runtime/checks.json');commands=load(folder,'runtime/commands.json');observations=load(folder,'runtime/process-observations.json')
        require(summary['scope']=='KM04_3_C1_actual_foreground_worker_process_stop_kill_budget_not_MCP_operation_or_sustained_schedule'
                and summary['checks']==len(checks)==len(set(checks))==21 and summary['actual_worker_process_cases']==3
                and summary['signals']==['SIGTERM','SIGKILL','worker_budget'] and summary['actual_CLI_commands']==len(commands)==10
                and summary['owned_processes_quiescent']is True and summary['late_write_refusals']==3
                and summary['actual_linked_recovery_publications']==3 and summary['actual_native_exit_codes']=='not_observed_not_claimed_zero'
                and summary['manager_MCP_worker_operation_verified']is False and summary['sustained_periodic_schedule_verified']is False
                and summary['full_KM04_3_complete']is False and summary['full_KM04_4_complete']is False
                and summary['paid_model_calls']==0 and summary['independent_acceptance']is False,'process scope/counts inflated')
        by_label={c['label']:c for c in commands};require(len(by_label)==10,'CLI labels duplicate')
        baseline=load(folder,'runtime/baseline.json');require(json.loads(by_label['baseline']['stdout'])==baseline and by_label['baseline']['exit_code']==0,'baseline CLI omitted')
        prior_row=next(row for row in baseline['state']['attempts']if row['id']==baseline['state']['last_success'])
        require(prior_row['outcome']=='succeeded','baseline not published')
        for label,number in [('sigterm',8),('sigkill',9),('budget',10)]:
            cmd=by_label[label];before=load(folder,'runtime/'+label+'-before.json');after=load(folder,'runtime/'+label+'-after.json')
            pending=next(row for row in before['attempts']if row['id']==before['pending']);preserved=next(row for row in after['attempts']if row['id']==pending['id'])
            require(cmd['exit_code']==(-9 if label=='sigkill' else 0) and cmd['stderr']==''
                    and preserved['request']==pending['request'] and after['pending']==pending['id'],'exit/pending obligation differs')
            if label!='sigkill':
                stopped=load(folder,'runtime/'+label+'-stopped.json');require(json.loads(cmd['stdout'])==stopped and stopped['state_snapshot_origin']=='last_confirmed_in_process_state' and stopped['state']['pending']==pending['id'] and stopped['stop_reason']==('budget'if label=='budget'else'signal')
                    and (stopped['disk_state_requires_reconciliation']is True and stopped['worker_stop_outcome']=='closed_requires_persistence_reconciliation'
                         and stopped['state_snapshot_origin']=='last_confirmed_in_process_state'
                         or stopped['disk_state_requires_reconciliation']is False and stopped['state']['phase']=='stopped' and stopped['worker_stop_outcome']=='closed_with_pending_intent'),'normal stop claimed completed cancellation')
            obs=next(o for o in observations if o['label']==label)
            require(obs['worker_exit_code']==cmd['exit_code'] and obs['late_write_exists']is False
                    and obs['observed_monotonic']>=obs['graph']['started_monotonic']+9
                    and obs['current_before']==obs['current_after']==prior_row['request']['new_version']
                    and obs['owned_native_processes'] and len(obs['post_stop_process_observations'])==len(obs['owned_native_processes'])+1
                    and all(r['running']is False and(not r['stat']or r['stat'].startswith('Z'))for r in obs['post_stop_process_observations'])
                    and obs['native_exit_codes']=='not_observable_after_parent_exit','live child/late writer/current swap hidden')
            reconciled=load(folder,'runtime/'+label+'-reconciled.json');recovered=load(folder,'runtime/'+label+'-recovered.json');pack=load(folder,'runtime/'+label+'-recovered-read.json')
            for suffix,value in [('reconcile',reconciled),('retry',recovered)]:
                require(by_label[label+'-'+suffix]['exit_code']==0 and json.loads(by_label[label+'-'+suffix]['stdout'])==value,'recovery CLI result omitted')
            canceled=next(r for r in reconciled['state']['attempts']if r['id']==pending['id'])
            require(canceled['outcome']=='canceled' and canceled['job']['terminal']is True and canceled['job']['activated']is False
                    and canceled['request']==pending['request'] and reconciled['state']['last_success']==prior_row['id']
                    and len(reconciled['state']['attempts'])==len(before['attempts']),'old job replaced or cancellation inferred from signal')
            final=next(r for r in recovered['state']['attempts']if r['id']==recovered['state']['last_success'])
            require(final['outcome']=='succeeded' and final['job']['activated']is True and final['attempt']==2
                    and final['previous_attempt']==pending['id'] and final['input_id']==pending['input_id']
                    and final['request']['idempotency_key']!=pending['request']['idempotency_key']
                    and pack['coordinates']==final['job']['candidate']['coordinates'] and any('return '+str(number)in json.dumps(b)for b in pack['bodies'])
                    and all(c['project_id']==pack['coordinates']['project_id'] and c['dataset_id']==pack['coordinates']['dataset_id']for c in pack['citations']),'retry/read linked to wrong input/version')
            prior_row=final
        with tarfile.open(fileobj=io.BytesIO(raw(folder,'fixture.tar')))as archive:
            names=set();links={};files={}
            for member in archive:
                path=PurePosixPath(member.name);require(not path.is_absolute()and '..'not in path.parts and member.name not in names,'unsafe fixture');names.add(member.name)
                if member.issym():
                    require(member.name=='fixture/dataset/current'and member.linkname==prior_row['request']['new_version'],'unexpected fixture link');links[member.name]=member.linkname
                else:
                    require(member.isfile()or member.isdir(),'unsafe fixture type')
                    if member.isfile():files[member.name]=archive.extractfile(member).read()
            configs=[json.loads(data) for name,data in files.items()if '/updates/worker/bundles/'in name and name.endswith('/cks.json')]
            require(configs,'missing native bundle')
            for config in configs:
                check=config['maintenance']['profiles'][0]['source_check']
                for script in(root/'plugins/knowledge-manager/scripts').glob('*.py'):
                    suffix='/'+script.relative_to(root).as_posix()
                    require(any(name.endswith(suffix) and digest==sha(script.read_bytes())for name,digest in check['pinned_files'].items()),'executed manager code differs from current source')
            require(len(links)==1 and not any('/workflows/'in name or '/late-'in name for name in names),'late write/review fabricated')
        plugin=load(folder,'plugin-validate.json');require(plugin['success']is True and plugin['strict']is True and not plugin['manifest']['errors'] and not plugin['manifest']['warnings'],'plugin validation failure hidden')
        history=load(folder,'historical-design.json');require(history['revision'].startswith('a801f551') and history['verification_exit_code']==0 and all(c['exit_code']==0 for c in history['checks']),'B2 immutable proof changed')
        progress=json.loads((root/'docs/spec-driven/KNOWLEDGE-ROADMAP-PROGRESS.json').read_text());item=next(i for i in progress['items']if i['id']=='KM-04')
        require(progress['completed_subtasks']==14 and progress['implementation_completed']==3 and [s['status']for s in item['subtasks']]==['complete','complete','in_progress','not_started']
                and item['independent_acceptance']=='not_recorded' and not roadmap.check(root=root),'C1 marked MCP/sustained/full complete')
    except (OSError,ValueError,KeyError,TypeError,IndexError,StopIteration,tarfile.TarError)as error:return [str(error)]
    return []


if __name__=='__main__':
    errors=check()
    for error in errors:print('FAIL:',error)
    print('manager worker host:','FAIL'if errors else 'PASS; actual CLI process lifecycle; MCP/sustained still pending')
    raise SystemExit(bool(errors))
