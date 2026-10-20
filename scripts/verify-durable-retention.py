#!/usr/bin/env python3
"""E2: actual durable MCP jobs, pinned readers, rollback and reviewed GC.

Restore only the committed E1 mock-embedding fixture into a fresh owned root.
No production dataset, paid model, ontology quality or independent acceptance.
"""
import argparse
import copy
import gzip
import hashlib
import importlib.util
import json
import os
from pathlib import Path, PurePosixPath
import subprocess
import tarfile
import time

import yaml
import dataset_contract as contract

ROOT = Path(__file__).resolve().parents[1]
E1 = ROOT / 'system/eval/knowledge-roadmap-2026-10-15/km-02-durable-host'


def require(value, message):
    if not value:
        raise ValueError(message)


def sha(data):
    return hashlib.sha256(data).hexdigest()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--out', required=True, type=Path)
    out = parser.parse_args().out.resolve()
    require(not out.exists(), 'fresh owned output required')
    out.mkdir()
    checks, commands, calls, statuses, sessions = [], [], [], [], []
    binary = ROOT / 'bin/cks'

    def write(name, value):
        (out / name).write_text(json.dumps(value, ensure_ascii=False, indent=2) + '\n')

    def checked(label, condition):
        require(condition, label)
        checks.append(label)
        write('checks.json', checks)
        print('PASS: ' + label, flush=True)

    def evidence(name):
        manifest = json.loads((E1 / 'manifest.json').read_text())
        row = next(r for r in manifest['artifacts'] if r['path'] == name)
        data = (E1 / name).read_bytes()
        require(sha(data) == row['sha256'], 'committed E1 input differs')
        return data

    fixture = evidence('fixture.tar.gz')
    write('input.json', {'E1_fixture_sha256': sha(fixture), 'E1_manifest_sha256': sha((E1 / 'manifest.json').read_bytes()),
                         'scope': 'actual_E1_CKG_CKV_mock_fixture_restored_to_new_owned_root'})
    import io
    with tarfile.open(fileobj=io.BytesIO(fixture), mode='r:gz') as archive:
        seen = set()
        for item in archive:
            path = PurePosixPath(item.name)
            require(item.name not in seen and not path.is_absolute() and '..' not in path.parts and path.parts[0] in ('project-a', 'project-b'), 'unsafe fixture path')
            seen.add(item.name)
            dest = out.joinpath(*path.parts)
            if item.isdir():
                dest.mkdir(parents=True, exist_ok=True)
            elif item.isfile():
                dest.parent.mkdir(parents=True, exist_ok=True)
                dest.write_bytes(archive.extractfile(item).read())
            elif item.issym():
                require(item.name == 'project-a/dataset/current' and item.linkname == 'v3', 'unsafe fixture link')
                dest.symlink_to(item.linkname)
            else:
                raise ValueError('unsupported fixture entry')
    checked('restore hash-bound actual E1 datasets into fresh owned roots', (out / 'project-a/dataset/v3/graph/graph.db').is_file())
    profiles = []
    for project in ('project-a', 'project-b'):
        path = out / project / 'setup.yaml'
        data = yaml.safe_load(path.read_text())
        data.update(src=str(out / project / 'source'), out=str(out / project / 'dataset'), graph_bin=str(ROOT / 'bin/ckg'), vector_bin=str(ROOT / 'bin/ckv'), cks_bin=str(binary))
        path.write_text(yaml.safe_dump(data))
        profiles.append({'id': project + '-default', 'project_id': project, 'setup_config': str(path), 'allowed_activation': ['hold_for_review', 'activate_after_gate'], 'timeout_seconds': 120})
    config = out / 'maintenance.yaml'
    config.write_text(yaml.safe_dump({'version': 1, 'name': 'cks', 'listen': {'transport': 'stdio', 'mcp_stdio': True},
        'logging': {'level': 'error', 'mode': 'prod'}, 'sanitize': {'rules_path': str(ROOT / 'system/policies/sanitization_rules.yaml')}, 'maintenance': {'profiles': profiles}}))
    rows = json.loads(gzip.decompress(evidence('runtime/observations.json.gz')))
    original = next(r for r in reversed(rows) if r['status']['project_id'] == 'project-a' and r['status']['target_version'] == 'v3' and r['status']['state'] == 'succeeded')
    original = copy.deepcopy(original)
    write('original.json', original)
    libspec = importlib.util.spec_from_file_location('retention_client', ROOT / 'scripts/eval-value-dependencies.py')
    lib = importlib.util.module_from_spec(libspec)
    libspec.loader.exec_module(lib)

    def connect(label, path):
        folder = out / label
        folder.mkdir()
        c = lib.Client(None, None, folder, command=[str(binary), 'mcp', '--config', str(path)])
        sessions.append(c)
        result = c.request('initialize', {'protocolVersion': '2024-11-05', 'capabilities': {}, 'clientInfo': {'name': 'owned-durable-retention', 'version': '1'}})
        require('error' not in result, 'MCP initialization failed')
        return c

    def tool(c, name, args):
        result = c.request('tools/call', {'name': name, 'arguments': args})
        require('error' not in result and not result['result'].get('isError'), 'public tool failed: ' + str(result))
        calls.append({'session': c.root.name, 'tool': name, 'arguments': args, 'response': result['result']})
        write('calls.json', calls)
        return result['result']['structuredContent']

    def dataset(c, name, args):
        return tool(c, 'cks.dataset.' + name, args)

    def ref(status):
        return {'schema_version': 1, **{k: status[k] for k in ('project_id', 'profile_id', 'job_id')}}

    def run(label, args, success=True):
        r = subprocess.run([str(a) for a in args], cwd=ROOT, capture_output=True, input=b'', timeout=120)
        (out / (label + '.stdout.txt')).write_bytes(r.stdout)
        (out / (label + '.stderr.txt')).write_bytes(r.stderr)
        commands.append({'label': label, 'command': [str(a) for a in args], 'exit_code': r.returncode, 'expected_success': success})
        write('commands.json', commands)
        require((r.returncode == 0) == success, 'unexpected CLI result: ' + label)
        return r.stdout

    def hashes(folder):
        return {p.relative_to(folder).as_posix(): sha(p.read_bytes()) for p in sorted(folder.rglob('*')) if p.is_file() and not p.is_symlink()}

    root = out / 'project-a/dataset'
    frozen_a, frozen_b = hashes(root / 'v1'), hashes(out / 'project-b/dataset/v1')
    history = root / '.maintenance-publications' / (original['status']['job_id'] + '.json')
    frozen_history = sha(history.read_bytes())
    write('preserved-before.json', {'A_v1': frozen_a, 'B_v1': frozen_b, 'A_v3_history': frozen_history})

    def observe(status, request, profile):
        contract.validate_status(status, request, profile)
        statuses.append({'status': status, 'request': request, 'profile': profile})
        write('observations.json', statuses)

    def build(c, version, marker):
        (out / 'project-a/source/consensus.go').write_text('package common\n// SharedConsensus returns an owned marker.\nfunc SharedConsensus() int { return ' + str(marker) + ' }\n')
        inspection = dataset(c, 'inspect_profile', {'schema_version': 1, 'project_id': 'project-a', 'profile_id': 'project-a-default'})
        require(inspection['capabilities']['job_persistence'] == 'durable_journal', 'missing durable capability')
        p = inspection['profile']
        request = {'schema_version': 1, 'project_id': 'project-a', 'profile_id': 'project-a-default',
                   **{k: p[k] for k in ('profile_digest', 'source_pin', 'input_digest', 'embedding_digest')},
                   'base': p['observed_base'], 'new_version': version, 'activation': 'activate_after_gate', 'idempotency_key': 'owned-retention-' + version, 'deadline_seconds': 120}
        value = dataset(c, 'build', request)
        observe(value, request, p)
        deadline = time.monotonic() + 120
        while not value['terminal']:
            require(time.monotonic() < deadline, 'build timeout')
            time.sleep(.05)
            value = dataset(c, 'status', ref(value))
            observe(value, request, p)
        require(value['state'] == 'succeeded', 'new actual candidate not published')
        return value

    def reader_config(version):
        folder, path = root / version, out / ('reader-' + version + '.yaml')
        model = json.loads((folder / 'vector/manifest.json').read_text())['embedding_model']
        run('gen-reader-' + version, [binary, 'mcp', 'gen-config', '--dataset-dir', folder, '--source-root', out / 'project-a/source', '--embed-model', model,
            '--sanitize-rules', ROOT / 'system/policies/sanitization_rules.yaml', '--out', path])
        data = yaml.safe_load(path.read_text())
        data['backends']['ckv']['provider'] = 'mock'
        data['listen'].update(transport='stdio', mcp_stdio=True)
        data['logging']['level'] = 'error'
        path.write_text(yaml.safe_dump(data))
        return path

    def read(c, label, version, marker):
        pack = tool(c, 'cks.context.get_for_task_v2', {'prompt': 'SharedConsensus'})
        write(label + '.json', pack)
        require(pack['coordinates'] == original['status']['candidate']['coordinates'] if version == 'v3' else pack['coordinates']['project_id'] == 'project-a', 'reader identity differs')
        require(pack['citations'] and pack['bodies'] and any(('return ' + str(marker)) in json.dumps(body) for body in pack['bodies']), 'retained implementation missing')
        require(all(c['project_id'] == 'project-a' and c['dataset_id'] == pack['coordinates']['dataset_id'] for c in pack['citations']), 'citation mixed projects')
        return pack

    def plan(label):
        path = out / (label + '.json')
        run(label, [binary, 'gc', '--out', root, '--keep-recent', '2', '--min-age', '0s', '--protect-version', 'v1', '--plan-file', path])
        return json.loads(path.read_text()), path

    def version(plan, name):
        return next(v for v in plan['versions'] if v['version'] == name)

    try:
        host = connect('maintenance-session', config)
        status = dataset(host, 'status', ref(original['status']))
        observe(status, original['request'], original['profile'])
        checked('restored durable terminal and original replay match E1 snapshot', status == original['status'] and dataset(host, 'build', original['request']) == status)
        read_config = reader_config('v3')
        pinned = connect('reader-v3', read_config)
        before = read(pinned, 'read-v3-before', 'v3', 404)
        checked('actual MCP reader pins retained v3 before rollback', before['coordinates']['dataset_id'] == status['candidate']['coordinates']['dataset_id'])
        run('rollback-v1', [binary, 'rollback', 'v1', '--config', out / 'project-a/setup.yaml'])
        checked('actual CLI rollback switches current to v1', os.readlink(root / 'current') == 'v1')
        after = read(pinned, 'read-v3-after-rollback', 'v3', 404)
        checked('pinned reader retains v3 coordinates and body through rollback', after['coordinates'] == before['coordinates'])
        checked('rollback does not rewrite historical succeeded publication receipt', dataset(host, 'status', ref(status)) == status and sha(history.read_bytes()) == frozen_history)
        four = build(host, 'v4', 909)
        checked('new v4 publication uses rolled back v1 as checked base', four['publication']['base']['version'] == 'v1' and os.readlink(root / 'current') == 'v4')
        five = build(host, 'v5', 1001)
        checked('new v5 publication advances current and preserves pinned v3', five['publication']['base']['version'] == 'v4' and os.readlink(root / 'current') == 'v5')
        active, active_path = plan('gc-active-reader')
        checked('GC plan protects actual pinned v3 and held v2', not version(active, 'v3')['delete'] and 'pinned_reader' in version(active, 'v3')['protected_reasons'] and not version(active, 'v2')['delete'] and 'review_hold' in version(active, 'v2')['protected_reasons'])
        run('gc-apply-active', [binary, 'gc', '--out', root, '--apply', '--plan-file', active_path])
        checked('reviewed GC apply leaves active reader dataset intact', (root / 'v3/graph/graph.db').is_file())
        read(pinned, 'read-v3-after-gc-protection', 'v3', 404)
        checked('actual pinned reader remains serviceable after protected GC', True)
        pinned.close()
        released, released_path = plan('gc-released-reader')
        checked('released v3 becomes GC eligible without manual reference edits', version(released, 'v3')['delete'] and 'pinned_reader' not in version(released, 'v3')['protected_reasons'])
        new_reader = connect('reader-v3-repin', read_config)
        read(new_reader, 'read-v3-repin', 'v3', 404)
        run('gc-stale-reader-plan', [binary, 'gc', '--out', root, '--apply', '--plan-file', released_path], success=False)
        checked('GC rejects reviewed plan made stale by a new live reader', (root / 'v3/graph/graph.db').is_file())
        new_reader.close()
        collectible, collectible_path = plan('gc-collectible')
        require(version(collectible, 'v3')['delete'], 'expected collection target missing')
        applied = json.loads(run('gc-apply-released', [binary, 'gc', '--out', root, '--apply', '--plan-file', collectible_path]))
        checked('reviewed GC actually removes released v3 and reclaims bytes', not (root / 'v3').exists() and applied['reclaimed_bytes'] > 0)
        checked('GC preserves current rollback target held candidate and explicit v1', os.readlink(root / 'current') == 'v5' and all((root / v / 'graph/graph.db').is_file() for v in ('v1', 'v2', 'v4', 'v5')))
        checked('GC retains immutable historical publication and exact job replay', sha(history.read_bytes()) == frozen_history and dataset(host, 'status', ref(status)) == status and dataset(host, 'build', original['request']) == status)
        host.close()
        restarted = connect('restarted-session', config)
        checked('host restart after version GC preserves historical terminal result', dataset(restarted, 'status', ref(status)) == status and dataset(restarted, 'build', original['request']) == status)
        deleted_config = out / 'deleted-reader.yaml'
        run('gen-deleted-reader-v3', [binary, 'mcp', 'gen-config', '--dataset-dir', root / 'v3', '--source-root', out / 'project-a/source', '--embed-model', 'mock', '--out', deleted_config])
        deleted = yaml.safe_load(deleted_config.read_text())
        deleted['backends']['ckv']['provider'] = 'mock'
        deleted['listen'].update(transport='stdio', mcp_stdio=True)
        deleted_config.write_text(yaml.safe_dump(deleted))
        run('refuse-reader-deleted-v3', [binary, 'mcp', '--config', deleted_config], success=False)
        checked('deleted v3 cannot be silently rebound as another dataset', not (root / 'v3').exists())
        latest = connect('reader-v5', reader_config('v5'))
        pack = read(latest, 'read-v5-after-gc', 'v5', 1001)
        checked('current v5 actual query returns new retained implementation after GC', pack['coordinates'] == five['candidate']['coordinates'])
        latest.close()
        restarted.close()
        preserved = {'A_v1': hashes(root / 'v1'), 'B_v1': hashes(out / 'project-b/dataset/v1'), 'A_v3_history': sha(history.read_bytes())}
        write('preserved-after.json', preserved)
        checked('old A v1 B held DB and publication history bytes stay unchanged', preserved == {'A_v1': frozen_a, 'B_v1': frozen_b, 'A_v3_history': frozen_history})
        checked('all five E2 stdio sessions end with clean EOF', len(sessions) == 5 and all(c.proc.returncode == 0 for c in sessions))
    finally:
        for c in sessions:
            if c.proc.poll() is None:
                c.close()
    summary = {'schema_version': 1, 'scope': 'KM02_3_E2_actual_reader_rollback_GC_mock_embedding', 'checks': len(checks), 'tool_calls': len(calls),
        'status_observations': len(statuses), 'sessions': len(sessions), 'EOF_exit_codes': [c.proc.returncode for c in sessions], 'paid_model_calls': 0,
        'independent_acceptance': False, 'quality': 'not_evaluated', 'binary_sha256': sha(binary.read_bytes())}
    write('summary.json', summary)
    print(json.dumps(summary))


if __name__ == '__main__':
    main()
