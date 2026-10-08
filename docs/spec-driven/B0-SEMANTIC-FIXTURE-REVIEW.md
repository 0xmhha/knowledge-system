# B0 개발 의미 사실·팩 결합 검토 자료

2026-10-03 · B0-05 진행 · **2026-10-06 테스트 사례 범위 승인 완료; 새 source-bound 적용 진행**. 공식 품질 지표는 null이다. 원문 질문/소스 책과 기존 승인 기록을 바꾸지 않았다. 이 문서는 F-03/F-05/F-06 개발 사례의 추가 입력과 source-bound 관계를 검토하기 위한 자료다.

## 검토할 결정

| 검토 ID | 제안과 근거 | 승인 범위 | 판정 |
|---|---|---|---|
| SF-03 | 원문 공통 개념 20개 유지. `project`/`프로젝트`는 `project`와 `dataset` 두 후보; 미등록 질의는 후보 없음 | 개발 사례의 다의어/무승격 기대 동작. 개념 원문은 proposed 유지, 규범 사실 승인으로 사용하지 않음 | 테스트 사례 승인 |
| SF-05-A | `DEV-project-a requires a refund limit of 10.`를 정책 인스턴스에 그대로 보존. Alpha는 A 식별 토큰을 반환 | A의 정책 문구·적용 subsystem=refund, alpha-function 개념과 Alpha IMPLEMENTED_BY 출처. Alpha가 환급 정책을 집행한다는 주장 없음 | 테스트 사례 승인 |
| SF-05-B | `DEV-project-b requires a refund limit of 20.`를 정책 인스턴스에 그대로 보존. Alpha는 B 식별 토큰을 반환 | B의 정책 문구·범위·Alpha 관계. A의 정책/인용을 사용할 수 없음 | 테스트 사례 승인 |
| SF-06 | README는 RefundLimit ≤10을 요구하고 `main.go:3`은 20을 반환. 추가 스펙은 req-refund-cap → ac-refund-cap, refund-cap 개념을 명시 | 요구·수용 기준 문구와 개념→RefundLimit의 구현 위치 관계를 검토. 요구 불충족이라는 후보 판정은 사람 검토 대상이며 코드 수용 승인과 구분 | 테스트 사례 승인 |

`SF-06`의 TestHealth는 산술 건강 시험이며 실제 `go test ./...`는 통과했다. 이 실행은 환급 수용 기준을 검사하지 않는다. `CHECKED_BY`, `TESTED_BY`, `ACCEPTED_BY` 관계와 CriterionDecision 수용 승인은 만들지 않았다. 검토자가 요구 문구나 구현 위치를 승인하더라도 구현이 요구를 만족한다는 승인으로 기록하지 않는다.

## 고정 입력과 실제 앵커

F-03의 원문 project_id는 `b0-f03-dev`다. 초기 생성기의 `f-03-dev` 오부여를 수정했고, 실패 전/수정 후 Python 시험과 실제 의미 추출 통과를 보관했다. 원문 온톨로지 바이트와 원래 Git 커밋은 동일하다.

F-05/F-06은 원래 파일을 바꾸지 않고 온톨로지·팩·proposed 정책, F-06의 proposed 스펙을 별도 결정적 커밋에 추가했다. `.cks/knowledge`의 잠금은 바이트/팩 신원을 고정하며 사람의 사실 승인은 아니다. 기존 draft 입력 책에 이 추가 입력이 승인된 것으로 반영하지 않았다. 공식 입력 동결 전에 이 변경을 함께 검토해야 한다.

| 사례 | project_id | source commit | dataset_id | native CKG 앵커 |
|---|---|---|---|---|
| F-03-DEV-current | `b0-f03-dev` | `ab18b7b83b62e55812bbf48446c5cb8af470c801` | `ccb4f58820e34fe748ca89ced4747dad10a5a0b7c44e76755238eebf5488190f` | `example.com/b0dev.ProjectIdentity` · `main.go:3-3` |
| F-05-DEV-a | `f-05-dev-a` | `80e9f2f327a67b69bf394b2bf3145891216332c3` | `d95135491074c6e2520bbad910a1d22449e668a8f179a5e0a104419a16910f45` | `example.com/b0dev.Alpha` · `main.go:3-3` |
| F-05-DEV-b | `f-05-dev-b` | `d29ea2ef74d2ee4cbefdc8bdcc3335cd5c02b636` | `bceecdf4928fa00bf3ed07a501d647fd1fb88948a2bfb9000e6803c413a78abe` | `example.com/b0dev.Alpha` · `main.go:3-3` |
| F-06-DEV-current | `f-06-dev` | `498c625431321aa7905bd6bf2746a50ff136010a` | `0ec083f511152b1cf60530df4656a87cee0835bec45694695ca3cd1ac4c37b8e` | `example.com/b0dev.RefundLimit` · `main.go:3-3` |

