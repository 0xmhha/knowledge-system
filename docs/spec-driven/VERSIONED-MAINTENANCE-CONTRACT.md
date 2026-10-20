# KM-02.1 버전 DB 구축·갱신 계약

2026-10-15 기준의 설계·적합성 계약이다. KM-02.2의 현재 공개 연결·설정·단계별 한계는 [실행 명세](./VERSIONED-MAINTENANCE-RUNTIME.md)를 따른다. 아래 capability 조사는 KM-02.1 완료 당시 관찰이며 현재 공개 도구 수를 뜻하지 않는다. [전체 명세](./KNOWLEDGE-MANAGER-AND-COMPARISON-SPEC.md)의 KM-02.1을 구체화한다. [기계 판독 스키마](../../system/contracts/versioned-maintenance/v1/schema.json), [상태 전이](../../system/contracts/versioned-maintenance/v1/state-machine.json), [capability 조사](../../system/contracts/versioned-maintenance/v1/capabilities.json), [101개 사례](../../system/contracts/versioned-maintenance/v1/fixtures.json)를 함께 관리한다. 공개 MCP 실행은 KM-02.2, 지속성·중복/장애 복구는 .3, 공개 실패 반례는 .4에 남는다.

## 목적과 현재 capability

매니저가 작업 접수 응답을 DB 완성으로 오인하지 않고, 지정한 프로젝트의 새 버전이 어떤 소스·설정·모델로 구축됐으며 검증/검토 대기/활성화 중 어디까지 진행했는지 판단하게 한다. 기존 pinned 버전을 직접 수정하지 않는다.

| 항목 | 현행 동작 | 새 계약에서 필요한 처리 |
|---|---|---|
| MCP index/setup/reindex | legacy 대상. pinned/versioned 경로는 명시 거부 | 기존 거부 유지. 별도 version-aware 도구에서 새 후보만 구축 |
| MCP setup_status | running/done/failed, process memory | scoped job ID·후보/활성화 증거·명확한 취소/중단 상태 |
| 작업 취소 | 내부 `Jobs.Cancel` 존재, 공개 도구 없음. 취소는 현재 failed로 끝남 | 공개 scoped 취소와 cancel_requested/canceled 구분 |
| 종료·재시작 | Shutdown은 작업 context 취소. 메모리 registry는 재시작 시 사라짐 | durable journal·idempotency 재조회·publication 복구 |
| version CLI | project/source/input/embedding pin, 새 버전 gate·hold·활성화·rollback 제공 | capture/Reindex/Gate/reader lease와 기존 변경 lock 재사용 |
| serving | 시작 시 버전과 collection을 고정 | current 전환과 조회 연결의 새 버전 채택을 분리 |

KM-02.1 당시 MCP35도구와 CLI help를 읽기 전용으로 확인했다. 당시에는 새 versioned build/cancel 도구를 노출하지 않았다. 논리 operation은 `inspect_profile`, `build`, `status`, `cancel`로 고정하며 공개 이름/schema는 KM-02.2에서 `cks.dataset.inspect_profile/build/status/cancel`로 등록했다. 기존 도구명이나 기본 도구 수를 설계 문서만으로 바꾸지 않는다.

## 프로젝트 profile과 입력 결속

호스트에 등록한 profile은 project ID, 허용 source/origin·output root, 정규화된 setup 설정, 실행 파일/모델 정책, capture 정책·자원 상한, 필수 gate와 활성화 정책을 소유한다. 같은 표시 이름·URL·함수·commit이어도 project ID를 합치지 않는다. caller는 profile ID를 선택하며 경로·endpoint·실행 명령·DB 파일·skip-vector를 요청으로 덮어쓰지 못한다. profile 등록/설치는 KM-03의 매니저 절차에 연결한다.

`inspect_profile`은 실제 등록된 profile을 읽고 다음 구축에 사용할 profile/input/embedding digest와 source pin, 현재 버전의 전체 좌표 및 capability를 반환한다. 현재 공개 구현의 process-lifetime 제한은 실행 명세에 명시한다. source 수집은 등록 범위에 한정하고 개인 세션/개인 로그를 기본 입력으로 읽지 않는다. preview 후 source가 바뀌면 build에서 거부하고 다시 inspect한다.

