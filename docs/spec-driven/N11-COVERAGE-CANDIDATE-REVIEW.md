# N11 POLICY·TRACE·ABS 후보와 현행 근거 재결합

2026-10-08 · **부분 후보 준비 / 미승인·미실행 / N11 전체 미완료** · [새 후보 책](../../system/eval/b0-knowledge-system/refactoring-n11-coverage-candidates-2026-10-08/candidate-question-book.json) · [대조 결과](../../system/eval/b0-knowledge-system/refactoring-n11-coverage-candidates-2026-10-08/summary.json)

N11 다음 단계로 비어 있던 POLICY·TRACE·ABS 후보를7개 작성했다. 이전CODE/WHY4개와 합쳐 작성11개 중, 이미 관측된 사실군의2개를 제외해 **검토 대상9개/제안군6개**가 남는다. 신규 검토 대상은 CAND07–11의5개이며 CAND01–04는 이전 후보를 보존/재결합했다. 승인된 독립 FINAL은0개, 새 model/FINAL 질의·예약0개다.

## 완료한 준비와 현재 범위

| 중요군 | 책에 남은 후보 행 | 제안 사실군 | 승인된 독립 FINAL |
|---|---|---|---|
| CODE | 2 | 2 | 0 |
| WHY | 2 | 2 | 0 |
| POLICY | 1 | 1 | 0 |
| TRACE | 2 | 2 | 0 |
| ABS | 2 | 1 | 0 |

CODE/WHY 쌍은 같은 사실군을 공유한다. 두 ABS도 실제 운영 복구 미지정이라는 한 군으로 보수적으로 묶었다. 총행을 독립 표본으로 세지 않는다. TRACE 두 항목은 구현의 stale 진단/ADR-link 검증 질문이며, 실제 검토된 ADR→요구→기준→code→test 경로와 같은 유형으로 인정할지 사람이 판정해야 한다. 이 작성만으로 모든 군의 실제 gold/10독립단위 조건을 충족했다고 주장하지 않는다.

## 현행 소스 좌표의 실제 불일치와 교정

source commit `c8d86be7c03bed03fa933b75f5c175c2a5c39137`에 기존11문항의 원문을 대조했다. N11-DEV-04는 reader 복구 변경 뒤 gc.go의 파일SHA와 위치가 달라졌다. 과거254–428행은 현행265–439행으로 이동했으며 **원문 span 바이트/SHA 자체는 동일**했다. 원본 span이 현행 파일에 정확히1회 존재하고 전체 행 경계에 놓인 것을 확인한 뒤 새 책에 위치/fileSHA를 기록했다. 다른10문항의 파일/위치는 동일했다.

[재결합 원장](../../system/eval/b0-knowledge-system/refactoring-n11-coverage-candidates-2026-10-08/source-rebinding.json)은 기존 source/ref와 새 source/ref·동일 본문 여부를 모두 보존한다. 기존 DEV JSON/부분 책·프로토콜·승인 입력5개·원 FINAL은 수정하지 않았다. 과거 판정을 새 source/책/전체 동결의 승인으로 자동 확장하지 않는다. 원문 일치도 질문/gold/독립성의 사람 승인으로 바꾸지 않는다.

## 이전 FINAL 관측의 제외 처리

과거 FINAL9파일/4,800행(정상v2 4,560/예상오류240)의 compressed/decompressed SHA를 원 artifact manifest와 확인했다. CAND05의 review.go74–106행은 B0-POLICY-02/B0-WHY-01에 동일fileSHA/겹치는 span으로480회 인용됐고 multi-reviewer quorum keyword도480회 있었다. 이는 반복 관측이며480독립 표본이 아니다.

CAND05와 같은 `review-record-adjudication` 군의 CAND06도 함께 제외했다. CAND06 자체108–129행 직접 교집합은0이지만 공유 사실군을 새 ID/세부 질문으로 세탁하지 않기 위한 보수적 제외다. 두 문항은 책의 excluded_candidate_questions·excluded_proposed_clusters에 남고 실제 요청 후보에 들어가지 않는다. 다른 행의 직접 교집합0은 개념적 오염 부재나 독립성을 보증하지 않는다. 이전DEV/진단/graph neighbor·사람의 source 노출은 추가 검토 대상이다.

기존 B0 gold/N11DEV/N12정책·ADR의 source 참조도 대조했다. instances.go의 기존정책 gold와 새TRACE/POLICY 후보는 같은 파일의 서로 다른 줄이다. 같은 파일/개념의 중복 여부는 사람이 판정해야 한다. ABS basis가 직접 인용되지 않았다는 사실도 승인된 다른 운영 기록이 없다는 증명은 아니다.

## 새 검토 대상의 질문·후보 답

