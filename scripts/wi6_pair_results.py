"""Replay actual DEV19 pairs, preserving partial/budget/unknown and initial failure."""
import argparse
import gzip
import json
import re
import shutil
import sqlite3
import subprocess
from collections import Counter
from pathlib import Path

import wi6_pair_contract as c
import wi6_version_readiness as old
from source_packet import oid

ROOT = old.ROOT
PACKET = old.BASE / 'wi6-paired-runtime-v1'
REVISION = '9158875e8c996b9914ec3fa963eed36065560384'
SCOPE_SHA = '232796d49838f4eae9169fa76ae4880c31d13a45fb987e28a1028fdc5483eb38'
CORRECTIVE_SHA = 'f9df91ff8615639c854cbe4d513fe93b24e462e108653b7147ed1c090325d21c'
TOOLS = ['scripts/'+s for s in ('wi6_pair_contract.py','run-wi6-pair-pilot.py','test_wi6_pair_contract.py',
         'wi6_pair_results.py','test_wi6_pair_results.py','wi6_version_readiness.py','wi6_dev_gold.py')]


def read(path):
    return json.loads(gzip.decompress(path.read_bytes()) if path.suffix=='.gz' else path.read_bytes())


def write(path, value):
    path.parent.mkdir(parents=True,exist_ok=True)
    path.write_text(json.dumps(value,ensure_ascii=False,indent=2)+'\n')


def compressed(path, data):
    path.parent.mkdir(parents=True,exist_ok=True);path.write_bytes(gzip.compress(data,mtime=0))


