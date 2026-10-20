#!/usr/bin/env python3
"""Reject forged stdio, pin, cap, source and acceptance evidence after rehashing."""
import importlib.util
import json
from pathlib import Path
import shutil
import sqlite3
from contextlib import closing
import tempfile
import unittest
import evidence_revision

ROOT=Path(__file__).resolve().parents[1]
PACKET=ROOT/'docs/spec-driven/operational-preparation/source-review-supplement/consumer-stdio-readiness-v1'
spec=importlib.util.spec_from_file_location('stdio_guard',ROOT/'scripts/check-consumer-stdio-readiness.py')
guard=importlib.util.module_from_spec(spec);spec.loader.exec_module(guard)


class StdioReadinessTests(unittest.TestCase):
    def setUp(self):
        t=tempfile.TemporaryDirectory();self.addCleanup(t.cleanup);self.packet=Path(t.name)/'packet';shutil.copytree(PACKET,self.packet)
    def rehash(self):
        p=self.packet/'manifest.json';d=json.loads(p.read_text());d['artifacts']={x.relative_to(self.packet).as_posix():guard.sha(x.read_bytes())for x in self.packet.rglob('*')if x.is_file()and x!=p};p.write_text(json.dumps(d))
    def change(self,name,action):
        p=self.packet/name;d=json.loads(p.read_text());action(d);p.write_text(json.dumps(d));self.rehash()
    def rejected(self):self.assertTrue(guard.check(self.packet,False))
    def accounting(self,action):self.change('stdio-readiness.json',action);self.assertIn('stdio accounting: stdio-readiness.json',guard.check(self.packet,False))
    def raw(self,label,action):
        p=self.packet/'sessions/main/messages.json';rows=json.loads(p.read_text());action(next(r for r in rows if r['label']==label));p.write_text(json.dumps(rows))
        # Update both raw streams too: semantic checks must still reject this.
        (p.parent/'stdin.jsonl').write_text('\n'.join(json.dumps(r['request']) for r in rows)+'\n')
        (p.parent/'stdout.jsonl').write_text('\n'.join(json.dumps(r['response']) for r in rows)+'\n');self.rehash()
    def test_current_packet(self):self.assertEqual(guard.check(PACKET),[])
    def test_tools_metadata_not_static_only(self):
        self.raw('tools',lambda r:r['response']['result']['tools'].pop());self.rejected()
    def test_wrong_input_schema(self):
        self.raw('tools',lambda r:next(t for t in r['response']['result']['tools'] if t['name']=='cks.context.policy_security')['inputSchema']['properties']['max_annotations'].update(maximum=999));self.rejected()
    def test_wrong_wire_name(self):
        self.raw('tools',lambda r:r['response']['result']['tools'][0].update(name='wrong.namespace'));self.rejected()
    def test_raw_request_binding(self):
        self.change('sessions/main/messages.json',lambda d:d[0]['request'].update(id=999));self.rejected()
    def test_raw_response_binding(self):
        self.change('sessions/main/messages.json',lambda d:d[0]['response'].update(id=999));self.rejected()
    def test_wrong_computed_pin(self):
        self.raw('baseline',lambda r:r['response']['result']['structuredContent']['analysis']['source'].update(commit='0'*40));self.rejected()
    def test_mismatch_cannot_publish(self):
        self.raw('mismatch-value_dependencies-expected_commit',lambda r:r['response']['result']['structuredContent'].update(status='partial',analysis={}));self.rejected()
    def test_pin_request_must_match_label(self):
        self.raw('mismatch-value_dependencies-expected_commit',lambda r:r['request']['params']['arguments'].update(expected_commit='1'*40));self.rejected()
    def test_rules_require_explicit_pack(self):
        self.raw('rules',lambda r:r['request']['params']['arguments'].update(rule_pack='global'));self.rejected()
    def test_rules_reuse_same_analysis(self):
        self.raw('rules',lambda r:r['response']['result']['structuredContent']['analysis'].update(edges=[]));self.rejected()
    def test_call_hops_not_value_transfer(self):
        self.raw('expansion',lambda r:r['response']['result']['structuredContent']['graph']['hops'][0].update(argument_mapping='proven'));self.rejected()
    def test_manual_metadata_not_authenticated(self):
        self.raw('annotations',lambda r:r['response']['result']['structuredContent']['annotations'][0]['provenance'].update(identity_trust='authenticated'));self.rejected()
    def test_untagged_not_safe(self):
        self.raw('untagged',lambda r:r['response']['result']['structuredContent'].update(safety_verdict='safe'));self.rejected()
    def test_cap_not_hidden(self):
        self.raw('cap-max_candidates',lambda r:r['response']['result']['structuredContent']['graph'].update(status='expanded_bounded'));self.rejected()
    def test_cap_request_real(self):
        self.raw('cap-max_candidates',lambda r:r['request']['params']['arguments']['limits'].update(max_candidates=128));self.rejected()
    def test_structural_pin_not_invented(self):
        self.raw('review',lambda r:r['response']['result']['structuredContent'].update(snapshot_sha256='0'*64));self.rejected()
    def test_fixture_commit_binding(self):
        p=self.packet/'fixture-source/logic.go';p.write_bytes(p.read_bytes()+b'\n');self.change('execution.json',lambda d:d['fixture_sha256'].update({'logic.go':guard.sha(p.read_bytes())}));self.rejected()
    def test_command_failure_not_pass(self):
        self.change('commands.json',lambda d:d[0].update(exit_code=1));self.rejected()
    def test_unexpected_stderr_not_suppressed(self):
        p=self.packet/'sessions/main/stderr.txt';p.write_bytes(p.read_bytes()+b'unknown warning\n');self.rehash();self.rejected()
    def test_force_kill_not_clean(self):
        self.change('sessions/main/lifecycle.json',lambda d:d.update(kill_sent=True));self.rejected()
    def test_resource_cap(self):
        self.change('sessions/main/lifecycle.json',lambda d:d.update(tool_requests=65));self.rejected()
    def test_unreaped_diagnostic(self):
        self.change('diagnostics/old-index-uncheckpointed/lifecycle.json',lambda d:d.update(child_reaped=False));self.rejected()
    def test_actual_role_not_invented(self):self.accounting(lambda d:d.update(actual_role_sessions=1))
    def test_candidate_not_adopted(self):self.accounting(lambda d:d.update(candidate_adopted=True))
    def test_formal_not_accepted(self):self.accounting(lambda d:d.update(formal_completed=1,independent_acceptance='accepted'))
    def test_timeout_trials_not_inflated(self):self.accounting(lambda d:d.update(deadline_trials=100))
    def test_diagnostic_denominator_not_hidden(self):self.accounting(lambda d:d.update(diagnostic_tool_requests=0,all_new_tool_requests=56))
    def test_partial_case_not_full(self):self.accounting(lambda d:d['planned_case_progress'][15].update(full_case_status='pass'))
    def test_role_case_still_pending(self):self.accounting(lambda d:d['planned_case_progress'][4].update(full_case_status='pass'))
    def test_formal_snapshot_not_advanced(self):
        self.change('formal-state.json',lambda d:d.update(execution_allowed=True));self.rejected()
    def test_extra_artifact_not_accepted_after_rehash(self):
        (self.packet/'unrecorded.txt').write_text('extra');self.rehash();self.rejected()
    def test_original_consumer_not_changed(self):
        self.change('execution.json',lambda d:d['original_after'].update(porcelain=' M plugin/agents/analyzer.md'));self.rejected()
    def test_go_delta_not_hidden(self):
        self.change('execution.json',lambda d:d.update(go_source_changes=['cmd/cks/main.go']));self.rejected()
    def test_binary_pin_not_self_declared(self):
        self.change('execution.json',lambda d:d['binary_sha256'].update(cks='0'*64));self.rejected()
    def test_database_not_replaced_after_rehash(self):
        path=self.packet/'index/graph.db'
        with closing(sqlite3.connect(path)) as db:
            db.execute("UPDATE manifest SET value=? WHERE key='src_commit'",('0'*40,));db.commit();db.execute('PRAGMA wal_checkpoint(TRUNCATE)')
        self.change('execution.json',lambda d:d['index_sha256'].update(index=guard.sha(path.read_bytes())));self.rejected()
    def test_plan_progress_has_22_cases_and_15_pending(self):
        r=guard.prepare.derive(PACKET)['stdio-readiness.json'];self.assertEqual(len(r['planned_case_progress']),22);self.assertEqual(r['full_cases_pending'],15);self.assertEqual(r['all_new_tool_requests'],113)


# Later stages changed inputs this packet binds; run at its recorded revision.
load_tests = evidence_revision.tests(__file__)


if __name__=='__main__':unittest.main()
