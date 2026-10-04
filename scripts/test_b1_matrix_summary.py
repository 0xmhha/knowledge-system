"""Failure oracles for paired, clustered citation and timing report preparation."""
import copy
import importlib.util
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

SPEC = importlib.util.spec_from_file_location('matrix_summary', Path(__file__).with_name('b1-summarize-matrix.py'))
M = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(M)


def sealed(pack):
    pack['metadata'].pop('integrity_hash', None)
    pack['metadata']['integrity_hash'] = M.sha(json.dumps(pack, ensure_ascii=False, sort_keys=True, separators=(',', ':')).encode())
    return pack


class PairedReportTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.source = Path(self.temp.name) / 'source'
        self.source.mkdir()
        self.raw = 'package fixture\n\nfunc Alpha() {} // 한글\nfunc Beta() {}\n'.encode()
        (self.source / 'main.go').write_bytes(self.raw)
        self.commit = 'c' * 40
        self.identity = {'source': {'project_id': 'fixture', 'source_commit': self.commit,
                                    'source_mode': 'committed', 'snapshot_id': 'b' * 64}, 'dataset_id': 'd' * 64}
        self.requests = {'requests': [{'id': q, 'arguments': {'prompt': 'Alpha', 'knowledge_as_of': '2026-10-01',
                                                               'knowledge_subsystem': q}} for q in ['alpha', 'alpha-variant']]}
        self.cases = {q['id']: {'group': 'code', 'request_id': q['id'], 'independent_unit_id': 'alpha-common',
                              'expected_commit': self.commit, 'expect_no_citations': False,
                              'expected_citations': [{'file': 'main.go', 'start_line': 3, 'end_line': 3}]}
                      for q in self.requests['requests']}
        self.report = {'schema_version': 1, 'state': 'captured', 'counts': {'warmup': 1, 'retrieval': 2, 'warm_latency': 3, 'cold_process': 1},
                       'arms': [{'id': a, 'config_sha256': 'a', 'config_sha256_after': 'a'} for a in M.ARMS],
                       'binary_sha256': 'a', 'binary_sha256_after': 'a',
                       'locked_files_sha256': {'input': 'a'}, 'locked_files_sha256_after': {'input': 'a'},
                       'source_head': self.commit, 'source_head_after': self.commit,
                       'live_executable_bits': {'main.go': False}, 'live_executable_bits_after': {'main.go': False},
                       'dataset_identity': self.identity, 'errors': []}
        self.rows = []
        group = 0
        for phase in M.PHASES:
            for req in self.requests['requests']:
                for iteration in range(1, self.report['counts'][phase] + 1):
                    for position in range(1, 9):
                        arm = M.ARMS[(group + position - 1) % 8]
                        row = {'sequence': len(self.rows) + 1, 'group': group, 'position': position, 'arm': arm,
                               'phase': phase, 'iteration': iteration, 'request_id': req['id'],
                               'call': {'measurement_id': 'query-' + str(len(self.rows) + 1), 'tool': 'cks.context.get_for_task_v2',
                                        'arguments': dict(req['arguments'], include_knowledge=arm.endswith('_on')),
                                        'elapsed_ns': iteration * 100 if phase == 'warm_latency' else 9000,
                                        'response': {'structuredContent': self.pack(4 if arm == 'baseline_off' else 3)}}}
                        if phase == 'cold_process':
                            row['process_start_through_response_ns'] = 10000
                        self.rows.append(row)
                    group += 1
        self.report['rows'] = len(self.rows)

    def pack(self, line):
        coordinates = {'project_id': 'fixture', 'dataset_id': 'd' * 64, 'snapshot_id': 'b' * 64,
                       'source_mode': 'committed', 'base_commit': self.commit}
        content = self.raw.splitlines(keepends=True)[line - 1]
        citation = dict(coordinates, file='main.go', start_line=line, end_line=line, commit_hash=self.commit,
                        file_sha256=M.sha(self.raw), content_sha256=M.sha(content))
        return sealed({'format_version': 2, 'coordinates': coordinates, 'query': 'Alpha', 'citations': [citation],
                       'bodies': [{'citation': citation, 'text': content.decode()}],
                       'metadata': {'integrity_hash_algo': 'sha256-v2'}})

    def result(self, **kwargs):
        return M.summarize(self.report, self.requests, self.rows, self.cases, self.source, {}, resamples=100, **kwargs)

    def test_repeated_variants_are_one_unit_and_paired_deltas_are_reproducible(self):
        result = self.result()
        self.assertEqual(result['observed_rows'], 112)
        self.assertEqual(result['independent_units'], 1)
        delta = result['paired_deltas_by_group']['code']['combined_on'][M.METRICS[0]]
        self.assertEqual(delta, {'mean': 1, 'descriptive_ci95': [1, 1], 'independent_units': 1, 'inference': 'inconclusive'})
        self.assertEqual(self.result()['paired_deltas_by_group'], result['paired_deltas_by_group'])
        self.assertIsNone(result['quality_metrics'])
        self.assertIsNone(result['human_answer_abstention_policy_reasoning_verdicts'])

    def test_warm_latency_excludes_warmup_retrieval_and_cold(self):
        value = self.result()['arm_summaries']['baseline_off']
        self.assertEqual(value['warm_p50_ns'], 200)
        self.assertEqual(value['warm_p95_ns'], 300)
        self.assertEqual(value['cold_process_p50_ns'], 10000)
        self.assertEqual(value['warm_successful_calls'], 6)
        self.assertEqual(value['warm_p95_ratio_to_baseline_off'], 1)
        self.assertEqual(value['backend_measurement_missing_rows'], 14)

    def test_public_go_dedup_commit_overlap_top10_and_mean_expected_rr(self):
        expected = [{'file': 'main.go', 'start_line': n, 'end_line': n, 'commit_hash': self.commit} for n in [3, 4]]
        a = dict(expected[0]); wrong = dict(a, commit_hash='wrong')
        actual = [wrong, a, a, expected[1]]
        metrics = M.citation_metrics(expected, actual)
        self.assertEqual(metrics[M.METRICS[0]], 1)
        self.assertEqual(metrics[M.METRICS[1]], 2/3)
        self.assertAlmostEqual(metrics[M.METRICS[2]], (1/2 + 1/3)/2)
        self.assertFalse(metrics['v1_no_citation_guard_pass'])
        self.assertEqual(M.citation_metrics([], actual)[M.METRICS[0]], 1)
        self.assertEqual(M.citation_metrics([], [])[M.METRICS[0]], 0)
        self.assertTrue(M.citation_metrics([], [])['v1_no_citation_guard_pass'])
        junk = [dict(a, file='other-' + str(i)) for i in range(10)]
        self.assertEqual(M.citation_metrics([a], junk + [a])[M.METRICS[0]], 0)
        self.assertEqual(M.citation_metrics([a], [dict(a, start_line=2, end_line=4)])[M.METRICS[0]], 1)

    def test_errors_remain_in_retrieval_and_conditional_latency_denominators(self):
        for row in self.rows:
            if row['arm'] == 'combined_on' and row['phase'] in ['retrieval', 'warm_latency'] and row['iteration'] == 1:
                row['call']['transport_error'] = 'injected test failure'
        result = self.result()
        self.assertEqual(result['per_question']['combined_on']['alpha'][M.METRICS[0]], .5)
        arm = result['arm_summaries']['combined_on']
        self.assertEqual(arm['failed_rows'], 4)
        self.assertEqual(arm['warm_latency_denominator'], {'planned': 6, 'observed': 6, 'failed': 2, 'missing': 0})
        self.assertEqual(arm['warm_successful_calls'], 4)
        self.cases['alpha']['expect_no_citations'] = True
        guard = self.result()['arm_summaries']['combined_on']['v1_no_citation_guard']
        self.assertEqual(guard, {'planned': 2, 'observed': 2, 'passed': 0, 'failed_or_missing': 2})

    def test_partial_cannot_become_complete_and_missing_attempts_are_zero(self):
        self.rows = self.rows[:16]
        self.report.update(state='partial', rows=16, errors=['cancelled'])
        result = self.result()
        self.assertEqual(result['status'], 'not_evaluable_as_complete')
        self.assertEqual(len(result['missing_slots']), 96)
        self.assertEqual(result['per_question']['combined_on']['alpha'][M.METRICS[0]], 0)
        self.report['state'] = 'captured'
        with self.assertRaisesRegex(ValueError, 'omitted'):
            self.result()

    def test_rotation_arguments_and_duplicate_ids_are_rejected(self):
        for mutate in ['arm', 'arguments', 'measurement_id', 'sequence_type']:
            with self.subTest(mutate=mutate):
                rows = copy.deepcopy(self.rows)
                if mutate == 'arm': rows[0]['arm'] = 'combined_on'
                elif mutate == 'arguments': rows[0]['call']['arguments']['include_knowledge'] = True
                elif mutate == 'measurement_id': rows[1]['call']['measurement_id'] = rows[0]['call']['measurement_id']
                else: rows[0]['sequence'] = True
                with self.assertRaises(ValueError):
                    M.summarize(self.report, self.requests, rows, self.cases, self.source, {}, resamples=100)

    def test_integrity_identity_query_and_source_failures_are_explicit(self):
        for change in ['integrity', 'identity', 'query', 'body', 'range', 'escape']:
            with self.subTest(change=change):
                rows = copy.deepcopy(self.rows)
                pack = rows[0]['call']['response']['structuredContent']
                if change == 'integrity': pack['metadata']['integrity_hash'] = 'bad'
                elif change == 'identity': pack['coordinates']['dataset_id'] = 'foreign'; sealed(pack)
                elif change == 'query': pack['query'] = 'different'; sealed(pack)
                elif change == 'body': pack['bodies'][0]['text'] = 'wrong'; sealed(pack)
                elif change == 'range': pack['citations'][0]['end_line'] = 50; sealed(pack)
                else: pack['citations'][0]['file'] = '../outside'; sealed(pack)
                result = M.summarize(self.report, self.requests, rows, self.cases, self.source, {}, resamples=100)
                self.assertTrue(result['measurements'][0]['issues'])
                self.assertEqual(result['measurements'][0]['citation_metrics'][M.METRICS[0]], 0)
        (self.source / 'main.go').write_bytes(b'changed source\n' * 4)
        result = self.result()
        self.assertEqual(sum(bool(m['issues']) for m in result['measurements']), 112)

    def test_backend_counts_link_sdk_ids_and_exclude_startup(self):
        folder = Path(self.temp.name)
        footprint = folder / 'footprints/baseline_off/cks-mcp.jsonl'
        footprint.parent.mkdir(parents=True)
        measured = {'measurement_id': self.rows[0]['call']['measurement_id'], 'tool': 'cks.context.get_for_task_v2',
                    'pending_calls': 0, 'calls': [{'ordinal': 1, 'backend': 'ckv', 'outcome': 'returned', 'options': {'K': 10}},
                                                {'ordinal': 2, 'backend': 'ckv', 'outcome': 'returned', 'options': {'K': 6}},
                                                {'ordinal': 3, 'backend': 'ckg', 'outcome': 'error', 'options': {}}]}
        startup = dict(measured, measurement_id='startup-id', tool='startup.intent_anchors')
        footprint.write_text('\n'.join(json.dumps({'event': 'measurement.backend_calls', 'summary': s}) for s in [startup, measured]))
        ledger, digest = M.backend_scopes(folder, 'baseline_off')
        self.assertEqual(list(ledger), [measured['measurement_id']])
        self.assertEqual(digest, M.sha(footprint.read_bytes()))
        result = M.summarize(self.report, self.requests, self.rows, self.cases, self.source, {'baseline_off': ledger}, resamples=100)
        info = result['measurements'][0]['backend_measurement']
        self.assertEqual(info['observed_ckv_k'], [6, 10])
        self.assertEqual(info['attempts_by_backend'], {'ckg': 1, 'ckv': 2})
        self.assertEqual(info['nonreturned_attempts'], 1)
        self.assertIsNone(result['measurements'][1]['backend_measurement'])
        footprint.write_text(footprint.read_text() + '\n' + json.dumps({'event': 'measurement.backend_calls', 'summary': measured}))
        with self.assertRaisesRegex(ValueError, 'duplicate backend'):
            M.backend_scopes(folder, 'baseline_off')

    def test_capture_drift_prevents_complete_classification_and_foreign_gold_refuses(self):
        self.report['binary_sha256_after'] = 'changed'
        self.assertEqual(self.result()['status'], 'not_evaluable_as_complete')
        self.cases['alpha']['expected_commit'] = 'foreign'
        with self.assertRaisesRegex(ValueError, 'gold source commit'):
            self.result()

    def test_nonfinite_and_duplicate_json_are_rejected(self):
        for raw in [b'{"a":1,"a":2}', b'{"a":NaN}']:
            with self.assertRaises(ValueError): M.decode(raw)

    def test_synthetic_bound_static_partition_model_counts_and_scope_path(self):
        raw = [(M.DATA / n).read_bytes() for n in ['questions.json', 'protocol-m2max-draft.json',
               'dynamic-fixtures-m2max-draft.json', 'human-review-m2max-2026-10-03.json', 'static-v2-scopes-m2max-draft.json']]
        fixtures = json.loads(raw[2]); fixtures['status'] = 'approved'
        for f in fixtures['fixtures']:
            f.update(review_state='approved', reviewer='synthetic-test-only', reviewed_at='2026-10-04T00:00:00Z')
        raw[2] = json.dumps(fixtures).encode()
        protocol = json.loads(raw[1]); protocol.update(status='approved', reviewer='synthetic-test-only',
            reviewed_at='2026-10-04T00:00:00Z', fixture_manifest_sha256=M.sha(raw[2]))
        raw[1] = json.dumps(protocol).encode()
        scope = json.loads(raw[4]); scope.update(status='approved', reviewer='synthetic-test-only',
            reviewed_at='2026-10-04T00:00:00Z', protocol_sha256=M.sha(raw[1]))
        raw[4] = json.dumps(scope).encode()
        review = json.loads(raw[3]); review['protocol_and_dynamic_fixture_decision'] = {
            'status': 'approved', 'reviewer': 'synthetic-test-only', 'reviewed_at': '2026-10-04T00:00:00Z',
            'protocol_sha256_after': M.sha(raw[1]), 'fixture_manifest_sha256_after': M.sha(raw[2])}
        review['decisions'].append({'scope': 'static_v2_query_scopes', 'status': 'approved',
            'reviewer': 'synthetic-test-only', 'reviewed_at': '2026-10-04T00:00:00Z',
            'scope_sha256_after': M.sha(raw[4])})
        raw[3] = json.dumps(review).encode()
        book = json.loads(raw[0]); scope_map = {q['id']: q for q in scope['question_scopes']}
        report = copy.deepcopy(self.report)
        report['dataset_identity']['source'].update(project_id=book['corpus_project'], source_commit=book['corpus_commit'])
        report['dataset_identity']['embedding_identity'] = {
            'Provider': protocol['model']['provider'], 'Model': protocol['model']['model'],
            'model_digest': protocol['model']['digest'], 'Dim': protocol['model']['dimension']}
        report['counts'] = {'retrieval': 5, 'warmup': 2, 'warm_latency': 20, 'cold_process': 3}
        for partition, n in [('development', 4), ('final', 8)]:
            selected = [q for q in book['questions'] if q['id'] in protocol[partition + '_questions']]
            requests = {'requests': [{'id': q['id'], 'arguments': {'prompt': q['prompt'],
                'knowledge_as_of': scope_map[q['id']]['knowledge_as_of'],
                'knowledge_subsystem': scope_map[q['id']]['knowledge_subsystem']}} for q in selected]}
            cases, definition, declared = M.static_cases(*raw, partition, scope['commit_time_evidence'], report, requests)
            self.assertEqual(len(cases), n)
            self.assertFalse(definition['official_execution_ready'])
            self.assertFalse(declared['v2_matrix_ready'])
            self.assertEqual(set(cases), set(protocol[partition + '_questions']))
            for failure in ['model', 'count', 'scope']:
                bad_report, bad_requests = copy.deepcopy(report), copy.deepcopy(requests)
                if failure == 'model': bad_report['dataset_identity']['embedding_identity']['model_digest'] = 'foreign'
                elif failure == 'count': bad_report['counts']['retrieval'] = 1
                else: bad_requests['requests'][0]['arguments']['knowledge_subsystem'] = 'wrong'
                with self.assertRaises(ValueError):
                    M.static_cases(*raw, partition, scope['commit_time_evidence'], bad_report, bad_requests)

    def captured_files(self):
        directory = Path(self.temp.name) / 'capture'; directory.mkdir()
        for arm in self.report['arms']:
            raw = ('semantic:\n    ontology_mode: ' + arm['id'].rsplit('_', 1)[0] + '\nretrieval:\n    recall_k: 10\n').encode()
            (directory / (arm['id'] + '.yaml')).write_bytes(raw)
            arm.update(config_sha256=M.sha(raw), config_sha256_after=M.sha(raw))
        rows = b'\n'.join(json.dumps(r, ensure_ascii=False).encode() for r in self.rows) + b'\n'
        requests = json.dumps(self.requests).encode()
        self.report.update(rows_sha256=M.sha(rows), request_sha256=M.sha(requests))
        (directory / 'rows.jsonl').write_bytes(rows)
        (directory / 'requests.json').write_bytes(requests)
        (directory / 'report.json').write_text(json.dumps(self.report))
        gold = directory / 'gold.json'
        gold.write_text(json.dumps({'schema_version': 1, 'diagnostic_only': True, 'cases': list(self.cases.values())}))
        return directory, gold

    def test_cli_replay_is_read_only_exclusive_and_rejects_tampered_rows(self):
        directory, gold = self.captured_files()
        unrelated = self.source / '.git'; unrelated.mkdir(); (unrelated / 'private-control').write_text('not evidence')
        output = directory / 'summary.json'
        command = [sys.executable, M.__file__, '--capture-dir', str(directory), '--gold', str(gold),
                   '--source-root', str(self.source), '--diagnostic-controls', '--output', str(output)]
        before = {str(p): M.sha(p.read_bytes()) for p in directory.iterdir()}
        result = subprocess.run(command, capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        summary = json.loads(output.read_bytes())
        self.assertEqual(summary['referenced_source_files_sha256'], {'main.go': M.sha(self.raw)})
        self.assertEqual(output.stat().st_mode & 0o777, 0o600)
        self.assertTrue(all(M.sha(Path(p).read_bytes()) == digest for p, digest in before.items()))
        original = output.read_bytes()
        result = subprocess.run(command, capture_output=True, text=True)
        self.assertEqual(result.returncode, 1)
        self.assertEqual(output.read_bytes(), original)
        (directory / 'rows.jsonl').write_bytes((directory / 'rows.jsonl').read_bytes() + b'\n')
        output.unlink()
        result = subprocess.run(command, capture_output=True, text=True)
        self.assertEqual(result.returncode, 1)
        self.assertIn('hashes differ', result.stderr)
        self.assertFalse(output.exists())

    def test_cli_pending_static_review_creates_no_final_report(self):
        directory, _ = self.captured_files()
        output = directory / 'final.json'
        result = subprocess.run([sys.executable, M.__file__, '--capture-dir', str(directory),
            '--gold', str(M.DATA / 'questions.json'), '--source-root', str(self.source),
            '--partition', 'final', '--output', str(output)], capture_output=True, text=True)
        self.assertEqual(result.returncode, 2, result.stderr)
        self.assertIn('protocol_review_pending', result.stderr)
        self.assertFalse(output.exists())

    def test_retained_configuration_requires_hash_mode_and_reviewed_k(self):
        directory, _ = self.captured_files()
        self.assertEqual(len(M.verify_retained_configs(directory, self.report, 10)), 8)
        file = directory / 'baseline_off.yaml'
        raw = file.read_bytes().replace(b'recall_k: 10', b'recall_k: 20'); file.write_bytes(raw)
        with self.assertRaisesRegex(ValueError, 'configuration hash'):
            M.verify_retained_configs(directory, self.report, 10)
        self.report['arms'][0]['config_sha256'] = M.sha(raw)
        with self.assertRaisesRegex(ValueError, 'runtime K'):
            M.verify_retained_configs(directory, self.report, 10)
        raw = raw.replace(b'ontology_mode: baseline', b'ontology_mode: combined'); file.write_bytes(raw)
        self.report['arms'][0]['config_sha256'] = M.sha(raw)
        with self.assertRaisesRegex(ValueError, 'ontology mode'):
            M.verify_retained_configs(directory, self.report)

    def test_initialize_failure_can_be_reported_without_fabricated_sdk_call(self):
        self.rows[0]['error'] = 'initialize failed'; self.rows[0]['call'] = {}
        result = self.result()
        self.assertEqual(result['arm_summaries']['baseline_off']['warmup_denominator']['failed'], 1)
        self.assertTrue(result['measurements'][0]['issues'])

    def test_real_static_review_pending_prevents_scoring(self):
        raw = [(M.DATA / n).read_bytes() for n in ['questions.json', 'protocol-m2max-draft.json',
               'dynamic-fixtures-m2max-draft.json', 'human-review-m2max-2026-10-03.json', 'static-v2-scopes-m2max-draft.json']]
        with self.assertRaisesRegex(M.INPUTS.PendingInputs, 'reviewed static'):
            M.static_cases(*raw, 'final', json.loads(raw[-1])['commit_time_evidence'], self.report, self.requests)


if __name__ == '__main__':
    unittest.main()
