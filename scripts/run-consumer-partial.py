#!/usr/bin/env python3
"""T10 actual supported partials and blocked incomplete/placeholder delivery."""
import argparse
import copy
import importlib.util
import json
from pathlib import Path
import shlex
import shutil
import sqlite3
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[1]


def module(name):
    spec = importlib.util.spec_from_file_location(name.replace('-', '_'), ROOT / 'scripts' / name)
    value = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(value)
    return value


host = module('run-consumer-opt-in.py')
base, stdio = host.base, host.stdio
dump, sha, require = base.dump, base.sha, base.require
PLAN = ROOT / 'scripts/fixtures/consumer-partial-plan.json'
FILES = ['deep-review-evidence.json', 'deep-review.json', 'review-report.md', 'sequence.mmd', 'modules.mmd', 'state.json']


def read(path):
    return json.loads(path.read_text())


def bash(parts):
    return {'name': 'Bash', 'input': {'command': shlex.join([str(p) for p in parts]), 'description': 'Owned T10 partial/unknown verification'}}


def tool(name, arguments):
    return {'name': 'mcp__plugin_coding-agent_cks__cks_context_' + name, 'input': arguments}


def workspace(root):
    return root / 'source/.coding-agent/tickets/KS-P06-REVIEW'


def verify(w):
    command = [sys.executable, str(base.CANDIDATE / 'plugin/scripts/deep_review.py'), 'verify', '--workspace', str(w)]
    q = subprocess.run(command, capture_output=True, text=True, timeout=20)
    return {'command': command, 'exit_code': q.returncode, 'stdout': q.stdout, 'stderr': q.stderr}


def receipt(folder, index, raw_error=False):
    rows = [json.loads(line) for line in (folder / 'mcp-frames.jsonl').read_text().splitlines()]
    requests = [json.loads(r['raw']) for r in rows if r['kind'] == 'request' and json.loads(r['raw']).get('method') == 'tools/call']
    q = requests[index]
    response = next(json.loads(r['raw']) for r in rows if r['kind'] == 'response' and json.loads(r['raw']).get('id') == q['id'])
    value = response['result'] if raw_error else stdio.payload(response)
    return {'tool': q['params']['name'], 'arguments': q['params']['arguments'], 'result': value}


def control(root, legacy, case, restore=False):
    require(str(root).startswith('/private/tmp/ks-consumer-role-t04-t10-') and str(legacy).startswith('/private/tmp/ks-consumer-role-t04-t10-'), 'owned T10 roots required')
    require(case in read(PLAN)['controls'], 'declared control required')
    w = workspace(root)
    out = root / 'controls' / case
    if restore:
        dump(out / 'after-attempt.json', base.snapshot(w))
        for name in FILES:
            (w / name).write_bytes((out / 'original' / name).read_bytes())
        result = verify(w)
        dump(out / 'restore.json', {'verification': result, 'workspace': base.snapshot(w)})
        require(result['exit_code'] == 0 and base.snapshot(w) == read(out / 'before.json'), 'exact restore failed')
        print(json.dumps({'control': case, 'restored': True, 'verification': result}))
        return
    (out / 'original').mkdir(parents=True)
    for name in FILES:
        (out / 'original' / name).write_bytes((w / name).read_bytes())
    dump(out / 'before.json', base.snapshot(w))
    bundle, report = read(w / 'deep-review-evidence.json'), read(w / 'deep-review.json')
    if case in ['unsupported', 'truncated-real', 'incomplete-rules']:
        bundle['rules'] = receipt(root / 'T10-matrix', ['unsupported', 'truncated-real', 'incomplete-rules'].index(case))
    elif case == 'requires-reindex':
        bundle['function_review'] = receipt(legacy / 'T10-legacy', 1, raw_error=True)
        require(bundle['function_review']['result'].get('isError') is True and 'requires_reindex' in str(bundle['function_review']['result']), 'actual legacy error required')
    elif case == 'all-unknown':
        for section in report['reasoning']['sections'].values():
            section.update(status='unknown', evidence_nodes=[], assumptions=[], unknowns=['Not independently reviewed; no completion claim.'])
    elif case == 'missing-section':
        report['reasoning']['sections'].pop('io')
    elif case == 'truncated-flag':
        bundle['rules']['result']['analysis']['truncated'] = True
    elif case == 'unknown-observed':
        report['reasoning']['sections']['resources']['status'] = 'observed'
    elif case == 'unknown-empty':
        report['reasoning']['sections']['resources']['unknowns'] = []
    elif case == 'safe-status':
        report['status'] = 'complete_safe'
    if case in ['unsupported', 'truncated-real', 'incomplete-rules', 'requires-reindex', 'truncated-flag']:
        dump(w / 'deep-review-evidence.json', bundle)
    else:
        dump(w / 'deep-review.json', report)
    result = verify(w)
    dump(out / 'mutated.json', {'verification': result, 'workspace': base.snapshot(w), 'origin': 'actual_MCP_receipt' if case in ['unsupported', 'truncated-real', 'incomplete-rules', 'requires-reindex'] else 'local_verifier_mutation'})
    require(result['exit_code'] == 2 and json.loads(result['stdout'])['status'] == 'invalid', 'invalid partial/placeholder accepted')
    print(json.dumps({'control': case, 'expected_invalid': True, 'verification': result}))