A/B의 canonical ID는 둘 다 `example.com/b0dev.Alpha`일 수 있다. 서로 다른 project/dataset/snapshot/commit과 원문 SHA를 함께 사용해야 하며 canonical ID만으로 합치지 않는다. 각 원장의 base_commit/base_tree와 supplement_commit/supplement_tree가 원본·추가 입력의 계보를 기록한다.

## 원문과 투영 열람

- [F-03-DEV-current 투영](../../system/eval/b0-knowledge-system/semantic-fixture-preparation-m2max-2026-10-03/real/F-03-DEV-current/projection-proposed.json) · [원문 코드](../../system/eval/b0-knowledge-system/semantic-fixture-preparation-m2max-2026-10-03/real/F-03-DEV-current/source/main.go) · [원천 온톨로지](../../system/eval/b0-knowledge-system/semantic-fixture-preparation-m2max-2026-10-03/real/F-03-DEV-current/source/ontology.yaml)
- [F-05-DEV-a 투영](../../system/eval/b0-knowledge-system/semantic-fixture-preparation-m2max-2026-10-03/real/F-05-DEV-a/projection-proposed.json) · [원문 코드](../../system/eval/b0-knowledge-system/semantic-fixture-preparation-m2max-2026-10-03/real/F-05-DEV-a/source/main.go) · [원천 온톨로지](../../system/eval/b0-knowledge-system/semantic-fixture-preparation-m2max-2026-10-03/real/F-05-DEV-a/source/ontology.yaml)
- [F-05-DEV-a proposed 정책](../../system/eval/b0-knowledge-system/semantic-fixture-preparation-m2max-2026-10-03/real/F-05-DEV-a/source/.cks/knowledge/policies/refund-cap.yaml) · [잠금](../../system/eval/b0-knowledge-system/semantic-fixture-preparation-m2max-2026-10-03/real/F-05-DEV-a/source/.cks/knowledge/knowledge.lock.json)
- [F-05-DEV-b 투영](../../system/eval/b0-knowledge-system/semantic-fixture-preparation-m2max-2026-10-03/real/F-05-DEV-b/projection-proposed.json) · [원문 코드](../../system/eval/b0-knowledge-system/semantic-fixture-preparation-m2max-2026-10-03/real/F-05-DEV-b/source/main.go) · [원천 온톨로지](../../system/eval/b0-knowledge-system/semantic-fixture-preparation-m2max-2026-10-03/real/F-05-DEV-b/source/ontology.yaml)
- [F-05-DEV-b proposed 정책](../../system/eval/b0-knowledge-system/semantic-fixture-preparation-m2max-2026-10-03/real/F-05-DEV-b/source/.cks/knowledge/policies/refund-cap.yaml) · [잠금](../../system/eval/b0-knowledge-system/semantic-fixture-preparation-m2max-2026-10-03/real/F-05-DEV-b/source/.cks/knowledge/knowledge.lock.json)
- [F-06-DEV-current 투영](../../system/eval/b0-knowledge-system/semantic-fixture-preparation-m2max-2026-10-03/real/F-06-DEV-current/projection-proposed.json) · [원문 코드](../../system/eval/b0-knowledge-system/semantic-fixture-preparation-m2max-2026-10-03/real/F-06-DEV-current/source/main.go) · [원천 온톨로지](../../system/eval/b0-knowledge-system/semantic-fixture-preparation-m2max-2026-10-03/real/F-06-DEV-current/source/ontology.yaml)
- [F-06-DEV-current proposed 정책](../../system/eval/b0-knowledge-system/semantic-fixture-preparation-m2max-2026-10-03/real/F-06-DEV-current/source/.cks/knowledge/policies/refund-cap.yaml) · [잠금](../../system/eval/b0-knowledge-system/semantic-fixture-preparation-m2max-2026-10-03/real/F-06-DEV-current/source/.cks/knowledge/knowledge.lock.json)
- [F-06 스펙·수용 기준](../../system/eval/b0-knowledge-system/semantic-fixture-preparation-m2max-2026-10-03/real/F-06-DEV-current/source/spec.yaml) · [README 요구](../../system/eval/b0-knowledge-system/semantic-fixture-preparation-m2max-2026-10-03/real/F-06-DEV-current/source/README.md) · [관련 없는 테스트](../../system/eval/b0-knowledge-system/semantic-fixture-preparation-m2max-2026-10-03/real/F-06-DEV-current/source/main_test.go)

