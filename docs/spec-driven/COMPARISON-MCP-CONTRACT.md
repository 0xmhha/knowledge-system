# 읽기 전용 다중 프로젝트 비교 MCP 계약

KM-06.1의 요청/설정 계약을 구현했다. 실제 공개 MCP 실행·근거 행렬·부분 실패·실제 프로젝트 검증은 KM-06.2–4다. 이 문서의 설계를 서버 실행 완료로 세지 않는다. [공통 개념/연결](./INDUSTRY-CONCEPT-COMPARISON.md), [상위 명세](./KNOWLEDGE-MANAGER-AND-COMPARISON-SPEC.md), [시험표](./KNOWLEDGE-ROADMAP-TEST-MATRIX.md)를 따른다.

## 목적과 경계

LLM/사용자가 선택한 프로젝트별 코드·문서·설정/정책·테스트의 비교 자료를 얻도록 한다. 동일 개념이나 유사 역할로 프로젝트 신원·구현·정책을 합치지 않는다. 도구는 source evidence를 제공하고 장단점·우월성·측정하지 않은 성능/보안성의 최종 결론을 만들지 않는다. 구현 문맥의 단일 target 경로는 기존 collection 구현 모드를 사용하며 새 요청은 comparison 모드만 받는다. patch target·쓰기·current 전환·job 접수를 제공하지 않는다.

공개 도구명은 선택 설정의 cks.comparison.describe와 cks.comparison.extract로 고정한다. 독립적인 비교 진입점은 mcp 명령 그룹 아래 comparison 하위 명령으로 계획한다. 기존 기본 MCP·collection·maintenance·manager의 measured 모드와 분리하여 source-only 비교 때문에 단일 프로젝트 backend나 dummy 모델을 열지 않는다. 실제 등록/진입점은 다음 단계에서 구현하며 현재 실행 명령으로 안내하지 않는다.

## 설정과 고정 신원

ComparisonProfileDocument schema1은 name, catalog_path, scheme_root, 선택적 bindings_path/reviews_path, sanitize_rules_path, scopes를 갖는1MiB 로컬 JSON이다. 절대·정규화 경로만 선언한다. 등록 범위는 최대128개다. schema decoder는 형상만 검사하고 파일 존재·원문 읽기 준비·실제 MCP 연결을 보장하지 않는다. runtime loader가 다음 단계에서 no-follow·파일 byte hash·전체 profile digest를 고정해야 한다. 그때까지 순수 계약 생성자의 profile digest는 호출자가 제공한 pin이다.

프로젝트 scope는 전체 V2Coordinates + industry + network/configuration/component_role/observed_at이다. scope ID는 이 tuple의 SHA-256이며 project name·로컬 binding ID·URI를 전역 신원으로 사용하지 않는다. 동등한 timezone 표현과 industry 대소문자/공백만 정규화하고 network/config/역할 ID는 임의로 합치지 않는다. 한 프로젝트의 다른 버전·network/config도 다른 scope다. catalog에 등록하지 않은 scope나 정규화 후 중복 scope는 거부한다.

catalog digest는 기존 canonical collection 계약을 재사용한다. runtime profile digest는 profile/선택 파일·scheme·sanitization의 실제 고정 내용을 포함해야 한다. 요청의 profile/catalog pin이 다르면 stale_configuration으로 거부하고 describe를 다시 요청해야 한다. 서버는 실행 중 다른 current·변경 파일·다른 프로젝트로 암묵 재선택하지 않는다. 값/목록을 반환한 호출자가 수정해도 서버의 선택 상태를 바꾸지 못한다.

## 요청 계약

| 필드 | 계약 |
|---|---|
| schema_version, mode | 1, comparison만 허용 |
| profile_digest, catalog_digest, collection_id | 실제 선택 snapshot과 정확히 일치 |
| participants | 해당 collection에 속한 명시 scope ID2–16개; 중복/unknown 거부 |
| concept | 선택 pack ID/version/digest/concept ID 전체 tuple |
| facets | 해당 개념에 정의된1–8개 축; 중복/unknown 거부 |
| question | 선택적4KiB 이하 의도 텍스트; 실행 지시/자동 사실 해석으로 사용하지 않음 |
| as_of | 명시 RFC3339; 서버 시각 이후 미래나 잘못된 시각 거부 |
| budget | body byte1–262144, citation1–128, timeout1–30000ms |
| include_proposed | 기본 false; 명시 true에서만 근거/경고를 가진 미검토 후보 허용 |