def derive(root=ROOT, packet=None):
    p=packet or root/PACKET;s=read(p/'SCOPE.json');scope=read(p/'CORRECTIVE-SCOPE.json')
    if c.sha((p/'SCOPE.json').read_bytes())!=SCOPE_SHA or c.sha((p/'CORRECTIVE-SCOPE.json').read_bytes())!=CORRECTIVE_SHA:
        raise ValueError('predeclared initial/corrective scope changed')
    for name,h in s['tools_sha256'].items():
        if c.sha((p/'initial-tools'/name).read_bytes())!=h:raise ValueError('initial runtime tools changed')
    if any(c.sha((root/name).read_bytes())!=h for name,h in scope['tools_sha256'].items()):raise ValueError('corrective runtime tool changed')
    if read(p/'actual/SCOPE.json.gz')!=scope:raise ValueError('actual corrective scope differs')
    data=c.gold.derive(root);inputs=[r for r in data['public'] if r['scope']=='production_source_candidate']
    if [r['case_id']for r in scope['cases']]!=[r['case_id']for r in inputs] or len(inputs)!=19:raise ValueError('DEV19 population changed')
    if any(r['source']!=i['source'] for r,i in zip(scope['cases'],inputs)):raise ValueError('original source identity changed')
    if oid('blob',(p/'module.go.mod.txt').read_bytes())!=scope['module_git_blob']:raise ValueError('source module blob differs')
    historical=old.snapshot(root,old.BASE/'ticket-rpc-lifecycle-results-v1',REVISION)
    flat=read(root/old.BASE/'ticket-rpc-lifecycle-results-v1/actual/candidates/ORIGINAL/base-flat-tree.json')
    git_blobs={r['path']:r['oid']for r in flat}
    if git_blobs['go.mod']!=scope['module_git_blob']:raise ValueError('module not bound to upstream Git tree')
    # Whole dataset inventories include original graph/vector/archive bytes;
    # mutable parent coordination in the owned root is outside the candidate.
    expected=scope['original_data_sha256']
    for phase in ('original-before','original-after','owned-before','owned-after'):
        if read(p/'actual'/(phase+'.json.gz'))!=expected:raise ValueError('original/owned data changed')
    for phase in ('original-before','original-after','owned-before','owned-after'):
        if read(p/'initial'/(phase+'.json.gz'))!=expected:raise ValueError('initial data changed')
    for directory,exit_code,requests in [('initial',1,1),('actual',0,78)]:
        session=read(p/directory/'session.json.gz')
        if session['exit_code']!=exit_code or session['requests']!=requests or not session['reaped'] or not session['reader_joined'] or session['term_sent'] or session['kill_sent'] or session['remaining_live_group_members']:
            raise ValueError('actual session termination differs')
    for directory,exit_code in [('initial-driver',1),('driver',0)]:
        r=read(p/directory/'receipt.json.gz')
        if r['exit_code']!=exit_code or not r['reaped'] or r['timeout'] or r['remaining_live_group_members']:
            raise ValueError('driver outcome/cleanup differs')
        for stream in ('stdout','stderr'):
            if c.sha(gzip.decompress((p/directory/(stream+'.gz')).read_bytes()))!=r[stream+'_sha256']:raise ValueError('driver raw output differs')
    receipt=read(p/'actual/receipt.json.gz')
    if any(receipt.get(k)!=v for k,v in dict(planned_cases=19,actual_pairs=19,tool_calls=76,automatic_retries=0,original_and_owned_data_preserved=True,binary_sha256=scope['binary_sha256'],warm_samples=0,formal_completed=0,independent_approved=0,quality_scores=None).items()):raise ValueError('pilot population falsely completed or promoted')
    if read(root/Path(scope['binary_proof']))['sha256']!=scope['binary_sha256']:raise ValueError('binary not source-bound')
    manifest=read(p/'proof/graph-manifest.json.gz')
    if c.sha(gzip.decompress((p/'proof/graph-manifest.json.gz').read_bytes()))!=expected['graph/manifest.json']:raise ValueError('actual graph manifest differs')
    identity=read(p/'proof/dataset-identity.json.gz')
    if c.sha(gzip.decompress((p/'proof/dataset-identity.json.gz').read_bytes()))!=expected['dataset-identity.json']or identity['dataset_id']!=manifest['dataset_id']or identity['source']['project_id']!=manifest['project_id']:raise ValueError('retained dataset identity differs')
    project,dataset=manifest['project_id'],manifest['dataset_id']
    if manifest['src_commit']!=scope['source_commit'] or manifest['source_mode']!='committed':raise ValueError('project source mode/commit differs')
    retained=read(p/'actual/retained-nodes.json.gz');proof=read(p/'proof/files.json')
    for node in retained.values():
        file=proof[node['path']];blob=gzip.decompress((p/file['artifact']).read_bytes())
        if c.sha(blob)!=node['file_sha256'] or oid('blob',blob)!=git_blobs[node['path']]:raise ValueError('returned file is not exact original Git blob')
        body=blob[node['start_byte']:node['end_byte_exclusive']]
        if c.sha(body)!=node['function_sha256'] or len(body)!=node['bytes']:raise ValueError('returned full function bytes differ')
    calls=read(p/'actual/calls.json.gz');pairs=read(p/'actual/pairs.json.gz');seeds=read(p/'actual/retained-seeds.json.gz')
    stdin=[json.loads(l)for l in gzip.decompress((p/'actual/stdin.jsonl.gz').read_bytes()).splitlines()]
    stdout=[json.loads(l)for l in gzip.decompress((p/'actual/stdout.jsonl.gz').read_bytes()).splitlines()]
    wire={r['id']:r for r in stdout if'id'in r}
    if len(calls)!=76 or len(pairs)!=19 or len(seeds)!=19 or [r['id']for r in stdin]!=list(range(1,79)) or len(wire)!=78:raise ValueError('actual request population/order differs')
    if [r for r in stdin if r['method']=='tools/call']!=[r['request']for r in calls] or any(wire[r['request']['id']]!=r['raw_response']for r in calls):raise ValueError('raw wire/call receipt differs')
    drafts={d['case_id']:d for d in data['gold']};observations=[];snapshots=set()
    for i,(case,pair,seed)in enumerate(zip(scope['cases'],pairs,seeds)):
        cid=case['case_id'];block=calls[i*4:i*4+4];order=['baseline','expansion']if i%2==0 else['expansion','baseline']
        if pair['case_id']!=cid or seed['case_id']!=cid or any(r['case_id']!=cid for r in block)or[r['phase']for r in block]!=['discovery','snapshot_probe',*['pilot_'+x for x in order]]or pair['order']!=order:raise ValueError('planned pair/order or discovery absent')
        node=seed['node'];s=case['source']
        canonical=scope['module']+'/'+str(Path(s['path']).parent)+'.'+('('+s['receiver']+').'if s['receiver']else'')+s['name']
        if node['canonical_id']!=canonical or canonical!=case['expected_canonical_id'] or any(node[k]!=s[k]for k in ('path','start_line','end_line','start_byte','end_byte_exclusive','file_sha256','function_sha256')):raise ValueError('seed declaration/canonical differs')
        found,_=c.unpack(block[0]['raw_response'])
        if block[0]['request']['params']['arguments']['name']!=canonical or not found or not any(x.get('file')==node['path']and x.get('start_line')==node['start_line']and x.get('end_line')==node['end_line']and x.get('commit_hash')==scope['source_commit']for x in found['citations']):raise ValueError('actual consumer discovery differs')
        pins=pair['pins'];snapshots.add(pins['snapshot_sha256'])
        probe,_=c.unpack(block[1]['raw_response'])
        if not probe or probe.get('snapshot_sha256')!=pins['snapshot_sha256']:raise ValueError('actual discovery probe snapshot differs')
        if pins['node_id']!=node['node_id']or pins['project_id']!=project or pins['dataset_id']!=dataset or pins['commit']!=scope['source_commit']or pins['function_sha256']!=s['function_sha256']:raise ValueError('actual pinned project/dataset/seed differs')
        requests=old.requests(pins)
        if pair['requests']!=requests:raise ValueError('paired request contract changed')
        projection_pins=dict(project_id=project,dataset_id=dataset,commit=scope['source_commit'],source_mode='committed',graph_snapshot_sha256=pins['snapshot_sha256'])
        arms={}
        for arm,call in zip(order,block[2:]):
            tool='cks.context.'+('value_dependencies'if arm=='baseline'else'value_dependency_expansion')
            if call['request']['params']['name']!=tool or call['request']['params']['arguments']!=requests[arm]or type(call['elapsed_ns'])is not int or call['elapsed_ns']<=0 or pair['arms'][arm]['request_id']!=call['request']['id']:raise ValueError('actual paired arguments/timing differ')
            arms[arm]=c.view(arm,call['raw_response'],retained,projection_pins,drafts[cid],data['locations'])
        observations.append(dict(case_id=cid,order=order,arms=arms,complete_pair=all(v['complete']for v in arms.values()),quality_score=None,warm_measurement=False))
    if len(snapshots)!=1:raise ValueError('mixed retained graph snapshots')
    summary=dict(kind='WI6_actual_DEV19_pilot_not_independent_acceptance',initial_failed_starts=1,initial_unreached_pairs=19,explicit_corrective_attempts=1,corrective_pairs=19,paired_arm_calls=38,discovery_calls=38,total_tool_calls=76,actual_sessions=2,successful_sessions=1,all_session_and_driver_groups_remaining=0,automatic_retries=0,exact_seed_resolutions=19,snapshot_count=1,retained_node_proofs=len(retained),complete_pairs=sum(o['complete_pair']for o in observations),status_counts={arm:dict(Counter(o['arms'][arm]['status']for o in observations))for arm in ('baseline','expansion')},raw_function_slots={arm:sum(len(o['arms'][arm]['raw_slots'])for o in observations)for arm in ('baseline','expansion')},outside_unknown_slots={arm:sum(s['AI_label']=='outside_unknown'for o in observations for s in o['arms'][arm]['raw_slots'])for arm in ('baseline','expansion')},warm_samples=0,quality_scores=None,independent_approved=0,formal_completed=0,formal_total=9,FINAL=0,D04='미정',freeze=None,execution_allowed=False,installed_agent_path_proved=False,paid_or_external_LLM_calls=0,embedding_inference_call_count=None)
    summary.update(verified_outside_gold_slots={arm:sum(s['identity_check']=='verified_retained_identity'and s['AI_label']=='outside_unknown'for o in observations for s in o['arms'][arm]['raw_slots'])for arm in ('baseline','expansion')},
        unresolved_identity_slots={arm:sum(s['identity_check']!='verified_retained_identity'for o in observations for s in o['arms'][arm]['raw_slots'])for arm in ('baseline','expansion')},
        seed_slots={arm:sum(s['seed']for o in observations for s in o['arms'][arm]['raw_slots'])for arm in ('baseline','expansion')},
        expansion_stop_reasons=dict(Counter(reason for o in observations for reason in o['arms']['expansion']['stop_reasons'])))
    return dict(summary=summary,observations=observations)


