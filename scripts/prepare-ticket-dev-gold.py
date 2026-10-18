#!/usr/bin/env python3
"""Write one shared WI-V1/V2 DEV ticket, never execute or approve it."""
import argparse
from pathlib import Path
import ticket_dev_gold as g
from importlib.util import module_from_spec, spec_from_file_location

sp=spec_from_file_location('draft_writer',Path(__file__).with_name('prepare-wi4-dev-gold.py'))
writer=module_from_spec(sp);sp.loader.exec_module(writer)
write=writer.write


def seal(p):
    write(p/'manifest.json',dict(kind=g.KIND,previous_source_revision=g.REVISION,
        tools=[dict(path=n,sha256=g.f.sha((g.ROOT/n).read_bytes())) for n in g.TOOLS],
        artifacts={x.relative_to(p).as_posix():g.f.sha(x.read_bytes()) for x in sorted(p.rglob('*')) if x.is_file() and x!=p/'manifest.json'}))


def prepare(p):
    p.mkdir(parents=True);data=g.derive()
    for name,key in [('public/DEV-ticket.json','public'),('private/requirements.json','requirements'),('private/candidates.json','candidates'),('private/AI-matrix.json','matrix'),('private/AI-membership.json','membership'),('private/source-anchors.json','anchors'),('SUMMARY.json','summary'),('private/secondary-derivation.json','secondary')]:write(p/name,data[key])
    for key in data['overlays']:
        for directory,suffix,blob in [('overlays','.go.txt',data['overlays'][key]),('diffs','.diff.txt',data['diffs'][key])]:
            x=p/'private'/directory/(key+suffix);x.parent.mkdir(parents=True,exist_ok=True);x.write_bytes(blob)
    write(p/'matching-contract.json',g.contract());write(p/'REVIEW-LEDGER.json',g.review_ledger(data))
    state=g.f.read(g.ROOT/g.previous.PACKET/'WORKLIST.json')
    state.update(current_stage='51_WIV1_V2_shared_DEV_ticket_conditional_oracle_and_candidate_AI_drafts',
        completed_in_this_scope=['WIV1_12_source_bound_requirement_AI_drafts','WIV1_25_scenario_candidate_300_requirement_units','WIV2_5role4byte_candidate_local_membership_and_actualAPI_unknown'],
        next_task='WI6_DEV_paired_base_expansion_complete_bounded_AI_relevance_and_noise_drafts',last_verified_date='2026-10-19')
    state['AI_DEV_draft_progress'].update({'WI-V1':'one_shared_exposed_DEV_ticket12requirements_AI_oracle_not_independent',
        'WI-V2':'same_shared_ticket5roles4bytes_local_AI_membership_actualAPI_unknown5',
        'remaining_work_items':['WI-6','WI-SP','WI-C']})
    state['remaining_all'][0].update(status='WI1_WI2_WI3_WI4_V1_V2_AI_DEV_prepared_other3_and_independent_inputs_pending',
        task='WI6/SP/C AI DEV; WI1~3 검토 부채와 WI4/V1/V2 실제 경로·독립 oracle/정답·소유 정리/설치 역할; D04·새 DEV/FINAL·완전 독립 gold·matching/충분성·F01~08 동결')
    write(p/'WORKLIST.json',state)
    (p/'REPORT.txt').write_text('WI-V1/V2 shared DEV ticket source/conditional AI drafts\n\n'
        'D04 미정, independent0, formal0/9, FINAL0, freeze null/execution false. One already-exposed ticket supports both work items, not2 independent tickets. '
        'Five scenarios share the same defect hypothesis. Five candidate/control roles contain only4 distinct method byte variants; REINTRO is ORIGINAL alias.\n'
        '12 explicit source-bound requirements,25 candidate-scenario cells,300 completely classified requirement units: '
        'conditional source fit/conflict, actual runtime unknown and not applicable. Raw old fixture labels do not author the matrix. '
        'Each candidate has original full-file/method hashes, exact witnesses, a proposed full-file overlay and diff SHA. '
        'No candidate was applied to a project commit/tree or run on an actual API in this stage.\n'
        'A/B are conditional fits only within5 finite live-loop extracted-method schedules; ORIGINAL/REINTRO retain the blocked-send cycle; '
        'EARLY lacks request/removal/ack despite fast return. Actual API/product membership of all5 remains unknown and independent acceptable set is null. '
        'No empty approved set, selected candidate, performance/fairness score or product-defect acceptance is invented.\n'
        'Public eth_unsubscribe true closes server-subscription err/removes registry but does not wait for downstream deferred Unsubscribe. '
        'Actual subscription transport/registration/callback entry and channel/object/source identity are required before root-cause RED or GREEN. '
        'Unsupported/missing/error API execution remains error/unknown. A complete controlled event trace oracle is a DEV proposal, not evidence of an actual API run. '
        'Separate source wait-graph derivation shares AI/source bias and is explicitly not an independent executable oracle.\n'
        'All-bad has no selection; tie is allowed; mandatory unknown/incomplete membership is DEFERRED. '
        'Actual normal/mutant/control tiers, applied source/tree/binary/driver pins, ownership cleanup, finalHEAD/four installed roles and independent sufficient DEV/FINAL remain pending.\n'
        'No target/MCP/model/node execution or quality score. Native source1738 files(Go1282)/approved inputs5 unchanged; prior full native tests reused. '
        'Prior stage50 evidence is preserved at20f470f2. Source/readable drafts: private/requirements.json, candidates.json, AI-matrix.json and AI-membership.json. '
        'Next and all remaining: WORKLIST.json.\n')
    seal(p)


if __name__=='__main__':
    parser=argparse.ArgumentParser();parser.add_argument('--output',type=Path,default=g.ROOT/g.PACKET)
    prepare(parser.parse_args().output)
