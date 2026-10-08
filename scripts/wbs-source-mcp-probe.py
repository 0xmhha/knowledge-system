#!/usr/bin/env python3
"""Check non-Git v2 archived evidence and legacy-tool refusal over real MCP."""

import hashlib
import json
import pathlib
import select
import subprocess
import sys
import time


def call(proc, request_id, name, arguments):
    proc.stdin.write(json.dumps({"jsonrpc": "2.0", "id": request_id,
        "method": "tools/call", "params": {"name": name, "arguments": arguments}}) + "\n")
    proc.stdin.flush()
    deadline = time.monotonic() + 20
    while time.monotonic() < deadline:
        if not select.select([proc.stdout], [], [], 0.25)[0]:
            if proc.poll() is not None:
                raise RuntimeError(f"MCP exited {proc.returncode}")
            continue
        reply = json.loads(proc.stdout.readline())
        if reply.get("id") == request_id:
            return reply["result"]
    raise TimeoutError(name)


def main():
    binary, config, log_path = sys.argv[1:4]
    with pathlib.Path(log_path).open("w") as log:
        proc = subprocess.Popen([binary, "mcp", "--config", config],
            stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=log,
            text=True, bufsize=1)
        try:
            proc.stdin.write(json.dumps({"jsonrpc": "2.0", "id": 1,
                "method": "initialize", "params": {"protocolVersion": "2025-03-26",
                "capabilities": {}, "clientInfo": {"name": "source-probe", "version": "1"}}}) + "\n")
            proc.stdin.flush()
            deadline = time.monotonic() + 20
            while time.monotonic() < deadline:
                if select.select([proc.stdout], [], [], 0.25)[0]:
                    if json.loads(proc.stdout.readline()).get("id") == 1:
                        break
            else:
                raise TimeoutError("initialize")
            proc.stdin.write('{"jsonrpc":"2.0","method":"notifications/initialized"}\n')
            proc.stdin.flush()
            for request_id, name, args in (
                (2, "cks.context.get_for_task", {"prompt": "Where is Alpha implemented?"}),
                (3, "cks.context.semantic_search", {"query": "Alpha"}),
                (4, "cks.context.find_symbol", {"name": "Alpha"}),
            ):
                result = call(proc, request_id, name, args)
                assert result.get("isError") is True, (name, result)
                assert "requires_v2" in json.dumps(result), (name, result)
            result = call(proc, 5, "cks.context.get_for_task_v2",
                {"prompt": "Where is Alpha implemented?"})
            assert not result.get("isError"), result
            pack = result.get("structuredContent") or result.get("structured_content")
            assert pack and pack["format_version"] == 2 and pack["citations"], pack
            assert pack["coordinates"]["source_mode"] == "snapshot-only"
            assert pack["coordinates"]["base_commit"] == ""
            assert any("Alpha" in body["text"] for body in pack["bodies"]), pack
            assert all("Zeta" not in body["text"] for body in pack["bodies"]), pack
            stamped = json.loads(json.dumps(pack))
            digest = stamped["metadata"].pop("integrity_hash")
            canonical = json.dumps(stamped, sort_keys=True, separators=(",", ":"),
                ensure_ascii=False).encode()
            assert hashlib.sha256(canonical).hexdigest() == digest
        finally:
            proc.terminate()
            try:
                proc.wait(timeout=5)
            except subprocess.TimeoutExpired:
                proc.kill()
                proc.wait()


if __name__ == "__main__":
    main()