모든 의미 EvidenceSpan은 같은 tuple의 보관 원문 줄 SHA와 맞으며 코드 앵커는 실제 CKG DB에서 찾았다. native `semantic promote` 검증기에 proposed 투영을 저장해 원문·좌표·AST 검증을 수행했다. 모든 개념/요구/관계의 status는 proposed이고 reviewed_by는 비어 있다. semantic_current는 0개이며 활성화하지 않았다. 최종 변형은 빌드·조회하지 않았다.

## 기존 proposed 입력의 실제 동작 확인

mock/승인된 실제 BGE-M3에서 각 4개 데이터셋을 준비하고 v2 기본 요청과 combined+include_knowledge 요청 14개씩을 실행했다. 실제 BGE-M3는 기존 승인 digest `790764642607…16bab`, 1024차원이며 전체 임베딩 모드를 강제했다. 실제 다의어 한/영 질의는 두 후보를 유지하고 미등록 질의는 no_match였다. mock은 일부 원문 후보가 없어 no_candidates를 반환하므로 그 행은 다의어 순위 품질의 증거로 사용하지 않는다.

proposed 상태에서는 관계 적용/텍스트 검색/boost가 0이고 정책 문맥은 unknown이며 확정 정책/요구/구현 추적을 추가하지 않았다. combined+지식 요청의 기존 인용·본문 집합은 baseline과 같고 v2 무결성 및 의미 DB SHA를 확인했다. 이는 무승격·정렬의 구조 진단이며 공식 Recall/MRR/지연 판정이 아니다.

첫 실제 요청은 `knowledge_context_failed`였다. Ontology 메타데이터를 추가한 뒤 지식 계층의 Verify 전에 해시를 갱신하지 않은 순서 오류였다. 수정 전 실제 MCP 오류와 Go 회귀 실패를 보관하고, 계층 진입 전에 stamp하도록 수정했다. 지식 요청의 의미 DB도 읽기 전용으로 열어 자동 쓰기/마이그레이션을 방지한다. 전체 Go·관련 race·vet·Python 회귀와 수정 후 실제 MCP를 확인했다.

[준비·검증 원장](../../system/eval/b0-knowledge-system/semantic-fixture-preparation-m2max-2026-10-03.json)에 실행 명령·입력 책/바이너리/원자료 SHA·투영·정확한 입력 Git bundle을 기록한다. 큰 데이터셋/모델/DB는 저장소에 넣지 않았다.

## 승인 이후의 적용 절차

SF-03/05-A/05-B/06의 테스트 사례 승인을 실제 원문과 시각·로컬 chat-user 기록에 연결했다. [승인 기록](./B0-APPROVED-INPUTS.md)을 따른다. 의미 사실 검토, 동적 사례/프로토콜 승인, 최종 수용 판정은 각각 기록한다. 소스 기반 개념/요구 검토 메타데이터와 정책 quorum 기록을 실제 입력에 반영하면 새 커밋·잠금·dataset_id가 필요하다. 승인 전 투영을 제자리에서 verified로 바꾸지 않는다. 새 원문/tuple로 다시 검증한 뒤 공식 B0-07/08과 8-arm 비교에 사용한다.

전체 우선순위와 상태는 [작업리스트](./EXECUTION-WORKLIST.md), 기존 동적/프로토콜 검토는 [B0-M2MAX-REVIEW](./B0-M2MAX-REVIEW.md)를 따른다.
