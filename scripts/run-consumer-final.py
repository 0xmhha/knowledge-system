#!/usr/bin/env python3
"""T22 bounded read-only environment census and owner-guarded restore/query."""
import argparse
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import plistlib
import shutil
import socket
import subprocess
import uuid

ROOT = Path(__file__).resolve().parents[1]
HOME_ROOT = Path('/Users/kevin')
PLAN = ROOT / 'scripts/fixtures/consumer-final-plan.json'
BASE_COMMIT = '39f4554034ae8fb9ad7a87b8b22b9a21cbebf368'
REPOS = {'cks_postrun': '/private/tmp/ks-host-schema-cks-20261017', 'consumer_postrun': '/private/tmp/ks-host-schema-consumer-20261017', 'original_postrun': '/private/tmp/ks-isolated-host-20261015/coding-agent', 'prior_candidate_postrun': '/private/tmp/ks-consumer-contract-candidate-20261017', 'chainbench': '/private/tmp/ks-isolated-host-20261015/chainbench'}


def module(name):
    spec = importlib.util.spec_from_file_location(name.replace('-', '_'), ROOT / 'scripts' / name)
    value = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(value)
    return value


base = module('consumer-compatible-host.py')
stdio = module('prepare-consumer-stdio-readiness.py')
own = module('consumer-process-ownership.py')
dump, sha, require = base.dump, base.sha, base.require


def run(command, cwd=ROOT):
    return subprocess.run(command, cwd=cwd, capture_output=True, text=True, timeout=30)


def git(path, *args):
    p = run(['git', '-C', str(path), *args])
    require(p.returncode == 0, 'local git read failed: ' + args[0])
    return p.stdout


def fingerprint(path):
    if path.is_symlink():
        return {'kind': 'symlink', 'target': os.readlink(path)}
    if not path.exists():
        return {'kind': 'absent'}
    require(path.is_file(), 'expected file or symlink: ' + str(path))
    return {'kind': 'file', 'sha256': sha(path), 'mode': path.stat().st_mode & 0o777, 'bytes': path.stat().st_size}


def global_state(plan):
    files = {name: fingerprint(HOME_ROOT / name) for name in plan['protected_global_files']}
    # Exclude unrelated per-session history/auth fields; persist only hashes of
    # the relevant MCP/plugin configuration projections, never their contents.
    p = HOME_ROOT / '.claude.json'
    if p.exists():
        d = json.loads(p.read_text())
        projection = {'mcpServers': d.get('mcpServers', {}), 'project_mcp': d.get('projects', {}).get(str(ROOT), {}).get('mcpServers', {})}
        files['.claude.json:selected_mcp_configuration'] = {'kind': 'json_projection', 'sha256': hashlib.sha256(json.dumps(projection, sort_keys=True).encode()).hexdigest()}
    else:
        files['.claude.json:selected_mcp_configuration'] = {'kind': 'absent'}
    for name in ['cks', 'ckg', 'ckv', 'chainbench-mcp', 'claude']:
        p = HOME_ROOT / '.local/bin' / name
        files['.local/bin/' + name] = fingerprint(p)
        if p.is_symlink() and p.resolve().is_file():
            files['.local/bin/' + name + ':resolved'] = fingerprint(p.resolve())
    for label, root in {'maintained': str(ROOT), **REPOS}.items():
        for name in plan['project_config_names']:
            files[label + ':' + name] = fingerprint(Path(root) / name)
    agents = HOME_ROOT / 'Library/LaunchAgents'
    for p in sorted(agents.glob('*.plist')):
        d = plistlib.loads(p.read_bytes())
        args = d.get('ProgramArguments', [])
        program = d.get('Program') or (args[0] if args else '')
        if Path(program).name in ['cks', 'ckg', 'ckv', 'chainbench-mcp']:
            files['LaunchAgents/' + p.name] = fingerprint(p)
    return files


