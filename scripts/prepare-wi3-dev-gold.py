#!/usr/bin/env python3
"""Materialize complete finite AI DEV relevance matrices; never run retrieval."""
import argparse
import json
from pathlib import Path
import wi3_dev_gold as g


def write(path,value):
    path.parent.mkdir(parents=True,exist_ok=True)
    path.write_text(json.dumps(value,ensure_ascii=False,indent=2,allow_nan=False)+'\n')


def seal(p):
    write(p/'manifest.json',dict(kind=g.KIND,previous_source_revision=g.REVISION,
        tools=[dict(path=n,sha256=g.f.sha((g.ROOT/n).read_bytes())) for n in g.TOOLS],
        artifacts={x.relative_to(p).as_posix():g.f.sha(x.read_bytes()) for x in sorted(p.rglob('*')) if x.is_file() and x!=p/'manifest.json'}))


def prepare(p):
    p.mkdir(parents=True);values=g.derive()
    write(p/'public/DEV-inputs.json',values['public'])
    (p/'public/wire-requests.json').write_bytes((g.ROOT/g.GAPS/'public-bundle/wire-requests.json').read_bytes())
    write(p/'private/location-catalog.json',values['locations']);write(p/'private/AI-gold.json',values['gold'])
    for d in values['gold']:
        (p/'private'/(d['case_id']+'-REVIEW.txt')).write_text(g.render_review(d,values['locations']))
    _,reviews,proof=g.original()
    for row in reviews:
        s=row['source'];raw=g.safe(g.ROOT/g.INPUTS,proof['files'][s['path']]['artifact']).read_bytes()
        path=p/'private/source'/(row['case_id']+'.go.txt');path.parent.mkdir(parents=True,exist_ok=True)
        path.write_bytes(raw[s['start_byte']:s['end_byte_exclusive']])
    write(p/'SUMMARY.json',values['summary']);write(p/'matching-contract.json',g.matching_contract())
    source=g.f.read(g.ROOT/g.previous.PACKET/'runtime/dataset-identity.json')
    snap=g.f.read(g.ROOT/g.previous.PACKET/'runtime/project-results.json')[0]['response']['snapshot_sha256']
    write(p/'proposed-reader-pins.json',dict(pins=dict(project_id=source['source']['project_id'],dataset_id=source['dataset_id'],commit=g.f.SOURCE_COMMIT,source_mode='committed',graph_snapshot_sha256=snap),status='historical_pilot_identity_not_fresh_retrieval_or_baseline_approval',actual_search_receipt=None))
    write(p/'exposure-and-families.json',dict(families=g.f.read(g.ROOT/g.GAPS/'family-review.json'),pairs=g.f.read(g.ROOT/g.GAPS/'pairwise-review.json'),prior=g.f.read(g.ROOT/g.GAPS/'prior-question-review.json'),all_exposed=True,independent_families=0,FINAL=0))
    state=g.f.read(g.ROOT/g.previous.PACKET/'WORKLIST.json')
    state.update(current_stage='48_WI3_DEV20_complete_selected20_function_AI_relevance_drafts',completed_in_this_scope=['WI3_existing20_symptom_only_DEV_inputs_source_bound','WI3_400_question_function_units_complete_bounded_AI_drafts','WI3_exposure_family_top10_matching_and_denominator_proposals'],next_task='WI3_semantic_unknown_support_context_and_actual_top10_matching_review_then_WI4_DEV_AI_drafts',last_verified_date='2026-10-19')
    state['AI_DEV_draft_progress']['WI-3']='20_DEV_inputs_400_bounded_AI_units_complete_actual_retrieval_and_independent_gold_pending'
    state['AI_DEV_draft_progress']['remaining_work_items']=['WI-4','WI-V1','WI-V2','WI-6','WI-SP','WI-C']
    state['remaining_all'][0].update(status='WI1_WI2_WI3_bounded_AI_DEV_prepared_other6_and_independent_inputs_pending',task='WI3 unknown·지원 문맥·실제 top10 소비자 매칭; WI4/V1/V2/6/SP/C DEV·완전 AI 초안; WI1/WI2 검토 부채; D04 역할/독립성·새 DEV/FINAL 가족·완전 독립 gold·매칭·충분성·F01~08 동결')
    write(p/'WORKLIST.json',state)
    (p/'REPORT.txt').write_text('WI-3 DEV20 and bounded complete AI relevance drafts\n\nD04 미정; independent approval0, formal0/9, FINAL0, freeze null/execution false.\nThe finite universe is the explicitly selected20 committed function bodies in17 files, NOT the whole repository. Each of20 questions has20 labelled locations, complete400 units, answer sets and empty-set rationales. Outside locations remain unlabelled/unknown.\nManual full-function reading plus original40 source fact anchors and exact source witness spans. Supporting context is distinct from direct behavior; source relevance is not an incident/root-cause verdict. Opaque/transitive/runtime contracts stay unknown. No retrieval output authors these labels.\nAll20 inputs are already exposed DEV;15 groups remain proposals, independent families0.190 pair comparisons and840 previous-question comparisons retained. No arbitrary splitting to meet the20-family threshold.\nPublic input/wire bundle contains no private locations, families or gold. Host must receive public files only; path separation is not OS isolation or a sealed FINAL.\nOffline matching proposal checks project/dataset/source-mode/commit/graph snapshot and exact source identity/spans. Raw first10 final candidate positions consume slots; duplicates collapse without refill, unknown/outside/error slots are retained. Backend K, graph signals and multi-round agent unions are different populations.\nCurrent raw wire still max_candidates10/max_seeds3/max_enriched3/max_calls32/depth2/budget_ms1000. No actual symptom retrieval, MCP, installed role, model or quality scoring this stage. Historical pilot pins are not fresh retrieval or baseline approval.\nCase/family macro, baseline/zero denominator and independent sufficient samples remain unfrozen. Native Go binding unchanged; prior full native tests reused, not rerun.\nReview: private/RQ-xxx-REVIEW.txt. Counts: SUMMARY.json. Current stage, next and all remaining work: WORKLIST.json.\n'+json.dumps(values['summary'],ensure_ascii=False,indent=2)+'\n')
    seal(p)


if __name__=='__main__':
    parser=argparse.ArgumentParser();parser.add_argument('--output',type=Path,default=g.ROOT/g.PACKET)
    prepare(parser.parse_args().output)
