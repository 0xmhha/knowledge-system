# 지식 구축·갱신 매니저와 프로젝트 비교 명세

2026-10-15 사용자 요청으로 채택한 기능 범위다. 온톨로지 효과 평가는 우선 필수 검증으로 이동한다. Claude 매니저 플러그인·지속 갱신·프로젝트 격리·산업별 비교는 구현할 작업으로 등록한다. 이 문서는 요구·설계 초안이며 기능 구현/설치/검증 완료를 뜻하지 않는다. 상태는 [전용 원장](./KNOWLEDGE-ROADMAP-PROGRESS.json), 전체 집계는 [작업표](../REMAINING-WORK.md)를 따른다.

## 1. 목적과 요청 대응

사용자가 문서를 먼저 작성해야만 유용해지는 구조를 줄인다. 매니저 에이전트가 등록된 프로젝트의 코드·기존 문서·변경·명시적으로 제공한 개발 경험을 수집하여 지식 입력 초안을 만들고, knowledge-system 도구로 DB를 구축·검증·갱신한다. 코드로 확인한 동작, 명시된 정책, 추정한 의도와 검토된 지식을 구분한다. 원문에 없는 설계 이유는 필요한 질문으로 보완하고 모르면 unknown으로 남긴다.

| 요청 | 등록 ID | 기대 결과 |
|---|---|---|
| 온톨로지 효과 평가 추가 | OE-01 | 동일 자료에서 off/on의 회수 품질·근거·오인·지연 차이를 실제 측정 |
| 1. 도구를 이용한 DB 구축 플러그인 | KM-02, KM-03 | 사용자가 수동 문서를 작성하지 않은 프로젝트에서도 최초 DB와 근거 있는 지식 초안 생성 |
| 2. 플러그인의 규칙적인 갱신 | KM-04 | 코드/문서/지식 변경을 반영하고 실패 시 이전 유효 DB 유지 |
| 3. 지식 시스템을 구축하는 매니저 에이전트 | KM-03, KM-04 | 초기화부터 수집·검토 대기·빌드·활성화·상태 확인·복구까지 관리 |
| 4. 임의 프로젝트 묶음의 격리 스키마 | KM-01 | 동명/동일 경로/포크/여러 버전에서도 프로젝트를 구분하고 구현 대상 오인 방지 |
| 5. 같은 산업 프로젝트의 비교 자료 | KM-05, KM-06 | 공통 개념별로 프로젝트별 코드·정책·근거·차이를 구조화해 LLM에 제공 |

요청1–3은 기존 WI-5의 구체화·확장이다. WI-5와 KM-03/04를 서로 다른 완료 기능으로 중복 집계하지 않는다. KM-02는 안전한 버전 DB 쓰기를 도구로 제공하기 위한 의존 작업이다. 구현 저장소·패키징 위치는 설계에서 정하며 미지정이라는 이유로 knowledge-system 측 조사/설계를 중단하지 않는다.

## 2. 현재 코드와 재사용 범위

조사 기준은 knowledge-system commit c4af3f22다. 소스 대조이며 새 runtime 검증은 하지 않았다.

| 기반 | 현재 코드/계약 | 새로 필요한 부분 |
|---|---|---|
| 프로젝트와 인용 좌표 | [V2Coordinates/CitationKeyV2](../../pkg/system/contract/v2.go), [semantic Snapshot](../../internal/system/semantic/model.go) | 여러 프로젝트를 묶는 catalog, 모든 join/cache/요청의 scope 확인, 포크/동명 격리 검증 |
| 버전 DB 구축·전환 | [coordinated setup](../../system/docs/SETUP.md), [internal/setup](../../internal/setup) | 매니저가 호출할 도구 계약과 지속 갱신 orchestration |
| MCP 운영 도구 | [setup](../../internal/system/mcp/setup_tool.go), [reindex](../../internal/system/mcp/reindex_tool.go) | 현재 ops.setup/reindex는 legacy 대상이며 pinned v2 유지보수는 거부한다. 이를 우회하지 않고 version-aware 도구를 설계·추가해야 함 |
| 지식 초안·검토·출처 | [팩 계약](./DOMAIN-PACK-CONTRACT-V1.md), [knowledgepack](../../internal/system/knowledgepack) | 에이전트의 자동 초안 수집, 변경 탐지, 검토 요청/상태 관리와 provenance |
| 온톨로지 | [의미 계약](./SEMANTIC-CONTRACT.md), [조회 MCP](../../system/docs/ontology-mcp.md), [선택 팩](./DOMAIN-PACK-CONTRACT-V1.md) | 공통 산업 개념과 프로젝트별 binding, 명시적 매핑, 프로젝트별 비교 자료 조회 |

현재 소스에는 일부 격리·버전 관리와 산업 팩 기반이 이미 있다. 이를 새 미구현으로 되돌리지 않는다. 반대로 기존 프로젝트별 인용이나 semantic 조회가 있다는 사실만으로 다중 프로젝트 비교와 매니저가 완성됐다고 표시하지 않는다. graph의 짧은 qualified_name이나 canonical import path도 서로 다른 포크 사이의 전역 신원으로 단독 사용하지 않는다. [정책 연결 진단](./POLICY-REFERENCE-DIAGNOSIS.md)의 경고 없는 오연결을 격리 반례로 참고한다.

## 3. 설계 원칙과 제안 스키마

물리적으로 하나의 DB에 모든 프로젝트를 합치는 방식과, 프로젝트별 immutable DB를 catalog로 묶는 방식을 비교한다. 첫 설계는 기존 보관/전환 계약을 재사용하는 논리적 collection을 우선 검토한다. 물리 배치와 무관하게 사용자는 선택한 프로젝트 묶음을 한 지식 기반으로 조회할 수 있어야 한다. 아래 논리 엔티티는 KM-01 catalog와 선택적 공개 조회 계약에 구현했다. 기존 DB migration은 추가하지 않았다.

| 엔티티 | 필수 식별·의미 |
|---|---|
| Project | 안정 project_id, 원천 등록 정보, 표시 이름, fork/upstream 관계. 디렉터리명·repo URL만으로 동일 프로젝트를 자동 판정하지 않음 |
| ProjectVersion | project_id + dataset_id + snapshot_id, commit/source mode, 등록된 source origin 목록과 선택한 component/network/config 범위 |
| Source/SymbolRef | 위 좌표 + origin_id + 원문 path/hash/span + local symbol/canonical ID. 같은 import path의 포크도 다른 참조 |
| Collection | 명시적으로 선택한 ProjectVersion 집합과 catalog 버전. 서로 다른 프로젝트의 current를 실행 중 무작위로 재선택하지 않음 |
| ConceptDefinition | concept scheme/pack ID + version + concept ID, 정의·포함/제외·별칭·적용 범위 |
| ProjectConceptBinding | 프로젝트의 로컬 개념·기능·코드와 공통 개념의 연결, source 근거·매핑 종류·검토 상태 |
| Claim/Policy/Decision | 프로젝트/버전·유효 기간·적용 범위·원문·관찰/명시/추정 구분·검토 기록. 코드 동작에서 정책 의도를 자동 확정하지 않음 |
| ComparisonFacet | 공통 비교 축 + 프로젝트별 값/단위/조건 + 근거 + unknown/not_applicable/unsupported/conflict 등 상태 |
| MaintenanceJob | 대상 ProjectVersion, idempotency/변경 기준, 단계/시도/오류/자원/후속 실행 상태, 이전/후보/활성 버전 |

질의에는 구현 대상 프로젝트와 선택 버전이 결속돼야 한다. 구현 문맥은 해당 프로젝트로 제한한다. 다른 프로젝트 코드는 명시 비교/참고 모드에서만 별도 출처 그룹으로 반환하며 패치 대상처럼 합치지 않는다. 검색·join·캐시·중복 제거·의미 매핑·history·export·DB 쓰기와 결과 인용 모두 같은 scope를 유지해야 한다.

