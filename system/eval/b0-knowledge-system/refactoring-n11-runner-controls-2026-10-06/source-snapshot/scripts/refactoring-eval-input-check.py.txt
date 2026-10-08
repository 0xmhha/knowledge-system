#!/usr/bin/env python3
"""Audit fresh evaluation inputs; never runs queries or grants product release.

Human records supply semantic gold/independence judgments. This checker checks
binding and consistency, not reviewer authenticity or the truth of judgments.
Live model, coordinated dataset, binary and run-state checks remain separate.
"""
import argparse
import datetime as dt
import hashlib
import json
from pathlib import Path, PurePosixPath
import re
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[1]
GROUPS = {"CODE", "WHY", "POLICY", "TRACE", "ABS"}
SHA40 = re.compile(r"[0-9a-f]{40}")
SHA64 = re.compile(r"[0-9a-f]{64}")


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def timestamp(value):
    if not isinstance(value, str):
        raise ValueError("timestamp missing")
    parsed = dt.datetime.fromisoformat(value.replace("Z", "+00:00"))
    if parsed.tzinfo is None:
        raise ValueError("timestamp needs timezone")
    return parsed


def decode(raw):
    if len(raw) > 10 << 20:
        raise ValueError("input exceeds10MiB")
    def pairs(rows):
        result = {}
        for key, value in rows:
            if key in result:
                raise ValueError("duplicate JSON key")
            result[key] = value
        return result
    return json.loads(raw, object_pairs_hook=pairs,
                      parse_constant=lambda value: (_ for _ in ()).throw(ValueError("nonfinite JSON")))


