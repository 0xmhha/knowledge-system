"""Same-source dataset diagnostics; never score generated answers or approve release."""
from pathlib import Path
import argparse
from collections import Counter, defaultdict
import copy
import gzip
import hashlib
import json
import math
import statistics
import subprocess
import time
import urllib.request

import yaml


def sha(path):
    with Path(path).open('rb') as stream:
        return hashlib.file_digest(stream, 'sha256').hexdigest()


def digest(raw):
    return hashlib.sha256(raw).hexdigest()


def write(path, value):
    Path(path).write_text(json.dumps(value, ensure_ascii=False, indent=2) + '\n')


def percentile(values, fraction):
    if not values:
        return None
    return sorted(values)[max(0, math.ceil(len(values) * fraction) - 1)]


def content_key(citation):
    # Identity is verified separately; compare only within the frozen source.
    return tuple(citation[k] for k in ('origin_id', 'file', 'start_line', 'end_line', 'file_sha256', 'content_sha256'))


def verify_pack(pack, version):
    if pack.get('format_version') != 2:
        raise ValueError('not a v2 pack')
    canonical = copy.deepcopy(pack)
    expected = canonical['metadata'].pop('integrity_hash')
    raw = json.dumps(canonical, sort_keys=True, ensure_ascii=False, separators=(',', ':')).encode()
    if digest(raw) != expected:
        raise ValueError('integrity mismatch')
    identity = json.loads((version / 'dataset-identity.json').read_text())
    source = identity['source']
    coords = {k: source[k] for k in ('project_id', 'snapshot_id', 'source_mode')}
    coords.update(dataset_id=identity['dataset_id'], base_commit=source['source_commit'])
    if pack['coordinates'] != coords:
        raise ValueError('foreign pack coordinates')
    manifest = json.loads((version / 'sources/manifest.json').read_text())
    files = {(r['origin_id'], r['path']): r for r in manifest['files']}
    seen = set()
    for c in pack['citations']:
        if any(c.get(k) != v for k, v in coords.items()) or c['commit_hash'] != source['source_commit']:
            raise ValueError('foreign citation coordinates')
        path = Path(c['file'])
        if path.is_absolute() or '..' in path.parts or path.parts[0] in ('.claude', 'logs'):
            raise ValueError('unsafe citation source')
        entry = files.get((c['origin_id'], c['file']))
        if entry is None or entry['sha256'] != c['file_sha256']:
            raise ValueError('citation absent from retained source manifest')
        raw = (version / 'sources/blobs' / entry['sha256']).read_bytes()
        lines = raw.splitlines(keepends=True)
        if digest(raw) != entry['sha256'] or not 1 <= c['start_line'] <= c['end_line'] <= len(lines):
            raise ValueError('source bytes or span mismatch')
        if digest(b''.join(lines[c['start_line'] - 1:c['end_line']])) != c['content_sha256']:
            raise ValueError('source span hash mismatch')
        key = content_key(c)
        if key in seen:
            raise ValueError('duplicate citation')
        seen.add(key)
    body_seen = set()
    for body in pack['bodies']:
        if body['citation'] not in pack['citations'] or content_key(body['citation']) in body_seen:
            raise ValueError('body has no unique citation')
        body_seen.add(content_key(body['citation']))
    if pack['evidence_state'] == 'complete' and len(body_seen) != len(seen):
        raise ValueError('complete without bodies')
    if len(seen) > 12 or sum(len(b['text'].encode()) for b in pack['bodies']) > 32000:
        raise ValueError('public budget exceeded')
    return True


def summarize_pack(pack):
    return {'citations': [content_key(c) for c in pack['citations']],
            'bodies': [(content_key(b['citation']), digest(b['text'].encode())) for b in pack['bodies']],
            'state': pack['evidence_state']}


def anchor_score(question, pack):
    anchor = question.get('expected_anchor')
    if anchor is None:
        return None
    citations = pack['citations'][:10] if pack else []
    ranks = [i + 1 for i, c in enumerate(citations) if c['origin_id'] == 'repo'
             and c['file'] == anchor['file'] and c['file_sha256'] == anchor['file_sha256']
             and c['start_line'] <= anchor['line'] <= c['end_line']]
    return {'hit_at_10': int(bool(ranks)), 'reciprocal_rank': 1 / min(ranks) if ranks else 0}


def model_guard(protocol):
    tags = json.load(urllib.request.urlopen('http://127.0.0.1:11434/api/tags', timeout=30))
    model = next(m for m in tags['models'] if m['name'] == protocol['model']['model'])
    if model['digest'] != protocol['model']['digest']:
        raise ValueError('live model changed')


def baselines_guard(protocol):
    for baseline in protocol['baselines']:
        root, version = Path(baseline['root']), Path(baseline['version'])
        if baseline['current_target'] is not None and str((root / 'current').readlink()) != baseline['current_target']:
            raise ValueError('operator current changed')
        for path, expected in baseline['files_sha256'].items():
            if sha(version / path) != expected:
                raise ValueError('frozen baseline changed')


