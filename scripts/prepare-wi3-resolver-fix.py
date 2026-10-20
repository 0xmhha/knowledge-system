#!/usr/bin/env python3
"""Materialize fresh resolver evidence without rewriting prior packets."""
import argparse,gzip,json,shutil
from pathlib import Path
import wi3_resolver_fix as g


def write(path,value):
    path.parent.mkdir(parents=True,exist_ok=True);path.write_text(json.dumps(value,ensure_ascii=False,indent=2)+'\n')


def compress(raw):return gzip.compress(raw,mtime=0)


def seal(p):
    write(p/'manifest.json',dict(kind=g.KIND,previous_source_revision=g.REVISION,
        tools=[dict(path=n,sha256=g.f.sha((g.ROOT/n).read_bytes()))for n in g.TOOLS],
        artifacts={x.relative_to(p).as_posix():g.f.sha(x.read_bytes())for x in sorted(p.rglob('*'))if x.is_file()and x!=p/'manifest.json'}))


def prepare(p,work):
    if p.exists()and any(x.name!='SCOPE.json'for x in p.iterdir()):raise ValueError('refusing evidence overwrite')
    p.mkdir(parents=True,exist_ok=True)
    # SCOPE is the declaration saved before execution, not a claim of current approval.
    if not(p/'SCOPE.json').exists():shutil.copyfile(g.ROOT/g.PACKET/'SCOPE.json',p/'SCOPE.json')
    shutil.copyfile(work/'source-binding.json',p/'source-binding.json')
    (p/'code-change.patch.gz').write_bytes(compress((work/'native-change.patch').read_bytes()))
    for x in (work/'native').iterdir():
        if not x.is_file()or x.name=='read-cks':continue
        y=p/'native'/x.name;y.parent.mkdir(exist_ok=True)
        if x.suffix in('.stdout','.stderr'):y.with_name(y.name+'.gz').write_bytes(compress(x.read_bytes()))
        else:shutil.copyfile(x,y)
    shutil.copyfile(work/'installed-tools.json',p/'native/installed-tools.json')
    shutil.copyfile(work/'lint-config-binding.json',p/'native/lint-config-binding.json')
    for src,dst in [('before-runtime','initial-runtime'),('before-runtime-declaration','before-runtime'),('after-runtime','after-runtime')]:
        for x in sorted((work/src).rglob('*')):
            if not x.is_file():continue
            # Footprints are startup diagnostics, not a new automatic label scope.
            y=p/dst/x.relative_to(work/src);y.parent.mkdir(parents=True,exist_ok=True)
            if x.suffix in('.jsonl','.txt')or x.name in('responses.json','before.json','after.json'):
                y.with_name(y.name+'.gz').write_bytes(compress(x.read_bytes()))
            elif x.name=='config.yaml':shutil.copyfile(x,y.with_name('config.txt'))
            else:shutil.copyfile(x,y)
    for name in ('before-unit.jsonl','before-handler.jsonl','after-targeted.jsonl'):
        y=p/'targeted'/(name+'.gz');y.parent.mkdir(exist_ok=True);y.write_bytes(compress((work/name).read_bytes()))
    d=g.derive(packet=p);write(p/'SUMMARY.json',d['summary']);write(p/'runtime-comparison.json',d['comparisons'])
    state=g.f.read(g.ROOT/g.previous.PACKET/'WORKLIST.json');state.update(current_stage='59_WI3_optional_resolver_measurement_parity_fresh_native_and_stdio',
        completed_in_this_scope=['optional_resolver_capability_preserved_without_inventing_unsupported_provider_support','resolver_delegation_scoped_private_measurement_and_legacy_pinned_MCP_route_regressions',
            'fresh_whole_native_and_actual_old_new_measurement_off_on_regressions'],next_task='WI3_unanchored_traversal_exact_identity_and_final_verification_budget_contract',last_verified_date='2026-10-19')
    state['remaining_all'][0]['task']='WI3 unanchored traversal/최종 검증 예산·부하/startup·자동 label 측정 scope; WI1 남은53occurrence/3block·부분타입/메모리 의미·독립 검토; WI2 독립 의미/build profile/지원확장/설치역할; WI4/V1/V2 실제경로/oracle·WI6 nonseed/쌍/warm·WI-SP overlay/ACL·WI-C 필수항목/전달; 새독립DEV/FINAL·gold·matching/충분성·F01~08'
    write(p/'WORKLIST.json',state)
    write(p/'acceptance-review.json',[dict(id=id,status='technical_evidence_verified_not_independent_acceptance',evidence=evidence)for id,evidence in [
        ('R01',['targeted/after-targeted.jsonl.gz']),('R02',['targeted/after-targeted.jsonl.gz']),('R03',['targeted/after-targeted.jsonl.gz']),
        ('R04',['targeted/after-targeted.jsonl.gz','runtime-comparison.json']),('R05',['initial-runtime/receipt.json','before-runtime/receipt.json','after-runtime/receipt.json']),
        ('R06',['native/receipts.json','source-binding.json']),('R07',['WORKLIST.json','SUMMARY.json'])]])
    (p/'REPORT.txt').write_text('WI3 optional resolver preservation under backend measurement\n\n'
        'WrapCKG used to expose only required Client methods, hiding optional CitationResolver on Real/Fake providers. The new conditional wrapper exposes the resolver iff the base supports it, delegates exact context/citation/result/error, and records one hashed-input resolver attempt only within an existing measurement scope. Typed unresolved-seed metadata is safe; no raw path, identity result or backend message is recorded.\n'
        'New adapter/legacy/pinned MCP handler regressions fail before and pass after the patch. Site/symbol/expansion handler outputs agree excluding elapsed timing only; forged retained hashes still refuse. '
        'Actual old CKS measured mode rejects a valid declaration site and drops all returned symbol identities for symbol/expansion requests. New CKS measured and unmeasured modes agree, and unmeasured behavior stays identical to the baseline excluding elapsed timing. '
        'The initial14call attempt used doc-comment line257 as site; its unmeasured seed_unresolved error is retained. Follow-up16call baseline and16call new run explicitly separate declaration line258 and comment line257. '
        'Comment/unknown sites, wrong commit, foreign dataset and forged hash still return no structured result. Original dataset source/graph/vector preserved; all6owned stdio sessions EOF0. No independent samples/quality scores.\n'
        'Fresh whole native build/test/race/fmt/vet/pinned lint/boundaries/docs and binary are bound to1738native/5approved inputs with4Go changes, base30e43a34 plus working-tree patch, not a clean final commit build. '
        'Counts and raw receipts are in SUMMARY/native. The16call plans use5000ms cooperative budgets: parity regression, not a default1second or warm latency acceptance gate. Two supplemental lint/docs configurations are captured before lint and verified unchanged after it. Initial source completion guard needs the new binding committed before it can reference that revision; finalization is separate and must not relabel the old guard/current native evidence. '
        'Automatic label_suspects measurement scope and exact-identity unanchored traversal normalization are not added. Installed agent/production/publication/paid or external LLM remain0. '
        'Independent0, FINAL0, formal0/9, freeze null, execution false; no real names/contact required. Next/all remaining in WORKLIST.json.\n')
    seal(p)


if __name__=='__main__':
    p=argparse.ArgumentParser();p.add_argument('--workspace',type=Path,required=True);p.add_argument('--output',type=Path,default=g.ROOT/g.PACKET);a=p.parse_args();prepare(a.output,a.workspace)
