#!/usr/bin/env python3
"""Exercise real manager CLI/native CKS with explicit controlled review responses."""
import argparse
import copy
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
    commands,checks = [],[]
    def write(name,value):
        (out/name).write_text(json.dumps(value,ensure_ascii=False,indent=2)+'\n')
    def checked(name,value):
        require(value,name)
        checks.append(name)
        write('checks.json',checks)
        print('PASS: '+name,flush=True)
    env = dict(os.environ,PYTHONDONTWRITEBYTECODE='1',DISABLE_TELEMETRY='1',CLAUDE_CONFIG_DIR=str(out/'claude-config'))
    (out/'claude-config').mkdir()
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
    registrations,sources = {},{}
    for name in ('a','b'):
        source = out/('source-'+name)
        source.mkdir()
        (source/'go.mod').write_text('module example.invalid/same-project\n\ngo 1.26.9\n')
        (source/'value.go').write_text('package fixture\n\nfunc Value() int { return 7 }\n')
        (source/'.env').write_text('PRIVATE_INPUT_SENTINEL\n')
        (source/'private').mkdir()
        (source/'private/reason.md').write_text('PRIVATE_REASON_SENTINEL\n')
        sources[name]=source
        registrations[name] = run('register-'+name,[sys.executable,MANAGER,'register','--source',source,'--dataset',out/('dataset-'+name),'--state-dir',out/('manager-'+name),
            '--cks-bin',ROOT/'bin/cks','--graph-bin',ROOT/'bin/ckg','--vector-bin',ROOT/'bin/ckv','--embedder','mock','--model-name','mock'])
    project = registrations['a']['project_id']
    registration = registrations['a']['registration_file']
    before = {str(p.relative_to(out)):sha(p.read_bytes())for source in sources.values()for p in source.rglob('*')if p.is_file()}
    write('source-before.json',before)
    witness = out/'reported-command-not-executed'
    write('provided.json',{'schema_version':1,'project_id':project,'records':[
        {'kind':'decision','text':'The fixture owner reports choosing a bounded queue.','actor':'reported-owner'},
        {'kind':'troubleshooting','text':'The supplied incident report observed saturation; its cause needs review.'},
        {'kind':'command_result','command':['touch',str(witness)],'exit_code':0,'stdout':'Supplied command report; not executed.'}]})
    collected = run('collect',[sys.executable,MANAGER,'collect','--registration',registration,'--exclude','private/**','--context-file',out/'provided.json'])
    cid = collected['collection_id']
    c = json.loads((Path(collected['collection_dir'])/'manifest.json').read_text())['collection']
    provided = {r['kind']:r for r in c['files']if r['origin_id']=='provided'}
    def ref(kind):
        row = provided['provided_'+kind]
        return {'origin_id':'provided','path':row['path'],'start_line':1,'end_line':row['lines']}
    code = {'path':'value.go','start_line':3,'end_line':3}
    claims = [
        {'kind':'fact','statement':'The recorded code returns 7.','refs':[code]},
        {'kind':'hypothesis','statement':'The reason for choosing 7 remains unknown.','refs':[code]},
        {'kind':'reported_decision','statement':'The fixture owner reports choosing a bounded queue.','refs':[ref('decision')]},
        {'kind':'troubleshooting','statement':'The supplied incident report observed saturation.','refs':[ref('troubleshooting')]},
        {'kind':'reported_command_result','statement':'The supplied command result was not reexecuted.','refs':[ref('command_result')]},
        {'kind':'question','statement':'Which constraints explain choosing 7?','refs':[code]},
        {'kind':'fact','statement':'The recorded CODE returns 7.','refs':[code]},
    ]
    write('claims.json',{'schema_version':1,'project_id':project,'collection_id':cid,'claims':claims})
    result = run('draft',[sys.executable,MANAGER,'draft','--registration',registration,'--collection-id',cid,'--claims-file',out/'claims.json'])
    did = result['draft_id']
    draft = json.loads((Path(result['draft_dir'])/'draft.json').read_text())
    plan = run('review-plan',[sys.executable,MANAGER,'review-plan','--registration',registration,'--draft-id',did])
    checked('review plan asks exact-subject questions without inventing answers',len(plan['questions'])==7 and all(q['answer_status']=='not_provided'for q in plan['questions'])and plan['human_approval']=='not_inferred')
    checked('duplicate and shared-evidence interpretations remain candidates',len(plan['possible_duplicates'])==1 and plan['shared_evidence_interpretations']and all('not_proven_contradiction'in row['classification']for row in plan['shared_evidence_interpretations']))
    checked('fact hypothesis reported decision troubleshooting command and question stay distinct',{c['kind']for c in plan['claims']}=={'fact','hypothesis','reported_decision','troubleshooting','reported_command_result','question'})
    run('refuse-unreviewed',[sys.executable,MANAGER,'adopt','--registration',registration,'--draft-id',did,'--review-revision','0'*64],error='review_required')
    checked('unreviewed input cannot be adopted',True)
    document = {'schema_version':1,'project_id':project,'draft_id':did,'expected_revision':None,'reviewer':'explicit-fixture-reviewer',
                'response_source':{'kind':'controlled_fixture','reference':'Owned integration protocol response; not independent human acceptance'},
                'decisions':[{'claim_id':claim['claim_id'],'disposition':'excluded'if index==6 else 'deferred'if claim['kind']in ('hypothesis','question')else 'accepted','reason':'Controlled integration response for this exact claim.'}for index,claim in enumerate(draft['claims'])],
                'resolutions':[]}
    for label,mutate,error in [
        ('missing-reviewer',lambda row:row.update(reviewer=''),'reviewer_required'),
        ('foreign-project',lambda row:row.update(project_id=registrations['b']['project_id']),'project_or_subject_mismatch'),
        ('wrong-claim',lambda row:row['decisions'][0].update(claim_id='0'*64),'review_invalid'),
        ('unresolved-interpretation',lambda row:row['decisions'][1].update(disposition='accepted'),'review_conflict_resolution_required'),
    ]:
        bad = copy.deepcopy(document)
        mutate(bad)
        write(label+'.json',bad)
        run('refuse-'+label,[sys.executable,MANAGER,'review','--registration',registration,'--review-file',out/(label+'.json')],error=error)
        checked(label+' review is refused before adoption',True)
    write('review-input.json',document)
    recorded = run('review',[sys.executable,MANAGER,'review','--registration',registration,'--review-file',out/'review-input.json'])
    checked('supplied controlled review records exact draft without identity authentication',recorded['stage']=='review_recorded'and recorded['review_authentication']=='not_performed')
    replay = run('review-replay',[sys.executable,MANAGER,'review','--registration',registration,'--review-file',out/'review-input.json'])
    checked('same supplied review replays the original immutable revision',replay['stage']=='review_replayed'and replay['review_revision']==recorded['review_revision'])
    bad = copy.deepcopy(document)
    bad['decisions'][0]['reason']='Changed intent without expected revision.'
    write('stale-review.json',bad)
    run('refuse-stale-review',[sys.executable,MANAGER,'review','--registration',registration,'--review-file',out/'stale-review.json'],error='review_revision_conflict')
    checked('changed review cannot overwrite a newer revision',True)
    adopted = run('adopt',[sys.executable,MANAGER,'adopt','--registration',registration,'--draft-id',did,'--review-revision',recorded['review_revision']])
    write('adoption-result.json',adopted)
    projected = Path(adopted['input_project'])
    receipt = json.loads((projected.parent/'receipt.json').read_text())
    checked('adoption creates a filtered exact-project source projection',adopted['stage']=='input_adopted_not_DB_published'and (projected/'value.go').read_bytes()==(sources['a']/'value.go').read_bytes()and not(projected/'.env').exists()and not(projected/'private').exists())
    body = (projected/'.cks/knowledge/questions/manager-reviewed.md').read_text()
    checked('only four accepted claims enter the reviewed overlay',len(receipt['accepted_claim_ids'])==4 and len(receipt['omitted_claim_ids'])==3 and all(claims[index]['statement']not in body for index in (1,5,6)))
    checked('reviewed source does not become automatic semantic or external human verification',receipt['response_source']['kind']=='controlled_fixture'and receipt['review_authentication']=='not_performed'and receipt['retrieval_verified']is False and receipt['db_publication']=='not_run')
    actual = run('native-validate',[ROOT/'bin/cks','knowledge','--project-root',projected,'validate'])
    checked('actual post-rename native reviewed overlay is locked',actual['status']=='locked'and actual['project_id']==project and actual['policy_count']==actual['decision_count']==0)
    again = run('adopt-replay',[sys.executable,MANAGER,'adopt','--registration',registration,'--draft-id',did,'--review-revision',recorded['review_revision']])
    checked('adoption replays same sealed input without rebuilding DB',again['stage']=='adoption_replayed'and again['adoption_id']==adopted['adoption_id'])
    run('refuse-foreign-adoption',[sys.executable,MANAGER,'adoption-status','--registration',registrations['b']['registration_file'],'--adoption-id',adopted['adoption_id']],error='adoption_invalid')
    checked('other project cannot select this adoption',True)
    original = sources['a'].joinpath('value.go').read_bytes()
    sources['a'].joinpath('value.go').write_text('package fixture\nfunc Value()int{return 8}\n')
    run('refuse-source-drift',[sys.executable,MANAGER,'adoption-status','--registration',registration,'--adoption-id',adopted['adoption_id']],error='collection_stale')
    checked('source drift blocks reuse of old review for future build',True)
    sources['a'].joinpath('value.go').write_bytes(original)
    run('adoption-status-restored',[sys.executable,MANAGER,'adoption-status','--registration',registration,'--adoption-id',adopted['adoption_id']])
    retraction = {'schema_version':1,'project_id':project,'adoption_id':adopted['adoption_id'],'expected_revision':adopted['revision'],
                  'actor':'explicit-fixture-reviewer','reason':'Controlled withdrawal of input. No published DB exists.','response_source':document['response_source']}
    write('retraction-input.json',retraction)
    withdrawn = run('retract',[sys.executable,MANAGER,'retract','--registration',registration,'--retraction-file',out/'retraction-input.json'])
    checked('retraction preserves input evidence and separates DB rollback',withdrawn['stage']=='input_retracted_DB_rollback_not_run'and projected.is_dir())
    repeat = run('retract-replay',[sys.executable,MANAGER,'retract','--registration',registration,'--retraction-file',out/'retraction-input.json'])
    checked('retraction replays without a duplicate event',repeat['stage']=='retraction_replayed'and repeat['revision']==withdrawn['revision'])
    run('refuse-retracted-status',[sys.executable,MANAGER,'adoption-status','--registration',registration,'--adoption-id',adopted['adoption_id']],error='adoption_not_current')
    run('refuse-retracted-readopt',[sys.executable,MANAGER,'adopt','--registration',registration,'--draft-id',did,'--review-revision',recorded['review_revision']],error='review_revision_conflict')
    checked('retracted input cannot be reused or readopted with the old review',True)
    state = run('review-status',[sys.executable,MANAGER,'review-status','--registration',registration,'--draft-id',did])
    checked('restarted CLI reconstructs three-event chain with no current adoption',state['stage']=='retracted'and state['events']==3 and state['current_adoption_id']is None)
    final = {str(p.relative_to(out)):sha(p.read_bytes())for source in sources.values()for p in source.rglob('*')if p.is_file()}
    write('source-after.json',final)
    checked('original sources and DB paths remain unchanged and reported command never executes',before==final and not any((source/'.cks').exists()for source in sources.values())and not any((out/('dataset-'+name)).exists()for name in sources)and not witness.exists())
    report = run('native-plugin',['claude','plugin','validate','--strict','--json',ROOT/'plugins/knowledge-manager'])
    checked('updated plugin passes actual strict native validation',report['success']is True and report['strict']is True)
    summary = {'schema_version':1,'scope':'KM03_3_supplied_review_filtered_input_adoption_retraction_only','checks':len(checks),'commands':len(commands),
               'paid_model_calls':0,'actual_native_post_rename_validations':1,'controlled_response_records':True,'external_human_identity_authenticated':False,
               'independent_human_acceptance':False,'original_source_adopted':False,'db_published':False,'managed_reader_verified':False,'full_KM03_complete':False,'independent_acceptance':False}
    write('summary.json',summary)
    print(json.dumps(summary))


if __name__=='__main__':
    main()
