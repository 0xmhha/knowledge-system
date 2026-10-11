"""Build the source-binding manifest for the N12 criterion-gap test step.

Starts from the previous active source map, re-hashes every bound path from
the working tree, adds any new cmd/internal/pkg Go file, and records which
paths changed. Run from the repository root.
"""
import hashlib, json, subprocess, sys
from pathlib import Path

D = Path("system/eval/b0-knowledge-system/refactoring-n12-criterion-gap-tests-2026-10-09")
PREV_REV = "eb70e2eca7cfa19de8207e8a2aebc1cc985cee9f"
PREV = "system/eval/b0-knowledge-system/refactoring-n12-anchor-binding-2026-10-09/manifest.json"
sha = lambda p: hashlib.sha256(Path(p).read_bytes()).hexdigest()

prev = json.loads(subprocess.check_output(["git", "show", f"{PREV_REV}:{PREV}"]))
inv = subprocess.check_output(["git", "ls-files", "--cached", "--others", "--exclude-standard", "*.go"], text=True)
live_go = {p for p in inv.splitlines() if p.startswith(("cmd/", "internal/", "pkg/"))}
paths = {p for p in prev["source_sha256"] if not p.endswith(".go")} | live_go
sources = {p: sha(p) for p in sorted(paths)}
changed = {p: d for p, d in sources.items() if prev["source_sha256"].get(p) != d}
approved = {p: sha(p) for p in prev["approved_input_sha256"]}
assert approved == prev["approved_input_sha256"], "approved inputs changed"
reports = ["docs/spec-driven/N12-CRITERION-GAP-PREREVIEW.md", "docs/spec-driven/REFACTORING-REMAINING-WORKLIST.md",
           "docs/spec-driven/REFACTORING-EXECUTION-SPEC.md"]
evidence = ["prereview.json", "mutation-checks.json", "related-tests-summary.json", "related-race.jsonl.gz", "build-manifest.py"]
manifest = {
    "schema_version": 1,
    "status": "N12_criterion_gap_tests_and_assistant_prereview_verified_individual_human_pending",
    "previous_active_binding": {"revision": PREV_REV, "manifest": PREV},
    "source_sha256": sources,
    "approved_input_sha256": approved,
    "changed_source_sha256": changed,
    "reports_sha256": {p: sha(p) for p in reports},
    "evidence_sha256": {p: sha(D / p) for p in evidence},
    "reviewed_candidate_identity_unchanged": {"commit": "36353d32f1c21fc0cb2622807fa9e5d05f2daac7",
        "snapshot_id": "672eae93464af931deb5b66605ba6d21615e50ced58c5a09cd82befc12f56ae7",
        "dataset_id": "1e82983e45ea35eb63debbea9bf88b6961d595cb5c540db312430cc47d19a341"},
    "new_tests_in_reviewed_candidate": False,
    "product_code_changed": False,
    "test_code_changed": True,
    "related_race_tests": 132, "related_race_subtests": 102,
    "assistant_prereview_is_human_decision": False,
    "actual_human_decisions": 0, "reviewer_name": None, "actual_human_active_review_ns": None,
    "actual_semantic_linked_runs": 0, "actual_FINAL_dispatch_count": 0, "actual_pilot_activation_count": 0,
    "model_calls": 0, "full_task_complete": False,
}
(D / "manifest.json").write_text(json.dumps(manifest, ensure_ascii=False, indent=1) + "\n")
print(len(sources), "sources;", "changed:", sorted(changed))
