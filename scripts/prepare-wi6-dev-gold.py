#!/usr/bin/env python3
"""Write source-only WI-6 DEV gold drafts, never execute the stress program."""
import argparse,gzip,json,shutil
from pathlib import Path
import wi6_dev_gold as g
from importlib.util import module_from_spec,spec_from_file_location
sp=spec_from_file_location('draft_writer',Path(__file__).with_name('prepare-wi4-dev-gold.py'));writer=module_from_spec(sp);sp.loader.exec_module(writer);write=writer.write


def seal(p):
    write(p/'manifest.json',dict(kind=g.KIND,previous_source_revision=g.REVISION,
        tools=[dict(path=n,sha256=g.f.sha((g.ROOT/n).read_bytes())) for n in g.TOOLS],
        artifacts={x.relative_to(p).as_posix():g.f.sha(x.read_bytes()) for x in sorted(p.rglob('*')) if x.is_file() and x!=p/'manifest.json'}))


def prepare(p,source):
    p.mkdir(parents=True);shutil.copytree(source,p/'private/source');data=g.derive(source=p/'private/source')
    (p/'.gitattributes').write_text(g.ATTRIBUTES)
    for name,key in [('public/DEV-inputs.json','public'),('private/location-catalog.json','locations'),('SUMMARY.json','summary')]:write(p/name,data[key])
    (p/'private/AI-gold.json.gz').write_bytes(gzip.compress(g.f.canonical(data['gold']),mtime=0))
    write(p/'matching-contract.json',g.contract());write(p/'REVIEW-LEDGER.json',g.review_ledger(data))
    state=g.f.read(g.ROOT/g.previous.PACKET/'WORKLIST.json')
    state.update(current_stage='52_WI6_DEV20_selected20_function_AI_relevance_and_seed_excluded_matching',
        completed_in_this_scope=['WI6_original20_DEV_questions_and_original_Git14files','WI6_complete400_bounded_AI_relevance_noise_unknown_units',
            'WI6_seed_excluded_scope_same_pair_and_warm_measurement_proposals'],
        next_task='WISP_DEV_manual_automatic_policy_security_untagged_and_refusal_AI_drafts',last_verified_date='2026-10-19')
    state['AI_DEV_draft_progress'].update({'WI-6':'20_DEV400_bounded_AI_units19production1source_only_stress_actual_pairs_warm_and_independence_pending','remaining_work_items':['WI-SP','WI-C']})
    state['remaining_all'][0].update(status='WI1_WI2_WI3_WI4_V1_V2_WI6_AI_DEV_prepared_other2_and_independent_inputs_pending',
        task='WI-SP/C AI DEV; WI1~3 검토 부채·WI4/V1/V2 실제경로/oracle·WI6 더넓은 nonseed gold/실제쌍/warm/역할; D04·새 DEV/FINAL·완전 독립 gold·matching/충분성·F01~08 동결')
    write(p/'WORKLIST.json',state)
    (p/'REPORT.txt').write_text('WI-6 DEV20 source-grounded finite AI relevance/noise drafts\n\n'
        'D04 미정; independent0, FINAL0, formal0/9, freeze null/execution false. Original20 questions and20 selected functions in14 source files. '
        '19 production proposals plus1 development stress SOURCE ONLY: do not run main, start nodes/mine/send transactions or include it as an operational family.\n'
        'Original Git commit/file/function proof and40 source anchors verified. All20 full function bodies manually read;400 question/function units labelled '
        'with direct seed, explicit producer/callee/consumer context, local noise and unresolved transitive/runtime context. '
        'Scope is local implementation/explicit source context among selected20, not entire repository or all transitive relevance. '
        'Same intent/file/topic never automatically supplies a relevant edge; Prune/RecoverPruning and Init/newLimbo have explicit calls, '
        'WriteBadBlock/ReadAllBadBlocks share explicit stored representation. Other uncertain routes remain unknown.\n'
        'Seed is reported separately and excluded from both primary scoring universes, with inclusion sensitivity proposed. '
        'Most selected universes have no independently relevant nonseed context: undefined recall remains inconclusive, not0/1 or an improvement claim. '
        'Additional nonseed helpers and independent semantics are necessary before assessing graph expansion quality. '
        'Outside returned functions remain unlabelled/unknown; never automatic noise or refill after capped slots/dedup.\n'
        'Paired baseline/expansion need same actual resolved seed, retained project/dataset/commit/snapshot, arm caps/order and all planned failed/partial slots. '
        'Actual seed/canonical/envelope adapter and paired MCP receipts are not collected in this source-authoring stage. '
        'Warm measurement is a NEW unfrozen proposal:3 warmup pairs +30 measured pairs per production case, owned reused stdio process/connection, '
        'single-concurrency alternating AB/BA, end-to-end perf_counter_ns, nearest-rank p95 per arm and ratio of the p95s. '
        'Do not import another experiment\'s warm20 or confuse ratio of p95 with p95 of pairwise ratios. '
        'Retain raw warmup/error/partial/missing samples and pins; insufficient pairs/nonpositive baseline/changed environment is inconclusive.\n'
        'No actual baseline/expansion/warm/model/target execution or quality score. Native source1738(Go1282)/approved-input5 unchanged, prior full Go tests reused. '
        'Prior ticket evidence remains byte unchanged at875cfc29; raw-diff format correction recorded separately. '
        'Review private/AI-gold.json.gz and location-catalog.json, REVIEW-LEDGER.json, next and all remaining WORKLIST.json.\n'
        +json.dumps(data['summary'],ensure_ascii=False,indent=2)+'\n')
    seal(p)


if __name__=='__main__':
    parser=argparse.ArgumentParser();parser.add_argument('--output',type=Path,default=g.ROOT/g.PACKET);parser.add_argument('--source-capture',type=Path,required=True)
    args=parser.parse_args();prepare(args.output,args.source_capture)
