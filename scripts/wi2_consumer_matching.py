"""WI2 source refinements and retained consumer adapters; no semantic approval."""
from collections import Counter, defaultdict
import copy
import gzip
import json
from pathlib import Path
import subprocess
import wi2_dev_gold as old
import wi1_consumer_matching as coordinates

f = old.flow
ROOT, BASE = old.ROOT, old.BASE
PACKET = BASE/'followups/wi2-consumer-matching-v1'
REVISION = '0993202df9799196540a078fd4020b661359c4fb'
KIND = 'WI2_consumer_context_AI_DEV_not_formal_execution'
TOOLS = ['scripts/'+n for n in ('wi2_context_audit.go.txt', 'wi2_consumer_matching.py',
         'prepare-wi2-consumer-matching.py', 'verify-wi2-consumer-runtime.py',
         'check-wi2-consumer-matching.py', 'test_wi2_consumer_matching.py',
         'wi2_dev_gold.py', 'wi1_dev_gold.py', 'wi1_consumer_matching.py',
         'wi1_dev_source.go.txt', 'source_packet.py', 'independent_acceptance.py',
         'check-operational-acceptance.py', 'test_operational_acceptance.py',
         'fixtures/wi2-context-expectations.json')]


def audit_inputs(root=ROOT):
    inputs, _ = old.source_inputs(root)
    by_file = {i['Path']: i for i in inputs}
    for caller in f.read(root/old.CALLERS):
        i = by_file[caller['path']]
        i['Starts'] = sorted(set(i['Starts']+[caller['function_site']['start_byte']]))
    return inputs


def readonly_captures(observation, audit):
    """Only immutable scalar parameters, no writes or address exposure anywhere."""
    eligible = []
    for binding in observation['bindings']:
        accesses = [a for a in audit['accesses'] or [] if a['binding'] == binding['id']]
        if (binding['origin'] == 'parameter' and binding['value_kind'] == 'scalar'
                and binding['flags'] == ['captured_by_closure']
                and any(a['closure_owner'] for a in accesses)
                and not any(a['mutation'] or a['address'] for a in accesses)):
            eligible.append(dict(binding=binding['id'], declaration=binding['declaration'],
                                 accesses=accesses, meaning='source read-only capture; no runtime value/attacker claim'))
    return eligible


def refined(public, original, observation, audit):
    evidence = readonly_captures(observation, audit)
    revised = copy.deepcopy(observation)
    proven = {r['binding'] for r in evidence}
    for b in revised['bindings']:
        if b['id'] in proven:
            b['flags'] = []
    # Source-only derivation. Consumer responses never feed these labels.
    result = old.draft_case(public, revised, audit)
    call = next(c for c in audit['calls'] if c['span'] == public['call_span'])
    owner = call['closure_owner']
    excluded = [r for r in audit['returns'] if r['closure_depth'] == call['closure_depth']
                and r['closure_owner'] != owner]
    # Stage46 used depth alone; exclude sibling literal returns in this overlay.
    bad_spans = {tuple(r['span'].values()) for r in excluded}
    if excluded:
        raise ValueError('new sibling-closure universe requires explicit expanded review, not silent dropping')
    previous = {r['id']: r for r in original['universe']}
    rows = []
    for row in result['universe']:
        was = previous[row['id']]
        rows.append(dict(unit_id=row['id'], previous_review_label=was['review_label'],
                         review_label=row['review_label'], contract_label=row['contract_label'],
                         rationale=row['rationale'], flow=row['flow'], scope_unknown_reasons=row['scope_unknown_reasons'],
                         callee_status=row['callee_status'], safety_verdict='unknown',
                         runtime_reachability='not_established', attacker_control='not_established'))
    return dict(case_id=public['case_id'], source=public['source'], call_span=public['call_span'],
                rule_id=public['rule_id'], source_only=True, readonly_capture_evidence=evidence,
                lexical_closure_owner=owner, excluded_sibling_returns=len(bad_spans),
                units=rows, independent_review=None, semantic_gold_complete=False)


