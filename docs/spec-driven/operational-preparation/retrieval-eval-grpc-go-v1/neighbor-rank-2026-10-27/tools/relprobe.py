"""Offline: cosine relevance of every Stage 3 neighbor (and seed) to the prompt, from the index's own vectors.

score = 1 - L2/2 over unit vectors, the same normalized score ckv reports (validated on pr-9003). Vectors are read from the sqlite-vec
vec0 shadow tables (chunk_vec_rowids -> chunk_vec_vector_chunks00), query embedded via Ollama /api/embed
with the registry query transform. A neighbor's relevance = max score of index chunks overlapping its span.
usage: relprobe.py <tag> <model> [--validate]
"""
import json, sqlite3, sys, urllib.request
import numpy as np
from pathlib import Path
W = Path('/Users/kevin/work/github/0xmhha/auto-coding/knowledge-data/grpc-go-retrieval-eval')
INSTR = "Given a natural-language question about a codebase, retrieve the most relevant code."
tag, model = sys.argv[1], sys.argv[2]
slug = model.replace(':', '-')
def embed(text):
    if model.startswith('qwen3'):
        text = "Instruct: " + INSTR + "\nQuery: " + text
    req = urllib.request.Request('http://localhost:11434/api/embed', json.dumps({'model': model, 'input': [text]}).encode(),
                                 {'Content-Type': 'application/json'})
    v = np.array(json.load(urllib.request.urlopen(req))['embeddings'][0], dtype=np.float32)
    return v / np.linalg.norm(v)
def load(snap):
    db = W / 'vec' / slug / snap / 'vector.db'
    con = sqlite3.connect(f'file:{db}?mode=ro', uri=True)
    dim = json.loads((W / 'vec' / slug / snap / 'manifest.json').read_text())['embedding_dim']
    blobs = {cid: np.frombuffer(b, dtype=np.float32).reshape(-1, dim) for cid, b in con.execute('select rowid, vectors from chunk_vec_vector_chunks00')}
    rows = con.execute('select r.chunk_id, r.chunk_offset, c.file, c.start_line, c.end_line from chunk_vec_rowids r join chunks c on c.id = r.id').fetchall()
    M = np.stack([blobs[cid][off] for cid, off, *_ in rows])
    meta = [(f, s, e) for _, _, f, s, e in rows]
    byfile = {}
    for i, (f, s, e) in enumerate(meta):
        byfile.setdefault(f, []).append((s, e, i))
    return M, meta, byfile
def rel(scores, byfile, f, s, e):
    best = 0.0
    for cs, ce, i in byfile.get(f, []):
        if cs <= e and s <= ce:
            best = max(best, float(scores[i]))
    return best
found = json.load(open(W / f'eval-{tag}' / f'found-{slug}.json'))['scenarios']
rep = {r['name']: r for r in json.load(open(W / f'eval-{tag}' / f'debug-report-{slug}.json'))['scenarios']}
out = {}
bysnap = {}
for f in found:
    bysnap.setdefault(f['snapshot'], []).append(f)
for snap, fs in bysnap.items():
    M, meta, byfile = load(snap)
    for f in fs:
        row = rep[f['name']]
        r = next(json.loads(l) for l in open(row['trace']) if json.loads(l)['seq'] == row['seq'])
        q = embed(r['prompt'])
        scores = 1 - np.sqrt(np.clip(2 - 2 * (M @ q), 0, None)) / 2
        if '--validate' in sys.argv:
            top = np.argsort(-scores)[:5]
            print(f['name'], [(meta[i][0], meta[i][1], round(float(scores[i]), 4)) for i in top]); sys.exit()
        s3 = [e for e in r['events'] if e['stage'] == 'stage3' and e['step'] == 'expand' and e['phase'] == 'after']
        nbs = s3[0]['data']['neighbors'] if s3 else []
        out[f['name']] = {'neighbors': [{'file': n['file'], 'start': n['start'], 'end': n['end'], 'kind': n['kind'], 'score': n['score'],
                                         'sources': n['sources'], 'rel': rel(scores, byfile, n['file'], n['start'], n['end'])} for n in nbs]}
    print(snap, len(fs), flush=True)
(W / f'eval-{tag}' / f'relprobe-{slug}.json').write_text(json.dumps(out))
