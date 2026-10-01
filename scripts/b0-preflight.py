#!/usr/bin/env python3
"""Read-only B0 readiness check; never turns draft answers into approved gold."""

import argparse
import datetime as dt
import hashlib
import json
import math
import os
from pathlib import Path, PurePosixPath
import platform
import re
import subprocess
import sys
import urllib.error
import urllib.parse
import urllib.request


ROOT = Path(__file__).resolve().parent.parent
DEFAULT_QUESTIONS = ROOT / "system/eval/b0-knowledge-system/questions.json"
SHA40 = re.compile(r"^[0-9a-f]{40}$")
SHA64 = re.compile(r"^[0-9a-f]{64}$")
GROUPS = {"code_location", "why_design", "policy_conflict", "abstention"}


def git(*args):
    return subprocess.run(["git", *args], cwd=ROOT, check=True,
                          stdout=subprocess.PIPE, stderr=subprocess.PIPE).stdout


def hardware():
    result = {"os": platform.system().lower(), "arch": platform.machine(),
              "cpu_count": os.cpu_count(), "memory_bytes": None}
    if result["os"] == "darwin":
        try:
            result["memory_bytes"] = int(subprocess.run(
                ["sysctl", "-n", "hw.memsize"], check=True, stdout=subprocess.PIPE,
                stderr=subprocess.DEVNULL, text=True).stdout.strip())
        except (OSError, ValueError, subprocess.CalledProcessError):
            pass
    elif result["os"] == "linux":
        try:
            for line in Path("/proc/meminfo").read_text().splitlines():
                if line.startswith("MemTotal:"):
                    result["memory_bytes"] = int(line.split()[1]) * 1024
                    break
        except (OSError, ValueError, IndexError):
            pass
    return result


def validate_questions(raw):
    book = json.loads(raw)
    if book.get("schema_version") != 1 or book.get("corpus_project") != "knowledge-system":
        raise ValueError("unsupported question set/project")
    commit = book.get("corpus_commit", "")
    if not SHA40.fullmatch(commit):
        raise ValueError("corpus_commit must be a full lowercase Git SHA")
    tree = git("rev-parse", f"{commit}^{{tree}}").decode().strip()
    questions = book.get("questions")
    if not isinstance(questions, list) or not questions:
        raise ValueError("questions must be nonempty")
    seen, anchors, approved = set(), [], 0
    blobs = {}
    for question in questions:
        qid = question.get("id")
        if not isinstance(qid, str) or not qid or qid in seen:
            raise ValueError("question ID missing or duplicate")
        seen.add(qid)
        if question.get("group") not in GROUPS or question.get("language") not in {"ko", "en"}:
            raise ValueError(f"{qid}: unknown group/language")
        if not question.get("prompt") or not question.get("candidate_answer"):
            raise ValueError(f"{qid}: prompt and candidate answer required")
        behavior, evidence = question.get("expected_behavior"), question.get("evidence")
        if behavior not in {"cite", "abstain"} or not isinstance(evidence, list):
            raise ValueError(f"{qid}: invalid expected behavior/evidence")
        if (behavior == "cite") != bool(evidence):
            raise ValueError(f"{qid}: cite needs evidence; abstain must have none")
        state = question.get("review_state")
        if state not in {"draft", "approved", "rejected"}:
            raise ValueError(f"{qid}: invalid review state")
        if state == "approved":
            if not question.get("reviewer") or not question.get("reviewed_at"):
                raise ValueError(f"{qid}: approved gold needs reviewer and review time")
            approved += 1
        for ref in evidence:
            path, anchor = ref.get("path"), ref.get("anchor")
            if not isinstance(path, str) or not isinstance(anchor, str) or not anchor:
                raise ValueError(f"{qid}: invalid evidence ref")
            pure = PurePosixPath(path)
            if pure.is_absolute() or ".." in pure.parts or "\\" in path or str(pure) != path:
                raise ValueError(f"{qid}: unsafe evidence path")
            if path not in blobs:
                blobs[path] = git("show", f"{commit}:{path}").decode("utf-8")
            body = blobs[path]
            if body.count(anchor) != 1:
                raise ValueError(f"{qid}: evidence anchor must appear once: {path}: {anchor!r}")
            offset = body.index(anchor)
            line = body.count("\n", 0, offset) + 1
            start, end = ref.get("start_line"), ref.get("end_line")
            if (not isinstance(start, int) or isinstance(start, bool) or
                    not isinstance(end, int) or isinstance(end, bool) or
                    start < 1 or end < start or end > len(body.splitlines()) or
                    not start <= line <= end):
                raise ValueError(f"{qid}: approved citation span must contain its anchor: {path}:{line}")
            anchors.append({"question_id": qid, "path": path,
                            "line": line, "start_line": start, "end_line": end})
    return {"corpus_commit": commit, "corpus_tree": tree,
            "question_count": len(questions), "approved_count": approved,
            "groups": {group: sum(q["group"] == group for q in questions) for group in sorted(GROUPS)},
            "anchors": anchors}


