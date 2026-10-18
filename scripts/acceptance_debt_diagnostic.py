"""Read-only WI1-3 debt triage; historical generic errors are not outage counts."""
from collections import Counter
import gzip,json,subprocess
from pathlib import Path
import wic_dev_gold as previous
ROOT,BASE,f=previous.ROOT,previous.BASE,previous.f
PACKET=BASE/'followups/wi123-debt-diagnostic-v1'
REVISION='b9e0db369211aba2c55f0db48d49f446c8023b88'
KIND='WI123_readonly_debt_diagnostic_not_formal_execution'
OLD=BASE/'followups/wi3-consumer-matching-v1'
SOURCE_INPUTS=[Path(n) for n in ('internal/system/suspects/compose.go','internal/system/ckgclient/identity.go','internal/system/ckgclient/real.go','internal/system/mcp/source_guard.go','cmd/cks/mcpcli/serve.go')]
TOOLS=['scripts/'+n for n in ('acceptance_debt_diagnostic.py','prepare-acceptance-debt-diagnostic.py','check-acceptance-debt-diagnostic.py','test_acceptance_debt_diagnostic.py','wic_dev_gold.py','prepare-wic-dev-gold.py','source_packet.py','independent_acceptance.py','check-operational-acceptance.py','test_operational_acceptance.py')]
BINARY_SHA='0f96f0d527732ad414416217506dc564a5b0014d8b8f81e335f8c1c88896e54c'


def packed(p):return json.loads(gzip.decompress(p.read_bytes()))
def citation_key(c):return tuple(c[k] for k in ('file','start_line','end_line','commit_hash'))


def old_cases(root=ROOT):
    rows=f.read(root/OLD/'runtime/primary-results.json');out=[]
    for row in rows:
        value=row['raw_response']['result']['structuredContent'];c=value['candidates'];seeds=[]
        for r in c:
            signal=r['signals']['distance']
            if signal.get('value')==0 and any(p['tool']=='seed' and p['anchor']==r['citation'] for p in signal['provenance']):seeds.append(r)
        missing=[r['citation'] for r in seeds if r['symbol']is None]
        signals=Counter((kind,r['signals'][kind]['status']) for r in c for kind in ('impact_category','concurrency'))
        reason_counts=Counter(r['signals'][kind].get('reason','') for r in c for kind in ('impact_category','concurrency') if r['signals'][kind]['status']=='error')
        out.append(dict(case_id=row['case_id'],partial=value['partial'],planned_candidate_slots=len(c),seed_count=len(seeds),unresolved_seeds=missing,
            fully_resolved_seed_set=not missing,seed_symbols=[r['symbol'] for r in seeds],
            shared_error_slots=sum(r['signals'][kind]['status']=='error' for r in c for kind in ('impact_category','concurrency')),
            signal_states={k+':'+v:n for (k,v),n in sorted(signals.items())},generic_error_reasons=dict(reason_counts),
            sufficient_source_failure_path='missing_seed_marks_every_enriched_candidate_and_later_success_never_erases_error' if missing else None,
            actual_past_backend_error_trace=None,exclusive_historical_cause_proven=False,
            stop_reasons=value['budget']['stop_reasons']))
    return out


def symbols(root=ROOT):
    return list(dict.fromkeys(c['symbol'] for r in f.read(root/OLD/'runtime/primary-results.json') for c in r['raw_response']['result']['structuredContent']['candidates'] if c['symbol']))


