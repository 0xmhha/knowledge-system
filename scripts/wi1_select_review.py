"""Supplemental select receive reference ownership; never independent gold."""
from collections import Counter
import copy
import json
import subprocess
from pathlib import Path
import wi1_consumer_matching as matching
import wi2_contract_audit as previous

ROOT, BASE, f = previous.ROOT, previous.BASE, previous.f
PACKET = BASE / 'followups/wi1-select-review-v1'
REVISION = 'c874ca8e6a27aec396ac5f87d1b30f44b4503638'
KIND = 'WI1_select_reference_supplement_not_formal_execution'
TOOLS = ['scripts/' + n for n in ('wi1_select_review.py', 'wi1_select_source.go.txt',
    'prepare-wi1-select-review.py', 'run-wi1-select-reference.py', 'check-wi1-select-review.py', 'test_wi1_select_review.py',
    'wi1_dev_gold.py', 'wi1_consumer_matching.py', 'wi2_contract_audit.py',
    'check-operational-acceptance.py', 'test_operational_acceptance.py')]


def derive(root=ROOT, packet=None):
    p = packet or root / PACKET
    g = matching.old
    original = g.packed(root / g.PACKET / 'observations.json.gz')
    observed = g.packed(p / 'observations.json.gz')
    candidates = {(r['source']['path'], r['source']['start_byte']): r
                  for r in f.read(root / g.FUNCTIONS)['functions']}
    runtime = {r['case_id']: r for r in f.read(root / matching.PACKET / 'runtime/project-results.json')}
    if len(original) != 30 or len(observed) != 30:
        raise ValueError('original30 function denominator differs')
    changes, pairs, comparisons = [], [], []
    value_labels, lexical_labels = Counter(), Counter()
    missing_phases = missing_blocks = disagreements = 0
    for old, new in zip(original, observed):
        candidate = candidates[(new['path'], new['span']['start'])]
        cid = candidate['candidate_id']
        patched = copy.deepcopy(old)
        if len(old['occurrences']) != len(new['occurrences']):
            raise ValueError('occurrence population changed')
        nodes = {r['id']: r for r in new['nodes']}
        blocks = {r['id']: r for r in new['blocks']}
        for a, b in zip(patched['occurrences'], new['occurrences']):
            if a == b:
                continue
            if a['owner'] or a['mapping_reason'] != 'ambiguous_CFG_AST_identity':
                raise ValueError('non-ambiguous owner replaced')
            if b['mapping_reason'] != 'selected_receive_LHS_exact_AST_case_identity':
                raise ValueError('receive case owner not explicitly evidenced')
            if {k:v for k,v in a.items() if k not in ('owner','mapping_reason')} != {k:v for k,v in b.items() if k not in ('owner','mapping_reason')}:
                raise ValueError('binding/role/source identity changed')
            n = nodes[b['owner']]
            if n['kind'] != 'Ident' or n['span'] != b['span'] or blocks[n['block']]['kind'] != 'SelectCaseBody':
                raise ValueError('receive definition assigned outside exact case LHS')
            changes.append(dict(case_id=cid, before=copy.deepcopy(a), after=b))
            a.update(b)
        if patched != new:
            raise ValueError('unrelated source observation changed')
        gold = g.derive_gold(new, candidate)
        oldgold = g.packed(root / g.PACKET / ('private/' + cid + '.json.gz'))
        if len(gold['data_universe']) != len(oldgold['data_universe']):
            raise ValueError('definition/use pair population changed')
        for a, b in zip(oldgold['data_universe'], gold['data_universe']):
            if (a['definition'],a['use'],a['binding']) != (b['definition'],b['use'],b['binding']):
                raise ValueError('pair identity changed')
            if a != b:
                pairs.append(dict(case_id=cid, before=a, after=b))
        overlay = matching.semantic_overlay(gold)
        oldoverlay = matching.semantic_overlay(oldgold)
        if [(r['definition'],r['use'],r['value_flow_label']) for r in overlay] != [(r['definition'],r['use'],r['value_flow_label']) for r in oldoverlay]:
            raise ValueError('source-only owner correction promoted semantic value gold')
        value_labels.update(r['value_flow_label'] for r in overlay)
        lexical_labels.update(r['lexical_label'] for r in overlay)
        analysis = runtime[cid]['response']['analysis']
        mapped = matching.match(new, analysis)
        compared = matching.comparison(gold, mapped, analysis)
        comparisons.append(dict(case_id=cid, mapping=mapped, comparison=compared))
        missing_phases += mapped['unmatched_occurrence_phases']
        missing_blocks += mapped['unmatched_blocks']
        disagreements += sum(r['lexical_agrees'] is False for r in compared['rows'])
    if len(changes) != 16 or len(pairs) != 37 or disagreements:
        raise ValueError('declared owner/pair/disagreement observations differ')
    if value_labels != Counter(unknown=1427,positive=212,negative=15) or lexical_labels != Counter(positive=1459,negative=195):
        raise ValueError('bounded semantic/lexical universe differs')
    controls = g.packed(p / 'controls-observations.json.gz')
    expected = dict(ReceiveLoop=1,AssignExisting=1,TwoValue=1,ShadowOuter=1,SendOnly=0,DefaultOnly=0,TwoCases=2,Closure=0)
    if len(controls) != len(expected):
        raise ValueError('synthetic control denominator differs')
    for o in controls:
        count = sum(r['mapping_reason']=='selected_receive_LHS_exact_AST_case_identity' for r in o['occurrences'])
        if count != expected[o['name']] or o['partial_file_type_errors']:
            raise ValueError('synthetic receive/control ownership differs')
        if o['name']=='TwoValue':
            ok = next(r for r in o['bindings'] if r['name']=='ok')
            definition = next(r for r in o['occurrences'] if r['binding']==ok['id'] and r['role']=='definition')
            if definition['owner']:
                raise ValueError('missing second receive CFG LHS silently invented')
    if f.read(p/'runtime-witness-output.json') != dict(before=[3,8],after=[5,8]):
        raise ValueError('controlled repeated receive witness differs')
    return dict(owners=changes,pairs=pairs,comparisons=comparisons,
        summary=dict(kind=KIND,functions=30,pairs=1654,owner_corrections=16,changed_pair_records=37,
            changed_functions=6,structural_unknown_to_reaching=32,structural_reaching_to_not_reaching=2,
            unchanged_reaching_reason_corrections=3,value_labels=dict(value_labels),lexical_labels=dict(lexical_labels),
            lexical_disagreements=0,unmatched_occurrence_phases=missing_phases,unmatched_blocks=missing_blocks,
            original_consumer_responses_reused=30,new_actual_consumer_calls=0,synthetic_controls=8,
            controlled_runtime_trials=1,controlled_runtime_receives=2,new_independent_samples=0,
            quality_score=None,independent_approved=0,FINAL=0,formal_completed=0,formal_total=9,
            freeze=None,execution_allowed=False,paid_or_external_LLM_calls=0))


