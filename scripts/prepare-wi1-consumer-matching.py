#!/usr/bin/env python3
"""Supplement frozen WI1 drafts; retain unknowns and actual consumer evidence."""
import argparse
import gzip
import json
from pathlib import Path
import shutil
import wi1_consumer_matching as g
old=g.old

def write(p,v):p.parent.mkdir(parents=True,exist_ok=True);p.write_text(json.dumps(v,ensure_ascii=False,indent=2)+'\n')
def compress(p,v):p.parent.mkdir(parents=True,exist_ok=True);p.write_bytes(gzip.compress(old.canonical(v),mtime=0))
def seal(p):
    sources=[str(old.PACKET/'manifest.json'),str(old.FUNCTIONS),str(old.SOURCES),'go.mod','go.sum',str(g.BASE/'followups/stablenet-comparison-v1/source-binding.json')]
    write(p/'manifest.json',dict(kind=g.KIND,previous_source_revision=g.REVISION,tools=[dict(path=x,sha256=old.sha((g.ROOT/x).read_bytes())) for x in g.TOOLS],sources=[dict(path=x,sha256=old.sha((g.ROOT/x).read_bytes())) for x in sources],artifacts={f.relative_to(p).as_posix():old.sha(f.read_bytes()) for f in sorted(p.rglob('*')) if f.is_file() and f.name!='manifest.json'}))
def prepare(p,runtime):
    p.mkdir(parents=True);shutil.copytree(runtime,p/'runtime')
    # Compress large raw wire logs deterministically, keeping exact stdout bytes.
    for f in list((p/'runtime').rglob('*.jsonl')):
        f.with_suffix(f.suffix+'.gz').write_bytes(gzip.compress(f.read_bytes(),mtime=0));f.unlink()
    cs=old.read(g.ROOT/old.FUNCTIONS)['functions'];obs={(o['path'],o['span']['start']):o for o in old.packed(g.ROOT/old.PACKET/'observations.json.gz')};audits=old.packed(p/'runtime/types-project.json.gz');raw=old.read(p/'runtime/project-results.json');cases=[]
    for c,r in zip(cs,raw):
        o=obs[(c['source']['path'],c['source']['start_byte'])];v=g.derive(c,o,r['response']['analysis'],audits);compress(p/'cases'/(c['candidate_id']+'.json.gz'),v);cases.append(v)
    controls=old.read(p/'runtime/control-results.json');co={o['name']:o for o in old.packed(g.ROOT/old.PACKET/'controls/observations.json.gz')}
    for r in controls:write(p/'controls'/(r['name']+'.json'),g.match(co[r['name']],r['response']['analysis']))
    summary=g.summarize(cases,controls);write(p/'SUMMARY.json',summary);write(p/'REVIEW-LEDGER.json',g.review_ledger(cases,obs))
    write(p/'matching-contract.json',dict(status='AI_supplement_proposed_not_frozen',reference_manifest=str(old.PACKET/'manifest.json'),project_source_commit=old.SOURCE_COMMIT,
         source_identity='selected canonical symbol + project dataset; commit/function SHA/graph snapshot SHA required; no name-only or file/line-only choice',
         locations='absolute UTF8 half-open bytes; line/column and raw IDs not sufficient',
         phases='read_write splits use before definition; exact declaration identity + owner + phase; consumer address/reference/aggregate/unknown roles preserved',
         entry='virtual execution source; function type location is not a variable declaration; individual entry definitions match their identifier spans',
         implicit_return='Rbrace anchor normalized to one-byte retained brace; explicit bare named return uses full return span, not declaration coordinates',
         compile_time_constant='non-runtime ValueSpec source node (block -1); not assigned invented executable CFG position',
         blocks='node sequence/kind/reachability and ordered successor constraints; unresolved empty/duplicate blocks remain unknown',
         branches='successor ordinal translates only through mapped source/target and raw consumer cfg edge; true/false labels never guessed from ordinal alone',
         unknown='lexical reachability, value category, concrete value, memory/alias/capture/range, source matching and analyzer status are separate axes; absent/unmatched edge is never negative semantic proof',
         scalar='scalar kind does not identify concrete runtime value; calls, exceptions, short circuit and runtime feasibility are outside scope',
         evidence_source='source-derived overlay independent of candidate edges; responses used only for matching and disagreement ledger; shared CFG dependency remains a common-mode limitation',
         formal_gold_approved=False,independent_review=None,precision_recall=None,scoring_allowed=False))
    work=old.read(g.ROOT/old.PACKET/'WORKLIST.json');work.update(current_stage='45_WI1_unknown_value_domains_synthetic_and_actual_consumer_matching',completed_in_this_scope=['WI1_layered_unknown_semantics','WI1_partial_file_vs_isolated_type_evidence','WI1_retained_project30_actual_CKS_matching','WI1_synthetic14_coordinate_controls'],next_task='WI2_DEV_inputs_and_complete_AI_drafts_then_remaining7; WI1_disagreement_independent_review_pending')
    work['remaining_all'][0]['status']='WI1_supplement_prepared_other8_AI_drafts_and_independent_inputs_pending';work['remaining_all'][0]['task']='나머지8개 WI DEV/AI 초안; WI1 actual 매칭 누락/lexical 차이·값 의미의 독립 검토; D04 역할·독립성, 새 DEV/FINAL 가족·완전 gold·매칭·충분성·F01~08 동결'
    write(p/'WORKLIST.json',work)
    (p/'REPORT.txt').write_text('WI-1 AI DEV supplemental semantics and real consumer matching\n\nD04 미정. Independent approval0, formal0/9, freeze null. AI proposal only.\n\nUnknown is not false, not an empty result, and not analyzer partial status.\nLexical def-use path existence, scalar/reference/aggregate/unknown category, concrete value, memory semantics and matching certainty are distinct.\nAll concrete runtime values remain unknown. Reference/aggregate bindings never prove memory contents or alias relationships.\nFull-file partial typing and isolated consumer typing are recorded independently with errors/type strings; neither imports or type-checks a complete package.\nFrozen stage44 labels are preserved; the supplemental value-flow unknown axis may be more conservative.\nSource entry, Rbrace returns, explicit bare returns and compile-time constants have separate coordinate rules. Read/write phases never merge.\nOriginal project30 and synthetic14 controls are separate populations. Source gold is never copied from consumer output.\nSame file/line discovery can return multiple artificial receiver candidates; select a unique canonical identity before pinned query.\nUnmatched source nodes/bindings/phases/blocks and lexical differences remain explicit AI review debt, no precision/recall or acceptance score.\nThe consumer does not emit reference/unknown absence as proof of safety. Control ordinal translation is a mapping contract, not semantic condition feasibility.\n\nDetailed counts: SUMMARY.json; per-case source/type/mapping/edge-comparison: cases/*.json.gz.\nRaw real CKS project30 and owned ckg synthetic14 responses: runtime/. Normal EOF receipts, pin refusals and retained inventory equality recorded.\nNo new paid/embedding model call. Prior native source validation reused only after source binding check; no fresh full native Go test claim.\nCurrent phase and all remaining work: WORKLIST.json.\n')
    seal(p)
if __name__=='__main__':
    a=argparse.ArgumentParser();a.add_argument('--runtime',type=Path,required=True);a.add_argument('--output',type=Path,default=g.ROOT/g.PACKET);v=a.parse_args();prepare(v.output,v.runtime)
