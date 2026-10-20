#!/usr/bin/env python3
"""Prepare WI-V1/V2 source, ticket, oracle and acceptable-set gaps; grant no acceptance."""
import argparse
import ast
import copy
import importlib.util
import json
from pathlib import Path
from collections import Counter

from source_packet import SourceCapture, safe, sha, verify

ROOT = Path(__file__).resolve().parents[1]
BASE = Path('docs/spec-driven/operational-preparation/source-review-supplement')
TICKET = BASE / 'subscription-ticket-v1'
PROBE = BASE / 'subscription-probe-v1'
HOST = Path('system/eval/operational-preparation-2026-10-15/isolated-host')
OLD = Path('system/eval/upper-work-items-implementation-2026-10-14')
REVISION = '0bf2f4d1bfeb6605006d556957ef8c045d8f8ed8'
INPUTS = {'public': TICKET/'public-ticket.json', 'private': TICKET/'private-review.json',
          'probe': PROBE/'summary.json', 'proposals': Path('scripts/fixtures/ticket-gap-proposals.json'),
          'host': HOST/'manifest.json', 'consumer_inventory': HOST/'coding_agent-source-files.json',
          'v1_manifest': OLD/'wi-v1/manifest.json', 'v1_summary': OLD/'wi-v1/smoke-summary.json',
          'v1_plan': OLD/'wi-v1/oracle-validity-plan.json', 'v1_report': OLD/'wi-v1/oracle-validity-report.json',
          'v2_manifest': OLD/'wi-v2/manifest.json',
          'v2_spec': OLD/'wi-v2/model-final/fix-selection-spec.json',
          'v2_report': OLD/'wi-v2/model-final/fix-selection-report.json',
          'v2_summary': OLD/'wi-v2/model-final/selection-smoke-summary.json',
          'v2_fixture': OLD/'wi-v2/fixture/selection-smoke-summary.json',
          'v2_initial': OLD/'wi-v2/model-initial/attempt-status.json'}
MANIFESTS = [TICKET/'manifest.json', PROBE/'manifest.json']
PROJECT = ['eth/backend.go', 'eth/downloader/api.go', 'rpc/service.go', 'rpc/handler.go', 'rpc/subscription.go']
ADOPTED = ['plugin/scripts/oracle_gate.py', 'plugin/scripts/go_oracle_driver.py', 'plugin/scripts/fix_selection.py',
           'plugin/hooks/hooks.json', 'plugin/hooks/fix-selection-guard.py',
           *['plugin/agents/'+r+'.md' for r in ['analyzer','planner','implementer','evaluator']]]
ATTRIBUTES = b'source/*/objects/*.commit binary\nsource/*/objects/*.tree binary\n'
OBLIGATIONS = ['independent_ticket_and_family', 'source_and_transport_reachability', 'defect_and_normal_contract',
               'primary_executable_oracle', 'independently_derived_executable_oracle', 'reintroduction_and_overfit_mutants',
               'frozen_red_carrier_and_candidate_trees', 'independent_acceptable_candidate_set',
               'comparison_axes_drivers_and_hard_gates', 'all_bad_tie_incomplete_mapping',
               'installed_four_role_handoff_and_final_head', 'sample_sufficiency_exposure_dev_final',
               'human_review_and_final_acceptance']


def load(name):
    spec = importlib.util.spec_from_file_location(name.replace('-','_'), ROOT/'scripts'/name)
    module = importlib.util.module_from_spec(spec); spec.loader.exec_module(module)
    return module


helpers = load('prepare-semantic-gap.py')
dump, unique, span = helpers.dump, helpers.unique, helpers.span


def bindings():
    return sorted(set(INPUTS.values()) | set(MANIFESTS) | {Path('scripts/prepare-semantic-gap.py'), Path('scripts/source_packet.py')})


