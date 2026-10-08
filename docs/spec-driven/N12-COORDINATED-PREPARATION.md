# N12 동일 신원의 색인·의미 근거 연결 준비

2026-10-08 최신 하위 단계: **N12 기능 version2의 새 실제 source/CKG·CKV/관계15 준비 검증**. [증거·기준별 누락](./N12-FUNCTIONAL-V2-TRACE-PREPARATION.md) — source1,437/CKG62·CKV65/742chunks·621canonical,15proposed 관계/11canonical/74span/27문서,related race7시험24하위시험·5거부·원 source/dataset/store 없이 archive 재생 통과. 전체Go DRIFT992는 유지했다. 개별 판정0/spec_unapproved3/활성0/새FINAL0, 전체8/18·요청5/15·남은10개 유지. 다음 N12 actual foreign-code-anchor 승격·active 보존 통합 시험 보강과 N11 gold·독립성/실제 검토자 판정. 남은 전체 N08/09/10/11/12/14/15/16/17/18. 이전 후보·승인input5는 보존하며 새 관계를 독립 FINAL로 세지 않는다.

2026-10-08 · **검토 대기 후보의 기술 연결 검증 완료 / N12 전체 미완료**

실제 보관한 소스에서 CKG·CKV·문서·canonical 근거를 같은 신원으로 결합했다. 생산 setup의 `hold-for-review`로 후보를 보관하고 proposed 의미 자료만 저장했다. 실제 내용/관계·검토자·활동 시간은 미판정이며 knowledge overlay/lock은 없다. N12-A–D와 전체8/18·요청5/15·잔여10개는 그대로다.

## 입력·실행 신원

원본 코드 기준667a9af5, 실행 CLI 소스3e204ee3은 같은 제품 소스 바이트다. Go1.26.8 Darwin ARM64에서 현행 CKS·CKG·CKV3개를 빌드하고 각 binary SHA를 보관했다. 소스는 이전 archive에서 복원한 준비 커밋 `562e37aa1c9680d283de85ccd6435e5f235c21d6`의1,410파일(11,985,941 bytes)이며 원본 전체 역사를 포함하지 않는다. 원래 ontology의 project ID `knowledge-system`을 유지했다.

| 최종 held 후보 | 값 |
|---|---|
| project_id | `knowledge-system` |
| snapshot_id | `40e20d549d5ad5f3ba2a2b843772010bb091b2ceb9706ea124fba5b4e912f952` |
| dataset_id | `cce2c1f8f5de6796be3a29cbb5c505b7acc2701b3f65a7d5493ab3e50eb3bf92` |
| source_mode / commit | committed / `562e37aa1c9680d283de85ccd6435e5f235c21d6` |
| 모델 | 승인 BGE-M3 digest `7907646426070047a77226ac3e684fbbe8410524f7b4a74d02837e43f2146bab`, native/target1024 |
| runtime | Ollama0.35.1, metadata 전후 동일 |
| 빌드 게이트 | strict complete embedding, canonical floor1.0, hold-for-review |

생산 filelist는 pkg/graph/types의 pinned Darwin ARM64 Go 의존·시험과 기존69검토참조의20Go파일·승인Go29·원문5문서를 합쳐58파일을 선택했다. 실제 CKG는 Go53파일을, CKV는 같은Go53+Markdown3개를 색인했다. ontology/spec YAML2개는 source archive에 보관해 의미 추출에 사용하며 vectorized로 표시하지 않는다. 실제 엔진 파일 집합과 filelist의 각 지원 언어 집합은 누락/초과0, source SHA도 일치한다. 전체 저장소를 색인했다고 주장하지 않는다.

## 완료한 기술 세부 단계

