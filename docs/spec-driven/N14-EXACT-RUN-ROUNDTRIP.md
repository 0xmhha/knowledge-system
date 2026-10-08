# N14 생산 exact 보고서 등록 통합 검증

2026-10-08 · **소유한 제어 fixture의 통합 검증 완료 / 실제 파일럿 수용 미완료**.

[영구 회귀 시험](../../internal/system/patch/review_roundtrip_test.go) · [race 결과](../../system/eval/b0-knowledge-system/refactoring-n14-run-roundtrip-2026-10-08/race-tests-summary.json) · [교정 초안 파서 결과](../../system/eval/b0-knowledge-system/refactoring-n14-run-roundtrip-2026-10-08/draft-parser-control.json).

## 목적과 검증 계약

앞 단계에서 보완한 보고서 검사는 손으로 만든 정상 보고서를 허용했다. 이번 단계는 생산 `ExecuteLinkedGoTestFor`가 생성한 정상 보고서를 공개 `patch.RecordRun`이 원문·CKG·retained semantic 검증을 거쳐 받아들이는지 확인한다. 시험 pass와 기준에 대한 사람의 의미 판정은 계속 분리한다.

root와 nested package 각각 별도 소유 Git 저장소를 만들고, Alpha의 반환값을 0에서 1로 바꾼 실제 두 커밋을 캡처한다. base/current와 held candidate도 이 임시 제어 데이터셋 안에만 존재한다. CKG는 공개 CLI로 실제 AST·canonical DB를 구축하며, source spans와 SHA는 그 DB의 실제 노드 및 원문에서 얻는다. 시스템→그래프 내부 import를 추가하지 않는다.

CKV는 mock 모델의 **좌표 manifest만 있는 제어 fixture**다. 선언한 chunk links가 없어 CKV DB 조회를 하지 않는다. 임베딩·실제 검색·CKV 품질·BGE 호환성의 증거로 사용하지 않는다. YAML과 관계의 `unit-fixture` reviewer는 시험 조건이며 실제 사람 검토가 아니다.

ontology/spec는 생산 파서로 추출하고, 개념→구현→시험→기준의 완전한 경로를 구성한다. 각 시험에는 별도 ID의 TESTED_BY 2개와 CHECKED_BY 2개를 부여하여 생산 runner의 전체 proof 집합과 등록 검사의 계약을 함께 확인한다. 모든 원문·canonical·dataset identity를 `PutAlignedRetained`/`LoadAlignedRetained`로 검증한 뒤 exact Go 시험을 실행한다.

| 검증 | root | nested |
|---|---|---|
| 실제 CKG canonical 및 retained 원문 일치 | 통과 | 통과 |
| exact TestAlpha 관찰·pass, snapshot 일치 | 통과 | 통과 |
| 생산 보고서 등록·같은 보고서 재시도·읽기 일치 | 통과 | 통과 |
| 다른 TESTED_BY ID 거부, run 파일 미생성 | 통과 | 통과 |
| 구조상 유효한 다른 output SHA로 기존 run 교체 거부·바이트 보존 | 통과 | 통과 |
| patch unconfirmed, 사람 결정 없음, current=base, semantic current 없음 | 통과 | 통과 |
| review-release/test-gate 미생성 | 통과 | 통과 |

보고서 output hash는 생산 runner가 계산한다. 이번 시험은 외부 보고서의 독립 서명/진위 인증이나 원시 출력과의 별도 attestation을 추가하지 않는다. report import가 기준을 자동 승인하지 않는 계약을 확인한다. 실제 파일럿 저장소·held candidate는 열거나 수정하지 않았다.

## 초안 교정과 확인 범위

N12의 이전 [미적용 ontology 초안](../../system/eval/b0-knowledge-system/refactoring-n12-concept-gap-audit-2026-10-08/functional-ontology-option.yaml)에 필수 `competency_questions`가 빠졌다. 이전 단계는 일반 YAML/참조 구조만 확인했으며 생산 projection 검증으로 보고하지 않았다. 생산 `ExtractOntology`가 이전 초안을 거부하는 것을 재현하고, 이전 파일을 보존한 채 [교정 초안](../../system/eval/b0-knowledge-system/refactoring-n14-run-roundtrip-2026-10-08/functional-ontology-option.yaml)에 질문 3개를 추가했다.

생산 ontology/spec 파서 및 병합 `Projection.Validate`가 개념3개·명세3개·기준4개를 허용한다. spec 초안의 바이트는 이전과 같다. 전부 proposed/reviewer 없음/assertions0이다. 파서 제어 좌표는 합성 값이며 source-backed validation=false, applied=false, human_approved=false다. 실제 snapshot·후보·lock에는 적용하지 않았다. 범위 선택 및 실제 개념·관계 판정은 계속 필요하다.

## 실행 증거와 완료 체크

- [x] 실제 CKG CLI/원문 retained 경로와 생산 exact runner→공개 등록 통합 회귀 추가.
- [x] root/nested 두 경로의 정상 등록·재시도, 잘못된 proof 거부·immutable report 보존, 자동 승인·승격 없음 확인.
- [x] 직접 관련 race 회귀 **47시험·33하위시험**, 실패0; patchcli는 no-test 컴파일 대상. 관련3패키지 vet, 엔진 경계, 대상 포맷 통과.
- [x] 이전 초안 생산 파서 거부 및 필수 질문 교정본의 파서·구조 검증. 원본 두 파일 SHA와 spec 불변 확인 보관.
- [ ] 실제 N12 명세 범위·개념/관계 검토를 반영한 source-backed 후보 구성.
- [ ] 실제 후보 exact 실행 보고서와 별도 사람의 기준 판정·수용/반려·rollback.

첫 제어 fixture 작성 중 YAML flow 질문의 물음표를 따옴표 없이 적어 파서부터 실패했다. 교정 후 정상 경로가 통과했다. 해당 원자료는 `authoring-tests.jsonl.gz`로 보존하며 제품 RED로 세지 않는다. `roundtrip-tests.jsonl.gz`와 `race-tests.jsonl.gz`의 원문/압축 SHA를 함께 기록한다. 실행 환경은 Go1.26.8 Darwin ARM64다. 새 Linux 실행이나 전체 Go 회귀 재실행은 없다. 앞 단계의 전체 회귀는 당시 증거로 보존한다.

실제 linked report0·사람 decision0·실제 patch 등록0·실제 promotion0·새 FINAL0·모델 HTTP0이다. actual human review time=null을 유지한다. **N14-A–E 미체크, 전체8/18·요청5/15·남은10개 유지**.

다음은 N12 명세 범위 선택 및 개념/관계 판정, N11 gold/군/독립성 판정과 부족 표본 보강이다. 이 조건을 충족한 뒤 source/runtime binding과 후보를 새로 구성한다. 남은 전체: **N08 검색 품질, N09 기권, N10 비용·지연, N11 독립 평가, N12 파일럿 검토, N14 변경 수용, N15 규모·이식성, N16 신뢰·권한, N17 실제 복구, N18 기본값·출시 판정**.
