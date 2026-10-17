#!/usr/bin/env python3
"""T09 actual two-project pinned tool and frozen delivery controls."""
import argparse
import copy
import importlib.util
import json
from pathlib import Path
import shlex
import subprocess
import sys
import time

ROOT = Path(__file__).resolve().parents[1]


def module(name):
    spec = importlib.util.spec_from_file_location(name.replace('-', '_'), ROOT / 'scripts' / name)
    value = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(value)
    return value


host = module('run-consumer-opt-in.py')
base, stdio = host.base, host.stdio
dump, sha, require = base.dump, base.sha, base.require
PLAN = ROOT / 'scripts/fixtures/consumer-foreign-pin-plan.json'
FILES = ['deep-review-evidence.json', 'deep-review.json', 'review-report.md', 'sequence.mmd', 'modules.mmd', 'state.json']


def read(path):
    return json.loads(path.read_text())


def bash(parts):
    return {'name': 'Bash', 'input': {'command': shlex.join([str(x) for x in parts]), 'description': 'Owned T09 pin and delivery control'}}


def tool(name, arguments):
    return {'name': 'mcp__plugin_coding-agent_cks__cks_context_' + name, 'input': arguments}


def matrix(query, foreign, other_node):
    rows = []
    for name in ['value_dependencies', 'rule_candidates', 'value_dependency_expansion', 'policy_security']:
        variants = [('node', query, 'positive'), ('canonical', {**{k: v for k, v in query.items() if k != 'node_id'}, 'canonical_id': read(base.P05 / 'execution.json')['selected']['canonical_id']}, 'positive')]
        variants += [('other-function', {**query, 'node_id': other_node}, 'source_mismatch'), ('foreign-node', {k: v for k, v in foreign.items() if k != 'canonical_id'}, 'source_mismatch'), ('local-id-collision', {**query, 'node_id': foreign['node_id']}, 'positive'), ('foreign-canonical', {**{k: v for k, v in query.items() if k != 'node_id'}, 'canonical_id': foreign['canonical_id']}, 'not_found')]
        for label, field in [('commit', 'expected_commit'), ('source', 'expected_source_sha256'), ('snapshot', 'expected_snapshot_sha256')]:
            variants.append((label, {**query, field: foreign[field]}, 'source_mismatch'))
        for label, function, expected in variants:
            arguments = {'rule_pack': 'go-local-review@1.0.0', 'function': function} if name == 'rule_candidates' else {'function': function, 'limits': {'max_functions': 8, 'max_hops': 3}} if name == 'value_dependency_expansion' else function
            rows.append({'label': name + '-' + label, 'tool': name, 'arguments': arguments, 'expected': expected})
    return rows


def verify(workspace):
    command = [sys.executable, str(base.CANDIDATE / 'plugin/scripts/deep_review.py'), 'verify', '--workspace', str(workspace)]
    p = subprocess.run(command, capture_output=True, text=True, timeout=20)
    return {'command': command, 'exit_code': p.returncode, 'stdout': p.stdout, 'stderr': p.stderr}


def control(root, foreign_root, workspace, case, restore=False):
    require(str(root).startswith('/private/tmp/ks-consumer-role-t04-t09-') and workspace.is_relative_to(root / 'source/.coding-agent/tickets'), 'owned control only')
    require(case in read(PLAN)['controls'], 'declared control required')
    out = root / 'controls' / case
    if restore:
        before = read(out / 'before.json')
        dump(out / 'after-attempt.json', base.snapshot(workspace))
        for name in FILES:
            (workspace / name).write_bytes((out / 'original' / name).read_bytes())
        result = verify(workspace)
        dump(out / 'restore.json', {'verification': result, 'workspace': base.snapshot(workspace)})
        require(result['exit_code'] == 0 and base.snapshot(workspace) == before, 'restore must preserve original bytes')
        print(json.dumps({'control': case, 'restored': True, 'verification': result}))
        return
    out.mkdir(parents=True)
    (out / 'original').mkdir()
    for name in FILES:
        (out / 'original' / name).write_bytes((workspace / name).read_bytes())
    dump(out / 'before.json', base.snapshot(workspace))
    foreign = read(root / 'execution-input.json')['foreign']
    bundle = read(workspace / 'deep-review-evidence.json')
    function = bundle['rules']['arguments']['function']
    if case == 'node':
        function['node_id'] = read(root / 'execution-input.json')['other_node']
    elif case == 'canonical':
        function.pop('node_id')
        function['canonical_id'] = foreign['canonical_id']
    elif case == 'file':
        bundle['function_review']['result']['seed']['file_path'] = 'other-project/logic.go'
    elif case in ['commit', 'source', 'snapshot']:
        field = {'commit': 'expected_commit', 'source': 'expected_source_sha256', 'snapshot': 'expected_snapshot_sha256'}[case]
        function[field] = foreign[field]
    elif case == 'ticket':
        state = read(workspace / 'state.json')
        state['ticket_id'] = 'KS-T09-OTHER'
        dump(workspace / 'state.json', state)
    elif case == 'foreign':
        other = foreign_root / 'source/.coding-agent/tickets/KS-P06-REVIEW'
        for name in FILES[:-1]:
            (workspace / name).write_bytes((other / name).read_bytes())
    if case not in ['ticket', 'foreign']:
        dump(workspace / 'deep-review-evidence.json', bundle)
    result = verify(workspace)
    dump(out / 'mutated.json', {'verification': result, 'workspace': base.snapshot(workspace)})
    require(result['exit_code'] == 2 and json.loads(result['stdout'])['status'] == 'invalid', 'foreign mismatch incorrectly validated')
    print(json.dumps({'control': case, 'expected_invalid': True, 'verification': result}))


