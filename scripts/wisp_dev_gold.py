"""WI-SP finite overlay/source AI drafts; literal tags are not semantic gold."""
from collections import Counter
import gzip,json,subprocess
from pathlib import Path
import yaml
import wi6_dev_gold as previous
from independent_acceptance import reviewer_identity_policy
from source_packet import safe,verify

ROOT,BASE,f=previous.ROOT,previous.BASE,previous.f
PACKET=BASE/'followups/wisp-dev-gold-v1'
REVISION='fc4186b9997caa8aa432cc2dab6fb3fbe6ff703f'
KIND='WISP_bounded_overlay_AI_DEV_not_formal_execution'
GAPS=BASE/'policy-security-gap-v1';SOURCE=GAPS/'source/project'
AUTHOR=Path('scripts/fixtures/wisp-dev-authoring.json')
POLICY=Path('projects/stablenet/evaluation/policy-0bf2f4d1.yaml')
SOURCE_INPUTS=[POLICY]+[Path(p) for p in ('pkg/graph/annotations/query.go','pkg/graph/policy/policy.go',
    'pkg/graph/security/security.go','pkg/graph/types/enums.go','internal/system/mcp/annotations.go','internal/system/mcp/acl_test.go')]
TOOLS=['scripts/'+s for s in ('wisp_dev_gold.py','prepare-wisp-dev-gold.py','check-wisp-dev-gold.py','test_wisp_dev_gold.py',
    'fixtures/wisp-dev-authoring.json','wi6_dev_gold.py','prepare-wi4-dev-gold.py','source_packet.py','independent_acceptance.py',
    'test_independent_acceptance.py','check-operational-acceptance.py','test_operational_acceptance.py')]
UNKNOWN_PROVENANCE=('source_sha256','source_line','version','reviewer','reviewed_at','approval')


def known(value):return dict(status='retained_operator_value',value=value)
def unknown():return dict(status='unknown',value='')


def policy_entries(root=ROOT):
    raw=(root/POLICY).read_bytes();text=raw.decode();data=yaml.safe_load(text)
    node=yaml.compose(text);rows=next(v for k,v in node.value if k.value=='policies').value
    if len(rows)!=len(data['policies']) or len(rows)!=42:raise ValueError('policy population differs')
    entries=[]
    for row,mark in zip(data['policies'],rows):
        a=len(text[:mark.start_mark.index].encode());b=len(text[:mark.end_mark.index].encode())
        entries.append(dict(row,kind='policy',annotation_id='policy:'+row['id'],
            authoring_source=dict(path=str(POLICY),sha256=f.sha(raw),start_byte=a,end_byte_exclusive=b,
                span_sha256=f.sha(raw[a:b]),start_line=mark.start_mark.line+1,end_line=mark.end_mark.line+1),
            semantic_approval=None))
    if len({r['id'] for r in entries})!=len(entries):raise ValueError('duplicate policy id')
    return entries


def expected_annotation(entry):
    """Modern typed payload only; source row is authoring metadata, never runtime trust."""
    return dict(id=entry['annotation_id'],kind='Policy' if entry['kind']=='policy' else 'SecurityPattern',
        name=entry['name'],category=entry.get('category',''),explanation=known(entry.get('description','')),
        modification_hint=known(entry.get('remediation','')),severity=known(entry.get('severity','')),
        provenance=dict(origin='operator_curated_metadata',identity_trust='not_authenticated',
                        **{k:unknown() for k in UNKNOWN_PROVENANCE}),
        related_symbol_binding='must_equal_actual_exact_symbol_node_id',
        source_path_binding='must_equal_declared_actual_overlay_ingest_path',
        automatic_rule_candidate=False,vulnerability_or_compliance_approved=False)


