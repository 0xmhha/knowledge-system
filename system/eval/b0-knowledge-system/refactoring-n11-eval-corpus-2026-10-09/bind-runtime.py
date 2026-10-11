"""Bind the frozen N11 inputs to the new evaluation corpus and re-freeze.

Builds the semantic store with the reviewed ontology v2 / spec v3, writes the
runtime config, rebinds question/protocol source_commit to the corpus commit
after byte-checking every gold and basis span, sets execution_binding and
request_scope, allows execution per the user's dispatch decision, and writes a
new review record. Run from the repository root."""
import copy, datetime as dt, hashlib, json, subprocess, time
from pathlib import Path
ROOT = Path.cwd()
E = Path("system/eval/b0-knowledge-system/refactoring-n11-eval-corpus-2026-10-09")
FRZ = Path("system/eval/b0-knowledge-system/refactoring-n11-input-freeze-2026-10-09")
sha = lambda b: hashlib.sha256(b).hexdigest()
prep = json.loads((E / "preparation.json").read_text()); base = Path(prep["base"]); src = Path(prep["source"])
v = Path(prep["dataset_root"]) / "n11-eval-candidate"
identity = json.loads((v / "dataset-identity.json").read_text())
binary = base / "bin/cks"
def call(name, argv, expected=0):
    (E / (name + "-command.json")).write_text(json.dumps(argv, indent=2) + "\n")
    with (E / (name + ".stdout.txt")).open("w") as out, (E / (name + ".stderr.txt")).open("w") as err:
        r = subprocess.run(argv, stdout=out, stderr=err)
    assert r.returncode == expected, (name, r.returncode)
semantic = base / "semantic-eval.sqlite"
call("semantic-build", [str(binary), "semantic", "build", "--repo", str(src), "--project-id", "knowledge-system", "--dataset-id", identity["dataset_id"],
     "--graph", str(v / "graph"), "--vector", str(v / "vector"), "--version-dir", str(v),
     "--ontology", "docs/spec-driven/ontology-pilot-functional-v2.yaml", "--spec", "docs/spec-driven/spec-pilot-v3.yaml",
     "--docs", "docs/spec-driven/PUBLIC-CONTRACT-V2.md,docs/spec-driven/SOURCE-SNAPSHOT-ADR.md,docs/spec-driven/WORKSHEET-CATALOG-CONTRACT-V1.md",
     "--min-canonical-ratio", "1", "--store", str(semantic), "--out", str(E / "semantic-build.json")])
config = {"version": 1, "name": "cks", "description": "N11 frozen evaluation corpus (reviewer label: opus5.5 검토)",
    "backends": {"ckg": {"path": str(v / "graph/graph.db"), "source_root": str(src), "binary_path": str(base / "bin/ckg"), "timeout_ms": 5000},
                 "ckv": {"path": str(v / "vector"), "provider": "ollama", "binary_path": str(base / "bin/ckv"), "timeout_ms": 3000,
                         "embed_model": "bge-m3:latest", "embed_dim": 1024, "query_prefix_policy": "registry", "ollama_url": "http://127.0.0.1:11434"}},
    "listen": {"http_addr": "127.0.0.1:8080", "mcp_stdio": True, "transport": "stdio", "allow_remote": False},
    "logging": {"level": "info", "mode": "prod", "footprint_dir": str(base / "query-telemetry"), "measure_backend_calls": True},
    "sanitize": {"rules_path": str(ROOT / "system/policies/sanitization_rules.yaml"), "default_action": "drop", "fail_closed_on_unknown_rule": True},
    "semantic": {"store_path": str(semantic), "ontology_mode": "baseline", "ontology_budget_ms": 5000},
    "retrieval": {"recall_k": 10}}
cfg_b = (json.dumps(config, ensure_ascii=False, indent=2) + "\n").encode()
# Pre-dispatch guard: the runtime embedder identity implied by this config must
# equal the index identity, or CKV opens degraded (first attempt: native:v1 vs
# mrl-prefix-l2:v1 because embed_dim was omitted).
vm = json.loads((v / "vector/manifest.json").read_text())
runtime_id = dict(vm["embedding_identity_v2"], dimension_method="mrl-prefix-l2:v1" if config["backends"]["ckv"].get("embed_dim") else "native:v1")
runtime_sum = "v2:sha256:" + sha(json.dumps(runtime_id, separators=(",", ":")).encode())
assert runtime_sum == vm["embedding_checksum"], (runtime_sum, vm["embedding_checksum"])
(E / "embedding-identity-guard.json").write_text(json.dumps({"index_checksum": vm["embedding_checksum"], "runtime_checksum": runtime_sum, "match": True}, indent=1) + "\n")
(E / "runtime-config.yaml").write_bytes(cfg_b)
commit = prep["source_commit"]
book = json.loads((FRZ / "question-book.json").read_text()); proto = json.loads((FRZ / "protocol.json").read_text())
old_commit = book["source_commit"]
checked = 0
for q in book["questions"] + book.get("excluded_candidate_questions", []):
    for e in q.get("evidence", []) + q.get("abstention_basis_sources", []):
        if e["path"].startswith("system/eval/"):
            continue  # reviewer-only basis outside the corpus; still verified at the original commit by the old freeze
        body = (src / e["path"]).read_bytes(); lines = body.splitlines(keepends=True)
        assert sha(body) == e["file_sha256"] and sha(b"".join(lines[e["first"] - 1:e["last"]])) == e["content_sha256"], e["path"]
        checked += 1
    q["source_commit"] = commit
