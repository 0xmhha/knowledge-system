#!/usr/bin/env python3
"""Recompute bounded SN-P04 real-source/BGE retrieval and isolation evidence."""
import argparse
import gzip
import hashlib
import json
from pathlib import Path
import subprocess
import sqlite3

ROOT = Path(__file__).resolve().parents[1]
REL = Path('docs/spec-driven/operational-preparation/source-review-supplement/followups/stablenet-comparison-v1')
CONCEPTS = ['staking', 'staking-policy', 'consensus', 'finality', 'mempool-admission', 'transaction-ordering']
PINS = {'go-stablenet': '0bf2f4d1bfeb6605006d556957ef8c045d8f8ed8', 'go-wemix': '1350376a680489298b3203a44d9ebddb7747f9cc'}
DIGEST = '7907646426070047a77226ac3e684fbbe8410524f7b4a74d02837e43f2146bab'


def require(ok, message):
    if not ok:
        raise ValueError(message)


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def read(p):
    return json.loads(p.read_text())


def packed(p):
    return json.loads(gzip.decompress(p.read_bytes()))


def payload(r):
    a = r.get('result', {})
    return a.get('structuredContent') or json.loads(a['content'][0]['text'])


def failed(r):
    return 'error' in r or r.get('result', {}).get('isError') is True


def lines(raw, start, end):
    # Go source reader splits on LF, retaining CR; Python splitlines also
    # splits other Unicode control characters that can occur inside strings.
    parts = raw.split(b'\n')
    values = [p + b'\n' for p in parts[:-1]]
    if parts[-1]:
        values.append(parts[-1])
    require(1 <= start <= end <= len(values), 'invalid source range')
    return b''.join(values[start - 1:end])


def hint_key(project, hit):
    return sha(json.dumps({'project': project, 'citation': hit['citation'], 'kind': hit.get('chunk_kind'), 'text': hit['text']}, sort_keys=True).encode())


def renderings(text):
    values, kept, added = text.split('\n'), [], 0
    for index, value in enumerate(values):
        if index == 0 or value.strip():
            kept.append(value)
            added += index > 0
            if added >= 5:
                break
    return {'full': text, 'signature+5': '\n'.join(kept), 'signature_only': next((v for v in values if v.strip()), '')}


