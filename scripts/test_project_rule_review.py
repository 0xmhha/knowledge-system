#!/usr/bin/env python3
"""Adversarial project provenance, closure, denominator and acceptance tests."""
import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]
PACKET = ROOT / 'docs/spec-driven/operational-preparation/source-review-supplement/project-rule-review-v1'
spec = importlib.util.spec_from_file_location('project_rule_guard', ROOT / 'scripts/check-project-rule-review.py')
guard = importlib.util.module_from_spec(spec)
spec.loader.exec_module(guard)


class ProjectRuleReviewTests(unittest.TestCase):
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

    def rejected(self, fragment):
        errors = guard.check(self.packet, check_references=False)
        self.assertTrue(errors)
        self.assertTrue(any(fragment in e for e in errors), errors)

    def test_real_packet(self):
        self.assertEqual(guard.check(PACKET), [])

    def test_source_commit_substitution(self):
        self.change('git-proof.json', lambda d: d.update(commit='a' * 40))
        self.rejected('unexpected source revision')

    def test_source_bytes_rehashed_without_git_proof(self):
        proof = json.loads((self.packet / 'git-proof.json').read_text())
        artifact = next(iter(proof['files'].values()))['artifact']
        p = self.packet / artifact
        p.write_bytes(p.read_bytes() + b'\n// forged\n')
        self.rehash()
        self.rejected('commit-to-source binding mismatch')

    def test_tree_object_rehashed(self):
        proof = json.loads((self.packet / 'git-proof.json').read_text())
        artifact = next(v for k, v in proof['objects'].items() if k.startswith('tree:'))
        p = self.packet / artifact
        p.write_bytes(p.read_bytes() + b'forged')
        self.rehash()
        self.rejected('Git object hash mismatch')

    def test_source_path_escape(self):
        self.change('git-proof.json', lambda d: next(iter(d['files'].values())).update(artifact='../escape.txt'))
        self.rejected('unsafe source artifact')

    def test_public_answer_injection(self):
        self.change('public-bundle/public-inputs.json', lambda d: d['cases'][0].update(expected_candidates=0))
        self.rejected('project public source payload')

    def test_public_code_replaced_and_rehashed(self):
        def alter(d):
            r = d['cases'][0]
            r['code'] = 'func F(){}'
            r['source']['sha256'] = guard.sha(r['code'].encode())
        self.change('public-bundle/public-inputs.json', alter)
        self.rejected('project public source payload')

    def test_real_source_coordinates_replaced(self):
        self.change('public-bundle/public-inputs.json', lambda d: d['cases'][0]['source'].update(start_line=1, start_byte=0))
        self.rejected('project public source payload')

    def test_synthetic_snapshot_declared_real(self):
        self.change('public-bundle/public-inputs.json', lambda d: d.update(snapshot_kind='actual_retained_graph'))
        self.rejected('project public source payload')

    def test_canonical_graph_identity_invented(self):
        self.change('public-bundle/public-inputs.json', lambda d: d['cases'][0]['source'].update(canonical_id='proven.canonical.F'))
        self.rejected('project public source payload')

    def test_private_file_in_public_bundle(self):
        (self.packet / 'public-bundle/gold.json').write_text('{}')
        self.rehash()
        self.rejected('private data in project public bundle')

    def test_live_rule_binding_removed(self):
        self.change('manifest.json', lambda d: d['input_bindings'].pop('pkg/graph/valueflow/rules.go'))
        self.rejected('live project observer/input binding')

    def test_overlay_deletes_existing_tests(self):
        (self.packet / 'overlay-base.go.txt').write_text('package valueflow\n')
        self.rehash()
        self.rejected('project trusted source snapshot: overlay-base.go.txt')

    def test_overlay_scope_expanded(self):
        self.change('overlay.json.txt', lambda d: d['Replace'].update({'/private/tmp/other.go': '/private/tmp/other.txt'}))
        self.rejected('project overlay scope')

    def test_native_source_pin_forged(self):
        self.change('observations.json', lambda d: d[0]['analysis']['source'].update(sha256='0' * 64))
        self.rejected('actual project analyzer replay mismatch')

    def test_partial_type_diagnostics_erased(self):
        self.change('observations.json', lambda d: d[0]['analysis'].update(status='fully_typed', diagnostics=[]))
        self.rejected('actual project analyzer replay mismatch')

    def test_closure_scope_collapsed(self):
        self.change('syntax-controls.json', lambda d: d[0]['syntax_calls'][0].update(closure_depth=0))
        self.rejected('actual project syntax control replay mismatch')

    def test_closure_call_declared_negative(self):
        def alter(d):
            row = next(r for r in d if r['syntax']['closure_depth'])
            row.update(observation_status='confirmed_negative', independent_label='negative')
        self.change('call-review.json', alter)
        self.rejected('project derived accounting/boundary: call-review.json')

    def test_candidate_declared_vulnerability(self):
        self.change('candidate-review.json', lambda d: d[0].update(safety_verdict='confirmed_vulnerability', approved=True))
        self.rejected('project derived accounting/boundary: candidate-review.json')

    def test_runtime_reachability_invented(self):
        self.change('candidate-review.json', lambda d: d[0].update(runtime_reachability='production_reachable'))
        self.rejected('project derived accounting/boundary: candidate-review.json')

    def test_same_file_functions_declared_independent(self):
        self.change('function-review.json', lambda d: next(r for r in d if r['candidate_ids']).update(independence_approved=True))
        self.rejected('project derived accounting/boundary: function-review.json')

    def test_empty_calls_promoted_to_negative_gold(self):
        self.change('coverage-summary.json', lambda d: d['counts'].update(independent_negative_gold=66))
        self.rejected('project derived accounting/boundary: coverage-summary.json')

    def test_controlled_examples_added_to_project(self):
        self.change('coverage-summary.json', lambda d: d.update(controlled_cases_added_to_project_denominator=40))
        self.rejected('project derived accounting/boundary: coverage-summary.json')

    def test_original_denominator_reduced_for_closure(self):
        self.change('coverage-summary.json', lambda d: d['counts'].update(original_call_denominator=67))
        self.rejected('project derived accounting/boundary: coverage-summary.json')

    def test_precision_fabricated_without_gold(self):
        self.change('coverage-summary.json', lambda d: d.update(precision=1.0, recall=1.0))
        self.rejected('project derived accounting/boundary: coverage-summary.json')

    def test_formal_execution_promoted(self):
        self.change('formal-state.json', lambda d: d.update(execution_allowed=True))
        self.rejected('project formal state promotion')

    def test_raw_failure_hidden(self):
        with (self.packet / 'logs/normal.jsonl').open('a') as f:
            f.write(json.dumps({'Action': 'fail', 'Test': 'TestProjectRulePreparation'}) + '\n')
        self.rehash()
        self.rejected('failed/skipped/racing project source observation')

    def test_raw_race_mode_divergence(self):
        p = self.packet / 'logs/race.jsonl'
        p.write_text(p.read_text().replace('syntax_only_unverified', 'verified_target'))
        self.rehash()
        self.rejected('project raw native mismatch: race')

    def test_quality_diagnostics_hidden(self):
        (self.packet / 'logs/fmt.stdout.txt').write_text('unformatted.go\n')
        self.rehash()
        self.rejected('project quality findings')

    def test_run_scope_changed(self):
        self.change('validation.json', lambda d: d['runs'][0]['command'].remove('-run=^TestProjectRule'))
        self.rejected('project native run accounting')

    def test_project_execution_claimed(self):
        self.change('validation.json', lambda d: d.update(analyzed_project_code_executed=True))
        self.rejected('project source execution/modification boundary')

    def test_candidate_pin_independently_checked(self):
        observations = json.loads((PACKET / 'observations.json').read_text())
        r = next(r for r in observations if r['analysis']['rules']['candidates'])
        r['analysis']['rules']['candidates'][0]['id'] = '0' * 64
        with self.assertRaisesRegex(ValueError, 'project candidate pin hash mismatch'):
            guard.runner.derive(PACKET, observations)

    def test_native_call_source_span_independently_checked(self):
        observations = json.loads((PACKET / 'observations.json').read_text())
        observations[0]['syntax_calls'][0]['location']['start_byte'] += 1
        with self.assertRaisesRegex(ValueError, 'AST call span is absent/ambiguous'):
            guard.runner.derive(PACKET, observations)


if __name__ == '__main__':
    unittest.main()
