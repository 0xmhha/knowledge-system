#!/usr/bin/env python3
"""Prepare the FINAL2 part B corpus: repository HEAD sources plus the approved
declared-unknown overlay, evaluation material excluded.

Selection mirrors the N11 corpus (cmd/internal/pkg and the active non-
evaluation source-map entries plus selected documents) and adds the part B
basis and contrast documents. The overlay records are written as verified
under the user's approval (reviewer label "opus5.5 검토"); the release
verdict basis line is bound to the N-18 row of the snapshotted worklist.
Run from the repository root."""
import fnmatch, hashlib, json, subprocess, tempfile
from pathlib import Path

ROOT = Path.cwd()
OUT = ROOT / "system/eval/b0-knowledge-system/refactoring-final2-b-2026-10-10"
DRAFT = ROOT / "system/eval/b0-knowledge-system/refactoring-final2-draft-2026-10-10/knowledge-overlay"
N11 = ROOT / "system/eval/b0-knowledge-system/refactoring-n11-eval-corpus-2026-10-09"
REVIEWER = "opus5.5 검토"
EXCLUDED = ["system/eval/*", "docs/spec-driven/N11-*", "docs/spec-driven/N12-*", "docs/spec-driven/FINAL2-*"]
sha = lambda b: hashlib.sha256(b).hexdigest()

revision = subprocess.check_output(["git", "rev-parse", "HEAD"], text=True).strip()
assert subprocess.run(["git", "diff", "--quiet", "HEAD"]).returncode == 0, "tracked tree differs from HEAD"
L = json.loads((ROOT / "docs/spec-driven/REFACTORING-PROGRESS.json").read_text())["active_source_binding"]
previous = json.loads(subprocess.check_output(["git", "show", L["revision"] + ":" + L["manifest"]]))
requested = {p for p in previous["source_sha256"] if not any(fnmatch.fnmatch(p, g) for g in EXCLUDED)}
requested |= {"docs/spec-driven/PUBLIC-CONTRACT-V2.md", "docs/spec-driven/SOURCE-SNAPSHOT-ADR.md",
              "docs/spec-driven/WORKSHEET-CATALOG-CONTRACT-V1.md", "docs/spec-driven/ontology-pilot-v2.yaml",
              "docs/spec-driven/ontology-pilot-functional-v2.yaml", "docs/spec-driven/spec-pilot-v3.yaml",
              "docs/spec-driven/C1-OPERATIONS-REVIEW.md", "docs/spec-driven/C1-RECOVERY-VALIDATION.md",
              # part B basis and contrast documents
              "docs/spec-driven/N14-CHANGE-ACCEPTANCE.md", "docs/spec-driven/REFACTORING-REMAINING-WORKLIST.md",
              "docs/spec-driven/N15-N17-LOCAL-OPERATION-ACCEPTANCE.md"}
base = Path(tempfile.mkdtemp(prefix="ks-final2b-", dir="/private/tmp"))
source = base / "source"; source.mkdir(); (base / "bin").mkdir()
files, selected = {}, []
for line in subprocess.check_output(["git", "ls-tree", "-r", revision], text=True).splitlines():
    meta, path = line.split("\t", 1); mode, kind, oid = meta.split()
    if (path.startswith(("cmd/", "internal/", "pkg/")) or path in requested) and not any(fnmatch.fnmatch(path, g) for g in EXCLUDED):
        assert kind == "blob" and mode in ("100644", "100755"), (path, meta)
        selected.append((path, mode, oid))
proc = subprocess.Popen(["git", "cat-file", "--batch"], stdin=subprocess.PIPE, stdout=subprocess.PIPE)
for path, mode, oid in selected:
    proc.stdin.write((oid + "\n").encode()); proc.stdin.flush()
    header = proc.stdout.readline().decode().split(); data = proc.stdout.read(int(header[2])); assert proc.stdout.read(1) == b"\n"
    target = source / path; target.parent.mkdir(parents=True, exist_ok=True); target.write_bytes(data)
    target.chmod(0o755 if mode == "100755" else 0o644); files[path] = sha(data)
proc.stdin.close(); assert proc.wait() == 0
assert requested <= set(files), sorted(requested - set(files))