def worklist(data):
    state=read(ROOT/old.BASE/'ticket-rpc-lifecycle-results-v1/WORKLIST.json')
    state.update(current_stage='71_actual_WI6_DEV19_paired_pilot_partial_not_quality_acceptance',
        completed_in_this_scope=['source_module_receiver_and_exact19_retained_seed_resolution',
        'actual_CKS_discovery19_snapshot_probes19_basic19_expansion19',
        'initial_admission_failure_preserved_corrective1_reader_coordination_restored',
        'original_owned_payload_and_all_process_groups_verified',
        'partial_budget_and_outside_unknown_preserved_no_quality_or_warm_claim'],
        next_task='WI6_partial_budget_stop_semantics_and_same_pinned_session_warm_samples_separate_from_quality',
        supplemental_runtime_summary=data['summary'],storage_finalization_record='verify_actual_signed_Git_after_commit')
    state['remaining_all'][2]['task']='WI-6: DEV19 actual retained resolution and basic/expanded pilot observed; default responses partial/budget remain. Warm raw samples, stop-reason semantics, broader nonseed independent complete gold/families/sufficiency and installed consumer paths remain.'
    state['AI_DEV_draft_progress']['WI-6']='actual19_DEV_pairs38arms; all_partial_or_budget; warm0_independent0_quality_null'
    return state


