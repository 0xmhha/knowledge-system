"""Layered source semantics and exact-coordinate consumer mappings, AI DEV only."""
from collections import Counter, defaultdict
import json
import subprocess
from pathlib import Path
import wi1_dev_gold as old
ROOT, BASE = old.ROOT, old.BASE
PACKET = BASE / 'followups/wi1-consumer-matching-v1'
REVISION = '4529b6d1615d8d88159dfe07e22b0cdf38d929cf'
TOOLS = ['scripts/' + n for n in ['wi1_type_audit.go.txt','wi1_consumer_matching.py','prepare-wi1-consumer-matching.py','check-wi1-consumer-matching.py','test_wi1_consumer_matching.py','verify-wi1-consumer-runtime.py','check-operational-acceptance.py','test_operational_acceptance.py']]
KIND = 'WI1_consumer_matching_AI_DEV_not_formal_execution'

def domain_evidence(observation, audits):
    contexts = {a['context']: {r['start']: r for r in a['rows']} for a in audits if a['path']==observation['path'] and a['start']==observation['span']['start']}
    rows=[]
    for b in observation['bindings']:
        evidence={context: values.get(b['declaration']['start']) for context,values in contexts.items()}
        rows.append(dict(binding=b['id'],name=b['name'],declaration=b['declaration'],reference_domain=b['value_kind'],contexts=evidence,
                         domain_differs=(evidence.get('partial_single_file') or {}).get('domain') != (evidence.get('isolated_function') or {}).get('domain'),
                         runtime_value_known=False, memory_contents_known=False))
    return rows


def semantic_overlay(gold):
    """Do not promote old labels. Separate lexical existence from value semantics."""
    bindings={b['id']: b for b in gold['bindings']};rows=[]
    for r in gold['data_universe']:
        b=bindings[r['binding']];reasons=[]
        if b['value_kind']=='unknown':reasons.append('partial_type_context_unknown')
        if b['origin']=='constant':reasons.append('compile_time_source_not_runtime_write')
        reasons.extend(b['flags'])
        if b['value_kind']=='reference_binding':reasons.append('reference_identity_not_memory_contents')
        if b['value_kind']=='aggregate_binding':reasons.append('aggregate_contents_or_alias_unknown')
        if r['structural_reaches'] is None:reasons.append('source_CFG_owner_unmapped')
        if r['label']=='excluded':reasons.append('outside_live_structural_universe')
        if r['label']=='unknown' and not reasons:reasons.append('legacy_unknown_not_independently_resolved')
        # Even a positive scalar lexical path does not reveal a concrete runtime value.
        rows.append(dict(definition=r['definition'],use=r['use'],binding=r['binding'],legacy_label=r['label'],
                         lexical_label=('unknown' if r['structural_reaches'] is None else 'positive' if r['structural_reaches'] else 'negative'),
                         value_flow_label='unknown' if reasons else r['label'],unknown_reasons=sorted(set(reasons)),
                         concrete_runtime_value='unknown',quality_scoring_allowed=False))
    return rows


def location(value):
    return value['start_byte'],value['end_byte']


