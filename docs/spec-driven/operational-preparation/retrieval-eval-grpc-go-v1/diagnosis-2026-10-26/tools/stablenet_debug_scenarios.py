"""Write requery/scenarios/<pr>.yaml so scripts/retrieval_debug_trace_report.py can join debug traces to PRs.

Ground truth here is file level, so each answer file becomes a whole-file expected citation;
the report's ranks then mean "first citation in an answer file". No expected_intent: the PR
set has none, so the report skips the intent check.
"""
import json

import yaml

import run
import run_chain

out = run.E / 'requery' / 'scenarios'
out.mkdir(parents=True, exist_ok=True)
for pid in run_chain.CHAIN:
    pr = json.loads((run.E / 'prs' / f'{pid}.json').read_text())
    sc = {'prompt': run.query_text(pr),
          'expected_citations': [{'file': f, 'start_line': 1, 'end_line': 10 ** 9} for f in pr['ground_truth']]}
    (out / f'{pid}.yaml').write_text(yaml.safe_dump(sc, allow_unicode=True))
print(len(run_chain.CHAIN), 'scenarios in', out)
