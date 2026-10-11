#!/usr/bin/env python3
"""Prototype of the proposed review traversal over a CKG graph.db.

For one seed function it builds three separate views instead of one mixed
neighbourhood walk:

  code     production code only (tests excluded), the way a reviewer reads a
           function: inputs, outputs, internal logic, then callers back to
           entry points; plus dynamic paths (interface dispatch, goroutines,
           channels, locks, timeout/cancellation).
  history  depth-1 lookups only: the commits and PRs that changed the seed.
           A commit is never traversed onward, because one commit links
           thousands of nodes (the cause of the subgraph blow-up).
  tests    names and count of test functions that call the seed; not walked.

Efficiency pattern: the test/production partition and the per-type adjacency
are materialised once at load time (one pass over the edge table), so every
view is a lookup or a bounded walk over production nodes only.

usage: review_view.py <graph.db> <source-root> <seed-qname> [...]
"""
import collections, json, sqlite3, sys, time

CODE_REL = ("calls", "invokes")
TEST_SEGMENTS = {"test", "tests", "testdata", "testutil", "testutils", "testhelpers"}


def is_test(path):
    """Port of pkg/system/testpath.IsTest."""
    if not path:
        return False
    base = path.rsplit("/", 1)[-1]
    if base.endswith("_test.go") or (base.endswith(".go") and base.startswith(("testutil", "testhelper"))):
        return True
    if base.endswith((".test.ts", ".test.tsx", ".spec.ts", ".spec.tsx", ".test.js", ".test.jsx", ".spec.js", ".spec.jsx", ".t.sol")):
        return True
    return any(seg in TEST_SEGMENTS for seg in path.split("/"))


