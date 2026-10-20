#!/usr/bin/env python3
"""Package AI source-reading proposals without promoting them to acceptance cases."""
import argparse
import hashlib
import json
from pathlib import Path
import subprocess

ROOT = Path(__file__).resolve().parents[1]
POOL = 'docs/spec-driven/OPERATIONAL-ACCEPTANCE-INPUT-CANDIDATES.json'


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def dump(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2) + '\n')


def formal_state(root):
    protocol = json.loads((root / 'docs/spec-driven/OPERATIONAL-ACCEPTANCE-PROTOCOL.json').read_text())
    progress = json.loads((root / 'docs/spec-driven/OPERATIONAL-ACCEPTANCE-PROGRESS.json').read_text())
    return {
        'freeze': protocol['freeze'], 'execution_allowed': protocol['execution_allowed'],
        'roles': protocol['roles'], 'model': protocol['model'],
        'freeze_requirements': protocol['freeze_requirements'],
        'input_groups': protocol['input_groups'], 'items': progress['items'],
        'reviewer_assignment': progress['reviewer_assignment'],
        'protocol_freeze': progress['protocol_freeze'],
    }


def prepare(repo, output, seeds):
    if output.exists():
        raise ValueError('output must be a new directory; preserve prior packets')
    pool_raw = (ROOT / POOL).read_bytes()
    pool = json.loads(pool_raw)
    functions = {f['candidate_id']: f for f in pool['functions']}
    public, private = [], []
    for seed in seeds:
        source = functions[seed['candidate_id']]
        raw = subprocess.check_output(['git', '-C', str(repo), 'show', pool['repository_commit'] + ':' + source['path']])
        if sha(raw) != source['file_sha256']:
            raise ValueError('pinned source mismatch')
        source_path = 'sources/' + source['path'] + '.txt'
        (output / source_path).parent.mkdir(parents=True, exist_ok=True)
        (output / source_path).write_bytes(raw)
        case_id = 'EXPANSION-DRAFT-' + seed['candidate_id']
        public.append({
            'case_id': case_id, 'candidate_id': seed['candidate_id'], 'work_item': 'WI-6',
            'intent': seed['intent'], 'prompt': seed['prompt'],
            'source': {k: source[k] for k in ['repository_commit', 'path', 'name', 'receiver', 'start_line', 'end_line', 'start_byte', 'end_byte_exclusive', 'file_sha256', 'function_sha256', 'git_blob']},
            'source_artifact': source_path, 'split': None, 'approved': False,
            'contains_gold': False, 'scope': seed['scope'],
        })
        lines = raw.splitlines(keepends=True)
        anchors = []
        for start, end, fact in seed['anchors']:
            begin, finish = sum(map(len, lines[:start-1])), sum(map(len, lines[:end]))
            anchors.append({'start_line': start, 'end_line': end, 'start_byte': begin,
                            'end_byte_exclusive': finish, 'span_sha256': sha(raw[begin:finish]),
                            'expected_source_fact_draft': fact})
        private.append({
            'case_id': case_id, 'origin': 'ai_manual_source_reading', 'approved': False,
            'independence_verified': False, 'reviewer': None, 'review_receipt': None,
            'intent_rationale_draft': seed['rationale'], 'anchors': anchors,
            'unknowns': seed['unknowns'], 'relevance_labels': None,
            'exhaustive_gold': False, 'precision_recall_scoring_allowed': False,
        })
    dump(output / 'public-inputs.json', {'schema_version': 1, 'status': 'ai_preparation_only',
         'gold_included': False, 'cases': public})
    dump(output / 'private-ai-review.json', {'schema_version': 1, 'status': 'ai_draft_not_independent_gold',
         'approved_gold_records': 0, 'runtime_executions': 0, 'review_drafts': private})
    dump(output / 'formal-state.json', formal_state(ROOT))
    dump(output / 'curation.json', seeds)
    generator = Path(__file__).read_bytes()
    (output / 'generator.py.txt').write_bytes(generator)
    artifacts = {p.relative_to(output).as_posix(): sha(p.read_bytes()) for p in sorted(output.rglob('*')) if p.is_file()}
    manifest = {
        'schema_version': 1, 'kind': 'source_review_preparation_not_acceptance',
        'repository_commit': pool['repository_commit'], 'candidate_pool': POOL,
        'candidate_pool_sha256': sha(pool_raw), 'artifacts': artifacts,
        'generator_sha256': sha(generator), 'independent_gold_records': 0,
        'cases_promoted_to_protocol': 0, 'formal_execution_allowed': False,
        'counts': {'public_inputs': len(public), 'ai_reviews': len(private),
                   'source_anchors': sum(len(x['anchors']) for x in private),
                   'production_candidates': sum(x['scope'] == 'production_source_candidate' for x in public),
                   'development_only_candidates': sum(x['scope'] == 'development_stress_source_only' for x in public)},
        'limitations': ['intent relevance is an AI proposal, not independently reviewed',
                       '20 cards do not prove 20 independent families; semantic overlap is unreviewed',
                       'DEV/FINAL allocation, budgets, canonical matching and relevance labels remain unset',
                       'source anchors are non-exhaustive and cannot support precision/recall scores',
                       'stress main is source-only development material; do not start nodes'],
    }
    dump(output / 'manifest.json', manifest)
    print(json.dumps(manifest['counts']))


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--repo', required=True, type=Path)
    parser.add_argument('--output', required=True, type=Path)
    parser.add_argument('--seeds', required=True, type=Path)
    args = parser.parse_args()
    prepare(args.repo, args.output, json.loads(args.seeds.read_text()))
