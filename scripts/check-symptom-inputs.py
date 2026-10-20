#!/usr/bin/env python3
"""Check symptom-only questions, source notes, overlap census and unapproved boundaries."""
import argparse
import importlib.util
import itertools
import json
from pathlib import Path
import re

from source_packet import safe, sha, verify

ROOT = Path(__file__).resolve().parents[1]
BASE = Path('docs/spec-driven/operational-preparation/source-review-supplement')
POOL = Path('docs/spec-driven/OPERATIONAL-ACCEPTANCE-INPUT-CANDIDATES.json')
PRIOR = Path('system/eval/stablenet-corpus-repair-2026-10-14/questions-frozen.json')
CANDIDATES = [4, 5, 9, 10, 14, 15, 19, 20, 23, 24, 25, 28, 29, 30, 33, 34, 35, 38, 39, 40]


def check(packet, check_references=True):
    errors = []

    def require(condition, message):
        if not condition:
            errors.append(message)

    try:
        manifest = json.loads((packet / 'manifest.json').read_text())
        require(manifest['kind'] == 'symptom_source_preparation_not_independent_acceptance', 'packet kind')
        require(manifest['candidate_pool'] == str(POOL) and manifest['candidate_pool_sha256'] == sha((ROOT / POOL).read_bytes()), 'candidate pool binding')
        require(manifest['prior_questions'] == str(PRIOR) and manifest['prior_questions_sha256'] == sha((ROOT / PRIOR).read_bytes()), 'prior question binding')
        inventory = {p.relative_to(packet).as_posix() for p in packet.rglob('*') if p.is_file() and p.name != 'manifest.json'}
        require(inventory == set(manifest['artifacts']), 'artifact inventory')
        for name, digest in manifest['artifacts'].items():
            require(sha(safe(packet, name).read_bytes()) == digest, 'artifact hash: ' + name)
        pool = json.loads((ROOT / POOL).read_text())
        functions = {f['candidate_id']: f for f in pool['functions']}
        require(manifest['repository_commit'] == pool['repository_commit'], 'source revision')
        proof = verify(packet, pool['repository_commit'])
        proposals = json.loads((packet / 'curation.json').read_text())
        curated = {p['case_id']: p for p in proposals}
        public = json.loads((packet / 'public-inputs.json').read_text())
        private = json.loads((packet / 'private-source-review.json').read_text())
        require(set(public) == {'schema_version', 'status', 'contains_gold', 'project_scope', 'cases'}, 'public top-level field leak')
        require(public['status'] == 'symptom_input_candidates_not_holdout' and public['contains_gold'] is False
                and public['project_scope'] == {'project': 'go-stablenet', 'commit': pool['repository_commit']}, 'public scope')
        require(private['status'] == 'ai_source_notes_not_independent_gold' and private['approved_gold_records'] == 0
                and private['approved_independent_families'] == 0 and private['formal_runs'] == private['retrieval_runs'] == 0
                and private['macro_recall_at_10'] is None and private['reviewer'] is None, 'gold/quality/runtime promotion')
        cards = {p['case_id']: p for p in public['cases']}
        reviews = {p['case_id']: p for p in private['reviews']}
        identities = {'RQ-' + str(i).zfill(3) for i in range(1, 21)}
        require(len(cards) == len(reviews) == len(curated) == len(public['cases']) == len(private['reviews']) == len(proposals) == 20
                and set(cards) == set(reviews) == set(curated) == identities, 'unique paired case inventory')
        require([r['candidate_id'] for r in private['reviews']] == ['SRC-' + str(i).zfill(3) for i in CANDIDATES], 'source candidate selection')
        groups, used = {}, set()
        for case_id, card in cards.items():
            require(set(card) == {'case_id', 'symptom_type', 'prompt', 'scenario_status', 'approved', 'split', 'contains_gold'}, 'public private-field leak')
            require(card['approved'] is False and card['split'] is None and card['contains_gold'] is False
                    and card['scenario_status'] == 'hypothetical_input_not_verified_incident', 'public approval/incident claim')
            require(20 <= len(card['prompt']) <= 4096 and not re.search(r'\.go\b|SRC-\d|SCHED-\d|source_anchors', card['prompt']), 'source location hint leak')
            for f in pool['functions']:
                require(not re.search(r'\b' + re.escape(f['name']) + r'\b', card['prompt']), 'source symbol hint leak')
            proposal, review = curated[case_id], reviews[case_id]
            require(card['prompt'] == proposal['prompt'] and card['symptom_type'] == proposal['symptom_type'], 'curation/public mismatch')
            source = functions[review['candidate_id']]
            require(review['source'] == source and review['candidate_id'] == proposal['candidate_id'], 'function source identity')
            raw = safe(packet, proof['files'][source['path']]['artifact']).read_bytes()
            require(sha(raw) == source['file_sha256'] and sha(raw[source['start_byte']:source['end_byte_exclusive']]) == source['function_sha256'], 'function bytes binding')
            require(review['origin'] == 'ai_manual_source_reading' and review['independent_family_approved'] is False
                    and review['approved'] is False and review['reviewer'] is None and review['review_receipt'] is None
                    and review['exhaustive_relevance_labels'] is None and review['canonical_match_rule'] is None
                    and review['independent_root_cause'] is None and review['quality_scoring_allowed'] is False
                    and review['gold_locations'] is None and review['final_eligibility'] == 'not_established'
                    and review['exposure_status'] == 'ai_curated_from_existing_public_source_pool', 'private independence/final/scoring promotion')
            require(review['unverified_assumptions'] == proposal['unknowns'] and bool(review['unverified_assumptions']), 'missing source limits')
            require(review['prior_question_topic_overlap_proposal'] == proposal['prior_topic_ids'], 'topic review provenance')
            require(review['family_group_proposal'] == proposal['group'], 'group curation mismatch')
            groups.setdefault(proposal['group'], []).append(case_id)
            require(len(review['source_anchors']) == len(proposal['anchors']) == 2, 'non-exhaustive source anchor count')
            lines = raw.splitlines(keepends=True)
            for anchor, proposed in zip(review['source_anchors'], proposal['anchors']):
                start, end, fact = proposed
                used.add(anchor['path'])
                require(source['start_line'] <= start <= end <= source['end_line'], 'anchor outside candidate function')
                begin, finish = sum(map(len, lines[:start-1])), sum(map(len, lines[:end]))
                require(anchor == {'path': source['path'], 'start_line': start, 'end_line': end, 'start_byte': begin,
                                   'end_byte_exclusive': finish, 'span_sha256': sha(raw[begin:finish]), 'source_fact_draft': fact}, 'anchor source/curation binding')
                require(fact not in card['prompt'], 'private source fact copied into prompt')
        require(len(groups) == 15 and sorted(map(len, groups.values())) == [1]*10 + [2]*5
                and groups == private['conservative_family_groups_proposal'], 'conservative family grouping')
        require(used == set(proof['files']), 'unreferenced source files')
        expected_pairs = []
        for a, b in itertools.combinations(private['reviews'], 2):
            expected_pairs.append({'a': a['case_id'], 'b': b['case_id'], 'same_group_proposed': a['family_group_proposal'] == b['family_group_proposal'],
                                   'same_source_file': a['source']['path'] == b['source']['path'], 'independence_status': 'not_reviewed'})
        require(private['pairwise_overlap_review'] == expected_pairs, 'pairwise independence inventory')
        prior = json.loads((ROOT / PRIOR).read_text())
        prior_ids = {q['id'] for q in prior['questions']}
        legacy = []
        for r, card in zip(private['reviews'], public['cases']):
            require(set(r['prior_question_topic_overlap_proposal']) <= prior_ids, 'unknown prior question reference')
            for q in prior['questions']:
                legacy.append({'case_id': r['case_id'], 'prior_question_id': q['id'],
                    'candidate_name_mentioned': bool(re.search(r'\b' + re.escape(r['source']['name']) + r'\b', q['prompt'])),
                    'exact_prompt_match': card['prompt'] == q['prompt'],
                    'ai_topic_overlap_flag': q['id'] in r['prior_question_topic_overlap_proposal'], 'semantic_independence': 'not_reviewed'})
        require(private['prior_question_comparison'] == legacy and len(legacy) == 840, 'prior overlap comparison inventory')
        require(len(private['limitations']) >= 5, 'missing exposure/sample limitations')
        counts = {'public_inputs': 20, 'private_reviews': 20, 'source_anchors': 40,
                  'symptom_types': len({p['symptom_type'] for p in public['cases']}), 'conservative_groups_proposed': 15,
                  'pairwise_reviews': 190, 'prior_question_comparisons': 840, 'source_files': len(used), 'approved_independent_families': 0}
        require(manifest['counts'] == counts and counts['symptom_types'] >= 4, 'count/coverage claims')
        spec = importlib.util.spec_from_file_location('formal_prepare', ROOT / 'scripts/prepare-source-review-supplement.py')
        mod = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(mod)
        require(json.loads((packet / 'formal-state.json').read_text()) == mod.formal_state(ROOT), 'formal state changed')
        if check_references:
            for name in ['PROTOCOL', 'PROGRESS']:
                d = json.loads((ROOT / f'docs/spec-driven/OPERATIONAL-ACCEPTANCE-{name}.json').read_text())
                require(d.get('symptom_input_preparation') == {'manifest': (packet / 'manifest.json').relative_to(ROOT).as_posix(),
                        'sha256': sha((packet / 'manifest.json').read_bytes()), 'status': 'symptom_drafts_not_independent_gold_or_holdout'}, 'ledger reference: ' + name)
    except (OSError, ValueError, KeyError, TypeError, IndexError) as exc:
        errors.append('invalid symptom packet: ' + str(exc))
    return errors


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('packet', type=Path)
    args = parser.parse_args()
    errors = check(args.packet.resolve())
    if errors:
        print('\n'.join(errors))
        raise SystemExit(1)
    print('PASS: 20 symptom-only cards, 40 source anchors, 15 proposed groups, 190/840 overlap records; independent families 0')
