#!/usr/bin/env python3
"""Prepare exposed rule review inputs; never run analyzed source or label gold."""
import argparse
from collections import Counter
import importlib.util
import json
from pathlib import Path
import re
import shutil
import subprocess
from functools import lru_cache

from source_packet import sha, safe

ROOT = Path(__file__).resolve().parents[1]
BASE = Path('docs/spec-driven/operational-preparation/source-review-supplement')
PRIOR = Path('docs/spec-driven/operational-preparation/policy-security-drafts')
ENGINE = Path('pkg/graph/valueflow/rules.go')
TESTS = Path('pkg/graph/valueflow/rules_test.go')
KINDS = ['parameter-command-name', 'discarded-conversion-error']


def load(name):
    spec = importlib.util.spec_from_file_location(name.replace('-', '_'), ROOT / 'scripts' / name)
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


def dump(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2, allow_nan=False) + '\n')


def bindings():
    return [ENGINE, TESTS, Path(__file__).relative_to(ROOT), PRIOR / 'manifest.json',
            PRIOR / 'private/rule-source-census.json', PRIOR / 'public/inputs.json']


def drafts():
    # Expectations are AI proposals about candidate matching, not risk/safety labels.
    command = [
        ('copy_chain', 1, ['flow'], 'func F(cmd string){a:=cmd;b:=a;exec.Command(b)}'),
        ('conditional_copy', 1, ['branch'], 'func F(cmd string,ok bool){name:="fixed";if ok{name=cmd}else{name="other"};exec.Command(name)}'),
        ('loop_copy', 1, ['loop'], 'func F(cmd string,n int){name:="fixed";for i:=0;i<n;i++{name=cmd};exec.Command(name)}'),
        ('inner_shadow_source', 1, ['shadow'], 'func F(cmd string){name:="fixed";{name:=cmd;exec.Command(name)};_=name}'),
        ('opaque_sanitizer_copy', 1, ['sanitizer'], 'func F(cmd string){a:=sanitize(cmd);b:=a;exec.Command(b)}'),
        ('context_copy', 1, ['external_target'], 'func F(cmd string){a:=cmd;exec.CommandContext(nil,a)}'),
        ('spoofed_receiver_copy', 1, ['external_target'], 'func F(cmd string,exec Fake){a:=cmd;exec.Command(a)}'),
        ('guarded_sink', 1, ['branch'], 'func F(cmd string,ok bool){if ok{exec.Command(cmd)}}'),
        ('compound_copy', 1, ['flow'], 'func F(cmd string){a:=cmd;a+="suffix";exec.Command(a)}'),
        ('early_exit_guard', 1, ['branch'], 'func F(cmd string,ok bool){if !ok{return};exec.Command(cmd)}'),
        ('kill_after_copy', 0, ['flow'], 'func F(cmd string){a:=cmd;b:=a;b="fixed";exec.Command(b)}'),
        ('all_branch_kill', 0, ['branch'], 'func F(cmd string,ok bool){a:=cmd;if ok{a="one"}else{a="two"};exec.Command(a)}'),
        ('inner_fixed_shadow', 0, ['shadow'], 'func F(cmd string){name:=cmd;{name:="fixed";exec.Command(name)};_=name}'),
        ('unrelated_sanitizer', 0, ['sanitizer'], 'func F(cmd string){_=sanitize(cmd);name:="fixed";exec.Command(name)}'),
        ('renamed_external_copy', 0, ['external_target'], 'func F(cmd string){a:=cmd;process.Command(a)}'),
        ('indirect_selector_sink', 0, ['unsupported'], 'func F(cmd string){box:=Box{cmd};exec.Command(box.Name)}'),
        ('sink_expression', 0, ['unsupported'], 'func F(cmd string){exec.Command(cmd+"suffix")}'),
        ('context_wrong_argument', 0, ['external_target'], 'func F(cmd string){exec.CommandContext(nil,"fixed",cmd)}'),
        ('parallel_rhs_fixed', 0, ['flow'], 'func F(cmd string){a,b:="fixed",cmd;_=b;exec.Command(a)}'),
        ('closure_only_sink', 0, ['unsupported'], 'func F(cmd string){a:=cmd;cb:=func(){exec.Command(a)};_=cb}'),
    ]
    conversion = [
        ('unsigned_direct', 1, ['external_target'], 'func F(s string)uint64{v,_:=strconv.ParseUint(s,16,32);return v}'),
        ('copy_chain', 1, ['flow'], 'func F(s string)int{v,_:=strconv.Atoi(s);a:=v;b:=a;return b}'),
        ('branch_copy', 1, ['branch'], 'func F(s string,ok bool)int{v,_:=strconv.Atoi(s);out:=0;if ok{out=v};return out}'),
        ('loop_copy', 1, ['loop'], 'func F(s string,n int)int{v,_:=strconv.Atoi(s);out:=0;for i:=0;i<n;i++{out=v};return out}'),
        ('inner_shadow_source', 1, ['shadow'], 'func F(s string,ok bool)int{v:=0;if ok{v,_:=strconv.Atoi(s);return v};return v}'),
        ('opaque_sanitizer', 1, ['sanitizer'], 'func F(s string)int{v,_:=strconv.Atoi(s);out:=clamp(v);return out}'),
        ('spoofed_receiver', 1, ['external_target'], 'func F(s string,strconv Fake)int{v,_:=strconv.Atoi(s);return v}'),
        ('compound_result', 1, ['flow'], 'func F(s string)int{v,_:=strconv.Atoi(s);v+=2;return v}'),
        ('guarded_return', 1, ['branch'], 'func F(s string,ok bool)int{v,_:=strconv.Atoi(s);if !ok{return 0};return v}'),
        ('signed_copy', 1, ['external_target'], 'func F(s string)int64{v,_:=strconv.ParseInt(s,16,32);copy:=v;return copy}'),
        ('named_error_discarded_later', 0, ['flow'], 'func F(s string)int{v,err:=strconv.Atoi(s);_=err;return v}'),
        ('kill_copy', 0, ['flow'], 'func F(s string)int{v,_:=strconv.Atoi(s);out:=v;out=4;return out}'),
        ('all_branch_kill', 0, ['branch'], 'func F(s string,ok bool)int{v,_:=strconv.Atoi(s);if ok{v=1}else{v=2};return v}'),
        ('inner_fixed_shadow', 0, ['shadow'], 'func F(s string,ok bool)int{v,_:=strconv.Atoi(s);if ok{v:=4;return v};return 0}'),
        ('unrelated_sanitizer', 0, ['sanitizer'], 'func F(s string)int{v,_:=strconv.Atoi(s);_=clamp(v);out:=1;return out}'),
        ('renamed_external_copy', 0, ['external_target'], 'func F(s string)int{v,_:=conversion.Atoi(s);out:=v;return out}'),
        ('return_expression', 0, ['unsupported'], 'func F(s string)int{v,_:=strconv.Atoi(s);return v+1}'),
        ('bare_return', 0, ['unsupported'], 'func F(s string)(out int){v,_:=strconv.Atoi(s);out=v;return}'),
        ('conversion_in_closure', 0, ['unsupported'], 'func F(s string)int{cb:=func()int{v,_:=strconv.Atoi(s);return v};_=cb;return 0}'),
        ('assigned_error_variable', 0, ['flow'], 'func F(s string)int{var v int;var err error;v,err=strconv.Atoi(s);_=err;return v}'),
    ]
    public, private = [], []
    for rindex, (rule, rows) in enumerate(zip(KINDS, [command, conversion]), 1):
        for index, (mechanism, count, tags, code) in enumerate(rows, 1):
            cid = f'RULE-{rindex}-{index:02d}'
            public.append({'id': cid, 'code': code, 'source_sha256': sha(code.encode()),
                           'split': None, 'scoring_allowed': False})
            private.append({'id': cid, 'rule_proposal': rule, 'mechanism': mechanism, 'coverage_tags': tags,
                            'candidate_count_proposal': count, 'proposal_label': 'positive' if count else 'negative',
                            'family_independence': 'not_reviewed', 'prior_exposure': 'development_draft_not_fresh_holdout',
                            'actual_detection': None, 'safety_verdict': 'unknown', 'gold': False, 'approved': False,
                            'reviewer': None, 'split': None})
    return public, private