def run(root, legacy, field):
    pins = read(host.PRIOR / 'binary-pins.json')
    for folder in [root, legacy, field]:
        base.setup(folder)
        dump(folder / 'binary-pins.json', pins)
        w = workspace(folder)
        w.mkdir(parents=True)
        dump(w / 'state.json', {'ticket_id': 'KS-P06-REVIEW', 'ticket_type': 'code_review', 'current_state': 'ANALYSIS', 'states': {'ANALYSIS': {'status': 'in_progress'}, 'PLANNING': {'status': 'pending'}}})
    old_db = legacy / 'legacy-graph'
    shutil.copytree(legacy / 'graph', old_db)
    before = sha(old_db / 'graph.db')
    with sqlite3.connect(old_db / 'graph.db') as db:
        db.execute('DROP TABLE code_edges')
        db.commit()
        checkpoint = db.execute('PRAGMA wal_checkpoint(TRUNCATE)').fetchall()
    dump(root / 'legacy-index-control.json', {'operation': 'DROP TABLE code_edges; checkpoint WAL', 'source_db_sha256': before, 'legacy_db_sha256': sha(old_db / 'graph.db'), 'checkpoint': checkpoint, 'owned_schema_fault_not_operational_outage': True})
    (legacy / 'cks.yaml').write_text((legacy / 'cks.yaml').read_text().replace(str(legacy / 'graph/graph.db'), str(old_db / 'graph.db')))
    rows = read(base.P05 / 'sessions/discovery/messages.json')
    nodes = {r['label'][5:]: r['response']['result']['structuredContent']['nodes'][0] for r in rows if r['label'].startswith('find-')}
    record = read(base.P05 / 'execution.json')
    query = {**record['query'], 'expected_snapshot_sha256': next(r['response']['result']['structuredContent']['snapshot_sha256'] for r in read(base.P05 / 'sessions/main/messages.json') if r['label'] == 'baseline')}
    field_query = {**query, 'node_id': nodes['Field']['id'], 'expected_source_sha256': __import__('hashlib').sha256(nodes['Field']['source'].encode()).hexdigest()}
    matrix = [
        {'label': 'unsupported', 'arguments': {'rule_pack': 'go-local-review@1.0.0', 'function': {k: v for k, v in {**query, 'node_id': nodes['Box']['id']}.items() if k != 'expected_source_sha256'}}},
        {'label': 'truncated-real', 'arguments': {'rule_pack': 'go-local-review@1.0.0', 'function': {**query, 'limits': {'max_nodes': 1}}}},
        {'label': 'incomplete-rules', 'arguments': {'rule_pack': 'go-local-review@1.0.0', 'function': query, 'max_flow_steps': 1}},
    ]
    dump(root / 'matrix-cases.json', matrix)
    for owned, q, symbol, case in [(root, query, record['selected']['canonical_id'], 'T10-build-seed'), (field, field_query, nodes['Field']['canonical_id'], 'T10-build-field')]:
        w = workspace(owned)
        actions = [tool('rule_candidates', {'rule_pack': 'go-local-review@1.0.0', 'function': q}), tool('review_function', {'symbol': symbol}), bash([sys.executable, ROOT / 'scripts/consumer-role-adapter.py', 'build', '--plugin', base.CANDIDATE / 'plugin', '--frames', owned / case / 'mcp-frames.jsonl', '--workspace', w]), {'name': 'Read', 'input': {'file_path': str(w / 'state.json')}}, {'state_operation': 'handoff'}]
        host.run_case(owned, case, 'analyzer', w, actions)
    host.run_case(root, 'T10-matrix', 'analyzer', workspace(root), [tool('rule_candidates', r['arguments']) for r in matrix])
    host.run_case(legacy, 'T10-legacy', 'analyzer', workspace(legacy), [tool('rule_candidates', {'rule_pack': 'go-local-review@1.0.0', 'function': query}), tool('review_function', {'symbol': record['selected']['canonical_id']})])
    actions = [{'name': 'Read', 'input': {'file_path': str(workspace(root) / 'state.json')}}]
    for case in read(PLAN)['controls']:
        command = [sys.executable, Path(__file__).resolve(), '--root', root, '--legacy', legacy, '--field', field, '--control', case]
        actions += [bash(command), {'name': 'Read', 'input': {'file_path': str(workspace(root) / 'state.json')}}, {'state_operation': 'complete'}, bash(command + ['--restore'])]
    actions += [{'name': 'Read', 'input': {'file_path': str(workspace(root) / 'state.json')}}, {'state_operation': 'complete'}, bash([sys.executable, base.CANDIDATE / 'plugin/scripts/deep_review.py', 'verify', '--workspace', workspace(root)])]
    host.run_case(root, 'T10-controls', 'planner', workspace(root), actions)
    host.run_case(field, 'T10-field-delivery', 'planner', workspace(field), [{'name': 'Read', 'input': {'file_path': str(workspace(field) / 'state.json')}}, bash([sys.executable, base.CANDIDATE / 'plugin/scripts/deep_review.py', 'verify', '--workspace', workspace(field)]), {'state_operation': 'complete'}, bash([sys.executable, base.CANDIDATE / 'plugin/scripts/deep_review.py', 'verify', '--workspace', workspace(field)])])
    dump(root / 'execution.json', {'query': query, 'field_query': field_query, 'root': str(root), 'legacy': str(legacy), 'field': str(field), 'driver_sha256': sha(Path(__file__)), 'plan_sha256': sha(PLAN), 'host_helper_sha256': sha(ROOT / 'scripts/run-consumer-opt-in.py'), 'host_cases': read(PLAN)['host_cases'], 'candidate_adopted': False, 'real_model_calls': 0, 'formal_runs': 0})


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ['root', 'legacy', 'field']:
        parser.add_argument('--' + name, type=Path, required=True)
    parser.add_argument('--control')
    parser.add_argument('--restore', action='store_true')
    args = parser.parse_args()
    paths = [getattr(args, name).resolve() for name in ['root', 'legacy', 'field']]
    require(all(str(p).startswith('/private/tmp/ks-consumer-role-t04-t10-') for p in paths), 'owned T10 roots required')
    if args.control:
        control(paths[0], paths[1], args.control, args.restore)
    else:
        run(*paths)
