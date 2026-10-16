#!/usr/bin/env python3
"""Adversarial checks for source scope accounting and formal-state boundaries."""
import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]
PACKET = ROOT / 'docs/spec-driven/operational-preparation/source-review-supplement/lexical-coverage-v1'


def load(name):
    spec = importlib.util.spec_from_file_location(name.replace('-', '_'), ROOT / 'scripts' / name)
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


guard = load('check-lexical-coverage.py')
prepare = load('prepare-lexical-coverage.py')


class LexicalCoverageTests(unittest.TestCase):
    def setUp(self):
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        self.packet = Path(tmp.name) / 'packet'
        shutil.copytree(PACKET, self.packet)

    def rehash(self):
        path = self.packet / 'manifest.json'
        manifest = json.loads(path.read_text())
        manifest['artifacts'] = {p.relative_to(self.packet).as_posix(): guard.sha(p.read_bytes())
                                 for p in self.packet.rglob('*') if p.is_file() and p.name != 'manifest.json'}
        path.write_text(json.dumps(manifest))

    def change(self, name, fn):
        path = self.packet / name
        value = json.loads(path.read_text())
        fn(value)
        path.write_text(json.dumps(value, ensure_ascii=False))
        self.rehash()

    def rewrite_census(self, fn):
        self.change('logs/analyzer.json', fn)
        rows = json.loads((self.packet / 'logs/analyzer.json').read_text())
        pool = json.loads((ROOT / guard.POOL).read_text())
        (self.packet / 'coverage.json').write_text(json.dumps(prepare.summarize(pool, rows)))
        self.rehash()

    def rejected(self, expected=None):
        errors = guard.check(self.packet, check_references=False)
        self.assertTrue(errors)
        if expected:
            self.assertIn(expected, errors)

    def test_real_packet_and_replay(self):
        self.assertEqual(guard.check(PACKET), [])

    def test_old_name_hint_cannot_replace_real_scope(self):
        def mutate(rows):
            row = next(r for r in rows if r['name'] == 'Status')
            row['features']['local_variable_shadow'] = True
        self.rewrite_census(mutate)
        self.rejected('actual scope replay mismatch')

    def test_nested_closure_loop_cannot_inflate_enclosing_function(self):
        def mutate(rows):
            row = next(r for r in rows if not r['features'].get('loop'))
            row['features']['loop'] = True
        self.rewrite_census(mutate)
        self.rejected('actual scope replay mismatch')

    def test_outer_binding_substitution_after_recount(self):
        def mutate(rows):
            row = next(r for r in rows if r['features'].get('local_variable_shadow'))
            e = next(e for e in row['events'] if e['kind'] == 'local_variable_shadow')
            e['outer'] = e['binding']
        self.rewrite_census(mutate)
        self.rejected('actual scope replay mismatch')

    def test_remove_shadow_after_recount(self):
        def mutate(rows):
            row = next(r for r in rows if r['features'].get('local_variable_shadow'))
            row['events'] = [e for e in row['events'] if e['kind'] != 'local_variable_shadow']
            row['features']['local_variable_shadow'] = False
        self.rewrite_census(mutate)
        self.rejected('actual scope replay mismatch')

    def test_short_reuse_promoted_to_shadow(self):
        def mutate(rows):
            row = next(r for r in rows if r['features'].get('same_scope_short_reuse'))
            next(e for e in row['events'] if e['kind'] == 'same_scope_short_reuse')['kind'] = 'assignment'
        self.rewrite_census(mutate)
        self.rejected('actual scope replay mismatch')

    def test_stress_main_counted_as_production(self):
        self.change('coverage.json', lambda d: d.update(production_candidates=40, development_stress_source_only=0))
        self.rejected('coverage counts/claims mismatch')

    def test_independence_claim(self):
        self.change('coverage.json', lambda d: d['requirements_check'].update(approved_independent_functions=40))
        self.rejected('coverage counts/claims mismatch')

    def test_ai_source_as_dependency_gold(self):
        self.change('coverage.json', lambda d: d['functions'][0].update(exhaustive_data_edges=[], quality_scoring_allowed=True))
        self.rejected('coverage counts/claims mismatch')

    def test_final_allocation(self):
        self.change('coverage.json', lambda d: d['functions'][0].update(split='FINAL'))
        self.rejected('coverage counts/claims mismatch')

    def test_precision_score_claim(self):
        self.change('coverage.json', lambda d: d.update(data_edge_precision=1.0))
        self.rejected('coverage counts/claims mismatch')

    def test_missing_function_in_analyzer_input(self):
        self.change('analyzer-inputs.json', lambda d: d[0]['Starts'].pop())
        self.rejected('analyzer input/selection mismatch')

    def test_source_substitution_despite_updated_hash(self):
        proof_path = self.packet / 'git-proof.json'
        proof = json.loads(proof_path.read_text())
        record = next(iter(proof['files'].values()))
        source = self.packet / record['artifact']
        raw = source.read_bytes() + b'// replacement\n'
        source.write_bytes(raw)
        record.update(sha256=guard.sha(raw))
        proof_path.write_text(json.dumps(proof))
        self.rehash()
        self.rejected()

    def test_tool_source_substitution(self):
        p = self.packet / 'main.go.txt'
        p.write_bytes(p.read_bytes() + b'// replaced\n')
        self.rehash()
        self.rejected('analyzer source binding: scripts/lexical_coverage.go.txt')

    def test_model_execution_claim(self):
        self.change('validation.json', lambda d: d.update(model_calls=1))
        self.rejected('execution boundary')

    def test_project_typecheck_claim(self):
        self.change('validation.json', lambda d: d.update(project_packages_compiled=28))
        self.rejected('execution boundary')

    def test_raw_test_failure_despite_pass_summary(self):
        p = self.packet / 'logs/race.jsonl'
        p.write_text(p.read_text() + json.dumps({'Action': 'fail', 'Test': 'TestLexicalCoverage'}) + '\n')
        self.rehash()
        self.rejected('control raw completion')

    def test_go_types_observation_tamper(self):
        p = self.packet / 'logs/normal.jsonl'
        p.write_text(p.read_text().replace('go_types_match=true', 'go_types_match=false'))
        self.rehash()
        self.rejected('go/types control observations')

    def test_format_failure_despite_exit_code(self):
        (self.packet / 'logs/fmt.stdout.txt').write_text('main.go\n')
        self.rehash()
        self.rejected('unformatted helper')

    def test_frozen_protocol_promotion(self):
        self.change('formal-state.json', lambda d: d.update(execution_allowed=True))
        self.rejected('formal state changed')


if __name__ == '__main__':
    unittest.main()
