"""Structural control fixtures only; not real independent evaluation cases."""
import copy
import importlib.util
import json
from pathlib import Path
import unittest

SCRIPT = Path(__file__).with_name("refactoring-eval-input-check.py")
SPEC = importlib.util.spec_from_file_location("refactoring_input_check", SCRIPT)
GATE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(GATE)
BASE = (SCRIPT.parents[1] / "system/eval/b0-knowledge-system/protocol-m2max-draft.json").read_bytes()
SOURCE = b"fixture source\n"
COMMIT = "a" * 40


def encode(value):
    return json.dumps(value, sort_keys=True).encode()


def fixture(n=10):
    # Every human record here is explicitly a synthetic validation fixture.
    protocol = json.loads(BASE)
    protocol.update(status="approved", schema_version=1, source_commit=COMMIT,
                    approved_original_protocol_sha256=GATE.sha(BASE),
                    important_groups=sorted(GATE.GROUPS), results=None,
                    previous_observed_final_excluded=protocol["final_questions"] + protocol["synthetic_final_fixtures"])
    cases = [{"id": "DEV-example", "split": "DEV", "group": "CODE", "cluster_id": "dev-fixture", "unit_id": "dev-unit"}]
    for group in sorted(GATE.GROUPS):
        for i in range(n):
            cases.append({"id": f"FINAL-{group}-{i}", "split": "FINAL", "group": group,
                          "cluster_id": f"fixture-{group}-{i}", "unit_id": f"fixture-unit-{group}-{i}"})
    for q in cases:
        q.update(review_state="approved", source_commit=COMMIT, prompt=q["id"], candidate_answer="fixture gold")
        if q["group"] == "ABS":
            q.update(expected_behavior="abstain", evidence=[], strict_zero_citations=True, abstention_basis="synthetic negative scope")
        else:
            q.update(expected_behavior="cite", evidence=[{"path": "example.go", "first": 1, "last": 1,
                     "file_sha256": GATE.sha(SOURCE), "content_sha256": GATE.sha(SOURCE)}])
    return protocol, {"schema_version": 1, "status": "approved", "source_commit": COMMIT, "questions": cases, "results": None}


def review(protocol, book):
    return {"status": "approved", "scope": "fresh-evaluation-inputs", "reviewer": "synthetic-control-fixture",
            "reviewed_at": "2026-01-01T00:00:00Z", "frozen_at": "2026-01-02T00:00:00Z",
            "protocol_sha256": GATE.sha(encode(protocol)), "questions_sha256": GATE.sha(encode(book)),
            "cases": {q["id"]: {"gold_approved": True, "independence_approved": True, "contamination_checked": True,
                                  "unit_id": q["unit_id"], "cluster_id": q["cluster_id"]} for q in book["questions"]}}


def run(protocol, book, record=None, observations=(), source=SOURCE):
    return GATE.audit(encode(protocol), encode(book), BASE, record if record is not None else review(protocol, book), observations,
                      lambda commit, path: source if commit == COMMIT and path == "example.go" else b"wrong source")


