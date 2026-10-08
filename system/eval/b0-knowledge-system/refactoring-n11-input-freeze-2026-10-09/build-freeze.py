"""Freeze the adopted N11 inputs under the user-designated reviewer label.

Drops withdrawn rows, adds the two replacement DEV questions, approves the
remaining rows, and writes a review record bound to the exact protocol and
question-book bytes. FINAL execution stays disallowed here
(execution_allowed=false); dispatch needs a separate user decision.
Run from the repository root."""
import copy, datetime as dt, hashlib, json, subprocess
from pathlib import Path

D = Path("system/eval/b0-knowledge-system/refactoring-n11-input-freeze-2026-10-09")
SRC = Path("system/eval/b0-knowledge-system/refactoring-n11-candidate-revision-2026-10-09")
REV = "479491baa50f818a5f7bb7651f35e9c7192ad906"
REVIEWER = "opus5.5 검토"
sha = lambda b: hashlib.sha256(b).hexdigest()

def span(path, first, last):
    body = subprocess.check_output(["git", "show", f"{REV}:{path}"])
    lines = body.decode().splitlines(keepends=True)
    return {"path": path, "first": first, "last": last, "file_sha256": sha(body),
            "content_sha256": sha("".join(lines[first - 1:last]).encode())}

book = json.loads((SRC / "candidate-question-book.json").read_text())
proto = json.loads((SRC / "candidate-protocol.json").read_text())
withdrawn = [q["id"] for q in book["questions"] if q["review_state"] == "withdrawn_proposed"]
qs = [copy.deepcopy(q) for q in book["questions"] if q["review_state"] != "withdrawn_proposed"]
qs += [
    {"id": "N11-DEV-08", "split": "DEV", "group": "POLICY", "proposed_cluster_id": "capture-default-limits", "language": "ko",
     "prompt": "명시 설정이 없을 때 source 캡처의 기본 파일 수, 단일 파일 크기, 총 크기 상한은 얼마인가요?",
     "candidate_answer": "기본 상한은 파일 100,000개, 단일 파일 32 MiB(32<<20), 총 4 GiB(4<<30)이다.",
     "expected_behavior": "cite", "policy_scope": "code_contract", "source_commit": REV,
     "evidence": [span("internal/setup/capture.go", 50, 54)], "replaces": "N11-DEV-06"},
    {"id": "N11-DEV-09", "split": "DEV", "group": "WHY", "proposed_cluster_id": "mutation-lock-ownership", "language": "ko",
     "prompt": "데이터셋 변경 잠금은 왜 PID나 기록 시각으로 소유권을 판단하지 않나요?",
     "candidate_answer": "소유권은 영구 inode에 대한 OS flock이 정하고, 프로세스가 끝나면 OS가 해제하므로 PID와 시각은 진단용일 뿐이다. 업그레이드 이전의 살아 있는 PID는 시각과 무관하게 보호하며 그 inode를 지우지 않는다.",
     "expected_behavior": "cite", "source_commit": REV,
     "evidence": [span("internal/setup/reindex.go", 60, 63), span("internal/setup/reindex.go", 87, 91)], "replaces": "N11-DEV-07"},
]
for q in qs:
    q["cluster_id"] = q["proposed_cluster_id"]
    q["unit_id"] = q.get("proposed_unit_id") or q["proposed_cluster_id"]
    q["review_state"] = "approved"
    q["reviewer"] = REVIEWER
    q["reviewer_kind"] = "ai_model_label_designated_by_user"
    q.setdefault("independence_review", None)
    q["independence_review"] = "approved: DEV/FINAL disjoint after DEV-06/07 replacement; CODE/WHY pairs share their cluster by design"
    q["contamination_review"] = "approved with corpus exclusion rule (docs/spec-driven/N11-*, N12-*, system/eval/** excluded); prior FINAL observation catalog checked by input gate"
book.update(status="approved", questions=qs, reviewer=REVIEWER, reviewer_kind="ai_model_label_designated_by_user",
            withdrawn_before_freeze=withdrawn, revision_of_sha256=sha((SRC / "candidate-question-book.json").read_bytes()))
book.pop("replacement_dev_needed", None)
proto.update(status="approved", reviewer=REVIEWER, reviewer_kind="ai_model_label_designated_by_user",
             execution_allowed=False, execution_binding=None,
             execution_note="FINAL dispatch is irreversible (durable reservation) and needs a separate user decision and a coordinated corpus binding; this freeze does not authorize execution.")
book_b = (json.dumps(book, ensure_ascii=False, indent=1) + "\n").encode()
proto_b = (json.dumps(proto, ensure_ascii=False, indent=1) + "\n").encode()
(D / "question-book.json").write_bytes(book_b)
(D / "protocol.json").write_bytes(proto_b)
now = dt.datetime.now(dt.timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")
review = {"status": "approved", "scope": "fresh-evaluation-inputs", "reviewer": REVIEWER,
          "reviewer_kind": "ai_model_label_designated_by_user", "independent_human_content_review": False,
          "reviewed_at": now, "frozen_at": now, "protocol_sha256": sha(proto_b), "questions_sha256": sha(book_b),
          "decision_sources": ["system/eval/b0-knowledge-system/refactoring-n11-candidate-revision-2026-10-09/decision.json",
                               "system/eval/b0-knowledge-system/refactoring-n12-reviewed-activation-2026-10-09/review-designation.json"],
          "cases": {q["id"]: {"gold_approved": True, "independence_approved": True, "contamination_checked": True,
                              "unit_id": q["unit_id"], "cluster_id": q["cluster_id"]} for q in qs}}
(D / "review-record.json").write_text(json.dumps(review, ensure_ascii=False, indent=1) + "\n")
print(json.dumps({"questions": len(qs), "DEV": sum(q["split"] == "DEV" for q in qs), "FINAL": sum(q["split"] == "FINAL" for q in qs), "withdrawn": withdrawn}))
