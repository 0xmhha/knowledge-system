#!/usr/bin/env python3
"""Run FINAL2 part A: DEV (candidate then reference), check both captures,
then FINAL under one reservation (user decision: user-decision.json). Load
and top CPU processes are recorded before each run. FINAL starts only if
both DEV captures are complete with no errors and the 5-minute load average
is at most 10 (waits up to 60 min). Run from the repository root."""
import json, os, subprocess, time
from pathlib import Path
R = Path("system/eval/b0-knowledge-system/refactoring-final2-a-2026-10-10")
B = Path("/private/tmp/ks-n11-eval-m13oocru")
def env(name):
    rec = {"at_utc": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()), "uptime": subprocess.check_output(["uptime"], text=True).strip(),
           "load": list(os.getloadavg()),
           "top_cpu": subprocess.check_output(["ps", "-Ao", "pcpu,etime,comm", "-r"], text=True).splitlines()[:9]}
    (R / (name + "-environment.json")).write_text(json.dumps(rec, indent=2) + "\n")
def run(name):
    start = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
    cmd = (R / (name + "-command.txt")).read_text()
    with (R / (name + "-run.stdout.txt")).open("w") as out, (R / (name + "-run.stderr.txt")).open("w") as err:
        rc = subprocess.run(["zsh", "-c", cmd], stdout=out, stderr=err).returncode
    (R / (name + "-run-exit.json")).write_text(json.dumps({"started_utc": start, "ended_utc": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()), "exit_code": rc}) + "\n")
    return rc
def complete(run_dir, rows):
    out = {}
    for capture in ("capture", "reference-capture"):
        path = run_dir / capture / "report.json"
        report = json.loads(path.read_text()) if path.exists() else {}
        out[capture] = {"state": report.get("state"), "rows": report.get("rows"), "errors": report.get("errors") or []}
    out["ok"] = all(v["state"] == "captured" and v["rows"] == rows and not v["errors"] for k, v in out.items() if k != "ok")
    return out
env("dev-before")
rc = run("dev")
gate = {"dev_exit": rc, "dev": complete(B / "run-F2A-DEV", 5 * 8 * 30)}
ok = rc == 0 and gate["dev"]["ok"]
waited = 0
while ok and os.getloadavg()[1] > 10 and waited < 3600:
    time.sleep(60); waited += 60
gate.update(load_wait_s=waited, load_ok=os.getloadavg()[1] <= 10)
gate["final_started"] = ok and gate["load_ok"]
(R / "final-gate.json").write_text(json.dumps(gate, indent=2) + "\n")
if not gate["final_started"]:
    raise SystemExit("FINAL not started: " + json.dumps(gate))
env("final-before")
rc = run("final")
env("final-after")
print(json.dumps({"gate": gate, "final_exit": rc, "final": complete(B / "run-F2A-FINAL", 10 * 8 * 30)}))
raise SystemExit(rc)
