#!/usr/bin/env python3
"""Prepare source-only rule census and annotation review drafts. No scoring/runtime."""
import argparse
import collections
import hashlib
import json
import re
from pathlib import Path
import subprocess
import tempfile

ROOT=Path(__file__).resolve().parents[1]
SELECTED=[('GetMaxMinterAllowance','existing_policy'),('GetMintProposalAmount','existing_policy'),('initializeValidator','existing_policy'),('CalculateDynamicSlot','existing_policy'),('GetMinterAt','security_proposal'),('GetBLSPublicKey','security_proposal'),('GetRefundableBalance','security_proposal'),('VarLenBytesToMultipleHash','security_proposal'),('GetMinterCount','no_direct_overlay_reference'),('HashToAddress','no_direct_overlay_reference'),('IncrementHash','no_direct_overlay_reference'),('checkSystemContractVersions','no_direct_overlay_reference')]
def sha(raw):return hashlib.sha256(raw).hexdigest()
def encode(value):return json.dumps(value,ensure_ascii=False,indent=2)+'\n'
def source_files(repo,revision):
 rows=[]
 for raw in subprocess.check_output(['git','-C',str(repo),'ls-tree','-rz',revision]).split(b'\0'):
  if not raw:continue
  meta,path=raw.split(b'\t',1);path=path.decode();parts=meta.split()
  if parts[1]==b'blob' and path.endswith('.go') and not path.endswith('_test.go'):
   rows.append({'Path':path,'Blob':parts[2].decode()})
 payload=('\n'.join(x['Blob'] for x in rows)+'\n').encode()
 data=subprocess.run(['git','-C',str(repo),'cat-file','--batch'],input=payload,capture_output=True,check=True).stdout
 offset=0
 for row in rows:
  end=data.index(b'\n',offset);oid,kind,size=data[offset:end].split();assert oid.decode()==row['Blob'] and kind==b'blob'
  start=end+1;raw=data[start:start+int(size)];assert data[start+int(size):start+int(size)+1]==b'\n';offset=start+int(size)+1
  row['Source']=raw.decode()
 assert offset==len(data)
 return rows

def hints(call):
 args=call['arguments'];out=[]
 if call['import_path_spelling']=='os/exec':
  index=1 if call['callee']=='CommandContext' else 0
  if len(args)>index and args[index]['kind']=='identifier' and args[index]['text'] in call['named_parameters']:
   out.append('parameter_command_spelling_hint_not_dataflow')
 else:
  lhs=call.get('assignment_lhs') or []
  if len(lhs)==2 and lhs[1]=='_' and lhs[0] in (call.get('explicit_return_identifier_spellings') or []):
   out.append('discarded_error_and_return_spelling_hint_not_dataflow')
 return out

