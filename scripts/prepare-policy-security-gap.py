#!/usr/bin/env python3
"""Audit WI-SP symbol/gold, provenance, peer/private boundaries and consumer gaps."""
import argparse
from collections import Counter
import importlib.util
import itertools
import json
from pathlib import Path
import re
import shutil

from source_packet import SourceCapture, safe, sha, verify

ROOT=Path(__file__).resolve().parents[1]
BASE=Path('docs/spec-driven/operational-preparation/source-review-supplement')
PRIOR=Path('docs/spec-driven/operational-preparation/policy-security-drafts')
HOST=Path('system/eval/operational-preparation-2026-10-15/isolated-host')
OLD=Path('system/eval/upper-work-items-implementation-2026-10-14/wi-sp')
REGEN=Path('system/eval/knowledge-roadmap-2026-10-16/policy-source-regeneration')
HISTORY_REVISION='17970c869be85f4242bd61eea0db7c5f01d2a4d8'
HISTORY_DOCS=['docs/REMAINING-WORK.md','docs/spec-driven/CURRENT-HANDOFF.md']
HISTORY_FILES=['history-validation.json','recorded-doc-tests.txt','current-doc-tests.txt']
SOURCE=[Path(p) for p in ['pkg/graph/annotations/query.go','pkg/graph/annotations/query_test.go',
 'pkg/graph/policy/policy.go','pkg/graph/security/security.go',
 'internal/system/mcp/annotations.go','internal/system/mcp/annotations_test.go','internal/system/mcp/acl_test.go',
 'internal/system/mcp/server.go','internal/system/ckgclient/annotations.go',
 'internal/system/mcp/testdata/agent-mcp.schema.json']]
NATIVE_SOURCE=sorted({p.relative_to(ROOT) for directory in ['pkg/graph/annotations','internal/system/mcp'] for p in (ROOT/directory).glob('*.go')})
CONSUMER=['plugin/agents/analyzer.md','plugin/contracts/cks-tools.schema.json']
ATTRIBUTES=b'source/*/objects/*.commit binary\nsource/*/objects/*.tree binary\n'
INPUTS={'prior_manifest':PRIOR/'manifest.json','public':PRIOR/'public/inputs.json','private':PRIOR/'private/review-drafts.json',
 'proposals':Path('scripts/fixtures/policy-security-gap-proposals.json'),
 'host':HOST/'manifest.json','consumer_inventory':HOST/'coding_agent-source-files.json',
 'regen_manifest':REGEN/'manifest.json','regen_summary':REGEN/'summary.json',
 'regen_binding':REGEN/'source-binding.json',
 'regen_generator':REGEN/'generator-status.json','regen_edges':REGEN/'persisted-policy-edges.json',
 'old_manifest':OLD/'manifest.json','old_summary':OLD/'execution/summary.json',
 'old_requests':OLD/'execution/cks/mcp.stdin.jsonl','old_responses':OLD/'execution/cks/mcp.stdout.jsonl',
 'old_access_log':OLD/'checks/ks-sp-query-access-final.log'}
for name in ['annotations','canonical','empty','missing','cap','after-deletion','rules-separate',
             'stale-expected_commit','stale-expected_source_sha256','stale-expected_snapshot_sha256']:
    INPUTS['old_'+name]=OLD/('execution/'+name+'.json')
NATIVE_TOP={'pkg/graph/annotations':['TestTypedRoundTripUnknownProvenanceAndEmptySafety',
 'TestCapsPinsUnsupportedPrivateAndCancellation','TestFinalPageAndMoreDriftClearAllData','TestStrictDecoder',
 'TestFinalManifestBlobSeedAndTagDriftClearAllData','TestPrivateAnnotationAndWrongRelationTargetFailClosed',
 'TestPageAndRecheckDeadlineClearResults'],
 'internal/system/mcp':['TestPolicySecurityPublicContractLegacyAndStrictRuntime',
 'TestPolicySecurityHTTPACLRejectsBeforeToolAndIgnoresForwardedIP','TestClientACL','TestClientACL_InvalidCIDR','TestClientACL_NilIP']}
