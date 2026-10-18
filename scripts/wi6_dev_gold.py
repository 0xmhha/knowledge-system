"""Finite source relevance drafts for paired WI-6, never retrieval-authored gold."""
from collections import Counter
import gzip,json,subprocess
from pathlib import Path
import ticket_dev_gold as previous
from source_packet import safe,verify
from wi3_consumer_matching import line_span
from wi3_dev_gold import match_hit

ROOT,BASE,f=previous.ROOT,previous.BASE,previous.f
PACKET=BASE/'followups/wi6-dev-gold-v1'
REVISION='875cfc29ae44fb99035dd884613e60946dd62a45'
KIND='WI6_bounded_paired_AI_DEV_not_formal_execution'
INPUTS=BASE/'expansion-v1';GAPS=BASE/'expansion-gap-v1'
AUTHOR=Path('scripts/fixtures/wi6-dev-authoring.json')
TOOLS=['scripts/'+s for s in ('wi6_dev_gold.py','prepare-wi6-dev-gold.py','check-wi6-dev-gold.py','test_wi6_dev_gold.py',
    'fixtures/wi6-dev-authoring.json','ticket_dev_gold.py','prepare-ticket-dev-gold.py','prepare-wi4-dev-gold.py',
    'wi3_consumer_matching.py','wi3_dev_gold.py','source_packet.py','independent_acceptance.py',
    'check-operational-acceptance.py','test_operational_acceptance.py')]
ATTRIBUTES='private/source/objects/'+f.SOURCE_COMMIT+'.commit -whitespace\n'