def derive(root=ROOT):
    proof=verify(root/SOURCE,f.SOURCE_COMMIT);original=f.read(root/GAPS/'public-inputs.json')['cases']
    old=f.read(root/GAPS/'private-symbol-gaps.json')['cases'];notes=f.read(root/AUTHOR)
    if len(original)!=12 or [r['id'] for r in original]!=[r['id'] for r in notes['symbols']]:raise ValueError('symbol population/order differs')
    policies=policy_entries(root);security=[]
    for r in old:
        s=r['existing_security_overlay_ai_proposal']
        if s is not None:security.append(dict(s,kind='security',annotation_id='security:'+s['id'],
            authoring_source=dict(path=str(GAPS/'private-symbol-gaps.json'),sha256=f.sha((root/GAPS/'private-symbol-gaps.json').read_bytes()),
                symbol_row=r['id'],status='previous_AI_proposal_not_applied_or_approved'),semantic_approval=None))
    if len(security)!=4:raise ValueError('security proposal population differs')
    entries=policies+security;gold=[];public=[]
    for case,note in zip(original,notes['symbols']):
        s=case['source'];raw=safe(root/SOURCE,proof['files'][s['path']]['artifact']).read_bytes();body=raw[s['start_byte']:s['end_byte_exclusive']]
        if f.sha(raw)!=s['file_sha256'] or f.sha(body)!=s['span_sha256']:raise ValueError('original source symbol differs')
        name=s['source_name_hint'];units=[];selected=[]
        for e in entries:
            targets=e.get('governs',[]) if e['kind']=='policy' else e['matches']
            applies=name in [v.strip() for v in targets]
            label='explicit_selected_relation' if applies else 'no_explicit_selected_relation'
            reason=('Exact governs/matches target equals this source-qualified symbol in the finite selected overlay.' if applies else
                    'The finite governs/matches targets do not include this exact qualified symbol; descriptions/substrings do not create edges. Semantic applicability remains unknown.')
            units.append(dict(annotation_id=e['annotation_id'],label=label,rationale=reason,semantic_applicability='unknown',
                overlay_state='existing_policy_source' if e['kind']=='policy' else 'AI_proposed_security_not_applied',
                source_ref=e['authoring_source']))
            if applies:selected.append(expected_annotation(e))
        expected_policies=[e['id'] for e in policies if e['annotation_id'] in [a['id'] for a in selected]]
        expected_security=[e['id'] for e in security if e['annotation_id'] in [a['id'] for a in selected]]
        if expected_policies!=note['policies'] or expected_security!=note['security']:raise ValueError('manual source review/selector classification differs')
        source=dict(s,commit=f.SOURCE_COMMIT,project='go-stablenet',source_mode='committed',body=body.decode())
        sets={label:[u['annotation_id'] for u in units if u['label']==label] for label in ('explicit_selected_relation','no_explicit_selected_relation')}
        gold.append(dict(case_id=case['id'],work_item_id='WI-SP',split='DEV',formal_split=None,
            source=source,source_behavior_draft=note['behavior'],semantic_unknown=note['semantic_unknown'],
            universe=units,answer_sets=sets,
            empty_set_rationales={k:'No '+k+' in exactly42 policy+4 AI security proposal entries; not semantic applicability or whole-project safety.' for k,v in sets.items() if not v},
            policy_only_expected=[a for a in selected if a['kind']=='Policy'],proposed_combined_expected=selected,
            policy_only_relative_untagged=not expected_policies,proposed_combined_relative_untagged=not selected,
            current_retained_annotations=None,current_symbol_resolution=None,actual_manual_query=None,actual_automatic_query=None,
            overlay_ingest_receipt=None,blob_hash_adapter=None,independent_family=None,reviewer=None,review_receipt=None,
            independently_approved=False,safety_verdict='unknown',semantic_gold_complete=False,
            finite_selector_matrix_complete=True,current_scores=None,formal_scoring_allowed=False,
            exposure='previously_exposed_DEV_not_fresh_FINAL'))
        public.append(dict(case,split='DEV',formal_split=None,contains_gold=False,approved=False,exposure='already_exposed_DEV',source_excerpt='source/'+case['id']+'.go.txt'))
    return dict(public=public,gold=gold,entries=entries,controls=notes['controls'],
        summary=dict(kind=KIND,DEV_symbols=12,source_files=5,policy_entries=42,AI_security_proposals=4,
            selector_units=552,selector_labels=dict(Counter(u['label'] for r in gold for u in r['universe'])),
            expected_policy_relations=5,proposed_security_relations=4,policy_only_untagged_symbols=sum(r['policy_only_relative_untagged'] for r in gold),
            proposed_combined_untagged_symbols=sum(r['proposed_combined_relative_untagged'] for r in gold),
            refusal_control_proposals=len(notes['controls']),actual_symbol_queries=0,actual_automatic_queries=0,proposed_security_applied=False,
            actual_scores=None,independent_approved=0,independent_families=0,FINAL=0,D04='미정',
            formal_completed=0,formal_total=9,freeze=None,execution_allowed=False,new_model_calls=0),
        parser=dict(name='PyYAML',version=yaml.__version__,purpose='finite authoring parse not independent semantic or runtime oracle'))


