"""The historical-test record must name real revisions and refuse changed evidence."""
import json
from pathlib import Path
import subprocess
import unittest

import evidence_revision as er
import recorded_locations as rl

HOOK = 'load_tests = evidence_revision.tests(__file__)'


def history(commit):
    return subprocess.run(['git', 'merge-base', '--is-ancestor', commit, json.loads(er.REVISIONS.read_text())['history_tag']],
                          cwd=er.ROOT, capture_output=True).returncode == 0


class Record(unittest.TestCase):
    def setUp(self):
        self.modules = json.loads(er.REVISIONS.read_text())['modules']

    def test_every_hooked_test_is_recorded_and_every_record_hooked(self):
        hooked = {p.stem for p in er.SCRIPTS.glob('test_*.py') if HOOK in p.read_text().splitlines()}
        self.assertEqual(hooked, set(self.modules))

    def test_runs_name_retained_commits_and_packets(self):
        for name in self.modules:
            for run in er.revisions(name):
                with self.subTest(module=name, commit=run['commit']):
                    self.assertTrue(history(run['commit']))
                    for path in run['packets']:
                        er.unchanged(run.get('overlay', {}).get('commit', run['commit']) if path in run.get('overlay', {}).get('paths', []) else run['commit'], [path])
                    for portable in run.get('portable', []):
                        self.assertTrue((er.SCRIPTS / portable).is_file())

    def test_split_runs_partition_by_name(self):
        for name, row in self.modules.items():
            if 'runs' in row:
                only = [n for r in row['runs'] for n in r.get('only', [])]
                excluded = [n for r in row['runs'] for n in r.get('exclude', [])]
                self.assertEqual(sorted(only), sorted(excluded), name)


class Guards(unittest.TestCase):
    def test_unknown_module_refused(self):
        with self.assertRaises(er.RevisionUnavailable):
            er.revisions('test_not_recorded')

    def test_modified_file_refused(self):
        # Every hooked test file gained its hook after its recorded commit.
        name, row = next(iter(json.loads(er.REVISIONS.read_text())['modules'].items()))
        commit = row.get('runs', [row])[0]['commit']
        with self.assertRaises(er.RevisionUnavailable):
            er.unchanged(commit, ['scripts/' + name + '.py'])

    def test_missing_commit_refused(self):
        with self.assertRaises(er.RevisionUnavailable):
            with er.checkout('0' * 40):
                pass

    def test_selection_keeps_order_and_names(self):
        class Sample(unittest.TestCase):
            def test_a(self):
                pass

            def test_b(self):
                pass
        suite = unittest.defaultTestLoader.loadTestsFromTestCase(Sample)
        prefix = Sample.__qualname__
        self.assertEqual([t._testMethodName for t in er._select(suite, dict(exclude=[prefix + '.test_a']))], ['test_b'])
        self.assertEqual([t._testMethodName for t in er._select(suite, dict(only=[prefix]))], ['test_a', 'test_b'])
        self.assertEqual(list(er._select(suite, dict(only=[prefix + '.test_']))), [])


class Locations(unittest.TestCase):
    def test_recorded_sdk_maps_by_tool(self):
        recorded = '/Users/someone/.gvm/gos/go1.23.12/bin/go'
        self.assertEqual(rl.tool_name(recorded), 'bin/go')
        self.assertEqual(rl.local(recorded), rl.root() / 'bin/go')
        with self.assertRaises(ValueError):
            rl.tool_name('/usr/local/go/bin/go')

    def test_env_moves_only_recorded_sdk(self):
        env = rl.local_env(dict(GOROOT='/Users/someone/.gvm/gos/go1.23.12', PATH='/Users/someone/.gvm/gos/go1.23.12/bin:/usr/bin', GOCACHE='/x'))
        self.assertEqual(env['GOROOT'], str(rl.root()))
        self.assertEqual(env['PATH'], str(rl.root() / 'bin') + ':/usr/bin')
        self.assertEqual(env['GOCACHE'], '/x')

    def test_run_checkout_path_by_repository_tail(self):
        self.assertTrue(rl.in_run_checkout('/a/b/pkg/graph/valueflow/rules_test.go', Path('pkg/graph/valueflow/rules_test.go')))
        self.assertFalse(rl.in_run_checkout('pkg/graph/valueflow/rules_test.go', 'pkg/graph/valueflow/rules_test.go'))
        self.assertFalse(rl.in_run_checkout('/a/b/pkg/graph/other_test.go', 'pkg/graph/valueflow/rules_test.go'))


if __name__ == '__main__':
    unittest.main()