def derive(root=ROOT,source=None):
    source=source or root/PACKET/'private/source'
    proof=verify(source,f.SOURCE_COMMIT)
    public=f.read(root/INPUTS/'public-inputs.json')['cases'];notes=f.read(root/AUTHOR)
    reviews={r['case_id']:r for r in f.read(root/GAPS/'private-family-pair-gaps.json')['cases']}
    if len(public)!=20 or [r['candidate_id'] for r in public]!=[r['id'] for r in notes['cases']]:raise ValueError('WI6 source population/order differs')
    by_id={r['id']:r for r in notes['cases']};locations=[]
    for r in public:
        s=r['source'];raw=safe(source,proof['files'][s['path']]['artifact']).read_bytes()
        if raw!=(root/INPUTS/r['source_artifact']).read_bytes() or f.sha(raw)!=s['file_sha256'] or f.sha(raw[s['start_byte']:s['end_byte_exclusive']])!=s['function_sha256']:raise ValueError('original function/source identity differs')
        refs=[]
        for a in reviews[r['case_id']]['existing_ai_anchors']:
            start,end,span=line_span(raw,a['start_line'],a['end_line'])
            if (start,end,f.sha(span))!=tuple(a[k] for k in ('start_byte','end_byte_exclusive','span_sha256')):raise ValueError('source anchor differs')
            refs.append(dict(a,text=span.decode()))
        identity=dict(project='go-stablenet',commit=f.SOURCE_COMMIT,source_mode='committed',path=s['path'],
            file_sha256=s['file_sha256'],function_sha256=s['function_sha256'],start_byte=s['start_byte'],end_byte_exclusive=s['end_byte_exclusive'])
        locations.append(dict(id=r['candidate_id'],key=f.sha(f.canonical(identity)),identity=identity,
            name=s['name'],receiver=s['receiver'],scope=r['scope'],behavior=by_id[r['candidate_id']]['behavior'],
            equivalence_spans=[dict(start_byte=a['start_byte'],end_byte_exclusive=a['end_byte_exclusive'],span_sha256=a['span_sha256']) for a in refs],source_refs=refs))
    ids={l['id'] for l in locations};gold=[];inputs=[]
    for r,n in zip(public,notes['cases']):
        positives={n['id']:'Direct implementation of the original local question. '+n['behavior']}
        positives.update(dict(n['relevant']));unknown=dict(n['unknown'])
        if not set(positives)<=ids or not set(unknown)<=ids or set(positives)&set(unknown):raise ValueError('relevance labels overlap/outside universe')
        units=[]
        for l in locations:
            label='relevant' if l['id'] in positives else 'unknown' if l['id'] in unknown else 'noise'
            rationale=positives.get(l['id']) or unknown.get(l['id']) or (
                'Within the explicitly local selected-function scope, this body implements a different question concern. '
                'Question: '+r['prompt']+' Candidate body: '+l['behavior']+
                ' This is not a claim of no transitive/runtime interaction or whole-repository irrelevance.')
            units.append(dict(location_id=l['id'],location_key=l['key'],label=label,rationale=rationale,
                relevance_kind='seed' if l['id']==n['id'] else 'explicit_source_context' if label=='relevant' else None,
                identity=l['identity'],runtime_effect_established=False))
        seed=next(l for l in locations if l['id']==n['id']);sets={label:[u['location_key'] for u in units if u['label']==label] for label in ('relevant','noise','unknown')}
        row=dict(case_id=r['case_id'],work_item_id='WI-6',split='DEV',formal_split=None,independent_family=None,
            prompt_sha256=f.sha(r['prompt'].encode()),question=r['prompt'],intent=r['intent'],seed_id=seed['id'],seed_key=seed['key'],
            seed_scope=r['scope'],universe=units,answer_sets=sets,
            empty_set_rationales={k:'No '+k+' in the selected20-function local-question AI vocabulary; unlabelled external context stays unknown.' for k,v in sets.items() if not v},
            source_unknowns=reviews[r['case_id']]['existing_unknowns'],
            completeness_scope=notes['scope'],complete_within_declared_universe=True,repository_wide_complete=False,
            seed_excluded_relevant=[k for k in sets['relevant'] if k!=seed['key']],
            seed_excluded_recall_defined=bool([k for k in sets['relevant'] if k!=seed['key']]),
            actual_baseline=None,actual_expansion=None,warm_samples=None,current_scores=None,
            reviewer=None,independent_review_receipt=None,formal_scoring_allowed=False,
            operational_trial_eligible=False if r['scope']=='development_stress_source_only' else None,
            exposure='exposed_DEV_prior_WI1_overlap_not_fresh_FINAL')
        gold.append(row)
        inputs.append(dict(r,split='DEV',formal_split=None,prior_exposure='exposed_DEV',
            runtime_execution_allowed=False,contains_gold=False,approved=False,
            actual_seed_resolution=None,actual_baseline_request=None,actual_expansion_request=None))
    summary=dict(kind=KIND,DEV_inputs=20,production_seed_proposals=19,stress_source_only=1,selected_functions=20,source_files=14,
        units=400,labels=dict(Counter(u['label'] for r in gold for u in r['universe'])),
        seed_excluded_positive_cases=sum(r['seed_excluded_recall_defined'] for r in gold),
        seed_excluded_undefined_cases=sum(not r['seed_excluded_recall_defined'] for r in gold),
        source_anchors=40,intents=4,independent_families=0,independent_approved=0,FINAL=0,formal_completed=0,formal_total=9,
        D04='미정',freeze=None,execution_allowed=False,current_scores=None,new_MCP_calls=0,new_model_calls=0,new_target_executions=0)
    return dict(public=inputs,locations=locations,gold=gold,summary=summary)


