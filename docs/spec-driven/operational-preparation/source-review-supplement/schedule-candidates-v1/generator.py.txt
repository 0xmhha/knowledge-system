#!/usr/bin/env python3
"""Freeze source-bound, unapproved schedule proposals with offline Git proofs."""
import argparse
import hashlib
import importlib.util
import itertools
import json
from pathlib import Path
import subprocess

ROOT = Path(__file__).resolve().parents[1]
BASE = Path('docs/spec-driven/operational-preparation/source-review-supplement')


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def dump(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2) + '\n')


def oid(kind, raw):
    return hashlib.sha1(kind.encode() + b' ' + str(len(raw)).encode() + b'\0' + raw).hexdigest()


def prepare(repo, goroot, output, proposals):
    if output.exists():
        raise ValueError('use a new output directory; preserve earlier evidence')
    pool = json.loads((ROOT / 'docs/spec-driven/OPERATIONAL-ACCEPTANCE-INPUT-CANDIDATES.json').read_text())
    revision = pool['repository_commit']
    output.mkdir(parents=True)

    def git(*args):
        return subprocess.check_output(['git', '-C', str(repo), *args])

    objects, files = {}, {}

    def save(kind, identity):
        key = kind + ':' + identity
        if key not in objects:
            raw = git('cat-file', kind, identity)
            if oid(kind, raw) != identity:
                raise ValueError('Git object mismatch')
            artifact = 'objects/' + identity + '.' + kind
            (output / artifact).parent.mkdir(exist_ok=True)
            (output / artifact).write_bytes(raw)
            objects[key] = artifact

    save('commit', revision)
    public = []
    for proposal in proposals:
        public.append(proposal.pop('public_input'))
        proposal.update(origin='ai_manual_source_reading_and_schedule_design', approved=False,
                        reviewer=None, review_receipt=None, independent_oracle=None,
                        accepted_family=False, verified_project_defect=False, operational_reachability='not_verified')
        for anchor in proposal['source_anchors']:
            path = anchor['path']
            if path not in files:
                raw = git('show', revision + ':' + path)
                parts = Path(path).parts
                for i in range(len(parts)):
                    treeish = revision + '^{tree}' if i == 0 else revision + ':' + '/'.join(parts[:i])
                    save('tree', git('rev-parse', treeish).decode().strip())
                artifact = 'sources/' + path + '.txt'
                (output / artifact).parent.mkdir(parents=True, exist_ok=True)
                (output / artifact).write_bytes(raw)
                files[path] = {'artifact': artifact, 'git_blob': oid('blob', raw), 'sha256': sha(raw)}
            raw = (output / files[path]['artifact']).read_bytes()
            lines = raw.splitlines(keepends=True)
            start, end = anchor['start_line'], anchor['end_line']
            if not 1 <= start <= end <= len(lines):
                raise ValueError('anchor outside source')
            begin, finish = sum(map(len, lines[:start-1])), sum(map(len, lines[:end]))
            anchor.update(start_byte=begin, end_byte_exclusive=finish, span_sha256=sha(raw[begin:finish]))
    dump(output / 'git-proof.json', {'commit': revision, 'objects': objects, 'files': files})
    dump(output / 'public-inputs.json', {'schema_version': 1, 'status': 'schedule_input_candidates_only',
         'contains_gold': False, 'cases': public})
    pairs = []
    for a, b in itertools.combinations(proposals, 2):
        pairs.append({'a': a['case_id'], 'b': b['case_id'],
                      'same_mechanism_proposed': a['mechanism'] == b['mechanism'],
                      'independence_status': 'not_reviewed', 'shared_risks':
                      ['same_ai_author', 'shared_project_snapshot', 'source_overlap_and_semantic_dependence_require_review']})
    dump(output / 'private-schedules.json', {'schema_version': 1, 'status': 'ai_schedule_drafts_not_independent_gold',
         'accepted_independent_families': 0, 'formal_runs': 0, 'new_local_probe_runs': 0,
         'reviewer': None, 'split': None, 'families': proposals, 'pairwise_overlap_review': pairs,
         'sampling_proposal': {'runs_per_arm': 100, 'minimum_baseline_symptom_events': 10,
                               'arms': ['baseline', 'candidate_fix'], 'candidate_fix_revision': None,
                               'baseline_revision': revision, 'sampling_approved': False,
                               'zero_or_insufficient_baseline': 'inconclusive',
                               'statistical_independence': 'not_reviewed'},
         'runtime_boundary': {'external_model_calls': 0, 'nodes_started': 0, 'host_clock_changes': 0,
                              'target_state_writes': 0, 'operational_execution_allowed': False}})
    stdlib = {}
    for path in ['src/container/list/list.go', 'src/time/time.go']:
        raw = (goroot / path).read_bytes()
        artifact = 'toolchain/' + path + '.txt'
        (output / artifact).parent.mkdir(parents=True, exist_ok=True)
        (output / artifact).write_bytes(raw)
        stdlib[path] = {'artifact': artifact, 'sha256': sha(raw)}
    dump(output / 'toolchain-notes.json', {'kind': 'local_fixture_semantics_not_target_runtime_adoption',
         'go_version': 'go1.26.9', 'sources': stdlib,
         'list_removal': 'Remove clears the element links; Next on a removed element returns nil',
         'monotonic_limit': 'Now.Add deadlines and Sub use monotonic readings when both contain them; wall-clock rollback alone is not a demonstrated trigger',
         'target_runtime_verified': False, 'clock_rollback_reproduced': False})
    spec = importlib.util.spec_from_file_location('formal_prepare', ROOT / 'scripts/prepare-source-review-supplement.py')
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    dump(output / 'formal-state.json', mod.formal_state(ROOT))
    (output / 'generator.py.txt').write_bytes(Path(__file__).read_bytes())
    references = {str(BASE / name / 'manifest.json'): sha((ROOT / BASE / name / 'manifest.json').read_bytes())
                  for name in ['context-v1', 'subscription-probe-v1']}
    manifest = {'schema_version': 1, 'kind': 'source_schedule_candidates_not_independent_acceptance',
                'repository_commit': revision, 'prior_packets': references,
                'counts': {'candidate_families': len(proposals), 'new_candidate_families': len(proposals)-1,
                           'schedule_proposals': sum(len(x['schedules']) for x in proposals),
                           'source_anchors': sum(len(x['source_anchors']) for x in proposals),
                           'source_files': len(files), 'pairwise_overlap_reviews': len(pairs),
                           'accepted_independent_families': 0},
                'artifacts': {p.relative_to(output).as_posix(): sha(p.read_bytes()) for p in sorted(output.rglob('*')) if p.is_file()}}
    dump(output / 'manifest.json', manifest)
    print(json.dumps(manifest['counts']))


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--repo', type=Path, required=True)
    parser.add_argument('--goroot', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--proposals', type=Path, required=True)
    args = parser.parse_args()
    prepare(args.repo, args.goroot, args.output, json.loads(args.proposals.read_text()))
