import hashlib
import json
from pathlib import Path
import tempfile
import unittest
from license_inventory import collect


class InventoryTests(unittest.TestCase):
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


if __name__ == '__main__':
    unittest.main()
