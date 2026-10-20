"""Source-only context overlay and observed retained symptom matching, AI DEV."""
from collections import Counter
import base64
import copy
import gzip
import json
from pathlib import Path
import re
import subprocess
import wi3_dev_gold as old

ROOT,BASE,f=old.ROOT,old.BASE,old.f
PACKET=BASE/'followups/wi3-consumer-matching-v1'
REVISION='8553fe9707fd2dc289126872eb1138fe2f021098'
KIND='WI3_observed_consumer_AI_DEV_not_formal_execution'
CONTEXT=Path('scripts/fixtures/wi3-context-refinement.json')
TOOLS=['scripts/'+n for n in ('wi3_consumer_matching.py','prepare-wi3-consumer-matching.py',
    'check-wi3-consumer-matching.py','test_wi3_consumer_matching.py',
    'verify-wi3-consumer-runtime.py','capture-wi3-retained-citations.py',
    'wi3_decl_spans.go.txt',
    'fixtures/wi3-context-refinement.json','wi3_dev_gold.py','fixtures/wi3-dev-authoring.json',
    'source_packet.py','independent_acceptance.py','wi2_consumer_matching.py','wi2_dev_gold.py',
    'wi1_dev_gold.py','wi1_consumer_matching.py','eval-value-dependencies.py',
    'check-operational-acceptance.py','test_operational_acceptance.py')]


def context_overlay(root=ROOT):
    """No runtime/packet input: consumer outputs can never author these labels."""
    data=old.derive(root);context=f.read(root/CONTEXT);_,reviews,proof=old.original(root)
    raw=old.safe(root/old.INPUTS,proof['files'][context['path']]['artifact']).read_bytes()
    s=context['function_span']
    if (f.sha(raw)!=context['file_sha256'] or context['source_commit']!=f.SOURCE_COMMIT
            or f.sha(raw[s['start_byte']:s['end_byte_exclusive']])!=context['function_sha256']):
        raise ValueError('static context source differs')
    for w in context['witnesses']+[context['caller_witness']]:
        text=raw[w['start_byte']:w['end_byte_exclusive']]
        if f.sha(text)!=w['span_sha256'] or text.decode()!=w['text']:raise ValueError('context witness differs')
    for change in context['changes']:
        gold=next(d for d in data['gold'] if d['case_id']==change['case_id'])
        unit=next(u for u in gold['universe'] if u['location_id']==change['location_id'])
        if unit['label']!=change['previous_label']:raise ValueError('original unknown missing')
        unit.update(previous_label=unit['label'],label=change['review_label'],relevance_kind=change['relevance_kind'],
            rationale=context['explanation'],context_source_sha256=context['function_sha256'])
    taxonomy=[]
    for gold in data['gold']:
        for u in gold['universe']:
            if u['label']=='unknown':taxonomy.append(dict(case_id=gold['case_id'],location_id=u['location_id'],
                location_key=u['location_key'],rationale=u['rationale'],meaning='bounded source relevance unresolved; not negative, safety, runtime cause or failed retrieval'))
        gold['answer_sets']={key:[u['location_key'] for u in gold['universe'] if u['label']==label] for key,label in
            [('relevant_locations','relevant'),('irrelevant_locations','irrelevant'),('unknown_locations','unknown')]}
        gold['empty_set_rationales']={k:'No units with this label in the selected20-function AI context overlay; outside universe remains unknown.' for k,v in gold['answer_sets'].items() if not v}
    return dict(gold=data['gold'],locations=data['locations'],context=context,unknowns=taxonomy,
        labels=dict(Counter(u['label'] for d in data['gold'] for u in d['universe'])),independent_review=None)


def line_span(raw,first,last):
    if type(first)is not int or type(last)is not int or first<1 or last<first:raise ValueError('invalid line coordinates')
    starts=[0]+[m.end() for m in re.finditer(b'\n',raw)]
    count=len(starts)-(starts[-1]==len(raw))
    if last>count:raise ValueError('line span outside retained bytes')
    a=starts[first-1];b=starts[last] if last<len(starts) else len(raw)
    return a,b,raw[a:b]


