"""Source-bound WI-C AI review vocabulary, never a semantic quality verdict."""
import gzip,json,subprocess
from pathlib import Path
import wisp_dev_gold as previous
from source_packet import safe,verify
from wi3_consumer_matching import line_span
ROOT,BASE,f=previous.ROOT,previous.BASE,previous.f
PACKET=BASE/'followups/wic-dev-gold-v1'
REVISION='3bdf444fa5c6f591aa9bce93497668d2110dd188'
KIND='WIC_bounded_review_AI_DEV_not_formal_execution'
GAPS=BASE/'review-gap-v1'
CONSUMER_REV='d1ebafb8ddf4eb487b8f1d6285135e174c40834b'
CONSUMER_FILES=['plugin/'+n for n in ('agents/analyzer.md','agents/planner.md','scripts/deep_review.py','contracts/cks-tools.schema.json','scripts/tests/fixtures/deep-review-bundle.json','scripts/tests/fixtures/deep-review-judgments.json')]
TOOLS=['scripts/'+n for n in ('wic_dev_gold.py','prepare-wic-dev-gold.py','check-wic-dev-gold.py','test_wic_dev_gold.py','wisp_dev_gold.py','prepare-wi4-dev-gold.py','source_packet.py','wi3_consumer_matching.py','independent_acceptance.py','check-operational-acceptance.py','test_operational_acceptance.py')]
SOURCE_INPUTS=[]
ATTRIBUTES='private/consumer/objects/*.commit binary\nprivate/consumer/objects/*.tree binary\n'
SECTIONS=('data_state','guards_exceptions','complexity','resources','io','priority_tradeoffs')
ARTIFACTS=('deep-review-evidence.json','deep-review.json','review-report.md','sequence.mmd','modules.mmd')


def sources(root):
    result={}
    for folder in (previous.previous.PACKET/'private/source',BASE/'context-v1'):
        p=root/folder;proof=verify(p,f.SOURCE_COMMIT)
        for name,record in proof['files'].items():
            raw=safe(p,record['artifact']).read_bytes()
            if name in result and result[name]!=raw:raise ValueError('overlapping original source differs')
            result[name]=raw
    return result


def consumer_sources(root,p):
    folder=p/'private/consumer';proof=verify(folder,CONSUMER_REV)
    adopted=f.read(root/BASE/'followups/consumer-adoption-v1/consumer-source.json')
    if proof['commit']!=adopted['consumer_final_commit'] or set(proof['files'])!=set(CONSUMER_FILES):raise ValueError('adopted consumer identity/population differs')
    result={}
    for name,r in proof['files'].items():
        raw=safe(folder,r['artifact']).read_bytes()
        if f.sha(raw)!=adopted['consumer_plugin_sha256'][name]:raise ValueError('adopted consumer bytes differ')
        result[name]=raw.decode()
    return result


