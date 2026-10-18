"""Finite source-reviewed WI3 DEV drafts, not corpus-wide or independent gold."""
from collections import Counter
import gzip
import json
from pathlib import Path
import subprocess
import wi2_consumer_matching as previous
from source_packet import verify, safe

f = previous.f
ROOT, BASE = previous.ROOT, previous.BASE
PACKET = BASE/'followups/wi3-dev-gold-v1'
REVISION = '5b72156debceff5af54e7bf94a8399cb02681b33'
KIND = 'WI3_bounded_AI_DEV_not_formal_execution'
INPUTS = BASE/'symptom-inputs-v1'
GAPS = BASE/'symptom-gap-v1'
AUTHOR = Path('scripts/fixtures/wi3-dev-authoring.json')
TOOLS = ['scripts/'+s for s in ('wi3_dev_gold.py','prepare-wi3-dev-gold.py',
    'check-wi3-dev-gold.py','test_wi3_dev_gold.py','fixtures/wi3-dev-authoring.json',
    'source_packet.py','independent_acceptance.py','wi2_consumer_matching.py',
    'wi2_dev_gold.py','wi1_dev_gold.py','wi1_consumer_matching.py',
    'check-operational-acceptance.py','test_operational_acceptance.py')]


def original(root=ROOT):
    proof = verify(root/INPUTS, f.SOURCE_COMMIT)
    public = f.read(root/INPUTS/'public-inputs.json')
    reviews = f.read(root/INPUTS/'private-source-review.json')['reviews']
    for row in reviews:
        s = row['source']; raw = safe(root/INPUTS, proof['files'][s['path']]['artifact']).read_bytes()
        if f.sha(raw) != s['file_sha256'] or f.sha(raw[s['start_byte']:s['end_byte_exclusive']]) != s['function_sha256']:
            raise ValueError('function source identity differs')
    return public, reviews, proof


def witness(raw, source, text):
    a, b = source['start_byte'], source['end_byte_exclusive']
    body = raw[a:b]; needle = text.encode()
    if body.count(needle) != 1:
        raise ValueError('ambiguous or missing manually authored witness '+text)
    start = a+body.index(needle)
    return dict(start_byte=start, end_byte_exclusive=start+len(needle),
                span_sha256=f.sha(needle), text=text)


