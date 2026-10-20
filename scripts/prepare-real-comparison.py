#!/usr/bin/env python3
"""Own pinned local clones and build two actual comparison datasets (mock embedding)."""
import argparse
import copy
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import time
ROOT=Path(__file__).resolve().parents[1]
sys.path.insert(0,str(ROOT/'plugins/knowledge-manager/scripts'))
import manager
from broker import Native
from update_native import value
PROJECTS=[('go-stablenet','/Users/kevin/work/github/0xmhha/auto-coding/chain/go-stablenet','0bf2f4d1bfeb6605006d556957ef8c045d8f8ed8'),('go-wemix','/Users/kevin/work/github/0xmhha/auto-coding/chain/go-wemix','1350376a680489298b3203a44d9ebddb7747f9cc')]


def main():
    parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--out',type=Path,required=True);parser.add_argument('--resume',action='store_true');args=parser.parse_args();out=args.out.resolve();manager.require(not out.exists()or args.resume and(out/'fixtures.json').is_file(),'fresh_output_required');out.mkdir(exist_ok=args.resume);prior={f['project']:f for f in json.loads((out/'fixtures.json').read_text())}if args.resume else{};fixtures=[];original=[]
    def write(name,v):(out/name).write_text(json.dumps(v,ensure_ascii=False,indent=2)+'\n')
    def git(repo,*args):return subprocess.check_output(['git','-c','core.hooksPath=/dev/null','-C',str(repo),*args],stderr=subprocess.PIPE).decode().strip()
    env=os.environ.copy();env.update(GOTOOLCHAIN='go1.26.9',GOCACHE='/private/tmp/ks-corpus-go-cache',GIT_LFS_SKIP_SMUDGE='1')
    for label,path,commit in PROJECTS:
        source=Path(path).resolve();head=git(source,'rev-parse','HEAD');status=git(source,'status','--porcelain=v1');manager.require(head==commit and not status,'original_source_must_match_clean_pinned_commit');original.append({'project':label,'source_root':str(source),'head':head,'status':status});write('original-before.json',original)
        base=out/label;base.mkdir(exist_ok=args.resume);clone=base/'source'
        if label in prior:
            f=prior[label];job=json.loads((out/(label+'-job.json')).read_text());manager.require(job['state']=='succeeded'and job['candidate']['coordinates']==f['coordinates']and f['source_commit']==commit and Path(f['version']).is_relative_to(base)and hashlib.sha256((Path(f['version'])/'sources/manifest.json').read_bytes()).hexdigest()==f['source_manifest_sha256']and(Path(f['version'])/'graph/graph.db').is_file()and(Path(f['version'])/'vector/vector.db').is_file(),'prior_build_not_verified');fixtures.append(f);print('REUSED VERIFIED BUILD: '+label,flush=True);continue
        if not clone.exists():
         with(base/'clone.stdout.txt').open('wb')as stdout,(base/'clone.stderr.txt').open('wb')as stderr:
             subprocess.run(['git','-c','core.hooksPath=/dev/null','clone','--no-hardlinks','--no-checkout',str(source),str(clone)],env=env,stdout=stdout,stderr=stderr,check=True,timeout=120)
             subprocess.run(['git','-c','core.hooksPath=/dev/null','-C',str(clone),'checkout','--detach',commit],env=env,stdout=stdout,stderr=stderr,check=True,timeout=120)
        exclusions=[];source_mode='committed';state=base/'manager'
        if label=='go-wemix':
            alias=clone/'cmd/gwemix';manager.require(git(clone,'rev-parse','HEAD')==commit,'owned_clone_mismatch')
            if alias.is_symlink():
                manager.require(os.readlink(alias)=='geth'and alias.resolve()==clone/'cmd/geth'and alias.resolve().is_dir(),'unexpected_source_alias');alias.unlink()
            manager.require(git(clone,'status','--porcelain=v1')=='D cmd/gwemix','unexpected_prepared_tree_change');source_mode='working-tree';state=base/'manager-regular-source';exclusions=[{'path':'cmd/gwemix','kind':'directory_symlink_alias','target':'geth','reason':'non-regular alias excluded from owned study tree; cmd/geth regular source retained; original repository unchanged'}];write(label+'-source-scope.json',{'source_mode':source_mode,'base_commit':commit,'prepared_git_status':git(clone,'status','--porcelain=v1'),'excluded_non_regular':exclusions})
        else:manager.require(git(clone,'rev-parse','HEAD')==commit and not git(clone,'status','--porcelain=v1'),'owned_clone_mismatch')
        print('PINNED: '+label+' '+commit,flush=True)
        os.environ.update(GOTOOLCHAIN='go1.26.9',GOCACHE='/private/tmp/ks-corpus-go-cache')
        manager.register(argparse.Namespace(source=str(clone),dataset=str(base/'dataset'),state_dir=str(state),cks_bin=str(ROOT/'bin/cks'),graph_bin=str(ROOT/'bin/ckg'),vector_bin=str(ROOT/'bin/ckv'),embedder='mock',model_name='mock',project_id='comparison-'+label,source_mode=source_mode))
        registration=str(state/'registration.json');r=manager.load_registration(registration);native=Native(r,state/'cks.json');calls=[]
        def tool(name,args):
            response=native.tool(name,args);calls.append({'tool':name,'arguments':copy.deepcopy(args),'response':copy.deepcopy(response)});write(label+'-build-calls.json',calls);return value(response)
        scope={'schema_version':1,'project_id':r['project_id'],'profile_id':r['profile_id']}
        try:
            profile=tool('cks.dataset.inspect_profile',scope)['profile'];write(label+'-profile.json',profile)
            request={**scope,**{k:profile[k]for k in('profile_digest','source_pin','input_digest','embedding_digest')},'base':profile['observed_base'],'new_version':'real-source-v1','activation':'activate_after_gate','idempotency_key':'real-comparison-'+label+'-'+commit[:12],'deadline_seconds':1800}
            job=tool('cks.dataset.build',request);end=time.monotonic()+1850
            while not job['terminal']:
                write(label+'-latest-job.json',job);manager.require(time.monotonic()<end,'real_build_timeout');time.sleep(1);job=tool('cks.dataset.status',{**scope,'job_id':job['job_id']})
            write(label+'-latest-job.json',job);manager.require(job['state']=='succeeded'and job['activated']and job['error']is None,'actual_db_build_failed')
        finally:
            code=native.close();write(label+'-build-exit.json',{'EOF_exit_code':code});manager.require(code==0,'actual_builder_EOF_failure')
        version=base/'dataset'/job['candidate']['version'];manifest=json.loads((version/'sources/manifest.json').read_text());origins=sorted({'repo'}|{f['origin_id']for f in manifest['files']});manager.require((version/'graph/graph.db').is_file()and(version/'vector/vector.db').is_file(),'actual_db_build_failed')
        manager.require(job['candidate']['coordinates']['source_mode']==source_mode and job['candidate']['coordinates']['base_commit']==commit,'native_source_commit_not_preserved')
        item={'project':label,'original_root':str(source),'source_commit':commit,'owned_clone':str(clone),'version':str(version),'coordinates':job['candidate']['coordinates'],'origins':origins,'source_files':len(manifest['files']),'source_manifest_sha256':hashlib.sha256((version/'sources/manifest.json').read_bytes()).hexdigest(),'embedding':'mock','quality_acceptance':False,'prepared_git_status':git(clone,'status','--porcelain=v1'),'excluded_non_regular':exclusions};fixtures.append(item);write(label+'-job.json',job);write('fixtures.json',fixtures);print('BUILT: '+label+' '+str(len(manifest['files']))+' retained files',flush=True)
    after=[{'project':label,'source_root':str(Path(path).resolve()),'head':git(path,'rev-parse','HEAD'),'status':git(path,'status','--porcelain=v1')}for label,path,commit in PROJECTS];write('original-after.json',after);manager.require(original==after,'original_source_changed');write('summary.json',{'scope':'two_actual_project_native_graph_vector_retained_builds_not_comparison_or_quality_yet','actual_native_dataset_builds':len(fixtures),'paid_model_calls':0,'original_sources_unchanged':True,'comparison_executed':False,'independent_quality_acceptance':False})

if __name__=='__main__':main()
