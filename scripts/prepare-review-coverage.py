#!/usr/bin/env python3
"""Inventory exposed WI-C material and open questions without approving a rubric."""
import argparse
from collections import Counter
import importlib.util
import itertools
import json
from pathlib import Path

from source_packet import safe, sha, verify

ROOT = Path(__file__).resolve().parents[1]
BASE = Path('docs/spec-driven/operational-preparation/source-review-supplement')
REVIEW = Path('docs/spec-driven/operational-preparation/review-drafts')
INPUTS = {'pool': Path('docs/spec-driven/OPERATIONAL-ACCEPTANCE-INPUT-CANDIDATES.json'),
          'public': REVIEW / 'public-inputs.json', 'private': REVIEW / 'gold-drafts.json',
          'lexical': BASE / 'lexical-coverage-v1/coverage.json',
          'context': BASE / 'context-v1/private-context-review.json',
          'mapping': Path('scripts/fixtures/review-coverage-proposals.json')}
MANIFESTS = [BASE / name / 'manifest.json' for name in ['lexical-coverage-v1', 'context-v1']]
IDS = ['SRC-001', 'SRC-002', 'SRC-006', 'SRC-007', 'SRC-011', 'SRC-016', 'SRC-021', 'SRC-027', 'SRC-031', 'SRC-036']


def load(name):
    spec = importlib.util.spec_from_file_location(name.replace('-', '_'), ROOT / 'scripts' / name)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def dump(path, value):
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2, allow_nan=False) + '\n')


def unique(rows, field):
    result = {r[field]: r for r in rows}
    if len(result) != len(rows):
        raise ValueError('duplicate ' + field)
    return result


def source_inputs():
    data = {key: json.loads((ROOT / path).read_text()) for key, path in INPUTS.items()}
    revision = data['pool']['repository_commit']
    proofs = {name: verify(ROOT / BASE / (name + '-v1'), revision) for name in ['lexical-coverage', 'context']}
    for name in MANIFESTS:
        packet = ROOT / name.parent
        manifest = json.loads((ROOT / name).read_text())
        for artifact, digest in manifest['artifacts'].items():
            if sha(safe(packet, artifact).read_bytes()) != digest:
                raise ValueError('prior artifact binding: ' + artifact)
    # All excerpt and anchor bytes are re-derived from the offline commit/tree/blob proof.
    files = proofs['lexical-coverage']['files']
    pool = unique(data['pool']['functions'], 'candidate_id')
    for public in data['public']['cases']:
        src = public['source']
        original = pool[public['candidate_id']]
        for key in ['repository_commit', 'path', 'name', 'receiver', 'start_line', 'end_line',
                    'start_byte', 'end_byte_exclusive', 'git_blob', 'file_sha256', 'function_sha256']:
            if src[key] != original[key]:
                raise ValueError('public/source pool identity: ' + key)
        record = files[src['path']]
        raw = safe(ROOT / BASE / 'lexical-coverage-v1', record['artifact']).read_bytes()
        begin, end = src['start_byte'], src['end_byte_exclusive']
        if not 0 <= begin < end <= len(raw) or sha(raw[begin:end]) != src['function_sha256']:
            raise ValueError('function source span')
        if record['git_blob'] != src['git_blob'] or record['sha256'] != src['file_sha256']:
            raise ValueError('function file identity')
        if sha(safe(ROOT, src['excerpt_path']).read_bytes()) != src['excerpt_sha256'] or src['excerpt_sha256'] != src['function_sha256']:
            raise ValueError('public source excerpt')
    private = unique(data['private']['review_drafts'], 'candidate_id')
    for public in data['public']['cases']:
        row = private[public['candidate_id']]
        if row['source'] != public['source']:
            raise ValueError('private/public source mismatch')
        src = row['source']
        raw = safe(ROOT / BASE / 'lexical-coverage-v1', files[src['path']]['artifact']).read_bytes()
        for anchor in row['required_rubric_anchors_draft']:
            span(raw, anchor)
            if not src['start_byte'] <= anchor['start_byte'] < anchor['end_byte_exclusive'] <= src['end_byte_exclusive']:
                raise ValueError('rubric anchor outside review body')
    for row in data['context']['reviews']:
        for anchor in row['anchors']:
            record = proofs['context']['files'][anchor['path']]
            span(safe(ROOT / BASE / 'context-v1', record['artifact']).read_bytes(), anchor)
    return data


def span(raw, anchor):
    lines = raw.splitlines(keepends=True)
    start, end = anchor['start_line'], anchor['end_line']
    if not 1 <= start <= end <= len(lines):
        raise ValueError('anchor line range')
    begin, finish = sum(map(len, lines[:start-1])), sum(map(len, lines[:end]))
    if (begin, finish, sha(raw[begin:finish])) != (anchor['start_byte'], anchor['end_byte_exclusive'], anchor['span_sha256']):
        raise ValueError('anchor byte/hash binding')


