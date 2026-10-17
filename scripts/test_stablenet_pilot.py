"""Keep diagnostic failures and provenance limits out of completed acceptance."""
import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import unittest

spec = importlib.util.spec_from_file_location('pilot', Path(__file__).with_name('check-stablenet-pilot.py'))
pilot = importlib.util.module_from_spec(spec)
spec.loader.exec_module(pilot)


class PilotEvidenceTests(unittest.TestCase):
    def setUp(self):
        temp = tempfile.TemporaryDirectory()
        self.addCleanup(temp.cleanup)
        self.packet = Path(temp.name) / 'packet'
        shutil.copytree(pilot.ROOT / pilot.REL, self.packet)

    def seal(self):
        p = self.packet / 'manifest.json'
        d = json.loads(p.read_text())
        d['artifacts'] = {str(f.relative_to(self.packet)): pilot.sha(f.read_bytes()) for f in self.packet.rglob('*') if f.is_file() and f != p}
        p.write_text(json.dumps(d))

    def change(self, name, mutation):
        p = self.packet / name
        d = json.loads(p.read_text())
        mutation(d)
        p.write_text(json.dumps(d, ensure_ascii=False))
        self.seal()

    def response(self, case, label, mutation):
        p = self.packet / 'sessions' / case / 'messages.json'
        rows = json.loads(p.read_text())
        row = next(r for r in rows if r['label'] == label)
        mutation(row['response'])
        p.write_text(json.dumps(rows))
        raw = p.with_name('stdout.jsonl')
        wire = [json.loads(line) for line in raw.read_text().splitlines()]
        wire = [row['response'] if r.get('id') == row['response']['id'] else r for r in wire]
        raw.write_text(''.join(json.dumps(r) + '\n' for r in wire))
        self.seal()

    def rejected(self):
        self.assertTrue(pilot.check(self.packet))

    def test_observed_failures_are_a_valid_not_ready_result(self):
        self.assertEqual([], pilot.check(self.packet))

    def test_cannot_promote_failed_default_retrieval(self):
        self.change('result.json', lambda d: d.update(default_retrieval_ready=True))
        self.rejected()

    def test_cannot_promote_formal_or_ontology_acceptance(self):
        self.change('result.json', lambda d: d.update(formal_completed=9, ontology_effect_evaluated=True))
        self.rejected()

    def test_same_model_name_cannot_hide_digest_change(self):
        self.change('model-identity.json', lambda d: d['selected_model'].update(digest='0' * 64))
        self.rejected()

    def test_cannot_drop_failed_input_from_denominator(self):
        self.change('sessions/retrieval/messages.json', lambda d: d.__setitem__(slice(None), [r for r in d if r['label'] != 'Q1-vector-all']))
        self.rejected()

    def test_v2_foreign_project_cannot_be_relabelled_as_local(self):
        self.response('retrieval', 'Q3-pack-v2', lambda r: r['result']['structuredContent']['citations'][0].update(project_id='foreign'))
        self.rejected()

    def test_typed_pin_refusal_requires_null_analysis(self):
        self.response('retrieval', 'wrong-commit', lambda r: r['result']['structuredContent'].update(analysis={}))
        self.rejected()

    def test_live_child_cannot_count_as_clean_exit(self):
        self.change('process-audit.json', lambda d: d['probes'][0].update(exit_code=0, stdout='live'))
        self.rejected()

    def test_changed_original_bytes_cannot_pass_preservation(self):
        self.change('original-after.json', lambda d: d.update(dataset_current_target='different-version'))
        self.rejected()

    def test_cannot_count_virtual_convention_as_verified_code(self):
        self.change('result.json', lambda d: d.update(non_citable_convention_hits=0, raw_hits_with_retained_file_witness=110))
        self.rejected()


if __name__ == '__main__':
    unittest.main()
