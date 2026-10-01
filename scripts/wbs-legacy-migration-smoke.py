#!/usr/bin/env python3
"""Replay a pinned pre-WBS v1 consumer with a local deterministic Ollama fixture."""

import hashlib
import http.server
import io
import json
import os
from pathlib import Path
import subprocess
import tarfile
import tempfile
import threading


BASELINE = "1ded9b3e47bc2e09062dba329c2f918423746fa4"
ROOT = Path(__file__).resolve().parent.parent


class OllamaHandler(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path != "/api/tags":
            self.send_error(404)
            return
        self.respond({"models": [{"name": "bge-m3:latest", "digest": "a" * 64}]})

    def do_POST(self):
        if self.path != "/api/embed":
            self.send_error(404)
            return
        request = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
        vectors = []
        for value in request["input"]:
            vector = [0.0] * 1024
            for index, byte in enumerate(hashlib.sha256(value.encode()).digest()):
                vector[index] = (byte + 1) / 256.0
            vectors.append(vector)
        self.respond({"model": request["model"], "embeddings": vectors})

    def respond(self, value):
        body = json.dumps(value).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, *_args):
        pass


def run(args, *, cwd=ROOT, env=None, output=None):
    if output is None:
        subprocess.run(args, cwd=cwd, env=env, check=True)
        return
    with open(output, "w", encoding="utf-8") as log:
        subprocess.run(args, cwd=cwd, env=env, check=True, stdout=log, stderr=log)


def prepare_config(path):
    content = path.read_text()
    for before, after in (("mcp_stdio: false", "mcp_stdio: true"),
                          ("transport: http", "transport: stdio")):
        assert before in content, before
        content = content.replace(before, after)
    path.write_text(content)


def main():
    scratch = Path(os.environ.get("KS_LEGACY_SMOKE_DIR") or tempfile.mkdtemp(prefix="ks-legacy-"))
    old, source = scratch / "old", scratch / "src"
    old.mkdir(parents=True, exist_ok=True)
    source.mkdir(parents=True, exist_ok=True)
    print(f"Legacy migration fixture: {scratch}", flush=True)
    archive = subprocess.run(["git", "archive", BASELINE], cwd=ROOT, check=True, stdout=subprocess.PIPE).stdout
    with tarfile.open(fileobj=io.BytesIO(archive)) as tar:
        tar.extractall(old)
    for name, command in (("ckg", "./cmd/graph"), ("ckv", "./cmd/vector"), ("cks", "./cmd/cks")):
        run(["go", "build", "-o", str(old / name), command], cwd=old)
    for name, command in (("cks", "./cmd/cks"), ("ckg", "./cmd/graph"), ("ckv", "./cmd/vector")):
        run(["go", "build", "-o", str(scratch / f"new-{name}"), command])

    server = http.server.HTTPServer(("127.0.0.1", 0), OllamaHandler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        endpoint = f"http://127.0.0.1:{server.server_port}"
        (source / "go.mod").write_text("module example.com/fixture\n\ngo 1.25\n")
        (source / "main.go").write_text("package fixture\nfunc Alpha() {}\n")
        (source / "README.md").write_text("# Fixture\nAlpha is a function.\n")
        run(["git", "init", "-q"], cwd=source)
        run(["git", "add", "."], cwd=source)
        run(["git", "-c", "commit.gpgsign=false", "-c", "user.name=Fixture",
             "-c", "user.email=fixture@example.invalid", "commit", "-qm", "fixture"], cwd=source)
        old_env = dict(os.environ, PATH=f"{old}:{os.environ['PATH']}")
        dataset = scratch / "dataset"
        run([str(old / "cks"), "setup", "--src", str(source), "--out", str(dataset),
             "--version", "auto", "--embedder", "ollama", "--model-name", "bge-m3",
             "--ollama-url", endpoint], env=old_env, output=scratch / "old-setup.log")
        for generation, binary in (("old", old / "cks"), ("new", scratch / "new-cks")):
            config = scratch / f"{generation}-mcp.yaml"
            run([str(binary), "mcp", "gen-config", "--dataset-dir", str(dataset / "current"),
                 "--name", f"{generation}-fixture", "--source-root", str(source),
                 "--embed-model", "bge-m3", "--ollama-url", endpoint,
                 "--sanitize-rules", str(ROOT / "system/policies/sanitization_rules.yaml"),
                 "--out", str(config)], output=scratch / f"{generation}-config.log")
            prepare_config(config)
        probe = ROOT / "scripts/wbs-mcp-pin-probe.py"
        run(["python3", str(probe), str(old / "cks"), str(scratch / "old-mcp.yaml"),
             "--once", "Where is Alpha implemented?"], output=scratch / "old-v1.json")
        run([str(scratch / "new-cks"), "doctor", "--src", str(source), "--dataset", str(dataset)],
            output=scratch / "new-doctor.json")
        run(["python3", str(probe), str(scratch / "new-cks"), str(scratch / "new-mcp.yaml"),
             "--v2-error-once", "Where is Alpha implemented?", "reindex_required"],
            output=scratch / "new-v2-error.json")
        old_result = json.loads((scratch / "old-v1.json").read_text())
        doctor = json.loads((scratch / "new-doctor.json").read_text())
        new_result = json.loads((scratch / "new-v2-error.json").read_text())
        assert old_result["serviceable"] and old_result["citation_files"] and len(old_result["citation_commits"]) == 1
        assert doctor["identity_status"] == "legacy_unpinned" and doctor["reindex_required"]
        assert new_result["code"] == "reindex_required"

    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=5)
    print(f"Legacy v1 replay and v2 migration smoke passed: {scratch}")


if __name__ == "__main__":
    main()
