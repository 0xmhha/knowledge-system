#!/usr/bin/env python3
"""Adversarial tests for rule source preparation and distinct denominators."""
import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]
PACKET = ROOT / 'docs/spec-driven/operational-preparation/source-review-supplement/rule-coverage-v1'
spec = importlib.util.spec_from_file_location('rule_guard', ROOT / 'scripts/check-rule-coverage.py')
guard = importlib.util.module_from_spec(spec)
spec.loader.exec_module(guard)


class RuleCoverageTests(unittest.TestCase):
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
        self.assertIn(message, guard.check(self.packet, check_references=False))

    def test_real_packet(self):
        self.assertEqual(guard.check(PACKET), [])

    def test_public_label_injection(self):
        self.change('public-inputs.json', lambda d: d['cases'][0].update(expected_count=1))
        self.rejected('source/proposal accounting: public-inputs.json')

    def test_public_source_replaced_and_rehashed(self):
        def alter(d):
            d['cases'][0]['code'] = 'func F(){}'
            d['cases'][0]['source_sha256'] = guard.sha(b'func F(){}')
        self.change('public-inputs.json', alter)
        self.rejected('source/proposal accounting: public-inputs.json')

    def test_private_approved(self):
        self.change('private-ai-review.json', lambda d: d['cases'][0].update(approved=True, gold=True))
        self.rejected('source/proposal accounting: private-ai-review.json')

    def test_sanitizer_candidate_declared_safe(self):
        self.change('private-ai-review.json', lambda d: next(r for r in d['cases'] if 'sanitizer' in r['coverage_tags']).update(safety_verdict='safe'))
        self.rejected('source/proposal accounting: private-ai-review.json')

    def test_actual_detection_fabricated(self):
        self.change('private-ai-review.json', lambda d: d['cases'][0].update(actual_detection=1))
        self.rejected('source/proposal accounting: private-ai-review.json')

    def test_independent_family_fabricated(self):
        self.change('private-ai-review.json', lambda d: d['cases'][0].update(family_independence='approved'))
        self.rejected('source/proposal accounting: private-ai-review.json')

    def test_final_split_fabricated(self):
        self.change('public-inputs.json', lambda d: d['cases'][0].update(split='FINAL', scoring_allowed=True))
        self.rejected('source/proposal accounting: public-inputs.json')

    def test_external_target_proof_fabricated(self):
        self.change('contract.json', lambda d: d.update(target_resolution='import_resolved'))
        self.rejected('source/proposal accounting: contract.json')

    def test_sanitizer_proof_fabricated(self):
        self.change('contract.json', lambda d: d.update(sanitizer='proven'))
        self.rejected('source/proposal accounting: contract.json')

    def test_source_to_vulnerability_promotion(self):
        self.change('contract.json', lambda d: d.update(vulnerability_verdict='confirmed'))
        self.rejected('source/proposal accounting: contract.json')

    def test_annotation_queries_counted_as_rules(self):
        self.change('coverage-summary.json', lambda d: d['counts'].update(controlled_source_drafts=52))
        self.rejected('source/proposal accounting: coverage-summary.json')

    def test_project_denominator_filled_from_controls(self):
        self.change('coverage-summary.json', lambda d: d['project_call_denominators'].update({'discarded-conversion-error': 68}))
        self.rejected('source/proposal accounting: coverage-summary.json')

    def test_project_syntax_absence_declared_negative(self):
        self.change('project-source-review.json', lambda d: d['calls'][0].update(actual_detection=0, safety_verdict='safe'))
        self.rejected('source/proposal accounting: project-source-review.json')

    def test_project_call_span_tamper(self):
        self.change('project-source-review.json', lambda d: d['calls'][0].update(raw_call_text='exec.Command(cmd)'))
        self.rejected('source/proposal accounting: project-source-review.json')

    def test_existing_fixtures_declared_new(self):
        self.change('existing-fixtures.json', lambda d: d[0].update(status='new_independent_gold'))
        self.rejected('source/proposal accounting: existing-fixtures.json')

    def test_quality_score_fabricated(self):
        self.change('coverage-summary.json', lambda d: d.update(precision=1.0, recall=1.0))
        self.rejected('source/proposal accounting: coverage-summary.json')

    def test_parser_result_promoted_to_analyzer(self):
        self.change('syntax-validation.json', lambda d: d.update(rule_backend_calls=40))
        self.rejected('isolated syntax validation')

    def test_parser_failure_hidden(self):
        self.change('syntax-validation.json', lambda d: d['records'][0].update(exit_code=1))
        self.rejected('isolated syntax validation')

    def test_formal_state_promotion(self):
        self.change('formal-state.json', lambda d: d.update(execution_allowed=True))
        self.rejected('formal state promotion')

    def test_unexpected_private_file_in_public_bundle(self):
        (self.packet / 'public-gold.json').write_text('{}')
        self.rehash()
        self.rejected('unexpected artifact')

    def test_generator_tamper(self):
        with (self.packet / 'generator.py.txt').open('a') as f:
            f.write('\n# changed\n')
        self.rehash()
        self.rejected('generator snapshot')

    def test_regeneration_byte_identical(self):
        output = self.packet.parent / 'regenerated'
        guard.prepare.prepare(output)
        self.assertEqual({p.relative_to(PACKET): p.read_bytes() for p in PACKET.rglob('*') if p.is_file()},
                         {p.relative_to(output): p.read_bytes() for p in output.rglob('*') if p.is_file()})


if __name__ == '__main__':
    unittest.main()
