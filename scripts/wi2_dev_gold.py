"""Source-derived bounded WI2 AI DEV drafts. Never import the candidate engine."""
from collections import Counter, defaultdict
import gzip
import json
from pathlib import Path
import subprocess
import wi1_dev_gold as flow
ROOT, BASE = flow.ROOT, flow.BASE
PACKET = BASE/'followups/wi2-dev-gold-v1'
PREVIOUS = BASE/'followups/wi1-consumer-matching-v1'
REVISION = '5d84449eae68db4911a33d9a7b6bc09ca153406d'
KIND = 'WI2_complete_bounded_AI_DEV_not_formal_execution'
CONTROL = BASE/'rule-coverage-v1/public-inputs.json'
PROJECT = BASE/'project-rule-review-v1'
CENSUS = Path('docs/spec-driven/operational-preparation/policy-security-drafts/private/rule-source-census.json')
CALLERS = BASE/'rule-caller-context-v1/caller-review.json'
TOOLS = ['scripts/'+n for n in ['wi2_source_syntax.go.txt','wi2_dev_gold.py','prepare-wi2-dev-gold.py','check-wi2-dev-gold.py','test_wi2_dev_gold.py','fixtures/wi2-dev-expectations.json','wi1_dev_source.go.txt','wi1_dev_gold.py','source_packet.py','check-operational-acceptance.py','test_operational_acceptance.py']]
SOURCES = [str(CONTROL),str(CENSUS),str(CALLERS),str(PROJECT/'manifest.json'),str(PREVIOUS/'manifest.json'),'go.mod','go.sum',str(BASE/'rule-coverage-v1/contract.json'),str(BASE/'followups/stablenet-comparison-v1/source-binding.json')]
RULE1, RULE2 = 'parameter-command-name','discarded-conversion-error'
PREFIX = 'package wi2dev\nimport (exec "os/exec"; process "os/exec"; strconv "strconv"; conversion "strconv")\ntype Fake struct{}\ntype Box struct{Name string}\nfunc sanitize(string) string\nfunc clamp(int) int\n'


def source_inputs(root=ROOT):
    from source_packet import verify, safe
    proof=verify(root/PROJECT,flow.SOURCE_COMMIT)
    census=flow.read(root/CENSUS)['calls'];by_function={}
    for c in census:by_function.setdefault((c['path'],c['function_start_byte']),[]).append(c)
    inputs=[];functions=[]
    for index,((path,start),calls) in enumerate(sorted(by_function.items()),1):
        ref=proof['files'][path];raw=safe(root/PROJECT,ref['artifact']).read_bytes();c=calls[0];end=c['function_end_byte_exclusive']
        if flow.sha(raw)!=c['file_sha256'] or flow.sha(raw[start:end])!=c['function_sha256']:raise ValueError('original project source hash differs')
        for call in calls:
            if flow.sha(raw[call['start_byte']:call['end_byte_exclusive']])!=call['span_sha256']:raise ValueError('call hash differs')
        functions.append(dict(id=f'PROJECT-{index:03d}',stratum='project',path=path,start=start,end=end,function_sha256=flow.sha(raw[start:end]),file_sha256=flow.sha(raw),code=raw[start:end].decode(),calls=calls,source_commit=flow.SOURCE_COMMIT))
    starts=defaultdict(list)
    for f in functions:starts[f['path']].append(f['start'])
    for path,values in starts.items():inputs.append(dict(Path=path,Source=safe(root/PROJECT,proof['files'][path]['artifact']).read_text(),Starts=values))
    for row in flow.read(root/CONTROL)['cases']:
        code=row['code'];assert flow.sha(code.encode())==row['source_sha256'];source=PREFIX+code+'\n';path='controlled/'+row['id']+'.go'
        f=dict(id=row['id'],stratum='controlled',path=path,start=len(PREFIX.encode()),end=len((PREFIX+code).encode()),function_sha256=row['source_sha256'],file_sha256=flow.sha(source.encode()),code=code,calls=[],source_commit=None)
        functions.append(f);inputs.append(dict(Path=path,Source=source,Starts=[f['start']]))
    if len(functions)!=91 or len(census)!=68 or len(proof['files'])!=34:raise ValueError('source census differs')
    return inputs,functions


