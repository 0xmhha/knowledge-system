"""Archive the accepted result (base and result versions, patch records, semantic store, source
bundle) and replay it with the original source, dataset and store hidden.
Original paths are restored at the end. Run from the repository root."""
import hashlib, json, shutil, subprocess, tarfile, tempfile
from pathlib import Path
E = Path("system/eval/b0-knowledge-system/refactoring-n14-change-acceptance-2026-10-10").resolve()
prep = json.loads((E / "preparation.json").read_text()); base = Path(prep["base"]); ds = Path(prep["dataset_root"]); src = Path(prep["source"])
assert str(base).startswith("/private/tmp/ks-n14-accept-")
sha = lambda p: hashlib.sha256(Path(p).read_bytes()).hexdigest()
stage = base / "archive-stage"; shutil.rmtree(stage, ignore_errors=True); stage.mkdir()
subprocess.run(["git", "bundle", "create", str(stage / "source.bundle"), "HEAD"], cwd=src, check=True, capture_output=True)
subprocess.run(["sqlite3", str(base / "semantic-result.sqlite"), "PRAGMA wal_checkpoint(TRUNCATE);"], check=True, capture_output=True)
members = [("dataset/base", ds / "base"), ("dataset/result", ds / "result"), ("dataset/.patches", ds / ".patches"), ("semantic-result.sqlite", base / "semantic-result.sqlite"),
           ("linked-runs", base / "linked-runs"), ("source.bundle", stage / "source.bundle")]
archive = E / "accepted-result.tar.gz"
with tarfile.open(archive, "w:gz", format=tarfile.PAX_FORMAT) as tar:
    for name, path in members:
        tar.add(path, arcname=name)
inventory = []
with tarfile.open(archive) as tar:
    for m in tar.getmembers():
        if m.isfile():
            inventory.append({"path": m.name, "sha256": hashlib.sha256(tar.extractfile(m).read()).hexdigest()})
hidden = [(p, p.with_name(p.name + ".hidden-for-replay")) for p in (src, ds, base / "semantic-result.sqlite")]
replay = {}
try:
    for p, h in hidden:
        assert p.exists() and not h.exists(); p.rename(h)
    for p in (base / "semantic-result.sqlite-wal", base / "semantic-result.sqlite-shm"):
        if p.exists(): p.rename(p.with_name(p.name + ".hidden-for-replay"))
    with tarfile.open(archive) as tar:
        tar.extractall(base, filter="data")
    subprocess.run(["git", "clone", "-q", str(base / "source.bundle"), str(src)], check=True)
    (ds / "current").symlink_to("result")
    assert all(sha(base / row["path"]) == row["sha256"] for row in inventory)
    driver = Path(tempfile.mkdtemp(prefix=".n14-replay-", dir=Path.cwd()))
    try:
        (driver / "main.go").write_text((E / "current-identity-driver.go.txt").read_text())
        r = subprocess.run(["go", "run", str(driver / "main.go"), str(ds), str(base / "semantic-result.sqlite"), str(src)], capture_output=True, text=True)
    finally:
        shutil.rmtree(driver)
    status = subprocess.run([str(base / "bin/cks"), "patch", "status", "--dataset", str(ds), "--patch-id", "retained-batch-7c57a388"], capture_output=True, text=True)
    replay = {"identity_driver_exit": r.returncode, "identity": json.loads(r.stdout.strip().splitlines()[-1]) if r.returncode == 0 else r.stderr[-400:],
              "patch_status_exit": status.returncode, "patch_status": json.loads(status.stdout) if status.returncode == 0 else status.stderr[-300:],
              "archived_files": len(inventory), "every_archived_file_sha_unchanged": True,
              "original_source_dataset_store_hidden": True}
finally:
    for path in (src, ds, base / "semantic-result.sqlite", base / "source.bundle", base / "linked-runs-restored"):
        if path.is_dir() and not path.is_symlink(): shutil.rmtree(path)
        elif path.exists(): path.unlink()
    for p, h in hidden:
        h.rename(p)
    for p in (base / "semantic-result.sqlite-wal", base / "semantic-result.sqlite-shm"):
        q = p.with_name(p.name + ".hidden-for-replay")
        if q.exists(): q.rename(p)
    shutil.rmtree(stage, ignore_errors=True)
(E / "archive-inventory.json").write_text(json.dumps(inventory, indent=1) + "\n")
replay.update(archive_sha256=sha(archive), archive_bytes=archive.stat().st_size)
(E / "archive-replay.json").write_text(json.dumps(replay, indent=2, default=str) + "\n")
print(json.dumps({k: replay[k] for k in ("identity_driver_exit", "patch_status_exit", "archived_files", "archive_bytes")}))