def inputs(packet):
    data = {k: json.loads((ROOT/p).read_text()) for k,p in INPUTS.items()}
    for path in MANIFESTS:
        for name,digest in json.loads((ROOT/path).read_text())['artifacts'].items():
            if sha(safe(ROOT/path.parent,name).read_bytes()) != digest:
                raise ValueError('prior packet binding: '+name)
    for owner in ['v1','v2']:
        directory = OLD/('wi-'+owner)
        for path in INPUTS.values():
            if path.is_relative_to(directory) and path.name != 'manifest.json':
                if data[owner+'_manifest']['files_sha256'][path.relative_to(directory).as_posix()] != sha((ROOT/path).read_bytes()):
                    raise ValueError('historical evidence binding')
    data['sources'] = {}
    for owner, paths, revision in [('project',PROJECT,REVISION),
                                  ('consumer',ADOPTED,data['host']['source_adoptions']['coding_agent']['source_commit'])]:
        root = packet/'source'/owner; proof = verify(root,revision)
        if set(proof['files']) != set(paths):
            raise ValueError('source inventory')
        data['sources'][owner] = {p:safe(root,r['artifact']).read_bytes() for p,r in proof['files'].items()}
    adopted = {r['path']:r['sha256'] for r in data['consumer_inventory']}
    for path, raw in data['sources']['consumer'].items():
        if sha(raw) != adopted[path]:
            raise ValueError('adopted source hash')
    if data['sources']['project']['eth/downloader/api.go'] != (ROOT/BASE/'context-v1/sources/eth/downloader/api.go.txt').read_bytes():
        raise ValueError('existing API source drift')
    protocol = json.loads((ROOT/'docs/spec-driven/OPERATIONAL-ACCEPTANCE-PROTOCOL.json').read_text())
    progress = json.loads((ROOT/'docs/spec-driven/OPERATIONAL-ACCEPTANCE-PROGRESS.json').read_text())
    data['criteria'] = {key:{'protocol':next(r for r in protocol['input_groups'] if r['id']==key),
                             'progress':next(r for r in progress['items'] if r['id']==key)} for key in ['WI-V1','WI-V2']}
    return data


def pure_function(raw, name, namespace):
    # Execute only the verified function AST, never module-level code or a CLI.
    node = next(n for n in ast.parse(raw).body if isinstance(n,ast.FunctionDef) and n.name==name)
    code = compile(ast.Module(body=[node],type_ignores=[]), '<pinned-pure-function>', 'exec')
    exec(code,namespace)
    return namespace[name]


def replay(data):
    source = data['sources']['consumer']
    classify = pure_function(source['plugin/scripts/oracle_gate.py'],'classify',{'json':json,'VERSION':1,'OUTPUT_LIMIT':65536})
    outcome = pure_function(source['plugin/scripts/go_oracle_driver.py'],'outcome',{})
    decide = pure_function(source['plugin/scripts/fix_selection.py'],'decide',{'ROLES':{'correctness','maintainability','regression'}})
    plan={'symptom_id':'local-gap-probe'}; oracle={'assertion_id':'local-assertion'}
    result={'schema_version':1,'symptom_id':plan['symptom_id'],'assertion_id':oracle['assertion_id'],
            'subject_commit':'a'*40,'case_id':'local-case','outcome':'symptom_fail'}
    base={'source_unchanged':True,'timed_out':False,'launch_error':None,'exit_code':1,
          'subject_commit':'a'*40,'case_id':'local-case','stdout':json.dumps(result),'stderr':''}
    rows=[]
    for name, change, output, want in [
        ('bound_symptom',{}, {},'RED'), ('bound_pass',{'exit_code':0},{'outcome':'pass'},'GREEN'),
        ('timeout',{'timed_out':True},{},'ERROR'),('launch_error',{'launch_error':'owned synthetic error'},{},'ERROR'),
        ('source_changed',{'source_unchanged':False},{},'ERROR'),('wrong_assertion',{}, {'assertion_id':'other'},'ERROR'),
        ('wrong_subject',{}, {'subject_commit':'b'*40},'ERROR'),('non_assertion_exit',{'exit_code':2},{},'ERROR'),
        ('missing_output',{'stdout':''},{},'ERROR')]:
        r=copy.deepcopy(base); value={**result,**output};r['stdout']=json.dumps(value);r.update(change)
        actual,reason=classify(r,oracle,plan)
        rows.append({'id':name,'kind':'oracle_classification','expected':want,'actual':actual,'reason':reason})
    events=[{'Action':'run','Test':'TestLocal'},{'Action':'output','Test':'TestLocal','Output':'ORACLE_ASSERTION[local]: failure'},
            {'Action':'fail','Test':'TestLocal'}]
    for name, ev, code, want in [
        ('selected_assertion',events,1,'symptom_fail'),
        ('build_failure',[{'Action':'build-fail'}],1,'error'),
        ('wrong_failure',[{'Action':'run','Test':'TestLocal'},{'Action':'fail','Test':'TestLocal'}],1,'error'),
        ('no_test',[],0,'error'),('selected_pass',[{'Action':'run','Test':'TestLocal'},{'Action':'pass','Test':'TestLocal'}],0,'pass')]:
        rows.append({'id':name,'kind':'go_adapter_outcome','expected':want,'actual':outcome(ev,code,'TestLocal','local')})
    axes={'axes':[{'id':'cost','unit':'synthetic-unit','direction':'min','required':True}]}
    cands=[{'id':i,'commit':c*40,'tree':c*40,'hard_eligible':True,'observation_complete':True,
            'metrics':{'cost':{'unit':'synthetic-unit','value':value}}} for i,c,value in [('A','a',1),('B','b',2)]]
    panel=[{'role':role,'opinion':{'preferred_candidate':'A','votes':[{'candidate_id':c['id'],'verdict':'accept'} for c in cands]}}
           for role in ['correctness','maintainability','regression']]
    for name,want in [('sole_supported','SELECTED'),('all_bad','REJECTED'),('tie','TIED'),
                      ('incomplete_alternative','DEFERRED'),('missing_metric','DEFERRED'),
                      ('panel_disagreement','DEFERRED'),('panel_missing','DEFERRED'),('preference_disagrees','DEFERRED')]:
        cs,ps=copy.deepcopy(cands),copy.deepcopy(panel)
        if name=='all_bad':
            for c in cs:c['hard_eligible']=False
        if name=='tie':cs[1]['metrics']['cost']['value']=1
        if name=='incomplete_alternative':cs[1]['observation_complete']=False;cs[1]['hard_eligible']=False
        if name=='missing_metric':cs[1]['metrics']['cost']['value']=None
        if name=='panel_disagreement':ps[0]['opinion']['votes'][1]['verdict']='abstain'
        if name=='panel_missing':ps.pop()
        if name=='preference_disagrees':ps[0]['opinion']['preferred_candidate']='B'
        actual=decide(axes,cs,ps)
        rows.append({'id':name,'kind':'candidate_decision','expected':want,'actual':actual['status'],'decision':actual})
    if len(unique(rows,'id'))!=22 or any(r['actual']!=r['expected'] for r in rows):
        raise ValueError('pinned pure-function replay mismatch')
    return {'kind':'local_synthetic_pure_function_replay_not_cli_api_or_host','observations':rows,
            'new_model_calls':0,'new_go_executions':0,'new_api_calls':0,'independent_units':0,'formal_runs':0}


