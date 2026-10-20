#!/usr/bin/env python3
"""Seal direct read-only provider receipts separately from primary WI3 results."""
import argparse,gzip,json,shutil
from pathlib import Path
import acceptance_debt_diagnostic as g
from importlib.util import module_from_spec,spec_from_file_location
sp=spec_from_file_location('writer',Path(__file__).with_name('prepare-wic-dev-gold.py'));m=module_from_spec(sp);sp.loader.exec_module(m);write=m.write


def seal(p):
    write(p/'manifest.json',dict(kind=g.KIND,previous_source_revision=g.REVISION,
        tools=[dict(path=n,sha256=g.f.sha((g.ROOT/n).read_bytes())) for n in g.TOOLS],
        source_inputs=[dict(path=str(n),sha256=g.f.sha((g.ROOT/n).read_bytes())) for n in g.SOURCE_INPUTS],
        artifacts={x.relative_to(p).as_posix():g.f.sha(x.read_bytes()) for x in sorted(p.rglob('*')) if x.is_file() and x!=p/'manifest.json'}))


def prepare(p,runtime,prepared):
    p.mkdir(parents=True)
    for folder in ('direct','all-symbols'):
        for x in sorted((runtime/folder).iterdir()):
            target=p/'runtime'/folder/x.name;target.parent.mkdir(parents=True,exist_ok=True)
            if x.name in ('responses.json','before.json','after.json','tools.json','mcp.stdin.jsonl','mcp.stdout.jsonl'):
                target.with_name(target.name+'.gz').write_bytes(gzip.compress(x.read_bytes(),mtime=0))
            else:shutil.copyfile(x,target)
    for name in ('direct.py','all-symbols.py'):shutil.copyfile(runtime/name,p/'runtime'/(name+'.txt'))
    shutil.copyfile(prepared/'go-stablenet.yaml',p/'runtime/config.txt')
    write(p/'runtime/identity.json',dict(binary_sha256=g.f.sha((prepared/'read-cks').read_bytes()),source_revision='38dd5c5449b1e57759e2532090dce101b3ad9124',
        new_paid_or_external_model_calls=0,scope='actual original binary, current direct graph-only calls; source and dataset inventory checked separately; not a historical backend trace',
        runner_sha256={name:g.f.sha((runtime/name).read_bytes()) for name in ('direct.py','all-symbols.py')}))
    data=g.derive(packet=p)
    for name,key in [('SUMMARY.json','summary'),('private/original-case-diagnostic.json','cases'),('private/current-provider-diagnostic.json','rows'),('REVIEW-LEDGER.json','debt')]:write(p/name,data[key])
    write(p/'matching-contract.json',g.contract())
    state=g.f.read(g.ROOT/g.previous.PACKET/'WORKLIST.json');state.update(current_stage='55_WI123_debt_triage_and_actual_WI3_graph_provider_diagnostic',
        completed_in_this_scope=['original20partial102correlated_generic_errors_traced_to_sufficient_unresolved_seed_path','actual40symbols80direct_graph_calls_and2unindexed_refusals_preserved','WI1_58_and_WI2_unknown_resolution_alias_debt_prioritized_not_marked_resolved'],
        next_task='WI3_expose_typed_seed_unresolved_reason_and_shared_seed_tests_then_fresh_native_and_MCP_verification',last_verified_date='2026-10-19')
    state['remaining_all'][0]['task']='WI3 typed seed_unresolved/실제provider/budget 구분 구현·native/MCP 재검증·unanchored traversal; WI1 select범위/53occurrence/3block·값종류; WI2 잘못된 stdlib호출대상/alias-flow/9심볼·2분석·13unknown; 전체 설치역할/독립 의미; WI4/V1/V2 실제경로/oracle·WI6 nonseed/쌍/warm·WI-SP overlay/ACL·WI-C 독립필수항목/실제전달; 새독립DEV/FINAL·gold·matching/충분성·F01~08'
    write(p/'WORKLIST.json',state)
    (p/'REPORT.txt').write_text('WI1-3 review debt and current actual WI3 graph provider diagnostic\n\n'
        'Original20symptom requests and all68candidate slots preserved;20partial.17cases have26unresolved seed slots. '
        'Compose explicitly marks all enriched candidates error when ANY seed cannot be resolved; later successful seed queries retain that error. '
        'Thus51impact+51concurrency generic error labels share a sufficient unresolved-seed source path; they are not102 independent failed backend calls. '
        'The other3cases RQ-003/RQ-004/RQ-019 have9observed slots per signal.8slots per signal remain not_requested. '
        'No original per-backend typed error trace exists, so this is not proof that all past calls succeeded or that no transient failure occurred.\n'
        'Current actual original read-cks binary and immutable same retained graph/dataset: all40distinct observed symbol names,80direct impact/concurrency requests succeeded. '
        'Separate exploratory6calls include4known-symbol successes and2unindexed-symbol explicit refusals. Both owned stdio sessions exit0;80call run before/after inventory matches original dataset. '
        'Both CKS model-list identity probes were sandbox-denied and CKV ran degraded; no real embedding inference occurred. These are real graph-only checks, not full retrieval/model readiness. No new semantic/embedding queries, paid/external LLM, target node or installed-agent role. Direct output has legacy citation scope; it does not replace retained primary receipts, create new retrieval precision/recall or establish whole-program safety.\n'
        'WI1 retains58debt entries:53 implicit occurrence phases,3ambiguous unreachable blocks,2select lifetime lexical differences. '
        'WI2 retains13 source unknowns,9 missing symbols/2unavailable analyses,4returned source disagreements and2unreturned AI positives. '
        'Prioritize typed seed_unresolved vs provider/budget reason, then stdlib target shadowing/alias propagation and select scope. Do not weaken matching, use name-only source joins or rewrite gold to agree with consumer. '
        'All66outside/unmapped WI3slots remain unknown, and independent semantic/installed role review remains pending. '
        'Real names/contact details never required. AI drafts6/6 prepared; independent0/FINAL0/formal0/9/freeze null/execution false. '
        'Original artifacts remain byte-exact. Native source1738(Go1282)/approved5 unchanged; prior native tests reused, no native implementation fix yet. '
        'Next implementation and all remaining work: WORKLIST.json.\n')
    seal(p)


if __name__=='__main__':
    parser=argparse.ArgumentParser();parser.add_argument('--output',type=Path,default=g.ROOT/g.PACKET);parser.add_argument('--runtime',type=Path,required=True);parser.add_argument('--prepared',type=Path,required=True);a=parser.parse_args();prepare(a.output,a.runtime,a.prepared)
