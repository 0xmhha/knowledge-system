"""N14 decisions, rejection control, promotion and rollback on the owned dataset.

1. Rejection control: a separate patch ID on the same result version gets the
   same runs and one agent-written rejected decision (explicitly not a human
   judgment); promote must refuse and current must stay on base.
2. The user's decisions (label "opus", all four approved, reasons adopted from
   the agent's draft) are recorded on the real patch, which is then promoted.
3. rollback to base and back to result through the public CLI."""
import json, subprocess
from pathlib import Path
E = Path("system/eval/b0-knowledge-system/refactoring-n14-change-acceptance-2026-10-10")
prep = json.loads((E / "preparation.json").read_text()); base = Path(prep["base"]); ds = prep["dataset_root"]
cks, store, runs = str(base / "bin/cks"), str(base / "semantic-result.sqlite"), base / "linked-runs"
def run(*args):
    r = subprocess.run([cks, *args], capture_output=True, text=True)
    return {"argv": ["cks", *args], "exit_code": r.returncode, "stdout_tail": r.stdout.strip()[-400:], "stderr_tail": r.stderr.strip()[-300:]}
current = lambda: subprocess.check_output(["readlink", ds + "/current"], text=True).strip()
RUNS = {"ac-filtered-recall": "F3-T03", "ac-filtered-completeness": "F3-T05", "ac-source-evidence": "F3-T12", "ac-ontology-fallback": "F3-T15"}
REASONS = {
    "ac-filtered-recall": "The change does not touch CKV search code; F3-T03 passes on the result snapshot.",
    "ac-filtered-completeness": "No search code change; F3-T05, F3-T07, F3-R02 and F3-R04 pass on the result snapshot.",
    "ac-source-evidence": "The change alters the retained evidence validation path; F3-T12 confirms refusal and active preservation on the result snapshot.",
    "ac-ontology-fallback": "No ontology path change; F3-T15, F3-T17 and F3-R14 pass on the result snapshot."}
log = {"current_start": current()}
assert log["current_start"] == "base"
C = "rejection-control-7c57a388"
ctl = [run("patch", "register", "--dataset", ds, "--version", "result", "--patch-id", C)]
for crit, rid in RUNS.items():
    ctl.append(run("patch", "record-run", "--dataset", ds, "--patch-id", C, "--semantic-store", store, "--report", str(runs / (rid + ".json"))))
ctl.append(run("patch", "decide", "--dataset", ds, "--patch-id", C, "--semantic-store", store, "--decision-id", "control-reject-source-evidence",
               "--criterion-id", "ac-source-evidence", "--reviewer", "rejection-control (agent synthetic; not a human judgment)",
               "--outcome", "rejected", "--reason", "Synthetic control: verifies that any rejected criterion holds the candidate."))
assert all(s["exit_code"] == 0 for s in ctl), ctl
refused = run("patch", "promote", "--dataset", ds, "--patch-id", C, "--semantic-store", store)
assert refused["exit_code"] != 0 and current() == "base", refused
log["rejection_control"] = {"steps": ctl, "promote": refused, "current_after": current()}
P = "retained-batch-7c57a388"
dec = []
for crit in RUNS:
    dec.append(run("patch", "decide", "--dataset", ds, "--patch-id", P, "--semantic-store", store, "--decision-id", "opus-" + crit,
                   "--criterion-id", crit, "--reviewer", "opus", "--outcome", "approved", "--reason", REASONS[crit]))
assert all(s["exit_code"] == 0 for s in dec), dec
log["decisions"] = dec
promoted = run("patch", "promote", "--dataset", ds, "--patch-id", P, "--semantic-store", store)
assert promoted["exit_code"] == 0 and current() == "result", promoted
log["promote"] = promoted; log["current_after_promote"] = current()
log["status_after_promote"] = run("patch", "status", "--dataset", ds, "--patch-id", P)
back = run("rollback", "base", "--out", ds, "--project-id", prep["project_id"])
log["rollback_to_base"] = back; log["current_after_rollback"] = current()
fwd = run("rollback", "result", "--out", ds, "--project-id", prep["project_id"])
log["rollforward_to_result"] = fwd; log["current_final"] = current()
(E / "accept.json").write_text(json.dumps(log, indent=2, ensure_ascii=False) + "\n")
print(json.dumps({k: log[k] for k in ("current_start", "current_after_promote", "current_after_rollback", "current_final")},
                 ) + " rollback rc=%d fwd rc=%d" % (back["exit_code"], fwd["exit_code"]))
