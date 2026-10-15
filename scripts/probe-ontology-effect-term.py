#!/usr/bin/env python3
"""Post-observation exact-term mechanism probe, kept outside the frozen 384 calls."""
import argparse
import hashlib
import json
from pathlib import Path
import subprocess
import time

ARMS = ('off', 'concept_text', 'relations', 'combined')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--run', type=Path, required=True)
    parser.add_argument('--cks-binary', type=Path, required=True)
    args = parser.parse_args()
    root, binary = args.run.resolve(), args.cks_binary.resolve()
    out = root / 'mechanism-probe'
    out.mkdir()
    requests = {'schema_version': 1, 'requests': [{'id': 'OE-X01', 'tool': 'cks.context.get_for_task_v2',
        'arguments': {'prompt': '쿼럼', 'explain_ontology': True}}]}
    def write(name, value):
        (out / name).write_text(json.dumps(value, ensure_ascii=False, indent=2) + '\n')
    write('requests.json', requests)
    write('protocol.json', {'scope': 'post_observation_exact_term_mechanism_probe_not_quality_evaluation',
        'chosen_after_main_results': True, 'part_of_frozen_384': False, 'planned_calls': 8,
        'expected_concept': 'wbft-quorum', 'query_source': 'existing approved preferred Korean term',
        'counts': {'warmup': 1, 'retrieval': 1, 'warm_latency': 0, 'cold_process': 0},
        'quality_evaluated': False, 'independent_acceptance': False})
    receipts = []
    for arm in ARMS:
        command = [str(binary), 'eval', 'capture', '--config', str(root / (arm + '.yaml')),
                   '--requests', str(out / 'requests.json'), '--output', str(out / (arm + '.json')),
                   '--warmup', '1', '--retrieval-runs', '1', '--warm-runs', '0', '--cold-runs', '0',
                   '--call-timeout', '30s']
        start = time.monotonic()
        result = subprocess.run(command, capture_output=True, timeout=180)
        (out / (arm + '.stdout.txt')).write_bytes(result.stdout)
        (out / (arm + '.stderr.txt')).write_bytes(result.stderr)
        receipts.append({'command': command, 'exit_code': result.returncode,
                         'elapsed_seconds': time.monotonic() - start})
        write('commands.json', receipts)
        result.check_returncode()
        report = json.loads((out / (arm + '.json')).read_text())
        if report['state'] != 'captured' or len(report['rows']) != 2:
            raise ValueError('incomplete mechanism probe')
    if hashlib.sha256((root / 'semantic.sqlite').read_bytes()).hexdigest() != json.loads((root / 'protocol.json').read_text())['semantic_sha256']:
        raise ValueError('semantic input changed')
    print('exact-term mechanism probe: 8 calls captured separately; source verification pending')


if __name__ == '__main__':
    main()