def contract():
    source = (ROOT / ENGINE).read_text()
    definition = re.search(r'const rulePackDefinition = `([^`]+)`', source).group(1)
    return {'pack_definition': json.loads(definition), 'pack_definition_sha256': sha(definition.encode()),
            'kind': 'implementation_contract_review_not_semantic_gold', 'engine_version': '2',
            'target_resolution': 'syntax_only_unverified', 'sanitizer': 'not_proven',
            'command': {'owner_spelling': 'exec', 'sink_arguments': {'Command': 0, 'CommandContext': 1},
                        'sink_shape': 'direct_identifier', 'source': 'named_scalar_parameter_entry_definition'},
            'conversion': {'owner_spelling': 'strconv', 'callees': ['Atoi', 'ParseInt', 'ParseUint'],
                           'source_assignment': 'two_lhs_one_rhs_blank_second_lhs', 'sink_shape': 'explicit_return_direct_identifier'},
            'flow': 'lexical_may_reach_corresponding_rhs_uses_not_path_feasibility',
            'uncertainties': ['import_alias_and_callee_identity_unresolved', 'spoofed_owner_may_match',
                              'opaque_sanitizer_effects_not_proven', 'parameters_not_proven_attacker_controlled',
                              'zero_candidates_never_establish_safety', 'closure_memory_alias_indirect_sinks_unsupported',
                              'structural_range_condition_may_be_unlocated'],
            'manual_annotations': 'separate_from_automatic_candidate_detection', 'vulnerability_verdict': None}


