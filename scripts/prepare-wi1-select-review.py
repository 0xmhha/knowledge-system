#!/usr/bin/env python3
"""Materialize a new supplemental reference without overwriting old evidence."""
import argparse,gzip,json,shutil
from pathlib import Path
import wi1_select_review as g


def write(path,value):
    path.parent.mkdir(parents=True,exist_ok=True)
    path.write_text(json.dumps(value,ensure_ascii=False,indent=2)+'\n')


def compress(path,value):
    path.write_bytes(gzip.compress(json.dumps(value,ensure_ascii=False,sort_keys=True,separators=(',',':')).encode(),mtime=0))


def seal(p):
    write(p/'manifest.json',dict(kind=g.KIND,previous_source_revision=g.REVISION,
        tools=[dict(path=x,sha256=g.f.sha((g.ROOT/x).read_bytes()))for x in g.TOOLS],
        artifacts={x.relative_to(p).as_posix():g.f.sha(x.read_bytes())for x in sorted(p.rglob('*'))if x.is_file()and x!=p/'manifest.json'}))


def prepare(p,work):
    p.mkdir(parents=True)
    for name in ('observations','controls-observations'):
        compress(p/(name+'.json.gz'),g.f.read(work/(name+'.json')))
    compress(p/'observer-inputs.json.gz',g.f.read(work/'inputs.json'))
    for src,dst in [('controls-inputs.json','controls-inputs.json'),('runtime-witness.go','runtime-witness.go.txt'),
                    ('runtime-witness-output.json','runtime-witness-output.json'),('observer-receipt.json','observer-receipt.json')]:
        shutil.copyfile(work/src,p/dst)
    data=g.derive(packet=p)
    for name,key in [('SUMMARY.json','summary'),('owner-deltas.json','owners'),('pair-deltas.json','pairs')]:write(p/name,data[key])
    compress(p/'comparisons.json.gz',data['comparisons'])
    state=g.f.read(g.ROOT/g.previous.PACKET/'WORKLIST.json')
    state.update(current_stage='58_WI1_select_receive_reference_owner_correction_and_old_consumer_rematching',
        completed_in_this_scope=['exact_AST_selected_receive_definition16_overlay_original30_preserved',
            'all1654_pairs_rederived_with37_record_deltas_and_no_semantic_value_label_promotion',
            'eight_type_valid_controls_and_two_receive_runtime_witness_no_new_consumer_run'],
        next_task='WI3_measurement_optional_CitationResolver_and_unanchored_traversal_contract',last_verified_date='2026-10-19')
    state['remaining_all'][0]['task']='WI1 남은53occurrence/3block·부분타입·값/메모리 의미와 독립 검토; WI3 unanchored traversal/계측 optional resolver·부하/startup/예산; WI2 독립 의미/build profile/지원확장/설치역할; WI4/V1/V2 실제경로/oracle·WI6 nonseed/쌍/warm·WI-SP overlay/ACL·WI-C 독립필수항목/실제전달; 새독립DEV/FINAL·gold·matching/충분성·F01~08'
    write(p/'WORKLIST.json',state)
    write(p/'matching-contract.json',dict(origin='AI_supplemental_reference_overlay',
        owner='CommClause receive LHS exact AST identity at its KindSelectCaseBody; preserve unknown when no unique root, including second comma-ok LHS absent from pinned CFG',
        root_cause='whole receive AssignStmt also appears during preliminary select evaluation, causing old observer ambiguity and missing fresh definition kill; no fuzzy/same-spelling/numeric-ID assignment',
        source_immutable=True,original_reference_immutable=True,original_consumer_responses_immutable=True,
        comparison='offline rematch of original30 actual MCP responses; not new native query, candidate fix, semantic precision score or independent sample',
        remaining='53 consumer occurrence phases and3 empty block ambiguities remain; value/reference/memory/partial-type unknowns unchanged',
        two_counterexample_pairs='SRC-027 d:o@18615 to u:o@18473 and u:o@18569 cannot pass the next selected receive definition',
        value_flow_labels_unchanged=True,independent_approved=0,formal_scoring_allowed=False,privacy=g.previous.contract()['privacy']))
    (p/'REPORT.txt').write_text('WI1 select receive reference correction — supplemental AI DEV overlay\n\n'
        'Pinned x/tools CFG retains the whole receive assignment during preliminary select expression evaluation, then its first LHS at the selected case. '
        'The old observer treated that LHS as ambiguous and omitted its definition kill. The new helper binds only a unique exact AST identifier in its own CommClause KindSelectCaseBody. '
        'All original function/AST/CFG/binding/occurrence bytes and old artifacts remain unchanged; a separate supplemental observation changes16owner fields across6functions. '
        'All1654 original definition/use pair identities are rederived using path search and transfer sets:37record changes (32unknown-to-reaching,2reaching-to-not-reaching,3same-reaching reason changes). '
        'The2SRC-027 backward pairs are blocked by the next receive definition. Original2diagnostic differences are retained as history. '
        'Value labels remain unknown1427/positive212/negative15; lexical structure now1459positive/195negative. This is no independent semantic gold or quality score.\n'
        'Eight isolated complete type-valid controls cover short declaration, existing-variable assignment, comma-ok second LHS, shadow, send, default, multiple cases and closure exclusion. '
        'The second comma-ok LHS stays unknown where the pinned CFG has no selected-case identifier. One compiled runtime witness with2buffered receives observes before3,8 and after5,8, illustrating fresh receive values without claiming every schedule or project typing.\n'
        'Offline rematching of the original30 actual MCP responses has0lexical disagreements, but53unmatched occurrence phases and3unmatched blocks remain. '
        'The candidate places some definitions at its preliminary assignment node rather than the selected-case node; do not relax exact owner matching or infer absent edges negative. '
        'No new native consumer calls, installed-agent roles, source/index writes, paid/external LLM, production activation or publication. '
        'Latest1738native/5approved-input binding and4107general/race results remain reusable; maintained Go source unchanged. '
        'Formal0/9, independent0, FINAL0, freeze null, execution false. No real names/contact required. Next/all remaining in WORKLIST.json.\n')
    seal(p)


if __name__=='__main__':
    p=argparse.ArgumentParser();p.add_argument('--workspace',type=Path,required=True);p.add_argument('--output',type=Path,default=g.ROOT/g.PACKET);a=p.parse_args();prepare(a.output,a.workspace)
