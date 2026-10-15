#!/usr/bin/env python3
"""Exercise opt-in scoped jobs through real stdio MCP and actual owned DBs.

Mock embedding is deliberate: this is lifecycle/identity proof, not retrieval
quality, durable recovery or independent acceptance.
"""
import argparse
import copy
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import time

import yaml
import dataset_contract as contract

ROOT = Path(__file__).resolve().parents[1]


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
    binary = ROOT / 'bin/cks'
    checks, calls, commands, sessions, statuses = [], [], [], [], []

    def write(name, value):
        path = out / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(json.dumps(value, ensure_ascii=False, indent=2) + '\n')

    def checked(label, condition):
        require(condition, label)
        checks.append(label)
        write('checks.json', checks)
        print('PASS: ' + label, flush=True)

    def run(label, args):
        r = subprocess.run([str(a) for a in args], cwd=ROOT, capture_output=True, timeout=120)
        (out / (label + '.stdout.txt')).write_bytes(r.stdout)
        (out / (label + '.stderr.txt')).write_bytes(r.stderr)
        commands.append({'label': label, 'command': [str(a) for a in args], 'exit_code': r.returncode})
        write('commands.json', commands)
        require(r.returncode == 0, 'command failed: ' + label)

    libspec = importlib.util.spec_from_file_location('maintenance_probe', ROOT / 'scripts/eval-value-dependencies.py')
    lib = importlib.util.module_from_spec(libspec)
    libspec.loader.exec_module(lib)

    def connect(label, config):
        folder = out / label
        folder.mkdir()
        c = lib.Client(None, None, folder, command=[str(binary), 'mcp', '--config', str(config)], namespace='cks.dataset.')
        reply = c.request('initialize', {'protocolVersion': '2024-11-05', 'capabilities': {}, 'clientInfo': {'name': 'owned-maintenance-verifier', 'version': '1'}})
        require('error' not in reply, 'initialization failed')
        sessions.append(c)
        return c

    def tool(c, operation, args, error=None):
        name = 'cks.dataset.' + operation
        reply = c.request('tools/call', {'name': name, 'arguments': args})
        require('error' not in reply and 'result' in reply, 'tool protocol failed')
        result = reply['result']
        calls.append({'session': c.root.name, 'tool': name, 'arguments': args, 'response': result})
        write('calls.json', calls)
        if error:
            require(result.get('isError') and result.get('structuredContent', {}).get('code') == error, 'wrong rejection: ' + error + ': ' + str(result))
        else:
            require(not result.get('isError'), 'unexpected failure: ' + str(result))
        return result.get('structuredContent', result)

    def source(project, marker):
        src = out / project / 'source'
        src.mkdir(parents=True, exist_ok=True)
        (src / 'go.mod').write_text('module example.invalid/common\n\ngo 1.26.9\n')
        (src / 'consensus.go').write_text('package common\n\n// SharedConsensus returns an owned test marker.\nfunc SharedConsensus() int { return ' + str(marker) + ' }\n')
        return src

    profiles = []
    for project, marker in [('project-a', 101), ('project-b', 202)]:
        src = source(project, marker)
        cfg = out / project / 'setup.yaml'
        cfg.write_text(yaml.safe_dump({'project_id': project, 'source_mode': 'snapshot-only', 'src': str(src),
            'out': str(out / project / 'dataset'), 'graph_bin': str(ROOT / 'bin/ckg'), 'vector_bin': str(ROOT / 'bin/ckv'),
            'cks_bin': str(binary), 'embedder': 'mock', 'model_name': 'mock', 'build_timeout': '2m'}))
        profiles.append({'id': project + '-default', 'project_id': project, 'setup_config': str(cfg),
                         'allowed_activation': ['hold_for_review', 'activate_after_gate'], 'timeout_seconds': 120})
    top = {'version': 1, 'name': 'cks', 'listen': {'transport': 'stdio', 'mcp_stdio': True},
           'logging': {'level': 'error', 'mode': 'prod'}, 'sanitize': {'rules_path': str(ROOT / 'system/policies/sanitization_rules.yaml')}}
    default_config = out / 'default.yaml'
    default_config.write_text(yaml.safe_dump(top))
    top['maintenance'] = {'profiles': profiles}
    config = out / 'maintenance.yaml'
    config.write_text(yaml.safe_dump(top))
    jobs = {}

    def inspect(c, project):
        data = tool(c, 'inspect_profile', {'schema_version': 1, 'project_id': project, 'profile_id': project + '-default'})
        contract.shape('profileContext', data['profile'])
        require(data['capabilities']['job_persistence'] == 'process_lifetime' and data['capabilities']['serving_adoption'] == 'not_observed', 'unsupported durability/adoption claim')
        return data['profile']

    def request(profile, version, mode='activate_after_gate'):
        return {'schema_version': 1, 'project_id': profile['project_id'], 'profile_id': profile['profile_id'],
                **{k: profile[k] for k in ('profile_digest', 'source_pin', 'input_digest', 'embedding_digest')},
                'base': profile['observed_base'], 'new_version': version, 'activation': mode,
                'idempotency_key': 'owned-request-key-' + profile['project_id'] + '-' + version, 'deadline_seconds': 120}

    def observe(value, req, profile):
        contract.validate_status(value, req, profile)
        statuses.append({'request': req, 'profile': profile, 'status': value})
        write('observations.json', statuses)

    def start(c, req, profile):
        value = tool(c, 'build', req)
        observe(value, req, profile)
        jobs[value['job_id']] = (req, profile)
        return value

    def ref(value):
        return {'schema_version': 1, **{k: value[k] for k in ('project_id', 'profile_id', 'job_id')}}

    def wait(c, value, phase=None):
        deadline = time.monotonic() + 120
        while not value['terminal'] and (phase is None or value['phase'] != phase):
            require(time.monotonic() < deadline, 'job did not finish')
            time.sleep(.05)
            value = tool(c, 'status', ref(value))
            req, profile = jobs[value['job_id']]
            observe(value, req, profile)
        return value

    def retained(project, version, marker):
        folder = out / project / 'dataset' / version
        identity = json.loads((folder / 'dataset-identity.json').read_text())
        manifest = json.loads((folder / 'sources/manifest.json').read_text())
        matches = [f for f in manifest['files'] if f['path'] == 'consensus.go']
        require(len(matches) == 1 and identity['source']['project_id'] == project, 'wrong retained identity')
        content = (folder / 'sources/blobs' / matches[0]['sha256']).read_bytes()
        require(sha(content) == matches[0]['sha256'] and ('return ' + str(marker)).encode() in content, 'retained source differs')
        return {str(p.relative_to(out)): sha(p.read_bytes()) for p in folder.rglob('*') if p.is_file() and not p.is_symlink()}

    try:
        c0 = connect('default-session', default_config)
        default_tools = c0.request('tools/list')['result']['tools']
        checked('default surface remains 33 tools without dataset jobs', len(default_tools) == 33 and not any(t['name'].startswith('cks.dataset.') for t in default_tools))
        c0.close()
        c = connect('maintenance-session', config)
        tools = c.request('tools/list')['result']['tools']
        write('tools.json', tools)
        names = {t['name']: t for t in tools}
        expected = {'cks.dataset.' + n for n in ('inspect_profile', 'build', 'status', 'cancel')}
        checked('four opt-in public tools expose schemas and read/write hints', len(tools) == 37 and expected <= names.keys() and
                all(names[n].get('outputSchema') and names[n]['annotations']['readOnlyHint'] == (n.endswith('inspect_profile') or n.endswith('status')) for n in expected))
        a, b = inspect(c, 'project-a'), inspect(c, 'project-b')
        checked('inspection does not create either dataset and observes absent base', a['observed_base'] == b['observed_base'] == {'state': 'absent'} and not any((out / p / 'dataset').exists() for p in ('project-a', 'project-b')))
        req = request(a, 'v1')
        for field, value, code in [('profile_digest', '0'*64, 'profile_stale'), ('input_digest', '0'*64, 'input_stale'), ('embedding_digest', '0'*64, 'embedding_stale')]:
            bad = copy.deepcopy(req)
            bad[field] = value
            tool(c, 'build', bad, code)
            checked('refuse stale ' + field, True)
        bad = copy.deepcopy(req)
        bad['source_pin']['snapshot_id'] = '0'*64
        tool(c, 'build', bad, 'source_stale')
        checked('refuse stale source snapshot', True)
        tool(c, 'inspect_profile', {'schema_version': 1, 'project_id': 'project-b', 'profile_id': 'project-a-default'}, 'project_mismatch')
        tool(c, 'build', dict(req, path='/private/other'), 'invalid_request')
        checked('refuse foreign profile and caller path injection', True)
        initial = start(c, req, a)
        published = wait(c, initial)
        checked('first actual graph/vector build publishes v1 with receipt and passed gates', published['state'] == 'succeeded' and published['activated'] and os.readlink(out / 'project-a/dataset/current') == 'v1')
        frozen = retained('project-a', 'v1', 101)
        write('retained-a1-before.json', frozen)
        checked('candidate identity and retained source bytes match project A', published['candidate']['coordinates']['project_id'] == 'project-a')
        replay = tool(c, 'build', req)
        checked('same key replays committed result before stale base/version checks', replay == published)
        tool(c, 'build', dict(req, new_version='different'), 'idempotency_conflict')
        checked('same key different request refuses conflict', True)
        tool(c, 'status', dict(ref(published), project_id='project-b'), 'job_unknown')
        tool(c, 'cancel', dict(ref(published), profile_id='project-b-default'), 'job_unknown')
        checked('foreign project/profile cannot observe or cancel job', True)
        cancel = tool(c, 'cancel', ref(published))
        checked('cancel after publication is too_late without rollback', cancel['decision'] == 'too_late' and cancel['status'] == published and os.readlink(out / 'project-a/dataset/current') == 'v1')
        held_b = wait(c, start(c, request(b, 'v1', 'hold_for_review'), b))
        checked('project B held candidate never creates current', held_b['state'] == 'held_for_review' and held_b['build_complete'] and not held_b['activated'] and not os.path.lexists(out / 'project-b/dataset/current'))
        retained('project-b', 'v1', 202)
        source('project-a', 303)
        newer = inspect(c, 'project-a')
        held_a = wait(c, start(c, request(newer, 'v2', 'hold_for_review'), newer))
        checked('A v2 held candidate preserves A v1 and B isolation', held_a['state'] == 'held_for_review' and os.readlink(out / 'project-a/dataset/current') == 'v1' and retained('project-a', 'v1', 101) == frozen)
        retained('project-a', 'v2', 303)
        source('project-a', 404)
        newer = inspect(c, 'project-a')
        stale = request(newer, 'v3')
        stale['base'] = {'state': 'absent'}
        tool(c, 'build', stale, 'base_stale')
        checked('reject stale observed current before building', not (out / 'project-a/dataset/v3').exists())
        final = wait(c, start(c, request(newer, 'v3'), newer))
        checked('second activation publishes exact v3 and preserves old immutable DB', final['state'] == 'succeeded' and final['publication']['base']['version'] == 'v1' and os.readlink(out / 'project-a/dataset/current') == 'v3' and retained('project-a', 'v1', 101) == frozen)
        retained('project-a', 'v3', 404)
        # Trusted operator wrapper delays only graph build, providing a stable
        # window for a real cancellation of the existing subprocess runner.
        wrapper = out / 'slow-graph'
        wrapper.write_text('#!/bin/sh\nif [ "$1" = "build" ]; then sleep 30; fi\nexec "' + str(ROOT / 'bin/ckg') + '" "$@"\n')
        wrapper.chmod(0o700)
        bcfg = out / 'project-b/setup.yaml'
        data = yaml.safe_load(bcfg.read_text())
        data['graph_bin'] = str(wrapper)
        bcfg.write_text(yaml.safe_dump(data))
        source('project-b', 505)
        b = inspect(c, 'project-b')
        slow = wait(c, start(c, request(b, 'v2'), b), 'build_graph')
        checked('slow graph job remains running before cancel', not slow['terminal'] and slow['phase'] == 'build_graph')
        busy = request(b, 'v3')
        tool(c, 'build', busy, 'writer_busy')
        checked('concurrent job on same dataset refused', True)
        requested = tool(c, 'cancel', ref(slow))
        checked('cancel acknowledgment is not false terminal success', requested['decision'] == 'accepted' and requested['status']['state'] == 'cancel_requested' and not requested['status']['terminal'])
        canceled = wait(c, requested['status'])
        checked('cancellation terminates child build and leaves current absent', canceled['state'] == 'canceled' and not canceled['activated'] and not os.path.lexists(out / 'project-b/dataset/current'))
        checked('terminal status remains immutable', tool(c, 'status', ref(canceled)) == canceled)
        c.close()
        restarted = connect('restarted-session', config)
        tool(restarted, 'status', ref(final), 'job_unknown')
        checked('process lifetime is explicit; reconnect does not invent durable jobs', inspect(restarted, 'project-a')['observed_base']['version'] == 'v3')
        restarted.close()
        for version, marker in [('v1', 101), ('v3', 404)]:
            folder = out / 'project-a/dataset' / version
            read_config = out / ('read-' + version + '.yaml')
            model = json.loads((folder / 'vector/manifest.json').read_text())['embedding_model']
            run('read-config-' + version, [binary, 'mcp', 'gen-config', '--dataset-dir', folder,
                '--source-root', out / 'project-a/source', '--embed-model', model,
                '--sanitize-rules', ROOT / 'system/policies/sanitization_rules.yaml', '--out', read_config])
            read = yaml.safe_load(read_config.read_text())
            read['backends']['ckv']['provider'] = 'mock'
            read['listen'].update(transport='stdio', mcp_stdio=True)
            read['logging']['level'] = 'error'
            read_config.write_text(yaml.safe_dump(read))
            reader = connect('reader-' + version, read_config)
            reply = reader.request('tools/call', {'name': 'cks.context.get_for_task_v2', 'arguments': {'prompt': 'SharedConsensus'}})
            require('error' not in reply and not reply['result'].get('isError'), 'actual DB query failed')
            pack = reply['result']['structuredContent']
            write('read-' + version + '.json', pack)
            identity = json.loads((folder / 'dataset-identity.json').read_text())
            require(pack['coordinates']['project_id'] == 'project-a' and pack['coordinates']['dataset_id'] == identity['dataset_id'], 'reader mixed dataset')
            require(pack['citations'] and pack['bodies'] and any(('return ' + str(marker)) in json.dumps(body) for body in pack['bodies']), 'reader lost retained implementation')
            require(all(c['project_id'] == 'project-a' and c['snapshot_id'] == identity['source']['snapshot_id'] for c in pack['citations']), 'reader mixed citation')
            checked('actual v2 query reads retained A ' + version + ' marker ' + str(marker), True)
            reader.close()
        checked('all five stdio sessions exit cleanly on EOF', all(s.proc.returncode == 0 for s in sessions))
        write('retained-a1-after.json', retained('project-a', 'v1', 101))
        checked('original retained files unchanged across builds cancellation and reconnect', retained('project-a', 'v1', 101) == frozen)
    finally:
        for client in sessions:
            if client.proc.poll() is None:
                client.close()
    summary = {'schema_version': 1, 'scope': 'KM02_2_actual_scoped_public_jobs_mock_embedding', 'checks': len(checks),
               'tool_calls': len(calls), 'status_observations': len(statuses), 'sessions': len(sessions),
               'EOF_exit_codes': [s.proc.returncode for s in sessions], 'binary_sha256': sha(binary.read_bytes()),
               'durability': 'process_lifetime_only_KM02_3_pending', 'quality': 'not_evaluated', 'independent_acceptance': False, 'paid_model_calls': 0}
    write('summary.json', summary)
    print(json.dumps(summary))


if __name__ == '__main__':
    main()
