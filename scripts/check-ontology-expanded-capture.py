"""Verify 512 actual calls and local API meter without claiming scored effect."""
import importlib.util
import json
from pathlib import Path
from knowledge_evidence import load,raw,require,verify_packet,unchanged_checkpoint_go
ROOT=Path(__file__).resolve().parents[1]
PACKET=ROOT/'system/eval/knowledge-roadmap-2026-10-16/oe-02-capture'


def check(folder=PACKET,root=ROOT):
    try:
        m,b=verify_packet(folder,root);require(m['checkpoint']=='OE01_2_expanded_four_arm_capture'and m['four_arm_capture_complete']is True,'capture missing')
        require(all(m[k]is False for k in('effect_scored','quality_acceptance','independent_acceptance','human_label_approval','full_OE01_complete')),'capture promoted to quality/effect verdict')
        v=load(folder,'validation.json');roadmap=unchanged_checkpoint_go(root,v,'dd554e79');require(all(code==0 for code in v['gates'].values()),'failed gate')
        spec=importlib.util.spec_from_file_location('expanded_capture_verifier',root/'scripts/score-ontology-effect-expanded.py');score=importlib.util.module_from_spec(spec);spec.loader.exec_module(score)
        verified,*_=score.verify(folder/'runtime');require(verified==load(folder,'runtime/verification.json')and verified['actual_capture_calls']==512 and verified['failed_calls']==verified['missing_slots']==0 and verified['capture_sessions_closed']==64 and verified['readiness_sessions_EOF0']==4 and verified['meter_active_at_exit']==0,'raw/summary differs')
        summary=load(folder,'runtime/summary.json');require(summary['stage']=='captured_not_scored_or_quality_accepted'and summary['actual_capture_calls']==512 and summary['quality_acceptance']is False and summary['independent_acceptance']is False and summary['paid_model_calls']==summary['LLM_answer_calls']==0,'execution scope differs')
        binding=load(folder,'runtime/input-binding.json');require(binding['preregistered_revision'].startswith('f73b9e36')and binding['signed_preregistration_verified']is True,'not signed preregistered input')
        require('effect-summary.json'not in{p.name for p in(folder/'runtime').iterdir()},'capture phase scored prematurely')
        require('documentation: OK (134 live documents)'in raw(folder,'docs-final.txt').decode(),'docs failed')
        history=load(folder,'historical-design.json');require(history['revision'].startswith('f73b9e36')and history['verification_exit_code']==0 and all(c['exit_code']==0 for c in history['checks']),'preregistration restore failed')
        p=json.loads((root/'docs/spec-driven/KNOWLEDGE-ROADMAP-PROGRESS.json').read_text());item=next(i for i in p['items']if i['id']=='OE-01');require(p['completed_subtasks']==26 and p['implementation_completed']==6 and item['status']=='in_progress'and[s['status']for s in item['subtasks']]==['complete','complete','in_progress','not_started']and item['independent_acceptance']=='not_recorded'and not roadmap.check(root=root),'OE ledger promoted')
    except(OSError,ValueError,KeyError,TypeError,IndexError,StopIteration,AssertionError)as e:return[str(e)]
    return []

if __name__=='__main__':
    errors=check()
    for e in errors:print('FAIL:',e)
    print('ontology expanded capture:','FAIL'if errors else'PASS;512 actual calls verified; effect scoring remains')
    raise SystemExit(bool(errors))