지원 언어/파서가 다른 임의 프로젝트도 수집 가능 범위와 미지원 범위를 보고한다. 텍스트 근거만 수집한 프로젝트를 완전한 코드 그래프라고 표시하지 않는다. 공통 타입과 개별 프로젝트 인스턴스를 구분하며 산업 팩은 선택·버전 고정한다. 일반 프로젝트에 블록체인 개념을 강제로 적용하지 않는다.

## 4. 기능별 세부 작업과 완료 기준

### KM-01 — 프로젝트 격리와 collection 스키마 · 우선순위 P0

- [x] KM-01.1: 기존 graph/vector/semantic/지식 팩·DB 쓰기·조회·cache/dedup·인용의 scope와 key를 조사하고, 재사용/결손 표와 migration 설계를 기록한다.
- [x] KM-01.2: Project/ProjectVersion/Collection과 scope 결속 계약을 구현한다. 동일 이름·경로·symbol·import path·commit인 별도 프로젝트/포크를 자동 합치지 않는다.
- [x] KM-01.3: 구현 질의와 비교 질의를 분리한다. 누락/불일치 target, 허용되지 않은 cross-project endpoint·다른 버전 join은 명시 거부/unknown으로 처리한다.
- [x] KM-01.4: 동명 두 프로젝트·동일 코드 포크·한 프로젝트 두 버전·중복 등록·재시작·cache warm 상태의 반례로 조회/쓰기/인용 혼입0을 검증한다. 기존 단일 프로젝트 read/rollback 호환 증거도 남긴다.

완료 결과: 프로젝트를 추가하거나 묶어도 출처가 보존되고, A 프로젝트 구현 요청이 B의 코드/정책을 A의 사실로 반환하지 않는다. 개념이 같다는 이유로 프로젝트 신원을 합치지 않는다.

### KM-02 — 버전 DB 구축·갱신 도구 계약 · P0, KM-01 의존

- [x] KM-02.1: 현행 legacy MCP와 versioned CLI의 실제 capability 차이를 고정하고, project/version/config·source 범위를 결속하는 새 요청/오류/상태 계약을 설계한다. 새 도구명은 구현 시 schema와 함께 확정한다.
- [x] KM-02.2: 소유한 대상의 비동기 build/status/cancel을 제공하고 source→후보→graph/vector/knowledge/semantic 정합 검사→활성화 경로를 연결한다. 검토 대기 후보와 활성 DB를 구분한다.
- [x] KM-02.3: idempotency·writer lock·재시도/중복 요청·timeout·부분 실패·rollback/read lease를 기존 전환 계약에 맞춰 구현한다. pinned 쓰기 거부를 legacy 도구나 직접 DB 수정으로 우회하지 않는다.
- [x] KM-02.4: 잘못된 프로젝트/버전/경로·graph/vector 한쪽 실패·의미 입력 불일치·취소·종료에서 기존 current와 조회가 보존되는지 공개 도구로 검증한다.

완료 결과: 플러그인이 도구 응답과 실제 DB 상태로 구축/갱신 성공 여부를 판단할 수 있다. 단순 job 시작 응답을 DB 생성 완료로 세지 않는다.

### KM-03 — Claude 지식 매니저 플러그인 · P0, KM-02 의존

- [x] KM-03.1: 프로젝트 등록·설치/진단·수집 범위와 capability 확인·초기 구축·상태 조회·복구를 수행하는 플러그인/매니저 실행 절차와 패키징 위치를 정한다.
- [x] KM-03.2: 기존 코드/문서/설정/변경과 사용자가 제공한 결정·트러블슈팅·명령/결과를 근거로 지식 입력 초안을 생성한다. 원문/hash/시점/도구/생성 주체·불확실성을 남기고, 문서가 없는 최소 프로젝트에서도 시작한다.
- [x] KM-03.3: 암묵지를 얻기 위한 필요한 질문, 사실/가설/설계 이유 분리, 중복·상충·비공개/제외 범위·검토 대기/채택/회수 경로를 연결한다. 에이전트가 reviewer 이름·사람 승인·모르는 이유를 만들어 넣지 않는다.
- [x] KM-03.4: 실제 Claude host에서 등록→수집→초안→검토 상태→도구 build→DB 조회를 검증한다. 문서 없음·누락 의존·근거 부족·중단/재실행·타 프로젝트 자료 입력의 실패도 보존한다.

완료 결과: 사용자가 구조화 문서를 수작업으로 먼저 만들지 않아도 코드 지식과 검토 가능한 경험 지식이 생성된다. 관찰할 수 없는 의도를 완전 자동 복원한다고 보장하지 않는다. 개인 세션/개인 로그를 기본 입력으로 읽지 않고 명시적으로 수집된 자료를 사용한다. 실제 모델 호출은 사용할 모델/인증/비용 범위를 해당 실행 직전에 정한다.

### KM-04 — 매니저의 지속·주기 갱신 · P0, KM-03 의존

- [x] KM-04.1: 수동 갱신, 구성 가능한 정기 실행, 코드/문서 변경 신호의 trigger와 마지막 성공 기준을 설계한다. source commit뿐 아니라 문서·지식 팩·ontology·config/model 변경도 추적한다.
- [x] KM-04.2: 무변경 no-op·변경/삭제/이름 변경·의미 연결 stale 판정·재검토 필요 범위를 구현한다. 근거가 바뀐 verified 지식의 검토를 무조건 승계하지 않는다. 증분 처리가 불가능한 경로는 새 버전 전체 rebuild로 처리한다.
- [x] KM-04.3: 잡 중복/겹침·lease·재시작·누락 실행의 다음 실행 처리·중단·retry/backoff·자원 상한·관측 상태를 구현한다. 활성 데이터는 설정된 gate를 통과한 후보로만 전환한다.
- [x] KM-04.4: 최소 두 번의 실제 내용 변경과 no-op, 실패 후 복구를 연속 실행해 최신 조회·이전 snapshot 재현·원천 삭제/회수 반영과 다른 프로젝트 불변을 확인한다. 실제 주기 trigger 실행 증거를 수동 호출 증거와 분리한다.

완료 결과: 최초 구축 후 지식 DB가 오래된 상태로 방치되지 않고, 갱신 실패가 기존 사용 가능 DB를 깨뜨리지 않는다. 실행 주기·자동 활성화/검토 정책은 설치 설정에 명시한다. 이번 문서 등록은 개인 머신에 scheduler를 설치하거나 유료 모델의 정기 호출을 시작하는 작업이 아니다.

지속 갱신을 매번 사람의 수동 문서 작성에 의존하게 만들지 않는다. 최초 설정에서 정한 프로젝트 범위와 gate 안에서는 코드·문서의 원문 수집과 기계 검증된 데이터셋 전환을 자동 수행할 수 있도록 설계한다. 새로 추정한 정책·설계 이유의 내용 검토는 별도로 남긴다. 자동 수집/색인 성공을 사람의 의미 승인으로 바꾸지 않는다.

### KM-05 — 공통 산업 개념과 프로젝트별 매핑 · P1, KM-01 의존

- [x] KM-05.1: 버전 관리되는 산업 팩과 비교 질문에서 출발해 개념·정의·별칭·포함/제외·비교 축을 설계한다. 첫 후보는 blockchain의 transaction, consensus/finality, transaction ordering, mempool/admission, staking/validator lifecycle과 정책이다.
- [x] KM-05.2: 공통 개념과 프로젝트별 구현·문서·설정/정책·테스트 근거를 연결한다. 같은 이름이 다른 의미인 경우와 이름이 달라도 같은 역할인 경우를 모두 검토한다.
- [x] KM-05.3: exact/close/broader/narrower/related 등 개념 매핑의 의미·범위·검토/버전 정책을 정한다. 의미 유사성을 entity 동일성으로 바꾸거나 close 관계를 자동 전이하지 않는다. 공통 core 타입은 재정의하지 않는다.
- [x] KM-05.4: 동일/유사/비교 불가·미수집·미지원·명시적 비적용을 구분하고, 일부 프로젝트에 없는 기능·서로 다른 L1/L2 역할·여러 network/config·서로 다른 산업 입력을 검증한다.