def read_bindings(packet):
    def read(name):return json.loads(gzip.decompress((packet/'runtime/bindings'/name).read_bytes()))
    source=read('sources/manifest.json.gz');dataset=read('dataset-identity.json.gz');graph=read('graph/manifest.json.gz')
    if source['identity']!=dataset['source']:raise ValueError('dataset/captured-source identity mismatch')
    native=json.dumps(graph,ensure_ascii=False,separators=(',',':')).replace('<','\\u003c').replace('>','\\u003e').replace('&','\\u0026').encode()
    identity=dataset['source'];pins=dict(project_id=identity['project_id'],dataset_id=dataset['dataset_id'],commit=identity['source_commit'],source_mode=identity['source_mode'],graph_snapshot_sha256=f.sha(native))
    return source,dataset,graph,pins


def retained_proof(ref,legacy,files,manifest,pins,dataset):
    expected=dict(project_id=pins['project_id'],dataset_id=pins['dataset_id'],snapshot_id=dataset['source']['snapshot_id'],
        source_mode=pins['source_mode'],base_commit=pins['commit'],commit_hash=pins['commit'],origin_id='repo')
    if not isinstance(ref,dict) or any(ref.get(k)!=v for k,v in expected.items()):raise ValueError('retained project/dataset/source scope differs')
    if legacy!= {k:ref[k] for k in ('file','start_line','end_line','commit_hash')}:raise ValueError('legacy and retained citation disagree')
    key=ref['origin_id'],ref['file'];raw=files[key];record=manifest[key]
    if ref['file_sha256']!=record['sha256'] or f.sha(raw)!=record['sha256']:raise ValueError('retained file hash differs')
    a,b,span=line_span(raw,ref['start_line'],ref['end_line'])
    if f.sha(span)!=ref['content_sha256']:raise ValueError('retained content hash differs')
    return dict(path=ref['file'],file_sha256=ref['file_sha256'],start_byte=a,end_byte_exclusive=b,span_sha256=f.sha(span)),span


def adapt_candidate(candidate,locations,files,manifest,pins,dataset,declarations=None):
    proof,span=retained_proof(candidate.get('retained_citation'),candidate['citation'],files,manifest,pins,dataset)
    candidates=[l for l in locations if l['identity']['path']==proof['path'] and l['identity']['file_sha256']==proof['file_sha256']]
    normalized=None;normalization=None
    for l in candidates:
        source=l['identity'];a,b=proof['start_byte'],proof['end_byte_exclusive']
        # Go line citations include one terminal LF beyond FuncDecl.End.
        if (a,b)==(source['start_byte'],source['end_byte_exclusive']) or (span.endswith(b'\n') and (a,b-1)==(source['start_byte'],source['end_byte_exclusive'])):
            body=span[:-1] if b-1==source['end_byte_exclusive'] else span
            if f.sha(body)==source['function_sha256']:
                normalized=dict(pins,**{k:source[k] for k in ('path','file_sha256','function_sha256','start_byte','end_byte_exclusive')},unit_kind='function');normalization='exact_function_optional_one_terminal_LF';break
        trimmed=span.strip(b' \t\r\n');leading=len(span)-len(span.lstrip(b' \t\r\n'))
        for witness in l['equivalence_spans']:
            if (a+leading,a+leading+len(trimmed),f.sha(trimmed))==(witness['start_byte'],witness['end_byte_exclusive'],witness['span_sha256']):
                normalized=dict(pins,path=source['path'],file_sha256=source['file_sha256'],function_sha256=source['function_sha256'],
                    start_byte=witness['start_byte'],end_byte_exclusive=witness['end_byte_exclusive'],span_sha256=witness['span_sha256'],unit_kind='span');normalization='exact_declared_witness_with_only_edge_whitespace';break
        if normalized:break
    # Supplemental observed adapter: only a complete AST-associated Doc+FuncDecl
    # envelope qualifies. A substring, neighboring function or arbitrary prefix
    # never acquires the selected function's identity.
    if normalized is None:
        declarations=declarations or {}
        for decl in declarations.get(proof['path'],[]):
            a,b=proof['start_byte'],proof['end_byte_exclusive']
            if not decl['has_doc'] or a!=decl['doc_start'] or not (b==decl['end'] or span.endswith(b'\n') and b-1==decl['end']):continue
            owners=[l for l in candidates if (l['identity']['start_byte'],l['identity']['end_byte_exclusive'],l['identity']['function_sha256'])==(decl['start'],decl['end'],decl['sha256'])]
            if len(owners)==1:
                s=owners[0]['identity'];normalized=dict(pins,**{k:s[k] for k in ('path','file_sha256','function_sha256','start_byte','end_byte_exclusive')},unit_kind='function');normalization='exact_AST_associated_Doc_and_complete_function';break
    overlaps=[l for l in candidates if proof['start_byte']<l['identity']['end_byte_exclusive'] and proof['end_byte_exclusive']>l['identity']['start_byte']]
    matching=old.match_hit(normalized,locations,pins) if normalized else dict(status='outside_selected_universe_unknown' if not overlaps else 'non_equivalent_source_span_unknown',location_key=None)
    return dict(source_proof=proof,normalization=normalization,matching=matching,
                raw_symbol=candidate.get('symbol'),symbol_not_used_as_identity=True,
                signals=copy.deepcopy(candidate['signals']))


