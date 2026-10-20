"""Replay every predeclared warm pair; incomplete analyses never become acceptance."""
import argparse
import gzip
import json
import math
import re
import sqlite3
import subprocess
from collections import Counter
from pathlib import Path

import wi6_pair_results as prior
import wi6_pair_contract as c
from source_packet import oid

ROOT = prior.ROOT
PACKET = prior.old.BASE / 'wi6-warm-runtime-v1'
REVISION = '3c23a71c3872dbef69a84726a58fb4ade27001e5'
SCOPE_SHA = '7447d304acc9a5b0278190d4f531dfa0500963183bd4527bf21930e8a7aa2a98'
TOOLS = ['scripts/' + name for name in ('run-wi6-warm.py', 'wi6_warm_results.py', 'test_wi6_warm_results.py')]
read, write, compressed = prior.read, prior.write, prior.compressed


def lines(path):
    opener = gzip.open if path.suffix == '.gz' else open
    with opener(path, 'rt') as stream:
        for line in stream:
            yield json.loads(line)


def p95(values):
    if not values or any(type(v) is not int or v <= 0 for v in values):
        raise ValueError('positive integer timings required')
    return sorted(values)[math.ceil(.95 * len(values)) - 1]


def latency(rows):
    """Failure/partial denominator is fixed; diagnostic p95s are never a quality gate."""
    measured = [r for r in rows if r['phase'] == 'measured']
    complete = [r for r in measured if r['complete_pair']]
    times = {arm: p95([r['arms'][arm]['elapsed_ns'] for r in measured])
             for arm in ('baseline', 'expansion')} if measured else {}
    return dict(measured_pairs=len(measured), complete_pairs=len(complete),
                noncomplete_pairs=len(measured)-len(complete),
                diagnostic_all_status_p95_ns=times,
                diagnostic_ratio_of_p95s=(times['expansion']/times['baseline'] if times else None),
                diagnostic_p95_by_status_ns={arm: {status: p95([r['arms'][arm]['elapsed_ns'] for r in measured if r['arms'][arm]['status'] == status])
                    for status in sorted({r['arms'][arm]['status'] for r in measured})} for arm in ('baseline', 'expansion')},
                complete_pair_latency_gate='inconclusive',
                reason='no_valid_complete_pairs' if not complete else 'independent_gold_and_sufficiency_not_approved',
                quality_score=None)


def expected_schedule(cases):
    return [dict(case_id=case['case_id'], phase=phase, index=i,
                 order=['baseline', 'expansion'] if i % 2 == 0 else ['expansion', 'baseline'])
            for case in cases for phase, n in [('warmup', 3), ('measured', 30)] for i in range(n)]


def collect_proof(work, packet):
    manifest = read(work/'data/graph/manifest.json')
    files = {r['path']: r['sha256'] for r in manifest['files']}
    retained = {}
    with sqlite3.connect((work/'data/graph/graph.db').as_uri()+'?mode=ro', uri=True) as connection:
        requests = {r['id']: r for r in lines(work/'stdin.jsonl')}
        for reply in lines(work/'stdout.jsonl'):
            request = requests.get(reply.get('id'), {})
            if request.get('method') != 'tools/call':
                continue
            arm = 'expansion' if request['params']['name'].endswith('value_dependency_expansion') else 'baseline'
            value, _ = c.unpack(reply)
            if value is None:
                continue
            for result in c.function_results(arm, value):
                source = ((result or {}).get('analysis') or {}).get('source') or {}
                node_id = source.get('node_id')
                if node_id and node_id not in retained:
                    retained[node_id], _ = c.retained_node(connection, node_id, files)
    compressed(packet/'proof/retained-nodes.json.gz', json.dumps(retained).encode())
    compressed(packet/'proof/graph-manifest.json.gz', (work/'data/graph/manifest.json').read_bytes())
    compressed(packet/'proof/dataset-identity.json.gz', (work/'data/dataset-identity.json').read_bytes())
    capture = read(work/'data/sources/manifest.json')
    originals = {r['path']: r for r in capture['files'] if r['origin_id'] == 'repo'}
    proof = {}
    for path in sorted({node['path'] for node in retained.values()}):
        file = originals[path]
        artifact = 'proof/files/'+file['sha256']+'.gz'
        compressed(packet/artifact, (work/'data/sources/blobs'/file['sha256']).read_bytes())
        proof[path] = dict(file_sha256=file['sha256'], artifact=artifact)
    write(packet/'proof/files.json', proof)


