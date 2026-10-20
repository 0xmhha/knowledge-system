"""WI2 compiler scope witnesses and same-input MCP observations, no gold rewriting."""
from collections import Counter
import gzip,json,subprocess
from pathlib import Path
import wi3_reason_fix as previous
import wi2_dev_gold as source
ROOT,BASE,f=previous.ROOT,previous.BASE,previous.f
PACKET=BASE/'followups/wi2-contract-audit-v1'
REVISION='64c462347ad32c435c9d5970b22ca29dd9afb892'
KIND='WI2_source_contract_compiler_MCP_audit_not_formal_execution'
OLD=BASE/'followups/wi2-consumer-matching-v1'
IDS=['RULE-1-05','RULE-1-07','RULE-1-15','RULE-2-06','RULE-2-07','RULE-2-16']
TOOLS=['scripts/'+n for n in ('wi2_contract_audit.py','prepare-wi2-contract-audit.py','check-wi2-contract-audit.py','test_wi2_contract_audit.py','prepare-wi2-scope-witnesses.py','verify-wi2-contract-runtime.py','wi2_contract_scope.go.txt','wi3_reason_fix.py','prepare-wi3-reason-fix.py','wi2_dev_gold.py','independent_acceptance.py','check-operational-acceptance.py','test_operational_acceptance.py')]
SOURCE_INPUTS=[Path(p)for p in ('pkg/graph/valueflow/rules.go','docs/graph/GO-RULE-CANDIDATES.md','internal/graph/detect/golang.go')]


def packed(p):return json.loads(gzip.decompress(p.read_bytes()))
def exact_revision(root=ROOT):return subprocess.check_output(['git','rev-parse',REVISION],cwd=root,text=True).strip()


