#!/usr/bin/env python3
"""Check B0 protocol input boundaries; export development-only diagnostic scenarios."""
import argparse
import datetime as dt
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile


ROOT = Path(__file__).resolve().parent.parent
DATA = ROOT / 'system/eval/b0-knowledge-system'
THRESHOLDS = {'safety_snapshot_secret_mixing_max': 0,
              'abstention_false_citation_delta_max': 0,
              'recall_at_10_delta_min': -0.02, 'mrr_delta_min': -0.02,
              'important_group_delta_min': -0.05, 'warm_p95_ratio_max': 1.25}


def module(name, file):
    spec = importlib.util.spec_from_file_location(name, Path(__file__).with_name(file))
    value = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(value)
    return value


EXPORT = module('static_export', 'b0-export-scenarios.py')
MATERIALIZE = module('static_fixtures', 'b0-materialize-fixtures.py')


class PendingInputs(ValueError):
    pass


def digest(raw):
    return hashlib.sha256(raw).hexdigest()


def decode(raw):
    def unique(pairs):
        result = {}
        for key, value in pairs:
            if key in result:
                raise ValueError('duplicate JSON key: ' + key)
            result[key] = value
        return result
    value = json.loads(raw, object_pairs_hook=unique)
    if not isinstance(value, dict):
        raise ValueError('input must be a JSON object')
    return value


def count(value, label, minimum=1):
    if type(value) is not int or value < minimum:
        raise ValueError(label + ' must be an integer >= ' + str(minimum))
    return value


def timestamp(value):
    if not isinstance(value, str):
        return False
    try:
        return dt.datetime.fromisoformat(value.replace('Z', '+00:00')).utcoffset() is not None
    except ValueError:
        return False


def reviewed(value):
    return (isinstance(value.get('reviewer'), str) and bool(value['reviewer'].strip())
            and timestamp(value.get('reviewed_at')))


def identifiers(values, label):
    if not isinstance(values, list) or any(not isinstance(v, str) for v in values) or len(set(values)) != len(values):
        raise ValueError(label + ' must be a list of unique string IDs')
    return set(values)


