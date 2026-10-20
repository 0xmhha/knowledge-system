#!/usr/bin/env python3
"""Verify private AI notes and commit-to-span Git proofs without a source checkout."""
import argparse
import hashlib
import importlib.util
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
BASE = Path('docs/spec-driven/operational-preparation/source-review-supplement')


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def oid(kind, raw):
    return hashlib.sha1(kind.encode() + b' ' + str(len(raw)).encode() + b'\0' + raw).hexdigest()


def tree_entries(raw):
    rows, pos = {}, 0
    while pos < len(raw):
        space = raw.index(b' ', pos)
        zero = raw.index(b'\0', space)
        mode, name = raw[pos:space].decode(), raw[space+1:zero].decode()
        if len(raw[zero+1:zero+21]) != 20 or name in rows or '/' in name:
            raise ValueError('invalid tree entry')
        rows[name] = (mode, raw[zero+1:zero+21].hex())
        pos = zero + 21
    return rows


def check(packet, root=ROOT, check_references=True):
    errors = []

    def require(value, message):
        if not value:
            errors.append(message)

    def safe(path):
        p = Path(path)
        if p.is_absolute() or '..' in p.parts or not (packet / p).resolve().is_relative_to(packet.resolve()) or (packet / p).is_symlink():
            raise ValueError('unsafe artifact path')
        return packet / p

    try:
        manifest = json.loads((packet / 'manifest.json').read_text())
        require(manifest['kind'] == 'private_source_context_supplement_not_gold', 'packet kind')
        prior = BASE / 'expansion-v1/manifest.json'
        require(manifest['previous_packet'] == str(prior) and manifest['previous_packet_sha256'] == sha((root / prior).read_bytes()), 'previous packet binding')
        files = {p.relative_to(packet).as_posix() for p in packet.rglob('*') if p.is_file() and p.name != 'manifest.json'}
        require(files == set(manifest['artifacts']), 'artifact inventory')
        for path, digest in manifest['artifacts'].items():
            require(sha(safe(path).read_bytes()) == digest, 'artifact hash: ' + path)
        pool = json.loads((root / 'docs/spec-driven/OPERATIONAL-ACCEPTANCE-INPUT-CANDIDATES.json').read_text())
        revision = pool['repository_commit']
        proof = json.loads((packet / 'git-proof.json').read_text())
        require(proof['commit'] == manifest['repository_commit'] == revision, 'fixed source commit')

        def object_raw(kind, identity):
            raw = safe(proof['objects'][kind + ':' + identity]).read_bytes()
            if oid(kind, raw) != identity:
                raise ValueError('Git object hash mismatch')
            return raw

        commit = object_raw('commit', revision)
        tree = commit.splitlines()[0].decode().removeprefix('tree ')
        require(len(tree) == 40, 'commit root tree')
        for path, record in proof['files'].items():
            parts, current = Path(path).parts, tree
            require(not Path(path).is_absolute() and '..' not in parts, 'safe source path')
            for i, name in enumerate(parts):
                mode, current = tree_entries(object_raw('tree', current))[name]
                require(mode in ('100644', '100755') if i == len(parts)-1 else mode == '40000', 'Git source mode')
            raw = safe(record['artifact']).read_bytes()
            require(current == record['git_blob'] == oid('blob', raw) and sha(raw) == record['sha256'], 'commit-to-source proof: ' + path)
        notes = json.loads((packet / 'private-context-review.json').read_text())
        require(notes['status'] == 'ai_draft_not_independent_gold' and notes['independent_gold_records'] == 0
                and notes['runtime_executions'] == 0, 'gold/runtime boundary')
        reviews = notes['reviews']
        require(len(reviews) == 5 and {r['candidate_id'] for r in reviews} == {'SRC-001', 'SRC-006', 'SRC-016', 'SRC-021', 'SRC-036'}, 'review coverage')
        used = set()
        for review in reviews:
            require(review['origin'] == 'ai_manual_source_reading' and review['approved'] is False
                    and review['reviewer'] is None and review['review_receipt'] is None
                    and review['verified_defect'] is False and review['exhaustive_gold'] is False
                    and review['precision_recall_scoring_allowed'] is False, 'AI interpretation promotion')
            require(bool(review['remaining_unknowns']) and bool(review['unsupported_conclusions']), 'missing interpretation limits')
            followup = review['followup_candidate']
            require(followup['independent_expected_result'] is None, 'unreviewed expected result promotion')
            if review['candidate_id'] == 'SRC-021':
                require(followup['kind'] == 'ai_schedule_hypothesis_not_verified_ticket'
                        and followup['independently_derived_oracle'] is None
                        and followup['tickets_promoted_to_protocol'] == 0, 'hypothesis/ticket boundary')
            for anchor in review['anchors']:
                used.add(anchor['path'])
                record = proof['files'][anchor['path']]
                raw = safe(record['artifact']).read_bytes()
                lines = raw.splitlines(keepends=True)
                start, end = anchor['start_line'], anchor['end_line']
                require(1 <= start <= end <= len(lines), 'anchor line range')
                begin, finish = sum(map(len, lines[:start-1])), sum(map(len, lines[:end]))
                require(anchor['start_byte'] == begin and anchor['end_byte_exclusive'] == finish
                        and anchor['span_sha256'] == sha(raw[begin:finish]), 'anchor source binding')
                require(bool(anchor['source_fact_draft']) and anchor['source_fact_draft'] not in followup['public_input_proposal'], 'source fact leaked into public proposal')
        require(used == set(proof['files']), 'unreferenced source files')
        require(manifest['counts'] == {'reviews': len(reviews), 'anchors': sum(len(x['anchors']) for x in reviews), 'source_files': len(used)}, 'counts')
        spec = importlib.util.spec_from_file_location('expansion_guard', root / 'scripts/check-source-review-supplement.py')
        guard = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(guard)
        errors.extend(guard.check(root / BASE / 'expansion-v1', root, check_references=check_references))
        if check_references:
            for name in ['PROTOCOL', 'PROGRESS']:
                d = json.loads((root / f'docs/spec-driven/OPERATIONAL-ACCEPTANCE-{name}.json').read_text())
                require(d.get('review_context_supplement') == {
                    'manifest': (packet / 'manifest.json').relative_to(root).as_posix(),
                    'sha256': sha((packet / 'manifest.json').read_bytes()),
                    'status': 'private_ai_context_not_verified_defects_or_gold'}, 'context ledger reference: ' + name)
    except (ValueError, KeyError, OSError, TypeError, IndexError, UnicodeError) as exc:
        errors.append('invalid context packet: ' + str(exc))
    return errors


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('packet', type=Path)
    args = parser.parse_args()
    errors = check(args.packet.resolve())
    if errors:
        print('\n'.join(errors))
        raise SystemExit(1)
    print('PASS: 5 private AI context notes; 19 source spans; offline commit/tree/blob proofs; no independent approval')