def match(observation, analysis):
    """No equality inferred from raw block/occurrence IDs or equal spelling."""
    source=analysis['source'];assert source['sha256']==observation['function_sha256']
    assert source['file']==observation['path'] and source['start_byte']==observation['span']['start']
    refs={n['id']:n for n in observation['nodes']};nodes={};node_rows=[]
    for n in refs.values():
        candidates=[v['id'] for v in analysis['nodes'] if v['kind']==n['kind'] and v['synthetic']==n['synthetic'] and location(v['location'])==(n['span']['start'],n['span']['end'])]
        node_rows.append(dict(reference=n['id'],consumer=candidates[0] if len(candidates)==1 else None,candidates=candidates,span=n['span'],synthetic=n['synthetic']))
        if len(candidates)==1:nodes[n['id']]=candidates[0]
    nodes['entry']='entry'
    variables={};binding_rows=[]
    for b in observation['bindings']:
        candidates=[v['id'] for v in analysis['variables'] if v['name']==b['name'] and location(v['declaration'])==(b['declaration']['start'],b['declaration']['end'])]
        if len(candidates)==1:variables[b['id']]=candidates[0]
        binding_rows.append(dict(reference=b['id'],consumer=candidates[0] if len(candidates)==1 else None,candidates=candidates))
    occurrences={};occurrence_rows=[]
    def is_definition(role):return role in ('definition','constant_definition')
    for o in observation['occurrences']:
        phases=[]
        if o['role'] in ('definition','write','read_write'):phases.append('definition')
        if o['role'] in ('read','read_write','address_read'):phases.append('use')
        for phase in phases:
            candidates=[v for v in analysis['occurrences'] if v['variable']==variables.get(o['binding']) and location(v['location'])==(o['span']['start'],o['span']['end']) and is_definition(v['role'])==(phase=='definition') and (v['node']==nodes.get(o['owner']) or not o['owner'] and v['role']=='constant_definition')]
            key=('d:' if phase=='definition' else 'u:')+o['id']
            if len(candidates)==1:occurrences[key]=candidates[0]['id']
            occurrence_rows.append(dict(reference=key,consumer=candidates[0]['id'] if len(candidates)==1 else None,consumer_roles=[v['role'] for v in candidates],candidates=[v['id'] for v in candidates],reference_owner=o['owner'],phase=phase,span=o['span']))
    # Structural node sequence, block role, reachability and successor constraints;
    # empty blocks are never assigned just because their numeric index agrees.
    block_candidates={}
    for b in observation['blocks']:
        if any(n not in nodes for n in b['nodes']):block_candidates[b['id']]=set();continue
        expected=[nodes[n] for n in b['nodes']]
        block_candidates[b['id']]={c['id'] for c in analysis['blocks'] if c['kind']==b['kind'] and c['reachable']==b['live'] and [n for n in c['nodes'] if n!='entry']==expected}
    cfg=[e for e in analysis['edges'] if e['kind']=='cfg'];successors=defaultdict(list)
    for e in cfg:successors[int(e['from'][1:])].append(int(e['to'][1:]))
    ref_predecessors=defaultdict(list);consumer_predecessors=defaultdict(list)
    for b in observation['blocks']:
        for ordinal,target in enumerate(b['successors']):ref_predecessors[target].append((b['id'],ordinal))
    for parent,targets in successors.items():
        for ordinal,target in enumerate(targets):consumer_predecessors[target].append((parent,ordinal))
    changed=True
    while changed:
        changed=False
        for b in observation['blocks']:
            reduced={c for c in block_candidates[b['id']] if len(successors[c])==len(b['successors']) and all(cs in block_candidates[rs] for rs,cs in zip(b['successors'],successors[c])) and len(ref_predecessors[b['id']])==len(consumer_predecessors[c]) and all(any(cp in block_candidates[rp] and co==ro for cp,co in consumer_predecessors[c]) for rp,ro in ref_predecessors[b['id']])}
            if reduced!=block_candidates[b['id']]:block_candidates[b['id']]=reduced;changed=True
    block_rows=[dict(reference=b['id'],candidates=sorted(block_candidates[b['id']]),consumer=next(iter(block_candidates[b['id']])) if len(block_candidates[b['id']])==1 else None) for b in observation['blocks']]
    branches=[]
    for b in observation['blocks']:
        if len(b['successors'])<2 or len(block_candidates[b['id']])!=1:continue
        cb=next(iter(block_candidates[b['id']]))
        edges=[e for e in cfg if e['from']=='b'+str(cb)]
        for ordinal,ref_to in enumerate(b['successors']):
            branches.append(dict(reference_block=b['id'],successor_ordinal=ordinal,reference_target=ref_to,consumer_edge=edges[ordinal],target_matches=int(edges[ordinal]['to'][1:]) in block_candidates[ref_to]))
    return dict(nodes=node_rows,bindings=binding_rows,occurrences=occurrence_rows,blocks=block_rows,branches=branches,
                unmatched_nodes=sum(x['consumer'] is None for x in node_rows),unmatched_bindings=sum(x['consumer'] is None for x in binding_rows),unmatched_occurrence_phases=sum(x['consumer'] is None for x in occurrence_rows),unmatched_blocks=sum(x['consumer'] is None for x in block_rows))


