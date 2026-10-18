#!/usr/bin/env python3
"""Seal bounded compiler witnesses and original-input MCP rechecks."""
import argparse,gzip,json,shutil
from pathlib import Path
import wi2_contract_audit as g
from importlib.util import module_from_spec,spec_from_file_location
sp=spec_from_file_location('writer',Path(__file__).with_name('prepare-wi3-reason-fix.py'));m=module_from_spec(sp);sp.loader.exec_module(m);write=m.write


def seal(p):
    write(p/'manifest.json',dict(kind=g.KIND,previous_source_revision=g.exact_revision(),
        tools=[dict(path=x,sha256=g.f.sha((g.ROOT/x).read_bytes()))for x in g.TOOLS],source_inputs=[dict(path=str(x),sha256=g.f.sha((g.ROOT/x).read_bytes()))for x in g.SOURCE_INPUTS],
        artifacts={x.relative_to(p).as_posix():g.f.sha(x.read_bytes())for x in sorted(p.rglob('*'))if x.is_file()and x!=p/'manifest.json'}))


def prepare(p,work):
    p.mkdir(parents=True)
    for name in ('witness-catalog.json','oracle-inputs.json','oracle-output.json'):
        x=p/'compiler'/name;x.parent.mkdir(exist_ok=True);shutil.copyfile(work/name,x)
    for folder in ('witnesses','original-source'):
        for x in sorted((work/folder).rglob('*')):
            if not x.is_file():continue
            y=p/'compiler'/folder/x.relative_to(work/folder);y=y.with_name(y.name+'.txt')if y.suffix=='.go'else y;y.parent.mkdir(parents=True,exist_ok=True);shutil.copyfile(x,y)
    m.copy_evidence(work/'runtime',p/'runtime')
    for name in ('controlled-before.json','controlled-after.json'):
        x=p/'runtime'/name;x.with_name(x.name+'.gz').write_bytes(gzip.compress(x.read_bytes(),mtime=0));x.unlink()
    # Mock vector initialization is an owned runtime resource, not evidence or a retained project index.
    if (p/'runtime/mock-vector').exists():shutil.rmtree(p/'runtime/mock-vector')
    data=g.derive(packet=p)
    for name,key in [('SUMMARY.json','summary'),('compiler-scope-review.json','scope'),('index-scope-review.json','missing'),('actual-contract-review.json','runtime')]:write(p/name,data[key])
    write(p/'matching-contract.json',g.contract());state=g.f.read(g.ROOT/g.previous.PACKET/'WORKLIST.json');state.update(current_stage='57_WI2_callee_alias_opaque_build_profile_and_budget_contract_triage',
        completed_in_this_scope=['six_differences_classified_without_rewriting_semantic_gold_or_changing_frozen_pack','eight_type_valid_supplemental_compiler_contexts_and16_build_selector_witnesses','nine_manifest_excluded_functions17inputs_and_two_historical_deadlines_explained_with_actual8_MCP_rechecks'],
        next_task='WI1_select_receive_scope_reference_and_WI3_traversal_measurement_optional_resolver_then_independent_preparation',last_verified_date='2026-10-19')
    state['remaining_all'][0]['task']='WI1 select범위 reference/53occurrence/3block·값종류; WI3 unanchored traversal/계측 optional resolver·부하/startup/예산; WI2 독립 의미/기록되지 않은 build profile/지원확장과 실제설치역할; WI4/V1/V2 실제경로/oracle·WI6 nonseed/쌍/warm·WI-SP overlay/ACL·WI-C 독립필수항목/실제전달; 새독립DEV/FINAL·gold·matching/충분성·F01~08'
    write(p/'WORKLIST.json',state)
    (p/'REPORT.txt').write_text('WI2 source/contract review: no semantic gold or frozen pack rewrite\n\n'
        'Six returned/missed differences are documented v1 boundaries:2local selector shadows,2renamed imports,2opaque helper effects. '
        'go-local-review@1.0.0 explicitly matches syntax with target_resolution syntax_only_unverified, sanitizer not_proven and lexical_may_reach; it does not prove stdlib target identity or helper value effects. '
        'Original semantic negatives/positives/unknowns,4returned disagreements and2unreturned AI positives remain in the original denominators. Contract conformance is not semantic precision.\n'
        'Eight new supplemental complete-file compiler contexts preserve the exact original6F fragments. New prefix adds Fake methods/helper implementations/import uses so go/types can check them. '
        '2local methods resolve to local variables, renamed process/ conversion imports resolve to os/exec/strconv, and identity-vs-constant helper pairs have identical F bytes but distinct full-file hashes. '
        'This is contextual compiler evidence, not a runtime invocation, replacement of the published incomplete prefix, package-wide project type proof or human independent approval.\n'
        'All9 unavailable source functions/17 project inputs belong to4 files absent from recorded graph manifest. Current16 MatchFile observations show none/gofuzz tags and Linux filename selection; exact historical GOOS/GOARCH/tags were not recorded, so exclusive historical cause remains unknown. '
        'The source archive/census includes them; graph absence cannot become safety or a negative fact. Original planned108inputs remain.\n'
        'Latest verified native binary actually rechecked8unchanged rule requests:6controlled cases retain candidate counts1/1/0/1/1/0;2formerly final-verification-deadline project functions now return partial analysis/matched_bounded empty rules with original1second budget. '
        'Both old deadline failures preserved; this current observation is not an implementation fix, quality score or fresh independent sample. Existing project archive/vector/graph and controlled graph inventory before/after equal;2owned stdio sessions EOF0. '
        'No maintained Go change, new full native suite, annotation write, real installed role, production change, publication or paid/external LLM. Reuse latest1738native/5approved-input binding and4107general/race tests at738c4a51. '
        'Independent semantics/build-profile completeness/new DEV/FINAL/F01-F08 remain pending, no real names/contact required. formal0/9, freeze null, execution false. Next/all remaining: WORKLIST.json.\n')
    seal(p)


if __name__=='__main__':
    p=argparse.ArgumentParser();p.add_argument('--output',type=Path,default=g.ROOT/g.PACKET);p.add_argument('--workspace',type=Path,required=True);a=p.parse_args();prepare(a.output,a.workspace)
