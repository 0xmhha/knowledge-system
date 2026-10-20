#!/usr/bin/env python3
"""KM-02.4: public failure matrix against existing current and pinned readers.

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

    def tool(c, name, args, error=None):
        result = c.request('tools/call', {'name': name, 'arguments': args})
        require('error' not in result and 'result' in result, 'public protocol failed')
        calls.append({'session': c.root.name, 'tool': name, 'arguments': args, 'response': result['result']})
        write('calls.json', calls)
        content = result['result']['structuredContent']
        require(bool(result['result'].get('isError')) == bool(error) and (not error or content.get('code') == error), 'unexpected public result: ' + str(result))
        return content

    def dataset(c, name, args, error=None):
        return tool(c, 'cks.dataset.' + name, args, error)

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


    cfgpath = out / 'project-a/setup.yaml'
    baseline = yaml.safe_load(cfgpath.read_text())
    frozen_current = hashes(root / 'v3')
    ready, release, late = out / 'step-ready', out / 'step-release', out / 'step-late'
    wrapper = out / 'owned-engine'
    current_request = {}

    def options(**changes):
        data = dict(baseline, **changes)
        cfgpath.write_text(yaml.safe_dump(data))

    def slow_engine():
        for path in (ready, release, late):
            if path.exists():
                path.unlink()
        wrapper.write_text('#!/bin/sh\nfor arg in "$@"; do\n if [ "$arg" = build ]; then\n  printf ready > "' + str(ready) + '"\n  (sleep 1; printf late > "' + str(late) + '") &\n  while [ ! -f "' + str(release) + '" ]; do sleep .05; done\n  break\n fi\ndone\nexec "' + str(ROOT / 'bin/ckg') + '" "$@"\n')
        wrapper.chmod(0o700)
        options(graph_bin=str(wrapper))

    def inspection(c):
        return dataset(c, 'inspect_profile', {'schema_version': 1, 'project_id': 'project-a', 'profile_id': 'project-a-default'})['profile']

    def request(p, version, seconds=120):
        return {'schema_version': 1, 'project_id': 'project-a', 'profile_id': 'project-a-default',
                **{k: p[k] for k in ('profile_digest', 'source_pin', 'input_digest', 'embedding_digest')},
                'base': p['observed_base'], 'new_version': version, 'activation': 'activate_after_gate', 'idempotency_key': 'owned-failure-' + version, 'deadline_seconds': seconds}

    def start(c, version, seconds=120):
        p = inspection(c)
        req = request(p, version, seconds)
        value = dataset(c, 'build', req)
        observe(value, req, p)
        current_request[value['job_id']] = (req, p)
        return value

    def wait(c, value, phase=None):
        deadline = time.monotonic() + 120
        while not value['terminal'] and (phase is None or value['phase'] != phase):
            require(time.monotonic() < deadline, 'job did not settle')
            time.sleep(.05)
            value = dataset(c, 'status', ref(value))
            observe(value, *current_request[value['job_id']])
        return value

    def await_step():
        deadline = time.monotonic() + 10
        while not ready.exists():
            require(time.monotonic() < deadline, 'owned graph not ready')
            time.sleep(.01)

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

    def read(c, label, marker=404):
        pack = tool(c, 'cks.context.get_for_task_v2', {'prompt': 'SharedConsensus'})
        write('read-' + label + '.json', pack)
        require(pack['citations'] and pack['bodies'] and any(('return ' + str(marker)) in json.dumps(body) for body in pack['bodies']), 'retained implementation lost')
        require(all(row['project_id'] == 'project-a' and row['dataset_id'] == pack['coordinates']['dataset_id'] for row in pack['citations']), 'cross-project citation')
        return pack

    def persistent_current():
        current = hashes(root / 'v3')
        allowed = {'vector/vector.db-wal', 'vector/vector.db-shm'}
        extras = set(current) - set(frozen_current)
        require(extras <= allowed, 'unexpected old dataset artifact')
        if 'vector/vector.db-wal' in extras:
            require((root / 'v3/vector/vector.db-wal').stat().st_size == 0, 'reader appended persistent WAL writes')
        return {name: digest for name, digest in current.items() if name not in extras}

    def preserved(label):
        pack = read(pinned, label)
        write('integrity-' + label + '.json', {'A_v3_before': frozen_current, 'A_v3_during': hashes(root / 'v3'), 'A_v3_persistent': persistent_current(), 'A_v3_reader_sidecars': {p.name: {'bytes': p.stat().st_size, 'sha256': sha(p.read_bytes())} for p in (root / 'v3/vector').glob('vector.db-*')}, 'A_v1_before': frozen_a, 'A_v1_during': hashes(root / 'v1'), 'B_v1_before': frozen_b, 'B_v1_during': hashes(out / 'project-b/dataset/v1'), 'history_before': frozen_history, 'history_during': sha(history.read_bytes())})
        checked('preserve current reader and other project after ' + label, os.readlink(root / 'current') == 'v3' and
                pack['coordinates'] == original['status']['candidate']['coordinates'] and persistent_current() == frozen_current and
                hashes(root / 'v1') == frozen_a and hashes(out / 'project-b/dataset/v1') == frozen_b and sha(history.read_bytes()) == frozen_history)

    def killed(c):
        c.proc.kill()
        c.proc.wait(timeout=10)
        c.proc.stdin.close()
        c.thread.join(timeout=2)
        c.stderr.close()
        c.stdin.close()
        c.stdout.close()
        require(c.proc.returncode == -9, 'expected actual host kill')

    try:
        host = connect('maintenance-session', config)
        pinned = connect('reader-v3', reader_config('v3'))
        p = inspection(host)
        base = request(p, 'not-admitted')
        for label, bad, error in [
            ('foreign-project', dict(base, project_id='project-b'), 'project_mismatch'),
            ('foreign-profile', dict(base, profile_id='project-b-default'), 'project_mismatch'),
            ('invalid-version', dict(base, new_version='../escape'), 'invalid_request'),
            ('existing-version', dict(base, new_version='v3'), 'version_exists'),
            ('caller-path', dict(base, out='/private/foreign'), 'invalid_request'),
            ('caller-command', dict(base, command=['/bin/sh']), 'invalid_request'),
        ]:
            dataset(host, 'build', bad, error)
            checked('public refusal ' + label, not (root / 'not-admitted').exists())
        foreign = copy.deepcopy(base)
        foreign['base']['coordinates']['project_id'] = 'project-b'
        dataset(host, 'build', foreign, 'project_mismatch')
        checked('public refusal foreign-base-coordinates', True)
        reference = ref(original['status'])
        reference['project_id'] = 'project-b'
        dataset(host, 'status', reference, 'job_unknown')
        dataset(host, 'cancel', reference, 'job_unknown')
        checked('public foreign job cannot be read or canceled', True)
        preserved('admission-refusals')

        for engine in ('graph', 'vector'):
            actual = ROOT / ('bin/ckg' if engine == 'graph' else 'bin/ckv')
            marker = out / (engine + '-failed')
            wrapper.write_text('#!/bin/sh\nfor arg in "$@"; do\n if [ "$arg" = build ]; then printf failed > "' + str(marker) + '"; exit 23; fi\ndone\nexec "' + str(actual) + '" "$@"\n')
            wrapper.chmod(0o700)
            options(**{engine + '_bin': str(wrapper)})
            failed = wait(host, start(host, 'failed-' + engine))
            candidate = root / failed['target_version']
            checked('actual ' + engine + ' engine failure cannot publish partial candidate', failed['state'] == 'failed' and not failed['build_complete'] and not failed['activated'] and marker.exists() and
                    (engine != 'vector' or (candidate / 'graph/graph.db').is_file()))
            checked(engine + ' failure removes owned scratch after writer exit', not (candidate / 'build-staging.json').exists() and not list(candidate.glob('.build-source-*')))
            preserved(engine + '-failure')

        corpus = out / 'foreign-semantic'
        corpus.mkdir()
        (corpus / 'claim.md').write_text('# Unmatched input\nForeign commit evidence.\n')
        write_manifest = {'schema_version': 1, 'snapshot': {'commit': 'f'*40}, 'source_root': str(out / 'project-b/source'), 'projection_sha256': 'a'*64,
                          'files': [{'path': 'claim.md', 'sha256': sha((corpus / 'claim.md').read_bytes())}]}
        (corpus / 'manifest.json').write_text(json.dumps(write_manifest))
        options(semantic_corpus=str(corpus))
        semantic = wait(host, start(host, 'failed-semantic'))
        checked('mismatched semantic corpus fails after actual graph before activation', semantic['state'] == 'failed' and not semantic['activated'] and (root / 'failed-semantic/graph/graph.db').is_file())
        preserved('semantic-mismatch')

        slow_engine()
        drift = wait(host, start(host, 'failed-source-drift'), 'build_graph')
        await_step()
        source_file = out / 'project-a/source/consensus.go'
        before_source = source_file.read_bytes()
        source_file.write_text('package common\nfunc SharedConsensus() int { return 2002 }\n')
        release.write_text('continue')
        drift = wait(host, drift)
        checked('source drift during real build blocks publication', drift['state'] == 'failed' and not drift['activated'])
        preserved('source-drift')
        source_file.write_bytes(before_source)

        slow_engine()
        drift = wait(host, start(host, 'failed-profile-drift'), 'build_graph')
        await_step()
        data = yaml.safe_load(cfgpath.read_text())
        data['build_timeout'] = '119s'
        cfgpath.write_text(yaml.safe_dump(data))
        release.write_text('continue')
        drift = wait(host, drift)
        checked('profile drift during real build blocks publication', drift['state'] == 'failed' and not drift['activated'])
        preserved('profile-drift')

        slow_engine()
        timed = wait(host, start(host, 'failed-timeout', 10))
        checked('actual deadline stops graph writer with deadline_exceeded', timed['state'] == 'failed' and timed['error']['code'] == 'deadline_exceeded' and not timed['activated'])
        # This wrapper deliberately has a one-second child marker. At a ten
        # second deadline it may have written before cancellation; wait/lease
        # and scratch proof establish quiescence, not absence of earlier writes.
        candidate = root / timed['target_version']
        checked('deadline terminal has no remaining owned scratch', not (candidate / 'build-staging.json').exists() and not list(candidate.glob('.build-source-*')))
        preserved('timeout')

        slow_engine()
        canceled = wait(host, start(host, 'failed-cancel'), 'build_graph')
        await_step()
        ack = dataset(host, 'cancel', ref(canceled))
        observe(ack['status'], *current_request[canceled['job_id']])
        checked('public cancel on active dataset acknowledges intent before terminal', ack['decision'] == 'accepted' and not ack['status']['terminal'])
        canceled = wait(host, ack['status'])
        time.sleep(1.2)
        checked('active dataset cancellation terminates child without late write', canceled['state'] == 'canceled' and not late.exists())
        preserved('cancel')

        slow_engine()
        stopped = wait(host, start(host, 'failed-eof'), 'build_graph')
        await_step()
        host.close()
        host = connect('eof-recovery', config)
        stopped = wait(host, dataset(host, 'status', ref(stopped)))
        observe(stopped, *current_request[stopped['job_id']])
        time.sleep(1.2)
        checked('EOF on active current preserves canceled state after restart', stopped['state'] == 'canceled' and not late.exists())
        preserved('EOF')

        slow_engine()
        crashed = wait(host, start(host, 'failed-host-kill'), 'build_graph')
        await_step()
        killed(host)
        host = connect('kill-recovery', config)
        crashed = wait(host, dataset(host, 'status', ref(crashed)))
        observe(crashed, *current_request[crashed['job_id']])
        time.sleep(1.2)
        checked('SIGKILL on active current recovers interrupted with no late writer', crashed['state'] == 'interrupted' and not late.exists())
        preserved('SIGKILL')

        options()
        checked('all failed work preserves original public terminal and replay', dataset(host, 'status', ref(original['status'])) == original['status'] and dataset(host, 'build', original['request']) == original['status'])
        source_file.write_text('package common\n// SharedConsensus is the recovery marker.\nfunc SharedConsensus() int { return 3003 }\n')
        successful = wait(host, start(host, 'v4'))
        checked('fresh valid build after failure matrix actually publishes v4', successful['state'] == 'succeeded' and successful['publication']['base']['version'] == 'v3' and os.readlink(root / 'current') == 'v4')
        read(pinned, 'old-reader-after-new-publish')
        latest = connect('reader-v4', reader_config('v4'))
        pack = read(latest, 'latest-after-failures', 3003)
        checked('old pinned reader and new v4 reader remain correctly separated', pack['coordinates'] == successful['candidate']['coordinates'] and persistent_current() == frozen_current)
        latest.close()
        pinned.close()
        host.close()
        checked('one expected host kill and four clean EOF sessions are accounted', len(sessions) == 5 and [c.proc.returncode for c in sessions] == [0,0,-9,0,0])
        write('preserved-after.json', {'A_v1': hashes(root / 'v1'), 'B_v1': hashes(out / 'project-b/dataset/v1'), 'A_v3': hashes(root / 'v3'), 'A_v3_history': sha(history.read_bytes())})
    finally:
        for c in sessions:
            if c.proc.poll() is None:
                c.close()
    summary = {'schema_version': 1, 'scope': 'KM02_4_actual_public_failure_preservation_mock_embedding', 'checks': len(checks), 'tool_calls': len(calls),
        'status_observations': len(statuses), 'sessions': len(sessions), 'EOF_exit_codes': [c.proc.returncode for c in sessions], 'paid_model_calls': 0,
        'independent_acceptance': False, 'quality': 'not_evaluated', 'binary_sha256': sha(binary.read_bytes())}
    write('summary.json', summary)
    print(json.dumps(summary))


if __name__ == '__main__':
    main()