def capture(workspace):
    protocol = json.loads((workspace / 'protocol.json').read_text())
    freeze = json.loads((workspace / 'questions-freeze.json').read_text())
    assert sha(workspace / 'questions.json') == freeze['questions_sha256']
    assert sha(workspace / 'requests.json') == freeze['requests_sha256']
    baselines_guard(protocol)
    model_guard(protocol)
    versions = {'baseline': Path(protocol['baselines'][0]['version']),
                'candidate': Path(protocol['candidate_dataset']) / protocol['candidate_version']}
    assert (Path(protocol['candidate_dataset']) / 'current').resolve() == versions['candidate'].resolve()
    for version in versions.values():
        identity = json.loads((version / 'dataset-identity.json').read_text())
        assert identity['source']['source_commit'] == protocol['source_commit']
        manifest = json.loads((version / 'vector/manifest.json').read_text())
        assert manifest['embedding_identity_v2']['model_digest'] == protocol['model']['digest']
    common = yaml.safe_load((Path(protocol['baselines'][0]['root']).parent / 'go-stablenet-cks-stdio.yaml').read_text())
    common['backends']['ckg'].update(source_root=protocol['source'], binary_path=str(workspace / 'bin/ckg'),
                                   policy_file=str(workspace / 'pack/policies/graph.yaml'))
    common['backends']['ckv']['binary_path'] = str(workspace / 'bin/ckv')
    common['vocab']['glossary_path'] = str(workspace / 'pack/domain-knowledge/glossary.yaml')
    common['domain']['project_dir'] = str(workspace / 'pack/domain-knowledge')
    corpus = Path(common['domain']['corpus_dir'])
    write(workspace / 'served-domain-corpus-pins.json', {str(p): sha(p) for p in corpus.rglob('*') if p.is_file()})
    common['semantic'].update(store_path='', ontology_mode='off')
    common.setdefault('retrieval', {})['type_neighbors'] = False
    out = workspace / 'capture'
    out.mkdir(exist_ok=False)
    write(out / 'common-config.json', common)
    rows = []; captures = []
    for block, order in enumerate([['baseline', 'candidate'], ['candidate', 'baseline']], 1):
        for arm in order:
            model_guard(protocol)
            for name, expected in protocol['binary_sha256'].items():
                assert sha(workspace / 'bin' / name) == expected
            directory = out / str(block) / arm
            directory.mkdir(parents=True)
            config = copy.deepcopy(common)
            config['backends']['ckg']['path'] = str(versions[arm] / 'graph/graph.db')
            config['backends']['ckv']['path'] = str(versions[arm] / 'vector')
            config['logging'].update(mode='prod', level='info', measure_backend_calls=True, footprint_dir=str(directory / 'events'))
            cfg = directory / 'config.yaml'
            cfg.write_text(yaml.safe_dump(config, sort_keys=False))
            normalized = copy.deepcopy(config)
            normalized['backends']['ckg'].pop('path'); normalized['backends']['ckv'].pop('path')
            normalized['logging'].pop('footprint_dir')
            if rows:
                assert normalized == normalized_config
            normalized_config = normalized
            output = directory / 'capture.json'
            argv = [str(workspace / 'bin/cks'), 'eval', 'capture', '--requests', str(workspace / 'requests.json'),
                    '--config', str(cfg), '--cks-mcp', str(workspace / 'bin/cks'), '--output', str(output),
                    '--warmup', '1', '--retrieval-runs', '3', '--warm-runs', '5', '--cold-runs', '0', '--call-timeout', '90s']
            started = time.monotonic()
            result = subprocess.run(argv, capture_output=True, text=True, timeout=2400)
            write(directory / 'command.json', {'argv': argv, 'exit': result.returncode, 'stdout': result.stdout,
                                               'stderr': result.stderr, 'elapsed_s': time.monotonic() - started})
            assert output.exists(), result.stderr
            cap = json.loads(output.read_text())
            assert cap['request_sha256'] == freeze['requests_sha256']
            assert cap['config_sha256'] == cap['config_sha256_after'] == sha(cfg)
            assert cap['binary_sha256'] == cap['binary_sha256_after'] == protocol['binary_sha256']['cks']
            captures.append({'block': block, 'arm': arm, 'exit': result.returncode, 'rows': len(cap['rows']), 'counts': cap['counts']})
            for row in cap['rows']:
                row.update(block=block, arm=arm)
                rows.append(row)
            write(out / 'progress.json', captures)
            print(json.dumps(captures[-1]), flush=True)
    baselines_guard(protocol)
    model_guard(protocol)
    assert all(sha(Path(p)) == h for p, h in json.loads((workspace / 'served-domain-corpus-pins.json').read_text()).items())
    write(out / 'summary.json', summarize(rows, versions, workspace))
    for path in out.rglob('*.json'):
        if path.name == 'capture.json':
            with path.open('rb') as source, gzip.open(str(path) + '.gz', 'wb') as target:
                target.write(source.read())
    print('comparative captures complete', len(rows), flush=True)