def caller_context(root, audits):
    inputs = {i['Path']: i['Source'].encode() for i in audit_inputs(root)}
    by_key = {(s['path'], s['span']['start']): s for s in audits}
    result = []
    for caller in f.read(root/old.CALLERS):
        raw = inputs[caller['path']]
        start, end = caller['site']['start_byte'], caller['site']['end_byte_exclusive']
        assert raw[start:end].decode() == caller['source_text']
        audit = by_key[caller['path'], caller['function_site']['start_byte']]
        call = next(c for c in audit['calls'] if c['span'] == dict(start=start, end=end))
        assert call['member'] == caller['target']
        direct_import = call['import'] == 'github.com/ethereum/go-ethereum/internal/build' and not call['owner_binding']
        result.append(dict(id=caller['id'], project='go-stablenet', source_commit=f.SOURCE_COMMIT,
                           path=caller['path'], file_sha256=f.sha(raw), call_span=call['span'],
                           call_sha256=f.sha(raw[start:end]), function_span=audit['span'],
                           target=call['member'], declared_import=call['import'], owner_binding=call['owner_binding'],
                           target_context='unshadowed_declared_build_import_not_package_type_or_link_proof' if direct_import else 'unknown',
                           first_argument=caller['first_argument'], closure_owner=call['closure_owner'],
                           literal_at_this_site_only=caller['first_argument']['kind'] == 'string_literal',
                           caller_to_callee='argument0 -> cmd parameter -> exec.Command name slot; wrapper-local review only',
                           transitively_proven=False, attacker_control='not_established', runtime_reachability='not_established',
                           safety_verdict='unknown', added_independent_cases=0, review_receipt=None))
    return result


def loc(value):
    return dict(start=value['start_byte'], end=value['end_byte'])


def response_identity(public, response, expected_project, adapter):
    """Project/dataset envelope then commit/file/function/graph coordinates."""
    if response['project'] != expected_project:
        raise ValueError('cross-project response')
    args = response['arguments']['function']
    result = response['response']
    analysis = result.get('analysis')
    if analysis is None:
        return False
    s, actual = public['source'], analysis['source']
    expected_commit = s['source_commit'] if expected_project == 'go-stablenet' else adapter['commit']
    if (args['expected_commit'] != expected_commit or actual['commit'] != expected_commit
            or actual['file'] != s['path'] or actual['sha256'] != s['function_sha256']
            or actual['start_byte'] != s['start'] or actual['bytes'] != s['end']-s['start']
            or args['expected_source_sha256'] != s['function_sha256']
            or args['expected_snapshot_sha256'] != actual['snapshot_sha256']
            or result['snapshot_sha256'] != actual['snapshot_sha256']):
        raise ValueError('source/pin/function coordinate identity differs')
    if expected_project != 'go-stablenet':
        adapted = next(a for a in adapter['adapters'] if a['path'] == s['path'])
        if adapted['original_file_sha256'] != s['file_sha256'] or adapted['byte_offset_delta'] != 0 or actual['node_id'] != args['node_id']:
            raise ValueError('synthetic file adapter differs')
    return True