def generate(data):
    public, private, probe = data['public'],data['private'],data['probe']
    if public['case_id']!='SUB-EXIT-DRAFT-001' or public['approved'] is not False or private['approved'] is not False:
        raise ValueError('development ticket identity/approval')
    if private['independently_derived_oracle'] is not None or private['independent_acceptable_candidate_set'] is not None or private['verified_project_defect'] is not False:
        raise ValueError('independent gold promotion')
    if private['independent_family_count']!=0 or probe['independent_families']!=0 or probe['formal_runtime_executions']!=0:
        raise ValueError('independent/formal execution promotion')
    if private['secondary_oracle_draft']['kind']!='same_ai_source_derived_wait_graph_not_independent':
        raise ValueError('secondary oracle independence')
    variants=unique(private['variants'],'id')
    if set(variants)!={'ORIGINAL','CAND-A','CAND-B','MUT-REINTRO','MUT-EARLY'} or len(probe['observations'])!=50:
        raise ValueError('variant/local denominator')
    for r in probe['observations']:
        if r['satisfies_ai_fixture_criteria'] != private['expected_draft_matrix'][r['variant']][r['scenario']]:
            raise ValueError('existing observation matrix')
    matrix=[]
    for key, count in sorted(Counter((r['variant'],r['mode']) for r in probe['observations']).items()):
        selected=[r for r in probe['observations'] if (r['variant'],r['mode'])==key]
        matrix.append({'variant':key[0],'mode':key[1],'observations':count,
                       'ai_fixture_criteria_met':sum(r['satisfies_ai_fixture_criteria'] for r in selected),
                       'completed_before_rescue':sum(r['completed_before_rescue'] for r in selected),
                       'two_sender_wait_witnesses':sum(r['two_sender_wait_witness'] for r in selected),
                       'all_owned_goroutines_stopped':all(r['all_owned_goroutines_stopped'] for r in selected),
                       'independent_units':0,'actual_api_runs':0})
    anchors=[];contracts=[]
    for key, owner, out in [('project_anchors','project',anchors),('contracts','consumer',contracts)]:
        for r in data['proposals'][key]:
            out.append({**r,'anchor':span(data['sources'][owner][r['path']],*r['lines']),
                        'kind':'ai_source_inspection_not_runtime_or_gold','approved':False})
        unique(out,'id')
    candidates=[]
    for cid,question in sorted(data['proposals']['candidate_questions'].items()):
        variant=variants[cid]
        candidates.append({'id':cid,'artifact':variant['artifact'],'sha256':sha((ROOT/TICKET/variant['artifact']).read_bytes()),
                           'source_question_draft':question,'local_observations':10,'ai_fixture_criteria_met':10,
                           'independent_acceptable_membership':None,'actual_api_receipt':None,
                           'applied_project_tree':None,'performance_or_fairness_measurements':None})
    if (ROOT/TICKET/variants['MUT-REINTRO']['artifact']).read_bytes() != (ROOT/TICKET/variants['ORIGINAL']['artifact']).read_bytes():
        raise ValueError('original/reintroduction identity')
    v1,v2=data['v1_summary'],data['v2_summary']
    report=data['v1_report']; selection=data['v2_report']
    if report['mechanical_verdict']!='PASS' or len(report['runs'])!=16 or not all(r['passed'] for r in report['checks']):
        raise ValueError('historical oracle execution')
    if selection['decision']!=v2['decision'] or v2['decision']['selected_candidate']!='loop' or v2['independent_human_acceptance']!='not_recorded':
        raise ValueError('historical selection scope')
    old={'kind':'scoped_historical_execution_not_current_ticket_acceptance',
         'v1':{'scope':v1['scope'],'tier':report['tier'],'symptom_id':report['symptom_id'],
               'oracle_runs':len(report['runs']),'mechanical_verdict':report['mechanical_verdict'],
               'run_matrix':[{k:r[k] for k in ['oracle_id','variant','case_id','outcome']} for r in report['runs']],
               'normal_controls':data['v1_plan']['controls'],'mutants':data['v1_plan']['mutants'],
               'completion_check':v1['completion_check'],'valid_completion_hook':v1['valid_completion_hook'],
               'invalid_report_hook':v1['invalid_report_hook'],'independent_human_acceptance':v1['independent_human_acceptance']},
         'v2':{'scope':v2['scope'],'panel_origin':v2['panel_origin'],'decision':v2['decision'],
               'candidates':[{k:r[k] for k in ['id','hard_eligible','observation_complete','metrics','regression'] if k in r} for r in selection['candidates']],
               'criteria':data['v2_spec']['criteria'],
               'panel_roles':[r['role'] for r in selection['panel']],
               'independent_human_acceptance':selection['independent_human_acceptance'],
               'fixture_panel_origin':data['v2_fixture']['panel_origin'],
               'initial_attempt_scope':data['v2_initial'],
               'selection_check':v2['selection_check'],'applied_tree_check':v2['applied_tree_check'],
               'final_current_head_oracle':v2['final_current_head_oracle'],'both_completion_guards':v2['both_completion_guards']},
         'current_development_ticket_covered':False,'approved_independent_tickets':0,
         'independent_acceptable_candidate_set':None,'new_model_calls':0}
    ticket={'status':'ai_gap_review_not_independent_defect_gold_or_acceptance','case_id':public['case_id'],
            'family_proposal':public['family_id_proposal'],'source':public['source'],
            'api_route_draft':{'subscribe_method':'eth_subscribe','subscribe_params':['syncing'],
                               'unsubscribe_method':'eth_unsubscribe','unsubscribe_params':['<subscription-id>'],
                               'requires_subscription_capable_transport':True,'runtime_receipt':None},
            'source_anchors':anchors,'same_scenarios':[s['id'] for s in private['scenarios']],
            'primary_oracle_ai_draft':private['primary_oracle_draft'],
            'secondary_oracle_ai_draft':private['secondary_oracle_draft'],
            'candidates':candidates,'mutants':[variants[k] for k in ['MUT-REINTRO','MUT-EARLY']],
            'reintroduction_bytes_equal_original':True,'local_matrix':matrix,
            'oracle_questions':data['proposals']['oracle_questions'],
            'acceptable_set_questions':data['proposals']['acceptable_set_questions'],
            'obligations':{k:'not_satisfied' for k in OBLIGATIONS},
            'independent_ticket':None,'independently_derived_oracle':None,'independent_acceptable_candidate_set':None,
            'reviewer':None,'review_receipt':None,'split':None,'verified_project_defect':False,
            'operational_gate':'not_run','quality_gate':'not_run','independent_acceptance':'not_recorded'}
    counts={'development_ticket_drafts':1,'ai_source_candidate_drafts':2,'mutants':2,'correlated_scenarios':5,
            'extracted_method_observations':50,'approved_independent_tickets':0,'pending_independent_ticket_minimum':2,
            'approved_independent_oracles':0,'frozen_acceptable_candidate_sets':0,'actual_api_receipts':0,
            'project_source_files':len(PROJECT),'adopted_source_files':len(ADOPTED),
            'project_source_anchors':len(anchors),'consumer_contract_anchors':len(contracts),
            'local_pure_function_checks':22,'formal_runs':0}
    return {'public-ticket.json':public,'private-ticket-gaps.json':ticket,
            'execution-contract-review.json':{'status':'source_contract_review_not_installed_role_execution',
                'adopted_revision':data['host']['source_adoptions']['coding_agent']['source_commit'],
                'contracts':contracts,'criteria_snapshot':data['criteria'],
                'independent_set_membership_adapter_or_receipt':None,'new_host_calls':0,'formal_runs':0},
            'historical-evidence-scope.json':old,'local-replay.json':replay(data),
            'gap-summary.json':{'kind':'ticket_api_independence_oracle_and_acceptable_set_gaps_preparation_only',
                'counts':counts,'work_item_scores':None,'protocol_or_criteria_changed':False,
                'missing_membership_is_null_not_empty_approved_set':True}}


