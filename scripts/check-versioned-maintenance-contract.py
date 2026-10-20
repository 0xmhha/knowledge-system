#!/usr/bin/env python3
"""Check designed maintenance contracts; never run dataset maintenance."""
import argparse
import importlib.util
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location('dataset_contract', ROOT / 'scripts/dataset_contract.py')
contract = importlib.util.module_from_spec(spec)
spec.loader.exec_module(contract)


def run_case(case):
    data, kind = case['data'], case['kind']
    try:
        if kind == 'request':
            contract.validate_request(data['request'], data['profile'])
        elif kind == 'status':
            contract.validate_status(data['status'], data['request'], data['profile'])
        elif kind == 'transition':
            contract.validate_transition(data['before'], data['after'])
        elif kind == 'observation':
            contract.validate_observation(data['before'], data['after'])
        elif kind == 'cancel':
            return contract.cancel_decision(data['state'], data['committed'])
        elif kind == 'idempotency':
            return contract.idempotency_decision(data['request'], data['record'])
        elif kind == 'reference':
            contract.validate_job_reference(data['reference'], data['owner'])
        elif kind == 'decode':
            contract.decode(data['raw'].encode())
        elif kind == 'fingerprint':
            contract.require(contract.digest(data['request']) == contract.digest(data['reordered']), 'fingerprint_differs')
        else:
            raise ValueError('unrecognized fixture kind')
    except contract.Rejected as error:
        return error.code
    return 'accept'


def evaluate():
    fixtures = json.loads((contract.CONTRACT / 'fixtures.json').read_text())
    rows = []
    ids = set()
    for case in fixtures['cases']:
        if case['id'] in ids:
            raise ValueError('duplicate contract case')
        ids.add(case['id'])
        observed = run_case(case)
        rows.append({'id': case['id'], 'kind': case['kind'], 'expected': case['expected'],
                     'observed': observed, 'passed': observed == case['expected']})
    return rows


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--out', type=Path)
    args = parser.parse_args()
    rows = evaluate()
    passed = sum(row['passed'] for row in rows)
    result = {'scope': 'KM-02.1_schema_and_lifecycle_design_only', 'cases': rows,
              'total': len(rows), 'passed': passed, 'failed': len(rows)-passed,
              'new_runtime_MCP_tools_exposed': False, 'dataset_maintenance_executed': False}
    if args.out:
        args.out.write_text(json.dumps(result, ensure_ascii=False, indent=2) + '\n')
    for row in rows:
        if not row['passed']:
            print('FAIL:', row['id'], row['expected'], row['observed'])
    print('versioned maintenance design:', passed, '/', len(rows), 'cases passed; runtime execution remains KM-02.2–04')
    raise SystemExit(passed != len(rows))
