import copy
import importlib.util
import json
from pathlib import Path
import unittest

SPEC=importlib.util.spec_from_file_location('state_reader',Path(__file__).with_name('b0-audit-state-isolation.py'))
MOD=importlib.util.module_from_spec(SPEC);SPEC.loader.exec_module(MOD)
ROOT=Path(__file__).resolve().parent.parent/'system/eval/b0-knowledge-system/state-isolation-preparation-m2max-2026-10-05'


def raw(x):return json.dumps(x,sort_keys=True,ensure_ascii=False,separators=(',',':')).encode()


class IsolationTests(unittest.TestCase):
    def setUp(self):
        self.bundle=json.loads((ROOT/'bundle.json').read_bytes());self.case=self.bundle['cases'][2]
        self.book=json.loads((ROOT/self.bundle['fixture_book']).read_bytes())
        self.fixture=next(f for f in self.book['fixtures'] if f['id']=='F-05-DEV')
        c=self.case
        self.identity=json.loads((ROOT/c['identity']).read_bytes());self.manifest=json.loads((ROOT/c['manifest']).read_bytes())
        self.sources={k:(ROOT/v).read_bytes() for k,v in c['sources'].items()}
        self.cap=json.loads((ROOT/c['fixed_capture']).read_bytes());self.requests=(ROOT/c['requests']).read_bytes()
        self.config=(ROOT/c['config']).read_bytes();self.backend=(ROOT/c['backend']).read_bytes()
        self.pack=self.cap['rows'][0]['call']['response']['structuredContent']

    def seal_pack(self):
        self.pack['metadata'].pop('integrity_hash',None)
        self.pack['metadata']['integrity_hash']=MOD.BASE.sha(raw(self.pack))

    def audit(self):
        return MOD.audit_case(self.case['id'],self.fixture,self.case['state'],self.identity,self.manifest,
            self.sources,raw(self.cap),self.requests,self.config,self.backend,(ROOT/self.case['sources']['main.go']).parent)

    def test_real_retained_capture(self):
        r=self.audit();self.assertEqual(r['expected_source_body_coverage'],[True,True])
        self.assertFalse(r['reviewed_policy_pack_activated']);self.assertIsNone(r['quality_metrics'])

    def test_foreign_coordinates_with_valid_outer_hash(self):
        self.pack['coordinates']['project_id']='f-05-dev-b';self.seal_pack()
        with self.assertRaisesRegex(ValueError,'foreign evidence'):self.audit()

    def test_foreign_citation_with_valid_outer_hash(self):
        self.pack['citations'][0]['base_commit']='a'*40;self.seal_pack()
        with self.assertRaisesRegex(ValueError,'foreign citation'):self.audit()

    def test_foreign_body_with_valid_outer_hash(self):
        self.pack['bodies'][0]['text']='DEV-project-b';self.seal_pack()
        with self.assertRaisesRegex(ValueError,'body hash'):self.audit()

    def test_missing_coverage_is_not_pass(self):
        self.pack['bodies']=[];self.seal_pack()
        r=self.audit();self.assertEqual(r['expected_source_body_coverage'],[False,False])

    def test_missing_calls_preserve_denominator(self):
        self.cap['rows']=[];self.cap['state']='partial'
        r=self.audit();self.assertEqual((r['planned_calls'],r['observed_calls'],r['missing_calls']),(1,0,1))

    def test_tool_error_is_failed_call(self):
        self.cap['rows'][0]['call']['response']={'isError':True,'structuredContent':{'code':'snapshot_mismatch'}}
        r=self.audit();self.assertEqual(r['failed_calls'],1);self.assertEqual(r['state'],'measurement_failed')

    def test_final_fixture_refused(self):
        self.fixture['evaluation_partition']='final'
        with self.assertRaisesRegex(ValueError,'DEV'):self.audit()

    def test_gold_arguments_refused(self):
        requests=json.loads(self.requests);requests['requests'][0]['arguments']['gold']='limit 10'
        self.requests=raw(requests);self.cap['request_sha256']=MOD.BASE.sha(self.requests)
        with self.assertRaisesRegex(ValueError,'arguments'):self.audit()

    def test_source_and_config_mutation(self):
        self.sources['main.go']+=b'// changed\n'
        with self.assertRaisesRegex(ValueError,'source bytes'):self.audit()
        self.sources['main.go']=self.sources['main.go'].removesuffix(b'// changed\n')
        self.config+=b'# changed\n'
        with self.assertRaisesRegex(ValueError,'binding'):self.audit()

    def test_wrong_actual_k_refused(self):
        events=[json.loads(line) for line in self.backend.splitlines()]
        mid=self.cap['rows'][0]['call']['measurement_id']
        for e in events:
            if e.get('event')=='measurement.backend_calls' and e['summary']['measurement_id']==mid:
                for c in e['summary']['calls']:
                    if c['backend']=='ckv' and c['method']=='semantic_search':c['options']['K']=20
        self.backend=b'\n'.join(raw(e) for e in events)
        with self.assertRaisesRegex(ValueError,'retrieval K'):self.audit()

    def test_stale_query_cannot_report_fresh(self):
        c=self.bundle['cases'][0];q=json.loads((ROOT/self.bundle['stale_fixed_query']).read_bytes())
        chunks=json.loads((ROOT/c['chunks']).read_bytes());q['metadata']['fresh']=True
        with self.assertRaisesRegex(ValueError,'freshness'):MOD.audit_query(q,chunks,c['state']['commit'],False)


if __name__=='__main__':unittest.main()
