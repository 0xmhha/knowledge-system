#!/usr/bin/env python3
"""Preserve actual attempts, obsolete run and the final source-bound proof."""
import argparse,gzip,json,shutil
from pathlib import Path
import wi3_walk_budget as g


def write(path,value):
    path.parent.mkdir(parents=True,exist_ok=True);path.write_text(json.dumps(value,ensure_ascii=False,indent=2)+'\n')


def compressed(raw):return gzip.compress(raw,mtime=0)


def seal(p):
    write(p/'manifest.json',dict(kind=g.KIND,previous_source_revision=g.REVISION,
        tools=[dict(path=n,sha256=g.f.sha((g.ROOT/n).read_bytes()))for n in g.TOOLS],
        artifacts={x.relative_to(p).as_posix():g.f.sha(x.read_bytes())for x in sorted(p.rglob('*'))if x.is_file()and x!=p/'manifest.json'}))


def native_copy(src,dst):
    dst.mkdir(parents=True)
    for x in sorted(src.iterdir()):
        if not x.is_file()or x.name=='read-cks':continue
        y=dst/x.name
        if x.suffix in ('.stdout','.stderr'):y.with_name(y.name+'.gz').write_bytes(compressed(x.read_bytes()))
        else:shutil.copyfile(x,y)


def prepare(p,work):
    if p.exists()and any(x.name!='SCOPE.json'for x in p.iterdir()):raise ValueError('refusing evidence overwrite')
    p.mkdir(parents=True,exist_ok=True)
    if not(p/'SCOPE.json').exists():shutil.copyfile(g.ROOT/g.PACKET/'SCOPE.json',p/'SCOPE.json')
    shutil.copyfile(work/'source-binding.json',p/'source-binding.json')
    (p/'code-change.patch.gz').write_bytes(compressed((work/'native-change.patch').read_bytes()))
    native_copy(work/'native-validated',p/'native')
    for name in ('installed-tools.json','lint-config-binding.json'):shutil.copyfile(work/name,p/'native'/name)
    shutil.copyfile(work/'candidate-binary.json',p/'native/candidate-binary.json')
    (p/'native/preflight-lint.txt.gz').write_bytes(compressed((work/'preflight-lint.txt').read_bytes()))
    native_copy(work/'native-final',p/'pre-lint-native')
    (p/'pre-lint-native/code-change.patch.gz').write_bytes(compressed((work/'pre-lint-native-change.patch').read_bytes()))
    prelint=g.f.read(p/'pre-lint-native/source-binding.json')
    for name in g.CHANGED:
        raw=(work/'native-source-final'/name).read_bytes()
        if g.f.sha(raw)!=prelint['source_sha256'][name]:raise ValueError('pre-lint owned source changed')
        dest=p/'pre-lint-native/source'/(name+'.txt.gz');dest.parent.mkdir(parents=True,exist_ok=True);dest.write_bytes(compressed(raw))
    native_copy(work/'native',p/'superseded-native')
    shutil.copyfile(work/'supersession.json',p/'supersession.json')
    (p/'superseded-native/code-change.patch.gz').write_bytes(compressed((work/'superseded-native-change.patch').read_bytes()))
    obsolete=g.f.read(p/'superseded-native/source-binding.json')
    for name in g.CHANGED:
        raw=(work/'native-source'/name).read_bytes()
        if g.f.sha(raw)!=obsolete['source_sha256'][name]:raise ValueError('obsolete owned source changed')
        dest=p/'superseded-native/source'/(name+'.txt.gz');dest.parent.mkdir(parents=True,exist_ok=True);dest.write_bytes(compressed(raw))
    for phase in ('before','after'):
        src=work/(phase+'-runtime')
        for x in sorted(src.rglob('*')):
            if not x.is_file():continue
            y=p/(phase+'-runtime')/x.relative_to(src);y.parent.mkdir(parents=True,exist_ok=True)
            if x.suffix in('.jsonl','.txt')or x.name in('responses.json','before.json','after.json'):y.with_name(y.name+'.gz').write_bytes(compressed(x.read_bytes()))
            elif x.name=='config.yaml':shutil.copyfile(x,y.with_name('config.txt'))
            else:shutil.copyfile(x,y)
    for name in ('before-core.jsonl','before-mcp.jsonl','before-budget-reason.jsonl','before-seed-error.jsonl','targeted-final.jsonl'):
        dest=p/'targeted'/(name+'.gz');dest.parent.mkdir(exist_ok=True);dest.write_bytes(compressed((work/name).read_bytes()))
    d=g.derive(packet=p);write(p/'SUMMARY.json',d['summary']);write(p/'runtime-comparison.json',d['runtime'])
    state=g.f.read(g.ROOT/g.previous.PACKET/'WORKLIST.json');state.update(current_stage='60_WI3_verified_walk_total_budget_dependency_reasons_and_request_measurement',
        completed_in_this_scope=['snapshot_verified_alias_walk_membership_cache_and_original_retrieval_preservation',
            'within_total_budget_final_verification_and_safe_dependency_seed_evidence_error_classification',
            'per_request_logical_measurements_fresh_native_and_actual124_attempts'],
        next_task='WI4_actual_reachability_schedule_repeatability_then_remaining_WI_real_paths_and_independent_preparation',last_verified_date='2026-10-19',
        storage_finalization_record=str(g.PACKET.parent/'wi3-walk-budget-v1-finalization.json'))
    state['remaining_all'][0]['task']='WI4 실제 도달성/스케줄/반복 실행; WI-V1/V2 실제 API/독립 티켓/수용 후보 오라클; WI6 nonseed/검색 쌍/warm; WI-SP 실제 overlay/ACL; WI-C 필수항목/전달/설치역할; WI1 남은53occurrence/3block·타입/메모리·독립 의미, WI2 독립 의미/build profile/지원/설치역할, WI3 독립 의미/새 가족·기본 예산/시작 SLA/site 거부 세부원인; 새 독립DEV/FINAL·완전gold/matching/충분성·F01~08'
    write(p/'WORKLIST.json',state)
    write(p/'acceptance-review.json',[dict(id='W'+str(i).zfill(2),status='pending_signed_commit_and_binding_finalization'if i==9 else'technical_evidence_verified_not_independent_acceptance',
        evidence=['SCOPE.json','SUMMARY.json','native/receipts.json','runtime-comparison.json','targeted/targeted-final.jsonl.gz'])for i in range(1,11)])
    (p/'REPORT.txt').write_text('WI3 verified graph walk, bounded final verification and request measurement\n\n'
        'Original candidate rows/retrieval citations/ranks/scores stay exact. On an otherwise unanchored one-hop edge, the snapshot-bound optional resolver may supply a validated same-file/same-commit mutually contained declaration. Only exact equality with that declared citation accepts the edge; geometry and same spelling never establish identity. Canonical aliases reuse graph rows without merging pre-existing retrieval rows. Per-request success/failure caches preserve uncertainty. Membership uses only original or verified canonical keys. Foreign/disjoint/overlap-only/ambiguous identities and interface bridge anchors remain refused.\n'
        'Work stops early, reserving10percent of effective time (max100ms) for final graph/vector checks within the original total and parent deadline. Call slots stay reserved; late non-cooperative work is discarded. Final expiry/cancel returns no payload and is distinct from actual snapshot drift. Retained source verification is still mandatory, outside the backend-only time budget, and context/evidence-byte/policy refusals have safe typed messages. Identity-dependent impact/concurrency distinguish budget, opaque provider error, unsupported resolver and unresolved seed instead of fabricating seed absence; no enrichment provider attempt is invented. Symbol/text seed failures expose safe classification only; site resolver refusals retain the legacy generic seed_unresolved contract, so detailed site refusal attribution remains review debt.\n'
        'Opt-in label_suspects starts and seals one request measurement scope, including early errors. All logical CKG methods used by Compose and CKV calls are recorded with hashed inputs; physical Ollama HTTP is separate. Current actual summaries/budget agreement, startup scopes, primary20 strict parity/time limitations and all124 old/new attempts are in runtime-comparison. Controls use5000ms; original20inputs keep1000ms and all caps. Time-limited variations remain inconclusive, never quality acceptance.\n'
        'The first owned native run was deliberately superseded after a new reproducible budget-as-missing-seed counterexample. Its source snapshots, partial receipts/raw logs, exact signal/control PID and verified cleanup are preserved. No operational or model daemon was terminated; only the explicitly owned finite test group. A second full build/test/race passed4158/18skip but lint rejected test syntax with QF1003; that failure and its exact source are preserved separately. Only that test syntax changed before the final fresh whole run. Final native build/test/race/fmt/vet/pinned lint/boundaries/docs and binary use a separate bound source clone; no obsolete run is relabelled current. Actual runtime candidate and final validated binary hashes must match.\n'
        'Source/index/model and prior gold/evidence remain unchanged. This is exposed regression/AI technical preparation, not new independent families, installed agent/production/publication or paid/external LLM. Independent0/FINAL0/formal0of9/freeze null/execution false, no real names/contact. W09 requires a later real signed code commit and committed source-binding finalization before storage completion may be claimed. Next/all remaining: WORKLIST.json.\n')
    seal(p)


if __name__=='__main__':
    p=argparse.ArgumentParser();p.add_argument('--workspace',type=Path,required=True);p.add_argument('--output',type=Path,default=g.ROOT/g.PACKET);a=p.parse_args();prepare(a.output,a.workspace)
