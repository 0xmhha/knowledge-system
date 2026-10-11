"""Compare current get_subgraph (depth 2) with the proposed code view on the
same seeds: how many current edges touch test code, and how many fall outside
the production code-relation neighbourhood (calls/invokes both directions,
interface bridge, used/instantiated types, depth 2)."""
import json, statistics, sys, time
sys.path.insert(0, sys.argv[1])
from review_view import Graph, CODE_REL, is_test
A = sys.argv[1]
g = Graph(f"{A}/graph-hist/graph.db", f"{A}/clone")
cur = json.load(open(f"{A}/current-subgraph.json"))
key = lambda nid: f"{g.node[nid][2]}:{g.node[nid][3]}"
rows = []
for q, c in cur.items():
    if c["err"]:
        continue
    seed = [i for i in g.by_qname[q] if i not in g.test][0]
    t0 = time.time(); view = g.review(q); vt = time.time() - t0
    near = {seed} | set(g.interface_seeds(seed))
    frontier = set(near)
    for _ in range(2):
        nxt = set()
        for n in frontier:
            for r in CODE_REL:
                for d, _ in g.out[n][r] + g.inn[n][r]:
                    if d not in g.test and d not in near:
                        nxt.add(d)
            for r in ("uses_type", "instantiates"):
                nxt.update(d for d, _ in g.out[n][r])
        near |= nxt; frontier = nxt
    nearkeys = {key(n) for n in near} | {f"{g.node[seed][2]}:{g.node[seed][3]}"}
    es = c["edges"] or []
    test_e = sum(1 for e in es if is_test(e.split("|")[0].rsplit(":", 1)[0]) or is_test(e.split("|")[1].rsplit(":", 1)[0]))
    unrelated = sum(1 for e in es if not (e.split("|")[0] in nearkeys and e.split("|")[1] in nearkeys))
    rows.append({"q": q, "cur_edges": len(es), "cur_ms": c["ms"], "test_edges": test_e, "outside_code_neighbourhood": unrelated,
                 "view_ms": round(vt * 1000, 2), "view_direct_callers": len(view["code"]["callers"]["direct_callers"]),
                 "view_test_callers": view["tests"]["test_callers"], "view_commits": view["history"]["commits_changing_seed"],
                 "view_bytes": len(json.dumps(view))})
tot = lambda k: sum(r[k] for r in rows)
summary = {"seeds": len(rows), "load_seconds": g.load_seconds,
  "current": {"edges": tot("cur_edges"), "test_touching": tot("test_edges"), "outside_code_neighbourhood": tot("outside_code_neighbourhood"),
              "ms_total": tot("cur_ms"), "ms_p50": statistics.median(r["cur_ms"] for r in rows), "ms_max": max(r["cur_ms"] for r in rows)},
  "proposed": {"ms_total": round(tot("view_ms"), 1), "ms_p50": statistics.median(r["view_ms"] for r in rows), "ms_max": max(r["view_ms"] for r in rows),
               "bytes_p50": statistics.median(r["view_bytes"] for r in rows), "bytes_max": max(r["view_bytes"] for r in rows),
               "seeds_with_history": sum(1 for r in rows if r["view_commits"]), "seeds_with_test_callers": sum(1 for r in rows if r["view_test_callers"])}}
json.dump({"summary": summary, "rows": rows}, open(f"{A}/compare.json", "w"), indent=1)
print(json.dumps(summary, indent=1))
worst = sorted(rows, key=lambda r: -r["outside_code_neighbourhood"])[:5]
for r in worst: print(r)