def span_key(expr):return expr['span']['start'],expr['span']['end']


def rule_of(call):
    if call['member'] in ('Command','CommandContext'):return RULE1
    if call['member'] in ('Atoi','ParseInt','ParseUint'):return RULE2
    return None


class Reference:
    """Backward equations over source definitions; opaque/memory steps stop unknown.

    Uses the pinned source observer's CFG and lexical objects, independently of
    candidate def_use/rules. Shared x/tools construction is a common-mode limit.
    """
    def __init__(self,observation,syntax):
        self.o,self.s=observation,syntax
        self.graph,self.live,self.defs,self.uses,_,_=flow.effects(observation)
        self.before=flow.iterative_definitions(self.graph,self.live,self.defs)
        self.bindings={b['id']:b for b in observation['bindings']};self.by_def={d['id']:d for d in self.defs}
        self.calls={c['span']['start']:c for c in syntax['calls']};self.writes={span_key(dict(span=w['target'])):w for w in syntax['writes']}
        self.dep={};self.unknown_details={}
        for d in self.defs:self.dep[d['id']]=self.definition(d)
        self.control_graph,self.eligible,self.pd=flow.postdominators(observation)

    def unknown(self,reason):return {'unknown:'+reason}
    def expr(self,e,at=None):
        kind=e['kind']
        if kind in ('BasicLit','zero_initialization'):return set()
        if kind=='Ident':
            if e['text'] in ('nil','true','false','_'):return set()
            matches=[u for u in self.uses if u['binding']==e['binding'] and u['span']==e['span'] and (at is None or u['owner']==at)]
            if len(matches)!=1 or not matches[0]['owner']:return self.unknown('unmapped_or_ambiguous_identifier_use')
            u=matches[0];b=self.bindings[u['binding']];defs={d for d in self.before.get(u['owner'],set()) if self.by_def[d]['binding']==u['binding']}
            result=defs or self.unknown('no_reaching_local_definition')
            if b['origin']=='range' or b['flags']:result=result|self.unknown('range_capture_or_address_boundary')
            return result
        if kind in ('ParenExpr','compound_value') or kind.startswith('BinaryExpr:') and kind not in ('BinaryExpr:&&','BinaryExpr:||') or kind in ('UnaryExpr:+','UnaryExpr:-','UnaryExpr:!','UnaryExpr:^'):
            return set().union(*(self.expr(c,at) for c in e['children']))
        if kind=='CallExpr':return self.unknown('opaque_call_result_not_identity_or_sanitizer_proof')
        return self.unknown('unsupported_value_expression:'+kind)

    def definition(self,d):
        b=self.bindings[d['binding']]
        if d['owner']=='entry':
            if b['origin']=='parameter':return {'parameter:'+b['id']}
            if b['origin'] in ('result_zero','constant'):return set()
            return self.unknown('receiver_or_entry_nonparameter')
        if not d['owner'] or d['owner'] not in self.live:return self.unknown('unmapped_or_unreachable_definition')
        w=self.writes.get((d['span']['start'],d['span']['end']))
        if w is None:return self.unknown('implicit_range_or_unmapped_write')
        e=w['expression'];call=self.calls.get(e['span']['start']) if e['kind']=='CallExpr' else None
        if call and rule_of(call)==RULE2 and w['tuple_index']==0 and w['tuple_size']==2:return {'conversion:'+str(call['span']['start'])}
        if w['tuple_size']>1 and e['kind']=='CallExpr':return self.unknown('opaque_multiple_result_assignment')
        return self.expr(e,d['owner'])

    def trace(self,expression):
        initial=self.expr(expression);todo=list(initial);seen=set();terms=set();definition_nodes=set()
        while todo:
            key=todo.pop()
            if key in seen:continue
            seen.add(key)
            if key in self.dep:
                definition_nodes.add(self.by_def[key]['owner']);todo.extend(self.dep[key])
            else:terms.add(key)
        use_nodes={u['owner'] for u in self.uses if u['span']['start']>=expression['span']['start'] and u['span']['end']<=expression['span']['end']}
        path_nodes=definition_nodes|use_nodes;conditions=[];block_by_node={n['id']:n['block'] for n in self.o['nodes']}
        dependent_blocks={block_by_node[n] for n in path_nodes if n in block_by_node}
        for block in self.o['blocks']:
            if len(block['successors'])<2 or block['id'] not in self.eligible:continue
            for ordinal,succ in enumerate(block['successors']):
                targets=sorted(t for t in dependent_blocks if t in self.eligible and t in self.pd[succ] and (t==block['id'] or t not in self.pd[block['id']]))
                if targets:conditions.append(dict(block_role=block['kind'],anchor=block['anchor'],successor_ordinal=ordinal,dependent_blocks=targets,meaning='structural normal-exit constraint; no true/false or runtime feasibility inference'))
        return dict(origins=sorted(t for t in terms if not t.startswith('unknown:')),unknown_reasons=sorted(t[8:] for t in terms if t.startswith('unknown:')),definition_spans=[dict(binding=self.by_def[k]['binding'],span=self.by_def[k]['span'],owner=self.by_def[k]['owner']) for k in sorted(seen) if k in self.by_def],conditions=conditions)