def diagnostic_rows(rows,planned):
    expected=[(symbol,method) for symbol in planned for method in ('impact_analysis','concurrency_impact')]
    if [(r['arguments']['symbol'],r['tool']) for r in rows]!=expected:raise ValueError('planned direct symbol/method denominator differs')
    out=[]
    for row in rows:
        if row['arguments']!={'symbol':row['arguments']['symbol'],'depth':2,'max_total':10}:raise ValueError('direct diagnostic caps differ')
        raw=row['response'];res=raw.get('result',{});payload=res.get('structuredContent')
        if 'error'in raw:state='protocol_error'
        elif res.get('isError'):state='tool_error'
        elif not isinstance(payload,dict) or payload.get('seed')!=row['arguments']['symbol'] or not isinstance(payload.get('result'),dict):state='invalid_response'
        else:
            result=payload['result'];state='structured_response'
            if row['tool']=='impact_analysis':
                count=0
                groups=result.get('groups')
                if groups is None:groups=[]  # legacy Go nil slice; raw wire retained, no whole-graph safety claim
                if not isinstance(groups,list):raise ValueError('invalid impact groups type')
                for group in groups:
                    for c in group['hits']:
                        if c['commit_hash']!=f.SOURCE_COMMIT or not isinstance(c['start_line'],int) or c['start_line']<1 or c['end_line']<c['start_line']:raise ValueError('direct impact source scope differs')
                        count+=1
                if count>10:raise ValueError('direct impact cap exceeded')
            else:
                if result.get('depth')!=2:raise ValueError('direct concurrency depth differs')
                if len(result.get('modules') or [])>10:raise ValueError('direct concurrency cap exceeded')
        out.append(dict(tool=row['tool'],symbol=row['arguments']['symbol'],state=state,
            runtime_scope='current_readonly_graph_provider_not_historical_call_trace',quality_scoring_allowed=False))
    return out


def debt(root=ROOT):
    records=[]
    for id,name in [('WI-1','wi1-consumer-matching-v1'),('WI-2','wi2-consumer-matching-v1'),('WI-3','wi3-consumer-matching-v1')]:
        path=BASE/'followups'/name/'REVIEW-LEDGER.json';ledger=f.read(root/path);summary=f.read(root/path.parent/'SUMMARY.json')
        if id=='WI-1':counts=dict(Counter(x['kind'] for x in ledger['items']));action='Investigate SRC-027 two select receive/loop lifetime lexical mismatches,53 implicit occurrence phases and3 unreachable empty block ambiguities; value/type/memory semantics unknown, no independent approval.'
        elif id=='WI-2':counts={k:len(v) for k,v in ledger.items() if isinstance(v,list)};action='Resolve local-object/different-import stdlib sink false matches and alias-flow misses; investigate9 unavailable symbols/2 analyses while preserving13 source unknowns; no vulnerability/safety inference.'
        else:counts={k:len(v) for k,v in ledger.items() if isinstance(v,list)};action='Distinguish seed_unresolved from provider faults; investigate unanchored traversal and source span identity without fuzzy joining;66 outside-selected slots remain unlabelled unknown.'
        records.append(dict(work_item_id=id,source_ledger=str(path),source_sha256=f.sha((root/path).read_bytes()),counts=counts,next_action=action,
            independently_resolved=False,installed_role_receipt=None,semantic_gold_review=None,quality_score=None))
    return records