def derive(root=ROOT):
    public, reviews, proof = original(root)
    notes = f.read(root/AUTHOR)
    ids = [r['case_id'] for r in reviews]
    if len(ids) != 20 or [n['case_id'] for n in notes] != ids:
        raise ValueError('20-card source authoring population differs')
    notes_by_id = {n['case_id']:n for n in notes}
    locations = []
    for row in reviews:
        s = row['source']; raw = safe(root/INPUTS, proof['files'][s['path']]['artifact']).read_bytes()
        identity = dict(project='go-stablenet', source_mode='committed', commit=f.SOURCE_COMMIT,
                        path=s['path'], file_sha256=s['file_sha256'], function_sha256=s['function_sha256'],
                        start_byte=s['start_byte'], end_byte_exclusive=s['end_byte_exclusive'])
        equivalence = [witness(raw,s,notes_by_id[row['case_id']]['primary_witness_text'])]
        for n in notes:
            for support in n['supporting_relevant']:
                if support['location_case_id'] == row['case_id']:
                    w = witness(raw,s,support['witness_text'])
                    if w not in equivalence: equivalence.append(w)
        locations.append(dict(id=row['case_id'], key=f.sha(f.canonical(identity)), identity=identity,
            name=s['name'], receiver=s['receiver'], canonical_id=None,
            source_behavior_draft=notes_by_id[row['case_id']]['source_behavior_draft'],
            equivalence_spans=equivalence, source_fact_anchors=row['source_anchors']))
    by_id = {l['id']:l for l in locations}
    gap = {r['case_id']:r for r in f.read(root/GAPS/'private-case-gaps.json')['cases']}
    wire = f.read(root/GAPS/'public-bundle/wire-requests.json')
    inputs, gold = [], []
    for card, row, note in zip(public['cases'],reviews,notes):
        identity = card['case_id']; review = gap[identity]
        # No location names, family answers, labels, or authoring paths in public inputs.
        p = dict(card, split='DEV', formal_split=None, approved=False, contains_gold=False,
                 prior_exposure='already_exposed_DEV_not_fresh_FINAL')
        inputs.append(p)
        positives = {identity:dict(kind='direct_behavior', rationale=note['source_behavior_draft'])}
        for r in note['supporting_relevant']:
            positives[r['location_case_id']] = dict(kind='supporting_context',rationale=r['rationale'])
        unknown = {r['location_case_id']:r['rationale'] for r in note['unknown']}
        if not set(positives) <= set(ids) or not set(unknown) <= set(ids) or set(positives)&set(unknown) or note['primary']!=identity:
            raise ValueError('manual relevance decisions overlap or use unknown identity')
        units = []
        for l in locations:
            label = 'relevant' if l['id'] in positives else 'unknown' if l['id'] in unknown else 'irrelevant'
            rationale = (positives[l['id']]['rationale'] if label=='relevant' else unknown[l['id']] if label=='unknown'
                else note['negative_review']+' Question contract: '+note['question_contract_draft']+
                     '; inspected local candidate behavior: '+l['source_behavior_draft'])
            units.append(dict(location_id=l['id'],location_key=l['key'],label=label,
                relevance_kind=positives.get(l['id'],{}).get('kind'),rationale=rationale,
                evidence_identity=l['identity'],root_cause_established=False,
                transitive_or_runtime_effect='not_established'))
        sets = {name:[u['location_key'] for u in units if u['label']==label] for name,label in
                [('relevant_locations','relevant'),('irrelevant_locations','irrelevant'),('unknown_locations','unknown')]}
        gold.append(dict(case_id=identity,work_item_id='WI-3',split='DEV',formal_split=None,
            prompt_sha256=f.sha(card['prompt'].encode()),family_proposal=row['family_group_proposal'],
            independent_family=None,exposure=review['exposure'],question_contract_draft=note['question_contract_draft'],
            universe=units,answer_sets=sets,
            empty_set_rationales={k:'No '+k+' in this explicitly selected20-function AI review universe; not a repository-wide absence claim.' for k,v in sets.items() if not v},
            source_unknowns=row['unverified_assumptions'],source_fact_anchors=row['source_anchors'],
            complete_within_declared_universe=True,semantic_gold_complete=False,
            repository_wide_labels=None,outside_universe_policy='unknown_unlabelled_never_automatic_negative',
            actual_retrieval_receipt=None,current_scores=None,reviewer=None,independent_review_receipt=None,
            formal_scoring_allowed=False,origin='AI_manual_full_function_source_review_not_retrieval_output'))
    summary = dict(kind=KIND,DEV_inputs=20,source_functions=20,source_files=17,symptom_types=7,
        proposed_groups=15,independent_approved_families=0,missing_independent_minimum=20,
        units=400,labels=dict(Counter(u['label'] for d in gold for u in d['universe'])),
        source_fact_anchors=40,exposed_DEV=20,FINAL_cases=0,independent_approved=0,
        formal_completed=0,formal_total=9,D04='미정',freeze=None,execution_allowed=False,
        semantic_gold_complete=False,repository_wide_gold_complete=False,current_scores=None,
        new_retrieval_calls=0,new_MCP_calls=0,external_model_calls=0)
    return dict(public=inputs,locations=locations,gold=gold,summary=summary,wire=wire)


def matching_contract():
    return dict(status='AI_proposed_offline_adapter_not_frozen',unit='question x selected function identity',
        universe='20 committed selected functions in17 files only; all400 units labelled; outside universe unknown',
        relevance_definition='direct implementation of the local question behavior or explicitly evidenced supporting producer/consumer context; related but unproved transitive contracts unknown; not root cause',
        identity='project_id/dataset_id/source_mode/commit/graph snapshot then path/file/function SHA and exact UTF8 full-function or declared witness span',
        canonical_mapping='pending_actual_retained_adapter; canonical or name-only response is unknown',
        top10='first10 raw returned candidate positions in discovery order; collapse repeated verified function keys without refill; unknown/refusal/outside slots retained',
        stage='label_suspects final candidates; not backend vector K, expanded signals, multi-round union, or installed-agent hypotheses',
        failures='planned20 input denominator retained; partial/budget/error/unreached/abstention remain separate coverage states; no success-only filtering',
        metrics='not_computed; future reviewed complete relevant sets, case/family macro and zero-denominator/baseline rules must be frozen',
        independent_review=None,formal_scoring_allowed=False)