def target_status(call,rule):
    if call['owner_binding']:return 'local_object_not_declared_import_target'
    expected='os/exec' if rule==RULE1 else 'strconv'
    if call['import']==expected:return 'declared_import_unshadowed_package_types_unresolved'
    if call['import']:return 'different_declared_import'
    return 'unresolved_fragment_or_package_target'


def classify(flow_trace,marker):
    if flow_trace['unknown_reasons']:return 'unknown'
    return 'positive' if marker in flow_trace['origins'] else 'negative'


def unit_id(unit):return flow.sha(flow.canonical({k:unit[k] for k in ('rule_id','call_span','source_identity','sink_span','sink_kind','sink_binding','source_scope')}))


def draft_case(public,observation,syntax):
    ref=Reference(observation,syntax);rule=public['rule_id'];call=next(c for c in syntax['calls'] if c['span']==public['call_span']);target=target_status(call,rule)
    rows=[]
    if rule==RULE1:
        index=1 if call['member']=='CommandContext' else 0
        argument=call['arguments'][index] if len(call['arguments'])>index else dict(kind='missing_argument',span=call['span'],text='',binding='',children=[])
        parameters=[b for b in observation['bindings'] if b['origin']=='parameter'] or [None]
        for b in parameters:
            trace=ref.trace(argument);marker='parameter:'+b['id'] if b else 'no_named_parameter'
            label=classify(trace,marker);reasons=[]
            if b and b['value_kind']!='scalar':reasons.append('parameter_value_domain_not_proven_scalar')
            if b and b['flags']:reasons.append('parameter_capture_or_address_boundary')
            if argument['kind']!='Ident':reasons.append('sink_outside_direct_identifier_contract')
            if call['closure_depth']:reasons.append('closure_execution_outside_local_function')
            if call['owner']!='exec':contract='negative'
            else:contract='unknown' if reasons or label=='unknown' else label
            review=label
            if target in ('local_object_not_declared_import_target','different_declared_import'):review='negative'
            elif call['closure_depth'] or label!='negative' and (reasons or target=='unresolved_fragment_or_package_target'):review='unknown'
            row=dict(rule_id=rule,call_span=call['span'],source_identity=b['id'] if b else 'no_named_parameter',source_span=b['declaration'] if b else None,sink_span=argument['span'],sink_kind='command_name_argument',sink_binding=argument['binding'],source_value_kind=b['value_kind'] if b else None,contract_label=contract,review_label=review,callee_status=target,flow=trace,scope_unknown_reasons=reasons,empty_source_rationale='function has no named parameter' if b is None else None)
            row['source_scope']={k:public['source'][k] for k in ('path','function_sha256','file_sha256','source_commit','fragment_prefix_bytes')};row['id']=unit_id(row);rows.append(row)
    else:
        lhs=call['assignment'];discarded=len(lhs)==2 and lhs[1]['kind']=='Ident' and lhs[1]['text']=='_';first=lhs[0] if lhs else None
        returns=[r for r in syntax['returns'] if r['closure_depth']==call['closure_depth']]
        sinks=[]
        for ret in returns:
            sinks.extend((value,'explicit_return_value',ret) for value in ret['values'])
            if not ret['values']:
                results=[b for b in observation['bindings'] if b['origin']=='result_zero']
                for b in results:
                    # Use the source observer's synthetic read at the actual return span.
                    sinks.append((dict(kind='Ident',span=ret['span'],text=b['name'],binding=b['id'],children=[]),'bare_named_return',ret))
                if not results:sinks.append((dict(kind='void_return',span=ret['span'],text='',binding='',children=[]),'void_return',ret))
        if not sinks:sinks=[(dict(kind='no_return',span=call['span'],text='',binding='',children=[]),'no_return',None)]
        for value,sink_kind,ret in sinks:
            trace=ref.trace(value);marker='conversion:'+str(call['span']['start']);label=classify(trace,marker);reasons=[]
            if sink_kind!='explicit_return_value' or value['kind']!='Ident':reasons.append('sink_outside_explicit_direct_identifier_contract')
            if first is None or first['kind']!='Ident' or not first['binding']:reasons.append('result_not_local_identifier_assignment')
            if call['closure_depth']:reasons.append('closure_execution_outside_local_function')
            contract='negative' if not discarded or call['owner']!='strconv' else 'unknown' if reasons or label=='unknown' else label
            review='negative' if not discarded or target in ('local_object_not_declared_import_target','different_declared_import') else label
            if discarded and (call['closure_depth'] or label!='negative' and (reasons or target=='unresolved_fragment_or_package_target')):review='unknown'
            row=dict(rule_id=rule,call_span=call['span'],source_identity=marker,source_span=first['span'] if first else None,sink_span=value['span'],sink_kind=sink_kind,sink_binding=value['binding'],blank_second_assignment=discarded,contract_label=contract,review_label=review,callee_status=target,flow=trace,scope_unknown_reasons=reasons,empty_source_rationale='no value returned from enclosing function' if sink_kind in ('no_return','void_return') else None)
            row['source_scope']={k:public['source'][k] for k in ('path','function_sha256','file_sha256','source_commit','fragment_prefix_bytes')};row['id']=unit_id(row);rows.append(row)
    if not rows or len({r['id'] for r in rows})!=len(rows):raise ValueError('empty or duplicated rule universe')
    for r in rows:
        label=r['review_label']
        if label=='positive':rationale='The exact selected source identity has a supported local RHS/CFG dependency witness to this sink; lexical may-reach, not runtime feasibility or attacker control.'
        elif target in ('local_object_not_declared_import_target','different_declared_import'):rationale='The callee owner is a local object or a different declared import, so this declared stdlib target condition is false; not a general safety verdict.'
        elif rule==RULE2 and not r['blank_second_assignment']:rationale='The direct two-result blank-second-target pattern is absent. This does not prove that the error was checked or correctly handled.'
        elif label=='negative':rationale='No dependency from this exact selected source to this sink in the declared bounded local equations; other arguments/declarations/slots are distinct. Not a safety proof.'
        else:rationale='Unresolved bounded flow/target or unsupported consumer scope: '+','.join(sorted(set(r['scope_unknown_reasons']+r['flow']['unknown_reasons']+(['callee_context_unresolved'] if target=='unresolved_fragment_or_package_target' else []))))
        r.update(rationale=rationale,safety_verdict='unknown',attacker_control='not_established',runtime_reachability='not_established')
    sets={name:[r['id'] for r in rows if r['review_label']==label] for name,label in [('rule_candidates','positive'),('negative_locations','negative'),('unsupported_or_unknown_flows','unknown')]}
    return dict(case_id=public['case_id'],work_item_id='WI-2',rule_pack=public['rule_pack'],split='DEV',formal_split=None,source=public['source'],rule_id=rule,universe=rows,answer_sets=sets,empty_set_rationales={name:'No units with this label in the complete declared source-call universe; AI rationale, independent review pending.' for name,values in sets.items() if not values},complete_within_declared_universe=True,semantic_gold_complete=False,independent_review_receipt=None,formal_scoring_allowed=False,reviewer=None,actual_MCP_matching=None)


