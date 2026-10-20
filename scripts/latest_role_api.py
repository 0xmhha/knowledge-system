"""Verify latest CKS role delivery and declared API errors without accepting gold."""
import gzip
import hashlib
import json
import subprocess
from pathlib import Path
import four_role_readiness as roles
from ticket_actual_rpc import flat_map, tree_oid
from source_packet import oid

ROOT = Path(__file__).resolve().parents[1]
PACKET = Path('docs/spec-driven/operational-preparation/source-review-supplement/followups/latest-four-role-api-v1')
REVISION = 'b8e3488e302e30efb21128032df3b8a9b4607528'
TOOLS = ['scripts/latest_role_api.py', 'scripts/test_latest_role_api.py', 'scripts/four_role_readiness.py', 'scripts/consumer-role-host.py', 'scripts/run-four-role-current.py', 'scripts/run-wi6-pair-pilot.py', 'scripts/run-full-host-owner.py', 'scripts/full_host_owner.py', 'scripts/full_host_factory.py', 'scripts/ticket_actual_rpc.py', 'scripts/source_packet.py', 'scripts/owner_schedule_contract.py', 'scripts/viewer_readiness.py', 'scripts/native_reliability.py', 'scripts/wi6_pair_contract.py']


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def read(p):
    raw = p.read_bytes()
    return json.loads(gzip.decompress(raw) if p.suffix == '.gz' else raw)


def require(value, message):
    if not value:
        raise ValueError(message)