def contract():
    return dict(status='AI_DEV_proposal_not_frozen',scope='original20 questions x selected20 source functions; explicit local implementation/producer/callee/consumer context only',
        gold='manual full-function source reading;40 original anchors; returned functions never author labels; transitive/runtime/unlabelled outside scope unknown',
        seed='report raw seed separately; same seed excluded from both primary recall/noise scoring universes; report inclusion sensitivity; zero relevant nonseed -> undefined/inconclusive',
        matching='project/dataset/source mode/commit/graph snapshot then file/function SHA and exact function or declared witness span; no name/substring-only joining',
        order='baseline context.value_dependencies and expanded context.value_dependency_expansion same resolved seed/snapshot/caps; preserve actual arm/type/order and all returned slots',
        dedup='verified function identity only within original capped output; keep seed/duplicate/unknown/outside slots, do not refill with unplanned later candidates',
        sampling='19 production proposals +1 source-only stress,0 independently approved;20 independent families and4 intents remain insufficient/unfrozen',
        effect='macro recall delta>=0.10 and noise fraction delta<=0.10 proposals; seed-only relevance cannot establish context benefit',
        failures='planned pairs retained for partial/error/unsupported/budget/unreached/abstention; never success-only denominator or unknown counted as noise',
        empty='empty returned or empty/unknown relevant gold is not recall0/1 or noise0; undefined/inconclusive with coverage reported',
        limits='explicit per-function and aggregate function/node/edge/fact/byte/step/depth budgets required; incoming calls/argument-return transfer/curated join not inferred from expansion',
        warm='separate30 paired post-warmup observations per production case/arm proposed;3 warmup pairs retained separately; same owned stdio process/connection and immutable DB; single concurrency alternating AB/BA order',
        measurement='client end-to-end perf_counter_ns around each call; retain raw args/response/status/time/errors; stable environment/hardware/cache/model/source pins required',
        quantile='nearest rank sorted sample ceil(0.95*n)-1 per arm; ratio=p95(expansion)/p95(baseline), not p95(pairwise ratios); planned failed slots retained',
        latency='nonpositive baseline/insufficient valid complete pairs/changed pins or unsupported measurement -> inconclusive; p95 ratio<=2.0 proposal only; report partial/error timing separately',
        independent_review=None,formal_scoring_allowed=False)


def project_output(hits,draft,locations,pins,cap):
    if type(cap)is not int or cap<1 or cap>64:raise ValueError('invalid declared function cap')
    if not isinstance(hits,list):raise ValueError('output slots must be a list')
    seen=set();rows=[];labels={u['location_key']:u['label'] for u in draft['universe']}
    for rank,hit in enumerate(hits[:cap],1):
        mapped=match_hit(hit,locations,pins) if isinstance(hit,dict) else dict(status='unknown_invalid_slot',location_key=None)
        key=mapped['location_key'];duplicate=bool(key and key in seen)
        if key:seen.add(key)
        rows.append(dict(raw_rank=rank,match=mapped,duplicate=duplicate,seed=bool(key and key==draft['seed_key']),
                         AI_label=labels.get(key,'outside_unknown'),primary_metric_eligible=bool(key and key!=draft['seed_key'] and not duplicate and labels.get(key) in ('relevant','noise'))))
    return dict(raw_slots=rows,score=None,independent_gold=False,actual_retrieval=False)


def review_ledger(data):
    return dict(complete=False,independent_approved=0,actual_pairs=0,warm_raw_samples=None,
        unknowns=[dict(case_id=r['case_id'],location_id=u['location_id'],rationale=u['rationale']) for r in data['gold'] for u in r['universe'] if u['label']=='unknown'],
        seed_excluded_empty_relevant=[r['case_id'] for r in data['gold'] if not r['seed_excluded_recall_defined']],
        missing=['reviewed_broader_nonseed_relevance_universe','independent20_production_families_and4intents',
            'actual_retained_seed_resolution_and_paired_calls','frozen_same_snapshot_scope_caps_and_arm_order',
            'source_adapter_for_actual_returned_function_envelopes','warm_definition_order_raw_samples_and_environment',
            'independent_semantics_matching_sufficiency','D04_F01_F08_and_installed_consumer_roles'])


def historical_errors(root=ROOT):
    errors=previous.historical_errors(root)
    for relative in (previous.PACKET,INPUTS,GAPS):
        p=root/relative;m=f.read(p/'manifest.json')
        if (p/'manifest.json').read_bytes()!=subprocess.check_output(['git','show',REVISION+':'+str(relative/'manifest.json')],cwd=root):errors.append('historical manifest differs '+str(relative))
        for name,h in m['artifacts'].items():
            if f.sha(safe(p,name).read_bytes())!=h:errors.append('historical artifact differs '+str(relative/name))
        for t in m.get('tools',[]):
            if f.sha(subprocess.check_output(['git','show',REVISION+':'+t['path']],cwd=root))!=t['sha256']:errors.append('historical tool differs '+t['path'])
    return errors