def derive(packet=None):
    p = packet or ROOT/PACKET
    scope = read(p/'SCOPE.json')
    if c.sha((p/'SCOPE.json').read_bytes()) != SCOPE_SHA or read(p/'actual/SCOPE.json.gz') != scope:
        raise ValueError('predeclared warm scope changed')
    if any(c.sha((ROOT/name).read_bytes()) != h for name, h in scope['tools_sha256'].items()):
        raise ValueError('frozen warm tool changed')
    original_scope = read(ROOT/prior.PACKET/'CORRECTIVE-SCOPE.json')
    pilot_pairs = read(ROOT/prior.PACKET/'actual/pairs.json.gz')
    expected_cases = [dict(case_id=r['case_id'], pins=r['pins'], requests=r['requests']) for r in pilot_pairs]
    if scope['cases'] != expected_cases:
        raise ValueError('warm cases differ from source-bound prior pilot')
    expected_inventory = scope['original_data_sha256']
    if expected_inventory != original_scope['original_data_sha256']:
        raise ValueError('dataset population changed')
    for phase in ('original-before', 'original-after', 'owned-before', 'owned-after'):
        if read(p/'actual'/(phase+'.json.gz')) != expected_inventory:
            raise ValueError('original/owned payload changed')
    receipt = read(p/'actual/receipt.json.gz')
    required = dict(planned_pairs=627, completed_pairs=627, tool_calls=1273, warmup_pairs=57,
                    measured_pairs=570, automatic_retries=0, binary_sha256=scope['binary_sha256'],
                    original_owned_payload_preserved=True, independent_approved=0, formal_completed=0,
                    formal_total=9, quality_scores=None, paid_or_external_LLM_calls=0,
                    embedding_inference_call_count=None)
    if receipt != required:
        raise ValueError('warm receipt incomplete or promoted')
    if read(ROOT/Path(original_scope['binary_proof']))['sha256'] != scope['binary_sha256']:
        raise ValueError('binary not bound to native source proof')
    session = read(p/'actual/session.json.gz')
    if session['exit_code'] != 0 or session['requests'] != 1275 or not session['reaped'] or not session['reader_joined'] or session['term_sent'] or session['kill_sent'] or session['remaining_live_group_members']:
        raise ValueError('actual MCP session not cleanly finished')
    driver = read(p/'driver/receipt.json.gz')
    if driver['exit_code'] != 0 or not driver['reaped'] or driver['timeout'] or driver['remaining_live_group_members']:
        raise ValueError('actual driver not cleanly finished')
    for stream in ('stdout', 'stderr'):
        if c.sha(gzip.decompress((p/'driver'/(stream+'.gz')).read_bytes())) != driver[stream+'_sha256']:
            raise ValueError('driver stream receipt changed')
    manifest = read(p/'proof/graph-manifest.json.gz')
    identity = read(p/'proof/dataset-identity.json.gz')
    for file, artifact in [('graph/manifest.json', 'graph-manifest.json'), ('dataset-identity.json', 'dataset-identity.json')]:
        if c.sha(gzip.decompress((p/'proof'/(artifact+'.gz')).read_bytes())) != expected_inventory[file]:
            raise ValueError('dataset proof differs')
    if identity['dataset_id'] != manifest['dataset_id'] or identity['source']['project_id'] != manifest['project_id'] or manifest['source_mode'] != 'committed' or manifest['src_commit'] != original_scope['source_commit']:
        raise ValueError('source/project/dataset proof differs')
    git_blobs = {r['path']: r['oid'] for r in read(ROOT/prior.old.BASE/'ticket-rpc-lifecycle-results-v1/actual/candidates/ORIGINAL/base-flat-tree.json')}
    retained, files = read(p/'proof/retained-nodes.json.gz'), read(p/'proof/files.json')
    for node in retained.values():
        proof = files[node['path']]
        blob = gzip.decompress((p/proof['artifact']).read_bytes())
        if c.sha(blob) != node['file_sha256'] or oid('blob', blob) != git_blobs[node['path']]:
            raise ValueError('returned file not exact original Git blob')
        body = blob[node['start_byte']:node['end_byte_exclusive']]
        if c.sha(body) != node['function_sha256'] or len(body) != node['bytes']:
            raise ValueError('returned full function differs')
    stdin = list(lines(p/'actual/stdin.jsonl.gz'))
    calls = list(lines(p/'actual/calls.jsonl.gz'))
    pairs = list(lines(p/'actual/pairs.jsonl.gz'))
    schedule = expected_schedule(scope['cases'])
    if len(calls) != 1273 or len(pairs) != 627 or [r['id'] for r in stdin] != list(range(1, 1276)):
        raise ValueError('request denominator differs')
    if [r for r in stdin if r['method'] == 'tools/call'] != [r['request'] for r in calls]:
        raise ValueError('call receipt differs from actual wire')
    if [dict((k, r[k]) for k in ('case_id', 'phase', 'index', 'order')) for r in pairs] != schedule:
        raise ValueError('predeclared pair/ABBA/warmup schedule differs')
    data = c.gold.derive(ROOT)
    drafts = {r['case_id']: r for r in data['gold']}
    case_by_id = {r['case_id']: r for r in scope['cases']}
    call_by_id = {r['request']['id']: r for r in calls}
    views = {}
    observed_ids = []
    for reply in lines(p/'actual/stdout.jsonl.gz'):
        rid = reply.get('id')
        if rid is None:
            raise ValueError('unexpected unbound notification in frozen response stream')
        observed_ids.append(rid)
        if rid not in call_by_id:
            continue
        call = call_by_id[rid]
        if type(call['elapsed_ns']) is not int or call['elapsed_ns'] <= 0:
            raise ValueError('invalid actual duration')
        case = case_by_id[call['case_id']]
        arm = call['arm']
        arguments = dict(case['requests'][arm])
        if call['phase'] == 'discovery_probe':
            arguments.pop('expected_snapshot_sha256')
        tool = 'cks.context.'+('value_dependencies' if arm == 'baseline' else 'value_dependency_expansion')
        if call['request']['params'] != dict(name=tool, arguments=arguments):
            raise ValueError('actual pin/limit/tool differs')
        value, _ = c.unpack(reply)
        if call['phase'] == 'discovery_probe':
            if not value or value.get('snapshot_sha256') != case['pins']['snapshot_sha256']:
                raise ValueError('actual warm probe snapshot differs')
            continue
        pins = dict(project_id=manifest['project_id'], dataset_id=manifest['dataset_id'],
                    commit=manifest['src_commit'], source_mode='committed',
                    graph_snapshot_sha256=case['pins']['snapshot_sha256'])
        view = c.view(arm, reply, retained, pins, drafts[call['case_id']], data['locations'])
        view['elapsed_ns'] = call['elapsed_ns']
        view['diagnostic_codes'] = sorted({d['code'] for result in c.function_results(arm, value or {})
            for d in ((result or {}).get('analysis') or {}).get('diagnostics', [])}) if value else []
        views[rid] = view
    if observed_ids != list(range(1, 1276)):
        raise ValueError('missing/duplicate/reordered actual response')
    observations = []
    expected_call_order = []
    for case in scope['cases']:
        cid = case['case_id']
        expected_call_order.append((cid, 'discovery_probe', None, 'baseline'))
        for row in schedule:
            if row['case_id'] == cid:
                expected_call_order.extend((cid, row['phase'], row['index'], arm) for arm in row['order'])
    if [(r['case_id'], r['phase'], r['index'], r['arm']) for r in calls] != expected_call_order:
        raise ValueError('actual call scheduling differs')
    for pair in pairs:
        arms = {}
        for arm in pair['order']:
            meta = pair['arms'][arm]
            rid = meta['request_id']
            call = call_by_id.get(rid, {})
            if any(call.get(k) != pair[k] for k in ('case_id', 'phase', 'index')) or call.get('arm') != arm or meta['elapsed_ns'] != call['elapsed_ns'] or meta['status'] != views[rid]['status']:
                raise ValueError('pair receipt does not match actual response')
            arms[arm] = views[rid]
        observations.append(dict(case_id=pair['case_id'], phase=pair['phase'], index=pair['index'],
                                 order=pair['order'], arms=arms,
                                 complete_pair=all(a['complete'] for a in arms.values())))
    measured = [r for r in observations if r['phase'] == 'measured']
    before, after = [read(p/'actual'/('environment-'+phase+'.json.gz')) for phase in ('before', 'after')]
    if before['resource_exclusivity_proved'] or after['resource_exclusivity_proved']:
        raise ValueError('resource exclusivity not established by this harness')
    summary = dict(kind=scope['kind'], planned_pairs=627, recorded_pairs=len(observations),
                   warmup_pairs=57, measured_pairs=len(measured), arm_calls=1254, probe_calls=19,
                   tool_calls=1273, protocol_requests=1275, sessions=1, concurrency=1,
                   all_session_and_driver_groups_remaining=0, automatic_retries=0,
                   retained_node_proofs=len(retained), original_owned_payload_preserved=True,
                   status_counts={arm: dict(Counter(r['arms'][arm]['status'] for r in measured)) for arm in ('baseline', 'expansion')},
                   expansion_stop_reasons=dict(Counter(reason for r in measured for reason in r['arms']['expansion'].get('stop_reasons', []))),
                   diagnostic_calls_by_code={arm: dict(Counter(code for r in measured for code in r['arms'][arm]['diagnostic_codes'])) for arm in ('baseline', 'expansion')},
                   latency=latency(observations), per_case_latency={case['case_id']: latency([r for r in observations if r['case_id'] == case['case_id']]) for case in scope['cases']},
                   source_identity_verified_slots={arm: sum(s['identity_check'] == 'verified_retained_identity' for r in measured for s in r['arms'][arm]['raw_slots']) for arm in ('baseline', 'expansion')},
                   raw_function_slots={arm: sum(len(r['arms'][arm]['raw_slots']) for r in measured) for arm in ('baseline', 'expansion')},
                   seed_slots={arm: sum(s['seed'] for r in measured for s in r['arms'][arm]['raw_slots']) for arm in ('baseline', 'expansion')},
                   verified_outside_gold_slots={arm: sum(s['identity_check'] == 'verified_retained_identity' and s['AI_label'] == 'outside_unknown' for r in measured for s in r['arms'][arm]['raw_slots']) for arm in ('baseline', 'expansion')},
                   unresolved_identity_slots={arm: sum(s['identity_check'] != 'verified_retained_identity' for r in measured for s in r['arms'][arm]['raw_slots']) for arm in ('baseline', 'expansion')},
                   model_metadata_stable=before['local_embedding_runtime_readonly_metadata'] == after['local_embedding_runtime_readonly_metadata'],
                   hardware_metadata_stable=all(before[k] == after[k] for k in ('platform', 'machine', 'processor', 'cpu_count', 'clock')),
                   resource_exclusivity_proved=False, timing_includes_capture_and_json_parse=True,
                   runtime_embedding_inference_calls=None, paid_or_external_LLM_calls=0,
                   independent_approved=0, formal_completed=0, formal_total=9, FINAL=0,
                   D04='미정', freeze=None, execution_allowed=False, quality_score=None,
                   installed_agent_path_proved=False)
    return dict(summary=summary, observations=observations)


