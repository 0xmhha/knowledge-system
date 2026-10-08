import hashlib
import json
from pathlib import Path
import tempfile
import unittest
from license_inventory import collect


class InventoryTests(unittest.TestCase):
    def test_translated_sqlite_notice_is_collected_and_missing_is_visible(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            cache = root / 'cache'
            module = cache / 'modernc.org/sqlite@v1.54.0'
            module.mkdir(parents=True)
            (module / 'LICENSE').write_bytes(b'Go wrapper notice\n')
            notice = module / 'SQLITE-LICENSE'
            notice.write_bytes(b'upstream SQLite notice\n')
            stage = root / 'stage'; stage.mkdir()
            summary = collect('dep modernc.org/sqlite v1.54.0', cache, stage)
            entry = json.loads((stage / 'third-party-licenses.json').read_text())['modules'][0]
            copied = [stage / f['path'] for f in entry['license_files']]
            self.assertIn(b'upstream SQLite notice\n', [p.read_bytes() for p in copied])
            self.assertEqual(entry['required_native_notice_paths'], ['SQLITE-LICENSE'])
            self.assertEqual(summary['missing_license_count'], 0)
            notice.unlink()
            second = root / 'second'; second.mkdir()
            summary = collect('dep modernc.org/sqlite v1.54.0', cache, second)
            entry = json.loads((second / 'third-party-licenses.json').read_text())['modules'][0]
            self.assertEqual(summary['missing_license_count'], 1)
            self.assertEqual(entry['missing_required_notices'], ['SQLITE-LICENSE'])
            self.assertEqual(entry['review_status'], 'missing_source_license')

    def test_required_native_nested_notice_is_collected_and_missing_is_visible(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            cache = root / 'cache'
            module = cache / 'github.com/tree-sitter/go-tree-sitter@v0.25.0'
            (module / 'src/unicode').mkdir(parents=True)
            (module / 'LICENSE').write_text('Go wrapper notice')
            (module / 'src/unicode/LICENSE').write_text('Unicode native notice')
            stage = root / 'stage'; stage.mkdir()
            summary = collect('dep github.com/tree-sitter/go-tree-sitter v0.25.0', cache, stage)
            inventory = json.loads((stage / 'third-party-licenses.json').read_text())
            paths = [f['path'] for f in inventory['modules'][0]['license_files']]
            self.assertTrue(any(p.endswith('/src/unicode/LICENSE') for p in paths))
            self.assertEqual(summary['missing_license_count'], 0)
            (module / 'src/unicode/LICENSE').unlink()
            second = root / 'second'; second.mkdir()
            summary = collect('dep github.com/tree-sitter/go-tree-sitter v0.25.0', cache, second)
            self.assertEqual(summary['missing_license_count'], 1)

    def test_vendored_notice_and_source_hashes_are_collected(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            cache = root / 'cache'
            module = cache / 'example.org/lib@v1.0.0'
            module.mkdir(parents=True)
            (module / 'LICENSE').write_text('module license')
            asset = root / 'native'
            (asset / 'tree_sitter').mkdir(parents=True)
            (asset / 'LICENSE').write_text('native license')
            (asset / 'parser.c').write_text('native code')
            (asset / 'tree_sitter/parser.h').write_text('header')
            stage = root / 'stage'
            stage.mkdir()
            summary = collect('dep example.org/lib v1.0.0', cache, stage, [('grammar', asset)])
            inventory = json.loads((stage / 'third-party-licenses.json').read_text())
            self.assertEqual(summary['vendored_asset_count'], 1)
            self.assertEqual(summary['missing_vendored_license_count'], 0)
            entry = inventory['vendored_assets'][0]
            self.assertEqual(len(entry['source_files']), 2)
            for file in entry['license_files']:
                self.assertEqual(hashlib.sha256((stage / file['path']).read_bytes()).hexdigest(), file['sha256'])
            for file in entry['source_files']:
                self.assertEqual(hashlib.sha256((asset / file['path']).read_bytes()).hexdigest(), file['sha256'])
            self.assertEqual(inventory['review_status'], 'pending')

    def test_missing_native_license_is_an_explicit_review_item(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            stage = root / 'stage'
            stage.mkdir()
            asset = root / 'native'
            asset.mkdir()
            (asset / 'parser.c').write_text('code')
            summary = collect('dep example.org/lib v1.0.0', root / 'cache', stage, [('grammar', asset)])
            self.assertEqual(summary['missing_vendored_license_count'], 1)
            inventory = json.loads((stage / 'third-party-licenses.json').read_text())
            self.assertEqual(inventory['vendored_assets'][0]['review_status'], 'missing_source_license')

    def test_explicit_notice_assets_preserve_bytes_and_refuse_path_escape(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            source = root / 'notices'; source.mkdir()
            (source / 'LICENSE').write_bytes(b'upstream notice\n')
            stage = root / 'stage'; stage.mkdir()
            collect('dep example.org/lib v1.0.0', root / 'cache', stage,
                    notice_assets=[('runtime', source, ['LICENSE'])])
            inventory = json.loads((stage / 'third-party-licenses.json').read_text())
            asset = inventory['vendored_assets'][0]
            self.assertEqual(asset['source_files'], [])
            self.assertEqual((stage / asset['license_files'][0]['path']).read_bytes(), b'upstream notice\n')
            for relative in ('../LICENSE', '/LICENSE', 'a/../LICENSE', 'a\\LICENSE'):
                with self.subTest(relative=relative), self.assertRaisesRegex(ValueError, 'unsafe notice path'):
                    collect('dep example.org/lib v1.0.0', root / 'cache', stage,
                            notice_assets=[('runtime', source, [relative])])


if __name__ == '__main__':
    unittest.main()
