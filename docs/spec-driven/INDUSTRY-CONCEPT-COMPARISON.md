# 산업 개념과 프로젝트별 비교 근거

현재 KM-05.1의 선택형 공통 어휘와 KM-05.2의 프로젝트별 원문 연결을 구현했다. 직접 매핑의 검토/회수·버전/stale 정책도 구현했다. 지원/적용 범위·비교 MCP·실제 다중 프로젝트 품질은 다음 단계에서 검증한다. [상위 명세](./KNOWLEDGE-MANAGER-AND-COMPARISON-SPEC.md), [시험표](./KNOWLEDGE-ROADMAP-TEST-MATRIX.md), [원장](./KNOWLEDGE-ROADMAP-PROGRESS.json)을 따른다.

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

## KM-05.2 프로젝트 연결 계약과 실행 계획

공통 어휘가 존재하는 것과 각 프로젝트의 역할이 그 개념에 해당한다는 판정은 다르다. ProjectBinding은 정확 concept key, 전체 V2Coordinates, network/configuration/component_role/observed_at, 로컬 역할 이름, 직접 mapping 종류와 이유, 근거 목록을 받는다. 다른 이름의 같은 역할과 같은 이름의 다른 역할을 모두 표현한다. 문자열 label로 자동 매핑하거나 프로젝트를 합치지 않는다.

binding ID는 `{coordinates, conditions, concept key, local binding id}` 범위 안에서만 유일하다. 같은 ID가 다른 프로젝트·버전·network/config/역할에 존재해도 충돌하지 않는다. 동일한 관찰 시각의 timezone 표현만 다른 중복은 거부한다. BindingDigest는 해석·조건·모든 원문 좌표/hash를 포함한다. 의미 승인이나 서명은 아니다. 원문·조건·매핑이 바뀌면 새 digest이며 검토 승계를 주장하지 않는다.

BindingDocument는 schema version1, 최대128개 연결, 최대1MiB의 엄격 JSON이다. 개별 연결은 기존 v2 예산과 같은 최대12개 원문 근거를 받는다. 각 근거는 고유 ID, code/document/configuration/policy/test 종류, 해당 공통 개념의 facet 집합과 전체 CitationV2다. span은 최대256줄이며 기본 source/body byte 예산도 지킨다. 없는 축·공통 팩 버전/digest 불일치·타 프로젝트/버전·범위 밖 origin·경로 탈출·누락 조건·중복을 거부한다. 검토 상태 필드를 입력에 끼워 넣어 verified로 만드는 경로는 없다.

MappingReader는 기존 collection session의 명시 catalog/member/target 검증과 reader lease를 재사용한다. evidencev2.BuildFromRefs가 지정된 retained archive의 실제 줄·파일/hash를 읽고 정화한 뒤 collection이 결과와 원문을 다시 대조한다. 라이브 checkout이나 current를 대신 선택하지 않는다. 누락·hash 불일치·정화 drop·예산으로 근거가 누락되면 완전한 연결 응답을 만들지 않는다. 공개 응답의 역할 이름·이유·조건 문자열도 정화한다. 요청 slice와 반환 slice의 소유권을 분리한다.

커밋된 원문의 citation.commit_hash는 base_commit과 같고, working-tree와 snapshot-only의 citation.commit_hash는 빈 값이다. working-tree는 base_commit을 별도 보존한다. 기존 보관 원천 계약과 다른 커밋 신원을 만들어 넣지 않는다.

응답은 review_state=proposed, verification=archived_source_hash_and_span_only를 명시한다. 근거 종류·관찰 조건·매핑 이유는 제공된 해석이며 파일 hash 일치가 이 해석의 의미적 옳음을 승인하지 않는다. policy로 분류한 문서도 승인된 규범으로 승격하지 않고, test source도 실제 시험 실행 통과로 표시하지 않는다. 실제 사람 검토와 변경/stale/회수는 KM-05.3에서 별도로 처리한다.

