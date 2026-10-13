"""Talk to a freshly started `cks mcp` over stdio, the way a coding agent does."""
from __future__ import annotations

import json
import queue
import re
import subprocess
import threading
import time
from pathlib import Path
from typing import Any, Dict, Optional


def stdio_variant(http_config: str) -> str:
    """Rewrite a gen-config YAML so the server speaks MCP over stdio.

    gen-config only writes HTTP listeners; the listen block is flat and
    machine-written, so two anchored line substitutions are enough.
    """
    text, n1 = re.subn(r"(?m)^(\s+)mcp_stdio: false$", r"\1mcp_stdio: true", http_config)
    text, n2 = re.subn(r"(?m)^(\s+)transport: http$", r"\1transport: stdio", text)
    if n1 != 1 or n2 != 1:
        raise ValueError("unexpected cks config layout: listen block not found")
    return text


class StdioClient:
    def __init__(self, cks: Path, config: Path, timeout: float = 120.0) -> None:
        self.timeout = timeout
        self.proc = subprocess.Popen([str(cks), "mcp", "--config", str(config)], stdin=subprocess.PIPE,
                                     stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
        self.lines: "queue.Queue[Optional[str]]" = queue.Queue()
        self.stderr: list = []
        threading.Thread(target=self._pump, daemon=True).start()
        threading.Thread(target=self._drain, daemon=True).start()
        self.next_id = 1

    def _pump(self) -> None:
        assert self.proc.stdout is not None
        for line in self.proc.stdout:
            self.lines.put(line)
        self.lines.put(None)

    def _drain(self) -> None:
        assert self.proc.stderr is not None
        for line in self.proc.stderr:
            self.stderr.append(line)

    def _send(self, msg: Dict[str, Any]) -> None:
        assert self.proc.stdin is not None
        self.proc.stdin.write(json.dumps(msg) + "\n")
        self.proc.stdin.flush()

    def request(self, method: str, params: Dict[str, Any]) -> Dict[str, Any]:
        ident = self.next_id
        self.next_id += 1
        self._send({"jsonrpc": "2.0", "id": ident, "method": method, "params": params})
        while True:
            try:
                line = self.lines.get(timeout=self.timeout)
            except queue.Empty:
                raise TimeoutError(f"no answer to {method} within {self.timeout:.0f}s") from None
            if line is None:
                tail = "".join(self.stderr[-5:]).strip()
                raise RuntimeError(f"server exited: {tail or 'no output'}")
            try:
                obj = json.loads(line)
            except ValueError:
                continue
            if obj.get("id") == ident:
                if "error" in obj:
                    raise RuntimeError(f"{method}: {obj['error'].get('message', obj['error'])}")
                return obj.get("result", {})

    def initialize(self) -> Dict[str, Any]:
        result = self.request("initialize", {"protocolVersion": "2025-06-18", "capabilities": {},
                                             "clientInfo": {"name": "ks-setup", "version": "1"}})
        self._send({"jsonrpc": "2.0", "method": "notifications/initialized"})
        return result

    def call_raw(self, tool: str, arguments: Dict[str, Any]) -> Dict[str, Any]:
        return self.request("tools/call", {"name": tool, "arguments": arguments})

    def call(self, tool: str, arguments: Dict[str, Any]) -> Dict[str, Any]:
        result = self.request("tools/call", {"name": tool, "arguments": arguments})
        return result.get("structuredContent") or {}

    def list_tools(self) -> list:
        return [t.get("name") for t in self.request("tools/list", {}).get("tools", [])]

    def close(self) -> None:
        try:
            if self.proc.stdin:
                self.proc.stdin.close()
            self.proc.wait(timeout=30)
        except subprocess.TimeoutExpired:
            self.proc.kill()
            self.proc.wait()


def busiest_function(graph_db: Path) -> Optional[str]:
    """The production function with the most production callers: a seed that
    exercises the review index the way an agent's first question would."""
    import sqlite3
    try:
        conn = sqlite3.connect(f"file:{graph_db}?mode=ro&immutable=1", uri=True)
        try:
            row = conn.execute(
                "SELECT n.qualified_name FROM code_edges e JOIN nodes n ON n.id = e.dst "
                "WHERE e.type IN ('calls','invokes') AND n.type IN ('Function','Method') "
                "GROUP BY e.dst ORDER BY count(*) DESC LIMIT 1").fetchone()
        finally:
            conn.close()
    except sqlite3.Error:
        return None
    return row[0] if row else None


def smoke_queries(cks: Path, config: Path, symbol: Optional[str], with_vector: bool) -> Dict[str, Any]:
    """Ask the questions a coding agent asks first and record whether they answer."""
    out: Dict[str, Any] = {}
    client = StdioClient(cks, config)
    try:
        client.initialize()
        if symbol:
            started = time.monotonic()
            r = client.call_raw("cks.context.review_function", {"symbol": symbol})
            sc = r.get("structuredContent") or {}
            code = sc.get("code") or {}
            out["review_function"] = {
                "symbol": symbol, "ok": not r.get("isError"), "ms": round((time.monotonic() - started) * 1000),
                "callers": len(code.get("callers") or []), "call_sites": code.get("call_sites_total"),
                "commits": (sc.get("history") or {}).get("commits_total"),
                "test_callers": (sc.get("tests") or {}).get("callers_total")}
        if with_vector:
            prompt = f"Where is {symbol.rsplit('.', 1)[-1]} implemented and who calls it?" if symbol else "Where is the entry point?"
            started = time.monotonic()
            r = client.call_raw("cks.context.get_for_task_v2", {"prompt": prompt})
            sc = r.get("structuredContent") or {}
            out["get_for_task_v2"] = {"prompt": prompt, "ok": not r.get("isError"),
                                      "ms": round((time.monotonic() - started) * 1000),
                                      "citations": len(sc.get("citations") or [])}
    finally:
        client.close()
    return out


def health(cks: Path, config: Path) -> Dict[str, Any]:
    """Return a compact view of cks.ops.health plus the tool count."""
    client = StdioClient(cks, config)
    try:
        client.initialize()
        tools = client.list_tools()
        h = client.call("cks.ops.health", {})
    finally:
        client.close()
    backends = h.get("backends") or {}
    graph, vector = backends.get("graph") or {}, backends.get("vector") or {}
    return {
        "status": h.get("status"),
        "serviceable": h.get("serviceable"),
        "tools": len(tools),
        "graph_reachable": graph.get("reachable"),
        "graph_schema": graph.get("schema_version"),
        "indexed_head": graph.get("indexed_head"),
        "vector_reachable": vector.get("reachable"),
        "vector_reason": vector.get("reason"),
    }


def semantic_status(cks: Path, config: Path) -> Dict[str, Any]:
    """Query the pinned semantic readiness tool without model queries or writes."""
    client = StdioClient(cks, config)
    try:
        client.initialize()
        if "cks.semantic.status" not in client.list_tools():
            return {"availability": "unsupported", "reason": "semantic_status_tool_missing"}
        return client.call("cks.semantic.status", {})
    finally:
        client.close()
