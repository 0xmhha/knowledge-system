"""Recalculate finite source-function metrics and preserve inconclusive quality scope."""
import importlib.util
import json
from pathlib import Path
from knowledge_evidence import load,raw,require,verify_packet,unchanged_checkpoint_go
ROOT=Path(__file__).resolve().parents[1]
PACKET=ROOT/'system/eval/knowledge-roadmap-2026-10-16/oe-03-score'


def check(folder=PACKET,root=ROOT):
    try:
        m,b=verify_packet(folder,root);require(m['checkpoint']=='OE01_3_expanded_function_retrieval_effect_scored'and m['effect_scored']is True,'effect analysis missing')
        require(all(m[k]is False for k in('quality_acceptance','independent_acceptance','human_label_approval','full_OE01_complete','LLM_answer_quality_evaluated')),'source retrieval promoted to general/answer quality')
        v=load(folder,'validation.json');roadmap=unchanged_checkpoint_go(root,v,'dd554e79');require(all(code==0 for code in v['gates'].values()),'failed validation')
        spec=importlib.util.spec_from_file_location('expanded_score_recompute',root/'scripts/score-ontology-effect-expanded.py');score=importlib.util.module_from_spec(spec);spec.loader.exec_module(score);summary,questions,verification,*_=score.score(folder/'runtime')
        require(summary==load(folder,'runtime/effect-summary.json')and questions==load(folder,'runtime/function-retrieval-scores.json')and verification==load(folder,'runtime/verification.json'),'derived effect metrics differ from raw data')
        require(summary['human_or_independent_gold']is False and summary['query_variants_are_not_independent_facts']is True and summary['quality_acceptance']is False and summary['independent_acceptance']is False and summary['LLM_answer_quality']is None and summary['default_recommendation']=='keep_off_pending_reviewed_links_and_broader_independent_evidence','scope/default changed')
        arms=summary['arms'];require(arms['off']['family_macro']==arms['relations']['family_macro']and arms['concept_text']['family_macro']==arms['combined']['family_macro'],'reported arm relation differs')
        by={(q['id'],q['arm']):q for q in questions};changed=[qid for qid in['EX-'+str(i).zfill(2)for i in range(1,15)]if any(by[(qid,'off')][key]!=by[(qid,'concept_text')][key]for key in('precision_at10','recall_at10','reciprocal_rank_at10'))];require(changed==['EX-08'],'benefit scope changed')
        require(all(set(o['found_unit_ids'])=={'handler-prepare'}for o in by[('EX-08','off')]['observations'])and all(set(o['found_unit_ids'])=={'handler-prepare','handler-commit'}for o in by[('EX-08','concept_text')]['observations']),'manual benefit source audit differs')
        require(all(arms[arm]['applied_relations_all_phases']==0 and arms[arm]['warm_observations']==80 and arms[arm]['cloud_model_API_calls']==0 and arms[arm]['local_electricity_hardware_cost']=='not_measured'for arm in arms),'relation/latency/cost scope promoted')
        require(all(o['out_of_intent_concepts']==['wbft-quorum']for o in by[('EX-16','concept_text')]['observations']),'foreign intent mismatch hidden')
        recommendation=load(folder,'recommendation.json');require(recommendation=={'ontology_default':'off','quality':'inconclusive','independent_acceptance':False,'human_gold':False,'source_retrieval_scope':'finite_complete_function_tasks','benefit_question_ids':['EX-08'],'relation_links_available':0,'generated_answer_quality':'not_executed','production_activation':False},'recommendation promoted')
        require('documentation: OK (134 live documents)'in raw(folder,'docs-final.txt').decode(),'docs failed')
        history=load(folder,'historical-design.json');require(history['revision'].startswith('e44c58f0')and history['verification_exit_code']==0 and all(c['exit_code']==0 for c in history['checks']),'capture restore failed')
        p=json.loads((root/'docs/spec-driven/KNOWLEDGE-ROADMAP-PROGRESS.json').read_text());item=next(i for i in p['items']if i['id']=='OE-01');require(p['completed_subtasks']==27 and p['implementation_completed']==6 and item['status']=='in_progress'and[s['status']for s in item['subtasks']]==['complete','complete','complete','not_started']and item['independent_acceptance']=='not_recorded'and not roadmap.check(root=root),'OE ledger promoted')
    except(OSError,ValueError,KeyError,TypeError,IndexError,StopIteration,AssertionError)as e:return[str(e)]
    return []

if __name__=='__main__':
    errors=check()
    for e in errors:print('FAIL:',e)
    print('ontology expanded effect:','FAIL'if errors else'PASS; source-function metrics recomputed; keep off/quality inconclusive')
    raise SystemExit(bool(errors))
