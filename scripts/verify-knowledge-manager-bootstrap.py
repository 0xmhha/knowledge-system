#!/usr/bin/env python3
"""Native plugin validation and actual launcher discovery; no model request."""
import argparse
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[1]
PLUGIN = ROOT/'plugins/knowledge-manager'


def require(value,message):
    if not value:
        raise ValueError(message)


def sha(data):
    return hashlib.sha256(data).hexdigest()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--out',required=True,type=Path)
    out = parser.parse_args().out.resolve()
    require(not out.exists(),'fresh owned root required')
    out.mkdir()
    source,state,dataset = out/'source',out/'manager',out/'dataset'
    source.mkdir()
    (source/'go.mod').write_text('module example.invalid/no-docs\n\ngo 1.26.9\n')
    (source/'consensus.go').write_text('package sample\nfunc Consensus()int{return 1}\n')
    before = {p.name:sha(p.read_bytes())for p in source.iterdir()}
    checks,commands = [],[]
    def write(name,value):
        (out/name).write_text(json.dumps(value,ensure_ascii=False,indent=2)+'\n')
    def checked(name,value):
        require(value,name)
        checks.append(name)
        write('checks.json',checks)
        print('PASS: '+name,flush=True)
    isolated = out/'claude-config'
    isolated.mkdir()
    environment = dict(os.environ,CLAUDE_CONFIG_DIR=str(isolated),DISABLE_TELEMETRY='1',PYTHONDONTWRITEBYTECODE='1')
    def run(label,args,cwd=ROOT):
        result = subprocess.run([str(a)for a in args],cwd=cwd,env=environment,capture_output=True,timeout=45)
        (out/(label+'.stdout.txt')).write_bytes(result.stdout)
        (out/(label+'.stderr.txt')).write_bytes(result.stderr)
        commands.append({'label':label,'command':[str(a)for a in args],'cwd':str(cwd),'exit_code':result.returncode})
        write('commands.json',commands)
        require(result.returncode==0,'command failed: '+label)
        return result.stdout
    version = run('claude-version',['claude','--version']).decode().strip()
    for label,target in [('manifest',PLUGIN),('skills',PLUGIN/'skills'),('agents',PLUGIN/'agents')]:
        report = json.loads(run('native-'+label,['claude','plugin','validate','--strict','--json',target]))
        require(report['success']is True,'native validation failed')
        checked('native strict '+label+' validation passes without model call',True)
    manager = PLUGIN/'scripts/manager.py'
    registration = json.loads(run('register',[sys.executable,manager,'register','--source',source,'--dataset',dataset,'--state-dir',state,
        '--cks-bin',ROOT/'bin/cks','--graph-bin',ROOT/'bin/ckg','--vector-bin',ROOT/'bin/ckv','--embedder','mock','--model-name','mock']))
    path = Path(registration['registration_file'])
    write('registration-result.json',registration)
    checked('document-free project registration writes only owned state', registration['stage']=='registered_not_built' and not dataset.exists() and before=={p.name:sha(p.read_bytes())for p in source.iterdir()})
    outside = out/'unrelated-cwd'
    outside.mkdir()
    offline = json.loads(run('doctor-offline',[sys.executable,manager,'doctor','--registration',path],outside))
    checked('offline diagnosis preserves explicit project and unimplemented boundaries', offline['project_id']==registration['project_id'] and all(offline[n]is False for n in ('draft_collection','review_workflow','managed_reader_broker','scheduling')))
    actual = json.loads(run('doctor-connect',[sys.executable,manager,'doctor','--registration',path,'--connect'],outside))
    checked('actual metadata doctor from unrelated cwd discovers four maintenance tools',len(actual['connection']['maintenance_tools'])==4 and actual['connection']['EOF_exit_code']==0 and actual['connection']['model_inspection']=='not_run')
    libspec = importlib.util.spec_from_file_location('bootstrap_client',ROOT/'scripts/eval-value-dependencies.py')
    lib = importlib.util.module_from_spec(libspec)
    libspec.loader.exec_module(lib)
    wire = out/'launcher-session'
    wire.mkdir()
    previous = os.environ.get('CKS_MANAGER_REGISTRATION')
    os.environ['CKS_MANAGER_REGISTRATION']=str(path)
    try:
        client = lib.Client(None,None,wire,command=[sys.executable,str(manager),'serve'])
    finally:
        if previous is None:
            os.environ.pop('CKS_MANAGER_REGISTRATION',None)
        else:
            os.environ['CKS_MANAGER_REGISTRATION']=previous
    try:
        reply = client.request('initialize',{'protocolVersion':'2024-11-05','capabilities':{},'clientInfo':{'name':'manager-bootstrap-verifier','version':'1'}})
        require('error'not in reply,'launcher initialization failed')
        notice = json.dumps({'jsonrpc':'2.0','method':'notifications/initialized'})+'\n'
        client.stdin.write(notice)
        client.stdin.flush()
        client.proc.stdin.write(notice)
        client.proc.stdin.flush()
        reply = client.request('tools/list')
        tools = reply['result']['tools']
        selected = {t['name']:t for t in tools if t['name'].startswith('cks.dataset.')}
        write('tools.json',tools)
        checked('plugin launcher execs actual CKS and preserves public tool schemas',set(selected)=={'cks.dataset.'+n for n in ('inspect_profile','build','status','cancel')} and all(t.get('outputSchema')for t in selected.values()) and len(tools)==37)
    finally:
        client.close()
    checked('launcher actual MCP EOF exits zero',client.proc.returncode==0)
    checked('metadata-only connections do not create datasets or source documents',not dataset.exists() and before=={p.name:sha(p.read_bytes())for p in source.iterdir()} and not list(source.rglob('*.md')))
    summary = {'schema_version':1,'scope':'KM03_1_native_package_registration_metadata_only','checks':len(checks),'claude_version':version,
               'native_validations':3,'actual_metadata_connections':2,'launcher_EOF_exit_code':client.proc.returncode,'paid_model_calls':0,
               'model_behavior_verified':False,'full_KM03_complete':False,'independent_acceptance':False}
    write('summary.json',summary)
    print(json.dumps(summary))


if __name__=='__main__':
    main()