def table(artifacts):
    ticket=artifacts['private-ticket-gaps.json']
    lines=['WI-V1/WI-V2 API·독립 티켓·오라클·수용 가능 후보 집합 — 비공개 AI 준비 자료',
           '개발 티켓 1개/AI 후보 2개/추출 관측 50건. 독립 승인 티켓·오라클·후보 집합 0개.',
           'eth_subscribe(["syncing"]) 경로 원문 확인; 실제 API/설치 역할 실행 영수증 없음.']
    for r in ticket['source_anchors']+artifacts['execution-contract-review.json']['contracts']:
        lines+=['',r['id']+' '+r['path']+':'+str(r['lines'][0]),r['fact'],'미충족: '+r['gap']]
    for r in ticket['candidates']:
        lines+=['',r['id']+' '+r['source_question_draft'],'독립 수용 가능 membership: null']
    lines+=['','독립 oracle 검토 질문:',*ticket['oracle_questions'],'','수용 가능 집합/비교 기준 질문:',*ticket['acceptable_set_questions']]
    return '\n'.join(lines)+'\n'


def prepare(output,project_repo,consumer_repo):
    if output.exists():raise ValueError('use a new output; preserve prior evidence')
    for checker,name in [('check-subscription-ticket.py','subscription-ticket-v1'),('check-subscription-probe.py','subscription-probe-v1')]:
        failures=load(checker).check(ROOT/BASE/name)
        if failures:raise ValueError('\n'.join(failures))
    output.mkdir(parents=True)
    adopted=json.loads((ROOT/INPUTS['host']).read_text())['source_adoptions']['coding_agent']['source_commit']
    for owner,repo,paths,rev in [('project',project_repo,PROJECT,REVISION),('consumer',consumer_repo,ADOPTED,adopted)]:
        capture=SourceCapture(repo,output/'source'/owner,rev)
        for path in paths:capture.source(path)
        dump(output/'source'/owner/'git-proof.json',capture.proof())
    artifacts=generate(inputs(output))
    for name,value in artifacts.items():dump(output/name,value)
    (output/'REVIEW-TABLE.txt').write_text(table(artifacts))
    (output/'.gitattributes').write_bytes(ATTRIBUTES)
    (output/'generator.py.txt').write_bytes(Path(__file__).read_bytes())
    dump(output/'formal-state.json',load('prepare-source-review-supplement.py').formal_state(ROOT))
    dump(output/'manifest.json',{'schema_version':1,'kind':'ticket_gap_preparation_only',
         'input_bindings':{str(p):sha((ROOT/p).read_bytes()) for p in bindings()},
         'generator_sha256':sha(Path(__file__).read_bytes()),'counts':artifacts['gap-summary.json']['counts'],
         'artifacts':{p.relative_to(output).as_posix():sha(p.read_bytes()) for p in sorted(output.rglob('*')) if p.is_file()}})
    print(json.dumps(artifacts['gap-summary.json']['counts']),flush=True)


if __name__=='__main__':
    parser=argparse.ArgumentParser(description=__doc__)
    for name in ['output','project-repo','consumer-repo']:parser.add_argument('--'+name,type=Path,required=True)
    args=parser.parse_args();prepare(args.output.resolve(),args.project_repo.resolve(),args.consumer_repo.resolve())
