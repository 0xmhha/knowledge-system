#!/usr/bin/env python3
"""KM-02.4 actual public failures; reuse Go proof only for identical inputs."""
import gzip
import hashlib
import importlib.util
import io
import json
from pathlib import Path, PurePosixPath
import tarfile

import dataset_contract as contract

ROOT = Path(__file__).resolve().parents[1]
PACKET = ROOT / 'system/eval/knowledge-roadmap-2026-10-15/km-02-public-failures'
E2 = 'system/eval/knowledge-roadmap-2026-10-15/km-02-durable-retention/'
E1 = 'system/eval/knowledge-roadmap-2026-10-15/km-02-durable-host/'
spec = importlib.util.spec_from_file_location('failure_roadmap', ROOT / 'scripts/check-knowledge-roadmap.py')
roadmap = importlib.util.module_from_spec(spec)
spec.loader.exec_module(roadmap)


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
        m = load('manifest.json')
        require(m['checkpoint'] == 'KM02_4_actual_public_failure_preservation' and m['full_KM02_complete'] is True
                and m['independent_acceptance'] is False and m['quality'] == 'not_evaluated', 'completion scope inflated')
        actual = {p.relative_to(folder).as_posix() for p in folder.rglob('*') if p.is_file() and p.name != 'manifest.json'}
        require(actual == {r['path'] for r in m['artifacts']} and len(actual) == len(m['artifacts']), 'artifact inventory differs')
        for row in m['artifacts']:
            path = (folder / row['path']).resolve()
            require(path.is_relative_to(folder), 'unsafe artifact path')
            data = path.read_bytes()
            require(sha(data) == row['sha256'] and len(data) == row['bytes'], 'artifact bytes differ')
            if 'uncompressed_sha256' in row:
                require(sha(gzip.decompress(data)) == row['uncompressed_sha256'], 'raw artifact differs')
        binding = load('source-binding.json')
        for name, digest in {**binding['source_sha256'], **binding['approved_input_sha256'], **m['source_sha256']}.items():
            path = (root / name).resolve()
            require(path.is_relative_to(root) and sha(path.read_bytes()) == digest, 'current source/input differs')
        require(binding['approval_or_completion_states_changed'] is False, 'prior acceptance changed')
        # Current production Go, Go fixtures/dependencies and all prior bound
        # inputs must match E2 before its actual count=1 Go proof is reused.
        previous_manifest = json.loads((root / E2 / 'manifest.json').read_text())
        previous_bytes = (root / E2 / 'source-binding.json').read_bytes()
        require(sha(previous_bytes) == next(r['sha256'] for r in previous_manifest['artifacts'] if r['path']=='source-binding.json'), 'E2 source binding was replaced')
        previous = json.loads(previous_bytes)
        for name, digest in {**previous['source_sha256'], **previous['approved_input_sha256']}.items():
            require(sha((root / name).read_bytes()) == digest, 'E2 Go/input proof cannot be reused after changes')
        expected_go = {n for n in previous['source_sha256'] if n.endswith('.go') and n.startswith(('cmd/','internal/','pkg/'))}
        actual_go = {p.relative_to(root).as_posix() for directory in ('cmd','internal','pkg') for p in (root/directory).rglob('*.go') if p.is_file()}
        require(actual_go == expected_go, 'Go files were added/removed; old proof cannot be reused')
        v = load('validation.json')
        require(v['Go_reference']['revision'].startswith('ae3f987d') and v['Go_reference']['maintained_Go_files_equal'] == 1253
                and v['Go_reference']['manifest_sha256'] == sha((root/E2/'manifest.json').read_bytes())
                and sum(n.endswith('.go') and n.startswith(('cmd/','internal/','pkg/')) for n in previous['source_sha256']) == 1253
                and v['current_Go_validation_reused'] is True and v['Go_inputs_unchanged'] is True, 'Go reference misrepresented')
        previous_manifest = json.loads((root / E2 / 'manifest.json').read_text())
        for label in ('module', 'race'):
            path = label + '.jsonl.gz'
            data = (root / E2 / path).read_bytes()
            require(sha(data) == next(r['sha256'] for r in previous_manifest['artifacts'] if r['path'] == path), 'referenced Go raw proof differs')
            counts, _ = roadmap.test_inventory(gzip.decompress(data))
            require(counts == v[label] and counts['failed_tests'] == counts['failed_packages'] == 0, 'Go reference/count/skip differs')
        require(all(x == 0 for x in v['gates'].values()) and 'documentation: OK' in raw('lint.txt').decode(), 'current validation gate missing/failed')
        history = load('historical-design.json')
        require(history['revision'].startswith('ae3f987d') and history['verification_exit_code'] == 0 and all(r['exit_code'] == 0 for r in history['checks']), 'historical E2 check failed')
        summary = load('runtime/summary.json')
        require(summary['scope'] == 'KM02_4_actual_public_failure_preservation_mock_embedding' and summary['checks'] == 36
                and load('runtime/checks.json') == EXPECTED_CHECKS and summary['sessions'] == 5 and summary['EOF_exit_codes'] == [0,0,-9,0,0]
                and summary['paid_model_calls'] == 0 and summary['independent_acceptance'] is False and summary['quality'] == 'not_evaluated', 'actual runtime scope/termination differs')
        calls, values, pairs = load('runtime/calls.json'), [], 0
        for session in ('maintenance-session', 'reader-v3', 'eof-recovery', 'kill-recovery', 'reader-v4'):
            ins = [json.loads(x) for x in raw('runtime/'+session+'/mcp.stdin.jsonl').splitlines()]
            outs = [json.loads(x) for x in raw('runtime/'+session+'/mcp.stdout.jsonl').splitlines()]
            require(ins and ins[0]['method'] == 'initialize' and len(ins) == len(outs) and [r['id'] for r in ins] == [r['id'] for r in outs]
                    and len({r['id'] for r in ins}) == len(ins) and all('error' not in r for r in outs), 'wire incomplete')
            pairs += len(ins)
            selected = []
            for request, reply in zip(ins, outs, strict=True):
                if request['method'] == 'tools/call':
                    result = reply['result']
                    selected.append({'session': session, 'tool': request['params']['name'], 'arguments': request['params']['arguments'], 'response': result})
                    if not result.get('isError'):
                        value = result['structuredContent']
                        values.append(value)
                        if request['params']['name'].endswith('.cancel'):
                            values.append(value['status'])
            require(selected == [r for r in calls if r['session'] == session], 'raw per-session calls differ')
        require(len(calls) == summary['tool_calls'] and pairs == m['wire_pairs'], 'wire/call denominator differs')
        refused = [r['response']['structuredContent']['code'] for r in calls if r['response'].get('isError')]
        require(sorted(refused) == sorted(['project_mismatch']*3 + ['invalid_request']*3 + ['version_exists'] + ['job_unknown']*2), 'public wrong-target/path refusal missing')
        observations = load('runtime/observations.json')
        require(len(observations) == summary['status_observations'], 'status denominator differs')
        last, terminal = {}, {}
        for row in observations:
            status = row['status']
            contract.validate_status(status, row['request'], row['profile'])
            require(status in values, 'status absent from raw wire')
            key = status['job_id']
            if key in last:
                before = last[key]
                require(before['revision'] <= status['revision'] and (before['revision'] != status['revision'] or before == status), 'status revision rewound/mutated')
            if key in terminal:
                require(terminal[key] == status, 'terminal status changed')
            if status['terminal']:
                terminal[key] = status
            last[key] = status
        states = {r['target_version']: r for r in terminal.values()}
        expected = {'failed-graph':'failed', 'failed-vector':'failed', 'failed-semantic':'failed', 'failed-source-drift':'failed',
                    'failed-profile-drift':'failed', 'failed-timeout':'failed', 'failed-cancel':'canceled', 'failed-eof':'canceled', 'failed-host-kill':'interrupted', 'v4':'succeeded'}
        require({k:v['state'] for k,v in states.items()} == expected and all(not r['activated'] and r['publication'] is None for k,r in states.items() if k!='v4')
                and states['failed-timeout']['error']['code'] == 'deadline_exceeded', 'actual partial failure/cancel/timeout decisions differ')
        require(all(r['serving_adoption'] == 'not_observed' for r in states.values()), 'serving adoption invented')
        original = load('runtime/original.json')
        prior_obs = json.loads(gzip.decompress((root / E1 / 'runtime/observations.json.gz').read_bytes()))
        require(original in prior_obs and any(r['tool'] == 'cks.dataset.build' and r['arguments'] == original['request'] and r['response']['structuredContent'] == original['status'] for r in calls), 'original replay changed')
        with tarfile.open(fileobj=io.BytesIO(raw('fixture.tar'))) as archive:
            members = {}
            for entry in archive:
                path = PurePosixPath(entry.name)
                require(not path.is_absolute() and '..' not in path.parts and entry.name not in members and (entry.isfile() or entry.isdir() or entry.issym()), 'unsafe fixture')
                if entry.issym():
                    require(entry.name == 'project-a/dataset/current' and entry.linkname == 'v4', 'unexpected pointer/link')
                members[entry.name] = entry.linkname if entry.issym() else archive.extractfile(entry).read() if entry.isfile() else None
        require(members['project-a/dataset/current'] == 'v4' and 'project-b/dataset/current' not in members
                and members['project-a/dataset/failed-vector/graph/graph.db'] and members['project-a/dataset/failed-semantic/graph/graph.db'], 'actual graph partials/pointers missing')
        require(json.loads(members['foreign-semantic/manifest.json'])['snapshot']['commit'] == 'f'*40, 'mismatched actual semantic input missing')
        preserved = load('runtime/preserved-after.json')
        for label in ('admission-refusals','graph-failure','vector-failure','semantic-mismatch','source-drift','profile-drift','timeout','cancel','EOF','SIGKILL'):
            row = load('runtime/integrity-'+label+'.json')
            require(row['A_v3_persistent'] == row['A_v3_before'] == preserved['A_v3'] and row['A_v1_before'] == row['A_v1_during'] == preserved['A_v1']
                    and row['B_v1_before'] == row['B_v1_during'] == preserved['B_v1'] and row['history_before'] == row['history_during'] == preserved['A_v3_history'], 'old current/project/history changed')
            extras = set(row['A_v3_during'])-set(row['A_v3_before'])
            require(extras <= {'vector/vector.db-shm','vector/vector.db-wal'} and all(row['A_v3_during'][k] == h for k,h in row['A_v3_before'].items())
                    and all(info['bytes'] == 0 for name,info in row['A_v3_reader_sidecars'].items() if name.endswith('-wal')), 'reader wrote old data or unexpected sidecar')
        def fixture_hashes(prefix):
            return {name[len(prefix):]: sha(body) for name,body in members.items() if name.startswith(prefix) and isinstance(body,bytes)}
        require(fixture_hashes('project-a/dataset/v3/') == preserved['A_v3'] and fixture_hashes('project-a/dataset/v1/') == preserved['A_v1']
                and fixture_hashes('project-b/dataset/v1/') == preserved['B_v1'], 'post-reader full inventory/bytes changed')
        for version in expected:
            if version != 'v4':
                prefix = 'project-a/dataset/'+version+'/'
                require(not any(n.startswith(prefix+'.build-source-') or n == prefix+'build-staging.json' for n in members), 'terminal left owned scratch')
        for name in sorted(r['path'] for r in m['artifacts'] if r['path'].startswith('runtime/read-') and r['path'].endswith('.json')):
            pack = load(name)
            version = 'v4' if name.endswith('latest-after-failures.json') else 'v3'
            prefix = 'project-a/dataset/'+version+'/'
            identity = json.loads(members[prefix+'dataset-identity.json'])
            files = {(r['origin_id'],r['path']):r for r in json.loads(members[prefix+'sources/manifest.json'])['files']}
            require(pack in values and pack['coordinates']['dataset_id'] == identity['dataset_id'] and pack['citations'] and pack['bodies'], 'actual reader proof missing')
            for c in pack['citations']:
                f = files[(c['origin_id'],c['file'])]
                data = members[prefix+'sources/blobs/'+f['sha256']]
                span = b''.join(data.splitlines(keepends=True)[c['start_line']-1:c['end_line']])
                require(c['project_id'] == 'project-a' and c['dataset_id'] == identity['dataset_id'] and c['file_sha256'] == sha(data) and c['content_sha256'] == sha(span), 'citation/source mixed')
        progress = json.loads((root / 'docs/spec-driven/KNOWLEDGE-ROADMAP-PROGRESS.json').read_text())
        item = next(r for r in progress['items'] if r['id']=='KM-02')
        require(progress['completed_subtasks']==8 and progress['implementation_completed']==2 and item['status']=='complete'
                and all(r['status']=='complete' for r in item['subtasks']) and not roadmap.check(root=root), 'roadmap completion boundary differs')
    except (OSError, ValueError, KeyError, TypeError, IndexError, StopIteration, tarfile.TarError) as error:
        return [str(error)]
    return []