def derive(root=ROOT,packet=None):
    p=packet or root/PACKET;cases=old_cases(root);planned=symbols(root)
    rows=diagnostic_rows(packed(p/'runtime/all-symbols/responses.json.gz'),planned)
    before=packed(p/'runtime/all-symbols/before.json.gz');after=packed(p/'runtime/all-symbols/after.json.gz')
    if before!=after:raise ValueError('read-only graph/source/vector inventory changed')
    historical=f.read(root/OLD/'runtime/retained-after.json')
    if before!=historical:raise ValueError('current direct provider dataset differs from original20 case dataset')
    if (p/'runtime/config.txt').read_bytes()!=(root/OLD/'runtime/config.txt').read_bytes():raise ValueError('original runtime config differs')
    session=f.read(p/'runtime/all-symbols/session.json');receipt=f.read(p/'runtime/all-symbols/receipt.json');identity=f.read(p/'runtime/identity.json')
    if session!={'exit':0,'requests':82,'new_LLM_calls':0,'local_embedding_queries':0}:raise ValueError('direct provider session/cleanup differs')
    if receipt!={'planned_symbols':40,'planned_tool_calls':80,'attempted':80,'preserved':True,'formal_completed':0,'quality_scores':None}:raise ValueError('direct provider execution receipt differs')
    if identity['binary_sha256']!=BINARY_SHA or identity['source_revision']!='38dd5c5449b1e57759e2532090dce101b3ad9124' or identity['new_paid_or_external_model_calls']!=0:raise ValueError('diagnostic binary/model identity differs')
    controls=packed(p/'runtime/direct/responses.json.gz');negative=[]
    for row in controls:
        r=row['response'].get('result',{})
        if row['arguments']['symbol']=='UNINDEXED_WI3_NEGATIVE_CONTROL':
            if not r.get('isError') or 'seed symbol unresolved' not in json.dumps(r):raise ValueError('unindexed symbol not explicitly refused')
            negative.append(row['tool'])
    if negative!=['impact_analysis','concurrency_impact']:raise ValueError('negative controls missing')
    for folder in ('direct','all-symbols'):
        stderr=(p/'runtime'/folder/'mcp.stderr.txt').read_text()
        if 'ckv embedder unavailable' not in stderr or 'operation not permitted' not in stderr or 'degraded mode' not in stderr:raise ValueError('recorded CKV degraded startup scope differs')
    ds=f.read(p/'runtime/direct/session.json')
    if ds!={'exit':0,'requests':7,'binary_sha256':BINARY_SHA,'LLM_calls':0}:raise ValueError('initial direct probe session differs')
    return dict(cases=cases,rows=rows,debt=debt(root),summary=dict(kind=KIND,original_cases=20,original_candidates=68,original_partial=20,
        original_seed_slots=sum(r['seed_count'] for r in cases),unresolved_seed_slots=sum(len(r['unresolved_seeds']) for r in cases),
        cases_with_unresolved_seed=sum(bool(r['unresolved_seeds']) for r in cases),shared_generic_error_labels=sum(r['shared_error_slots'] for r in cases),
        fully_resolved_seed_cases=[r['case_id'] for r in cases if r['fully_resolved_seed_set']],
        current_direct_symbols=len(planned),current_direct_tool_calls=len(rows),current_direct_states=dict(Counter(r['state'] for r in rows)),
        exploratory_calls=6,unindexed_controls=2,retained_inventory_files=len(before),retained_preserved=True,
        historical_backend_outage_count=None,exclusive_historical_cause_proven=False,new_embedding_queries=0,new_paid_or_external_model_calls=0,
        CKV_state='degraded_startup_identity_probe_sandbox_denied',startup_identity_probe_failures=2,full_retrieval_readiness_verified=False,
        independent_approved=0,FINAL=0,formal_completed=0,formal_total=9,freeze=None,execution_allowed=False,quality_scores=None))


def contract():
    return dict(scope='Current actual direct40symbol x2graph calls plus original20 symptom result/source triage; not a new primary symptom evaluation or formal acceptance',
        sufficient_path='Compose missing seed identity unconditionally marks all enriched candidates error; later successes keep error. Thus102 output error labels are correlated, not102 independent failed backend calls.',
        uncertainty='No original per-call typed error trace was captured. Current80 success receipts do not exclude historical transient failures or prove exclusive cause.',
        model_scope='Both CKS startup model-list probes were sandbox-denied; CKV degraded mode. Actual graph tools remain read-only real graph, not full retrieval/model readiness. No semantic queries or inference attempts.',
        remaining='Do not replace original partial/error records with direct successes. All20 primary rows and66 outside/unmapped slots remain; no name-only identity or zero-fill for unknown/unsupported.',
        planned_fix='Preserve error state and provenance, expose typed seed_unresolved reason separately from unknown backend_error and budget_exhausted; add shared-seed counterexamples and fresh native+MCP validation.',
        WI1='Lexical/value/type/memory meaning and select-scope mismatch remain explicit; no numeric block-id guessing or unknown->negative.',
        WI2='Caller/import/receiver target resolution and alias dependency candidates require actual source/consumer fix, not matcher relaxation or rewriting gold.',
        privacy=previous.previous.reviewer_identity_policy(),independent_review=None,formal_scoring_allowed=False)


def historical_errors(root=ROOT):
    errors=previous.historical_errors(root)
    for relative in (previous.PACKET,):
        p=root/relative;m=f.read(p/'manifest.json')
        if (p/'manifest.json').read_bytes()!=subprocess.check_output(['git','show',REVISION+':'+str(relative/'manifest.json')],cwd=root):errors.append('historical manifest differs '+str(relative))
        for n,h in m['artifacts'].items():
            if f.sha(previous.safe(p,n).read_bytes())!=h:errors.append('historical artifact differs '+str(relative/n))
        for t in m.get('tools',[]):
            if f.sha(subprocess.check_output(['git','show',REVISION+':'+t['path']],cwd=root))!=t['sha256']:errors.append('historical tool differs '+t['path'])
    return errors


