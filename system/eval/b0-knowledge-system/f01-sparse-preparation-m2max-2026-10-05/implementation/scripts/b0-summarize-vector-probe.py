#!/usr/bin/env python3
"""Replay diagnostic raw CKV ranks and F-01 controls; never issue final queries."""
import argparse
import importlib.util
import math
import os
from pathlib import Path
import re
import sys
import json

SPEC = importlib.util.spec_from_file_location('raw_probe_base', Path(__file__).with_name('b1-summarize-matrix.py'))
BASE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(BASE)


def require(condition, message):
    if not condition:
        raise ValueError(message)


def integer(value, minimum=0):
    return type(value) is int and value >= minimum


def digest(value):
    return isinstance(value, str) and re.fullmatch('[0-9a-f]{64}', value) is not None


def finite(value):
    return type(value) in (int, float) and math.isfinite(value)


def location(item, sources):
    require(isinstance(item.get('file'), str), 'missing source path')
    # Also reject traversal even when an attacker supplied a matching map key.
    path = Path(item['file'])
    require(not path.is_absolute() and '..' not in path.parts and '\\' not in item['file'], 'unsafe source path')
    require(item['file'] in sources, 'source outside sealed input inventory')
    lines = sources[item['file']].splitlines(keepends=True)
    require(integer(item.get('start_line'), 1) and integer(item.get('end_line'), 1)
            and item['start_line'] <= item['end_line'] <= len(lines), 'invalid source range')
    return b''.join(lines[item['start_line'] - 1:item['end_line']])


def overlap(expected, chunk):
    return (expected['file'] == chunk['file'] and expected['commit_hash'] == chunk['commit_hash']
            and expected['start_line'] <= chunk['end_line'] and chunk['start_line'] <= expected['end_line'])


def raw_metrics(expected, hits):
    # Rank is the raw candidate position, including distinct overlapping chunks.
    # Do not replace a raw slot by deduplicating composed evidence citations.
    ranks = [next((i + 1 for i, hit in enumerate(hits) if overlap(e, hit['chunk'])), None) for e in expected]
    return {'raw_ckv_source_span_recall_at_k': sum(r is not None for r in ranks) / len(expected) if expected else None,
            'raw_ckv_candidate_precision_at_k': sum(any(overlap(e, h['chunk']) for e in expected) for h in hits) / len(hits) if hits else 0.0,
            'raw_ckv_mean_expected_reciprocal_rank_at_k': sum(1/r if r else 0 for r in ranks) / len(expected) if expected else None,
            'expected_span_first_raw_ranks': ranks, 'returned_raw_candidates': len(hits)}