def generate(data):
    public = unique(data['public']['cases'], 'candidate_id')
    private = unique(data['private']['review_drafts'], 'candidate_id')
    context = unique(data['context']['reviews'], 'candidate_id')
    mapping = unique(data['mapping'], 'candidate_id')
    lexical = unique(data['lexical']['functions'], 'candidate_id')
    if sorted(public) != IDS or set(private) != set(public) or set(mapping) != set(public):
        raise ValueError('review/mapping case inventory')
    if data['public']['gold_included'] is not False or data['public']['input_candidate_pool_sha256'] != sha((ROOT / INPUTS['pool']).read_bytes()):
        raise ValueError('public gold/pool boundary')
    if data['private']['independent_gold_records'] != 0 or data['private']['approved_cases'] != 0 or data['private']['runtime_executions'] != 0:
        raise ValueError('private draft promotion')
    if set(context) != {'SRC-001', 'SRC-006', 'SRC-016', 'SRC-021', 'SRC-036'}:
        raise ValueError('existing context inventory')
    initial = data['pool']['review_candidates']
    # Initial pool metadata is historical. Actual public packet already records this replacement.
    initial_ids = {r['candidate_id'] if isinstance(r, dict) else r for r in initial}
    if initial_ids - set(public) != {'SRC-026'} or set(public) - initial_ids != {'SRC-027'}:
        raise ValueError('historical replacement inventory')
    replacement = data['public']['review_candidate_replacement']
    if replacement['excluded'] != 'SRC-026' or replacement['replacement'] != 'SRC-027':
        raise ValueError('historical replacement receipt')
    rows = []
    for cid in IDS:
        case, old, proposal = public[cid], private[cid], mapping[cid]
        anchors = unique(old['required_rubric_anchors_draft'], 'id')
        if case['approved'] is not False or case['contains_gold'] is not False or case['split'] is not None:
            raise ValueError('public approval/split promotion')
        if old['approved'] is not False or old['independence_verified'] is not False or old['review_receipt'] is not None or old['all_defects_or_dependencies_enumerated'] is not False:
            raise ValueError('review approval/exhaustiveness promotion')
        if old['criticality_and_priority'] != 'needs_human_requirements_review':
            raise ValueError('priority already adjudicated; review protocol')
        if lexical[cid]['scope'] != 'production_source_candidate':
            raise ValueError('nonproduction review case')
        categories = {}
        used = set()
        for category in ['state_guard', 'resources_cost']:
            numbers = proposal[category]
            refs = [f'{cid}-R{n}' for n in numbers]
            if not refs or len(refs) != len(set(refs)) or not set(refs) <= anchors.keys():
                raise ValueError('invalid category anchor mapping')
            used.update(refs)
            categories[category] = {'status': 'ai_source_material_mapping_not_required_item_gold',
                                    'anchor_ids': refs, 'approved': False}
        if used != anchors.keys() or not proposal['priority_question'] or not proposal['resource_scope']:
            raise ValueError('unmapped anchor or missing scope/question')
        categories['priority'] = {'status': 'needs_human_requirements_review', 'assigned_level': None,
                                  'required_items': None, 'question': proposal['priority_question'], 'approved': False}
        ctx = context.get(cid)
        rows.append({'candidate_id': cid, 'case_id': case['case_id'], 'module': lexical[cid]['module'],
                     'source': case['source'], 'existing_ai_anchors': list(anchors.values()),
                     'categories_proposal': categories, 'resource_scope_limit': proposal['resource_scope'],
                     'existing_unknowns': old['unknowns_and_scope_limits'],
                     'context_supplement': {'same_case_not_new_review': True, 'available': ctx is not None,
                         'anchors': ctx['anchors'] if ctx else [], 'remaining_unknowns': ctx['remaining_unknowns'] if ctx else [],
                         'unsupported_conclusions': ctx['unsupported_conclusions'] if ctx else []},
                     'exposure': 'public_source_and_private_ai_drafts_already_exposed',
                     'independent_family': None, 'independence_status': 'not_reviewed', 'split': None,
                     'approved': False, 'reviewer': None, 'review_receipt': None,
                     'required_items_gold': None, 'gold_exhaustive': False,
                     'typed_report_artifact': None, 'diagram_artifact': None, 'state_handoff_receipt': None,
                     'host_execution_receipt': None, 'quality_scoring_allowed': False})
    pairs = [{'a': a['candidate_id'], 'b': b['candidate_id'], 'same_module': a['module'] == b['module'],
              'same_source_file': a['source']['path'] == b['source']['path'],
              'semantic_independence': 'not_reviewed'} for a, b in itertools.combinations(rows, 2)]
    counts = {'public_case_proposals': len(rows), 'module_labels': len({r['module'] for r in rows}),
              'source_files': len({r['source']['path'] for r in rows}),
              'existing_ai_rubric_anchors': sum(len(r['existing_ai_anchors']) for r in rows),
              'same_case_context_supplements': len(context),
              'existing_context_anchors': sum(len(r['anchors']) for r in context.values()),
              'state_guard_material_mappings': len(rows), 'resources_cost_material_mappings': len(rows),
              'priority_pending': len(rows), 'case_pairs': len(pairs),
              'same_module_pairs': sum(p['same_module'] for p in pairs),
              'same_file_pairs': sum(p['same_source_file'] for p in pairs),
              'approved_independent_reviews': 0, 'independent_gold_records': 0,
              'formal_runs': 0, 'new_kernel_calls': 0, 'mcp_calls': 0, 'model_calls': 0}
    summary = {'kind': 'review_material_inventory_not_independent_rubric_or_acceptance',
               'source_commit': data['pool']['repository_commit'], 'counts': counts,
               'module_distribution': dict(sorted(Counter(r['module'] for r in rows).items())),
               'historical_replacement': replacement,
               'material_scope': '10 public case proposals; five attached context supplements; not 15 independent reviews',
               'categories_are_ai_proposals': True, 'required_item_completeness': 'not_reviewed',
               'formal_sample_sufficiency': 'not_reviewed', 'quality_scores': None,
               'remaining': ['independently review case/family identity and exposure; freeze DEV/FINAL and sufficiency',
                             'review source contracts and all required state/guard and resource/cost items',
                             'resolve each priority question using requirements and impact; freeze rubric and gold',
                             'collect typed report, diagram, source hashes, state handoff and host execution receipts',
                             'adjudicate operational, quality and independent acceptance after protocol freeze']}
    # This is a verbatim public copy. No category mapping, questions, private anchors or answer hints are injected.
    return {'public-inputs.json': data['public'], 'private-coverage.json': {'status': 'ai_draft_not_independent_gold', 'cases': rows},
            'pairwise-review.json': pairs, 'coverage-summary.json': summary}


