"""Reject promoted completion or rewritten actual fault/recovery observations."""
import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import unittest
spec = importlib.util.spec_from_file_location('errorscheck', Path(__file__).with_name('check-stablenet-errors.py'))
check = importlib.util.module_from_spec(spec)
spec.loader.exec_module(check)

class ErrorEvidenceTests(unittest.TestCase):

    def setUp(self):
        folder = tempfile.TemporaryDirectory()
        self.addCleanup(folder.cleanup)
        self.packet = Path(folder.name) / 'packet'
        shutil.copytree(check.ROOT / check.REL, self.packet)

    def seal(self):
        p = self.packet / 'manifest.json'
        d = json.loads(p.read_text())
        d['artifacts'] = {str(x.relative_to(self.packet)): check.sha(x.read_bytes()) for x in self.packet.rglob('*') if x.is_file() and x != p}
        p.write_text(json.dumps(d))

    def change(self, path, fn):
        p = self.packet / path
        d = json.loads(p.read_text())
        fn(d)
        p.write_text(json.dumps(d, ensure_ascii=False))
        self.seal()

    def response(self, case, label, fn):
        root = self.packet / case
        p = root / 'messages.json'
        rows = json.loads(p.read_text())
        row = next((r for r in rows if r['label'] == label))
        fn(row['response'])
        result = row['response'].get('result', {})
        for content in result.get('content', []):
            if content.get('type') == 'text':
                try:
                    json.loads(content['text'])
                    content['text'] = json.dumps(result['structuredContent'])
                except (ValueError, KeyError):
                    pass
        p.write_text(json.dumps(rows))
        p = root / 'stdout.jsonl'
        wire = [json.loads(s) for s in p.read_text().splitlines()]
        p.write_text(''.join((json.dumps(row['response'] if r.get('id') == row['response']['id'] else r) + '\n' for r in wire)))
        p = root / 'attempts.json'
        attempts = json.loads(p.read_text())
        next((a for a in attempts if a['label'] == label))['response'] = row['response']
        p.write_text(json.dumps(attempts))
        self.seal()

    def rejected(self):
        self.assertTrue(check.check(self.packet))

    def test_real_initial_and_final_evidence_is_valid(self):
        self.assertEqual([], check.check(self.packet))

    def test_cannot_promote_formal_completion(self):
        self.change('execution.json', lambda d: d.update(formal_completed=9))
        self.rejected()

    def test_worklist_cannot_hide_unfinished_comparison(self):
        self.change('WORKLIST.json', lambda d: d.update(next_task=None, pilot_steps_completed=['SN-P01', 'SN-P02', 'SN-P03', 'SN-P04']))
        self.rejected()

    def test_source_failure_cannot_poison_model_health(self):
        self.response('fixed-source', 'health-after-source', lambda r: r['result']['structuredContent']['backends']['vector'].update(model_reachable=False))
        self.rejected()

    def test_embedding_failure_cannot_become_source_error(self):
        self.response('fixed-recovery', 'embedding-failed', lambda r: r['result']['structuredContent'].update(code='source_missing'))
        self.rejected()

    def test_corruption_error_cannot_become_missing_source(self):
        self.response('fixed-recovery', 'retained-corrupt', lambda r: r['result']['structuredContent'].update(code='source_missing'))
        self.rejected()

    def test_health_only_recovery_must_restore_serviceability(self):
        self.response('fixed-recovery', 'health-recovered-without-raw-query', lambda r: r['result']['structuredContent'].update(serviceable=False))
        self.rejected()

    def test_historical_bug_witness_cannot_be_rewritten_as_healthy(self):
        self.response('baseline-source', 'health-poisoned', lambda r: r['result']['structuredContent'].update(serviceable=True))
        self.rejected()

    def test_initial_failure_cannot_be_dropped(self):
        self.change('diagnostics/initial-guard-run/fixed-recovery/messages.json', lambda rows: rows.__setitem__(slice(None), [r for r in rows if r['label'] != 'retained-missing']))
        self.rejected()

    def test_owned_native_interruption_cannot_be_hidden(self):
        self.change('native-initial-interruption.json', lambda d: d.update(group_empty_after=False))
        self.rejected()

    def test_proxy_fault_must_have_been_exercised(self):
        self.change('proxy-requests.json', lambda rows: rows.__setitem__(slice(None), [r for r in rows if r['step'] != 'fixed-recovery:embedding-failed']))
        self.rejected()

    def test_owned_listener_must_be_closed(self):
        self.change('proxy-cleanup.json', lambda d: d.update(closed=False))
        self.rejected()

    def test_owned_process_identity_must_be_gone(self):
        self.change('process-audit.json', lambda d: d['probes'][0].update(identity_gone=False))
        self.rejected()
if __name__ == '__main__':
    unittest.main()
