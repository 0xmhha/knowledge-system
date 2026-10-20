#!/usr/bin/env python3
"""Check the live execution ledger without treating AI preparation as acceptance."""
import hashlib
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
LEDGER = Path('docs/spec-driven/REMAINING-13-EXECUTION.json')
IDS = {'WI-V1', 'WI-V2', 'WI-6', 'WI-SP', 'WI-C', 'WI-1', 'WI-2', 'WI-3', 'WI-4',
       'INDEPENDENT-PREP', 'ONTOLOGY', 'FORMAL-9', 'OPTIONAL-12'}


def check(data, root=ROOT):
    errors = []
    rows = data.get('rows', [])
    if len(rows) != 13 or {r.get('id') for r in rows} != IDS:
        errors.append('exactly the declared thirteen groups are required')
    completed = 0
    for row in rows:
        if row.get('completion_status') != 'complete':
            continue
        completed += 1
        evidence = row.get('completion_evidence', [])
        expected = row.get('done_when', [])
        if not expected or [r.get('criterion') for r in evidence] != expected:
            errors.append(row['id']+': every completion criterion needs its own evidence')
            continue
        for entry in evidence:
            path = (root/entry.get('path', '')).resolve()
            digest = entry.get('sha256')
            if not path.is_relative_to(root.resolve()) or not path.is_file() or hashlib.sha256(path.read_bytes()).hexdigest() != digest:
                errors.append(row['id']+': completion evidence missing or changed')
            if entry.get('result') != 'verified' or entry.get('scope') != 'complete_declared_criterion':
                errors.append(row['id']+': bounded preparation is not complete criterion evidence')
            if 'independent' in entry['criterion'].lower() and (entry.get('review', {}).get('kind') != 'human' or not entry.get('review', {}).get('anonymous_role_id')):
                errors.append(row['id']+': anonymous independent review receipt required')
    if data.get('groups_total') != 13 or data.get('groups_completed') != completed:
        errors.append('group completion count differs from evidence-backed rows')
    if data.get('personal_names_required') is not False or data.get('security_user_deferrals_preserved') is not True:
        errors.append('user privacy/deferred scope policy changed')
    if data.get('formal_completed') != 0 or data.get('freeze') is not None or data.get('execution_allowed') is not False:
        errors.append('this unfrozen DEV ledger cannot self-promote formal acceptance')
    return errors


if __name__ == '__main__':
    data = json.loads((ROOT/LEDGER).read_text())
    errors = check(data)
    if errors:
        raise SystemExit('\n'.join(errors))
    print(json.dumps(dict(groups=13, completed=data['groups_completed'], formal_completed=0,
                          next=data['next']), ensure_ascii=False))
