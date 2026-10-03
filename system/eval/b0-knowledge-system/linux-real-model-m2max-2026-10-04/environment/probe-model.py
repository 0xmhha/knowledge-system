import json,hashlib,math,urllib.request
from pathlib import Path
root=Path('/out')
def request(label,path,data=None):
 raw=None if data is None else json.dumps(data,ensure_ascii=False,separators=(',',':')).encode();r=urllib.request.Request('http://127.0.0.1:11434'+path,data=raw,headers={'Content-Type':'application/json'} if raw else {})
 if raw:(root/(label+'-request.json')).write_bytes(raw+b'\n')
 with urllib.request.urlopen(r,timeout=120) as response:result=response.read()
 (root/(label+'-response.json')).write_bytes(result);return json.loads(result)
show=request('show','/api/show',{'model':'bge-m3:latest'});result=request('embedding','/api/embed',{'model':'bge-m3:latest','input':['registry: The quartz module records a committed guide.','registry: 석영 모듈의 커밋된 안내 문서를 확인합니다.'],'truncate':False,'options':{'num_ctx':8192,'num_batch':8192}})
assert len(result['embeddings'])==2;norms=[]
for v in result['embeddings']:
 assert len(v)==1024 and all(math.isfinite(x) for x in v);norm=math.sqrt(sum(x*x for x in v));assert abs(norm-1)<0.001;norms.append(norm)
ps=request('residency','/api/ps');assert len(ps['models'])==1;model=ps['models'][0];assert model['digest']=='7907646426070047a77226ac3e684fbbe8410524f7b4a74d02837e43f2146bab' and model['size_vram']==0
(root/'probe-summary.json').write_text(json.dumps({'scope':'Linux arm64 CPU actual BGE-M3 API diagnostic, not retrieval quality','dimension':1024,'finite_vectors':2,'norms':norms,'server_model_digest':model['digest'],'size_vram':model['size_vram'],'options':{'num_ctx':8192,'num_batch':8192},'official_quality_metrics':None},indent=2)+'\n');print('two finite normalized 1024-dimension vectors; CPU residency matches approved digest')
