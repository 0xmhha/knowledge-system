#!/usr/bin/env python3
"""Probe Ollama's effective embedding input limit using a pinned source blob.

Diagnostic only: it never writes a dataset or scores B0 gold questions.
"""

import argparse
import datetime as dt
import hashlib
import json
from pathlib import Path
import re
import subprocess
import sys
import time
import urllib.error
import urllib.parse
import urllib.request


ROOT = Path(__file__).resolve().parent.parent
DEFAULT_COMMIT = "71cb71cd55960833e930269e272f7a4a060be3aa"
DEFAULT_PATH = "cmd/cks/knowledgecli/knowledge.go"


def read_json(url, payload=None, timeout=5):
    request = urllib.request.Request(url, data=payload,
                                     headers={"Content-Type": "application/json"})
    with urllib.request.urlopen(request, timeout=timeout) as response:
        return json.load(response)


def model_digest(url, exact):
    matches = [m for m in read_json(url + "/api/tags").get("models", [])
               if m.get("name") == exact]
    if len(matches) != 1 or not re.fullmatch(r"[0-9a-f]{64}", matches[0].get("digest", "")):
        raise ValueError("model tag or digest is absent/ambiguous")
    return matches[0]["digest"]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--commit", default=DEFAULT_COMMIT)
    parser.add_argument("--path", default=DEFAULT_PATH)
    parser.add_argument("--model", default="bge-m3:latest")
    parser.add_argument("--ollama-url", default="http://127.0.0.1:11434")
    parser.add_argument("--num-ctx", type=int, default=8192)
    parser.add_argument("--lengths", default="4000,5000,5500,6000,8000,12205")
    parser.add_argument("--output", type=Path)
    args = parser.parse_args()
    parsed = urllib.parse.urlparse(args.ollama_url)
    if parsed.scheme != "http" or parsed.hostname not in {"127.0.0.1", "localhost", "::1"}:
        raise ValueError("Ollama URL must be local HTTP loopback")
    if args.num_ctx <= 0 or not args.model or ":" not in args.model:
        raise ValueError("positive num_ctx and exact tagged model required")
    lengths = [int(item) for item in args.lengths.split(",")]
    if not lengths or any(n <= 0 for n in lengths) or len(set(lengths)) != len(lengths):
        raise ValueError("lengths must be unique positive byte counts")
    if args.path.startswith("/") or ".." in args.path.split("/"):
        raise ValueError("source path must be repository-relative")
    source = subprocess.run(["git", "show", f"{args.commit}:{args.path}"], cwd=ROOT,
                            check=True, capture_output=True).stdout
    if any(n > len(source) for n in lengths):
        raise ValueError("probe length exceeds pinned source byte length")
    endpoint = args.ollama_url.rstrip("/")
    digest = model_digest(endpoint, args.model)
    version = read_json(endpoint + "/api/version").get("version")
    if not isinstance(version, str) or not version:
        raise ValueError("Ollama version unavailable")
    probes = []
    for length in lengths:
        sample = source[:length].decode("utf-8", errors="ignore")
        payload = json.dumps({"model": args.model, "input": sample, "truncate": False,
                              "options": {"num_ctx": args.num_ctx}}).encode()
        start = time.monotonic()
        try:
            response = read_json(endpoint + "/api/embed", payload, timeout=180)
            vectors = response.get("embeddings", [])
            if len(vectors) != 1 or not vectors[0] or response.get("model") != args.model:
                raise ValueError("embedding response shape/model changed")
            status = "accepted"
            tokens = response.get("prompt_eval_count")
        except urllib.error.HTTPError as exc:
            body = exc.read(1024)
            try:
                message = json.loads(body).get("error", "")
            except (ValueError, AttributeError):
                message = ""
            if exc.code != 400 or message != "the input length exceeds the context length":
                raise ValueError(f"unexpected Ollama HTTP {exc.code}; probe aborted") from exc
            status, tokens = "context_rejected", None
        probes.append({"requested_bytes": length, "actual_bytes": len(sample.encode()),
                       "status": status, "prompt_eval_count": tokens,
                       "duration_ms": round((time.monotonic() - start) * 1000)})
    if model_digest(endpoint, args.model) != digest:
        raise ValueError("model digest changed during context probe")
    result = {"schema_version": 1, "gate": "B0-diagnostic", "quality_metrics": None,
              "checked_at": dt.datetime.now(dt.timezone.utc).isoformat(),
              "source_commit": args.commit, "source_path": args.path,
              "source_sha256": hashlib.sha256(source).hexdigest(),
              "model": args.model, "model_digest": digest,
              "ollama_version": version, "requested_num_ctx": args.num_ctx,
              "truncate": False, "probes": probes}
    rendered = json.dumps(result, ensure_ascii=False, indent=2) + "\n"
    if args.output:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(rendered)
    else:
        sys.stdout.write(rendered)


if __name__ == "__main__":
    try:
        main()
    except (OSError, ValueError, subprocess.CalledProcessError) as exc:
        print(f"b0-context-probe: {exc}", file=sys.stderr)
        sys.exit(1)
