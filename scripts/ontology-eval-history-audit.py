#!/usr/bin/env python3
"""Inventory saved evaluation artifacts and conservatively audit proposed gold.

Repository artifact coverage and semantic independence are separate. Parse
failures, unsupported citations and unbound file revisions remain visible.
No MCP/model calls, input approval, or FINAL eligibility are granted here.
"""
import argparse
import collections
import gzip
import importlib.util
import json
from pathlib import Path
import re
import sys

SPEC = importlib.util.spec_from_file_location('exposure_helpers', Path(__file__).with_name('ontology-eval-exposure-audit.py'))
E = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(E)
MAX_JSON_BYTES = 64 * 1024 * 1024


def first(row, keys):
    return next((row[k] for k in keys if k in row), None)


def location(row):
    if not isinstance(row, dict):
        return None
    name = first(row, ['file', 'File', 'path', 'Path', 'file_path', 'filePath'])
    if not isinstance(name, str):
        return None
    name = name.replace('\\', '/')
    if '/consensus/wbft/' in name:
        name = 'consensus/wbft/' + name.split('/consensus/wbft/', 1)[1]
    name = name.removeprefix('./')
    return {'file': name,
            'start_line': first(row, ['start_line', 'StartLine', 'first', 'start', 'line_start']),
            'end_line': first(row, ['end_line', 'EndLine', 'last', 'end', 'line_end']),
            'file_sha256': first(row, ['file_sha256', 'fileSHA256']),
            'commit_hash': first(row, ['commit_hash', 'source_commit', 'commit'])}


def body_texts(view):
    for key in ['bodies', 'body', 'context', 'instructions']:
        value = view.get(key)
        if isinstance(value, str):
            yield value
        elif isinstance(value, list):
            for body in value:
                if isinstance(body, str):
                    yield body
                elif isinstance(body, dict) and isinstance(body.get('text'), str):
                    yield body['text']


def representations(root):
    """Yield response views and authored gold; avoid MCP text/structured mirrors."""
    stack = [root]
    while stack:
        node = stack.pop()
        if isinstance(node, list):
            stack.extend(reversed(node))
            continue
        if not isinstance(node, dict):
            continue
        structured = node.get('structuredContent')
        if isinstance(structured, dict):
            stack.append(structured)
            continue
        citations = node.get('citations')
        if isinstance(citations, list) and any(k in node for k in ['query', 'metadata', 'coordinates', 'format_version']):
            yield 'response', node
            continue
        if node.get('type') == 'text' and isinstance(node.get('text'), str):
            text = node['text'].lstrip()
            if text.startswith(('{', '[')):
                # JSON-looking legacy MCP payloads must not silently vanish.
                stack.append(E.decode(text))
            continue
        for key in ['expected_citations', 'evidence', 'gold', 'abstention_basis_sources']:
            spans = node.get(key)
            if isinstance(spans, list) and any(location(span) is not None for span in spans):
                yield 'authored_gold', {'query': node.get('question', node.get('prompt', '')),
                                        'citations': spans, 'bodies': []}
        stack.extend(reversed(list(node.values())))


def records(path):
    opener = gzip.open if path.name.endswith('.gz') else open
    # go list -json saves adjacent pretty-printed objects, even with .jsonl
    # names. Bound each value, not a whole multi-record FINAL archive.
    decoder, text, offset, number, eof = json.JSONDecoder(), '', 0, 0, False
    with opener(path, 'rt', encoding='utf-8') as stream:
        while True:
            while offset < len(text) and text[offset].isspace():
                offset += 1
            if offset == len(text) and eof:
                E.require(number > 0, 'empty artifact has no JSON evidence')
                break
            try:
                _, end = decoder.raw_decode(text, offset)
                # A value ending on the chunk boundary may still continue.
                if end == len(text) and not eof:
                    raise json.JSONDecodeError('need boundary', text, end)
            except json.JSONDecodeError:
                if eof:
                    raise
                text = text[offset:]
                offset = 0
                E.require(len(text.encode('utf-8')) <= MAX_JSON_BYTES, 'JSON value exceeds limit')
                chunk = stream.read(65536)
                text += chunk
                eof = not chunk
                continue
            raw = text[offset:end]
            E.require(len(raw.encode('utf-8')) <= MAX_JSON_BYTES, 'JSON value exceeds limit')
            number += 1
            yield number, E.decode(raw)
            offset = end


