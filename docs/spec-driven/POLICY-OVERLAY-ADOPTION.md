# 고정 소스용 정책 수정 적용·조회 검증

2026-10-15 사용자 응답 “새 평가용 overlay로 채택 (권고)”에 따라 아래7건 수정이 적용된 기존 별도 평가 profile의 선택을 승인 기록으로 확정했다. [승인 영수증](../../system/eval/operational-preparation-2026-10-15/adopted-policy-user-approval/receipt.json)에 질문 ID·정확한 응답·source/policy/profile hash와7개 참조를 보존했다. 기존 평가 근거를 다시 검사해 통과했으며 원본 YAML의 reviewer/approval, 운영 current, 독립 gold/품질 수용은 변경하지 않았다. 생성기 재생성 보완은 별도 잔여다.

2026-10-15. [정책 참조 진단](./POLICY-REFERENCE-DIAGNOSIS.md)의 수정7건을 [별도 평가 정책](../../projects/stablenet/evaluation/policy-0bf2f4d1.yaml)과 [profile](../../projects/stablenet/evaluation/policy-0bf2f4d1.profile.json)로 채택했다. 실제 CKG build→SQLite 저장→공개 CKS stdio 도구까지 검증했다. 원래 정책·domain entry·과거 source-bound packet·운영 current는 변경하지 않았다. 새 평가 profile의 로컬 적용 완료이며 전체 정책 의미의 독립 수용·온톨로지 효과·자동 재생성 수정은 완료가 아니다.

## 실행 범위와 결과

[원자료 manifest](../../system/eval/operational-preparation-2026-10-15/adopted-policy/manifest.json)에 명령·바이너리 buildinfo/SHA·source inventory·DB manifest·저장된 정책/연결 행·실제 MCP 요청/응답·초기 실패를 결속했다. [실행기](../../scripts/eval-adopted-policy.py)는 source commit0bf2f4d1bfeb6605006d556957ef8c045d8f8ed8의 non-test Go882개와 원래 go.mod/go.sum을 소유한 임시 경로에 복구한다. Go packages discovery가 실제 선택한 파일은843개다. build tag/플랫폼으로 제외된 파일과 non-Go 자료·테스트를 포함한 전체 프로젝트 build 검증으로 확대하지 않는다. 대상의 테스트·명령·노드는 실행하지 않았다.

| 검증 | 관측 |
|---|---|
| graph build |125,089 node /176,822 edge, parse error0, unresolved ref0 |
| 정책 저장 |42개 policy /146개 governed_by |
| 소스 진단과 저장 위치 대조 |146개 모두 정책/target/file/line 일치 |
| 수정7개 참조·6개 distinct target |올바른 원문 구간 SHA와 실제 정책 조회 일치 |
| canonical/source/snapshot 조회 |7건씩 실제 기본 조회와 동일한 symbol·annotation |
| 공개 거부/음성 제어 |source mismatch, snapshot mismatch 거부; EVM assembler에 계약 재생성 정책 없음 |
| MCP |advertised tools33개, 실제 policy_security 호출24개, 초기화/도구 목록 포함 요청·응답26쌍 |
| 종료 |CKS EOF exit0, 강제 종료 없음 |

CKV는 mock이며 이 도구는 graph annotation 경로만 사용했다. 임베딩·온톨로지 retrieval·실제 Claude host·유료 모델·답변 품질을 검증한 실행이 아니다. 임시 소스에는 Git metadata가 없어 API의 indexed_commit은 빈 값이다. 이를 원래 commit으로 꾸미지 않고 외부 Git blob/source inventory와 원문 구간 SHA로 결속했다. graph DB 약170MB는 manifest에 기록한 소유 경로에 보존하고, 저장 행·wire·실행기와 재현 pin을 Git에 보존했다. DB가 Git에 포함됐다고 주장하지 않는다.

원래 YAML의 row/hash/version/reviewer/approval은 실제 API에서 여전히 unknown이다. profile의 원문 hash가 있다는 사실로 API가 보존하지 않은 provenance를 채우지 않았다. 수동 정책 조회를 자동 보안 규칙이나 취약점 판정으로 세지 않는다.

## 재생성 문제는 별도 잔여

