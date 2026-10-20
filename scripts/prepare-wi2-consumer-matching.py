#!/usr/bin/env python3
"""Seal a supplemental AI source overlay and actual stdio observations."""
import argparse
import gzip
import json
from pathlib import Path
import shutil
import wi2_consumer_matching as g


def write(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2, allow_nan=False)+'\n')


def packed(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_bytes(gzip.compress(g.f.canonical(value), mtime=0))


def seal(packet):
    write(packet/'manifest.json', dict(kind=g.KIND, previous_source_revision=g.REVISION,
          tools=[dict(path=p, sha256=g.f.sha((g.ROOT/p).read_bytes())) for p in g.TOOLS],
          artifacts={p.relative_to(packet).as_posix(): g.f.sha(p.read_bytes())
                     for p in sorted(packet.rglob('*')) if p.is_file() and p != packet/'manifest.json'}))


def prepare(packet, runtime, audit, controls):
    packet.mkdir(parents=True)
    shutil.copytree(audit, packet/'source-audit', ignore=shutil.ignore_patterns('main.go', 'observer'))
    shutil.copytree(runtime, packet/'runtime')
    shutil.copytree(controls, packet/'context-probe')
    for path in (packet/'runtime').rglob('*.jsonl'):
        path.with_suffix(path.suffix+'.gz').write_bytes(gzip.compress(path.read_bytes(), mtime=0))
        path.unlink()
    values = g.derive(g.ROOT, packet)
    for name in ('overlays', 'callers', 'mappings'):
        packed(packet/'private'/(name+'.json.gz'), values[name])
    write(packet/'SUMMARY.json', values['summary'])
    write(packet/'REVIEW-LEDGER.json', g.review_ledger(g.ROOT, values))
    state = g.f.read(g.ROOT/g.old.PACKET/'WORKLIST.json')
    state.update(current_stage='47_WI2_unknown_callee_caller_and_actual_consumer_matching',
                 completed_in_this_scope=['WI2_source_unknown_taxonomy_readonly_capture_refinement',
                    'WI2_declared_callee_and14_callsite_contexts_source_verified',
                    'WI2_whole_function_CKS_matching_and_coverage_unknowns_preserved'],
                 next_task='WI3_DEV_inputs_and_complete_bounded_AI_answer_drafts')
    state['AI_DEV_draft_progress']['WI-2'] = '108_AI_DEV_drafts_source_context_and_actual_consumer_matching_observed_not_independent_gold'
    state['remaining_all'][0].update(status='WI1_WI2_AI_DEV_prepared_other7_and_independent_inputs_pending',
        task='WI3/4/V1/V2/6/SP/C DEV·완전 AI 초안; WI1 매칭/의미 부채와 WI2 인덱스 미도달·상한·unknown·대상 해석/조건 매칭 부채; D04 역할/독립성·새 DEV/FINAL 가족·완전 독립 gold·매칭·충분성·F01~08 동결')
    write(packet/'WORKLIST.json', state)
    write(packet/'matching-contract.json', dict(kind='AI_proposed_observed_adapter_not_frozen',
          population='108 DEV questions grouped into91 functions, project68 calls in51 functions; caller14 separate',
          identity='project/dataset envelope then commit/file/function SHA/graph snapshot, exact UTF8 declaration/source-result and sink span; no name-only joins',
          controlled_adapter='same-width unique package spelling only in owned prefix, original fragment bytes and offsets exact; old/new file SHA explicit',
          response_scope='both rules for whole function once; one candidate can join at most one declared unit, siblings/unmatched remain unknown',
          absence='bounded non-return observation only; unavailable/incomplete/unsupported and source unknown never become semantic negative',
          conditions='raw node/structural-block and source owner retained; branch ordinal/semantic equality not accepted from node identity alone',
          source_review='source-only readonly scalar parameter capture refinement; address/write/opaque/reference/closure boundaries remain explicit',
          caller_context='14 original committed callsites, literal actual at one site is not universal wrapper safety, typed linkage/attacker/runtime unproved',
          independent_review=None, formal_scoring_allowed=False, precision_recall=None))
    lines = ['WI-2 supplemental source/context and actual consumer matching — AI DEV only', '',
             'D04 미정; independent approval0, FINAL0, formal0/9, freeze null, execution false.',
             'Stage46 packet and historical tool bindings remain byte-exact. No consumer output feeds source labels.',
             'Read-only scalar parameter captures are separated from closure writes/address escapes. Opaque helpers and unproved caller propagation remain unknown.',
             'Calls and returns record lexical closure owner; sibling same-depth literals cannot be silently merged.',
             'Both rules returned for a whole function are mapped once to selected call/source-result/sink identities. Repeated questions/callers are not independent samples.',
             'Project discovery gaps and primary budget-exhausted responses stay unknown. No retry-until-pass. Native index coverage is not the full source census.',
             'Owned controlled prefix changes package spelling only, same byte width; original fragments, UTF8 coordinates and original/new file hashes retained.',
             'Returned candidate is syntax-only, sanitizer not proven, lexical may-reach. Empty response is not safety. No precision/recall or independent quality verdict.',
             'Raw stdio, pin/cap/error/incomplete responses and normal EOF retained. Original retained graph/sources/vector inventories equal.',
             'Accepted primary observations:82 rule queries (42 original functions +40 synthetic),9 original symbols unavailable.80 matched-bounded,2 primary budget-exhausted. Four accepted sessions EOF0. Earlier harness diagnostics and extra42 project queries are separate in runtime/diagnostics/, not extra independent cases.',
             'Native Go source binding is checked against prior full build/general/race/fmt/vet/lint results; these are not freshly rerun native tests.',
             'Current details: SUMMARY.json, private/overlays.json.gz, private/callers.json.gz, private/mappings.json.gz.',
             'Review debt: REVIEW-LEDGER.json. Raw source counterexamples: context-probe/.',
             'Next: WI-3 DEV inputs and complete bounded AI drafts. All remaining work: WORKLIST.json.', '',
             json.dumps(values['summary'], ensure_ascii=False, indent=2)]
    (packet/'REPORT.txt').write_text('\n'.join(lines)+'\n')
    # Reviewable plain text avoids requiring a reader to open compressed JSON.
    for name in ('overlays', 'callers'):
        (packet/('REVIEW-'+name.upper()+'.txt')).write_text('\n'.join(json.dumps(v, ensure_ascii=False, sort_keys=True) for v in values[name])+'\n')
    seal(packet)


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('--runtime', type=Path, required=True)
    parser.add_argument('--audit', type=Path, required=True)
    parser.add_argument('--context-probe', type=Path, required=True)
    parser.add_argument('--output', type=Path, default=g.ROOT/g.PACKET)
    args = parser.parse_args()
    prepare(args.output, args.runtime, args.audit, args.context_probe)