### N11-FINAL-CAND-07 · TRACE

질문: 저장된 semantic projection은 있지만 현재 graph/vector/source 증명이 stale이면 TraceStatus는 어떤 requirement 경로를 반환하나요?

후보 답: Snapshot과 stale 상태 및 오류 이유를 반환하되 Requirements는 빈 배열이다. 과거 경로를 현재의 유효한 추적으로 제시하지 않는다.

제안 사실군: `trace-stale-diagnostic`. 원문/기권 basis: internal/system/semantic/trace.go:64–78 · fileSHA f1aff6f1dda09d09784444267934bb23ddbd60eca9e2f6e1377984562202e575 · spanSHA fa2b9f7d60bfe2176c9606c68b9cf15d80ffbd48f2f815c4a9daa10bb83c8a5c.

코드의 구현/입력 규칙이 권위 범위다. 실제 조직 정책의 적합성 또는 파일럿 관계의 사람 수용으로 확대하지 않는다.

### N11-FINAL-CAND-08 · TRACE

질문: verified knowledge trace-link가 존재해도 어떤 ADR 근거 조건을 만족하지 못하면 LoadInstances가 거부하나요?

후보 답: 참조 ADR이 존재해야 하고 verified link라면 ADR도 verified여야 한다. 요구사항이 ADR RequirementIDs에 포함되고 link EvidenceRefs에 ADR SourceRef가 있어야 하며 그렇지 않으면 reviewed ADR evidence 부족 오류다.

제안 사실군: `trace-link-adr-evidence`. 원문/기권 basis: internal/system/knowledgepack/instances.go:414–423 · fileSHA 931cff918a97e2a2a50008dcfdeb0ad571e901098501fb34d52ba26bed296731 · spanSHA 24e73a98870166f61c66ca45da26825088006b692dc06c9fae532180571c051f.

코드의 구현/입력 규칙이 권위 범위다. 실제 조직 정책의 적합성 또는 파일럿 관계의 사람 수용으로 확대하지 않는다.

### N11-FINAL-CAND-09 · ABS

질문: 이 프로젝트에서 실제 운영 복구를 위해 승인한 RTO와 RPO는 각각 몇 분인가요?

후보 답: 현재 바인딩한 자료에는 실제 승인 RTO/RPO 값이 없으므로 답변을 보류한다. 작은 시험 실행 시간을 운영 목표로 추정하지 않는다.

제안 사실군: `operating-recovery-facts-undeclared`. 원문/기권 basis: system/eval/b0-knowledge-system/refactoring-n15-n17-native-controls-2026-10-07/summary.json:44–44 · fileSHA 3d12f552ade4a7f61f8423158db4cab3b1f6e06f1ef7f5216aabb82264f15113 · spanSHA 4c746c7a68f741f7ac39564fef4311f9157fa0be966d26a7e14d189b91c270be.

ABS는 positive gold evidence가 빈 배열이며 strict_zero_citations=true다. 위 basis 문서를 답의 근거 citation으로 반환하도록 승인한 것이 아니다. 실제 대상 코퍼스/권위/시점에 승인된 값·담당자가 없는지는 사람이 별도 판정한다.

### N11-FINAL-CAND-10 · ABS

질문: 이 프로젝트의 장애 시 실제 운영 백업과 failover 복구를 맡도록 지정한 담당자는 누구인가요?

후보 답: 현재 바인딩한 운영 역할 기록에는 실제 복구 담당자가 미지정이다. fixture 검토자나 chat-user를 운영 담당자로 추정하지 않고 답변을 보류한다.

제안 사실군: `operating-recovery-facts-undeclared`. 원문/기권 basis: docs/spec-driven/C1-OPERATIONS-REVIEW.md:15–15 · fileSHA ddec29f3f23416ede13c5444b607c79d25e37b1d76d222b1f238e24dc994e45a · spanSHA 32d74b402ed9d6dd6f9a8a8b624e640d445a164080f088db752d96fa6751e90f.

ABS는 positive gold evidence가 빈 배열이며 strict_zero_citations=true다. 위 basis 문서를 답의 근거 citation으로 반환하도록 승인한 것이 아니다. 실제 대상 코퍼스/권위/시점에 승인된 값·담당자가 없는지는 사람이 별도 판정한다.

### N11-FINAL-CAND-11 · POLICY

질문: LoadInstances가 knowledge overlay의 항목 파일을 읽을 때 어떤 확장자와 읽기 상한을 허용하고 directory/symlink 항목은 어떻게 처리하나요?

후보 답: policies/relations/reviews/trace-links는 .yaml, decisions는 .md만 허용한다. directory 또는 symlink entry를 거부하고 ReadSourceFileNoFollow로 각 파일을4MiB 상한 안에서 읽는다. 전체 파일 수/총합 바이트 상한을 보장한다는 뜻은 아니다.

