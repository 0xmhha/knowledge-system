"""Source-grounded WI-1 DEV drafts. No candidate analysis, scoring or approval."""
from collections import Counter, defaultdict, deque
import gzip
import hashlib
import json
import subprocess
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
BASE = Path('docs/spec-driven/operational-preparation/source-review-supplement')
PACKET = BASE / 'followups/wi1-dev-gold-v1'
FUNCTIONS = BASE / 'semantic-gap-v1/private-function-gaps.json'
SOURCES = BASE / 'sample-locations-v1/observation-inputs.json'
SOURCE_COMMIT = '0bf2f4d1bfeb6605006d556957ef8c045d8f8ed8'
TOOL_PATHS = ['scripts/' + p for p in ('wi1_dev_source.go.txt', 'wi1_dev_gold.py', 'prepare-wi1-dev-gold.py',
    'check-wi1-dev-gold.py', 'test_wi1_dev_gold.py', 'check-operational-acceptance.py',
    'test_operational_acceptance.py', 'fixtures/wi1-dev-controls.json')]
SOURCE_PATHS = [str(FUNCTIONS), str(SOURCES), 'go.mod', 'go.sum',
    str(BASE / 'followups/independent-freeze-v1/manifest.json'),
    str(BASE / 'followups/stablenet-comparison-v1/source-binding.json'),
    str(BASE / 'followups/stablenet-comparison-v1/result.json')]


def canonical(value):
    return json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(',', ':')).encode()


def sha(value):
    return hashlib.sha256(value).hexdigest()


def read(path):
    return json.loads(path.read_text())


def packed(path):
    return json.loads(gzip.decompress(path.read_bytes()))


def reachable(graph, start, forbidden=()):
    blocked, seen = set(forbidden), set()
    todo = [start]
    while todo:
        node = todo.pop()
        if node in seen or node in blocked:
            continue
        seen.add(node)
        todo.extend(graph.get(node, []))
    return seen


def effects(observation):
    nodes = {n['id']: n for n in observation['nodes']}
    blocks = {b['id']: b for b in observation['blocks']}
    graph = {'entry': ['block:0']}
    for block in blocks.values():
        steps = block['nodes']
        graph['block:' + str(block['id'])] = steps[:1] or ['tail:' + str(block['id'])]
        for index, step in enumerate(steps):
            graph[step] = steps[index + 1:index + 2] or ['tail:' + str(block['id'])]
        graph['tail:' + str(block['id'])] = ['block:' + str(s) for s in block['successors']]
    live = reachable(graph, 'entry')
    definitions, uses = [], []
    for occurrence in observation['occurrences']:
        if occurrence['role'] in ('definition', 'write', 'read_write'):
            definitions.append(dict(occurrence, id='d:' + occurrence['id']))
        if occurrence['role'] in ('read', 'read_write', 'address_read'):
            uses.append(dict(occurrence, id='u:' + occurrence['id']))
    kills = defaultdict(set)
    last = {}
    for definition in sorted(definitions, key=lambda d: (d['span']['start'], d['id'])):
        if definition['owner']:
            kills[definition['binding']].add(definition['owner'])
            last[(definition['owner'], definition['binding'])] = definition['id']
    return graph, live, definitions, uses, kills, last


def path_definition_reaches(definition, use, graph, live, kills, last):
    """A nonempty CFG path from a definition to a use, with no intervening kill.

    Reads precede writes at an atomic CFG step. Returning to the same step in a
    loop is distinct from consuming its newly written definition immediately.
    """
    if not definition['owner'] or not use['owner']:
        return None
    if definition['owner'] not in live or use['owner'] not in live:
        return False
    if last[(definition['owner'], definition['binding'])] != definition['id']:
        return False
    todo, seen = list(graph[definition['owner']]), set()
    while todo:
        node = todo.pop()
        if node in seen:
            continue
        seen.add(node)
        if node == use['owner']:
            return True
        if node not in kills[definition['binding']]:
            todo.extend(graph[node])
    return False


