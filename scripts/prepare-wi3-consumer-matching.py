#!/usr/bin/env python3
"""Seal context supplement and actual retained stdio observations; no acceptance."""
import argparse
import gzip
import json
from pathlib import Path
import shutil
import wi3_consumer_matching as g


def write(path,value):
    path.parent.mkdir(parents=True,exist_ok=True);path.write_text(json.dumps(value,ensure_ascii=False,indent=2,allow_nan=False)+'\n')


def packed(path,value):
    path.parent.mkdir(parents=True,exist_ok=True);path.write_bytes(gzip.compress(g.f.canonical(value),mtime=0))


def seal(p):
    write(p/'manifest.json',dict(kind=g.KIND,previous_source_revision=g.REVISION,
        tools=[dict(path=n,sha256=g.f.sha((g.ROOT/n).read_bytes())) for n in g.TOOLS],
        artifacts={x.relative_to(p).as_posix():g.f.sha(x.read_bytes()) for x in sorted(p.rglob('*')) if x.is_file() and x!=p/'manifest.json'}))


def prepare(p,runtime,audit):
    p.mkdir(parents=True);shutil.copytree(runtime,p/'runtime')
    shutil.copytree(audit,p/'source-audit',ignore=shutil.ignore_patterns('main.go','observer'))
    for path in (p/'runtime').rglob('*.jsonl'):
        path.with_suffix(path.suffix+'.gz').write_bytes(gzip.compress(path.read_bytes(),mtime=0));path.unlink()
    values=g.derive(g.ROOT,p)
    for name in ('overlay','cases'):packed(p/'private'/(name+'.json.gz'),values[name])
    write(p/'SUMMARY.json',values['summary']);write(p/'matching-contract.json',g.matching_contract())
    write(p/'REVIEW-LEDGER.json',g.review_ledger(values))
    (p/'SOURCE-CONTEXT-REVIEW.txt').write_text(json.dumps(values['overlay']['context'],ensure_ascii=False,indent=2)+'\nRemaining source unknowns:\n'+'\n'.join(json.dumps(r,ensure_ascii=False,sort_keys=True) for r in values['overlay']['unknowns'])+'\n')
    (p/'CONSUMER-REVIEW.txt').write_text('\n'.join(json.dumps(c,ensure_ascii=False,sort_keys=True) for c in values['cases'])+'\n')
    state=g.f.read(g.ROOT/g.old.PACKET/'WORKLIST.json');state.update(current_stage='49_WI3_source_context_and_actual_partial_symptom_top10_matching',
        completed_in_this_scope=['WI3_source_unknown_and_static_support_context_overlay','WI3_20_actual_symptom_only_CKS_queries_and_retained_provenance','WI3_rank_preserving_exact_source_adapter_and_refusal_boundaries'],
        next_task='WI4_DEV_inputs_and_complete_bounded_AI_outcome_drafts',last_verified_date='2026-10-19')
    state['AI_DEV_draft_progress']['WI-3']='20_DEV_400_AI_units_actual_symptom_partial_matching_observed_not_independent_gold'
    state['remaining_all'][0].update(status='WI1_WI2_WI3_AI_DEV_and_observed_matching_prepared_other6_and_independent_inputs_pending',task='WI4/V1/V2/6/SP/C DEV·완전 AI 초안; WI1/WI2 검토 부채와 WI3 source unknown·partial/provider/범위 밖/미연결·설치 역할/독립 의미 부채; D04 역할/독립성·새 DEV/FINAL 가족·완전 독립 gold·매칭·충분성·F01~08 동결')
    write(p/'WORKLIST.json',state)
    (p/'REPORT.txt').write_text('WI-3 source context and actual symptom-only retained CKS matching — AI DEV only\n\nD04 미정, independent approval0, FINAL0, formal0/9, freeze null/execution false.\nStage48 original20 DEV/400 AI units and tools remain byte-exact at8553fe97. A separate source-only overlay records two runReorg static supporting-context refinements; the source labels never consume retrieval outputs. Runtime causality and independent semantic approval remain absent.\nActual original20 symptom-only requests retain1000ms/max_candidates10/seeds3/enriched3/calls32/depth2. No source hints injected, no primary retries, no success-only denominator. Raw stdio, partial/unknown/unsupported/error signals and stop reasons retained.\nAll returned candidate and nested signal citations are checked against project/dataset/source snapshot/origin/commit, original file hash and exact line content hash. Source bytes copied from retained archive and compared with original Git commit.\nThe prior strict matching proposal maps0 actual candidates. A separate supplemental adapter admits only exact AST-associated Doc+complete FuncDecl envelopes and maps2;66 returned slots remain outside selected20/unlabelled. Exact function/witness paths remain supported. This observed adapter is an unfrozen AI proposal, not post-hoc quality scoring. Arbitrary overlap and name-only joins remain unknown. Original ranks/slots preserved; no rank11 refill or independent quality score.\nWrong commit, invalid candidate cap and foreign-project expansion are actual refusal probes. Normal EOF and retained before/after inventory equality recorded. Local BGE-M3/native1024 digest and Ollama0.40.4 identified before/after. No paid/external model or installed-agent role run.\nTechnical connection/matching observation is complete; partial/provider coverage, outside20 labels, independent gold/metrics/families and role investigation remain debt in REVIEW-LEDGER.json. Full native Go validation reused by unchanged source binding, not rerun here.\nNext WI-4 DEV/complete bounded AI outcome drafts. All remaining work: WORKLIST.json.\n'+json.dumps(values['summary'],ensure_ascii=False,indent=2)+'\n')
    seal(p)


if __name__=='__main__':
    parser=argparse.ArgumentParser();parser.add_argument('--runtime',type=Path,required=True);parser.add_argument('--source-audit',type=Path,required=True);parser.add_argument('--output',type=Path,default=g.ROOT/g.PACKET);a=parser.parse_args();prepare(a.output,a.runtime,a.source_audit)