def derive(root=ROOT,packet=None):
    p=packet or root/PACKET;raws=sources(root);consumer=consumer_sources(root,p)
    cases=f.read(root/GAPS/'private-review-gaps.json')['cases'];public=[];gold=[]
    for c in cases:
        s=c['source'];raw=raws[s['path']];body=raw[s['start_byte']:s['end_byte_exclusive']]
        if f.sha(raw)!=s['file_sha256'] or f.sha(body)!=s['function_sha256']:raise ValueError('exact review source differs')
        facts=[]
        for i,a in enumerate(c['existing_ai_anchors']+c['context_supplement'].get('anchors',[]),1):
            path=a.get('path',s['path']);start,end,span=line_span(raws[path],a['start_line'],a['end_line'])
            if (start,end,f.sha(span))!=tuple(a[k] for k in ('start_byte','end_byte_exclusive','span_sha256')):raise ValueError('review source/context anchor differs')
            facts.append(dict(id=a.get('id',c['candidate_id']+'-CTX-'+str(i)),kind='required_AI_source_observation',
                claim=a.get('expected_source_fact_draft',a.get('source_fact_draft')),source=dict(project='go-stablenet',commit=f.SOURCE_COMMIT,path=path,
                    file_sha256=f.sha(raws[path]),start_line=a['start_line'],end_line=a['end_line'],start_byte=start,end_byte_exclusive=end,span_sha256=f.sha(span)),
                text=span.decode(),independent_approved=False,semantic_match=None))
        questions=[]
        for i,q in enumerate(c['required_item_questions'],1):
            questions.append(dict(id=c['candidate_id']+'-Q'+str(i),original=q,question=q.replace('getStartEnd','calculateBlockRange'),
                required_AI_followup=True,answer_status='unknown',answer=None,
                rationale='Caller/requirement/runtime evidence not established by the finite source anchors; disclose limitation instead of inventing measurements or a confirmed defect.'))
        unsupported=list(c['context_supplement'].get('unsupported_conclusions',[]))
        unsupported+= {'SRC-002':['Gossip nil return proves peer delivery and no payload race'],
            'SRC-007':['nil result proves a normal empty list rather than a DB/RLP error'],
            'SRC-011':['returned nextKey proves complete state dump and all resources closed'],
            'SRC-027':['exit always wins ready select or closure return proves outer worker returned'],
            'SRC-031':['Resolve always returns the newest independently verified ENR']} .get(c['candidate_id'],[])
        gold.append(dict(case_id=c['case_id'],candidate_id=c['candidate_id'],work_item_id='WI-C',split='DEV',formal_split=None,
            module=c['module'],source=dict(s,project='go-stablenet',body=body.decode()),required_source_observations=facts,
            required_followups=questions,optional_experiment=dict(kind='unexecuted_proposal',question=c['resource_scope_limit'],
                expected_receipt='owned exact source/caller/schedule or profile; raw success/error/partial/unknown and resource cleanup evidence',actual=None),
            priority=dict(c['priority_triage_draft'],allowed_consumer_enum=['high','medium','low'],
                compatibility_note='Current adopted Planner and Analyzer agree on high/medium/low and32 findings; no stale critical/30 conflict.'),
            six_section_mapping=c['six_section_mapping_proposal'],unknowns=c['existing_unknowns']+c['context_supplement'].get('remaining_unknowns',[]),
            unsupported_conclusions=unsupported,finite_AI_vocabulary_complete=True,semantic_required_gold_complete=False,
            repository_wide_complete=False,independent_family=None,independent_review_receipt=None,actual_dual_tool=None,
            actual_installed_role=None,actual_report_artifacts=None,omission_count=None,priority_correctness=None,
            quality_scoring_allowed=False,exposure='already_exposed_DEV_not_fresh_FINAL'))
        public.append(dict(case_id=c['case_id'],candidate_id=c['candidate_id'],module=c['module'],split='DEV',formal_split=None,
            source={k:v for k,v in s.items() if not k.startswith('excerpt')},source_excerpt='source/'+c['candidate_id']+'.go.txt',
            task='Review data/state, guards/exceptions, complexity, resources, I/O and priority tradeoffs with exact source evidence and unknowns. Deliver current typed artifacts and validated handoff.',
            contains_gold=False,approved=False,exposure='already_exposed_DEV'))
    contract=consumer_contract(consumer)
    return dict(public=public,gold=gold,consumer=contract,summary=dict(kind=KIND,DEV_cases=len(gold),modules=len({r['module'] for r in gold}),
        required_AI_source_observations=sum(len(r['required_source_observations']) for r in gold),required_AI_followups=sum(len(r['required_followups']) for r in gold),
        optional_unexecuted_experiments=len(gold),unsupported_conclusions=sum(len(r['unsupported_conclusions']) for r in gold),
        AI_drafts_requested=6,AI_drafts_prepared=6,actual_project_dual_tool_calls=0,actual_installed_role_runs=0,
        independent_approved=0,independent_families=0,FINAL=0,D04='review_approval_pending_no_personal_names_required',
        formal_completed=0,formal_total=9,freeze=None,execution_allowed=False,actual_quality_scores=None,new_model_calls=0))


