# KM-04 지속·주기 갱신 계약

등록된 프로젝트의 최신 공개 원문을 자동 수집·색인하여 DB가 오래된 채 방치되는 문제를 해결한다. 매번 사람에게 문서나 승인 파일을 쓰도록 요구하지 않는다. 자동 관찰 입력은 원문과 기계 검증 결과이며, 사람에게 검토받은 정책·설계 이유와 구별한다. [상위 명세](./KNOWLEDGE-MANAGER-AND-COMPARISON-SPEC.md), [진행 원장](./KNOWLEDGE-ROADMAP-PROGRESS.json), [시험표](./KNOWLEDGE-ROADMAP-TEST-MATRIX.md)를 함께 갱신한다.

## 실행 범위와 단계

| 단계 | 결과 | 완료 증거 |
|---|---|---|
| KM-04.1 | 명시 정책·trigger·pin·성공/실패 계약 | 정책 검증·시각/성공 판정 반례와 문서 일치 |
| KM-04.2 | 선택 원문의 변경/삭제/no-op·자동 관찰 입력·검토 무효화 | 실제 변경 목록·filtered 입력·새 DB/거부 경로 |
| KM-04.3 | durable intent·한 writer·재시작/retry·상한·관측 | 중복/중단/불확실 접수·복구·자원 시험 |
| KM-04.4 | 실제 연속/주기 운영 | 두 번 변경·삭제·no-op·실패/복구·이전 조회·다른 프로젝트 불변 |

계약이나 unit 통과만으로 실제 주기 실행 또는 KM-04 전체를 완료로 표시하지 않는다. 플러그인 로드만으로 scheduler를 켜지 않는다. 개인 머신의 서비스·cron 설치나 유료 모델 정기 호출은 이 구현 범위에 포함하지 않는다. 명시 foreground worker가 실행 중일 때만 구성한 일정으로 동작한다. worker 종료/중단과 재실행 사이에 백그라운드 실행이 계속됐다고 주장하지 않는다.

## 정책·입력과 trigger

정책은 schema version, 정확한 project/profile, 등록 bundle digest에 결속한다. 알 수 없는 필드·타 프로젝트·등록 변경·잘못된 경로·무한 예산은 거부한다. 정책 원문 hash와 적용 시각을 소유 상태에 보존한다. 정책 설정은 자동화 범위의 선택이며 지식 내용에 대한 사람 승인/신원 인증을 의미하지 않는다.

| 설정 | 의미·상한 |
|---|---|
| enabled | 명시 boolean. false이면 수동 포함 모든 새 갱신 접수 중지; 기존 작업 상태/취소/복구는 유지 |
| triggers | manual / periodic / change의 중복 없는 선택. 모든 호출에 실제 trigger 기록 |
| interval_seconds | 1–604800초. periodic은 wall-clock 만기이며 change는 이 간격의 bounded polling; 실제 파일 이벤트 감시로 주장하지 않음 |
| activation | 기본 hold_for_review. activate_after_gate는 이 정책에서 명시 선택했을 때 관찰 입력의 실제 required gate를 통과한 후보에만 허용 |
| collection | 제외 패턴·명시 context file·파일/byte 상한. 기존 비공개·링크·비정상 UTF-8 거부/제외 계약을 재사용 |
| materials | 최대16개 명시 source 하위 corpus root. document/knowledge/ontology의 **원문 관찰 자료**로 수집; native semantic/domain 권위로 자동 채택하지 않음 |
| retry | 최대1–10회, 초기/최대 대기1–3600/86400초. 최댓값 이하 지수 backoff, 실행 중 busy에 새 요청 생성 금지 |
| deadline_seconds | 한 build1–3600초. native timeout/cancel과 결속 |

material은 등록 source의 실제 하위 디렉터리만 허용하고 source 전체·서로 겹치는 root·링크·dataset/state/엔진 입력을 거부한다. source 내부 `.cks/knowledge` 같은 명시 corpus는 기본 수집 제외를 우회하는 암묵 경로가 아니라 별도 origin과 제한을 가진 선택 입력이다. 원문을 읽었다는 사실로 ontology/domain 팩 의미 검증을 통과했다고 하지 않는다. 변경된 팩의 기존 verified 관계는 stale로 보고 새 해석/검토를 요구한다. 자연어 의미 해석·비교는 KM-05/06과 OE-01에서 별도로 검증한다.