검증은 실제 retained source archive와 fixture identity manifest를 이용한다. graph/vector 인덱싱이나 공개 MCP의 완료 근거로 확대하지 않는다. 서로 다른 역할의3개 프로젝트,5개 근거 종류, committed/미커밋 working-tree/snapshot-only, 해시·범위·정화·라이브 변경/보관 손상 경계를 확인한다. 실제 다중 프로젝트 DB/공개 도구는 KM-06.4에 남긴다.


## KM-05.2 완료 기록

KM-05.2 완료: 명시 common concept/version/digest와 프로젝트·버전·network/config/역할/관찰 시점에 결속한 원문 연결을 구현했다. 서로 다른 이름의 같은 역할과 같은 이름의 다른 역할을3개 프로젝트의5종류 원문으로 검증하고, 동일 로컬ID를 서로 다른 scope에 보존했다. 기존 collection reader lease·target 검증과 retained evidence 정화/재검증을 재사용한다. code/document/configuration/policy/test와 원문 hash/span을 구분하며 모든 연결은 proposed다. 실제 사람 승인·그래프/벡터 검색 품질·공개 비교 MCP는 완료로 세지 않는다.

비교 패키지59개(어휘29+연결30)와 최종 전체 Go3921(3903통과18skip)·race895(893통과2skip)를 검증했다. 초기 손상 fixture의 읽기 전용 권한 문제와 working-tree commit/base 검증 오류의 실패 원자료를 보존했다. 전체 소스 재검증 이후 완료로 표시한다. [동결 증거](../../system/eval/knowledge-roadmap-2026-10-16/km-05-bindings/manifest.json)를 따른다. 전체18/28·기능4/7이다. 다음은 KM-05.3의 직접 매핑 의미·검토/회수·버전/stale 정책이다. 남은10개는 KM-05.3–4, KM-06.1–4, OE-01.1–4이며 정책 생성기 보완·운영/독립 수용9개·선택 후속12개는 별도 유지한다. 사용자가 멈추기 전까지 단계별 서명 커밋으로 계속한다.

## KM-05.3 직접 매핑 의미와 검토/버전 계약

매핑은 **프로젝트의 로컬 역할 → 공통 개념** 방향의 직접 연결이다. exact는 명시 조건에서 정의 범위가 맞는다는 주장이고, close는 상당한 중첩과 남는 차이, broader는 로컬 역할이 공통 개념보다 넓은 범위, narrower는 더 좁은 범위, related는 포함/동일성을 주장하지 않는 관련성이다. 다섯 종류 모두 entity identity=false, automatic transitivity=false다. 서로 다른 프로젝트가 같은 개념에 연결됐다는 이유로 코드·정책·인용 ID를 합치거나 A→B→C의 새 연결을 만들지 않는다. core20·기존 온톨로지 스키마/기본 순위는 변경하지 않는다.

MappingReviewDocument는 schema1·명시 min_approvals1–8·최대512개 기록의1MiB 엄격 JSON이다. 기록에는 이벤트ID, local binding ID, 전체 concept/version/digest·프로젝트/버전·조건, 정확 BindingDigest, verified/rejected/withdrawn 판정, 제공된 reviewer_id·이유·시각이 필요하다. 검토 파일을 받지 않은 경우는 proposed이며 이름·응답·승인을 생성하지 않는다. ASCII lower_snake_case 속성 이름만 받으며 값의 UTF-8은 보존한다. 대소문자/유니코드 별칭이 같은 struct 필드를 덮는 입력도 거부한다.

판정은 명시 as_of 시각을 사용한다. 관찰 전 검토·잘못된 시각·미래 승인을 현재 승인으로 쓰지 않는다. 현재 target/BindingDigest와 전체 header가 일치하는 기록만 적용한다. 시각이 가장 늦은 각 검토자 ID의 판정을 사용하고 같은 ID의 중복 기록으로 정족수를 채우지 않는다. 시각이 같은 상반 판정이나 검토자 간 승인/거부 충돌은 disputed/conflict로 남긴다. 같은 scoped 이벤트 ID의 바이트를 고쳐 덮는 문서는 거부하고 새 이벤트 ID를 요구한다. 배열 순서를 승인 우선순위로 사용하지 않는다.

