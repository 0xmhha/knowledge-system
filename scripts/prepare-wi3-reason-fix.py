#!/usr/bin/env python3
"""Preserve fresh native logs, every MCP attempt and unchanged earlier evidence."""
import argparse,gzip,json,shutil
from pathlib import Path
import wi3_reason_fix as g
from importlib.util import module_from_spec,spec_from_file_location
sp=spec_from_file_location('writer',Path(__file__).with_name('prepare-acceptance-debt-diagnostic.py'));m=module_from_spec(sp);sp.loader.exec_module(m);write=m.write


def seal(p):
    write(p/'manifest.json',dict(kind=g.KIND,previous_source_revision=g.REVISION,
        tools=[dict(path=n,sha256=g.f.sha((g.ROOT/n).read_bytes())) for n in g.TOOLS],
        artifacts={x.relative_to(p).as_posix():g.f.sha(x.read_bytes()) for x in sorted(p.rglob('*')) if x.is_file() and x!=p/'manifest.json'}))


def copy_evidence(source,target):
    for x in sorted(source.rglob('*')):
        if not x.is_file():continue
        name=x.relative_to(source);y=target/name;y.parent.mkdir(parents=True,exist_ok=True)
        if x.name.endswith(('.jsonl','.stdout','.stderr')) or x.name in ('primary-results.json','retained-before.json','retained-after.json','tools.json'):
            y.with_name(y.name+'.gz').write_bytes(gzip.compress(x.read_bytes(),mtime=0))
        else:shutil.copyfile(x,y)


def prepare(p,work):
    p.mkdir(parents=True);native=work/'native'
    for x in native.iterdir():
        if not x.is_file() or x.name=='read-cks':continue
        y=p/'native'/x.name;y.parent.mkdir(exist_ok=True)
        if x.name.endswith(('.stdout','.stderr')):y.with_name(y.name+'.gz').write_bytes(gzip.compress(x.read_bytes(),mtime=0))
        else:shutil.copyfile(x,y)
    shutil.copyfile(native/'source-binding.json',p/'source-binding.json')
    (p/'native-change.patch.gz').write_bytes(gzip.compress((work/'native-change.patch').read_bytes(),mtime=0))
    copy_evidence(work/'runtime',p/'initial-startup');copy_evidence(work/'runtime-startup120',p/'concurrent-runtime');copy_evidence(work/'runtime-serial',p/'runtime')
    data=g.derive(packet=p)
    write(p/'SUMMARY.json',data['summary']);write(p/'private/current-cases.json',data['cases']);write(p/'matching-contract.json',g.contract())
    state=g.f.read(g.ROOT/g.previous.PACKET/'WORKLIST.json');state.update(current_stage='56_WI3_typed_seed_failure_reason_fresh_native_and_actual_retained_MCP',
        completed_in_this_scope=['typed_wrapped_seed_unresolved_reason_without_status_value_or_provenance_relaxation','fresh_source_bound_build_general_race_fmt_vet_lint','all_MCP_attempts_and7controlled_source_identity_regressions_preserved'],
        next_task='WI2_import_target_shadowing_alias_flow_and_WI3_unanchored_traversal_measurement_optional_resolver',last_verified_date='2026-10-19')
    state['remaining_all'][0]['task']='WI2 stdlib호출대상/alias-flow/9심볼·2분석·13unknown; WI1 select범위/53occurrence/3block·값종류; WI3 unanchored traversal/부하·startup·예산/measurement wrapper optional resolver; 전체 설치역할/독립 의미; WI4/V1/V2 실제경로/oracle·WI6 nonseed/쌍/warm·WI-SP overlay/ACL·WI-C 독립필수항목/실제전달; 새독립DEV/FINAL·gold·matching/충분성·F01~08'
    write(p/'WORKLIST.json',state)
    (p/'REPORT.txt').write_text('WI3 typed failure reason implementation and fresh native/MCP regression\n\n'
        'Wrapped ErrSeedUnresolved now reports status error/reason seed_unresolved. Unknown provider failures remain backend_error; budget_exhausted precedence preserved. '
        'No error message exposure or changed search input, candidate/source identity, null/partial status or provenance. '
        'Aggregate signal metadata is not a complete per-backend failure timeline; a reason does not prove exclusive cause. '
        'New native tests cover wrapped failures, budget precedence, no private detail leakage, and missing seed before/after success with both seed anchors retained.\n'
        'Fresh whole-module native build/general/race/fmt/vet/pinned lint/boundaries/docs and runtime binary built from owned clone3869a548 plus exactly2bound Go source changes. '
        '1738 native/dependency inputs and5approved input hashes bound; no historical native test relabelled current. Binary/source mode is explicit working-tree build, not new HEAD binary provenance.\n'
        'Every MCP attempt retained. Initial20second initialize wait timed out before any tool call:20planned primary inputs unreached, EOFexit0. '
        'Next explicit120second startup run used original20symptom inputs/1000ms budgets during native validation;11tool errors and9partial responses retained as concurrent-runtime. '
        'Serial runtime runs only after native process completed, with same original20inputs/budgets plus7controlled requests. '
        'Missing-only/both missing-known orders preserve error seed_unresolved and null, known-only observes graph results, foreign project/bad cap/stale commit refuse. '
        'Original primary20 results remain immutable; these are exposed-input regressions, not new independent DEV/FINAL or retrieval quality acceptance. '
        'All tool errors/partial/unreached slots retained; source/graph/vector before/after equality and actual local BGE-M3 digest/runtime before/after recorded. '
        'Startup anchor embeddings not physically counted; no paid/external LLM, installed role, production activation or publication. '
        'Initial timeout and concurrent-run errors are not hidden or used to relax tool budgets; no production startup/performance SLA pass. '
        'WI1/WI2 semantic/target/alias and WI3 traversal/measurement source-inspected optional resolver issue remain. '
        'Independent0/FINAL0/formal0/9/freeze null/execution false; no names or contact details required. All remaining and next: WORKLIST.json.\n'
        +json.dumps(data['summary'],ensure_ascii=False,indent=2)+'\n')
    seal(p)


if __name__=='__main__':
    parser=argparse.ArgumentParser();parser.add_argument('--output',type=Path,default=g.ROOT/g.PACKET);parser.add_argument('--workspace',type=Path,required=True);a=parser.parse_args();prepare(a.output,a.workspace)
