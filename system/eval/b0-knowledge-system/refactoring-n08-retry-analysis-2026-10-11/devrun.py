"""usage: devrun.py <cks binary> <config> <label>
Query every dev-set prompt once through cks mcp (get_for_task_v2) and score
first-gold rank, Recall@10 and RR@10 against the evidence spans."""
import json, subprocess, sys, statistics
binary, cfg, label = sys.argv[1:4]
qs = json.load(open("devset.json"))
p = subprocess.Popen([binary, "mcp", "--config", cfg], stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL)
def send(o): p.stdin.write((json.dumps(o) + "\n").encode()); p.stdin.flush()
def recv():
    while True:
        m = json.loads(p.stdout.readline())
        if "id" in m: return m
send({"jsonrpc": "2.0", "id": 0, "method": "initialize", "params": {"protocolVersion": "2024-11-05", "capabilities": {}, "clientInfo": {"name": "dev", "version": "0"}}}); recv()
send({"jsonrpc": "2.0", "method": "notifications/initialized"})
rows = []
for i, q in enumerate(qs, 1):
    send({"jsonrpc": "2.0", "id": i, "method": "tools/call", "params": {"name": "cks.context.get_for_task_v2", "arguments": {"prompt": q["prompt"]}}})
    sc = recv()["result"].get("structuredContent", {})
    cits = [(c["file"], c["start_line"], c["end_line"]) for c in sc.get("citations", [])]
    hit = lambda c, e: c[0] == e["path"] and c[1] <= e["last"] and c[2] >= e["first"]
    rank = next((k for k, c in enumerate(cits[:10], 1) if any(hit(c, e) for e in q["evidence"])), None)
    recall = sum(any(hit(c, e) for c in cits[:10]) for e in q["evidence"]) / len(q["evidence"])
    rows.append({"id": q["id"], "group": q["group"], "rank": rank, "rr": 1 / rank if rank else 0.0, "recall": recall, "citations": [f"{a}:{b}-{c}" for a, b, c in cits]})
p.stdin.close(); p.wait(timeout=60)
json.dump(rows, open(f"devrun-{label}.json", "w"), indent=1)
print(label, "recall", round(statistics.mean(r["recall"] for r in rows), 3), "mrr", round(statistics.mean(r["rr"] for r in rows), 3))