def derive(root=ROOT,packet=None):
    p=packet or root/PACKET;original={r['id']:r for r in source.source_inputs(root)[1]};catalog=f.read(p/'compiler/witness-catalog.json');output=f.read(p/'compiler/oracle-output.json');by={r['ID']:r for r in output};scope=[]
    if len(catalog)!=8 or len(output)!=18:raise ValueError('compiler witness/build profile denominator differs')
    for w in catalog:
        src=(p/'compiler/witnesses'/(w['id']+'.go.txt')).read_bytes();r=by[w['id']]
        if r['Errors'] or r['MatchErrors']:raise ValueError('supplemental witness not type-valid')
        expected_helpers=dict(sanitize='"constant"',clamp='7')if w['id'].endswith('-constant')else dict(sanitize='v',clamp='v')
        if r['HelperReturns']!=expected_helpers:raise ValueError('opaque helper counter-context changed')
        if f.sha(src)!=w['full_file_sha256'] or f.sha(src[w['fragment_start']:w['fragment_end']])!=original[w['original_case_id']]['function_sha256'] or (r['FunctionStart'],r['FunctionEnd'])!=(w['fragment_start'],w['fragment_end']):raise ValueError('original fragment/full-file compiler binding differs')
        expected='os/exec'if w['original_case_id'].startswith('RULE-1')else'strconv';calls=r['Calls']
        if len(calls)!=1:raise ValueError('selected compiler target population differs')
        c=calls[0];local=w['original_case_id']in('RULE-1-07','RULE-2-07')
        if local:
            if c['OwnerKind']!='*types.Var' or c['ImportPath'] or not c['ObjectPackage'].startswith('example.invalid/wi2audit/'):raise ValueError('shadowed target was promoted to stdlib')
        elif c['OwnerKind']!='*types.PkgName' or c['ImportPath']!=expected or c['ObjectPackage']!=expected:raise ValueError('stdlib alias target scope differs')
        scope.append(dict(case_id=w['original_case_id'],witness_id=w['id'],fragment_sha256=w['fragment_sha256'],full_file_sha256=w['full_file_sha256'],
            compiler_target='supplemental_local_method'if local else'supplemental_declared_stdlib_function',object_package=c['ObjectPackage'],source_span=dict(start=c['Start'],end=c['End']),
            scope='supplemental complete prefix only; original published prefix/retained function blob is not rewritten',independent_semantic_approval=False))
    for id in IDS:
        if not by['ORIGINAL-'+id]['Errors']:raise ValueError('original published prefix unexpectedly claimed type-valid')
    for id in ('RULE-1-05','RULE-2-06'):
        pair=[w for w in catalog if w['original_case_id']==id]
        if len(pair)!=2 or pair[0]['fragment_sha256']!=pair[1]['fragment_sha256'] or pair[0]['full_file_sha256']==pair[1]['full_file_sha256']:raise ValueError('opaque helper counter-context lost')
    manifest=packed(root/BASE/'followups/wi3-consumer-matching-v1/runtime/bindings/graph/manifest.json.gz');paths={r['path']for r in manifest['files']};missing=[]
    profiles={'build/ci.go':{'darwin_default':False,'linux_default':False,'darwin_none_tag':True,'darwin_gofuzz_tag':False},
        'build/update-license.go':{'darwin_default':False,'linux_default':False,'darwin_none_tag':True,'darwin_gofuzz_tag':False},
        'crypto/signify/signify_fuzz.go':{'darwin_default':False,'linux_default':False,'darwin_none_tag':False,'darwin_gofuzz_tag':True},
        'metrics/disk_linux.go':{'darwin_default':False,'linux_default':True,'darwin_none_tag':False,'darwin_gofuzz_tag':False}}
    oldledger=f.read(root/OLD/'REVIEW-LEDGER.json');inputs={i['Path']:i['Source'].encode() for i in source.source_inputs(root)[0]}
    for row in oldledger['consumer_symbol_unavailable']:
        fn=original[row['function_id']];path=fn['path'];w=by[path]
        raw=(p/'compiler/original-source'/ (path+'.txt')).read_bytes()
        if raw!=inputs[path] or path in paths or w['Matches']!=profiles[path] or w['MatchErrors']:raise ValueError('manifest/source/build-selection exclusion differs')
        missing.append(dict(function_id=fn['id'],case_ids=row['case_ids'],path=path,file_sha256=f.sha(raw),recorded_graph_manifest_contains_file=False,
            current_MatchFile_profiles=w['Matches'],historical_build_environment=None,
            interpretation='known manifest exclusion with current build constraint witness; original exact GOOS/GOARCH/tags not captured, exclusive historical cause unknown',quality_score=None))
    old={r['function_id']:r for name in ('controlled-results.json','project-results.json') for r in f.read(root/OLD/'runtime'/name)}
    actual=f.read(p/'runtime/responses.json');expected=IDS+['PROJECT-016','PROJECT-045'];runtime=[]
    if [r['function_id']for r in actual]!=expected:raise ValueError('planned8 actual query denominator/order differs')
    for r in actual:
        id=r['function_id']
        if r['arguments']!=old[id]['arguments']:raise ValueError('old query/pin/cap changed')
        result=r['raw_response'].get('result',{});value=result.get('structuredContent');analysis=(value or{}).get('analysis');rules=(analysis or{}).get('rules')
        if result.get('isError') or not analysis or not rules:raise ValueError('current rule contract cannot be checked from absent analysis')
        if rules['pack_id']!='go-local-review' or rules['pack_version']!='1.0.0' or rules['status']!='matched_bounded':raise ValueError('rule pack/status changed')
        candidates=rules['candidates'];expected_count=0 if id in ('RULE-1-15','RULE-2-16','PROJECT-016','PROJECT-045')else 1
        if len(candidates)!=expected_count:raise ValueError('recorded contract behavior differs')
        for c in candidates:
            if c['target_resolution']!='syntax_only_unverified' or c['sanitizer']!='not_proven' or c['certainty']!='lexical_may_reach':raise ValueError('unverified candidate promoted to semantic proof')
        runtime.append(dict(function_id=id,status=value['status'],rule_status=rules['status'],candidates=len(candidates),
            prior_status=old[id]['response']['status'],observation_kind='same_input_current_probe_not_replacement_or_new_sample',source_semantic_approved=False))
    receipt=f.read(p/'runtime/receipt.json');sessions=f.read(p/'runtime/sessions.json')
    if receipt['planned']!=8 or receipt['attempted']!=8 or receipt['retries']!=0 or receipt['current_scores']is not None or receipt['formal_completed']!=0 or receipt['paid_or_external_LLM_calls']!=0 or receipt['installed_agent_role_runs']!=0:raise ValueError('actual runtime scope/quality promoted')
    if receipt['binary_sha256']!=f.read(root/previous.PACKET/'native/binary.json')['sha256']:raise ValueError('binary differs from latest verified native source')
    if sessions!=[dict(scope='controlled',exit=0,requests=8),dict(scope='project',exit=0,requests=4)]:raise ValueError('actual sessions/EOF cleanup differ')
    for prefix in ('retained','controlled'):
        if packed(p/'runtime'/(prefix+'-before.json.gz'))!=packed(p/'runtime'/(prefix+'-after.json.gz')):raise ValueError('existing source/index changed')
    return dict(scope=scope,missing=missing,runtime=runtime,summary=dict(kind=KIND,review_cases=6,type_valid_supplemental_contexts=8,original_incomplete_prefix_typecheck_probes=6,opaque_same_fragment_context_pairs=2,
        original_manifest_excluded_functions=9,affected_original_project_inputs=sum(len(r['case_ids'])for r in missing),excluded_source_files=4,build_profile_observations=16,historical_build_environment=None,
        actual_same_input_rule_queries=8,actual_EOF_exit0=2,former_budget_limited_current_available=2,original_primary_failures_preserved=True,
        original_semantic_labels_and_denominators_preserved=True,new_independent_samples=0,source_only_semantic_review='AI_plus_compiler_supplement_not_human_approval',
        current_semantic_quality_scores=None,independent_approved=0,FINAL=0,formal_completed=0,formal_total=9,freeze=None,execution_allowed=False,paid_or_external_LLM_calls=0))