def iterative_definitions(graph, live, definitions):
    """Separate transfer-set derivation used to cross-check path search."""
    by_owner = defaultdict(dict)
    by_id = {d['id']: d for d in definitions}
    for d in sorted(definitions, key=lambda d: (d['span']['start'], d['id'])):
        if d['owner']:
            by_owner[d['owner']][d['binding']] = d['id']
    before = {n: set() for n in graph}
    after = {n: set() for n in graph}
    predecessors = defaultdict(set)
    for node, successors in graph.items():
        for successor in successors:
            predecessors[successor].add(node)
    changed = True
    iterations = 0
    while changed:
        changed = False; iterations += 1
        if iterations > len(graph) * (len(definitions) + 1) + 1:
            raise ValueError('reference dataflow did not converge')
        for node in graph:
            if node not in live:
                continue
            incoming = set().union(*(after[p] for p in predecessors[node]))
            generated = by_owner[node]
            outgoing = {d for d in incoming if by_id[d]['binding'] not in generated} | set(generated.values())
            if incoming != before[node] or outgoing != after[node]:
                before[node], after[node] = incoming, outgoing; changed = True
    return before


def postdominators(observation):
    blocks = {b['id']: b for b in observation['blocks'] if b['live']}
    graph = {b['id']: b['successors'] or ['exit'] for b in blocks.values()}
    graph['exit'] = []
    can_exit = {node for node in graph if 'exit' in reachable(graph, node)}
    no_exit = set(graph) - can_exit
    eligible = {node for node in can_exit if not (reachable(graph, node) & no_exit)}
    pd = {node: {target for target in eligible if 'exit' not in reachable(graph, node, [target])} for node in eligible}
    # Different formulation: greatest fixed point of successor intersections.
    fixed = {node: ({'exit'} if node == 'exit' else set(eligible)) for node in eligible}
    changed = True
    while changed:
        changed = False
        for node in eligible - {'exit'}:
            value = {node} | set.intersection(*(fixed[s] for s in graph[node]))
            if value != fixed[node]:
                fixed[node] = value; changed = True
    if fixed != pd:
        raise ValueError('path-cut and fixed-point postdominators differ')
    return graph, eligible, pd


def canonical_mapping(observation):
    node_by_id = {n['id']: n for n in observation['nodes']}
    block_keys, node_keys, duplicate_count = {}, {'entry': 'entry'}, Counter()
    for block in observation['blocks']:
        fingerprint = {'kind': block['kind'], 'anchor': block['anchor'],
                       'nodes': [{k: node_by_id[n][k] for k in ('kind', 'span', 'synthetic')} for n in block['nodes']]}
        key = sha(canonical(fingerprint)); ordinal = duplicate_count[key]; duplicate_count[key] += 1
        block_key = sha(canonical([observation['function_sha256'], fingerprint, ordinal]))
        block_keys[block['id']] = block_key
        for index, raw in enumerate(block['nodes']):
            node_keys[raw] = sha(canonical([block_key, index, node_by_id[raw]['span'], node_by_id[raw]['kind'], node_by_id[raw]['synthetic']]))
    return block_keys, node_keys, sum(n - 1 for n in duplicate_count.values())


