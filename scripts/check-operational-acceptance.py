#!/usr/bin/env python3
"""Check preparation consistency and structural prerequisites, not gold semantics."""
import argparse
import copy
import hashlib
import json
import math
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
PREFIX = "docs/spec-driven/OPERATIONAL-ACCEPTANCE-"
IDS = {"WI-V1", "WI-V2", "WI-3", "WI-4", "WI-1", "WI-2", "WI-C", "WI-6", "WI-SP"}
FREEZE_IDS = {f"F-{n:02d}" for n in range(1, 9)}


def protocol_hash(protocol):
    payload = copy.deepcopy(protocol)
    for field in ("freeze", "execution_allowed", "blocked_by"):
        payload.pop(field, None)
    for row in payload.get("freeze_requirements", []):
        if isinstance(row.get("receipt"), dict):
            row["receipt"].pop("protocol_sha256", None)
    for row in payload.get("input_groups", []):
        if isinstance(row.get("review_receipt"), dict):
            row["review_receipt"].pop("protocol_sha256", None)
    raw = json.dumps(payload, ensure_ascii=False, sort_keys=True, separators=(",", ":")).encode()
    return hashlib.sha256(raw).hexdigest()


def human_receipt(receipt, digest):
    return (isinstance(receipt, dict) and receipt.get("kind") == "human"
            and bool(receipt.get("name")) and receipt.get("decision") == "approved"
            and receipt.get("protocol_sha256") == digest)


def check_review_draft_packet(root, reference):
    """Verify draft separation and source anchors; never validate fact semantics."""
    errors = []

    def read(path, expected):
        if not isinstance(path, str) or not isinstance(expected, str):
            raise ValueError("missing draft path/hash")
        target = (root / path).resolve()
        if not target.is_relative_to(root.resolve()):
            raise ValueError("draft reference escapes repository")
        raw = target.read_bytes()
        if hashlib.sha256(raw).hexdigest() != expected:
            raise ValueError("draft file hash differs")
        return raw

    try:
        public = json.loads(read(reference["public_path"], reference["public_sha256"]))
        private = json.loads(read(reference["gold_draft_path"], reference["gold_draft_sha256"]))
        forbidden = {"expected_answer", "expected_source_fact_draft", "required_rubric_anchors_draft", "data_anchors", "control_anchors", "gold_draft_path"}

        def leaks(value):
            if isinstance(value, dict):
                return bool(forbidden.intersection(value)) or any(leaks(v) for v in value.values())
            if isinstance(value, list):
                return any(leaks(v) for v in value)
            return False

        if public.get("gold_included") is not False or leaks(public):
            errors.append("public draft contains gold labels or references")
        cases = public.get("cases", [])
        reviews = private.get("review_drafts", [])
        ids = [c.get("case_id") for c in cases]
        if (len(cases) != reference.get("review_cases") or len(set(ids)) != len(ids)
                or set(ids) != {r.get("case_id") for r in reviews}):
            errors.append("public/private draft case identities differ")
        if private.get("independent_gold_records") != 0 or private.get("approved_cases") != 0:
            errors.append("AI draft packet promoted to independent gold")
        if private.get("ckg_output_used") is not False or private.get("runtime_executions") != 0:
            errors.append("draft origin/runtime status differs from declared source-only preparation")
        public_sources = {c["case_id"]: c["source"] for c in cases}
        for case in cases:
            if case.get("approved") is not False or case.get("split") is not None:
                errors.append("public draft marked approved or assigned to holdout")
        for row in reviews:
            if (row.get("origin") != "ai_manual_source_reading" or row.get("approved") is not False
                    or row.get("independence_verified") is not False or row.get("review_receipt") is not None):
                errors.append("AI review draft marked independently approved")
            source = row["source"]
            if public_sources.get(row.get("case_id")) != source:
                errors.append("public/private source identity differs")
            raw = read(source["excerpt_path"], source["excerpt_sha256"])
            if hashlib.sha256(raw).hexdigest() != source["function_sha256"]:
                errors.append("function excerpt hash differs from pinned function")
            for fact in row.get("required_rubric_anchors_draft", []):
                a = fact["start_byte"] - source["start_byte"]
                b = fact["end_byte_exclusive"] - source["start_byte"]
                if not (0 <= a < b <= len(raw)) or hashlib.sha256(raw[a:b]).hexdigest() != fact["span_sha256"]:
                    errors.append("review source anchor bytes/hash differ")
                elif (source["start_line"] + raw[:a].count(b"\n") != fact["start_line"]
                      or source["start_line"] + raw[:b].count(b"\n") - 1 != fact["end_line"]):
                    errors.append("review source anchor line mapping differs")
        anchors = private.get("value_control_anchor_drafts", [])
        if len(anchors) != reference.get("value_control_anchor_functions"):
            errors.append("value/control draft function count differs")
        for row in anchors:
            if row.get("approved") is not False or row.get("precision_recall_scoring_allowed") is not False:
                errors.append("non-exhaustive anchors promoted to scorable gold")
            source = row["source"]
            raw = read(source["excerpt_path"], source["excerpt_sha256"])
            for edge in row["data_anchors"] + row["control_anchors"]:
                for key in ("definition", "use", "predicate", "dependent_statement"):
                    if key not in edge:
                        continue
                    anchor = edge[key]
                    a, b = anchor["start_byte"] - source["start_byte"], anchor["end_byte_exclusive"] - source["start_byte"]
                    if not (0 <= a < b <= len(raw)) or hashlib.sha256(raw[a:b]).hexdigest() != anchor["sha256"]:
                        errors.append("value/control source anchor bytes/hash differ")
                    elif source["start_line"] + raw[:a].count(b"\n") != anchor["line"]:
                        errors.append("value/control anchor line mapping differs")
    except (OSError, ValueError, KeyError, TypeError) as exc:
        errors.append("review draft packet invalid: " + str(exc))
    return errors