class InputAuditTests(unittest.TestCase):
    def test_valid_binding_is_not_live_execution_or_release_permission(self):
        p, b = fixture()
        result = run(p, b)
        self.assertTrue(result["input_ready"], result)
        self.assertEqual(set(result["final_independent_clusters_by_group"].values()), {10})
        self.assertEqual(result["sampling_verdict"], "sufficient_recorded_units")
        self.assertFalse(result["execution_ready"])
        self.assertFalse(result["product_release_approved"])

    def test_repeats_translations_do_not_raise_nine_units_to_ten(self):
        p, b = fixture(9)
        for q in list(b["questions"]):
            duplicate = copy.deepcopy(q)
            duplicate.update(id=q["id"] + "-ko-repeat", prompt=q["prompt"] + " translated")
            b["questions"].append(duplicate)
        result = run(p, b)
        self.assertTrue(result["input_ready"], result)
        self.assertEqual(set(result["final_independent_clusters_by_group"].values()), {9})
        self.assertEqual(result["sampling_verdict"], "inconclusive")

    def test_dev_final_overlap_and_cluster_laundering_rejected(self):
        for mode in ("overlap", "split-unit", "same-prompt"):
            p, b = fixture()
            if mode == "overlap":
                b["questions"][1]["cluster_id"] = b["questions"][0]["cluster_id"]
            elif mode == "split-unit":
                b["questions"][2]["unit_id"] = b["questions"][1]["unit_id"]
            else:
                b["questions"][2]["prompt"] = b["questions"][1]["prompt"].swapcase()
            with self.subTest(mode=mode):
                self.assertFalse(run(p, b)["input_ready"])

    def test_unreviewed_case_and_missing_human_cluster_judgment_rejected(self):
        for key in ("gold_approved", "independence_approved", "contamination_checked"):
            p, b = fixture()
            record = review(p, b)
            record["cases"][b["questions"][1]["id"]][key] = False
            self.assertFalse(run(p, b, record)["input_ready"])
        p, b = fixture()
        b["questions"][0]["review_state"] = "draft"
        self.assertFalse(run(p, b)["input_ready"])

    def test_new_id_cannot_reuse_declared_old_final_fact(self):
        p, b = fixture()
        b["questions"][1]["previous_observed_final_unit"] = p["previous_observed_final_excluded"][0]
        self.assertIn("old_final_reused:" + b["questions"][1]["id"], run(p, b)["pending_reasons"])
        p, b = fixture()
        p["previous_observed_final_excluded"] = []
        self.assertIn("old_final_exclusions_incomplete", run(p, b)["pending_reasons"])

    def test_observed_final_and_post_observation_freeze_rejected(self):
        p, b = fixture()
        observation = {"first_observed_at": "2026-01-01T12:00:00Z", "final_clusters": [b["questions"][1]["cluster_id"]],
                       "questions_sha256": GATE.sha(encode(b)), "protocol_sha256": GATE.sha(encode(p))}
        result = run(p, b, observations=[observation])
        self.assertFalse(result["input_ready"])
        self.assertIn("FINAL_already_observed", result["pending_reasons"])
        self.assertIn("observation_before_freeze", result["pending_reasons"])
        observation.update(final_clusters=["unrelated-old-cluster"], questions_sha256="b"*64, protocol_sha256="c"*64)
        self.assertTrue(run(p, b, observations=[observation])["input_ready"])

    def test_changed_gold_or_protocol_after_freeze_rejected(self):
        p, b = fixture()
        record = review(p, b)
        b["questions"][1]["candidate_answer"] = "changed after review"
        self.assertIn("freeze_binding_mismatch", run(p, b, record)["pending_reasons"])
        p, b = fixture()
        p["thresholds"]["warm_p95_ratio_max"] = 99
        self.assertIn("baseline_changed:thresholds", run(p, b)["pending_reasons"])

    def test_model_arm_gold_span_and_strict_abstention_drift_rejected(self):
        for mode in ("model", "arms", "source", "span", "abstention", "source-revision"):
            p, b = fixture()
            if mode == "model": p["model"]["digest"] = "b"*64
            if mode == "arms": p["pack_axis"] = ["on"]
            if mode == "span": b["questions"][0]["evidence"][0]["last"] = 2
            if mode == "source-revision": b["questions"][0]["source_commit"] = "c"*40
            if mode == "abstention": next(q for q in b["questions"] if q["group"] == "ABS")["strict_zero_citations"] = False
            with self.subTest(mode=mode):
                result = run(p, b, source=b"changed\n" if mode == "source" else SOURCE)
                self.assertFalse(result["input_ready"])

    def test_retained_line_coordinates_preserve_crlf_and_bare_cr(self):
        p, b = fixture()
        source = b"first\rinside\r\nsecond\n"
        for q in b["questions"]:
            if q["evidence"]:
                q["evidence"][0].update(first=1, last=1, file_sha256=GATE.sha(source), content_sha256=GATE.sha(b"first\rinside\r\n"))
        self.assertTrue(run(p, b, source=source)["input_ready"])

    def test_ambiguous_json_keys_and_bool_coordinate_rejected(self):
        with self.assertRaisesRegex(ValueError, "duplicate JSON key"):
            GATE.decode(b'{"status":"approved","status":"draft"}')
        p, b = fixture()
        b["questions"][0]["evidence"][0]["first"] = True
        self.assertFalse(run(p, b)["input_ready"])


if __name__ == "__main__":
    unittest.main()