def run(root, foreign_root):
    require(not root.exists() and not foreign_root.exists(), 'fresh owned roots required')
    base.setup(root)
    base.setup(foreign_root)
    pins = read(host.PRIOR / 'binary-pins.json')
    for folder in [root, foreign_root]:
        dump(folder / 'binary-pins.json', pins)
    ckg = Path('/private/tmp/ks-host-schema-runtime-20261017/bin/ckg')
    require(sha(ckg) == pins['ckg_sha256'], 'CKG executable pin')
    repo = foreign_root / 'source'
    (repo / 'go.mod').write_text((repo / 'go.mod').read_text().replace('example.invalid/consumer-stdio-fixture', 'example.invalid/foreign-pin-fixture'))
    (repo / 'logic.go').write_text((repo / 'logic.go').read_text().replace('command = "fixed"', 'command = "foreign-fixed"'))
    commands = []
    def command(label, argv, cwd=repo):
        started = time.monotonic()
        p = subprocess.run(argv, cwd=cwd, capture_output=True, text=True, timeout=180)
        commands.append({'label': label, 'argv': argv, 'cwd': str(cwd), 'exit_code': p.returncode, 'stdout': p.stdout, 'stderr': p.stderr, 'elapsed_seconds': time.monotonic() - started})
        dump(root / 'commands.json', commands)
        require(p.returncode == 0, 'owned command failed: ' + label)
        return p.stdout.strip()
    command('format', ['gofmt', '-w', 'logic.go'])
    command('fixture-test', ['go', 'test', './...'])
    command('fixture-vet', ['go', 'vet', './...'])
    command('git-add', ['git', 'add', 'go.mod', 'logic.go'])
    command('git-commit', ['git', '-c', 'user.name=Owned T09 fixture', '-c', 'user.email=fixture@example.invalid', '-c', 'commit.gpgsign=false', 'commit', '-qm', 'test: distinct foreign pin fixture'])
    commit = command('git-head', ['git', 'rev-parse', 'HEAD'])
    graph = foreign_root / 'foreign-graph'
    command('index', [str(ckg), 'build', '--src', str(repo), '--out', str(graph), '--lang', 'go', '--policy-file', str(repo / 'policy.yaml'), '--security-pattern-file', str(repo / 'security.yaml')])
    (foreign_root / 'cks.yaml').write_text((foreign_root / 'cks.yaml').read_text().replace(str(foreign_root / 'graph/graph.db'), str(graph / 'graph.db')))
    discovery = stdio.OwnedClient([str(ckg), 'mcp', '--graph', str(graph), '--namespace', 'flow_probe'], root / 'discovery-B')
    try:
        discovery.initialize()
        found = stdio.payload(discovery.request('find-Seed', 'tools/call', {'name': 'flow_probe.context.find_symbol', 'arguments': {'name': 'Seed', 'exact': False, 'include_blobs': True}}))['nodes']
        selected = next(node for node in found if node['name'] == 'Seed')
    finally:
        discovery.close()
    record = read(base.P05 / 'execution.json')
    query = {**record['query'], 'expected_snapshot_sha256': next(r['response']['result']['structuredContent']['snapshot_sha256'] for r in read(base.P05 / 'sessions/main/messages.json') if r['label'] == 'baseline')}
    foreign_query = {'node_id': selected['id'], 'expected_commit': commit, 'expected_source_sha256': sha_bytes(selected['source'].encode())}
    native = stdio.OwnedClient([str(base.BINARY), 'mcp', '--config', str(foreign_root / 'cks.yaml')], root / 'native-B')
    try:
        native.initialize()
        positive = stdio.payload(native.request('B-positive', 'tools/call', {'name': 'cks.context.value_dependencies', 'arguments': dict(foreign_query)}))
        foreign_query['expected_snapshot_sha256'] = positive['snapshot_sha256']
        stale = stdio.payload(native.request('stale-A-on-B', 'tools/call', {'name': 'cks.context.value_dependencies', 'arguments': query}))
        require(stale['status'] == 'source_mismatch' and stale.get('analysis') is None, 'stale A request accepted by B')
    finally:
        native.close()
    foreign = {**foreign_query, 'canonical_id': selected['canonical_id']}
    other_node = next(r['response']['result']['structuredContent']['nodes'][0]['id'] for r in read(base.P05 / 'sessions/discovery/messages.json') if r['label'] == 'find-Empty')
    dump(root / 'execution-input.json', {'query': query, 'foreign': foreign, 'foreign_selected': selected, 'other_node': other_node, 'foreign_root': str(foreign_root), 'driver_sha256': sha(Path(__file__)), 'plan_sha256': sha(PLAN)})
    rows = matrix(query, foreign, other_node)
    dump(root / 'matrix-cases.json', rows)
    workspace = root / 'source/.coding-agent/tickets/KS-P06-REVIEW'
    foreign_workspace = foreign_root / 'source/.coding-agent/tickets/KS-P06-REVIEW'
    for folder in [workspace, foreign_workspace]:
        folder.mkdir(parents=True)
        dump(folder / 'state.json', {'ticket_id': 'KS-P06-REVIEW', 'ticket_type': 'code_review', 'current_state': 'ANALYSIS', 'states': {'ANALYSIS': {'status': 'in_progress'}, 'PLANNING': {'status': 'pending'}}})
    host.run_case(root, 'T09-matrix', 'analyzer', workspace, [tool(r['tool'], r['arguments']) for r in rows])
    for owned, w, q, symbol, case in [(root, workspace, query, record['selected']['canonical_id'], 'T09-build-A'), (foreign_root, foreign_workspace, foreign_query, selected['canonical_id'], 'T09-build-B')]:
        actions = [tool('rule_candidates', {'rule_pack': 'go-local-review@1.0.0', 'function': q}), tool('review_function', {'symbol': symbol}), bash([sys.executable, ROOT / 'scripts/consumer-role-adapter.py', 'build', '--plugin', base.CANDIDATE / 'plugin', '--frames', owned / case / 'mcp-frames.jsonl', '--workspace', w]), {'name': 'Read', 'input': {'file_path': str(w / 'state.json')}}, {'state_operation': 'handoff'}]
        host.run_case(owned, case, 'analyzer', w, actions)
    actions = [{'name': 'Read', 'input': {'file_path': str(workspace / 'state.json')}}]
    for case in read(PLAN)['controls']:
        args = [sys.executable, Path(__file__).resolve(), '--root', root, '--foreign-root', foreign_root, '--workspace', workspace, '--control', case]
        actions += [bash(args), {'name': 'Read', 'input': {'file_path': str(workspace / 'state.json')}}, {'state_operation': 'complete'}]
        if case == 'foreign':
            state = read(workspace / 'state.json')
            state['current_state'] = 'COMPLETED'
            state['states']['ANALYSIS']['deep_review_sha256'] = sha(foreign_workspace / 'deep-review.json')
            state['states']['PLANNING'] = {'status': 'completed', 'review_report_sha256': sha(foreign_workspace / 'review-report.md')}
            actions.append({'name': 'Write', 'input': {'file_path': str(workspace / 'state.json'), 'content': json.dumps(state)}})
        actions.append(bash(args + ['--restore']))
    actions += [{'name': 'Read', 'input': {'file_path': str(workspace / 'state.json')}}, {'state_operation': 'complete'}, bash([sys.executable, base.CANDIDATE / 'plugin/scripts/deep_review.py', 'verify', '--workspace', workspace])]
    host.run_case(root, 'T09-delivery', 'planner', workspace, actions)
    dump(root / 'execution.json', {**read(root / 'execution-input.json'), 'host_helper_sha256': sha(ROOT / 'scripts/run-consumer-opt-in.py'), 'host_cases': read(PLAN)['host_cases'], 'candidate_adopted': False, 'real_model_calls': 0, 'formal_runs': 0})


def sha_bytes(raw):
    import hashlib
    return hashlib.sha256(raw).hexdigest()


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--root', type=Path, required=True)
    parser.add_argument('--foreign-root', type=Path, required=True)
    parser.add_argument('--workspace', type=Path)
    parser.add_argument('--control')
    parser.add_argument('--restore', action='store_true')
    args = parser.parse_args()
    root, foreign = args.root.resolve(), args.foreign_root.resolve()
    require(str(root).startswith('/private/tmp/ks-consumer-role-t04-t09-') and str(foreign).startswith('/private/tmp/ks-consumer-role-t04-t09-'), 'owned T09 roots required')
    if args.control:
        control(root, foreign, args.workspace.resolve(), args.control, args.restore)
    else:
        run(root, foreign)