def old_cases():
    source = (ROOT / TESTS).read_text()
    output = []
    quoted = r'"(?:\\.|[^"\\])*"'
    for fname, rule in [('TestRulePackScalarCommandCorpus', KINDS[0]), ('TestRulePackDiscardedErrorCorpus', KINDS[1])]:
        section = source.split('func ' + fname + '(', 1)[1].split('\nfunc ', 1)[0]
        for match in re.finditer(r'^\s*\{(' + quoted + r'), (' + quoted + r'), ([01])\},$', section, re.M):
            name, code, count = match.groups()
            output.append({'test': fname, 'name': json.loads(name), 'code': json.loads(code),
                           'expected_candidates_in_existing_test': int(count), 'rule': rule,
                           'status': 'existing_developer_fixture_not_new_independent_gold'})
    if len(output) != 23:
        raise ValueError('existing rule fixture inventory changed; review contract')
    return output


def project_review():
    census = json.loads((ROOT / PRIOR / 'private/rule-source-census.json').read_text())
    rows = []
    for call in census['calls']:
        raw = safe(ROOT / PRIOR / 'private', call['source_excerpt_private']).read_bytes()
        a, b = call['start_byte'] - call['function_start_byte'], call['end_byte_exclusive'] - call['function_start_byte']
        if sha(raw) != call['function_sha256'] or sha(raw[a:b]) != call['span_sha256']:
            raise ValueError('project call source binding mismatch')
        command = call['import_path_spelling'] == 'os/exec'
        direct = command and call['arguments'][0]['kind'] == 'identifier' and call['arguments'][0]['text'] in call['named_parameters']
        blank = not command and len(call.get('assignment_lhs') or []) == 2 and call['assignment_lhs'][1] == '_'
        rows.append({'id': call['id'], 'rule_proposal': KINDS[0] if command else KINDS[1],
                     'source': call, 'raw_call_text': raw[a:b].decode(),
                     'direct_parameter_spelling_hint': bool(direct), 'blank_second_lhs_spelling_hint': bool(blank),
                     'interpretation': 'syntax_screen_only_not_detected_positive_or_confirmed_negative',
                     'actual_detection': None, 'gold': False, 'approved': False, 'safety_verdict': 'unknown',
                     'independent_family': None})
    return {'kind': 'existing_project_source_screen_not_rule_execution', 'source_commit': census['source_commit'],
            'source_functions': len({(r['path'], r['function_start_byte']) for r in census['calls']}), 'calls': rows}


