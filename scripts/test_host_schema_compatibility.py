#!/usr/bin/env python3
"""Rehashed mutations must not turn schema-only evidence into runtime/role proof."""
import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import unittest
import evidence_revision
ROOT=Path(__file__).resolve().parents[1]
s=importlib.util.spec_from_file_location('compat_check',ROOT/'scripts/check-host-schema-compatibility.py');c=importlib.util.module_from_spec(s);s.loader.exec_module(c)
PACKET=ROOT/c.prepare.BASE/'host-schema-compatibility-v1'


class Compatibility(unittest.TestCase):
    def setUp(self):
        self.tmp=tempfile.TemporaryDirectory(prefix='ks-compat-guard-');self.p=Path(self.tmp.name)/'packet';shutil.copytree(PACKET,self.p)
    def tearDown(self):self.tmp.cleanup()
    def edit(self,path,fn):
        p=self.p/path;d=json.loads(p.read_text());fn(d);p.write_text(json.dumps(d,ensure_ascii=False,indent=2)+'\n')
    def rehash(self):
        p=self.p/'manifest.json';m=json.loads(p.read_text());m['artifacts']={f.relative_to(self.p).as_posix():c.sha(f)for f in sorted(self.p.rglob('*'))if f.is_file()and f!=p};p.write_text(json.dumps(m,indent=2)+'\n')
    def rejected(self):self.rehash();self.assertTrue(c.check(self.p,False))
    def test_valid(self):self.assertEqual(c.check(self.p,False),[])
    def test_missing_pair(self):self.edit('role-readiness.json',lambda d:d.update(role_tool_pairs_succeeded=5));self.rejected()
    def test_false_adoption(self):self.edit('execution.json',lambda d:d.update(candidate_adopted=True));self.rejected()
    def test_real_model(self):self.edit('execution.json',lambda d:d.update(real_model_calls=1));self.rejected()
    def test_formal(self):self.edit('execution.json',lambda d:d.update(formal_runs=1));self.rejected()
    def test_dirty_native(self):self.edit('execution.json',lambda d:d['cks_postrun'].update(porcelain='M runtime'));self.rejected()
    def test_old_candidate(self):self.edit('execution.json',lambda d:d['prior_candidate_postrun'].update(commit='0'*40));self.rejected()
    def test_pin_source(self):self.edit('binary-pins.json',lambda d:d.update(cks_commit='0'*40));self.rejected()
    def test_pin_binary(self):self.edit('binary-pins.json',lambda d:d.update(cks_sha256='0'*64));self.rejected()
    def test_native_unreaped(self):self.edit('native-matrix/new/lifecycle.json',lambda d:d.update(child_reaped=False));self.rejected()
    def test_native_forcekill(self):self.edit('native-matrix/new/lifecycle.json',lambda d:d.update(kill_sent=True));self.rejected()
    def test_duplicate_raw_erased(self):
        self.edit('native-matrix/new/messages.json',lambda d:next(r for r in d if r['label']=='value_dependencies-invalid-7').update(request_raw='{}\n'));self.rejected()
    def test_positive_became_refusal(self):
        self.edit('native-matrix/new/messages.json',lambda d:next(r for r in d if r['label']=='value_dependencies-node')['response'].update(error={'code':-1}));self.rejected()
    def test_elapsed_is_only_ignored_field(self):
        self.edit('native-matrix/new/messages.json',lambda d:next(r for r in d if r['label']=='value_dependencies-node')['response']['result']['structuredContent'].update(status='forged'));self.rejected()
    def test_extra_native_request(self):self.edit('native-matrix/new/messages.json',lambda d:d.append({'label':'hidden-attempt'}));self.rejected()
    def test_test_failed(self):self.edit('validation/test.receipt.json',lambda d:d.update(exit_code=1));self.rejected()
    def test_race_failed(self):self.edit('validation/race.receipt.json',lambda d:d.update(exit_code=1));self.rejected()
    def test_native_counts_forged(self):self.edit('validation/test.receipt.json',lambda d:d['test_results'].update(pass_=999));self.rejected()
    def test_host_unreaped(self):self.edit('runs/P06-analyzer-03/exit.json',lambda d:d.update(host_reaped=False));self.rejected()
    def test_responder_open(self):self.edit('runs/P06-analyzer-03/exit.json',lambda d:d.update(responder_closed=False));self.rejected()
    def test_cks_wrong_binary(self):self.edit('runs/P06-analyzer-03/invocation.json',lambda d:d.update(cks_sha256='0'*64));self.rejected()
    def test_host_wrong_role(self):self.edit('runs/P06-analyzer-03/invocation.json',lambda d:d.update(role='planner'));self.rejected()
    def test_external_provider(self):self.edit('runs/P06-analyzer-03/invocation.json',lambda d:d['env'].update(ANTHROPIC_BASE_URL='https://provider.invalid'));self.rejected()
    def test_host_tool_removed(self):
        self.edit('runs/P06-analyzer-03/gateway-requests.json',lambda d:next(r for r in d if 'scripted_response'in r)['body']['tools'].pop());self.rejected()
    def test_guard_forged(self):
        self.edit('runs/P06-planner-delivery/gateway-requests.json',lambda d:[x.update(is_error=False)for r in d if 'scripted_response'in r for m in r['body'].get('messages',[])for x in m.get('content',[])if isinstance(x,dict)and x.get('tool_use_id')=='fixture_tool_4']);self.rejected()
    def test_generation_hash(self):self.edit('runs/P06-planner-reentry/workspace-after/state.json',lambda d:d['states']['ANALYSIS'].update(deep_review_sha256='0'*64));self.rejected()
    def test_tamper_cli_allowed(self):self.edit('runs/P06-planner-boundary/workspace-after/shell-tamper-receipt.json',lambda d:d.update(delivery_allowed=True));self.rejected()
    def test_formal_freeze(self):self.edit('formal-state.json',lambda d:d.update(execution_allowed=True));self.rejected()
    def test_timeout_attempt_omitted(self):self.edit('native-diagnostics.json',lambda d:d.pop('race-14min-initial'));self.rejected()
    def test_native_runtime_source_tampered(self):
        proof=json.loads((self.p/'cks-source/git-proof.json').read_text());p=self.p/'cks-source'/proof['files']['pkg/graph/valueflow/query.go']['artifact'];p.write_bytes(p.read_bytes()+b'\n// altered\n');self.rejected()
    def test_consumer_mirror_source_tampered(self):
        proof=json.loads((self.p/'consumer-source/git-proof.json').read_text());p=self.p/'consumer-source'/proof['files']['plugin/contracts/cks-tools.schema.json']['artifact'];p.write_bytes(p.read_bytes()+b'\n ');self.rejected()


# Later stages changed inputs this packet binds; run at its recorded revision.
load_tests = evidence_revision.tests(__file__)


if __name__=='__main__':unittest.main()
