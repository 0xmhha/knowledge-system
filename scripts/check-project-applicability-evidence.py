"""Source-bound project source bindings checkpoint; semantic review and MCP remain separate."""
import json
import re
from pathlib import Path
from knowledge_evidence import load,raw,require,verify_packet,current_go
ROOT=Path(__file__).resolve().parents[1]
PACKET=ROOT/'system/eval/knowledge-roadmap-2026-10-16/km-05-applicability'


def check(folder=PACKET,root=ROOT):
    try:
        m,b=verify_packet(folder,root)
        require(m['checkpoint']=='KM05_4_scoped_applicability_and_reader_capabilities'and m['full_KM05_complete']is True and m['applicability_runtime_implemented']is True and m['project_bindings_implemented']is True and m['mapping_review_runtime_implemented']is True and m['human_mapping_approval']is False and m['comparison_MCP_implemented']is False and m['quality_acceptance']is False and m['independent_acceptance']is False,'scope promoted')
        v=load(folder,'validation.json');roadmap=current_go(root,folder,v,b,race_packages=7)
        require(all(code==0 for code in v['gates'].values())and v['maintained_Go_files']==1266,'gate or source inventory changed')
        for label in('module','race'):
            rows=[json.loads(l)for l in raw(folder,label+'.jsonl').splitlines()]
            cases=[r for r in rows if r.get('Action')=='pass'and r.get('Test')and r.get('Package','').endswith('/internal/system/comparison')]
            code='\n'.join((root/('internal/system/comparison/'+name)).read_text()for name in('scheme_test.go','bindings_test.go','mapping_policy_test.go','coverage_test.go'));top=set(re.findall(r'^func (Test\w+)\(',code,re.M))
            require(len(cases)==117 and {r['Test']for r in cases if '/'not in r['Test']}==top and len(top)==30,'scheme test denominator changed')
        pack=json.loads((root/'system/packs/industry-blockchain/pack.yaml').read_text());side=json.loads((root/'system/packs/industry-blockchain/comparison.json').read_text())
        require(pack['pack_id']==side['pack_id']=='industry.blockchain'and pack['version']==side['version']=='1.0.0'and len(pack['concepts'])==len(side['concepts'])==8 and sum(len(c['facets'])for c in side['concepts'])==26,'vocabulary inventory changed')
        require(load(folder,'review-scope.json')['actual_human_reviews_created']==0 and load(folder,'review-scope.json')['reviewer_identity_authenticated']is False,'human identity/approval invented')
        scope=load(folder,'coverage-scope.json');require(scope['unsupported_scope']=='comparison_reader_capability'and scope['not_collected_scope']=='selected_mapping_collection'and scope['actual_graph_vector_indexing_verified']is False,'scope changed to project feature absence or actual indexing')
        require('0 issues.'in raw(folder,'build-lint.txt').decode()and 'documentation: OK (133 live documents)'in raw(folder,'docs-final.txt').decode(),'lint/docs failure')
        history=load(folder,'historical-design.json');require(history['revision'].startswith('f2eef29d')and history['verification_exit_code']==0 and all(c['exit_code']==0 for c in history['checks']),'prior mapping review checkpoint failed')
        progress=json.loads((root/'docs/spec-driven/KNOWLEDGE-ROADMAP-PROGRESS.json').read_text());item=next(i for i in progress['items']if i['id']=='KM-05')
        require(progress['completed_subtasks']==20 and progress['implementation_completed']==5 and item['status']=='complete'and[s['status']for s in item['subtasks']]==['complete','complete','complete','complete']and item['independent_acceptance']=='not_recorded'and not roadmap.check(root=root),'mapping/comparison promoted or ledger inconsistent')
    except(OSError,ValueError,KeyError,TypeError,IndexError,StopIteration)as error:return[str(error)]
    return []


if __name__=='__main__':
    errors=check()
    for e in errors:print('FAIL:',e)
    print('project applicability:', 'FAIL'if errors else'PASS; common mapping library complete; public comparison and effect evaluation remain')
    raise SystemExit(bool(errors))
