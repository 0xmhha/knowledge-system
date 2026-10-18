#!/usr/bin/env python3
"""Owned compiler/reference probes, no candidate consumer or model invocation."""
import argparse,gzip,json,os,subprocess
from pathlib import Path
import wi1_select_review as g

CONTROLS=[
 ('ReceiveLoop','func ReceiveLoop(ch chan int) int { out:=0; for i:=0;i<2;i++ { select { case v:=<-ch: if v<5 { v=5 };out+=v } };return out }'),
 ('AssignExisting','func AssignExisting(ch chan int) int { v:=0; for i:=0;i<2;i++ { select { case v=<-ch: if v<5 { v=5 } } };return v }'),
 ('TwoValue','func TwoValue(ch chan int) bool { select { case v,ok:=<-ch: _=v;return ok };return false }'),
 ('ShadowOuter','func ShadowOuter(ch chan int) int { v:=7;select { case v:=<-ch:_=v;default: };return v }'),
 ('SendOnly','func SendOnly(ch chan int) { v:=7;select { case ch<-v:default: } }'),
 ('DefaultOnly','func DefaultOnly() int { v:=7;select { default:v=9 };return v }'),
 ('TwoCases','func TwoCases(a,b chan int) int { select { case x:=<-a:return x;case y:=<-b:return y } }'),
 ('Closure','func Closure(ch chan int) { f:=func(){ select { case x:=<-ch:_=x } };_=f }')]
WITNESS='''package main
import("encoding/json"; "os")
func main(){
 ch:=make(chan int,2);ch<-3;ch<-8
 seen:=[]int{};after:=[]int{}
 for i:=0;i<2;i++ { select { case interval:=<-ch:
  seen=append(seen,interval)
  if interval<5 { interval=5 }
  after=append(after,interval)
 } }
 json.NewEncoder(os.Stdout).Encode(map[string]any{"before":seen,"after":after})
}
'''


def run(work):
    work.mkdir(parents=True)
    env=dict(os.environ,GOCACHE=str(work/'go-cache'),GOPROXY='off',GOSUMDB='off',GOTOOLCHAIN='local')
    calls=[]
    def execute(args,label,input=None):
        r=subprocess.run(args,cwd=g.ROOT,env=env,input=input,capture_output=True,timeout=180)
        (work/(label+'-stdout.txt')).write_bytes(r.stdout);(work/(label+'-stderr.txt')).write_bytes(r.stderr)
        calls.append(dict(argv=args,exit=r.returncode,stdin_sha256=None if input is None else g.f.sha(input),stdout_sha256=g.f.sha(r.stdout),stderr_sha256=g.f.sha(r.stderr)))
        r.check_returncode();return r.stdout
    helper=(g.ROOT/'scripts/wi1_select_source.go.txt').read_bytes();(work/'observer.go').write_bytes(helper)
    execute(['go','build','-o',str(work/'observer'),str(work/'observer.go')],'build')
    cs=g.f.read(g.ROOT/g.matching.old.FUNCTIONS)['functions']
    inputs=[dict(x,Starts=[c['source']['start_byte']for c in cs if c['source']['path']==x['Path']])for x in g.f.read(g.ROOT/g.matching.old.SOURCES)]
    controls=[dict(Path='controls/'+name+'.go',Source='package controls\n'+s+'\n',Starts=[len(b'package controls\n')])for name,s in CONTROLS]
    for values,name,label in [(inputs,'inputs','observations'),(controls,'controls-inputs','controls-observations')]:
        raw=json.dumps(values,ensure_ascii=False).encode();(work/(name+'.json')).write_bytes(raw)
        (work/(label+'.json')).write_bytes(execute([str(work/'observer')],label,raw))
    (work/'runtime-witness.go').write_text(WITNESS)
    (work/'runtime-witness-output.json').write_bytes(execute(['go','run',str(work/'runtime-witness.go')],'runtime-witness'))
    receipt=dict(go_version=execute(['go','version'],'version').decode().strip(),helper_sha256=g.f.sha(helper),
        executable_sha256=g.f.sha((work/'observer').read_bytes()),candidate_valueflow_imported=False,new_consumer_calls=0,
        scope='original30 partial single-file types; controls type-valid; shared pinned CFG, no independent semantic approval',calls=calls,files={})
    def compressed(name):return gzip.compress(json.dumps(g.f.read(work/name),ensure_ascii=False,sort_keys=True,separators=(',',':')).encode(),mtime=0)
    for src,dst in [('observations.json','observations.json.gz'),('controls-observations.json','controls-observations.json.gz'),('inputs.json','observer-inputs.json.gz')]:receipt['files'][dst]=g.f.sha(compressed(src))
    for src,dst in [('controls-inputs.json','controls-inputs.json'),('runtime-witness.go','runtime-witness.go.txt'),('runtime-witness-output.json','runtime-witness-output.json')]:receipt['files'][dst]=g.f.sha((work/src).read_bytes())
    (work/'observer-receipt.json').write_text(json.dumps(receipt,ensure_ascii=False,indent=2)+'\n')


if __name__=='__main__':
    p=argparse.ArgumentParser();p.add_argument('--workspace',type=Path,required=True);a=p.parse_args();run(a.workspace.resolve())