def unpack_response(row):
    raw=row['raw_response']
    if 'error'in raw:return None,'protocol_error'
    result=raw.get('result',{})
    if result.get('isError'):return None,'tool_error'
    value=result.get('structuredContent')
    if value is None:
        content=result.get('content',[])
        if content and content[0].get('text'):value=json.loads(content[0]['text'])
    return value,'partial' if value and value['partial'] else 'response'


def derive(root,packet):
    overlay=context_overlay(root);source,dataset,graph,pins=read_bindings(packet)
    expected=f.read(root/old.PACKET/'proposed-reader-pins.json')['pins']
    if pins!=expected:raise ValueError('actual reader belongs to another frozen pilot scope')
    source_records={(r['origin_id'],r['path']):r for r in source['files']}
    captured=json.loads(gzip.decompress((packet/'runtime/captured-source-files.json.gz').read_bytes()))
    files={(r['origin_id'],r['path']):base64.b64decode(r['base64'],validate=True) for r in captured}
    if len(files)!=len(captured):raise ValueError('duplicate captured source identity')
    for r in captured:
        if f.sha(files[r['origin_id'],r['path']])!=r['sha256'] or r['sha256']!=source_records[r['origin_id'],r['path']]['sha256']:raise ValueError('captured file differs')
    audit=json.loads(gzip.decompress((packet/'source-audit/stdout.json.gz').read_bytes()))
    declarations={row['path']:row['functions'] for row in audit}
    if len(declarations)!=len(files):raise ValueError('declaration audit population differs')
    for row in audit:
        raw=files['repo',row['path']]
        if f.sha(raw)!=row['sha256']:raise ValueError('declaration file differs')
        for decl in row['functions']:
            if f.sha(raw[decl['start']:decl['end']])!=decl['sha256']:raise ValueError('function declaration hash differs')
    original_requests=f.read(root/old.PACKET/'public/wire-requests.json')['requests']
    rows=f.read(packet/'runtime/primary-results.json')
    if [r['case_id'] for r in rows]!=[r['case_id'] for r in original_requests]:raise ValueError('planned20 input denominator filtered or duplicated')
    gold={d['case_id']:d for d in overlay['gold']};cases=[];signals=Counter();anchor_count=0
    for row,request in zip(rows,original_requests):
        if row['arguments']!=request['arguments']:raise ValueError('source hint, cap or symptom changed')
        value,state=unpack_response(row);mapped=[]
        if value is not None:
            expected_coords=dict(project_id=pins['project_id'],dataset_id=pins['dataset_id'],snapshot_id=dataset['source']['snapshot_id'],source_mode=pins['source_mode'],base_commit=pins['commit'])
            if value['coordinates']!=expected_coords or value['snapshot']!=pins['commit'] or value['symptom']!=request['arguments']['symptom']:raise ValueError('response source/symptom differs')
            if len(value['candidates'])>10:raise ValueError('candidate cap exceeded')
            for key in ('max_candidates','max_seeds','max_enriched','max_calls','depth'):
                if value['budget'][key]!=request['arguments'][key]:raise ValueError('runtime cap differs')
            if value['budget']['time_budget_ms']!=1000:raise ValueError('primary deadline changed')
            seen=set()
            for rank,candidate in enumerate(value['candidates'][:10],1):
                adapted=adapt_candidate(candidate,overlay['locations'],files,source_records,pins,dataset,declarations);anchor_count+=1
                for name,signal in candidate['signals'].items():
                    signals[name+':'+signal['status']]+=1
                    for provenance in signal['provenance']:
                        if provenance['snapshot']!=pins['commit']:raise ValueError('signal snapshot differs')
                        retained_proof(provenance.get('retained_anchor'),provenance['anchor'],files,source_records,pins,dataset);anchor_count+=1
                key=adapted['matching']['location_key'];unit=next((u for u in gold[row['case_id']]['universe'] if u['location_key']==key),None)
                adapted.update(raw_rank=rank,duplicate=bool(key and key in seen),AI_source_label=unit['label'] if unit else None)
                if key:seen.add(key)
                mapped.append(adapted)
        cases.append(dict(case_id=row['case_id'],status=state,raw_candidates=mapped,primary_partial=value['partial'] if value else None,
            stop_reasons=value['budget']['stop_reasons'] if value else None,raw_count=len(mapped),quality_score=None,
            source_gold_is_AI_only=True,installed_agent_role_receipt=None))
    summary=dict(kind=KIND,planned=20,attempted=20,statuses=dict(Counter(c['status'] for c in cases)),
        raw_candidates=sum(c['raw_count'] for c in cases),candidate_matching=dict(Counter(r['matching']['status'] for c in cases for r in c['raw_candidates'])),
        strict_prior_adapter_mapped=sum(r['matching']['status']=='mapped' and r['normalization']!='exact_AST_associated_Doc_and_complete_function' for c in cases for r in c['raw_candidates']),
        supplemental_AST_doc_envelopes=sum(r['normalization']=='exact_AST_associated_Doc_and_complete_function' for c in cases for r in c['raw_candidates']),
        mapped_by_AI_label=dict(Counter(r['AI_source_label'] for c in cases for r in c['raw_candidates'] if r['AI_source_label'])),
        duplicate_function_slots=sum(r['duplicate'] for c in cases for r in c['raw_candidates']),
        retained_anchor_instances_verified=anchor_count,captured_source_files=len(files),signal_statuses=dict(signals),
        overlay_source_units=400,overlay_labels=overlay['labels'],source_unknowns=len(overlay['unknowns']),
        source_refinements=2,independent_approved=0,FINAL_cases=0,D04='미정',formal_completed=0,formal_total=9,
        freeze=None,execution_allowed=False,current_scores=None,paid_or_external_model_calls=0,installed_agent_role_runs=0)
    return dict(overlay=overlay,cases=cases,summary=summary)


