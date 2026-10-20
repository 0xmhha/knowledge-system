#!/usr/bin/env python3
"""Adversarial native-evidence, source-pin and formal-acceptance boundary tests."""
import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import unittest
import evidence_revision

ROOT = Path(__file__).resolve().parents[1]
PACKET = ROOT / 'docs/spec-driven/operational-preparation/source-review-supplement/rule-observations-v1'
spec = importlib.util.spec_from_file_location('rule_observation_guard', ROOT / 'scripts/check-rule-observations.py')
guard = importlib.util.module_from_spec(spec)
spec.loader.exec_module(guard)


class RuleObservationTests(unittest.TestCase):
    def setUp(self):
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        self.packet = Path(tmp.name) / 'packet'
        shutil.copytree(PACKET, self.packet)

    def rehash(self):
        p = self.packet / 'manifest.json'
        d = json.loads(p.read_text())
        d['artifacts'] = {p.relative_to(self.packet).as_posix(): guard.sha(p.read_bytes())
                          for p in self.packet.rglob('*') if p.is_file() and p.name != 'manifest.json'}
        p.write_text(json.dumps(d))

    def change(self, name, fn):
        p = self.packet / name
        d = json.loads(p.read_text())
        fn(d)
        p.write_text(json.dumps(d, ensure_ascii=False))
        self.rehash()

    def rejected(self, message):
        errors = guard.check(self.packet, check_references=False)
        self.assertTrue(errors)
        self.assertTrue(any(message in e for e in errors), errors)

    def test_real_packet(self):
        self.assertEqual(guard.check(PACKET), [])

    def test_public_gold_injection(self):
        self.change('public-bundle/public-inputs.json', lambda d: d['cases'][0].update(expected_count=1))
        self.rejected('public-only input payload')

    def test_private_file_in_public_bundle(self):
        (self.packet / 'public-bundle/private.json').write_text('{}')
        self.rehash()
        self.rejected('private data in public bundle')

    def test_template_replaced_and_rehashed(self):
        (self.packet / 'template.go.txt').write_text('package valueflow\n')
        self.rehash()
        self.rejected('overlay changes trusted template or existing tests')

    def test_existing_tests_deleted(self):
        (self.packet / 'overlay-base.go.txt').write_text('package valueflow\n')
        self.rehash()
        self.rejected('overlay changes trusted template or existing tests')

    def test_overlay_scope_expanded(self):
        self.change('overlay.json.txt', lambda d: d['Replace'].update({'/private/tmp/other.go': '/private/tmp/fake.go'}))
        self.rejected('overlay scope')

    def test_live_source_binding_removed(self):
        self.change('manifest.json', lambda d: d['input_bindings'].pop('pkg/graph/valueflow/rules.go'))
        self.rejected('live analyzer/input binding')

    def test_candidate_id_replaced(self):
        self.change('observations.json', lambda d: d[0]['analysis']['rules']['candidates'][0].update(id='f' * 64))
        self.rejected('actual analyzer replay mismatch')

    def test_sink_span_replaced(self):
        self.change('observations.json', lambda d: d[0]['analysis']['rules']['candidates'][0]['sink_location'].update(start_byte=401))
        self.rejected('actual analyzer replay mismatch')

    def test_sanitizer_claim_promoted(self):
        self.change('observations.json', lambda d: d[4]['analysis']['rules']['candidates'][0].update(sanitizer='proven_safe'))
        self.rejected('actual analyzer replay mismatch')

    def test_target_claim_promoted(self):
        self.change('observations.json', lambda d: d[6]['analysis']['rules']['candidates'][0].update(target_resolution='resolved_os_exec'))
        self.rejected('actual analyzer replay mismatch')

    def test_shadow_bindings_collapsed(self):
        def alter(d):
            d[3]['analysis']['rules']['candidates'][0]['source_occurrence'] = d[3]['analysis']['rules']['candidates'][0]['sink_occurrence']
        self.change('observations.json', alter)
        self.rejected('actual analyzer replay mismatch')

    def test_partial_type_diagnostics_erased(self):
        self.change('observations.json', lambda d: d[0]['analysis'].update(diagnostics=[], status='fully_typed'))
        self.rejected('actual analyzer replay mismatch')

    def test_false_runtime_path_claim(self):
        self.change('case-review.json', lambda d: d[0].update(runtime_path_feasible=True))
        self.rejected('derived source/quality boundary: case-review.json')

    def test_condition_source_fabricated(self):
        def alter(d):
            r = next(r for r in d if any(c['conditions'] for c in r['candidate_spans']))
            r['candidate_spans'][0]['conditions'][0]['source_text'] = 'fabricated'
        self.change('case-review.json', alter)
        self.rejected('derived source/quality boundary: case-review.json')

    def test_range_unknown_location_fabricated(self):
        def alter(d):
            r = next(r for r in d if r['control_id'] == 'range_unknown')
            c = next(c for c in r['observation']['rules']['candidates'][0]['conditions'] if c['location_status'] == 'structural_block_unlocated')
            c['location'] = {'start_byte': 400, 'end_byte': 401, 'start_line': 20, 'end_line': 20}
        self.change('controls.json', alter)
        self.rejected('actual control replay mismatch')

    def test_cap_stop_promoted_to_complete(self):
        self.change('controls.json', lambda d: d[0]['observation']['rules'].update(status='matched_bounded'))
        self.rejected('actual control replay mismatch')

    def test_raw_failure_hidden_by_pass_summary(self):
        p = self.packet / 'logs/normal.jsonl'
        with p.open('a') as f:
            f.write(json.dumps({'Action': 'fail', 'Test': 'TestRulePreparationInputs'}) + '\n')
        self.rehash()
        self.rejected('failed/skipped/racing native observation')

    def test_raw_race_mode_divergence(self):
        p = self.packet / 'logs/race.jsonl'
        text = p.read_text().replace('syntax_only_unverified', 'resolved_target')
        p.write_text(text)
        self.rehash()
        self.rejected('raw native observation/control mismatch: race')

    def test_regression_skip_hidden(self):
        p = self.packet / 'logs/regression-normal.jsonl'
        with p.open('a') as f:
            f.write(json.dumps({'Action': 'skip', 'Test': 'TestRulePackScalarCommandCorpus'}) + '\n')
        self.rehash()
        self.rejected('raw existing regression completion')

    def test_quality_finding_hidden(self):
        (self.packet / 'logs/lint.stdout.txt').write_text('one lint finding\n')
        self.rehash()
        self.rejected('native quality finding: lint')

    def test_wrong_test_run_scope(self):
        self.change('validation.json', lambda d: d['runs'][0]['command'].remove('-run=^TestRulePreparation'))
        self.rejected('native run accounting')

    def test_latency_result_fabricated(self):
        self.change('coverage-summary.json', lambda d: d.update(latency_measurement='p95_benchmark_passed'))
        self.rejected('derived source/quality boundary: coverage-summary.json')

    def test_controlled_counts_promoted_to_project(self):
        self.change('coverage-summary.json', lambda d: d.update(project_cases_analyzed=40))
        self.rejected('derived source/quality boundary: coverage-summary.json')

    def test_ai_agreement_promoted_to_precision(self):
        self.change('coverage-summary.json', lambda d: d.update(precision=1.0, recall=1.0))
        self.rejected('derived source/quality boundary: coverage-summary.json')

    def test_independent_gold_fabricated(self):
        self.change('coverage-summary.json', lambda d: d['counts'].update(independent_gold_records=40))
        self.rejected('derived source/quality boundary: coverage-summary.json')

    def test_formal_execution_promoted(self):
        self.change('formal-state.json', lambda d: d.update(execution_allowed=True))
        self.rejected('formal state promotion')

    def test_pack_hash_is_checked_independently(self):
        records = json.loads((PACKET / 'observations.json').read_text())
        records[0]['analysis']['rules']['pack_sha256'] = 'd' * 64
        with self.assertRaisesRegex(ValueError, 'unexpected rule pack/completion'):
            guard.runner.derive(records)

    def test_occurrence_location_is_checked_independently(self):
        records = json.loads((PACKET / 'observations.json').read_text())
        records[0]['analysis']['rules']['candidates'][0]['sink_location']['start_byte'] += 1
        with self.assertRaisesRegex(ValueError, 'candidate occurrence identity mismatch'):
            guard.runner.derive(records)

    def test_candidate_pin_is_checked_independently(self):
        records = json.loads((PACKET / 'observations.json').read_text())
        records[0]['analysis']['rules']['candidates'][0]['id'] = 'd' * 64
        with self.assertRaisesRegex(ValueError, 'candidate pin hash mismatch'):
            guard.runner.derive(records)


# Later stages changed inputs this packet binds; run at its recorded revision.
load_tests = evidence_revision.tests(__file__)


if __name__ == '__main__':
    unittest.main()