기본 예산 생성기는 위 최대값을 반환한다. 요청은 양수 상한을 명시하고0·초과·형식 오류를 거부한다. 누락된 참여자/개념을 자연어 일부 문자열에서 추측하지 않는다. metadata의 unknown 용어·복수 별칭 후보는 describe에서 선택해야 한다. 같은 프로젝트라도 다른 scope를 명시했다면 비교할 수 있다. 다른 collection의 멤버는 그 프로젝트가 설정에 있다는 이유로 선택할 수 없다.

요청은 설정/원천/경로·patch target·backend를 바꾸는 필드를 받지 않는다. 엄격 JSON과 lower_snake_case 속성 이름을 적용한다. question 및 원문은 데이터이며 명령으로 실행하지 않는다. 이 계약의 Select/DecodeRequest는 routing/예산 검증이며 backend 호출이나 자료 조회 성공이 아니다.

## 후속 행렬·오류·노출 설계

KM-06.2는 각 participant/facet에 공통 정의·로컬 구현/정책·source 종류·인용·mapping 이유/검토·조건·unknown을 분리한 행렬을 제공한다. 공통/차이 후보에는 양쪽 프로젝트의 근거와 이유를 요구하며 동일 용어나 source text가 행동 동일성을 증명한다고 주장하지 않는다. 기본은 현재 recorded review가 있는 자료이고 명시 후보 모드는 proposed를 유지한다.

KM-06.3은 한 프로젝트의 source 부재/손상과 다른 프로젝트의 정상 결과를 분리한다. 전체 profile/catalog 형상과 membership을 먼저 검증한 후 선택 버전별로 reader를 열어 한 실패가 시작 시 전체를 막지 않도록 한다. 등록 밖 경로/프로젝트로 fallback하지 않는다. 같은 project/scope의 상충은 유지하고 다른 프로젝트의 차이는 한 내부 모순으로 만들지 않는다. 요청 예산/기한·응답 byte와 인용 상한·reader lease·EOF/신호 정리를 검증한다. 부분 실패/누락/초과를 조용히 숨기거나 자료 완전성으로 표현하지 않는다.

KM-06.4는 둘 이상의 실제 로컬 프로젝트를 고정 버전/범위로 DB화해 staking/정책·consensus/finality·mempool/ordering 자료를 공개 도구로 확인한다. 임시 복제본에서 작업하고 원본 프로젝트는 수정하지 않는다. 모의 embedding의 lifecycle/transport 결과를 실제 품질로 확대하지 않는다. 실제 모델 비용·사람/독립 수용을 임의 생성하지 않는다.

설정 읽기·도구 등록·형상/계약 통과는 serving readiness가 아니다. describe는 configured와 아직 검증하지 않은 source/read 상태를 구분해야 한다. 실제 조회·부분 실패·호스트 연결·품질 효과는 각각의 시험 결과로 기록한다. 기존 온톨로지 기본 off·quality=inconclusive는 이 계약 승인/구현으로 바꾸지 않는다.


## KM-06.1 완료 기록

KM-06.1 완료: 읽기 전용 comparison 모드·고정 profile/catalog·명시 scope 참여자/공통 개념/축·시각/예산 계약을 구현했다. 이름이나 자연어에서 대상을 추측하지 않고2–16개 scope와1–8개 축을 선택한다. stale/타 collection/동일 범위 중복·구현 모드·요청 경로/target 변조·미래 시각/초과 예산을 거부한다. 한 프로젝트의 명시적인 다른 조건도 비교 가능하며 호출자 변경으로 고정 라우팅을 바꾸지 못한다. 이 단계는 계약50개 검증이며 actual source read/MCP 호출0이다.

비교167개(기존117+계약50), 전체 Go4029(4011통과18skip)·race1003(1001통과2skip)·빌드/fmt/vet/lint·문서134개를 검증했다. 순수 profile/요청 검증이며 실제 file loader/source query/공개 MCP 실행은 아니다. [동결 증거](../../system/eval/knowledge-roadmap-2026-10-16/km-06-contract/manifest.json)를 따른다. 전체21/28·기능5/7이다. 다음은 KM-06.2의 공개 MCP·프로젝트별 근거 행렬/bundle 구현이다. 남은7개는 KM-06.2–4, OE-01.1–4이며 정책 생성기 보완·운영/독립 수용9개·선택 후속12개는 별도 유지한다. 사용자가 멈추기 전까지 단계별 서명 커밋으로 계속한다.
