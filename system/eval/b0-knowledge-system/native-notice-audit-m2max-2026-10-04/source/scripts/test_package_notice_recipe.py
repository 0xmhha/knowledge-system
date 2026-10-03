"""Native attribution recipe must not silently describe different source bytes."""
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

SPEC = importlib.util.spec_from_file_location('package_host', Path(__file__).with_name('package-host.py'))
HOST = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(HOST)


class NativeRecipeTests(unittest.TestCase):
    def test_recipe_binds_version_source_set_source_bytes_and_notice(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp); cache = root / 'cache'; go = root / 'go'
            source = cache / 'github.com/tree-sitter/go-tree-sitter@v0.25.0/src/example.c'
            source.parent.mkdir(parents=True); source.write_bytes(b'original native source')
            notice = root / 'third-party-notices/tree-sitter/LICENSE'
            notice.parent.mkdir(parents=True); notice.write_bytes(b'upstream notice')
            (go / 'src/vendor/example').mkdir(parents=True)
            (go / 'LICENSE').write_bytes(b'Go notice')
            (go / 'src/vendor/example/LICENSE').write_bytes(b'vendor notice')
            solidity = root / 'internal/graph/parse/solidity/binding/parser.c'
            solidity.parent.mkdir(parents=True); solidity.write_bytes(b'original Solidity parser')
            recipe = {'module': 'github.com/tree-sitter/go-tree-sitter', 'version': 'v0.25.0',
                      'runtime_c_sources': {'src/example.c': HOST.digest(source)},
                      'license_sha256': HOST.digest(notice),
                      'solidity_upstream_sources': [{'engine': 'graph', 'path': 'src/parser.c',
                                                    'local_sha256': HOST.digest(solidity)}]}
            (notice.parent / 'provenance.json').write_text(json.dumps(recipe))
            original_root = HOST.ROOT; HOST.ROOT = root
            self.addCleanup(setattr, HOST, 'ROOT', original_root)
            modules = 'dep github.com/tree-sitter/go-tree-sitter v0.25.0'
            assets = HOST.supplementary_notices(modules, cache, go)
            self.assertEqual(len(assets), 2)
            self.assertEqual(assets[1][2], ['LICENSE', 'src/vendor/example/LICENSE'])
            with self.assertRaisesRegex(ValueError, 'version review'):
                HOST.supplementary_notices(modules.replace('v0.25.0', 'v0.26.0'), cache, go)
            solidity.write_bytes(b'changed Solidity parser')
            with self.assertRaisesRegex(ValueError, 'local Solidity source differs'):
                HOST.supplementary_notices(modules, cache, go)
            solidity.write_bytes(b'original Solidity parser')
            source.write_bytes(b'changed native source')
            with self.assertRaisesRegex(ValueError, 'source differs'):
                HOST.supplementary_notices(modules, cache, go)
            source.write_bytes(b'original native source')
            extra = source.with_name('new.h'); extra.write_text('new include')
            with self.assertRaisesRegex(ValueError, 'source set differs'):
                HOST.supplementary_notices(modules, cache, go)
            extra.unlink(); notice.write_bytes(b'changed upstream notice')
            with self.assertRaisesRegex(ValueError, 'notice differs'):
                HOST.supplementary_notices(modules, cache, go)


if __name__ == '__main__':
    unittest.main()