| build 필드 | 의미 |
|---|---|
| schema_version | 1 |
| project_id / profile_id | 명시한 프로젝트·호스트 설정 신원 |
| profile_digest | 실행/수집/활성화 정책과 설정의 정규화 digest |
| source_pin | mode, snapshot_id, base_commit. committed는 전체 commit, snapshot-only는 빈 commit |
| input_digest | 원문/지식/semantic/domain 등 등록 입력의 역할별 바이트 digest |
| embedding_digest | provider/model/tag/digest/dim/query 변환 등을 포함한 실제 embedding identity digest |
| base | 첫 구축은 state=absent, 갱신은 state=pinned+version+전체 V2 좌표 |
| new_version | 새 단일 경로 요소. current(대소문자 포함), 경로 탈출·기존 버전 충돌 거부 |
| activation | hold_for_review 또는 activate_after_gate. profile이 허용한 정책만 선택 |
| idempotency_key | 16–128자. project/profile 안에서 동일 요청을 재식별 |
| deadline_seconds | 10–86,400초 범위이며 profile 상한 이하. 작업 전체 capture/build/gate에 적용 |

모든 필드는 명시적으로 제공한다. JSON은64KiB 이하, 중복 key·unknown·후행 값·비유한 수와 정수가 아닌 숫자 표기를 거부한다. canonical JSON의 key 순서와 공백은 의미가 없으며 정수는 정수로 파싱한다. request fingerprint는 domain prefix와 정렬된 전체 요청을 결속한다. 모델·config·입력 변경도 새 요청이며 같은 idempotency key를 재사용해 다른 구축으로 바꿀 수 없다.

처리 순서는 strict decode → project/profile의 현재 접근 범위 확인 → scoped key의 durable replay/conflict 판정 → 새 작업에 대한 profile/source/input/model/base/version 확인 → 동일 dataset writer lock 안의 최종 재확인이다. 같은 요청의 replay는 같은 job을 반환하며 source/current가 이후 바뀌었다고 새 구축을 시작하지 않는다. 같은 key의 다른 fingerprint는 idempotency_conflict다. 실패/취소 후 의도한 새 실행은 새 key를 사용한다. durable 저장·lock·재시도 구현은 KM-02.3에서 검증한다.

## 접수·완료·활성화 상태

job reference는 schema_version/project_id/profile_id/job_id를 모두 받는다. 다른 프로젝트·profile의 job은 job_unknown이며 job ID만으로 우회하지 않는다. status에는 요청 digest, target version, 증가하는 revision, state/phase, terminal/build_complete/activated, 후보 좌표·입력/모델 digest, gate별 결과와 publication receipt를 포함한다. 응답에 filesystem path·raw subprocess 출력·임의 오류 문자열을 넣지 않는다.

| 상태 | 종료 상태 | 의미 |
|---|---|---|
| queued | 아니오 | 요청과 scoped key를 기록한 접수. 후보/완료 없음 |
| running | 아니오 | capture/build/gate/publish 진행 |
| cancel_requested | 아니오 | 취소 접수. subprocess 종료·정리 확인은 아직 |
| recovering | 아니오 | 재시작 후 journal/current/receipt 정합 복구. 성공/취소를 추정하지 않음 |
| held_for_review | 예 | 필수 gate 통과한 후보. build_complete=true, activated=false |
| succeeded | 예 | gate 통과와 실제 publication receipt 확인. build_complete=true, activated=true |
| failed | 예 | 원인 code 보존. 검증까지 끝났어도 활성화 실패는 성공이 아님 |
| canceled | 예 | 실제 실행 중단·정리 확인. publication 없음 |
| interrupted | 예 | 중단을 확인하고 publication이 없음을 판정. 복구 중과 구분 |

phase는 accepted/capture/build_graph/build_vector/build_knowledge/build_semantic/gate/publish/reconcile/finished다. queued는 accepted이고 recovering은 reconcile이다. 종료 상태는 finished다. 상태 전이표의 terminal을 다시 running으로 바꾸지 않으며 별도 재실행은 새 job이다. 같은 상태의 polling은 허용한다. 관찰 revision은 역행할 수 없고 같은 revision의 본문·job 신원은 같아야 한다. terminal snapshot은 불변이며 늦게 온 응답으로 진행 상태를 되돌리지 않는다. cancel_requested에서 succeeded로의 전이는 허용하지 않는다.

필수 gate는 source·identity·graph·vector이며 지식/semantic 입력이 설정되면 그 gate도 필수다. 결과는 not_run/passed/failed/not_configured로 구분한다. 필수 gate를 not_configured로 표시할 수 없고, build_complete=true에는 모든 구성된 gate의 passed와 정확한 후보 좌표·source/input/model 결속이 필요하다. 실제 실행은 기존 source capture, InspectVersionIdentity, ConfiguredInputDigest, Gate 경로에 연결하며 새 계약 검증기의 통과를 실제 DB 검증으로 대체하지 않는다.

