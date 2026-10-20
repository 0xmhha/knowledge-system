#!/usr/bin/env python3
"""Verify actual E2 reader/rollback/GC proof and KM-02.3 completion boundary."""
import gzip
import hashlib
import importlib.util
import io
import json
from pathlib import Path, PurePosixPath
import tarfile

import dataset_contract as contract

ROOT = Path(__file__).resolve().parents[1]
PACKET = ROOT / 'system/eval/knowledge-roadmap-2026-10-15/km-02-durable-retention'
E1 = 'system/eval/knowledge-roadmap-2026-10-15/km-02-durable-host/'
spec = importlib.util.spec_from_file_location('retention_roadmap', ROOT / 'scripts/check-knowledge-roadmap.py')
roadmap = importlib.util.module_from_spec(spec)
spec.loader.exec_module(roadmap)


def sha(data):
    return hashlib.sha256(data).hexdigest()


def require(value, message):
    if not value:
        raise ValueError(message)


def check(folder=PACKET, root=ROOT):
    try:
        folder, root = folder.resolve(), root.resolve()
        def raw(name):
            p = folder / name
            return p.read_bytes() if p.exists() else gzip.decompress((folder / (name + '.gz')).read_bytes())
        def load(name):
            return json.loads(raw(name))
        manifest = load('manifest.json')
        require(manifest['checkpoint'] == 'KM02_3_E2_actual_public_reader_rollback_GC'
                and manifest['KM02_3_complete'] is True and manifest['full_KM02_complete'] is False
                and manifest['independent_acceptance'] is False, 'completion scope inflated')
        actual = {p.relative_to(folder).as_posix() for p in folder.rglob('*') if p.is_file() and p.name != 'manifest.json'}
        require(actual == {r['path'] for r in manifest['artifacts']} and len(actual) == len(manifest['artifacts']), 'artifact inventory differs')
        for row in manifest['artifacts']:
            p = (folder / row['path']).resolve()
            require(p.is_relative_to(folder) and p.is_file(), 'unsafe artifact')
            data = p.read_bytes()
            require(sha(data) == row['sha256'] and len(data) == row['bytes'], 'artifact bytes differ')
            if 'uncompressed_sha256' in row:
                require(sha(gzip.decompress(data)) == row['uncompressed_sha256'], 'raw artifact differs')
        binding = load('source-binding.json')
        for path, digest in {**binding['source_sha256'], **binding['approved_input_sha256'], **manifest['source_sha256']}.items():
            p = (root / path).resolve()
            require(p.is_relative_to(root) and sha(p.read_bytes()) == digest, 'source/input binding differs')
        require(binding['approval_or_completion_states_changed'] is False, 'prior acceptance changed')
        inp = load('runtime/input.json')
        require(inp['E1_fixture_sha256'] == sha((root / E1 / 'fixture.tar.gz').read_bytes())
                and inp['E1_manifest_sha256'] == sha((root / E1 / 'manifest.json').read_bytes()), 'actual E1 input differs')
        previous = json.loads(gzip.decompress((root / E1 / 'runtime/observations.json.gz').read_bytes()))
        original = load('runtime/original.json')
        require(original in previous and original['status']['state'] == 'succeeded' and original['status']['target_version'] == 'v3', 'original not from actual E1 wire')
        summary, checks = load('runtime/summary.json'), load('runtime/checks.json')
        require(summary['scope'] == 'KM02_3_E2_actual_reader_rollback_GC_mock_embedding' and summary['checks'] == 21
                and checks == EXPECTED_CHECKS and summary['sessions'] == 5 and summary['EOF_exit_codes'] == [0]*5
                and summary['paid_model_calls'] == 0 and summary['quality'] == 'not_evaluated' and summary['independent_acceptance'] is False, 'runtime scope/count/exit differs')
        wire, values, pairs = [], [], 0
        for name in ('maintenance-session', 'reader-v3', 'reader-v3-repin', 'restarted-session', 'reader-v5'):
            inputs = [json.loads(x) for x in raw('runtime/' + name + '/mcp.stdin.jsonl').splitlines()]
            outputs = [json.loads(x) for x in raw('runtime/' + name + '/mcp.stdout.jsonl').splitlines()]
            require(inputs and inputs[0]['method'] == 'initialize' and len(inputs) == len(outputs)
                    and [r['id'] for r in inputs] == [r['id'] for r in outputs]
                    and len({r['id'] for r in inputs}) == len(inputs) and all('error' not in r for r in outputs), 'wire response missing')
            pairs += len(inputs)
            for request, reply in zip(inputs, outputs, strict=True):
                if request['method'] == 'tools/call':
                    require(not reply['result'].get('isError'), 'public tool failed')
                    wire.append({'session': name, 'tool': request['params']['name'], 'arguments': request['params']['arguments'], 'response': reply['result']})
                    values.append(reply['result']['structuredContent'])
        calls = load('runtime/calls.json')
        require(len(wire) == len(calls) == summary['tool_calls'] and pairs == manifest['wire_pairs']
                and {r['session'] for r in wire} == {r['session'] for r in calls}
                and all([r for r in wire if r['session'] == name] == [r for r in calls if r['session'] == name] for name in {r['session'] for r in wire}), 'raw per-session call trace differs')
        observations = load('runtime/observations.json')
        require(len(observations) == summary['status_observations'], 'status denominator differs')
        last, terminal = {}, {}
        for row in observations:
            status = row['status']
            contract.validate_status(status, row['request'], row['profile'])
            require(status in values, 'status not from wire')
            key = status['job_id']
            if key in last:
                before = last[key]
                require(before['revision'] <= status['revision'] and (before['revision'] != status['revision'] or before == status), 'revision mutation/rewind')
            if key in terminal:
                require(terminal[key] == status, 'terminal changed')
            if status['terminal']:
                terminal[key] = status
            last[key] = status
        require(sorted(s['target_version'] for s in terminal.values()) == ['v3', 'v4', 'v5'] and all(s['state'] == 'succeeded' and s['serving_adoption'] == 'not_observed' for s in terminal.values()), 'actual publications missing/adoption invented')
        old = original['status']
        require(sum(r['tool'] == 'cks.dataset.build' and r['arguments'] == original['request'] and r['response']['structuredContent'] == old for r in calls) == 3, 'original replay missing before/after GC/restart')
        plans = {name: load('runtime/' + name + '.json') for name in ('gc-active-reader', 'gc-released-reader', 'gc-collectible')}
        def item(plan, version):
            return next(v for v in plan['versions'] if v['version'] == version)
        active = item(plans['gc-active-reader'], 'v3')
        require(not active['delete'] and 'pinned_reader' in active['protected_reasons'] and 'review_hold' in item(plans['gc-active-reader'], 'v2')['protected_reasons'], 'live reader/held protection missing')
        require(all(item(plans[n], 'v3')['delete'] and 'pinned_reader' not in item(plans[n], 'v3')['protected_reasons'] for n in ('gc-released-reader', 'gc-collectible')), 'released reader collection missing')
        for name, plan in plans.items():
            require(json.loads(raw('runtime/' + name + '.stdout.txt')) == plan, 'plan not from CLI stdout')
        commands = load('runtime/commands.json')
        require(len({r['label'] for r in commands}) == len(commands) == 11, 'CLI receipt missing')
        require({r['label'] for r in commands if r['exit_code'] != 0} == {'gc-stale-reader-plan', 'refuse-reader-deleted-v3'}
                and all((r['exit_code'] == 0) == r['expected_success'] for r in commands), 'expected CLI rejection/success differs')
        result = json.loads(raw('runtime/gc-apply-released.stdout.txt'))
        require(result['status'] == 'applied' and result['plan_digest'] == plans['gc-collectible']['digest']
                and result['reclaimed_bytes'] == plans['gc-collectible']['reclaim_bytes'] > 0, 'actual collection receipt differs')
        before, after = load('runtime/preserved-before.json'), load('runtime/preserved-after.json')
        require(before == after and len(before['A_v1']) > 5 and len(before['B_v1']) > 5, 'preserved old bytes differ')
        def inventory(data):
            result = {}
            with tarfile.open(fileobj=io.BytesIO(data)) as archive:
                for entry in archive:
                    p = PurePosixPath(entry.name)
                    require(not p.is_absolute() and '..' not in p.parts and entry.name not in result and (entry.isfile() or entry.isdir() or entry.issym()), 'unsafe fixture')
                    if entry.issym():
                        require(entry.name == 'project-a/dataset/current' and entry.linkname in ('v3', 'v5'), 'unexpected fixture link')
                    result[entry.name] = (entry.linkname if entry.issym() else archive.extractfile(entry).read() if entry.isfile() else None)
            return result
        prior = inventory((root / E1 / 'fixture.tar.gz').read_bytes())
        final = inventory(raw('fixture.tar'))
        require(final['project-a/dataset/current'] == 'v5' and not any(n.startswith('project-a/dataset/v3/') for n in final)
                and 'project-b/dataset/current' not in final, 'actual pointer/deletion/isolation differs')
        for label, prefix in [('A_v1', 'project-a/dataset/v1/'), ('B_v1', 'project-b/dataset/v1/')]:
            require({n[len(prefix):]: sha(b) for n,b in final.items() if n.startswith(prefix) and isinstance(b, bytes)} == before[label], 'retained fixture differs')
            require(all(sha(prior[prefix+n]) == h for n,h in before[label].items()), 'preserved bytes not from E1')
        name = 'project-a/dataset/.maintenance-publications/' + old['job_id'] + '.json'
        require(final[name] == prior[name] and sha(final[name]) == before['A_v3_history'], 'historical publication changed after GC')
        for version, marker, labels in [('v3', 404, ['read-v3-before', 'read-v3-after-rollback', 'read-v3-after-gc-protection', 'read-v3-repin']), ('v5', 1001, ['read-v5-after-gc'])]:
            source = prior if version == 'v3' else final
            prefix = 'project-a/dataset/' + version + '/'
            identity = json.loads(source[prefix + 'dataset-identity.json'])
            require(source[prefix + 'graph/graph.db'] and source[prefix + 'vector/vector.db'], 'real DB absent')
            files = {(r['origin_id'], r['path']): r for r in json.loads(source[prefix + 'sources/manifest.json'])['files']}
            for label in labels:
                pack = load('runtime/' + label + '.json')
                require(pack in values and pack['coordinates']['dataset_id'] == identity['dataset_id'] and pack['citations'] and pack['bodies']
                        and any(('return '+str(marker)) in json.dumps(body) for body in pack['bodies']), 'actual retained query missing')
                for c in pack['citations']:
                    row = files[(c['origin_id'], c['file'])]
                    data = source[prefix + 'sources/blobs/' + row['sha256']]
                    span = b''.join(data.splitlines(keepends=True)[c['start_line']-1:c['end_line']])
                    require(c['project_id'] == 'project-a' and c['dataset_id'] == identity['dataset_id'] and c['file_sha256'] == sha(data) and c['content_sha256'] == sha(span), 'citation/source/project differs')
        for version in ('v1', 'v2', 'v4', 'v5'):
            require(final['project-a/dataset/' + version + '/graph/graph.db'], 'protected actual DB removed')
        validation = load('validation.json')
        for label in ('module', 'race'):
            data = raw(label + '.jsonl')
            inventory_counts, _ = roadmap.test_inventory(data)
            require(inventory_counts == validation[label] and inventory_counts['failed_tests'] == inventory_counts['failed_packages'] == 0
                    and roadmap.test_rows(data) == load(label+'-tests.json'), 'Go proof/count/skip differs')
        require(validation['module']['passed_tests'] == 3794 and validation['module']['skipped_tests'] == 18
                and validation['race']['passed_tests'] == 784 and validation['race']['skipped_tests'] == 2
                and all(v == 0 for v in validation['gates'].values()) and 'documentation: OK' in raw('lint.txt').decode(), 'validation gate/denominator differs')
        history = load('historical-design.json')
        require(history['revision'].startswith('0fa42cef') and history['verification_exit_code'] == 0 and history['current_Go_validation_reused'] is False
                and all(r['exit_code'] == 0 for r in history['checks']), 'historical E1 proof differs')
        progress = json.loads((root / 'docs/spec-driven/KNOWLEDGE-ROADMAP-PROGRESS.json').read_text())
        item02 = next(r for r in progress['items'] if r['id'] == 'KM-02')
        require(progress['completed_subtasks'] == 7 and item02['subtasks'][2]['status'] == 'complete'
                and item02['subtasks'][3]['status'] != 'complete' and not roadmap.check(root=root), 'roadmap completion boundary differs')
    except (OSError, ValueError, KeyError, TypeError, IndexError, StopIteration, tarfile.TarError) as error:
        return [str(error)]
    return []