def comparison(gold, mapping, analysis):
    occurrences={r['reference']:r['consumer'] for r in mapping['occurrences'] if r['consumer']}
    actual={(r['definition'],r['use']) for r in analysis['def_use']}
    rows=[]
    for r in semantic_overlay(gold):
        d,u=occurrences.get(r['definition']),occurrences.get(r['use'])
        exists=(d,u) in actual if d and u else None
        rows.append(dict(r,consumer_edge_present=exists,matching_status='mapped' if d and u else 'unknown',
                         lexical_agrees=None if exists is None or r['lexical_label']=='unknown' else exists==(r['lexical_label']=='positive')))
    mapped=set(occurrences.values())
    extras=[r for r in analysis['def_use'] if r['definition'] not in mapped or r['use'] not in mapped]
    blocks={gold['matching']['block_keys'][str(r['reference'])]:r['consumer'] for r in mapping['blocks'] if r['consumer'] is not None}
    branches={(gold['matching']['block_keys'][str(r['reference_block'])],r['successor_ordinal']):r['consumer_edge']['condition'] for r in mapping['branches']}
    control_actual={(e['from'],e['to'],e.get('condition','')) for e in analysis['edges'] if e['kind']=='control_dependence'}
    consumer_blocks={b['id']:b for b in analysis['blocks']};control_rows=[]
    for r in gold['control_universe']:
        source_block,target_block=blocks.get(r['source']),blocks.get(r['target']);condition=branches.get((r['source'],r['branch']))
        known=source_block is not None and target_block is not None and condition is not None
        actual_present=None
        if known:
            source_nodes=consumer_blocks[source_block]['nodes'];target_nodes=consumer_blocks[target_block]['nodes']
            controller=source_nodes[-1] if source_nodes else 'b'+str(source_block)
            dependents=target_nodes or ['b'+str(target_block)]
            present=[(controller,target,condition) in control_actual for target in dependents]
            actual_present=all(present) if any(present) else False
            if any(present) and not all(present):actual_present='mixed_node_projection'
        control_rows.append(dict(source=r['source'],branch=r['branch'],target=r['target'],reference_label=r['label'],consumer_present=actual_present,matching_status='mapped' if known else 'unknown',agrees=None if not known or r['label']=='unknown' else actual_present==(r['label']=='positive')))
    return dict(rows=rows,control_rows=control_rows,unmapped_consumer_def_use=extras,quality_score=None,independent_review=None)


def historical_errors(root):
    errors=[];packet=root/old.PACKET;m=old.read(packet/'manifest.json')
    for name,h in m['artifacts'].items():
        if old.sha((packet/name).read_bytes())!=h:errors.append('frozen DEV draft changed '+name)
    for r in m['tools']:
        body=subprocess.check_output(['git','show',REVISION+':'+r['path']],cwd=root)
        if old.sha(body)!=r['sha256']:errors.append('historical verifier differs '+r['path'])
    for r in m['sources']:
        if old.sha((root/r['path']).read_bytes())!=r['sha256']:errors.append('frozen DEV source binding differs '+r['path'])
    previous=root/BASE/'followups/independent-freeze-v1/manifest.json';authoring=old.read(previous)
    for name,h in authoring['artifacts'].items():
        if old.sha((previous.parent/name).read_bytes())!=h:errors.append('frozen authoring artifact changed '+name)
    for r in authoring['tool_refs']:
        body=subprocess.check_output(['git','show','0f28a9a756f05cc29416ef953e4926ad24cc4ee8:'+r['path']],cwd=root)
        if old.sha(body)!=r['sha256']:errors.append('historical authoring verifier differs '+r['path'])
    return errors


def reference(root,packet):
    return dict(manifest=str(PACKET/'manifest.json'),sha256=old.sha((packet/'manifest.json').read_bytes()),status='AI_DEV_semantics_and_actual_matching_not_independent_gold',report=str(PACKET/'REPORT.txt'),worklist=str(PACKET/'WORKLIST.json'))


