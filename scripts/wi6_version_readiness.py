"""Read-only historical/current WI-6 checks; never runtime or gold approval."""
import argparse
import gzip
import hashlib
import json
import subprocess
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
BASE = Path('docs/spec-driven/operational-preparation/source-review-supplement/followups')
HISTORY = BASE / 'wi6-dev-gold-v1'
HISTORY_REVISION = 'fc4186b9997caa8aa432cc2dab6fb3fbe6ff703f'
VERIFIED_REVISION = 'f5d7da318e92f3dbbb024f396895b99033d1383d'
CURRENT = BASE / 'ticket-rpc-overlap-v1'
OUTPUT = BASE / 'wi6-version-readiness-v1'
DOCS = [Path('docs/spec-driven') / ('OPERATIONAL-ACCEPTANCE-' + s + '.json')
        for s in ('PROTOCOL', 'PROGRESS', 'DECISION-PROPOSAL')]
DOCS += [BASE.parent / 'GAPS.json']
LOCAL_LIMITS = dict(max_source_bytes=262144, max_ast_nodes=4096, max_blocks=256,
                    max_nodes=1024, max_edges=4096, max_iterations=128,
                    max_facts=65536, timeout_ms=1000)
EXPANSION_LIMITS = dict(max_functions=4, max_hops=1, max_candidates=32,
                       max_total_nodes=2048, max_total_edges=8192,
                       max_total_facts=131072, timeout_ms=1000)


def sha(data):
    return hashlib.sha256(data).hexdigest()


def read_at(root, revision, path):
    return subprocess.check_output(['git', 'show', revision + ':' + str(path)], cwd=root)


def load(root, path):
    return json.loads((root / path).read_bytes())


def bounded_path(root, name):
    p = Path(name)
    if p.is_absolute() or '..' in p.parts or p.as_posix() != name or not p.parts:
        raise ValueError('noncanonical evidence path')
    result = root / p
    if not result.resolve().is_relative_to(root.resolve()):
        raise ValueError('evidence path leaves packet')
    return result


def snapshot(root, packet, revision, reader=read_at):
    """Compare artifacts with their frozen manifest, tools at their own revision."""
    raw = (root / packet / 'manifest.json').read_bytes()
    if raw != reader(root, revision, packet / 'manifest.json'):
        raise ValueError('manifest differs from historical Git revision')
    manifest = json.loads(raw)
    actual = {p.relative_to(root / packet).as_posix(): sha(bounded_path(root / packet, p.relative_to(root / packet).as_posix()).read_bytes())
              for p in (root / packet).rglob('*') if p.is_file() and p != root / packet / 'manifest.json'}
    if actual != manifest['artifacts']:
        raise ValueError('evidence inventory/hash differs')
    for name in manifest['artifacts']:
        bounded_path(root / packet, name)
    differences = []
    names = [t['path'] for t in manifest['tools']]
    if len(names) != len(set(names)):
        raise ValueError('duplicate bound tool')
    for tool in manifest['tools']:
        current = bounded_path(root, tool['path'])
        if sha(reader(root, revision, tool['path'])) != tool['sha256']:
            raise ValueError('historical tool does not match its frozen digest')
        if sha(current.read_bytes()) != tool['sha256']:
            differences.append(tool['path'])
    return dict(revision=revision, manifest_sha256=sha(raw), artifacts=len(actual),
                tools=len(names), current_tool_differences=differences,
                historical_bytes_verified=True, runtime_reexecuted=False)


def plan(inputs):
    if len(inputs) != 20 or len({r['case_id'] for r in inputs}) != 20:
        raise ValueError('expected exposed DEV population20')
    cases, excluded = [], []
    for row in inputs:
        if row['split'] != 'DEV' or row['approved'] is not False:
            raise ValueError('DEV proposal relabelled or approved')
        if row['scope'] == 'development_stress_source_only':
            excluded.append(row['case_id'])
            continue
        if row['scope'] != 'production_source_candidate':
            raise ValueError('undeclared input scope')
        cases.append(dict(case_id=row['case_id'], source=row['source'],
                          selected_node=None, project_id=None, dataset_id=None,
                          snapshot_sha256=None, resolved=False,
                          actual_baseline_request=None, actual_expansion_request=None))
    if len(cases) != 19 or len(excluded) != 1:
        raise ValueError('production/source-only population differs')
    return dict(status='unfrozen_DEV_proposal_no_execution', cases=cases,
                excluded_source_only=excluded, per_function_limits=LOCAL_LIMITS,
                expansion_limits=EXPANSION_LIMITS,
                baseline_tool='cks.context.value_dependencies',
                expansion_tool='cks.context.value_dependency_expansion',
                discovery='exact retained file/function SHA, commit, coordinates and node; no name-only joining',
                request_shape='baseline=function; expansion={function:same_baseline,limits:expansion_limits}',
                warmup_pairs_per_case=3, measured_pairs_per_case=30,
                phase_order='warmup3 then measured30; AB/BA alternation restarts per phase',
                first_order='AB', planned_pairs=19 * 33, planned_tool_calls=19 * 33 * 2,
                discovery_calls_included=False, actual_pairs=0, actual_tool_calls=0,
                actual_warm_samples=0, single_concurrency=True, connection_reused=True,
                measurement='client perf_counter_ns around each complete tool call; retain all raw slots/status/errors',
                latency='nearest-rank p95 per arm, ratio of p95s; undefined/insufficient remains inconclusive',
                semantic_scope='outgoing call hops between function-local analyses; no argument/return/memory transfer proof',
                relevance='existing bounded AI20-function universe; seed excluded; outside unknown; no automatic noise',
                gold_independent=False, formal_scoring_allowed=False,
                formal_completed=0, FINAL=0, freeze=None, execution_allowed=False)


