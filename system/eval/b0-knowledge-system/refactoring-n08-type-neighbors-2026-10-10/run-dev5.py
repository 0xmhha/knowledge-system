"""Development DEV measurement of the uses_type declaration neighbors (N08 candidate B) with request-scoped verification.

Runs the same `cks eval matrix` command the N11 runner dispatches, with the
DEV3 requests, the frozen runtime config and the protocol repetition counts,
but with binary cks-7077741f. It is not a frozen evaluation: the protocol and
review record are not rebound. Records host load before and after."""
import json, os, subprocess, time
from pathlib import Path
HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[3]
BASE = Path("/private/tmp/ks-n11-eval-m13oocru")
BIN = BASE / "bin/cks-7077741f"
OUT = BASE / "run-DEV5-n08"

def env(name):
    top = subprocess.run(["ps", "-Ao", "%cpu,etime,comm", "-r"], capture_output=True, text=True).stdout.splitlines()[:9]
    rec = {"at_utc": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()), "uptime": subprocess.check_output(["uptime"], text=True).strip(),
           "load": list(os.getloadavg()), "top_cpu": top}
    (HERE / f"{name}-environment.json").write_text(json.dumps(rec, indent=1) + "\n")

argv = [str(BIN), "eval", "matrix", "--requests", str(BASE / "run-DEV3/requests.json"),
        "--config", str(ROOT / "system/eval/b0-knowledge-system/refactoring-n11-eval-corpus-2026-10-09/runtime-config.yaml"),
        "--cks-mcp", str(BIN), "--output", str(OUT / "capture"), "--environment-ledger",
        "--warmup", "2", "--retrieval-runs", "5", "--warm-runs", "20", "--cold-runs", "3"]
OUT.mkdir()
(HERE / "dev5-command.json").write_text(json.dumps({"cwd": str(BASE / "source"), "argv": argv}, indent=1) + "\n")
env("dev5-before")
start = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
with open(HERE / "dev5-run.stdout.txt", "w") as so, open(HERE / "dev5-run.stderr.txt", "w") as se:
    code = subprocess.run(argv, cwd=BASE / "source", stdout=so, stderr=se).returncode
(HERE / "dev5-run-exit.json").write_text(json.dumps({"started_utc": start, "ended_utc": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()), "exit_code": code}) + "\n")
env("dev5-after")
print(json.dumps({"exit": code}))
