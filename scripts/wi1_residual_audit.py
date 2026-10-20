"""Classify all residual coordinate mismatches, without inventing missing semantics."""
import argparse
import json
import subprocess
from collections import Counter
from pathlib import Path

import wi1_consumer_matching as m

ROOT = m.ROOT
BASE = m.BASE/'followups'
OUTPUT = BASE/'wi1-residual-classification-v1.json'


def derive():
    observations = m.old.packed(ROOT/BASE/'wi1-select-review-v1/observations.json.gz')
    comparisons = m.old.packed(ROOT/BASE/'wi1-select-review-v1/comparisons.json.gz')
    actual = {r['case_id']: r['response']['analysis'] for r in m.old.read(ROOT/m.PACKET/'runtime/project-results.json')}
    occurrences, blocks = [], []
    for o, comparison in zip(observations, comparisons):
        cid = comparison['case_id']
        analysis = actual[cid]
        refs = {r['id']: r for r in o['occurrences']}
        bindings = {r['id']: r for r in o['bindings']}
        nodes = {r['id']: r for r in o['nodes']}
        reference_blocks = {r['id']: r for r in o['blocks']}
        consumer_nodes = {r['id']: r for r in analysis['nodes']}
        consumer_variables = {r['id']: r for r in analysis['variables']}
        for row in comparison['mapping']['occurrences']:
            if row['consumer'] is not None:
                continue
            ref = refs[row['reference'][2:]]
            binding = bindings[ref['binding']]
            same = [r for r in analysis['occurrences'] if
                    (r['location']['start_byte'], r['location']['end_byte']) == (ref['span']['start'], ref['span']['end'])]
            if row['phase'] != 'definition' or ref['role'] != 'definition':
                raise ValueError('new residual phase requires explicit review')
            if ref['mapping_reason'] == 'selected_receive_LHS_exact_AST_case_identity':
                if len(same) != 1 or same[0]['role'] != 'definition':
                    raise ValueError('selected receive coordinate not uniquely observed')
                owner = consumer_nodes[same[0]['node']]
                variable = consumer_variables[same[0]['variable']]
                if owner['kind'] != 'AssignStmt' or reference_blocks[nodes[ref['owner']]['block']]['kind'] != 'SelectCaseBody' or variable['name'] != binding['name'] or (variable['declaration']['start_byte'], variable['declaration']['end_byte']) != (binding['declaration']['start'], binding['declaration']['end']):
                    raise ValueError('selected receive binding/owner evidence differs')
                classification = 'selected_receive_definition_attached_to_preselection_assignment'
                action = 'consumer_fix_required; emit_definition_only_in_selected_case; do_not_relax_reference_owner_match'
            else:
                variables = [r for r in analysis['variables'] if
                             (r['declaration']['start_byte'], r['declaration']['end_byte']) == (binding['declaration']['start'], binding['declaration']['end'])]
                if same or len(variables) != 1 or variables[0]['kind'] != 'range_unknown':
                    raise ValueError('unmodeled definition is not declared range_unknown')
                classification = 'range_definition_explicitly_outside_scalar_assignment_support'
                action = 'retain_unknown_definition_and_range_binding_unsupported; no_missing_edge_as_negative'
            occurrences.append(dict(case_id=cid, source_sha256=o['function_sha256'], path=o['path'],
                                    reference=row['reference'], name=binding['name'], span=ref['span'],
                                    reference_owner=ref['owner'], consumer_same_coordinate=same,
                                    classification=classification, action=action,
                                    semantic_value='unknown', independent_approval=None))
        for row in comparison['mapping']['blocks']:
            if row['consumer'] is not None:
                continue
            ref = reference_blocks[row['reference']]
            if ref['kind'] != 'Unreachable' or ref['live'] or ref['nodes'] or len(row['candidates']) != 3:
                raise ValueError('new block ambiguity requires explicit review')
            candidates = [r for r in analysis['blocks'] if r['id'] in row['candidates']]
            if len(candidates) != 3 or any(r['kind'] != 'Unreachable' or r['reachable'] or r['nodes'] for r in candidates):
                raise ValueError('ambiguous blocks are no longer indistinguishable empty unreachable blocks')
            blocks.append(dict(case_id=cid, reference=ref, candidates=candidates,
                               classification='unreachable_empty_blocks_have_no_distinguishing_public_anchor',
                               action='unmatched_unknown; do_not_map_by_numeric_id', independent_approval=None))
    counts = dict(Counter(r['classification'] for r in occurrences))
    if counts != {'range_definition_explicitly_outside_scalar_assignment_support': 37,
                  'selected_receive_definition_attached_to_preselection_assignment': 16} or len(blocks) != 3:
        raise ValueError('residual denominator/classification changed')
    inputs = [BASE/'wi1-select-review-v1'/name for name in ('observations.json.gz', 'comparisons.json.gz')]
    inputs += [m.PACKET/'runtime/project-results.json', Path('pkg/graph/valueflow/dependence.go')]
    return dict(kind='WI1_AI_DEV_residual_diagnostic_not_independent_gold',
                source_revision='3c23a71c3872dbef69a84726a58fb4ade27001e5',
                input_sha256={str(p): m.old.sha(subprocess.check_output(['git', 'show', '3c23a71c3872dbef69a84726a58fb4ade27001e5:'+str(p)], cwd=ROOT)
                    if str(p) == 'pkg/graph/valueflow/dependence.go' else (ROOT/p).read_bytes()) for p in inputs},
                occurrence_phases=53, blocks=3, classifications=counts,
                occurrences=occurrences, ambiguous_blocks=blocks,
                residual_classification_complete=True, consumer_fix_completed=False,
                semantic_gold_complete=False, independent_approved=0, formal_completed=0,
                new_actual_consumer_calls=0, existing_consumer_responses_reused=30,
                quality_score=None, personal_names_required=False)


if __name__ == '__main__':
    p = argparse.ArgumentParser()
    p.add_argument('command', choices=['write', 'check'])
    a = p.parse_args()
    data = derive()
    target = ROOT/OUTPUT
    if a.command == 'write':
        target.write_text(json.dumps(data, ensure_ascii=False, indent=2)+'\n')
    elif m.old.read(target) != data:
        raise SystemExit('residual classification changed or falsely promoted')
    print(json.dumps({k: data[k] for k in ('occurrence_phases', 'blocks', 'classifications', 'consumer_fix_completed', 'independent_approved')}))
