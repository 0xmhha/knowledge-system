#!/usr/bin/env python3
"""Check that every cited gold span lies in a file the frozen dataset indexes.

usage: index-scope-check.py <question-book.json> <dataset-version-dir> <output.json>
A gold file must have CKV chunks overlapping the gold lines and (for code)
CKG nodes in the same file; otherwise the question cannot be retrieved by
any binary and is reported. Exit 1 when any gold span is out of scope."""
import json, os, sqlite3, sys
book, version, out = sys.argv[1:4]
q = json.load(open(book))
uri = lambda p: 'file:' + p + '?mode=ro&immutable=1'
v = sqlite3.connect(uri(os.path.join(version, 'vector', 'vector.db')), uri=True)
cols = [r[1] for r in v.execute("pragma table_info('chunks')")]
fcol = next(c for c in ('file', 'file_path', 'path', 'source_path') if c in cols)
scol = next(c for c in ('start_line', 'line_start', 'first_line') if c in cols)
ecol = next(c for c in ('end_line', 'line_end', 'last_line') if c in cols)
g = sqlite3.connect(uri(os.path.join(version, 'graph', 'graph.db')), uri=True)
graph_files = {r[0] for r in g.execute('select distinct file_path from nodes')}
rows, bad = [], []
for item in q['questions']:
    for e in item.get('evidence', []):
        chunks = v.execute(f'select count(*) from chunks where {fcol}=? and {scol}<=? and {ecol}>=?', (e['path'], e['last'], e['first'])).fetchone()[0]
        code = not e['path'].endswith('.md')
        ok = chunks > 0 and (not code or e['path'] in graph_files)
        rows.append({'id': item['id'], 'path': e['path'], 'first': e['first'], 'last': e['last'],
                     'overlapping_ckv_chunks': chunks, 'ckg_file_indexed': e['path'] in graph_files, 'in_scope': ok})
        if not ok:
            bad.append(item['id'])
result = {'schema_version': 1, 'question_book': book, 'dataset_version': version, 'spans': rows,
          'out_of_scope_questions': sorted(set(bad)), 'pass': not bad}
open(out, 'w').write(json.dumps(result, indent=1) + '\n')
print(json.dumps({'pass': result['pass'], 'out_of_scope_questions': result['out_of_scope_questions']}))
sys.exit(0 if result['pass'] else 1)
