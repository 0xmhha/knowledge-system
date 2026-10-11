#!/usr/bin/env python3
"""Run FINAL2 part B: DEV, check the capture, then FINAL (user decision:
refactoring-final2-a-2026-10-10/user-decision.json). Part B latency is not
judged, so there is no load gate; the environment is still recorded.
Run from the repository root after part A finishes."""
import json, os, subprocess, time
from pathlib import Path
R = Path("system/eval/b0-knowledge-system/refactoring-final2-b-2026-10-10")
B = Path(json.loads((R / "preparation.json").read_text())["base"])
def env(name):
    rec = {"at_utc": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()), "uptime": subprocess.check_output(["uptime"], text=True).strip(),
           "load": list(os.getloadavg()), "top_cpu": subprocess.check_output(["ps", "-Ao", "pcpu,etime,comm", "-r"], text=True).splitlines()[:9]}
    (R / (name + "-environment.json")).write_text(json.dumps(rec, indent=2) + "\n")
def run(name):
    start = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
    with (R / (name + "-run.stdout.txt")).open("w") as out, (R / (name + "-run.stderr.txt")).open("w") as err:
        rc = subprocess.run(["zsh", "-c", (R / (name + "-command.txt")).read_text()], stdout=out, stderr=err).returncode
    (R / (name + "-run-exit.json")).write_text(json.dumps({"started_utc": start, "ended_utc": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()), "exit_code": rc}) + "\n")
    return rc
env("dev-before"); rc = run("dev")
report = json.loads((B / "run-F2B-DEV/capture/report.json").read_text()) if (B / "run-F2B-DEV/capture/report.json").exists() else {}
gate = {"dev_exit": rc, "state": report.get("state"), "rows": report.get("rows"), "errors": report.get("errors") or []}
gate["final_started"] = rc == 0 and gate["state"] == "captured" and gate["rows"] == 1 * 8 * 30 and not gate["errors"]
(R / "final-gate.json").write_text(json.dumps(gate, indent=2) + "\n")
if not gate["final_started"]:
    raise SystemExit("FINAL not started: " + json.dumps(gate))
env("final-before"); rc = run("final"); env("final-after")
print(json.dumps({"gate": gate, "final_exit": rc})); raise SystemExit(rc)