def summarize(probe_raw, manifest_raw, controls, source_root):
    probe, manifest = BASE.decode(probe_raw), BASE.decode(manifest_raw)
    require(controls.get('schema_version') == 1 and controls.get('diagnostic_only') is True,
            'only explicit diagnostic controls supported; official DEV/FINAL execution remains blocked')
    require(controls.get('probe_sha256') == BASE.sha(probe_raw), 'probe byte binding mismatch')
    require(probe.get('schema_version') == 1 and probe.get('gate') == 'B0-vector-probe'
            and probe.get('quality_metrics') is None, 'not a raw diagnostic probe')
    require(probe.get('state') in ('recorded', 'error'), 'nonterminal probe state')
    result = {'schema_version': 1, 'diagnostic_only': True, 'quality_metrics': None,
              'official_execution_ready': False, 'official_f01_verdict': None,
              'human_answer_policy_abstention_verdicts': None,
              'probe_sha256': BASE.sha(probe_raw), 'manifest_sha256': BASE.sha(manifest_raw),
              'planned_probe_calls': 1, 'failed_probe_calls': int(probe['state'] == 'error')}
    if probe['state'] == 'error':
        result.update(state='measurement_failed', diagnostic_scores=None, f01={'state': 'measurement_failed'})
        return result  # Missing timing/model/source data is never invented on failure.
    require(probe.get('coordinate_state') == 'pinned', 'unpinned probe cannot be source-bound')
    for key in ('coordinates', 'embedding_identity', 'prompt', 'filter', 'k', 'max_exact_candidates'):
        require(probe.get(key) == controls.get(key), 'probe/control mismatch: ' + key)
    coords = probe['coordinates']
    require(isinstance(coords.get('project_id'), str) and bool(coords['project_id'])
            and all(digest(coords.get(k)) for k in ['dataset_id', 'snapshot_id'])
            and isinstance(coords.get('commit'), str) and re.fullmatch('[0-9a-f]{40}', coords['commit']), 'invalid pinned coordinates')
    require({k: manifest.get(v) for k,v in [('project_id','project_id'), ('dataset_id','dataset_id'),
            ('snapshot_id','snapshot_id'), ('commit','src_commit')]} == coords, 'sidecar coordinates differ')
    require(manifest.get('embedding_identity_v2') == probe['embedding_identity'], 'sidecar model identity differs')
    require(probe.get('identity_verified_after') is True, 'post-query model verification missing')
    require(BASE.sha(manifest_raw) == probe.get('manifest_sha256') == probe.get('manifest_sha256_after'), 'manifest seal differs')
    require(digest(probe.get('database_sha256_before')) and probe['database_sha256_before']
            == probe.get('database_sha256_after') == manifest.get('db_sha256'), 'database seal differs')
    k, budget = probe['k'], probe['max_exact_candidates']
    require(integer(k, 1) and integer(budget), 'K/budget must be integers')
    filter_matches(probe['filter'], {'file':'__filter_validation_only__'})
    identity = probe['embedding_identity']
    vector = probe.get('query_vector')
    require(integer(identity.get('Dim'), 1) and isinstance(vector, list) and len(vector) == identity['Dim']
            and all(finite(v) for v in vector), 'invalid query vector')
    require(integer(probe.get('embed_ns')) and isinstance(probe.get('search_ns'), dict)
            and set(probe['search_ns']) == {'unfiltered','filtered','budget','oracle'}
            and all(integer(v) for v in probe['search_ns'].values()), 'invalid phase timing')
    sources, source_hashes = {}, {}
    for entry in manifest.get('input_files', []):
        file = entry['path']
        require(entry.get('origin_id') == 'repo' and file not in sources and digest(entry.get('sha256')), 'invalid source inventory')
        raw = BASE.source_bytes(source_root, file)
        require(BASE.sha(raw) == entry['sha256'], 'sealed source file hash differs')
        sources[file], source_hashes[file] = raw, BASE.sha(raw)
    require(bool(sources), 'sealed source inventory missing')
    expected = controls.get('expected_citations')
    require(isinstance(expected, list), 'explicit diagnostic expected source spans required')
    for citation in expected:
        location(citation, sources)
        require(citation.get('commit_hash') == coords['commit'], 'foreign expected commit')
    ranks, invariants = {}, []
    for name in ('unfiltered', 'filtered', 'budget'):
        search = probe[name]
        require(search.get('Status') in ('complete', 'incomplete', 'cancelled') and isinstance(search.get('Reason'), str), 'unknown search state')
        require(all(integer(search.get(v)) for v in ('CandidateCount', 'SearchedCount'))
                and (search.get('EligibleCount') is None or integer(search['EligibleCount'])), 'invalid search counts')
        require('Hits' in search, 'missing raw Hits field')
        hits = search['Hits'] if search['Hits'] is not None else []
        require(isinstance(hits, list) and len(hits) <= k, 'raw hit count exceeds K')
        ids, ordered = set(), []
        for rank, hit in enumerate(hits, 1):
            chunk, score = hit['chunk'], hit['score']
            span = location(chunk, sources)
            require(isinstance(chunk.get('id'), str) and chunk['id'] and chunk['id'] not in ids, 'duplicate/missing raw chunk ID')
            ids.add(chunk['id'])
            require(chunk.get('commit_hash') == coords['commit'], 'foreign raw chunk commit')
            require(isinstance(chunk.get('text'), str) and bool(chunk['text'])
                    and BASE.sha(chunk['text'].encode()) == chunk.get('content_sha256')
                    and chunk['text'].encode() in span, 'parser-node text/source span differs')
            require(finite(score.get('vector_distance')) and score['vector_distance'] >= 0
                    and integer(score.get('vector_rank'), 1) and score['vector_rank'] == rank
                    and finite(score.get('normalized')), 'invalid raw score/rank')
            ordered.append({'rank': rank, 'id': chunk['id'], 'file': chunk['file'], 'start_line': chunk['start_line'],
                            'end_line': chunk['end_line'], 'commit_hash': chunk['commit_hash'],
                            'distance': score['vector_distance'], 'normalized': score['normalized'],
                            'parser_text_sha256': chunk['content_sha256'], 'source_span_sha256': BASE.sha(span)})
        if search['Status'] == 'complete' and search['Reason']:
            invariants.append(name + '_complete_with_reason')
        if search['Status'] == 'cancelled':
            invariants.append(name + '_search_cancelled')
        if any(a['distance'] > b['distance'] + 1e-6 for a,b in zip(ordered, ordered[1:])):
            invariants.append(name + '_distance_order_differs')
        if name != 'unfiltered':
            for hit in hits:
                if not filter_matches(probe['filter'], hit['chunk']):
                    invariants.append(name + '_hit_outside_filter')
        ranks[name] = dict(status=search['Status'], reason=search['Reason'], candidate_count=search['CandidateCount'],
                           eligible_count=search['EligibleCount'], searched_count=search['SearchedCount'],
                           raw_candidates=ordered, diagnostic_scores=raw_metrics(expected, hits))
    oracle = probe['oracle']
    require(integer(oracle.get('eligible_count')), 'invalid exact eligible count')
    top = oracle.get('hits')
    require(isinstance(top, list) and len(top) == min(k, oracle['eligible_count']), 'invalid exact top K count')
    def validate_inventory(items):
        ids = set()
        for item in items:
            location(item, sources)
            require(isinstance(item.get('id'), str) and item['id'] and item['id'] not in ids
                    and finite(item.get('distance')) and item['distance'] >= 0, 'invalid exact inventory')
            ids.add(item['id'])
        require(items == sorted(items, key=lambda x: (x['distance'], x['id'])), 'exact distance/ID order differs')
    validate_inventory(top)
    inventory = oracle.get('eligible_hits')
    if inventory is not None:
        require(isinstance(inventory, list) and len(inventory) == oracle['eligible_count'], 'full exact inventory count differs')
        validate_inventory(inventory)
        require(inventory[:k] == top, 'exact top K differs from full inventory')
    filtered = ranks['filtered']
    agreement = (filtered['status'] == 'complete' and not filtered['reason']
                 and len(filtered['raw_candidates']) == len(top)
                 and all(all(a[v] == b[v] for v in ('id','file','start_line','end_line'))
                         and abs(a['distance'] - b['distance']) <= 1e-6
                         for a,b in zip(filtered['raw_candidates'], top)))
    # The old helper boolean compared IDs/distances only. Preserve any mismatch
    # with this stricter source-location agreement instead of trusting the flag.
    require(type(probe.get('exact_agreement')) is bool and type(probe.get('sparse_fixture_qualified')) is bool, 'missing helper flags')
    if agreement != probe['exact_agreement']:
        invariants.append('reported_exact_agreement_differs')
    if filtered['eligible_count'] is not None and filtered['eligible_count'] != oracle['eligible_count']:
        invariants.append('filtered_eligible_count_differs')
    for name in ('filtered','budget'):
        if ranks[name]['candidate_count'] < oracle['eligible_count']:
            invariants.append(name + '_prefilter_count_below_exact_eligible_count')
    sparse = any(h['file'] not in {a['file'] for a in ranks['unfiltered']['raw_candidates']} for h in top)
    if sparse != probe['sparse_fixture_qualified']:
        invariants.append('reported_sparse_helper_differs')
    f01 = {'state': 'not_requested', 'full_eligible_inventory_present': inventory is not None,
           'normal_exact_agreement': agreement, 'sparse_helper': sparse}
    contract = controls.get('f01')
    if contract is not None:
        require(integer(contract.get('k'), 1) and integer(contract.get('max_exact_candidates'))
                and integer(contract.get('eligible_count'), 1) and integer(contract.get('complete_hit_count'), 1)
                and isinstance(contract.get('eligible_files'), list) and bool(contract['eligible_files'])
                and len(set(contract['eligible_files'])) == len(contract['eligible_files']), 'invalid F-01 control inventory')
        require(contract['eligible_count'] >= len(contract['eligible_files'])
                and contract['complete_hit_count'] == min(contract['k'], contract['eligible_count']), 'inconsistent F-01 control counts')
        for file in contract['eligible_files']:
            require(file in sources, 'F-01 control file outside sealed source inventory')
        reasons = []
        if k != contract['k']: reasons.append('K_differs')
        if budget != contract['max_exact_candidates']: reasons.append('budget_differs')
        if ranks['unfiltered']['status'] != 'complete' or ranks['unfiltered']['reason']: reasons.append('unfiltered_top_K_not_complete')
        if not sparse: reasons.append('unfiltered_top_K_omits_no_exact_target_file')
        if oracle['eligible_count'] != contract['eligible_count']: reasons.append('eligible_count_differs')
        if inventory is None: reasons.append('full_eligible_inventory_missing')
        elif set(h['file'] for h in inventory) != set(contract['eligible_files']): reasons.append('full_eligible_files_differ')
        if min(k, oracle['eligible_count']) != contract['complete_hit_count']: reasons.append('complete_hit_count_differs')
        qualified = not reasons
        budget_ok = (budget > 0 and ranks['budget']['candidate_count'] > budget
                     and ranks['budget']['status'] == 'incomplete' and ranks['budget']['reason'] == 'candidate_limit')
        f01.update(state=('fixture_not_qualified' if not qualified else
                          'diagnostic_controls_pass' if agreement and budget_ok and not invariants else 'diagnostic_controls_fail'),
                   qualification_reasons=reasons, budget_incomplete_candidate_limit=budget_ok,
                   full_eligible_files=sorted({h['file'] for h in inventory}) if inventory is not None else None)
    result.update(state='replayed', coordinates=coords, embedding_identity=identity, k=k, max_exact_candidates=budget,
                  source_files_sha256=source_hashes, source_binding='sealed_manifest_files_and_parser_text_within_lines',
                  database_verification='retained_probe_before_after_seals_only; replay_does_not_open_database',
                  searches=ranks, oracle=oracle, f01=f01, invariant_violations=invariants,
                  timing_ns={'embed': probe['embed_ns'], **probe['search_ns']},
                  statistical_inference='inconclusive_single_diagnostic_query')
    return result


