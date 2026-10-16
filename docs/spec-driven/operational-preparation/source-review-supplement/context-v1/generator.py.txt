#!/usr/bin/env python3
"""Freeze non-exhaustive AI caller/callee notes with offline Git object proofs."""
import argparse
import hashlib
import json
from pathlib import Path
import subprocess

ROOT = Path(__file__).resolve().parents[1]
BASE = Path('docs/spec-driven/operational-preparation/source-review-supplement')


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def object_id(kind, raw):
    return hashlib.sha1(kind.encode() + b' ' + str(len(raw)).encode() + b'\0' + raw).hexdigest()


def dump(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2) + '\n')


def prepare(repo, output, proposals):
    if output.exists():
        raise ValueError('preserve previous packets; output must not exist')
    pool = json.loads((ROOT / 'docs/spec-driven/OPERATIONAL-ACCEPTANCE-INPUT-CANDIDATES.json').read_text())
    revision = pool['repository_commit']

    def git(*args):
        return subprocess.check_output(['git', '-C', str(repo), *args])

    objects, files = {}, {}

    def save_object(kind, oid):
        key = kind + ':' + oid
        if key not in objects:
            raw = git('cat-file', kind, oid)
            if object_id(kind, raw) != oid:
                raise ValueError('Git object identity mismatch')
            path = 'objects/' + oid + '.' + kind
            (output / path).parent.mkdir(parents=True, exist_ok=True)
            (output / path).write_bytes(raw)
            objects[key] = path

    save_object('commit', revision)
    for proposal in proposals:
        proposal.update(origin='ai_manual_source_reading', approved=False, reviewer=None,
                        review_receipt=None, verified_defect=False, exhaustive_gold=False,
                        precision_recall_scoring_allowed=False)
        for anchor in proposal['anchors']:
            path = anchor['path']
            if path not in files:
                raw = git('show', revision + ':' + path)
                parts = Path(path).parts
                for i in range(len(parts)):
                    treeish = revision + '^{tree}' if i == 0 else revision + ':' + '/'.join(parts[:i])
                    save_object('tree', git('rev-parse', treeish).decode().strip())
                artifact = 'sources/' + path + '.txt'
                (output / artifact).parent.mkdir(parents=True, exist_ok=True)
                (output / artifact).write_bytes(raw)
                files[path] = {'artifact': artifact, 'git_blob': object_id('blob', raw), 'sha256': sha(raw)}
            raw = (output / files[path]['artifact']).read_bytes()
            lines = raw.splitlines(keepends=True)
            begin = sum(map(len, lines[:anchor['start_line']-1]))
            finish = sum(map(len, lines[:anchor['end_line']]))
            anchor.update(start_byte=begin, end_byte_exclusive=finish, span_sha256=sha(raw[begin:finish]))
    dump(output / 'private-context-review.json', {'schema_version': 1, 'status': 'ai_draft_not_independent_gold',
         'independent_gold_records': 0, 'runtime_executions': 0, 'reviews': proposals})
    dump(output / 'git-proof.json', {'commit': revision, 'objects': objects, 'files': files})
    raw = Path(__file__).read_bytes()
    (output / 'generator.py.txt').write_bytes(raw)
    prior = ROOT / BASE / 'expansion-v1/manifest.json'
    manifest = {'schema_version': 1, 'kind': 'private_source_context_supplement_not_gold',
                'repository_commit': revision, 'previous_packet': str(prior.relative_to(ROOT)),
                'previous_packet_sha256': sha(prior.read_bytes()),
                'counts': {'reviews': len(proposals), 'anchors': sum(len(x['anchors']) for x in proposals),
                           'source_files': len(files)},
                'artifacts': {p.relative_to(output).as_posix(): sha(p.read_bytes()) for p in sorted(output.rglob('*')) if p.is_file()}}
    dump(output / 'manifest.json', manifest)
    print(json.dumps(manifest['counts']))


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--repo', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--proposals', type=Path, required=True)
    args = parser.parse_args()
    prepare(args.repo, args.output, json.loads(args.proposals.read_text()))