def reference(root=ROOT):
    return dict(manifest=str(PACKET/'manifest.json'),sha256=f.sha((root/PACKET/'manifest.json').read_bytes()),status='WI123_readonly_diagnostic_not_semantic_resolution_or_acceptance',report=str(PACKET/'REPORT.txt'),worklist=str(PACKET/'WORKLIST.json'))


def current_indices(root=ROOT,protocol_override=None,ledger_override=None):
    paths=[Path('docs/spec-driven/OPERATIONAL-ACCEPTANCE-'+n+'.json') for n in ('PROTOCOL','PROGRESS','DECISION-PROPOSAL')]+[BASE/'GAPS.json'];docs=[f.read(root/p) for p in paths]
    if protocol_override is not None:docs[0]=protocol_override
    if ledger_override is not None:docs[1]=ledger_override
    old=json.loads(subprocess.check_output(['git','show',REVISION+':'+str(paths[0])],cwd=root));ref=reference(root);state=f.read(root/PACKET/'WORKLIST.json');current=dict(kind=KIND,manifest=ref['manifest'],manifest_sha256=ref['sha256']);errors=[]
    for d,path in zip(docs,paths):
        old=json.loads(subprocess.check_output(['git','show',REVISION+':'+str(path)],cwd=root))
        if d.get('reviewer_identity_policy')!=previous.previous.reviewer_identity_policy():errors.append('reviewer privacy policy differs')
        if d.get('wi123_debt_diagnostic')!=ref or d.get('current_execution_state')!=state or d.get('current_readiness_preparation')!=current:errors.append('diagnostic current reference differs')
        for key,value in old.items():
            if not key.endswith('_preparation') or key=='current_readiness_preparation' or not isinstance(value,dict) or 'manifest' not in value:continue
            expected=dict(value)
            if key=='wic_dev_gold_preparation':expected['historical_source_revision']=REVISION
            if d.get(key)!=expected:errors.append('previous preparation promoted '+key)
    if docs[0]['freeze']is not None or docs[0]['execution_allowed']is not False or any(docs[0]['roles'].values()):errors.append('diagnostic cannot freeze or assign reviewers')
    link=docs[0]['adoption_acceptance_decision_proposal']
    if link['sha256']!=f.sha((root/paths[2]).read_bytes()) or any(d.get('adoption_acceptance_decision_proposal')!=link for d in (docs[1],docs[3])):errors.append('proposal link differs')
    return errors


def validate(root=ROOT,packet=None):
    p=packet or root/PACKET;errors=[]
    try:
        m=f.read(p/'manifest.json');actual={x.relative_to(p).as_posix():f.sha(x.read_bytes()) for x in p.rglob('*') if x.is_file() and x!=p/'manifest.json'}
        if m['kind']!=KIND or m['previous_source_revision']!=REVISION or [t['path'] for t in m['tools']]!=TOOLS or actual!=m['artifacts']:errors.append('diagnostic manifest scope/hash differs')
        for t in m['tools']+m['source_inputs']:
            if f.sha((root/t['path']).read_bytes())!=t['sha256']:errors.append('source/tool binding differs '+t['path'])
        if [t['path'] for t in m['source_inputs']]!=[str(x) for x in SOURCE_INPUTS]:errors.append('source input population differs')
        errors.extend(historical_errors(root));data=derive(root,p)
        for name,key in [('SUMMARY.json','summary'),('private/original-case-diagnostic.json','cases'),('private/current-provider-diagnostic.json','rows'),('REVIEW-LEDGER.json','debt')]:
            if f.read(p/name)!=data[key]:errors.append('derived diagnostic differs '+name)
        if f.read(p/'matching-contract.json')!=contract():errors.append('diagnostic scope promoted')
        state=f.read(p/'WORKLIST.json')
        if state['formal_completed']!=0 or state['formal_total']!=9 or state['reviewer_assignment']!='미정' or state['independent_quality_or_ontology_acceptance']is not False or state['production_activation']is not False or state['publication']is not False:errors.append('worklist falsely completed')
        from independent_acceptance import native_reuse_record
        if native_reuse_record(root)['changed_bound_files']:errors.append('native source changed')
    except (ValueError,KeyError,TypeError,OSError,IndexError) as exc:errors.append(str(exc))
    return errors
