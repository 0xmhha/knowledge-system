# N12 검토 대기 ADR의 보관 lock 연결

2026-10-08 · **ADR 2개 기술 lock·동일 신원 연결 검증 완료 / N12 전체 미완료**.

원문·의미·코드·시험의 근거를 검토한 바이트에 고정하는 것이 목적이다. lock은 바이트 일치만 보증한다. 사람의 내용/관계·권위·범위 판정과 실제 운영 담당자 결정을 대신하지 않는다. 이전 [held 연결 준비](./N12-COORDINATED-PREPARATION.md)는 당시 overlay/lock이 없던 별도 후보로 보존했다.

## 범위와 설계

P12-P05/P06의 원문 SOURCE-SNAPSHOT-ADR에서 문제·결정·근거·구 대안과 원문 인용을 proposed ADR로 작성했다. 원문 61–65/67–69줄의 file/span SHA와 원본667a9af5·보관 소스562e37aa를 함께 기록했다. 날짜·검토자·이유는 비어 있고 requirement/trace 관계는 생성하지 않았다. 새 메타데이터의 해석과 제안 scope도 사람 검토 대상이다.

정책 P12-P01–04의 실제 운영 담당자는 알려지지 않았다. 제품 schema는 proposed 정책에도 owner를 요구한다. 임의 담당자나 unknown placeholder를 넣어 통과시키지 않고 정책 instance 0개를 유지했다. 이4개 주장과 SUPPORTS/ABOUT 관계의 기존 semantic 검토 자료는 그대로 남아 있다. pack owner의 `Codex preparation tooling`은 기술 초안 작성 주체이며 운영 정책 담당자나 사람 권위 승인이 아니다.

`local.n12@0.1.0`의 독립 `source-adr` 타입1개만 명시 선택했다. D1 공통20개를 재정의하지 않는다. 최소 승인1명 규칙을 유지하고 review record를 만들지 않았다. `knowledge lock`의 실제 raw overlay/pack digest를 source archive와 v4 KnowledgeProjection에 결합했다. v4 pack 투영은 타입/제약/lock을 나타내며 ADR instance나 기존12 semantic 관계를 verified로 만드는 기능이 아니다.

## 검증한 세부 단계

- [x] 원 소스1,410파일과 기존 검토 책·claim seeds·승인 Go29/입력5·제품 소스/의존1,155 바이트 불변 확인. 별도 준비 Git32d7e264에 overlay/pack6개만 추가하고 원본 전체 역사는 복사하지 않음.
- [x] 생산 knowledge init/digest/validate/lock/review 성공. ADR2/proposed/사람review0, 정책·knowledge relation·trace0, 원문 provenance와 빈 날짜 보존.
- [x] 새 생산 setup committed/strict complete embeddings/canonical floor1.0/hold-for-review 성공. repo1,416+knowledge origin1 = 보관1,417개 항목. 선택58파일 중 CKG53Go·CKV56Go/Markdown 입력은 기존 범위와 SHA 일치. overlay는 CKV 검색 입력으로 색인했다고 표시하지 않음.
- [x] retained API에서 동일 lock/ADR2를 재구성하고 policy/decision 선택 모두 unknown·applicable0 확인. 새 snapshot의 v4 의미 자료와 문서27/27·코드36참조/고유canonical33·원문span·CKV chunk 연결 검증.
- [x] proposed SQLite 저장/재조회 바이트 동일·semantic current 없음. 생산 `semantic build --include-packs --extract-only --min-canonical-ratio 1` 성공, store 파일 생성/활성화0. 이 CLI 실행은 ontology/spec+pack 추출이며 section/claim 연결은 별도 driver 검증이다.
- [x] overlay변경/미검토ADR verified/선택pack digest변경/quorum0/project변경/lock digest변조/owner없는정책 7종 실제 API 거부. owner없는 정책은 실제 CLI에서도 거부. 수정/제어 후 원 소스와 lock 복원·Git clean 확인.
- [x] source/history·CKG/CKV·hold·identity archive를 독립 위치로 복원. 원래 임시 소스/데이터셋/semantic DB 경로를 숨긴 상태에서도 retained lock·원문·canonical·CKV·v4 alignment 재검증 통과, 원본 복원. 모델 호출0.

| 최종 후보 | 값 |
|---|---|
| project_id | knowledge-system |
| source_commit | `32d7e2641640b82995cf443bd8a2553dd0ce69b9` |
| snapshot_id | `a0457cc1c51791dfdfbdaa3cd7272364bf9197f5aadbe968bd0701c40136bea5` |
| dataset_id | `5a24904fb0ad21bfd0bae7f6d05d0afef1e52de7c03ffa8f11c97be8966bda47` |
| proposed lock digest | `da964815fe34cbc6912db0cab703e933b349f7dbeda28a4db0037c44e3034a56` |
| CKG | 9,106nodes/21,148edges, parse/unresolved0 |
| CKV | 666chunks, 566/566canonical code symbols, floor1.0 |
| 사람 승인·운영 담당자·활동시간 | 0 / 미정 / null |

