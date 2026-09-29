#!/usr/bin/env python3
"""Probe one MCP process before and after an external dataset swap.

The control directory receives ready.json, then waits for a file named next.
It writes after.json and exits. The shell can swap current between markers.
"""

import json
import pathlib
import select
import subprocess
import sys
import time


def request(proc, message, request_id, timeout=20):
    proc.stdin.write(json.dumps(message, separators=(",", ":")) + "\n")
    proc.stdin.flush()
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        if proc.poll() is not None:
            raise RuntimeError(f"MCP process exited {proc.returncode}")
        if not select.select([proc.stdout], [], [], 0.25)[0]:
            continue
        line = proc.stdout.readline()
        if not line:
            raise RuntimeError("MCP stream ended")
        response = json.loads(line)
        if response.get("id") == request_id:
            if "error" in response:
                raise RuntimeError(response["error"])
            return response["result"]
    raise TimeoutError(f"MCP request {request_id} timed out")


def tool(proc, request_id, name, arguments):
    result = request(
        proc,
        {"jsonrpc": "2.0", "id": request_id, "method": "tools/call",
         "params": {"name": name, "arguments": arguments}},
        request_id,
    )
    return result.get("structuredContent") or result.get("structured_content") or result


def snapshot(proc, health_id, pack_id):
    health = tool(proc, health_id, "cks.ops.health", {})
    pack = tool(proc, pack_id, "cks.context.get_for_task",
                {"prompt": "Where is the Alpha function implemented?"})
    return {
        "commit": health["alignment"]["src_commit"],
        "serviceable": health["serviceable"],
        "citation_commits": sorted({c["commit_hash"] for c in pack["citations"]}),
        "citation_files": sorted({c["file"] for c in pack["citations"]}),
    }


def main():
    binary, config, control = sys.argv[1:4]
    control = pathlib.Path(control)
    control.mkdir(parents=True, exist_ok=True)
    with (control / "server.log").open("w") as log:
        proc = subprocess.Popen(
            [binary, "mcp", "--config", config],
            stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=log,
            text=True, bufsize=1,
        )
        try:
            request(proc, {"jsonrpc": "2.0", "id": 1, "method": "initialize",
                           "params": {"protocolVersion": "2025-03-26", "capabilities": {},
                                      "clientInfo": {"name": "wbs-probe", "version": "1"}}}, 1)
            proc.stdin.write('{"jsonrpc":"2.0","method":"notifications/initialized"}\n')
            proc.stdin.flush()
            (control / "ready.json").write_text(json.dumps(snapshot(proc, 2, 3)))
            deadline = time.monotonic() + 60
            while not (control / "next").exists():
                if time.monotonic() > deadline:
                    raise TimeoutError("waiting for dataset swap")
                time.sleep(0.1)
            (control / "after.json").write_text(json.dumps(snapshot(proc, 4, 5)))
        finally:
            proc.terminate()
            try:
                proc.wait(timeout=5)
            except subprocess.TimeoutExpired:
                proc.kill()
                proc.wait()


if __name__ == "__main__":
    main()
