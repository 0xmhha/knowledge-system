#!/usr/bin/env python3
"""Run the clean DEV rerun, check it, then the single FINAL run (user decisions:
final-timing-decision.json, rerun-conditions-decision.json). Load and the top
CPU processes are recorded before each run. FINAL starts only if DEV3 exits 0
with a captured report and no errors, and the 5-minute load average is at most
10 (waits up to 60 min). Run from the repository root."""
import json, os, subprocess, time
from pathlib import Path
R = Path("system/eval/b0-knowledge-system/refactoring-n11-evaluation-run-2026-10-09")
B = Path("/private/tmp/ks-n11-eval-m13oocru")
def env(name):
    rec = {"at_utc": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()), "uptime": subprocess.check_output(["uptime"], text=True).strip(),
           "load": list(os.getloadavg()),
           "top_cpu": subprocess.check_output(["ps", "-Ao", "pcpu,etime,comm", "-r"], text=True).splitlines()[:9]}
    (R / (name + "-environment.json")).write_text(json.dumps(rec, indent=2) + "\n"); return rec
def run(name):
    start = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
    cmd = (R / (name + "-command.txt")).read_text()
    with (R / (name + "-run.stdout.txt")).open("w") as out, (R / (name + "-run.stderr.txt")).open("w") as err:
        rc = subprocess.run(["zsh", "-c", cmd], stdout=out, stderr=err).returncode
    (R / (name + "-run-exit.json")).write_text(json.dumps({"started_utc": start, "ended_utc": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()), "exit_code": rc}) + "\n")
    return rc
env("dev3-before")
rc = run("dev3")
report = json.loads((B / "run-DEV3/capture/report.json").read_text())
ok = rc == 0 and report["state"] == "captured" and not report.get("errors") and report["rows"] == 1680
gate = {"dev3_exit": rc, "dev3_state": report["state"], "dev3_errors": len(report.get("errors") or []), "dev3_rows": report["rows"], "dev3_ok": ok}
waited = 0
while ok and os.getloadavg()[1] > 10 and waited < 3600:
    time.sleep(60); waited += 60
gate["load_wait_s"] = waited; gate["load_ok"] = os.getloadavg()[1] <= 10
gate["final_started"] = ok and gate["load_ok"]
(R / "final-gate.json").write_text(json.dumps(gate, indent=2) + "\n")
if not gate["final_started"]:
    raise SystemExit("FINAL not started: " + json.dumps(gate))
env("final-before")
rc = run("final")
env("final-after")
print(json.dumps({"dev3": gate, "final_exit": rc}))
raise SystemExit(rc)