def reference(root=ROOT):
    return dict(manifest=str(PACKET/'manifest.json'),sha256=f.sha((root/PACKET/'manifest.json').read_bytes()),
        status='AI_select_reference_supplement_not_independent_gold',report=str(PACKET/'REPORT.txt'),worklist=str(PACKET/'WORKLIST.json'))


def historical_errors(root=ROOT):
    errors = previous.historical_errors(root)
    p = root/previous.PACKET
    m = f.read(p/'manifest.json')
    if (p/'manifest.json').read_bytes() != subprocess.check_output(['git','show',REVISION+':'+str(previous.PACKET/'manifest.json')],cwd=root):
        errors.append('previous WI2 manifest changed')
    for name,h in m['artifacts'].items():
        if f.sha((p/name).read_bytes()) != h:
            errors.append('historical WI2 artifact changed '+name)
    for r in m['tools']:
        if f.sha(subprocess.check_output(['git','show',REVISION+':'+r['path']],cwd=root)) != r['sha256']:
            errors.append('historical WI2 tool changed '+r['path'])
    return errors


def current_indices(root=ROOT,protocol_override=None,ledger_override=None):
    paths=[Path('docs/spec-driven/OPERATIONAL-ACCEPTANCE-'+n+'.json')for n in ('PROTOCOL','PROGRESS','DECISION-PROPOSAL')]+[BASE/'GAPS.json']
    docs=[f.read(root/x)for x in paths]
    if protocol_override is not None: docs[0]=protocol_override
    if ledger_override is not None: docs[1]=ledger_override
    ref=reference(root);state=f.read(root/PACKET/'WORKLIST.json')
    current=dict(kind=KIND,manifest=ref['manifest'],manifest_sha256=ref['sha256']);errors=[]
    for d,path in zip(docs,paths):
        prior=json.loads(subprocess.check_output(['git','show',REVISION+':'+str(path)],cwd=root))
        if d.get('wi1_select_review')!=ref or d.get('current_execution_state')!=state or d.get('current_readiness_preparation')!=current:
            errors.append('WI1 select current reference/state differs')
        if d.get('reviewer_identity_policy') != previous.contract()['privacy']:
            errors.append('reviewer privacy policy changed')
        for key,value in prior.items():
            if key=='current_readiness_preparation' or not isinstance(value,dict) or 'manifest'not in value: continue
            expected=dict(value)
            if key=='wi2_contract_audit': expected['historical_source_revision']=REVISION
            if d.get(key)!=expected: errors.append('previous preparation changed '+key)
    p=docs[0]
    if p['freeze']is not None or p['execution_allowed']is not False or any(p['roles'].values()):
        errors.append('reference correction cannot approve/freeze/assign roles')
    link=p['adoption_acceptance_decision_proposal']
    if link['sha256']!=f.sha((root/paths[2]).read_bytes())or any(d.get('adoption_acceptance_decision_proposal')!=link for d in (docs[1],docs[3])):
        errors.append('proposal link differs')
    return errors


