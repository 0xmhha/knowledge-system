"""Current roadmap regression entrypoint; retain the frozen foundation suite.

Earlier tests selected KM-01 and KM-03.1, which are now complete.
Keep all seven checks, using a consistent synthetic pending task after closure.
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
    def pending_fixture(self):
        data = json.loads((self.root / 'docs/spec-driven/KNOWLEDGE-ROADMAP-PROGRESS.json').read_text())
        pending = [s['id'] for i in data['items'] for s in i['subtasks'] if s['status'] != 'complete']
        if pending:
            return pending[0]
        # Only the disposable test tree is changed. Prove the pending baseline
        # is valid before testing an inconsistent promotion of any one view.
        item = data['items'][-1]
        task = item['subtasks'][-1]
        task['status'] = 'not_started'
        item['status'] = 'in_progress'
        data['completed_subtasks'] -= 1
        data['implementation_completed'] -= 1
        (self.root / 'docs/spec-driven/KNOWLEDGE-ROADMAP-PROGRESS.json').write_text(json.dumps(data))
        qid = task['id']
        path = self.root / data['spec']
        path.write_text(path.read_text().replace('- [x] ' + qid + ':', '- [ ] ' + qid + ':'))
        path = self.root / data['test_matrix']
        lines = path.read_text().replace('전체 완료는7/7', '전체 완료는6/7').splitlines()
        for index, line in enumerate(lines):
            parts = line.split('|')
            if len(parts) >= 6 and parts[1].strip() == qid:
                parts[3] = ' 미착수 '
                lines[index] = '|'.join(parts)
        path.write_text('\n'.join(lines) + '\n')
        self.assertEqual(foundation.guard.check(self.folder, self.root), [])
        return qid

    def test_partial_feature_cannot_be_marked_complete(self):
        self.pending_fixture()
        def promote(data):
            unfinished = next(item for item in data['items'] if any(s['status'] != 'complete' for s in item['subtasks']))
            unfinished['status'] = 'complete'
        self.edit_json('docs/spec-driven/KNOWLEDGE-ROADMAP-PROGRESS.json', promote)
        self.assertTrue(foundation.guard.check(self.folder, self.root))


    def unfinished_id(self):
        return self.pending_fixture()

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
