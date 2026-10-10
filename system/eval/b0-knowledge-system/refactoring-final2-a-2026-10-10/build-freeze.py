"""Freeze FINAL2 part A v2 under the user's approvals ("모두 승인, 표기 opus5.5 검토" for the draft,
"승인, 표기 opus5.5 검토" for the v2 replacements).

Approves the drafted question book and protocol unchanged except for
review fields, pins the candidate and reference binaries, declares ABS out
of scope (part B judges it), and writes a review record bound to the exact
bytes. Run from the repository root."""
import datetime as dt, hashlib, json
from pathlib import Path

D = Path("system/eval/b0-knowledge-system/refactoring-final2-a-2026-10-10")
DRAFT = Path("system/eval/b0-knowledge-system/refactoring-final2-draft-2026-10-10")
REVIEWER, KIND = "opus5.5 검토", "ai_model_label_designated_by_user"
CANDIDATE = "dc6cf0a9e794f5180ca87cd414fbab780c73d344711c22fa84e3c36d0e5ca5ab"  # cks built from 7077741f
sha = lambda b: hashlib.sha256(b).hexdigest()

decision_v1 = {"schema_version": 1, "asked": "FINAL2 초안(문서 FINAL2-DRAFT-REVIEW.md)을 어떻게 처리할까요?",
            "answer": "모두 승인, 표기 opus5.5 검토", "answered_via": "chat option selection", "date": "2026-10-10",
            "scope": "part A questions/gold and protocol, part B unknown records and questions, reviewer label",
            "draft_document": "docs/spec-driven/FINAL2-DRAFT-REVIEW.md", "user_active_time_measured": None,
            "independent_human_content_review": False}
decision = {"schema_version": 1, "decisions": [decision_v1, {"asked": "A v2의 교체 문항 11개를 승인할까요?",
            "answer": "승인, 표기 opus5.5 검토 (권장)", "answered_via": "chat option selection", "date": "2026-10-10",
            "scope": "eleven part A replacement questions (index scope defect); protocol and part B unchanged",
            "draft_document": "docs/spec-driven/FINAL2-DRAFT-REVIEW.md section 6", "user_active_time_measured": None,
            "independent_human_content_review": False}]}
(D / "user-decision.json").write_text(json.dumps(decision, ensure_ascii=False, indent=1) + "\n")

book = json.loads((DRAFT / "part-a-question-book-v2.json").read_text())
for q in book["questions"]:
    q.update(review_state="approved", reviewer=REVIEWER, reviewer_kind=KIND,
             independence_review="approved: new fact clusters disjoint from observed N11 DEV/FINAL and B0 questions; DEV/FINAL disjoint",
             contamination_review="approved: frozen N11 corpus excludes docs/spec-driven/N11-*, N12-* and system/eval/**; observation catalog checked by input gate")
book.update(status="approved", reviewer=REVIEWER, reviewer_kind=KIND, draft_sha256=sha((DRAFT / "part-a-question-book-v2.json").read_bytes()),
            index_scope_check="index-scope-check.json (required pass before freeze)")
proto = json.loads((DRAFT / "part-a-protocol-draft.json").read_text())
proto["execution_binding"] = dict(proto["execution_binding"], binary_sha256=CANDIDATE)
proto.update(status="approved", reviewer=REVIEWER, reviewer_kind=KIND, execution_allowed=True,
             binding_review_state="bound_by_user_approval_of_draft",
             final_groups_out_of_scope={"ABS": "judged by FINAL2 part B on a corpus that carries the declared-unknown records"},
             draft_sha256=sha((DRAFT / "part-a-protocol-draft.json").read_bytes()),
             execution_note="User approved the FINAL2 draft and its execution (user-decision.json). Candidate and reference binaries run under one reservation. Every important group is below ten units: N08 is descriptive/inconclusive; N10 is judged by the frozen warm p95 ratio on the candidate run.")
book_b = (json.dumps(book, ensure_ascii=False, indent=1) + "\n").encode()
proto_b = (json.dumps(proto, ensure_ascii=False, indent=1) + "\n").encode()
(D / "question-book.json").write_bytes(book_b)
(D / "protocol.json").write_bytes(proto_b)
now = dt.datetime.now(dt.timezone.utc).replace(microsecond=0)
review = {"status": "approved", "scope": "fresh-evaluation-inputs", "reviewer": REVIEWER, "reviewer_kind": KIND,
          "independent_human_content_review": False,
          "reviewed_at": (now - dt.timedelta(seconds=1)).isoformat().replace("+00:00", "Z"),
          "frozen_at": now.isoformat().replace("+00:00", "Z"),
          "protocol_sha256": sha(proto_b), "questions_sha256": sha(book_b),
          "decision_sources": [str(D / "user-decision.json"), "docs/spec-driven/FINAL2-DRAFT-REVIEW.md"],
          "cases": {q["id"]: {"gold_approved": True, "independence_approved": True, "contamination_checked": True,
                              "unit_id": q["unit_id"], "cluster_id": q["cluster_id"]} for q in book["questions"]}}
(D / "review-record.json").write_text(json.dumps(review, ensure_ascii=False, indent=1) + "\n")
known = json.loads(Path("system/eval/b0-knowledge-system/refactoring-n11-input-controls-2026-10-06/known-final-observations.json").read_text())
n11 = json.loads(Path("system/eval/b0-knowledge-system/refactoring-n11-evaluation-run-2026-10-09/final-registry.json").read_text())["records"]
(D / "known-observations.json").write_text(json.dumps(known + n11, ensure_ascii=False, indent=1) + "\n")
print(json.dumps({"questions": len(book["questions"]), "protocol_sha256": sha(proto_b), "questions_sha256": sha(book_b)}))
