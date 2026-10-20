#!/usr/bin/env python3
"""Verify E1 actual durable host protocol, retained bytes and fresh Go validation."""
import gzip
import hashlib
import importlib.util
import io
import json
from pathlib import Path, PurePosixPath
import tarfile

import dataset_contract as contract

ROOT = Path(__file__).resolve().parents[1]
PACKET = ROOT / 'system/eval/knowledge-roadmap-2026-10-15/km-02-durable-host'


def module(name, path):
    spec = importlib.util.spec_from_file_location(name, path)
    value = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(value)
    return value


roadmap = module('maintenance_current_roadmap', ROOT / 'scripts/check-knowledge-roadmap.py')


def require(value, message):
    if not value:
        raise ValueError(message)


def sha(data):
    return hashlib.sha256(data).hexdigest()


def check(folder=PACKET, root=ROOT):
    try:
        folder, root = folder.resolve(), root.resolve()
        def raw(name):
            p = folder / name
            return p.read_bytes() if p.exists() else gzip.decompress((folder / (name + '.gz')).read_bytes())
        def load(name):
            return json.loads(raw(name))
        manifest = load('manifest.json')
        require(manifest['scope'] == 'KM02_3_E1_actual_public_durable_host_only'
                and manifest['KM02_2_complete'] and not manifest['full_KM02_complete']
                and manifest['durable_recovery_verified'] and manifest['KM02_3_complete'] is False and not manifest['independent_acceptance'], 'scope promoted')
        paths = [r['path'] for r in manifest['artifacts']]
        actual = {p.relative_to(folder).as_posix() for p in folder.rglob('*') if p.is_file() and p.name != 'manifest.json'}
        require(len(paths) == len(set(paths)) and set(paths) == actual, 'artifact inventory differs')
        for row in manifest['artifacts']:
            p = (folder / row['path']).resolve()
            require(p.is_relative_to(folder) and p.is_file(), 'unsafe artifact')
            data = p.read_bytes()
            require(sha(data) == row['sha256'] and len(data) == row['bytes'], 'artifact bytes differ')
            if 'uncompressed_sha256' in row:
                require(sha(gzip.decompress(data)) == row['uncompressed_sha256'], 'raw artifact differs')
        for name, digest in manifest['source_sha256'].items():
            p = (root / name).resolve()
            require(p.is_relative_to(root) and sha(p.read_bytes()) == digest, 'runtime source differs')
        binding = load('source-binding.json')
        for name, digest in {**binding['source_sha256'], **binding['approved_input_sha256']}.items():
            p = (root / name).resolve()
            require(p.is_relative_to(root) and sha(p.read_bytes()) == digest, 'current source/input binding differs')
        require(binding['approval_or_completion_states_changed'] is False, 'prior acceptance altered')
        summary = load('runtime/summary.json')
        require(summary['scope'] == 'KM02_3_E1_actual_durable_scoped_jobs_mock_embedding' and summary['checks'] == 43
                and summary['sessions'] == 9 and summary['EOF_exit_codes'] == [0,0,0,0,0,-9,-9,0,0] and summary['paid_model_calls'] == 0
                and summary['durability'] == 'durable_journal_E2_rollback_GC_pending' and summary['quality'] == 'not_evaluated'
                and summary['independent_acceptance'] is False, 'runtime scope/termination differs')
        checks = load('runtime/checks.json')
        require(len(checks) == len(set(checks)) == summary['checks'] and checks == EXPECTED_CHECKS, 'runtime checks missing')
        wire_calls, replies_by_value = [], []
        wire_pairs = 0
        for session in ('default-session', 'maintenance-session', 'restarted-session', 'reader-v1', 'reader-v3', 'crash-host', 'crash-recovery', 'cancel-recovery', 'shutdown-recovery'):
            inputs = [json.loads(x) for x in raw('runtime/' + session + '/mcp.stdin.jsonl').splitlines()]
            outputs = [json.loads(x) for x in raw('runtime/' + session + '/mcp.stdout.jsonl').splitlines()]
            require(len(inputs) == len(outputs) and len({r['id'] for r in inputs}) == len(inputs)
                    and [r['id'] for r in inputs] == [r['id'] for r in outputs]
                    and inputs[0]['method'] == 'initialize' and all('error' not in r for r in outputs), 'wire missing or failed')
            wire_pairs += len(inputs)
            for request, reply in zip(inputs, outputs, strict=True):
                if request['method'] == 'tools/call' and request['params']['name'].startswith('cks.dataset.'):
                    wire_calls.append({'session': session, 'tool': request['params']['name'], 'arguments': request['params']['arguments'], 'response': reply['result']})
                    if not reply['result'].get('isError'):
                        replies_by_value.append(reply['result']['structuredContent'])
                        if request['params']['name'].endswith('.cancel'):
                            replies_by_value.append(reply['result']['structuredContent']['status'])
        calls = load('runtime/calls.json')
        require(calls == wire_calls and len(calls) == summary['tool_calls'], 'call trace differs')
        require(wire_pairs == manifest['wire_pairs'], 'wire denominator differs')
        tools = load('runtime/tools.json')
        names = {t['name']: t for t in tools}
        require(len(tools) == 37 and all(names['cks.dataset.' + n].get('outputSchema') for n in ('inspect_profile', 'build', 'status', 'cancel')), 'tool/schema missing')
        observations = load('runtime/observations.json')
        require(len(observations) == summary['status_observations'], 'status denominator differs')
        last, terminal = {}, {}
        for row in observations:
            status, request, profile = row['status'], row['request'], row['profile']
            contract.validate_status(status, request, profile)
            require(status in replies_by_value, 'status absent from raw wire')
            job_id = status['job_id']
            if job_id in last:
                before = last[job_id]
                require(before['revision'] <= status['revision'] and (before['revision'] != status['revision'] or before == status), 'revision rewind or mutation')
            if job_id in terminal:
                require(terminal[job_id] == status, 'terminal status changed')
            if status['terminal']:
                terminal[job_id] = status
            last[job_id] = status
        require(sorted(s['state'] for s in terminal.values()) == ['canceled', 'canceled', 'canceled', 'held_for_review', 'held_for_review', 'interrupted', 'succeeded', 'succeeded'], 'real terminal cases missing')
        require(all(s['serving_adoption'] == 'not_observed' for s in last.values()), 'serving adoption invented')
        requested = [r['response']['structuredContent'] for r in calls if r['tool'].endswith('.cancel') and not r['response'].get('isError')]
        require(any(r['decision'] == 'too_late' and r['status']['activated'] for r in requested)
                and any(r['decision'] == 'accepted' and r['status']['state'] == 'cancel_requested' and not r['status']['terminal'] for r in requested), 'cancel/publication distinction missing')
        before, after = load('runtime/retained-a1-before.json'), load('runtime/retained-a1-after.json')
        require(before == after and len(before) > 5, 'retained DB changed')
        with tarfile.open(fileobj=io.BytesIO(raw('fixture.tar'))) as archive:
            members = {m.name: m for m in archive}
            require(len(members) == len(archive.getmembers()), 'duplicate fixture member')
            for m in members.values():
                p = PurePosixPath(m.name)
                require(not p.is_absolute() and '..' not in p.parts and (m.isfile() or m.isdir() or m.issym()), 'unsafe fixture')
                if m.issym():
                    require(m.name == 'project-a/dataset/current' and m.linkname == 'v3', 'unexpected pointer')
            def fixture(name):
                m = members[name]
                require(m.isfile(), 'fixture is not a regular file')
                return archive.extractfile(m).read()
            require('project-b/dataset/current' not in members, 'held/canceled B published')
            for name, digest in before.items():
                require(sha(fixture(name)) == digest, 'old retained file bytes differ')
            for version, marker in [('v1', 101), ('v2', 303), ('v3', 404)]:
                prefix = 'project-a/dataset/' + version + '/'
                identity = json.loads(fixture(prefix + 'dataset-identity.json'))
                require(identity['source']['project_id'] == 'project-a' and members[prefix + 'graph/graph.db'].size > 0 and members[prefix + 'vector/vector.db'].size > 0, 'actual DB/identity missing')
                source = json.loads(fixture(prefix + 'sources/manifest.json'))
                for row in source['files']:
                    data = fixture(prefix + 'sources/blobs/' + row['sha256'])
                    require(sha(data) == row['sha256'], 'retained blob hash differs')
                    if row['path'] == 'consensus.go':
                        require(('return ' + str(marker)).encode() in data, 'wrong retained implementation')
                if version != 'v2':
                    pack = load('runtime/read-' + version + '.json')
                    require(pack['coordinates']['dataset_id'] == identity['dataset_id'] and pack['citations'] and pack['bodies'], 'actual read proof missing')
                    files = {(f['origin_id'], f['path']): f for f in source['files']}
                    for citation in pack['citations']:
                        f = files[(citation['origin_id'], citation['file'])]
                        data = fixture(prefix + 'sources/blobs/' + f['sha256'])
                        span = b''.join(data.splitlines(keepends=True)[citation['start_line']-1:citation['end_line']])
                        require(citation['project_id'] == 'project-a' and citation['dataset_id'] == identity['dataset_id'] and citation['file_sha256'] == f['sha256'] and citation['content_sha256'] == sha(span), 'citation differs')
        validation = load('validation.json')
        for label in ('module', 'race'):
            data = raw(label + '.jsonl')
            inventory, _ = roadmap.test_inventory(data)
            require(inventory == validation[label] and inventory['failed_tests'] == inventory['failed_packages'] == 0, 'Go completion/skip denominator differs')
            require(roadmap.test_rows(data) == load(label + '-tests.json'), 'Go test inventory differs')
        require(validation['module']['test_cases'] >= 3811 and validation['race']['passed_tests'] >= 783
                and validation['module']['skipped_tests'] == 19 and validation['race']['skipped_tests'] == 2, 'old Go proof reused for changed runtime')
        require(all(v == 0 for v in validation['gates'].values()) and set(validation['gates']) >= {'build', 'lint', 'runtime', 'historical_design', 'roadmap'}, 'validation gate failed/missing')
        require('0 issues.' in raw('lint.txt').decode() or 'documentation: OK' in raw('lint.txt').decode(), 'lint evidence missing')
        race_rows = load('race-tests.json')
        require(any(r['test'] == 'TestPublicServiceClosePersistsCancellationBeforeStoppingChild' and r['state'] == 'pass' for r in race_rows), 'clean EOF cancellation regression missing')
        historical = load('historical-design.json')
        require(historical['verification_exit_code'] == 0 and historical['current_Go_validation_reused'] is False and all(r['exit_code'] == 0 for r in historical['checks']), 'historical design proof differs')
        require(historical['revision'].startswith('ed74cd7f'), 'wrong historical D2 checkpoint')
        for call in calls:
            if call['tool'].endswith('.inspect_profile') and not call['response'].get('isError'):
                require(call['response']['structuredContent']['capabilities']['job_persistence'] == 'durable_journal', 'public persistence capability differs')
        for version in ('v3', 'v4', 'v5'):
            prefix = 'project-b/dataset/' + version + '/'
            require(not any(name.startswith(prefix + '.build-source-') or name == prefix + 'build-staging.json' for name in members), 'abandoned scratch survived')
        require(not roadmap.check(root=root), 'current roadmap differs')
    except (OSError, ValueError, KeyError, TypeError, IndexError, tarfile.TarError) as error:
        return [str(error)]
    return []