@lru_cache(maxsize=2)
def parse_sources(payload):
    gofmt = shutil.which('gofmt')
    if not gofmt:
        raise ValueError('installed gofmt required for parse-only checks')
    output = []
    for row in json.loads(payload):
        p = subprocess.run([gofmt, '-e'], input=('package fixture\n' + row['code'] + '\n').encode(),
                           capture_output=True, timeout=5)
        if p.returncode:
            raise ValueError('draft syntax rejected: ' + row['id'] + ': ' + p.stderr.decode())
        output.append({'id': row['id'], 'source_sha256': row['source_sha256'], 'exit_code': 0,
                       'formatted_source_sha256': sha(p.stdout), 'diagnostics': p.stderr.decode()})
    return {'kind': 'isolated_syntax_parse_only_no_typecheck_or_analyzer_execution', 'records': output,
            'analyzed_source_executed': False, 'rule_backend_calls': 0, 'model_calls': 0, 'nodes_started': 0}


def documents():
    public, private = drafts()
    project = project_review()
    old = old_cases()
    by_id = {r['id']: r for r in public}
    for row in private:
        code = by_id[row['id']]['code']
        row['exact_prior_fixture_matches'] = [r['name'] for r in old if r['code'] == code]
        row['semantic_overlap_with_prior'] = 'not_reviewed_even_when_text_differs'
    summary = {'kind': 'rule_coverage_preparation_not_detection_quality',
               'counts': {'supported_rules': 2, 'controlled_source_drafts': 40, 'existing_developer_cases': len(old),
                          'project_call_records': len(project['calls']), 'project_source_functions': project['source_functions'],
                          'annotation_queries_separate': 12,
                          'direct_parameter_spelling_hints': sum(r['direct_parameter_spelling_hint'] for r in project['calls']),
                          'blank_second_lhs_spelling_hints': sum(r['blank_second_lhs_spelling_hint'] for r in project['calls']),
                          'approved_independent_cases': 0, 'independent_gold_records': 0},
               'proposed_controlled_matrix': {rule: dict(Counter(r['proposal_label'] for r in private if r['rule_proposal'] == rule)) for rule in KINDS},
               'project_call_denominators': dict(Counter(r['rule_proposal'] for r in project['calls'])),
               'denominators_are_separate': True, 'precision': None, 'recall': None, 'confusion_matrix': None,
               'formal_runs': 0, 'rule_backend_calls': 0, 'model_calls': 0, 'nodes_started': 0,
               'limits': ['forty correlated development proposals are not independent cases or fresh holdout',
                          'syntax screen absence is not a negative label or proof of no project positives',
                          'twelve annotation queries cannot fill per-rule positive/negative denominators',
                          'candidate expectations require actual bounded analyzer observation and independent review',
                          'project positive sufficiency remains unknown; controlled inputs cannot repair that denominator']}
    return {'public-inputs.json': {'kind': 'exposed_unapproved_controlled_sources_no_labels', 'cases': public},
            'private-ai-review.json': {'kind': 'private_unapproved_candidate_match_proposals_not_safety', 'cases': private},
            'contract.json': contract(), 'existing-fixtures.json': old, 'project-source-review.json': project,
            'coverage-summary.json': summary}


def prepare(output):
    if output.exists():
        raise ValueError('fresh output required')
    errors = load('check-policy-security-preparation.py').check(ROOT / PRIOR, ROOT)
    if errors:
        raise ValueError('; '.join(errors))
    docs = documents()
    output.mkdir(parents=True)
    for name, value in docs.items():
        dump(output / name, value)
    dump(output / 'syntax-validation.json', parse_sources(json.dumps(docs['public-inputs.json']['cases'])))
    dump(output / 'formal-state.json', load('prepare-source-review-supplement.py').formal_state(ROOT))
    (output / 'generator.py.txt').write_bytes(Path(__file__).read_bytes())
    dump(output / 'manifest.json', {'schema_version': 1, 'kind': 'rule_input_preparation_only',
         'input_bindings': {str(p): sha((ROOT / p).read_bytes()) for p in bindings()},
         'counts': docs['coverage-summary.json']['counts'],
         'artifacts': {p.relative_to(output).as_posix(): sha(p.read_bytes()) for p in sorted(output.rglob('*')) if p.is_file()}})


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('output', type=Path)
    args = parser.parse_args()
    prepare(args.output.resolve())
    print('PASS: rule source proposals and syntax validation only; no analyzer/quality/acceptance execution')