def derive_gold(observation, candidate):
    graph, live, definitions, uses, kills, last = effects(observation)
    expected = iterative_definitions(graph, live, definitions)
    variables = {v['id']: v for v in observation['bindings']}
    block_keys, node_keys, ambiguity = canonical_mapping(observation)
    by_binding = defaultdict(list)
    for definition in definitions:
        by_binding[definition['binding']].append(definition)
    binding_flags = {key: set(value['flags']) for key, value in variables.items()}
    for definition in definitions:
        if not definition['owner']:
            binding_flags[definition['binding']].add('unmapped_definition')
    rows, shadow_negatives = [], []
    for use in uses:
        binding = variables[use['binding']]
        for definition in by_binding[use['binding']]:
            reaches = path_definition_reaches(definition, use, graph, live, kills, last)
            if reaches is not None and reaches != (definition['id'] in expected.get(use['owner'], set())):
                raise ValueError('independent reaching-definition methods differ')
            reason = 'kill_free_nonempty_CFG_path' if reaches else 'no_kill_free_CFG_path'
            label = 'positive' if reaches else 'negative'
            if definition['owner'] not in live or use['owner'] not in live:
                label, reason = 'excluded', 'unmapped_or_structurally_unreachable'
            if reaches is None or binding_flags[use['binding']] or binding['value_kind'] == 'unknown' or binding['origin'] == 'constant':
                label = 'unknown'
                reason = '|'.join(sorted(binding_flags[use['binding']])) or 'unresolved_value_domain_or_compile_time_mapping'
            rows.append({'definition': definition['id'], 'use': use['id'], 'binding': use['binding'],
                         'value_domain': binding['value_kind'], 'structural_reaches': reaches,
                         'label': label, 'reason': reason})
        for other in definitions:
            if other['binding'] != use['binding'] and variables[other['binding']]['name'] == binding['name']:
                shadow_negatives.append({'definition': other['id'], 'use': use['id'],
                                         'label': 'negative', 'reason': 'different_lexical_declaration'})
    block_graph, eligible, pd = postdominators(observation)
    control_rows, control_edges = [], []
    blocks = {b['id']: b for b in observation['blocks']}
    for source in observation['blocks']:
        if not source['live'] or len(source['successors']) < 2:
            continue
        for branch, successor in enumerate(source['successors']):
            for target in observation['blocks']:
                if not target['live']:
                    continue
                label, reason = 'unknown', 'normal_exit_eligibility_unknown'
                if source['id'] in eligible and target['id'] in eligible:
                    positive = target['id'] in pd[successor] and (target['id'] == source['id'] or target['id'] not in pd[source['id']])
                    label = 'positive' if positive else 'negative'; reason = 'normal_exit_structural_path_cut'
                control_rows.append({'source': block_keys[source['id']], 'branch': branch,
                                     'target': block_keys[target['id']], 'label': label, 'reason': reason})
                if label == 'positive':
                    controller = node_keys[source['nodes'][-1]] if source['nodes'] else 'empty:' + block_keys[source['id']]
                    targets = [node_keys[n] for n in target['nodes']] or ['empty:' + block_keys[target['id']]]
                    control_edges.extend({'controller': controller, 'branch': branch, 'dependent': node} for node in targets)
    return {
        'case_id': candidate['candidate_id'], 'work_item_id': 'WI-1', 'split': 'DEV', 'formal_split': None,
        'origin': 'ai_source_grounded_reference_draft', 'independent_approved': False, 'reviewer': None,
        'formal_gold_complete': False, 'formal_scoring_allowed': False,
        'complete_within_declared_structural_universe': True,
        'scope': 'own-function partial-go-types local Var/Const binding pairs and pinned x/tools structural normal-exit CFG; not memory effects, whole-program typing, exception/short-circuit feasibility or independent semantic truth',
        'source': candidate['source'], 'project': 'go-stablenet', 'source_commit': SOURCE_COMMIT,
        'actual_retained_query_receipt': None,
        'bindings': observation['bindings'], 'occurrences': observation['occurrences'],
        'quarantined_signature_occurrences': observation['quarantined_signature_occurrences'],
        'boundaries': observation['boundaries'],
        'data_universe': rows, 'shadow_identity_negative_controls': shadow_negatives,
        'uses_without_local_definition': [{'use': u['id'], 'label': 'unknown', 'reason': 'no_local_definition_observed'} for u in uses if not by_binding[u['binding']]],
        'control_universe': control_rows, 'control_node_projection': control_edges,
        'control_nonexit_unknown_blocks': [block_keys[b['id']] for b in observation['blocks'] if b['live'] and b['id'] not in eligible],
        'matching': {'block_keys': {str(k): v for k, v in block_keys.items()}, 'node_keys': node_keys,
                     'duplicate_block_fingerprints': ambiguity, 'shared_CFG_constructor': 'golang.org/x/tools/go/cfg@v0.48.0',
                     'candidate_runtime_ID_mapping': None, 'frozen': False},
        'checks': {'data_path_vs_transfer_agreement': True, 'control_path_cut_vs_fixed_point_agreement': True,
                   'data_pair_rows': len(rows), 'data_labels': dict(Counter(r['label'] for r in rows)),
                   'shadow_negative_rows': len(shadow_negatives), 'control_pair_rows': len(control_rows),
                   'control_labels': dict(Counter(r['label'] for r in control_rows)),
                   'all_rows_labelled': True, 'independent_gold_records': 0},
    }