manual은 명시 호출, periodic은 마지막으로 **끝난 확인** 이후 만기, change는 polling 때 계산한 입력 fingerprint 차이다. 시계를 뒤로 돌렸으면 즉시 반복하지 않고 기존 미래 만기를 유지하며 clock skew를 표시한다. 재시작 시 놓친 만기는 한 번만 합쳐 실행하고 누락 횟수만큼 job을 만들지 않는다. 첫 실행에는 기준선이 없음을 표시한다. configuration이나 enabled 변경을 실행이 시작된 과거 시점으로 소급하지 않는다.

## 변경 신원과 no-op

입력 fingerprint는 선택한 공개 repo 파일의 origin/path/hash, 명시 material과 제공 자료의 원문 hash, 제외/수집 정책, collector version, source Git 신원, 등록 config·sanitize·실행 파일 hash, embedding 설정·실제 inspect의 model identity, source/input/profile pin을 추적한다. 수집 시각·mtime만 바뀐 경우와 실제 byte 변경을 구별한다. Git commit이 같아도 문서·untracked source·제공 기록·material의 내용 변경을 놓치지 않는다. 등록 config/engine byte가 바뀌면 자동 재등록하지 않고 typed blocked 상태를 보고한다.

각 origin의 이전/현재 전체 선택 파일 집합에서 added/modified/deleted를 계산한다. 이름 변경은 hash가 같은 삭제/추가의 **rename 후보**로만 보고하고, 여러 동일 hash가 있으면 임의로 짝짓지 않는다. 삭제된 본문을 최신 자료로 다시 복사하지 않는다. 삭제된 자료를 참조한 기존 claim/review는 역사적 당시 기록으로 보존하고 현재 검증 승인으로 승계하지 않는다. 전체 제외 자료가 바뀐 경우 selected scope 밖임을 유지한다.

no-op은 마지막 성공 입력 fingerprint와 동일하고, 해당 후보/게시 버전과 실제 좌표·gate·기준 current가 여전히 유효할 때만 허용한다. 마지막 실패·취소·중단·불확실 접수는 성공 기준선이 아니다. held 후보를 no-op으로 재사용해도 상태는 held다. 외부 current 전환·후보 삭제/손상·model drift·검토 회수는 no-op을 무효화하거나 재조정을 요구한다. source 일부만 읽었거나 검증하지 못한 입력을 무변경이라고 표시하지 않는다.

증분 엔진을 구현한 것으로 주장하지 않는다. 변경 시 새 immutable version으로 전체 rebuild한다. 바뀌지 않은 입력의 재확인은 이전 성공을 덮거나 새 DB를 생성하지 않는다.

## 자동 관찰 입력과 검토

자동 입력은 선택된 원문의 소유 사본과 origin/hash/수집 범위·미해석 상태를 설명하는 기계 생성 지식이다. source 명령·제공 명령 결과는 데이터이며 실행하지 않는다. 사용자가 원래 문서를 만들지 않은 코드도 수집·색인한다. 원문에 없는 의도·정책·결정이나 reviewer 이름·human_response를 생성하지 않는다. 모든 관찰은 semantic_review=not_performed를 유지한다.

기존 수동 검토 workflow와 자동 관찰 workflow를 별도로 저장한다. 자동 경로에서 review.record/adopt를 호출하거나 controlled_fixture 리뷰를 만들어 사람 검토를 흉내내지 않는다. 기존 reviewed adoption의 source가 바뀌면 현재성 검사는 계속 거부한다. 자동 DB에는 검토된 의미를 무조건 복사하지 않는다. 이전 검토의 정확한 근거가 최신 입력에서 유지됐는지와 무관하게 의미 승계는 별도 명시 절차이며 이번 자동 관찰 경로에서는 수행하지 않는다.

## 작업 상태·중복·복구

등록별 한 writer와 broker/유지 보수 journal 소유 계약을 함께 지킨다. 기존 interactive broker와 worker가 겹치면 busy로 거부하고 lock/journal을 지우지 않는다. intent를 원자 저장/fsync한 뒤 native build를 호출한다. intent에는 정확한 정책 hash·immutable 입력·모든 inspect pin·base·version·key·deadline·activation이 들어간다. 접수 결과를 잃으면 같은 요청만 replay한다. 모르는 job을 새 key로 다시 접수하지 않는다.

