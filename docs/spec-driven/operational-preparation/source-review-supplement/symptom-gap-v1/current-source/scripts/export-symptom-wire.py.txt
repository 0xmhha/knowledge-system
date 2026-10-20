#!/usr/bin/env python3
"""Materialize parser-preflight requests from one public JSON file only."""
import argparse
import json
from pathlib import Path
import re


def render(public):
    if set(public) != {'schema_version', 'status', 'contains_gold', 'project_scope', 'cases'}:
        raise ValueError('unexpected public envelope fields')
    if public['status'] != 'symptom_input_candidates_not_holdout' or public['contains_gold'] is not False:
        raise ValueError('public input status or gold flag')
    scope = public['project_scope']
    if set(scope) != {'project', 'commit'} or scope['project'] != 'go-stablenet' or not re.fullmatch('[0-9a-f]{40}', scope['commit']):
        raise ValueError('public project scope')
    requests, identities = [], set()
    for card in public['cases']:
        if set(card) != {'case_id', 'symptom_type', 'prompt', 'scenario_status', 'approved', 'split', 'contains_gold'}:
            raise ValueError('private field in public card')
        if card['approved'] is not False or card['split'] is not None or card['contains_gold'] is not False:
            raise ValueError('unapproved input promoted')
        if card['scenario_status'] != 'hypothetical_input_not_verified_incident' or not 20 <= len(card['prompt']) <= 4096:
            raise ValueError('invalid hypothetical symptom')
        if re.search(r'\.go\b|SRC-\d|SCHED-\d|source_anchors', card['prompt']):
            raise ValueError('source hint in public prompt')
        if card['case_id'] in identities:
            raise ValueError('duplicate public case')
        identities.add(card['case_id'])
        requests.append({'case_id': card['case_id'], 'name': 'cks.context.label_suspects',
                         'arguments': {'symptom': card['prompt'], 'commit': scope['commit'],
                                       'max_candidates': 10, 'max_seeds': 3, 'max_enriched': 3,
                                       'max_calls': 32, 'depth': 2, 'budget_ms': 1000}})
    if identities != {'RQ-' + str(i).zfill(3) for i in range(1, 21)}:
        raise ValueError('expected exactly 20 draft input cards')
    return {'kind': 'public_symptom_wire_preflight_only', 'execution_allowed': False,
            'source_commit': scope['commit'], 'requests': requests}


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--public-input', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    if args.output.exists():
        raise ValueError('preserve previous exported requests')
    wire = render(json.loads(args.public_input.read_text()))
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(wire, ensure_ascii=False, indent=2) + '\n')
    print('Prepared 20 public-only text-seed requests; execution remains disabled')
