# KM-01 프로젝트·버전 격리와 collection 기반

2026-10-15 현재 immutable catalog·구현/비교 reader를 실제 CKG/CKV backend와 선택적 MCP 조회 도구에 연결했다. 여러 프로젝트의 DB를 물리적으로 합치지 않고 각 pinned DB를 명시적 collection으로 묶는다. KM-01.1–04와 전체 KM-01의 로컬 구현·격리 검증을 완료했다. 실제 embedding 품질·독립 수용은 별도다. DB 구축·갱신 쓰기와 산업 개념별 비교는 각각 KM-02와 KM-05/06에서 구현한다.

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
| export/history/새 MCP | 기존 단일 버전 검증을 전제한 개별 표면 | 향후 adapter가 같은 scope를 전달하고 공개 응답/쓰기에서도 반례 시험해야 함. 연결한 collection 조회 외의 모든 표면까지 검증됐다고 표시하지 않음 |

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

현재 `cks mcp`는 collection 설정이 있을 때 공개 조회 adapter를 연결한다. KM-01.4 최종 반례를 통과했고 다음은 KM-02.1의 version-aware 도구 capability/요청 설계다. 산업 공통 개념/프로젝트 매핑은 KM-05, 비교 자료 수집은 KM-06이다.

## MCP 연결과 설정

기본 단일 프로젝트 도구33개를 유지한다. 상위 CKS 설정에 아래 선택 항목을 추가하면 `cks.collection.status`, `cks.collection.get_context` 두 읽기 도구가 추가되어35개가 된다. [연결 코드](../../cmd/cks/mcpcli/collection.go), [공개 도구](../../internal/system/mcp/collection.go), [설정 검증](../../internal/system/config/collection.go)을 따른다.

```yaml
collection:
  catalog_path: /absolute/catalog.json
  member_configs:
    - /absolute/project-a-v1.yaml
    - /absolute/project-b-v1.yaml
```

각 member는 기존 CKS 설정이며 graph는 `<version>/graph/graph.db`, vector는 같은 `<version>/vector`여야 한다. catalog의 immutable 버전마다 정확히 하나의 설정을 지정한다. `current`, 경로 불일치·중복·누락, 중첩 collection, 상위와 다른 sanitizer, 실제 backend 또는 embedding identity 검증 실패는 시작 단계에서 거부한다. member마다 자신의 model/provider·source root·vocab·semantic/ontology·retrieval 설정을 사용한다. 모든 member는 같은 sanitizer를 사용하며 서버 시작 전에 archive와 manifest를 확인하고 reader lease를 잡는다. caller는 경로나 provider를 도구 인자로 선택할 수 없다.

`cks.collection.status`는 `configured` 상태, catalog digest, collection별 전체 좌표와 member 수, `write_enabled=false`를 제공한다. 로컬 경로를 공개하지 않으며 검색 품질이나 운영 준비 완료 판정이 아니다.

`cks.collection.get_context`는 `catalog_digest`, `collection_id`, `mode`, `query`와 전체 V2 좌표를 받는다. `implementation`은 `target` 하나, `comparison`은 `participants`2–16개를 요구한다. 결과는 선택한 좌표별 `groups[].evidence`이며 평탄하게 합치지 않는다. 각 member의 기존 V2 조회 경로를 실행하고 collection reader가 인용·정제 본문·해시를 다시 확인한다. 한 member라도 실패하면 전체를 거부하며 부분 그룹을 공개하지 않는다. 잘못된 scope·오래된 digest·누락 target·미지원 mode는 명시 오류이고 기본 대상 추론은 하지 않는다.

등록된 프로젝트의 근거를 분리해서 가져오는 단계다. 산업 개념의 동등성 판정, 장단점 평가, 새로운 version-aware 쓰기, 실제 매니저 플러그인 연결은 이후 작업이다. 기본 도구가 제공하는 기존 단일 프로젝트 기능과 collection 조회를 구분한다.

## 현재 검증 범위

이전 기반 packet은17개 상위 시험/하위40사례의 fixture·race 검증을 보존한다. 이번 [공개 조회 packet](../../system/eval/knowledge-roadmap-2026-10-15/km-01-public-read/manifest.json)은 실제 CKG/CKV DB3개와 mock embedding으로 수행한24개 확인 항목, MCP 도구21호출/요청·응답26쌍, 정상 EOF 종료3회를 보존한다. 같은 이름·module·함수의 두 프로젝트, A의 두 버전, 교대/warm 조회, 명시 비교, 잘못된 scope7종과 시작 거부6종, 원문 손상·복원, pinned reindex 거부, 재시작과 기본33도구 호환을 검증했다. DB/manifest18파일과 current 포인터2개가 그대로임을 확인했다.

전체 모듈은3,619개 중3,600개 통과·19개 명시적 skip·실패0이다. 관련4패키지 race는444개 통과·2개 skip·실패0이며 기반40사례도 포함한다. 새 상위 시험6개/하위 포함14사례는 모두 통과했다. mock 검색의 인용/출처 격리 증거이며 실제 embedding 검색 품질·산업 비교·독립 수용을 통과시킨 것이 아니다. 전체 목록과 gate는 [시험표](./KNOWLEDGE-ROADMAP-TEST-MATRIX.md)를 따른다.

