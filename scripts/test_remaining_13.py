"""Reject completion flags without the declared outcome evidence."""
import importlib.util
import json
import unittest
from pathlib import Path

spec = importlib.util.spec_from_file_location('ledger_guard', Path(__file__).with_name('check-remaining-13.py'))
guard = importlib.util.module_from_spec(spec)
spec.loader.exec_module(guard)


class Ledger(unittest.TestCase):
    def setUp(self):
        self.data = json.loads((guard.ROOT/guard.LEDGER).read_text())

    def test_current_ledger(self):
        self.assertEqual(guard.check(self.data), [])

    def test_flag_and_count_alone_cannot_complete_work(self):
        self.data['rows'][0]['completion_status'] = 'complete'
        self.data['groups_completed'] = 1
        self.assertTrue(guard.check(self.data))

    def test_group_cannot_disappear(self):
        self.data['rows'].pop()
        self.assertTrue(guard.check(self.data))

    def test_draft_cannot_freeze_or_require_personal_names(self):
        self.data['freeze'] = 'approved-by-ai'
        self.data['personal_names_required'] = True
        self.assertEqual(len(guard.check(self.data)), 2)


if __name__ == '__main__':
    unittest.main()