def reference(root=ROOT):
    return dict(manifest=str(PACKET/'manifest.json'),sha256=f.sha((root/PACKET/'manifest.json').read_bytes()),
        status='WI6_DEV20_bounded_AI_relevance_seed_excluded_unknown_not_independent_gold',report=str(PACKET/'REPORT.txt'),worklist=str(PACKET/'WORKLIST.json'))


def current_indices(root=ROOT,protocol_override=None,ledger_override=None):
    paths=[Path('docs/spec-driven/OPERATIONAL-ACCEPTANCE-'+n+'.json') for n in ('PROTOCOL','PROGRESS','DECISION-PROPOSAL')]+[BASE/'GAPS.json'];docs=[f.read(root/p) for p in paths]
    if protocol_override is not None:docs[0]=protocol_override
    if ledger_override is not None:docs[1]=ledger_override
    ref=reference(root);state=f.read(root/PACKET/'WORKLIST.json');current=dict(kind=KIND,manifest=ref['manifest'],manifest_sha256=ref['sha256']);errors=[]
    old=json.loads(subprocess.check_output(['git','show',REVISION+':'+str(paths[0])],cwd=root))
    for d in docs:
        if d.get('wi6_dev_gold_preparation')!=ref or d.get('current_execution_state')!=state or d.get('current_readiness_preparation')!=current:errors.append('WI6 current reference differs')
        for key in ('ticket_dev_gold_preparation',)+previous.PREPARATIONS:
            expected=dict(old[key])
            if key=='ticket_dev_gold_preparation':expected['historical_source_revision']=REVISION
            if d.get(key)!=expected:errors.append('previous preparation promoted '+key)
    if docs[0]['freeze']is not None or docs[0]['execution_allowed']is not False or any(docs[0]['roles'].values()):errors.append('AI draft cannot freeze or invent reviewers')
    link=docs[0]['adoption_acceptance_decision_proposal']
    if link['sha256']!=f.sha((root/paths[2]).read_bytes()) or any(d.get('adoption_acceptance_decision_proposal')!=link for d in (docs[1],docs[3])):errors.append('proposal link differs')
    return errors


def validate(root=ROOT,packet=None):
    p=packet or root/PACKET;errors=[]
    try:
        m=f.read(p/'manifest.json');actual={x.relative_to(p).as_posix():f.sha(x.read_bytes()) for x in p.rglob('*') if x.is_file() and x!=p/'manifest.json'}
        if m['kind']!=KIND or m['previous_source_revision']!=REVISION or [t['path'] for t in m['tools']]!=TOOLS or actual!=m['artifacts']:errors.append('WI6 manifest scope/hash differs')
        for t in m['tools']:
            if f.sha((root/t['path']).read_bytes())!=t['sha256']:errors.append('source tool differs '+t['path'])
        errors.extend(historical_errors(root));data=derive(root,p/'private/source')
        if (p/'.gitattributes').read_text()!=ATTRIBUTES:errors.append('raw signed Git object format scope differs')
        for name,key in [('public/DEV-inputs.json','public'),('private/location-catalog.json','locations'),('SUMMARY.json','summary')]:
            if f.read(p/name)!=data[key]:errors.append('WI6 derived data differs '+name)
        if json.loads(gzip.decompress((p/'private/AI-gold.json.gz').read_bytes()))!=data['gold']:errors.append('WI6 complete bounded gold differs')
        if f.read(p/'matching-contract.json')!=contract():errors.append('WI6 matching/warm proposal changed or promoted')
        if f.read(p/'REVIEW-LEDGER.json')!=review_ledger(data):errors.append('WI6 debt erased or promoted')
        state=f.read(p/'WORKLIST.json')
        if state['formal_completed']!=0 or state['formal_total']!=9 or state['reviewer_assignment']!='미정' or state['independent_quality_or_ontology_acceptance']is not False or state['production_activation']is not False or state['publication']is not False:errors.append('worklist falsely completed')
        from independent_acceptance import native_reuse_record
        if native_reuse_record(root)['changed_bound_files']:errors.append('native source changed')
    except (ValueError,TypeError,KeyError,OSError,IndexError,StopIteration) as exc:errors.append(str(exc))
    return errors