## KM-01.4 최종 검증 완료 기준

아래 공개 경로 반례를 최종52항목으로 실행하고 원시 MCP·HTTP·Git 원문·보존 기록을 재검증했다. [최종 packet](../../system/eval/knowledge-roadmap-2026-10-15/km-01-final-isolation/manifest.json)을 근거로 KM-01.4와 전체 KM-01의 로컬 격리를 완료했다.

1. 같은 이름·module·함수·소스 바이트·Git commit의 두 프로젝트를 실제 DB로 만들고 서로 다른 project ID와 인용 좌표를 유지한다.
2. A의 코드 변경 두 버전과 B의 동일 소스/서로 다른 embedding 두 버전을 동시에 등록한다. 64차원 mock과 로컬 시험 응답기의96/128차원 model/digest를 분리한다. 반복·비교·재시작에서 근거를 다른 프로젝트/버전으로 대체하거나 중복 제거하지 않는다.
3. 개별 조회가 선택한 member의 embedding 구성만 호출하는지 로컬 HTTP 원자료와 확인한다. model/digest 변동·잘못된 설정·중복 등록·부적절한 scope는 거부하고 정상 대상은 계속 읽힌다.
4. 공개 쓰기 표면 index(full/incremental), setup, reindex에 root/version/current/그 아래 새 경로를 지정한다. A/B 간 잘못된 대상도 포함해 job 미생성·DB/원문/current 불변을 확인한다. 새 version-aware 쓰기 자체는 KM-02의 별도 구현 범위다.
5. 실제 rollback과 재시작에서 catalog pin이 current를 따라 바뀌지 않고 이전/새 버전을 각각 읽는다. 기존 단일 프로젝트 read·legacy maintenance·GC/rollback 호환 회귀 근거도 연결한다.
6. 원시 MCP 요청/응답, embedding HTTP 기록, source/DB 보존 기록, 전체 시험 목록과 소스 해시를 저장하고 누락·변조·범위 과장을 거부하는 검사로 재검증한다.

실제 embedding 모델 품질, 산업 개념 매핑·장단점 비교와 독립 수용은 이 격리 완료 기준에 포함하지 않는다. 이번 시험 응답기는 구조와 출처 검증 전용이며 유료 모델 호출을 수행하지 않는다.

## 최종 격리 결과와 한계

같은 Git commit·전체 retained file hash·module·함수를 가진 A/B fork를 각각 실제 DB로 만들었다. A는 변경된 두 commit/버전, B는 같은 소스의96차원·128차원 모델 버전이다. 총4DB에서52항목·46도구호출·52요청/응답·정상 EOF3회를 확인했다. 개별 조회의 embedding HTTP 기록은 선택한 member의 모델만 호출했고,4개 참여 버전의 비교는 동일 원문도 별도 그룹으로 유지했다. DB/identity/manifest24파일과 current2개를 보존했다.

공개 CKS35도구의 변경 표면을 조사해 index(full/incremental), setup, reindex의18개 요청을 실행했다. A/B의 root/version/current/current 아래 새 경로를 모두 거부했고 job을 시작하지 않았으며 거부 전후의 파일·모드·디렉터리·symlink 전체 목록이 같다. GC dry-run은4개 pin 모두 `pinned_reader`로 보호했다. 실제 A rollback 후에도 reader는 고정한 새/이전 버전을 읽었고 재시작에서도 pin을 유지했다. 멤버 변경은 digest를 바꾸고 이전 digest와 제거된 멤버를 거부했다. version-aware 새 쓰기는 여전히 KM-02의 계약/구현 대상이다.

모델 digest가 변경되면 해당 member는 실패하고 부분 근거를 공개하지 않는다. 기존 Real CKV의 `modelDown`은 실패 후 유지되므로 원 digest를 되돌려도 reader 재시작이 필요하다. 다른 프로젝트와 같은 프로젝트의 다른 모델 member는 정상이다. 재시작 후 동일 원 digest의 조회 회복을 확인했다. 첫 실행의 즉시 회복 기대 실패는 `initial-run/`에 보존했고 구현을 바꾸거나 실패를 지우지 않았다.

embedding은64차원 mock과 loopback 결정론적 시험 응답기의96/128차원 프로토콜이다. 실제 모델을 실행하거나 품질을 평가한 결과가 아니다. 테스트에서는 전체 도구 목록을 조사하되 기존 단일 프로젝트 조회35개 전체가 collection scope를 받는다고 설명하지 않는다. 새 collection 조회2개가 scope를 받으며 standalone CKG/CKV·새 유지보수 API의 권한 확장은 별도 계약이다.

이번 단계는 Go 소스를 변경하지 않았다. 이전 전체 Go3,619개·관련 race446개와 build/fmt/lint 원자료는1,652개 소스의 동일 바이트를 다시 확인해 재사용했다. 문서와 최종 protocol·증거 검사만 새로 실행하며 실행한 시험과 재사용 근거를 [전체 시험표](./KNOWLEDGE-ROADMAP-TEST-MATRIX.md)에 구분한다.
