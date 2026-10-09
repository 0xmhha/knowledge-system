"""Refusal controls, then one recorded run per criterion via cks patch record-run."""
import json, subprocess, tempfile
from pathlib import Path
E = Path("system/eval/b0-knowledge-system/refactoring-n14-change-acceptance-2026-10-10")
N12 = Path("system/eval/b0-knowledge-system/refactoring-n12-reviewed-activation-2026-10-09")
prep = json.loads((E / "preparation.json").read_text()); base = Path(prep["base"]); runs = base / "linked-runs"
PATCH = "retained-batch-7c57a388"
def record(report):
    r = subprocess.run([str(base / "bin/cks"), "patch", "record-run", "--dataset", prep["dataset_root"], "--patch-id", PATCH,
                        "--semantic-store", str(base / "semantic-result.sqlite"), "--report", str(report)], capture_output=True, text=True)
    return r.returncode, r.stderr.strip()[-200:]
tmp = Path(tempfile.mkdtemp(prefix="n14-controls-", dir=str(base)))
good = json.loads((runs / "F3-T12.json").read_text())
controls = {}
controls["stale_base_snapshot_run"] = record(N12 / "linked-run-F3-T12.json")
wrong = dict(good, criterion_id="ac-filtered-recall"); (tmp / "wrong.json").write_text(json.dumps(wrong))
controls["criterion_swapped"] = record(tmp / "wrong.json")
cmd_only = dict(good, test_observed=False, test_passed=False); (tmp / "cmd.json").write_text(json.dumps(cmd_only))
controls["command_only_not_observed"] = record(tmp / "cmd.json")
unrelated = json.loads((runs / "F3-T03.json").read_text()); unrelated["criterion_id"] = "ac-source-evidence"
(tmp / "unrelated.json").write_text(json.dumps(unrelated))
controls["unrelated_test_for_criterion"] = record(tmp / "unrelated.json")
assert all(code != 0 for code, _ in controls.values()), controls
assert not (Path(prep["dataset_root"]) / ".patches" / PATCH / "runs").exists() or not any((Path(prep["dataset_root"]) / ".patches" / PATCH / "runs").iterdir())
recorded = {}
for criterion, rid in [("ac-filtered-recall", "F3-T03"), ("ac-filtered-completeness", "F3-T05"), ("ac-source-evidence", "F3-T12"), ("ac-ontology-fallback", "F3-T15")]:
    code, err = record(runs / (rid + ".json")); assert code == 0, (rid, err)
    recorded[criterion] = rid
current = subprocess.check_output(["readlink", str(Path(prep["dataset_root"]) / "current")], text=True).strip()
out = {"refusal_controls": {k: {"exit_code": c, "stderr_tail": e} for k, (c, e) in controls.items()},
       "recorded_runs": recorded, "current_after": current, "decisions_recorded": 0}
(E / "record-runs.json").write_text(json.dumps(out, indent=2) + "\n"); print(json.dumps(out))
