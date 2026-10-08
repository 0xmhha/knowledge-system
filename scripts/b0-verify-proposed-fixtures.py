#!/usr/bin/env python3
"""Verify unreviewed fixture layers never become normative MCP facts.

Diagnostic development requests only. No gold answers enter retrieval, and
no quality or latency gate is scored. Captures and their original errors are
preserved in a new output directory.
"""
import argparse
import hashlib
import json
from pathlib import Path
import subprocess

ROOT = Path(__file__).resolve().parent.parent

def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--review-dir',required=True,type=Path)
    parser.add_argument('--out',required=True,type=Path)
    args=parser.parse_args()
    src=args.review_dir.resolve();out=args.out.resolve();out.mkdir(parents=True,exist_ok=False)
    review=json.loads((src/'review-manifest.json').read_bytes())
    book_raw=(ROOT/'system/eval/b0-knowledge-system/dynamic-fixtures-m2max-draft.json').read_bytes()
    assert hashlib.sha256(book_raw).hexdigest()==review['input_manifest_sha256']
    book=json.loads(book_raw)
    assert review['partition']=='development' and review['diagnostic_only'] and review['quality_metrics'] is None
    binary=ROOT/'bin/cks';assert hashlib.sha256(binary.read_bytes()).hexdigest()==review['binary_sha256']
    checks=[];observations=[]
    def run(label,command):
        result=subprocess.run([str(a)for a in command],cwd=ROOT,capture_output=True,timeout=300)
        path=out/(label+'.txt');path.write_bytes(result.stdout+result.stderr)
        checks.append({'id':label,'command':[str(a)for a in command],'exit_code':result.returncode,'raw_output':path.name,'sha256':hashlib.sha256(path.read_bytes()).hexdigest()})
        (out/'checks.json').write_text(json.dumps(checks,indent=2)+'\n')
        if result.returncode:raise RuntimeError('capture failed; see '+str(path))
    rows=0
    for case in review['cases']:
        assert case['family'] in ['F-03','F-05','F-06']
        assert case['fixture_id']==case['family']+'-DEV'
        assert case['state'] in (['a','b'] if case['family']=='F-05' else ['current'])
        assert case['id']==case['fixture_id']+'-'+case['state']
        label=case['id'];source=next(f for f in book['fixtures']if f['id']==case['fixture_id'])
        state=next(s for f in json.loads((src/'fixtures/materialization.json').read_text())['fixtures']if f['id']==case['fixture_id']for s in f['states']if s['name']==case['state'])
        repo=src/'fixtures'/state['repository'];version=(src/label/'dataset/current').resolve()
        model=json.loads((version/'vector/manifest.json').read_text())['embedding_model']
        store=src/label/'semantic-proposed.db';store_hash=hashlib.sha256(store.read_bytes()).hexdigest()
        packs={}
        for arm in ['baseline','combined']:
            config=out/(label+'-'+arm+'.yaml')
            run(label+'-'+arm+'-config',[binary,'mcp','gen-config','--dataset-dir',version,'--source-root',repo,'--embed-model',model,'--semantic-store',src/label/'semantic-proposed.db','--out',config])
            lines=[]
            for line in config.read_text().splitlines():
                line=line.replace('provider: ""','provider: '+review['embedder']).replace('provider: ollama','provider: '+review['embedder'])
                line=line.replace('mcp_stdio: false','mcp_stdio: true').replace('transport: http','transport: stdio')
                lines.append(line)
                if line.lstrip().startswith('store_path:'):lines.extend(['    ontology_mode: '+arm,'    ontology_budget_ms: 5000'])
            config.write_text('\n'.join(lines)+'\n')
            requests=out/(label+'-'+arm+'-requests.json')
            request_items=[]
            for i,q in enumerate(source['queries']):
                arguments={'prompt':q['prompt']}
                if arm=='combined':arguments.update(include_knowledge=True,knowledge_as_of='2026-10-03',knowledge_subsystem='refund')
                request_items.append({'id':'q'+str(i),'tool':'cks.context.get_for_task_v2','arguments':arguments})
            requests.write_text(json.dumps({'schema_version':1,'requests':request_items},ensure_ascii=False,indent=2)+'\n')
            capture=out/(label+'-'+arm+'.json')
            run(label+'-'+arm+'-capture',[binary,'eval','capture','--requests',requests,'--config',config,'--output',capture,'--warmup','0','--retrieval-runs','1','--warm-runs','0','--cold-runs','0'])
            report=json.loads(capture.read_bytes());assert report['state']=='captured'
            assert report['binary_sha256']==report['binary_sha256_after']==review['binary_sha256']
            assert report['config_sha256']==report['config_sha256_after']==hashlib.sha256(config.read_bytes()).hexdigest()
            packs[arm]={}
            for row in report['rows']:
                rows+=1;response=row['call']['response'];assert not response.get('isError',False)
                pack=response.get('structuredContent') or json.loads(response['content'][0]['text'])
                assert pack['coordinates']['dataset_id']==case['dataset']['dataset_id']
                clone=json.loads(json.dumps(pack));want=clone['metadata'].pop('integrity_hash')
                assert hashlib.sha256(json.dumps(clone,sort_keys=True,ensure_ascii=False,separators=(',',':')).encode()).hexdigest()==want
                packs[arm][row['request_id']]=pack
                if arm=='combined':
                    diag=pack['metadata']['ontology']
                    assert diag['state'] in ['active','no_match','no_candidates'],diag
                    assert diag['applied_relations']==0 and diag['boosted_citations']==0 and diag.get('text_search_calls',0)==0,diag
                    if case['family']=='F-03':
                        expected_matches = 0 if row['request_id']=='q2' or diag['state']=='no_candidates' else 2
                        assert diag['matched_concepts']==expected_matches,diag
                    semantic=pack.get('semantic')
                    if case['family']!='F-03':
                        context=semantic['knowledge_context']
                        for field in ['applicable_policies','decisions','relations','constraints','related_requirements','test_links','trace_links']:
                            assert context.get(field,[])==[],context
                        coding=semantic['coding_context']
                        for field in ['implemented_behavior','required_behavior','rationale','constraints']:
                            assert coding.get(field,[])==[],coding
                        assert context['state']=='unknown',context
                    observations.append({'case':label,'request_id':row['request_id'],'ontology':diag,'semantic':semantic})
        assert hashlib.sha256(store.read_bytes()).hexdigest()==store_hash,'semantic store mutated on read'
        for request_id in packs['baseline']:
            for key in ['citations','bodies']:assert packs['baseline'][request_id][key]==packs['combined'][request_id][key],(label,request_id,key)
    summary={'state':'verified_proposed_not_normative','quality_metrics':None,'partition':'development','requests':rows,'observations':observations,'checks':checks,'review_manifest_sha256':hashlib.sha256((src/'review-manifest.json').read_bytes()).hexdigest()}
    (out/'summary.json').write_text(json.dumps(summary,ensure_ascii=False,indent=2)+'\n')
    print(json.dumps({'state':summary['state'],'requests':rows,'quality_metrics':None}))

if __name__=='__main__':main()
