import json, subprocess, sys
binary, cfgdir = sys.argv[1], sys.argv[2]
reqs = json.load(open("/private/tmp/ks-n11-eval-m13oocru/run-DEV5-n08/capture/requests.json"))["requests"]
out = {}
for v in ("off", "on"):
    p = subprocess.Popen([binary, "mcp", "--config", f"{cfgdir}/{v}.yaml"], cwd=cfgdir, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL)
    def send(o): p.stdin.write((json.dumps(o) + "\n").encode()); p.stdin.flush()
    def recv():
        while True:
            m = json.loads(p.stdout.readline())
            if "id" in m: return m
    send({"jsonrpc": "2.0", "id": 0, "method": "initialize", "params": {"protocolVersion": "2024-11-05", "capabilities": {}, "clientInfo": {"name": "smoke", "version": "0"}}}); recv()
    send({"jsonrpc": "2.0", "method": "notifications/initialized"})
    for i, r in enumerate(reqs, 1):
        send({"jsonrpc": "2.0", "id": i, "method": "tools/call", "params": {"name": r["tool"], "arguments": r["arguments"]}})
        sc = recv()["result"]["structuredContent"]
        out.setdefault(r["id"], {})[v] = [f'{c["file"]}:{c["start_line"]}-{c["end_line"]}' for c in sc["citations"]]
    p.stdin.close(); p.wait(timeout=60)
res = {q: {"same": v["off"] == v["on"], "only_on": [c for c in v["on"] if c not in v["off"]], "off_is_on_minus_inserted": [c for c in v["on"] if c in v["off"]] == v["off"]} for q, v in out.items()}
json.dump({"citations": out, "comparison": res}, open(f"{cfgdir}/smoke-result.json", "w"), indent=1)
for q, r in res.items(): print(q, r)