완료 결과: `staking` 등의 공통 질문에서 각 프로젝트의 해당 근거로 이동할 수 있다. 모든 프로젝트가 staking을 구현한다거나 L1 합의와 L2 sequencing이 같은 기능이라고 가정하지 않는다. 원천 부재는 기능 부재의 증거가 아니다.

### KM-06 — 비교 근거 추출과 MCP 제공 · P1, KM-01/05 의존

- [x] KM-06.1: 명시 project/version 집합 + 공통 개념/질문 + 비교 축 + 시점/조건/상한을 받는 비교 요청 계약을 설계한다. 근거 수집을 제공하며 장단점의 최종 판단은 LLM/사용자에게 맡긴다.
- [x] KM-06.2: 프로젝트별 개념 정의·실제 동작·정책·관련 코드/문서/시험·인용을 분리한 행렬과 구조화 bundle을 구현한다. 공통점/차이 후보에는 각각의 근거와 매핑 이유를 붙인다.
- [x] KM-06.3: 출처 불일치·상충·일부 프로젝트 누락·시점 차이·지원 차이·자료 부족을 응답에 유지한다. 서로 다른 프로젝트의 정책 차이를 한 프로젝트 내부 모순으로 처리하지 않는다. 코드를 읽은 것만으로 TPS·보안성·우월성을 측정했다고 주장하지 않는다.
- [x] KM-06.4: 둘 이상의 실제 프로젝트에서 staking/정책, 합의/finality, mempool/거래 순서의 질의를 수행한다. 구현 대상 격리·근거 완전성·비교 축 누락·오매핑·부분 실패·예산/접근 범위를 검증하고 LLM이 검토할 수 있는 자료를 보존한다.

완료 결과 예시: “A/B/C의 staking 해제 조건을 비교”하면 프로젝트별 적용 버전·조건·코드 위치·명시 정책·unknown이 반환된다. “합의 알고리즘 비교”는 참여/선출·quorum/finality·실패 가정과 원문 근거를, “mempool 비교”는 admission·정렬·eviction·수수료·동시성과 설정 근거를 제공한다. 도구와 실제 고정 원문의 비교를 구현했다. 위 예시는 보장된 실제 체인 사실표가 아니며 미수집/unknown을 보존한다.

### OE-01 — 온톨로지 효과 평가 · P0 필수, 단일 프로젝트부터 시작

- [x] OE-01.1: 고정 go-stablenet 자료에서 공통 개념·한영/다른 용어·다의어·정책/구현 차이·unknown 질의와 기대 근거를 준비한다. 옛13/42질문은 회귀로 유지하며 새 효과의 독립 근거로 재사용하지 않는다. 비교 전 사용할 범위·분모·기준을 고정한다.
- [x] OE-01.2: 같은 corpus/source/snapshot/model/검색 상한에서 off와 concept_text/relations/combined 등 필요한 arm을 비교한다. 새 corpus 효과와 ontology 효과를 혼동하지 않는다. 기존 지식과 go-stablenet 비교는 corpus별 off/on으로 나누거나 별도 질문으로 보고한다.
- [x] OE-01.3: 관련 근거 recall/precision·순위·질의 coverage·unknown/오연결·설명 가능성과 warm 지연/호출·측정 가능한 비용을 기록한다. 반복 호출을 독립 사실로 세지 않고, 품질 개선 없음·악화·표본 부족도 결과로 보고한다. LLM 생성 답변 효과는 검색 효과와 별도로 실제 모델을 사용할 때만 평가한다.
- [ ] OE-01.4: KM-05/06 이후 다중 프로젝트 비교의 개념 매핑 coverage·잘못된 동일시·프로젝트 오인·근거 정확도·누락을 off/on 또는 명시 baseline과 비교한다. 원자료/한계·사용 범위와 유지/수정/기본값 제안을 남긴다.

완료 결과: “온톨로지를 넣으면 더 좋을 것”이라는 기대를 수치·근거와 실패 사례로 확인한다. 단일 프로젝트 평가 결과와 다중 프로젝트 평가 결과를 따로 기록하며 후자가 미실행이면 OE-01 전체 완료로 체크하지 않는다. 예전 온톨로지 결과의 quality=inconclusive·기본 off를 새 설계 승인만으로 변경하지 않는다.

## 5. 실행 순서와 완료 기록

2026-10-15 진행: [수정 정책의 로컬 적용 검증](./POLICY-OVERLAY-ADOPTION.md)을 완료했다. 원래 생성기는 아직 기존 잘못된 이름을 재생성하므로 이 부분을 해결 완료로 세지 않는다. [OE-01 첫 개발 관찰](./ONTOLOGY-EFFECT-DEV-OBSERVATION.md)은 질문12개/4모드384회와 별도 정확 용어8회를 수행했다. 실패/누락0과 인용3,616개의 원문 일치를 확인했지만 활성288회 no_match·최종 순위 차이0이다. 독립 정답·평가 범위 확대와 자연어 개념 해석/검토된 구현 연결·다중 프로젝트 평가는 남는다. 새7개 완료는0/7이다.

정책 참조7건과 OE-01의 개발 관찰384회·진단8회를 기록했고 독립 품질·확대/다중 프로젝트 평가는 남는다. KM-01 격리4/4와 KM-02 설계/공개 실행/영속 복구/실패 검증4/4를 완료했다. 전체8/28·기능2/7이며 다음은 KM-03.1 매니저 패키지·등록/설치/진단/초기 구축/복구다. 이후 KM-03.2–4 → KM-04.1–4 → KM-05.1–4 → KM-06.1–4 → OE-01.1–4를 진행한다. 고정 임계치·실제 비교 프로젝트/버전·실행 주기는 각 설계와 근거로 결정하며 미결정을 완료로 세지 않는다.

각 하위 작업은 목적·설계·코드 변경·적절한 검증·원자료/한계 링크를 기록한 뒤에만 완료로 전환한다. 문서 작성은 기능 구현 완료가 아니며, feature scope 채택과 입력 정답 승인·사람의 독립 수용은 다르다. 기존 WI9개 구현 원장과 과거 source-bound packet은 변경하지 않는다. 기존9개 운영 검증은 별도 범위이며 새7개와 분모를 합치지 않는다.

로컬 설계·코드 구현·회귀/도구 검증은 정답 검토자나 유료 모델 지정 없이 진행한다. 실제 모델/외부 서비스·노드가 필요한 실행과 사람의 독립 수용에만 해당 조건을 적용한다. 임시 fixture 통과는 실제 host 주기 실행이나 실제 다중 프로젝트 효과를 대체하지 않는다. 원장의 not_started/in_progress/blocked/complete는 해당 작업 범위만 뜻하며, complete에는 구현과 검증 evidence가 모두 필요하다.

## 6. 개념적 근거