EXPECTED_CHECKS = [
    "default surface remains 33 tools without dataset jobs",
    "four opt-in public tools expose schemas and read/write hints",
    "inspection does not create either dataset and observes absent base",
    "refuse stale profile_digest",
    "refuse stale input_digest",
    "refuse stale embedding_digest",
    "refuse stale source snapshot",
    "refuse foreign profile and caller path injection",
    "first actual graph/vector build publishes v1 with receipt and passed gates",
    "candidate identity and retained source bytes match project A",
    "same key replays committed result before stale base/version checks",
    "same key different request refuses conflict",
    "foreign project/profile cannot observe or cancel job",
    "cancel after publication is too_late without rollback",
    "project B held candidate never creates current",
    "A v2 held candidate preserves A v1 and B isolation",
    "reject stale observed current before building",
    "second activation publishes exact v3 and preserves old immutable DB",
    "slow graph job remains running before cancel",
    "concurrent job on same dataset refused",
    "cancel acknowledgment is not false terminal success",
    "cancellation terminates child build and leaves current absent",
    "terminal status remains immutable",
    "durable reconnect preserves exact terminal snapshot and replay",
    "actual v2 query reads retained A v1 marker 101",
    "actual v2 query reads retained A v3 marker 404",
    "all five stdio sessions exit cleanly on EOF",
    "original retained files unchanged across builds cancellation and reconnect",
    'actual graph child owns durable admission and staging before host kill',
    'SIGKILL restart recovers pre-gate job as interrupted without publication',
    'owner loss stops late graph writer before terminal recovery',
    'restart removes only owned scratch and preserves captured diagnostics',
    'interrupted original request replays exactly after restart',
    'restarted key still refuses a different request',
    'accepted cancel intent is persisted before acknowledgment',
    'cancel survives host kill and cannot become successful publication',
    'canceled restart has no late child write or owned scratch',
    'normal EOF waits for active child cancellation and persists canceled result',
    'EOF shutdown leaves no late child writer or scratch',
    'published result remains immutable through unrelated crashes',
    'new crashed jobs remain project scoped',
    'all expected EOF exits and two explicit host kills are accounted',
    'retained A databases and B held database survive crashes'
]

if __name__ == '__main__':
    errors = check()
    for error in errors:
        print('FAIL:', error)
    print('durable host evidence:', 'FAIL' if errors else 'PASS; E1 only; rollback/GC E2 pending')
    raise SystemExit(bool(errors))
