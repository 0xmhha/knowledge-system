import json, subprocess, sys, time
binary, cfg, cwd = sys.argv[1:4]
p = subprocess.Popen([binary, "mcp", "--config", cfg], cwd=cwd, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
def send(o): p.stdin.write((json.dumps(o) + "\n").encode()); p.stdin.flush()
def recv():
    while True:
        line = p.stdout.readline()
        if not line: return None
        m = json.loads(line)
        if "id" in m: return m
t0 = time.monotonic()
send({"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {"protocolVersion": "2024-11-05", "capabilities": {}, "clientInfo": {"name": "probe", "version": "0"}}}); recv()
send({"jsonrpc": "2.0", "method": "notifications/initialized"})
t1 = time.monotonic()
send({"jsonrpc": "2.0", "id": 2, "method": "tools/call", "params": {"name": "cks.context.get_for_task_v2", "arguments": {"prompt": "벡터 재색인 체크포인트는 어떻게 읽히나요?", "knowledge_as_of": "2026-10-09", "knowledge_subsystem": "knowledge-system"}}})
r = recv(); t2 = time.monotonic()
p.stdin.close(); t3 = time.monotonic()
try:
    p.wait(timeout=60)
except subprocess.TimeoutExpired:
    p.kill(); p.wait()
t4 = time.monotonic()
print(json.dumps({"cfg": cfg.rsplit("/", 1)[1], "init_s": round(t1 - t0, 2), "call_s": round(t2 - t1, 2), "exit_after_stdin_close_s": round(t4 - t3, 2), "rc": p.returncode, "error": bool(r and r.get("error"))}))
