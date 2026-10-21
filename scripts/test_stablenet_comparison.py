"""Mutations must not turn technical comparison evidence into accepted quality."""
import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import unittest
import evidence_revision

spec = importlib.util.spec_from_file_location('comparisoncheck', Path(__file__).with_name('check-stablenet-comparison.py'))
check = importlib.util.module_from_spec(spec)
spec.loader.exec_module(check)
# The run's owned selected indexes lived under a temporary root that was removed
# after the run; a fresh checkout cannot re-read them.
OWNED_INDEXES = [Path(f['version']) / 'vector/vector.db' for f in check.read(check.ROOT / check.REL / 'fixtures.json')]
LIVE_INDEXES = all(p.is_file() for p in OWNED_INDEXES)


class ComparisonEvidenceTests(unittest.TestCase):
    def setUp(self):
        temp = tempfile.TemporaryDirectory()
        self.addCleanup(temp.cleanup)
        self.packet = Path(temp.name) / 'packet'
        shutil.copytree(check.ROOT / check.REL, self.packet)

    def seal(self):
        p = self.packet / 'manifest.json'
        d = json.loads(p.read_text())
        d['artifacts'] = {str(x.relative_to(self.packet)): check.sha(x.read_bytes()) for x in self.packet.rglob('*') if x.is_file() and x != p}
        p.write_text(json.dumps(d))

    def change(self, name, fn):
        p = self.packet / name
        d = json.loads(p.read_text())
        fn(d)
        p.write_text(json.dumps(d, ensure_ascii=False))
        self.seal()

    def response(self, folder, label, fn):
        root = self.packet / folder
        p = root / 'messages.json'
        rows = json.loads(p.read_text())
        row = next(r for r in rows if r['label'] == label)
        fn(row['response']['result']['structuredContent'])
        r = row['response']
        for c in r['result'].get('content', []):
            if c.get('type') == 'text':
                c['text'] = json.dumps(r['result']['structuredContent'])
        p.write_text(json.dumps(rows))
        p = root / 'stdout.jsonl'
        wire = [json.loads(s) for s in p.read_text().splitlines()]
        p.write_text(''.join(json.dumps(r if x.get('id') == r['id'] else x) + '\n' for x in wire))
        p = root / 'attempts.json'
        tried = json.loads(p.read_text())
        next(a for a in tried if a['label'] == label)['response'] = r
        p.write_text(json.dumps(tried))
        self.seal()

    def rejected(self):
        self.assertTrue(check.check(self.packet, LIVE_INDEXES))

    def matrix(self, name, fn):
        p = self.packet / 'comparison' / (name + '.json')
        original = json.loads(p.read_text())
        rows = json.loads((self.packet / 'comparison/comparison-session/messages.json').read_text())
        label = next(r['label'] for r in rows if r['response'].get('result', {}).get('structuredContent') == original)
        self.response('comparison/comparison-session', label, fn)
        rows = json.loads((self.packet / 'comparison/comparison-session/messages.json').read_text())
        changed = next(r['response']['result'] for r in rows if r['label'] == label)
        p.write_text(json.dumps(changed['structuredContent']))
        p = self.packet / 'comparison/calls.json'
        calls = json.loads(p.read_text())
        next(c for c in calls if c['response'].get('structuredContent') == original)['response'] = changed
        p.write_text(json.dumps(calls))
        self.seal()

    def test_actual_evidence_valid(self):
        self.assertEqual([], check.check(self.packet, live=False))

    def test_bound_sources_match_checkout(self):
        binding = check.read(self.packet / 'source-binding.json')
        for name, digest in {**binding['source_sha256'], **binding['approved_input_sha256']}.items():
            self.assertEqual(check.sha((check.ROOT / name).read_bytes()), digest, name)

    @unittest.skipUnless(LIVE_INDEXES, 'owned selected indexes were not retained after the run')
    def test_actual_evidence_valid_against_owned_indexes(self):
        self.assertEqual([], check.check(self.packet))

    def test_formal_promotion_refused(self):
        self.change('execution.json', lambda d: d.update(formal_completed=9))
        self.rejected()

    def test_quality_promotion_refused(self):
        self.change('execution.json', lambda d: d.update(independent_quality_acceptance=True))
        self.rejected()

    def test_mock_cannot_be_relabeled_as_real_bge(self):
        self.change('fixtures.json', lambda d: d[1]['embedding'].update(Dim=64))
        self.rejected()

    def test_original_mock_identity_preserved(self):
        self.change('legacy-wemix-identity.json', lambda d: d['embedding_identity'].update(dim=1024))
        self.rejected()

    def test_foreign_v2_project_even_with_consistent_wire_refused(self):
        self.response('search-go-wemix', 'C1-v2', lambda p: p['coordinates'].update(project_id='foreign-project'))
        self.rejected()

    def test_foreign_citation_even_with_consistent_wire_refused(self):
        self.response('search-go-stablenet', 'C1-v2', lambda p: p['citations'][0].update(dataset_id='0' * 64))
        self.rejected()

    def test_source_body_cannot_be_rewritten(self):
        self.response('search-go-wemix', 'C1-v2', lambda p: p['bodies'][0].update(text='foreign source body'))
        self.rejected()

    def test_successful_foreign_pin_cannot_be_claimed(self):
        self.response('search-go-stablenet', 'foreign-expected_commit', lambda p: p.update(status='partial'))
        self.rejected()

    def test_missing_attempt_cannot_be_hidden(self):
        self.change('search-go-wemix/attempts.json', lambda d: d.pop())
        self.rejected()

    def test_unreaped_session_cannot_pass(self):
        self.change('comparison/comparison-session/lifecycle.json', lambda d: d.update(child_reaped=False))
        self.rejected()

    def test_mixed_project_startup_cannot_pass(self):
        self.change('swapped-go-wemix/lifecycle.json', lambda d: d.update(exit_code=0))
        self.rejected()

    def test_current_activation_cannot_pass(self):
        self.change('build-exit.json', lambda d: d.update(current_exists=True))
        self.rejected()

    def test_remaining_ontology_acceptance_cannot_be_hidden(self):
        self.change('WORKLIST.json', lambda d: d.update(remaining_all=[]))
        self.rejected()

    def test_candidate_cannot_cite_foreign_project(self):
        self.matrix('matrix-consensus', lambda p: p['candidates'][0]['evidence'][0]['citations'][0].update(project_id='foreign-project'))
        self.rejected()

    def test_proposed_mapping_cannot_be_promoted(self):
        self.matrix('matrix-consensus', lambda p: p['groups'][0]['cells'][0]['mappings'][0]['binding'].update(review_state='verified'))
        self.rejected()

    def test_not_collected_cannot_mean_feature_absence(self):
        self.matrix('matrix-staking', lambda p: p['groups'][0]['cells'][0].update(state='not_applicable'))
        self.rejected()

    def test_finality_cannot_be_inferred_from_commit(self):
        self.matrix('matrix-finality', lambda p: p['groups'][0]['cells'][0].update(state='similar'))
        self.rejected()

    def test_common_concept_cannot_merge_entity_identity(self):
        self.matrix('matrix-consensus', lambda p: p['groups'][0]['cells'][0].update(entity_identity=True))
        self.rejected()

    def test_initial_collector_failure_cannot_be_hidden(self):
        self.change('diagnostics/initial-empty-hit-run/search-go-stablenet/messages.json', lambda d: d.pop())
        self.rejected()

    def test_indexed_flow_hint_cannot_be_rewritten_as_code(self):
        def replace(p):
            hit = next(h for h in p['hits'] if h.get('chunk_kind') == 'flow_step')
            hit['text'] = 'invented code quotation'
        self.response('search-go-stablenet', 'C1-vector-all', replace)
        self.rejected()

    def test_resealed_request_views_must_still_match_actual_stdin(self):
        self.change('comparison/bootstrap-session/messages.json', lambda rows: rows[1]['request']['params'].update(arguments={'unknown': 'forged'}))
        self.change('comparison/bootstrap-session/attempts.json', lambda rows: rows[1]['params'].update(arguments={'unknown': 'forged'}))
        self.change('comparison/calls.json', lambda rows: rows[0].update(arguments={'unknown': 'forged'}))
        self.rejected()


# Later stages changed inputs this packet binds; run at its recorded revision.
load_tests = evidence_revision.tests(__file__)


if __name__ == '__main__':
    unittest.main()
