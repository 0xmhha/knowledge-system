#!/usr/bin/env python3
"""Write bounded source-derived WI-4 DEV drafts without starting target work."""
import argparse
import json
from pathlib import Path
import wi4_dev_gold as g


def write(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2, allow_nan=False) + '\n')


def seal(p):
    write(p / 'manifest.json', dict(kind=g.KIND, previous_source_revision=g.REVISION,
          tools=[dict(path=n, sha256=g.f.sha((g.ROOT / n).read_bytes())) for n in g.TOOLS],
          artifacts={x.relative_to(p).as_posix(): g.f.sha(x.read_bytes()) for x in sorted(p.rglob('*')) if x.is_file() and x != p / 'manifest.json'}))


def prepare(p):
    p.mkdir(parents=True); data = g.derive()
    for name, key in [('public/DEV-inputs.json', 'public'), ('private/AI-gold.json', 'gold'),
                      ('SUMMARY.json', 'summary'), ('toolchain-scope.json', 'toolchain')]: write(p / name, data[key])
    for row in data['gold']: (p / 'private' / (row['case_id'] + '-REVIEW.txt')).write_text(g.render_review(row))
    write(p / 'matching-contract.json', g.contract()); write(p / 'REVIEW-LEDGER.json', g.review_ledger(data))
    state = g.f.read(g.ROOT / g.previous.PACKET / 'WORKLIST.json')
    state.update(current_stage='50_WI4_DEV21_conditional_outcome_AI_drafts',
                 completed_in_this_scope=['WI4_source_and_entry_bound21_DEV_inputs', 'WI4_finite_conditional_outcome_AI_drafts',
                                          'WI4_unknown_infra_denominator_and_matching_proposal'],
                 next_task='WIV1_WIV2_DEV_shared_ticket_complete_AI_oracle_and_candidate_set_drafts', last_verified_date='2026-10-19')
    state['AI_DEV_draft_progress'].update({'WI-4': '21_conditional_schedule_drafts_not_actual_target_trials_or_independent_gold',
                                         'remaining_work_items': ['WI-V1', 'WI-V2', 'WI-6', 'WI-SP', 'WI-C']})
    state['remaining_all'][0].update(status='WI1_WI2_WI3_WI4_AI_DEV_prepared_other5_and_independent_inputs_pending',
        task='WI-V1/V2/6/SP/C AI DEV; WI1~3 검토 부채와 WI4 실제 도달성·독립 oracle·동결 arm/분모·600초/정리·설치 역할; D04·새 DEV/FINAL 가족·독립 gold·매칭/충분성·F01~08 동결')
    write(p / 'WORKLIST.json', state)
    (p / 'REPORT.txt').write_text('WI-4 DEV21 conditional source outcome AI drafts\n\n'
        'D04 미정, independent approval0, formal0/9, FINAL0, freeze null, execution false.\n'
        'Five previous proposed mechanisms and21 correlated schedules are exposed DEV, never21 independent families. '
        'Each has strengthened path/readiness/identity preconditions, finite enumerated legal/source observations, symptom candidates, '
        'explicit unresolved-product/unmodelled outcomes and four infrastructure/unreached categories. '
        'Source-language permitted behavior is distinct from a product-contract violation; symptoms are conditional AI candidates.\n'
        'Both ready worker select outcomes are legal; no frequency, fairness or whole-worker termination bound is inferred. '
        'Synthetic UDP deadline/cursor observations do not prove monotonic clock rollback or real timer firing. '
        'Gossip cache/nil returns are not delivery proof, and pool backpressure requires actual subscriber path/contract review. '
        'Subscription acknowledgement, closure return, rescue and original observation are separate events.\n'
        'Original Git object chain and exact full file/span hashes are verified. Historical go1.26.9 stdlib bytes explain extracted fixture semantics; '
        'they do not adopt a target runtime. Old evidence and the prior WI3 runtime packet remain unchanged.\n'
        'Unknown outcomes remain an explicit fourth AI draft set: the old three-set formal form cannot silently represent them as legal or infrastructure. '
        'Independent formal-schema review remains pending. Each draft is complete only over its stated finite vocabulary, never all traces/product contracts.\n'
        'No actual target/API trials, new MCP/model calls, node starts or quality score. All planned/valid/symptom/unknown/infra/unreached/missing/cleanup slots '
        'must be preserved in future evidence.100 planned per arm,10 baseline symptoms and5 independent families remain proposals; '
        'zero/undefined/insufficient baseline and no recurrence are inconclusive. Runtime absolute binomial bound is not a family-paired ratio interval.\n'
        'No production/global config or external publication. Native source unchanged; prior full Go tests reused, not rerun.\n'
        'Review private/<schedule>-REVIEW.txt; next and all remaining work WORKLIST.json.\n'
        + json.dumps(data['summary'], ensure_ascii=False, indent=2) + '\n')
    seal(p)


if __name__ == '__main__':
    parser = argparse.ArgumentParser(); parser.add_argument('--output', type=Path, default=g.ROOT / g.PACKET)
    prepare(parser.parse_args().output)
