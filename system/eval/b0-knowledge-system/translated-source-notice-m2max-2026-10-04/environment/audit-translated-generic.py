from pathlib import Path
import json,hashlib,zipfile,re
import sys
root=Path(sys.argv[1]);env=json.loads((root/'go-environment.json').read_text());cache=Path(env['GOMODCACHE'])
packages=json.loads((root/'modernc-selected-packages.json').read_text());build=(root/'binary-build-info.txt').read_text();sums=Path('go.sum').read_text().splitlines();records=[];modules={};seen=set()
def sha(raw):return hashlib.sha256(raw).hexdigest()
for p in packages:
 module=p['Module'];key=(module['Path'],module['Version']);mroot=Path(module['Dir']);prefix=f'{key[0]}@{key[1]}/'
 if key not in modules:
  dl=cache/'cache/download'/key[0]/'@v';origin=json.loads((dl/(key[1]+'.info')).read_text());h1=(dl/(key[1]+'.ziphash')).read_text().strip()
  assert ' '.join([*key,h1]) in sums
  assert any(line.split()[:3]==['dep',*key] and line.split()[3]==h1 for line in build.splitlines())
  modules[key]={'module':key[0],'version':key[1],'go_sum':h1,'origin_metadata':origin,'notice_files':[]}
 with zipfile.ZipFile(cache/'cache/download'/key[0]/'@v'/(key[1]+'.zip')) as z:
  for selection in ['GoFiles','CgoFiles','CFiles','CXXFiles','MFiles','HFiles','SFiles','SysoFiles','EmbedFiles']:
   for filename in p.get(selection,[]):
    source=Path(p['Dir'])/filename;rel=str(source.relative_to(mroot));identity=(*key,rel)
    if identity in seen:continue
    seen.add(identity);raw=source.read_bytes();original=z.read(prefix+rel);assert raw==original
    record={'module':key[0],'version':key[1],'package':p['ImportPath'],'selection':selection,'path':rel,'bytes':len(raw),'sha256':sha(raw),'identical_to_module_zip':True}
    if source.suffix=='.go':
     text=raw.decode();record['generated']=text.startswith('// Code generated') or text.startswith('//code generated')
     if 'SQLITE_SOURCE_ID' in text:
      m=re.search(r'const SQLITE_SOURCE_ID = "([^"]+)"',text)
      if m:record['sqlite_source_id']=m.group(1)
      m=re.search(r'const SQLITE_VERSION = "([^"]+)"',text)
      if m:record['sqlite_version']=m.group(1)
    records.append(record)
  if not modules[key]['notice_files']:
   notices=sorted(q for q in mroot.iterdir() if q.is_file() and (q.name.lower().startswith(('license','licence','copying','notice')) or q.name=='SQLITE-LICENSE'))
   for source in notices:
    raw=source.read_bytes();assert raw==z.read(prefix+source.name);modules[key]['notice_files'].append({'path':source.name,'bytes':len(raw),'sha256':sha(raw),'identical_to_module_zip':True})
assert packages and len(modules)==4 and records
result={'scope':f"{env['GOOS']}/{env['GOARCH']} {env['GOVERSION']} dependency-selected modernc files; module ZIP bytes and pinned distribution h1; not original C/header regeneration or legal adequacy",'package_count':len(packages),'selected_files':len(records),'selected_bytes':sum(r['bytes'] for r in records),'modules':list(modules.values()),'files':records,'go_sum_file_sha256':sha(Path('go.sum').read_bytes()),'legal_review':'pending'}
(root/'translated-source-audit.json').write_text(json.dumps(result,indent=2)+'\n');print('Selected packages/files/bytes:',len(packages),len(records),result['selected_bytes'],'module notices',sum(len(m['notice_files']) for m in modules.values()))