def map_function(public_rows, golds, observation, syntax, response, adapter):
    """One whole-function response; exact source/sink identity, never names alone."""
    units = [(p, u) for p in public_rows for u in golds[p['case_id']]['universe']]
    expected_project = public_rows[0]['source']['project']
    if response['response'] is None:
        return dict(function_id=response['function_id'], status='consumer_symbol_unavailable_unknown',
                    candidates=[], units=[dict(case_id=p['case_id'], unit_id=u['id'],
                       consumer_presence=None, status='consumer_symbol_unavailable_unknown', candidate_ids=[])
                       for p, u in units], unmatched_candidates=[], coordinate_mapping=None)
    known = response_identity(public_rows[0], response, expected_project, adapter)
    analysis = response['response']['analysis']
    if not known:
        return dict(function_id=response['function_id'], status='analysis_unavailable_unknown',
                    candidates=[], units=[dict(case_id=p['case_id'], unit_id=u['id'], consumer_presence=None,
                       status='analysis_unavailable_unknown', candidate_ids=[]) for p, u in units],
                    unmatched_candidates=[], coordinate_mapping=None)
    mapping = coordinates.match(observation, analysis)
    occurrences = {o['id']: o for o in analysis['occurrences']}
    variables = {v['id']: v for v in analysis['variables']}
    nodes = {n['id']: n for n in analysis['nodes']}
    blocks = {b['id']: b for b in analysis['blocks']}
    rules = analysis['rules']
    if rules['pack_sha256'] != 'a1edcb4a8c8496c25e4e053784b93d528f17b765947961e37bb5f8d61304dd2d':
        raise ValueError('rule pack identity differs')
    candidate_rows = []
    by_unit = defaultdict(list)
    for candidate in rules['candidates']:
        source, sink = occurrences.get(candidate['source_occurrence']), occurrences.get(candidate['sink_occurrence'])
        if source is None or sink is None:
            raise ValueError('candidate occurrence absent')
        if loc(source['location']) != loc(candidate['source_location']) or loc(sink['location']) != loc(candidate['sink_location']):
            raise ValueError('candidate occurrence coordinates differ')
        variable = variables[source['variable']]
        matches = []
        for p, unit in units:
            if candidate['rule_id'] != unit['rule_id'] or loc(candidate['sink_location']) != unit['sink_span']:
                continue
            if unit['rule_id'] == old.RULE1:
                ok = (source['node'] == 'entry' and loc(variable['declaration']) == unit['source_span']
                      and loc(candidate['source_location']) == unit['source_span'])
            else:
                # Conversion call is selected by its unique assignment-result definition, not variable spelling.
                ok = unit['source_span'] == loc(candidate['source_location'])
            if ok:
                matches.append(dict(case_id=p['case_id'], unit_id=unit['id']))
        if len(matches) > 1:
            raise ValueError('ambiguous call/unit mapping')
        normalized = []
        for condition in candidate['conditions']:
            identifier = condition['node']
            controller_block = None
            if identifier in nodes:
                origins = [n for n in mapping['nodes'] if n['consumer'] == identifier]
                location_valid = condition.get('location') == nodes[identifier]['location']
                status = 'source_node' if len(origins) == 1 and location_valid else 'unknown'
                controller_block = nodes[identifier]['block']
            elif identifier.startswith('b') and identifier[1:].isdigit() and int(identifier[1:]) in blocks:
                origins = [b for b in mapping['blocks'] if b['consumer'] == int(identifier[1:])]
                status = 'structural_block' if len(origins) == 1 and condition.get('location') is None else 'unknown'
                controller_block = int(identifier[1:])
            else:
                origins, status = [], 'unknown'
            branches = [dict(reference_block=b['reference_block'], successor_ordinal=b['successor_ordinal'],
                             anchor=next(x['anchor'] for x in observation['blocks'] if x['id'] == b['reference_block']),
                             consumer_condition=b['consumer_edge'].get('condition')) for b in mapping['branches']
                        if b['consumer_edge']['from'] == 'b'+str(controller_block)
                        and b['consumer_edge'].get('condition') == condition['condition'] and b['target_matches']]
            edge_present = any(e['kind'] == 'control_dependence' and e['from'] == identifier
                               and e.get('condition') == condition['condition'] and e['to'] in candidate['path']
                               for e in analysis['edges'])
            normalized.append(dict(raw=condition, status=status, source_branches=branches,
                                   branch_matching='unique_structural_ordinal' if len(branches) == 1 and edge_present else 'unknown',
                                   edge_on_candidate_path=edge_present, reference_owners=[o['reference'] for o in origins]))
        row = dict(candidate_id=candidate['id'], rule_id=candidate['rule_id'], matches=matches,
                   source_occurrence=candidate['source_occurrence'], sink_occurrence=candidate['sink_occurrence'],
                   source_location=candidate['source_location'], sink_location=candidate['sink_location'],
                   conditions=normalized, path=candidate['path'],
                   status='mapped_unit' if matches else 'unmatched_sibling_or_outside_universe_unknown',
                   target_resolution=candidate['target_resolution'], sanitizer=candidate['sanitizer'])
        candidate_rows.append(row)
        for match in matches:
            by_unit[match['unit_id']].append(candidate['id'])
    complete = rules['status'] == 'matched_bounded' and not analysis['truncated']
    unit_rows = []
    for p, unit in units:
        observed = by_unit[unit['id']]
        status = 'returned_candidate' if observed else 'not_returned_bounded_not_safety' if complete else 'incomplete_unknown'
        unit_rows.append(dict(case_id=p['case_id'], unit_id=unit['id'], candidate_ids=observed,
                              consumer_presence=True if observed else False if complete else None, status=status))
    return dict(function_id=response['function_id'], status=rules['status'], source=analysis['source'],
                candidates=candidate_rows, units=unit_rows,
                unmatched_candidates=[c['candidate_id'] for c in candidate_rows if not c['matches']],
                coordinate_mapping=mapping)


