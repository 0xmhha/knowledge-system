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
import update_contract
import update_worker
PROJECTS=[('go-stablenet','/Users/kevin/work/github/0xmhha/auto-coding/chain/go-stablenet','0bf2f4d1bfeb6605006d556957ef8c045d8f8ed8'),('go-wemix','/Users/kevin/work/github/0xmhha/auto-coding/chain/go-wemix','1350376a680489298b3203a44d9ebddb7747f9cc')]


def main():
    parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--out',type=Path,required=True);out=parser.parse_args().out.resolve();manager.require(not out.exists(),'fresh_output_required');out.mkdir();fixtures=[];original=[]
    def write(name,v):(out/name).write_text(json.dumps(v,ensure_ascii=False,indent=2)+'\n')
    def git(repo,*args):return subprocess.check_output(['git','-c','core.hooksPath=/dev/null','-C',str(repo),*args],stderr=subprocess.PIPE).decode().strip()
    env=os.environ.copy();env.update(GOTOOLCHAIN='go1.26.9',GOCACHE='/private/tmp/ks-corpus-go-cache',GIT_LFS_SKIP_SMUDGE='1')
    for label,path,commit in PROJECTS:
        source=Path(path).resolve();head=git(source,'rev-parse','HEAD');status=git(source,'status','--porcelain=v1');manager.require(head==commit and not status,'original_source_must_match_clean_pinned_commit');original.append({'project':label,'source_root':str(source),'head':head,'status':status});write('original-before.json',original)
        base=out/label;base.mkdir();clone=base/'source'
        with(base/'clone.stdout.txt').open('wb')as stdout,(base/'clone.stderr.txt').open('wb')as stderr:
            subprocess.run(['git','-c','core.hooksPath=/dev/null','clone','--no-hardlinks','--no-checkout',str(source),str(clone)],env=env,stdout=stdout,stderr=stderr,check=True,timeout=120)
            subprocess.run(['git','-c','core.hooksPath=/dev/null','-C',str(clone),'checkout','--detach',commit],env=env,stdout=stdout,stderr=stderr,check=True,timeout=120)
        manager.require(git(clone,'rev-parse','HEAD')==commit and not git(clone,'status','--porcelain=v1'),'owned_clone_mismatch');print('PINNED: '+label+' '+commit,flush=True)
        os.environ.update(GOTOOLCHAIN='go1.26.9',GOCACHE='/private/tmp/ks-corpus-go-cache')
        manager.register(argparse.Namespace(source=str(clone),dataset=str(base/'dataset'),state_dir=str(base/'manager'),cks_bin=str(ROOT/'bin/cks'),graph_bin=str(ROOT/'bin/ckg'),vector_bin=str(ROOT/'bin/ckv'),embedder='mock',model_name='mock',project_id='comparison-'+label,source_mode='committed'))
        registration=str(base/'manager/registration.json');r=manager.load_registration(registration);worker=update_worker.Worker(registration);last=''
        try:
            policy=update_contract.defaults(r);policy.update(enabled=True,activation='activate_after_gate',deadline_seconds=1800);worker.journal.configure(policy);end=time.monotonic()+1900
            while True:
                worker.tick();success=worker.journal.state['last_success'];rows=worker.journal.state['attempts'];latest=rows[-1]if rows else None
                marker=json.dumps(latest,sort_keys=True)if latest else'waiting'
                if latest and marker!=last:write(label+'-latest-job.json',latest);last=marker
                if success:row=copy.deepcopy(worker.journal.row(success));break
                if latest and latest.get('outcome')in('failed','canceled','refused'):raise RuntimeError(label+' build terminal: '+str(latest))
                manager.require(time.monotonic()<end,'real_build_timeout');time.sleep(1)
        finally:worker.close()
        version=base/'dataset'/row['job']['candidate']['version'];manifest=json.loads((version/'sources/manifest.json').read_text());origins=sorted({'repo'}|{f['origin_id']for f in manifest['files']});manager.require(row['outcome']=='succeeded'and(version/'graph/graph.db').is_file()and(version/'vector/vector.db').is_file(),'actual_db_build_failed')
        item={'project':label,'original_root':str(source),'source_commit':commit,'owned_clone':str(clone),'version':str(version),'coordinates':row['job']['candidate']['coordinates'],'origins':origins,'source_files':len(manifest['files']),'source_manifest_sha256':hashlib.sha256((version/'sources/manifest.json').read_bytes()).hexdigest(),'embedding':'mock','quality_acceptance':False};fixtures.append(item);write(label+'-job.json',row);write('fixtures.json',fixtures);print('BUILT: '+label+' '+str(len(manifest['files']))+' retained files',flush=True)
    after=[{'project':label,'source_root':str(Path(path).resolve()),'head':git(path,'rev-parse','HEAD'),'status':git(path,'status','--porcelain=v1')}for label,path,commit in PROJECTS];write('original-after.json',after);manager.require(original==after,'original_source_changed');write('summary.json',{'scope':'two_actual_project_native_graph_vector_retained_builds_not_comparison_or_quality_yet','actual_native_dataset_builds':len(fixtures),'paid_model_calls':0,'original_sources_unchanged':True,'comparison_executed':False,'independent_quality_acceptance':False})

if __name__=='__main__':main()