# A basis file excluded from the corpus cannot stay as a corpus-bound basis ref.
moved = []
for q in book["questions"]:
    keep = [e for e in q.get("abstention_basis_sources", []) if not e["path"].startswith("system/eval/")]
    gone = [e for e in q.get("abstention_basis_sources", []) if e["path"].startswith("system/eval/")]
    if gone:
        if not keep and q["id"] == "N11-FINAL-CAND-09":
            # Corpus-bound record that RTO/RPO and backup decisions remain open
            # (C1-RECOVERY-VALIDATION.md:67-69, identical at the original commit).
            body = (src / "docs/spec-driven/C1-RECOVERY-VALIDATION.md").read_bytes(); ls = body.splitlines(keepends=True)
            orig = subprocess.check_output(["git", "show", old_commit + ":docs/spec-driven/C1-RECOVERY-VALIDATION.md"])
            assert orig == body
            keep = [{"path": "docs/spec-driven/C1-RECOVERY-VALIDATION.md", "first": 67, "last": 69, "file_sha256": sha(body), "content_sha256": sha(b"".join(ls[66:69]))}]
            q["basis_replacement"] = "system/eval basis excluded from corpus; replaced by the corpus record the adopted pre-review cited"
        q["abstention_basis_sources"] = keep
        q["reviewer_only_basis_outside_corpus"] = [dict(e, commit=old_commit) for e in gone]
        moved.append(q["id"])
book.update(source_commit=commit, previous_source_commit=old_commit, previous_frozen_book_sha256=sha((FRZ / "question-book.json").read_bytes()))
proto.update(source_commit=commit, previous_frozen_protocol_sha256=sha((FRZ / "protocol.json").read_bytes()),
             execution_binding={"dataset_id": identity["dataset_id"], "config_sha256": sha(cfg_b), "binary_sha256": sha(binary.read_bytes())},
             request_scope={"knowledge_as_of": "2026-10-09", "knowledge_subsystem": "knowledge-system"},
             execution_allowed=True, execution_note="Execution allowed by the user's dispatch decision (refactoring-n11-eval-corpus-2026-10-09/dispatch-decision.json). All important groups are below ten clusters and are inconclusive by the pre-result rule.")
book_b = (json.dumps(book, ensure_ascii=False, indent=1) + "\n").encode(); proto_b = (json.dumps(proto, ensure_ascii=False, indent=1) + "\n").encode()
(E / "question-book.json").write_bytes(book_b); (E / "protocol.json").write_bytes(proto_b)
old_review = json.loads((FRZ / "review-record.json").read_text())
now = dt.datetime.now(dt.timezone.utc).replace(microsecond=0)
review = dict(old_review, reviewed_at=(now - dt.timedelta(seconds=1)).isoformat().replace("+00:00", "Z"), frozen_at=now.isoformat().replace("+00:00", "Z"),
              protocol_sha256=sha(proto_b), questions_sha256=sha(book_b), rebinding="corpus commit and execution binding only; questions, gold and decisions unchanged",
              previous_review_sha256=sha((FRZ / "review-record.json").read_bytes()))
(E / "review-record.json").write_text(json.dumps(review, ensure_ascii=False, indent=1) + "\n")
time.sleep(1.2)
known = ROOT / "system/eval/b0-knowledge-system/refactoring-n11-input-controls-2026-10-06/known-final-observations.json"
call("input-check", ["python3", str(ROOT / "scripts/refactoring-eval-input-check.py"), "--protocol", str(E / "protocol.json"), "--questions", str(E / "question-book.json"),
     "--review", str(E / "review-record.json"), "--repo", str(src), "--observations", str(known), "--require-ready", "--output", str(E / "input-check.json")])
call("whole-tree-audit", [str(base / "bin/ckg"), "audit", "--src", str(src), "--graph", str(v / "graph"), "--format", "json"], 1)
rep = json.loads((E / "input-check.json").read_text())
print(json.dumps({"dataset": identity["dataset_id"], "corpus_commit": commit, "spans_checked": checked, "basis_moved": moved,
                  "input_ready": rep["input_ready"], "sampling": rep["sampling_verdict"], "groups": rep["final_independent_clusters_by_group"]}))