NATIVE_FILES=['native-validation.json','normal.jsonl','normal.stderr.txt','race.jsonl','race.stderr.txt']
OBLIGATIONS=['independent_symbols_exposure_family_and_split','policy_security_untagged_sufficiency',
 'independent_exact_annotation_field_gold','source_canonical_snapshot_and_relation_matching',
 'manual_automatic_and_legacy_typed_provenance','invalid_stale_private_and_empty_control_mapping',
 'peer_acl_forwarded_headers_and_deployed_topology','installed_consumer_exposure_and_handoff',
 'baseline_denominators_caps_and_refusal_scoring','human_review_and_final_acceptance']


def load(name):
    spec=importlib.util.spec_from_file_location(name.replace('-','_'),ROOT/'scripts'/name)
    m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m);return m


helpers=load('prepare-semantic-gap.py')
dump,unique,span=helpers.dump,helpers.unique,helpers.span


def bindings():
    return sorted(set(INPUTS.values())|set(SOURCE)|set(NATIVE_SOURCE)|
      {Path('go.mod'),Path('go.sum'),Path('scripts/prepare-semantic-gap.py'),Path('scripts/source_packet.py'),
       Path('scripts/verify-policy-security-history.py'),Path('scripts/check-source-policy-regeneration.py'),
       Path('scripts/test_source_policy_regeneration.py')})


def inputs(packet):
    d={}
    for k,p in INPUTS.items():
        raw=(ROOT/p).read_text()
        d[k]=[json.loads(s) for s in raw.splitlines() if s.strip()] if p.suffix=='.jsonl' else raw if p.suffix=='.log' else json.loads(raw)
    for row in d['prior_manifest']['artifacts']:
        if sha(safe(ROOT/PRIOR,row['path']).read_bytes())!=row['sha256']:raise ValueError('prior source artifact binding')
    old=d['old_manifest']['files_sha256'];regen={r['path']:r['sha256'] for r in d['regen_manifest']['artifacts']}
    for p in INPUTS.values():
        if p.is_relative_to(OLD) and p.name!='manifest.json':
            if old[p.relative_to(OLD).as_posix()]!=sha((ROOT/p).read_bytes()):raise ValueError('historical execution binding')
        if p.is_relative_to(REGEN) and p.name!='manifest.json':
            if regen[p.relative_to(REGEN).as_posix()]!=sha((ROOT/p).read_bytes()):raise ValueError('regeneration evidence binding')
    d['sources']={}
    expected_project={r['source']['path'] for r in d['public']['cases']}
    revisions={'project':d['prior_manifest']['source_commit'],
               'consumer':d['host']['source_adoptions']['coding_agent']['source_commit'],
               'history-docs':HISTORY_REVISION}
    inventory={r['path']:r['sha256'] for r in d['consumer_inventory']}
    for owner,revision in revisions.items():
        root=packet/'source'/owner;proof=verify(root,revision)
        expected=expected_project if owner=='project' else set(CONSUMER) if owner=='consumer' else set(HISTORY_DOCS)
        if set(proof['files'])!=expected:raise ValueError('source inventory')
        d['sources'][owner]={p:safe(root,r['artifact']).read_bytes() for p,r in proof['files'].items()}
        if owner=='consumer' and any(sha(raw)!=inventory[p] for p,raw in d['sources'][owner].items()):raise ValueError('adopted consumer source binding')
    d['current_source']={str(p):(ROOT/p).read_bytes() for p in SOURCE}
    d['native']=native_validation(packet)
    d['history']=history_validation(packet,d)
    protocol=json.loads((ROOT/'docs/spec-driven/OPERATIONAL-ACCEPTANCE-PROTOCOL.json').read_text())
    progress=json.loads((ROOT/'docs/spec-driven/OPERATIONAL-ACCEPTANCE-PROGRESS.json').read_text())
    d['criteria']={'protocol':next(r for r in protocol['input_groups'] if r['id']=='WI-SP'),
                   'progress':next(r for r in progress['items'] if r['id']=='WI-SP')}
    return d


