#!/usr/bin/env python3
"""Keep source/fixture observations separate from independent defect acceptance."""
import copy
import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import unittest
import evidence_revision

ROOT=Path(__file__).resolve().parents[1]
PACKET=ROOT/'docs/spec-driven/operational-preparation/source-review-supplement/ticket-gap-v1'
spec=importlib.util.spec_from_file_location('ticket_gap_guard',ROOT/'scripts/check-ticket-gap.py')
guard=importlib.util.module_from_spec(spec);spec.loader.exec_module(guard)


class TicketGapTests(unittest.TestCase):
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

    def ticket(self,action):
        self.change('private-ticket-gaps.json',action)
        self.reject('ticket gap accounting: private-ticket-gaps.json')

    def history(self,action):
        self.change('historical-evidence-scope.json',action)
        self.reject('ticket gap accounting: historical-evidence-scope.json')

    def test_real_packet(self):
        self.assertEqual(guard.check(PACKET),[])

    def test_candidate_count_is_not_ticket_count(self):
        self.change('gap-summary.json',lambda d:d['counts'].update(approved_independent_tickets=2))
        self.reject('ticket gap accounting: gap-summary.json')

    def test_five_scenarios_are_not_independent_tickets(self):
        self.ticket(lambda d:d.update(independent_ticket=['scenario']*5))

    def test_same_ai_wait_graph_is_not_independent_oracle(self):
        self.ticket(lambda d:d.update(independently_derived_oracle=d['secondary_oracle_ai_draft']))

    def test_missing_acceptable_set_is_not_approved_empty_set(self):
        self.ticket(lambda d:d.update(independent_acceptable_candidate_set=[]))

    def test_ai_fixture_candidates_are_not_gold(self):
        self.ticket(lambda d:d.update(independent_acceptable_candidate_set=['CAND-A','CAND-B']))

    def test_candidate_membership_not_inferred(self):
        self.ticket(lambda d:d['candidates'][0].update(independent_acceptable_membership=True))

    def test_actual_api_receipt_not_invented(self):
        self.ticket(lambda d:d['api_route_draft'].update(runtime_receipt='passed'))

    def test_subscription_route_not_normal_rpc_method(self):
        self.ticket(lambda d:d['api_route_draft'].update(subscribe_method='eth_syncing'))

    def test_rpc_unsubscribe_not_internal_acknowledgement(self):
        self.ticket(lambda d:next(r for r in d['source_anchors'] if r['id']=='API-06').update(gap='RPC response proves downstream confirmation'))

    def test_mux_lifetime_unknown_preserved(self):
        self.ticket(lambda d:next(r for r in d['source_anchors'] if r['id']=='API-10').update(gap=''))

    def test_original_and_reintroduction_not_distinct_defects(self):
        self.ticket(lambda d:d.update(reintroduction_bytes_equal_original=False))

    def test_rescue_completion_not_pre_rescue_success(self):
        self.ticket(lambda d:next(r for r in d['local_matrix'] if r['variant']=='ORIGINAL').update(completed_before_rescue=5))

    def test_normal_and_race_not_independent_units(self):
        self.ticket(lambda d:d['local_matrix'][0].update(independent_units=5))

    def test_fixture_not_actual_api(self):
        self.ticket(lambda d:d['local_matrix'][0].update(actual_api_runs=5))

    def test_no_project_patch_adopted(self):
        self.ticket(lambda d:d['candidates'][0].update(applied_project_tree='a'*40))

    def test_performance_measurement_not_invented(self):
        self.ticket(lambda d:d['candidates'][0].update(performance_or_fairness_measurements={'latency':0}))

    def test_missing_obligation_not_satisfied(self):
        self.ticket(lambda d:d['obligations'].update(installed_four_role_handoff_and_final_head='satisfied'))

    def test_history_execution_success_preserved(self):
        self.history(lambda d:d['v1'].update(oracle_runs=0,mechanical_verdict='not_run'))

    def test_historical_model_panel_not_erased(self):
        self.history(lambda d:d['v2'].update(panel_origin='fixture'))

    def test_historical_fixture_not_current_ticket(self):
        self.history(lambda d:d.update(current_development_ticket_covered=True))

    def test_historical_panel_not_human_acceptance(self):
        self.history(lambda d:d['v2'].update(independent_human_acceptance='approved'))

    def test_superseded_initial_attempt_not_final(self):
        self.history(lambda d:d['v2']['initial_attempt_scope'].update(status='final completion evidence'))

    def test_replay_does_not_become_host_execution(self):
        self.change('local-replay.json',lambda d:d.update(kind='installed_host_execution'))
        self.reject('ticket gap accounting: local-replay.json')

    def test_incomplete_alternative_cannot_be_called_bad(self):
        self.change('local-replay.json',lambda d:next(r for r in d['observations'] if r['id']=='incomplete_alternative').update(actual='SELECTED'))
        self.reject('ticket gap accounting: local-replay.json')

    def test_all_bad_tie_and_missing_remain_distinct(self):
        rows={r['id']:r for r in guard.prepare.replay(self.inputs)['observations']}
        for name,status in [('all_bad','REJECTED'),('tie','TIED'),('missing_metric','DEFERRED'),('timeout','ERROR'),('no_test','error')]:
            self.assertEqual(rows[name]['actual'],status)

    def test_formal_execution_not_promoted(self):
        self.change('formal-state.json',lambda d:d.update(execution_allowed=True))
        self.reject('formal state changed')

    def test_public_oracle_leak_rejected(self):
        self.change('public-ticket.json',lambda d:d.update(acceptable_candidates=['CAND-A']))
        self.reject('ticket gap accounting: public-ticket.json')

    def test_project_source_tamper(self):
        p=self.packet/'source/project/sources/rpc/handler.go.txt';p.write_bytes(p.read_bytes()+b'\n');self.rehash()
        self.assertTrue(any('commit-to-source binding mismatch' in e for e in guard.check(self.packet,check_references=False)))

    def test_consumer_source_tamper_before_execution(self):
        p=self.packet/'source/consumer/sources/plugin/scripts/fix_selection.py.txt';p.write_bytes(p.read_bytes()+b'\n');self.rehash()
        self.assertTrue(any('commit-to-source binding mismatch' in e for e in guard.check(self.packet,check_references=False)))

    def test_object_text_normalization_rejected(self):
        (self.packet/'.gitattributes').write_text('source/** text\n');self.rehash()
        self.reject('Git object preservation attributes')

    def test_input_oracle_promotion_rejected(self):
        d=copy.deepcopy(self.inputs);d['private']['independently_derived_oracle']={'approved':True}
        with self.assertRaisesRegex(ValueError,'gold promotion'):guard.prepare.generate(d)

    def test_inventory_and_unknowns(self):
        out=guard.prepare.generate(self.inputs);c=out['gap-summary.json']['counts']
        self.assertEqual((c['development_ticket_drafts'],c['ai_source_candidate_drafts'],c['approved_independent_tickets']),(1,2,0))
        self.assertEqual((c['project_source_anchors'],c['consumer_contract_anchors'],c['local_pure_function_checks']),(13,14,22))
        self.assertIsNone(out['private-ticket-gaps.json']['independent_acceptable_candidate_set'])
        self.assertTrue(all(v=='not_satisfied' for v in out['private-ticket-gaps.json']['obligations'].values()))


# Later stages changed inputs this packet binds; run at its recorded revision.
load_tests = evidence_revision.tests(__file__)


if __name__=='__main__':unittest.main()