def historical_errors(root=ROOT):
    errors=old.historical_errors(root);p=root/old.PACKET;m=f.read(p/'manifest.json')
    if subprocess.check_output(['git','show',REVISION+':'+str(old.PACKET/'manifest.json')],cwd=root)!=(p/'manifest.json').read_bytes():errors.append('frozen stage48 manifest differs')
    for name,digest in m['artifacts'].items():
        if f.sha((p/name).read_bytes())!=digest:errors.append('frozen stage48 artifact differs '+name)
    for row in m['tools']:
        if f.sha(subprocess.check_output(['git','show',REVISION+':'+row['path']],cwd=root))!=row['sha256']:errors.append('historical verifier differs '+row['path'])
    return errors


def review_ledger(values):
    return dict(source_unknowns=values['overlay']['unknowns'],source_support_refinements=values['overlay']['context'],
        partial_cases=[dict(case_id=c['case_id'],status=c['status'],stop_reasons=c['stop_reasons']) for c in values['cases']],
        unlabelled_or_unmapped_slots=[dict(case_id=c['case_id'],rank=r['raw_rank'],source=r['source_proof'],status=r['matching']['status']) for c in values['cases'] for r in c['raw_candidates'] if r['matching']['location_key']is None],
        quality_scores=None,independent_review=None,
        remaining='partial/provider/canonical/span coverage, outside20 semantic labels, independent family/gold/metric freeze and installed-agent investigation are not accepted')


def reference(root,packet):
    return dict(manifest=str(PACKET/'manifest.json'),sha256=f.sha((packet/'manifest.json').read_bytes()),status='WI3_AI_context_and_actual_partial_symptom_matching_not_independent_gold',report=str(PACKET/'REPORT.txt'),worklist=str(PACKET/'WORKLIST.json'))


