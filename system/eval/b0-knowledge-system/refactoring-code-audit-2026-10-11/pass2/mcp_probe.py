import json, subprocess, sys
binary, graph = sys.argv[1], sys.argv[2]
calls = json.loads(sys.argv[3])
p = subprocess.Popen([binary, "mcp", "--graph", graph], stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
def send(o): p.stdin.write((json.dumps(o)+"\n").encode()); p.stdin.flush()
def recv():
    while True:
        line = p.stdout.readline()
        if not line: return None
        m = json.loads(line)
        if "id" in m: return m
send({"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"p","version":"0"}}}); recv()
send({"jsonrpc":"2.0","method":"notifications/initialized"})
send({"jsonrpc":"2.0","id":1,"method":"tools/list"}); tools=[t["name"] for t in recv()["result"]["tools"]]
print("tools:", tools)
for i,(name,args) in enumerate(calls, 2):
    import time; t0=time.time(); send({"jsonrpc":"2.0","id":i,"method":"tools/call","params":{"name":name,"arguments":args}})
    r = recv()
    print(name, args, f"{time.time()-t0:.2f}s", "->", "PROCESS EXITED" if r is None else json.dumps(r)[:160])
    if r is None:
        p.wait(); print("exit code", p.returncode, "stderr tail:", p.stderr.read().decode()[-300:]); break
else:
    p.stdin.close(); p.wait()