def match_hit(hit, locations, pins):
    """Offline proposal only. Actual wire/citation adapter remains pending."""
    for key in ('project_id','dataset_id','commit','source_mode','graph_snapshot_sha256'):
        if key not in hit: return dict(status='unknown_missing_scope',location_key=None)
        if hit[key] != pins[key]: return dict(status='refused_wrong_'+key,location_key=None)
    required = ('path','file_sha256','function_sha256','start_byte','end_byte_exclusive','unit_kind')
    if any(k not in hit for k in required): return dict(status='unknown_missing_location',location_key=None)
    candidates = [l for l in locations if all(hit[k]==l['identity'][k] for k in ('path','file_sha256','function_sha256'))]
    if len(candidates)!=1: return dict(status='outside_or_unresolved_unknown',location_key=None)
    l = candidates[0]; a,b = hit['start_byte'],hit['end_byte_exclusive']
    accepted = (hit['unit_kind']=='function' and (a,b)==(l['identity']['start_byte'],l['identity']['end_byte_exclusive']))
    accepted = accepted or (hit['unit_kind']=='span' and any((a,b,hit.get('span_sha256'))==(s['start_byte'],s['end_byte_exclusive'],s['span_sha256']) for s in l['equivalence_spans']))
    return dict(status='mapped' if accepted else 'unknown_non_equivalent_span',location_key=l['key'] if accepted else None)


def project_top10(hits, locations, pins):
    seen = set(); rows = []
    for rank,hit in enumerate(hits[:10],1):
        row = match_hit(hit,locations,pins); key = row['location_key']
        row.update(raw_rank=rank,duplicate=bool(key and key in seen))
        if key: seen.add(key)
        rows.append(row)
    return dict(raw_positions=rows,unique_mapped_keys=list(dict.fromkeys(r['location_key'] for r in rows if r['location_key'])),
                raw_limit=10,score=None,actual_retrieval=False)


def historical_errors(root=ROOT):
    errors = previous.historical_errors(root)
    for relative in (previous.PACKET,INPUTS,GAPS,BASE/'symptom-wire-preflight-v1'):
        p=root/relative; m=f.read(p/'manifest.json')
        original_manifest=subprocess.check_output(['git','show',REVISION+':'+str(relative/'manifest.json')],cwd=root)
        if original_manifest!=(p/'manifest.json').read_bytes(): errors.append('frozen manifest differs '+str(relative))
        for name,digest in m['artifacts'].items():
            if f.sha((p/name).read_bytes())!=digest: errors.append('frozen artifact differs '+str(relative/name))
        if relative==previous.PACKET:
            for row in m['tools']:
                if f.sha(subprocess.check_output(['git','show',REVISION+':'+row['path']],cwd=root))!=row['sha256']: errors.append('historical tool differs '+row['path'])
    return errors


def reference(root,packet):
    return dict(manifest=str(PACKET/'manifest.json'),sha256=f.sha((packet/'manifest.json').read_bytes()),
        status='WI3_DEV20_bounded_AI_gold_not_independent_gold',report=str(PACKET/'REPORT.txt'),worklist=str(PACKET/'WORKLIST.json'))


def current_indices(root=ROOT,protocol_override=None,ledger_override=None):
    paths=[Path('docs/spec-driven/OPERATIONAL-ACCEPTANCE-'+n+'.json') for n in ('PROTOCOL','PROGRESS','DECISION-PROPOSAL')]+[BASE/'GAPS.json']
    docs=[f.read(root/p) for p in paths]
    if protocol_override is not None: docs[0]=protocol_override
    if ledger_override is not None: docs[1]=ledger_override
    ref=reference(root,root/PACKET);state=f.read(root/PACKET/'WORKLIST.json');current=dict(kind=KIND,manifest=ref['manifest'],manifest_sha256=ref['sha256']);errors=[]
    old=json.loads(subprocess.check_output(['git','show',REVISION+':'+str(paths[0])],cwd=root))
    for d in docs:
        if d.get('wi3_dev_gold_preparation')!=ref or d.get('current_execution_state')!=state or d.get('current_readiness_preparation')!=current: errors.append('WI3 current reference/state differs')
        for key in ('wi2_consumer_matching_preparation','wi2_dev_gold_preparation','wi1_consumer_matching_preparation','wi1_dev_gold_preparation','independent_freeze_preparation'):
            expected=dict(old[key])
            if key=='wi2_consumer_matching_preparation': expected['historical_source_revision']=REVISION
            if d.get(key)!=expected: errors.append('historical preparation promoted '+key)
    if docs[0]['freeze'] is not None or docs[0]['execution_allowed'] is not False or any(docs[0]['roles'].values()): errors.append('AI source drafts cannot freeze, execute or assign reviewers')
    proposal=docs[0]['adoption_acceptance_decision_proposal']
    if proposal['sha256']!=f.sha((root/paths[2]).read_bytes()) or any(d.get('adoption_acceptance_decision_proposal')!=proposal for d in (docs[1],docs[3])): errors.append('proposal link differs')
    return errors