def filter_matches(filters, chunk):
    # Deliberately support the reviewed F-01 path shape, not a different glob
    # implementation. Other patterns need a Go Matches oracle before support.
    allowed = {'language','path','symbol_kinds','chunk_kinds','exclude_tests','commit_hash'}
    require(isinstance(filters, dict) and set(filters) <= allowed, 'unsupported filter fields')
    path = filters.get('path', '')
    require(not any(c in path for c in '?[\\') and path.count('*') <= 1, 'unsupported diagnostic glob')
    if path:
        if '*' in path:
            prefix, suffix = path.split('*')
            file = chunk['file']
            if not file.startswith(prefix) or not file.endswith(suffix): return False
            middle = file[len(prefix):len(file)-len(suffix) if suffix else len(file)]
            if '/' in middle: return False
        elif chunk['file'] != path: return False
    return (not filters.get('language') or filters['language'] == chunk.get('language')) and (
        not filters.get('symbol_kinds') or chunk.get('symbol_kind') in filters['symbol_kinds']) and (
        not filters.get('chunk_kinds') or chunk.get('chunk_kind') in filters['chunk_kinds']) and (
        not filters.get('commit_hash') or filters['commit_hash'] == chunk.get('commit_hash')) and (
        not filters.get('exclude_tests') or not chunk.get('is_test', False) and not is_test_path(chunk['file']))