def check(root, protocol, ledger, require_frozen=False):
    errors, pending = [], []
    digest = protocol_hash(protocol)

    def file_hash(path, expected):
        if not isinstance(path, str) or not isinstance(expected, str):
            return False
        target = (root / path).resolve()
        if not target.is_relative_to(root.resolve()) or not target.is_file():
            return False
        return hashlib.sha256(target.read_bytes()).hexdigest() == expected

    groups = protocol.get("input_groups", [])
    items = ledger.get("items", [])
    requirements = protocol.get("freeze_requirements", [])
    if len(groups) != 9 or {r.get("id") for r in groups} != IDS:
        errors.append("input group identities must cover exactly nine work items")
    if len(items) != 9 or {r.get("id") for r in items} != IDS:
        errors.append("acceptance ledger identities must cover exactly nine work items")
    if len(requirements) != 8 or {r.get("id") for r in requirements} != FREEZE_IDS:
        errors.append("freeze checklist must contain F-01 through F-08 exactly once")
    if not file_hash(protocol.get("source_inventory"), protocol.get("source_inventory_sha256")):
        errors.append("source preparation inventory missing or hash differs")
    for field in ("input_candidate_pool", "host_preflight"):
        reference = protocol.get(field)
        if reference is not None:
            if not isinstance(reference, dict) or not file_hash(reference.get("path"), reference.get("sha256")):
                errors.append("preparation reference missing or hash differs: " + field)
    if protocol.get("review_draft_packet") is not None:
        errors.extend(check_review_draft_packet(root, protocol["review_draft_packet"]))
    patches = protocol.get("readiness_candidate_patches", [])
    if patches != ledger.get("readiness_fixes", []):
        errors.append("protocol/ledger readiness patch references differ")
    for patch in patches:
        if not file_hash(patch.get("manifest"), patch.get("manifest_sha256")):
            errors.append("readiness patch manifest missing or hash differs")
            continue
        manifest_path = root / patch["manifest"]
        manifest = json.loads(manifest_path.read_text())
        if manifest.get("candidate_commit") != patch.get("candidate_commit"):
            errors.append("readiness candidate source identities differ")
        for record in (patch, manifest):
            if (record.get("operational_gate") != "not_run" or record.get("quality_gate") != "not_run"
                    or record.get("independent_acceptance") != "not_recorded"):
                errors.append("controlled readiness patch promoted to operational or independent acceptance")
        if manifest.get("kind") != "controlled_development_regression_not_operational_independent_acceptance":
            errors.append("readiness patch evidence kind differs")
        for artifact in manifest.get("artifacts", []):
            path = manifest_path.parent / artifact["path"]
            if not path.resolve().is_relative_to(manifest_path.parent.resolve()):
                errors.append("readiness artifact path escapes packet")
                continue
            relative = path.relative_to(root).as_posix()
            if not file_hash(relative, artifact.get("sha256")):
                errors.append("readiness artifact missing or hash differs")
    for row in requirements:
        if row.get("decision_status") != "approved" or not human_receipt(row.get("receipt"), digest):
            pending.append(row.get("id", "unknown freeze requirement"))
    roles = protocol.get("roles", {})
    for role in ("gold_author", "independent_content_reviewer", "final_scope_acceptor", "execution_operator", "independence_declaration"):
        if not roles.get(role):
            pending.append("role:" + role)
    model = protocol.get("model", {})
    for field in ("llm_provider", "llm_model", "llm_version_or_digest", "currency", "cost_accounting_source"):
        if not model.get(field):
            pending.append("model:" + field)
    limit = model.get("llm_cost_limit")
    if isinstance(limit, bool) or not isinstance(limit, (int, float)) or not math.isfinite(limit) or limit < 0:
        pending.append("model:nonnegative_cost_limit")
    if not protocol.get("measurement_proposal", {}).get("latency_percentile_method"):
        pending.append("measurement:percentile_method")
    all_splits = {"DEV": set(), "FINAL": set()}
    for row in groups:
        label = row.get("id", "unknown")
        for field in ("baseline_definition", "canonical_matching_rules", "sample_sufficiency_review"):
            if not row.get(field):
                pending.append(label + ":" + field)
        cases = row.get("cases", [])
        if not cases:
            pending.append(label + ":new_cases")
        case_ids, split_families = set(), {"DEV": set(), "FINAL": set()}
        for case in cases:
            missing = [f for f in protocol.get("case_record_required_fields", []) if case.get(f) is None]
            if missing:
                pending.append(label + ":case_fields")
            case_id, family, split = case.get("case_id"), case.get("family_id"), case.get("split")
            if not case_id or case_id in case_ids or not family or split not in split_families:
                errors.append(label + ": invalid case identity or split")
                continue
            case_ids.add(case_id)
            split_families[split].add(family)
            if case.get("origin") in ("old_fixture", "old_question", "regression"):
                errors.append(label + ": regression input cannot become independent gold")
            for path, h in (("public_input_path", "public_input_sha256"), ("gold_path", "gold_sha256")):
                if not file_hash(case.get(path), case.get(h)):
                    pending.append(label + ":case_file_hash")
        if split_families["DEV"] & split_families["FINAL"]:
            errors.append(label + ": DEV/FINAL family overlap")
        for split in all_splits:
            all_splits[split].update(split_families[split])
        minimum = row.get("final_minimum_independent_units")
        if isinstance(minimum, bool) or not isinstance(minimum, int) or minimum < 1:
            pending.append(label + ":FINAL_minimum_units")
        elif len(split_families["FINAL"]) < minimum:
            pending.append(label + ":FINAL_insufficient_units")
        for field in ("input_manifest", "gold_manifest"):
            if not file_hash(row.get(field), row.get(field + "_sha256")):
                pending.append(label + ":" + field)
        if not human_receipt(row.get("review_receipt"), digest):
            pending.append(label + ":input_review")
    if all_splits["DEV"] & all_splits["FINAL"]:
        errors.append("DEV/FINAL family overlap across work items")
    if not protocol.get("runtime_recipes") or not protocol.get("owned_resources"):
        pending.append("runtime:recipes_and_owned_resources")
    for recipe in protocol.get("runtime_recipes", []):
        argv = recipe.get("argv_template", [])
        if not isinstance(argv, list) or not argv:
            errors.append("runtime recipe lacks argv")
            continue
        forbidden = {"--dangerously-skip-permissions", "--allow-dangerously-skip-permissions", "--bare", "--safe-mode"}
        if forbidden.intersection(argv) or "bypassPermissions" in argv:
            errors.append("host recipe bypasses permissions or required hooks")
        if recipe.get("status") != "ready" or any("{" in str(a) or "}" in str(a) for a in argv):
            pending.append("runtime:unresolved_recipe:" + recipe.get("id", "unknown"))
        if not recipe.get("timeout_seconds"):
            pending.append("runtime:recipe_timeout:" + recipe.get("id", "unknown"))
        if not protocol.get("freeze") and recipe.get("execution_status") != "not_run":
            errors.append("runtime recipe execution recorded before freeze")
    freeze = protocol.get("freeze")
    frozen = (not pending and isinstance(freeze, dict) and bool(freeze.get("frozen_at"))
              and freeze.get("protocol_sha256") == digest
              and human_receipt(freeze.get("scope_receipt"), digest))
    if freeze is not None and not frozen:
        errors.append("freeze recorded before structural prerequisites or receipt/hash matched")
    if protocol.get("execution_allowed") is not frozen:
        errors.append("execution_allowed differs from structural freeze readiness")
    expected_blockers = {r["id"] for r in requirements if r.get("decision_status") != "approved"}
    if set(protocol.get("blocked_by", [])) != expected_blockers:
        errors.append("blocked_by does not match pending freeze decisions")
    if ledger.get("protocol_freeze") != freeze:
        errors.append("ledger and protocol freeze records differ")
    phases = ledger.get("phases", [])
    if len(phases) != 5 or {r.get("id") for r in phases} != {str(n) for n in range(5)}:
        errors.append("ledger must contain phases 0 through 4")
    if not frozen:
        if ledger.get("execution_status") != "not_started" or ledger.get("execution_receipts"):
            errors.append("execution recorded before structural freeze")
        for phase in phases:
            allowed = {"not_started", "preparing"} if phase.get("id") == "0" else {"not_started"}
            if phase.get("status") not in allowed:
                errors.append("phase advancement recorded before structural freeze")
        for item in items:
            if (item.get("operational_gate") != "not_run" or item.get("quality_gate") != "not_run"
                    or item.get("independent_acceptance") != "not_recorded"
                    or item.get("evidence") is not None or item.get("review_receipt") is not None):
                errors.append("item execution or acceptance recorded before structural freeze")
    for item in items:
        states = {
            "operational_gate": {"not_run", "failed", "partial", "pass"},
            "quality_gate": {"not_run", "inconclusive", "failed", "scoped_validated"},
            "independent_acceptance": {"not_recorded", "awaiting_review", "rejected", "scope_accepted"},
        }
        for field, allowed in states.items():
            if item.get(field) not in allowed:
                errors.append("unknown item verdict: " + field)
        if item.get("operational_gate") == "pass" or item.get("quality_gate") == "scoped_validated":
            evidence = item.get("evidence")
            if not isinstance(evidence, dict) or not file_hash(evidence.get("path"), evidence.get("sha256")):
                errors.append("positive operational/quality verdict missing hash-bound evidence manifest")
        if item.get("independent_acceptance") == "scope_accepted":
            evidence = item.get("evidence")
            if (not isinstance(evidence, dict) or not file_hash(evidence.get("path"), evidence.get("sha256"))
                    or not human_receipt(item.get("review_receipt"), digest)):
                errors.append("human scope acceptance missing evidence or bound human receipt")
    for phase in phases:
        if phase.get("id") != "0" and phase.get("status") == "complete":
            evidence = phase.get("evidence")
            if not isinstance(evidence, dict) or not file_hash(evidence.get("path"), evidence.get("sha256")):
                errors.append("completed execution phase missing hash-bound evidence manifest")
    if require_frozen and not frozen:
        errors.append("protocol is not frozen; prerequisite data and review remain pending")
    return errors, sorted(set(pending))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--require-frozen", action="store_true")
    args = parser.parse_args()
    try:
        protocol = json.loads((ROOT / (PREFIX + "PROTOCOL.json")).read_text())
        ledger = json.loads((ROOT / (PREFIX + "PROGRESS.json")).read_text())
        errors, pending = check(ROOT, protocol, ledger, args.require_frozen)
    except (OSError, ValueError, TypeError, KeyError) as exc:
        print("operational acceptance: FAIL:", exc)
        return 1
    for error in errors:
        print("FAIL:", error)
    print(f"operational acceptance: {'FAIL' if errors else 'PASS'}; pending structural prerequisites={len(pending)}; runtime/quality/human semantics not verified")
    return bool(errors)


if __name__ == "__main__":
    raise SystemExit(main())