def native_validation(packet):
    root=packet/'native';m=json.loads((root/'native-validation.json').read_text())
    if m['kind']!='owned_current_existing_annotation_and_http_handler_controls_not_deployed_host_or_independent_acceptance' or not m['go_version'].startswith('go version go1.26.9 '):raise ValueError('native execution scope/toolchain')
    if any(m[k] is not False for k in ['network_listener_started','new_project_code_executed']) or m['new_model_calls']!=0 or m['formal_runs']!=0:raise ValueError('native operational promotion')
    prefix='github.com/0xmhha/knowledge-system/'
    expected={(prefix+p,t) for p,names in NATIVE_TOP.items() for t in names}
    acl=(ROOT/'internal/system/mcp/acl_test.go').read_text()
    subcases=re.findall(r'"([^"]+)"\s*,\s*HTTPPolicy\{',acl)
    if len(subcases)!=12:raise ValueError('native ACL source case inventory')
    expected|={(prefix+'internal/system/mcp','TestClientACL/'+s.replace(' ','_')) for s in subcases}
    if [r['mode'] for r in m['runs']]!=['normal','race']:raise ValueError('native normal/race inventory')
    for record in m['runs']:
        mode=record['mode'];raw=(root/(mode+'.jsonl')).read_text();rows=[json.loads(s) for s in raw.splitlines() if s]
        passes=[(r['Package'],r['Test']) for r in rows if r['Action']=='pass' and r.get('Test')]
        if record['exit_code']!=0 or set(passes)!=expected or len(passes)!=len(expected) or record['top_level_tests']!=12 or record['all_test_pass_records']!=24:
            raise ValueError('native pass/skip/failure inventory')
        if record['tests']!=[{'package':p,'test':t} for p,t in passes] or any(r['Action'] in ['fail','skip'] for r in rows) or 'WARNING: DATA RACE' in raw or (root/(mode+'.stderr.txt')).read_bytes():raise ValueError('native raw/summary completion mismatch')
        packages=[r['Package'] for r in rows if r['Action']=='pass' and not r.get('Test')]
        if set(packages)!={prefix+p for p in NATIVE_TOP} or len(packages)!=2:raise ValueError('native package completion')
    return m


def history_validation(packet,d):
    root=packet/'history';r=json.loads((root/'history-validation.json').read_text())
    if r['kind']!='historical_packet_checked_with_recorded_document_context_not_current_document_guard_pass' or r['current_context_guard_passed'] is not False:
        raise ValueError('historical current-doc mismatch hidden')
    if r['current_context_errors']!=['invalid source regeneration evidence: source/input bytes differ'] or r['recorded_document_revision']!=HISTORY_REVISION:
        raise ValueError('historical context identity')
    if any(r[k] is not False for k in ['guard_or_test_source_modified','workspace_docs_or_prior_packet_modified','formal_acceptance_granted']) or r['only_guard_root_argument_changed'] is not True:
        raise ValueError('historical validation changed evidence or scope')
    expected={**d['regen_binding']['source_sha256'],**d['regen_binding']['approved_input_sha256'],**d['regen_manifest']['source_sha256']}
    rows=unique(r['mismatches'],'path')
    if set(rows)!=set(HISTORY_DOCS):raise ValueError('historical document mismatch inventory')
    for name,row in rows.items():
        if row['expected_sha256']!=expected[name] or sha(d['sources']['history-docs'][name])!=expected[name] or row['current_sha256']!=sha((ROOT/name).read_bytes()) or row['introduced_by_this_task'] is not False:
            raise ValueError('historical/current document binding')
        if row['expected_sha256']==row['current_sha256']:raise ValueError('historical mismatch scope erased')
    recorded=(root/'recorded-doc-tests.txt').read_text();current=(root/'current-doc-tests.txt').read_text()
    if r['recorded_context_tests']!=14 or r['recorded_context_failures']!=0 or r['recorded_context_errors']!=0 or 'Ran 14 tests' not in recorded or '\nOK\n' not in recorded or 'FAILED' in recorded:
        raise ValueError('historical recorded-doc tests not complete')
    if 'Ran 14 tests' not in current or 'FAILED (failures=1)' not in current or 'source/input bytes differ' not in current:
        raise ValueError('historical current-doc failure not preserved')
    return r