- [x] 현행 CLI3개·binary SHA와 승인 모델 digest/native1024·runtime·build 입력 신원 확인.
- [x] 생산 filelist가 clean 준비 커밋과 필수20Go·승인Go29·원문5문서에 결합; CKG53/CKV56 파일의 정확 범위/SHA 대조.
- [x] 생산 setup의 strict complete embedding·canonical floor1.0·hold 게이트 통과. 최종 CKG9,106nodes/21,148edges, parse errors/unresolved refs0; CKV666chunks·566/566 코드 심벌 canonical 연결, source chunk truncated0.
- [x] 보관 원문에서 새 snapshot의 proposed projection 재추출. 문서27/27구간의 CKV chunk와36코드 참조/고유canonical33·원문SHA를 결합.20concepts/3requirements/4기준·6claims/12assertions·96evidence를 검증하고 proposed SQLite 저장·재조회 동일성 확인.
- [x] 다른 snapshot/path/AST/canonical/kind·spanSHA·CKV chunk·source root·vector dataset·미검토 부모의 verified assertion·누락 overlay/lock·검토 없는 후보 승격,12종 거부 제어 통과. 마지막2개는 실제 CLI에서도 exit1로 거부. dataset current와 semantic current는 없다.
- [x] 실제 held source/index archive의 독립 위치 재생에서 소스/데이터셋 신원·보관span·canonical·CKV 링크·floor1.0 재검증. 원자료와 모델 호출·제약·승인 불변 확인을 명세/원장에 결합.

36개 canonical evidence를 붙이는 작업은 IMPLEMENTED_BY/TESTED_BY/CHECKED_BY 관계를 생성하지 않는다. 원래12개 SUPPORTS/ABOUT 주장도 proposed이며 사람 판정·reviewed precision은0/null이다. 검토에서 승인되기 전까지 코드 연결을 의미 수용으로 계산하지 않는다.

## 원래 경고·거부와 수정 기록

첫 후보는 준비용 project ID `knowledge-system-n12-proposed-source`로 source/CKG/CKV의 구조 검증에 통과했으나 실제 ontology 추출이 project mismatch로 거부됐다. 원문을 고치지 않고 `knowledge-system`으로 별도 후보를 다시 생성했다. 첫 후보·실행·모델 호출과 거부 자료는 `initial-*`로 보존한다. 두 실행 모두 floor1.0이며 기준을 낮추지 않았다. 이전 소스 캡처 문서의 준비용 snapshotcef82547은 별도 프로젝트 신원이고 최종 의미 자료의 snapshot40e20d54로 자동 덮어쓰지 않았다.

범위 없는 생산 `ckg audit`는 전체 Go 빌드1,051파일을 DB53개와 비교해998개 missing/extra0, exit1/DRIFT를 반환했다.998개 모두 선택 Go 범위 밖이다. setup은 이 감사를 기존 soft warning으로 기록했고 hold는 성립했다. 이 경고를 PASS로 바꾸지 않았으며, 별도 선택 범위53↔53 검증과 구별한다. 전체 Go 빌드 범위의 parity 또는 전체 저장소 검색 품질을 입증한 자료가 아니다.

감사 CLI를 처음 `--db`로 호출한 argument 오류와 추가 제어 반복의 고정 foreign-vector 디렉터리 충돌은 준비 도구 오류로 별도 보존했다. 각각 올바른 `--graph`와 독립 임시 fixture 경로로 수정했다. 제품 실패·완료 제어 개수에 합산하지 않았다. 제품 소스와 캡처 보호·원래 승인 입력 변경0이다.

## 실제 모델 호출·보관 증거

| 단계 | embedding HTTP | 색인 입력 | 시작 차원 확인 입력 | 오류 |
|---|---:|---:|---:|---:|
| 첫 project mismatch 후보 |62|666|2|0|
| 원문 project로 수정한 후보 |62|666|2|0|
| 합계 |124|1,332|4|0|