def legacy_expected(description):
    return dict(explanation=known(description),modification_hint=unknown(),severity=unknown(),
                source_row_is_not_retained=True,identity_trust='not_authenticated')


def contract():
    return dict(status='AI_DEV_not_frozen',scope='12 exact source symbols x42 literal policy entries+4 unapplied AI security proposals; explicit relation vocabulary only',
        semantics='manual relation is not vulnerability, compliance or semantic applicability gold; all552 semantic applicability units unknown',
        identity='actual routed store/session project+dataset binding, canonical/node identity, committed source/span/blob and graph snapshot; bare annotation result lacks explicit project/dataset fields',
        wire='cks.context.policy_security, exactly one node_id/canonical_id; expected_commit/source_blob_sha256/snapshot pins; optional automatic cks.context.rule_candidates separate',
        source='original full-file SHA differs from retained function blob SHA; actual AST/envelope adapter required before setting expected_source_sha256',
        fields='modern typed operator description/hint/severity retain known empty string; legacy merged description has unknown separate hint/severity',
        provenance='authoring YAML bytes/row/hash/source proof kept outside runtime; actual source YAML hash/row/version/reviewer/time/approval stay unknown; graph metadata SHA is not authentication',
        overlays='policy-only predicted5 relations; proposed combined predicted9 relations conditional on exact isolated ingest; no current graph/security absence inferred from old policy-only edge capture',
        untagged='8 policy-only /4 proposed-combined relative empty symbols; no_annotations requires known complete symbol/page, not not_found/unavailable/unsupported/partial; safety unknown',
        refusal='16 typed decoder/pin/private/spoof/drift/ACL/empty/automatic/foreign-store proposals; branch-specific status preserved; planned denominator includes errors and unresolved coverage',
        ACL='socket peer ACL before tool, forwarded header spoof denied; not per-symbol/project authorization or reviewer authentication',
        defaults='32 annotation cap/1000ms cooperative timeout; explicit1..128 and1..5000; page truncation partial, synchronous hard memory/preemptive deadline not guaranteed',
        recheck='snapshot_changed clears symbol/tags/more on final manifest/blob/node/annotation/page/more drift; not transaction or ABA proof',
        acceptance='independent12 symbols with4 policy/4 security/4 relative-untagged and reviewed controls required;100percent match/refusal thresholds remain proposals, no score now',
        independent_review=None,formal_scoring_allowed=False)


def review_ledger(data):
    return dict(complete=False,independent_approved=0,actual_queries=0,
        semantic_unknowns=[dict(case_id=r['case_id'],rationale=r['semantic_unknown']) for r in data['gold']],
        missing=['independent_symbol_families_DEV_FINAL_and_annotation_semantics','reviewed_security_overlay_and_actual_isolated_ingest',
            'actual_source_blob_canonical_and_routed_project_dataset_session_binding','actual_manual_and_optin_automatic_paths',
            'actual_stale_private_spoof_and_deployed_HTTP_ACL_receipts','reviewed_typed_legacy_empty_and_cap_denominators',
            'installed_consumer_role_and_handoff','D04_and_F01_F08'],
        provenance_unknown_fields=list(UNKNOWN_PROVENANCE),wire_project_dataset_fields_present=False)


def historical_errors(root=ROOT):
    errors=previous.historical_errors(root)
    for relative in (previous.PACKET,GAPS):
        p=root/relative;m=f.read(p/'manifest.json')
        if (p/'manifest.json').read_bytes()!=subprocess.check_output(['git','show',REVISION+':'+str(relative/'manifest.json')],cwd=root):errors.append('historical manifest differs '+str(relative))
        for name,h in m['artifacts'].items():
            if f.sha(safe(p,name).read_bytes())!=h:errors.append('historical artifact differs '+str(relative/name))
        for t in m.get('tools',[]):
            if f.sha(subprocess.check_output(['git','show',REVISION+':'+t['path']],cwd=root))!=t['sha256']:errors.append('historical tool differs '+t['path'])
    return errors


def reference(root=ROOT):
    return dict(manifest=str(PACKET/'manifest.json'),sha256=f.sha((root/PACKET/'manifest.json').read_bytes()),
        status='WISP_DEV12_finite_AI_overlay_contract_not_independent_security_gold',report=str(PACKET/'REPORT.txt'),worklist=str(PACKET/'WORKLIST.json'))


