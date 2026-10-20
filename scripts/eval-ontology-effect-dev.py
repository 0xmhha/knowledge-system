#!/usr/bin/env python3
"""Source-bound, descriptive four-arm ontology evaluation; no human quality approval."""
import argparse
import copy
import hashlib
import importlib.util
import json
from pathlib import Path
import subprocess
import time
import urllib.request

import yaml

ROOT = Path(__file__).resolve().parents[1]
ARMS = ['off', 'concept_text', 'relations', 'combined']
COUNTS = {'warmup': 1, 'retrieval': 2, 'warm_latency': 5, 'cold_process': 0}
REFS = {
    'OE-D01': [('consensus/wbft/validator/default.go', 222, 229)],
    'OE-D02': [('consensus/wbft/validator/default.go', 222, 229)],
    'OE-D03': [('consensus/wbft/core/roundchange.go', 103, 212)],
    'OE-D04': [('consensus/wbft/core/roundchange.go', 103, 212)],
    'OE-D05': [('consensus/wbft/core/preprepare.go', 113, 150), ('consensus/wbft/core/prepare.go', 87, 125), ('consensus/wbft/core/commit.go', 90, 130)],
    'OE-D06': [('consensus/wbft/core/qbft_msg_set.go', 66, 71)],
    'OE-D07': [('consensus/wbft/core/core.go', 154, 164)],
    'OE-D08': [('consensus/wbft/core/core.go', 154, 164)],
}

def sha(raw):
    return hashlib.sha256(raw).hexdigest()

def write(path, value):
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2) + '\n')

def percentile(values, q):
    ordered = sorted(values)
    pos = (len(ordered) - 1) * q
    low = int(pos)
    return ordered[low] + (ordered[min(low + 1, len(ordered) - 1)] - ordered[low]) * (pos - low)

