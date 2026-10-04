"""Failure oracles for protocol binding, held-out input boundaries and export."""
import importlib.util
import json
from pathlib import Path
import tempfile
import subprocess
import sys
import unittest

SPEC = importlib.util.spec_from_file_location('static_inputs', Path(__file__).with_name('b0-prepare-static-inputs.py'))
INPUTS = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(INPUTS)


class StaticInputTest(unittest.TestCase):
    def setUp(self):
        self.raw = [(INPUTS.DATA / name).read_bytes() for name in (
            'questions.json', 'protocol-m2max-draft.json', 'dynamic-fixtures-m2max-draft.json',
            'human-review-m2max-2026-10-03.json')]
        # Keep failure prerequisites explicit when real approvals are recorded.
        # These changes exist only in memory or this test's temporary files.
        fixtures = json.loads(self.raw[2])
        fixtures['status'] = 'proposal_pending_human_review'
        for fixture in fixtures['fixtures']:
            fixture.update(review_state='draft', reviewer=None, reviewed_at=None)
        self.raw[2] = json.dumps(fixtures).encode()
        protocol = json.loads(self.raw[1])
        protocol.update(status='draft', reviewer=None, reviewed_at=None,
                        fixture_manifest_sha256=INPUTS.digest(self.raw[2]))
        self.raw[1] = json.dumps(protocol).encode()

    def inspect_changed(self, index, change):
        raw = list(self.raw)
        value = json.loads(raw[index])
        change(value)
        raw[index] = json.dumps(value).encode()
        return INPUTS.inspect(*raw)

    def test_explicit_draft_inputs_remain_pending_with_independent_final_counts(self):
        report, selected = INPUTS.inspect(*self.raw)
        self.assertEqual(report['status'], 'pending')
        self.assertEqual(report['pending_reasons'], ['protocol_review_pending', 'dynamic_fixture_review_pending'])
        self.assertEqual(len(selected), 4)
        self.assertEqual(len(report['withheld_ids']), 8)
        self.assertEqual(report['static_approved_count'], 12)
        self.assertEqual(report['dynamic_approved_count'], 0)
        self.assertEqual(sum(report['final_independent_questions_by_group'].values()), 8)
        self.assertTrue(all(n < 10 for n in report['final_independent_questions_by_group'].values()))
        self.assertIsNone(report['metrics'])

    def test_development_export_withholds_final_prompts_and_candidate_answers(self):
        report, selected = INPUTS.inspect(*self.raw)
        with tempfile.TemporaryDirectory() as temp:
            out = Path(temp) / 'development'
            INPUTS.export(report, selected, out)
            files = list(out.glob('*.yaml'))
            self.assertEqual(len(files), 4)
            manifest = json.loads((out / 'manifest.json').read_text())
            self.assertTrue(manifest['diagnostic_only'])
            self.assertFalse(manifest['v2_requests_exported'])
            text = '\n'.join(file.read_text() for file in files)
            book = json.loads(self.raw[0])
            for question in book['questions']:
                self.assertNotIn(question['candidate_answer'], text)
                if question['id'] in report['withheld_ids']:
                    self.assertNotIn(question['prompt'], text)
            for file in files:
                self.assertEqual(file.stat().st_mode & 0o777, 0o600)
                self.assertEqual(INPUTS.digest(file.read_bytes()), manifest['scenario_sha256'][file.name])
            with self.assertRaisesRegex(ValueError, 'already exists'):
                INPUTS.export(report, selected, out)

    def test_final_export_refuses_draft_before_creating_output(self):
        report, selected = INPUTS.inspect(*self.raw, partition='final')
        with tempfile.TemporaryDirectory() as temp:
            out = Path(temp) / 'final'
            with self.assertRaises(INPUTS.PendingInputs):
                INPUTS.export(report, selected, out)
            self.assertFalse(out.exists())

    def test_cli_ready_guard_and_existing_report_do_not_publish_or_overwrite(self):
        with tempfile.TemporaryDirectory() as temp:
            out, report = Path(temp) / 'inputs', Path(temp) / 'report.json'
            source_args = []
            for flag, raw in zip(('questions', 'protocol', 'fixtures', 'human-review'), self.raw):
                file = Path(temp) / (flag + '.json')
                file.write_bytes(raw)
                source_args.extend(['--' + flag, str(file)])
            command = [sys.executable, str(Path(INPUTS.__file__)), '--require-ready',
                       '--out-dir', str(out), '--output', str(report), *source_args]
            result = subprocess.run(command, capture_output=True, text=True)
            self.assertEqual(result.returncode, 2, result.stderr)
            self.assertFalse(out.exists())
            original = report.read_bytes()
            self.assertEqual(report.stat().st_mode & 0o777, 0o600)
            result = subprocess.run(command, capture_output=True, text=True)
            self.assertEqual(result.returncode, 1)
            self.assertEqual(report.read_bytes(), original)

    def test_overlap_unknown_omitted_and_duplicate_static_ids_are_rejected(self):
        def overlap(p):
            p['final_questions'][0] = p['development_questions'][0]
        for mutate in (overlap,
                       lambda p: p['final_questions'].pop(),
                       lambda p: p['final_questions'].__setitem__(0, 'B0-NOTFOUND-01'),
                       lambda p: p['development_questions'].append(p['development_questions'][0])):
            with self.subTest(mutate=mutate), self.assertRaisesRegex(ValueError, 'partition|unique'):
                self.inspect_changed(1, mutate)

    def test_stale_hash_tree_model_and_human_review_bindings_are_rejected(self):
        cases = [(1, lambda p: p.update(question_set_sha256='0' * 64), 'hash'),
                 (1, lambda p: p.update(fixture_manifest_sha256='0' * 64), 'hash'),
                 (1, lambda p: p.update(corpus_tree='0' * 40), 'commit/tree'),
                 (1, lambda p: p['model'].update(digest='0' * 64), 'human model'),
                 (3, lambda p: p['decisions'][0].update(question_set_sha256_after='0' * 64), 'human gold')]
        for index, mutate, message in cases:
            with self.subTest(message=message), self.assertRaisesRegex(ValueError, message):
                self.inspect_changed(index, mutate)

    def test_fixture_source_and_partition_cannot_be_changed_with_only_a_new_manifest_hash(self):
        for mutate in (lambda f: f['fixtures'][0]['sources'].update({'go.mod': 'changed source'}),
                       lambda f: f['fixtures'][0].update(evaluation_partition='final')):
            with self.subTest(mutate=mutate):
                raw = list(self.raw)
                fixtures = json.loads(raw[2]); mutate(fixtures)
                raw[2] = json.dumps(fixtures).encode()
                protocol = json.loads(raw[1]); protocol['fixture_manifest_sha256'] = INPUTS.digest(raw[2])
                raw[1] = json.dumps(protocol).encode()
                with self.assertRaisesRegex(ValueError, 'source hash|family/partition'):
                    INPUTS.inspect(*raw)

    def test_threshold_relaxation_boolean_counts_and_mode_collapse_are_rejected(self):
        cases = [(lambda p: p['thresholds'].update(warm_p95_ratio_max=2), 'threshold'),
                 (lambda p: p['thresholds'].update(safety_snapshot_secret_mixing_max=False), 'threshold'),
                 (lambda p: p.update(retrieval_runs=True), 'integer'),
                 (lambda p: p['warm_latency'].update(measured_runs=0), 'integer'),
                 (lambda p: p.update(retrieval_k=20), 'Recall@10'),
                 (lambda p: p.update(paired_arms=['baseline'] * 4), 'four modes')]
        for mutate, message in cases:
            with self.subTest(message=message), self.assertRaisesRegex(ValueError, message):
                self.inspect_changed(1, mutate)

    def test_duplicate_json_keys_are_rejected(self):
        with self.assertRaisesRegex(ValueError, 'duplicate JSON'):
            INPUTS.decode(b'{"status":"draft","status":"approved"}')

    def metadata_approved(self):
        raw = list(self.raw)
        fixtures = json.loads(raw[2])
        fixtures['status'] = 'approved'
        for fixture in fixtures['fixtures']:
            fixture.update(review_state='approved', reviewer='synthetic-test-only', reviewed_at='2026-10-03T00:00:00Z')
        raw[2] = json.dumps(fixtures).encode()
        protocol = json.loads(raw[1])
        protocol.update(status='approved', reviewer='synthetic-test-only', reviewed_at='2026-10-03T00:00:00Z',
                        fixture_manifest_sha256=INPUTS.digest(raw[2]))
        raw[1] = json.dumps(protocol).encode()
        return raw

    def test_metadata_only_approval_cannot_replace_pending_human_decision(self):
        raw = self.metadata_approved()
        review = json.loads(raw[3])
        review['protocol_and_dynamic_fixture_decision'] = {'status': 'pending', 'verbatim_response': '검토 후 결정'}
        raw[3] = json.dumps(review).encode()
        report, selected = INPUTS.inspect(*raw, partition='final')
        self.assertEqual(report['status'], 'pending')
        self.assertEqual(report['pending_reasons'], ['protocol_review_pending', 'dynamic_fixture_review_pending'])
        with tempfile.TemporaryDirectory() as temp:
            out = Path(temp) / 'final'
            with self.assertRaises(INPUTS.PendingInputs):
                INPUTS.export(report, selected, out)
            self.assertFalse(out.exists())

    def test_synthetic_approval_path_binds_updated_fixture_bytes_and_v1_abstention(self):
        # Synthetic metadata is confined to a unit-test temporary directory;
        # no repository approval or final model execution is performed.
        raw = self.metadata_approved()
        protocol = json.loads(raw[1])
        review = json.loads(raw[3])
        review['protocol_and_dynamic_fixture_decision'] = {
            'status': 'approved', 'reviewer': 'synthetic-test-only',
            'reviewed_at': '2026-10-03T00:00:00Z',
            'protocol_sha256_after': INPUTS.digest(raw[1]),
            'fixture_manifest_sha256_after': INPUTS.digest(raw[2])}
        raw[3] = json.dumps(review).encode()
        report, selected = INPUTS.inspect(*raw, partition='final')
        self.assertEqual(report['status'], 'approved_input_definition')
        self.assertEqual(report['dynamic_approved_count'], 12)
        self.assertIn('runtime', report['scope'])
        with tempfile.TemporaryDirectory() as temp:
            out = Path(temp) / 'synthetic-final'
            INPUTS.export(report, selected, out)
            self.assertEqual(len(list(out.glob('*.yaml'))), 8)
            abstention = json.loads((out / 'b0-abs-01.yaml').read_text())
            self.assertTrue(abstention['expect_no_citations'])
            self.assertNotIn('candidate_answer', abstention)
        # Stale or absent human bindings cannot publish held-out inputs.
        for field in ('protocol_sha256_after', 'fixture_manifest_sha256_after', 'reviewed_at'):
            changed = json.loads(raw[3])
            changed['protocol_and_dynamic_fixture_decision'][field] = 'stale'
            mutated = list(raw); mutated[3] = json.dumps(changed).encode()
            pending, _ = INPUTS.inspect(*mutated)
            self.assertEqual(pending['status'], 'pending')
        # Changing only a status cannot substitute for reviewer/time metadata.
        protocol.update(reviewer=None, reviewed_at=None)
        raw[1] = json.dumps(protocol).encode()
        pending, _ = INPUTS.inspect(*raw)
        self.assertIn('protocol_review_pending', pending['pending_reasons'])


if __name__ == '__main__':
    unittest.main()