def inspect(questions_raw, protocol_raw, fixtures_raw, review_raw, partition='development'):
    if partition not in {'development', 'final'}:
        raise ValueError('unknown partition')
    book, protocol, fixtures, review = map(decode, (questions_raw, protocol_raw, fixtures_raw, review_raw))
    if not isinstance(book.get('questions'), list) or any(not isinstance(q, dict) for q in book['questions']):
        raise ValueError('static question inventory missing')
    checked = EXPORT.PREFLIGHT.validate_questions(questions_raw)
    if protocol.get('schema_version') != 1 or fixtures.get('schema_version') != 1 or review.get('schema_version') != 1:
        raise ValueError('unsupported protocol/fixture/review schema')
    ids = {q['id'] for q in book['questions']}
    if any(not re.fullmatch(r'B0-[A-Z]+-[0-9]{2}', qid) for qid in ids):
        raise ValueError('static B0 IDs must be safe filenames')
    if protocol.get('question_set_sha256') != digest(questions_raw) or protocol.get('fixture_manifest_sha256') != digest(fixtures_raw):
        raise ValueError('protocol input hash differs from supplied question/fixture bytes')
    if protocol.get('corpus_commit') != checked['corpus_commit'] or protocol.get('corpus_tree') != checked['corpus_tree'] or fixtures.get('corpus_commit') != checked['corpus_commit']:
        raise ValueError('corpus commit/tree mismatch')
    if any(q.get('review_state') == 'approved' and not reviewed(q) for q in book['questions']):
        raise ValueError('approved static gold needs a reviewer and timezone-aware review time')
    development = identifiers(protocol.get('development_questions'), 'development_questions')
    final = identifiers(protocol.get('final_questions'), 'final_questions')
    if development & final or development | final != ids or len(development) != 4 or len(final) != 8:
        raise ValueError('static partition must be disjoint development 4/final 8 and cover every question')
    selected_fixtures = MATERIALIZE.validate(fixtures, 'all', allow_draft=True)
    if any(f.get('review_state') not in {'draft', 'approved', 'rejected'} for f in selected_fixtures):
        raise ValueError('unknown dynamic fixture review status')
    all_fixture_ids = {f'F-0{family}-{suffix}' for family in range(1, 7) for suffix in ('DEV', 'FINAL')}
    if {f['id'] for f in selected_fixtures} != all_fixture_ids:
        raise ValueError('dynamic input must contain all six development/final families')
    for part in ('development', 'final'):
        actual = {f['id'] for f in selected_fixtures if f['evaluation_partition'] == part}
        if identifiers(protocol.get('synthetic_' + part + '_fixtures'), part + '_fixtures') != actual:
            raise ValueError('protocol dynamic partition differs from manifest')
    if protocol.get('paired_arms') != ['baseline', 'concept_text', 'relations', 'combined'] or protocol.get('pack_axis') != ['off', 'on']:
        raise ValueError('paired experiment requires the fixed four modes and off/on pack axis')
    if protocol.get('thresholds') != THRESHOLDS or any(type(v) not in (int, float) for v in protocol['thresholds'].values()):
        raise ValueError('regression thresholds differ from the design contract')
    if protocol.get('results') is not None or fixtures.get('result_metrics') is not None:
        raise ValueError('input proposal contains result metrics; pre-result review required')
    if protocol.get('query_prefix_policy') != 'registry' or protocol.get('static_filters') != {}:
        raise ValueError('this static protocol requires registry query prefix and empty static filters')
    k = count(protocol.get('retrieval_k'), 'retrieval_k')
    if k != 10 or count(protocol.get('synthetic_f01_k'), 'synthetic_f01_k') != 5:
        raise ValueError('Recall@10 and F-01 K5 contracts must be preserved')
    latency, cold, uncertainty = (protocol.get(key) for key in ('warm_latency', 'cold_start', 'uncertainty'))
    if any(not isinstance(value, dict) for value in (latency, cold, uncertainty)):
        raise ValueError('latency/cold/uncertainty contracts are missing')
    runs = {'retrieval': count(protocol.get('retrieval_runs'), 'retrieval_runs'),
            'warmup': count(latency.get('warmup_runs'), 'warmup_runs', 0),
            'warm_latency': count(latency.get('measured_runs'), 'warm measured_runs'),
            'cold_process': count(cold.get('runs'), 'cold runs')}
    if latency.get('percentile_method') != 'nearest_rank' or not cold.get('definition'):
        raise ValueError('warm percentile or separate cold definition missing')
    count(uncertainty.get('bootstrap_seed'), 'bootstrap_seed', 0)
    count(uncertainty.get('resamples'), 'resamples')
    if type(uncertainty.get('ci')) not in (int, float) or not 0 < uncertainty['ci'] < 1 or not uncertainty.get('resampling_unit') or not uncertainty.get('small_group_policy'):
        raise ValueError('independent sample/uncertainty contract missing')
    model = protocol.get('model')
    if not isinstance(model, dict) or model.get('provider') != 'ollama' or not isinstance(model.get('model'), str) or not isinstance(model.get('digest'), str) or not EXPORT.PREFLIGHT.SHA64.fullmatch(model['digest']):
        raise ValueError('exact protocol model identity missing')
    count(model.get('dimension'), 'model dimension')
    if model.get('model') != fixtures.get('model_candidate') or model.get('runtime_options') != EXPORT.PREFLIGHT.ollama_runtime_options(model['model']):
        raise ValueError('fixture model or runtime options differ from protocol')
    decisions = review.get('decisions')
    if not isinstance(decisions, list) or any(not isinstance(v, dict) for v in decisions):
        raise ValueError('human decision inventory missing')
    model_decisions = [d for d in decisions if d.get('scope') == 'embedding_model']
    if len(model_decisions) != 1 or any(model_decisions[0].get(field) != model.get(field) for field in ('model', 'digest', 'dimension')):
        raise ValueError('protocol model differs from recorded human model selection')
    gold_decisions = [d for d in decisions if d.get('scope') == 'all_12_questions_candidates_citation_spans_and_abstention_review']
    if len(gold_decisions) != 1 or gold_decisions[0].get('question_set_sha256_after') != digest(questions_raw) or identifiers(gold_decisions[0].get('question_ids'), 'review question IDs') != ids:
        raise ValueError('question bytes differ from recorded human gold review')
    if not isinstance(review.get('reviewer'), str) or not review['reviewer'].strip() or not timestamp(review.get('recorded_at')):
        raise ValueError('human review identifier/time missing')
    reasons = []
    if checked['approved_count'] != checked['question_count']:
        reasons.append('static_gold_review_pending')
    if protocol.get('status') not in {'draft', 'approved', 'rejected'}:
        raise ValueError('unknown protocol review status')
    if protocol.get('status') != 'approved' or not reviewed(protocol):
        reasons.append('protocol_review_pending')
    approved = sum(f.get('review_state') == 'approved' and reviewed(f) for f in selected_fixtures)
    if fixtures.get('status') not in {'proposal_pending_human_review', 'approved', 'rejected'}:
        raise ValueError('unknown dynamic manifest review status')
    if approved != len(selected_fixtures) or fixtures.get('status') != 'approved':
        reasons.append('dynamic_fixture_review_pending')
    selected = development if partition == 'development' else final
    counts = {group: sum(q['group'] == group and q['id'] in final for q in book['questions'])
              for group in sorted(EXPORT.PREFLIGHT.GROUPS)}
    result = {'schema_version': 1, 'gate': 'B0-protocol-input-definition',
              'status': 'pending' if reasons else 'approved_input_definition',
              'scope': 'static/dynamic input bytes and partition declarations; not corpus scope, semantic-fact, runtime, quality or release readiness',
              'partition': partition, 'diagnostic_only': bool(reasons),
              'official_execution_ready': False, 'v2_matrix_ready': False,
              'question_set_sha256': digest(questions_raw), 'protocol_sha256': digest(protocol_raw),
              'fixture_manifest_sha256': digest(fixtures_raw), 'human_review_sha256': digest(review_raw),
              'corpus_commit': checked['corpus_commit'], 'corpus_tree': checked['corpus_tree'],
              'static_approved_count': checked['approved_count'], 'dynamic_approved_count': approved,
              'selected_ids': [q['id'] for q in book['questions'] if q['id'] in selected],
              'withheld_ids': [q['id'] for q in book['questions'] if q['id'] not in selected],
              'retrieval_k': k, 'runs': runs, 'model': model, 'pending_reasons': reasons,
              'final_independent_questions_by_group': counts,
              'subgroup_release_inference': 'inconclusive: every final group has fewer than 10 independent questions',
              'metrics': None,
              'remaining_execution_prerequisites': ['corpus source-coverage decision and fresh strict build audit',
                  'source-bound semantic fact/supplement review and locked pack/semantic identities',
                  'v2 knowledge date/subsystem request scopes and runtime config K/filter binding',
                  'actual environment/model/binary/input preflight and independent final execution']}
    return result, [q for q in book['questions'] if q['id'] in selected]