def contract():
    return dict(pack='Frozen go-local-review@1.0.0 syntax-only, unverified import target/sanitizer, lexical may-reach and documented rename/alias boundary. No automatic pack upgrade or matcher relaxation.',
        shadow='Local selector parameters explain documented false positives. Original negative semantic labels remain negative; literal contract conformance is separate from semantic precision.',
        alias='process.Command/conversion.Atoi are stdlib targets in declared supplemental full-file imports; original body-only matcher intentionally does not resolve renamed imports. Semantic misses remain recorded.',
        opaque='Identity and constant helper contexts share original F bytes with distinct full-file hashes; a body-only lexical witness cannot prove helper effect. Original unknowns remain unknown; no sanitizer/taint/safety verdict.',
        prefix='New type-valid supplemental prefix adds local Fake methods/helper bodies/import uses; original published incomplete prefix and graph blobs unchanged. Compiler target proof is confined to that explicit full-file context.',
        missing='9functions/17project inputs absent from recorded graph files in4 paths. Current MatchFile profiles show tag/platform selection; exact historical build environment unrecorded. Never drop original planned cases or call absence safe.',
        budget='Original2 final verification deadline results preserved. Current2 same-argument probes return partial analysis and matched_bounded empty rules; not a code fix, safety proof or independent sample.',
        native='Reuse latest committed1738native/5approved-input source binding and4107general/race tests; no maintained Go source changes or new full native run this stage.',
        scope='AI technical triage complete for these6differences/9availability/2budget observations; independent semantics, production profile completeness, installed role and new DEV/FINAL/F01-F08 remain open.',
        privacy=previous.contract()['privacy'],formal_scoring_allowed=False,independent_review=None)


def historical_errors(root=ROOT):
    errors=previous.historical_errors(root)
    for relative in (previous.PACKET,OLD):
        p=root/relative;m=f.read(p/'manifest.json')
        if (p/'manifest.json').read_bytes()!=subprocess.check_output(['git','show',exact_revision(root)+':'+str(relative/'manifest.json')],cwd=root):errors.append('historical manifest differs '+str(relative))
        for n,h in m['artifacts'].items():
            if f.sha((p/n).read_bytes())!=h:errors.append('historical artifact differs '+n)
        if relative==previous.PACKET:
            for t in m['tools']:
                if f.sha(subprocess.check_output(['git','show',exact_revision(root)+':'+t['path']],cwd=root))!=t['sha256']:errors.append('historical tool differs '+t['path'])
    return errors