def validate(root=ROOT,packet=None):
    p=packet or root/PACKET;errors=[]
    try:
        m=f.read(p/'manifest.json')
        actual={x.relative_to(p).as_posix():f.sha(x.read_bytes())for x in p.rglob('*')if x.is_file()and x!=p/'manifest.json'}
        if m['kind']!=KIND or m['previous_source_revision']!=REVISION or m['artifacts']!=actual or [r['path']for r in m['tools']]!=TOOLS:
            errors.append('WI1 select manifest scope/hash differs')
        for r in m['tools']:
            if f.sha((root/r['path']).read_bytes())!=r['sha256']: errors.append('tool changed '+r['path'])
        errors.extend(historical_errors(root));errors.extend(previous.previous.native_errors(root,root/previous.previous.PACKET))
        data=derive(root,p)
        for name,key in [('SUMMARY.json','summary'),('owner-deltas.json','owners'),('pair-deltas.json','pairs')]:
            if f.read(p/name)!=data[key]:errors.append('derived reference differs '+name)
        if matching.old.packed(p/'comparisons.json.gz')!=data['comparisons']:errors.append('rematching differs')
        receipt=f.read(p/'observer-receipt.json')
        for name,digest in receipt['files'].items():
            if f.sha((p/name).read_bytes())!=digest:errors.append('observer evidence differs '+name)
        if receipt['helper_sha256']!=f.sha((root/'scripts/wi1_select_source.go.txt').read_bytes()) or receipt['candidate_valueflow_imported']is not False or receipt['new_consumer_calls']!=0:
            errors.append('observer provenance differs')
        if len(receipt['calls'])!=5 or any(r['exit']!=0 for r in receipt['calls']):errors.append('observer execution unsuccessful or incomplete')
        contract=f.read(p/'matching-contract.json')
        if contract.get('formal_scoring_allowed')is not False or contract.get('independent_approved')!=0 or contract.get('value_flow_labels_unchanged')is not True or contract.get('privacy')!=previous.contract()['privacy']:
            errors.append('reference contract promoted or privacy changed')
        state=f.read(p/'WORKLIST.json')
        if state['formal_completed']!=0 or state['formal_total']!=9 or state['reviewer_assignment']!='미정' or state['independent_quality_or_ontology_acceptance']is not False or state['publication']is not False or state['production_activation']is not False:
            errors.append('worklist falsely completed')
    except (ValueError,KeyError,TypeError,OSError,IndexError) as exc: errors.append(str(exc))
    return errors
