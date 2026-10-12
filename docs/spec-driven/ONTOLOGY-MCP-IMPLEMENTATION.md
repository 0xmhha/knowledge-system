# 온톨로지 MCP 보완 구현 기록

2026-10-13 · **O01 준비 상태 구현·검증 완료, O02–04 미완료**. 기존N17/18·잔여N16 유지. 온톨로지1/4·잔여3개, 전체 잔여4개(N16/O02/O03/O04).

## O01: cks.semantic.status

입력 없음. 검색이나 모델 호출 없이 이 MCP 서버에 고정된 source/dataset와 읽기 전용 의미 projection을 검증한다. `mode`와 `availability`를 분리하므로 검색 mode off여도 자료가 준비돼 있으면 `read_tools_ready=true`, `retrieval_ready=false`다.

- state: off, ready, unreviewed 또는 불가 상태.
- availability: unconfigured, dataset_missing, store_missing, store_unavailable, unsupported_dataset, projection_missing, stale, budget_exceeded, ready. 잘못된 mode는 unsupported.
- reason: 안정된 이유 코드. 원문·DB error·로컬 path는 제공하지 않는다.
- counts/snapshot: retained source와 engine coordinate/해당 evidence 검증 통과 시만 제공. 개념/요구/관계와 verified 수 및 실제 usable 구현 관계 수를 구별한다.
- read_tools_ready는 정렬된 자료를 읽을 수 있다는 뜻. retrieval_ready는 설정 mode에 맞는 검토 개념/구현 자료가 있다는 뜻이며 실제 query match/품질/시험/criterion 수용을 의미하지 않는다. relations mode에는 usable reviewed 구현이 필요하고 text/combined는 reviewed 개념이 필요하다.
- 새 도구가 missing store를 생성하거나 current/semantic activation을 바꾸지 않는다. source/store 불일치와 deadline 뒤 결과는 counts/좌표 없이 거부한다. 기본 budget1초, 서버 설정에서 최대5초.

[코드](../../internal/system/mcp/semantic_status.go), [수용 시험](../../internal/system/mcp/semantic_status_test.go). 실제 retained YAML capture/ontology parser·SQLite에 문서-only fixture를 저장한 positive/off/verified-proposed 및 누락/stale/취소/경로 비노출/current 없음·store 바이트 불변을 확인했다. 코드 앵커/CKV DB가 없는 문서 fixture이며 실제 운영 도메인 수용이라고 기록하지 않는다.

MCP 등록 없음 RED→도구25개 등록/input/output 계약 GREEN. 관련3패키지 전체 race **171시험·156하위시험 pass, fail0/skip0**, vet/엔진 경계/diff 검증 통과. 진행 보호 Python **11시험 pass**; O목록/원장 누락·중복·완료 표기 불일치·완료된 next 거부가 추가됐다. 처음 만든 fixture의 추출기 종류 오류는 제품 regression과 구별해 보존했다. [원자료](../../system/eval/b0-knowledge-system/ontology-o01-status-2026-10-13/verification.json).

다음 O02는 같은 pinned retained 검증을 공유해 용어 후보·개념/관계·요구 추적을 조회한다. O03은 설치 연결, O04는 설명/적용 전후 평가다. 새 실제 도메인 승인/모델 평가/운영 활성화는 아직 없다.
