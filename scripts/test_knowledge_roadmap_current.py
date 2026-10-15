"""Current roadmap regression entrypoint; retain the frozen foundation suite.

Earlier tests selected KM-01 and KM-03.1, which are now complete.
Keep all seven checks but choose an actually unfinished feature/task each time.
The original script and its source-bound evidence remain immutable.
"""
import importlib.util
import json
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location('foundation_roadmap_tests', Path(__file__).with_name('test_knowledge_roadmap.py'))
foundation = importlib.util.module_from_spec(spec)
spec.loader.exec_module(foundation)


class CurrentRoadmapTests(foundation.RoadmapTests):
    def test_partial_feature_cannot_be_marked_complete(self):
        def promote(data):
            unfinished = next(item for item in data['items'] if any(s['status'] != 'complete' for s in item['subtasks']))
            unfinished['status'] = 'complete'
        self.edit_json('docs/spec-driven/KNOWLEDGE-ROADMAP-PROGRESS.json', promote)
        self.assertTrue(foundation.guard.check(self.folder, self.root))


    def unfinished_id(self):
        data = json.loads((self.root / 'docs/spec-driven/KNOWLEDGE-ROADMAP-PROGRESS.json').read_text())
        return next(s['id'] for item in data['items'] for s in item['subtasks'] if s['status'] != 'complete')

    def test_unfinished_checkbox_cannot_be_ticked(self):
        qid = self.unfinished_id()
        path = self.root / 'docs/spec-driven/KNOWLEDGE-MANAGER-AND-COMPARISON-SPEC.md'
        before = path.read_text()
        marker = '- [ ] ' + qid + ':'
        self.assertEqual(before.count(marker), 1)
        path.write_text(before.replace(marker, '- [x] ' + qid + ':'))
        self.assertTrue(foundation.guard.check(self.folder, self.root))

    def test_matrix_cannot_promote_pending_implementation(self):
        qid = self.unfinished_id()
        path = self.root / 'docs/spec-driven/KNOWLEDGE-ROADMAP-TEST-MATRIX.md'
        lines = path.read_text().splitlines()
        changed = 0
        for index, line in enumerate(lines):
            parts = line.split('|')
            if len(parts) >= 6 and parts[1].strip() == qid:
                self.assertNotEqual(parts[3].strip(), '완료')
                parts[3] = ' 완료 '
                lines[index] = '|'.join(parts)
                changed += 1
        self.assertEqual(changed, 1)
        path.write_text('\n'.join(lines) + '\n')
        self.assertTrue(foundation.guard.check(self.folder, self.root))


if __name__ == '__main__':
    unittest.main()
