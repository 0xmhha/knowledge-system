"""Dependency metadata retains identities and diagnostics but not load addresses."""
import importlib.util
from pathlib import Path
import subprocess
import unittest
from unittest.mock import patch

SPEC = importlib.util.spec_from_file_location('package_host', Path(__file__).with_name('package-host.py'))
HOST = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(HOST)


class DependencyMetadataTests(unittest.TestCase):
    def inspect(self, system, output):
        with patch.object(HOST.platform, 'system', return_value=system), \
             patch.object(HOST.shutil, 'which', return_value='/usr/bin/inspector'), \
             patch.object(HOST.subprocess, 'check_output', return_value=output):
            return HOST.native_dependencies(Path('/temporary/stage/cks'))

    def test_linux_retains_all_dependencies_versions_and_missing_libraries(self):
        output = ('\tlinux-vdso.so.1 (0x0000ffff12340000)\n'
                  '\tlibm.so.6 => /lib/libm.so.6 (0x0000ffff22220000)\n'
                  '\tlibversion.so (ABI 0x123) => not found\n'
                  '\t/lib/ld-linux-aarch64.so.1 (0x0000ffff33330000)\n')
        expected = ['\tlinux-vdso.so.1', '\tlibm.so.6 => /lib/libm.so.6',
                    '\tlibversion.so (ABI 0x123) => not found', '\t/lib/ld-linux-aarch64.so.1']
        self.assertEqual(self.inspect('Linux', output), expected)
        self.assertEqual(self.inspect('Linux', output.replace('ffff', 'abcd')), expected)

    def test_static_binary_diagnostic_is_not_discarded(self):
        self.assertEqual(self.inspect('Linux', '\tstatically linked\n'), ['\tstatically linked'])

    def test_darwin_header_is_removed_and_versions_are_unchanged(self):
        output = ('/temporary/stage/cks:\n'
                  '\t/usr/lib/libSystem.B.dylib (compatibility version 1.0.0, current version 1359.0.0)\n')
        self.assertEqual(self.inspect('Darwin', output),
                         ['\t/usr/lib/libSystem.B.dylib (compatibility version 1.0.0, current version 1359.0.0)'])

    def test_inspector_failure_remains_a_diagnostic(self):
        with patch.object(HOST.platform, 'system', return_value='Linux'), \
             patch.object(HOST.shutil, 'which', return_value='/usr/bin/ldd'), \
             patch.object(HOST.subprocess, 'check_output', side_effect=subprocess.CalledProcessError(
                 1, ['ldd'], output='not a dynamic executable\n')):
            self.assertEqual(HOST.native_dependencies(Path('/temporary/stage/cks')),
                             ['dependency inspection failed: not a dynamic executable'])


if __name__ == '__main__':
    unittest.main()
