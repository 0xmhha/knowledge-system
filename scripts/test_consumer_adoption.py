"""Protect real integration evidence from accidental completion promotion."""
import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import unittest
import evidence_revision

spec = importlib.util.spec_from_file_location('adoption', Path(__file__).with_name('check-consumer-adoption.py'))
adoption = importlib.util.module_from_spec(spec)
spec.loader.exec_module(adoption)


class AdoptionTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.packet = Path(self.temp.name) / 'packet'
        shutil.copytree(adoption.ROOT / adoption.REL, self.packet)

    def change(self, name, transform):
        path = self.packet / name
        value = json.loads(path.read_text())
        transform(value)
        path.write_text(json.dumps(value, ensure_ascii=False) + '\n')
        manifest_path = self.packet / 'manifest.json'
        manifest = json.loads(manifest_path.read_text())
        manifest['artifacts'][name] = adoption.sha(path)
        manifest_path.write_text(json.dumps(manifest))

    def rejected(self, phrase):
        self.assertTrue(any(phrase in e for e in adoption.check(self.packet)))

    def test_real_packet_matches_current_source(self):
        self.assertEqual([], adoption.check(self.packet))

    def test_formal_completion_cannot_follow_technical_success(self):
        self.change('integration-result.json', lambda d: d.update(formal_completed=9))
        self.rejected('scope/independence')

    def test_reviewer_name_cannot_be_invented(self):
        self.change('integration-result.json', lambda d: d.update(reviewer_assignment='AI'))
        self.rejected('scope/independence')

    def test_native_counter_must_match_raw_full_run(self):
        self.change('validation/native-isolated/receipts.json', lambda d: d[1]['tests'].update(**{'pass': 9999}))
        self.rejected('raw native test totals')

    def test_failed_native_race_cannot_be_hidden(self):
        self.change('validation/native-isolated/receipts.json', lambda d: d[2].update(exit_code=1))
        self.rejected('fresh native validation')

    def test_consumer_followup_cannot_change_executed_plugin(self):
        self.change('integration-result.json', lambda d: d.update(consumer_final_plugin_tree='0' * 40))
        self.rejected('plugin equality')

    def test_approved_ontology_scope_is_not_new_quality_measurement(self):
        self.change('adoption.json', lambda d: d.update(new_quality_or_ontology_measurement=True))
        self.rejected('derived adoption accounting')

    def test_actual_write_must_refuse_tampered_report(self):
        def alter(rows):
            for r in rows:
                if 'scripted_response' not in r:
                    continue
                for message in r['body']['messages']:
                    for value in message.get('content', []):
                        if isinstance(value, dict) and value.get('is_error') is True:
                            value['is_error'] = False
        self.change('hosts/planner-delivery/gateway-requests.json', alter)
        self.rejected('tamper refusal/normal delivery')

    def test_owned_process_audit_cannot_mark_live_child_complete(self):
        self.change('postrun-process-audit.json', lambda d: d['probes'][0].update(exit_code=0, stdout='live'))
        self.rejected('owned PID audit')

    def test_historical_guards_cannot_be_relabelled_as_current(self):
        self.change('integration-result.json', lambda d: d.update(historical_results_are_current_source_tests=True))
        self.rejected('historical results mislabelled')

    def test_protected_operational_pointer_changes_are_rejected(self):
        self.change('scope-preservation.json', lambda d: d.update(operational_metadata_pointers_equal=False))
        self.rejected('protected scope changed')

    def test_missing_artifact_is_rejected(self):
        (self.packet / 'binary-pins.json').unlink()
        self.rejected('packet inventory')


# Later stages changed inputs this packet binds; run at its recorded revision.
load_tests = evidence_revision.tests(__file__)


if __name__ == '__main__':
    unittest.main()
