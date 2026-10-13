"""Saved-artifact coverage must not imply independence or hide bad inputs."""
import copy
import gzip
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest
from unittest import mock

S = importlib.util.spec_from_file_location('history', Path(__file__).with_name('ontology-eval-history-audit.py'))
H = importlib.util.module_from_spec(S)
S.loader.exec_module(H)


class HistoryContract(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        raw = b'package test\nfunc Unique() {\n return\n}\n'
        (self.root / 'a.go').write_bytes(raw)
        self.gold = dict(file='a.go', start_line=2, end_line=3, commit_hash='a'*40,
                         file_sha256=H.E.sha(raw), content_sha256=H.E.sha(b''.join(raw.splitlines(keepends=True)[1:3])),
                         relation_assertion=False)
        self.book = dict(review_state='proposed', independent_units_approved=0,
                         coordinates={'base_commit':'a'*40}, questions=[dict(id='Q1', review_state='proposed',
                         question='Find F', fact_cluster_proposed='fact', independent_FINAL_eligible=False,
                         evidence=[self.gold])])
        self.audit = H.HistoryAudit(self.book, self.root)
        self.view = dict(query='other', citations=[dict(self.gold)], bodies=[])

    def consume(self, view=None, role='response'):
        self.audit.consume(role, self.view if view is None else view, 'fixture:1')
        return self.audit.report([])['candidates'][0]

    def test_standalone_nested_and_text_mirror_formats(self):
        text = {'type':'text', 'text':json.dumps(self.view)}
        wrappers = [self.view, {'rows':[{'response':self.view}]}, {'content':[text]},
                    {'structuredContent':self.view, 'content':[text]}]
        for wrapper in wrappers:
            self.assertEqual(list(H.representations(wrapper)), [('response', self.view)])

    def test_repeated_views_not_independent_units(self):
        for _ in range(20): self.consume()
        report = self.audit.report([])
        self.assertEqual(report['views']['observed_response'],20)
        self.assertEqual(report['views']['unique_response'],1)
        self.assertEqual(report['candidates'][0]['response_range_exposure'],1)
        self.assertEqual(report['approved_independent_units'],0)
        self.assertEqual(report['FINAL_eligible_questions'],0)

    def test_authored_gold_with_same_commit_without_sha(self):
        citation = dict(self.gold)
        del citation['file_sha256']
        representation = list(H.representations({'question':'old', 'expected_citations':[citation]}))
        self.assertEqual(len(representation),1)
        row = self.consume(representation[0][1], representation[0][0])
        self.assertEqual(row['authored_gold_exposure'],1)
        self.assertEqual(row['response_range_exposure'],0)

    def test_revision_changes_and_boolean_range_are_uncertain(self):
        for overrides in [dict(commit_hash='b'*40,file_sha256='b'*64),dict(start_line=True)]:
            audit=H.HistoryAudit(self.book,self.root)
            audit.consume('response',dict(self.view,citations=[dict(self.gold,**overrides)]),'fixture')
            row=audit.report([])['candidates'][0]
            self.assertEqual(row['unknown_revision_or_range'],1)
            self.assertEqual(row['queue_state'],'historical_revision_review_needed')

    def test_no_citation_body_text_still_quarantines(self):
        row=self.consume(dict(query='other',citations=[],body='func Unique() {\r\n return'))
        self.assertEqual(row['body_text_exposure'],1)
        self.assertEqual(row['queue_state'],'quarantined_prior_exposure')

    def test_query_match_without_gold_is_exposure(self):
        row=self.consume(dict(query=' FIND  f\n',citations=[],bodies=None))
        self.assertEqual(row['prior_query_exact_matches'],1)
        self.assertEqual(row['queue_state'],'quarantined_prior_exposure')

    def test_body_string_or_null_does_not_crash(self):
        for value in [None,'source text',{}]:
            self.consume(dict(self.view,bodies=value))

    def test_legacy_location_aliases(self):
        span=H.location(dict(File='/old/consensus/wbft/core/core.go',first=2,last=3))
        self.assertEqual(span['file'],'consensus/wbft/core/core.go')
        self.assertEqual((span['start_line'],span['end_line']),(2,3))

    def test_unsupported_citation_blocks_machine_coverage(self):
        self.consume(dict(self.view,citations=[7]))
        report=self.audit.report([])
        self.assertFalse(report['machine_catalog_fully_parsed'])
        self.assertEqual(report['unsupported_citation_views'],1)

    def test_failed_catalog_not_empty_exposure_proof(self):
        report=self.audit.report([dict(path='bad.json',state='unread_or_invalid')])
        self.assertFalse(report['machine_catalog_fully_parsed'])
        self.assertFalse(report['semantic_independence_review_complete'])
        self.assertEqual(report['unread_or_failed_files'],['bad.json'])

    def test_known_empty_artifact_preserves_coverage_limit(self):
        report=self.audit.report([dict(path='empty.json',state='unread_or_invalid',empty_artifact=True)])
        self.assertTrue(report['nonempty_catalog_fully_parsed'])
        self.assertFalse(report['machine_catalog_fully_parsed'])
        self.assertEqual(report['empty_artifact_files'],['empty.json'])

    def test_symbol_mention_is_review_cue_not_proven_exposure(self):
        self.gold['canonical_id']='pkg.Unique'
        audit=H.HistoryAudit(self.book,self.root)
        audit.consume('text_history',dict(citations=[],body='Changed symbols: Unique'),'design.md')
        row=audit.report([])['candidates'][0]
        self.assertEqual(row['text_symbol_review_cues'],[dict(origin='design.md',symbols=['Unique'])])
        self.assertEqual(row['queue_state'],'content_and_semantic_independence_review_needed')
        self.assertFalse(row['independent_FINAL_eligible'])

    def test_exact_snippet_in_design_document_quarantines(self):
        row=self.consume(dict(citations=[],body='func Unique() {\n return'),role='text_history')
        self.assertEqual(row['text_history_snippet_exposure'],1)
        self.assertEqual(row['queue_state'],'quarantined_prior_exposure')

    def test_jsonl_gzip_and_pretty_object_streams(self):
        for name in ['capture.json','capture.jsonl','capture.json.gz','capture.jsonl.gz']:
            path=self.root/name
            data=(json.dumps(self.view,indent=2)+'\n'+json.dumps(self.view)+'\n').encode()
            path.write_bytes(gzip.compress(data) if name.endswith('.gz') else data)
            self.assertEqual(list(H.records(path)),[(1,self.view),(2,self.view)])

    def test_empty_malformed_and_duplicate_records_fail(self):
        for raw in [b'',b'{"citations":[],"citations":[1]}',b'{"x":NaN}',b'{}\n{bad}',b'{}garbage']:
            path=self.root/'bad.jsonl'
            path.write_bytes(raw)
            with self.assertRaises(ValueError): list(H.records(path))
        with self.assertRaises(ValueError):
            list(H.representations({'content':[{'type':'text','text':'{bad}'}]}))

    def test_uncompressed_limit_applies_to_gzip(self):
        path=self.root/'large.json.gz'
        path.write_bytes(gzip.compress(b'{"x":"'+b'a'*100+b'"}'))
        with mock.patch.object(H,'MAX_JSON_BYTES',20), self.assertRaises(ValueError):
            list(H.records(path))

    def test_many_small_records_can_exceed_whole_file_limit(self):
        path=self.root/'many.jsonl.gz'
        path.write_bytes(gzip.compress(b'{"x":1}\n'*100))
        with mock.patch.object(H,'MAX_JSON_BYTES',20):
            self.assertEqual(len(list(H.records(path))),100)

    def test_value_crossing_read_boundary_and_utf8(self):
        path=self.root/'boundary.jsonl.gz'
        rows=[{'text':'가'*40000},{'x':2}]
        path.write_bytes(gzip.compress(('\n'.join(json.dumps(r,ensure_ascii=False) for r in rows)).encode()))
        self.assertEqual([r for _,r in H.records(path)],rows)

    def test_changed_source_and_inferred_approval_rejected(self):
        book=copy.deepcopy(self.book)
        book['questions'][0]['independent_FINAL_eligible']=True
        with self.assertRaises(ValueError): H.HistoryAudit(book,self.root)
        (self.root/'a.go').write_text('changed')
        with self.assertRaises(ValueError): H.HistoryAudit(self.book,self.root)


if __name__=='__main__': unittest.main()
