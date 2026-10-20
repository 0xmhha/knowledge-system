#!/usr/bin/env python3
"""Verify source identity, public/private separation and unapproved draft boundaries."""
import argparse
import hashlib
import importlib.util
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
POOL = 'docs/spec-driven/OPERATIONAL-ACCEPTANCE-INPUT-CANDIDATES.json'
PUBLIC_KEYS = {'case_id', 'candidate_id', 'work_item', 'intent', 'prompt', 'source',
               'source_artifact', 'split', 'approved', 'contains_gold', 'scope'}
INTENTS = {'state_guard', 'resource_lifecycle', 'error_handling', 'value_propagation'}


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def check(packet, root=ROOT, check_references=True):
    errors = []

    def require(condition, message):
        if not condition:
            errors.append(message)

    def safe(path):
        p = Path(path)
        if p.is_absolute() or '..' in p.parts or (packet / p).resolve().is_relative_to(packet.resolve()) is False:
            raise ValueError('unsafe artifact path')
        if (packet / p).is_symlink():
            raise ValueError('symlink artifact')
        return packet / p

    try:
        manifest = json.loads((packet / 'manifest.json').read_text())
        require(manifest['kind'] == 'source_review_preparation_not_acceptance', 'packet kind')
        require(manifest['independent_gold_records'] == 0 and manifest['cases_promoted_to_protocol'] == 0
                and manifest['formal_execution_allowed'] is False, 'manifest promotion')
        pool_raw = (root / POOL).read_bytes()
        require(manifest['candidate_pool'] == POOL and manifest['candidate_pool_sha256'] == sha(pool_raw), 'candidate pool binding')
        pool = json.loads(pool_raw)
        functions = {f['candidate_id']: f for f in pool['functions']}
        require(manifest['repository_commit'] == pool['repository_commit'], 'source revision')
        files = {p.relative_to(packet).as_posix() for p in packet.rglob('*') if p.is_file() and p.name != 'manifest.json'}
        require(files == set(manifest['artifacts']), 'artifact inventory')
        for path, digest in manifest['artifacts'].items():
            require(sha(safe(path).read_bytes()) == digest, 'artifact hash: ' + path)
        require(sha((packet / 'generator.py.txt').read_bytes()) == manifest['generator_sha256'], 'generator provenance')
        public = json.loads((packet / 'public-inputs.json').read_text())
        private = json.loads((packet / 'private-ai-review.json').read_text())
        seeds = json.loads((packet / 'curation.json').read_text())
        require(public['status'] == 'ai_preparation_only' and public['gold_included'] is False, 'public gold boundary')
        require(private['status'] == 'ai_draft_not_independent_gold' and private['approved_gold_records'] == 0
                and private['runtime_executions'] == 0, 'private approval/runtime boundary')
        pub = {c['case_id']: c for c in public['cases']}
        reviews = {c['case_id']: c for c in private['review_drafts']}
        curated = {s['candidate_id']: s for s in seeds}
        original_ids = {x['candidate_id'] for x in pool['expansion_candidates']}
        require(len(pub) == len(public['cases']) == 20 and len(reviews) == len(private['review_drafts']) == 20
                and set(pub) == set(reviews), 'unique paired cases')
        require(set(curated) == original_ids and {c['candidate_id'] for c in pub.values()} == original_ids, 'candidate set')
        require({c['intent'] for c in pub.values()} == INTENTS, 'intent coverage')
        for case_id, case in pub.items():
            require(set(case) == PUBLIC_KEYS, 'public field leak: ' + case_id)
            require(case_id == 'EXPANSION-DRAFT-' + case['candidate_id'] and case['work_item'] == 'WI-6', 'case identity')
            require(case['approved'] is False and case['split'] is None and case['contains_gold'] is False, 'public promotion')
            source = functions[case['candidate_id']]
            expected_keys = {'repository_commit', 'path', 'name', 'receiver', 'start_line', 'end_line', 'start_byte', 'end_byte_exclusive', 'file_sha256', 'function_sha256', 'git_blob'}
            require(case['source'] == {k: source[k] for k in expected_keys}, 'source identity: ' + case_id)
            require(case['source_artifact'] == 'sources/' + source['path'] + '.txt', 'source artifact identity')
            raw = safe(case['source_artifact']).read_bytes()
            require(sha(raw) == source['file_sha256'], 'source file hash')
            blob = hashlib.sha1(b'blob ' + str(len(raw)).encode() + b'\0' + raw).hexdigest()
            require(blob == source['git_blob'], 'git blob binding')
            require(sha(raw[source['start_byte']:source['end_byte_exclusive']]) == source['function_sha256'], 'function hash')
            lines = raw.splitlines(keepends=True)
            require(sum(map(len, lines[:source['start_line']-1])) == source['start_byte'], 'function start line')
            # Function AST end excludes its trailing newline.
            require(raw[:source['end_byte_exclusive']].count(b'\n') + 1 == source['end_line'], 'function end line')
            seed = curated[case['candidate_id']]
            require(all(case[k] == seed[k] for k in ['intent', 'prompt', 'scope']), 'curation/public mismatch')
            expected_scope = 'development_stress_source_only' if case['candidate_id'] == 'SRC-026' else 'production_source_candidate'
            require(case['scope'] == expected_scope, 'stress/production boundary')
            review = reviews[case_id]
            require(review['origin'] == 'ai_manual_source_reading' and review['approved'] is False
                    and review['independence_verified'] is False and review['reviewer'] is None
                    and review['review_receipt'] is None and review['relevance_labels'] is None
                    and review['exhaustive_gold'] is False and review['precision_recall_scoring_allowed'] is False,
                    'private gold/scoring promotion')
            require(review['intent_rationale_draft'] == seed['rationale'] and review['unknowns'] == seed['unknowns']
                    and bool(review['unknowns']), 'curation/private mismatch')
            require(len(review['anchors']) == len(seed['anchors']) == 2, 'source anchor count')
            for anchor, proposal in zip(review['anchors'], seed['anchors']):
                start, end, fact = proposal
                require(source['start_line'] <= start <= end <= source['end_line'], 'anchor outside function')
                begin, finish = sum(map(len, lines[:start-1])), sum(map(len, lines[:end]))
                require(anchor == {'start_line': start, 'end_line': end, 'start_byte': begin,
                                   'end_byte_exclusive': finish, 'span_sha256': sha(raw[begin:finish]),
                                   'expected_source_fact_draft': fact}, 'anchor bytes/lines/curation mismatch')
                require(fact not in case['prompt'], 'private source fact copied into public prompt')
        counts = {'public_inputs': len(pub), 'ai_reviews': len(reviews),
                  'source_anchors': sum(len(x['anchors']) for x in reviews.values()),
                  'production_candidates': sum(x['scope'] == 'production_source_candidate' for x in pub.values()),
                  'development_only_candidates': sum(x['scope'] == 'development_stress_source_only' for x in pub.values())}
        require(counts == manifest['counts'], 'count claims')
        spec = importlib.util.spec_from_file_location('source_review_prepare', root / 'scripts/prepare-source-review-supplement.py')
        mod = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(mod)
        require(json.loads((packet / 'formal-state.json').read_text()) == mod.formal_state(root), 'formal acceptance state changed')
        if check_references:
            relative = (packet / 'manifest.json').relative_to(root).as_posix()
            for name in ['PROTOCOL', 'PROGRESS']:
                ledger = json.loads((root / f'docs/spec-driven/OPERATIONAL-ACCEPTANCE-{name}.json').read_text())
                ref = ledger.get('source_review_supplement', {})
                require(ref == {'manifest': relative, 'sha256': sha((packet / 'manifest.json').read_bytes()),
                                'status': 'ai_preparation_only_not_gold_or_acceptance'}, 'ledger reference: ' + name)
    except (KeyError, ValueError, OSError, TypeError) as exc:
        errors.append('invalid packet: ' + str(exc))
    return errors


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('packet', type=Path)
    args = parser.parse_args()
    errors = check(args.packet.resolve())
    if errors:
        print('\n'.join(errors))
        raise SystemExit(1)
    print('PASS: 20 source-bound input/AI review pairs; 40 anchors; no gold, split or acceptance promotion')