def current_indices(root=ROOT,protocol_override=None,ledger_override=None):
    paths=[Path('docs/spec-driven/OPERATIONAL-ACCEPTANCE-'+n+'.json') for n in ('PROTOCOL','PROGRESS','DECISION-PROPOSAL')]+[BASE/'GAPS.json']
    docs=[old.read(root/p) for p in paths]
    if protocol_override is not None:docs[0]=protocol_override
    if ledger_override is not None:docs[1]=ledger_override
    ref=reference(root,root/PACKET);current=dict(kind=KIND,manifest=ref['manifest'],manifest_sha256=ref['sha256']);state=old.read(root/PACKET/'WORKLIST.json');errors=[]
    for d in docs:
        if d.get('wi1_consumer_matching_preparation')!=ref or d.get('current_readiness_preparation')!=current or d.get('current_execution_state')!=state:errors.append('WI1 matching current reference/state differs')
        prev=d.get('wi1_dev_gold_preparation',{})
        expected_prev=dict(manifest=str(old.PACKET/'manifest.json'),sha256=old.sha((root/old.PACKET/'manifest.json').read_bytes()),status='DEV30_bounded_AI_drafts_not_independent_gold',report=str(old.PACKET/'REPORT.txt'),worklist=str(old.PACKET/'WORKLIST.json'),historical_source_revision=REVISION)
        if prev!=expected_prev:errors.append('previous AI draft source binding differs')
        if d.get('independent_freeze_preparation',{}).get('historical_source_revision')!='0f28a9a756f05cc29416ef953e4926ad24cc4ee8':errors.append('previous authoring provenance differs')
    p=docs[0]
    if p['freeze'] is not None or p['execution_allowed'] is not False or any(p['roles'].values()):errors.append('AI matching cannot approve formal execution or assign roles')
    prop=docs[0]['adoption_acceptance_decision_proposal']
    if prop['sha256']!=old.sha((root/paths[2]).read_bytes()) or any(d.get('adoption_acceptance_decision_proposal')!=prop for d in (docs[1],docs[3])):errors.append('proposal link differs')
    return errors


def wire_errors(packet):
    errors=[]
    for session, rows in [('cks-project',old.read(packet/'runtime/project-results.json')),('controls',old.read(packet/'runtime/control-results.json'))]:
        base=packet/'runtime'/session
        def wire(name):
            file=base/('mcp.'+name+'.jsonl.gz')
            return [json.loads(line) for line in __import__('gzip').decompress(file.read_bytes()).splitlines()]
        requests={r['id']:r for r in wire('stdin')};responses={r['id']:r for r in wire('stdout') if 'id' in r}
        observed=[]
        if session=='cks-project' and (packet/'runtime/cks-retry').exists():
            retry=packet/'runtime/cks-retry'
            import gzip
            reqs={r['id']:r for r in [json.loads(line) for line in gzip.decompress((retry/'mcp.stdin.jsonl.gz').read_bytes()).splitlines()]}
            for line in gzip.decompress((retry/'mcp.stdout.jsonl.gz').read_bytes()).splitlines():
                r=json.loads(line);req=reqs[r['id']]
                if req['method']=='tools/call':observed.append((req['params']['arguments'],r['result']['structuredContent']))
        for key,req in requests.items():
            if req['method']!='tools/call' or not req['params']['name'].endswith('.value_dependencies'):continue
            response=responses[key].get('result',{})
            if response.get('isError'):continue
            result=response.get('structuredContent')
            if result is None and response.get('content'):result=json.loads(response['content'][0]['text'])
            observed.append((req['params']['arguments'],result))
        for row in rows+(old.read(packet/'runtime/project-attempts.json') if session=='cks-project' else []):
            if not any(result==row['response'] and (session=='controls' or args==row['arguments']) for args,result in observed):errors.append('actual response missing from raw wire '+session)
    return errors