def validate(root=ROOT,packet=None):
    p=packet or root/PACKET;errors=[]
    try:
        m=f.read(p/'manifest.json');actual={x.relative_to(p).as_posix():f.sha(x.read_bytes()) for x in p.rglob('*') if x.is_file() and x!=p/'manifest.json'}
        if m['kind']!=KIND or m['previous_source_revision']!=REVISION or [r['path'] for r in m['tools']]!=TOOLS or actual!=m['artifacts']: errors.append('manifest scope/inventory differs')
        for row in m['tools']:
            if f.sha((root/row['path']).read_bytes())!=row['sha256']: errors.append('tool binding differs '+row['path'])
        errors.extend(historical_errors(root))
        from independent_acceptance import native_reuse_record
        if native_reuse_record(root)['changed_bound_files']: errors.append('native source changed')
        expected=derive(root)
        _,reviews,proof=original(root)
        for row in reviews:
            s=row['source'];raw=safe(root/INPUTS,proof['files'][s['path']]['artifact']).read_bytes()
            if (p/'private/source'/(row['case_id']+'.go.txt')).read_bytes()!=raw[s['start_byte']:s['end_byte_exclusive']]: errors.append('source excerpt differs')
        for name,where in [('public','public/DEV-inputs.json'),('locations','private/location-catalog.json'),('gold','private/AI-gold.json')]:
            if f.read(p/where)!=expected[name]: errors.append('bounded draft/public/source differs '+name)
        if f.read(p/'SUMMARY.json')!=expected['summary']: errors.append('summary labels/approval differs')
        if (p/'public/wire-requests.json').read_bytes()!=(root/GAPS/'public-bundle/wire-requests.json').read_bytes(): errors.append('wire inputs changed or leaked gold')
        if f.read(p/'matching-contract.json')!=matching_contract(): errors.append('matching proposal changed or promoted')
        source=f.read(root/previous.PACKET/'runtime/dataset-identity.json');snap=f.read(root/previous.PACKET/'runtime/project-results.json')[0]['response']['snapshot_sha256']
        pins=dict(project_id=source['source']['project_id'],dataset_id=source['dataset_id'],commit=f.SOURCE_COMMIT,source_mode='committed',graph_snapshot_sha256=snap)
        if f.read(p/'proposed-reader-pins.json')!=dict(pins=pins,status='historical_pilot_identity_not_fresh_retrieval_or_baseline_approval',actual_search_receipt=None): errors.append('search pins or execution fabricated')
        exposure=f.read(p/'exposure-and-families.json')
        if exposure!=dict(families=f.read(root/GAPS/'family-review.json'),pairs=f.read(root/GAPS/'pairwise-review.json'),prior=f.read(root/GAPS/'prior-question-review.json'),all_exposed=True,independent_families=0,FINAL=0): errors.append('exposure/family evidence hidden or promoted')
        state=f.read(p/'WORKLIST.json')
        if state['reviewer_assignment']!='미정' or state['formal_completed']!=0 or state['formal_total']!=9 or state['independent_quality_or_ontology_acceptance'] is not False or state['production_activation'] is not False or state['publication'] is not False: errors.append('worklist promoted')
        for d in expected['gold']:
            if (p/'private'/(d['case_id']+'-REVIEW.txt')).read_text()!=render_review(d,expected['locations']): errors.append('readable review differs')
    except (KeyError,ValueError,TypeError,OSError,IndexError,StopIteration,AssertionError) as exc: errors.append(str(exc))
    return errors


def render_review(gold,locations):
    names={l['key']:l for l in locations}
    lines=[gold['case_id']+' — bounded AI DEV, independent reviewer unassigned',gold['question_contract_draft'],'']
    for u in gold['universe']:
        l=names[u['location_key']];lines.append(json.dumps(dict(location=l,review=u),ensure_ascii=False,sort_keys=True))
    lines+=['',json.dumps({k:gold[k] for k in ('answer_sets','empty_set_rationales','source_unknowns')},ensure_ascii=False,sort_keys=True),'Outside selected20 functions: unknown/unlabelled. No root-cause, safety, retrieval score, family independence or FINAL approval.']
    return '\n'.join(lines)+'\n'
