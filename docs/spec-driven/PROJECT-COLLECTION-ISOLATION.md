# KM-01 프로젝트·버전 격리와 collection 기반

2026-10-15 구현 범위는 immutable catalog와 구현/비교 읽기의 공통 Go 계층이다. 여러 프로젝트의 DB를 물리적으로 합치지 않고 각 pinned DB를 명시적 collection으로 묶는다. 새 CLI/MCP 공개 도구, DB 구축·갱신 쓰기, 산업별 비교는 아직 연결하지 않았다. 명세의 KM-01.1/02는 완료하고, 공개 경로와 실제 쓰기·비교 검증이 필요한 KM-01.3/04 및 KM-01 전체는 진행 중이다.

상태는 [기능 명세](./KNOWLEDGE-MANAGER-AND-COMPARISON-SPEC.md), [진행 원장](./KNOWLEDGE-ROADMAP-PROGRESS.json), [전체 시험표](./KNOWLEDGE-ROADMAP-TEST-MATRIX.md)를 따른다. 구현은 [catalog](../../internal/system/collection/catalog.go)와 [session](../../internal/system/collection/session.go), 시험은 [collection_test.go](../../internal/system/collection/collection_test.go)다.

## 조사: 재사용할 경계와 부족한 부분

| 영역 | 현재 신원·격리 경계 | collection에서 필요한 처리 |
|---|---|---|
| 프로젝트 등록 | [init](../../cmd/cks/doctorcli/init.go)의 명시 ID 또는 무작위 ID. 이름/URL/경로로 ID를 만들지 않음 | 기존 ID를 그대로 등록. 같은 이름·URL의 다른 프로젝트를 합치지 않음 |
| dataset/source | [identity](../../internal/setup/identity.go)의 project/source/snapshot/input/embedding 결속과 엔진 manifest 검사 | 각 version을 실제 identity와 대조. legacy를 pinned로 추정하지 않음 |
| CKG symbol/관계/history | [schema](../../internal/graph/persist/schema.sql)의 node/canonical/edge/PR/commit ID는 해당 DB 안의 join 경계 | import path·짧은 qualified name·node ID를 프로젝트 간 전역 식별자로 사용하지 않음 |
| CKV 조회 | [CKV client](../../internal/system/ckvclient/real.go)는 구성한 인덱스와 embedding identity를 사용 | 선택 버전별 backend를 사용해야 함. 다른 DB의 결과를 현재 대상으로 재표기하지 않음 |
| semantic 저장 | [store](../../internal/system/semantic/store.go)의 project_id+dataset_id PK, project별 current. [Snapshot](../../internal/system/semantic/model.go)에 source 좌표 포함 | project/current를 자동 순회하지 않고 catalog에 명시한 dataset/snapshot만 선택 |
| 지식 팩 | [lock](../../internal/system/knowledgepack/lock.go)의 project_id와 pack origin/hash, retained 지식 근거 | 공통 pack의 개념이 같아도 프로젝트 인스턴스/코드 신원을 합치지 않음 |
| 인용/dedup | [CitationKeyV2](../../pkg/system/contract/v2.go)의 project/dataset/snapshot/origin/path/span/hash. 옛 Citation.Key는 다른 계약 | V2 pack 검증 재사용, 비교 결과는 프로젝트·버전별 그룹으로 유지 |
| archive 검증 cache | [identity cache](../../internal/setup/identity_cache.go)는 절대 버전 경로와 파일 fingerprint를 사용. [request verification](../../internal/setup/request_verification.go)은 요청 내 재사용 후 재검증 | 세션의 고정 버전과 기존 before/after 검증을 사용. 비교의 마지막에도 모든 참여 버전을 재확인 |
| embedding LRU | [cache](../../internal/vector/embed/cache/lru.go)는 embedder 인스턴스 내 텍스트→벡터. 출처/검색 결과 cache가 아님 | 벡터 재사용과 출처 결과 격리를 구분. 기존 LRU를 collection 조회 cache로 간주하지 않음 |
| DB 쓰기/전환 | [setup](../../internal/system/mcp/setup_tool.go), [reindex](../../internal/system/mcp/reindex_tool.go)의 legacy 제한. version CLI의 writer/promotion/reader lease는 별도 계약 | 기존 pinned 쓰기 거부 유지. collection은 읽기 전용이며 새 version-aware 쓰기는 KM-02에서 연결 |
| export/history/새 MCP | 기존 단일 버전 검증을 전제한 개별 표면 | 향후 adapter가 같은 scope를 전달하고 공개 응답/쓰기에서도 반례 시험해야 함. 이번 공통 계층 통과만으로 모두 연결됐다고 표시하지 않음 |

## 선택한 계약

schema_version=1 catalog는 Project, Version, Collection 목록이다. Project는 명시 ID·표시 이름·원천 URI·선택적인 upstream ID를 가진다. fork 관계는 metadata이며 alias나 동일성 승인이 아니다. 중복 ID·없는 upstream·순환 fork는 거부한다. 같은 URL, 같은 소스, 같은 commit을 가진 서로 다른 ID는 별도 프로젝트로 유지한다.

