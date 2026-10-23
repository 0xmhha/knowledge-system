"""Keep reindexed snapshot indexes aligned with their own snapshot's graph."""
import json
import tempfile
import unittest
from pathlib import Path

import retrieval_snapshot_chain as x


class Chain(unittest.TestCase):
    def test_copied_index_is_pointed_at_this_snapshot_graph(self):
        with tempfile.TemporaryDirectory() as tmp:
            vector = Path(tmp) / 'vector'
            vector.mkdir()
            (vector / 'manifest.json').write_text(json.dumps({'sources': {'ckg': {'path': '/old/graph', 'digest': 'd'}}, 'indexed_head': 'a'}))
            x.point_at_graph(vector, Path('/new/graph'))
            data = json.loads((vector / 'manifest.json').read_text())
            self.assertEqual(data['sources']['ckg'], {'path': '/new/graph', 'digest': 'd'})
            self.assertEqual(data['indexed_head'], 'a')

    def test_graph_ready_requires_db_and_matching_head(self):
        with tempfile.TemporaryDirectory() as tmp:
            graph = Path(tmp) / 'graph'
            graph.mkdir()
            self.assertFalse(x.graph_ready(graph, 'abc'))
            (graph / 'manifest.json').write_text(json.dumps({'src_commit': 'abc'}))
            self.assertFalse(x.graph_ready(graph, 'abc'))
            (graph / 'graph.db').write_bytes(b'')
            self.assertTrue(x.graph_ready(graph, 'abc'))
            self.assertFalse(x.graph_ready(graph, 'def'))

    def test_model_slug(self):
        self.assertEqual(x.model_slug('qwen3-embedding:4b', 0), 'qwen3-embedding-4b')
        self.assertEqual(x.model_slug('qwen3-embedding:4b', 1024), 'qwen3-embedding-4b-d1024')


if __name__ == '__main__':
    unittest.main()
