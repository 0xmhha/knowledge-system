"""Policy/trigger design is not a working scheduler or model acceptance."""
import ast
import json
from pathlib import Path

from knowledge_evidence import load, raw, require, verify_packet, unchanged_checkpoint_go

ROOT=Path(__file__).resolve().parents[1]
PACKET=ROOT/'system/eval/knowledge-roadmap-2026-10-15/km-04-contract'


def check(folder=PACKET,root=ROOT):
    try:
        manifest,_=verify_packet(folder,root)
        require(manifest['checkpoint']=='KM04_1_explicit_policy_trigger_and_success_contract'
                and manifest['KM04_1_complete']is True and manifest['full_KM04_complete']is False
                and manifest['scheduler_runtime_verified']is False and manifest['independent_acceptance']is False
                and manifest['paid_model_calls']==0,'design promoted to scheduler/quality acceptance')
        validation=load(folder,'validation.json')
        roadmap=unchanged_checkpoint_go(root,validation,'cea1c943')
        require(all(v==0 for v in validation['gates'].values())and validation['policy_unit_tests']==27,'contract/gate failure hidden')
        data=raw(folder,'contract-tests.txt').decode()
        tree=ast.parse((root/'scripts/test_knowledge_manager_update_contract.py').read_text())
        names=sorted(n.name for n in ast.walk(tree)if isinstance(n,ast.FunctionDef)and n.name.startswith('test_'))
        require(len(names)==27 and names==load(folder,'test-inventory.json')
                and all(name+' ('in data for name in names)and 'Ran 27 tests'in data and '\nOK\n'in data
                and not any(word in data for word in ('FAILED','ResourceWarning')),'test inventory/result differs')
        require('0 issues.'in raw(folder,'build-lint.txt').decode()and 'documentation: OK (132 live documents)'in raw(folder,'build-lint.txt').decode(),'lint/docs result differs')
        history=load(folder,'historical-design.json')
        require(history['revision'].startswith('cea1c943')and history['verification_exit_code']==0
                and all(row['exit_code']==0 for row in history['checks']),'actual host historical proof changed')
        progress=json.loads((root/'docs/spec-driven/KNOWLEDGE-ROADMAP-PROGRESS.json').read_text())
        item=next(i for i in progress['items']if i['id']=='KM-04')
        require(progress['completed_subtasks']==13 and progress['implementation_completed']==3 and item['status']=='in_progress'
                and item['subtasks'][0]['status']=='complete'and all(s['status']=='not_started'for s in item['subtasks'][1:])
                and item['independent_acceptance']=='not_recorded'and not roadmap.check(root=root),'contract/implementation ledger mismatch')
    except (OSError,ValueError,KeyError,TypeError,IndexError,StopIteration)as error:
        return [str(error)]
    return []


if __name__=='__main__':
    errors=check()
    for error in errors:print('FAIL:',error)
    print('manager update contract:','FAIL'if errors else 'PASS; policy/trigger design; runtime still pending')
    raise SystemExit(bool(errors))