def generate(d):
    public=unique(d['public']['cases'],'id');private=unique(d['private']['cases'],'id')
    if len(public)!=12 or set(public)!=set(private) or d['private']['approved'] is not False or d['private']['gold_records']!=0:raise ValueError('twelve unapproved inputs')
    cases=[]
    for key,p in public.items():
        r=private[key];src=p['source'];raw=d['sources']['project'][src['path']]
        if sha(raw)!=src['file_sha256'] or sha(raw[src['start_byte']:src['end_byte_exclusive']])!=src['span_sha256'] or raw[src['start_byte']:src['end_byte_exclusive']]!=(ROOT/PRIOR/'public'/p['source_excerpt']).read_bytes():raise ValueError('symbol source span')
        if r['gold'] is not False or r['approved'] is not False or r['split'] is not None or r['safety_verdict']!='unknown':raise ValueError('independent gold promotion')
        edges=[row for row in d['regen_edges'] if row['qualified_name']==src['source_name_hint']]
        if sorted(row['dst'].removeprefix('policy:') for row in edges)!=sorted(r['expected_policy_ids_from_exact_overlay_spelling']):raise ValueError('saved selected policy relation comparison')
        cases.append({'id':key,'source':src,'group_proposal':r['group_proposal'],
          'existing_exact_policy_spelling_proposal':r['expected_policy_ids_from_exact_overlay_spelling'],
          'existing_security_overlay_ai_proposal':r['security_entry_proposal'],
          'saved_source_regeneration_policy_edges':edges,'saved_edge_scope':'policy_only_not_all_security_tags_or_current_symbol_response',
          'independent_annotation_gold':None,'source_yaml_and_review_receipt':None,'independent_symbol_family':None,
          'canonical_symbol_resolution_receipt':None,'current_symbol_mcp_receipt':None,
          'prior_literal_filter_only':r['prior_literal_overlap'] is False,
          'prior_source_pool_filter_only':r['prior_source_pool_overlap'] is False,
          'semantic_independence':'not_reviewed','gold':False,'approved':False,'reviewer':None,'split':None,'safety_verdict':'unknown'})
    if Counter(r['group_proposal'] for r in cases)!={'existing_policy':4,'security_proposal':4,'no_direct_overlay_reference':4}:raise ValueError('proposal groups')
    pairs=[{'a':a['id'],'b':b['id'],'same_file':a['source']['path']==b['source']['path'],
      'shared_policy_ids':sorted(set(a['existing_exact_policy_spelling_proposal'])&set(b['existing_exact_policy_spelling_proposal'])),
      'semantic_independence':'not_reviewed'} for a,b in itertools.combinations(cases,2)]
    contracts=[]
    for r in d['proposals']['contracts']:
        source=d['sources']['consumer'] if r['scope']=='adopted_consumer' else d['current_source']
        contracts.append({**r,'anchor':span(source[r['path']],*r['lines']),'approved':False})
    unique(contracts,'id')
    front=d['sources']['consumer']['plugin/agents/analyzer.md'].decode().split('---',2)[1]
    tools=[s.strip()[2:] for s in front.split('tools:\n',1)[1].split('\nskills:',1)[0].splitlines() if s.strip().startswith('- ')]
    consumer=json.loads(d['sources']['consumer']['plugin/contracts/cks-tools.schema.json'])['providers']['cks']['tools']['cks_context_policy_security']
    metadata=json.loads(d['current_source']['internal/system/mcp/testdata/agent-mcp.schema.json'])['tools']['cks.context.policy_security']
    alias='mcp__plugin_coding-agent_cks__cks_context_policy_security'
    exposure={'wire_name':consumer['wire_name'],'metadata_input_parity':consumer['input']==metadata['input'],
      'explicit_analyzer_alias':alias,'explicit_analyzer_tool_present':alias in tools,
      'effective_host_permissions':None,'runtime_denial_proven':False,'installed_role_receipt':None}
    if exposure['explicit_analyzer_tool_present'] or not exposure['metadata_input_parity']:raise ValueError('consumer exposure scope changed')
    old=d['old_annotations'];unknown=['source_sha256','source_line','version','reviewer','reviewed_at','approval']
    if old['status']!='annotations_found' or len(old['annotations'])!=3 or old['safety_verdict']!='unknown':raise ValueError('historical positive scope')
    for a in old['annotations']:
        if a['provenance']['identity_trust']!='not_authenticated' or any(a['provenance'][k]!={'status':'unknown','value':''} for k in unknown):raise ValueError('historical provenance promotion')
    if d['old_empty']['status']!='no_annotations' or d['old_missing']['status']!='not_found' or d['old_summary']['cks_requests']!=18:raise ValueError('historical negative/wire scope')
    if d['old_after-deletion']['annotations']!=old['annotations'] or d['old_canonical']['annotations']!=old['annotations']:raise ValueError('historical retained/canonical parity')
    requests=unique(d['old_requests'],'id');responses=unique(d['old_responses'],'id')
    if len(requests)!=18 or set(requests)!=set(responses):raise ValueError('historical request response inventory')
    wire=[{'id':key,'method':r['method'],'tool':r.get('params',{}).get('name'),
      'rpc_error':'error' in responses[key],'tool_error':responses[key].get('result',{}).get('isError',False)} for key,r in requests.items()]
    if 'PASS: TestPolicySecurityHTTPACLRejectsBeforeToolAndIgnoresForwardedIP' not in d['old_access_log']:raise ValueError('historical HTTP handler success')
    regen=d['regen_summary'];gen=d['regen_generator']
    if regen['policy_nodes']!=42 or regen['policy_edges']!=146 or gen['regeneration_fix_complete'] is not True or regen['production_activated'] is not False or regen['ontology_effect_evaluated'] is not False:raise ValueError('regeneration completion/scope')
    counts={'source_symbol_proposals':12,'policy_group_proposals':4,'security_group_proposals':4,'relative_untagged_group_proposals':4,
      'project_source_files':len(d['sources']['project']),'pairwise_reviews':66,'same_file_pairs':sum(r['same_file'] for r in pairs),
      'saved_selected_policy_edges':sum(len(r['saved_source_regeneration_policy_edges']) for r in cases),
      'approved_independent_symbols':0,'independent_annotation_gold_records':0,'current_symbol_mcp_receipts':0,
      'contract_anchors':len(contracts),'adopted_consumer_source_files':2,'missing_explicit_analyzer_tools':1,
      'native_top_level_tests_per_mode':12,'native_pass_records_including_subtests_per_mode':24,'native_modes':2,
      'historical_recorded_doc_context_tests':14,'preexisting_current_doc_binding_mismatches':2,
      'historical_fixture_cks_requests':18,'new_external_mcp_sessions':0,'new_network_listeners':0,'new_model_calls':0,'formal_runs':0}
    return {'public-inputs.json':d['public'],
      'private-symbol-gaps.json':{'kind':'source_and_saved_policy_relation_review_not_independent_annotation_gold','cases':cases,
        'obligations':{k:'not_satisfied' for k in OBLIGATIONS},'gold_questions':d['proposals']['gold_questions']},
      'pairwise-review.json':pairs,
      'boundary-and-consumer-review.json':{'kind':'source_and_owned_handler_controls_not_deployed_consumer_acceptance',
        'contracts':contracts,'boundary_questions':d['proposals']['boundary_questions'],'criteria_snapshot':d['criteria'],
        'consumer_exposure_gap':exposure,'peer_acl_is_reviewer_authentication':False,
        'peer_acl_is_per_symbol_or_project_authorization':False,'metadata_hash_authenticates_yaml_or_reviewer':False,
        'deployed_listener_and_proxy_receipt':None,'independent_control_gold':None,'expected_refusal_fraction':None},
      'historical-evidence-scope.json':{'kind':'prior_fixture_and_regenerated_policy_not_independent_twelve_symbol_acceptance',
        'fixture_summary':d['old_summary'],'fixture_annotations':old['annotations'],'fixture_wire_requests':wire,
        'empty_status':d['old_empty']['status'],'missing_status':d['old_missing']['status'],
        'typed_and_canonical_and_deleted_source_results_preserved':True,
        'historical_http_handler_test_passed':True,'fixture_is_deployed_http_listener':False,
        'source_regeneration_summary':regen,'source_regeneration_status':gen,
        'current_twelve_independent_symbol_acceptances':0,'current_twelve_actual_mcp_receipts':0},
      'native-check-summary.json':d['native'],
      'history-context-review.json':d['history'],
      'gap-summary.json':{'kind':'policy_security_symbol_provenance_gold_boundary_and_consumer_gaps_preparation_only',
        'counts':counts,'annotation_match_fraction':None,'permission_refusal_fraction':None,
        'policy_only_saved_edges_do_not_establish_security_absence':True,'protocol_or_criteria_changed':False}}