class Graph:
    def __init__(self, db_path, src_root):
        self.db = sqlite3.connect(f"file:{db_path}?mode=ro&immutable=1", uri=True)
        self.src_root = src_root
        self._src = {}
        t0 = time.time()
        self.node = {}
        for nid, typ, qn, fp, sl, el, sb, eb, sig in self.db.execute(
                "select id,type,qualified_name,file_path,start_line,end_line,start_byte,end_byte,coalesce(signature,'') from nodes"):
            self.node[nid] = (typ, qn, fp, sl, el, sb, eb, sig)
        self.by_qname = collections.defaultdict(list)
        for nid, n in self.node.items():
            if n[0] in ("Function", "Method", "Struct", "Interface"):
                self.by_qname[n[1]].append(nid)
        self.test = {nid for nid, n in self.node.items() if is_test(n[2])}
        self.out = collections.defaultdict(lambda: collections.defaultdict(list))
        self.inn = collections.defaultdict(lambda: collections.defaultdict(list))
        for s, d, typ, line in self.db.execute("select src,dst,type,coalesce(line,0) from edges"):
            self.out[s][typ].append((d, line))
            self.inn[d][typ].append((s, line))
        self.load_seconds = round(time.time() - t0, 2)

    def q(self, nid):
        return self.node[nid][1]

    def loc(self, nid):
        n = self.node[nid]
        return f"{n[2]}:{n[3]}"

    def text(self, nid):
        n = self.node[nid]
        if n[2] not in self._src:
            try:
                self._src[n[2]] = open(f"{self.src_root}/{n[2]}", "rb").read()
            except OSError:
                self._src[n[2]] = b""
        return self._src[n[2]][n[5]:n[6]].decode("utf-8", "replace")

    # --- code view -------------------------------------------------------
    def data_in_out(self, seed):
        return {
            "signature": self.node[seed][7],
            "uses_types": sorted({self.q(d) for d, _ in self.out[seed]["uses_type"]}),
            "instantiates": sorted({self.q(d) for d, _ in self.out[seed]["instantiates"]}),
            "writes_fields": sorted({self.q(d) for d, _ in self.out[seed]["writes_field"]}),
            "returns": sum(1 for d, _ in self.out[seed]["contains"] if self.node[d][0] == "ReturnStmt"),
        }

    def logic(self, seed, limit=40):
        steps, control = [], collections.Counter()
        for d, _ in self.out[seed]["contains"]:
            typ = self.node[d][0]
            if typ == "CallSite":
                steps.append((self.node[d][5], self.node[d][3], self.text(d).split("\n")[0][:110]))
            elif typ in ("IfStmt", "LoopStmt", "SwitchStmt"):
                control[typ] += 1
        steps.sort()
        internal = [f"{self.q(d)} @{line}" for d, line in sorted(
            [x for r in CODE_REL for x in self.out[seed][r] if x[0] not in self.test], key=lambda x: x[1])]
        return {"control": dict(control), "call_sites_in_order": [f"L{l} {t}" for _, l, t in steps[:limit]],
                "call_sites_total": len(steps), "project_callees": internal}

    def interface_seeds(self, seed):
        """Concrete method T.M -> interface methods I.M for each I that T implements."""
        qn = self.q(seed)
        parts = qn.rsplit(".", 1)
        if len(parts) != 2:
            return []
        owner, method = parts
        out = []
        for oid in self.by_qname.get(owner, []):
            for iface, _ in self.out[oid]["implements"]:
                for mid in self.by_qname.get(self.q(iface) + "." + method, []):
                    out.append(mid)
        return out

    def callers(self, seed, depth=5, max_paths=12):
        """Production-only reverse walk to entry points (no production caller)."""
        starts = [seed] + self.interface_seeds(seed)
        prev = {s: None for s in starts}
        frontier, roots, visited_edges = list(starts), [], 0
        for _ in range(depth):
            nxt = []
            for n in frontier:
                found = False
                for r in CODE_REL:
                    for c, _ in self.inn[n][r]:
                        visited_edges += 1
                        if c in self.test or self.node[c][0] not in ("Function", "Method"):
                            continue
                        found = True
                        if c not in prev:
                            prev[c] = n
                            nxt.append(c)
                if not found and n not in starts:
                    roots.append(n)
            frontier = nxt
            if not frontier:
                break
        roots += frontier  # depth limit reached: treat the edge of the walk as roots

        def path(n):
            p = []
            while n is not None:
                p.append(self.q(n))
                n = prev[n]
            return " -> ".join(p)
        direct = sorted({self.q(c) for s in starts for r in CODE_REL for c, _ in self.inn[s][r] if c not in self.test})
        return {"via_interfaces": [self.q(s) for s in starts[1:]], "direct_callers": direct,
                "paths_from_entry_points": [path(r) for r in roots[:max_paths]], "entry_points": len(roots),
                "production_callers_seen": len(prev) - len(starts), "edges_visited": visited_edges}

    def dynamic(self, seed):
        gor = [d for d, _ in self.out[seed]["spawns"]]
        chans = lambda nid, t: sorted({self.q(d) for d, _ in self.out[nid][t]})
        impls = {}
        for d, _ in [x for r in CODE_REL for x in self.out[seed][r]]:
            if self.node[d][0] == "Method" and d not in self.test:
                owner = self.q(d).rsplit(".", 1)[0]
                for oid in self.by_qname.get(owner, []):
                    if self.node[oid][0] == "Interface":
                        impls[self.q(d)] = sorted({self.q(s) for s, _ in self.inn[oid]["implements"] if s not in self.test})
        return {
            "spawns_goroutines": [{"at": self.loc(g), "sends_to": chans(g, "sends_to"), "recvs_from": chans(g, "recvs_from")} for g in gor],
            "sends_to": chans(seed, "sends_to"), "recvs_from": chans(seed, "recvs_from"),
            "acquires_locks": chans(seed, "acquires_lock"), "releases_locks": chans(seed, "releases_lock"),
            "timeout_paths": chans(seed, "timeout_path"), "cancellation_paths": chans(seed, "cancellation_path"),
            "interface_calls_and_implementations": impls,
        }

    # --- history view ----------------------------------------------------
    def history(self, seed, limit=10):
        commits = {}
        for hunk, _ in self.inn[seed]["modifies"]:
            for c, _ in self.inn[hunk]["has_hunk"]:
                sig = self.node[c][7]
                ts, _, subject = sig.partition(": ")
                commits[c] = (int(ts) if ts.isdigit() else 0, self.q(c).removeprefix("commit:")[:10], subject[:100])
        prs = self.db.execute("select number, title, merged_at from node_prs where node_id=? order by merged_at desc limit ?", (seed, limit)).fetchall()
        rows = sorted(commits.values(), reverse=True)
        return {"commits_changing_seed": len(rows), "recent": [f"{c} {s}" for _, c, s in rows[:limit]],
                "prs": [f"#{n} {t} ({m[:10]})" for n, t, m in prs]}

    # --- tests view ------------------------------------------------------
    def tests(self, seed, limit=10):
        callers = sorted({self.q(c) for r in CODE_REL for c, _ in self.inn[seed][r] if c in self.test})
        return {"test_callers": len(callers), "examples": callers[:limit]}

    def review(self, qname):
        ids = [i for i in self.by_qname.get(qname, []) if i not in self.test]
        if not ids:
            return {"seed": qname, "error": "not found among production functions"}
        seed = ids[0]
        t0 = time.time()
        out = {"seed": qname, "at": self.loc(seed),
               "code": {"data": self.data_in_out(seed), "logic": self.logic(seed),
                        "callers": self.callers(seed), "dynamic": self.dynamic(seed)},
               "history": self.history(seed), "tests": self.tests(seed)}
        h, tst = out["history"], out["tests"]
        out["available"] = (f"history: {h['commits_changing_seed']} commits, {len(h['prs'])} PRs (change_history); "
                            f"tests: {tst['test_callers']} test callers (find_callers with tests)")
        out["seconds"] = round(time.time() - t0, 4)
        return out


if __name__ == "__main__":
    g = Graph(sys.argv[1], sys.argv[2])
    print(json.dumps({"load_seconds": g.load_seconds, "views": [g.review(q) for q in sys.argv[3:]]}, ensure_ascii=False, indent=1))