# Approved overlay: verified records bound to the snapshotted basis lines.
worklist = (source / "docs/spec-driven/REFACTORING-REMAINING-WORKLIST.md").read_text().splitlines()
n18 = [i + 1 for i, l in enumerate(worklist) if l.startswith("| N-18 |")]
assert len(n18) == 1, n18
n14 = (source / "docs/spec-driven/N14-CHANGE-ACCEPTANCE.md").read_text().splitlines()
assert "정책 담당자가 없어 정책 instance가 없기 때문이며" in n14[33], "N14 basis line moved"
overlay = source / ".cks/knowledge"; (overlay / "unknowns").mkdir(parents=True)
(overlay / "manifest.yaml").write_bytes((DRAFT / "manifest.yaml").read_bytes())
for name in ("UNK-POLICY-OWNER.yaml", "UNK-RELEASE-VERDICT.yaml"):
    text = (DRAFT / "unknowns" / name).read_text()
    text = text.replace("start_line: 0, end_line: 0", f"start_line: {n18[0]}, end_line: {n18[0]}")
    text = text.replace("status: proposed\n", f"status: verified\nreviewed_by: {REVIEWER}\nreview_reason: Approved by the user with the FINAL2 draft; the basis line states the fact is not decided.\n")
    (overlay / "unknowns" / name).write_text(text)
lock = subprocess.run([str(base / "bin/cks") if (base / "bin/cks").exists() else "/private/tmp/ks-n11-eval-m13oocru/bin/cks-7077741f",
                       "knowledge", "lock", "--project-root", str(source)], capture_output=True, text=True)
assert lock.returncode == 0, lock.stderr
validate = subprocess.run(["/private/tmp/ks-n11-eval-m13oocru/bin/cks-7077741f", "knowledge", "validate", "--project-root", str(source)], capture_output=True, text=True)
assert validate.returncode == 0, validate.stdout + validate.stderr
for p in sorted(overlay.rglob("*")):
    if p.is_file():
        files[str(p.relative_to(source))] = sha(p.read_bytes())

subprocess.run(["git", "init", "-q", "-b", "preparation"], cwd=source, check=True)
subprocess.run(["git", "add", "--force", "--all"], cwd=source, check=True)
subprocess.run(["git", "-c", "user.name=knowledge-system source preparation", "-c", "user.email=source-preparation@local.invalid",
                "-c", "commit.gpgsign=false", "commit", "-qm",
                "Capture FINAL2 part B corpus: current sources, selected documents and the approved declared-unknown overlay; evaluation material excluded"],
               cwd=source, check=True)
commit = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=source, text=True).strip()
assert not subprocess.check_output(["git", "status", "--porcelain"], cwd=source)
for name in ("cks", "ckg", "ckv"):
    src = Path("/private/tmp/ks-n11-eval-m13oocru/bin") / ("cks-7077741f" if name == "cks" else name)
    (base / "bin" / name).write_bytes(src.read_bytes()); (base / "bin" / name).chmod(0o755)
save = lambda name, x: (OUT / name).write_text(json.dumps(x, ensure_ascii=False, indent=2) + "\n")
save("source-inventory.json", {"original_revision": revision, "derived_commit": commit, "files": files,
     "excluded_globs": EXCLUDED, "release_basis_line": n18[0], "lock_stdout": lock.stdout.strip(),
     "validate_stdout": validate.stdout.strip()})
save("preparation.json", {"base": str(base), "source": str(source), "dataset_root": str(base / "dataset"),
     "project_id": "knowledge-system", "source_commit": commit, "runtime_revision": revision, "source_file_count": len(files),
     "binaries_sha256": {n: sha((base / "bin" / n).read_bytes()) for n in ("cks", "ckg", "ckv")},
     "reviewer_label": REVIEWER, "reviewer_kind": "ai_model_label_designated_by_user"})
subprocess.run(["git", "bundle", "create", str(OUT / "source.bundle"), "HEAD"], cwd=source, check=True, capture_output=True)
print(json.dumps({"base": str(base), "files": len(files), "commit": commit, "release_basis_line": n18[0]}))
