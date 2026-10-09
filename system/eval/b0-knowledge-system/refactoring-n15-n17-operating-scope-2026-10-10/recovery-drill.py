#!/usr/bin/env python3
"""N17 drill on the operating go-stablenet dataset (local personal scope).
1 backup the current version to the chosen backup directory (same Mac);
2 simulate loss by renaming the dataset root (nothing is deleted);
3 restore from backup into a fresh dataset root and run `cks rollback` to
  recreate current and the reader lease; 4 verify identity, retained source and
  a real MCP query. RTO is measured from loss to the first successful query.
Usage: recovery-drill.py <release>"""
import hashlib, json, os, shutil, subprocess, sys, time
from pathlib import Path
P = Path(__file__).resolve().parent
OP = Path("/Users/kevin/work/github/0xmhha/auto-coding/knowledge-data/ks-operating")
BK = Path("/Users/kevin/work/github/0xmhha/auto-coding/knowledge-data/ks-operating-backup")
release = sys.argv[1]; cks = str(OP / "releases" / release / "bin" / "cks")
ds = OP / "dataset"; version = os.readlink(ds / "current")
def tree_sha(root):
    out = {}
    for p in sorted(root.rglob("*")):
        if p.is_file() and not p.is_symlink():
            out[str(p.relative_to(root))] = hashlib.sha256(p.read_bytes()).hexdigest()
    return out
log = {"release": release, "version": version}
stamp = time.strftime("%Y%m%dT%H%M%S")
t0 = time.monotonic()
backup = BK / stamp / version
backup.parent.mkdir(parents=True)
subprocess.run(["ditto", str(ds / version), str(backup)], check=True)
log["backup_s"] = round(time.monotonic() - t0, 1)
original = tree_sha(ds / version); copied = tree_sha(backup)
assert original == copied
log.update(backup_dir=str(backup), backup_files=len(copied), backup_bytes=sum(p.stat().st_size for p in backup.rglob("*") if p.is_file()),
           backup_sha_identical=True, backup_source_head=json.loads((backup / "dataset-identity.json").read_text())["source"]["source_commit"])
lost = OP / ("dataset.lost-" + stamp)
loss_at = time.monotonic()
ds.rename(lost)
probe = subprocess.run([sys.executable, str(P / "consumer-check.py"), release, "drill-during-loss"], capture_output=True, text=True)
log["query_during_loss_exit"] = probe.returncode
restore_start = time.monotonic()
ds.mkdir()
subprocess.run(["ditto", str(backup), str(ds / version)], check=True)
rb = subprocess.run([cks, "rollback", version, "--out", str(ds), "--project-id", "go-stablenet"], capture_output=True, text=True)
log["rollback"] = {"exit_code": rb.returncode, "stderr_tail": rb.stderr.strip()[-300:]}
assert rb.returncode == 0, rb.stderr
assert tree_sha(ds / version) == original
log["restore_copy_and_rollback_s"] = round(time.monotonic() - restore_start, 1)
q = subprocess.run([sys.executable, str(P / "consumer-check.py"), release, "drill-after-restore"], capture_output=True, text=True)
log["query_after_restore_exit"] = q.returncode
after = json.loads((P / "consumer-drill-after-restore.json").read_text())
ok = q.returncode == 0 and all(not r["is_error"] and r["citations"] > 0 for r in after["queries"])
first_query_ms = after["queries"][0]["elapsed_ms"] if after["queries"] else None
log["rto_measured_s"] = round(time.monotonic() - loss_at, 1)
log["rto_to_first_successful_query_s"] = round((restore_start - loss_at) + log["restore_copy_and_rollback_s"] + after["server_initialize_s"] + (first_query_ms or 0) / 1000, 1)
log.update(restored_queries_ok=ok, restored_version_sha_identical=True, current_after=os.readlink(ds / "current"),
           index_data_loss="none: backup taken from the same version that was lost (RPO measured 0 for this drill)",
           lost_copy_kept_at=str(lost), rto_target_s=3600, rpo_target_h=24,
           rto_met=log["rto_measured_s"] <= 3600, limits=["single host; no hardware failure or device loss tested", "backup on the same disk as the dataset (user decision)",
           "drill run by the agent; the user's acceptance is recorded separately"])
(P / "recovery-drill.json").write_text(json.dumps(log, indent=2) + "\n"); print(json.dumps({k: log[k] for k in ("backup_s", "restore_copy_and_rollback_s", "rto_measured_s", "rto_to_first_successful_query_s", "restored_queries_ok", "query_during_loss_exit")}))