def seal(p):
    write(p/'manifest.json',dict(kind='WI6_actual_DEV19_pilot_not_independent_acceptance',previous_revision=REVISION,
        tools=[dict(path=n,sha256=c.sha((ROOT/n).read_bytes()))for n in TOOLS],artifacts={str(x.relative_to(p)):c.sha(x.read_bytes())for x in sorted(p.rglob('*'))if x.is_file()and x!=p/'manifest.json'}))


def prepare(work):
    p=ROOT/PACKET
    if (p/'actual').exists():raise ValueError('refusing raw result overwrite')
    for source,label in [(work,'actual'),(work.parent/'ks-wi6-pair-pilot-corrective-driver-20261019','driver')]:
        for x in sorted(source.iterdir()):
            if x.is_file():compressed(p/label/(x.name+'.gz'),x.read_bytes())
    compressed(p/'proof/graph-manifest.json.gz',(work/'data/graph/manifest.json').read_bytes())
    retained=read(work/'retained-nodes.json');captured=read(work/'data/sources/manifest.json');files={x['path']:x for x in captured['files']if x['origin_id']=='repo'};proof={}
    for path in sorted({n['path']for n in retained.values()}):
        file=files[path];blob=(work/'data/sources/blobs'/file['sha256']).read_bytes();name='proof/files/'+file['sha256']+'.gz';compressed(p/name,blob);proof[path]=dict(file_sha256=file['sha256'],artifact=name)
    write(p/'proof/files.json',proof)
    data=derive();write(p/'SUMMARY.json',data['summary']);write(p/'observations.json',data['observations']);write(p/'WORKLIST.json',worklist(data));seal(p)


def validate(packet=None):
    p=packet or ROOT/PACKET;m=read(p/'manifest.json')
    if m['kind']!='WI6_actual_DEV19_pilot_not_independent_acceptance'or m['previous_revision']!=REVISION or[t['path']for t in m['tools']]!=TOOLS:raise ValueError('paired manifest scope/tool population changed')
    if m['artifacts']!={str(x.relative_to(p)):c.sha(x.read_bytes())for x in p.rglob('*')if x.is_file()and x!=p/'manifest.json'}or any(c.sha((ROOT/t['path']).read_bytes())!=t['sha256']for t in m['tools']):raise ValueError('paired evidence/tool inventory changed')
    data=derive(packet=p)
    if read(p/'SUMMARY.json')!=data['summary']or read(p/'observations.json')!=data['observations']or read(p/'WORKLIST.json')!=worklist(data):raise ValueError('partial/unknown result changed or promoted')
    return data


def stored():
    revision=subprocess.check_output(['git','log','-1','--format=%H','--',str(PACKET/'manifest.json')],cwd=ROOT,text=True).strip()
    if not re.fullmatch('[0-9a-f]{40}',revision):raise ValueError('paired runtime packet not committed')
    m=read(ROOT/PACKET/'manifest.json');paths=[PACKET/'manifest.json',*[PACKET/n for n in m['artifacts']],*[Path(n)for n in TOOLS]]
    paths += [Path('docs/spec-driven')/n for n in ('CURRENT-HANDOFF.md','KNOWLEDGE-ROADMAP-TEST-MATRIX.md','OPERATIONAL-ACCEPTANCE-PROTOCOL.md')]
    if any(subprocess.check_output(['git','show',revision+':'+str(p)],cwd=ROOT)!=(ROOT/p).read_bytes()for p in paths):raise ValueError('committed pair evidence/tools/docs differ')
    result=subprocess.run(['git','verify-commit',revision],cwd=ROOT,stdout=subprocess.PIPE,stderr=subprocess.PIPE)
    if result.returncode:raise ValueError('actual pair commit signature not verified')
    return revision


if __name__=='__main__':
    p=argparse.ArgumentParser();p.add_argument('command',choices=['prepare','check']);p.add_argument('--workspace',type=Path);p.add_argument('--require-formal',action='store_true');p.add_argument('--require-stored',action='store_true');a=p.parse_args()
    if a.command=='prepare':prepare(a.workspace.resolve())
    else:
        data=validate()
        if a.require_stored:stored()
        if a.require_formal:raise SystemExit('DEV pilot partial/budget responses are not independent frozen gold or warm evidence')
        print(json.dumps(data['summary'],ensure_ascii=False))