def derive(p):
    spec = read(p/'SPEC.json')
    require(spec['source_revision'] == REVISION and spec['independent_approved'] == 0 and spec['execution_allowed'] is False, 'source/acceptance scope changed')
    binary = read(p/'native/binary.json.gz')
    binding = read(p/'native/source-binding.json.gz')
    info = gzip.decompress((p/'native/build-info.txt.gz').read_bytes()).decode()
    require(binary['source_revision'] == binding['source_revision'] == REVISION and binary['clean_source'] and binding['current_clean'], 'latest clean source binding differs')
    require('vcs.revision='+REVISION in info and 'vcs.modified=false' in info and 'go1.26.8' in info, 'fresh binary build identity differs')
    require(binary['source_binding_sha256'] == sha(gzip.decompress((p/'native/source-binding.json.gz').read_bytes())), 'native source binding receipt differs')
    for values in binding['input_maps'].values():
        for n, digest in values.items():
            require(sha((ROOT/n).read_bytes()) == digest, 'compiled input changed: '+n)
    build = read(p/'native/build/receipt.json.gz')
    require(build['exit_code'] == 0 and build['reaped'] and not any(build[k] for k in ('timeout', 'term_sent', 'kill_sent', 'remaining_live_group_members')), 'fresh CKS build or cleanup failed')
    for stream in ('stdout', 'stderr'):
        require(sha(gzip.decompress((p/('native/build/'+stream+'.gz')).read_bytes())) == build[stream+'_sha256'], 'build stream differs')
    scope = read(p/'roles/evidence/SCOPE.json.gz')
    require(scope['binary'] == binary and scope['actual_LLM_inference'] is False and scope['paid_or_external_LLM_calls'] == 0, 'role binary or model scope changed')
    actual = roles.inspect_attempt(p/'roles')
    require(len(actual) == 4 and all(r['completion_refused'] for r in actual), 'actual role delivery incomplete')
    native_rpc = read(p/'rpc/SCOPE.json.gz')
    require(native_rpc['source_commit'] == roles.owner.factory.COMMIT and native_rpc['source_instrumentation'] is False and native_rpc['negative_cases'] == 12, 'RPC source/scope differs')
    require(native_rpc['planned_slots'] == [r+'-'+v for r in roles.owner.ROLES for v in ('general', 'race')], 'RPC candidate/profile population differs')
    attempts = read(p/'rpc/attempts.json.gz')
    require([r['slot'] for r in attempts] == native_rpc['planned_slots'], 'RPC attempt population differs')
    codes = {f'RPC-N{i:02d}': -32602 for i in range(1, 13)}
    codes.update({'RPC-N01': -32601, 'RPC-N05': -32601, 'RPC-N06': -32601, 'RPC-N08': -32000, 'RPC-N11': -32000})
    observed = 0
    for attempt in attempts:
        q = p/'rpc/slots'/attempt['slot']
        receipt = read(q/'receipt.json.gz')
        require(receipt == attempt and receipt['exit_code'] == 0 and receipt['reaped'] and not any(receipt[k] for k in ('timeout', 'term_sent', 'kill_sent', 'remaining_live_group_members', 'tmp_children_after')), 'RPC process outcome/cleanup differs')
        process = read(q/'process/receipt.json.gz')
        for stream in ('stdout', 'stderr'):
            require(sha(gzip.decompress((q/('process/'+stream+'.gz')).read_bytes())) == process[stream+'_sha256'], 'RPC raw stream differs')
        rows = [json.loads(line) for line in gzip.decompress((q/'process/stdout.gz').read_bytes()).splitlines()]
        negatives = [r['value'] for r in rows if r['event'] == 'negative']
        require(len(negatives) == 12 and {r['id'] for r in negatives} == set(codes), 'RPC negative case population incomplete')
        require(all(r['code'] == codes[r['id']] and r['message'] and r['classification'] == 'EXPECTED_API_ERROR_not_symptom_RED' for r in negatives), 'API error classified as RED or wrong result')
        require([r['event'] for r in rows].count('positive_registry_control') == 1 and [r['event'] for r in rows].count('post_error_health') == 1 and rows[-2:] == [{'event': 'owned_node_closed', 'value': True}, {'event': 'complete', 'value': {'negative_cases': 12, 'independent_approved': 0, 'formal_completed': 0, 'native_timer_not_exercised': True}}], 'RPC positive control/close missing')
        observed += len(negatives)
    require(read(p/'rpc/original-before.json.gz') == read(p/'rpc/original-after.json.gz') and gzip.decompress((p/'rpc/original-status-before.txt.gz').read_bytes()) == gzip.decompress((p/'rpc/original-status-after.txt.gz').read_bytes()), 'original upstream source changed')
    for role in native_rpc['unique_build_roles']:
        q = p/'rpc/candidates'/role
        b = read(q/'source-binding.json.gz')
        require(b['source_commit'] == native_rpc['source_commit'] and b['source_instrumentation'] is False and b['production_patch'] is False, 'RPC candidate source scope changed')
        overlay = (ROOT/roles.owner.PACKET.parent/'ticket-dev-gold-v1/private/overlays'/(role+'.go.txt')).read_bytes()
        require(sha(overlay) == b['api_sha256'] == sha(gzip.decompress((q/'api.go.gz').read_bytes())), 'candidate method overlay changed')
        trees = read(q/'tree-binding.json.gz')
        base = json.loads((ROOT/roles.owner.PACKET.parent/'ticket-method-schedules-v1/actual/candidates/ORIGINAL/base-flat-tree.json').read_bytes())
        project = flat_map(base)
        project['eth/downloader/api.go'] = dict(mode='100644', kind='blob', oid=oid('blob', overlay))
        fixture = gzip.decompress((p/'drivers/phase89-api-negative-main.go.gz').read_bytes())
        main = 'cmd/ks-owned-negative-matrix/main.go'
        build_tree = dict(project)
        build_tree[main] = dict(mode='100644', kind='blob', oid=oid('blob', fixture))
        require(b['additions'] == {main: sha(fixture)} and b['base_tracked'] == read(p/'rpc/original-before.json.gz'), 'RPC source/addition population differs')
        expected = dict(b['base_tracked'])
        expected['eth/downloader/api.go'] = dict(mode=expected['eth/downloader/api.go']['mode'], sha256=sha(overlay))
        require(b['applied_tracked'] == expected and flat_map(trees['project_flat_tree']) == project and flat_map(trees['build_flat_tree']) == build_tree, 'RPC Git tree or source population differs')
        require(tree_oid(trees['project_flat_tree']) == trees['applied_project_tree'] and tree_oid(trees['build_flat_tree']) == trees['build_input_tree'], 'RPC Git tree identities differ')
        binaries = read(q/'binaries.json.gz')
        for variant in ('general', 'race'):
            slot = next(r for r in attempts if r['slot'] == role+'-'+variant)
            require(slot['binary_sha256'] == binaries[variant]['sha256'], 'RPC executed binary differs')
            build = read(q/('build-'+variant)/'receipt.json.gz')
            require(build['exit_code'] == 0 and not build['timeout'] and build['reaped'] and not build['remaining_live_group_members'], 'RPC candidate build failed')
            expected_argv = ['/Users/kevin/.gvm/gos/go1.23.12/bin/go', 'build'] + (['-race'] if variant == 'race' else []) + ['-o', binaries[variant]['path'], './cmd/ks-owned-negative-matrix']
            require(build['argv'] == expected_argv, 'RPC declared build profile differs')
            metadata = gzip.decompress((q/(variant+'-build-info.txt.gz')).read_bytes()).decode()
            require('go1.23.12' in metadata and 'GOOS=darwin' in metadata and 'GOARCH=arm64' in metadata and 'vcs.revision='+native_rpc['source_commit'] in metadata and (variant != 'race' or '-race=true' in metadata), 'RPC actual build identity differs')
    for variant in ('general', 'race'):
        original = next(r for r in attempts if r['slot'] == 'ORIGINAL-'+variant)
        alias = next(r for r in attempts if r['slot'] == 'MUT-REINTRO-'+variant)
        require(original['binary_sha256'] == alias['binary_sha256'], 'original alias counted as new source')
    mcp_scope = read(p/'mcp/SCOPE.json.gz')
    records = read(p/'mcp/results.json.gz')
    require(len(mcp_scope['cases']) == mcp_scope['negative_cases'] == 35 and len(records) == 37, 'MCP declared case population incomplete')
    raw = {r['id']: r for r in [json.loads(line) for line in gzip.decompress((p/'mcp/stdout.jsonl.gz').read_bytes()).splitlines()] if 'id' in r}
    for case, record in zip(mcp_scope['cases'], records):
        require(record['case_id'] == case['id'] and record['request']['params'] == {'name': case['tool'], 'arguments': case['args']} and record['raw_response'] == raw[record['request']['id']], 'MCP raw request/response differs')
        wire = record['raw_response']; result = wire.get('result', {})
        error = bool(wire.get('error') or result.get('isError'))
        value = result.get('structuredContent', {})
        require(error if case['expected'] in ('tool_error', 'protocol_or_tool_error') else not error and value.get('status') == case['expected'], 'MCP expected error/status differs')
        require(record['classification'] == 'EXPECTED_API_ERROR_OR_UNCERTAIN_STATUS_not_symptom_RED', 'MCP error promoted to symptom RED')
    for record in records[35:]:
        wire = record['raw_response']; result = wire['result']; value = result.get('structuredContent') or json.loads(result['content'][0]['text'])
        require(wire == raw[record['request']['id']] and not result.get('isError') and value['status'] == 'partial' and value['analysis']['source']['sha256'] == scope['source_pins']['function_sha256'], 'post-error partial/provenance changed')
    session = read(p/'mcp/session.json.gz')
    require(session['exit_code'] == 0 and session['reader_joined'] and not any(session[k] for k in ('term_sent', 'kill_sent', 'remaining_live_group_members')), 'MCP cleanup differs')
    return dict(kind='latest_CKS_actual_four_role_and_declared_API_errors_DEV_not_independent_acceptance',source_revision=REVISION,actual_roles=4,actual_CKS_role_sessions=4,partial_MCP_calls=2,context_writes=2,downstream_Reads=3,completion_refusals=4,selection_refusals=2,rpc_nodes=10,rpc_negative_cases=observed,rpc_unique_source_variants=4,mcp_negative_or_uncertain_cases=35,mcp_post_error_partial_controls=2,new_actual_LLM_calls=0,paid_calls=0,full12_requirement_acceptance=False,installed_positive_WIV1_oracle=False,formal_completed=0,independent_approved=0,FINAL=0,freeze=None,execution_allowed=False)


def seal(p):
    (p/'manifest.json').write_text(json.dumps(dict(tools={n: sha((ROOT/n).read_bytes()) for n in TOOLS}, artifacts={str(x.relative_to(p)): sha(x.read_bytes()) for x in sorted(p.rglob('*')) if x.is_file() and x.name != 'manifest.json'}), indent=2)+'\n')


def check(p=None):
    p = p or ROOT/PACKET
    try:
        m = read(p/'manifest.json')
        require(m['tools'] == {n: sha((ROOT/n).read_bytes()) for n in TOOLS}, 'review verification tool changed')
        require(m['artifacts'] == {str(x.relative_to(p)): sha(x.read_bytes()) for x in p.rglob('*') if x.is_file() and x.name != 'manifest.json'}, 'review packet bytes changed')
        require(read(p/'SUMMARY.json') == derive(p), 'derived role/API result or acceptance boundary changed')
        return []
    except (OSError, ValueError, KeyError, TypeError, IndexError, StopIteration) as error:
        return [str(error)]


if __name__ == '__main__':
    errors = check()
    if errors:
        raise SystemExit('\n'.join(errors))
    print('latest roles4; RPC negative120; MCP negative/status35+partial2; independent/formal0')