Version은 기존 V2Coordinates(project_id/dataset_id/snapshot_id/source_mode/base_commit), 실제 immutable version 디렉터리, 수집된 origin 목록이다. 같은 project/dataset을 다른 snapshot이나 경로로 재등록하지 않는다. 절대 정규 경로를 사용하고 `current` 및 symlink 경로는 허용하지 않는다. 모든 graph/vector/retained identity는 기존 setup 검증으로 확인한다. 등록 origin은 실제 archive의 목록과 일치해야 한다.

Collection의 member는 명시한 전체 V2Coordinates다. 한 프로젝트의 두 버전도 명시적으로 포함할 수 있다. 요청은 collection ID와 catalog digest를 고정해야 한다. 이름이나 URI로 구현 대상을 추측하지 않고, 누락 target에 default/current를 적용하지 않는다.

catalog는 프로젝트·버전·멤버 순서와 무관하게 정규 직렬화한다. 등록 내용이나 멤버가 바뀌면 digest가 바뀐다. 경로도 운영 구성이므로 relocation은 새 digest를 만든다. reader는 caller의 slice와 독립된 복사본을 소유한다. JSON은1MiB 이내이며 unknown 필드·중복 key·후행 값을 거부한다. 최대64프로젝트·128버전·64collection·collection당32멤버로 시작한다.

구현 질의는 정확한 target1개만 호출한다. 비교 질의는 target 없이 명시한2–16개 참여 버전을 호출하고 각각의 V2 evidence pack을 별도 group으로 반환한다. 비교의 flat citations나 patch 대상은 만들지 않는다. 없는 멤버·다른 snapshot·중복 참여자·잘못된 mode·오래된 catalog pin은 backend를 부르기 전에 거부한다.

이 공통 계층은 한 그룹이라도 실패하면 결과 전체를 거부한다. KM-06의 부분 비교 응답은 각 프로젝트의 실패/unknown을 드러내는 새 계약으로 추가하며, 다른 프로젝트의 근거로 조용히 대체하지 않는다.

## 반환 근거와 실행 경계

Session.Open은 실제 archive와 좌표·origin을 확인하고 기존 reader lease를 유지한다. 검토 대기 후보를 활성화하지 않는다. GC-aware marker가 없는 과거 pinned archive는 기존 GC의 보존 규칙을 그대로 따른다. Close는 실행 중 읽기가 끝난 뒤 lease를 해제하며 이후 요청을 거부한다.

읽기 전후 identity와 V2 integrity를 확인하고 모든 인용의 retained file/span hash를 대조한다. 본문은 같은 원문에 세션의 sanitizer를 다시 적용한 값과 일치해야 한다. ontology 정의 검색의 진단 출처도 같은 좌표·원문 범위 해시인지 확인한다. 응답을 재서명한 형태로 잘못된 본문·해시가 들어와도 거부한다. 비교의 후반 요청이 앞선 archive를 바꾸는 경우도 마지막 전체 pin 검증에서 결과를 폐기한다.

ScopedRef는 전체 좌표+origin+상대 path+local ID로 키를 만든다. ScopeKey는 catalog/collection/mode/target 또는 참여 버전/query를 묶는 **라우팅 namespace**다. 완전한 검색 결과 cache key가 아니다. adapter는 tool 옵션·model/config·sanitizer·권한 문맥을 추가해야 한다. 이번 구현은 결과 cache나 cross-project join을 새로 활성화하지 않는다. Fetcher는 요청이 끝난 뒤 반환 pack의 소유권을 reader에게 넘겨야 한다.

## 이행과 호환

기존 graph/vector/semantic DB·identity·원문·도구 DTO는 변경하지 않았다. 새 catalog는 별도 설정이다. 기존 stable project ID를 사용하고 InspectVersionIdentity로 확인한 pinned 버전만 등록한다. legacy DB는 기존 경로로 읽고, collection에 넣으려면 정상적인 versioned build가 필요하다. 현재 저장된 DB의 ID를 새 catalog에 맞춰 직접 고치지 않는다.

현재 구현은 아직 CLI/MCP가 호출하지 않는 기반이다. KM-01.3/04의 공개 adapter·실제 backend 쓰기/조회/캐시 검증을 남긴다. 다음 작업은 KM-01.3의 실제 backend·공개 조회 adapter 연결이다. 이후 KM-02.1의 version-aware 도구 capability/요청 설계로 이어간다. 산업 공통 개념/프로젝트 매핑은 KM-05, 비교 자료 수집은 KM-06이다.

## 현재 검증 범위

새 패키지의 race 시험17개(하위 사례 포함40개)를 통과했다. 동일 커밋의 두 실제 Git fixture와 retained source를 사용했다. graph/vector manifest는 fixture이며 실제 그래프 검색이나 embedding 모델 효과를 검증한 것은 아니다. 기존 rollback/read/GC 함수와의 fixture 호환을 시험했다. 기존 공개 CLI/MCP의 다중 프로젝트 연결, 실제 DB 쓰기 및 실제 프로젝트 비교는 미실행이다. 전체 목록과 저장한 원자료는 [시험표](./KNOWLEDGE-ROADMAP-TEST-MATRIX.md)를 따른다.

최종 전체 모듈은3,605개 중3,586개 통과·19개 명시적 skip·실패0이다. 빌드·fmt·lint·문서 gate와 작업표 허위 완료 방지7개도 통과했다. 전체 시험의 skip·fixture 범위·공개 미연결을 통과로 바꾸지 않는다.
