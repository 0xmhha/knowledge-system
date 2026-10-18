#!/usr/bin/env python3
"""Materialize finite manual-overlay AI drafts; never ingest a security tag."""
import argparse,gzip,json
from pathlib import Path
import wisp_dev_gold as g
from importlib.util import module_from_spec,spec_from_file_location
sp=spec_from_file_location('draft_writer',Path(__file__).with_name('prepare-wi4-dev-gold.py'));writer=module_from_spec(sp);sp.loader.exec_module(writer);write=writer.write


def seal(p):
    write(p/'manifest.json',dict(kind=g.KIND,previous_source_revision=g.REVISION,
        tools=[dict(path=n,sha256=g.f.sha((g.ROOT/n).read_bytes())) for n in g.TOOLS],
        source_inputs=[dict(path=str(n),sha256=g.f.sha((g.ROOT/n).read_bytes())) for n in g.SOURCE_INPUTS],
        artifacts={x.relative_to(p).as_posix():g.f.sha(x.read_bytes()) for x in sorted(p.rglob('*')) if x.is_file() and x!=p/'manifest.json'}))


def prepare(p):
    p.mkdir(parents=True);data=g.derive()
    for name,key in [('public/DEV-inputs.json','public'),('private/overlay-catalog.json','entries'),('private/controls.json','controls'),('SUMMARY.json','summary'),('parser-scope.json','parser')]:write(p/name,data[key])
    (p/'private/AI-gold.json.gz').write_bytes(gzip.compress(g.f.canonical(data['gold']),mtime=0))
    for public,gold in zip(data['public'],data['gold']):
        x=p/'public'/public['source_excerpt'];x.parent.mkdir(parents=True,exist_ok=True);x.write_bytes(gold['source']['body'].encode())
    write(p/'matching-contract.json',g.contract());write(p/'REVIEW-LEDGER.json',g.review_ledger(data))
    write(p/'reviewer-identity-policy.json',g.reviewer_identity_policy())
    state=g.f.read(g.ROOT/g.previous.PACKET/'WORKLIST.json')
    state.update(current_stage='53_WISP_DEV12_literal_manual_overlay_AI_drafts_and_provenance_boundaries',
        completed_in_this_scope=['WISP_12_exact_source_symbols_42policy4proposal_overlay_catalog','WISP_552_finite_selector_AI_units_with_semantic_unknown',
            'WISP_typed_legacy_provenance_and16_refusal_control_proposals'],
        next_task='WIC_DEV_required_optional_priority_artifacts_handoff_and_installed_role_AI_drafts',last_verified_date='2026-10-19')
    state['AI_DEV_draft_progress'].update({'WI-SP':'12_DEV552_literal_overlay_AI_units_typed_legacy_and16control_proposals_actual_ingest_and_independent_semantics_pending','remaining_work_items':['WI-C']})
    state['remaining_all'][0].update(status='WI1_WI2_WI3_WI4_V1_V2_WI6_WISP_AI_DEV_prepared_WIC_and_independent_inputs_pending',
        task='WI-C AI DEV; WI1~3 검토 부채·WI4/V1/V2 실제경로/oracle·WI6 nonseed gold/쌍/warm·WI-SP 독립의미/overlay ingest/store session/실제수동자동/ACL/역할; D04·새 DEV/FINAL·완전 독립 gold·matching/충분성·F01~08 동결')
    write(p/'WORKLIST.json',state)
    (p/'REPORT.txt').write_text('WI-SP DEV12 finite manual overlay/source AI drafts\n\n'
        'D04 미정; independent0/FINAL0/formal0/9/freeze null/execution false.12 previously exposed exact symbols in5 source files, '
        '42 complete literal policy entries from the approved evaluation overlay plus4 previously drafted UNAPPLIED security review placeholders. '
        'Every source body read and original Git/file/span proof checked.552 symbol/entry units:9 explicit-selected relations,543 no-explicit-selected relations. '
        'ALL semantic applicability remains unknown: literal absence is not non-applicability or safety, and operator low severity is not vulnerability proof.\n'
        'Policy-only prediction has5 relations on4 symbols/8 relatively untagged; proposed combined overlay has9 relations on8 symbols/4 relatively untagged. '
        'These are finite authoring predictions, not an actual current symbol response or evidence that security tags were ingested. '
        'Names mentioned in descriptions (GetMinterCount/IncrementHash/VarLenBytesToMultipleHash) never create missing governs edges.\n'
        'YAML authoring bytes/row spans and source hashes retained in catalog. Runtime typed payload uses exact Policy/SecurityPattern enum, '
        'known operator description/hint/severity including empty strings; legacy merged description leaves separate hint/severity unknown. '
        'Actual YAML hash/row/version/reviewer/reviewtime/approval remain unknown and metadata SHA is not authentication. '
        'Actual exact symbol id and configured ingest source path must be bound separately, not guessed from authoring values.\n'
        '16 control proposals cover decoder/selectors/caps/pins/private/relations/final snapshot_changed/ACL/empty/missing/partial/automatic/foreign store. '
        'Peer socket ACL does not authenticate reviewer or provide per-project/symbol authorization. Annotation wire lacks explicit project/dataset fields; '
        'actual routed store/session, canonical, blob, commit and snapshot identity remains required. Full-file SHA must not replace function blob SHA. '
        'No_annotations, not_found, unavailable, unsupported, budget_exhausted and partial are distinct; final drift clears data, with no snapshot transaction/ABA guarantee.\n'
        'No graph ingest, actual manual or automatic query, new listener/model/node or quality score. '
        'Same-source/name/contract symbols are not independently approved families.100percent match/refusal remains a proposed future gate. '
        'Native source1738(Go1282)/approved-input5 unchanged, prior full native tests reused. Prior WI6 packet remains unchanged atfc4186b9. '
        'Review private/AI-gold.json.gz, overlay-catalog.json and controls.json; next and all remaining WORKLIST.json.\n'
        +json.dumps(data['summary'],ensure_ascii=False,indent=2)+'\n'
        +'\nReviewer privacy correction: real names and personal contacts are never required/requested. New receipts accept reviewer_id aliases; legacy name is optional identifier compatibility. Alias alone does not approve content or establish independence: actual decision and artifact/protocol hash binding remain mandatory. D04 pending concerns review/approval, not disclosure of real identity. reviewer-identity-policy.json records the user preference.\n')
    seal(p)


if __name__=='__main__':
    parser=argparse.ArgumentParser();parser.add_argument('--output',type=Path,default=g.ROOT/g.PACKET);prepare(parser.parse_args().output)
