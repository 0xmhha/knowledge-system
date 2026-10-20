#!/usr/bin/env python3
"""Installed Claude Code -> declared updates MCP -> actual mock DB; fake local API."""
import argparse
import copy
import importlib.util
import json
from pathlib import Path
import shutil
import subprocess
import threading

ROOT=Path(__file__).resolve().parents[1]


def module(name,path):
    spec=importlib.util.spec_from_file_location(name,path);m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m);return m


base=module('updates_actual_Claude_base',ROOT/'scripts/verify-knowledge-manager-host.py')


class UpdateActions(base.Actions):
    def __init__(self,case,fixture,out):
        self.case,self.fixture,self.out=case,fixture,out
        self.steps,self.observed,self.fault=[],[],None
        self.index,self.awaiting,self.done=0,None,False
        self.kill_ready,self.kill_release=threading.Event(),threading.Event()
        self.state=fixture.setdefault('workflow',{})
        if case=='updates-full':
            self.add('cks.manager.update_policy_template',lambda:{},self.template)
            self.add('cks.manager.update_configure',lambda:{'policy_file':str(out/'policy.json')},self.configured)
            self.add('cks.manager.update_start',lambda:{'trigger':'manual','max_iterations':1,'max_seconds':60,'poll_seconds':.03},self.started)
            self.add('cks.manager.update_status',lambda:{},self.finished)
            self.add('cks.manager.update_read',self.read_args,self.read)
        else:
            self.add('cks.manager.update_status',lambda:{},self.persisted)
            self.add('cks.manager.update_read',self.read_args,self.read)
            self.add('cks.manager.update_start',lambda:{'trigger':'periodic','max_iterations':10000,'max_seconds':60,'poll_seconds':.03},self.started)
            self.add('cks.manager.update_status',lambda:{},self.noop)
            self.add('cks.manager.update_read',self.read_args,self.read)
        self.add('cks.manager.update_stop',lambda:{},self.stopped)
        self.add('cks.manager.update_status',lambda:{},self.final)

    def template(self,value):
        base.require(value['enabled']is False and value['activation']=='hold_for_review','unsafe default update policy')
        policy=copy.deepcopy(value)
        policy.update(enabled=True,triggers=['manual','periodic','change'],interval_seconds=1,deadline_seconds=120)
        base.write(self.out/'policy.json',policy)

    def configured(self,value):
        base.require(value['configured_policy']is not None and value['attempts_recorded']==0 and value['last_success']is None,'configuration claimed a build')

    def started(self,value):
        base.require(value['owner_stdin_EOF_enabled']is True and value['actual_job_or_read_success_inferred']is False,'spawn was declared a completed job')
        self.state['process']=value['process']

    def finished(self,value):
        row=value['last_success']
        if row is None:
            base.require(value['process']['exit_code']is None,'update child ended without successful build')
            return False
        job=row['job']
        base.require(value['project_id']==self.fixture['registration']['project_id'] and row['outcome']=='held'
            and job['state']=='held_for_review' and job['build_complete']is True and job['activated']is False,'unscoped/unverified/incorrectly published actual job')
        self.state['job']=job;self.state['intent_id']=row['intent_id'];self.state['revision']=value['ledger_revision']

    def persisted(self,value):
        base.require(value['last_success']['intent_id']==self.state['intent_id'] and value['last_success']['job']['candidate']==self.state['job']['candidate'],'restart changed retained scope')
        self.state['revision']=value['ledger_revision']

    def noop(self,value):
        base.require(value['last_success']['intent_id']==self.state['intent_id'] and value['attempts_recorded']==1,'restart admitted a duplicate source job')
        base.require(value['process']['exit_code']is None,'periodic child stopped before checked no-op')
        if value['phase']!='no_op'or value['ledger_revision']<=self.state['revision']:return False
        self.state['actual_noop_status']=value

    def read_args(self):return {'job_id':self.state['job']['job_id'],'expected_coordinates':self.state['job']['candidate']['coordinates'],'prompt':'HostValue returns 42'}

    def read(self,value):
        pack=value['pack'];base.require(value['stage']=='actual_retained_update_read' and pack['coordinates']==self.state['job']['candidate']['coordinates']
            and any('return 42'in json.dumps(b)for b in pack['bodies'])
            and all(c['project_id']==pack['coordinates']['project_id']and c['dataset_id']==pack['coordinates']['dataset_id']for c in pack['citations'])
            and value['publication_inferred_from_read']is False,'actual host read missing/mixed source')
        self.state['read']=pack

    def stopped(self,value):
        base.require(value['status']['process']['running']is False and value['status']['process']['exit_code']==0,'owned updater did not stop normally')
        self.state['stopped']=value

    def final(self,value):
        base.require(value['last_success']['intent_id']==self.state['intent_id'] and value['process']['running']is False
            and value['publication_or_serving_read_inferred']is False,'final metadata overclaimed serving state')


def main():
    parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--out',required=True,type=Path);parser.add_argument('--claude',default=shutil.which('claude'))
    args=parser.parse_args();out=args.out.resolve();base.require(not out.exists()and args.claude,'fresh output and installed CLI required');out.mkdir()
    version=subprocess.check_output([args.claude,'--version'],text=True).strip()
    base.write(out/'host-version.json',{'version':version,'executable':str(Path(args.claude).resolve()),'sha256':base.sha(Path(args.claude).resolve())})
    transport=module('updates_actual_Claude_transport',base.TRANSPORT)
    subject=base.fixture(out/'fixture','updates');observations=[]
    for case in('updates-full','updates-restart'):
        observation=base.run_host(args.claude,subject,case,out/case,transport,actions_factory=UpdateActions,manager_mode='updates')
        observations.append(observation);print('PASS: actual Claude '+case,flush=True)
    base.write(out/'summary.json',{'schema_version':1,'scope':'KM04_3_C2_actual_Claude_updates_MCP_local_scripted_API_not_model_quality',
        'host_version':version,'actual_host_runs':2,'actual_tool_results':sum(r['actual_tool_results']for r in observations),
        'observations':observations,'same_retained_job_after_restart':True,'checked_noop_on_periodic_restart':True,
        'real_model_calls':0,'real_model_cost_USD':0,'mock_embedding_not_quality':True,'independent_acceptance':False})


if __name__=='__main__':main()