def validate_packet(root=ROOT, packet=None):
    packet = packet or root / PACKET
    errors = []
    try:
        manifest = read(packet / 'manifest.json')
        if (manifest['kind'] != 'WI1_exposed_DEV_complete_bounded_AI_drafts_not_formal_gold'
                or [r['path'] for r in manifest['tools']] != TOOL_PATHS
                or [r['path'] for r in manifest['sources']] != SOURCE_PATHS):
            errors.append('manifest scope or required bindings differ')
        actual = {p.relative_to(packet).as_posix(): sha(p.read_bytes()) for p in packet.rglob('*') if p.is_file() and p != packet / 'manifest.json'}
        if manifest['artifacts'] != actual:
            errors.append('artifact inventory/hash differs')
        for ref in manifest['sources'] + manifest['tools']:
            target = (root / ref['path']).resolve()
            if not target.is_relative_to(root.resolve()) or sha(target.read_bytes()) != ref['sha256']:
                errors.append('source/tool binding differs ' + ref['path'])
        candidates = read(root / FUNCTIONS)['functions']
        files = {r['Path']: r['Source'].encode() for r in read(root / SOURCES)}
        observations = {(o['path'], o['span']['start']): o for o in packed(packet / 'observations.json.gz')}
        if len(observations) != 30 or set(observations) != {(c['source']['path'], c['source']['start_byte']) for c in candidates}:
            errors.append('observation identity/count differs')
        raw_output = gzip.decompress((packet / 'observer-stdout.json.gz').read_bytes())
        receipt = read(packet / 'observer-receipt.json')
        if (json.loads(raw_output) != packed(packet / 'observations.json.gz') or sha(raw_output) != receipt['stdout_sha256']
                or receipt['exit_code'] != 0 or receipt['candidate_valueflow_imported_or_invoked'] is not False):
            errors.append('source observer receipt differs or candidate used as oracle')
        public = read(packet / 'public/DEV-inputs.json')
        if len(public) != 30 or {r['case_id'] for r in public} != {r['candidate_id'] for r in candidates}:
            errors.append('DEV identity/count differs')
        for row, candidate in zip(public, candidates):
            source = candidate['source']; raw = files[source['path']]
            if sha(raw) != source['file_sha256'] or sha(raw[source['start_byte']:source['end_byte_exclusive']]) != source['function_sha256']:
                errors.append('original source bytes differ')
            if row != public_case(candidate):
                errors.append('public DEV input differs or leaks private gold')
            excerpt = (packet / ('public/source/' + candidate['candidate_id'] + '.go.txt')).read_bytes()
            if excerpt != raw[source['start_byte']:source['end_byte_exclusive']]:
                errors.append('source excerpt differs')
            observation = observations[(source['path'], source['start_byte'])]
            if observation['function_sha256'] != source['function_sha256']:
                errors.append('CFG observation source mismatch')
            for binding in observation['bindings']:
                site = binding['declaration']
                if raw[site['start']:site['end']].decode() != binding['name']:
                    errors.append('binding declaration token/source differs')
            gold = packed(packet / ('private/' + candidate['candidate_id'] + '.json.gz'))
            if gold != derive_gold(observation, candidate):
                errors.append('incomplete or promoted AI gold ' + candidate['candidate_id'])
            if (packet / ('private/' + candidate['candidate_id'] + '-REVIEW.txt')).read_text() != render_review(gold, observation):
                errors.append('human-readable AI review differs')
        summary = read(packet / 'SUMMARY.json')
        if summary != summarize([packed(packet / ('private/' + c['candidate_id'] + '.json.gz')) for c in candidates]):
            errors.append('summary counts or approval differs')
        work = read(packet / 'WORKLIST.json')
        if (work['formal_completed'] != 0 or work['formal_total'] != 9 or work['reviewer_assignment'] != '미정'
                or work['independent_quality_or_ontology_acceptance'] is not False
                or work['production_activation'] is not False or work['publication'] is not False):
            errors.append('worklist falsely promotes DEV drafts to acceptance')
        from independent_acceptance import native_reuse_record
        if native_reuse_record(root)['changed_bound_files']:
            errors.append('native source/input changed')
    except (OSError, ValueError, KeyError, TypeError, IndexError, StopIteration) as exc:
        errors.append(str(exc))
    return errors


