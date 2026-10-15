# 산업 개념과 프로젝트별 비교 근거

현재 구현은 KM-05.1의 선택형 공통 어휘다. 프로젝트별 구현 연결·검토/버전 처리·비교 MCP·실제 다중 프로젝트 품질은 다음 단계에서 검증한다. [상위 명세](./KNOWLEDGE-MANAGER-AND-COMPARISON-SPEC.md), [시험표](./KNOWLEDGE-ROADMAP-TEST-MATRIX.md), [원장](./KNOWLEDGE-ROADMAP-PROGRESS.json)을 따른다.

## 목적과 기존 계층

동일 산업의 프로젝트에서 같은 질문에 해당하는 자료를 찾되 프로젝트 신원이나 구현을 합치지 않는다. 공통 개념은 자료를 나누어 검토하는 어휘다. 어떤 체인이 기능을 지원한다거나 정책이 승인됐다는 근거가 아니다. 이 문서의 정의는 제품의 비교용 정의이며 특정 체인의 사실·성능·보안성 평가가 아니다.

기존 [지식 팩 계약](./DOMAIN-PACK-CONTRACT-V1.md)의 Pack/ConceptType과 정확 버전·파일 tree digest를 그대로 사용한다. [산업 팩](../../system/packs/industry-blockchain/pack.yaml)은 선택형 industry.blockchain 1.0.0이며 공통 core20 타입·기존 ontology 파일·운영 dataset을 수정하지 않는다. [질문 sidecar](../../system/packs/industry-blockchain/comparison.json)는 같은 팩 디렉터리에 있어 기존 pack digest/lock에 포함된다. 기존 knowledgepack loader가 pack.yaml을 읽고 resolver가 의존/타입/상속 참조를 검사한 뒤 새 comparison 계층이 comparison.json을 검증한다. 현재 단일 선택 팩은 의존 팩을 자동 탐색하지 않으며 누락 의존은 거부한다. 기존 팩 소비자는 sidecar의 비교 의미를 자동 활성화하지 않는다.

concept key는 `{pack_id, version, digest, concept_id}`다. 같은 이름/버전에서 한 byte라도 바뀌면 digest가 달라진다. alias, 정의, 질문 변경도 새 pin이 필요하다. core나 코드 symbol ID를 개념 ID로 다시 정의하지 않는다. 단일 공통 개념을 선택해도 project_id/dataset_id/snapshot_id/origin/path가 다른 근거는 다른 참조로 유지한다.

## 비교 질문에서 선택한 첫 어휘

| 개념 | 구분하는 범위 | 비교 축 |
|---|---|---|
| transaction | 거래 형식·검증·실행; 합의/정책 의도는 별도 | validation, execution, fees |
| consensus | 참여자의 원장 합의; L2 ordering만으로 합의 동일시 금지 | participants, quorum, fault-assumptions |
| finality | 최종 판정과 명시 rollback/settlement 가정 | condition, rollback, settlement |
| transaction-ordering | 제안 거래의 선택·순서; 합의/최종성 보장과 별도 | selection, ordering, fairness |
| mempool-admission | pending 처리·교체·퇴출·상한 | admission, ordering, replacement, eviction, concurrency |
| staking | 지원하는 경우에 한해 stake/delegation·출금·벌칙 | entry, exit, penalty |
| validator-lifecycle | 참여 자격·active set·순환·제거; stake 필수 가정 금지 | admission, rotation, removal |
| staking-policy | 명시 문서/설정의 규칙; 코드에서 승인 의도 생성 금지 | withdrawal-conditions, limits, exceptions |

8개 개념·26개 축 모두 포함/제외, 적용 가능한 역할의 어휘, 비교 질문과 필요한 조건을 가진다. 각 축은 project version, network/configuration, component role, 관찰 시점을 요구한다. roles는 가능성 범위이며 해당 역할/기능을 프로젝트가 지원한다는 판정이 아니다. non-blockchain 프로젝트나 staking 없는 네트워크에 강제로 적용하지 않는다. 성능/우월성 값은 정적 소스만으로 생성하지 않는다.

## 용어 해석과 모호성

LoadScheme은 명시한 로컬 팩만 읽는다. 네트워크 검색·다운로드·자동 설치·기본 선택은 하지 않는다. Resolve는 공백/대소문자 정규화 후 ID 또는 명시 별칭의 정확 일치만 찾는다. 한영 별칭이 있어도 임의 자연어 질문을 해석하는 LLM이라고 주장하지 않는다. 알려진 용어 하나면 exact_term, 여러 개면 ambiguous, 없으면 unknown이다. exact_term은 단어 사전의 일치이고 프로젝트 구현의 exact 매핑 판정과 다르다.

