#!/usr/bin/env python3
"""Reproduce the retained diagnostic reports without invoking MCP or a model."""
import argparse
import hashlib
import json
from pathlib import Path
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[4]
DATA = ROOT / 'system/eval/b0-knowledge-system'
GOLDS = Path(__file__).resolve().parent


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def write(path, value):
    with path.open('x', encoding='utf-8') as stream:
        json.dump(value, stream, ensure_ascii=False, indent=2)
        stream.write('\n')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--out-dir', type=Path, required=True, help='new outputs; existing evidence is never overwritten')
    args = parser.parse_args()
    args.out_dir.mkdir(parents=True, exist_ok=True)
    records, frozen = [], {}
    for capture in ['matrix-capture', 'environment-ledger']:
        for model in ['real', 'mock']:
            name = capture + '-' + model
            folder = DATA / (capture + '-m2max-2026-10-04') / model
            source = DATA / 'b1-pack-matrix-m2max-2026-10-04' / model / 'source'
            gold, target = GOLDS / (name + '-gold.json'), args.out_dir / (name + '-summary.json')
            for path in [p for p in folder.rglob('*') if p.is_file()] + [p for p in source.rglob('*') if p.is_file()] + [gold]:
                frozen[str(path.relative_to(ROOT))] = digest(path)
            command = [sys.executable, str(ROOT / 'scripts/b1-summarize-matrix.py'), '--capture-dir', str(folder),
                       '--gold', str(gold), '--source-root', str(source), '--diagnostic-controls', '--output', str(target)]
            result = subprocess.run(command, capture_output=True, text=True)
            write(args.out_dir / (name + '-cli.log'), {'command': command, 'exit_code': result.returncode,
                                                      'stdout': result.stdout, 'stderr': result.stderr})
            if result.returncode:
                raise ValueError(name + ': ' + result.stderr)
            summary = json.loads(target.read_bytes())
            record = {'capture': name, 'report': target.name, 'status': summary['status'],
                      'observed_rows': summary['observed_rows'], 'planned_rows': summary['planned_rows'],
                      'request_variants': summary['request_variants'], 'independent_units': summary['independent_units'],
                      'failed_rows': sum(bool(m['issues']) for m in summary['measurements']),
                      'missing_backend_measurements': sum(m['backend_measurement'] is None for m in summary['measurements']),
                      'backend_nonreturned_attempts': sum((m['backend_measurement'] or {}).get('nonreturned_attempts', 0) for m in summary['measurements']),
                      'observed_ckv_k': sorted({k for m in summary['measurements'] if m['backend_measurement'] for k in m['backend_measurement']['observed_ckv_k']}),
                      'quality_metrics': None}
            records.append(record)
    after = {path: digest(ROOT / path) for path in frozen}
    if after != frozen:
        raise ValueError('retained input bytes changed')
    write(args.out_dir / 'replay-input-audit.json', {'before_sha256': frozen, 'after_sha256': after,
                                                   'unchanged': True, 'file_count': len(frozen)})
    write(args.out_dir / 'replay-summary.json', {'schema_version': 1, 'diagnostic_only': True,
        'captures': records, 'sdk_rows_total': sum(r['observed_rows'] for r in records),
        'captures_not_pooled': True, 'quality_metrics': None, 'human_verdicts': None})
    print(json.dumps({'sdk_rows_total': sum(r['observed_rows'] for r in records), 'frozen_files': len(frozen),
                      'unchanged': True, 'captures': records}))


if __name__ == '__main__':
    main()