def derive(root, packet):
    previous = root/old.PACKET
    public = f.read(previous/'public/controlled-DEV.json')+f.read(previous/'public/project-DEV.json')
    golds = {p['case_id']: f.packed(previous/'private'/(p['case_id']+'.json.gz')) for p in public}
    obs = {(o['path'], o['span']['start']): o for o in f.packed(previous/'flow-stdout.json.gz')}
    audits = f.packed(packet/'source-audit/stdout.json.gz')
    syn = {(a['path'], a['span']['start']): a for a in audits}
    overlays = [refined(p, golds[p['case_id']], obs[p['source']['path'], p['source']['start']],
                        syn[p['source']['path'], p['source']['start']]) for p in public]
    grouped = defaultdict(list)
    for p in public:
        grouped[p['source']['function_id']].append(p)
    adapter = f.read(packet/'runtime/controlled-source.json')
    actual = f.read(packet/'runtime/project-results.json')+f.read(packet/'runtime/controlled-results.json')
    mappings = []
    for response in actual:
        rows = grouped[response['function_id']]
        s = rows[0]['source']
        mappings.append(map_function(rows, golds, obs[s['path'], s['start']],
                                     syn[s['path'], s['start']], response, adapter))
    if len(mappings) != 91 or len({m['function_id'] for m in mappings}) != 91:
        raise ValueError('whole-function population differs')
    changes = [dict(case_id=o['case_id'], **u) for o in overlays for u in o['units']
               if u['previous_review_label'] != u['review_label']]
    labels = {u['unit_id']: u['review_label'] for o in overlays for u in o['units']}
    alignment = Counter(labels[u['unit_id']] for m in mappings for u in m['units'] if u['consumer_presence'] is True)
    summary = dict(kind='WI2_AI_DEV_context_and_observed_matching_not_independent_gold',
                   source_inputs=108, source_functions=91, source_units=538,
                   review_labels=dict(Counter(u['review_label'] for o in overlays for u in o['units'])),
                   source_label_changes=changes,
                   consumer_function_statuses=dict(Counter(m['status'] for m in mappings)),
                   consumer_unit_statuses=dict(Counter(u['status'] for m in mappings for u in m['units'])),
                   observed_candidates=sum(len(m['candidates']) for m in mappings),
                   returned_candidates_by_AI_source_label=dict(alignment),
                   unmatched_candidates=sum(len(m['unmatched_candidates']) for m in mappings),
                   conditions=dict(Counter(c['status'] for m in mappings for v in m['candidates'] for c in v['conditions'])),
                   condition_branches=dict(Counter(c['branch_matching'] for m in mappings for v in m['candidates'] for c in v['conditions'])),
                   caller_contexts=14, independent_approved=0, FINAL_cases=0, D04='미정',
                   formal_completed=0, formal_total=9, freeze=None, execution_allowed=False,
                   current_scores=None, semantic_gold_complete=False,
                   formal_scoring_allowed=False, external_model_calls=0)
    return dict(overlays=overlays, callers=caller_context(root, audits), mappings=mappings, summary=summary)


def historical_errors(root=ROOT):
    errors = old.historical_errors(root)
    p = root/old.PACKET
    m = f.read(p/'manifest.json')
    if subprocess.check_output(['git', 'show', REVISION+':'+str(old.PACKET/'manifest.json')], cwd=root) != (p/'manifest.json').read_bytes():
        errors.append('frozen stage46 manifest differs')
    for name, digest in m['artifacts'].items():
        if f.sha((p/name).read_bytes()) != digest:
            errors.append('frozen stage46 artifact differs '+name)
    for row in m['tools']:
        if f.sha(subprocess.check_output(['git', 'show', REVISION+':'+row['path']], cwd=root)) != row['sha256']:
            errors.append('historical stage46 tool differs '+row['path'])
    return errors


