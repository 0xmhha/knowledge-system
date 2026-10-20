#!/usr/bin/env python3
"""Prepare symptom-only input cards and separate AI source notes; never author gold."""
import argparse
import copy
import importlib.util
import itertools
import json
from pathlib import Path
import re

from source_packet import SourceCapture, sha

ROOT = Path(__file__).resolve().parents[1]
BASE = Path('docs/spec-driven/operational-preparation/source-review-supplement')
POOL = Path('docs/spec-driven/OPERATIONAL-ACCEPTANCE-INPUT-CANDIDATES.json')
PRIOR = Path('system/eval/stablenet-corpus-repair-2026-10-14/questions-frozen.json')


def dump(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2) + '\n')


def prepare(repo, output, proposals):
    if output.exists():
        raise ValueError('use a new output; do not change frozen evidence')
    proposals = copy.deepcopy(proposals)
    pool = json.loads((ROOT / POOL).read_text())
    functions = {f['candidate_id']: f for f in pool['functions']}
    prior = json.loads((ROOT / PRIOR).read_text())
    revision = pool['repository_commit']
    output.mkdir(parents=True)
    capture = SourceCapture(repo, output, revision)
    public, private = [], []
    for p in proposals:
        source = functions[p['candidate_id']]
        raw = capture.source(source['path'])
        if sha(raw) != source['file_sha256'] or sha(raw[source['start_byte']:source['end_byte_exclusive']]) != source['function_sha256']:
            raise ValueError('candidate function identity mismatch')
        public.append({'case_id': p['case_id'], 'symptom_type': p['symptom_type'], 'prompt': p['prompt'],
                       'scenario_status': 'hypothetical_input_not_verified_incident',
                       'approved': False, 'split': None, 'contains_gold': False})
        anchors = []
        for start, end, fact in p['anchors']:
            anchor = capture.span(source['path'], start, end)
            anchor['source_fact_draft'] = fact
            anchors.append(anchor)
        private.append({'case_id': p['case_id'], 'candidate_id': p['candidate_id'],
                        'source': source, 'source_anchors': anchors, 'origin': 'ai_manual_source_reading',
                        'family_group_proposal': p['group'], 'independent_family_approved': False,
                        'approved': False, 'reviewer': None, 'review_receipt': None,
                        'exhaustive_relevance_labels': None, 'canonical_match_rule': None,
                        'independent_root_cause': None, 'quality_scoring_allowed': False,
                        'gold_locations': None, 'unverified_assumptions': p['unknowns'],
                        'prior_question_topic_overlap_proposal': p['prior_topic_ids'],
                        'exposure_status': 'ai_curated_from_existing_public_source_pool',
                        'final_eligibility': 'not_established'})
    groups = {}
    for p in private:
        groups.setdefault(p['family_group_proposal'], []).append(p['case_id'])
    pairwise = []
    for a, b in itertools.combinations(private, 2):
        pairwise.append({'a': a['case_id'], 'b': b['case_id'],
                         'same_group_proposed': a['family_group_proposal'] == b['family_group_proposal'],
                         'same_source_file': a['source']['path'] == b['source']['path'],
                         'independence_status': 'not_reviewed'})
    legacy = []
    for p, card in zip(private, public):
        for q in prior['questions']:
            legacy.append({'case_id': p['case_id'], 'prior_question_id': q['id'],
                           'candidate_name_mentioned': bool(re.search(r'\b' + re.escape(p['source']['name']) + r'\b', q['prompt'])),
                           'exact_prompt_match': card['prompt'] == q['prompt'],
                           'ai_topic_overlap_flag': q['id'] in p['prior_question_topic_overlap_proposal'],
                           'semantic_independence': 'not_reviewed'})
    dump(output / 'public-inputs.json', {'schema_version': 1, 'status': 'symptom_input_candidates_not_holdout',
         'contains_gold': False, 'project_scope': {'project': 'go-stablenet', 'commit': revision}, 'cases': public})
    dump(output / 'private-source-review.json', {'schema_version': 1, 'status': 'ai_source_notes_not_independent_gold',
         'approved_gold_records': 0, 'approved_independent_families': 0, 'formal_runs': 0,
         'retrieval_runs': 0, 'macro_recall_at_10': None, 'reviewer': None,
         'reviews': private, 'conservative_family_groups_proposal': groups,
         'pairwise_overlap_review': pairwise, 'prior_question_comparison': legacy,
         'limitations': ['20 cards do not establish 20 independent fact families',
                        '15 conservative source-flow groups are AI proposals, not reviewed units',
                        'literal-name or exact-prompt mismatch does not establish semantic independence',
                        'source notes are non-exhaustive and cannot be scored as gold or root causes',
                        'public cards and source selection are exposed; FINAL eligibility is not established']})
    dump(output / 'git-proof.json', capture.proof())
    dump(output / 'curation.json', proposals)
    spec = importlib.util.spec_from_file_location('formal_prepare', ROOT / 'scripts/prepare-source-review-supplement.py')
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    dump(output / 'formal-state.json', mod.formal_state(ROOT))
    (output / 'generator.py.txt').write_bytes(Path(__file__).read_bytes())
    (output / 'source-packet.py.txt').write_bytes((ROOT / 'scripts/source_packet.py').read_bytes())
    dump(output / 'manifest.json', {'schema_version': 1, 'kind': 'symptom_source_preparation_not_independent_acceptance',
         'repository_commit': revision, 'candidate_pool': str(POOL), 'candidate_pool_sha256': sha((ROOT / POOL).read_bytes()),
         'prior_questions': str(PRIOR), 'prior_questions_sha256': sha((ROOT / PRIOR).read_bytes()),
         'counts': {'public_inputs': len(public), 'private_reviews': len(private),
                    'source_anchors': sum(len(p['source_anchors']) for p in private),
                    'symptom_types': len({p['symptom_type'] for p in public}),
                    'conservative_groups_proposed': len(groups), 'pairwise_reviews': len(pairwise),
                    'prior_question_comparisons': len(legacy), 'source_files': len(capture.files),
                    'approved_independent_families': 0},
         'artifacts': {p.relative_to(output).as_posix(): sha(p.read_bytes()) for p in sorted(output.rglob('*')) if p.is_file()}})
    print('Prepared symptom candidates:', len(public), 'cards;', len(groups), 'unreviewed groups; gold 0')


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--repo', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--proposals', type=Path, required=True)
    args = parser.parse_args()
    prepare(args.repo, args.output, json.loads(args.proposals.read_text()))
