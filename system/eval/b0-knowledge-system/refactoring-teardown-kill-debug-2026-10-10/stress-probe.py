import json, subprocess, sys, time, os
binary, capdir, cwd, n = sys.argv[1], sys.argv[2], sys.argv[3], int(sys.argv[4])
arms = ["baseline_off","baseline_on","concept_text_off","concept_text_on","relations_off","relations_on","combined_off","combined_on"]
def start(cfg):
    return subprocess.Popen([binary, "mcp", "--config", cfg], cwd=cwd, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL)
def call(p, i):
    def send(o): p.stdin.write((json.dumps(o) + "\n").encode()); p.stdin.flush()
    def recv():
        while True:
            m = json.loads(p.stdout.readline())
            if "id" in m: return m
    send({"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {"protocolVersion": "2024-11-05", "capabilities": {}, "clientInfo": {"name": "p", "version": "0"}}}); recv()
    send({"jsonrpc": "2.0", "method": "notifications/initialized"})
    send({"jsonrpc": "2.0", "id": 2, "method": "tools/call", "params": {"name": "cks.context.get_for_task_v2", "arguments": {"prompt": "벡터 재색인 체크포인트는 어떻게 읽히나요? %d" % i, "knowledge_as_of": "2026-10-09", "knowledge_subsystem": "knowledge-system"}}}); recv()
warm = [start(f"{capdir}/{a}.yaml") for a in arms]
for i, p in enumerate(warm): call(p, i)
out = []
for i in range(n):
    arm = arms[2 + i % 6]
    p = start(f"{capdir}/{arm}.yaml"); call(p, 100 + i)
    t = time.monotonic(); closed_ms = int(time.time()*1000); p.stdin.close()
    try: p.wait(timeout=60)
    except subprocess.TimeoutExpired: p.kill(); p.wait()
    out.append((arm, round(time.monotonic() - t, 2), p.pid, closed_ms))
for p in warm: p.stdin.close()
for p in warm: p.wait(timeout=60)
print(json.dumps({"load": os.getloadavg(), "cold_exit_s": out, "over_5s": sum(x[1] > 5 for x in out)}))