def ollama_identity(url, model):
    parsed = urllib.parse.urlparse(url)
    if parsed.scheme != "http" or parsed.hostname not in {"127.0.0.1", "localhost", "::1"}:
        raise ValueError("B0 Ollama endpoint must be local HTTP loopback")
    if not model:
        return None, "model_not_selected"
    exact = model if ":" in model else model + ":latest"
    try:
        with urllib.request.urlopen(url.rstrip("/") + "/api/version", timeout=3) as response:
            version = json.load(response).get("version")
        if not isinstance(version, str) or not version:
            return None, "ollama_version_invalid"
        with urllib.request.urlopen(url.rstrip("/") + "/api/tags", timeout=3) as response:
            tags = json.load(response).get("models", [])
        matches = [tag for tag in tags if tag.get("name") == exact]
        if len(matches) != 1 or not SHA64.fullmatch(matches[0].get("digest", "")):
            return None, "model_absent_or_digest_invalid"
        payload = json.dumps({"model": exact, "input": "CKS B0 identity probe", "truncate": False}).encode()
        request = urllib.request.Request(url.rstrip("/") + "/api/embed", payload,
                                         {"Content-Type": "application/json"})
        with urllib.request.urlopen(request, timeout=120) as response:
            embedded = json.load(response)
        vectors = embedded.get("embeddings", [])
        if len(vectors) != 1 or not vectors[0] or not all(
                isinstance(x, (int, float)) and math.isfinite(x) for x in vectors[0]):
            return None, "embedding_probe_invalid"
        with urllib.request.urlopen(url.rstrip("/") + "/api/tags", timeout=3) as response:
            after = json.load(response).get("models", [])
        if len([tag for tag in after if tag.get("name") == exact and
                tag.get("digest") == matches[0]["digest"]]) != 1:
            return None, "model_digest_changed_during_probe"
        return {"provider": "ollama", "model": exact, "digest": matches[0]["digest"],
                "dimension": len(vectors[0]), "endpoint": url,
                "server_version": version}, None
    except (OSError, ValueError, KeyError, urllib.error.URLError, json.JSONDecodeError):
        return None, "ollama_unavailable_or_probe_failed"


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--questions", type=Path, default=DEFAULT_QUESTIONS)
    parser.add_argument("--model", default="", help="exact local Ollama model name or tag")
    parser.add_argument("--ollama-url", default="http://127.0.0.1:11434")
    parser.add_argument("--output", type=Path)
    parser.add_argument("--require-ready", action="store_true")
    args = parser.parse_args()
    raw = args.questions.read_bytes()
    question_set = validate_questions(raw)
    host = hardware()
    model, model_reason = ollama_identity(args.ollama_url, args.model)
    reasons = []
    if question_set["approved_count"] != question_set["question_count"]:
        reasons.append("gold_answers_need_human_approval")
    if model_reason:
        reasons.append(model_reason)
    if host["memory_bytes"] is None:
        reasons.append("hardware_memory_unknown")
    result = {"schema_version": 1, "gate": "B0", "status": "ready" if not reasons else "pending",
              "checked_at": dt.datetime.now(dt.timezone.utc).isoformat(),
              "question_set_sha256": hashlib.sha256(raw).hexdigest(),
              "question_set": question_set, "hardware": host, "model": model,
              "pending_reasons": reasons, "metrics": None}
    rendered = json.dumps(result, ensure_ascii=False, indent=2) + "\n"
    if args.output:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(rendered)
    else:
        sys.stdout.write(rendered)
    if args.require_ready and reasons:
        return 2
    return 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except (ValueError, OSError, subprocess.CalledProcessError, json.JSONDecodeError) as exc:
        print(f"b0-preflight: {exc}", file=sys.stderr)
        sys.exit(1)