def reference(root=ROOT):
    return dict(manifest=str(PACKET/'manifest.json'),sha256=f.sha((root/PACKET/'manifest.json').read_bytes()),status='WI2_compiler_and_actual_contract_triage_not_independent_semantic_acceptance',report=str(PACKET/'REPORT.txt'),worklist=str(PACKET/'WORKLIST.json'))


def current_indices(root=ROOT,protocol_override=None,ledger_override=None):
    paths=[Path('docs/spec-driven/OPERATIONAL-ACCEPTANCE-'+n+'.json')for n in('PROTOCOL','PROGRESS','DECISION-PROPOSAL')]+[BASE/'GAPS.json'];docs=[f.read(root/x)for x in paths]
    if protocol_override is not None:docs[0]=protocol_override
    if ledger_override is not None:docs[1]=ledger_override
    ref=reference(root);state=f.read(root/PACKET/'WORKLIST.json');current=dict(kind=KIND,manifest=ref['manifest'],manifest_sha256=ref['sha256']);errors=[]
    for d,path in zip(docs,paths):
        old=json.loads(subprocess.check_output(['git','show',exact_revision(root)+':'+str(path)],cwd=root))
        if d.get('wi2_contract_audit')!=ref or d.get('current_execution_state')!=state or d.get('current_readiness_preparation')!=current:errors.append('WI2 audit current reference differs')
        if d.get('reviewer_identity_policy')!=contract()['privacy']:errors.append('reviewer privacy policy differs')
        for key,value in old.items():
            if key=='current_readiness_preparation' or not isinstance(value,dict) or 'manifest'not in value:continue
            expected=dict(value)
            if key=='wi3_reason_fix':expected['historical_source_revision']=exact_revision(root)
            if d.get(key)!=expected:errors.append('previous preparation promoted '+key)
    if docs[0]['freeze']is not None or docs[0]['execution_allowed']is not False or any(docs[0]['roles'].values()):errors.append('compiler/technical audit cannot freeze or assign reviewers')
    link=docs[0]['adoption_acceptance_decision_proposal']
    if link['sha256']!=f.sha((root/paths[2]).read_bytes())or any(d.get('adoption_acceptance_decision_proposal')!=link for d in(docs[1],docs[3])):errors.append('proposal link differs')
    return errors


def validate(root=ROOT,packet=None):
    p=packet or root/PACKET;errors=[]
    try:
        m=f.read(p/'manifest.json');actual={x.relative_to(p).as_posix():f.sha(x.read_bytes())for x in p.rglob('*')if x.is_file()and x!=p/'manifest.json'}
        if m['kind']!=KIND or m['previous_source_revision']!=exact_revision(root)or [t['path']for t in m['tools']]!=TOOLS or actual!=m['artifacts']:errors.append('WI2 audit manifest scope/hash differs')
        for t in m['tools']+m['source_inputs']:
            if f.sha((root/t['path']).read_bytes())!=t['sha256']:errors.append('source/tool differs '+t['path'])
        if [t['path']for t in m['source_inputs']]!=[str(x)for x in SOURCE_INPUTS]:errors.append('source input population differs')
        errors.extend(historical_errors(root));errors.extend(previous.native_errors(root,root/previous.PACKET));data=derive(root,p)
        for name,key in [('SUMMARY.json','summary'),('compiler-scope-review.json','scope'),('index-scope-review.json','missing'),('actual-contract-review.json','runtime')]:
            if f.read(p/name)!=data[key]:errors.append('derived audit differs '+name)
        if f.read(p/'matching-contract.json')!=contract():errors.append('semantic/contract scope promoted')
        state=f.read(p/'WORKLIST.json')
        if state['formal_completed']!=0 or state['formal_total']!=9 or state['reviewer_assignment']!='미정' or state['independent_quality_or_ontology_acceptance']is not False or state['production_activation']is not False or state['publication']is not False:errors.append('worklist falsely completed')
    except (ValueError,TypeError,KeyError,OSError,IndexError)as exc:errors.append(str(exc))
    return errors