def consumer_contract(texts):
    analyzer=texts['plugin/agents/analyzer.md'];planner=texts['plugin/agents/planner.md'];script=texts['plugin/scripts/deep_review.py']
    aliases=['mcp__plugin_coding-agent_cks__cks_context_'+n for n in ('value_dependencies','rule_candidates','value_dependency_expansion','policy_security','review_function')]
    if not all(a in analyzer for a in aliases) or not all(a in planner for a in aliases if a.endswith(('review_function','rule_candidates'))):raise ValueError('current tool aliases missing')
    for needle in ('high | medium | low','at most 32 findings','ANALYSIS -> PLANNING -> COMPLETION'):
        if needle not in planner:raise ValueError('current Planner contract differs')
    for needle in ("MAX_FINDINGS=32","'identity_only_unverified_snapshot'","'bounded_cfg_not_execution_trace'","'retained_file_membership'","'independent_acceptance':'not_recorded'"):
        if needle not in script:raise ValueError('current typed contract differs')
    return dict(commit=CONSUMER_REV,files={n:f.sha(v.encode()) for n,v in texts.items()},
        aliases=aliases,priority_enum=['high','medium','low'],max_findings=32,sections=list(SECTIONS),artifacts=list(ARTIFACTS),
        prior_alias_and_priority_conflict='resolved_by_adopted_candidate_not_reopened',
        structural_snapshot='identity_only_unverified_snapshot_not_atomic_cross_call',sequence='bounded_CFG_flowchart_not_runtime_sequence_trace',
        modules='retained_file_membership_interprocedural_unknown',
        handoff='Analyzer ANALYSIS owns typed build; Planner PLANNING reuses5 exact artifacts and verifies CLI, binds report hash; Orchestrator checks completion hook and CLI. Revisions rebuild5 and clear stale PLANNING binding.',
        installed_role='plugin metadata absolute root, actual exposed tools/hook dispatch/CLI and source/state hashes require installed-role receipts; source registration and unit fixtures do not prove real model execution',
        limitations='Write/Edit hook coverage is not universal shell-edit protection; hashes are not reviewer/transport authentication; syntax/byte equality does not prove diagram rendering, semantic omission0 or priority correctness',
        quality='required source/requirement/impact gold and independent semantic matching remain separate from6sections/criterionIDs/high-medium-low enum',
        actual_installed_role_receipt=None,actual_dual_tool_receipt=None,actual_render_receipt=None)


def contract():
    return dict(status='AI_DEV_not_frozen',scope='10 exposed source reviews in8 modules;64 exact original source/context observations plus20 unknown required followups; optional10 experiments remain unexecuted',
        complete='finite authored vocabulary only; not complete product requirements or independent semantic gold',
        identity='project/dataset/session+commit/source mode/canonical/span/blob/snapshot; original full-file SHA is not retained function blob SHA; exact actual adapter required',
        matching='candidate claims must match meaning and exact support, assumptions and limitations; criterion/section/id/name presence and byte hashes alone never establish semantic coverage',
        omitted='missing observation/followup/required artifact, error, partial and abstention remain in planned denominators; unknown followup with accurate limitation is distinct from fabricated coverage',
        priorities='conditional source triage, no level assigned without requirements/reachability/impact; consumer high/medium/low <=32 only checks format',
        artifacts='5 source/state-bound typed outputs plus actual CLI/hook/state/installed role receipts; CFG and membership scope retained; render and semantic evaluation separate',
        scoring='omission_count and priority correctness null until reviewed complete required set and matching; no success percentage from64anchors or6sections',
        acceptance='fresh independent10cases >=5modules, complete independently approved gold, required omission0 and source/artifact/state binding1.0 remain proposals pending F01-F08',
        independent_review=None,formal_scoring_allowed=False)


def review_ledger(data):
    return dict(complete=False,independent_approved=0,
        source_observation_count=data['summary']['required_AI_source_observations'],followup_count=20,
        resolved_historical_contract_gaps=['five Analyzer aliases','Planner high_medium_low_max32','typed five-artifact handoff source contract'],
        missing=['independent_case_families_fresh_DEV_FINAL','complete_semantic_required_set_equivalent_answers_and_priority_impact_gold',
            'actual_dual_tool_pins_and_structural_snapshot_limitations','actual_installed_role_state_CLI_hook_handoff',
            'actual_five_artifact_delivery_and_rendering','all_planned_error_partial_unknown_abstention_denominators','independent_review_and_F01_F08'],
        reviewer_identity_policy=previous.reviewer_identity_policy(),actual_project_receipts=0)


def historical_errors(root=ROOT):
    errors=previous.historical_errors(root)
    for relative in (previous.PACKET,GAPS):
        p=root/relative;m=f.read(p/'manifest.json')
        if (p/'manifest.json').read_bytes()!=subprocess.check_output(['git','show',REVISION+':'+str(relative/'manifest.json')],cwd=root):errors.append('historical manifest differs '+str(relative))
        for n,h in m['artifacts'].items():
            if f.sha(safe(p,n).read_bytes())!=h:errors.append('historical artifact differs '+str(relative/n))
        for t in m.get('tools',[]):
            if f.sha(subprocess.check_output(['git','show',REVISION+':'+t['path']],cwd=root))!=t['sha256']:errors.append('historical tool differs '+t['path'])
    return errors


def reference(root=ROOT):
    return dict(manifest=str(PACKET/'manifest.json'),sha256=f.sha((root/PACKET/'manifest.json').read_bytes()),status='WIC_DEV10_finite_AI_review_not_independent_required_gold',report=str(PACKET/'REPORT.txt'),worklist=str(PACKET/'WORKLIST.json'))