def seal(p):
    write(p/'manifest.json', dict(previous_revision=REVISION, kind='WI6_warm_DEV_only',
          tools={n: c.sha((ROOT/n).read_bytes()) for n in TOOLS},
          artifacts={str(x.relative_to(p)): c.sha(x.read_bytes()) for x in sorted(p.rglob('*')) if x.is_file() and x.name != 'manifest.json'}))


def prepare(work, driver):
    p = ROOT/PACKET
    if (p/'actual').exists():
        raise ValueError('refusing raw capture overwrite')
    for source, label in [(work, 'actual'), (driver, 'driver')]:
        for x in sorted(source.iterdir()):
            if x.is_file():
                compressed(p/label/(x.name+'.gz'), x.read_bytes())
    collect_proof(work, p)
    data = derive(p)
    write(p/'SUMMARY.json', data['summary'])
    write(p/'observations.json', data['observations'])
    seal(p)


def validate(packet=None):
    p = packet or ROOT/PACKET
    m = read(p/'manifest.json')
    if m != dict(previous_revision=REVISION, kind='WI6_warm_DEV_only',
                 tools={n: c.sha((ROOT/n).read_bytes()) for n in TOOLS},
                 artifacts={str(x.relative_to(p)): c.sha(x.read_bytes()) for x in p.rglob('*') if x.is_file() and x.name != 'manifest.json'}):
        raise ValueError('warm evidence inventory changed')
    data = derive(p)
    if read(p/'SUMMARY.json') != data['summary'] or read(p/'observations.json') != data['observations']:
        raise ValueError('warm replay changed or falsely promoted')
    return data