def matching_contract():
    return dict(kind='observed_retained_adapter_AI_proposal_not_frozen',
        coordinates='verify legacy/v2 equality, project/dataset/source snapshot/commit/mode/origin, file and exact inclusive-line content SHA; compute UTF8 offsets from retained bytes',
        normalization='prior strict adapter unchanged; supplemental proposal also admits exact AST-associated Doc+complete FuncDecl envelope with at most one terminal LF; no arbitrary containment/name-only/canonical-only mapping',
        rank='first10 original final candidates; preserve outside/unknown slots, function dedup without refill; backend rank/score stays backend-specific',
        source_overlay='400 original units preserved;2 source-only static runReorg support refinements; no runtime/root-cause inference; remaining unknowns explicit',
        denominator='all20 primary requests retained with partial/error/budget/stop-reason state; no success-only metric or silent retries',
        independent_review=None,formal_scoring_allowed=False,quality_scores=None)


def current_indices(root=ROOT,protocol_override=None,ledger_override=None):
    paths=[Path('docs/spec-driven/OPERATIONAL-ACCEPTANCE-'+n+'.json') for n in ('PROTOCOL','PROGRESS','DECISION-PROPOSAL')]+[BASE/'GAPS.json'];docs=[f.read(root/p) for p in paths]
    if protocol_override is not None:docs[0]=protocol_override
    if ledger_override is not None:docs[1]=ledger_override
    ref=reference(root,root/PACKET);state=f.read(root/PACKET/'WORKLIST.json');current=dict(kind=KIND,manifest=ref['manifest'],manifest_sha256=ref['sha256']);errors=[]
    old_doc=json.loads(subprocess.check_output(['git','show',REVISION+':'+str(paths[0])],cwd=root))
    for d in docs:
        if d.get('wi3_consumer_matching_preparation')!=ref or d.get('current_execution_state')!=state or d.get('current_readiness_preparation')!=current:errors.append('current WI3 observed reference/state differs')
        for key in ('wi3_dev_gold_preparation','wi2_consumer_matching_preparation','wi2_dev_gold_preparation','wi1_consumer_matching_preparation','wi1_dev_gold_preparation','independent_freeze_preparation'):
            expected=dict(old_doc[key])
            if key=='wi3_dev_gold_preparation':expected['historical_source_revision']=REVISION
            if d.get(key)!=expected:errors.append('historical preparation promoted '+key)
    if docs[0]['freeze']is not None or docs[0]['execution_allowed']is not False or any(docs[0]['roles'].values()):errors.append('AI observed matching cannot assign reviewer/freeze/execute')
    proposal=docs[0]['adoption_acceptance_decision_proposal']
    if proposal['sha256']!=f.sha((root/paths[2]).read_bytes()) or any(d.get('adoption_acceptance_decision_proposal')!=proposal for d in (docs[1],docs[3])):errors.append('proposal link differs')
    return errors


def wire_errors(packet,root=ROOT):
    requests=[json.loads(x) for x in gzip.decompress((packet/'runtime/primary/mcp.stdin.jsonl.gz').read_bytes()).splitlines()]
    responses=[json.loads(x) for x in gzip.decompress((packet/'runtime/primary/mcp.stdout.jsonl.gz').read_bytes()).splitlines()]
    by_id={r['id']:r for r in requests};actual=[]
    for response in responses:
        request=by_id.get(response.get('id'),{})
        if request.get('method')=='tools/call':actual.append((request['params'],response))
    rows=f.read(packet/'runtime/primary-results.json');boundaries=f.read(packet/'runtime/boundaries.json');errors=[]
    for row in rows+boundaries:
        if sum(params==dict(name='cks.context.label_suspects',arguments=row['arguments']) and raw==row['raw_response'] for params,raw in actual)!=1:errors.append('raw transport missing/duplicate '+row['case_id'])
    if len(actual)!=len(rows)+len(boundaries) or len(boundaries)!=3:errors.append('actual tool-call population differs')
    for row in boundaries:
        result=row['raw_response'].get('result',{})
        if result.get('isError')is not True:errors.append('actual refusal boundary accepted')
    return errors