def requests(resolution):
    """Build paired arguments only after exact retained discovery, not from names."""
    keys = {'node_id', 'project_id', 'dataset_id', 'commit', 'function_sha256', 'snapshot_sha256'}
    if set(resolution) != keys or any(not isinstance(v, str) or not v for v in resolution.values()):
        raise ValueError('complete independently retained discovery pins required')
    if len(resolution['node_id']) > 128:
        raise ValueError('node identity exceeds actual API bound')
    for key, size in [('commit', 40), ('function_sha256', 64), ('snapshot_sha256', 64)]:
        value = resolution[key]
        if len(value) != size or any(c not in '0123456789abcdef' for c in value):
            raise ValueError('invalid retained pin')
    function = dict(node_id=resolution['node_id'], expected_commit=resolution['commit'],
                    expected_source_sha256=resolution['function_sha256'],
                    expected_snapshot_sha256=resolution['snapshot_sha256'], limits=dict(LOCAL_LIMITS))
    # Project/dataset are caller/session pins, not invented MCP input fields.
    return dict(session_pins={k: resolution[k] for k in ('project_id', 'dataset_id')},
                baseline=function,
                expansion=dict(function=dict(function, limits=dict(LOCAL_LIMITS)), limits=dict(EXPANSION_LIMITS)))


def readiness(root=ROOT):
    history = snapshot(root, HISTORY, HISTORY_REVISION)
    current = snapshot(root, CURRENT, VERIFIED_REVISION)
    if current['current_tool_differences']:
        raise ValueError('latest verified runtime tools changed')
    for p in DOCS:
        if (root / p).read_bytes() != read_at(root, VERIFIED_REVISION, p):
            raise ValueError('active protocol changed from verified phase67')
    protocol = load(root, DOCS[0])
    if protocol['freeze'] is not None or protocol['execution_allowed'] is not False or any(protocol['roles'].values()):
        raise ValueError('unexecuted preparation cannot freeze/assign/approve')
    ref = protocol['wi6_dev_gold_preparation']
    if ref['historical_source_revision'] != HISTORY_REVISION or ref['sha256'] != history['manifest_sha256']:
        raise ValueError('historical WI6 reference differs')
    reuse = load(root, CURRENT / 'native-reuse.json')
    binding = load(root, Path(reuse['source_binding']))
    if sha((root / reuse['source_binding']).read_bytes()) != reuse['source_binding_sha256']:
        raise ValueError('latest native binding differs')
    native = {**binding['source_sha256'], **binding['approved_input_sha256']}
    if any(sha(bounded_path(root, p).read_bytes()) != h for p, h in native.items()):
        raise ValueError('current native source/approved input differs from latest verified binding')
    # Re-derive source-grounded gold without running target code, a model or MCP.
    import wi6_dev_gold as old
    data = old.derive(root, root / HISTORY / 'private/source')
    for name, key in [('public/DEV-inputs.json', 'public'), ('private/location-catalog.json', 'locations'), ('SUMMARY.json', 'summary')]:
        if load(root, HISTORY / name) != data[key]:
            raise ValueError('source-grounded historical derivation differs')
    if json.loads(gzip.decompress((root / HISTORY / 'private/AI-gold.json.gz').read_bytes())) != data['gold']:
        raise ValueError('source-grounded complete bounded gold differs')
    return dict(status='version_aware_preparation_verified_not_runtime', history=history,
                latest_verified_runtime=current,
                current_native_source_files=len(binding['source_sha256']),
                current_approved_inputs=len(binding['approved_input_sha256']),
                native_reuse_checked=True, new_native_runs=0,
                source_questions=20, bounded_AI_units=400,
                seed_excluded_defined=data['summary']['seed_excluded_positive_cases'],
                seed_excluded_undefined=data['summary']['seed_excluded_undefined_cases'],
                actual_new_MCP_calls=0, actual_new_pairs=0, actual_warm_samples=0,
                formal_completed=0, formal_total=9, independent_approved=0,
                D04='미정', FINAL=0, freeze=None, execution_allowed=False)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--require-runtime', action='store_true')
    args = parser.parse_args()
    try:
        result = readiness()
        if args.require_runtime:
            raise ValueError('actual WI6 pairs/warm observations and independent frozen gold remain unavailable')
        print(json.dumps(result, ensure_ascii=False, indent=2))
    except (OSError, ValueError, KeyError, TypeError, subprocess.CalledProcessError) as exc:
        print(json.dumps({'error': str(exc), 'runtime_complete': False}, ensure_ascii=False))
        return 1
    return 0


if __name__ == '__main__':
    raise SystemExit(main())
