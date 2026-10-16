#!/usr/bin/env python3
"""Validate unapproved subscription ticket proposals against pinned source evidence."""
import argparse
import hashlib
import importlib.util
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
BASE = Path('docs/spec-driven/operational-preparation/source-review-supplement')
VARIANTS = {'ORIGINAL': 'original.go.txt', 'CAND-A': 'candidate-a.go.txt',
            'CAND-B': 'candidate-b.go.txt', 'MUT-REINTRO': 'reintroduction.go.txt',
            'MUT-EARLY': 'early-return.go.txt'}
SCENARIOS = {'normal_delivery_then_unsubscribe', 'blocked_delivery_then_consumer_exit',
             'late_delivery_before_confirmation', 'confirmation_delayed', 'concurrent_unsubscribe_once'}


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def load_script(name):
    spec = importlib.util.spec_from_file_location(name.replace('-', '_'), ROOT / 'scripts' / name)
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


def check(packet, check_references=True):
    errors = []

    def require(condition, message):
        if not condition:
            errors.append(message)

    try:
        manifest = json.loads((packet / 'manifest.json').read_text())
        require(manifest['kind'] == 'subscription_ticket_preparation_only', 'packet kind')
        context = BASE / 'context-v1/manifest.json'
        require(manifest['context_manifest'] == str(context) and manifest['context_manifest_sha256'] == sha((ROOT / context).read_bytes()), 'context binding')
        errors.extend(load_script('check-review-context.py').check((ROOT / context).parent))
        inventory = {p.name for p in packet.iterdir() if p.is_file() and p.name != 'manifest.json'}
        require(inventory == set(manifest['artifacts']), 'artifact inventory')
        for name, digest in manifest['artifacts'].items():
            require(Path(name).name == name and not (packet / name).is_symlink(), 'unsafe artifact')
            require(sha((packet / name).read_bytes()) == digest, 'artifact hash: ' + name)
        source = (ROOT / BASE / 'context-v1/sources/eth/downloader/api.go.txt').read_bytes().splitlines(keepends=True)
        require([(s['artifact'], s['start_line'], s['end_line']) for s in manifest['source_spans']] ==
                [('source-types.go.txt', 167, 178), ('original.go.txt', 183, 198)], 'source extraction identity')
        for span in manifest['source_spans']:
            raw = b''.join(source[span['start_line']-1:span['end_line']])
            require(raw == (packet / span['artifact']).read_bytes() and sha(raw) == span['sha256'], 'exact source extraction')
        require((packet / 'reintroduction.go.txt').read_bytes() == (packet / 'original.go.txt').read_bytes(), 'reintroduction mutant binding')
        public = json.loads((packet / 'public-ticket.json').read_text())
        require(set(public) == {'schema_version', 'status', 'case_id', 'family_id_proposal', 'work_items', 'source', 'prompt', 'symptom', 'split', 'approved', 'contains_gold'}, 'public/private separation')
        require(public['status'] == 'development_input_draft_not_independent_ticket' and public['approved'] is False
                and public['contains_gold'] is False and public['split'] is None, 'public promotion')
        require(public['case_id'] == 'SUB-EXIT-DRAFT-001' and public['work_items'] == ['WI-V1', 'WI-V2'], 'ticket identity')
        require(public['source'] == {'repository_commit': '0bf2f4d1bfeb6605006d556957ef8c045d8f8ed8', 'path': 'eth/downloader/api.go',
                                     'entry_points': ['Syncing', 'eventLoop', 'SyncStatusSubscription.Unsubscribe']}, 'ticket source')
        private = json.loads((packet / 'private-review.json').read_text())
        require(private['status'] == 'ai_development_oracles_not_independent_gold' and private['approved'] is False
                and private['reviewer'] is None and private['review_receipt'] is None
                and private['independently_derived_oracle'] is None and private['independent_acceptable_candidate_set'] is None
                and private['verified_project_defect'] is False and private['formal_runtime_executions'] == 0
                and private['local_fixture_executions'] == 0 and private['tickets_promoted_to_protocol'] == 0
                and private['independent_family_count'] == 0, 'independence or execution promotion')
        require(private['case_id'] == public['case_id'] and private['origin'] == 'ai_source_reading_and_fixture_design', 'private identity')
        require(len(private['variants']) == 5 and {v['id']: v['artifact'] for v in private['variants']} == VARIANTS, 'variants')
        require(len(private['scenarios']) == 5 and {s['id'] for s in private['scenarios']} == SCENARIOS, 'correlated scenario identity')
        expected = {v: {s: (v != 'MUT-EARLY' and not (v in {'ORIGINAL', 'MUT-REINTRO'} and s == 'blocked_delivery_then_consumer_exit'))
                        for s in SCENARIOS} for v in VARIANTS}
        require(private['expected_draft_matrix'] == expected, 'AI draft expectation matrix')
        require(private['secondary_oracle_draft']['kind'] == 'same_ai_source_derived_wait_graph_not_independent', 'secondary oracle independence')
        require(len(private['primary_oracle_draft']) == 5 and len(private['limitations']) >= 5, 'missing criteria/limits')
        require(manifest['counts'] == {'public_tickets': 1, 'ai_fix_candidates': 2, 'mutants': 2,
                                      'scenario_proposals': 5, 'approved_independent_tickets': 0}, 'candidate counts')
        require(json.loads((packet / 'formal-state.json').read_text()) == load_script('prepare-source-review-supplement.py').formal_state(ROOT), 'formal state changed')
        if check_references:
            for name in ['PROTOCOL', 'PROGRESS']:
                d = json.loads((ROOT / f'docs/spec-driven/OPERATIONAL-ACCEPTANCE-{name}.json').read_text())
                require(d.get('subscription_ticket_preparation') == {
                    'manifest': (packet / 'manifest.json').relative_to(ROOT).as_posix(),
                    'sha256': sha((packet / 'manifest.json').read_bytes()),
                    'status': 'development_draft_not_independent_ticket_or_gold'}, 'ledger reference: ' + name)
    except (KeyError, ValueError, OSError, TypeError) as exc:
        errors.append('invalid ticket packet: ' + str(exc))
    return errors


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('packet', type=Path)
    args = parser.parse_args()
    errors = check(args.packet.resolve())
    if errors:
        print('\n'.join(errors))
        raise SystemExit(1)
    print('PASS: 1 development ticket; 2 AI fix candidates; 2 mutants; 5 correlated schedules; independent tickets 0')
