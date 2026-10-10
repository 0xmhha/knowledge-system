#!/usr/bin/env python3
"""Wait for an idle machine, then run FINAL2 part A and, if part A completes,
part B (user decision in v2-attempt1-diagnosis.json). Idle means the 5-minute
load average is at most 6; the wait is capped at 12 hours. The zero-error
gates inside each part are unchanged. Run from the repository root."""
import json, os, subprocess, time
from pathlib import Path
A = Path("system/eval/b0-knowledge-system/refactoring-final2-a-2026-10-10")
B = Path("system/eval/b0-knowledge-system/refactoring-final2-b-2026-10-10")
start, waited, samples = time.time(), 0, []
while os.getloadavg()[1] > 6 and waited < 12 * 3600:
    if waited % 600 == 0:
        samples.append({"t_s": waited, "load": [round(x, 2) for x in os.getloadavg()]})
    time.sleep(60); waited += 60
idle = {"waited_s": waited, "load_at_start": [round(x, 2) for x in os.getloadavg()], "idle_reached": os.getloadavg()[1] <= 6, "samples": samples}
(A / "idle-wait.json").write_text(json.dumps(idle, indent=1) + "\n")
if not idle["idle_reached"]:
    raise SystemExit("idle not reached within 12 h")
a = subprocess.run(["python3", str(A / "run-dev-final.py")]).returncode
if a != 0:
    raise SystemExit(f"part A exited {a}; part B not started")
b = subprocess.run(["python3", str(B / "run-dev-final.py")]).returncode
print(json.dumps({"idle": idle, "part_a_exit": a, "part_b_exit": b})); raise SystemExit(b)