def validate(root=ROOT,packet=None):
    p=packet or root/PACKET;errors=[]
    try:
        m=old.read(p/'manifest.json');actual={f.relative_to(p).as_posix():old.sha(f.read_bytes()) for f in p.rglob('*') if f.is_file() and f.name!='manifest.json'}
        if m['artifacts']!=actual:errors.append('artifact inventory/hash differs')
        if m['kind']!=KIND or [r['path'] for r in m['tools']]!=TOOLS:errors.append('manifest contract differs')
        for r in m['tools']+m['sources']:
            if old.sha((root/r['path']).read_bytes())!=r['sha256']:errors.append('tool/source differs '+r['path'])
        errors.extend(historical_errors(root));errors.extend(wire_errors(p))
        from independent_acceptance import native_reuse_record
        if native_reuse_record(root)['changed_bound_files']:errors.append('native source changed')
        cs=old.read(root/old.FUNCTIONS)['functions'];observed={(o['path'],o['span']['start']):o for o in old.packed(root/old.PACKET/'observations.json.gz')}
        raw=old.read(p/'runtime/project-results.json');audits=old.packed(p/'runtime/types-project.json.gz')
        if len(raw)!=30 or [r['case_id'] for r in raw]!=[c['candidate_id'] for c in cs]:errors.append('actual project census differs')
        for c,r in zip(cs,raw):
            src=c['source'];a=r['response']['analysis'];args=r['arguments'];o=observed[(src['path'],src['start_byte'])]
            if args['expected_commit']!=old.SOURCE_COMMIT or args['expected_source_sha256']!=src['function_sha256'] or args['expected_snapshot_sha256']!=r['response']['snapshot_sha256'] or a['source']['commit']!=old.SOURCE_COMMIT or a['source']['snapshot_sha256']!=args['expected_snapshot_sha256'] or a['source']['node_id']!=args['node_id'] or a['truncated']:errors.append('actual retained pins/truncation differ')
            target=p/'cases'/(c['candidate_id']+'.json.gz');expected=derive(c,o,a,audits,root)
            if old.packed(target)!=expected:errors.append('AI overlay or actual matching differs '+c['candidate_id'])
        controls=old.read(p/'runtime/control-results.json');co={o['name']:o for o in old.packed(root/old.PACKET/'controls/observations.json.gz')}
        if len(controls)!=14 or {r['name'] for r in controls}!=set(co):errors.append('synthetic controls cannot substitute project cases')
        for row in controls:
            if old.read(p/'controls'/(row['name']+'.json'))!=match(co[row['name']],row['response']['analysis']):errors.append('synthetic coordinate match differs')
        cases=[old.packed(p/'cases'/(c['candidate_id']+'.json.gz')) for c in cs]
        if old.read(p/'REVIEW-LEDGER.json')!=review_ledger(cases,observed):errors.append('review debt omitted or promoted')
        expected_summary=summarize(cases,controls)
        if old.read(p/'SUMMARY.json')!=expected_summary:errors.append('summary promotes or omits results')
        receipt=old.read(p/'runtime/receipt.json')
        contract=old.read(p/'matching-contract.json')
        if contract['formal_gold_approved'] is not False or contract['independent_review'] is not None or contract['scoring_allowed'] is not False or contract['precision_recall'] is not None:errors.append('matching contract promotes AI draft')
        prior=old.read(p/'runtime/project-attempts.json')
        if len(prior)!=30 or sum(r['response']['status']=='budget_exhausted' for r in prior)!=receipt.get('initial_budget_exhausted_calls') or receipt.get('bounded_retry_calls')!=receipt.get('initial_budget_exhausted_calls'):errors.append('initial bounded failure not preserved')
        pins=old.read(p/'runtime/binary-provenance.json')['pins']
        historical_pins=old.read(root/BASE/'followups/stablenet-comparison-v1/binary-pins.json')
        if pins!=historical_pins or receipt['graph_binary_sha256']!=pins['ckg']['sha256'] or receipt['cks_binary_sha256']!=pins['reader']['sha256']:errors.append('actual binary source identity differs')
        if len(receipt['EOF_exit_codes']) not in (3,4) or any(receipt['EOF_exit_codes']) or not receipt['original_retained_files_preserved'] or old.read(p/'runtime/before.json')!=old.read(p/'runtime/after.json'):errors.append('process/resource preservation differs')
        for n in old.read(p/'runtime/pin-negatives.json'):
            if n['response']['status']!='source_mismatch' or n['response']['analysis'] is not None:errors.append('wrong pin accepted')
        work=old.read(p/'WORKLIST.json')
        if work['formal_completed']!=0 or work['reviewer_assignment']!='미정' or work['independent_quality_or_ontology_acceptance'] or work['production_activation'] or work['publication']:errors.append('worklist promotes AI evidence')
    except (KeyError,ValueError,TypeError,OSError,AssertionError,IndexError) as exc:errors.append(str(exc))
    return errors


