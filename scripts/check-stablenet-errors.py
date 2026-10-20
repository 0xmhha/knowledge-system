#!/usr/bin/env python3
"""Recompute SN-P03 actual search classification and bounded recovery evidence."""
import argparse
import copy
import gzip
import hashlib
import json
from pathlib import Path
import subprocess
ROOT = Path(__file__).resolve().parents[1]
REL = Path('docs/spec-driven/operational-preparation/source-review-supplement/followups/stablenet-errors-v1')
CASES = ['baseline-source', 'baseline-model', 'fixed-source', 'fixed-recovery']
NEW_TESTS = ['TestEmbeddingFailurePreservesCauseAndSeparatesCallerContext', 'TestModelHealthSourceErrorsDoNotPoisonOrClearModelEvidence', 'TestModelHealthRecoversByTTLProbeAndSuccessfulQuery', 'TestModelHealthCancellationAndEmptyProbe', 'TestModelHealthConcurrentProbesShareOneCachedObservation', 'TestSemanticSearchErrorClassesAndBoundedPublicMessage', 'TestSemanticSearchPreflightPreservesSourceCodeBeforeBackend', 'TestSearchSeparatesEmbeddingFromRetainedSourceFailure']

def require(ok, message):
    if not ok:
        raise ValueError(message)

def sha(raw):
    return hashlib.sha256(raw).hexdigest()

def read(p):
    return json.loads(p.read_text())

def payload(r):
    result = r.get('result', {})
    if 'structuredContent' in result:
        return result['structuredContent']
    try:
        return json.loads(result['content'][0]['text'])
    except (KeyError, ValueError):
        return {'text': str(result.get('content', r))}

def failed(r):
    return 'error' in r or r.get('result', {}).get('isError') is True

def source_lines(raw):
    parts = raw.split(b'\n')
    if raw.endswith(b'\n'):
        return [p + b'\n' for p in parts[:-1]]
    return [p + b'\n' for p in parts[:-1]] + ([parts[-1]] if raw else [])