held는 후보 검증 완료로 종료하며 자동으로 current를 바꾸지 않는다. 새 검토/채택 행위는 기존 review release 근거를 사용하고 reviewer·승인을 만들지 않는다. held job을 cancel해서 후보를 삭제하거나 승인된 것으로 바꾸지 않는다.

succeeded의 publication receipt는 기대 base, 활성화한 version/전체 좌표, publication 시각과 별도 digest를 결속한다. dataset 변경 lock 아래에서 expected base를 다시 비교하고 pointer swap의 crash 복구를 확인한 뒤 발행한다. swap 후 영수증 기록 전 장애는 recovering에서 reconcile한다. 이미 publication이 일어난 것을 failed/canceled로 숨기지 않는다. 다른 writer가 current를 먼저 바꿨다면 base_stale로 거부하고 그 변경을 되돌리지 않는다.

receipt는 **그 작업의 전환 이력**이다. 이후 다른 작업의 정상 전환으로 current가 다시 바뀔 수 있으므로 status의 성공을 영구적인 현재 활성 상태로 해석하지 않는다. 실시간 current와 과거 receipt는 별도 관찰이다. serving_adoption은 이 계약에서 not_observed로 고정한다. 매니저는 별도 catalog 갱신/reader 재연결 후 반환 좌표로 read probe를 실행해야 실제 조회 반영을 확인할 수 있다.

## 취소·오류·중단 정책

queued/running이고 publication 전이면 accepted, 이미 취소 중이면 already_requested다. recovering은 reconcile_required로 먼저 정합을 판정한다. publication 이후 또는 succeeded면 too_late이며 자동 rollback하지 않는다. 그 외 종료 job은 already_terminal이다. accepted는 취소가 끝났다는 뜻이 아니며 canceled status를 기다린다. publication 직전 취소 판정은 같은 writer lock·commit 경계에서 순서를 정해 accepted 후 성공으로 뒤집히지 않게 한다.

공개 오류는 고정 code/retryable/phase다. profile_unknown/profile_stale/project_mismatch/source_stale/input_stale/embedding_stale/base_stale/version_exists/idempotency_conflict/writer_busy/unsupported_capability/job_unknown/cancel_too_late/reconcile_required/gate_failed/deadline_exceeded/canceled/interrupted/operation_failed 등을 구분한다. 오류 문자열을 분석해 성공 여부를 추정하지 않는다. 일시적인 writer_busy와 inspect/설정 갱신이 필요한 stale, 완료된 job을 새 key로 다시 실행해야 하는 실패를 구분한다.

실패·취소·timeout은 이 작업이 새 current를 발행하지 않도록 한다. 다른 writer의 변경은 보존한다. 후보 진단은 project/version별로 격리하며 기존 source/DB·reader pin·보호된 rollback 버전을 삭제하지 않는다. 기존 legacy 도구의 pinned 쓰기 거부와 reader lease/GC 규칙을 유지한다.

## 구현 이행·완료 기준

| 단계 | 결과물 | 남은 검증 |
|---|---|---|
| KM-02.1 | capability 차이·strict schema·상태/오류/취소/CAS/idempotency 설계,101사례 적합성 | 문서·소스·원자료 결속 |
| KM-02.2 | host profile 연결·공개 inspect/build/status/cancel·새 후보·gate·hold/활성화 | 소유한 실제 DB의 공개 성공/보류/취소·기본 도구 호환 |
| KM-02.3 | journal·scoped replay·writer lock·commit 경계·재시작/rollback/read lease | 중복·timeout·부분 실패·publication 전후 crash·복구 |
| KM-02.4 | 공개 실패 반례·오류/좌표/기존 current 보존 | 다른 프로젝트/버전·한쪽 engine 실패·입력 변경·취소/종료 |

101사례는 설계 적합성 시험이다. 실제 비동기 작업·durable job·MCP 취소·자동 전환을 구현하거나 검증한 것으로 세지 않는다. 전체 진행 단계와 시험 목록은 [작업·시험표](./KNOWLEDGE-ROADMAP-TEST-MATRIX.md), 기계 상태는 [원장](./KNOWLEDGE-ROADMAP-PROGRESS.json)을 따른다.
