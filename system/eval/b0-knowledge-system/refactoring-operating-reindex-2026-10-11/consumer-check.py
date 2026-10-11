#!/usr/bin/env python3
"""Query the operating go-stablenet dataset the way a coding agent does: a
fresh `cks mcp` stdio server, initialize, then cks.context.get_for_task_v2.
Usage: consumer-check.py <release> <label>. Smoke check only: no gold answers,
so this measures serving, citation provenance and latency, not answer quality.
Copied from the N15 check; adds cks.context.review_function calls (schema 1.24)."""
import json, subprocess, sys, time
from pathlib import Path
P = Path(__file__).resolve().parent
OP = Path("/Users/kevin/work/github/0xmhha/auto-coding/knowledge-data/ks-operating")
release, label = sys.argv[1], sys.argv[2]
cks = str(OP / "releases" / release / "bin" / "cks")
cfg = OP / "cks-operating-stdio.yaml"
QUERIES = ["블록 헤더 검증은 어디서 하나요?",
           "Where are system contracts deployed at genesis?",
           "트랜잭션 풀에서 수수료가 부족한 트랜잭션은 어디서 거부되나요?",
           "How does the consensus engine choose the block proposer?",
           "NativeCoinAdapter의 mint 권한은 어떻게 검사하나요?"]
REVIEWS = ["snap.Syncer.assignTrienodeHealTasks", "types.Block.Hash", "rlp.EncodeToBytes"]
def rpc(proc, msg):
    proc.stdin.write(json.dumps(msg) + "\n"); proc.stdin.flush()
    if "id" not in msg:
        return None
    while True:
        line = proc.stdout.readline()
        if not line:
            raise RuntimeError("server closed: " + proc.stderr.read()[-400:])
        obj = json.loads(line)
        if obj.get("id") == msg["id"]:
            return obj
rows = []
start = time.monotonic()
proc = subprocess.Popen([cks, "mcp", "--config", str(cfg)], stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
try:
    init = rpc(proc, {"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {"protocolVersion": "2025-06-18", "capabilities": {}, "clientInfo": {"name": "n15-consumer-check", "version": "1"}}})
    first_ready = time.monotonic() - start
    rpc(proc, {"jsonrpc": "2.0", "method": "notifications/initialized"})
    for i, q in enumerate(QUERIES, 2):
        t = time.monotonic()
        res = rpc(proc, {"jsonrpc": "2.0", "id": i, "method": "tools/call", "params": {"name": "cks.context.get_for_task_v2", "arguments": {"prompt": q}}})
        r = res.get("result", {}); sc = r.get("structuredContent") or {}
        cites = sc.get("citations") or []
        rows.append({"query": q, "elapsed_ms": round((time.monotonic() - t) * 1000), "is_error": bool(r.get("isError")),
                     "error": None if not r.get("isError") else sc, "citations": len(cites),
                     "files": [c["file"] + ":" + str(c["start_line"]) + "-" + str(c["end_line"]) for c in cites[:5]],
                     "base_commit": (sc.get("coordinates") or {}).get("base_commit"), "dataset_id": (sc.get("coordinates") or {}).get("dataset_id")})
    reviews = []
    for i, sym in enumerate(REVIEWS, 100):
        t = time.monotonic()
        res = rpc(proc, {"jsonrpc": "2.0", "id": i, "method": "tools/call", "params": {"name": "cks.context.review_function", "arguments": {"symbol": sym}}})
        r = res.get("result", {}); sc = r.get("structuredContent") or {}
        code = sc.get("code") or {}
        reviews.append({"symbol": sym, "elapsed_ms": round((time.monotonic() - t) * 1000), "is_error": bool(r.get("isError")),
                        "error": None if not r.get("isError") else (r.get("content") or [{}])[0].get("text"),
                        "callers": len(code.get("callers") or []), "caller_paths": len(code.get("caller_paths") or []),
                        "caller_walk_truncated": code.get("caller_walk_truncated"), "call_sites": code.get("call_sites_total"),
                        "commits": (sc.get("history") or {}).get("commits_total"), "test_callers": (sc.get("tests") or {}).get("callers_total"),
                        "available": sc.get("available")})
finally:
    proc.stdin.close(); proc.wait(timeout=30)
out = {"release": release, "label": label, "server_initialize_s": round(first_ready, 2), "server_info": init.get("result", {}).get("serverInfo"), "queries": rows, "reviews": reviews}
(P / ("consumer-" + label + ".json")).write_text(json.dumps(out, ensure_ascii=False, indent=2) + "\n")
print(json.dumps({"label": label, "init_s": out["server_initialize_s"], "results": [(r["citations"], r["elapsed_ms"], r["is_error"]) for r in rows],
                  "reviews": [(r["symbol"], r["elapsed_ms"], r["is_error"], r["callers"], r["commits"]) for r in reviews]}, ensure_ascii=False))