def render_review(gold, observation):
    lines = [gold['case_id'] + ' — AI DEV draft, NOT independently approved',
             gold['source']['path'] + ' @ ' + SOURCE_COMMIT, gold['scope'],
             'Public source: ../public/source/' + gold['case_id'] + '.go.txt', '',
             'Bindings (absolute UTF8 declaration spans, flags preserve unsupported meaning):']
    for binding in gold['bindings']:
        lines.append(json.dumps(binding, ensure_ascii=False, sort_keys=True))
    lines += ['', 'Complete definition/use pair labels, grouped by use. d/o identifiers encode source byte positions.']
    by_use = defaultdict(list)
    for row in gold['data_universe']:
        by_use[row['use']].append(row)
    for use, rows in sorted(by_use.items()):
        lines.append(use + ': ' + '; '.join(r['definition'] + '=' + r['label'] + '(' + r['reason'] + ')' for r in rows))
    lines += ['', 'Same-name shadow identity negatives:']
    lines.extend(r['definition'] + ' -> ' + r['use'] + ' negative:different_lexical_declaration' for r in gold['shadow_identity_negative_controls'])
    lines += ['', 'CFG block matching table (raw indices are only hints):']
    for block in observation['blocks']:
        lines.append(gold['matching']['block_keys'][str(block['id'])] + ' ' + json.dumps(block, ensure_ascii=False, sort_keys=True))
    lines += ['', 'Control positive/unknown branch-block pairs:']
    lines.extend(json.dumps(r, sort_keys=True) for r in gold['control_universe'] if r['label'] != 'negative')
    lines += ['All other rows of the declared live branch/target universe are negative; complete matrix is in the adjacent .json.gz.',
              'Non-exiting unknown block keys: ' + json.dumps(gold['control_nonexit_unknown_blocks']),
              'Quarantined function signatures: ' + json.dumps(gold['quarantined_signature_occurrences'], ensure_ascii=False, sort_keys=True),
              '', 'Boundaries / unresolved identifiers, NEVER negative safety gold:']
    lines.extend(json.dumps(r, ensure_ascii=False, sort_keys=True) for r in gold['boundaries'])
    lines += ['', 'Human reviewer: unassigned. Actual retained query and consumer-ID matching: not executed/frozen.']
    return '\n'.join(lines) + '\n'


