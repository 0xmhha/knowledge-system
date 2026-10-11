"""Rebind the frozen N11 inputs to a rebuilt cks binary before FINAL.

Only protocol.execution_binding.binary_sha256 and the execution note change.
Questions, gold, clusters, thresholds, corpus, config and the semantic store
are unchanged. FINAL has not been observed or reserved. Run from the
repository root with a clean tracked product tree."""
import datetime as dt, hashlib, json, subprocess, time
from pathlib import Path
ROOT = Path.cwd()
E = Path("system/eval/b0-knowledge-system/refactoring-n11-rebind-2026-10-09")
PREV = Path("system/eval/b0-knowledge-system/refactoring-n11-eval-corpus-2026-10-09")
sha = lambda b: hashlib.sha256(b).hexdigest()
prep = json.loads((PREV / "preparation.json").read_text()); base = Path(prep["base"]); src = Path(prep["source"])
rev = subprocess.check_output(["git", "rev-parse", "HEAD"], text=True).strip()
dirty = subprocess.run(["git", "diff", "--quiet", "HEAD", "--", "cmd", "internal", "pkg", "go.mod", "go.sum"]).returncode
assert dirty == 0, "tracked product tree differs from HEAD"
binary = base / ("bin/cks-" + rev[:8])
assert not binary.exists(), "binary path already used"
build = subprocess.run(["go", "build", "-o", str(binary), "./cmd/cks"], capture_output=True, text=True)
assert build.returncode == 0, build.stderr
(E / "runtime-build.json").write_text(json.dumps({"source_revision": rev, "argv": ["go", "build", "-o", str(binary), "./cmd/cks"],
    "exit_code": 0, "binary_sha256": sha(binary.read_bytes()),
    "go_version": subprocess.check_output(["go", "version"], text=True).strip(),
    "unchanged_binaries": json.loads((PREV / "runtime-binaries.json").read_text())["binaries"] | {"cks": "replaced"},
    "previous_cks_sha256": json.loads((PREV / "protocol.json").read_text())["execution_binding"]["binary_sha256"]}, indent=2) + "\n")
proto = json.loads((PREV / "protocol.json").read_text())
binding = dict(proto["execution_binding"], binary_sha256=sha(binary.read_bytes()))
proto.update(execution_binding=binding, previous_frozen_protocol_sha256=sha((PREV / "protocol.json").read_bytes()),
             execution_note="Execution allowed by the user's dispatch decision. The user chose to fix the DEV-observed latency before FINAL "
                            "(refactoring-n11-evaluation-run-2026-10-09/final-timing-decision.json); only the cks binary changed. "
                            "All important groups are below ten clusters and are inconclusive by the pre-result rule.")
book_b = (PREV / "question-book.json").read_bytes()
proto_b = (json.dumps(proto, ensure_ascii=False, indent=1) + "\n").encode()
(E / "question-book.json").write_bytes(book_b); (E / "protocol.json").write_bytes(proto_b)
old_review = json.loads((PREV / "review-record.json").read_text())
now = dt.datetime.now(dt.timezone.utc).replace(microsecond=0)
review = dict(old_review, reviewed_at=(now - dt.timedelta(seconds=1)).isoformat().replace("+00:00", "Z"), frozen_at=now.isoformat().replace("+00:00", "Z"),
              protocol_sha256=sha(proto_b), questions_sha256=sha(book_b),
              rebinding="cks binary only (" + rev + "); questions, gold, clusters, thresholds, corpus and config unchanged; FINAL unobserved",
              previous_review_sha256=sha((PREV / "review-record.json").read_bytes()))
review["decision_sources"] = old_review["decision_sources"] + ["system/eval/b0-knowledge-system/refactoring-n11-evaluation-run-2026-10-09/final-timing-decision.json"]
(E / "review-record.json").write_text(json.dumps(review, ensure_ascii=False, indent=1) + "\n")
time.sleep(1.2)
known = ROOT / "system/eval/b0-knowledge-system/refactoring-n11-input-controls-2026-10-06/known-final-observations.json"
argv = ["python3", str(ROOT / "scripts/refactoring-eval-input-check.py"), "--protocol", str(E / "protocol.json"), "--questions", str(E / "question-book.json"),
        "--review", str(E / "review-record.json"), "--repo", str(src), "--observations", str(known), "--require-ready", "--output", str(E / "input-check.json")]
(E / "input-check-command.json").write_text(json.dumps(argv, indent=2) + "\n")
r = subprocess.run(argv, capture_output=True, text=True)
(E / "input-check.stdout.txt").write_text(r.stdout); (E / "input-check.stderr.txt").write_text(r.stderr)
assert r.returncode == 0, r.stderr
rep = json.loads((E / "input-check.json").read_text())
print(json.dumps({"binary": str(binary), "binary_sha256": binding["binary_sha256"], "input_ready": rep["input_ready"],
                  "sampling": rep["sampling_verdict"], "groups": rep["final_independent_clusters_by_group"]}))