제안 사실군: `knowledge-instance-file-admission`. 원문/기권 basis: internal/system/knowledgepack/instances.go:274–302 · fileSHA 931cff918a97e2a2a50008dcfdeb0ad571e901098501fb34d52ba26bed296731 · spanSHA f409618c9e5586bd143e968fd33b4f439d9f6f28601457413a01f2001911c705.

코드의 구현/입력 규칙이 권위 범위다. 실제 조직 정책의 적합성 또는 파일럿 관계의 사람 수용으로 확대하지 않는다.

## 입력 보호 변경과 검증

제외 catalog를 무시하던 checker가 coherently 승인된 합성 책에 제외 문항을 다시 넣어도 input_ready=true를 반환하는 RED를 재현했다. 실제 사람의 승인이나 질의가 아니다. checker는 이제 excluded ID/cluster/unit·proposed cluster/unit의 재삽입, catalog 불일치/중복/누락 형식을 거부한다. default의 전체 책 사전 검토와 기존 FINAL 관측/예약·group/표본/소스/동결 검사는 유지한다. 제외 내용 자체의 진실성이나 다른 ID/단위까지 의미를 바꾼 사람이 만든 기록의 진실성을 인증하지 않는다.

입력/runner22시험 통과, 같은 제외 재삽입 RED→GREEN, 예전GC 좌표를 새fileSHA에 붙이는 실제 CLI 음성 제어 거부를 확인했다. 실제 미승인 후보 책은 source/gold 오류0·기존 모델/임계치 변화0이며 --require-ready exit2, sampling=inconclusive다. runner도 child/output/registry 생성 없이 exit2로 거부했다. 최초 proposed-unit 시험 fixture가 cluster ID를 unit ID로 잘못 넣은 실패는 보관하고 참조를 교정했다. 모델/제품 MCP 실패나 사람 반려로 집계하지 않는다.

## 다음 실제 검토와 남은 전체

CAND01–04 및 CAND07–11의 gold/범주/사실군과 개념적 오염을 실제 검토한다. CAND07/08이 TRACE 중요군에 적합하지 않으면 실제 파일럿 trace 질문으로 새로 준비한다. CAND09/10은 corpus/source 시점과 실제 무답 필요 여부·strict인용0을 따로 판정한다. 전체 입력과 모델/config/binary/dataset/runtime을 source에 결합하고 사전 동결하기 전에는 DEV도 dispatch하지 않는다. 중요군10독립단위 미만은 inconclusive/default disabled/release withheld다.

현재 전체8/18·요청5/15 완료, 남은N08/09/10/11/12/14/15/16/17/18 10개. 다음은 이 후보의 실제 부분 검토 및 기존 DEV7/프로토콜·N12 실제 내용 판정 반영, 충분한 독립 FINAL 범위 준비다. 새 기본값/출시/전체 완료 체크는 없다.

2026-10-08 N11 검토 대기 코퍼스 연결: [코퍼스·색인·실행 신원 준비](./N11-HELD-CORPUS-PREPARATION.md) — 이전 N12 색인의6Go 누락/ABS2보관 누락을 실제 대조하고 별도 Git19803801/snapshotb1204cf1/datasetee6f91be의 source1,419/선택66·CKG59/CKV63/785chunks/667canonical을 결합했다. 원문16/16·positive14/14 실제 canonical/chunk·ABS2 uncited basis, 무답 basis 검사 누락 RED11→GREEN25와 실제 DEV/FINAL runner child/output/registry0, 원본 source/index/semantic DB 없이 archive 재생 통과. 새 embedding HTTP71/색인785/probe3, 전체Go DRIFT992 유지. scope/gold/군·독립성/사람 판정/동결은 미승인·FINAL0, N11-A–D unchecked·전체8/18·요청5/15·잔여10개 유지. 다음 N11 실제 판정/독립 FINAL 보강과 N12 판정이다.

2026-10-08 N11 후보 보강: [독립 FINAL 후보·범위 부족분](./N11-INDEPENDENT-POOL-EXPANSION.md). 신규CODE/WHY4문항/2제안군 추가, 기존 사실과 겹칠 위험의 신규4문항 격리. 검토 대상FINAL13/8제안군·DEV7, 승인된 중요군 모두0. 과거DEV/FINAL27gzip/11,632행 및 실제20원문 참조 대조, 정보 메타데이터 교정·새runtime 미결합, 실제input checker exit2/source오류0; 모델/검색/빌드0. 전체8/18·요청5/15·잔여10개 N08/09/10/11/12/14/15/16/17/18 유지. 다음 N11/N12 실제 내용·군/독립성 판정과 부족 범위 보강.
