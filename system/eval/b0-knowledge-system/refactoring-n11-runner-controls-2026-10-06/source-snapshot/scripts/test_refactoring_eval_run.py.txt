"""Synthetic control records only; no actual reviewer or fresh evaluation units."""
import contextlib
import copy
import importlib.util
import io
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import time
import types
import unittest
from unittest.mock import patch

import test_refactoring_eval_input_check as F

SCRIPT=Path(__file__).with_name('refactoring-eval-run.py')
SPEC=importlib.util.spec_from_file_location('guarded_runner',SCRIPT)
RUN=importlib.util.module_from_spec(SPEC);SPEC.loader.exec_module(RUN)

class RunnerTests(unittest.TestCase):
    def setUp(self):
        self.temp=tempfile.TemporaryDirectory(prefix='ks-eval-run-control-');self.addCleanup(self.temp.cleanup)
        self.root=Path(self.temp.name);repo=self.root/'repo';repo.mkdir()
        p,b=F.fixture();p.update(execution_allowed=True, request_scope={'knowledge_as_of':'2026-10-06','knowledge_subsystem':'test-fixture'})
        binary=self.root/'binary';binary.write_bytes(b'control executable identity only')
        cfg=b'control config bytes'
        p['execution_binding']={'dataset_id':'synthetic-control-only','config_sha256':F.GATE.sha(cfg),'binary_sha256':F.GATE.sha(binary.read_bytes())}
        self.p,self.b=p,b
        data={'protocol':F.encode(p),'questions':F.encode(b),'review':F.encode(F.review(p,b)),'baseline':F.BASE,'known_observations':b'[]','config':cfg,'registry':b'{"schema_version":1,"records":[]}'}
        paths={}
        for name,raw in data.items():
            path=self.root/name;path.write_bytes(raw);paths[name]=path
        self.args=types.SimpleNamespace(**paths,repo=repo,binary=binary,output=self.root/'run',split='FINAL')
        original=RUN.GATE.audit
        self.audit_patch=patch.object(RUN.GATE,'audit',side_effect=lambda *a:original(*a[:-1],lambda commit,path:F.SOURCE))
        self.audit_patch.start();self.addCleanup(self.audit_patch.stop)

    def launch(self,command,**kwargs):
        # Dispatch must happen after a durable reservation; gold never appears
        # in the generated request-only file. Frozen original bytes are retained.
        rows=RUN.load_registry(self.args.registry)['records'];self.assertEqual(len(rows),1)
        self.assertEqual(rows[0]['state'],'FINAL_reserved');self.assertNotIn('first_observed_at',rows[0])
        request=json.loads((self.args.output/'requests.json').read_text())
        self.assertEqual(len(request['requests']),50)
        self.assertNotIn('candidate_answer',(self.args.output/'requests.json').read_text())
        self.assertEqual((self.args.output/'protocol.json').read_bytes(),self.args.protocol.read_bytes())
        self.assertIn('--evaluation-binding',command);self.assertIn('--environment-ledger',command)
        self.assertEqual(command[command.index('--retrieval-runs')+1],'5')
        return types.SimpleNamespace(returncode=5)

    def test_reservation_precedes_dispatch_and_failed_dispatch_blocks_reuse(self):
        self.assertEqual(RUN.run(self.args,self.launch),5)
        self.args.output=self.root/'retry'
        with self.assertRaisesRegex(ValueError,'already reserved'):
            RUN.run(self.args,lambda *a,**k:self.fail('reused FINAL was dispatched'))
        self.assertFalse(self.args.output.exists())

    def test_unreviewed_input_has_no_child_output_or_registry_mutation(self):
        self.args.review=None;before=self.args.registry.read_bytes()
        with contextlib.redirect_stdout(io.StringIO()):
            self.assertEqual(RUN.run(self.args,lambda *a,**k:self.fail('draft was dispatched')),2)
        self.assertEqual(before,self.args.registry.read_bytes());self.assertFalse(self.args.output.exists())

    def test_runtime_and_scope_must_be_frozen_before_dispatch(self):
        for mode in ('binary','config','scope','execution'):
            with self.subTest(mode=mode):
                p=copy.deepcopy(self.p)
                if mode=='binary':p['execution_binding']['binary_sha256']='b'*64
                if mode=='config':p['execution_binding']['config_sha256']='b'*64
                if mode=='scope':p.pop('request_scope')
                if mode=='execution':p['execution_allowed']=False
                self.args.protocol.write_bytes(F.encode(p));self.args.review.write_bytes(F.encode(F.review(p,self.b)))
                with self.assertRaises(ValueError):RUN.run(self.args,lambda *a,**k:self.fail('bad binding dispatched'))
                self.assertFalse(self.args.output.exists());self.assertEqual(RUN.load_registry(self.args.registry)['records'],[])

    def test_registry_corruption_and_symlink_are_not_reset(self):
        self.args.registry.write_bytes(b'{broken')
        with self.assertRaises(ValueError):RUN.run(self.args,lambda *a,**k:self.fail('corrupt history dispatched'))
        other=self.root/'other';other.write_bytes(b'{"schema_version":1,"records":[]}');self.args.registry.unlink();self.args.registry.symlink_to(other)
        with self.assertRaises(OSError):RUN.run(self.args,lambda *a,**k:self.fail('symlink registry dispatched'))
        self.assertFalse(self.args.output.exists())

    def test_real_second_process_cannot_take_live_registry_lock(self):
        code="""import importlib.util,sys
spec=importlib.util.spec_from_file_location('gate',sys.argv[1]);m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
try:
 with m.registry_lock(sys.argv[2]):sys.exit(9)
except ValueError:sys.exit(0)
"""
        with RUN.registry_lock(self.args.registry):
            child=subprocess.run([sys.executable,'-c',code,str(SCRIPT),str(self.args.registry)],capture_output=True)
        self.assertEqual(child.returncode,0,child.stderr.decode())
        with RUN.registry_lock(self.args.registry):pass

    def test_sigkill_after_reservation_keeps_consumed_set(self):
        # Actual separate process, durable write, SIGKILL before any query.
        code="""import importlib.util,sys,pathlib,os,signal,json
spec=importlib.util.spec_from_file_location('gate',sys.argv[1]);m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
p=pathlib.Path(sys.argv[2]);record=json.loads(sys.argv[3])
with m.registry_lock(p):
 m.reserve(p,m.load_registry(p),record)
 os.kill(os.getpid(),signal.SIGKILL)
"""
        record={'state':'FINAL_reserved','reserved_at':'2026-01-03T00:00:00Z','run_id':'synthetic-killed-control','final_clusters':[self.b['questions'][1]['cluster_id']], 'questions_sha256':F.GATE.sha(F.encode(self.b)), 'protocol_sha256':F.GATE.sha(F.encode(self.p))}
        child=subprocess.run([sys.executable,'-c',code,str(SCRIPT),str(self.args.registry),json.dumps(record)],capture_output=True)
        self.assertEqual(child.returncode,-9,child.stderr.decode())
        self.assertEqual(RUN.load_registry(self.args.registry)['records'],[record])
        with self.assertRaisesRegex(ValueError,'already reserved'):RUN.run(self.args,lambda *a,**k:self.fail('consumed after crash'))

    def test_child_retains_serialization_when_wrapper_is_sigkilled(self):
        code="""import importlib.util,sys,os,signal,subprocess
spec=importlib.util.spec_from_file_location('gate',sys.argv[1]);m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
with m.registry_lock(sys.argv[2]) as fd:
 child=subprocess.Popen([sys.executable,'-c','import time;time.sleep(1)'],pass_fds=(fd,),stdout=subprocess.DEVNULL)
 print(child.pid,flush=True)
 os.kill(os.getpid(),signal.SIGKILL)
"""
        parent=subprocess.Popen([sys.executable,'-c',code,str(SCRIPT),str(self.args.registry)],stdout=subprocess.PIPE,text=True)
        child_pid=int(parent.stdout.readline());parent.stdout.close();self.assertEqual(parent.wait(),-9)
        with self.assertRaisesRegex(ValueError,'owns the registry lock'):
            with RUN.registry_lock(self.args.registry):self.fail('live orphan evaluation overlapped')
        time.sleep(1.1)
        with RUN.registry_lock(self.args.registry):pass

    def test_dev_does_not_reserve_final_clusters(self):
        self.args.split='DEV'
        def launch(*a,**k):
            self.assertEqual(RUN.load_registry(self.args.registry)['records'],[])
            return types.SimpleNamespace(returncode=0)
        self.assertEqual(RUN.run(self.args,launch),0)

if __name__=='__main__':unittest.main()