def public_inputs(functions,syntax):
    observed={(s['path'],s['span']['start']):s for s in syntax};public=[]
    for f in functions:
        s=observed[f['path'],f['start']];targets=[c for c in s['calls'] if rule_of(c)]
        requested=[(c['id'],next(x for x in targets if x['span']==dict(start=c['start_byte'],end=c['end_byte_exclusive'])),c) for c in f['calls']] if f['stratum']=='project' else [(f['id'],c,None) for c in targets]
        if f['stratum']=='controlled' and len(requested)!=1:raise ValueError('controlled rule call census differs')
        for identity,call,original in requested:
            source={k:f[k] for k in ('path','start','end','function_sha256','file_sha256','source_commit')};source['function_id']=f['id'];source['project']='go-stablenet' if f['stratum']=='project' else 'controlled-dev-fixture'
            source['fragment_prefix_bytes']=len(PREFIX.encode()) if f['stratum']=='controlled' else 0
            source['import_context']='explicit_AI_DEV_fixture_assumption_not_original_evidence' if f['stratum']=='controlled' else 'original_committed_full_file_partial_type_context'
            public.append(dict(case_id=identity,stratum=f['stratum'],work_item_id='WI-2',rule_pack='go-local-review@1.0.0',split='DEV',formal_split=None,prior_exposure='exposed_DEV_not_fresh_FINAL',rule_id=rule_of(call),call_span=call['span'],call_sha256=flow.sha(f['code'].encode()[call['span']['start']-f['start']:call['span']['end']-f['start']]),source=source,source_excerpt='source/'+f['id']+'.go.txt',prompt='지정한 rule-shaped 호출의 source–sink 조합을 원문·선언 좌표와 함께 전수 분류한다. positive/negative/unknown을 구분하고 빈 탐지를 안전으로 해석하지 않는다. target/import/호출자/메모리/클로저·명시/암묵 반환의 범위를 밝힌다.'))
    return public


