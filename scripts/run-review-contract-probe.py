#!/usr/bin/env python3
"""Exercise adopted local review validation; no models, MCP or project commands."""
import argparse
import copy
import importlib.util
import io
import json
from pathlib import Path
import sys
import tempfile
import unittest

from source_packet import sha

ROOT=Path(__file__).resolve().parents[1]


def load(path,name):
    spec=importlib.util.spec_from_file_location(name,path)
    m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m);return m


def run(repo,output):
    if output.exists():raise ValueError('fresh output required')
    prep=load(ROOT/'scripts/prepare-review-gap.py','review_gap_prepare')
    inventory={r['path']:r['sha256'] for r in json.loads((ROOT/prep.INPUTS['consumer_inventory']).read_text())}
    source={p:sha((repo/p).read_bytes()) for p in prep.CONSUMER}
    if any(digest!=inventory[p] for p,digest in source.items()):raise ValueError('adopted source changed')
    output.mkdir(parents=True)
    tests=load(repo/'plugin/scripts/tests/test_deep_review.py','adopted_deep_review_tests')
    stream=io.StringIO();result=unittest.TextTestRunner(stream=stream,verbosity=1).run(unittest.defaultTestLoader.loadTestsFromModule(tests))
    (output/'original-tests.txt').write_text(stream.getvalue())
    if result.testsRun!=15 or not result.wasSuccessful() or result.skipped:raise ValueError('adopted tests failed')
    review=tests.review;b=tests.bundle();j=tests.judgments();checks={}

    def valid(label,bundle,judgments):
        report=review.build(bundle,judgments);review.validate(report,bundle)
        assert report['independent_acceptance']=='not_recorded'
        checks[label]=True
        return report

    def rejected(label,operation):
        try:operation()
        except review.Invalid:checks[label]=True;return
        raise ValueError('expected refusal missing: '+label)

    valid('baseline_valid',b,j)
    one=copy.deepcopy(j);one['criteria']=one['criteria'][:1]
    valid('one_criterion_valid',b,one)
    empty=copy.deepcopy(j);empty['criteria']=[]
    rejected('empty_criteria_rejected',lambda:review.build(b,empty))
    nofindings=copy.deepcopy(j);nofindings['findings']=[]
    valid('empty_findings_valid',b,nofindings)
    changed=copy.deepcopy(j)
    for finding in changed['findings']:finding['priority']='low' if finding['priority']!='low' else 'high'
    valid('changed_priority_valid',b,changed)
    claim=copy.deepcopy(j);claim['sections']['priority_tradeoffs']['claim']='Deliberately unadjudicated priority statement for local format control'
    valid('semantic_claim_not_adjudicated',b,claim)
    pins=copy.deepcopy(b)
    for k in list(pins['rules']['arguments']['function']):
        if k.startswith('expected_'):del pins['rules']['arguments']['function'][k]
    valid('request_expected_pins_optional',pins,j)
    structural=copy.deepcopy(b);structural['function_review']['result']['code']['signature']='unverified structural content control'
    report=valid('structural_content_not_snapshot_verified',structural,j)
    assert report['structural_review']['snapshot_association']=='identity_only_unverified_snapshot'
    unknown=copy.deepcopy(j)
    for section in unknown['sections'].values():section.update(status='unknown',unknowns=['not reviewed'])
    rejected('all_unknown_rejected',lambda:review.build(b,unknown))
    with tempfile.TemporaryDirectory() as temp:
        workspace,state=tests.DeepReviewTests().make_workspace(temp)
        (workspace/'review-report.md').write_text('unbound report')
        rejected('report_tamper_rejected',lambda:review.verify_workspace(workspace,state))
        review.write_review(workspace,b,j)
        (workspace/'sequence.mmd').write_text('flowchart TD\nforged --> safe\n')
        rejected('diagram_tamper_rejected',lambda:review.verify_workspace(workspace,state))
        review.write_review(workspace,b,j)
        state['current_state']='COMPLETED';state['states']['PLANNING']={'status':'completed'}
        rejected('legacy_planner_missing_report_hash_rejected',lambda:review.verify_workspace(workspace,state))
    receipt={'kind':'owned_local_contract_probe_not_model_or_installed_host',
      'original_tests':{'run':15,'failures':0,'errors':0,'skipped':0},'checks':checks,
      'adopted_source_sha256':source,'runner_sha256':sha(Path(__file__).read_bytes()),
      'new_mcp_calls':0,'new_model_calls':0,'formal_acceptance':'not_recorded'}
    (output/'result.json').write_text(json.dumps(receipt,ensure_ascii=False,indent=2)+'\n')
    if any(sha((repo/p).read_bytes())!=digest for p,digest in source.items()):raise ValueError('source changed during probe')
    print('Adopted tests15 and local contract probes12 passed; semantics/host acceptance not granted')


if __name__=='__main__':
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--consumer-repo',type=Path,required=True);parser.add_argument('--output',type=Path,required=True)
    args=parser.parse_args();run(args.consumer_repo.resolve(),args.output.resolve())
