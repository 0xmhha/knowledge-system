"""Freeze FINAL2 part B under the user's approval of the draft ("모두 승인,
표기 opus5.5 검토"). The four approved questions are bound to the derived
corpus commit; one unjudged DEV plumbing question is added because the input
gate requires a DEV split. Run from the repository root."""
import datetime as dt, hashlib, json
from pathlib import Path

D = Path("system/eval/b0-knowledge-system/refactoring-final2-b-2026-10-10")
A = Path("system/eval/b0-knowledge-system/refactoring-final2-a-2026-10-10")
prep = json.loads((D / "preparation.json").read_text()); src = Path(prep["source"]); commit = prep["source_commit"]
REVIEWER, KIND = "opus5.5 검토", "ai_model_label_designated_by_user"
sha = lambda b: hashlib.sha256(b).hexdigest()
def span(path, first, last):
    data = (src / path).read_bytes(); lines = data.splitlines(keepends=True)
    return {"path": path, "first": first, "last": last, "file_sha256": sha(data), "content_sha256": sha(b"".join(lines[first - 1:last]))}
release_line = json.loads((D / "source-inventory.json").read_text())["release_basis_line"]
common = dict(language="ko", source_commit=commit, review_state="approved", reviewer=REVIEWER, reviewer_kind=KIND,
              authority_scope="bound repository records in the derived corpus only",
              independence_review="approved: new fact clusters; questions and declared-unknown match terms were written by the same agent (recorded weakness)",
              contamination_review="approved: derived corpus excludes system/eval/**, docs/spec-driven/N11-*, N12-*, FINAL2-*")
abs_rule = "citing the recorded declared-unknown basis while abstaining is correct; citing any other source as the answer is not"
qs = [
 dict(id="F2B-DEV-01", split="DEV", group="TRACE", cluster_id="n14-base-rollback-refusal", expected_behavior="cite",
      prompt="patch 수용 시험에서 base 버전으로 rollback이 거부된 이유는 무엇인가요?",
      candidate_answer="base는 N12의 hold 후보를 복사한 것이라 검토 해제된 적이 없어, rollback이 explicit patch review를 요구하며 거부되었다.",
      evidence=[span("docs/spec-driven/N14-CHANGE-ACCEPTANCE.md", 18, 18)], note="unjudged DEV plumbing question added at freeze (input gate requires a DEV split)"),
 dict(id="F2B-ABS-01", split="FINAL", group="ABS", cluster_id="policy-owner-unassigned", expected_behavior="abstain",
      prompt="이 프로젝트에서 knowledge 정책 레코드를 작성하고 승인하는 정책 담당자는 누구로 지정되어 있나요?",
      candidate_answer="지정되지 않았다. 정책 담당자가 없어 정책 instance가 없다.", evidence=[], strict_zero_citations=False,
      abstention_basis="the derived corpus records that no policy owner exists; declared unknown UNK-POLICY-OWNER",
      abstention_basis_sources=[span("docs/spec-driven/N14-CHANGE-ACCEPTANCE.md", 34, 34)], abstention_citation_rule=abs_rule),
 dict(id="F2B-ABS-02", split="FINAL", group="ABS", cluster_id="release-verdict-pending", expected_behavior="abstain",
      prompt="ontology와 knowledge 기능을 기본으로 켜고 출시하기로 한 판정은 언제, 누가 내렸나요?",
      candidate_answer="판정되지 않았다. 합격 근거가 없으면 disabled와 출시 보류를 유지한다.", evidence=[], strict_zero_citations=False,
      abstention_basis="the derived corpus records that the N-18 default/release verdict is pending; declared unknown UNK-RELEASE-VERDICT",
      abstention_basis_sources=[span("docs/spec-driven/REFACTORING-REMAINING-WORKLIST.md", release_line, release_line)], abstention_citation_rule=abs_rule),
 dict(id="F2B-ANS-01", split="FINAL", group="POLICY", cluster_id="policy-record-required-fields", expected_behavior="cite", contrast_of="F2B-ABS-01",
      prompt="knowledge 정책 레코드가 verified 상태로 받아들여지려면 어떤 필드와 날짜 조건을 갖춰야 하나요?",
      candidate_answer="ID·type·statement·owner·scope·자기 파일 source_ref·검토자와 이유·public/restricted visibility가 필요하고, verified면 effective_from이 있어야 하며 effective_to는 그 이후여야 한다.",
      evidence=[span("internal/system/knowledgepack/instances.go", 534, 557)]),
 dict(id="F2B-ANS-02", split="FINAL", group="POLICY", cluster_id="local-trust-path", expected_behavior="cite", contrast_of="F2B-ABS-02",
      prompt="로컬 운영에서 설치한 cks 바이너리를 무엇으로 신뢰하기로 했나요?",
      candidate_answer="로컬 소스 빌드를 commit과 binary SHA로 확인한다.",
      evidence=[span("docs/spec-driven/N15-N17-LOCAL-OPERATION-ACCEPTANCE.md", 14, 14)]),
]
for q in qs:
    q.update(common); q["unit_id"] = q["proposed_cluster_id"] = q["cluster_id"]