def validate(root=ROOT,packet=None):
    p=packet or root/PACKET;errors=[]
    try:
        m=f.read(p/'manifest.json');actual={x.relative_to(p).as_posix():f.sha(x.read_bytes()) for x in p.rglob('*') if x.is_file() and x!=p/'manifest.json'}
        if m['kind']!=KIND or m['previous_source_revision']!=REVISION or [r['path'] for r in m['tools']]!=TOOLS or actual!=m['artifacts']:errors.append('manifest scope/hash differs')
        for row in m['tools']:
            if f.sha((root/row['path']).read_bytes())!=row['sha256']:errors.append('tool binding differs '+row['path'])
        errors.extend(historical_errors(root));expected=derive(root,p)
        captured=json.loads(gzip.decompress((p/'runtime/captured-source-files.json.gz').read_bytes()))
        stdin=json.dumps([dict(Path=r['path'],Source=base64.b64decode(r['base64']).decode()) for r in captured],ensure_ascii=False).encode()
        receipt=f.read(p/'source-audit/receipt.json')
        if receipt['exit']!=0 or receipt['stdin_sha256']!=f.sha(stdin) or receipt['stdout_sha256']!=f.sha(gzip.decompress((p/'source-audit/stdout.json.gz').read_bytes())) or receipt['stderr_sha256']!=f.sha((p/'source-audit/stderr.txt').read_bytes()) or receipt['helper_sha256']!=f.sha((root/'scripts/wi3_decl_spans.go.txt').read_bytes()):errors.append('source declaration receipt differs')
        for name in ('overlay','cases'):
            if json.loads(gzip.decompress((p/'private'/(name+'.json.gz')).read_bytes()))!=expected[name]:errors.append('source/consumer '+name+' differs')
        if f.read(p/'SUMMARY.json')!=expected['summary']:errors.append('summary denominator/labels/approval differs')
        if f.read(p/'REVIEW-LEDGER.json')!=review_ledger(expected):errors.append('review debt erased or promoted')
        errors.extend(wire_errors(p,root));receipt=f.read(p/'runtime/receipt.json');session=f.read(p/'runtime/session.json')
        if receipt['planned']!=20 or receipt['attempted']!=20 or receipt['unreached']!=0 or receipt['primary_retries']!=0 or receipt['eof_exit']!=0 or session['exit']!=0:errors.append('primary denominator/EOF/retry contract differs')
        if receipt['independent_approved']!=0 or receipt['current_scores']is not None or receipt['formal_completed']!=0 or receipt['paid_or_external_model_calls']!=0 or receipt['installed_agent_role_runs']!=0 or receipt['freeze']is not None:errors.append('runtime promoted to approval or role')
        if receipt['binary_sha256']!='0f96f0d527732ad414416217506dc564a5b0014d8b8f81e335f8c1c88896e54c':errors.append('native reader differs')
        before=f.read(p/'runtime/retained-before.json')
        if before!=f.read(p/'runtime/retained-after.json') or receipt['retained_preserved']is not True:errors.append('retained assets changed')
        for name,key in [('sources/manifest.json.gz','sources/manifest.json'),('graph/manifest.json.gz','graph/manifest.json')]:
            if f.sha(gzip.decompress((p/'runtime/bindings'/name).read_bytes()))!=before[key]:errors.append('retained binding differs')
        for name in ('model-before.json','model-after.json'):
            model=f.read(p/'runtime'/name)
            if model['model']['digest']!='7907646426070047a77226ac3e684fbbe8410524f7b4a74d02837e43f2146bab' or model['runtime']['version']!='0.40.4' or model['show']['model_info']['bert.embedding_length']!=1024:errors.append('local model/runtime differs')
        dataset=json.loads(gzip.decompress((p/'runtime/bindings/dataset-identity.json.gz').read_bytes()))
        if dataset!=f.read(root/old.previous.PACKET/'runtime/dataset-identity.json'):errors.append('model/dataset identity differs from frozen pilot')
        from independent_acceptance import native_reuse_record
        if native_reuse_record(root)['changed_bound_files']:errors.append('native source binding changed')
        state=f.read(p/'WORKLIST.json')
        if state['formal_completed']!=0 or state['formal_total']!=9 or state['reviewer_assignment']!='미정' or state['independent_quality_or_ontology_acceptance']is not False or state['production_activation']is not False or state['publication']is not False:errors.append('worklist promotes observations')
        contract=f.read(p/'matching-contract.json')
        if contract!=matching_contract():errors.append('matching proposal changed or promoted')
    except (KeyError,ValueError,TypeError,OSError,IndexError,StopIteration,AssertionError) as exc:errors.append(str(exc))
    return errors