로컬 transparent relay의 실제 method/path·상태·bytes·입력/출력수·요청/응답SHA·latency를 기록했다. 입력 본문과 vector는 HTTP 관측 로그에 저장하지 않았다. 소스 adapter의 `dimension probe` 요청 SHA에 맞는 각2호출을 분리했으며 전체 HTTP384회는 metadata 호출을 포함한다. embedding 입력 전체1,336개가 성공했고 `truncate:false`다. 이는 색인 준비 비용이며 DEV/FINAL 검색·warm/cold 질의 지연·p95 평가나 사람의 활동 시간으로 사용하지 않는다.

[모델 계측](../../system/eval/b0-knowledge-system/refactoring-n12-coordinated-preparation-2026-10-08/model-call-accounting.json), [범위 대조](../../system/eval/b0-knowledge-system/refactoring-n12-coordinated-preparation-2026-10-08/index-scope-check.json), [proposed 자료](../../system/eval/b0-knowledge-system/refactoring-n12-coordinated-preparation-2026-10-08/projection-proposed.json), [검토 큐](../../system/eval/b0-knowledge-system/refactoring-n12-coordinated-preparation-2026-10-08/review-queue.json), [거부 제어](../../system/eval/b0-knowledge-system/refactoring-n12-coordinated-preparation-2026-10-08/negative-controls.json).

실제 source blobs/history·CKG/CKV DB·신원·hold·filelist·footprint를 [held-candidate.tar.gz](../../system/eval/b0-knowledge-system/refactoring-n12-coordinated-preparation-2026-10-08/held-candidate.tar.gz)에 보관했다(12,569,317 bytes,1,420files). 종료된 lifecycle lock·초기 backup·zero-length SQLite WAL/shared-memory sidecar를 제외했고 권위 DB SHA를 확인했다. 새 위치에 풀어 생산 검증기로 재생했으며 새 모델 호출0이다. source root의 논리 좌표는 원래 manifest의 절대 경로로 검증했다. 이번 로컬 archive 검증을 운영 이동·독립 소비자 지원 수용으로 계산하지 않는다.

## 남은 판정과 다음 작업

이 단계에서 technical_CKG_CKV_source_alignment_built=true, approved_coordinated_dataset_complete=false, knowledge_lock_built=false, human_approved=0, actual_human_review_time=null, current/semantic_activation0이다. 원래69참조·검토 책·claim seeds·승인 Go29와5개 승인 입력의 바이트를 보존했다. 제품 코드가 바뀌지 않아 이전 전체 Go 회귀를 참조하며 이번에 전체 Go 시험을 재실행했다고 주장하지 않는다.

다음은 N11 gold·독립성과 N12의 실제 내용/관계·입력 범위·검토자 판정을 반영한 knowledge overlay/lock 결합, N14의 실제 patch/run 및 criterion별 수용이다. 전체8/18·요청5/15 완료, 남은 전체10개: N08 검색 품질, N09 무답·기권, N10 비용·지연, N11 독립 평가, N12 실제 파일럿 검토, N14 실제 의미 수용, N15 지원 환경·외부 소비자, N16 운영 권한·고지, N17 운영 복구, N18 최종 수용·출시 판정.

2026-10-08 N12 ADR 기술 lock 연결: [검토 대기 ADR의 보관 lock](./N12-PROPOSED-LOCK-PREPARATION.md) — 원소스1,410 불변, proposed ADR2/정책0(운영 owner 미정)/review0의 partial overlay·lock과 별도 held snapshota0457cc1/dataset5a24904f를 생성했다. 생산 knowledge·include-packs CLI/v4 의미27문서·36코드참조/33canonical·7 lock 거부·원 소스/색인/semantic DB 없이 archive 재생 통과. 새 BGE embed HTTP62=probe2+색인60/666입력, 전체Go DRIFT998 유지. 승인 lock/사람 내용·관계·운영 판정은 미완료, N12-A–D unchecked·전체8/18·요청5/15·잔여10개 유지. 다음 N11 실제 gold/독립성 및 N12 판정/정책 담당자·시점 확인.
