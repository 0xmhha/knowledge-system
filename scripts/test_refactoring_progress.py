"""Exercise partial completion checks against actual committed project evidence."""
import copy
import importlib.util
import json
from pathlib import Path
import unittest
from unittest import mock
import contextlib
import io

ROOT = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location("progress", ROOT / "scripts/check-refactoring-progress.py")
progress = importlib.util.module_from_spec(spec)
spec.loader.exec_module(progress)


# Last revision where N-15 and N-17 still carried partial check records;
# later completions must not change what these fixtures exercise.
FIXTURE_REVISION = "63bf48a144c338e0246cb502a2149480ffe373bd"


class PartialCompletionTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.ledger = json.loads(progress.recorded(FIXTURE_REVISION, "docs/spec-driven/REFACTORING-PROGRESS.json"))
        cls.live = progress.recorded(FIXTURE_REVISION, progress.SPEC).decode()

    def task(self, ident="N-15"):
        return copy.deepcopy(next(t for t in self.ledger["tasks"] if t["id"] == ident))

    def test_actual_partial_evidence_passes_without_completing_tasks(self):
        self.assertEqual(progress.verify_partial_checks(self.task(), self.live), 3)
        self.assertEqual(progress.verify_partial_checks(self.task("N-17"), self.live), 2)
        self.assertNotEqual(self.task()["state"], "complete")
        self.assertNotEqual(self.task("N-17")["state"], "complete")

    def test_original_missing_native_receipt_and_not_run_rejected(self):
        task = self.task()
        task.update(native_ci_evidence_revision="0" * 40, native_ci_manifest="missing-proof.json", ci_status="not_run")
        with self.assertRaisesRegex(ValueError, "reference disagrees"):
            progress.verify_partial_checks(task, self.live)

    def test_missing_check_record_rejected(self):
        task = self.task()
        task["verified_checks"].pop()
        with self.assertRaisesRegex(ValueError, "lack evidence"):
            progress.verify_partial_checks(task, self.live)

    def test_duplicate_and_foreign_check_rejected(self):
        for foreign in (False, True):
            with self.subTest(foreign=foreign):
                task = self.task()
                row = copy.deepcopy(task["verified_checks"][0])
                if foreign:
                    row["check"] = "N17-A"
                task["verified_checks"].append(row)
                with self.assertRaisesRegex(ValueError, "invalid or duplicate"):
                    progress.verify_partial_checks(task, self.live)

    def test_binding_and_scope_cannot_be_removed(self):
        for field in ("scope", "manifest", "evidence_revision", "evidence_ref"):
            with self.subTest(field=field):
                task = self.task()
                task["verified_checks"][-1].pop(field)
                with self.assertRaisesRegex(ValueError, "binding/scope missing"):
                    progress.verify_partial_checks(task, self.live)

    def test_unchecked_operating_criterion_cannot_reuse_native_receipt(self):
        task = self.task()
        task["verified_checks"][-1]["check"] = "N15-D"
        live = self.live.replace("[x] N15-C", "[ ] N15-C").replace("[ ] N15-D", "[x] N15-D")
        with self.assertRaisesRegex(ValueError, "not checked in bound historical specification"):
            progress.verify_partial_checks(task, live)

    def test_ci_execution_status_and_run_cannot_drift(self):
        for field, value in (("ci_status", "not_run"), ("native_ci_run", 1)):
            with self.subTest(field=field):
                task = self.task()
                task[field] = value
                with self.assertRaisesRegex(ValueError, "ledger field disagrees"):
                    progress.verify_partial_checks(task, self.live)

    def test_bad_commit_or_artifact_is_rejected_even_with_matching_reference(self):
        for revision, path in (("0" * 40, self.task()["native_ci_manifest"]),
                               (self.task()["native_ci_evidence_revision"], "missing-proof.json")):
            with self.subTest(revision=revision, path=path):
                task = self.task()
                task.update(native_ci_evidence_revision=revision, native_ci_manifest=path)
                row = task["verified_checks"][-1]
                row.update(evidence_revision=revision, manifest=path)
                with self.assertRaises(progress.subprocess.CalledProcessError):
                    progress.verify_partial_checks(task, self.live)


class OntologyProgressTests(unittest.TestCase):
 def fixture(self):
  tasks=[{"id":f"O-{i:02d}","state":"pending"}for i in range(1,5)]
  worklist="0/4\n"+"\n".join(f"| {t['id']} | pending |"for t in tasks)
  return {"ontology_tasks":tasks,"ontology_next_task":"O-01"},worklist
 def check(self,ledger,worklist):
  with contextlib.redirect_stdout(io.StringIO()),mock.patch.object(progress,"verify_partial_checks",return_value=0),mock.patch.object(progress,"verify_manifest")as verify:
   progress.verify_ontology_progress(ledger,worklist,"",{"pending","complete"},1)
   return verify.call_count
 def test_pending_is_not_completion(self):
  l,w=self.fixture();self.assertEqual(self.check(l,w),0)
 def test_missing_duplicate_and_completed_next_rejected(self):
  for kind in ['absent','duplicate','next']:
   with self.subTest(kind=kind):
    l,w=self.fixture()
    if kind=='absent':l.pop('ontology_tasks')
    elif kind=='duplicate':l['ontology_tasks'][1]['id']='O-01'
    else:l['ontology_tasks'][0]['state']='complete';w=w.replace('0/4','1/4').replace('| O-01 | pending |','| O-01 | 완료 |')
    with self.assertRaises(ValueError):self.check(l,w)
 def test_complete_requires_manifest_and_agrees_with_table(self):
  l,w=self.fixture();l['ontology_tasks'][0]['state']='complete';l['ontology_next_task']='O-02'
  with self.assertRaises(ValueError):self.check(l,w)
  w=w.replace('0/4','1/4').replace('| O-01 | pending |','| O-01 | 완료 |')
  self.assertEqual(self.check(l,w),1)


if __name__ == "__main__":
    unittest.main()