def summarize(drafts):
    strata={}
    for stratum in ('controlled','project'):
        selected=[d for d in drafts if d['stratum']==stratum];rows=[r for d in selected for r in d['universe']]
        strata[stratum]=dict(inputs=len(selected),functions=len({d['source']['function_id'] for d in selected}),units=len(rows),review_labels=dict(Counter(r['review_label'] for r in rows)),contract_labels=dict(Counter(r['contract_label'] for r in rows)),per_rule={rule:dict(inputs=sum(d['rule_id']==rule for d in selected),units=sum(r['rule_id']==rule for r in rows)) for rule in (RULE1,RULE2)})
    return dict(kind='complete_bounded_AI_DEV_drafts_not_independent_gold',strata=strata,AI_DEV_inputs=108,source_functions=91,project_files=34,independent_approved=0,FINAL_cases=0,D04='미정',formal_completed=0,formal_total=9,freeze=None,execution_allowed=False,current_scores=None,semantic_gold_complete=False,new_MCP_calls=0,candidate_engine_calls=0,additional_external_model_calls=0)


def render_review(public,gold):
    lines=[public['case_id']+' — AI DEV bounded draft, not independent gold',public['source']['path']+' '+str(public['call_span']),public['rule_id'],'','Source/excerpt identity: '+json.dumps(public['source'],ensure_ascii=False,sort_keys=True),'','Complete declared source–sink units:']
    lines.extend(json.dumps(r,ensure_ascii=False,sort_keys=True) for r in gold['universe'])
    lines+=['','Answer sets: '+json.dumps(gold['answer_sets'],ensure_ascii=False,sort_keys=True),'Empty-set rationales: '+json.dumps(gold['empty_set_rationales'],ensure_ascii=False,sort_keys=True),'Human reviewer: unassigned; no scoring, no safety/attacker-control/runtime verdict; actual MCP mapping not frozen.']
    return '\n'.join(lines)+'\n'