class HistoryAudit:
    def __init__(self, book, source_root):
        E.audit(book, [], source_root)  # validates all current gold bytes/approval bounds
        self.book = book
        self.by_file = collections.defaultdict(list)
        self.rows = {}
        self.seen = set()
        self.views = collections.Counter()
        self.unsupported = []
        for q in book['questions']:
            self.rows[q['id']] = {'id': q['id'], 'response_range_exposure': 0, 'response_body_exposure': 0,
                                  'body_text_exposure': 0, 'authored_gold_exposure': 0,
                                  'text_history_snippet_exposure': 0, 'text_symbol_review_cues': [],
                                  'prior_query_exact_matches': 0,
                                  'unknown_revision_or_range': 0, 'example_locations': []}
            for gold in q['evidence']:
                raw = E.source(source_root, gold['file'])
                snippet = b''.join(raw.splitlines(keepends=True)[gold['start_line'] - 1:gold['end_line']]).decode().strip()
                self.by_file[gold['file']].append((q['id'], gold, snippet))

    def consume(self, role, view, origin):
        self.views['observed_' + role] += 1
        digest = E.sha(json.dumps([role, view], sort_keys=True, ensure_ascii=False, separators=(',', ':')).encode())
        if digest in self.seen:
            self.views['duplicate_views'] += 1
            return
        self.seen.add(digest)
        self.views['unique_' + role] += 1
        texts = list(body_texts(view))
        flags = collections.defaultdict(set)
        for q in self.book['questions']:
            query = view.get('query')
            if isinstance(query, str) and E.normalize_query(query) == E.normalize_query(q['question']):
                flags[q['id']].add('prior_query_exact_matches')
        # Delivered source text is exposure even when its citation is absent,
        # points to a different file, or has an unbound revision.
        for targets in self.by_file.values():
            for qid, _, snippet in targets:
                if role in ('response', 'text_history') and any(snippet in text.replace('\r\n', '\n') for text in texts):
                    flags[qid].add('body_text_exposure' if role == 'response' else 'text_history_snippet_exposure')
        if role == 'text_history':
            for q in self.book['questions']:
                names = {g.get('canonical_id', '').rsplit('.', 1)[-1] for g in q['evidence']}
                hits = sorted(name for name in names if name and any(re.search(r'\b' + re.escape(name) + r'\b', text) for text in texts))
                if hits:
                    self.rows[q['id']]['text_symbol_review_cues'].append({'origin': origin, 'symbols': hits})
        for raw_citation in view['citations']:
            citation = location(raw_citation)
            if citation is None:
                self.unsupported.append(origin)
                continue
            targets = self.by_file.get(citation['file'], [])
            for qid, gold, snippet in targets:
                start, end = citation['start_line'], citation['end_line']
                exact_bytes = citation['file_sha256'] == gold['file_sha256']
                same_commit = citation['commit_hash'] == gold['commit_hash']
                valid_range = type(start) is int and type(end) is int and 1 <= start <= end
                if valid_range and (exact_bytes or same_commit):
                    if start <= gold['end_line'] and gold['start_line'] <= end:
                        flags[qid].add('authored_gold_exposure' if role == 'authored_gold' else 'response_range_exposure')
                        bodies = view.get('bodies')
                        if role == 'response' and isinstance(bodies, list) and any(b.get('citation') == raw_citation for b in bodies if isinstance(b, dict)):
                            flags[qid].add('response_body_exposure')
                else:
                    flags[qid].add('unknown_revision_or_range')
        for qid, kinds in flags.items():
            row = self.rows[qid]
            for kind in kinds:
                row[kind] += 1
            if len(row['example_locations']) < 8:
                row['example_locations'].append({'origin': origin, 'role': role, 'flags': sorted(kinds)})

    def report(self, catalog):
        excluded = []
        unknown = []
        prospective = []
        for qid, row in self.rows.items():
            if any(row[k] for k in ['response_range_exposure', 'response_body_exposure', 'body_text_exposure', 'authored_gold_exposure', 'prior_query_exact_matches', 'text_history_snippet_exposure']):
                row['queue_state'] = 'quarantined_prior_exposure'
                excluded.append(qid)
            elif row['unknown_revision_or_range']:
                row['queue_state'] = 'historical_revision_review_needed'
                unknown.append(qid)
            else:
                row['queue_state'] = 'content_and_semantic_independence_review_needed'
                prospective.append(qid)
            row['independent_FINAL_eligible'] = False
        unread = [r['path'] for r in catalog if r['state'] != 'parsed']
        empty = [r['path'] for r in catalog if r.get('empty_artifact')]
        return {'scope': 'inventoried repository evaluation JSON/JSONL/gzip artifacts and optional Markdown/YAML history excluding candidate origins',
                'artifact_files': len(catalog), 'parsed_files': len(catalog) - len(unread),
                'unread_or_failed_files': unread, 'unsupported_citation_views': len(self.unsupported),
                'empty_artifact_files': empty, 'nonempty_catalog_fully_parsed': set(unread) == set(empty) and not self.unsupported,
                'unsupported_examples': self.unsupported[:20], 'views': dict(self.views),
                'candidates': list(self.rows.values()), 'quarantined_ids': excluded,
                'historical_revision_review_ids': unknown, 'prospective_review_ids': prospective,
                'machine_catalog_fully_parsed': not unread and not self.unsupported,
                'semantic_independence_review_complete': False, 'approved_independent_units': 0,
                'FINAL_eligible_questions': 0, 'quality_evaluated': False, 'new_model_calls': 0,
                'limits': ['missing or changed file hashes/revisions are not compared by coincidental line number',
                           'identical commit permits comparison of authored gold without full-file SHA',
                           'repeated and mirrored evidence views are not independent units',
                           'text symbol mentions are review cues, not line overlap or independent facts',
                           'chat/external/deleted history and unrequested text roots are outside this machine catalog',
                           'absence of source exposure never proves semantic independence']}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--book', type=Path, required=True)
    parser.add_argument('--source-root', type=Path, required=True)
    parser.add_argument('--eval-root', type=Path, required=True)
    parser.add_argument('--exclude-origin', type=Path, action='append', default=[])
    parser.add_argument('--text-root', type=Path, action='append', default=[])
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    E.require(not args.output.exists(), 'output already exists')
    exclusions = [p.resolve() for p in args.exclude_origin] + [args.book.parent.resolve()]
    paths = sorted(p for p in args.eval_root.rglob('*') if p.is_file() and
                   p.name.endswith(('.json', '.jsonl', '.json.gz', '.jsonl.gz')) and
                   not any(p.resolve().is_relative_to(e) for e in exclusions))
    auditor_bytes = Path(__file__).read_bytes()
    book_bytes = args.book.read_bytes()
    book = E.decode(book_bytes)
    audit = HistoryAudit(book, args.source_root)
    catalog = []
    for index, path in enumerate(paths, 1):
        row = {'path': str(path), 'sha256': None, 'state': 'parsed', 'records': 0, 'kind': 'JSON'}
        try:
            raw = path.read_bytes()
            row['sha256'] = E.sha(raw)
            row['empty_artifact'] = not raw
            for number, record in records(path):
                row['records'] += 1
                for role, view in representations(record):
                    audit.consume(role, view, str(path) + ':' + str(number))
        except (ValueError, OSError, KeyError, TypeError, RecursionError) as error:
            row['state'] = 'unread_or_invalid'
            row['reason'] = type(error).__name__ + ': ' + str(error)[:180]
        catalog.append(row)
        if index % 200 == 0:
            print(json.dumps({'parsed_inventory': index, 'planned': len(paths), 'views': dict(audit.views)}), flush=True)
    text_paths = sorted({p for root in args.text_root for p in root.rglob('*') if p.is_file() and
                         p.suffix in ('.md', '.yaml', '.yml') and
                         not any(p.resolve().is_relative_to(e) for e in exclusions)})
    for path in text_paths:
        row = {'path': str(path), 'sha256': None, 'state': 'parsed', 'records': 0, 'kind': 'text_history'}
        try:
            raw = path.read_bytes()
            row['sha256'] = E.sha(raw)
            E.require(len(raw) <= MAX_JSON_BYTES, 'text artifact exceeds limit')
            audit.consume('text_history', {'citations': [], 'body': raw.decode('utf-8')}, str(path))
            row['records'] = 1
        except (ValueError, OSError, UnicodeError) as error:
            row.update(state='unread_or_invalid', reason=type(error).__name__ + ': ' + str(error)[:180])
        catalog.append(row)
    report = audit.report(catalog)
    report['JSON_artifact_files'] = len(paths)
    report['text_history_files'] = len(text_paths)
    report['catalog'] = catalog
    E.require(args.book.read_bytes() == book_bytes, 'book changed during audit')
    E.require(Path(__file__).read_bytes() == auditor_bytes, 'auditor changed during audit')
    report['input_sha256'] = {'book': E.sha(book_bytes), 'auditor': E.sha(auditor_bytes)}
    with args.output.open('x', encoding='utf-8') as stream:
        stream.write(json.dumps(report, ensure_ascii=False, indent=2) + '\n')
    print(json.dumps({k: report[k] for k in ['artifact_files', 'parsed_files', 'quarantined_ids',
                                           'historical_revision_review_ids', 'prospective_review_ids', 'FINAL_eligible_questions']}))


if __name__ == '__main__':
    try:
        main()
    except (ValueError, OSError, KeyError, TypeError) as error:
        print('ontology history audit: FAIL: ' + str(error), file=sys.stderr)
        sys.exit(1)
