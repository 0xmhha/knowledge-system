#!/usr/bin/env python3
"""Reject synthetic/fixture evidence promotion and incompatible metric matching."""
import copy
import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import unittest

ROOT=Path(__file__).resolve().parents[1]
PACKET=ROOT/'docs/spec-driven/operational-preparation/source-review-supplement/schedule-gap-v1'
spec=importlib.util.spec_from_file_location('schedule_gap_guard',ROOT/'scripts/check-schedule-gap.py')
guard=importlib.util.module_from_spec(spec)
spec.loader.exec_module(guard)


class ScheduleGapTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.inputs=guard.prepare.inputs(PACKET)

    def setUp(self):
        tmp=tempfile.TemporaryDirectory();self.addCleanup(tmp.cleanup)
        self.packet=Path(tmp.name)/'packet';shutil.copytree(PACKET,self.packet)

    def rehash(self):
        p=self.packet/'manifest.json';d=json.loads(p.read_text())
        d['artifacts']={p.relative_to(self.packet).as_posix():guard.sha(p.read_bytes()) for p in self.packet.rglob('*') if p.is_file() and p.name!='manifest.json'}
        p.write_text(json.dumps(d))

    def change(self,name,action):
        p=self.packet/name;d=json.loads(p.read_text());action(d)
        p.write_text(json.dumps(d,ensure_ascii=False));self.rehash()

    def reject(self,message):
        self.assertIn(message,guard.check(self.packet,check_references=False))

    def family(self,action,cid=None):
        self.change('private-family-gaps.json',lambda d:action(next(r for r in d['families'] if cid is None or r['case_id']==cid)))
        self.reject('schedule gap accounting: private-family-gaps.json')

    def runtime(self,action):
        self.change('execution-contract-review.json',action)
        self.reject('schedule gap accounting: execution-contract-review.json')

    def test_real_packet(self):
        self.assertEqual(guard.check(PACKET),[])

    def test_schedules_promoted_to_families(self):
        self.change('gap-summary.json',lambda d:d['counts'].update(approved_independent_families=21))
        self.reject('schedule gap accounting: gap-summary.json')

    def test_pair_review_omitted(self):
        self.change('pairwise-family-review.json',lambda d:d.pop())
        self.reject('schedule gap accounting: pairwise-family-review.json')

    def test_different_mechanisms_become_independence(self):
        self.change('pairwise-family-review.json',lambda d:d[0].update(semantic_independence='approved'))
        self.reject('schedule gap accounting: pairwise-family-review.json')

    def test_source_reachability_invented(self):
        self.family(lambda r:r.update(target_reachability_receipt='passed'))

    def test_independent_oracle_invented(self):
        self.family(lambda r:r.update(independent_oracles=['primary','independent']))

    def test_unknown_contracts_removed(self):
        self.family(lambda r:r.update(existing_unknowns=[],existing_contract_questions=[]))

    def test_worker_dispatch_anchor_forged(self):
        self.family(lambda r:r['entry_review_draft']['anchor'].update(span_sha256='0'*64),'SCHED-003')

    def test_simultaneous_select_forced_to_failure(self):
        self.change('schedule-obligations.json',lambda d:next(r for r in d if r['schedule']['id']=='WORK-both')['schedule'].update(primary_expectation_draft='exit must always win'))
        self.reject('schedule gap accounting: schedule-obligations.json')

    def test_schedule_omitted(self):
        self.change('schedule-obligations.json',lambda d:d.pop())
        self.reject('schedule gap accounting: schedule-obligations.json')

    def test_missing_trials_become_one_hundred(self):
        self.family(lambda r:r.update(baseline_valid_trials=100,fix_valid_trials=100))

    def test_unknown_symptom_count_becomes_observed_zero(self):
        self.family(lambda r:r.update(baseline_symptom_events=0))

    def test_fix_plan_frozen_without_review(self):
        self.family(lambda r:r.update(fix_plan={'commit':'a'*40}))

    def test_600_second_receipt_invented(self):
        self.family(lambda r:r.update(mode9_600_second_receipt='passed'))

    def test_timing_neutrality_promoted(self):
        self.family(lambda r:r.update(timing_neutrality='proven'))

    def test_normal_race_become_arms(self):
        self.runtime(lambda d:d['subscription_extracted_matrix'][0].update(mode='baseline',operational_trials=100))

    def test_local_observations_inflated(self):
        self.change('gap-summary.json',lambda d:d['counts'].update(subscription_local_observations=100))
        self.reject('schedule gap accounting: gap-summary.json')

    def test_wall_clock_reproduction_invented(self):
        self.runtime(lambda d:d.update(wall_clock_rollback_reproduced=True))

    def test_old_sixty_trial_fixture_becomes_current_acceptance(self):
        self.runtime(lambda d:d['historical_comparison'].update(current_five_families_covered=5,acceptance='passed'))

    def test_short_window_becomes_ten_minutes(self):
        self.runtime(lambda d:d['historical_mode9']['windows'][0].update(planned_seconds=600))

    def test_no_recurrence_becomes_fix_success(self):
        self.runtime(lambda d:d['historical_mode9']['windows'][0].update(verdict='FIX_ACCEPTED'))

    def test_absolute_bound_becomes_relative_ratio(self):
        self.runtime(lambda d:d['metric_boundary'].update(equivalence_established=True))

    def test_cluster_method_silently_replaced(self):
        self.runtime(lambda d:d['metric_boundary']['criteria_snapshot']['statistics_proposal'].update(method='independent trials only'))

    def test_cleanup_receipt_invented(self):
        self.family(lambda r:r.update(owned_cleanup_receipt='passed'))

    def test_formal_execution_promotion(self):
        self.change('formal-state.json',lambda d:d.update(execution_allowed=True))
        self.reject('formal state changed')

    def test_public_oracle_injection(self):
        self.change('public-inputs.json',lambda d:d['cases'][0].update(expected_result='private oracle'))
        self.reject('schedule gap accounting: public-inputs.json')

    def test_adopted_source_tamper(self):
        p=self.packet/'adopted-source/chainbench/sources/internal/experiment/run.go.txt';p.write_bytes(p.read_bytes()+b'\n');self.rehash()
        self.assertTrue(any('commit-to-source binding mismatch' in e for e in guard.check(self.packet,check_references=False)))

    def test_object_text_normalization_policy_rejected(self):
        (self.packet/'.gitattributes').write_text('adopted-source/** text\n');self.rehash()
        self.reject('Git object preservation attributes')

    def test_duplicate_family_input(self):
        d=copy.deepcopy(self.inputs);d['private']['families'].append(d['private']['families'][0])
        with self.assertRaisesRegex(ValueError,'duplicate'):
            guard.prepare.generate(d)

    def test_inventory_properties(self):
        out=guard.prepare.generate(self.inputs);c=out['gap-summary.json']['counts']
        self.assertEqual((c['candidate_families'],c['schedule_proposals'],c['approved_independent_families']),(5,21,0))
        self.assertEqual((c['subscription_local_observations'],c['udp_synthetic_observations']),(50,20))
        self.assertEqual(len(out['pairwise-family-review.json']),10)
        for f in out['private-family-gaps.json']['families']:
            self.assertIsNone(f['baseline_valid_trials']);self.assertIsNone(f['baseline_symptom_events'])
            self.assertTrue(all(v=='not_satisfied' for v in f['obligations'].values()))


if __name__=='__main__':
    unittest.main()