def main():
    ap = argparse.ArgumentParser(description=__doc__)
    for flag in ('version-dir', 'source-repo', 'cks-binary', 'out'):
        ap.add_argument('--' + flag, type=Path, required=True)
    args = ap.parse_args()
    version, repo, binary, out = args.version_dir.resolve(), args.source_repo.resolve(), args.cks_binary.resolve(), args.out.resolve()
    if out.exists():
        raise ValueError('fresh output required')
    out.mkdir(parents=True)
    identity = json.loads((version / 'dataset-identity.json').read_text())
    source_manifest = json.loads((version / 'sources/manifest.json').read_text())
    files = {(r['origin_id'], r['path']): r for r in source_manifest['files']}
    baseline = json.loads((ROOT / 'docs/spec-driven/operational-preparation/policy-security-drafts/manifest.json').read_text())
    for row in baseline['files']:
        record = files[('repo', row['path'])]
        if record['sha256'] != row['sha256'] or sha((version / 'sources/blobs' / record['sha256']).read_bytes()) != row['sha256']:
            raise ValueError('Go source differs from pinned baseline')
    delta = subprocess.check_output(['git', '-C', str(repo), 'diff', '--name-status', baseline['source_commit'], identity['source']['source_commit']], text=True)
    if delta.splitlines() != ['A\tdocs/approved-meaning/ontology.yaml', 'A\tdocs/approved-meaning/spec.yaml']:
        raise ValueError('candidate source differs beyond approved meaning files')
    ontology = files[('repo', 'docs/approved-meaning/ontology.yaml')]
    approved = ROOT / 'projects/stablenet/domain-knowledge/meaning-approved/ontology.yaml'
    if ontology['sha256'] != sha(approved.read_bytes()):
        raise ValueError('approved ontology bytes differ')
    original_paths = [version / 'graph/graph.db', version / 'vector/vector.db', version / 'graph/manifest.json', version / 'vector/manifest.json', version / 'dataset-identity.json', version / 'sources/manifest.json', version / 'reader-protocol.json', version / 'review-hold.json']
    preserved = {str(p): sha(p.read_bytes()) for p in original_paths}
    receipts = []
    started = time.monotonic()
    def run(label, command, timeout=180):
        if time.monotonic() - started > 1800:
            raise ValueError('development wall-clock budget exhausted')
        t = time.monotonic()
        result = subprocess.run([str(x) for x in command], cwd=ROOT, capture_output=True, timeout=timeout)
        (out / (label + '.stdout.txt')).write_bytes(result.stdout)
        (out / (label + '.stderr.txt')).write_bytes(result.stderr)
        receipts.append({'label': label, 'command': [str(x) for x in command], 'exit_code': result.returncode, 'elapsed_seconds': time.monotonic() - t})
        write(out / 'commands.json', receipts)
        result.check_returncode()
        return result
    def model():
        with urllib.request.urlopen('http://127.0.0.1:11434/api/tags', timeout=10) as response:
            rows = json.load(response)['models']
        selected = next(r for r in rows if r['name'] == 'bge-m3:latest')
        if selected['digest'] != '7907646426070047a77226ac3e684fbbe8410524f7b4a74d02837e43f2146bab':
            raise ValueError('local model digest differs')
        return {'name': selected['name'], 'digest': selected['digest']}
    write(out / 'model-before.json', model())
    store, projection = out / 'semantic.sqlite', out / 'projection.json'
    run('semantic-build', [binary, 'semantic', 'build', '--repo', repo, '--project-id', identity['source']['project_id'],
        '--dataset-id', identity['dataset_id'], '--graph', version / 'graph', '--vector', version / 'vector',
        '--version-dir', version, '--store', store, '--out', projection, '--ontology', 'docs/approved-meaning/ontology.yaml'])
    projected = json.loads(projection.read_text())
    if len(projected['concepts']) != 4 or projected.get('assertions'):
        raise ValueError('unexpected concept/implementation link count')
    store_sha = sha(store.read_bytes())
    public = json.loads((ROOT / 'docs/spec-driven/operational-preparation/ontology-effect-dev/requests.json').read_text())
    reference_rows = []
    for qid, spans in REFS.items():
        references = []
        for path, first, last in spans:
            f = files[('repo', path)]
            raw = (version / 'sources/blobs' / f['sha256']).read_bytes()
            references.append({'file': path, 'start_line': first, 'end_line': last, 'file_sha256': f['sha256'],
                'span_sha256': sha(b''.join(raw.splitlines(keepends=True)[first - 1:last]))})
        reference_rows.append({'id': qid, 'references': references})
    write(out / 'private-location-references.json', {'kind': 'AI_source_location_proxies_not_exhaustive_gold', 'independent_approval': False, 'cases': reference_rows})
    configs = {}
    normalized = []
    for arm in ARMS:
        config = out / (arm + '.yaml')
        run('config-' + arm, [binary, 'mcp', 'gen-config', '--dataset-dir', version, '--source-root', repo,
            '--name', 'ontology-effect-development', '--semantic-store', store, '--ontology-mode', arm,
            '--ontology-budget-ms', '5000', '--embed-model', 'bge-m3:latest', '--embed-dim', '1024',
            '--ollama-url', 'http://127.0.0.1:11434', '--sanitize-rules', ROOT / 'system/policies/sanitization_rules.yaml', '--out', config])
        cfg = yaml.safe_load(config.read_text())
        cfg['listen']['transport'] = 'stdio'
        cfg['listen']['mcp_stdio'] = True
        cfg['backends']['ckv']['provider'] = 'ollama'
        cfg['logging']['level'] = 'error'
        config.write_text(yaml.safe_dump(cfg, sort_keys=False, allow_unicode=True))
        n = copy.deepcopy(cfg)
        n['semantic'].pop('ontology_mode', None)
        normalized.append(n)
        configs[arm] = config
    if any(n != normalized[0] for n in normalized[1:]):
        raise ValueError('non-ontology configuration differences')
    protocol = {'kind': 'source_bound_development_observation_not_independent_acceptance', 'coordinates': identity['source'],
        'dataset_id': identity['dataset_id'], 'source_delta_from_0bf2': delta.splitlines(), 'equal_non_test_Go_files': 882,
        'arms': ARMS, 'counts': COUNTS, 'planned_calls': 384, 'question_ids': [q['id'] for q in public['requests']],
        'order': 'rotate four arm order per question', 'binary_sha256': sha(binary.read_bytes()), 'semantic_sha256': store_sha,
        'requests_sha256': sha(json.dumps(public, ensure_ascii=False, sort_keys=True).encode()),
        'new_independent_FINAL': 0, 'independent_units_approved': 0, 'LLM_answer_quality': None,
        'quality_acceptance': False, 'production_activation': False, 'mode_defaults_changed': False,
        'metrics': ['source_location_hit_at_10', 'source_location_reciprocal_rank_at_10', 'citation_order_changes', 'ontology_states', 'warm_latency'],
        'limitations': ['eight positive questions / four overlapping WBFT families; four unscored controls',
            'location proxies are AI-authored and incomplete; not precision/recall or generated-answer quality',
            'relations have zero verified code links; no relation boost can be inferred from approved definitions',
            'source code matches 0bf2 but tuple is baca2869 with approved meaning files; retained-corpus dataset474440d4 is not used',
            'five warm observations/question/arm, shared host; descriptive p95 only']}
    write(out / 'protocol.json', protocol)
    lib_spec = importlib.util.spec_from_file_location('stdio_probe', ROOT / 'scripts/eval-value-dependencies.py')
    lib = importlib.util.module_from_spec(lib_spec)
    lib_spec.loader.exec_module(lib)
    readiness = {}
    for arm in ARMS:
        directory = out / ('readiness-' + arm)
        directory.mkdir()
        client = lib.Client(None, None, directory, command=[str(binary), 'mcp', '--config', str(configs[arm])], namespace='cks.semantic.')
        try:
            client.request('initialize', {'protocolVersion': '2024-11-05', 'capabilities': {}, 'clientInfo': {'name': 'ontology-effect-readiness', 'version': '1'}})
            readiness[arm] = client.tool('status', {})
            status = readiness[arm]
            if status['availability'] != 'ready' or not status['read_tools_ready']:
                raise ValueError('semantic projection unavailable: ' + arm + ': ' + str(status))
            if status['mode'] != arm or status['retrieval_ready'] != (arm in ('concept_text', 'combined')):
                raise ValueError('unexpected retrieval readiness: ' + arm + ': ' + str(status))
        finally:
            client.close()
        write(out / 'readiness.json', readiness)
    print('semantic candidate built; four arm readiness recorded', flush=True)
    captures = []
    for index, question in enumerate(public['requests']):
        requests = out / (question['id'] + '-request.json')
        write(requests, {'schema_version': 1, 'requests': [question]})
        order = ARMS[index % 4:] + ARMS[:index % 4]
        for arm in order:
            label = question['id'] + '-' + arm
            capture = out / (label + '.json')
            run(label, [binary, 'eval', 'capture', '--config', configs[arm], '--requests', requests, '--output', capture,
                '--warmup', '1', '--retrieval-runs', '2', '--warm-runs', '5', '--cold-runs', '0', '--call-timeout', '30s'], timeout=360)
            report = json.loads(capture.read_text())
            if report['state'] != 'captured' or len(report['rows']) != 8:
                raise ValueError('incomplete capture ' + label)
            captures.append({'question': question['id'], 'arm': arm, 'file': capture.name, 'rows': len(report['rows'])})
            write(out / 'capture-inventory.json', captures)
        print(question['id'] + ': four arms captured (' + str(len(captures) * 8) + '/384)', flush=True)
    if sha(store.read_bytes()) != store_sha or any(sha(Path(p).read_bytes()) != s for p, s in preserved.items()):
        raise ValueError('input dataset/semantic bytes changed')
    write(out / 'preservation.json', {'paths': preserved, 'semantic_sha256': store_sha, 'unchanged': True})
    write(out / 'model-after.json', model())
    print('capture complete: 384 recorded calls; scoring and source verification pending', flush=True)

if __name__ == '__main__':
    main()