def review_ledger(root, values):
    labels = {u['unit_id']: u for o in values['overlays'] for u in o['units']}
    public = [r for name in ('controlled-DEV.json', 'project-DEV.json') for r in f.read(root/old.PACKET/'public'/name)]
    return dict(AI_source_unknown_units=[dict(case_id=o['case_id'], **u) for o in values['overlays'] for u in o['units'] if u['review_label'] == 'unknown'],
        consumer_symbol_unavailable=[dict(function_id=v['function_id'], case_ids=sorted({u['case_id'] for u in v['units']}),
             source=next(r['source'] for r in public if r['source']['function_id'] == v['function_id']))
             for v in values['mappings'] if v['status'] == 'consumer_symbol_unavailable_unknown'],
        primary_analysis_unavailable=[dict(function_id=v['function_id'], case_ids=sorted({u['case_id'] for u in v['units']}))
             for v in values['mappings'] if v['status'] == 'analysis_unavailable_unknown'],
        returned_source_disagreements=[dict(function_id=v['function_id'], **u, source_review=labels[u['unit_id']])
             for v in values['mappings'] for u in v['units'] if u['consumer_presence'] is True and labels[u['unit_id']]['review_label'] != 'positive'],
        unreturned_AI_positive=[dict(function_id=v['function_id'], **u, source_review=labels[u['unit_id']])
             for v in values['mappings'] for u in v['units'] if u['consumer_presence'] is False and labels[u['unit_id']]['review_label'] == 'positive'],
        independent_review=None, quality_scores=None, blocking_independent_gold=True,
        policy='source semantics, consumer contract observations and coverage unknowns remain separate; no silent relabeling')


def reference(root, packet):
    return dict(manifest=str(PACKET/'manifest.json'), sha256=f.sha((packet/'manifest.json').read_bytes()),
                status='WI2_AI_DEV_source_context_and_observed_matching_not_independent_gold',
                report=str(PACKET/'REPORT.txt'), worklist=str(PACKET/'WORKLIST.json'))


def current_indices(root=ROOT, protocol_override=None, ledger_override=None):
    paths = [Path('docs/spec-driven/OPERATIONAL-ACCEPTANCE-'+n+'.json') for n in ('PROTOCOL', 'PROGRESS', 'DECISION-PROPOSAL')]+[BASE/'GAPS.json']
    docs = [f.read(root/p) for p in paths]
    if protocol_override is not None:
        docs[0] = protocol_override
    if ledger_override is not None:
        docs[1] = ledger_override
    ref = reference(root, root/PACKET)
    state = f.read(root/PACKET/'WORKLIST.json')
    current = dict(kind=KIND, manifest=ref['manifest'], manifest_sha256=ref['sha256'])
    original = json.loads(subprocess.check_output(['git', 'show', REVISION+':'+str(paths[0])], cwd=root))
    errors = []
    for d in docs:
        if d.get('wi2_consumer_matching_preparation') != ref or d.get('current_execution_state') != state or d.get('current_readiness_preparation') != current:
            errors.append('WI2 consumer current reference/state differs')
        for key in ('wi2_dev_gold_preparation', 'wi1_consumer_matching_preparation', 'wi1_dev_gold_preparation', 'independent_freeze_preparation'):
            expected = dict(original[key])
            if key == 'wi2_dev_gold_preparation':
                expected['historical_source_revision'] = REVISION
            if d.get(key) != expected:
                errors.append('historical preparation promoted '+key)
    p = docs[0]
    if p['freeze'] is not None or p['execution_allowed'] is not False or any(p['roles'].values()):
        errors.append('AI consumer matching cannot freeze/execute/assign reviewers')
    proposal = p['adoption_acceptance_decision_proposal']
    if proposal['sha256'] != f.sha((root/paths[2]).read_bytes()) or any(d.get('adoption_acceptance_decision_proposal') != proposal for d in (docs[1], docs[3])):
        errors.append('proposal link differs')
    return errors