def main():
 parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--repo',type=Path,required=True);parser.add_argument('--out',type=Path,required=True);args=parser.parse_args()
 assert not args.out.exists(),'fresh output required';args.out.mkdir(parents=True)
 public_dir=args.out/'public';private_dir=args.out/'private';public_dir.mkdir();private_dir.mkdir();(private_dir/'calls').mkdir()
 revision=json.loads((ROOT/'docs/spec-driven/OPERATIONAL-ACCEPTANCE-SOURCE-INVENTORY.json').read_text())['repositories']['go_stablenet']['candidate_source_commit']
 prior_path=ROOT/'system/eval/stablenet-corpus-repair-2026-10-14/questions-frozen.json'
 prior=json.loads(prior_path.read_text());prior_text='\n'.join(q['prompt'] for q in prior['questions'])
 prior_functions=json.loads((ROOT/'docs/spec-driven/OPERATIONAL-ACCEPTANCE-INPUT-CANDIDATES.json').read_text())['functions']
 files=source_files(args.repo,revision);helper=ROOT/'scripts/acceptance_source_inventory.go.txt'
 with tempfile.TemporaryDirectory(prefix='ks-source-census-',dir='/private/tmp') as tmp:
  path=Path(tmp)/'main.go';path.write_bytes(helper.read_bytes())
  p=subprocess.run(['go','run',str(path)],input=json.dumps(files).encode(),capture_output=True,timeout=120,check=True)
 parsed=json.loads(p.stdout);by_path={r['Path']:r for r in files}
 # YAML parsed only for this preparation command; verification uses frozen bytes/records.
 import yaml
 policy_path=ROOT/'projects/stablenet/policies/graph.yaml';policy_raw=policy_path.read_bytes();policy=yaml.safe_load(policy_raw)['policies']
 grouped=collections.defaultdict(list)
 for symbol in parsed['symbols']:grouped[symbol['source_name_hint']].append(symbol)
 policy_refs=[]
 for row in policy:
  for target in row.get('governs',[]):
   matches=grouped.get(target.strip(),[])
   policy_refs.append({'policy_id':row['id'],'target_spelling':target,'source_matches':matches,'classification':'unique_source_name_hint' if len(matches)==1 else 'ambiguous_source_name_hint' if matches else 'no_non_generated_source_name_hint','graph_resolution':'not_run'})
 for i,call in enumerate(parsed['calls'],1):
  call.update(id=f'CALL-{i:03d}',syntax_hints=hints(call),gold_status='not_authored',independent_fact_family=None)
  raw=by_path[call['path']]['Source'].encode();excerpt=raw[call['function_start_byte']:call['function_end_byte_exclusive']]
  call['function_sha256']=sha(excerpt);call['source_excerpt_private']='calls/'+call['id']+'-source.txt'
  (private_dir/call['source_excerpt_private']).write_bytes(excerpt)
 public=[];private=[];security=[];extracts=[]
 for index,(name,group) in enumerate(SELECTED,1):
  matches=[r for r in grouped['systemcontracts.'+name] if r['kind'] in ('function','method')]
  assert len(matches)==1,(name,len(matches));r=matches[0];
  assert not re.search(r'\b'+re.escape(name)+r'\b',prior_text),('prior literal overlap',name)
  assert not any(x['path']==r['path'] and x['start_byte']==r['start_byte'] for x in prior_functions),('prior source pool overlap',name)
  raw=by_path[r['path']]['Source'].encode();excerpt=raw[r['start_byte']:r['end_byte_exclusive']];identity=f'SPC-{index:03d}'
  excerpt_path=public_dir/(identity+'-source.txt');excerpt_path.write_bytes(excerpt);extracts.append(excerpt_path)
  public.append({'id':identity,'repository_commit':revision,'source':r,'source_excerpt':excerpt_path.name,'prompt':'이 source symbol을 정확히 식별한 뒤 retained policy/security annotation과 provenance·unknown을 조회한다. 빈 결과를 안전 판정으로 해석하지 않는다.','split':None,'scoring_allowed':False})
  attached=[x['id'] for x in policy if r['source_name_hint'] in x.get('governs',[])]
  if group=='existing_policy':assert attached,(name,group)
  if group=='no_direct_overlay_reference':assert not attached,(name,attached)
  if group=='security_proposal':
   entry={'id':identity.lower()+'-manual-review','name':name+' manual review proposal','category':'evaluation-proposal','severity':'low','description':'운영자가 검토할 수 있도록 제안한 메타데이터. 확인된 취약점이나 독립 판정이 아니다.','remediation':'원문·호출자 가정을 독립 검토한 뒤 수정 필요 여부를 판단한다.','matches':[r['source_name_hint']]};security.append(entry)
  private.append({'id':identity,'group_proposal':group,'expected_policy_ids_from_exact_overlay_spelling':attached,'security_entry_proposal':security[-1] if group=='security_proposal' else None,'expected_provenance_unknown':['source_sha256','source_line','version','reviewer','reviewed_at','approval'],'safety_verdict':'unknown','approved':False,'independent_review':'pending','gold':False,'split':None,'prior_literal_overlap':False,'prior_source_pool_overlap':False,'semantic_family_overlap':'not_reviewed','limits':'source-name hints are not graph/canonical resolution; untagged is relative to chosen overlay and never a safety claim'})
 census={'schema_version':1,'kind':'source_only_call_syntax_not_rule_detection_or_gold','source_commit':revision,'non_test_go_files':len(files),'generated_files_skipped':parsed['generated_files_skipped'],'non_generated_symbol_count':len(parsed['symbols']),'calls':parsed['calls'],'call_counts':dict(collections.Counter(r['import_path_spelling']+'.'+r['callee'] for r in parsed['calls'])),'syntax_hint_counts':dict(collections.Counter(x for r in parsed['calls'] for x in r['syntax_hints'])),'limitations':['all tracked non-test Go files regardless of build tags/platform; includes build/example/support code','qualified call spelling can be shadowed; package/type binding unresolved','identifier and return spellings do not prove reaching definitions or supported detector results','generated files and function-external initializers are not covered; missing candidates do not prove absence','no vulnerable/positive/negative labels, no independent families, no quality denominator']}
 (public_dir/'inputs.json').write_text(encode({'schema_version':1,'kind':'unapproved_public_source_queries_no_gold','cases':public,'runtime_executed':False}))
 (private_dir/'review-drafts.json').write_text(encode({'schema_version':1,'kind':'ai_operator_annotation_proposal_not_independent_gold','cases':private,'security_overlay_proposal':{'security_patterns':security},'approved':False,'gold_records':0,'cases_promoted':0,'existing_overlay_reference':{'path':'projects/stablenet/policies/graph.yaml','repository_commit':subprocess.check_output(['git','rev-parse','HEAD'],cwd=ROOT,text=True).strip(),'sha256':sha(policy_raw)},'policy_reference_inventory':policy_refs,'yaml_parser':{'name':'PyYAML','version':yaml.__version__},'source_matching_is_graph_resolution':False}))
 (private_dir/'rule-source-census.json').write_text(encode(census));(private_dir/'existing-policy.yaml.txt').write_bytes(policy_raw)
 manifest={'schema_version':1,'kind':'unapproved_source_preparation_not_gold_or_runtime','source_commit':revision,'helper':{'path':str(helper.relative_to(ROOT)),'sha256':sha(helper.read_bytes())},'generator':{'path':str(Path(__file__).relative_to(ROOT)),'sha256':sha(Path(__file__).read_bytes())},'files':[{'path':r['Path'],'git_blob':r['Blob'],'sha256':sha(r['Source'].encode())} for r in files],'symbols_selected':12,'policy_rows':len(policy),'policy_target_rows':len(policy_refs),'policy_target_classifications':dict(collections.Counter(r['classification'] for r in policy_refs)),'go_version':subprocess.check_output(['go','version'],text=True).strip(),'prior_literal_filter':{'path':str(prior_path.relative_to(ROOT)),'sha256':sha(prior_path.read_bytes()),'function_name_filter_only_not_semantic_independence':True,'excluded_initial_draft':['systemcontracts.splitAndTrim']},'public_directory':'public','private_directory':'private','approved_cases':0,'gold_records':0,'runtime_executed':False,'artifacts':[]}
 manifest['artifacts']=[{'path':p.relative_to(args.out).as_posix(),'sha256':sha(p.read_bytes()),'bytes':p.stat().st_size} for p in sorted(args.out.rglob('*')) if p.is_file()]
 (args.out/'manifest.json').write_text(encode(manifest))
 print(encode({'source_files':len(files),'policy_rows':len(policy),'policy_target_classifications':manifest['policy_target_classifications'],'call_counts':census['call_counts'],'syntax_hints':census['syntax_hint_counts'],'SP_source_queries':12,'approved_cases':0,'gold_records':0}))
if __name__=='__main__':main()
