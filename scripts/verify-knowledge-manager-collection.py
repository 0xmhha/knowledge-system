#!/usr/bin/env python3
"""Real helper collection and native CKS proposed overlays, without DB/model."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[1]
MANAGER = ROOT/'plugins/knowledge-manager/scripts/manager.py'


def require(value,message):
    if not value:
        raise ValueError(message)


def sha(data):
    return hashlib.sha256(data).hexdigest()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--out',required=True,type=Path)
    out = parser.parse_args().out.resolve()
    require(not out.exists(),'fresh owned output required')
    out.mkdir()
    checks,commands = [],[]
    def write(name,value):
        (out/name).write_text(json.dumps(value,ensure_ascii=False,indent=2)+'\n')
    def checked(name,value):
        require(value,name)
        checks.append(name)
        write('checks.json',checks)
        print('PASS: '+name,flush=True)
    env = dict(os.environ,PYTHONDONTWRITEBYTECODE='1',DISABLE_TELEMETRY='1')
    (out/'claude-config').mkdir()
    env['CLAUDE_CONFIG_DIR']=str(out/'claude-config')
    def run(label,args,error=None):
        p = subprocess.run([str(a)for a in args],cwd=ROOT,env=env,capture_output=True,timeout=60)
        (out/(label+'.stdout.txt')).write_bytes(p.stdout)
        (out/(label+'.stderr.txt')).write_bytes(p.stderr)
        commands.append({'label':label,'command':[str(a)for a in args],'exit_code':p.returncode,'expected_error':error})
        write('commands.json',commands)
        require((p.returncode==0)==(error is None),'unexpected command result: '+label)
        if error:
            value = json.loads(p.stderr)
            require(value['code']==error,'wrong refusal: '+label)
            return value
        return json.loads(p.stdout)
    sources,registrations = {},{}
    for project in ('a','b'):
        source = out/('source-'+project)
        source.mkdir()
        (source/'go.mod').write_text('module example.invalid/same-fork\n\ngo 1.26.9\n')
        (source/'consensus.go').write_text('package common\n\n// SharedConsensus returns the fixture marker.\nfunc SharedConsensus() int { return 101 }\n')
        (source/'config.json').write_text('{"batch_limit":64}\n')
        (source/'.env').write_text('PRIVATE_INPUT_MUST_NOT_ENTER_COLLECTION\n')
        (source/'private').mkdir()
        (source/'private/decision.txt').write_text('EXCLUDED_DECISION_MUST_NOT_ENTER_COLLECTION\n')
        sources[project]=source
        value = run('register-'+project,[sys.executable,MANAGER,'register','--source',source,'--dataset',out/('dataset-'+project),'--state-dir',out/('manager-'+project),
                   '--cks-bin',ROOT/'bin/cks','--graph-bin',ROOT/'bin/ckg','--vector-bin',ROOT/'bin/ckv','--embedder','mock','--model-name','mock'])
        registrations[project]=value
    git_commands = []
    for args in (['init','--quiet'],['add','--','.'],['-c','user.name=fixture','-c','user.email=fixture@example.invalid','-c','commit.gpgsign=false','-c','core.hooksPath=/dev/null','commit','--quiet','-m','Controlled source fixture']):
        command = ['git','-C',str(sources['a']),*args]
        process = subprocess.run(command,env=env,capture_output=True,timeout=20)
        git_commands.append({'command':command,'exit_code':process.returncode,'stdout':process.stdout.decode(),'stderr':process.stderr.decode()})
        require(process.returncode==0,'fixture Git setup failed')
    write('git-commands.json',git_commands)
    checked('same source names bytes and module keep separate project registrations',registrations['a']['project_id']!=registrations['b']['project_id'])
    before = {str(p.relative_to(out)):sha(p.read_bytes())for root in sources.values()for p in root.rglob('*')if p.is_file()}
    write('source-before.json',before)
    witness = out/'reported-command-was-not-run'
    material = out/'provided-context.json'
    material.write_text(json.dumps({'schema_version':1,'project_id':registrations['a']['project_id'],'records':[
        {'kind':'decision','text':'The supplied owner reports choosing a bounded queue.','actor':'fixture-owner'},
        {'kind':'command_result','command':['touch',str(witness)],'exit_code':0,'stdout':'Supplied result; not executed by the manager.'}]}))
    a_path = registrations['a']['registration_file']
    b_path = registrations['b']['registration_file']
    a = run('collect-a',[sys.executable,MANAGER,'collect','--registration',a_path,'--exclude','private/**','--context-file',material])
    b = run('collect-b',[sys.executable,MANAGER,'collect','--registration',b_path,'--exclude','private/**'])
    write('collections.json',{'a':a,'b':b})
    checked('document-free source collection emits immutable provenance without source writes',a['stage']=='collected_not_reviewed'and not list(sources['a'].rglob('*.md'))and before=={str(p.relative_to(out)):sha(p.read_bytes())for root in sources.values()for p in root.rglob('*')if p.is_file()})
    checked('cross-project collection IDs stay distinct for identical code',a['collection_id']!=b['collection_id'])
    payload = json.loads((Path(a['collection_dir'])/'manifest.json').read_text())['collection']
    checked('collection identity is separate from CKS snapshot and meaning verification',payload['cks_snapshot_id']is None and payload['coverage']['whole_project_semantics_verified']is False)
    emitted = b''.join(p.read_bytes()for p in Path(a['collection_dir']).rglob('*')if p.is_file())
    checked('private source paths are excluded from actual collected blobs and manifest',b'PRIVATE_INPUT_MUST_NOT_ENTER_COLLECTION'not in emitted and b'EXCLUDED_DECISION_MUST_NOT_ENTER_COLLECTION'not in emitted and payload['coverage']['excluded_counts'])
    again = run('collect-a-replay',[sys.executable,MANAGER,'collect','--registration',a_path,'--exclude','private/**','--context-file',material])
    checked('identical collection replays same ID and observation',again['stage']=='collection_replayed'and again['collection_id']==a['collection_id'])
    evidence = run('evidence-a',[sys.executable,MANAGER,'evidence','--registration',a_path,'--collection-id',a['collection_id'],'--path','consensus.go','--start-line','4','--end-line','4'])
    other = run('evidence-b',[sys.executable,MANAGER,'evidence','--registration',b_path,'--collection-id',b['collection_id'],'--path','consensus.go','--start-line','4','--end-line','4'])
    checked('exact actual source span file/hash and project identity are preserved',evidence['evidence']['file_sha256']==sha((sources['a']/'consensus.go').read_bytes())and evidence['evidence']['content_sha256']==sha(evidence['text'].encode())and 'return 101'in evidence['text'])
    checked('identical code span cannot merge project evidence identities',evidence['evidence']['evidence_id']!=other['evidence']['evidence_id']and evidence['evidence']['project_id']!=other['evidence']['project_id'])
    provided = next(row for row in payload['files']if row['origin_id']=='provided'and row['kind']=='provided_decision')
    command = next(row for row in payload['files']if row['origin_id']=='provided'and row['kind']=='provided_command_result')
    checked('provided actor is reported provenance and absent actor stays unknown',provided['source_actor']=='fixture-owner'and command['source_actor']is None and command['reported_not_reexecuted']is True)
    checked('provided command report is never executed',not witness.exists())
    default = run('draft-default',[sys.executable,MANAGER,'draft','--registration',a_path,'--collection-id',a['collection_id']])
    doc = json.loads((Path(default['draft_dir'])/'draft.json').read_text())
    checked('automatic draft keeps every claim proposed without human approval',doc['human_approval']=='not_recorded'and all(c['status']=='proposed'and c['reviewed_by']is None and c['semantic_verification']=='not_run'for c in doc['claims']))
    checked('provided decisions and command reports retain separate draft kinds',{'reported_decision','reported_command_result'}<={c['kind']for c in doc['claims']})
    claims = {'schema_version':1,'project_id':a['project_id'],'collection_id':a['collection_id'],'claims':[
        {'kind':'fact','statement':'The source excerpt shows the fixture marker 101; interpretation awaits review.','refs':[{'path':'consensus.go','start_line':4,'end_line':4}]},
        {'kind':'hypothesis','statement':'The marker may be used by tests; this hypothesis is unverified.','refs':[{'path':'consensus.go','start_line':4,'end_line':4}]},
        {'kind':'reported_decision','statement':'The supplied owner reports choosing a bounded queue.','refs':[{'origin_id':'provided','path':provided['path'],'start_line':1,'end_line':provided['lines']}]},
        {'kind':'reported_command_result','statement':'The supplied command result is a report and was not rerun.','refs':[{'origin_id':'provided','path':command['path'],'start_line':1,'end_line':command['lines']}]},
        {'kind':'question','statement':'Why was marker 101 selected? This collection does not establish the rationale.','refs':[{'path':'consensus.go','start_line':4,'end_line':4}]}]}
    write('claims-input.json',claims)
    custom = run('draft-claims',[sys.executable,MANAGER,'draft','--registration',a_path,'--collection-id',a['collection_id'],'--claims-file',out/'claims-input.json'])
    write('draft-results.json',{'default':default,'custom':custom})
    generated = json.loads((Path(custom['draft_dir'])/'draft.json').read_text())
    checked('fact hypothesis reported decision and question remain separate proposals',{r['kind']for r in generated['claims']}=={'fact','hypothesis','reported_decision','reported_command_result','question'}and all(r['review_reason']is None for r in generated['claims']))
    for label,value in [('default',default),('custom',custom)]:
        actual = run('native-validate-'+label,[ROOT/'bin/cks','knowledge','--project-root',Path(value['draft_dir'])/'project','validate'])
        checked('actual native '+label+' overlay validates and remains structurally locked',actual['status']=='locked'and actual['project_id']==a['project_id']and actual['policy_count']==actual['decision_count']==0)
    replay = run('draft-claims-replay',[sys.executable,MANAGER,'draft','--registration',a_path,'--collection-id',a['collection_id'],'--claims-file',out/'claims-input.json'])
    checked('sealed draft replays without rewriting its source or review',replay['stage']=='draft_replayed'and replay['draft_id']==custom['draft_id'])
    wrong = dict(claims,project_id=registrations['b']['project_id'])
    write('foreign-claims.json',wrong)
    run('refuse-foreign-claims',[sys.executable,MANAGER,'draft','--registration',a_path,'--collection-id',a['collection_id'],'--claims-file',out/'foreign-claims.json'],error='project_mismatch')
    checked('foreign project claims cannot enter the draft',True)
    invented = dict(claims,claims=[{'kind':'reported_decision','statement':'The team chose this for scalability.','refs':[{'path':'consensus.go','start_line':4,'end_line':4}]}])
    write('invented-decision.json',invented)
    run('refuse-invented-reason',[sys.executable,MANAGER,'draft','--registration',a_path,'--collection-id',a['collection_id'],'--claims-file',out/'invented-decision.json'],error='decision_requires_reported_source')
    checked('code alone cannot invent a reported human decision',True)
    sources['a'].joinpath('consensus.go').write_text('package common\n\n// SharedConsensus returns a changed fixture marker.\nfunc SharedConsensus() int { return 303 }\n')
    newer = run('collect-a-changed',[sys.executable,MANAGER,'collect','--registration',a_path,'--exclude','private/**','--context-file',material])
    write('changed-collection.json',newer)
    old = run('evidence-old-after-change',[sys.executable,MANAGER,'evidence','--registration',a_path,'--collection-id',a['collection_id'],'--path','consensus.go','--start-line','4','--end-line','4'])
    checked('source change creates new collection while old evidence stays reproducible',newer['collection_id']!=a['collection_id']and old==evidence)
    newer_payload = json.loads((Path(newer['collection_dir'])/'manifest.json').read_text())['collection']
    diff_lines = next(row['lines']for row in newer_payload['files']if row['origin_id']=='change'and row['path']=='consensus.go')
    change = run('evidence-change',[sys.executable,MANAGER,'evidence','--registration',a_path,'--collection-id',newer['collection_id'],'--origin-id','change','--path','consensus.go','--start-line','1','--end-line',str(diff_lines)])
    checked('actual Git working tree diff preserves removed and added source lines',change['evidence']['origin_id']=='change'and change['evidence']['source_kind']=='git_diff'and '-func SharedConsensus() int { return 101 }'in change['text']and '+func SharedConsensus() int { return 303 }'in change['text'])
    checked('Git changes preserve actual base commit and bounded selected file coverage',newer_payload['git']['tracked_changes']is True and len(newer_payload['git']['commit'])==40 and newer_payload['coverage']['git_changes']['scope']=='selected_existing_files_HEAD_to_worktree'and newer_payload['coverage']['git_changes']['deleted_files']=='separate_KM04_change_tracking')
    state = run('draft-status',[sys.executable,MANAGER,'draft-status','--registration',a_path,'--draft-id',custom['draft_id']])
    checked('old proposed draft remains an explicit as-of collection after source changes',state['draft']['collection_id']==a['collection_id']and state['draft']['db_adoption']=='not_run')
    checked('no original knowledge overlay dataset or command side effect is created',not any((source/'.cks').exists()for source in sources.values())and not any((out/('dataset-'+p)).exists()for p in sources)and not witness.exists())
    report = run('native-plugin-validation',['claude','plugin','validate','--strict','--json',ROOT/'plugins/knowledge-manager'])
    checked('updated plugin passes actual strict native validation',report['success']is True)
    summary = {'schema_version':1,'scope':'KM03_2_source_collection_and_proposed_native_overlay_only','checks':len(checks),'paid_model_calls':0,
               'native_knowledge_post_rename_validations':2,'source_adopted':False,'human_review_verified':False,'db_built':False,'managed_reader_verified':False,
               'full_KM03_complete':False,'independent_acceptance':False,'actual_Git_change_evidence':True,'synthetic_private_sentinels_in_test_source_only':True}
    write('summary.json',summary)
    print(json.dumps(summary))


if __name__=='__main__':
    main()