def current_indices(root=ROOT,protocol_override=None,ledger_override=None):
    paths=[Path('docs/spec-driven/OPERATIONAL-ACCEPTANCE-'+n+'.json') for n in ('PROTOCOL','PROGRESS','DECISION-PROPOSAL')]+[BASE/'GAPS.json'];docs=[f.read(root/p) for p in paths]
    if protocol_override is not None:docs[0]=protocol_override
    if ledger_override is not None:docs[1]=ledger_override
    ref=reference(root);state=f.read(root/PACKET/'WORKLIST.json');current=dict(kind=KIND,manifest=ref['manifest'],manifest_sha256=ref['sha256']);errors=[]
    old=json.loads(subprocess.check_output(['git','show',REVISION+':'+str(paths[0])],cwd=root))
    for d in docs:
        if d.get('reviewer_identity_policy')!=reviewer_identity_policy():errors.append('pseudonymous reviewer privacy policy differs')
        if d.get('wisp_dev_gold_preparation')!=ref or d.get('current_execution_state')!=state or d.get('current_readiness_preparation')!=current:errors.append('WI-SP current reference differs')
        for key in ('wi6_dev_gold_preparation','ticket_dev_gold_preparation')+previous.previous.PREPARATIONS:
            expected=dict(old[key])
            if key=='wi6_dev_gold_preparation':expected['historical_source_revision']=REVISION
            if d.get(key)!=expected:errors.append('previous preparation promoted '+key)
    if docs[0]['freeze']is not None or docs[0]['execution_allowed']is not False or any(docs[0]['roles'].values()):errors.append('AI cannot freeze or assign independent reviewers')
    link=docs[0]['adoption_acceptance_decision_proposal']
    if link['sha256']!=f.sha((root/paths[2]).read_bytes()) or any(d.get('adoption_acceptance_decision_proposal')!=link for d in (docs[1],docs[3])):errors.append('proposal link differs')
    return errors


def validate(root=ROOT,packet=None):
    p=packet or root/PACKET;errors=[]
    try:
        m=f.read(p/'manifest.json');actual={x.relative_to(p).as_posix():f.sha(x.read_bytes()) for x in p.rglob('*') if x.is_file() and x!=p/'manifest.json'}
        if m['kind']!=KIND or m['previous_source_revision']!=REVISION or [t['path'] for t in m['tools']]!=TOOLS or actual!=m['artifacts']:errors.append('WI-SP manifest scope/hash differs')
        for t in m['tools']+m['source_inputs']:
            if f.sha((root/t['path']).read_bytes())!=t['sha256']:errors.append('source/tool binding differs '+t['path'])
        if [t['path'] for t in m['source_inputs']]!=[str(x) for x in SOURCE_INPUTS]:errors.append('source input population differs')
        errors.extend(historical_errors(root));data=derive(root)
        for name,key in [('public/DEV-inputs.json','public'),('private/overlay-catalog.json','entries'),('private/controls.json','controls'),('SUMMARY.json','summary'),('parser-scope.json','parser')]:
            if f.read(p/name)!=data[key]:errors.append('WI-SP derived draft differs '+name)
        if json.loads(gzip.decompress((p/'private/AI-gold.json.gz').read_bytes()))!=data['gold']:errors.append('WI-SP bounded gold differs')
        for public,gold in zip(data['public'],data['gold']):
            if safe(p/'public',public['source_excerpt']).read_bytes()!=gold['source']['body'].encode():errors.append('public source excerpt differs')
        if f.read(p/'matching-contract.json')!=contract():errors.append('WI-SP contract changed or promoted')
        if f.read(p/'REVIEW-LEDGER.json')!=review_ledger(data):errors.append('WI-SP debt erased or promoted')
        if f.read(p/'reviewer-identity-policy.json')!=reviewer_identity_policy():errors.append('reviewer identity policy requests personal names or invents approval')
        state=f.read(p/'WORKLIST.json')
        if state['formal_completed']!=0 or state['formal_total']!=9 or state['reviewer_assignment']!='미정' or state['independent_quality_or_ontology_acceptance']is not False or state['production_activation']is not False or state['publication']is not False:errors.append('worklist falsely completed')
        from independent_acceptance import native_reuse_record
        if native_reuse_record(root)['changed_bound_files']:errors.append('native source changed')
    except (ValueError,TypeError,KeyError,OSError,IndexError,StopIteration,yaml.YAMLError) as exc:errors.append(str(exc))
    return errors