def table(artifacts):
    lines=['WI-SP 독립 symbol·manual/automatic 출처·gold·ACL/private/spoof·소비자 공백 — 비공개 AI 자료',
      '12제안=정책4/보안proposal4/직접policy참조없음4. 독립 승인/gold/현재12symbol MCP receipt0개.',
      '일반/race 각12개 기존 top-level 테스트+12개 ACL subtest 통과는 배포 host/독립 수용 증거가 아니다.']
    for r in artifacts['private-symbol-gaps.json']['cases']:
        lines+=['',r['id']+' '+r['source']['source_name_hint']+' '+r['group_proposal'],
          '저장 policy 관계만: '+json.dumps(r['saved_source_regeneration_policy_edges'],ensure_ascii=False)]
    for r in artifacts['boundary-and-consumer-review.json']['contracts']:
        lines+=['',r['id']+' '+r['path']+':'+str(r['lines'][0]),r['fact'],'미충족: '+r['gap']]
    lines+=['','정답/출처 질문:',*artifacts['private-symbol-gaps.json']['gold_questions'],
            '','경계/소비자 질문:',*artifacts['boundary-and-consumer-review.json']['boundary_questions']]
    return '\n'.join(lines)+'\n'


def prepare(output,project_repo,consumer_repo,native_evidence,history_evidence):
    if output.exists():raise ValueError('use a new output; preserve prior evidence')
    output.mkdir(parents=True);(output/'native').mkdir()
    for name in NATIVE_FILES:shutil.copyfile(native_evidence/name,output/'native'/name)
    (output/'history').mkdir()
    for name in HISTORY_FILES:shutil.copyfile(history_evidence/name,output/'history'/name)
    public=json.loads((ROOT/INPUTS['public']).read_text())['cases']
    source_commit=json.loads((ROOT/INPUTS['prior_manifest']).read_text())['source_commit']
    consumer_commit=json.loads((ROOT/INPUTS['host']).read_text())['source_adoptions']['coding_agent']['source_commit']
    for owner,repo,paths,revision in [('project',project_repo,sorted({r['source']['path'] for r in public}),source_commit),
                                     ('consumer',consumer_repo,CONSUMER,consumer_commit),
                                     ('history-docs',ROOT,HISTORY_DOCS,HISTORY_REVISION)]:
        capture=SourceCapture(repo,output/'source'/owner,revision)
        for path in paths:capture.source(path)
        dump(output/'source'/owner/'git-proof.json',capture.proof())
    artifacts=generate(inputs(output))
    for name,value in artifacts.items():dump(output/name,value)
    (output/'REVIEW-TABLE.txt').write_text(table(artifacts));(output/'.gitattributes').write_bytes(ATTRIBUTES)
    (output/'generator.py.txt').write_bytes(Path(__file__).read_bytes())
    dump(output/'formal-state.json',load('prepare-source-review-supplement.py').formal_state(ROOT))
    dump(output/'manifest.json',{'schema_version':1,'kind':'policy_security_gap_preparation_only',
      'input_bindings':{str(p):sha((ROOT/p).read_bytes()) for p in bindings()},
      'generator_sha256':sha(Path(__file__).read_bytes()),'counts':artifacts['gap-summary.json']['counts'],
      'artifacts':{p.relative_to(output).as_posix():sha(p.read_bytes()) for p in sorted(output.rglob('*')) if p.is_file()}})
    print(json.dumps(artifacts['gap-summary.json']['counts']),flush=True)


if __name__=='__main__':
    parser=argparse.ArgumentParser(description=__doc__)
    for name in ['output','project-repo','consumer-repo','native-evidence','history-evidence']:parser.add_argument('--'+name,type=Path,required=True)
    args=parser.parse_args();prepare(args.output.resolve(),args.project_repo.resolve(),args.consumer_repo.resolve(),args.native_evidence.resolve(),args.history_evidence.resolve())
