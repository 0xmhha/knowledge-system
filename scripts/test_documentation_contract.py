import importlib.util
from pathlib import Path
import tempfile
import unittest

import documentation_contract as contract

spec = importlib.util.spec_from_file_location('docscheck', Path(__file__).with_name('check-docs.py'))
docscheck = importlib.util.module_from_spec(spec)
spec.loader.exec_module(docscheck)


class DocumentationContractTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        (self.root / 'docs').mkdir()
        (self.root / 'docs/current.md').write_text('# Current\n## 한국어 제목\n## Repeat\n## Repeat\n')

    def test_new_unclassified_document_is_rejected(self):
        catalog = {'schema_version': 1, 'documents': {'docs/current.md': {'role': 'reference'}}}
        findings = contract.validate_catalog(self.root, catalog, {'docs/current.md', 'docs/new.md'})
        self.assertTrue(any('missing from' in f[2] for f in findings))

    def test_deleted_catalog_entry_is_rejected(self):
        findings = contract.validate_catalog(self.root, {'schema_version': 1, 'documents': {'docs/deleted.md': {'role': 'reference'}}}, set())
        self.assertTrue(any('missing' in f[2] for f in findings))

    def test_current_document_cannot_claim_frozen_role(self):
        findings = contract.validate_catalog(self.root, {'schema_version': 1, 'documents': {'docs/current.md': {'role': 'frozen_evidence'}}}, {'docs/current.md'})
        self.assertTrue(any('cannot bypass' in f[2] for f in findings))

    def test_parent_escape_is_rejected(self):
        findings = contract.validate_catalog(self.root, {'schema_version': 1, 'documents': {'../secret.md': {'role': 'reference'}}}, {'../secret.md'})
        self.assertTrue(any('invalid catalog path' in f[2] for f in findings))

    def test_frozen_source_is_not_reclassified_as_current(self):
        p = self.root / 'system/eval/snapshot/source.md'
        p.parent.mkdir(parents=True)
        p.write_text('# Recorded\n')
        findings = contract.validate_catalog(self.root, {'schema_version': 1, 'documents': {'system/eval/snapshot/source.md': {'role': 'reference'}}}, {'system/eval/snapshot/source.md'})
        self.assertTrue(any('must use role' in f[2] for f in findings))

    def test_missing_local_target_is_rejected(self):
        self.assertTrue(contract.validate_links(self.root, 'docs/current.md', '[old](deleted.md)'))

    def test_leftover_empty_directory_link_is_rejected(self):
        (self.root / 'docs/removed-adr').mkdir()
        findings = contract.validate_links(self.root, 'docs/current.md',
            '[old](removed-adr/)', repository_files={'docs/current.md'})
        self.assertTrue(any('no repository files' in f[2] for f in findings))

    def test_repository_directory_link_resolves(self):
        self.assertEqual([], contract.validate_links(self.root, 'docs/current.md',
            '[current](./)', repository_files={'docs/current.md'}))

    def test_missing_heading_is_rejected(self):
        self.assertTrue(contract.validate_links(self.root, 'docs/current.md', '[old](current.md#removed)'))

    def test_unicode_and_duplicate_headings_resolve(self):
        text = '[ko](current.md#한국어-제목) [second](current.md#repeat-1)'
        self.assertEqual([], contract.validate_links(self.root, 'docs/current.md', text))

    def test_fenced_example_and_external_record_are_not_local_paths(self):
        text = '```md\n[example](missing.md)\n```\n[history](https://example.com/blob/revision/old.md)'
        self.assertEqual([], contract.validate_links(self.root, 'docs/current.md', text))

    def test_reference_link_target_is_validated(self):
        self.assertTrue(contract.validate_links(self.root, 'docs/current.md', '[guide]: gone.md "guide"'))

    def test_catalogued_current_handoff_is_checked(self):
        rows = {'docs/current.md': {'role': 'workflow'}}
        self.assertTrue(docscheck.should_check('docs/current.md', '# Handoff\narchive of decisions', rows))

    def test_unknown_subcommand_is_rejected_in_command_context(self):
        findings = []
        docscheck.check_invocation('docs/current.md', 1, 'cks missing --help', {'cks': {'flags': {'help'}, 'subs': {'setup': {'flags': {'help'}, 'subs': {}}}}}, findings, strict_subcommands=True)
        self.assertTrue(any('no subcommand' in f[2] for f in findings))

    def test_unknown_continuation_flag_is_rejected(self):
        lines = list(docscheck.logical_lines('cks setup \\\n  --imaginary value'))
        findings = []
        docscheck.check_invocation('docs/current.md', lines[0][0], lines[0][1], {'cks': {'flags': set(), 'subs': {'setup': {'flags': {'help'}, 'subs': {}}}}}, findings, strict_subcommands=True)
        self.assertTrue(any('imaginary' in f[2] for f in findings))

    def test_current_catalog_paths_are_checked_outside_old_globs(self):
        rows = {'internal/parser/README.md': {'role': 'reference'},
                'system/eval/input.md': {'role': 'frozen_evidence'}}
        self.assertEqual({'internal/parser/README.md'}, docscheck.current_reference_paths(rows))

    def flag_grammar(self):
        return {'cks': {'flags': {'help'}, 'subs': {
            'patch': {'flags': {'dataset', 'verbose'}, 'value_flags': {'dataset'}, 'subs': {
                'record-run': {'flags': {'report'}, 'value_flags': {'report'}, 'subs': {}}
            }}
        }}}

    def test_value_flags_before_subcommand_do_not_hide_it(self):
        findings = []
        docscheck.check_invocation('docs/current.md', 1,
            'cks patch --dataset "data path" record-run --report run.json',
            self.flag_grammar(), findings, strict_subcommands=True)
        self.assertEqual([], findings)

    def test_boolean_flags_do_not_consume_subcommand(self):
        findings = []
        docscheck.check_invocation('docs/current.md', 1,
            'cks patch --verbose missing', self.flag_grammar(), findings,
            strict_subcommands=True)
        self.assertTrue(any('no subcommand' in f[2] for f in findings))

    def test_shell_comments_and_redirections_are_not_subcommands(self):
        for tail in ['# operator note', '> log.txt', '2>/dev/null']:
            with self.subTest(tail=tail):
                findings = []
                docscheck.check_invocation('docs/current.md', 1,
                    'cks patch ' + tail, self.flag_grammar(), findings,
                    strict_subcommands=True)
                self.assertEqual([], findings)

    def test_unknown_flag_before_comment_is_still_rejected(self):
        findings = []
        docscheck.check_invocation('docs/current.md', 1,
            'cks patch --imaginary # operator note', self.flag_grammar(), findings,
            strict_subcommands=True)
        self.assertTrue(any('imaginary' in f[2] for f in findings))


if __name__ == '__main__':
    unittest.main()
