#!/usr/bin/env python3
"""Adversarial caller evidence, contract and independent-acceptance boundary tests."""
import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]
PACKET = ROOT / 'docs/spec-driven/operational-preparation/source-review-supplement/rule-caller-context-v1'
spec = importlib.util.spec_from_file_location('caller_context_guard', ROOT / 'scripts/check-rule-caller-context.py')
guard = importlib.util.module_from_spec(spec)
spec.loader.exec_module(guard)


class CallerContextTests(unittest.TestCase):
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

    def test_expected_answer_in_scanner_input(self):
        self.change('source-inputs.json', lambda d: d[0].update(expected_count=14))
        self.rejected('caller source payload/private data boundary')

    def test_source_body_substitution(self):
        self.change('source-inputs.json', lambda d: d[0].update(Source='package p'))
        self.rejected('caller source payload/private data boundary')

    def test_prior_commit_proof_substitution(self):
        self.change('source-proof-reference.json', lambda d: d.update(repository_commit='a' * 40))
        self.rejected('caller source proof reference')

    def test_helper_source_replaced_and_rehashed(self):
        (self.packet / 'main.go.txt').write_text('package main\nfunc main(){}\n')
        self.rehash()
        self.rejected('caller source snapshot: main.go.txt')

    def test_live_source_binding_removed(self):
        self.change('manifest.json', lambda d: d['input_bindings'].pop('scripts/source_packet.py'))
        self.rejected('caller live input bindings')

    def test_target_resolution_promoted(self):
        self.change('logs/source-observations.json', lambda d: d[0]['references'][0].update(resolution='resolved_graph_call'))
        self.rejected('actual caller source replay mismatch')

    def test_literal_replaced_in_raw_ast(self):
        self.change('logs/source-observations.json', lambda d: d[0]['references'][1]['first_argument'].update(literal='safe-command'))
        self.rejected('actual caller source replay mismatch')

    def test_direct_call_promoted_from_function_value(self):
        self.change('caller-review.json', lambda d: d[0].update(kind='resolved_runtime_call'))
        self.rejected('caller source/claim accounting: caller-review.json')

    def test_caller_source_span_fabricated(self):
        self.change('caller-review.json', lambda d: d[0]['site'].update(start_byte=0))
        self.rejected('caller source/claim accounting: caller-review.json')

    def test_literal_command_declared_safe(self):
        self.change('caller-review.json', lambda d: d[1].update(vulnerability_verdict='safe', approved=True))
        self.rejected('caller source/claim accounting: caller-review.json')

    def test_runtime_reachability_invented(self):
        self.change('caller-review.json', lambda d: d[0].update(runtime_reachability='production_reachable'))
        self.rejected('caller source/claim accounting: caller-review.json')

    def test_attacker_control_invented(self):
        self.change('caller-review.json', lambda d: d[0].update(attacker_control='confirmed'))
        self.rejected('caller source/claim accounting: caller-review.json')

    def test_caller_multiplicity_counted_as_gold(self):
        self.change('coverage-summary.json', lambda d: d['counts'].update(independent_gold_records=14))
        self.rejected('caller source/claim accounting: coverage-summary.json')

    def test_project_denominator_filled_with_callers(self):
        self.change('coverage-summary.json', lambda d: d.update(original_project_call_denominator_unchanged=82))
        self.rejected('caller source/claim accounting: coverage-summary.json')

    def test_precision_fabricated(self):
        self.change('coverage-summary.json', lambda d: d.update(precision=1.0, recall=1.0))
        self.rejected('caller source/claim accounting: coverage-summary.json')

    def test_whole_callgraph_completeness_claimed(self):
        self.change('coverage-summary.json', lambda d: d.update(repository_callgraph_complete=True))
        self.rejected('caller source/claim accounting: coverage-summary.json')

    def test_ai_contract_approved(self):
        self.change('private-ai-context.json', lambda d: d['reviews'][0].update(approved=True, gold=True))
        self.rejected('caller source/claim accounting: private-ai-context.json')

    def test_ai_review_false_reviewer_receipt(self):
        self.change('private-ai-context.json', lambda d: d['reviews'][0].update(reviewer='invented'))
        self.rejected('caller source/claim accounting: private-ai-context.json')

    def test_anchor_bytes_replaced(self):
        self.change('private-ai-context.json', lambda d: d['reviews'][0]['anchors'][0].update(source_text='forged source'))
        self.rejected('caller source/claim accounting: private-ai-context.json')

    def test_build_tag_removed(self):
        self.change('context-anchors.json', lambda d: d.update(build_constraints=[]))
        self.rejected('caller source/claim accounting: context-anchors.json')

    def test_project_execution_claimed(self):
        self.change('validation.json', lambda d: d.update(target_source_parsed_not_executed=False))
        self.rejected('caller execution/determinism boundary')

    def test_real_backend_execution_claimed(self):
        self.change('validation.json', lambda d: d.update(context_scanner_kernel_calls=14))
        self.rejected('caller runtime promotion')

    def test_formal_execution_promoted(self):
        self.change('formal-state.json', lambda d: d.update(execution_allowed=True))
        self.rejected('caller formal state promotion')

    def test_native_skip_hidden(self):
        with (self.packet / 'logs/race.jsonl').open('a') as f:
            f.write(json.dumps({'Action': 'skip', 'Test': 'TestCallerScanner/literal'}) + '\n')
        self.rehash()
        self.rejected('caller raw native completion')

    def test_lint_finding_hidden(self):
        (self.packet / 'logs/lint.stdout.txt').write_text('one finding\n')
        self.rehash()
        self.rejected('caller quality findings')

    def test_source_hash_independently_checked(self):
        observations = json.loads((PACKET / 'logs/source-observations.json').read_text())
        observations[0]['sha256'] = '0' * 64
        with self.assertRaisesRegex(ValueError, 'caller observed source hash mismatch'):
            guard.prepare.derive(observations)

    def test_argument_source_independently_checked(self):
        observations = json.loads((PACKET / 'logs/source-observations.json').read_text())
        observations[0]['references'][0]['first_argument']['text'] = 'forged'
        with self.assertRaisesRegex(ValueError, 'caller argument source mismatch'):
            guard.prepare.derive(observations)


if __name__ == '__main__':
    unittest.main()
