#!/usr/bin/env python3
"""Verify an old packet against its recorded docs without reverting current docs."""
import argparse
import functools
import importlib.util
import io
import json
import os
from pathlib import Path
import subprocess
import unittest

from source_packet import SourceCapture, sha

ROOT=Path(__file__).resolve().parents[1]
PACKET=Path('system/eval/knowledge-roadmap-2026-10-16/policy-source-regeneration')
DOCS=['docs/REMAINING-WORK.md','docs/spec-driven/CURRENT-HANDOFF.md']
REVISION='17970c869be85f4242bd61eea0db7c5f01d2a4d8'


def load(name):
    spec=importlib.util.spec_from_file_location('history_'+name.replace('-','_'),ROOT/'scripts'/name)
    m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m);return m


def verify(output):
    if output.exists():raise ValueError('fresh output required')
    output.mkdir(parents=True);view=output/'recorded-doc-view';view.mkdir()
    guard=load('check-source-policy-regeneration.py')
    current_errors=guard.check()
    if current_errors!=['invalid source regeneration evidence: source/input bytes differ']:
        raise ValueError('unexpected current-context historical guard result')
    binding=json.loads((ROOT/PACKET/'source-binding.json').read_text())
    manifest=json.loads((ROOT/PACKET/'manifest.json').read_text())
    expected={**binding['source_sha256'],**binding['approved_input_sha256'],**manifest['source_sha256']}
    mismatch=[p for p,digest in expected.items() if sha((ROOT/p).read_bytes())!=digest]
    if set(mismatch)!=set(DOCS):raise ValueError('historical mismatch scope changed')
    # A separate source view has ordinary files (no external symlink bypass).
    # Hard links only save copying; the two docs are unlinked before replacement.
    names=subprocess.check_output(['git','ls-files','-z'],cwd=ROOT).split(b'\0')
    for raw in names:
        if not raw:continue
        p=Path(raw.decode());source=ROOT/p;target=view/p
        if not source.is_file():continue
        target.parent.mkdir(parents=True,exist_ok=True)
        if source.is_symlink():target.write_bytes(source.read_bytes())
        else:os.link(source,target)
    capture=SourceCapture(ROOT,output/'history-docs',REVISION)
    rows=[];head=subprocess.check_output(['git','rev-parse','HEAD'],cwd=ROOT,text=True).strip()
    for name in DOCS:
        recovered=capture.source(name)
        if sha(recovered)!=expected[name]:raise ValueError('recorded document Git binding')
        current=(ROOT/name).read_bytes();at_start=subprocess.check_output(['git','show',head+':'+name],cwd=ROOT)
        if current!=at_start or sha(at_start)==expected[name]:raise ValueError('mismatch was introduced in this task')
        target=view/name;target.unlink();target.write_bytes(recovered)
        rows.append({'path':name,'expected_sha256':expected[name],'current_sha256':sha(current),
                     'preexisting_at_revision':head,'introduced_by_this_task':False})
    (output/'history-docs/git-proof.json').write_text(json.dumps(capture.proof(),indent=2)+'\n')
    tests=load('test_source_policy_regeneration.py')
    original=tests.guard.check
    tests.guard.check=functools.partial(original,root=view)
    stream=io.StringIO()
    result=unittest.TextTestRunner(stream=stream,verbosity=1).run(unittest.defaultTestLoader.loadTestsFromModule(tests))
    (output/'recorded-doc-tests.txt').write_text(stream.getvalue())
    if result.testsRun!=14 or not result.wasSuccessful():raise ValueError('recorded-document-context historical tests failed')
    receipt={'kind':'historical_packet_checked_with_recorded_document_context_not_current_document_guard_pass',
      'current_context_errors':current_errors,'current_context_guard_passed':False,'mismatches':rows,
      'recorded_document_revision':REVISION,'recorded_context_tests':14,'recorded_context_failures':0,
      'recorded_context_errors':0,'guard_or_test_source_modified':False,'only_guard_root_argument_changed':True,
      'workspace_docs_or_prior_packet_modified':False,'formal_acceptance_granted':False}
    (output/'history-validation.json').write_text(json.dumps(receipt,ensure_ascii=False,indent=2)+'\n')
    for name in DOCS:
        if sha((ROOT/name).read_bytes())!=next(r['current_sha256']for r in rows if r['path']==name):
            raise ValueError('current documents changed during historical validation')
    print('Historical tests: 14 passed in recorded-doc context; preexisting current-doc mismatch retained')


if __name__=='__main__':
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--output',type=Path,required=True)
    verify(p.parse_args().output.resolve())
