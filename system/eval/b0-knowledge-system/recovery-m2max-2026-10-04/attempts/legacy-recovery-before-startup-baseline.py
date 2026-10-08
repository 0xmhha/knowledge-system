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
ROOT = Path('/Users/kevin/work/github/0xmhha/auto-coding/knowledge/knowledge-system')
CURRENT = Path('/private/tmp/ks-translated-source-20261004/darwin/unpacked/knowledge-system-darwin-arm64-7b5d7bb89618-dirty-preview')


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
        os.symlink(CURRENT / name, scratch / f"new-{name}")

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
        def snapshot(path):
            values, transient = {}, {}
            for f in sorted(path.rglob("*")):
                if f.is_file():
                    name = str(f.relative_to(path))
                    entry = {"bytes": f.stat().st_size, "sha256": hashlib.sha256(f.read_bytes()).hexdigest(), "executable": bool(f.stat().st_mode & 0o111)}
                    if name in {"vector/vector.db-wal", "vector/vector.db-shm"}:
                        if name.endswith("-wal"):
                            assert entry["bytes"] == 0
                        transient[name] = entry
                    else:
                        values[name] = entry
            return values, transient
        previous = (dataset / "current").resolve()
        before, runtime_before = snapshot(previous)
        backup = scratch / "legacy-backup"
        import shutil
        shutil.copytree(previous, backup)
        assert snapshot(backup)[0] == before
        setup_config = scratch / "new-setup.yaml"
        run([str(CURRENT / "cks"), "init", "--src", str(source), "--dataset", str(dataset), "--config-out", str(setup_config), "--embedder", "mock"], output=scratch / "new-init.log")
        run([str(CURRENT / "cks"), "setup", "--config", str(setup_config), "--version", "reindexed"], output=scratch / "new-reindex.log")
        assert os.readlink(dataset / "current") == "reindexed"
        assert snapshot(previous)[0] == before
        rebuilt_config = scratch / "rebuilt-mcp.yaml"
        run([str(CURRENT / "cks"), "mcp", "gen-config", "--dataset-dir", str(dataset / "current"), "--source-root", str(source), "--sanitize-rules", str(CURRENT / "policies/sanitization_rules.yaml"), "--out", str(rebuilt_config)], output=scratch / "rebuilt-config.log")
        prepare_config(rebuilt_config)
        raw = rebuilt_config.read_text().replace('provider: ""', 'provider: mock').replace('embed_model: bge-m3', 'embed_model: mock-feature-hash-v1')
        rebuilt_config.write_text(raw)
        requests = scratch / "requests.json"
        requests.write_text(json.dumps({"schema_version": 1, "requests": [{"id": "rebuilt-v2", "tool": "cks.context.get_for_task_v2", "arguments": {"prompt": "Where is Alpha implemented?"}}]}) + "\n")
        run([str(CURRENT / "cks"), "eval", "capture", "--requests", str(requests), "--config", str(rebuilt_config), "--output", str(scratch / "rebuilt-capture.json"), "--warmup", "0", "--retrieval-runs", "1", "--warm-runs", "0", "--cold-runs", "0", "--call-timeout", "30s"], output=scratch / "rebuilt-capture.log")
        report = json.loads((scratch / "rebuilt-capture.json").read_text())
        assert report["state"] == "captured" and len(report["rows"]) == 1
        pack = report["rows"][0]["call"]["response"]["structuredContent"]
        assert pack["format_version"] == 2 and pack["citations"]
        import copy
        canonical = copy.deepcopy(pack)
        expected_hash = canonical["metadata"].pop("integrity_hash")
        assert hashlib.sha256(json.dumps(canonical, sort_keys=True, ensure_ascii=False, separators=(",", ":")).encode()).hexdigest() == expected_hash
        version = dataset / "reindexed"
        identity = json.loads((version / "dataset-identity.json").read_text())
        for c in pack["citations"]:
            source_bytes = (version / "sources/blobs" / c["file_sha256"]).read_bytes()
            assert c["dataset_id"] == identity["dataset_id"] and c["snapshot_id"] == identity["source"]["snapshot_id"] and c["commit_hash"] == identity["source"]["source_commit"]
            assert hashlib.sha256(source_bytes).hexdigest() == c["file_sha256"]
            assert hashlib.sha256(b"".join(source_bytes.splitlines(keepends=True)[c["start_line"] - 1:c["end_line"]])).hexdigest() == c["content_sha256"]
        run([str(CURRENT / "cks"), "rollback", previous.name, "--config", str(setup_config)], output=scratch / "legacy-rollback.log")
        assert os.readlink(dataset / "current") == previous.name and snapshot(previous)[0] == before
        run(["python3", str(probe), str(old / "cks"), str(scratch / "old-mcp.yaml"), "--once", "Where is Alpha implemented?"], output=scratch / "restored-old-v1.json")
        restored = json.loads((scratch / "restored-old-v1.json").read_text())
        assert restored == old_result and snapshot(previous)[0] == before and snapshot(backup)[0] == before
        run([str(CURRENT / "cks"), "doctor", "--src", str(source), "--dataset", str(dataset)], output=scratch / "restored-new-doctor.json")
        restored_doctor = json.loads((scratch / "restored-new-doctor.json").read_text())
        assert restored_doctor["identity_status"] == "legacy_unpinned" and restored_doctor["reindex_required"]
        proof = {"scope": "pre-WBS v1 old data preservation/reindex/rollback diagnostic; synthetic HTTP vectors then mock reindex; no actual BGE quality or operating release", "baseline_commit": BASELINE, "old_version": previous.name, "original_payload_before": before, "original_payload_after": snapshot(previous)[0], "backup_payload_after": snapshot(backup)[0], "runtime_before": runtime_before, "runtime_after": snapshot(previous)[1], "old_v1_before": old_result, "old_v1_after": restored, "new_v2_citations": len(pack["citations"]), "new_dataset_id": identity["dataset_id"], "new_v2_integrity_and_source_sha": True, "quality_metrics": None, "binary_sha256": {name: hashlib.sha256((CURRENT / name).read_bytes()).hexdigest() for name in ["cks", "ckg", "ckv"]}}
        (scratch / "verification.json").write_text(json.dumps(proof, indent=2) + "\n")

    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=5)
    print(f"Legacy v1 replay and v2 migration smoke passed: {scratch}")


if __name__ == "__main__":
    main()