book = {"schema_version": 1, "status": "approved", "source_commit": commit, "reviewer": REVIEWER, "reviewer_kind": KIND,
        "purpose": "FINAL2 part B: declared-unknown abstention and answerable contrasts on a corpus carrying the approved overlay",
        "questions": qs}
pa = json.loads((A / "protocol.json").read_text())
config_raw = (D / "runtime-config.yaml").read_bytes()
ident = json.loads((Path(prep["dataset_root"]) / "final2b-candidate/dataset-identity.json").read_text())
proto = dict(pa)
for k in ("binary_comparison", "final_groups_out_of_scope", "draft_sha256", "previous_frozen_n11_protocol_sha256"):
    proto.pop(k, None)
proto.update(source_commit=commit, status="approved", reviewer=REVIEWER, reviewer_kind=KIND, execution_allowed=True,
             execution_binding={"dataset_id": ident["dataset_id"], "config_sha256": sha(config_raw), "binary_sha256": prep["binaries_sha256"]["cks"]},
             request_scope={"knowledge_as_of": "2026-10-10", "knowledge_subsystem": "knowledge-system"},
             final_groups_out_of_scope={"CODE": "judged by FINAL2 part A", "WHY": "judged by FINAL2 part A", "TRACE": "judged by FINAL2 part A"},
             corpus_exclusions={"rule": "Evaluation material is excluded from the derived corpus.", "paths": ["system/eval/**", "docs/spec-driven/N11-*", "docs/spec-driven/N12-*", "docs/spec-driven/FINAL2-*"], "decided_by": "user approval of the FINAL2 draft"},
             abstention_scoring={"rule": abs_rule, "mechanical_only": True, "answer_level_abstention": None},
             part_b_rule="Mechanical: off-basis citation rate on ABS rows and Recall@10 on contrast rows, knowledge on (baseline_on) versus off (baseline_off). Every group is below ten units: inconclusive. Ontology arms have no projection for this dataset (unavailable); part B latency is not judged.",
             execution_note="User approved the FINAL2 draft and its execution (refactoring-final2-a-2026-10-10/user-decision.json).")
book_b = (json.dumps(book, ensure_ascii=False, indent=1) + "\n").encode()
proto_b = (json.dumps(proto, ensure_ascii=False, indent=1) + "\n").encode()
(D / "question-book.json").write_bytes(book_b); (D / "protocol.json").write_bytes(proto_b)
now = dt.datetime.now(dt.timezone.utc).replace(microsecond=0)
review = {"status": "approved", "scope": "fresh-evaluation-inputs", "reviewer": REVIEWER, "reviewer_kind": KIND,
          "independent_human_content_review": False,
          "reviewed_at": (now - dt.timedelta(seconds=1)).isoformat().replace("+00:00", "Z"), "frozen_at": now.isoformat().replace("+00:00", "Z"),
          "protocol_sha256": sha(proto_b), "questions_sha256": sha(book_b),
          "decision_sources": [str(A / "user-decision.json"), "docs/spec-driven/FINAL2-DRAFT-REVIEW.md"],
          "cases": {q["id"]: {"gold_approved": True, "independence_approved": True, "contamination_checked": True,
                              "unit_id": q["unit_id"], "cluster_id": q["cluster_id"]} for q in qs}}
(D / "review-record.json").write_text(json.dumps(review, ensure_ascii=False, indent=1) + "\n")
known = json.loads((A / "known-observations.json").read_text())
(D / "known-observations.json").write_text(json.dumps(known, ensure_ascii=False, indent=1) + "\n")
print(json.dumps({"questions": len(qs), "dataset": ident["dataset_id"], "protocol_sha256": sha(proto_b)}))
