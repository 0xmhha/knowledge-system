"""Current roadmap regression entrypoint; retain the frozen foundation suite.

The original partial-feature test selected KM-01, which is now fully complete.
Reuse its seven checks with the partial test aimed at an unfinished feature.
The original script and its source-bound evidence remain immutable.
"""
import importlib.util
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


if __name__ == '__main__':
    unittest.main()