def derive(packet, live=True):
    execution = read(packet / 'execution.json')
    require(execution['formal_completed'] == 0 and execution['formal_total'] == 9 and execution['reviewer_assignment'] == '미정', 'formal promotion')
    require(execution['paid_LLM_calls'] == execution['production_writes'] == execution['external_publication'] == 0 and execution['independent_quality_acceptance'] is False, 'scope promotion')
    binding = read(packet / 'source-binding.json')
    if live:
        for name, digest in {**binding['source_sha256'], **binding['approved_input_sha256']}.items():
            require(sha((ROOT / name).read_bytes()) == digest, 'live source/input drift')
    committed = subprocess.check_output(['git', 'show', execution['native_source_commit'] + ':' + str(REL / 'source-binding.json')], cwd=ROOT)
    require(sha(committed) == sha((packet / 'source-binding.json').read_bytes()), 'committed native source binding')
    native = read(packet / 'validation/native/receipts.json')
    require([r['label'] for r in native] == ['build', 'test', 'race', 'lint', 'build-bins'], 'full native inventory')
    full_counts = {}
    for row in native:
        require(row['source_head'] == execution['native_source_commit'] and row['exit_code'] == 0 and row['reaped'] and not row['timeout'] and not row['term_sent'] and not row['kill_sent'], 'fresh native validation incomplete')
        for stream in ['stdout', 'stderr']:
            raw = gzip.decompress((packet / 'validation/native' / (row['label'] + '.' + stream + '.gz')).read_bytes())
            require(sha(raw) == row[stream + '_sha256'], 'native raw checksum')
            if stream == 'stdout' and row['label'] in ['test', 'race']:
                require('-count=1' in row['argv'] and row['argv'][-1] == './...' and '-run' not in row['argv'], 'filtered/cached native validation')
                events = [json.loads(s) for s in raw.decode().splitlines() if s.startswith('{')]
                counts = {a: sum(x.get('Action') == a and bool(x.get('Test')) for x in events) for a in ['pass', 'skip', 'fail']}
                require(counts == row['tests'] and counts['pass'] > 4086 and not any(x.get('Action') == 'fail' for x in events), 'native denominator')
                require(any(x.get('Action') == 'pass' and x.get('Test') == 'TestPinnedBackendPairCannotDowngradeToLegacy' for x in events), 'native boundary regression absent')
                full_counts[row['label']] = counts
    targeted = read(packet / 'validation/targeted-receipt.json')
    require(targeted['exit_code'] == 0 and targeted['reaped'] and not targeted['forced_termination'] and targeted['argv'] == ['go', 'test', '-race', '-json', '-count=1', './internal/system/comparison', './internal/system/collection'], 'fresh comparison race validation')
    native_raw = (packet / 'validation/targeted.stdout.jsonl').read_bytes()
    require(sha(native_raw) == targeted['stdout_sha256'], 'targeted raw checksum')
    native_rows = [json.loads(s) for s in native_raw.decode().splitlines() if s.startswith('{')]
    require({a: sum(x.get('Action') == a and bool(x.get('Test')) for x in native_rows) for a in ['pass', 'skip', 'fail']} == targeted['tests'] == {'pass': 232, 'skip': 0, 'fail': 0}, 'targeted denominator')
    build = read(packet / 'build-exit.json')
    require(build['exit_code'] == 0 and build['reaped'] and not build['forced_termination'] and not build['current_exists'], 'held build failure/activation')
    fixtures = read(packet / 'fixtures.json')
    require([f['project'] for f in fixtures] == list(PINS), 'project inventory')
    require(fixtures[0]['coordinates']['project_id'] != fixtures[1]['coordinates']['project_id'], 'project collision')
    require(fixtures[0]['embedding'] == fixtures[1]['embedding'], 'unequal embedding identity')
    embedding = fixtures[0]['embedding']
    require(embedding['model_digest'] == DIGEST and embedding['Dim'] == embedding['native_dim'] == 1024 and embedding['Provider'] == 'ollama', 'mock or wrong model used')
    old = read(packet / 'legacy-wemix-identity.json')
    require(old['embedding_identity']['model'] == 'mock-feature-hash-v1' and old['embedding_identity']['dim'] == 64, 'legacy model relabeled')
    require(read(packet / 'model-before.json')['model']['digest'] == read(packet / 'model-after.json')['model']['digest'] == DIGEST, 'runtime model drift')
    probe = read(packet / 'model-probe.json')
    require(probe['model'] == 'bge-m3:latest' and probe['vector_dimensions'] == [1024] and probe['actual_local_inference'], 'actual model probe absent')
    pins = read(packet / 'binary-pins.json')
    require(pins['reader']['source'] == execution['native_source_commit'] == '38dd5c5449b1e57759e2532090dce101b3ad9124', 'reader source differs from native tests')
    info = (packet / 'reader-build-info.txt').read_text()
    require('vcs.revision=' + execution['native_source_commit'] in info and 'vcs.modified=false' in info, 'dirty/unpinned reader')
    manifests = {}
    for f in fixtures:
        label = f['project']
        require(f['source_commit'] == PINS[label] and f['coordinates']['base_commit'] == PINS[label], 'source pin')
        identity = read(packet / (label + '-identity.json'))
        require(identity['dataset_id'] == f['coordinates']['dataset_id'] and identity['source']['snapshot_id'] == f['coordinates']['snapshot_id'], 'dataset pin')
        manifest = packed(packet / (label + '-sources-manifest.json.gz'))
        require(len(manifest['files']) == f['source_files'] and sha(gzip.decompress((packet / (label + '-sources-manifest.json.gz')).read_bytes())) == f['source_manifest_sha256'], 'retained manifest drift')
        manifests[label] = {(r['origin_id'], r['path']): r for r in manifest['files']}
    require(fixtures[0]['source_files'] == 2295 and fixtures[1]['source_files'] == 2019, 'corpus coverage relabeled')
    require(fixtures[1]['coordinates']['source_mode'] == 'working-tree' and fixtures[1]['prepared_git_status'] == 'D cmd/gwemix', 'owned alias exclusion hidden')
    indexed_rows = read(packet / 'indexed-hints.json')
    indexed = {(r['project'], r['hit_key']): r for r in indexed_rows}
    require(len(indexed) == len(indexed_rows) == 86, 'indexed hint inventory')
    if live:
        before = read(packet / 'owned-before.json')
        for f in fixtures:
            database = Path(f['version']) / 'vector/vector.db'
            require(sha(database.read_bytes()) == before[f['project']]['vector/vector.db']['sha256'], 'live selected index drift')
            db = sqlite3.connect(database.as_uri() + '?mode=ro&immutable=1', uri=True)
            db.row_factory = sqlite3.Row
            try:
                for projection in indexed_rows:
                    if projection['project'] != f['project']:
                        continue
                    row = db.execute('select id,file,start_line,end_line,commit_hash,chunk_kind,text,content_sha256,flow_meta,provenance from chunks where id=?', (projection['row']['id'],)).fetchone()
                    require(row is not None and dict(row) == projection['row'], 'indexed hint detached from selected database')
            finally:
                db.close()
    equal = read(packet / 'source-equality.json')
    require(all(equal[label]['equal'] and equal[label]['base_commit'] == pin and equal[label]['repo_files_verified_against_original_git_archive'] == (2224 if label == 'go-stablenet' else 2019) for label, pin in PINS.items()), 'original source byte comparison missing')
    require(old['source']['snapshot_id'] == fixtures[1]['coordinates']['snapshot_id'] and old['dataset_id'] != fixtures[1]['coordinates']['dataset_id'], 'model migration source/dataset identity')
    conditions = read(packet / 'implementation-conditions.json')
    require(len(conditions) == 2 and all('config' in c['body'].lower() and 'clique' in c['body'].lower() for c in conditions), 'configuration dependence hidden')
    for c in conditions:
        raw = (packet / 'source-witness' / (c['file_sha256'] + '.txt')).read_bytes()
        require(sha(raw) == c['file_sha256'] and sha(lines(raw, c['start_line'], c['end_line'])) == c['content_sha256'] and c['body'] == lines(raw, c['start_line'], c['end_line']).decode(), 'factory excerpt source mismatch')
    query = read(packet / 'query-inputs.json')
    require(query['exposed_diagnostic_inputs'] and not query['independent_gold'] and len(query['inputs']) == 6 and query['k'] == 10, 'independent input promotion')
    sessions, records, attempts = [], {}, 0
    diagnostic = 'diagnostics/initial-empty-hit-run/search-go-stablenet'
    final_folders = ['search-' + f['project'] for f in fixtures] + ['comparison/' + x for x in ['bootstrap-session', 'comparison-session', 'bad-mapping-session']]
    diagnostic_folders = sorted(str(p.parent.relative_to(packet)) for p in (packet / 'diagnostics').rglob('lifecycle.json'))
    require(len(diagnostic_folders) == 6 and diagnostic in diagnostic_folders, 'diagnostic sessions hidden')
    folders = final_folders + diagnostic_folders
    for folder in folders + ['swapped-' + f['project'] for f in fixtures]:
        root = packet / folder
        life, rows, tried = read(root / 'lifecycle.json'), read(root / 'messages.json'), read(root / 'attempts.json')
        swap = folder.startswith('swapped-')
        require(life['exit_code'] == (1 if swap else 0) and life['child_reaped'] and life['eof_sent'] and life['reader_joined'] and not life['terminate_sent'] and not life['kill_sent'], 'stdio cleanup')
        require(life['command'][0] == execution['reader_binary_path'], 'different reader binary')
        if swap:
            require(len(tried) == 1 and not rows and tried[0]['outcome'] == 'ValueError' and read(root / 'stderr.txt')['code'] == 'operation_failed', 'startup refusal lost')
        else:
            wire = [json.loads(s) for s in (root / 'stdout.jsonl').read_text().splitlines()]
            incoming = [r for r in (json.loads(s) for s in (root / 'stdin.jsonl').read_text().splitlines()) if 'id' in r]
            require(len(tried) == len(rows) == life['rpc_requests'] and all(a['outcome'] == 'response' and a['response'] == r['response'] and a['method'] == r['request']['method'] and a['params'] == r['request'].get('params') and r['response'] in wire for a, r in zip(tried, rows)), 'attempt/wire denominator')
            require([r['request'] for r in rows] == incoming, 'derived request differs from actual stdin')
            require(sum(r['request']['method'] == 'tools/call' for r in rows) == life['tool_requests'], 'tool denominator')
            records[folder] = {r['label']: r for r in rows}
        sessions.append(life)
        attempts += life['tool_requests']
    raw_count = pack_count = cite_count = body_count = metadata_count = raw_refusals = 0

    def verify_source(label, cite, text=None, structured=False, allow_empty_commit=False):
        nonlocal cite_count
        f = next(f for f in fixtures if f['project'] == label)
        origin, path = cite.get('origin_id', 'repo'), cite['file']
        if path.startswith('@cks/docs/'):
            index, path = path[len('@cks/docs/'):].split('/', 1)
            origin = 'docs:corpus.' + index
        row = manifests[label].get((origin, path))
        require(row is not None, 'foreign citation path/origin')
        raw = (packet / 'source-witness' / (row['sha256'] + '.txt')).read_bytes()
        require(sha(raw) == row['sha256'], 'source witness hash')
        body = lines(raw, cite['start_line'], cite['end_line'])
        if structured:
            require(all(cite.get(k) == v for k, v in f['coordinates'].items()) and cite['file_sha256'] == row['sha256'] and cite['content_sha256'] == sha(body), 'foreign structured coordinates/hash')
        if text is not None:
            require(text == body.decode(), 'returned source body mismatch')
        # Legacy raw hits have no explicit project coordinate. Their source
        # identity is checked against the explicitly selected session archive.
        if origin == 'repo':
            allowed = {PINS[label]} if f['coordinates']['source_mode'] == 'committed' else ({''} if structured else {'', PINS[label]})
            if allow_empty_commit:
                allowed.add('')
            require(cite['commit_hash'] in allowed, 'citation commit')
        cite_count += 1

    raw_hint_count = 0

    def verify_hit(label, hit):
        nonlocal raw_hint_count
        cite = hit['citation']
        verify_source(label, cite, allow_empty_commit=hit.get('chunk_kind') == 'flow_step')
        if not hit.get('text'):
            return
        projection = indexed.get((label, hint_key(label, hit)))
        require(projection is not None, 'raw indexed hint not in selected database projection')
        row = projection['row']
        require(row['file'] == cite['file'] and row['start_line'] == cite['start_line'] and row['end_line'] == cite['end_line'] and row['commit_hash'] == cite['commit_hash'] and row['chunk_kind'] == hit['chunk_kind'] and sha(row['text'].encode()) == row['content_sha256'] and renderings(row['text'])[projection['rendering']] == hit['text'], 'raw hint identity/rendering mismatch')
        if row['chunk_kind'] == 'flow_step':
            corpus_record = manifests[label][('docs:corpus.1', 'corpus.jsonl')]
            raw = (packet / 'source-witness' / (corpus_record['sha256'] + '.txt')).read_bytes()
            require(sha(raw) == corpus_record['sha256'], 'flow hint retained corpus hash')
            meta = json.loads(row['flow_meta'])
            record = next(r for r in (json.loads(s) for s in raw.decode().splitlines()) if r['type'] == 'step' and r['id'] == meta['step_id'] and r['flow'] == meta['flow_id'])
            derived = record.get('prose', '')
            if record.get('symbol'):
                derived += ' ' + record['symbol']
            whens = [b['when'] for b in record.get('branches', []) if b.get('when')]
            if whens:
                derived += ' ' + ' '.join(whens)
            require(derived.strip() == row['text'] and record['file'] == cite['file'] and record['line'] == cite['start_line'] == cite['end_line'], 'flow hint invented or confused with its code anchor')
        raw_hint_count += 1

    for f in fixtures:
        label = f['project']
        rows = records['search-' + label]
        require(payload(rows['health']['response'])['serviceable'] and payload(rows['health-after']['response'])['serviceable'], 'model/service unavailable')
        for q in query['inputs']:
            for kind in ['keyword', 'vector-go', 'vector-all', 'v2']:
                row = rows[q['id'] + '-' + kind]
                args = row['request']['params']['arguments']
                expected = {'prompt': q['prompt'], 'intent': 'arch_explain', 'explain_ontology': True} if kind == 'v2' else {'query': q['keyword'] if kind == 'keyword' else q['prompt'], 'k': 10, 'exclude_tests': True}
                if kind == 'vector-go':
                    expected['language'] = 'go'
                require(args == expected, 'paired request mismatch')
                p = payload(row['response'])
                if kind != 'v2' and label == 'go-wemix':
                    require(failed(row['response']) and p['code'] == 'requires_v2' and set(p) == {'code', 'message'}, 'working-tree raw search bypass')
                    raw_count += 1
                    raw_refusals += 1
                    continue
                require(not failed(row['response']), 'actual retrieval error')
                if kind == 'v2':
                    require(p['format_version'] == 2 and p['coordinates'] == f['coordinates'], 'v2 foreign project/pin')
                    require(p['citations'] and len(p['bodies']) <= len(p['citations']), 'v2 citations absent')
                    for c in p['citations']:
                        verify_source(label, c, structured=True)
                    for b in p['bodies']:
                        require(b['citation'] in p['citations'], 'body without citation')
                        verify_source(label, b['citation'], b['text'], True)
                        body_count += 1
                    pack_count += 1
                else:
                    for hit in (p.get('hits') or []):
                        if hit.get('non_citable_metadata'):
                            require(hit['chunk_kind'] == 'convention', 'unexpected noncitable result')
                            metadata_count += 1
                            continue
                        verify_hit(label, hit)
                    raw_count += 1
        for key in ['expected_commit', 'expected_source_sha256', 'expected_snapshot_sha256']:
            p = payload(rows['foreign-' + key]['response'])
            require(p['status'] == 'source_mismatch' and p['analysis'] is None, 'foreign pin admitted')
        require(payload(rows['exact-pin-recovered']['response'])['analysis'], 'exact pin not restored')
    initial = records[diagnostic]
    require(len(initial) == 20 and payload(initial['C5-keyword']['response'])['hits'] is None and not failed(initial['C5-keyword']['response']), 'initial null-hit collector finding hidden')
    require(read(packet / 'diagnostics/initial-empty-hit-run/diagnosis.json')['collector_exit_code'] == 1, 'initial collector exit hidden')
    diagnostic_packs = diagnostic_raw = diagnostic_bodies = 0
    for label, row in initial.items():
        require(row['request'] == records['search-go-stablenet'][label]['request'], 'retry input changed')
        if row['request']['method'] != 'tools/call' or label in ['health', 'ontology-status']:
            continue
        require(not failed(row['response']), 'initial product error hidden')
        p = payload(row['response'])
        if p.get('format_version') == 2:
            require(p['coordinates'] == fixtures[0]['coordinates'], 'initial v2 mixed project')
            for c in p['citations']:
                verify_source('go-stablenet', c, structured=True)
            for b in p['bodies']:
                verify_source('go-stablenet', b['citation'], b['text'], True)
                diagnostic_bodies += 1
            diagnostic_packs += 1
        else:
            for hit in (p.get('hits') or []):
                if not hit.get('non_citable_metadata'):
                    verify_hit('go-stablenet', hit)
            diagnostic_raw += 1
    require(diagnostic_packs == 4 and diagnostic_raw == 13, 'initial attempt denominator')
    # Preserve the complete pre-fix paired run and the two focused probes.
    # These demonstrate the real routing gap; they are not final guard passes.
    for folder in diagnostic_folders:
        if folder == diagnostic:
            continue
        for row in records[folder].values():
            if row['request']['method'] != 'tools/call' or failed(row['response']):
                continue
            p = payload(row['response'])
            label = 'go-wemix' if folder.endswith('/search-go-wemix') else 'go-stablenet'
            if p.get('format_version') == 2:
                for c in p['citations']:
                    verify_source(label, c, structured=True)
                for b in p['bodies']:
                    verify_source(label, b['citation'], b['text'], True)
                    diagnostic_bodies += 1
                diagnostic_packs += 1
            for hit in (p.get('hits') or []):
                if not hit.get('non_citable_metadata'):
                    verify_hit(label, hit)
    probe = records['diagnostics/cross-probe-complete']
    require(payload(probe['health']['response'])['serviceable'] is False and failed(probe['v2']['response']) and failed(probe['legacy-pack']['response']) and not failed(probe['keyword']['response']) and payload(probe['keyword']['response'])['hits'], 'pre-fix alignment/direct keyword gap hidden')
    require(diagnostic_packs == 16, 'diagnostic v2 denominator')
    shared = read(packet / 'shared-symbols.json')
    require(len(shared) == 2 and shared[0]['node'][1] == shared[1]['node'][1] and shared[0]['source']['sha256'] != shared[1]['source']['sha256'], 'same canonical symbol not independently resolved')
    for selected in shared:
        label, source = selected['project'], selected['source']
        f = next(f for f in fixtures if f['project'] == label)
        actual = payload(records['search-' + label]['shared-name-baseline']['response'])
        require(actual['analysis']['source'] == source and source['commit'] == PINS[label] and source['canonical_id'] == selected['node'][1] and selected['coordinates'] == f['coordinates'], 'shared function detached from selected project')
        raw = (packet / 'source-witness' / (manifests[label][('repo', source['file'])]['sha256'] + '.txt')).read_bytes()
        body = raw[source['start_byte']:source['start_byte'] + source['bytes']]
        require(len(body) == source['bytes'] and sha(body) == source['sha256'], 'shared function body from foreign project')
    require(payload(records['search-go-wemix']['foreign-wbft-node']['response'])['status'] == 'not_found', 'foreign project node admitted')
    require(payload(records['search-go-stablenet']['foreign-wbft-node']['response']).get('analysis'), 'foreign node positive control missing')
    comp = packet / 'comparison'
    calls = read(comp / 'calls.json')
    observed = [r for folder, rows in records.items() if folder.startswith('comparison/') for r in rows.values() if r['request']['method'] == 'tools/call']
    require(len(observed) == len(calls) and all(any(r['response']['result'] == c['response'] and r['request']['params']['arguments'] == c['arguments'] and r['request']['params']['name'] == c['tool'] for r in observed) for c in calls), 'comparison raw calls missing')
    cells = []
    for concept in CONCEPTS:
        matrix = read(comp / ('matrix-' + concept + '.json'))
        require(any(c['response'].get('structuredContent') == matrix for c in calls), 'matrix not actual response')
        require(len(matrix['groups']) == 2, 'comparison project missing')
        used_bytes = used_citations = 0
        for f, group in zip(fixtures, matrix['groups']):
            require(group['participant']['scope']['coordinates'] == f['coordinates'], 'comparison project mixed')
            for cell in group['cells']:
                cells.append(cell)
                require(cell['entity_identity'] is False, 'concept became entity identity')
                for mapping in cell['mappings']:
                    require(mapping['binding']['review_state'] == 'proposed' and not mapping['review']['reviewer_identity_authenticated'], 'invented mapping approval')
                    for evidence in mapping['binding']['evidence']:
                        verify_source(f['project'], evidence['reference']['citation'], evidence['body'], True)
                        used_bytes += len(evidence['body'].encode())
                        used_citations += 1
        if concept in ['staking', 'staking-policy']:
            require(all(cell['state'] == 'not_collected' and not cell['mappings'] for g in matrix['groups'] for cell in g['cells']), 'staking absence inferred')
        if concept == 'finality':
            require(matrix['groups'][0]['cells'][0]['state'] == 'unknown' and all(c['state'] == 'not_collected' for c in matrix['groups'][1]['cells']), 'finality overclaim')
        for candidate in matrix['candidates']:
            require(len(candidate['participants']) == 2 and {x['scope_id'] for x in candidate['evidence']} == set(candidate['participants']) and all(x['citations'] for x in candidate['evidence']), 'one-sided comparison proof')
            for proof in candidate['evidence']:
                group = next(g for g in matrix['groups'] if g['participant']['scope_id'] == proof['scope_id'])
                label = next(f['project'] for f in fixtures if f['coordinates'] == group['participant']['scope']['coordinates'])
                source_cites = [e['reference']['citation'] for cell in group['cells'] for m in cell['mappings'] if m['binding']['binding_digest'] == proof['binding_digest'] for e in m['binding']['evidence']]
                for citation in proof['citations']:
                    require(citation in source_cites, 'candidate citation detached from its project mapping')
                    verify_source(label, citation, structured=True)
                    used_citations += 1
        require(matrix['used_body_bytes'] == used_bytes and matrix['used_citations'] == used_citations, 'comparison budget counters')
    summary = read(comp / 'summary.json')
    require(len(cells) == 40 and sum(c['state'] == 'similar' for c in cells) == 7 and sum(c['state'] == 'unknown' for c in cells) == 18 and sum(c['state'] == 'not_collected' for c in cells) == 15, 'comparison coverage states')
    require(summary['checks'] == 22 and summary['actual_comparison_MCP_sessions'] == 3 and not summary['semantic_quality_or_independent_acceptance'], 'comparison checks/scope')
    require(len([c for c in calls if c['expected_error'] == 'request_invalid']) == 4, 'foreign request refusals absent')
    for label in ['default-reviewed-only', 'missing-B', 'bad-mapping', 'candidate-budget', 'body-budget']:
        value = read(comp / (label + '.json'))
        require(any(c['response'].get('structuredContent') == value for c in calls), 'negative evidence detached')
    require(all(not c['mappings'] for g in read(comp / 'default-reviewed-only.json')['groups'] for c in g['cells']), 'proposed mappings silently enabled')
    missing = read(comp / 'missing-B.json')
    require(not missing['candidates'] and all(c['state'] == 'unavailable' and not c['mappings'] for c in missing['groups'][1]['cells']), 'missing archive falls back')
    wrong = read(comp / 'bad-mapping.json')
    require(wrong['groups'][0]['cells'][0]['state'] == 'unavailable' and not wrong['candidates'], 'bad mapping admitted')
    require(read(comp / 'after-live-delete.json') == read(comp / 'matrix-mempool-admission.json'), 'live source fallback')
    budget = read(comp / 'candidate-budget.json')
    require(budget['used_citations'] == 2 and budget['omitted_candidates'] == 1 and budget['candidate_state'] == 'budget_exceeded' and not budget['candidates'], 'citation budget silently exceeded')
    budget = read(comp / 'body-budget.json')
    require(budget['used_body_bytes'] == budget['used_citations'] == 0 and all(g['cells'][1]['state'] == 'budget_exceeded' and not g['cells'][1]['mappings'] for g in budget['groups']), 'source body silently truncated')
    require(read(comp / 'before.json') == read(comp / 'after.json'), 'owned bytes not restored')
    preservation = read(packet / 'preservation.json')
    require(all(preservation[k] for k in ['original_and_previous_datasets_equal', 'owned_source_scope_restored', 'global_configuration_equal', 'operational_metadata_equal', 'prior_source_checkouts_equal', 'unowned_engines_preserved', 'no_owned_current', 'owned_reader_leases_released']), 'resource preservation')
    require(read(packet / 'protected-before.json') == read(packet / 'protected-after.json'), 'original dataset changed')
    require(read(packet / 'owned-before.json') == read(packet / 'owned-after.json'), 'owned dataset changed')
    before, after = read(packet / 'scope-before.json'), read(packet / 'scope-after.json')
    require(before['global'] == after['global'] and before['operations']['metadata'] == after['operations']['metadata'] and before['source_repos'] == after['source_repos'] and all(x in after['operations']['engines'] for x in before['operations']['engines']), 'scope observations differ')
    require(all(r['identity_gone'] for r in read(packet / 'process-audit.json')['probes']), 'owned process remains')
    if (packet / 'WORKLIST.json').exists():
        w = read(packet / 'WORKLIST.json')
        require(w['pilot_steps_completed'] == ['SN-P01', 'SN-P02', 'SN-P03', 'SN-P04'] and w['formal_completed'] == 0 and {r['id'] for r in w['remaining_all']} == {'FORMAL-PREP', 'FORMAL-9', 'ONTOLOGY', 'OPTIONAL-12'}, 'worklist hides unfinished acceptance')
    final_attempts = sum(read(packet / f / 'lifecycle.json')['tool_requests'] for f in final_folders)
    return {'status': 'SN-P04_scoped_real_project_comparison_pass', 'actual_tool_attempts': attempts, 'final_tool_attempts': final_attempts, 'diagnostic_tool_attempts': attempts - final_attempts, 'normal_EOF_exit0_reaped': len(folders), 'final_normal_EOF_exit0_reaped': 5, 'diagnostic_normal_EOF_exit0_reaped': len(diagnostic_folders), 'expected_startup_refusals_exit1_reaped': 2, 'paired_inputs_per_project': 6, 'raw_searches': raw_count, 'successful_raw_searches': raw_count - raw_refusals, 'expected_legacy_requires_v2_refusals': raw_refusals, 'diagnostic_initial_raw_searches': diagnostic_raw, 'v2_packs': pack_count, 'diagnostic_v2_packs': diagnostic_packs, 'citation_checks': cite_count, 'v2_bodies': body_count, 'diagnostic_v2_bodies': diagnostic_bodies, 'noncitable_metadata_hits': metadata_count, 'indexed_hint_checks': raw_hint_count, 'unique_indexed_hint_projections': len(indexed_rows), 'concepts': CONCEPTS, 'concept_checks': 22, 'foreign_pin_refusals': 6, 'foreign_node_refusals': 1, 'embedding_dimension': 1024, 'native_test': full_counts['test'], 'native_race': full_counts['race'], 'formal_completed': 0, 'formal_total': 9, 'independent_quality_acceptance': False}


def check(packet, live=True):
    try:
        manifest = read(packet / 'manifest.json')
        actual = {str(p.relative_to(packet)): sha(p.read_bytes()) for p in packet.rglob('*') if p.is_file() and p != packet / 'manifest.json'}
        require(manifest['artifacts'] == actual, 'artifact inventory/hash')
        require(manifest['checker_sha256'] == sha(Path(__file__).read_bytes()), 'checker drift')
        result = derive(packet, live)
        require(result == read(packet / 'result.json'), 'derived result differs')
        return []
    except (ValueError, KeyError, OSError, TypeError, IndexError, StopIteration, subprocess.CalledProcessError) as error:
        return [str(error)]


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--packet', type=Path, default=ROOT / REL)
    parser.add_argument('--no-live-source', action='store_true')
    args = parser.parse_args()
    errors = check(args.packet, not args.no_live_source)
    print(json.dumps({'errors': errors, 'live_source_checked': not args.no_live_source}, ensure_ascii=False))
    raise SystemExit(bool(errors))