def table(artifacts):
    lines = ['WI-C 검토 자료 점검 — 비공개 AI 초안; 독립 정답·수용 아님',
             '입력 10개 / 모듈 표기 8개 / 파일 9개 / 기존 근거 45곳 / 같은 사례 문맥 5개·19곳',
             '상태/guard·자원/비용 연결은 자료 위치 제안이며 필수 항목의 충분성 검증이 아니다.',
             '우선순위 10개 전부 미정. 독립 리뷰·gold·typed 보고서·그림·handoff·host 실행 증거는 미확보.']
    for row in artifacts['private-coverage.json']['cases']:
        src, cat, ctx = row['source'], row['categories_proposal'], row['context_supplement']
        lines += ['', f"{row['candidate_id']} {row['module']} {src['path']}:{src['start_line']}",
                  '상태/guard 근거 제안: ' + ', '.join(cat['state_guard']['anchor_ids']),
                  '자원/비용 근거 제안: ' + ', '.join(cat['resources_cost']['anchor_ids']),
                  row['resource_scope_limit'], f"기존 외부 문맥: {len(ctx['anchors'])}곳 (동일 사례)",
                  '판정 질문: ' + cat['priority']['question']]
    return '\n'.join(lines) + '\n'


def bindings(data):
    return sorted(set(INPUTS.values()) | set(MANIFESTS) | {Path(c['source']['excerpt_path']) for c in data['public']['cases']})


def prepare(output):
    if output.exists():
        raise ValueError('use a new output; preserve prior evidence')
    data = source_inputs()
    artifacts = generate(data)
    output.mkdir(parents=True)
    for name, value in artifacts.items():
        dump(output / name, value)
    (output / 'REVIEW-TABLE.txt').write_text(table(artifacts))
    dump(output / 'formal-state.json', load('prepare-source-review-supplement.py').formal_state(ROOT))
    raw = Path(__file__).read_bytes()
    (output / 'generator.py.txt').write_bytes(raw)
    dump(output / 'manifest.json', {'schema_version': 1, 'kind': 'review_coverage_preparation_only',
         'input_bindings': {str(path): sha((ROOT / path).read_bytes()) for path in bindings(data)},
         'generator_sha256': sha(raw), 'counts': artifacts['coverage-summary.json']['counts'],
         'artifacts': {p.name: sha(p.read_bytes()) for p in sorted(output.iterdir()) if p.is_file()}})
    print(json.dumps(artifacts['coverage-summary.json']['counts']))


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output', required=True, type=Path)
    prepare(parser.parse_args().output.resolve())