실제 `cks domain sync`도 실행했다. 결과는 기존 정책을 byte 단위로 그대로 재생성하며 수정 정책은 생성하지 못한다. 현재 생성기의 package 이름 추정은 디렉터리 basename을 사용한다. compiler 디렉터리의 package compile, engine 디렉터리의 package wbftengine 같은 경우를 놓친다. 명시 receiver 누락과 이 source pin에서 파일이 다른 ErrInvalidFeePayer anchor도 별도 확인이 필요하다.

따라서 수정 YAML을 원래 generated 정책 위에 덮어써 다음 setup에서 freshness 오류가 나게 만들지 않았다. 새 profile은 `domain_sync_managed=false`인 명시적인 로컬 평가 입력이다. 기존 setup.yaml의 기본 정책 선택은 유지한다. 후속 생성기 보완에서는 source pin·anchor file·receiver를 실제 선언과 대조하고, 잘못된/모호한 anchor는 경고/거부하며 재생성 결과가 올바른 정책과 일치하는지 검증해야 한다. 명시 qname만 적으면 항상 해결된다고 가정하지 않는다. 현재 함수는 다른 package prefix를 다시 붙일 수 있다.

## 검증 이력과 완료 경계

최초 실행은 toolchain launcher와 GOSUMDB 설정 충돌로 graph 시작 전에 실패했다. 캐시된 Go1.26.9 binary를 명시한 뒤 진행했다. 첫 graph/basic 조회 성공은 최종 강화 검사로 세지 않는다. 그 다음 실행에서는 검증기가 파일 hash를 symbol blob hash로 오해해 실패했고, 실제 원문 구간 hash로 수정한 최종 실행에서 통과했다. 모든 실패 원자료와 CKS 정상 종료를 남겼다.

증거 guard 시험은 처음에 수정7개 밖의 저장 위치 변조를 놓쳤다. guard를146개 전체의 source-bound 위치 대조로 강화했고 최종8개 시험이 통과했다. 재해시한 효과/독립 수용 승격·source 위치·누락 edge·YAML 승인·재생성 해결 주장·canonical wire 변조를 거부한다. 이 시험은 실제 ontology 효과 평가를 대체하지 않는다.

- 완료: 고정 source용 수정 policy profile과 실제 graph/CKS 로컬 적용 검증.
- 미완료: 원래 domain sync의 일반적인 source-aware 재생성 수정, 정식 독립 의미 수용, ontology 효과 측정.
- 다음: 아래 OE-01 자료를 사용해 보관 원문의 ontology 적합성과 별도 semantic DB 후보를 준비한다. 기존 DB에 의미 데이터를 사후 삽입하거나 current를 바꾸지 않는다.

## 초기 OE-01 개발 입력 준비 기록

[질문12개](./operational-preparation/ontology-effect-dev/requests.json)는 WBFT4개 family 제안의 검색 질문8개와 unknown/다른 프로젝트/성능/버전 제어 질문4개다. [비공개 검토 초안](./operational-preparation/ontology-effect-dev/private-review.json)은 정답을 아직 작성하지 않았고 기존 WBFT pilot과 의미 중복을 미검토로 기록했다. 새 독립 FINAL이나12개 독립 표본이 아니다. [실행 초안](./operational-preparation/ontology-effect-dev/execution-draft.json)은 동일 corpus에서 off/concept_text/relations/combined를 비교하도록 한다. 반복·채점 근거·기준을 고정하기 전에는 효과를 채점하지 않는다.

기존 retained corpus dataset474440d4의 project/source pin은 승인된 ontology4개 정의와 일치한다. 그러나 이 버전에는 semantic DB가 없다. 승인 정의의 retained-source 포함 여부·정합과 코드 관계 상태를 확인한 뒤 소유한 별도 경로에서 semantic 후보를 만들어야 한다. 승인된 정의가 있다는 사실만으로 구현 관계나 품질 개선을 만들어 넣지 않는다. 이 문단은 초기 준비 시점의 기록이다. 이후 로컬384회·별도 진단8회의 관찰은 [OE-01 관찰 기록](./ONTOLOGY-EFFECT-DEV-OBSERVATION.md)에 있고 품질은 inconclusive다. 최신 전체 진행은 [전용 원장](./KNOWLEDGE-ROADMAP-PROGRESS.json)을 따른다.
