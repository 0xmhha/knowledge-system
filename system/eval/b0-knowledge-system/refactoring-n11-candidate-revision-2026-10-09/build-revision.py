"""Apply the user-adopted N11 pre-review recommendations as a PROPOSED
question book and protocol draft. Originals are read, never written.
Run from the repository root."""
import copy, hashlib, json, subprocess
from pathlib import Path

D = Path("system/eval/b0-knowledge-system/refactoring-n11-candidate-revision-2026-10-09")
SRC = Path("system/eval/b0-knowledge-system/refactoring-scope-adoption-2026-10-08")
REV = "479491baa50f818a5f7bb7651f35e9c7192ad906"
sha = lambda b: hashlib.sha256(b).hexdigest()

def span(path, first, last):
    body = subprocess.check_output(["git", "show", f"{REV}:{path}"])
    lines = body.decode().splitlines(keepends=True)
    assert 1 <= first <= last <= len(lines), (path, first, last)
    return {"path": path, "first": first, "last": last, "file_sha256": sha(body),
            "content_sha256": sha("".join(lines[first - 1:last]).encode())}

book = json.loads((SRC / "candidate-question-book.json").read_text())
# The recomputation method must reproduce every recorded evidence digest.
for q in book["questions"]:
    for e in q["evidence"]:
        assert span(e["path"], e["first"], e["last"]) == {k: e[k] for k in ("path", "first", "last", "file_sha256", "content_sha256")}, (q["id"], e["path"])

new = copy.deepcopy(book)
Q = {q["id"]: q for q in new["questions"]}
def ev(qid): return Q[qid]["evidence"]
changes = []
def note(qid, what): changes.append({"id": qid, "change": what})

# Evidence range extensions / additions.
e = ev("N11-DEV-01")[0]; ev("N11-DEV-01").insert(0, span(e["path"], 12, 12)); note("N11-DEV-01", "add line 12 (versioned_setup_required)")
e = ev("N11-DEV-02")[0]; ev("N11-DEV-02")[0] = span(e["path"], 97, 148); note("N11-DEV-02", "extend span to 97-148")
ev("N11-DEV-03").extend([span("internal/setup/durability.go", 21, 23), span("internal/setup/reindex.go", 441, 454)])
Q["N11-DEV-03"]["candidate_answer"] += " 호출자는 current 포인터가 바뀌지 않았다고 가정하거나 rollback하지 말고 상태를 검사하고 재시도해야 한다."
note("N11-DEV-03", "add durability.go 21-23, reindex.go 441-454; add caller guidance from durability.go:22")
Q["N11-DEV-04"]["candidate_answer"] = "reader protocol이 없으면 unknown_legacy_readers, 신원 검사에 실패하면 unverified_or_legacy, reader lock이 점유되어 있으면 pinned_reader 사유를 붙인다. 사유가 하나라도 있으면 Delete=false로 회수하지 않는다."
note("N11-DEV-04", "separate the three protection reasons")
e = ev("N11-DEV-05")[0]; ev("N11-DEV-05")[0] = span(e["path"], 41, 155); note("N11-DEV-05", "extend span to 41-155")
e = ev("N11-FINAL-CAND-07")[0]; ev("N11-FINAL-CAND-07")[0] = span(e["path"], 54, 78); note("N11-FINAL-CAND-07", "extend span to 54-78")
Q["N11-FINAL-CAND-08"]["candidate_answer"] = "ADR이 없으면 상태와 무관하게 거부한다. link가 verified이면 ADR도 verified여야 하고, RequirementIDs를 포함하며, EvidenceRefs에 ADR SourceRef가 있어야 한다."
note("N11-FINAL-CAND-08", "clarify condition scope")
e = ev("N11-FINAL-CAND-14")[0]; ev("N11-FINAL-CAND-14")[0] = span(e["path"], 24, 74); note("N11-FINAL-CAND-14", "extend span to 24-74 (safeRelativePath)")
Q["N11-FINAL-CAND-09"]["prompt_scope_note"] = "'this project' means knowledge-system"
Q["N11-FINAL-CAND-10"]["expected_behavior"] = "abstain"
for qid in ("N11-FINAL-CAND-09", "N11-FINAL-CAND-10"):
    Q[qid]["strict_zero_citations"] = False
    Q[qid]["abstention_citation_rule"] = "citing the recorded unassigned/null operating fact while abstaining is correct; citing any source as the answer is not"
    note(qid, "ABS may cite the unassigned record (user corpus rule)")

# Withdrawals.
for qid, why in [("N11-FINAL-CAND-04", "WHY without rationale source; duplicate cluster of CAND-03"),
                 ("N11-FINAL-CAND-13", "WHY without rationale source; relies on SQLite semantics outside span"),
                 ("N11-FINAL-CAND-15", "behavior question (CODE) in the CAND-14 cluster"),
                 ("N11-DEV-06", "shares the KnownFields + trailing-EOF fact with FINAL-CAND-03; DEV replaced per user decision"),
                 ("N11-DEV-07", "shares the span-SHA construction fact with FINAL-CAND-14; DEV replaced per user decision")]:
    Q[qid]["review_state"] = "withdrawn_proposed"; Q[qid]["withdrawal_reason"] = why; note(qid, "withdraw: " + why)

for q in new["questions"]:
    q["reviewer"] = None
    q.setdefault("review_state", "draft")
new["status"] = "proposed_revision_not_runnable"
new["revision_of_sha256"] = sha((SRC / "candidate-question-book.json").read_bytes())
new["replacement_dev_needed"] = ["N11-DEV-06 (POLICY code_contract)", "N11-DEV-07 (WHY)"]
(D / "candidate-question-book.json").write_text(json.dumps(new, ensure_ascii=False, indent=1) + "\n")

proto = json.loads((SRC / "candidate-protocol.json").read_text())
proto["status"] = "draft_not_runnable_revision"
proto["revision_of_sha256"] = sha((SRC / "candidate-protocol.json").read_bytes())
proto["corpus_exclusions"] = {"rule": "Evaluation-preparation material is excluded from the frozen corpus because it contains candidate answers.",
    "paths": ["docs/spec-driven/N11-*", "docs/spec-driven/N12-*", "system/eval/**"], "decided_by": "user (reviewer name pending)"}
proto["abstention_scoring"] = {"rule": "An ABS item is correct when the answer abstains; citing the recorded unassigned/null fact is allowed. Answering with an asserted fact is wrong.",
    "replaces": "strict_zero_citations=true for ABS items", "requires_new_pre_result_review": True}
proto["small_group_policy_confirmed"] = "docs/spec-driven/N11-SHORTFALL-RULE.md"
(D / "candidate-protocol.json").write_text(json.dumps(proto, ensure_ascii=False, indent=1) + "\n")

active = [q for q in new["questions"] if q["review_state"] != "withdrawn_proposed"]
clusters = {}
for q in active:
    clusters.setdefault((q["split"], q["group"]), set()).add(q["proposed_cluster_id"])
summary = {"changes": changes, "active_questions": {s: sum(q["split"] == s for q in active) for s in ("DEV", "FINAL")},
           "clusters": {f"{s}/{g}": len(c) for (s, g), c in sorted(clusters.items())},
           "all_groups_below_10": all(len(c) < 10 for (s, g), c in clusters.items() if s == "FINAL"),
           "recomputation_method_reproduced_all_recorded_digests": True}
(D / "revision-summary.json").write_text(json.dumps(summary, ensure_ascii=False, indent=1) + "\n")
print(json.dumps(summary["active_questions"]), json.dumps(summary["clusters"]))