상태는 idle / checking / building / held / succeeded / no_op / retry_wait / blocked / stopped를 구별한다. 마지막 확인 시각, 마지막 성공 자료/좌표, 현재/후보, 실제 native job, attempt·다음 만기, 변경/삭제 요약·stale 사유·오류와 불확실 intent를 보존한다. 메모리나 job 접수를 durable 성공으로 표시하지 않는다. 저장 불확실성이 생기면 해당 프로세스는 재조정 없이 계속 쓰지 않는다.

재시작은 기록된 config와 정확한 원래 요청으로 기존 job을 조회/replay한 뒤 새 source를 판단한다. native accepted job이 이미 terminal이면 원래 결과를 먼저 보존한다. 취소·실패 후 retry는 이전 intent와 연결한 새 version/key/attempt이며, 같은 실패 job을 재접수해 새 실행인 것처럼 세지 않는다. 입력이 그 사이 달라지면 이전 실패의 retry를 새 입력으로 바꾸지 않고 별도 cycle로 기록한다. writer/child quiescence를 확인하기 전 다음 실행을 시작하지 않는다.

프로세스 EOF·SIGINT/SIGTERM은 소유 child 종료와 durable cancellation을 기다린다. 실제 host 강제 종료 후 재실행은 native durable 기록과 관리 intent를 대조한다. native 오류를 정상 종료로 숨기지 않는다. 기본128개 내 pending/관측 기록과 명시 worker 시간/iteration 상한을 둔다. history가 차면 조용히 삭제하지 않고 정리/운영 한계를 보고한다. 보관/GC는 기존 read lease·rollback 계약을 따르며 자동으로 원천이나 이전 DB를 지우지 않는다.

## 성공과 전환 기준

| 관찰 | 판정 |
|---|---|
| 도구 접수/실행 중 | 성공 아님; 기준선 변경 금지 |
| held_for_review + build_complete + activated=false + 정확한 후보/required gate | 구축 성공/검토 대기; 게시 성공 아님 |
| succeeded + build_complete + activated=true + 정확한 후보/required gate + 실제 current 일치 | 게시 성공; 조회 확인은 별도 |
| 실제 pinned reader 응답·인용/원문·project/dataset 좌표 일치 | 해당 시점의 실제 조회 성공 |
| failed/canceled/deadline/blocked/unknown admission | 마지막 성공·기존 사용 가능 DB 보존, 원인/후속 상태 기록 |

configured gate를 통과하기 전 활성화하지 않는다. source/model/기준 current가 구축 중 변하면 stale 또는 base conflict로 새 활성화를 거부한다. 이전 snapshot은 당시 좌표로 조회할 수 있어야 하며 최신 current로 바꿔치기하지 않는다. 자동 갱신의 reader도 기존 reviewed-input reader를 암묵 변경하지 않고 명시 자동 상태/좌표로 선택한다.

## 필수 검증 목록

정책/ID/상한·수동/정기/변경/disabled·clock skew·성공 위조/누락 gate, byte와 Git 독립 변경·명시 corpus 변경·삭제/rename 후보·제외/비공개·검토 stale, no-op 손상/외부 current 변경, 중복 writer·접수 응답 손실·저장 불확실·EOF/kill·deadline·retry 상한·restart missed tick을 검증한다. 최종 실제 실행은 v1→내용 변경 v2→no-op→삭제/두 번째 변경 v3→의도한 실패→복구, 실제 periodic와 change polling, 보존 v1 조회와 다른 등록 DB 불변을 포함한다. mock embedding의 lifecycle 결과를 검색 품질/온톨로지 개선이나 운영 독립 수용으로 확대하지 않는다.


## KM-04.1 검증 기록

KM-04.1 완료: 명시 정책·수동/정기/변경 polling·clock skew/놓친 일정 합치기·등록/프로젝트/명시 corpus 범위·재시도 상한·required gate와 held/게시 성공 계약을27개 반례로 검증했다. 문서 없는 source의 자동 관찰 입력과 사람의 의미 검토를 분리하고 변경된 verified 지식의 무조건 승계를 금지했다. 이 단계는 설계/정책 판정이며 scheduler 실행·자동 DB 갱신 완료가 아니다. build/fmt/vet/lint·문서132개·증거 보호10개를 통과했다. Go/의존/fixture byte가 C와 같아3,820통과·18skip 및 race810통과·2skip의 committed 원자료를 참조했으며 새 Go 실행으로 세지 않는다.

