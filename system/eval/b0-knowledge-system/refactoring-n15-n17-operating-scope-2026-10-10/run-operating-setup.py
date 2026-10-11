#!/usr/bin/env python3
"""Operating setup for go-stablenet with one installed release; relay only to local approved Ollama.
Usage: run-operating-setup.py <release> <version>  (run from the knowledge-system repository root)
Writes <version>-* records into this evidence folder. The source repository is read only."""
import hashlib, json, os, subprocess, sys, threading, time, urllib.error, urllib.request
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
P = Path(__file__).resolve().parent
OP = Path("/Users/kevin/work/github/0xmhha/auto-coding/knowledge-data/ks-operating")
SRC = "/Users/kevin/work/github/wemade/go-stablenet"
release, version = sys.argv[1], sys.argv[2]
binroot = OP / "releases" / release / "bin"
records = []; lock = threading.Lock()
class Relay(BaseHTTPRequestHandler):
    def log_message(self, *a): pass
    def do_GET(self): self.forward()
    def do_POST(self): self.forward()
    def forward(self):
        assert self.path in {"/api/tags", "/api/version", "/api/show", "/api/embed"}, self.path
        body = self.rfile.read(int(self.headers.get("Content-Length", "0"))); start = time.monotonic_ns()
        req = urllib.request.Request("http://127.0.0.1:11434" + self.path, data=body if self.command == "POST" else None, method=self.command, headers={"Content-Type": "application/json"})
        row = {"method": self.command, "path": self.path, "request_bytes": len(body)}
        if self.path == "/api/embed":
            value = json.loads(body).get("input", []); row["embedding_inputs"] = len(value) if isinstance(value, list) else 1
        try:
            try:
                with urllib.request.urlopen(req, timeout=600) as r: status, data, kind = r.status, r.read(), r.headers.get("Content-Type", "application/json")
            except urllib.error.HTTPError as e: status, data, kind = e.code, e.read(), e.headers.get("Content-Type", "application/json")
            row.update(status=status, response_bytes=len(data))
            if self.path == "/api/embed" and status == 200:
                obj = json.loads(data); row.update(embedding_outputs=len(obj.get("embeddings", [])), prompt_eval_count=obj.get("prompt_eval_count"))
            self.send_response(status); self.send_header("Content-Type", kind); self.send_header("Content-Length", str(len(data))); self.end_headers(); self.wfile.write(data)
        except Exception as e:
            row["transport_error_type"] = type(e).__name__; self.send_error(502, "local relay transport failure")
        finally:
            row["elapsed_ns"] = time.monotonic_ns() - start
            with lock:
                records.append(row)
server = ThreadingHTTPServer(("127.0.0.1", 0), Relay); threading.Thread(target=server.serve_forever, daemon=True).start()
with urllib.request.urlopen("http://127.0.0.1:11434/api/tags", timeout=10) as r: tags = json.load(r)
chosen = [m for m in tags["models"] if m["name"] == "bge-m3:latest"]
assert len(chosen) == 1 and chosen[0]["digest"] == "7907646426070047a77226ac3e684fbbe8410524f7b4a74d02837e43f2146bab"
head = subprocess.check_output(["git", "-C", SRC, "rev-parse", "HEAD"], text=True).strip()
assert not subprocess.check_output(["git", "-C", SRC, "status", "--porcelain"])
argv = [str(binroot / "cks"), "setup", "--src", SRC, "--out", str(OP / "dataset"), "--project-id", "go-stablenet", "--source-mode", "committed",
        "--version", version, "--graph-bin", str(binroot / "ckg"), "--vector-bin", str(binroot / "ckv"), "--embedder", "ollama",
        "--model-name", "bge-m3:latest", "--embed-dim", "1024", "--ollama-url", "http://127.0.0.1:%d" % server.server_port,
        "--query-prefix-policy", "registry", "--build-timeout", "2h", "--progress", "json"]
(P / (version + "-setup-command.json")).write_text(json.dumps({"argv": argv, "release": release, "source_head": head,
    "environment_overrides": {"CKV_REQUIRE_COMPLETE_EMBEDDINGS": "1"}}, indent=2) + "\n")
start = time.monotonic_ns()
with (P / (version + "-setup.stdout.jsonl")).open("w") as out, (P / (version + "-setup.stderr.txt")).open("w") as err:
    result = subprocess.run(argv, env=dict(os.environ, CKV_REQUIRE_COMPLETE_EMBEDDINGS="1"), stdout=out, stderr=err)
elapsed = time.monotonic_ns() - start
server.shutdown()
embeds = [r for r in records if r["path"] == "/api/embed"]
assert subprocess.check_output(["git", "-C", SRC, "rev-parse", "HEAD"], text=True).strip() == head
res = {"release": release, "version": version, "source_head": head, "exit_code": result.returncode, "elapsed_s": round(elapsed / 1e9, 1),
       "http_attempts": len(records), "embedding_http_attempts": len(embeds), "embedding_inputs": sum(r.get("embedding_inputs", 0) for r in embeds),
       "embedding_outputs": sum(r.get("embedding_outputs", 0) for r in embeds), "embedding_failed": sum(r.get("status") != 200 for r in embeds),
       "embedding_model_ns_total": sum(r["elapsed_ns"] for r in embeds), "source_repository_modified": False}
(P / (version + "-setup-result.json")).write_text(json.dumps(res, indent=2) + "\n"); print(json.dumps(res))
raise SystemExit(result.returncode)