EXPECTED_CHECKS = ['restore hash-bound actual E1 datasets into fresh owned roots', 'public refusal foreign-project', 'public refusal foreign-profile', 'public refusal invalid-version', 'public refusal existing-version', 'public refusal caller-path', 'public refusal caller-command', 'public refusal foreign-base-coordinates', 'public foreign job cannot be read or canceled', 'preserve current reader and other project after admission-refusals', 'actual graph engine failure cannot publish partial candidate', 'graph failure removes owned scratch after writer exit', 'preserve current reader and other project after graph-failure', 'actual vector engine failure cannot publish partial candidate', 'vector failure removes owned scratch after writer exit', 'preserve current reader and other project after vector-failure', 'mismatched semantic corpus fails after actual graph before activation', 'preserve current reader and other project after semantic-mismatch', 'source drift during real build blocks publication', 'preserve current reader and other project after source-drift', 'profile drift during real build blocks publication', 'preserve current reader and other project after profile-drift', 'actual deadline stops graph writer with deadline_exceeded', 'deadline terminal has no remaining owned scratch', 'preserve current reader and other project after timeout', 'public cancel on active dataset acknowledges intent before terminal', 'active dataset cancellation terminates child without late write', 'preserve current reader and other project after cancel', 'EOF on active current preserves canceled state after restart', 'preserve current reader and other project after EOF', 'SIGKILL on active current recovers interrupted with no late writer', 'preserve current reader and other project after SIGKILL', 'all failed work preserves original public terminal and replay', 'fresh valid build after failure matrix actually publishes v4', 'old pinned reader and new v4 reader remain correctly separated', 'one expected host kill and four clean EOF sessions are accounted']

if __name__ == '__main__':
    errors = check()
    for error in errors:
        print('FAIL:', error)
    print('public failure evidence:', 'FAIL' if errors else 'PASS; KM-02 complete; manager work remains')
    raise SystemExit(bool(errors))