def stored():
    revision = subprocess.check_output(['git', 'log', '-1', '--format=%H', '--', str(PACKET/'manifest.json')], cwd=ROOT, text=True).strip()
    if not re.fullmatch('[0-9a-f]{40}', revision):
        raise ValueError('warm packet not committed')
    m = read(ROOT/PACKET/'manifest.json')
    paths = [PACKET/'manifest.json', *[PACKET/n for n in m['artifacts']], *map(Path, TOOLS),
             Path('docs/REMAINING-WORK.md'), Path('docs/spec-driven/REMAINING-13-EXECUTION.json')]
    if any(subprocess.check_output(['git', 'show', revision+':'+str(p)], cwd=ROOT) != (ROOT/p).read_bytes() for p in paths):
        raise ValueError('stored warm evidence/tools/ledger differs')
    result = subprocess.run(['git', 'verify-commit', revision], cwd=ROOT, capture_output=True)
    if result.returncode:
        raise ValueError('warm commit signature not verified')
    return revision


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('command', choices=['prepare', 'check'])
    parser.add_argument('--workspace', type=Path)
    parser.add_argument('--driver', type=Path)
    parser.add_argument('--require-formal', action='store_true')
    parser.add_argument('--require-stored', action='store_true')
    args = parser.parse_args()
    if args.command == 'prepare':
        prepare(args.workspace.resolve(), args.driver.resolve())
    else:
        result = validate()
        if args.require_stored:
            stored()
        if args.require_formal:
            raise SystemExit('formal acceptance unavailable: anonymous independent review/freeze missing')
        print(json.dumps(result['summary'], ensure_ascii=False, indent=2))