def wire_errors(packet):
    errors = []
    for label in ('project', 'controlled'):
        base = packet/'runtime'/label
        stdin = [json.loads(line) for line in gzip.decompress((base/'mcp.stdin.jsonl.gz').read_bytes()).splitlines()]
        stdout = [json.loads(line) for line in gzip.decompress((base/'mcp.stdout.jsonl.gz').read_bytes()).splitlines()]
        requests = {r['id']: r for r in stdin}
        pairs = []
        for response in stdout:
            req = requests.get(response.get('id'), {})
            if req.get('method') == 'tools/call':
                result = response.get('result', {}).get('structuredContent')
                if result is None and not response.get('result', {}).get('isError'):
                    texts = response.get('result', {}).get('content', [])
                    if texts and texts[0].get('text'):
                        result = json.loads(texts[0]['text'])
                pairs.append((req['params'], result, response))
        for row in f.read(packet/'runtime'/(label+'-results.json')):
            if row['response'] is None:
                continue
            if sum(params == dict(name='cks.context.rule_candidates', arguments=row['arguments']) and result == row['response']
                   for params, result, _ in pairs) != 1:
                errors.append('rule response missing/duplicated raw transport '+row['function_id'])
            off = row['default_off_response']
            args = dict(row['arguments']['function'])
            args.pop('expected_snapshot_sha256')
            if not any(params == dict(name='cks.context.value_dependencies', arguments=args) and result == off for params, result, _ in pairs):
                errors.append('default-off response missing raw transport')
            if off.get('analysis') and off['analysis'].get('rules') is not None:
                errors.append('ordinary analysis enabled rule pack')
        boundaries = f.read(packet/'runtime'/(label+'-boundaries.json'))
        if len(boundaries) != 7:
            errors.append('pin/cap/error/incomplete boundary population differs')
        for b in boundaries:
            if not any(params == dict(name='cks.context.rule_candidates', arguments=b['arguments'])
                       and (raw == b['raw_response'] if 'raw_response' in b else result == b['response'])
                       for params, result, raw in pairs):
                errors.append('boundary response missing raw transport')
    return errors


