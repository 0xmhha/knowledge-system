"""Diagnostic raw-ranking, source-binding and sparse-filter failure oracles."""
import copy
import importlib.util
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

SPEC = importlib.util.spec_from_file_location('raw_summary', Path(__file__).with_name('b0-summarize-vector-probe.py'))
M = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(M)


def encoded(value):
    return json.dumps(value, ensure_ascii=False, separators=(',', ':')).encode()


class RawVectorReportTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.source = self.root / 'source'
        self.source.mkdir()
        self.commit = 'c' * 40
        coords = {'project_id': 'synthetic-reader-test', 'dataset_id': 'd'*64, 'snapshot_id': 'a'*64, 'commit': self.commit}
        identity = {'Provider':'mock', 'Model':'synthetic-reader', 'Dim':2, 'version':2}
        filters = {'language':'go', 'path':'src/*.go', 'symbol_kinds':['Function'], 'chunk_kinds':['symbol'], 'exclude_tests':True}
        files, eligible, distractors, exact = [], [], [], []
        for group, target in [('eligible', eligible), ('distractor', distractors)]:
            for i in range(5):
                file = 'src/' + ('distractor/' if group == 'distractor' else '') + f'{i}.go'
                text = f'// 한글 source {group} {i}\nfunc Alpha{i}() {{}}'
                raw = ('package fixture\n\n' + text + '\n').encode()
                path = self.source / file
                path.parent.mkdir(exist_ok=True, parents=True)
                path.write_bytes(raw)
                files.append({'origin_id':'repo', 'path':file, 'sha256':M.BASE.sha(raw)})
                distance = 2+i/10 if group == 'eligible' else i/10
                target.append({'chunk':{'id':group + str(i), 'file':file, 'start_line':3, 'end_line':4,
                                       'language':'go', 'symbol_kind':'Function', 'chunk_kind':'symbol',
                                       'commit_hash':self.commit, 'text':text, 'content_sha256':M.BASE.sha(text.encode())},
                               'score':{'vector_distance':distance, 'normalized':.5, 'vector_rank':i+1}})
                if group == 'eligible':
                    exact.append({'id':group + str(i), 'file':file, 'start_line':3, 'end_line':4, 'distance':distance})
        self.manifest = {'project_id':coords['project_id'], 'dataset_id':coords['dataset_id'], 'snapshot_id':coords['snapshot_id'],
                         'src_commit':self.commit, 'db_sha256':'b'*64, 'embedding_identity_v2':identity, 'input_files':files}
        search = lambda hits: {'Hits':hits, 'CandidateCount':5, 'EligibleCount':5, 'SearchedCount':5, 'Status':'complete', 'Reason':''}
        self.probe = {'schema_version':1, 'gate':'B0-vector-probe', 'quality_metrics':None, 'state':'recorded',
                      'coordinates':coords, 'coordinate_state':'pinned', 'embedding_identity':identity, 'identity_verified_after':True,
                      'manifest_sha256':M.BASE.sha(encoded(self.manifest)), 'manifest_sha256_after':M.BASE.sha(encoded(self.manifest)),
                      'database_sha256_before':'b'*64, 'database_sha256_after':'b'*64, 'prompt':'synthetic raw-report test',
                      'k':5, 'max_exact_candidates':2, 'filter':filters, 'query_vector':[1.,0.], 'embed_ns':10,
                      'search_ns':{'unfiltered':20,'filtered':30,'budget':40,'oracle':50},
                      'unfiltered':search(distractors), 'filtered':search(eligible),
                      'budget':{'Hits':None, 'CandidateCount':5, 'EligibleCount':None, 'SearchedCount':0,
                                'Status':'incomplete', 'Reason':'candidate_limit'},
                      'oracle':{'eligible_count':5,'hits':exact,'eligible_hits':copy.deepcopy(exact)},
                      'exact_agreement':True, 'sparse_fixture_qualified':True}
        self.controls = {'schema_version':1, 'diagnostic_only':True,
                         **{key:copy.deepcopy(self.probe[key]) for key in ('coordinates','embedding_identity','prompt','filter','k','max_exact_candidates')},
                         'expected_citations':[dict(file='src/0.go',start_line=3,end_line=4,commit_hash=self.commit),
                                               dict(file='src/4.go',start_line=3,end_line=4,commit_hash=self.commit)],
                         'f01':{'k':5,'max_exact_candidates':2,'eligible_count':5,'complete_hit_count':5,
                                'eligible_files':[f'src/{i}.go' for i in range(5)]}}

    def result(self):
        raw = encoded(self.probe)
        self.controls['probe_sha256'] = M.BASE.sha(raw)
        return M.summarize(raw, encoded(self.manifest), self.controls, self.source)

    def test_source_bound_raw_ranks_and_f01_remain_diagnostic(self):
        r = self.result()
        self.assertEqual(r['f01']['state'], 'diagnostic_controls_pass')
        self.assertEqual(r['invariant_violations'], [])
        scores = r['searches']['filtered']['diagnostic_scores']
        self.assertEqual(scores['expected_span_first_raw_ranks'], [1,5])
        self.assertEqual(scores['raw_ckv_source_span_recall_at_k'], 1)
        self.assertEqual(scores['raw_ckv_candidate_precision_at_k'], .4)
        self.assertEqual(scores['raw_ckv_mean_expected_reciprocal_rank_at_k'], .6)
        self.assertEqual(r['searches']['budget']['diagnostic_scores']['raw_ckv_source_span_recall_at_k'], 0)
        self.assertIsNone(r['quality_metrics'])
        self.assertIsNone(r['official_f01_verdict'])
        self.assertFalse(r['official_execution_ready'])
        self.assertNotEqual(r['searches']['filtered']['raw_candidates'][0]['parser_text_sha256'],
                            r['searches']['filtered']['raw_candidates'][0]['source_span_sha256'])

    def test_missing_full_eligible_inventory_is_not_a_pass(self):
        del self.probe['oracle']['eligible_hits']
        self.assertEqual(self.result()['f01']['state'], 'fixture_not_qualified')
        self.assertIn('full_eligible_inventory_missing', self.result()['f01']['qualification_reasons'])

    def test_k1_smoke_is_never_promoted_to_f01_or_recall_at_10(self):
        self.probe['k'] = self.controls['k'] = 1
        self.probe['oracle'] = {'eligible_count':1,'hits':self.probe['oracle']['hits'][:1]}
        self.probe['filtered']['Hits'] = self.probe['filtered']['Hits'][:1]
        self.probe['filtered']['EligibleCount'] = 1
        self.probe['unfiltered'] = copy.deepcopy(self.probe['filtered'])
        self.probe['budget'] = copy.deepcopy(self.probe['filtered'])
        self.probe['sparse_fixture_qualified'] = False
        r = self.result()
        self.assertEqual(r['k'],1)
        self.assertEqual(r['f01']['state'],'fixture_not_qualified')
        self.assertIn('K_differs',r['f01']['qualification_reasons'])
        self.assertFalse(r['f01']['sparse_helper'])
        self.assertFalse(any('at_10' in k for k in r['searches']['filtered']['diagnostic_scores']))

    def test_extra_eligible_file_outside_top_k_is_not_hidden(self):
        self.probe['oracle']['eligible_count'] = 6
        self.probe['oracle']['eligible_hits'].append({'id':'extra', 'file':'src/distractor/0.go',
                                                    'start_line':3,'end_line':4,'distance':3})
        self.assertIn('full_eligible_files_differ', self.result()['f01']['qualification_reasons'])
        self.assertIn('filtered_eligible_count_differs', self.result()['invariant_violations'])

    def test_wrong_budget_status_or_reason_does_not_pass(self):
        for update in [{'Status':'complete','Reason':''}, {'Reason':'deadline_exceeded'}, {'CandidateCount':2}]:
            with self.subTest(update=update):
                old = copy.deepcopy(self.probe['budget'])
                self.probe['budget'].update(update)
                self.assertEqual(self.result()['f01']['state'], 'diagnostic_controls_fail')
                self.probe['budget'] = old

    def test_partial_unfiltered_top_k_does_not_qualify_sparse_fixture(self):
        self.probe['unfiltered'].update(Status='incomplete',Reason='deadline_exceeded')
        r = self.result()
        self.assertEqual(r['f01']['state'],'fixture_not_qualified')
        self.assertIn('unfiltered_top_K_not_complete',r['f01']['qualification_reasons'])

    def test_wrong_exact_location_is_preserved_despite_matching_id_distance(self):
        self.probe['filtered']['Hits'][0]['chunk']['start_line'] = 2
        r = self.result()
        self.assertFalse(r['f01']['normal_exact_agreement'])
        self.assertIn('reported_exact_agreement_differs', r['invariant_violations'])
        self.assertEqual(r['f01']['state'], 'diagnostic_controls_fail')

    def test_exact_order_and_inventory_count_refuse_corruption(self):
        self.probe['oracle']['eligible_hits'].reverse()
        with self.assertRaisesRegex(ValueError, 'order'): self.result()
        self.probe['oracle']['eligible_hits'].reverse()
        self.probe['oracle']['eligible_hits'].pop()
        with self.assertRaisesRegex(ValueError, 'count'): self.result()

    def test_source_sha_parser_text_commit_and_path_refuse_corruption(self):
        original = copy.deepcopy(self.probe)
        for field, value, error in [('text','forged text','text/source'), ('commit_hash','f'*40,'foreign'),
                                   ('file','../0.go','unsafe'), ('content_sha256','a'*64,'text/source')]:
            self.probe = copy.deepcopy(original)
            self.probe['filtered']['Hits'][0]['chunk'][field] = value
            with self.subTest(field=field), self.assertRaisesRegex(ValueError, error): self.result()
        self.probe = original
        (self.source / 'src/0.go').write_text('modified\n')
        with self.assertRaisesRegex(ValueError, 'source file hash'): self.result()

    def test_bad_query_rank_duplicate_id_and_nonfinite_distance_refused(self):
        original = copy.deepcopy(self.probe)
        for change in [lambda p:p.update(query_vector=[1]),
                       lambda p:p['filtered']['Hits'][0]['score'].update(vector_rank=2),
                       lambda p:p['filtered']['Hits'][1]['chunk'].update(id=p['filtered']['Hits'][0]['chunk']['id']),
                       lambda p:p['filtered']['Hits'][0]['score'].update(vector_distance=float('nan'))]:
            self.probe = copy.deepcopy(original)
            change(self.probe)
            with self.assertRaises(ValueError): self.result()

    def test_seals_controls_and_metadata_only_official_mode_refused(self):
        original = copy.deepcopy(self.probe)
        for update in [{'database_sha256_after':'a'*64}, {'manifest_sha256_after':'a'*64},
                       {'identity_verified_after':False}, {'coordinate_state':'unpinned'}]:
            self.probe = copy.deepcopy(original)
            self.probe.update(update)
            with self.assertRaises(ValueError): self.result()
        self.probe = original
        self.controls['diagnostic_only'] = False
        with self.assertRaisesRegex(ValueError, 'diagnostic'): self.result()

    def test_raw_candidate_slots_are_not_composed_citation_dedup(self):
        hit = self.probe['filtered']['Hits'][0]
        duplicate_span = copy.deepcopy(hit)
        duplicate_span['chunk']['id'] = 'different-chunk-same-span'
        actual = [hit, duplicate_span, self.probe['filtered']['Hits'][4]]
        scores = M.raw_metrics(self.controls['expected_citations'], actual)
        self.assertEqual(scores['expected_span_first_raw_ranks'], [1,3])
        self.assertAlmostEqual(scores['raw_ckv_mean_expected_reciprocal_rank_at_k'], 2/3)

    def test_test_support_commit_and_glob_filter_rules(self):
        filters = {'exclude_tests':True}
        for path in ['src/testutil.go','tests/a.go','x/a.spec.ts','x/a.t.sol','x/a_test.go']:
            self.assertFalse(M.filter_matches(filters, {'file':path}))
        self.assertFalse(M.filter_matches({'path':'src/*.go'}, {'file':'src/nested/a.go'}))
        self.assertFalse(M.filter_matches({'commit_hash':self.commit}, {'file':'a.go','commit_hash':'f'*40}))
        with self.assertRaisesRegex(ValueError, 'unsupported'): M.filter_matches({'path':'src/**/*.go'}, {'file':'src/a.go'})

    def test_error_call_retains_denominator_without_invented_metrics(self):
        self.probe = {'schema_version':1,'gate':'B0-vector-probe','quality_metrics':None,'state':'error'}
        r = self.result()
        self.assertEqual(r['failed_probe_calls'], 1)
        self.assertEqual(r['planned_probe_calls'], 1)
        self.assertIsNone(r['diagnostic_scores'])
        self.assertEqual(r['f01']['state'], 'measurement_failed')

    def test_cli_exclusive_output_and_input_binding(self):
        self.result()
        paths = {name:self.root / (name + '.json') for name in ['probe','manifest','controls','out']}
        for name in ['probe','manifest','controls']: paths[name].write_bytes(encoded(getattr(self,name)))
        command = [sys.executable, str(Path(M.__file__)), '--source-root', str(self.source)]
        for key,path in paths.items(): command.extend(['--'+key, str(path)])
        first = subprocess.run(command,capture_output=True)
        self.assertEqual(first.returncode,0,first.stderr)
        self.assertEqual(paths['out'].stat().st_mode & 0o777,0o600)
        before = paths['out'].read_bytes()
        self.assertEqual(subprocess.run(command,capture_output=True).returncode,1)
        self.assertEqual(before,paths['out'].read_bytes())
        paths['out'].unlink()
        self.probe['budget']['Reason'] = 'deadline_exceeded'
        self.result()
        paths['probe'].write_bytes(encoded(self.probe))
        paths['controls'].write_bytes(encoded(self.controls))
        self.assertEqual(subprocess.run(command,capture_output=True).returncode,2)
        self.assertEqual(json.loads(paths['out'].read_text())['f01']['state'],'diagnostic_controls_fail')
        paths['probe'].write_text('{}')
        paths['out'].unlink()
        self.assertEqual(subprocess.run(command,capture_output=True).returncode,1)
        self.assertFalse(paths['out'].exists())


if __name__ == '__main__':
    unittest.main()