def operations(plan):
    root = Path(plan['operational_dataset_root'])
    metadata = {}
    # Walk without following symlinks and with a fixed maximum depth. Never
    # open live DBs or resolve a dataset pointer into a mutation target.
    if root.exists():
        for parent, dirs, files in os.walk(root, followlinks=False):
            rel = Path(parent).relative_to(root)
            if len(rel.parts) >= plan['operational_scan_depth']:
                dirs[:] = []
            for name in [*dirs, *files]:
                p = Path(parent) / name
                if len(p.relative_to(root).parts) <= plan['operational_scan_depth'] and (p.is_symlink() or name in plan['operational_metadata_names']):
                    metadata[str(p.relative_to(root))] = fingerprint(p)
    p = run(['ps', '-axo', 'pid=,comm='])
    require(p.returncode == 0, 'engine census failed')
    engines = []
    for line in p.stdout.splitlines():
        row = line.strip().split(None, 1)
        if len(row) == 2 and Path(row[1]).name in ['cks', 'ckg', 'ckv', 'chainbench-mcp']:
            identity = own.snapshot(int(row[0]))
            require(identity is not None, 'engine changed during census')
            parent = own.snapshot(identity['ppid'])
            engines.append({'pid': identity['pid'], 'ppid': identity['ppid'], 'birth': identity['birth'], 'binary': Path(row[1]).name, 'command_sha256': hashlib.sha256(identity['command'].encode()).hexdigest(), 'parent_binary': Path(parent['command'].split()[0]).name if parent else None, 'owned_by_T22': False})
    p = run(['launchctl', 'list'])
    require(p.returncode == 0, 'read-only launchd census failed')
    labels = []
    for line in p.stdout.splitlines():
        row = line.split()
        if len(row) == 3 and not row[2].startswith('com.apple.') and any(k in row[2].lower() for k in ['knowledge', '.cks', '.ckg', '.ckv', 'chainbench']):
            labels.append({'pid': row[0], 'last_exit': row[1], 'label': row[2]})
    return {'root_exists': root.exists(), 'metadata': dict(sorted(metadata.items())), 'engines': sorted(engines, key=lambda r: r['pid']), 'launchd_services': sorted(labels, key=lambda r: r['label']), 'full_database_bytes_hashed': False, 'availability_or_auth_probed': False}


def sources():
    repos = {}
    for label, path in REPOS.items():
        commit = git(path, 'rev-parse', 'HEAD').strip()
        verified = run(['git', '-C', path, 'verify-commit', commit])
        repos[label] = {'commit': commit, 'tree': git(path, 'rev-parse', 'HEAD^{tree}').strip(), 'porcelain': git(path, 'status', '--porcelain'), 'signature_exit': verified.returncode, 'signature_output_sha256': hashlib.sha256((verified.stdout + verified.stderr).encode()).hexdigest(), 'remote_refs': git(path, 'for-each-ref', '--format=%(refname) %(objectname)', 'refs/remotes')}
    files = git(ROOT, 'ls-tree', '-r', '--name-only', BASE_COMMIT, '--', 'cmd', 'internal', 'pkg', 'go.mod', 'go.sum').splitlines()
    protected = {name: fingerprint(ROOT / name) for name in files}
    for name in ['docs/REMAINING-WORK.md', 'docs/spec-driven/CURRENT-HANDOFF.md']:
        protected[name] = fingerprint(ROOT / name)
    return {'repos': repos, 'maintained_files': protected, 'binaries': {name: sha(Path('/private/tmp/ks-host-schema-runtime-20261017/bin') / name) for name in ['cks', 'ckg']}, 'host_sha256': sha(base.HOST), 'chainbench_sha256': sha(Path('/private/tmp/ks-isolated-host-20261015/bin/chainbench-mcp')), 'root_head': git(ROOT, 'rev-parse', 'HEAD').strip(), 'root_remote_refs': git(ROOT, 'for-each-ref', '--format=%(refname) %(objectname)', 'refs/remotes')}


def safe_owned(root, path, token):
    require(str(root).startswith('/private/tmp/ks-consumer-final-') and root.resolve() == root and not root.is_symlink(), 'fresh owned root required')
    require((root / 'owner-token.txt').read_text() == token, 'owned token differs')
    require(path != root and path.resolve().is_relative_to(root), 'foreign path or symlink escape refused')
    return path


def owned_state(root, plan):
    return {name: fingerprint(root / name) for name in [*plan['owned_restore_files'], 'active']}


def restore(root, token, plan):
    for name in plan['owned_restore_files']:
        p = safe_owned(root, root / name, token)
        p.write_bytes((root / 'backup' / name).read_bytes())
    p = root / 'active'
    require(p.is_symlink() and p.parent == root, 'owned pointer required')
    safe_owned(root, p, token)
    p.unlink()
    p.symlink_to('graph')


