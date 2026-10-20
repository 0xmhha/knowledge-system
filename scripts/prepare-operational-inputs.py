#!/usr/bin/env python3
"""Extract source-only input candidates; do not author gold or freeze a holdout."""
import argparse
import collections
import hashlib
import json
from pathlib import Path
import re
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[1]
PREFIXES = ['core/state/', 'core/rawdb/', 'core/txpool/', 'rpc/', 'p2p/', 'eth/downloader/', 'miner/', 'consensus/wbft/']

def sha(raw):
    return hashlib.sha256(raw).hexdigest()

def module(path):
    return '/'.join(path.split('/')[:2]) if path.startswith(('core/', 'eth/', 'consensus/')) else path.split('/')[0]

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--repo', required=True)
    parser.add_argument('--output', required=True)
    args = parser.parse_args()
    inventory_path = ROOT / 'docs/spec-driven/OPERATIONAL-ACCEPTANCE-SOURCE-INVENTORY.json'
    inventory = json.loads(inventory_path.read_text())
    revision = inventory['repositories']['go_stablenet']['candidate_source_commit']
    def git(*argv):
        return subprocess.check_output(['git', '-C', args.repo, *argv])
    old_path = ROOT / 'system/eval/stablenet-corpus-repair-2026-10-14/questions-frozen.json'
    old = json.loads(old_path.read_text())
    prior_text = '\n'.join(q.get('prompt', '') for q in old['questions'])
    files = []
    for row in git('ls-tree', '-r', revision).decode().splitlines():
        meta, path = row.split('\t', 1)
        if not path.endswith('.go') or path.endswith('_test.go') or not any(path.startswith(p) for p in PREFIXES):
            continue
        if any(p in path for p in ('/testdata/', '/fuzzing/', '/fuzz/')):
            continue
        files.append({'Path': path, 'Source': git('show', revision + ':' + path).decode(), 'Blob': meta.split()[2]})
    helper = ROOT / 'scripts/acceptance_ast_metadata.go.txt'
    with tempfile.TemporaryDirectory(prefix='ks-acceptance-ast-', dir='/private/tmp') as tmp:
        source = Path(tmp) / 'main.go'
        source.write_bytes(helper.read_bytes())
        result = subprocess.run(['go', 'run', str(source)], input=json.dumps(files).encode(), capture_output=True, check=True, timeout=120)
    pool = json.loads(result.stdout)
    # This only filters literal name mentions, not semantic/family leakage.
    eligible = [r for r in pool if not re.search(r'\b' + re.escape(r['name']) + r'\b', prior_text)]
    selected = []
    targets = ['assignment', 'branch', 'loop', 'short_declaration', 'return', 'repeated_short_declaration_name_hint']
    for directory in sorted({module(r['path']) for r in eligible}):
        candidates = [r for r in eligible if module(r['path']) == directory]
        if len(candidates) < 5:
            raise ValueError('insufficient source candidates in ' + directory)
        for _ in range(5):
            counts = collections.Counter(k for r in selected for k in r['syntax_features'])
            candidates.sort(key=lambda r: (-sum(counts[t] < 4 and t in r['syntax_features'] for t in targets), -len(r['syntax_features']), r['path'], r['start_line']))
            chosen = candidates.pop(0)
            chosen.update(candidate_id='SRC-' + str(len(selected)+1).zfill(3), module=directory, repository_commit=revision, gold_status='not_authored', independent_status='not_reviewed', split=None)
            selected.append(chosen)
    coverage = dict(sorted(collections.Counter(k for r in selected for k in r['syntax_features']).items()))
    reviews, expansions = [], []
    for offset in range(2):
        for directory in sorted({r['module'] for r in selected}):
            ids = [r['candidate_id'] for r in selected if r['module'] == directory]
            if len(reviews) < 10: reviews.append(ids[offset])
    for offset in range(3):
        for directory in sorted({r['module'] for r in selected}):
            if len(expansions) < 20: expansions.append([r['candidate_id'] for r in selected if r['module'] == directory][offset])
    payload = {'schema_version': 1, 'status': 'source_candidates_only_not_gold_or_holdout', 'source_inventory_sha256': sha(inventory_path.read_bytes()), 'repository_commit': revision, 'extraction': {'method': 'Go standard-library AST syntax metadata; no CKG output or model call', 'go_version': subprocess.check_output(['go', 'version'], text=True).strip(), 'helper': 'scripts/acceptance_ast_metadata.go.txt', 'helper_sha256': sha(helper.read_bytes()), 'generator': 'scripts/prepare-operational-inputs.py', 'generator_sha256': sha(Path(__file__).read_bytes()), 'source_prefixes': PREFIXES, 'source_files_parsed': len(files), 'non_generated_functions_in_span_range': len(pool), 'span_filter_lines_difference': [4,110], 'prior_name_filter': {'input': str(old_path.relative_to(ROOT)), 'sha256': sha(old_path.read_bytes()), 'prior_questions': len(old['questions']), 'functions_excluded_by_literal_name': len(pool)-len(eligible), 'semantic_family_overlap': 'not_determined'}}, 'selection': {'method': 'deterministic module-balanced syntax coverage proposal; not random sampling', 'functions': len(selected), 'modules': len({r['module'] for r in selected}), 'syntax_feature_counts': coverage, 'shadowing_hint_semantics': 'repeated := identifier name only; lexical shadowing is not proved', 'nested_closure_features': 'included_in_enclosing_function_metadata', 'final_allocation': None, 'sample_sufficiency': 'not_reviewed'}, 'functions': selected, 'public_input_templates': {'WI-1': '해당 함수의 지역 값·제어 의존과 source span을 반환하고 지원 한계·unknown을 표시한다.', 'WI-C': '해당 함수의 상태·guard·자원/비용·우선순위를 원문에 근거하여 검토하고 typed 보고서·그림을 만든다.', 'WI-6': '해당 함수에서 선언한 intent에 관련된 함수 문맥을 같은 snapshot과 cap의 baseline/확장 쌍으로 비교한다.'}, 'review_candidates': reviews, 'expansion_candidates': [{'candidate_id': c, 'intent_proposal': ['state_guard','resource_lifecycle','error_handling','value_propagation'][i%4], 'intent_relevance': 'not_reviewed'} for i,c in enumerate(expansions)], 'work_items_without_new_input_candidates': ['WI-V1','WI-V2','WI-3','WI-4','WI-2','WI-SP'], 'gold_authors': None, 'reviewers': None, 'cases_promoted_to_protocol': 0, 'runtime_executed': False}
    Path(args.output).write_text(json.dumps(payload, ensure_ascii=False, indent=2)+'\n')
    print(f'Source-only candidates: {len(selected)} functions, {len(reviews)} reviews, {len(expansions)} expansion intents; no gold, holdout or runtime execution.')

if __name__ == '__main__':
    main()