def audit(protocol_raw, questions_raw, baseline_raw, review, observations, read_source):
    protocol, book, baseline = map(decode, (protocol_raw, questions_raw, baseline_raw))
    errors, counts = [], {group: set() for group in GROUPS}
    def check(condition, reason):
        if not condition:
            errors.append(reason)
    check(type(protocol.get("schema_version")) is int and protocol["schema_version"] == 1 and type(book.get("schema_version")) is int and book["schema_version"] == 1, "unsupported_schema")
    check(protocol.get("approved_original_protocol_sha256") == sha(baseline_raw), "baseline_protocol_binding_mismatch")
    for key in ("retrieval_k", "retrieval_runs", "warm_latency", "cold_start", "paired_arms", "pack_axis", "order", "thresholds", "uncertainty"):
        check(protocol.get(key) == baseline.get(key), "baseline_changed:" + key)
    for key in ("provider", "model", "digest", "dimension", "runtime_options"):
        check(protocol.get("model", {}).get(key) == baseline.get("model", {}).get(key), "model_input_changed:" + key)
    check(set(protocol.get("important_groups", [])) == GROUPS, "important_groups_incomplete")
    old_ids = set(baseline.get("final_questions", []) + baseline.get("synthetic_final_fixtures", []))
    check(old_ids and old_ids <= set(protocol.get("previous_observed_final_excluded", [])), "old_final_exclusions_incomplete")
    check(protocol.get("status") == "approved", "protocol_needs_pre_result_review")
    commit = book.get("source_commit", "")
    check(isinstance(commit, str) and SHA40.fullmatch(commit) is not None and commit == protocol.get("source_commit"), "source_commit_mismatch")
    check(book.get("status") == "approved", "question_book_needs_review")
    check(protocol.get("results") is None and book.get("results") is None, "results_already_recorded")
    review = review or {}
    check(review.get("status") == "approved" and bool(review.get("reviewer")) and review.get("scope") == "fresh-evaluation-inputs", "human_input_review_missing")
    check(review.get("protocol_sha256") == sha(protocol_raw) and review.get("questions_sha256") == sha(questions_raw), "freeze_binding_mismatch")
    try:
        reviewed, frozen = timestamp(review.get("reviewed_at")), timestamp(review.get("frozen_at"))
        check(reviewed <= frozen <= dt.datetime.now(dt.timezone.utc), "freeze_time_invalid")
    except (ValueError, TypeError):
        errors.append("freeze_time_missing")
        frozen = None
    decisions = review.get("cases", {})
    questions = book.get("questions", [])
    check(isinstance(questions, list) and bool(questions), "questions_missing")
    if not isinstance(questions, list):
        questions = []
    seen, units, prompts, split_clusters = set(), {}, {}, {"DEV": set(), "FINAL": set()}
    final_clusters = set()
    for q in questions:
        if not isinstance(q, dict):
            errors.append("invalid_question")
            continue
        case_start = len(errors)
        ident, split, group = q.get("id"), q.get("split"), q.get("group")
        check(isinstance(ident, str) and re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9_-]*", ident) is not None and ident.lower() not in seen, "invalid_or_duplicate_question_id")
        if isinstance(ident, str):
            seen.add(ident.lower())
        check(split in split_clusters and group in GROUPS, "invalid_group_or_split:" + str(ident))
        check(ident not in old_ids and not q.get("previous_observed_final_unit"), "old_final_reused:" + str(ident))
        check(q.get("review_state") == "approved" and bool(q.get("prompt")) and bool(q.get("candidate_answer")), "gold_not_approved:" + str(ident))
        check(q.get("source_commit") == commit, "question_source_mismatch:" + str(ident))
        d = decisions.get(ident, {})
        cluster, unit = q.get("cluster_id"), q.get("unit_id")
        check(isinstance(cluster, str) and bool(cluster) and isinstance(unit, str) and bool(unit), "unit_cluster_missing:" + str(ident))
        check(all(d.get(key) is True for key in ("gold_approved", "independence_approved", "contamination_checked")) and d.get("cluster_id") == cluster and d.get("unit_id") == unit, "case_judgment_missing:" + str(ident))
        if cluster and unit and split in split_clusters and group in GROUPS:
            check(unit not in units or units[unit] == cluster, "unit_split_into_multiple_clusters:" + str(ident))
            units[unit] = cluster
            prompt = " ".join(str(q.get("prompt", "")).casefold().split())
            check(prompt not in prompts or prompts[prompt] == cluster, "same_prompt_counted_as_independent:" + str(ident))
            prompts[prompt] = cluster
            split_clusters[split].add(cluster)
            if split == "FINAL":
                final_clusters.add(cluster)
        refs, behavior = q.get("evidence"), q.get("expected_behavior")
        check(isinstance(refs, list) and behavior in {"cite", "abstain"} and (behavior == "cite") == bool(refs), "evidence_behavior_mismatch:" + str(ident))
        if behavior == "abstain":
            check(group == "ABS" and q.get("strict_zero_citations") is True and bool(q.get("abstention_basis")), "abstention_scope_missing:" + str(ident))
        for ref in refs if isinstance(refs, list) else []:
            try:
                path = ref["path"]
                pure = PurePosixPath(path)
                if pure.is_absolute() or ".." in pure.parts or str(pure) != path or "\\" in path:
                    raise ValueError("unsafe source path")
                first, last = ref["first"], ref["last"]
                if type(first) is not int or type(last) is not int:
                    raise ValueError("noninteger span")
                source = read_source(commit, path)
                parts = source.split(b"\n")
                lines = [part + b"\n" for part in parts[:-1]] + ([parts[-1]] if parts[-1] else [])
                if not 1 <= first <= last <= len(lines) or sha(source) != ref["file_sha256"] or sha(b"".join(lines[first-1:last])) != ref["content_sha256"]:
                    raise ValueError("stale source or span")
            except (ValueError, KeyError, TypeError, OSError, subprocess.CalledProcessError):
                errors.append("invalid_source_gold:" + str(ident))
        if len(errors) == case_start and split == "FINAL" and group in GROUPS:
            counts[group].add(cluster)
    check(bool(split_clusters["DEV"]), "DEV_missing")
    check(not split_clusters["DEV"] & split_clusters["FINAL"], "DEV_FINAL_cluster_overlap")
    for group, clusters in counts.items():
        check(bool(clusters), "FINAL_group_missing:" + group)
    for observation in observations:
        try:
            reserved = observation.get("state") == "FINAL_reserved"
            observed = timestamp(observation["reserved_at"] if reserved else observation["first_observed_at"])
            check(isinstance(observation["final_clusters"], list) and bool(observation["final_clusters"]) and all(isinstance(x, str) and x for x in observation["final_clusters"]), "invalid_observation_clusters")
            check(SHA64.fullmatch(observation["questions_sha256"]) is not None and SHA64.fullmatch(observation["protocol_sha256"]) is not None, "invalid_observation_binding")
            check(not final_clusters & set(observation["final_clusters"]), "FINAL_reserved_or_started" if reserved else "FINAL_already_observed")
            if observation["questions_sha256"] == sha(questions_raw):
                check(False, "question_set_reserved_or_started" if reserved else "question_set_already_observed")
                if frozen is not None:
                    check(observed >= frozen, "reservation_before_freeze" if reserved else "observation_before_freeze")
        except (ValueError, KeyError, TypeError):
            errors.append("invalid_observation_record")
    errors = sorted(set(errors))
    n = {group: len(clusters) for group, clusters in sorted(counts.items())}
    sufficient = all(size >= 10 for size in n.values())
    return {"schema_version": 1, "scope": "input bindings and recorded review/cluster consistency only",
            "input_ready": not errors, "execution_ready": False,
            "execution_pending": ["live model identity", "coordinated dataset/source/binary binding", "runner observation-state enforcement"],
            "final_independent_clusters_by_group": n,
            "sampling_verdict": "sufficient_recorded_units" if not errors and sufficient else "inconclusive",
            "product_release_approved": False, "pending_reasons": errors,
            "questions_sha256": sha(questions_raw), "protocol_sha256": sha(protocol_raw)}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--protocol", type=Path, required=True)
    parser.add_argument("--questions", type=Path, required=True)
    parser.add_argument("--review", type=Path)
    parser.add_argument("--observations", type=Path, required=True, help="explicit known FINAL observation records JSON array")
    parser.add_argument("--baseline", type=Path, default=ROOT / "system/eval/b0-knowledge-system/protocol-m2max-draft.json")
    parser.add_argument("--repo", type=Path, default=ROOT)
    parser.add_argument("--output", type=Path)
    parser.add_argument("--require-ready", action="store_true", help="exit2 unless input-ready; does not authorize execution")
    args = parser.parse_args()
    def read(commit, path):
        spec = commit + ":" + path
        size = int(subprocess.check_output(["git", "-C", str(args.repo), "cat-file", "-s", spec]))
        if size > 32 << 20:
            raise ValueError("source file exceeds capture limit")
        return subprocess.check_output(["git", "-C", str(args.repo), "show", spec])
    observations = decode(args.observations.read_bytes())
    if not isinstance(observations, list):
        raise ValueError("observations must be array")
    result = audit(args.protocol.read_bytes(), args.questions.read_bytes(), args.baseline.read_bytes(),
                   decode(args.review.read_bytes()) if args.review else None, observations, read)
    rendered = json.dumps(result, ensure_ascii=False, indent=2) + "\n"
    if args.output:
        args.output.write_text(rendered)
    else:
        sys.stdout.write(rendered)
    return 2 if args.require_ready and not result["input_ready"] else 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except (ValueError, OSError, TypeError, KeyError, subprocess.CalledProcessError) as error:
        print("refactoring-eval-input-check: " + str(error), file=sys.stderr)
        sys.exit(1)