def is_test_path(file):
    # Same forward-slash path rules as pkg/system/testpath.IsTest.
    base = file.split('/')[-1]
    return (base.endswith(('_test.go','.test.ts','.test.tsx','.spec.ts','.spec.tsx',
                           '.test.js','.test.jsx','.spec.js','.spec.jsx','.t.sol'))
            or base.endswith('.go') and base.startswith(('testutil','testhelper'))
            or any(p in {'test','tests','testdata','testutil','testutils','testhelpers'} for p in file.split('/')))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for arg in ('probe','manifest','controls','source-root','out'):
        parser.add_argument('--' + arg, type=Path, required=True)
    args = parser.parse_args()
    try:
        controls_raw = args.controls.read_bytes()
        result = summarize(args.probe.read_bytes(), args.manifest.read_bytes(), BASE.decode(controls_raw), args.source_root)
        result['controls_sha256'] = BASE.sha(controls_raw)
        result['reader_sha256'] = BASE.sha(Path(__file__).read_bytes())
        with open(args.out, 'x', encoding='utf-8', opener=lambda path, flags: os.open(path, flags, 0o600)) as stream:
            json.dump(result, stream, ensure_ascii=False, indent=2, allow_nan=False)
            stream.write('\n')
        return 2 if result['state'] == 'measurement_failed' or result.get('invariant_violations') or result['f01']['state'] == 'diagnostic_controls_fail' else 0
    except (ValueError, KeyError, TypeError, OSError) as err:
        print('raw vector replay refused: ' + str(err), file=sys.stderr)
        return 1


if __name__ == '__main__':
    sys.exit(main())
