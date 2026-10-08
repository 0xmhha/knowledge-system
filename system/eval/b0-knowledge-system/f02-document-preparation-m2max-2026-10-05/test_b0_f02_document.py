"""Failure oracles for parent/split/tail and CKV snippet projections."""
import copy
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

SPEC=importlib.util.spec_from_file_location('f02_audit',Path(__file__).with_name('b0-audit-f02-document.py'))
M=importlib.util.module_from_spec(SPEC);SPEC.loader.exec_module(M)


class F02AuditTest(unittest.TestCase):
    def setUp(self):
        book=json.loads((M.ROOT/'system/eval/b0-knowledge-system/dynamic-fixtures-m2max-draft.json').read_bytes())
        self.fixture=next(f for f in book['fixtures'] if f['id']=='F-02-DEV')
        self.file='docs/decision.md';self.source=self.fixture['sources'][self.file].encode();self.commit='c'*40
        lines=self.source.splitlines(keepends=True);self.children=[]
        parent=M.chunk_id(self.file,1,203,M.BASE.sha(self.source))
        for ordinal,(start,end) in enumerate([(1,2),(3,62),(63,122),(123,182),(183,203)],1):
            raw=b''.join(lines[start-1:end]);digest=M.BASE.sha(raw)
            self.children.append({'id':M.chunk_id(self.file,start,end,digest),'file':self.file,'start_line':start,'end_line':end,
                'commit_hash':self.commit,'language':'markdown','chunk_kind':'doc','text':raw.decode(),
                'content_sha256':digest,'parent_id':parent,'parent_start_line':1,'parent_end_line':203,
                'part_ordinal':ordinal,'heading_path':['Refund decision']})

    def structure(self):
        return M.audit_children(self.children,self.fixture,self.source,self.commit,6144)

    def query(self,density):
        return {'metadata':{'indexed_head_ckv':self.commit,'fresh':True},'hits':[
            {'chunk_id':c['id'],'citation':{'file':self.file,'start_line':c['start_line'],'end_line':c['end_line'],'commit_hash':self.commit},
             'parent_citation':{'file':self.file,'start_line':1,'end_line':203,'commit_hash':self.commit},
             'heading_path':'Refund decision','snippet':M.density_view(c['text'],density),'density':density}
            for c in self.children]}

    def test_lossless_children_and_final_tail_parent_identity(self):
        r=self.structure()
        self.assertEqual(r['child_count'],5);self.assertTrue(r['child_bytes_reassemble_source'])
        self.assertEqual(r['source_bytes'],20290);self.assertEqual(r['parent_span'],[1,203])
        self.assertEqual(r['tail_child_ids'],[self.children[-1]['id']])

    def test_dropped_reordered_duplicate_and_gapped_children_refused(self):
        original=copy.deepcopy(self.children)
        for mutate in [lambda c:c.pop(),lambda c:c[1].update(part_ordinal=1),
                       lambda c:c[1].update(start_line=4),lambda c:c[3].update(part_ordinal=5)]:
            self.children=copy.deepcopy(original);mutate(self.children)
            with self.assertRaises(ValueError):self.structure()

    def test_forged_text_hash_chunk_id_parent_and_heading_refused(self):
        original=copy.deepcopy(self.children)
        for field,value in [('text','fabricated'),('content_sha256','a'*64),('id','a'*64),('parent_id','a'*64),
                            ('parent_end_line',202),('heading_path',[]),('commit_hash','a'*40),('file','other.md')]:
            self.children=copy.deepcopy(original);self.children[2][field]=value
            with self.subTest(field=field),self.assertRaises(ValueError):self.structure()

    def test_source_or_final_partition_substitution_refused(self):
        with self.assertRaises(ValueError):M.audit_children(self.children,self.fixture,self.source+b'\n',self.commit,6144)
        self.fixture['id']='F-02-FINAL';self.fixture['evaluation_partition']='final'
        with self.assertRaisesRegex(ValueError,'FINAL'):self.structure()

    def test_monolithic_parent_and_oversized_children_are_not_split_success(self):
        self.children=self.children[:1]
        with self.assertRaisesRegex(ValueError,'not split'):self.structure()
        self.setUp()
        with self.assertRaises(ValueError):M.audit_children(self.children,self.fixture,self.source,self.commit,500)

    def test_density_can_drop_tail_text_while_preserving_its_citation(self):
        structure=self.structure()
        for density,keeps in [('full',True),('signature+N',False),('signature_only',False)]:
            report=M.audit_query(json.dumps(self.query(density)).encode(),'diagnostic',self.children,structure,self.commit)
            self.assertEqual(report['tail_citation_ranks'],[5])
            self.assertEqual(report['tail_snippet_observations'][0]['snippet_keeps_final_line'],keeps)

    def test_query_forged_parent_stale_commit_heading_or_body_refused(self):
        structure=self.structure();original=self.query('full')
        for mutate in [lambda q:q['hits'][0]['parent_citation'].update(end_line=202),
                       lambda q:q['hits'][0]['citation'].update(commit_hash='a'*40),
                       lambda q:q['hits'][0].update(heading_path='foreign'),
                       lambda q:q['hits'][0].update(snippet='forged'),
                       lambda q:q['hits'][0].update(stale_citation=True),
                       lambda q:q['metadata'].update(fresh=False)]:
            query=copy.deepcopy(original);mutate(query)
            with self.assertRaises(ValueError):M.audit_query(json.dumps(query).encode(),'diagnostic',self.children,structure,self.commit)

    def test_signature_context_preserves_public_skip_blank_rule(self):
        text='first\n\nsecond\nthird\nfourth\nfifth\nsixth\nseventh\n'
        self.assertEqual(M.density_view(text,'signature+N'),'first\nsecond\nthird\nfourth\nfifth\nsixth')
        self.assertEqual(M.density_view('\n\nfirst\n','signature_only'),'first')
        with self.assertRaises(ValueError):M.density_view(text,'unknown')

    def sdk_inputs(self,source_root):
        path=source_root/self.file;path.parent.mkdir(parents=True);path.write_bytes(self.source)
        requests={'requests':[{'id':str(i),'tool':'cks.context.get_for_task_v2','arguments':{'prompt':q['prompt']}} for i,q in enumerate(self.fixture['queries'])]}
        config=b'synthetic-fixture-only\n'
        manifest={'src_commit':self.commit,'project_id':'fixture','dataset_id':'d'*64,'snapshot_id':'a'*64,'source_mode':'committed'}
        coords={'project_id':'fixture','dataset_id':'d'*64,'snapshot_id':'a'*64,'source_mode':'committed','base_commit':self.commit}
        citation=dict(coords,file=self.file,start_line=203,end_line=203,commit_hash=self.commit,
                      file_sha256=M.BASE.sha(self.source),content_sha256=M.BASE.sha(self.source.splitlines(keepends=True)[202]))
        rows=[]
        for req in requests['requests']:
            pack={'format_version':2,'coordinates':coords,'query':req['arguments']['prompt'],'semantic':None,
                  'citations':[citation],'bodies':[{'citation':citation,'text':self.source.splitlines(keepends=True)[202].decode()}],
                  'metadata':{'integrity_hash_algo':'sha256-v2'}}
            pack['metadata']['integrity_hash']=M.BASE.sha(json.dumps(pack,sort_keys=True,ensure_ascii=False,separators=(',',':')).encode())
            rows.append({'request_id':req['id'],'phase':'retrieval','iteration':1,'call':{'measurement_id':'mid-'+req['id'],
                'tool':req['tool'],'arguments':req['arguments'],'response':{'structuredContent':pack}}})
        request_raw=json.dumps(requests,ensure_ascii=False).encode()
        capture={'state':'captured','quality_metrics':None,'request_sha256':M.BASE.sha(request_raw),'config_sha256':M.BASE.sha(config),
                 'config_sha256_after':M.BASE.sha(config),'binary_sha256':'b'*64,'binary_sha256_after':'b'*64,
                 'counts':{'retrieval':1,'warmup':0,'warm_latency':0,'cold_process':0},'rows':rows}
        return capture,request_raw,config,manifest

    def test_sdk_source_bound_tail_and_failed_missing_denominators(self):
        with tempfile.TemporaryDirectory() as temp:
            root=Path(temp);c,r,y,m=self.sdk_inputs(root)
            result=M.audit_sdk(json.dumps(c,ensure_ascii=False).encode(),r,y,self.fixture,m,root)
            self.assertEqual(result['planned_calls'],2);self.assertEqual(result['rows'][0]['tail_body_count'],1)
            c['rows'][0]['call']['transport_error']='injected';c['rows'].pop()
            result=M.audit_sdk(json.dumps(c,ensure_ascii=False).encode(),r,y,self.fixture,m,root)
            self.assertEqual(result['planned_calls'],2);self.assertEqual(result['failed_calls'],1);self.assertEqual(result['missing_calls'],1)

    def test_sdk_wrong_slot_mid_args_hash_and_foreign_source_refused(self):
        with tempfile.TemporaryDirectory() as temp:
            root=Path(temp);original,r,y,m=self.sdk_inputs(root)
            for mutate in [lambda c:c.update(config_sha256_after='a'*64),
                           lambda c:c['rows'][1]['call'].update(measurement_id='mid-0'),
                           lambda c:c['rows'][0]['call']['arguments'].update(prompt='different'),
                           lambda c:c['rows'][0].update(iteration=2),
                           lambda c:c['rows'][0]['call']['response']['structuredContent']['coordinates'].update(project_id='foreign')]:
                capture=copy.deepcopy(original);mutate(capture)
                with self.assertRaises(ValueError):M.audit_sdk(json.dumps(capture,ensure_ascii=False).encode(),r,y,self.fixture,m,root)

    def test_raw_failure_is_preserved_without_tail_or_exact_scores(self):
        probe={'state':'error','error':'injected failure','prompt':self.fixture['queries'][0]['prompt'],'k':10,'filter':{}}
        result=M.audit_raw_probe(json.dumps(probe).encode(),b'{}',self.fixture,None,None,None)
        self.assertEqual(result['state'],'measurement_failed');self.assertIsNone(result['tail_raw_ranks'])


if __name__=='__main__':unittest.main()
