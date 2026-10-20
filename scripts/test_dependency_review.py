#!/usr/bin/env python3
"""Tamper tests for private source evidence, scope identity and acceptance claims."""
import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]
PACKET = ROOT / 'docs/spec-driven/operational-preparation/source-review-supplement/dependency-review-v1'
spec = importlib.util.spec_from_file_location('dependency_guard', ROOT / 'scripts/check-dependency-review.py')
guard = importlib.util.module_from_spec(spec)
spec.loader.exec_module(guard)


class DependencyReviewTests(unittest.TestCase):
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

    def rejected(self, message=None):
        errors = guard.check(self.packet, check_references=False)
        self.assertTrue(errors)
        if message:
            self.assertIn(message, errors)

    def test_real_packet(self):
        self.assertEqual(guard.check(PACKET), [])

    def test_binding_substitution_even_after_rehash(self):
        def alter(observations):
            rows = observations[0]['functions'][0]['occurrences']
            o = next(o for o in rows if o['name'] == 'ok' and o['site']['start_line'] == 191 and o['role'] == 'read')
            o['declaration'] = next(o['declaration'] for o in rows if o['name'] == 'ok' and o['site']['start_line'] == 189)
        self.change('logs/locations.json', alter)
        self.rejected()

    def test_continue_owner_changed_to_outer_loop(self):
        def alter(observations):
            rows = observations[2]['functions'][0]['markers']
            m = next(m for m in rows if m['kind'] == 'continue' and m['site']['start_line'] == 187)
            m['target'] = next(m['site'] for m in rows if m['kind'] == 'for' and m['site']['start_line'] == 138)
        self.change('logs/locations.json', alter)
        self.rejected('actual source observation replay mismatch')

    def test_return_site_changed(self):
        def alter(observations):
            m = next(m for m in observations[3]['functions'][0]['markers'] if m['kind'] == 'return')
            m['site']['start_line'] += 1
        self.change('logs/locations.json', alter)
        self.rejected('actual source observation replay mismatch')

    def test_incomplete_project_check_promoted(self):
        self.change('validation.json', lambda d: d.update(project_typecheck_passed=True))
        self.rejected('validation scope')

    def test_partial_diagnostics_erased(self):
        self.change('logs/locations.json', lambda d: d[0].update(partial_type_error_count=0, partial_type_error_examples=[]))
        self.rejected('partial typecheck boundary')

    def test_reference_promoted_to_scalar_gold(self):
        self.change('private-review.json', lambda d: d['data_reviews'][3].update(formal_edge_kind='data_dependence', value_domain='scalar'))
        self.rejected('private review locations/claims mismatch')

    def test_definition_list_marked_exhaustive(self):
        self.change('private-review.json', lambda d: d['data_reviews'][0].update(complete_definition_set=True))
        self.rejected('private review locations/claims mismatch')

    def test_range_support_invented(self):
        def alter(d):
            next(r for r in d['data_reviews'] if r['backend_mapping'] == 'range_binding_unsupported')['backend_mapping'] = 'scalar_supported'
        self.change('private-review.json', alter)
        self.rejected('private review locations/claims mismatch')

    def test_postdominance_invented_from_exit_owner(self):
        self.change('private-review.json', lambda d: d['boundaries'][0].update(formal_edge_kind='control_dependence'))
        self.rejected('private review locations/claims mismatch')

    def test_exclusion_promoted_to_independent_verdict(self):
        self.change('private-review.json', lambda d: d['exclusion_reviews'][0].update(approved=True, independence_verified=True))
        self.rejected('private review locations/claims mismatch')

    def test_final_allocation(self):
        self.change('private-review.json', lambda d: d['contexts'][0].update(split='FINAL', final_eligibility='approved'))
        self.rejected('private review locations/claims mismatch')

    def test_precision_score_claim(self):
        self.change('private-review.json', lambda d: d.update(data_precision=1.0))
        self.rejected('private review locations/claims mismatch')

    def test_canonical_matching_freeze(self):
        self.change('matching-contract.json', lambda d: d.update(canonical_matching_frozen=True, scoring_allowed=True))
        self.rejected('matching proposal/freeze mismatch')

    def test_source_selection_changed(self):
        self.change('location-inputs.json', lambda d: d[0]['Starts'].append(0))
        self.rejected('source observer input mismatch')

    def test_engine_snapshot_changed(self):
        p = self.packet / 'engine-source/pkg/graph/valueflow/dependence.go.txt'
        p.write_bytes(p.read_bytes().replace(b'range_binding_unsupported', b'range_binding_supported'))
        self.rehash()
        self.rejected('source binding: pkg/graph/valueflow/dependence.go')

    def test_original_source_substitution(self):
        proof = json.loads((self.packet / 'git-proof.json').read_text())
        record = next(iter(proof['files'].values()))
        p = self.packet / record['artifact']
        p.write_bytes(p.read_bytes() + b'// changed\n')
        record['sha256'] = guard.sha(p.read_bytes())
        (self.packet / 'git-proof.json').write_text(json.dumps(proof))
        self.rehash()
        self.rejected()

    def test_raw_failed_control(self):
        p = self.packet / 'logs/race.jsonl'
        p.write_text(p.read_text() + json.dumps({'Action': 'fail', 'Test': 'TestDependencyReviewLocations'}) + '\n')
        self.rehash()
        self.rejected('raw control completion')

    def test_forged_control_observation(self):
        p = self.packet / 'logs/normal.jsonl'
        p.write_text(p.read_text().replace('pass=true', 'pass=false'))
        self.rehash()
        self.rejected('raw control observations')

    def test_backend_execution_claim(self):
        self.change('validation.json', lambda d: d.update(dependency_backend_calls=4))
        self.rejected('execution boundary')

    def test_old_lint_failure_removed(self):
        (self.packet / 'prior-quality-diagnostic/logs/lint.stdout.txt').write_text('all passed\n')
        self.rehash()
        self.rejected('prior lint finding lost')

    def test_formal_acceptance_promoted(self):
        self.change('formal-state.json', lambda d: d.update(execution_allowed=True))
        self.rejected('formal state promotion')


if __name__ == '__main__':
    unittest.main()
