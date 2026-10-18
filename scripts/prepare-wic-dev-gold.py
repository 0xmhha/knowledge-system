#!/usr/bin/env python3
"""Prepare source reviews and adopted consumer proof without running an agent."""
import argparse,gzip,shutil
from pathlib import Path
import wic_dev_gold as g
from importlib.util import module_from_spec,spec_from_file_location
sp=spec_from_file_location('draft_writer',Path(__file__).with_name('prepare-wi4-dev-gold.py'));writer=module_from_spec(sp);sp.loader.exec_module(writer);write=writer.write


def seal(p):
    write(p/'manifest.json',dict(kind=g.KIND,previous_source_revision=g.REVISION,
        tools=[dict(path=n,sha256=g.f.sha((g.ROOT/n).read_bytes())) for n in g.TOOLS],source_inputs=[],
        artifacts={x.relative_to(p).as_posix():g.f.sha(x.read_bytes()) for x in sorted(p.rglob('*')) if x.is_file() and x!=p/'manifest.json'}))


def prepare(p,consumer):
    p.mkdir(parents=True);shutil.copytree(consumer,p/'private/consumer');data=g.derive(packet=p)
    for name,key in [('public/DEV-inputs.json','public'),('SUMMARY.json','summary'),('consumer-contract.json','consumer')]:write(p/name,data[key])
    (p/'private/AI-gold.json.gz').write_bytes(gzip.compress(g.f.canonical(data['gold']),mtime=0))
    for public,gold in zip(data['public'],data['gold']):
        x=p/'public'/public['source_excerpt'];x.parent.mkdir(parents=True,exist_ok=True);x.write_bytes(gold['source']['body'].encode())
    write(p/'matching-contract.json',g.contract());write(p/'REVIEW-LEDGER.json',g.review_ledger(data));write(p/'reviewer-identity-policy.json',g.previous.reviewer_identity_policy())
    (p/'.gitattributes').write_text(g.ATTRIBUTES)
    state=g.f.read(g.ROOT/g.previous.PACKET/'WORKLIST.json');state.update(current_stage='54_WIC_DEV10_AI_review_vocabulary_and_current_adopted_consumer_contract',
        completed_in_this_scope=['WIC10reviews_64original_source_context_AI_observations_20unknown_followups','current_adopted_aliases_priorities5artifact_handoff_contract_bound','all6requested_AI_DEV_drafts_prepared_not_independent_gold'],
        next_task='WI1_WI2_WI3_review_debt_actual_partial_provider_error_and_installed_consumer_boundaries',last_verified_date='2026-10-19')
    state['AI_DEV_draft_progress'].update({'WI-C':'10DEV8modules64source_observations20unknown_followups_current_adopted_consumer_contract_semantic_gold_pending','remaining_work_items':[]})
    state['remaining_all'][0].update(status='all9_WI_AI_DEV_prepared_actual_path_review_and_independent_inputs_pending',task='WI1~3 미확정 의미/partial/backend_error/범위 밖/설치 역할; WI4/V1/V2 실제경로와 oracle; WI6 nonseed gold/실제쌍/warm; WI-SP 독립의미/실제overlay/ACL; WI-C 독립필수항목/우선순위/실제report와설치역할; 새독립DEV/FINAL·완전gold·matching/충분성·F01~08')
    write(p/'WORKLIST.json',state)
    (p/'REPORT.txt').write_text('WI-C DEV source review AI draft, not independent acceptance\n\n'
        '10 previously exposed functions across8 modules;64 exact original source/context observations and20 required followups with unknown answers. '
        '10 optional owned source/runtime/profile experiment proposals remain unexecuted. All claims keep original Git/file/span hashes and text. '
        'The original Status getStartEnd question is retained and normalized to the actual calculateBlockRange helper; historical packet unchanged. '
        'Finite AI vocabulary completeness does not establish complete product requirements, priority correctness or zero omissions.\n'
        'Adopted coding-agent d1ebafb8 full Git/source proof and original plugin hashes bound. Five Analyzer aliases and Planner high/medium/low max32 are already resolved, not reopened historical gaps. '
        'Current typed consumer retains six sections, source evidence/assumptions/unknowns, optional findings and explicit review criteria. Criteria IDs/section presence/syntax do not prove semantic coverage. '
        'Five typed artifacts and Analyzer->Planner->Orchestrator state/CLI/hook/hash handoff contract recorded. Source registration is not installed-role execution. '
        'Structural snapshot identity is not atomic cross-call proof; sequence.mmd is CFG flowchart, modules.mmd is retained-file membership with interprocedural unknown. Byte verification is not rendering or semantic quality. '
        'Actual project dual-tool calls/role execution/report/renderer receipts0; omitted-item and priority scores null; no model calls, agent dispatch, node, production changes or publication. '
        'Real names/contact details never required/requested; alias and actual artifact-bound approval evidence are separate. '
        'All6 newly requested AI drafts now prepared, independent0/FINAL0/formal0/9/freeze null/execution false. '
        'Native source1738(Go1282) and approved5 unchanged; prior complete native tests reused. Next review WI1~3 debt and independent preparation/ontology effect, see WORKLIST.json.\n')
    seal(p)


if __name__=='__main__':
    parser=argparse.ArgumentParser();parser.add_argument('--output',type=Path,default=g.ROOT/g.PACKET);parser.add_argument('--consumer-source',type=Path,required=True);a=parser.parse_args();prepare(a.output,a.consumer_source)
