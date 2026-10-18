import sys,json,hashlib,subprocess
from pathlib import Path
R=Path(__file__).resolve().parents[1];sys.path.insert(0,str(R/'scripts'));import wi2_dev_gold as g
import argparse
p=argparse.ArgumentParser();p.add_argument('--out',type=Path,required=True);out=p.parse_args().out.resolve();out.mkdir(parents=True,exist_ok=True);sources,functions=g.source_inputs();by={f['id']:f for f in functions};inputs=[];catalog=[]
ids=['RULE-1-05','RULE-1-07','RULE-1-15','RULE-2-06','RULE-2-07','RULE-2-16']
prefix='''package audit
import(exec "os/exec";process "os/exec";strconv "strconv";conversion "strconv")
var _,_,_,_ = exec.Command,process.Command,strconv.Atoi,conversion.Atoi
type Fake struct{}
func(Fake)Command(string){}
func(Fake)Atoi(string)(int,error){return 0,nil}
'''
for id in ids:
 for variant in (['identity','constant'] if id in ('RULE-1-05','RULE-2-06')else['scope']):
  helpers=('func sanitize(v string)string{return v}\nfunc clamp(v int)int{return v}\n' if variant!='constant'else'func sanitize(v string)string{return "constant"}\nfunc clamp(v int)int{return 7}\n')
  fn=by[id];source=prefix+helpers+fn['code']+'\n';key=id+'-'+variant;path=out/'witnesses'/(key+'.go.txt');path.parent.mkdir(exist_ok=True);path.write_text(source)
  inputs.append(dict(ID=key,Path=key+'.go',Source=source,Package='example.invalid/wi2audit/'+key,Profiles=None))
  catalog.append(dict(id=key,original_case_id=id,fragment_sha256=fn['function_sha256'],full_file_sha256=hashlib.sha256(source.encode()).hexdigest(),fragment_start=len((prefix+helpers).encode()),fragment_end=len((prefix+helpers+fn['code']).encode()),purpose='supplemental type-valid scope witness; new helper context does not replace old incomplete prefix or approve old semantics'))
for id in ids:
 fn=by[id];full=next(i['Source']for i in sources if i['Path']==fn['path']);inputs.append(dict(ID='ORIGINAL-'+id,Path='original-'+id+'.go',Source=full,Package='example.invalid/wi2original/'+id,Profiles=None))
profiles=[dict(Name='darwin_default',GOOS='darwin',GOARCH='arm64',Tags=[]),dict(Name='linux_default',GOOS='linux',GOARCH='arm64',Tags=[]),dict(Name='darwin_none_tag',GOOS='darwin',GOARCH='arm64',Tags=['none']),dict(Name='darwin_gofuzz_tag',GOOS='darwin',GOARCH='arm64',Tags=['gofuzz'])]
for path in ['build/ci.go','build/update-license.go','crypto/signify/signify_fuzz.go','metrics/disk_linux.go']:
 raw=next(x['Source']for x in sources if x['Path']==path);p=out/'original-source'/path;p.parent.mkdir(parents=True,exist_ok=True);p.write_text(raw);inputs.append(dict(ID=path,Path=str(p),Source='',Package='',Profiles=profiles))
(out/'oracle-inputs.json').write_text(json.dumps(inputs,ensure_ascii=False,indent=2)+'\n');(out/'witness-catalog.json').write_text(json.dumps(catalog,indent=2)+'\n');(out/'scope-oracle.go').write_text((R/'scripts/wi2_contract_scope.go.txt').read_text())