def derive(c,o,a,audits,root=ROOT):
    gold=old.packed(root/old.PACKET/'private'/(c['candidate_id']+'.json.gz'));mapping=match(o,a)
    return dict(case_id=c['candidate_id'],source=c['source'],scope='AI DEV supplemental proposal; not independent semantic gold',type_evidence=domain_evidence(o,audits),mapping=mapping,comparison=comparison(gold,mapping,a),consumer_status=a['status'],consumer_capability=a['capability'],consumer_diagnostics=a['diagnostics'],independent_review=None,formal_scoring_allowed=False)


def summarize(cases,controls):
    rows=[r for c in cases for r in c['comparison']['rows']]
    return dict(project_functions=len(cases),synthetic_controls=len(controls),pair_universe=len(rows),legacy_unknown=sum(r['legacy_label']=='unknown' for r in rows),lexical_labels=dict(Counter(r['lexical_label'] for r in rows)),value_flow_labels=dict(Counter(r['value_flow_label'] for r in rows)),unknown_reasons=dict(Counter(v for r in rows for v in r['unknown_reasons'])),
                type_context_domain_differences=sum(r['domain_differs'] for c in cases for r in c['type_evidence']),mapping_totals={k:sum(c['mapping'][k] for c in cases) for k in ['unmatched_nodes','unmatched_bindings','unmatched_occurrence_phases','unmatched_blocks']},
                matched_pairs=sum(r['matching_status']=='mapped' for r in rows),lexical_disagreements=sum(r['lexical_agrees'] is False for r in rows),unmapped_consumer_def_use=sum(len(c['comparison']['unmapped_consumer_def_use']) for c in cases),
                control_pairs=sum(len(c['comparison']['control_rows']) for c in cases),control_unmatched_pairs=sum(r['matching_status']=='unknown' for c in cases for r in c['comparison']['control_rows']),control_lexical_disagreements=sum(r['agrees'] is False for c in cases for r in c['comparison']['control_rows']),quality_scores=None,independent_approved=0,D04='미정',formal_completed=0,formal_total=9,freeze=None,execution_allowed=False,semantic_gold_complete=False,paid_model_calls=0)


def review_ledger(cases, observations):
    items=[]
    for c in cases:
        o=observations[c['source']['path'],c['source']['start_byte']]
        occurrences={r['id']:r for r in o['occurrences']};bindings={r['id']:r for r in o['bindings']};blocks={r['id']:r for r in o['blocks']}
        for r in c['mapping']['occurrences']:
            if r['consumer'] is None:
                source=occurrences[r['reference'][2:]]
                items.append(dict(case_id=c['case_id'],kind='unmatched_occurrence_phase',source_occurrence=source,binding=bindings[source['binding']],consumer_candidates=r['candidates'],resolution='unknown: unsupported implicit range definition or multiple source CFG owners; no negative semantic inference'))
        for r in c['mapping']['blocks']:
            if r['consumer'] is None:items.append(dict(case_id=c['case_id'],kind='unmatched_block',block=blocks[r['reference']],candidates=r['candidates'],resolution='duplicate empty unreachable source blocks; outside live control pair universe'))
        for r in c['comparison']['rows']:
            if r['lexical_agrees'] is False:items.append(dict(case_id=c['case_id'],kind='lexical_difference',pair=r,resolution='AI source model/consumer difference in select receive binding/loop lifetime; unresolved typing; semantic label remains unknown pending independent review'))
    return dict(status='AI_source_review_draft_not_independent',items=items,human_review=None,scoring_allowed=False)