## 실제 비용·제약·오류 보존

승인 BGE-M3 digest790764642607…16bab/native/target1024·Ollama0.35.1·truncate:false를 유지했고 metadata 전후 동일했다. 이번 새 색인 실제 HTTP192, embedding HTTP62 중 시작 차원 probe2와 색인60회/666입력, embedding 입력 총668·실패0이다. 이전 후보들의 HTTP124/색인1,332는 역사적 별도 실행이며 이번 비용으로 덮어쓰지 않았다. 새 DEV/FINAL 검색, p95 평가, LLM 호출, 사람 활동 측정은 없다.

생산 전체 Go 감사는1,051 대53의 DRIFT/exit1·선택 밖998 missing/extra0를 유지했다. 선택 범위 일치와 전체 저장소 감사 PASS를 혼동하지 않는다. 새 제품 코드 변경이 없으므로 c85ec462의 전체 Go 회귀 근거를 참조하며 이번 전체 Go 회귀를 다시 실행했다고 쓰지 않는다.

처음 pack 초안이 존재하지 않는 core decision 타입을 extends해 거부됐고, 독립 local artifact로 고쳤다. 최초 schema-only CLI 호출의 필수 project/dataset/store 인자 누락도 거부돼 올바른 신원/경로로 수정했다. 원래 출력/원인을 별도 보관했다. 준비 도구 오류를 제품 실패·실제 반려·성공 거부 제어 개수로 합산하지 않는다.

[검증 원자료](../../system/eval/b0-knowledge-system/refactoring-n12-proposed-lock-2026-10-08/retained-knowledge-report.json), [원문/provenance와 담당자 공백](../../system/eval/b0-knowledge-system/refactoring-n12-proposed-lock-2026-10-08/overlay-provenance.json), [거부 제어](../../system/eval/b0-knowledge-system/refactoring-n12-proposed-lock-2026-10-08/negative-controls.json), [모델 계측](../../system/eval/b0-knowledge-system/refactoring-n12-proposed-lock-2026-10-08/model-call-accounting.json), [원본 경로 없이 재생](../../system/eval/b0-knowledge-system/refactoring-n12-proposed-lock-2026-10-08/archive-originals-hidden.json).

[held-candidate.tar.gz](../../system/eval/b0-knowledge-system/refactoring-n12-proposed-lock-2026-10-08/held-candidate.tar.gz)는12,568,768bytes/1,426files, SHA256 `0c4403f1d7701fd18ef598b16ac99ee51b7087bf26f18a47bdc2deebfbff0ad0`이다. 종료한 lifecycle locks·SQLite sidecars·초기 backup을 제외하고 권위 DB SHA를 확인했다. 논리 source root 좌표는 기존 절대 경로를 유지한다. 운영 이동이나 독립 소비자 수용을 입증한 자료가 아니다.

## 남은 판정과 순서

proposed_partial_knowledge_lock_built=true, approved_knowledge_lock_built=false, policy_instance_coverage=0/4, proposed_ADR_instance_coverage=2/2, human_approved=0이다. 기존6claims/12assertions·20concepts/3requirements/4criteria 모두 사람 판정 대기이며 N12-A–D를 체크하지 않는다. dataset current/semantic activation·공개 전송0이다.

다음은 N11 실제 gold·독립성·오염 판정과 N12의33항목/12관계 및 ADR metadata/입력 범위·권위·검토자 판정, 정책 담당자/시점 확인이다. 판정을 받아 새 review record/overlay/lock을 봉인해야 하며 현재 후보를 제자리 변경하거나 승인했다고 재해석하지 않는다.

| 우선순위 | 남은 전체 작업 | 완료에 필요한 것 |
|---|---|---|
| P1 | N11 독립 평가 | 실제 gold/군·오염 판정, 충분한 독립 표본과 사전 동결 |
| P1 | N12 실제 파일럿 검토 | 내용/관계·권위/범위·담당자/시점 판정, 승인된 overlay/lock 결합 |
| P2 | N08 검색 품질 | DEV 개선과 독립 FINAL 회귀 |
| P2 | N09 무답·기권 | 인용0와 실제 답변 기권의 별도 판정 |
| P2 | N10 비용·지연 | 동일8arm 실제 비용·오류 분모·warm/cold p95 |
| P3 | N14 실제 의미 수용 | 실제 patch/run·criterion별 사람 판정·승격/반려/rollback |
| P4 | N15 지원 환경·외부 소비자 | 규모/모델·독립 소비자 upgrade 실제 수용 |
| P4 | N16 운영 권한·고지 | 실제 역할·키 신뢰/교체/폐기·전이 고지 |
| P4 | N17 운영 복구 | 담당자/RTO/RPO와 실제 대상호스트 장애 drill |
| P5 | N18 최종 수용 | 전체 근거·제약·기본값/출시 판정 |

현재 전체8/18·요청5/15 완료, 잔여10개. 이 기술 세부 단계의 완료를 상위 항목 완료로 계산하지 않는다.