[정책 계약 packet](../../system/eval/knowledge-roadmap-2026-10-15/km-04-contract/manifest.json)을 따른다. 전체13/28·기능3/7이다. 다음은 KM-04.2의 변경/삭제/no-op·자동 관찰 입력 구현이다. 남은15개는 KM-04.2–4, KM-05.1–4, KM-06.1–4, OE-01.1–4이며 정책 생성기·운영/독립 수용9개·선택 후속12개는 별도로 유지한다. 사용자가 멈추기 전까지 단계별 서명 커밋하며 계속한다.


## 변경·자동 관찰 입력 사용

정책 초안은 아래 명령의 JSON 결과로 시작한다. 기본 enabled=false·hold_for_review이며 이는 일정 설정이나 작업 접수가 아니다. 원하는 공개 범위와 상한을 명시한 정책 파일을 제공한다.

```sh
python3 plugins/knowledge-manager/scripts/manager.py update-policy-template \
  --registration /workspace/manager/acme/registration.json
python3 plugins/knowledge-manager/scripts/manager.py prepare-update \
  --registration /workspace/manager/acme/registration.json \
  --policy-file /workspace/manager-policy/acme.json \
  --previous-input <previous-input-id>
```

prepare-update는 선택 원문의 immutable 관찰 입력을 만들고 변경을 반환한다. 기존 입력이 없으면 previous-input을 생략한다. 생성된 source 사본과 기계 지식에 실제 CKS knowledge init/validate/lock/validate를 수행한다. 원래 프로젝트·review workflow·dataset은 변경하지 않는다. input_unchanged는 DB no-op 성공이 아니며 DB_health_or_noop_verified=false다. 실제 native inspect/status·현재 model/source/input pin·required gate와 정확한 reader 결과를 다시 확인한 뒤에만 worker가 no-op으로 사용할 수 있다.

명시 corpus의 마지막 파일이 삭제되면 새 입력에서 해당 원문을 제거한다. 이미 적용된 정책의 root 자체가 삭제된 경우만 worker가 이전 정책과 대조해 삭제로 추적하도록 허용한다. 최초/임의 정책에 없는 디렉터리를 자동 채택하지 않는다. 전체 원문이 사라져 수집할 source가 없으면 이전 DB를 보존하고 근거 부족으로 거부한다.


## KM-04.2 검증 기록

KM-04.2 완료: 자동 관찰 입력·선택 파일의 추가/수정/삭제·same-byte rename 후보·명시 knowledge/ontology 원문·제공 기록·검토 stale/no-op pin을 구현했다. 사용자 source에 문서를 쓰거나 review/human_response를 만들지 않고 native knowledge init/validate/lock/validate를 수행한다. 정책27·입력32·기존 manager103의162개와 실제 공개 MCP16항목·4개 held DB·82호출·11연결 EOF0·증거 보호14개를 검증했다. 실제 두 번 변경·삭제 후 최신 원문/인용과 이전 snapshot/타 프로젝트 DB 보존, 실제 inspect/status/reader가 있는 무변경 판정을 확인했다. 메타데이터/input equality만으로 DB no-op을 승인하지 않는다. 이 실행은 수동 검증이며 주기 worker 검증이 아니다. build/fmt/vet/lint·문서132개를 통과했고 Go1256개·의존/fixture byte가 C와 같아 원래3,820통과/18skip·race810통과/2skip을 참조했다. 새 Go 실행으로 세지 않는다.

[관찰 입력 packet](../../system/eval/knowledge-roadmap-2026-10-15/km-04-inputs/manifest.json)을 따른다. 전체14/28·기능3/7이며 다음은 KM-04.3의 durable intent·한 writer·재시작/retry·실제 worker와 gated 전환이다. 남은14개는 KM-04.3–4, KM-05.1–4, KM-06.1–4, OE-01.1–4다. 정책 생성기·운영/독립 수용9개·선택 후속12개는 별도 유지하며 멈추라는 요청 전까지 단계별 서명 커밋으로 계속한다.


## KM-04.3의 A/B/C 실행 구분

A는 native 유지 보수의 원문 재확인, B는 매니저의 durable worker·일정·중복/retry와 상태, C는 실제 주기/중단/복구 연결 검증으로 나눈다. A만 통과해도 KM-04.3 또는 전체 KM-04를 완료로 표시하지 않는다.