def historical_errors(root):
    errors=[];p=root/PREVIOUS;m=flow.read(p/'manifest.json')
    original=subprocess.check_output(['git','show',REVISION+':'+str(PREVIOUS/'manifest.json')],cwd=root)
    if original!=(p/'manifest.json').read_bytes():errors.append('previous frozen manifest differs from Git')
    for name,h in m['artifacts'].items():
        if flow.sha((p/name).read_bytes())!=h:errors.append('previous frozen WI1 artifact differs '+name)
    for row in m['tools']:
        raw=subprocess.check_output(['git','show',REVISION+':'+row['path']],cwd=root)
        if flow.sha(raw)!=row['sha256']:errors.append('previous source verifier binding differs '+row['path'])
    import wi1_consumer_matching
    errors.extend(wi1_consumer_matching.historical_errors(root))
    return errors


def reference(root,packet):
    return dict(manifest=str(PACKET/'manifest.json'),sha256=flow.sha((packet/'manifest.json').read_bytes()),status='WI2_DEV108_bounded_complete_AI_drafts_not_independent_gold',report=str(PACKET/'REPORT.txt'),worklist=str(PACKET/'WORKLIST.json'))


def current_indices(root=ROOT,protocol_override=None,ledger_override=None):
    paths=[Path('docs/spec-driven/OPERATIONAL-ACCEPTANCE-'+n+'.json') for n in ('PROTOCOL','PROGRESS','DECISION-PROPOSAL')]+[BASE/'GAPS.json'];docs=[flow.read(root/p) for p in paths]
    if protocol_override is not None:docs[0]=protocol_override
    if ledger_override is not None:docs[1]=ledger_override
    ref=reference(root,root/PACKET);state=flow.read(root/PACKET/'WORKLIST.json');current=dict(kind=KIND,manifest=ref['manifest'],manifest_sha256=ref['sha256']);errors=[]
    old_protocol=json.loads(subprocess.check_output(['git','show',REVISION+':'+str(paths[0])],cwd=root))
    for d in docs:
        if d.get('wi2_dev_gold_preparation')!=ref or d.get('current_execution_state')!=state or d.get('current_readiness_preparation')!=current:errors.append('WI2 current reference/state differs')
        for key in ('wi1_consumer_matching_preparation','wi1_dev_gold_preparation','independent_freeze_preparation'):
            expected=dict(old_protocol[key])
            if key=='wi1_consumer_matching_preparation':expected['historical_source_revision']=REVISION
            if d.get(key)!=expected:errors.append('historical preparation promoted or source binding differs '+key)
    p=docs[0]
    if p['freeze'] is not None or p['execution_allowed'] is not False or any(p['roles'].values()):errors.append('AI draft cannot freeze, execute or invent reviewers')
    proposal=p['adoption_acceptance_decision_proposal']
    if proposal['sha256']!=flow.sha((root/paths[2]).read_bytes()) or any(d.get('adoption_acceptance_decision_proposal')!=proposal for d in (docs[1],docs[3])):errors.append('proposal link differs')
    return errors