def current_indices(root=ROOT, protocol_override=None, ledger_override=None):
    errors = []
    packet = root / PACKET
    reference = {'manifest': str(PACKET / 'manifest.json'), 'sha256': sha((packet / 'manifest.json').read_bytes()),
                 'status': 'DEV30_bounded_AI_drafts_not_independent_gold',
                 'report': str(PACKET / 'REPORT.txt'), 'worklist': str(PACKET / 'WORKLIST.json')}
    state = read(packet / 'WORKLIST.json')
    paths = [Path('docs/spec-driven/OPERATIONAL-ACCEPTANCE-' + name + '.json') for name in ('PROTOCOL', 'PROGRESS', 'DECISION-PROPOSAL')]
    paths.append(BASE / 'GAPS.json')
    documents = [read(root / p) for p in paths]
    if protocol_override is not None:
        documents[0] = protocol_override
    if ledger_override is not None:
        documents[1] = ledger_override
    current = {'kind': 'WI1_DEV_AI_gold_not_formal_execution', 'manifest': reference['manifest'], 'manifest_sha256': reference['sha256']}
    for document in documents:
        if document.get('wi1_dev_gold_preparation') != reference or document.get('current_execution_state') != state:
            errors.append('WI1 current reference/state differs')
        if document.get('current_readiness_preparation') != current:
            errors.append('current preparation kind or scope differs')
    proposal_ref = documents[0].get('adoption_acceptance_decision_proposal')
    if (not proposal_ref or proposal_ref['sha256'] != sha((root / paths[2]).read_bytes())
            or any(d.get('adoption_acceptance_decision_proposal') != proposal_ref for d in (documents[1], documents[3]))):
        errors.append('current decision proposal linkage differs')
    protocol = documents[0]
    if protocol['freeze'] is not None or protocol['execution_allowed'] is not False or any(protocol['roles'].values()):
        errors.append('unassigned AI DEV cannot enable formal execution or invent roles')
    historical = protocol['independent_freeze_preparation']
    previous_revision = '0f28a9a756f05cc29416ef953e4926ad24cc4ee8'
    if historical.get('historical_source_revision') != previous_revision:
        errors.append('previous authoring scope not source-pinned historical')
    if any(d.get('independent_freeze_preparation') != historical for d in documents[1:]):
        errors.append('previous authoring provenance references differ')
    previous = root / historical['manifest']
    if sha(previous.read_bytes()) != historical['sha256']:
        errors.append('previous preparation manifest changed')
    previous_manifest = read(previous)
    for name, expected in previous_manifest['artifacts'].items():
        if sha((previous.parent / name).read_bytes()) != expected:
            errors.append('frozen previous authoring artifact changed')
    for tool in previous_manifest['tool_refs']:
        body = subprocess.check_output(['git', 'show', previous_revision + ':' + tool['path']], cwd=root)
        if sha(body) != tool['sha256']:
            errors.append('previous source verifier does not match pinned Git blob')
    return errors


def public_case(candidate):
    source = candidate['source']
    return {'case_id': candidate['candidate_id'], 'work_item_id': 'WI-1', 'split': 'DEV', 'formal_split': None,
            'prior_exposure': 'already_exposed_source_not_independent_holdout', 'source_commit': SOURCE_COMMIT,
            'source': {k: source[k] for k in ('path', 'name', 'receiver', 'start_byte', 'end_byte_exclusive', 'file_sha256', 'function_sha256')},
            'source_excerpt': 'source/' + candidate['candidate_id'] + '.go.txt',
            'prompt': '주어진 함수의 지역 정의–사용과 구조적 제어 의존을 원문 좌표·선언 identity와 함께 작성한다. range·주소·클로저·미해결 타입·비종료 등 지원 한계와 unknown을 구분한다. 다른 함수/프로젝트의 동명 변수를 합치지 않는다.'}


def summarize(golds):
    return {'kind': 'complete_bounded_AI_DEV_drafts_not_independent_gold', 'functions': len(golds),
            'source_files': len({g['source']['path'] for g in golds}),
            'bindings': sum(len(g['bindings']) for g in golds), 'occurrences': sum(len(g['occurrences']) for g in golds),
            'quarantined_signature_occurrences': sum(len(g['quarantined_signature_occurrences']) for g in golds),
            'data_pairs': sum(len(g['data_universe']) for g in golds),
            'data_labels': dict(sum((Counter(r['label'] for r in g['data_universe']) for g in golds), Counter())),
            'control_pairs': sum(len(g['control_universe']) for g in golds),
            'control_labels': dict(sum((Counter(r['label'] for r in g['control_universe']) for g in golds), Counter())),
            'shadow_negatives': sum(len(g['shadow_identity_negative_controls']) for g in golds),
            'all30_structural_drafts_complete': len(golds) == 30 and all(g['complete_within_declared_structural_universe'] for g in golds),
            'semantic_gold_complete': False, 'independent_approved': 0, 'FINAL_cases': 0,
            'candidate_backend_calls': 0, 'MCP_calls': 0, 'additional_external_model_calls': 0,
            'quality_scores': None, 'formal_completed': 0, 'formal_total': 9, 'D04': '미정', 'freeze': None}