등록 profile의 선택적 source_check는 command(argv), pinned_files(절대 canonical 파일→SHA-256), timeout_seconds(1–60)를 갖는다. operator가 소유한 설정이며 도구 요청으로 실행 파일/인자를 선택할 수 없다. 기본 nil이면 기존 profile digest·도구 목록·동작을 유지한다. argv 최대32개·고정 파일64개, 파일당128MiB·전체256MiB를 제한하고 링크/하드링크·byte 변경·명령 실패·deadline을 거부한다. shell 문자열로 재해석하지 않으며 stdout/stderr는 보관하지 않는다. 실행 전후 파일 pin을 확인하고 기존 guardian의 owner socket·process group·deadline 감독을 사용한다.

source_check는 native inspect/admission, 후보 완료 확인과 pointer lock 안의 최종 게시 직전에 실행한다. 매니저는 실제 Python interpreter와 plugin 코드·소유 immutable 정책/등록·입력 manifest/capture를 결속하고 verify-update-source로 원래 source/material/provided fingerprint를 다시 계산한다. build가 사용하는 불변 사본만 확인하고 바뀐 원래 source를 놓치는 문제를 막는다. 이는 확인 시점의 원문 신원 검사이며 의미 승인/수용이나 파일 시스템 전체를 원자적으로 잠그는 보장은 아니다.

이 원문 전용 probe만 DB 소유 검사를 native에 맡긴다. native 접수는 root owner가 기록되기 전에 소유 journal을 만들 수 있기 때문이다. probe는 등록/config/엔진·입력 owner/hash와 원래 source를 읽고 확인하며 DB에 쓰거나 소유권을 채택하지 않는다. 기본 등록/doctor·prepare-update와 모든 DB 쓰기는 기존 소유 검사를 유지한다. 이미 접수된 작업의 exact replay/status는 source check의 현재 결과와 분리해 과거 결과를 재현한다.


## KM-04.3 A 검증 기록

KM-04.3 A 원문 전환 전 확인 완료: operator profile의 선택적 source_check로 실제 Python/코드·소유 정책/등록/불변 입력을 고정하고 admission/후보 완료/게시 pointer lock 안에서 원래 source fingerprint를 재확인한다. 실제 MCP14항목·90호출·3개 job에서 v1 게시→v2 원문 변경으로 게시 거부/current·기존 reader 보존→v3 복구 게시/조회와5연결 EOF0을 확인했다. Go 전체3,862사례(3,844통과·18skip), 관련6패키지 race836(834통과·2skip)을 새로 실행했고 Python174개·증거 보호14개·build/fmt/vet/lint·문서132개를 통과했다. 추가 source-check23사례와 이미 지난 deadline1사례는 전체/race 분모 안에 포함하며 중복 합산하지 않는다.

처음에는 원문 probe가 native 접수 중의 아직 root owner가 없는 소유 journal과 충돌했다. 원문 전용 probe만 읽기 확인과 DB 소유 검증을 분리했고 기본 등록·doctor·prepare·DB 쓰기의 소유 검증은 유지했다. Native 오류의 실제 structuredContent를 읽도록 검증기도 수정했다. 최초 race의 기존 deadline 시험1회 late-writer 실패는5회 반복에서 재현되지 않았다. timer callback 도착만 가정하지 않도록 exec 직전/직후 절대 시각 확인을 보강하고 과거 deadline 거부 시험을 추가했다. 두 경계를 각각5회 확인한 뒤 최종 전체·race를 순차 실행해 실패0을 확인했다. 최초 실패의 단일 원인을 입증했다고 주장하지 않으며 initial-failures의 원자료를 보존한다.

[원문 확인 packet](../../system/eval/knowledge-roadmap-2026-10-15/km-04-source-check/manifest.json)을 따른다. 현재 소스1,707입력=Go1,258+기타449이며 승인5개는 보존했다. 직전2c03322f의 입력 검증을 동결 snapshot에서 별도 확인했다. A는 native 원문 검사 범위다. KM-04.3의 B durable worker/일정/중복/retry와 C 중단/복구 연결, KM-04.4 실제 주기 실행은 남는다. 전체14/28·기능3/7을 유지한다. 남은14개는 KM-04.3–4, KM-05.1–4, KM-06.1–4, OE-01.1–4이며 정책 생성기·운영/독립 수용9개·선택 후속12개는 별도로 유지한다. 사용자가 멈추기 전까지 단계별 서명 커밋하며 다음 B를 진행한다.