def summarize(rows, versions, workspace):
    questions = {q['id']: q for q in json.loads((workspace / 'questions.json').read_text())['questions']}
    grouped = {}; stats = {arm: {'planned': 756, 'observed': 0, 'failed': 0, 'valid_packs': 0,
                               'invalid_packs': [], 'warm_ms': [], 'anchor': [], 'states': Counter()} for arm in versions}
    for row in rows:
        arm = row['arm']; stat = stats[arm]; stat['observed'] += 1
        key = (row['block'], row['request_id'], row['phase'], row['iteration'])
        call = row['call']; response = call.get('response') or {}
        failed = bool(row.get('error') or call.get('transport_error') or response.get('isError'))
        pack = response.get('structuredContent')
        if not pack and not failed:
            text = next((c['text'] for c in response.get('content', []) if c.get('type') == 'text'), '')
            try: pack = json.loads(text)
            except ValueError: failed = True
        if not failed:
            try:
                verify_pack(pack, versions[arm]); stat['valid_packs'] += 1
            except (ValueError, KeyError, TypeError) as error:
                stat['invalid_packs'].append({'id': row['request_id'], 'error': str(error)}); failed = True
        if failed:
            stat['failed'] += 1; pack = None
        else:
            stat['states'][pack['evidence_state']] += 1
            if row['phase'] == 'warm_latency': stat['warm_ms'].append(call['elapsed_ns'] / 1e6)
        if row['phase'] == 'retrieval':
            score = anchor_score(questions[row['request_id']], pack)
            if score is not None: stat['anchor'].append({'id': row['request_id'], **score})
        grouped.setdefault(key, {})[arm] = summarize_pack(pack) if pack else None
    pairs = []
    for key, arms in grouped.items():
        old, new = arms.get('baseline'), arms.get('candidate')
        pairs.append({'block': key[0], 'id': key[1], 'phase': key[2], 'iteration': key[3],
                      'both_valid': old is not None and new is not None,
                      'citation_order_changed': old['citations'] != new['citations'] if old and new else None,
                      'body_outputs_changed': old['bodies'] != new['bodies'] if old and new else None})
    for arm, stat in stats.items():
        stat['missing'] = max(0, stat['planned'] - stat['observed'])
        stat['warm_n'] = len(stat['warm_ms'])
        stat['warm_p95_ms'] = percentile(stat['warm_ms'], .95)
        stat['warm_median_ms'] = statistics.median(stat['warm_ms']) if stat['warm_ms'] else None
        stat['anchor_cases'] = 12
        stat['source_derived_anchor_hit_at_10'] = statistics.mean(r['hit_at_10'] for r in stat['anchor']) if stat['anchor'] else None
        stat['source_derived_anchor_RR_at_10'] = statistics.mean(r['reciprocal_rank'] for r in stat['anchor']) if stat['anchor'] else None
        stat['states'] = dict(stat['states'])
    telemetry = {}
    for arm in versions:
        calls = Counter(); pending = 0; scopes = 0
        expected_measurements = {row['call'].get('measurement_id') for row in rows if row['arm'] == arm}
        expected_measurements.discard(None)
        for path in (workspace / 'capture').glob('*/' + arm + '/events/cks-mcp.jsonl'):
            for line in path.read_text().splitlines():
                event = json.loads(line)
                if event.get('event') == 'measurement.backend_calls':
                    summary = event['summary']
                    if summary.get('measurement_id') not in expected_measurements: continue
                    scopes += 1; pending += summary.get('pending_calls', 0)
                    for call in summary.get('calls', []):
                        if call.get('backend') == 'ollama_http': calls[call.get('options', {}).get('path', 'unknown')] += 1
        telemetry[arm] = {'observed_scopes': scopes, 'pending_calls': pending, 'HTTP_attempts': dict(calls),
                          'scope': 'emitted backend scopes; startup probes outside request scope are not inferred'}
    return {'backend_observation': telemetry, 'scope': 'same-source engineering observations; source-derived anchor diagnostics, not independent quality',
            'responses': len(rows), 'pairs': pairs, 'arms': stats,
            'citation_order_changed_pairs': sum(p['citation_order_changed'] is True for p in pairs),
            'body_outputs_changed_pairs': sum(p['body_outputs_changed'] is True for p in pairs),
            'both_valid_pairs': sum(p['both_valid'] for p in pairs),
            'generated_answers_scored': False, 'independent_quality_accepted': False,
            'limitations': ['reused prompts and mechanically derived signature anchors', 'warm model/shared host; no cold-model metric',
                            'small per-query timing sample; alternating arm blocks reduce but do not remove order/load effects',
                            'supporting citations outside one expected anchor are not automatically incorrect',
                            'source-citation membership/body comparison excludes dataset identity only after verifying each identity']}


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--workspace', type=Path, required=True)
    args = parser.parse_args()
    capture(args.workspace.resolve())