‘검증’은 consensus와 mempool-admission의 두 후보를 의도적으로 유지한다. 질문자/상위 에이전트가 비교하려는 의미와 축을 선택해야 한다. ‘staking algorithm speed’처럼 등록하지 않은 복합 문장은 unknown이며 일부 문자열만으로 임의의 개념을 고르지 않는다. 반환 slice를 바꿔도 선택된 팩의 다음 조회를 오염시키지 않는다.

## 후속 연결·비교 구현의 경계

KM-05.2는 정확 concept key와 프로젝트/버전/관찰 시점/network/config/component 조건, 로컬 기능 설명과 원문 근거를 가진 별도 binding을 추가한다. 코드·문서·설정/정책·테스트를 구분하며 원문 hash/span을 실제 retained source에서 검증한다. 공통 개념이 같아도 symbol이나 policy entity를 합치지 않는다. 같은 이름의 다른 의미와 다른 이름의 같은 역할을 모두 사례로 검증한다.

KM-05.3에서 exact/close/broader/narrower/related는 방향을 명시한 직접 비교 관계다. close와 related는 자동 전이하지 않고 exact도 전역 entity 동일성으로 사용하지 않는다. proposed/검토 기록·현재 source와 불일치한 stale·회수·팩 버전 변경을 구분하며 실제 사람 승인자를 만들지 않는다. 검토 없는 관찰 연결은 proposed 상태로 자료를 제공하고 verified로 자동 승격하지 않는다.

KM-05.4는 supported/unknown/not_collected/unsupported/not_applicable/incomparable 등의 근거와 범위를 구분한다. 근거를 찾지 못한 결과를 기능이 없다는 선언으로 바꾸지 않는다. 명시적 비적용에도 출처가 필요하다. L1 consensus/L2 sequencing, network/config 차이, 비블록체인과 미지원 파서를 검증한다.

KM-06은 명시적인 비교 project/version 집합과 concept/facets/조건/예산을 받고 각 프로젝트의 원문·인용·mapping 이유·미확인/상충을 분리한 자료를 제공한다. 최종 장단점 해석은 LLM/사용자에게 남긴다. 기존 collection의 구현 대상 제한은 유지한다. 이번 KM-05.1은 해당 runtime을 완료한 것으로 세지 않는다.

## KM-05.1 검증 기준

8개 정의·26개 질문 축, 한영/충돌 별칭, unknown·명시 선택, source byte 변경 identity, 호출자 변경 격리, pack/version/개념/축 누락·중복·범위·unknown key·중복 JSON key·과도한 깊이/크기·symlink 거부를 검증한다. 기존 pack loader/resolver·source no-follow와 tree digest를 사용하며 누락 의존·없는 core 확장·상속 순환도 거부한다. 실제 프로젝트 근거 회수와 ontology 효과는 KM-05.2–4, KM-06, OE-01에 남긴다.



## KM-05.1 완료 기록

KM-05.1 완료: 기존 knowledgepack loader를 재사용한 선택형 industry.blockchain1.0.0 팩에8개념·26비교 축을 추가했다. 한영 별칭·포함/제외·역할/버전/network/config/시점 조건, 정확 byte pin과 unknown/ambiguous 계약을29개 Go 사례로 검증했다. 단어의 exact_term 일치를 구현 매핑 승인이나 project/entity 동일성으로 해석하지 않는다. 공통 core20·운영 dataset·기본 자동 선택은 변경하지 않았다. 프로젝트 연결·비교 MCP·ontology 품질은 남는다.

[동결 원자료](../../system/eval/knowledge-roadmap-2026-10-16/km-05-scheme/manifest.json)를 따른다. 전체17/28·기능4/7이다. 다음은 KM-05.2의 프로젝트별 코드·문서·설정/정책·테스트 근거 연결이다. 남은11개는 KM-05.2–4, KM-06.1–4, OE-01.1–4이며 정책 생성기 보완·운영/독립 수용9개·선택 후속12개는 별도 유지한다. 사용자가 멈추기 전까지 단계별 서명 커밋으로 계속한다.

최종 소스의 전체 Go3891실행(3873통과18skip,136패키지)과 race865실행(863통과2skip,7패키지)을 별도 순차 실행했다. 초기 lint1건의 표현을 수정하고 기존 resolver의 의존/타입/상속 검사도 연결한 뒤 전체 시험을 재실행했다. 초기 실패 로그를 보존하며 skip을 통과로 합산하지 않는다.
