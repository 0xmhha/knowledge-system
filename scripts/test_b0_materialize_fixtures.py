import copy
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest


SPEC = importlib.util.spec_from_file_location("b0_materialize", Path(__file__).with_name("b0-materialize-fixtures.py"))
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)


class MaterializationTests(unittest.TestCase):
    def setUp(self):
        self.book = json.loads(MODULE.DEFAULT.read_bytes())

    def test_review_is_required_and_not_manufactured(self):
        # Explicit draft prerequisite; real repository inputs may be approved.
        self.book['status'] = 'proposal_pending_human_review'
        for fixture in self.book['fixtures']:
            fixture.update(review_state='draft', reviewer=None, reviewed_at=None)
        with self.assertRaisesRegex(ValueError, "review is pending"):
            MODULE.validate(self.book, "development", False)
        incomplete = copy.deepcopy(self.book)
        for fixture in incomplete["fixtures"]:
            fixture["review_state"] = "approved"
        with self.assertRaisesRegex(ValueError, "review is pending"):
            MODULE.validate(incomplete, "development", False)
        with tempfile.TemporaryDirectory() as scratch:
            draft = Path(scratch) / 'draft.json'
            draft.write_text(json.dumps(self.book))
            result = MODULE.materialize(draft, Path(scratch) / "fixtures", allow_draft=True)
            self.assertTrue(result["diagnostic_only"])
            self.assertIsNone(result["quality_metrics"])
            self.assertTrue(all(f["review_state"] == "draft" and f["reviewer"] is None for f in result["fixtures"]))

    def test_changed_source_and_path_escape_fail_before_writes(self):
        changed = copy.deepcopy(self.book)
        changed["fixtures"][0]["sources"]["go.mod"] += "// tampered\n"
        with self.assertRaisesRegex(ValueError, "hash mismatch"):
            MODULE.validate(changed, "development", True)
        for unsafe in ["../escape.go", "/absolute.go", ".git/config", "nested/.git/config", "x//y.go", "x\\y.go"]:
            with self.subTest(path=unsafe), self.assertRaises(ValueError):
                MODULE.safe_path(unsafe)

    def test_reproducible_commits_independent_projects_and_state_history(self):
        with tempfile.TemporaryDirectory() as scratch:
            left, right = Path(scratch) / "left", Path(scratch) / "right"
            a = MODULE.materialize(MODULE.DEFAULT, left, allow_draft=True)
            b = MODULE.materialize(MODULE.DEFAULT, right, allow_draft=True)
            self.assertEqual(a, b)
            for fixture in a["fixtures"]:
                for state in fixture["states"]:
                    repo = left / state["repository"]
                    self.assertFalse((repo / ".git/objects/info/alternates").exists())
                    tracked = set(MODULE.git(repo, "ls-tree", "-r", "--name-only", state["commit"]).splitlines())
                    self.assertEqual(tracked, set(state["source_sha256"]))
                    if fixture["family"] == "F-03":
                        ontology = MODULE.git(repo, "show", state["commit"] + ":ontology.yaml")
                        self.assertIn("project_id: " + state["project_id"] + "\n", ontology)

                    for relative, expected in state["source_sha256"].items():
                        actual = MODULE.git(repo, "show", state["commit"] + ":" + relative, raw=True)
                        self.assertEqual(MODULE.digest(actual), expected)
                    self.assertEqual(MODULE.git(repo, "status", "--porcelain"), "")
            old, new = next(f for f in a["fixtures"] if f["family"] == "F-04")["states"]
            repo = left / old["repository"]
            self.assertEqual(old["project_id"], new["project_id"])
            self.assertEqual(MODULE.git(repo, "rev-parse", new["commit"] + "^"), old["commit"])
            self.assertNotEqual(MODULE.git(repo, "show", old["commit"] + ":main.go"), MODULE.git(repo, "show", new["commit"] + ":main.go"))
            pa, pb = next(f for f in a["fixtures"] if f["family"] == "F-05")["states"]
            self.assertNotEqual(pa["project_id"], pb["project_id"])
            self.assertNotEqual(pa["repository"], pb["repository"])
            self.assertEqual(set(pa["source_sha256"]), set(pb["source_sha256"]))
            with self.assertRaisesRegex(ValueError, "output already exists"):
                MODULE.materialize(MODULE.DEFAULT, left, allow_draft=True)
            self.assertEqual(json.loads((left / "materialization.json").read_text()), a)


if __name__ == "__main__":
    unittest.main()
