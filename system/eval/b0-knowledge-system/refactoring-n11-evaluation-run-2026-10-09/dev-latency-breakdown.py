#!/usr/bin/env python3
"""Split DEV warm latency into backend-reported and non-backend time per arm."""
import gzip, json, statistics, sys
from pathlib import Path
d = Path(sys.argv[1])
backend = {}
for f in sorted(d.glob('footprint-*.jsonl.gz')):
    for line in gzip.open(f, 'rt'):
        e = json.loads(line)
        if e.get('event') == 'measurement.backend_calls' and e['summary'].get('tool') == 'cks.context.get_for_task_v2':
            backend[e['summary']['measurement_id']] = sum(c.get('elapsed_ns', 0) for c in e['summary']['calls'])
arms = {}
for line in gzip.open(d / 'rows.jsonl.gz', 'rt'):
    r = json.loads(line)
    if r['phase'] == 'warm_latency':
        c = r['call']; b = backend[c['measurement_id']]
        arms.setdefault(r['arm'], []).append((c['elapsed_ns'], b, c['elapsed_ns'] - b))
out = {a: {'warm_rows': len(v), 'elapsed_p50_ms': statistics.median(x[0] for x in v) / 1e6,
           'backend_p50_ms': statistics.median(x[1] for x in v) / 1e6,
           'non_backend_p50_ms': statistics.median(x[2] for x in v) / 1e6} for a, v in sorted(arms.items())}
print(json.dumps(out, indent=2))
