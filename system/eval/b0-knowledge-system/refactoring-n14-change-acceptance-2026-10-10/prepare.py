"""Build an owned base/result pair for one real change acceptance.

base   = copy of the N12 reviewed held candidate (source 0b9085e7, dataset daf9608b), made current
result = the same captured source plus the four files changed by 7c57a388, committed on top
N12 originals are copied, never modified. Run from the repository root."""
import hashlib, json, shutil, subprocess, tempfile
from pathlib import Path
ROOT = Path.cwd()
E = Path("system/eval/b0-knowledge-system/refactoring-n14-change-acceptance-2026-10-10")
N12 = json.loads(Path("system/eval/b0-knowledge-system/refactoring-n12-reviewed-activation-2026-10-09/preparation.json").read_text())
sha = lambda b: hashlib.sha256(b).hexdigest()
CHANGE = "7c57a388"
changed = subprocess.check_output(["git", "diff", "--name-only", "985e22d89d9a6df39d1bf4f8edf07289fc1decf6", "HEAD", "--", "cmd", "internal", "pkg", "go.mod", "go.sum"], text=True).split()
assert sorted(changed) == sorted(["internal/setup/source_read.go", "internal/setup/source_read_test.go",
    "internal/system/evidencev2/knowledge_trace_test.go", "internal/system/semantic/validate.go"]), changed
base = Path(tempfile.mkdtemp(prefix="ks-n14-accept-", dir="/private/tmp"))
src, dataset, bins = base / "source", base / "dataset", base / "bin"
subprocess.run(["git", "clone", "-q", "--no-hardlinks", N12["source"], str(src)], check=True)
assert subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=src, text=True).strip() == N12["source_commit"]
dataset.mkdir()
shutil.copytree(Path(N12["dataset_root"]) / "reviewed-v3-candidate", dataset / "base", symlinks=True)
(dataset / "current").symlink_to("base")
bins.mkdir()
for name, origin in [("ckg", Path(N12["base"]) / "bin/ckg"), ("ckv", Path(N12["base"]) / "bin/ckv"),
                     ("cks", Path("/private/tmp/ks-n11-eval-m13oocru/bin/cks-7c57a388"))]:
    shutil.copy2(origin, bins / name)
before = {p: sha((src / p).read_bytes()) for p in changed}
for p in changed:
    (src / p).write_bytes(subprocess.check_output(["git", "show", "HEAD:" + p]))
after = {p: sha((src / p).read_bytes()) for p in changed}
assert all(before[p] != after[p] for p in changed)
subprocess.run(["git", "add", "--"] + changed, cwd=src, check=True)
subprocess.run(["git", "-c", "user.name=knowledge-system change acceptance", "-c", "user.email=change-acceptance@local.invalid",
                "-c", "commit.gpgsign=false", "commit", "-qm", "Apply retained-archive batching change (" + CHANGE + ")"], cwd=src, check=True)
result_commit = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=src, text=True).strip()
assert not subprocess.check_output(["git", "status", "--porcelain"], cwd=src)
diff = subprocess.check_output(["git", "diff", "--name-only", N12["source_commit"], result_commit], cwd=src, text=True).split()
assert sorted(diff) == sorted(changed)
prep = {"base": str(base), "source": str(src), "dataset_root": str(dataset), "project_id": "knowledge-system",
        "base_version": "base", "base_source_commit": N12["source_commit"], "result_version": "result", "result_source_commit": result_commit,
        "change_revision": subprocess.check_output(["git", "rev-parse", CHANGE], text=True).strip(),
        "changed_files": {p: {"base_sha256": before[p], "result_sha256": after[p]} for p in changed},
        "binaries_sha256": {n: sha((bins / n).read_bytes()) for n in ("cks", "ckg", "ckv")},
        "n12_originals_modified": False, "reviewer_label": "opus5.5 검토", "reviewer_kind": "ai_model_label_designated_by_user"}
(E / "preparation.json").write_text(json.dumps(prep, indent=2) + "\n")
print(json.dumps({"base": str(base), "result_commit": result_commit, "changed": len(changed)}))
