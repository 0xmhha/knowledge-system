"""BFS from external entry points along calls edges (depth <= 10) and report
which sink kinds each entry reaches, with one shortest path per (entry, sink
function). Writes reach.json."""
import json, sqlite3, collections, sys
A = sys.argv[1]
db = sqlite3.connect(f"file:{A}/graph/graph.db?mode=ro&immutable=1", uri=True)
fn = {r[0]: r[1:] for r in db.execute("select id, qualified_name, file_path, start_line from nodes where type in ('Function','Method')")}
adj = collections.defaultdict(set)
for s, d in db.execute("select src,dst from edges where type in ('calls') "):
    if s in fn and d in fn: adj[s].add(d)
entries = {}
for i, (q, f, l) in fn.items():
    if f.endswith("_test.go"): continue
    if (f.startswith("internal/graph/server/") and q.startswith("server.Server.handle")) \
       or (f.startswith("internal/system/viewer/") and ".handle" in q.lower()) \
       or (f.startswith("internal/system/mcp/") and q.split(".")[-1].startswith("handle")) \
       or ("/mcp" in f and q.split(".")[-1].startswith("handle")) \
       or (f.startswith("internal/graph/mcp") ):
        entries[i] = q
sinks = json.load(open(f"{A}/sinks.json"))
sinkfn = collections.defaultdict(set)
for k, rows in sinks.items():
    for r in rows:
        if r["fn_id"]: sinkfn[r["fn_id"]].add(k)
res = collections.defaultdict(list)
for e, q in sorted(entries.items(), key=lambda x: x[1]):
    prev = {e: None}; dq = collections.deque([(e, 0)])
    while dq:
        n, d = dq.popleft()
        if d >= 10: continue
        for m in adj[n]:
            if m not in prev: prev[m] = n; dq.append((m, d + 1))
    for n in prev:
        for k in sinkfn.get(n, ()):
            path = []; x = n
            while x is not None: path.append(fn[x][0]); x = prev[x]
            res[k].append({"entry": q, "sink_fn": fn[n][0], "file": fn[n][1], "path": list(reversed(path))})
json.dump(res, open(f"{A}/reach.json", "w"), indent=1)
print("entries", len(entries))
for k, v in sorted(res.items()): print(f"{k:11s} paths={len(v):4d} sink_fns={len({r['sink_fn'] for r in v})} entries={len({r['entry'] for r in v})}")