withdrawn은 그 검토자 ID의 앞선 투표만 회수한다. 다른 검토자의 유효 투표를 지우지 않으며 정족수 미달은 proposed다. rejected·withdrawn·disputed는 유효 승인으로 계산하지 않는다. 검토자 ID는 **제공된 기록 label**이며, 서로 다른 사람임을 인증하거나 독립 품질 수용을 제공하지 않는다. 응답에 reviewer_identity_authenticated=false와 independent_quality_acceptance=false를 유지한다. 조직 인증/서명이 필요한 수용은 별도 어댑터가 필요하다.

검토의 state와 validity를 나눈다. 정확 현재 기록은 current다. 같은 프로젝트/network/config/역할의 논리적 연결에서 원문·스냅샷·해석/이유·관찰 시점·팩 버전/digest가 달라졌으면 현재 승인으로 승계하지 않고 stale/proposed로 보고한다. 타 프로젝트·network/config/역할의 기록은 현재 target의 승인이 아니다. 과거의 immutable 버전을 명시 선택하면 당시 원문·검토를 계속 확인할 수 있다. 새 팩은 정확 키를 다시 선택하고 검토해야 하며 자동 migration이나 관계 closure를 수행하지 않는다.

공개 경로에 사용할 ReadReviewedBinding은 retained 원문 무결성을 다시 확인한 결과와 기록 판정을 함께 반환한다. 보관 blob이 손상되면 과거 verified 기록이 있어도 원문/확정 연결을 내보내지 않는다. 단독 AssessMappingReview는 기록 판정의 계산이며 실제 source 조회 완료를 뜻하지 않는다. 매핑 승인으로 원문 정책 내용의 승인·test 실행 통과·프로젝트 우월성을 판정하지 않는다.

이번 검증의 검토자는 controlled fixture label이며 실제 사용자/담당자의 승인 기록이 아니다. 실제 프로젝트의 기존 proposed 연결을 자동 승격하지 않는다. 공개 비교 MCP와 비적용/미수집/지원 차이는 다음 단계에서 검증한다.


## KM-05.3 완료 기록

KM-05.3 완료: exact/close/broader/narrower/related를 local→shared 직접 관계로 고정하고 entity 동일시/자동 전이를 금지했다. 정확 source/해석/팩 pin의 검토 기록, 중복 검토자 정족수·최신 판정·회수·충돌·미래 시각·stale와 과거 버전 보존을 구현했다. 공개용 reader는 보관 원문을 재확인하며 승인 기록으로 손상을 우회하지 않는다. 검토자 label은 인증된 신원이 아니고 실제 사람 승인은 생성하지 않았다. 입력 필드의 대소문자/유니코드 별칭 덮기도 재현·거부했다.

비교 패키지93개(어휘29+연결30+검토34), 전체 Go3955(3937통과18skip)·race929(927통과2skip)·빌드/fmt/vet/lint·문서133개를 검증했다. 초기 필드 alias 덮기 실패 원자료를 보존하며 source payload의 UTF-8을 제한하지 않는다. [동결 증거](../../system/eval/knowledge-roadmap-2026-10-16/km-05-mapping-review/manifest.json)를 따른다. 전체19/28·기능4/7이다. 다음은 KM-05.4의 비적용/미수집/미지원·역할/산업/network/config 차이 검증이다. 남은9개는 KM-05.4, KM-06.1–4, OE-01.1–4이며 정책 생성기 보완·운영/독립 수용9개·선택 후속12개는 별도 유지한다. 사용자가 멈추기 전까지 단계별 서명 커밋으로 계속한다.