def current_indices(root=ROOT,protocol_override=None,ledger_override=None):
    paths=[Path('docs/spec-driven/OPERATIONAL-ACCEPTANCE-'+n+'.json') for n in ('PROTOCOL','PROGRESS','DECISION-PROPOSAL')]+[BASE/'GAPS.json'];docs=[f.read(root/p) for p in paths]
    if protocol_override is not None:docs[0]=protocol_override
    if ledger_override is not None:docs[1]=ledger_override
    old=json.loads(subprocess.check_output(['git','show',REVISION+':'+str(paths[0])],cwd=root));ref=reference(root);state=f.read(root/PACKET/'WORKLIST.json');current=dict(kind=KIND,manifest=ref['manifest'],manifest_sha256=ref['sha256']);errors=[]
    for d in docs:
        if d.get('reviewer_identity_policy')!=previous.reviewer_identity_policy():errors.append('reviewer privacy policy differs')
        if d.get('wic_dev_gold_preparation')!=ref or d.get('current_execution_state')!=state or d.get('current_readiness_preparation')!=current:errors.append('WI-C current reference differs')
        for key in ('wisp_dev_gold_preparation','wi6_dev_gold_preparation','ticket_dev_gold_preparation')+previous.previous.previous.PREPARATIONS:
            expected=dict(old[key])
            if key=='wisp_dev_gold_preparation':expected['historical_source_revision']=REVISION
            if d.get(key)!=expected:errors.append('previous preparation promoted '+key)
    if docs[0]['freeze']is not None or docs[0]['execution_allowed']is not False or any(docs[0]['roles'].values()):errors.append('AI cannot freeze or assign reviewers')
    link=docs[0]['adoption_acceptance_decision_proposal']
    if link['sha256']!=f.sha((root/paths[2]).read_bytes()) or any(d.get('adoption_acceptance_decision_proposal')!=link for d in (docs[1],docs[3])):errors.append('proposal link differs')
    return errors


def validate(root=ROOT,packet=None):
    p=packet or root/PACKET;errors=[]
    try:
        m=f.read(p/'manifest.json');actual={x.relative_to(p).as_posix():f.sha(x.read_bytes()) for x in p.rglob('*') if x.is_file() and x!=p/'manifest.json'}
        if m['kind']!=KIND or m['previous_source_revision']!=REVISION or [t['path'] for t in m['tools']]!=TOOLS or actual!=m['artifacts']:errors.append('WI-C manifest scope/hash differs')
        for t in m['tools']:
            if f.sha((root/t['path']).read_bytes())!=t['sha256']:errors.append('tool binding differs '+t['path'])
        if m['source_inputs']!=[]:errors.append('unexpected source input')
        errors.extend(historical_errors(root));data=derive(root,p)
        for name,key in [('public/DEV-inputs.json','public'),('SUMMARY.json','summary'),('consumer-contract.json','consumer')]:
            if f.read(p/name)!=data[key]:errors.append('WI-C derived draft differs '+name)
        if json.loads(gzip.decompress((p/'private/AI-gold.json.gz').read_bytes()))!=data['gold']:errors.append('WI-C bounded gold differs')
        for a,b in zip(data['public'],data['gold']):
            if safe(p/'public',a['source_excerpt']).read_bytes()!=b['source']['body'].encode():errors.append('public source excerpt differs')
        if (p/'.gitattributes').read_text()!=ATTRIBUTES:errors.append('raw Git object scope differs')
        if f.read(p/'matching-contract.json')!=contract() or f.read(p/'REVIEW-LEDGER.json')!=review_ledger(data):errors.append('WI-C contract/debt changed or promoted')
        if f.read(p/'reviewer-identity-policy.json')!=previous.reviewer_identity_policy():errors.append('privacy policy changed')
        state=f.read(p/'WORKLIST.json')
        if state['formal_completed']!=0 or state['formal_total']!=9 or state['reviewer_assignment']!='미정' or state['independent_quality_or_ontology_acceptance']is not False or state['production_activation']is not False or state['publication']is not False or state['AI_DEV_draft_progress']['remaining_work_items']!=[]:errors.append('worklist falsely completed')
        from independent_acceptance import native_reuse_record
        if native_reuse_record(root)['changed_bound_files']:errors.append('native source changed')
    except (ValueError,TypeError,KeyError,OSError,IndexError) as exc:errors.append(str(exc))
    return errors