온톨로지는 공통 개념·관계와 프로젝트별 표현을 연결해 비교 자료를 조직하는 기반으로 적합하다. [W3C SKOS mapping properties](https://www.w3.org/TR/skos-reference/#mapping)도 개념 체계 사이의 exact/close/상하위/관련 매핑을 구분한다. 이 설계는 그 구분을 참고하며 현행 저장소가 SKOS/OWL reasoner를 구현했다고 주장하지 않는다. [open-world 설명](https://www.w3.org/TR/skos-reference/)은 자료가 빠졌다는 사실만으로 없다고 결론내리지 않는 근거다.

[Ethereum optimistic rollup 설명](https://ethereum.org/en/developers/docs/scaling/optimistic-rollups/)에서 sequencer의 거래 순서 결정과 L1 제출을 구분하듯, 비교 축도 공통 개념 아래에서 역할·가정·적용 범위를 보존한다. 온톨로지 자체가 source extraction·프로젝트별 mapping·비교 API 또는 검증된 장단점 판정을 자동 제공하는 것은 아니다. 이 작업표는 그 연결 기능을 구현 대상으로 명시한다.

전체28개 세부 작업과 이번17개 시험의 상태는 [전체 시험표](./KNOWLEDGE-ROADMAP-TEST-MATRIX.md)에서 함께 확인한다. 전체 작업 완료와 공통 계층 fixture 통과를 구분한다.


KM-02.4의 공개 실패36항목과 마지막 정상 v4 게시/조회까지 검증해 KM-02를 완료했다. 기존 current/원문/인용·타 프로젝트·publication history를 보존하고 graph/vector/semantic 실패·drift·deadline/취소/종료를 구분했다. Go 구현과 E2 입력이 모두 같음을 확인한 뒤 E2 전체 Go/race를 명시적으로 참조했다. 새 실행 증거로 가장하지 않는다. 전체8/28·기능2/7 완료, 다음은 KM-03.1의 매니저 패키지·등록/설치/진단/초기 구축/복구 절차다.

KM-03.1의 패키징·등록/설치/진단/초기 구축·복구 절차와 helper/launcher를 [매니저 계약](./KNOWLEDGE-MANAGER-PLUGIN.md)에 고정했다. 등록14·native3·실제 metadata bootstrap9항목과 증거 보호10개를 확인했다. 초안/검토/reader/주기는 아직 미구현이다. 전체9/28·기능2/7이며 다음은 KM-03.2의 문서 없는 프로젝트 수집·근거 초안이다.


KM-03.2 완료: 문서 없는 두 프로젝트에서 source/config/Git 변경·제공 결정/트러블슈팅/명령 보고를 별도 origin으로 수집하고 정확한 file/span hash를 보존했다. 수집23·등록14·실제 CKS 초안/lock 및 native plugin24항목·증거 보호13개를 검증했다. 전체10/28·기능2/7이며 사람 승인/원래 source 채택/DB 검색은 완료로 세지 않는다. 다음은 KM-03.3 검토·채택·회수다. [수집 증거](../../system/eval/knowledge-roadmap-2026-10-15/km-03-collection/manifest.json)와 [매니저 계약](./KNOWLEDGE-MANAGER-PLUGIN.md)을 따른다.


KM-03.3 완료: 질문·중복/공유 근거 해석 후보, 정확한 subject의 제공 검토·CAS event, 제외 자료를 재수집하지 않는 filtered input 채택·회수를 구현했다. 검토35·수집23·등록14·실제 CLI/native CKS24항목과 증거 보호14개를 검증했다. 통제 fixture 응답은 실제 사람 인증·독립 수용으로 세지 않는다. 원래 source와 DB는 바꾸지 않았다. 전체11/28·기능2/7이며 다음은 KM-03.4 실제 Claude host/managed reader와 구축·조회 연결이다. [review packet](../../system/eval/knowledge-roadmap-2026-10-15/km-03-review/manifest.json)을 따른다.


KM-03.4 A 중간 완료: 검토 문서의 명시 색인·별도 retained origin과 held 후보 실제 원문/인용14항목을 검증했다. 새 전체 Go3,820통과/18skip·race810통과/2skip이며 과거 Go를 재사용하지 않았다. [A packet](../../system/eval/knowledge-roadmap-2026-10-15/km-03-document-index/manifest.json)을 따른다. B의 adoption-bound profile/reader와 C 실제 Claude host는 남아 KM-03.4 체크박스는 미완료, 전체11/28·기능2/7이다.


KM-03.4 B 중간 완료: 실제 scoped plugin18도구를 구현하고29 unit·26 actual proxy 항목/3 EOF0·수집23·등록14·native3·증거14개를 검증했다. 검토 입력과 정확한 후보 reader를 연결하고 drift/손상/회수/재시작 거부·known-job exact resume를 확인했다. Go는 A와 byte 동일 후 참조했다. [B packet](../../system/eval/knowledge-roadmap-2026-10-15/km-03-broker/manifest.json)을 따른다. C의 실제 Claude host는 남아 KM-03.4는 진행 중, 전체11/28·기능2/7을 유지한다.


## KM-03.4 C 완료 기록

KM-03.4 C 완료: 실제 설치 Claude Code2.1.300을8회 실행해68개 도구 결과를 원시 MCP·실제 API 입력과 대조했다. 문서 없는 입력의 수집→제공 검토→filtered adoption→held build→실제 원문/인용과 재시작 조회, 근거 부족·타 프로젝트 거부, 엔진 누락, 진행 중 정상 종료·host 강제 중단 후 durable 취소/exact replay/새 구축·조회를 확인했다. 정상 매니저 종료는 모두0이며 엔진 누락의 시작 거부2회(각2)와 의도한 host SIGKILL1회(-9)는 별도로 보존했다. 실제 SDK SIGINT에서 매니저가-2로 끝나던 문제를 graceful 종료로 수정하고 SIGINT/SIGTERM 회귀를 추가했다. 등록14·수집23·검토35·broker31의103개를 CI manager-test 진입점에 연결했고 로컬 실행을 통과했다. 원격 CI 실행으로 주장하지 않는다. 전체 Go3,838사례(3,820통과·18skip), 관련6패키지 race812(810통과·2skip)를 새로 실행했으며 build/fmt/vet/lint·문서와 native strict3개·증거 보호16개를 통과했다.

로컬 시험 응답기는 실제 host가 호출할 동작만 제공하고 MCP 결과는 실제 매니저/CKS에서 받았다. 모델 호출·비용은0이며 자율 에이전트 성능·의미 품질·독립 사람 수용을 판정하지 않았다. strict MCP fixture는 plugin 선언의 실제 launcher를 명시 연결했다. 처음의 strict discovery 설정·실제 SIGINT 종료 오류·시험 parser/retry 단언 및 test-only ResourceWarning은 initial-failures에 보존했다. 원본 source와 기존 current는 수정하지 않았다.

[C packet](../../system/eval/knowledge-roadmap-2026-10-15/km-03-host/manifest.json)을 따른다. 전체12/28·기능3/7 완료다. 남은16개는 KM-04.1–4(정기/변경 갱신), KM-05.1–4(공통 개념 매핑), KM-06.1–4(비교 근거 MCP), OE-01.1–4(온톨로지 효과 평가)다. 다음은 KM-04.1의 trigger·성공 기준·변경 pin 설계다. 정책 생성기 보완·운영/독립 수용9개·선택 후속12개는 별도로 유지한다. 사용자가 멈추기 전까지 단계별 서명 커밋하며 이어간다.


## KM-04.1 계약 검증 완료

KM-04.1 완료: 명시 정책·수동/정기/변경 polling·clock skew/놓친 일정 합치기·등록/프로젝트/명시 corpus 범위·재시도 상한·required gate와 held/게시 성공 계약을27개 반례로 검증했다. 문서 없는 source의 자동 관찰 입력과 사람의 의미 검토를 분리하고 변경된 verified 지식의 무조건 승계를 금지했다. 이 단계는 설계/정책 판정이며 scheduler 실행·자동 DB 갱신 완료가 아니다. build/fmt/vet/lint·문서132개·증거 보호10개를 통과했다. Go/의존/fixture byte가 C와 같아3,820통과·18skip 및 race810통과·2skip의 committed 원자료를 참조했으며 새 Go 실행으로 세지 않는다.

[지속 갱신 계약](./KNOWLEDGE-MANAGER-UPDATES.md)과 [정책 계약 packet](../../system/eval/knowledge-roadmap-2026-10-15/km-04-contract/manifest.json)을 따른다. 전체13/28·기능3/7이다. 다음은 KM-04.2의 변경/삭제/no-op·자동 관찰 입력 구현이다. 남은15개는 KM-04.2–4, KM-05.1–4, KM-06.1–4, OE-01.1–4이며 정책 생성기·운영/독립 수용9개·선택 후속12개는 별도로 유지한다. 사용자가 멈추기 전까지 단계별 서명 커밋하며 계속한다.


## KM-04.2 변경·자동 관찰 입력 검증 완료

KM-04.2 완료: 자동 관찰 입력·선택 파일의 추가/수정/삭제·same-byte rename 후보·명시 knowledge/ontology 원문·제공 기록·검토 stale/no-op pin을 구현했다. 사용자 source에 문서를 쓰거나 review/human_response를 만들지 않고 native knowledge init/validate/lock/validate를 수행한다. 정책27·입력32·기존 manager103의162개와 실제 공개 MCP16항목·4개 held DB·82호출·11연결 EOF0·증거 보호14개를 검증했다. 실제 두 번 변경·삭제 후 최신 원문/인용과 이전 snapshot/타 프로젝트 DB 보존, 실제 inspect/status/reader가 있는 무변경 판정을 확인했다. 메타데이터/input equality만으로 DB no-op을 승인하지 않는다. 이 실행은 수동 검증이며 주기 worker 검증이 아니다. build/fmt/vet/lint·문서132개를 통과했고 Go1256개·의존/fixture byte가 C와 같아 원래3,820통과/18skip·race810통과/2skip을 참조했다. 새 Go 실행으로 세지 않는다.

[관찰 입력 packet](../../system/eval/knowledge-roadmap-2026-10-15/km-04-inputs/manifest.json)을 따른다. 전체14/28·기능3/7이며 다음은 KM-04.3의 durable intent·한 writer·재시작/retry·실제 worker와 gated 전환이다. 남은14개는 KM-04.3–4, KM-05.1–4, KM-06.1–4, OE-01.1–4다. 정책 생성기·운영/독립 수용9개·선택 후속12개는 별도 유지하며 멈추라는 요청 전까지 단계별 서명 커밋으로 계속한다.


## KM-04.3 중간 A 검증

KM-04.3 A 원문 전환 전 확인 완료: operator profile의 선택적 source_check로 실제 Python/코드·소유 정책/등록/불변 입력을 고정하고 admission/후보 완료/게시 pointer lock 안에서 원래 source fingerprint를 재확인한다. 실제 MCP14항목·90호출·3개 job에서 v1 게시→v2 원문 변경으로 게시 거부/current·기존 reader 보존→v3 복구 게시/조회와5연결 EOF0을 확인했다. Go 전체3,862사례(3,844통과·18skip), 관련6패키지 race836(834통과·2skip)을 새로 실행했고 Python174개·증거 보호14개·build/fmt/vet/lint·문서132개를 통과했다. 추가 source-check23사례와 이미 지난 deadline1사례는 전체/race 분모 안에 포함하며 중복 합산하지 않는다.

처음에는 원문 probe가 native 접수 중의 아직 root owner가 없는 소유 journal과 충돌했다. 원문 전용 probe만 읽기 확인과 DB 소유 검증을 분리했고 기본 등록·doctor·prepare·DB 쓰기의 소유 검증은 유지했다. Native 오류의 실제 structuredContent를 읽도록 검증기도 수정했다. 최초 race의 기존 deadline 시험1회 late-writer 실패는5회 반복에서 재현되지 않았다. timer callback 도착만 가정하지 않도록 exec 직전/직후 절대 시각 확인을 보강하고 과거 deadline 거부 시험을 추가했다. 두 경계를 각각5회 확인한 뒤 최종 전체·race를 순차 실행해 실패0을 확인했다. 최초 실패의 단일 원인을 입증했다고 주장하지 않으며 initial-failures의 원자료를 보존한다.

[원문 확인 packet](../../system/eval/knowledge-roadmap-2026-10-15/km-04-source-check/manifest.json)을 따른다. 현재 소스1,707입력=Go1,258+기타449이며 승인5개는 보존했다. 직전2c03322f의 입력 검증을 동결 snapshot에서 별도 확인했다. A는 native 원문 검사 범위다. KM-04.3의 B durable worker/일정/중복/retry와 C 중단/복구 연결, KM-04.4 실제 주기 실행은 남는다. 전체14/28·기능3/7을 유지한다. 남은14개는 KM-04.3–4, KM-05.1–4, KM-06.1–4, OE-01.1–4이며 정책 생성기·운영/독립 수용9개·선택 후속12개는 별도로 유지한다. 사용자가 멈추기 전까지 단계별 서명 커밋하며 다음 B를 진행한다.


## KM-04.3 중간 B1 검증

KM-04.3 B1 durable 상태 기반 완료: 정책을 소유 불변 파일로 저장하고, 실제 native inspect 형식의 source/model/input/base와 version/key/deadline을 포함한 정확한 intent를 접수 전에 원자 저장/fsync한다. 기존 reviewed broker와 하나의 writer lease를 공유하고 pending intent·미종결 reviewed job의 교차 실행을 막는다. 응답 손실 후 원래 intent 보존, 단조 job revision·terminal 불변, 현재 좌표/required gate/실제 reader 확인을 성공 조건으로 하고 실패·취소 시 마지막 성공을 보존한다. retry는 같은 cycle/input의 새 key/version으로만 연결하고 상한을 새 cycle로 우회하지 못하게 했다. 저장 불확실성은 프로세스를 중지 상태로 만들고 재시작 때 디스크 상태를 읽는다. 50개 상태·오류 시험과 기존174개가 통과했다. 이 중 실제 CKS inspect/EOF를 확인하지만 상태 전이용 job/read 응답은 통제 관찰이며 자동 DB 실행 증거로 세지 않는다. configure-updates/update-status는 설정·상태만 제공하고 worker나 scheduler를 시작하지 않는다.

[상태/재시도 계약과 검증](./KNOWLEDGE-MANAGER-UPDATES.md)을 따른다. 전체14/28·기능3/7을 유지한다. 다음은 B2 실제 worker·주기/change polling·exact replay와 reader 연결이고 C 실제 강제 종료/복구 및 KM-04.4 연속 주기 검증이 남는다. 남은14개는 KM-04.3–4, KM-05.1–4, KM-06.1–4, OE-01.1–4이며 정책 생성기 보완·운영/독립 수용9개·선택 후속12개는 별도 유지한다. 사용자가 멈추기 전까지 단계별 서명 커밋으로 계속한다.


## KM-04.3 중간 B2 검증

KM-04.3 B2 foreground worker 연결 완료: 명시 enabled 정책의 manual/periodic/change polling을 bounded run-updates로 실행한다. immutable 관찰 입력과 원문 source_check를 native profile에 결속하고 intent fsync 후 실제 build/status/read를 연결했다. 미확정 접수는 같은 config/request/key로 replay하고 known job은 source 재수집 전에 조회한다. 명확한 접수 전 거부와 불확실한 transport/persistence를 구분하며 실패·취소·refusal은 같은 cycle/input의 새 version/key로 제한 재시도한다. 실제 current와 retained reader를 확인해야 게시/무변경 성공으로 판정한다. 기존18도구 reviewed MCP broker는 hold-only이며 별도 foreground worker와 하나의 writer lease를 공유한다.

Python247개(기존224+worker23), 실제 worker14항목·native RPC 관찰97개·6job(게시4/의도한 실패1/응답 손실 후 같은 job 취소 복구1)·16연결 EOF0을 통과했다. v1 게시/조회→무변경→code8/원문 삭제 v2→보존 v1 조회→code9 엔진 실패/current 보존→due periodic retry2 복구 게시→code10 접수 응답 손실/정상 EOF→code11 변경 후 원래 job 먼저 복구→별도 code11 게시를 검증했다. periodic due retry는 실제 관찰이지만 지속 schedule/실제 worker host SIGKILL은 아직 미검증이다. Go 전체3862(3844통과18skip)와 관련6패키지 race836(834통과2skip)을 새로 실행했다. build/fmt/vet/lint·문서132개와 직전 B1 동결 증거16개를 확인했다. 표준 manager-test에 갱신5개 suite를 추가했으며 원격 CI 통과를 주장하지 않는다.

[foreground 실행/경계](./KNOWLEDGE-MANAGER-UPDATES.md)를 따른다. 전체14/28·기능3/7은 유지한다. 다음은 C 실제 foreground worker host 중단/SIGKILL·재시작·기한/잔여 child·MCP host와의 운영 연결이다. 그 다음 KM-04.4의 실제 지속 periodic/change와 두 번 변경·삭제/no-op·실패 복구·다른 프로젝트 불변을 종합 검증한다. 남은14개는 KM-04.3–4, KM-05.1–4, KM-06.1–4, OE-01.1–4이며 정책 생성기 보완·운영/독립 수용9개·선택 후속12개는 별도 유지한다. 사용자가 멈추기 전까지 단계별 서명 커밋으로 계속한다.


## KM-04.3 중간 C1 검증

KM-04.3 C1 실제 foreground 프로세스 중단 검증 완료: 실제 CLI worker의 SIGTERM·SIGKILL·시간 상한3경로를21항목/CLI10회로 검증했다. 지연 중인 실제 native graph를 중단하고 원래 current·intent를 보존하며, native/graph가 실행 중이 아님을 확인하고 예정된8초 지연 쓰기 시점 이후까지 늦은 쓰기가 없음을 관찰했다. 재시작 시 원래 job의 terminal canceled/activated=false를 먼저 조회하고, 같은 input/cycle의 attempt2·새 key/version으로3건 모두 정상 게시/원문 조회에 복구했다. parent 종료 뒤 native 자식의 exit code는 직접 관측할 수 없어0으로 표시하지 않았다. 이는 실제 CLI 프로세스/자식 정리 증거이며 MCP 갱신 운영 연결과 지속 schedule 검증은 별도다. 반복 검증에서 SIGTERM이 상태 저장 중 도착하면 마지막 확인 메모리와 디스크 상태가 다를 수 있는 경계를 발견했다. 자식 종료/intent 보존 후에도 추가 write를 금지하고 last_confirmed_in_process_state·disk_state_requires_reconciliation·closed_requires_persistence_reconciliation을 명시하도록 보완했다. 실제 native 접수 뒤 fsync 중 신호를 재현하는1개를 포함해 Python248개를 새로 검증했다.

[실제 프로세스 증거와 C2 경계](./KNOWLEDGE-MANAGER-UPDATES.md)를 따른다. 전체14/28·기능3/7을 유지한다. 다음은 C2 MCP 세션의 명시 갱신 시작·상태·중지·정확한 DB 조회와 EOF/host 종료 연결이다. 이후 KM-04.4의 실제 지속 periodic/change와 두 번 변경·삭제/no-op·실패 복구·다른 프로젝트 불변을 종합 검증한다. 남은14개는 KM-04.3–4, KM-05.1–4, KM-06.1–4, OE-01.1–4이며 정책 생성기 보완·운영/독립 수용9개·선택 후속12개는 별도 유지한다. 사용자가 멈추기 전까지 단계별 서명 커밋으로 계속한다.


## KM-04.3 통합 완료

KM-04.3 완료: A native 원문 전환 전 확인, B1 durable intent/한 lease/상한, B2 실제 foreground worker, C1 실제 CLI TERM/KILL/budget·저장 중 신호 보고, C2 갱신 MCP 운영 연결을 통합 검증했다. plugin0.6.0은 기존 reviewed 기본18도구를 유지하고 명시 updates 모드에6도구(policy template/configure/status/start/stop/read)를 제공한다. MCP는 별도 소유 worker를 감독하면서 sealed 상태와 지정 retained DB 조회를 계속 처리한다. 부모 EOF/강제 종료는 소유 stdin 종료로 worker에 전달하고, 기록된 과거 PID를 signal하지 않는다. 재시작은 원래 job/요청을 먼저 조정한다.

Python268개(기존224+worker/MCP44), 실제 공개 MCP20항목·222도구호출·3연결(의도한 부모SIGKILL-9/정상EOF0/정상EOF0), 부모 중단2경로의 Native/graph 정리·9초 이후 지연 쓰기 부재/current 보존·2건 연결 재시도 게시를 확인했다. 실제 Claude Code2.1.300의 설치된 plugin/skill/agent를 로컬 시험 응답기에 연결해2회/도구36개·MCP정상 종료0으로 raw source 구축/held조회·같은 job 재시작·periodic no-op·중지를 검증했다. 유료 모델/실제 모델 호출0이며 agent 자율성·검색 품질·독립 수용을 측정했다고 주장하지 않는다. Go/의존/fixture byte가 같은 B2의 Go3862(3844통과18skip)·race836(834통과2skip)을 byte 대조해 참조하며 새 Go 실행으로 세지 않는다. build/fmt/vet/lint·plugin strict·문서132개와 C1 동결 증거16개를 확인했다.

[공개 모드·진행/회복 계약](./KNOWLEDGE-MANAGER-UPDATES.md)을 따른다. 전체15/28·기능3/7이다. 다음은 KM-04.4의 지속 periodic/change·두 번 변경·삭제/no-op·실패/복구·이전 snapshot과 다른 프로젝트 보존을 통합 확인하는 단계다. 남은13개는 KM-04.4, KM-05.1–4, KM-06.1–4, OE-01.1–4이며 정책 생성기 보완·운영/독립 수용9개·선택 후속12개는 별도로 유지한다. 사용자가 멈추기 전까지 단계별 서명 커밋으로 계속한다.


## KM-04 최종 연속 갱신 검증

KM-04.4 완료: 실제 두 프로젝트에서20항목·MCP221호출·정상 EOF2개를 검증했다. A는 periodic/change로7→8→9→10을 게시하고 문서·명시 ontology 원자료 root 삭제와 제공 메모 교체를 반영했다. 실제 graph 실패1건은 current/마지막 성공을 보존했고 같은 입력의 연결된 attempt2가 복구했다. no-op은 중복 job을 만들지 않았고 v1 코드/삭제된 자료의 당시 인용을 계속 조회했다. 동명 module/Value를 가진 B의 DB 전체 파일 hash·current·실제 본문/인용은 불변이었다. 비공개 자료 제외·자동 의미/사람 검토 미생성도 확인했다. 이는 mock embedding의 실제 lifecycle 검증이며 검색 품질·온톨로지 효과·독립 수용은 별도다.

전체16/28·기능4/7 완료다. 다음은 KM-05.1의 산업 개념·정의·별칭·포함/제외·비교 축 설계다. 남은12개는 KM-05.1–4, KM-06.1–4, OE-01.1–4이며 정책 생성기 보완·운영/독립 수용9개·선택 후속12개는 별도로 유지한다. 사용자가 멈추기 전까지 단계별 서명 커밋하며 계속한다.


## KM-05.1 공통 개념 팩

KM-05.1 완료: 기존 knowledgepack loader를 재사용한 선택형 industry.blockchain1.0.0 팩에8개념·26비교 축을 추가했다. 한영 별칭·포함/제외·역할/버전/network/config/시점 조건, 정확 byte pin과 unknown/ambiguous 계약을29개 Go 사례로 검증했다. 단어의 exact_term 일치를 구현 매핑 승인이나 project/entity 동일성으로 해석하지 않는다. 공통 core20·운영 dataset·기본 자동 선택은 변경하지 않았다. 프로젝트 연결·비교 MCP·ontology 품질은 남는다.

[설계와 사용 경계](./INDUSTRY-CONCEPT-COMPARISON.md)를 따른다. 전체17/28·기능4/7이다. 다음은 KM-05.2의 프로젝트별 코드·문서·설정/정책·테스트 근거 연결이다. 남은11개는 KM-05.2–4, KM-06.1–4, OE-01.1–4이며 정책 생성기 보완·운영/독립 수용9개·선택 후속12개는 별도 유지한다. 사용자가 멈추기 전까지 단계별 서명 커밋으로 계속한다.


## KM-05.2 프로젝트별 원문 연결

KM-05.2 완료: 명시 common concept/version/digest와 프로젝트·버전·network/config/역할/관찰 시점에 결속한 원문 연결을 구현했다. 서로 다른 이름의 같은 역할과 같은 이름의 다른 역할을3개 프로젝트의5종류 원문으로 검증하고, 동일 로컬ID를 서로 다른 scope에 보존했다. 기존 collection reader lease·target 검증과 retained evidence 정화/재검증을 재사용한다. code/document/configuration/policy/test와 원문 hash/span을 구분하며 모든 연결은 proposed다. 실제 사람 승인·그래프/벡터 검색 품질·공개 비교 MCP는 완료로 세지 않는다.

[설계/검증 경계](./INDUSTRY-CONCEPT-COMPARISON.md)를 따른다. 전체18/28·기능4/7이다. 다음은 KM-05.3의 직접 매핑 의미·검토/회수·버전/stale 정책이다. 남은10개는 KM-05.3–4, KM-06.1–4, OE-01.1–4이며 정책 생성기 보완·운영/독립 수용9개·선택 후속12개는 별도 유지한다. 사용자가 멈추기 전까지 단계별 서명 커밋으로 계속한다.


## KM-05.3 직접 매핑과 검토 정책

KM-05.3 완료: exact/close/broader/narrower/related를 local→shared 직접 관계로 고정하고 entity 동일시/자동 전이를 금지했다. 정확 source/해석/팩 pin의 검토 기록, 중복 검토자 정족수·최신 판정·회수·충돌·미래 시각·stale와 과거 버전 보존을 구현했다. 공개용 reader는 보관 원문을 재확인하며 승인 기록으로 손상을 우회하지 않는다. 검토자 label은 인증된 신원이 아니고 실제 사람 승인은 생성하지 않았다. 입력 필드의 대소문자/유니코드 별칭 덮기도 재현·거부했다.

[설계/검증 경계](./INDUSTRY-CONCEPT-COMPARISON.md)를 따른다. 전체19/28·기능4/7이다. 다음은 KM-05.4의 비적용/미수집/미지원·역할/산업/network/config 차이 검증이다. 남은9개는 KM-05.4, KM-06.1–4, OE-01.1–4이며 정책 생성기 보완·운영/독립 수용9개·선택 후속12개는 별도 유지한다. 사용자가 멈추기 전까지 단계별 서명 커밋으로 계속한다.


## KM-05 공통 개념/매핑 계층 완료

KM-05.4 완료: exact/similar 후보·incomparable·not_collected·unknown·unsupported·not_applicable·conflict·unavailable을 분리했다. 다른 산업/역할·L1 consensus/L2 sequencing·network/config를 구분하고, Rust 원문 읽기와 Rust AST 미지원을 혼동하지 않는다. 비적용/미결정 선언은 source/조건/검토와 함께 처리하며 검색 무응답·거부/회수를 기능 부재로 바꾸지 않는다. schema2 선언을 digest에 포함하고 기존 schema1 식별값은 유지했다. 상충은 같은 프로젝트/조건 안에서만 판단하며 보관 손상은 결과를 폐기한다.

[설계/검증 경계](./INDUSTRY-CONCEPT-COMPARISON.md)를 따른다. 전체20/28·기능5/7 완료다. 다음은 KM-06.1 비교 요청·MCP 계약 설계다. 남은8개는 KM-06.1–4, OE-01.1–4이며 정책 생성기 보완·운영/독립 수용9개·선택 후속12개는 별도 유지한다. 사용자가 멈추기 전까지 단계별 서명 커밋으로 계속한다.


## KM-06.1 비교 요청 계약

KM-06.1 완료: 읽기 전용 comparison 모드·고정 profile/catalog·명시 scope 참여자/공통 개념/축·시각/예산 계약을 구현했다. 이름이나 자연어에서 대상을 추측하지 않고2–16개 scope와1–8개 축을 선택한다. stale/타 collection/동일 범위 중복·구현 모드·요청 경로/target 변조·미래 시각/초과 예산을 거부한다. 한 프로젝트의 명시적인 다른 조건도 비교 가능하며 호출자 변경으로 고정 라우팅을 바꾸지 못한다. 이 단계는 계약50개 검증이며 actual source read/MCP 호출0이다.

[공개 계약/후속 설계](./COMPARISON-MCP-CONTRACT.md)를 따른다. 전체21/28·기능5/7이다. 다음은 KM-06.2의 공개 MCP·프로젝트별 근거 행렬/bundle 구현이다. 남은7개는 KM-06.2–4, OE-01.1–4이며 정책 생성기 보완·운영/독립 수용9개·선택 후속12개는 별도 유지한다. 사용자가 멈추기 전까지 단계별 서명 커밋으로 계속한다.


## KM-06.2 실제 비교 MCP

KM-06.2 완료: 고정된 로컬 profile/catalog·선택 pack/연결/검토·정제 규칙의 실제 byte digest를 고정하는 loader와 독립 stdio 비교 MCP를 구현했다. describe/extract2도구만 노출하고 읽기 전용·비파괴·동일 요청 반복 가능·외부 네트워크 비사용 표시를 명시한다. 각 프로젝트/조건·개념/축의 source 종류·본문/인용·매핑 이유/검토 상태·unknown을 분리하며 양쪽 근거가 있는 공통/차이 후보만 제공한다. 기존 매니저18/updates6도구에 자동 추가된 것으로 세지 않는다.

비교171개(계약167+runtime4), 전체 Go4033(4015통과18skip)·race1007(1005통과2skip,7패키지)·빌드/fmt/vet/lint·문서134개를 검증했다. 실제 native mock2DB 구축 후12항목·9도구호출·2세션 EOF0이며 유료 모델0이다. 통제 source fixture의 실제 조회/transport 검증이며 실제 체인 비교·의미 품질/독립 수용은 아니다.

전체22/28·기능5/7이다. 다음은 KM-06.3 부분 실패·상충/누락·기한/예산·reader/종료 보호의 완성이다. 남은6개는 KM-06.3–4, OE-01.1–4이며 정책 생성기 보완·운영/독립 수용9개·선택 후속12개는 별도 유지한다. 사용자가 멈추기 전까지 단계별 서명 커밋으로 계속한다.


## KM-06.3 부분 실패·예산·종료 보호

KM-06.3 완료: 모든 참여자를 읽은 뒤 응답 직전에 각 archive를 다시 검증한다. 뒤 프로젝트를 여는 동안 앞 archive가 바뀌면 해당 프로젝트의 본문/인용·후보만 폐기하고 나머지 프로젝트와 명시 좌표는 보존한다. 같은 scope의 적용/비적용 상충과 다른 프로젝트의 차이, 미수집/unknown/손상 상태를 구분한다. 후보 proof의 복사 인용까지 max_citations에 포함하며 예산에 맞지 않으면 후보 전체를 제외하고 omitted_candidates/candidate_state로 알린다. 요청 기한/취소에는 이전 부분 성공을 공개하지 않고 Close는 진행 중 reader 정리를 기다린다.

비교192개(기존171+보호21,47개 상위), 전체 Go4054(4036통과18skip)·race1028(1026통과2skip,7패키지)·빌드/fmt/vet/lint·문서134개를 검증했다. 실제 mock2DB·기반12+보호9항목·18도구호출·4세션(EOF3개와 입력이 열린 SIGTERM1개 모두 exit0)이다. 원문 손상 후 정확히 복구했고 DB/manifest/profile/catalog/binding 전체 hash를 보존했다. 실제 체인 비교·의미 품질/독립 수용은 아니다.

전체23/28·기능5/7이다. 다음은 KM-06.4 둘 이상 실제 프로젝트의 고정 DB·staking/정책·consensus/finality·mempool/ordering 비교 검증이다. 남은5개는 KM-06.4, OE-01.1–4이며 정책 생성기 보완·운영/독립 수용9개·선택 후속12개는 별도 유지한다. 사용자가 멈추기 전까지 단계별 서명 커밋으로 계속한다.


## KM-06.4 실제 프로젝트 비교 완료

KM-06.4 완료: go-stablenet0bf2f4d1의committed 원문2,224파일과 go-wemix1350376a의working-tree 원문2,019파일로 실제 native graph/vector DB를 구축했다. go-wemix의 cmd/gwemix→geth alias만 소유 복제본에서 제외하고 BaseCommit·제외 목록·준비 Git 상태를 기록했다. 비교하는7개 Go 파일은 원본 커밋 바이트와 동일하며 원본 저장소는 변경하지 않았다. 모든 매핑은 미검토 후보다.6개 개념·40개 프로젝트/축 셀에서7개similar·18개unknown·15개not_collected를 관찰했다. 이는 선택한 매핑/근거의 범위이며 실제 네트워크나 기능 부재·finality·TPS·보안성·우월성의 판정이 아니다.

실제 비교MCP22항목·19도구호출·3EOF0, 원문/범위 hash와 본문·프로젝트/모드별 인용·양쪽 후보·기본 미검토 제외·live 소스 삭제 후 동일 응답·한쪽 archive 누락/오매핑·범위/예산 거부·원본/DB 보존을 검증했다. 새 Go 변경이 없음을1273개 Go와 모든 결속 입력의 hash로 확인해 직전 전체Go4054(4036통과18skip)·race1028(1026통과2skip,7패키지)·빌드/fmt/vet/lint 결과를 재사용했다. 새 실제 corpus/MCP 시험과 과거 Go 실행을 구분한다. 유료 모델 호출0이며 ontology 효과/독립 품질 수용은 아니다.

전체24/28·기능6/7이다. 다음은 OE-01.1의 새 질문·기대 근거·평가 범위/분모 고정이다. 남은4개는 OE-01.1 질문/기대 근거, OE-01.2 동일 자료4모드, OE-01.3 근거·순위·unknown·지연/비용, OE-01.4 다중 프로젝트 매핑/오인/누락 효과 평가다. 정책 생성기 보완·운영/독립 수용9개·선택 후속12개는 별도 유지하며 단계별 서명 로컬 커밋으로 계속한다.


## OE-01.1 확대 평가 사전 등록

OE-01.1 완료: 등록 용어의 한영 표현·다른 용어·다의어(validatorMu/currentMutex)·주석/구현 차이·범위 밖을 포함한 새16질문을 사전 등록했다. 함수 찾기14질문은5사실군·고유 함수11개의 완전한 source span을 기준으로 top10의 함수 회수 precision/recall·순위를 계산하며, 제어2질문은 일반 source relevance를 채점하지 않고 ontology 의도 범위와 원문 무결성을 관찰한다. 일반 행동/정책의 완전한 관련 근거나 답변 정답을 뜻하지 않는다. 기준은 assistant source-derived이며 사람/독립 gold 승인은0이다. 이전13/42문항을 새 독립 효과 입력으로 재사용하지 않고, 기존 사실군과 표현 변형의 중복도 명시한다.

source commit baca2869·동일 비시험 Go882개·승인 정의4개/검토된 구현 연결0개·BGE-M3 digest790764642607…16bab/1024차원을 고정했다. 현재 로컬 Ollama0.40.4를 기록하므로 과거 서버 버전의 절대 지연을 새 관찰과 합산하지 않는다. 동일 자료/모델/모드 외 설정과 ontology mode만 바꾸는4arm, 질문별 준비1/검색2/warm5·순서 교대·총512회 계획이다. 사전 등록 단계의 새 검색/LLM 답변/유료 모델 호출은 모두0이다. 후속 실행은 소유 loopback proxy로 API 경로·해시/bytes·상태·시간·서버가 제공한 token count를 계측하고, bytes에서 token/금액을 추정하지 않는다. 작은 표본은 기술 통계로 보고한다.

전체25/28·기능6/7이다. 다음은 OE-01.2의 고정512회4모드 실행이다. 남은3개는 OE-01.2 동일 자료4모드, OE-01.3 함수 근거·순위·ontology 범위/unknown·지연/API 호출/관측 비용, OE-01.4 다중 프로젝트 매핑/오인/누락 효과 평가다. 정책 생성기 보완·운영/독립 수용9개·선택 후속12개는 별도 유지한다. 기본 off·quality=inconclusive는 실제 품질 승인 없이 바꾸지 않는다.


## OE-01.2 확대4모드 실제 실행

OE-01.2 완료: 사전 등록16질문×4모드×8호출=512회를 실제 로컬 BGE-M3로 수집했다.4개 readiness와64개 capture 세션의 종료, 질문/모드/단계/반복 슬롯·설정/바이너리·source/snapshot·인용/본문/무결성 hash를 검증했다. 검토된 정의4개/구현 연결0개를 보존하고, mode 외 설정은 동일하다. 원본 graph/vector/source·의미 DB·바이너리·모델/서버가 전후 동일하며 소유 loopback API 계측기는 진행 중 요청0으로 종료했다. 이 단계는 실행/원문 검증이며 효과 점수나 품질 수용이 아니다.

인용4896개·본문4896개·원문 blob65개를 검증했고 실패/누락0이다. 실제 API 20420개를 metadata/준비/질문-arm별로 기록했다. 기본 off·quality=inconclusive와 사람/독립 gold0·실제 LLM 답변0·유료 모델0을 유지한다. 초기 off 기본값 정규화 오류와 계측 로그 전체 재작성에 따른 제어 중단을 별도로 보존했다. 초기 지연/부분 실행은 본512회 지표에 합산하지 않는다.

전체26/28·기능6/7이다. 다음은 OE-01.3의 고정 함수 회수 precision/recall·순위·ontology 의도 범위/unknown·warm 지연/API 호출/관측 비용 재계산이다. 남은2개는 OE-01.3 분석, OE-01.4 다중 프로젝트 매핑/오인/누락 효과 평가다. 정책 생성기 보완·운영/독립 수용9개·선택 후속12개는 별도 유지한다.


## OE-01.3 확대 효과·비용 분석

OE-01.3 완료: 고정512회 원자료에서 유한한 완전 함수 찾기 과제의 precision/recall·순위·의도한 ontology 개념/범위 진단·warm 지연·실제 로컬 API 호출/서버 token 수를 재계산했다.5사실군 평균 recall은 off54.44%→concept_text/combined56.67%, precision12.87%→13.54%, MRR0.4612→0.4862다. 개선은 EX-08 영어 메시지 처리 질문1개에서 prepare만 회수하던 결과에 commit이 추가된 것으로 한정된다. 나머지13개 채점 질문의 지표는 같았다. hit@10은68.33%로 동일하다. 일반 행동/정책의 semantic relevance나 답변 정확도로 확대하지 않는다.

각 arm의 warm80개 p95는 off583.1ms, concept_text736.2ms(1.263배), relations607.4ms(1.042배), combined697.4ms(1.196배)다. capture batch embedding API는 off/relations1584개·31536서버 prompt tokens, concept_text/combined1664개·56112tokens다. 준비/모델 확인도 포함한 도구 실행 batch 비용이며8호출×16질문의512개 독립 사실이 아니다. 토큰은 서버 보고값이고 금액/전력/하드웨어 비용은 추정하지 않았다. 유료/LLM 답변0이다. 활성 모드는 의도 개념13질문 중9개를 매칭했고, scope 의도 진단3질문 중 외부 go-wemix 대상 EX-16에도 WBFT quorum을 매칭했다. relations는 검토된 구현 연결0·rank 변화0이다. 정의 검색/combined의 최종 인용 순서는32짝 중18짝에서 달랐지만 함수 회수 이득은1질문뿐이다. 작은 source-derived 개발 표본과 지연 관찰로 기본 off·quality=inconclusive를 유지한다.

전체27/28·기능6/7이다. 다음이자 마지막 세부 작업은 OE-01.4의 실제2프로젝트 공통 개념 매핑 효과·오인/누락 평가다. 정책 생성기 보완·운영/독립 수용9개·선택 후속12개는 별도 유지한다. 실제 LLM 답변·독립 정답·운영 활성화는 미실행/미승인 상태로 보존한다.