EXPECTED_CHECKS = [
    'restore hash-bound actual E1 datasets into fresh owned roots',
    'restored durable terminal and original replay match E1 snapshot',
    'actual MCP reader pins retained v3 before rollback',
    'actual CLI rollback switches current to v1',
    'pinned reader retains v3 coordinates and body through rollback',
    'rollback does not rewrite historical succeeded publication receipt',
    'new v4 publication uses rolled back v1 as checked base',
    'new v5 publication advances current and preserves pinned v3',
    'GC plan protects actual pinned v3 and held v2',
    'reviewed GC apply leaves active reader dataset intact',
    'actual pinned reader remains serviceable after protected GC',
    'released v3 becomes GC eligible without manual reference edits',
    'GC rejects reviewed plan made stale by a new live reader',
    'reviewed GC actually removes released v3 and reclaims bytes',
    'GC preserves current rollback target held candidate and explicit v1',
    'GC retains immutable historical publication and exact job replay',
    'host restart after version GC preserves historical terminal result',
    'deleted v3 cannot be silently rebound as another dataset',
    'current v5 actual query returns new retained implementation after GC',
    'old A v1 B held DB and publication history bytes stay unchanged',
    'all five E2 stdio sessions end with clean EOF',
]

if __name__ == '__main__':
    errors = check()
    for error in errors:
        print('FAIL:', error)
    print('durable retention evidence:', 'FAIL' if errors else 'PASS; KM-02.3 only; KM-02.4 pending')
    raise SystemExit(bool(errors))
