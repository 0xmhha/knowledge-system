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
    from independent_acceptance import reviewer_identity
    return (isinstance(receipt, dict) and receipt.get("kind") == "human"
            and bool(reviewer_identity(receipt)) and receipt.get("decision") == "approved"
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
    refreshed = protocol.get("current_readiness_preparation")
    if refreshed != ledger.get("current_readiness_preparation"):
        errors.append("protocol/ledger current readiness references differ")
    if refreshed is not None:
        try:
            if not file_hash(refreshed.get("manifest"), refreshed.get("manifest_sha256")):
                raise ValueError("current readiness scope or hash differs")
            if refreshed.get("kind") == "independent_freeze_authoring_not_formal_execution":
                import independent_acceptance
                errors.extend(independent_acceptance.packet_errors(root, (root / refreshed["manifest"]).parent))
                errors.extend(independent_acceptance.current_index_errors(root))
            elif refreshed.get("kind") == "WI1_select_reference_supplement_not_formal_execution":
                import wi1_select_review
                errors.extend(wi1_select_review.validate(root, (root / refreshed["manifest"]).parent))
                errors.extend(wi1_select_review.current_indices(root, protocol, ledger))
            elif refreshed.get("kind") == "WI2_source_contract_compiler_MCP_audit_not_formal_execution":
                import wi2_contract_audit
                errors.extend(wi2_contract_audit.validate(root, (root / refreshed["manifest"]).parent))
                errors.extend(wi2_contract_audit.current_indices(root, protocol, ledger))
            elif refreshed.get("kind") == "WI3_typed_failure_reason_native_MCP_not_formal_execution":
                import wi3_reason_fix
                errors.extend(wi3_reason_fix.validate(root, (root / refreshed["manifest"]).parent))
                errors.extend(wi3_reason_fix.current_indices(root, protocol, ledger))
            elif refreshed.get("kind") == "WI123_readonly_debt_diagnostic_not_formal_execution":
                import acceptance_debt_diagnostic
                errors.extend(acceptance_debt_diagnostic.validate(root, (root / refreshed["manifest"]).parent))
                errors.extend(acceptance_debt_diagnostic.current_indices(root, protocol, ledger))
            elif refreshed.get("kind") == "WIC_bounded_review_AI_DEV_not_formal_execution":
                import wic_dev_gold
                errors.extend(wic_dev_gold.validate(root, (root / refreshed["manifest"]).parent))
                errors.extend(wic_dev_gold.current_indices(root, protocol, ledger))
            elif refreshed.get("kind") == "WISP_bounded_overlay_AI_DEV_not_formal_execution":
                import wisp_dev_gold
                errors.extend(wisp_dev_gold.validate(root, (root / refreshed["manifest"]).parent))
                errors.extend(wisp_dev_gold.current_indices(root, protocol, ledger))
            elif refreshed.get("kind") == "WI6_bounded_paired_AI_DEV_not_formal_execution":
                import wi6_dev_gold
                errors.extend(wi6_dev_gold.validate(root, (root / refreshed["manifest"]).parent))
                errors.extend(wi6_dev_gold.current_indices(root, protocol, ledger))
            elif refreshed.get("kind") == "WIV1_V2_bounded_ticket_AI_DEV_not_formal_execution":
                import ticket_dev_gold
                errors.extend(ticket_dev_gold.validate(root, (root / refreshed["manifest"]).parent))
                errors.extend(ticket_dev_gold.current_indices(root, protocol, ledger))
            elif refreshed.get("kind") == "WI4_bounded_schedule_AI_DEV_not_formal_execution":
                import wi4_dev_gold
                errors.extend(wi4_dev_gold.validate(root, (root / refreshed["manifest"]).parent))
                errors.extend(wi4_dev_gold.current_indices(root, protocol, ledger))
            elif refreshed.get("kind") == "WI3_observed_consumer_AI_DEV_not_formal_execution":
                import wi3_consumer_matching
                errors.extend(wi3_consumer_matching.validate(root, (root / refreshed["manifest"]).parent))
                errors.extend(wi3_consumer_matching.current_indices(root, protocol, ledger))
            elif refreshed.get("kind") == "WI3_bounded_AI_DEV_not_formal_execution":
                import wi3_dev_gold
                errors.extend(wi3_dev_gold.validate(root, (root / refreshed["manifest"]).parent))
                errors.extend(wi3_dev_gold.current_indices(root, protocol, ledger))
            elif refreshed.get("kind") == "WI2_consumer_context_AI_DEV_not_formal_execution":
                import wi2_consumer_matching
                errors.extend(wi2_consumer_matching.validate(root, (root / refreshed["manifest"]).parent))
                errors.extend(wi2_consumer_matching.current_indices(root, protocol, ledger))
            elif refreshed.get("kind") == "WI2_complete_bounded_AI_DEV_not_formal_execution":
                import wi2_dev_gold
                errors.extend(wi2_dev_gold.validate(root, (root / refreshed["manifest"]).parent))
                errors.extend(wi2_dev_gold.current_indices(root, protocol, ledger))
            elif refreshed.get("kind") == "WI1_consumer_matching_AI_DEV_not_formal_execution":
                import wi1_consumer_matching
                errors.extend(wi1_consumer_matching.validate(root, (root / refreshed["manifest"]).parent))
                errors.extend(wi1_consumer_matching.current_indices(root, protocol, ledger))
            elif refreshed.get("kind") == "WI1_DEV_AI_gold_not_formal_execution":
                import wi1_dev_gold
                errors.extend(wi1_dev_gold.validate_packet(root, (root / refreshed["manifest"]).parent))
                errors.extend(wi1_dev_gold.current_indices(root, protocol, ledger))
            elif refreshed.get("kind") == "preparation_only_not_formal_execution":
                import importlib.util
                spec = importlib.util.spec_from_file_location("current_readiness", Path(__file__).with_name("check-acceptance-readiness.py"))
                readiness = importlib.util.module_from_spec(spec)
                spec.loader.exec_module(readiness)
                errors.extend(readiness.check((root / refreshed["manifest"]).parent, root))
            else:
                raise ValueError("current readiness scope or hash differs")
            if refreshed.get("kind") in ("independent_freeze_authoring_not_formal_execution", "WI1_DEV_AI_gold_not_formal_execution", "WI1_consumer_matching_AI_DEV_not_formal_execution", "WI2_complete_bounded_AI_DEV_not_formal_execution", "WI2_consumer_context_AI_DEV_not_formal_execution", "WI3_bounded_AI_DEV_not_formal_execution", "WI3_observed_consumer_AI_DEV_not_formal_execution", "WI4_bounded_schedule_AI_DEV_not_formal_execution", "WIV1_V2_bounded_ticket_AI_DEV_not_formal_execution", "WI6_bounded_paired_AI_DEV_not_formal_execution", "WISP_bounded_overlay_AI_DEV_not_formal_execution", "WIC_bounded_review_AI_DEV_not_formal_execution", "WI123_readonly_debt_diagnostic_not_formal_execution", "WI3_typed_failure_reason_native_MCP_not_formal_execution", "WI2_source_contract_compiler_MCP_audit_not_formal_execution", "WI1_select_reference_supplement_not_formal_execution"):
                historical = protocol.get("historical_readiness_preparation")
                if (historical != ledger.get("historical_readiness_preparation")
                        or not isinstance(historical, dict)
                        or historical.get("source_revision") != "9de07a8264bc4aea7054dcea8abb6559801d1cef"
                        or historical.get("manifest") != "system/eval/operational-preparation-2026-10-16/readiness-refresh/manifest.json"
                        or historical.get("status") != "historical_not_current_source_validation"
                        or not file_hash(historical.get("manifest"), historical.get("manifest_sha256"))):
                    raise ValueError("historical readiness evidence lost or promoted")
        except (OSError, ValueError, KeyError, TypeError) as exc:
            errors.append("current readiness preparation invalid: " + str(exc))
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
    host = protocol.get("host_connection_verification")
    if host != ledger.get("host_connection_verification"):
        errors.append("protocol/ledger local host references differ")
    if host is not None:
        try:
            if (host.get("scope") != "owned_local_session_only"
                    or not file_hash(host.get("manifest"), host.get("manifest_sha256"))):
                raise ValueError("local host scope or manifest binding differs")
            manifest_path = root / host["manifest"]
            manifest = json.loads(manifest_path.read_text())
            if (manifest.get("kind") != "local_scripted_transport_actual_host_not_model_acceptance"
                    or manifest.get("adoption_scope") != "owned_local_session_only"
                    or manifest.get("status") != "pass"
                    or manifest.get("actual_upstream_model_inference") != "not_run"
                    or manifest.get("actual_agent_pipeline_acceptance") != "not_run"
                    or manifest.get("operational_gate") != "not_run"
                    or manifest.get("quality_gate") != "not_run"
                    or manifest.get("independent_acceptance") != "not_recorded"):
                raise ValueError("local scripted host verification promoted beyond its scope")
            for artifact in manifest["artifacts"]:
                path = manifest_path.parent / artifact["path"]
                if (not path.resolve().is_relative_to(manifest_path.parent.resolve())
                        or not file_hash(path.relative_to(root).as_posix(), artifact["sha256"])):
                    raise ValueError("local host artifact missing or altered")
            import importlib.util
            spec = importlib.util.spec_from_file_location("local_host_probe", Path(__file__).with_name("operational-host-probe.py"))
            probe = importlib.util.module_from_spec(spec)
            spec.loader.exec_module(probe)
            cases = manifest["final_cases"]
            if len(cases) != 5 or {case["case"] for case in cases} != {"valid", "tools", "incomplete", "tampered", "cancel"}:
                raise ValueError("local host case coverage differs")
            for case in cases:
                folder = manifest_path.parent / "final" / case["attempt"]
                if not folder.resolve().is_relative_to(manifest_path.parent.resolve()):
                    raise ValueError("local host case path escapes packet")
                assessed = probe.assess(folder)
                if assessed.get("actual_host_version") != manifest["host"]["version"].split()[0]:
                    raise ValueError("host version identity differs")
                init = next(json.loads(line) for line in (folder / "host.stdout.jsonl").read_text().splitlines()
                            if json.loads(line).get("subtype") == "init")
                plugin = next(row for row in init["plugins"] if row["name"] == "coding-agent")
                if plugin["path"] != manifest["source_adoptions"]["coding_agent"]["plugin_root"]:
                    raise ValueError("host plugin path differs from adopted source")
                if assessed.get("status") != "PASS" or {k:v for k,v in case.items() if k != "attempt"} != assessed:
                    raise ValueError("actual host evidence disagrees with its recorded verdict")
            for patch in patches:
                component = patch["component"].replace("-", "_")
                adopted = manifest["source_adoptions"][component]
                if (patch.get("adopted_commit") != adopted["source_commit"]
                        or patch.get("adoption_scope") != "owned_local_session_only"):
                    raise ValueError("adopted source/host scope is unbound")
        except (OSError, ValueError, KeyError, TypeError, StopIteration) as exc:
            errors.append("local host readiness invalid: " + str(exc))
    source_preparation = protocol.get("policy_security_input_preparation")
    if source_preparation != ledger.get("policy_security_input_preparation"):
        errors.append("protocol/ledger policy-security preparation references differ")
    if source_preparation is not None:
        if not file_hash(source_preparation.get("manifest"), source_preparation.get("manifest_sha256")):
            errors.append("policy-security preparation manifest binding differs")
        else:
            import importlib.util
            spec = importlib.util.spec_from_file_location("source_packet_check", Path(__file__).with_name("check-policy-security-preparation.py"))
            packet_check = importlib.util.module_from_spec(spec)
            spec.loader.exec_module(packet_check)
            errors.extend(packet_check.check((root / source_preparation["manifest"]).parent, root))
    diagnosis = protocol.get("policy_reference_diagnosis")
    if diagnosis != ledger.get("policy_reference_diagnosis"):
        errors.append("protocol/ledger policy-reference diagnosis references differ")
    if diagnosis is not None:
        if (diagnosis.get("kind") != "unapproved_unapplied_ast_probe_only"
                or not file_hash(diagnosis.get("manifest"), diagnosis.get("manifest_sha256"))):
            errors.append("policy-reference diagnosis scope or manifest binding differs")
        else:
            import importlib.util
            spec = importlib.util.spec_from_file_location("policy_diagnosis_check", Path(__file__).with_name("check-policy-reference-diagnosis.py"))
            checker = importlib.util.module_from_spec(spec)
            spec.loader.exec_module(checker)
            errors.extend(checker.check((root / diagnosis["manifest"]).parent, root))
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