def validate(root=ROOT,packet=None):
    p=packet or root/PACKET;errors=[]
    try:
        m=flow.read(p/'manifest.json');actual={x.relative_to(p).as_posix():flow.sha(x.read_bytes()) for x in p.rglob('*') if x.is_file() and x!=p/'manifest.json'}
        if m['kind']!=KIND or [r['path'] for r in m['tools']]!=TOOLS or [r['path'] for r in m['sources']]!=SOURCES or m['previous_source_revision']!=REVISION:errors.append('manifest scope differs')
        if actual!=m['artifacts']:errors.append('artifact inventory/hash differs')
        for row in m['sources']+m['tools']:
            target=(root/row['path']).resolve()
            if not target.is_relative_to(root.resolve()) or flow.sha(target.read_bytes())!=row['sha256']:errors.append('source/tool differs '+row['path'])
        errors.extend(historical_errors(root))
        from independent_acceptance import native_reuse_record
        if native_reuse_record(root)['changed_bound_files']:errors.append('native source binding changed')
        inputs,functions=source_inputs(root);raw=json.dumps(inputs,ensure_ascii=False).encode()
        if flow.packed(p/'private/source-inputs.json.gz')!=inputs:errors.append('source inputs differ')
        data={}
        for label,helper in [('syntax','scripts/wi2_source_syntax.go.txt'),('flow','scripts/wi1_dev_source.go.txt')]:
            receipt=flow.read(p/(label+'-receipt.json'));body=gzip.decompress((p/(label+'-stdout.json.gz')).read_bytes());stderr=(p/(label+'-stderr.txt')).read_bytes()
            if receipt['exit_code']!=0 or receipt['stdin_sha256']!=flow.sha(raw) or receipt['stdout_sha256']!=flow.sha(body) or receipt['stderr_sha256']!=flow.sha(stderr) or receipt['helper_sha256']!=flow.sha((root/helper).read_bytes()) or receipt['candidate_engine_imported_or_invoked'] is not False:errors.append('source observer receipt differs '+label)
            values=json.loads(body);data[label]={(o['path'],o['span']['start']):o for o in values}
            if len(values)!=91 or len(data[label])!=91 or set(data[label])!={(f['path'],f['start']) for f in functions}:errors.append('91-function source observer census differs')
        expected=public_inputs(functions,list(data['syntax'].values()));actual_public=flow.read(p/'public/controlled-DEV.json')+flow.read(p/'public/project-DEV.json')
        if sorted(expected,key=lambda x:x['case_id'])!=sorted(actual_public,key=lambda x:x['case_id']):errors.append('DEV input identity/count differs or public input leaks gold')
        for fn in functions:
            if (p/'public/source'/(fn['id']+'.go.txt')).read_bytes()!=fn['code'].encode():errors.append('source excerpt differs')
        drafts=[]
        for row in expected:
            key=row['source']['path'],row['source']['start'];o=data['flow'][key]
            if o['function_sha256']!=row['source']['function_sha256']:errors.append('flow source identity differs')
            gold=draft_case(row,o,data['syntax'][key]);gold['stratum']=row['stratum'];drafts.append(gold)
            if flow.packed(p/'private'/(row['case_id']+'.json.gz'))!=gold:errors.append('incomplete or promoted AI gold '+row['case_id'])
            if (p/'private'/(row['case_id']+'-REVIEW.txt')).read_text()!=render_review(row,gold):errors.append('readable AI review differs '+row['case_id'])
        if flow.read(p/'SUMMARY.json')!=summarize(drafts):errors.append('summary population/labels/approval differs')
        context=flow.read(p/'public/controlled-context.json')
        if context['prefix']!=PREFIX or context['prefix_bytes']!=len(PREFIX.encode()) or context['status']!='AI_proposed_DEV_context_not_independently_approved':errors.append('controlled context assumption promoted or changed')
        callers=flow.read(p/'private/caller-context.json')
        if callers['references']!=flow.read(root/CALLERS) or callers['not_added_to_108_DEV_inputs'] is not True or callers['independent_review_receipt'] is not None:errors.append('caller evidence promoted or population differs')
        contract=flow.read(p/'matching-contract.json')
        if contract['formal_scoring_allowed'] is not False or contract['independent_review'] is not None or contract['actual_consumer_matching'] is not None or contract['retained_MCP_receipt'] is not None or contract['precision_recall'] is not None:errors.append('matching/approval fabricated')
        work=flow.read(p/'WORKLIST.json')
        if work['formal_completed']!=0 or work['formal_total']!=9 or work['reviewer_assignment']!='미정' or work['independent_quality_or_ontology_acceptance'] is not False or work['production_activation'] is not False or work['publication'] is not False:errors.append('worklist promotes drafts to formal acceptance')
    except (KeyError,ValueError,TypeError,OSError,IndexError,StopIteration,AssertionError) as exc:errors.append(str(exc))
    return errors