def export(result, questions, output):
    if result['partition'] == 'final' and result['pending_reasons']:
        raise PendingInputs('final input export requires protocol and dynamic approval; no final prompt was exported')
    if any(q['review_state'] != 'approved' for q in questions):
        raise PendingInputs('selected static questions need human approval')
    if output.exists() or output.is_symlink():
        raise ValueError('output already exists; prepared inputs are immutable')
    output.parent.mkdir(parents=True, exist_ok=True)
    output.mkdir(mode=0o700)  # exclusive ownership; manifest is the publication marker
    temp = Path(tempfile.mkdtemp(prefix='.staging-', dir=output))
    try:
        for question in questions:
            value = EXPORT.scenario(question, result['corpus_commit'], result['runs']['retrieval'])
            file = temp / (question['id'].lower() + '.yaml')
            file.write_text(json.dumps(value, ensure_ascii=False, indent=2) + '\n', encoding='utf-8')
            file.chmod(0o600)
        manifest = dict(result, scenario_count=len(questions),
                        scenario_sha256={p.name: digest(p.read_bytes()) for p in sorted(temp.iterdir())},
                        v1_oracle_note='expect_no_citations is independent of a human answer-abstention verdict',
                        v2_requests_exported=False)
        file = temp / 'manifest.json'
        file.write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + '\n', encoding='utf-8')
        file.chmod(0o600)
        for child in sorted(temp.iterdir(), key=lambda p: p.name == 'manifest.json'):
            os.rename(child, output / child.name)
        temp.rmdir()
    except BaseException:
        shutil.rmtree(output)
        raise


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--questions', type=Path, default=DATA / 'questions.json')
    parser.add_argument('--protocol', type=Path, default=DATA / 'protocol-m2max-draft.json')
    parser.add_argument('--fixtures', type=Path, default=MATERIALIZE.DEFAULT)
    parser.add_argument('--human-review', type=Path, default=DATA / 'human-review-m2max-2026-10-03.json')
    parser.add_argument('--partition', choices=['development', 'final'], default='development')
    parser.add_argument('--out-dir', type=Path)
    parser.add_argument('--output', type=Path, help='readiness report; not a measurement result')
    parser.add_argument('--require-ready', action='store_true', help='exit 2 while input-definition approvals are pending')
    args = parser.parse_args()
    result, selected = inspect(*(p.read_bytes() for p in (args.questions, args.protocol, args.fixtures, args.human_review)), args.partition)
    rendered = json.dumps(result, ensure_ascii=False, indent=2) + '\n'
    if args.output:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        # Readiness output must never overwrite gold/protocol/review inputs.
        fd = os.open(args.output, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
        with os.fdopen(fd, 'w', encoding='utf-8') as file:
            file.write(rendered)
    else:
        print(rendered, end='')
    if args.require_ready and result['pending_reasons']:
        return 2
    if args.out_dir:
        export(result, selected, args.out_dir)
    return 0


if __name__ == '__main__':
    try:
        sys.exit(main())
    except PendingInputs as exc:
        print('b0-prepare-static-inputs: ' + str(exc), file=sys.stderr)
        sys.exit(2)
    except (OSError, ValueError, subprocess.CalledProcessError) as exc:
        print('b0-prepare-static-inputs: ' + str(exc), file=sys.stderr)
        sys.exit(1)