def collect(root):
    plan = json.loads(PLAN.read_text())
    require(not root.exists(), 'fresh collection required')
    before = {'global': global_state(plan), 'operations': operations(plan), 'sources': sources()}
    require(before['sources']['root_head'] == BASE_COMMIT, 'base HEAD changed')
    base.setup(root)
    token = uuid.uuid4().hex
    (root / 'owner-token.txt').write_text(token)
    (root / 'active').symlink_to('graph')
    dump(root / 'before.json', before)
    original = owned_state(root, plan)
    for name in plan['owned_restore_files']:
        p = root / 'backup' / name
        p.parent.mkdir(parents=True, exist_ok=True)
        p.write_bytes((root / name).read_bytes())
    controls = []
    for label, target in [('foreign', HOME_ROOT / '.claude/settings.json'), ('symlink-escape', root / 'escape/settings.json')]:
        if label == 'symlink-escape':
            (root / 'escape').symlink_to(HOME_ROOT / '.claude')
        try:
            safe_owned(root, target, token)
        except ValueError as error:
            controls.append({'case': label, 'refused_before_mutation': True, 'reason': str(error)})
        else:
            raise ValueError('foreign mutation path accepted')
    (root / 'escape').unlink()
    # Mutate only predeclared owned files/pointer; restore even if comparison
    # or the positive query fails. Operational roots never enter this function.
    try:
        safe_owned(root, root / 'source/logic.go', token).write_text((root / 'source/logic.go').read_text() + '\n// owned T22 rollback fault\n')
        safe_owned(root, root / 'graph/graph.db', token).write_bytes(b'owned T22 damaged graph\n')
        safe_owned(root, root / 'cks.yaml', token).write_text('owned T22 invalid config\n')
        (root / 'fault-version').mkdir()
        (root / 'active').unlink()
        (root / 'active').symlink_to('fault-version')
        changed = owned_state(root, plan)
        require(changed != original and sum(changed[n] != original[n] for n in original) == 4, 'restore fault not observed')
        dump(root / 'restore-control.json', {'original': original, 'mutated': changed, 'comparison_before_restore': 'rejected_not_byte_identical', 'boundary_controls': controls})
    finally:
        restore(root, token, plan)
    restored = owned_state(root, plan)
    require(restored == original and git(root / 'source', 'status', '--porcelain') == '', 'owned restore not exact')
    query = dict(json.loads((base.P05 / 'execution.json').read_text())['query'])
    query['expected_snapshot_sha256'] = next(r['response']['result']['structuredContent']['snapshot_sha256'] for r in json.loads((base.P05 / 'sessions/main/messages.json').read_text()) if r['label'] == 'baseline')
    client = stdio.OwnedClient([str(base.BINARY), 'mcp', '--config', str(root / 'cks.yaml')], root / 'query')
    identity = own.snapshot(client.proc.pid)
    try:
        client.initialize()
        response = client.tool('restored-pinned-query', 'value_dependencies', query)
        value = stdio.payload(response)
        require(value['status'] == 'partial' and value['analysis'] is not None, 'restored query unavailable')
    finally:
        client.close()
    dump(root / 'restored.json', {'state': restored, 'source_git_clean': True, 'query': query, 'identity': identity, 'after_query': owned_state(root, plan)})
    # An actually listening owned socket is the positive control. Its closure
    # is separate from old endpoints, which may have been reused by others.
    listener = socket.socket()
    listener.bind(('127.0.0.1', 0))
    listener.listen(2)
    address = listener.getsockname()
    def connect():
        probe = socket.socket()
        probe.settimeout(.5)
        try:
            return probe.connect_ex(address)
        finally:
            probe.close()
    live = connect()
    listener.close()
    closed = connect()
    require(live == 0 and closed != 0 and listener.fileno() == -1, 'owned socket cleanup')
    dump(root / 'listener.json', {'address': list(address), 'before_connect_errno': live, 'after_connect_errno': closed, 'socket_fileno_after_close': listener.fileno(), 'listener_owned_by_this_runner': True, 'old_endpoints_not_contacted': True})
    after = {'global': global_state(plan), 'operations': operations(plan), 'sources': sources()}
    dump(root / 'after.json', after)
    require(before == after, 'protected source/global/operations/refs changed')
    audit = run(['ps', '-p', str(client.proc.pid), '-o', 'pid=,ppid=,stat=,lstart=,command='])
    self_probe = run(['ps', '-p', str(os.getpid()), '-o', 'pid='])
    dump(root / 'cleanup.json', {'pid': os.getpid(), 'self_probe': {'exit_code': self_probe.returncode, 'stdout': self_probe.stdout}, 'child_probe': {'pid': client.proc.pid, 'exit_code': audit.returncode, 'stdout': audit.stdout}, 'child_identity': identity, 'matching_identity_alive': own.alive(identity), 'other_sessions_signalled': 0, 'retained_owned_fixture_for_review': True})
    dump(root / 'execution.json', {'root': str(root), 'base_commit': BASE_COMMIT, 'driver_sha256': sha(Path(__file__)), 'plan_sha256': sha(PLAN), 'actual_CKS_tools': 1, 'actual_host_sessions': 0, 'real_model_calls': 0, 'formal_runs': 0, 'push_PR_tracker_operations': 0, 'production_mutations': 0, 'candidate_adopted': False, 'snapshot_scope': 'current_T22_window_not_historical_global_baseline'})
    print('T22 collected:current protected snapshots equal; owned restore/query/reap/listener close confirmed')


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--root', type=Path, required=True)
    args = parser.parse_args()
    collect(args.root.resolve())