def derive(packet, live=True):
    e = read(packet / 'execution.json')
    require(e['formal_completed'] == 0 and e['formal_total'] == 9 and (e['reviewer_assignment'] == '미정') and (e['paid_LLM_calls'] == e['production_writes'] == e['external_publication'] == 0), 'scope/formal promotion')
    if (packet / 'WORKLIST.json').exists():
        work = read(packet / 'WORKLIST.json')
        require(work['formal_completed'] == 0 and work['formal_total'] == 9 and (work['reviewer_assignment'] == '미정') and (work['next_task'] == 'SN-P04') and (work['pilot_steps_completed'] == ['SN-P01', 'SN-P02', 'SN-P03']) and ({r['id'] for r in work['remaining_all']} == {'SN-P04', 'FORMAL-PREP', 'FORMAL-9', 'ONTOLOGY', 'OPTIONAL-12'}), 'worklist hides unfinished work')
    binding = read(packet / 'source-binding.json')
    committed = subprocess.check_output(['git', 'show', e['native_source_commit'] + ':' + str(REL / 'source-binding.json')], cwd=ROOT)
    require(sha(committed) == sha((packet / 'source-binding.json').read_bytes()), 'committed source binding')
    if live:
        for name, digest in {**binding['source_sha256'], **binding['approved_input_sha256']}.items():
            require(sha((ROOT / name).read_bytes()) == digest, 'current source/input changed')
    native = read(packet / 'validation/native/receipts.json')
    require([r['label'] for r in native] == ['build', 'test', 'race', 'lint', 'build-bins'], 'full native inventory')
    counts = {}
    for r in native:
        require(r['source_head'] == e['native_source_commit'] and r['exit_code'] == 0 and r['reaped'] and (not r['timeout']) and (not r['term_sent']) and (not r['kill_sent']), 'native validation not complete')
        for stream in ['stdout', 'stderr']:
            raw = gzip.decompress((packet / 'validation/native' / (r['label'] + '.' + stream + '.gz')).read_bytes())
            require(sha(raw) == r[stream + '_sha256'], 'native raw checksum')
            if stream == 'stdout' and r['label'] in ['test', 'race']:
                require('-count=1' in r['argv'] and r['argv'][-1] == './...' and ('-run' not in r['argv']), 'filtered/cached full suite')
                rows = [json.loads(s) for s in raw.decode().splitlines() if s.startswith('{')]
                observed = {a: sum((x.get('Action') == a and bool(x.get('Test')) for x in rows)) for a in ['pass', 'skip', 'fail']}
                require(observed == r['tests'] and observed['pass'] > 4065 and (not any((x.get('Action') == 'fail' for x in rows))), 'native test accounting')
                require(all((any((x.get('Action') == 'pass' and x.get('Test') == name for x in rows)) for name in NEW_TESTS)), 'new contract tests absent')
                counts[r['label']] = observed
    for arm, name in [('fixed', 'cks-build-info.txt'), ('baseline', 'baseline-build-info.txt')]:
        info = (packet / name).read_text()
        pin = read(packet / 'binary-pins.json')[arm]
        require('vcs.revision=' + pin['source'] in info and 'vcs.modified=false' in info, 'clean actual binary source')
    require(read(packet / 'binary-pins.json')['fixed']['source'] == e['native_source_commit'], 'fixed reader not native tested source')
    records = {}
    sessions = []
    packs = []
    for prefix in ['', 'diagnostics/initial-guard-run/']:
        for case in CASES:
            folder = packet / prefix / case
            life = read(folder / 'lifecycle.json')
            require(life['exit_code'] == 0 and life['eof_sent'] and life['reader_joined'] and life['child_reaped'] and (not life['terminate_sent']) and (not life['kill_sent']), 'stdio EOF/reap')
            rows = read(folder / 'messages.json')
            wire = [json.loads(s) for s in (folder / 'stdout.jsonl').read_text().splitlines()]
            attempts = read(folder / 'attempts.json')
            require(len(attempts) == len(rows) == life['rpc_requests'] and sum((r['request']['method'] == 'tools/call' for r in rows)) == life['tool_requests'], 'attempt denominator')
            require(all((row['response'] in wire for row in rows)) and all((a['outcome'] == 'response' and a['response'] == r['response'] and (a['method'] == r['request']['method']) and (a['params'] == r['request'].get('params')) for a, r in zip(attempts, rows))), 'wire response or attempts hidden')
            records[prefix + case] = {r['label']: r for r in rows}
            sessions.append(life)
            if not prefix:
                for row in rows:
                    p = payload(row['response'])
                    if not failed(row['response']) and p.get('format_version') == 2:
                        packs.append(('old' if case in ['baseline-source', 'fixed-source'] else 'new', p))
    paired = [records[case][label]['request']['params'] for case, label in [('baseline-source', 'source-missing'), ('baseline-model', 'embedding-failed'), ('fixed-source', 'source-missing'), ('fixed-recovery', 'baseline-search'), ('fixed-recovery', 'embedding-failed')]]
    require(all((x == paired[0] for x in paired)) and paired[0]['arguments']['k'] == 10, 'actual baseline/fixed search inputs differ')

    def get(case, label):
        return payload(records[case][label]['response'])

    def health(case, label, up):
        p = get(case, label)
        require(p['backends']['vector']['reachable'] and p['backends']['vector'].get('model_reachable', False) is up and (p['serviceable'] is up), 'model health ' + case + ':' + label)

    def error(case, label, code):
        r = records[case][label]['response']
        p = payload(r)
        require(failed(r) and p['code'] == code and (set(p) == {'code', 'message'}), 'public failure class ' + label)
        encoded = json.dumps(r)
        require('/private/' not in encoded and 'source_missing: lstat' not in encoded, 'unbounded public error')
        return p
    for case in CASES:
        health(case, 'health-before', True)
    require(failed(records['baseline-source']['source-missing']['response']) and 'source_missing' in json.dumps(records['baseline-source']['source-missing']['response']), 'old source bug not exercised')
    health('baseline-source', 'health-poisoned', False)
    health('baseline-source', 'health-recovered', True)
    health('baseline-model', 'health-down', False)
    health('baseline-model', 'health-still-latched', False)
    health('baseline-model', 'health-recovered', True)
    error('fixed-source', 'source-missing', 'source_missing')
    health('fixed-source', 'health-after-source', True)
    require(get('fixed-source', 'known-good-v2')['format_version'] == 2, 'source failure blocked usable v2')
    for label, code in [('retained-missing', 'source_missing'), ('retained-corrupt', 'snapshot_mismatch'), ('embedding-failed', 'embedding_failed'), ('empty-embedding', 'embedding_failed'), ('v2-blocked', 'service_unavailable')]:
        error('fixed-recovery', label, code)
    for label in ['health-after-missing', 'health-after-corrupt', 'health-recovered-without-raw-query', 'health-recovered-immediately', 'probe-only-recovered']:
        health('fixed-recovery', label, True)
    for label in ['health-down', 'health-down-after-empty', 'probe-detects-outage']:
        health('fixed-recovery', label, False)
    for label in ['baseline-v2', 'corrupt-restored-v2', 'v2-after-health-recovery', 'final-v2']:
        require(get('fixed-recovery', label)['format_version'] == 2, 'v2 restore missing')
    rows = list(records['fixed-recovery'])
    require(rows.index('health-down') < rows.index('health-recovered-without-raw-query') < rows.index('v2-after-health-recovery') < rows.index('raw-immediate-recovery'), 'recovery order changed')
    initial = records['diagnostics/initial-guard-run/fixed-recovery']['retained-missing']['response']
    require(failed(initial) and initial.get('result', {}).get('structuredContent') is None and ('source_missing' in json.dumps(initial)), 'initial preflight gap hidden')
    interruption = read(packet / 'native-initial-interruption.json')
    initial_native = read(packet / 'diagnostics/initial-guard-run/native-initial/receipts.json')
    require(interruption['signal'] == 'SIGTERM' and interruption['group_empty_after'] and (interruption['unowned_signals'] == 0) and (initial_native[-1]['exit_code'] == -15) and (initial_native[-1]['source_head'] == e['initial_source_commit']), 'owned initial interruption hidden/misattributed')
    proxy = read(packet / 'proxy-requests.json')
    events = read(packet / 'proxy-events.json')
    require(all((x['end'] - x['start'] >= 5 for x in events if x.get('event') == 'TTL_wait')) and sum((x.get('event') == 'TTL_wait' for x in events)) == 4, 'TTL observation missing')
    require(any((r['step'] == 'fixed-source:source-missing' and r['path'] == '/api/embed' and (r['vector_dims'] == [1024]) and (not r['injected']) for r in proxy)), 'real embedding before old source failure absent')
    for label, mode, status in [('embedding-failed', '503', 503), ('empty-embedding', 'empty', 200), ('probe-detects-outage', '503', 503)]:
        require(any((r['step'] == 'fixed-recovery:' + label and r['path'] == '/api/embed' and (r['mode'] == mode) and (r['status'] == status) and r['injected'] for r in proxy)), 'owned fault not exercised ' + label)
    for label in ['health-recovered-without-raw-query', 'probe-only-recovered']:
        require(any((r['step'] == 'fixed-recovery:' + label and r['path'] == '/api/embed' and (r['request'].get('input') == ['liveness probe']) and (r['vector_dims'] == [1024]) and (not r['injected']) for r in proxy)), 'health-only recovery not a real model probe')
    require(not any((r['step'] == 'fixed-recovery:health-recovered-immediately' and r['path'] == '/api/embed' for r in proxy)), 'query recovery depended on subsequent health probe')
    cleanup = read(packet / 'proxy-cleanup.json')
    require(cleanup['closed'] and cleanup['server_thread_joined'] and (cleanup['shared_model_stop_or_signal'] == 0) and (cleanup['requests'] == len(proxy)), 'owned endpoint cleanup')
    initial_cleanup = read(packet / 'diagnostics/initial-guard-run/proxy-cleanup.json')
    initial_proxy = read(packet / 'diagnostics/initial-guard-run/proxy-requests.json')
    require(initial_cleanup['closed'] and initial_cleanup['server_thread_joined'] and (initial_cleanup['requests'] == len(initial_proxy)), 'initial listener/logs not preserved')
    identity = {arm: read(packet / (arm + '-identity.json')) for arm in ['old', 'new']}
    require(identity['old']['embedding_identity'] == identity['new']['embedding_identity'], 'paired model identity changed')
    require(identity['new']['embedding_identity']['model_digest'] == read(packet / 'model-before.json')['model']['digest'] and identity['new']['embedding_identity']['Dim'] == 1024, 'model digest/dimension')
    witness = {(r['origin_id'], r['path']): r['sha256'] for r in read(packet / 'witness-index.json')}
    inventory = {(r['origin_id'], r['path']): r['sha256'] for r in json.loads(gzip.decompress((packet / 'new-sources-manifest.json.gz').read_bytes()))['files']}
    require(all((inventory[k] == v for k, v in witness.items())), 'witness differs from retained manifest')
    citations = 0
    for arm, pack in packs:
        i = identity[arm]
        coords = {k: i['source'][k] for k in ['project_id', 'snapshot_id', 'source_mode']}
        coords.update(dataset_id=i['dataset_id'], base_commit=i['source']['source_commit'])
        require(pack['coordinates'] == coords, 'pack identity mixing')
        unsigned = copy.deepcopy(pack)
        digest = unsigned['metadata'].pop('integrity_hash')
        require(sha(json.dumps(unsigned, ensure_ascii=False, sort_keys=True, separators=(',', ':')).encode()) == digest, 'v2 integrity')
        require([b['citation'] for b in pack['bodies']] == pack['citations'], 'v2 body/citation order')
        for b in pack['bodies']:
            c = b['citation']
            raw = (packet / 'source-witness' / (witness[c['origin_id'], c['file']] + '.txt')).read_bytes()
            excerpt = b''.join(source_lines(raw)[c['start_line'] - 1:c['end_line']])
            require(all((c[k] == v for k, v in coords.items())) and c['start_line'] > 0 and ('<convention>' not in c['file']) and (sha(raw) == c['file_sha256']) and (sha(excerpt) == c['content_sha256']) and (b['text'] == excerpt.decode()), 'retained v2 source/body/hash')
            citations += 1
    selected = read(packet / 'external-selected.json')
    require(witness[selected['origin_id'], selected['path']] == selected['sha256'] and selected['file'] == '@cks/docs/' + selected['origin_id'].split('.')[-1] + '/' + selected['path'], 'selected external source namespace')
    require(selected['origin_id'].startswith('docs:corpus.') and selected['file'].startswith('@cks/docs/') and (sha((packet / 'source-witness' / (selected['sha256'] + '.txt')).read_bytes()) == selected['sha256']), 'actual external mutation target source')
    require(read(packet / 'datasets-before.json') == read(packet / 'datasets-after.json') and all((read(packet / (a + '-owned-before.json')) == read(packet / (a + '-owned-after.json')) for a in ['old', 'new'])), 'recorded dataset bytes not restored')
    before, after = (read(packet / 'scope-before.json'), read(packet / 'scope-after.json'))
    require(before['global'] == after['global'] and before['operations']['metadata'] == after['operations']['metadata'] and (before['source_repos'] == after['source_repos']) and all((r in after['operations']['engines'] for r in before['operations']['engines'])), 'recorded original scope changed')
    preserve = read(packet / 'preservation.json')
    require(all((preserve[k] for k in ['original_and_previous_datasets_equal', 'owned_reader_leases_released', 'owned_source_clean', 'global_configuration_equal', 'operational_metadata_equal', 'prior_source_checkouts_equal', 'unowned_engines_preserved', 'no_owned_current'])) and all(preserve['owned_dataset_bytes_restored'].values()) and (preserve['model_digest_after'] == identity['new']['embedding_identity']['model_digest']), 'preservation boundary')
    audit = read(packet / 'process-audit.json')
    require(audit['self']['exit_code'] == 0 and all((p['identity_gone'] for p in audit['probes'])) and ({r['pid'] for r in sessions} <= {p['pid'] for p in audit['probes']}), 'owned process cleanup')
    return {'status': 'SN-P03_scoped_runtime_classification_recovery_pass', 'native_source_commit': e['native_source_commit'], 'native_test': counts['test'], 'native_race': counts['race'], 'baseline_source_poison_and_model_latch_reproduced': True, 'specific_source_integrity_errors_preserved': True, 'real_healthy_embedding_before_source_failure': True, 'controlled_embedding_faults': ['503', 'empty_response'], 'health_only_TTL_recovery': True, 'successful_raw_query_immediate_recovery': True, 'caller_context_scope': 'native tests; MCP cancellation notification not tested', 'final_stdio_sessions': 4, 'initial_diagnostic_stdio_sessions_preserved': 4, 'normal_EOF_exit0_reaped': len(sessions), 'all_actual_tool_attempts': sum((x['tool_requests'] for x in sessions)), 'final_tool_attempts': sum((x['tool_requests'] for x in sessions[:4])), 'proxy_HTTP_observations': len(proxy), 'initial_proxy_HTTP_observations': len(initial_proxy), 'all_proxy_HTTP_observations': len(proxy) + len(initial_proxy), 'successful_v2_packs_verified': len(packs), 'v2_citations_verified': citations, 'owned_initial_native_interruption': 1, 'final_native_interruption': 0, 'shared_model_restarts_or_signals': 0, 'formal_completed': 0, 'formal_total': 9, 'reviewer_assignment': '미정', 'independent_quality_evaluated': False, 'ontology_effect_evaluated': False, 'production_activation': False, 'paid_LLM_calls': 0, 'SN-P04_complete': False}

def check(packet, live=True):
    try:
        manifest = read(packet / 'manifest.json')
        require(sha((ROOT / manifest['checker']).read_bytes()) == manifest['checker_sha256'], 'checker source binding')
        paths = {str(p.relative_to(packet)) for p in packet.rglob('*') if p.is_file() and p.name != 'manifest.json'}
        require({'WORKLIST.json', 'REPORT.txt', 'result.json'} <= paths, 'completion documents missing')
        require(paths == set(manifest['artifacts']), 'packet inventory')
        for name, digest in manifest['artifacts'].items():
            require(sha((packet / name).read_bytes()) == digest, 'artifact hash')
        require(derive(packet, live) == read(packet / 'result.json'), 'derived completion promotion')
        return []
    except (OSError, ValueError, KeyError, TypeError, IndexError, subprocess.SubprocessError) as e:
        return [str(e)]
if __name__ == '__main__':
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--packet', type=Path, default=ROOT / REL)
    p.add_argument('--no-live-source', action='store_true')
    a = p.parse_args()
    errors = check(a.packet, not a.no_live_source)
    print('\n'.join(errors) if errors else 'SN-P03 runtime classification/recovery verified; formal0/9; no production activation')
    raise SystemExit(bool(errors))
