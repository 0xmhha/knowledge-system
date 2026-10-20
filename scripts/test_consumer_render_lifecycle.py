#!/usr/bin/env python3
"""Rehashed evidence mutations cannot hide lifecycle failure or assert quality."""
import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import unittest
ROOT=Path(__file__).resolve().parents[1]
s=importlib.util.spec_from_file_location('p07_check',ROOT/'scripts/check-consumer-render-lifecycle.py');c=importlib.util.module_from_spec(s);s.loader.exec_module(c)
PACKET=ROOT/c.p.PACKET
class P07(unittest.TestCase):
    def setUp(self):self.tmp=tempfile.TemporaryDirectory(prefix='ks-p07-guard-');self.p=Path(self.tmp.name)/'packet';shutil.copytree(PACKET,self.p)
    def tearDown(self):self.tmp.cleanup()
    def edit(self,path,fn):
        p=self.p/path;d=json.loads(p.read_text());fn(d);p.write_text(json.dumps(d,ensure_ascii=False,indent=2)+'\n')
    def edit_lines(self,path,fn):
        p=self.p/path;d=[json.loads(l)for l in p.read_text().splitlines()];fn(d);p.write_text(''.join(json.dumps(x)+'\n'for x in d))
    def rehash(self):
        p=self.p/'manifest.json';m=json.loads(p.read_text());m['artifacts']={f.relative_to(self.p).as_posix():c.p.digest(f)for f in self.p.rglob('*')if f.is_file()and f!=p};p.write_text(json.dumps(m,indent=2)+'\n')
    def rejected(self):self.rehash();self.assertTrue(c.check(self.p,False))
    def test_valid(self):self.assertEqual(c.check(self.p,False),[])
    def test_missing_output(self):(self.p/'renders/P06-analyzer-handoff-sequence/output.png').unlink();self.rejected()
    def test_png_bytes(self):
        p=self.p/'renders/P06-analyzer-handoff-sequence/output.png';p.write_bytes(b'invalid png');self.rejected()
    def test_svg_shape(self):
        p=self.p/'renders/P06-analyzer-handoff-modules/output.svg';p.write_text('<svg/>');self.rejected()
    def test_typed_mermaid(self):
        p=self.p/'renders/P06-analyzer-handoff-sequence/input.mmd';p.write_text(p.read_text()+' A-->B\n');self.rejected()
    def test_fake_render_success(self):self.edit('renders/renders.json',lambda d:d[0].update(exit_code=1));self.rejected()
    def test_renderer_cleanup(self):self.edit('renders/renders.json',lambda d:d[0]['cleanup'].update(all_observed_identities_gone=False));self.rejected()
    def test_renderer_rescue(self):self.edit('renders/renders.json',lambda d:d[0].update(rescue=[{'pid':1}]));self.rejected()
    def test_renderer_forced(self):self.edit('renders/renders.json',lambda d:d[0].update(forced=True));self.rejected()
    def test_bad_version(self):self.edit('renders/toolchain.json',lambda d:d['versions'].update(mermaid='unmeasured'));self.rejected()
    def test_download_hidden(self):self.edit('renders/toolchain.json',lambda d:d.update(download_commands=['npm install']));self.rejected()
    def test_visual_acceptance(self):self.edit('renders/visual-check.json',lambda d:d.update(independent_acceptance='approved'));self.rejected()
    def test_false_P07(self):self.edit('render-lifecycle.json',lambda d:d.update(P07_complete=False));self.rejected()
    def test_forced_exit_as_normal(self):self.edit('hosts/sigkill/exit.json',lambda d:d.update(classification='normal_completion',host_exit=0));self.rejected()
    def test_sigint_zero_as_normal(self):self.edit('hosts/sigint/exit.json',lambda d:d.update(classification='normal_completion'));self.rejected()
    def test_cancel_without_ready(self):self.edit('hosts/sigterm/exit.json',lambda d:d.update(cancel_ready=False));self.rejected()
    def test_foreign_signal_pid(self):self.edit('hosts/sigterm/exit.json',lambda d:d['signal'].update(target_pid=1));self.rejected()
    def test_child_not_reaped(self):self.edit_lines('hosts/sigint/mcp-frames.jsonl',lambda d:[r.update(child_reaped=False)for r in d if r['kind']=='exit']);self.rejected()
    def test_PID_absence_without_exit(self):self.edit_lines('hosts/sigint/mcp-frames.jsonl',lambda d:d.__setitem__(slice(None),[r for r in d if r['kind']!='exit']));self.rejected()
    def test_CKS_wrong_exit(self):self.edit_lines('hosts/sigterm/mcp-frames.jsonl',lambda d:[r.update(exit_code=143)for r in d if r['kind']=='exit']);self.rejected()
    def test_signal_worker_not_joined(self):self.edit_lines('hosts/sigint/mcp-frames.jsonl',lambda d:[r.update(signal_worker_joined=False)for r in d if r['kind']=='exit']);self.rejected()
    def test_repeat_signal_erased(self):self.edit_lines('hosts/sigint/mcp-frames.jsonl',lambda d:d.__setitem__(slice(None),[r for r in d if r['kind']!='signal']));self.rejected()
    def test_gateway_handler_leak(self):self.edit('hosts/sigterm/exit.json',lambda d:d['gateway_cleanup'].update(all_handler_threads_joined=False));self.rejected()
    def test_gateway_socket_leak(self):self.edit('hosts/normal/exit.json',lambda d:d['gateway_cleanup'].update(server_socket_closed=False));self.rejected()
    def test_host_rescue_hidden(self):self.edit('hosts/sigterm/exit.json',lambda d:d.update(rescue=[{'signal':9}]));self.rejected()
    def test_live_PID_ignored(self):self.edit('postrun-process-audit.json',lambda d:d['probes'][0].update(exit_code=0,stdout='123'));self.rejected()
    def test_audit_failed_open(self):self.edit('postrun-process-audit.json',lambda d:d['auditor_self_probe'].update(exit_code=1,stdout=''));self.rejected()
    def test_audit_PID_omitted(self):self.edit('postrun-process-audit.json',lambda d:d['probes'].pop());self.rejected()
    def test_budget_bypassed(self):self.edit('native/proxy-budget/boundary.json',lambda d:d.update(forwarded_tool_requests=65));self.rejected()
    def test_budget_refusal_erased(self):self.edit_lines('native/proxy-budget/mcp-frames.jsonl',lambda d:d.__setitem__(slice(None),[r for r in d if r['kind']!='budget_refused']));self.rejected()
    def test_native65_as_role_claim(self):self.edit('native/proxy-budget/boundary.json',lambda d:d.update(scope='actual_role'));self.rejected()
    def test_initial_alias_failure_erased(self):self.edit('diagnostics/budget-diagnosis.json',lambda d:d.update(initial_controller_exit=0));self.rejected()
    def test_historical_unknown_hidden(self):self.edit('render-lifecycle.json',lambda d:d.update(historical_unconfirmed_CKS_exit_receipts=0));self.rejected()
    def test_historical_missing_exit_forged(self):self.edit_lines('diagnostics/hosts/sigint/mcp-frames.jsonl',lambda d:d.append({'kind':'exit','exit_code':0,'child_reaped':True}));self.rejected()
    def test_final_driver_pin(self):self.edit('hosts/normal/invocation.json',lambda d:d.update(driver_sha256='0'*64));self.rejected()
    def test_proxy_pin(self):self.edit_lines('hosts/normal/mcp-frames.jsonl',lambda d:[r.update(proxy_sha256='0'*64)for r in d if r['kind']=='owner']);self.rejected()
    def test_actual_role_changed(self):self.edit('hosts/normal/invocation.json',lambda d:d.update(role='planner'));self.rejected()
    def test_workspace_changed(self):self.edit('hosts/normal/exit.json',lambda d:d.update(workspace_unchanged=False));self.rejected()
    def test_inflight_claim(self):self.edit('render-lifecycle.json',lambda d:d.update(in_flight_CKS_query_cancellation='verified'));self.rejected()
    def test_ontology_claim(self):self.edit('render-lifecycle.json',lambda d:d.update(ontology_effect_acceptance='accepted'));self.rejected()
    def test_formal_execution(self):self.edit('formal-state.json',lambda d:d.update(execution_allowed=True));self.rejected()
    def test_source_adoption(self):self.edit('execution.json',lambda d:d.update(candidate_adopted=True));self.rejected()
    def test_prior_inventory_omitted(self):self.edit('preservation.json',lambda d:d['files'].pop(next(iter(d['files']))));self.rejected()
    def test_extra_profile_file(self):
        p=self.p/'hosts/normal/claude-config';p.mkdir();(p/'settings.json').write_text('{}');self.rejected()
if __name__=='__main__':unittest.main()