def validate(root=ROOT, packet=None):
    p = packet or root/PACKET
    errors = []
    try:
        manifest = f.read(p/'manifest.json')
        actual = {x.relative_to(p).as_posix(): f.sha(x.read_bytes()) for x in p.rglob('*') if x.is_file() and x != p/'manifest.json'}
        if manifest['kind'] != KIND or manifest['previous_source_revision'] != REVISION or [r['path'] for r in manifest['tools']] != TOOLS or actual != manifest['artifacts']:
            errors.append('manifest scope/inventory differs')
        for row in manifest['tools']:
            if f.sha((root/row['path']).read_bytes()) != row['sha256']:
                errors.append('tool binding differs '+row['path'])
        errors.extend(historical_errors(root))
        from independent_acceptance import native_reuse_record
        if native_reuse_record(root)['changed_bound_files']:
            errors.append('native source binding changed')
        receipt = f.read(p/'source-audit/receipt.json')
        raw = json.dumps(audit_inputs(root), ensure_ascii=False).encode()
        if (receipt['exit'] != 0 or receipt['stdin_sha256'] != f.sha(raw)
                or receipt['helper_sha256'] != f.sha((root/'scripts/wi2_context_audit.go.txt').read_bytes())
                or receipt['stdout_sha256'] != f.sha(gzip.decompress((p/'source-audit/stdout.json.gz').read_bytes()))
                or receipt['stderr_sha256'] != f.sha((p/'source-audit/stderr.txt').read_bytes())):
            errors.append('source audit receipt differs')
        expected = derive(root, p)
        for key in ('overlays', 'callers', 'mappings'):
            if f.packed(p/'private'/(key+'.json.gz')) != expected[key]:
                errors.append('source/consumer '+key+' differs')
        if f.read(p/'SUMMARY.json') != expected['summary']:
            errors.append('summary labels/approval/denominators differ')
        if f.read(p/'REVIEW-LEDGER.json') != review_ledger(root, expected):
            errors.append('review debt erased or relabeled')
        controls = f.read(root/'scripts/fixtures/wi2-context-expectations.json')
        controls_raw = json.dumps([{k:r[k] for k in ('Path', 'Source', 'Starts')} for r in controls], ensure_ascii=False).encode()
        values = {}
        for name in ('audit', 'flow'):
            receipt = f.read(p/'context-probe'/(name+'-receipt.json'))
            raw = gzip.decompress((p/'context-probe'/(name+'.json.gz')).read_bytes())
            if receipt['exit'] != 0 or receipt['stdin_sha256'] != f.sha(controls_raw) or receipt['stdout_sha256'] != f.sha(raw):
                errors.append('context source counterexample receipt differs')
            values[name] = json.loads(raw)
        actual_counts = {r['id']:len(readonly_captures(o, a)) for r, o, a in zip(controls, values['flow'], values['audit'])}
        if actual_counts != {r['id']:r['expected_readonly_parameters'] for r in controls} or len(values['audit']) != 7 or len(values['flow']) != 7:
            errors.append('source counterexamples differ')
        errors.extend(wire_errors(p))
        retained = f.read(p/'runtime/retained-before.json')
        manifest_bytes = gzip.decompress((p/'runtime/retained-source-manifest.json.gz').read_bytes())
        capture = json.loads(manifest_bytes)
        dataset = f.read(p/'runtime/dataset-identity.json')
        if (f.sha(manifest_bytes) != retained['sources/manifest.json'] or capture['identity'] != dataset['source']
                or dataset['dataset_id'] != '7d5355a19e255a7733eddc7615c9eb4e419531babde63f184c09992c2e041ab6'
                or capture['identity']['project_id'] != 'p-3fa6368dd3f6757c6b6a86121f0a9b7d'
                or capture['identity']['source_commit'] != f.SOURCE_COMMIT):
            errors.append('retained project/dataset envelope differs')
        previous_inventory = f.read(root/old.PREVIOUS/'runtime/before.json')
        previous_queries = f.read(root/old.PREVIOUS/'runtime/project-results.json')
        pinned_graphs = {r['response']['snapshot_sha256'] for r in previous_queries}
        if retained['graph/manifest.json'] != previous_inventory['graph/manifest.json'] or retained['graph/graph.db'] != previous_inventory['graph/graph.db']:
            errors.append('retained graph differs from frozen stage45 inventory')
        for row in f.read(p/'runtime/project-results.json'):
            if row['response'] is not None and row['response']['snapshot_sha256'] not in pinned_graphs:
                errors.append('project response belongs to another retained graph snapshot')
        adapter = f.read(p/'runtime/controlled-source.json')
        if len(adapter['adapters']) != 40 or adapter['original_fragments_unchanged'] is not True:
            errors.append('controlled adapter census differs')
        inputs, _ = old.source_inputs(root)
        by_path = {i['Path']:i['Source'] for i in inputs}
        for a in adapter['adapters']:
            package = 'w'+f.sha(a['path'].encode())[:5]
            raw = by_path[a['path']].replace('package wi2dev\n', 'package '+package+'\n', 1).encode()
            if a['package'] != package or a['actual_file_sha256'] != f.sha(raw) or a['prefix_bytes'] != len(old.PREFIX.encode()):
                errors.append('controlled same-width package adapter differs')
        original_files = {r['path']:r['sha256'] for r in capture['files'] if r['origin_id'] == 'repo'}
        for row in f.read(root/old.PACKET/'public/project-DEV.json'):
            if original_files.get(row['source']['path']) != row['source']['file_sha256']:
                errors.append('retained original file envelope differs')
        runtime = f.read(p/'runtime/receipt.json')
        if runtime['independent_approved'] != 0 or runtime['current_scores'] is not None or runtime['formal_completed'] != 0 or runtime['external_model_calls'] != 0:
            errors.append('runtime promoted or external model invented')
        if any(s['exit'] != 0 for s in runtime['sessions']) or len(runtime['sessions']) != 4 or runtime['retries'] != 0:
            errors.append('normal EOF/session/retry contract differs')
        if f.read(p/'runtime/retained-before.json') != f.read(p/'runtime/retained-after.json') or runtime['original_retained_files_preserved'] is not True:
            errors.append('retained assets changed')
        binaries = f.read(p/'runtime/binaries.json')
        if binaries['cks_sha256'] != '0f96f0d527732ad414416217506dc564a5b0014d8b8f81e335f8c1c88896e54c' or binaries['ckg_sha256'] != '3f5ec649ab832b31c5db1e8e191cba6c63cead073964b51212848ed32cd19f0b':
            errors.append('retained binary identity differs')
        work = f.read(p/'WORKLIST.json')
        if work['formal_completed'] != 0 or work['formal_total'] != 9 or work['reviewer_assignment'] != '미정' or work['independent_quality_or_ontology_acceptance'] is not False or work['production_activation'] is not False or work['publication'] is not False:
            errors.append('worklist promoted to approval')
        contract = f.read(p/'matching-contract.json')
        if contract['formal_scoring_allowed'] is not False or contract['independent_review'] is not None:
            errors.append('matching proposal promoted')
    except (KeyError, ValueError, TypeError, OSError, IndexError, StopIteration, AssertionError) as exc:
        errors.append(str(exc))
    return errors
