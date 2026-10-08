# N11 개발 입력7개 사전 검토

**개발 실험 입력의 부분 검토 자료 / 미승인·미실행.** 독립 FINAL은 아직0이고 전체 N11 완료나 출시 판정을 요청하지 않는다. 기존 JSON의 바이트·질문·정답·출처·프로토콜은 바꾸지 않았다. 사람의 부분 판정이 오면 별도 원장에 기록하고 이후 전체 입력 동결에서 명시적으로 결합한다.

원문 커밋: `076f6de27b38423a3a2538cb95e41994d092e91b`. 개발 질문 JSON SHA256 `87648fcdf8872c1b6f7f4df2267ba97a34b1088f0c3b13ccdec6be1375042476`. 프로토콜 초안 SHA256 `9ca52cff273ee9897c19641ef757f3ae57ecaef9f8ec3eb2d12db413722472a5`.

BGE-M3/1024차원·기존 digest, K10/검색5회,8arm(pack off/on × baseline/concept_text/relations/combined), warm2+20회, fresh CKS3회·사용자 모델 resident 조건, 회전·직렬 지연측정과 기존 안전/품질/p95 임계치를 유지한다. 새 source/model/dataset·lock·binary/runner 검증은 별도 실행 조건이다. 부분 검토만으로 현재 checker의 execution_ready=false를 true로 바꾸지 않는다.

개발 질문은 공개된 새 구현 경계를 진단하는 용도다. 아래7개가 서로 다른 ID/주제를 가진다는 사실만으로 독립 평가 단위라고 승인하지 않는다. 기존 FINAL/과거 DEV·N12 정책 주장과 같은 사실이면 공유 cluster로 묶고 오염을 기록한다. DEV 결과는 수정에 사용할 수 있으므로 이 질문과 같은 사실을 새 FINAL에 배정하지 않는다.

## N11-DEV-01 · CODE

질문: 아직 존재하지 않는 DB 경로가 current symlink 아래에 있으면 유지보수 쓰기 경계는 어떻게 확인하나요?

후보 답: 각 existing ancestor의 symlink를 풀고 dataset/source markers 및 manifest pins를 검사한다. pinned/versioned 흔적은 versioned_setup_required로 거부한다.

제안 사실 묶음: `maintenance-ancestor`. 판정자/내용·출처·묶음·과거 오염 판정은 아직 null.

원문: [internal/setup/maintenance.go:28–85](../../internal/setup/maintenance.go). span SHA256 `a6beb50580605c2273c3ef7c3868647420b5409f27742da1c00ffcbfbefeb4cd`.

## N11-DEV-02 · CODE

질문: 빌드 자원 보고서의 engine_output_estimate_bytes 값은 무엇이며 staging 예상량과 어떤 차이가 있나요?

후보 답: 엔진 출력 예상량은 계산하지 않아 null이다. source/staging copy 최소량과 임시/출력 free space, reserve를 따로 기록한다.

제안 사실 묶음: `resource-estimate`. 판정자/내용·출처·묶음·과거 오염 판정은 아직 null.

원문: [internal/setup/resources.go:103–148](../../internal/setup/resources.go). span SHA256 `c6334738bdf0d7311208c4d3492a679f5c869b37c2ee6e0688d7792d336f72bf`.

## N11-DEV-03 · WHY

질문: current 변경 뒤 부모 디렉터리 동기화가 실패하면 왜 일반 실패와 다른 오류를 반환하나요?

후보 답: rename 이후 저장 내구성이 확정되지 않으므로 ErrDurabilityUncertain을 반환한다. current가 이미 변경됐을 수 있다.

제안 사실 묶음: `promotion-parent-sync`. 판정자/내용·출처·묶음·과거 오염 판정은 아직 null.

원문: [internal/setup/durability.go:230–241](../../internal/setup/durability.go). span SHA256 `5f8c0695789a80ed11113a823416b949b8bd6f0081ef1dce956eee62bf2f1678`.

## N11-DEV-04 · POLICY

질문: GC 계획에서 현재 버전 외에 활성 reader와 legacy 버전은 어떤 삭제 보호를 받나요?

후보 답: current/실제 rollback/review hold/reader lease를 보호한다. reader protocol이 없는 legacy 후보는 unverified/legacy 보호 사유로 삭제하지 않는다.

제안 사실 묶음: `gc-readers`. 판정자/내용·출처·묶음·과거 오염 판정은 아직 null.

원문: [internal/setup/gc.go:254–428](../../internal/setup/gc.go). span SHA256 `79cb4b1eafa066457ad074f48257c777033b3fc296fba2fd7ed9658a9457f6e6`.

## N11-DEV-05 · CODE

질문: 첫 원문 span이32000bytes를 넘지만 뒤의 작은 span이 들어갈 때 v2 evidence 구성은 어떻게 진행하나요?

후보 답: 큰 span 전체를 생략하고 뒤의 전체 span을 선택한다. 선택되면 partial/예산 진단이며 하나도 들어가지 않으면 typed budget_exceeded다. 원문 좌표나 해시를 잘라서 만들지 않는다.

제안 사실 묶음: `whole-span-budget`. 판정자/내용·출처·묶음·과거 오염 판정은 아직 null.

원문: [internal/system/evidencev2/pack.go:47–155](../../internal/system/evidencev2/pack.go). span SHA256 `d4dfb857291d314e7d1191816e9eb9b4c5994248877785f95f25b6b5a890ffa3`.

## N11-DEV-06 · POLICY

질문: 프로젝트 이름이 같은데 worksheet_packs를 지정하지 않으면 조직 위험 catalog가 자동으로 적용되나요?

후보 답: 명시적으로 선택한 local pack만 version/digest/의존/자료를 검증해 읽는다. 선택이 없거나 old pack에 catalog가 없으면 공통 경로다.

제안 사실 묶음: `worksheet-selection`. 판정자/내용·출처·묶음·과거 오염 판정은 아직 null.

원문: [internal/system/knowledgepack/worksheet.go:54–127](../../internal/system/knowledgepack/worksheet.go). span SHA256 `09e76b5e7b6e205e84e5fcb5a92a09a1a1f982d084ef122af01f0bd3a500a166`.

## N11-DEV-07 · WHY

질문: worksheet catalog의 출처 파일SHA가 맞아도 spanSHA를 별도로 확인하는 이유와 그 검증 범위는 무엇인가요?

후보 답: 전체 파일과 inclusive 줄 범위의 원문 바이트 SHA를 둘 다 비교한다. 원문 신원·좌표 일치를 확인하며 내용의 사실성·권위에 대한 사람 판정을 대신하지 않는다.

제안 사실 묶음: `worksheet-provenance`. 판정자/내용·출처·묶음·과거 오염 판정은 아직 null.

원문: [internal/system/knowledgepack/worksheet.go:176–200](../../internal/system/knowledgepack/worksheet.go). span SHA256 `b9766560c3dbc41c0c27d5483093cdd710a2b21bab2cc924583b3653ad5d96f1`.

## 필요한 부분 판정

N11-DEV-01–07의 질문·후보 답·출처 범위가 개발 진단용으로 적합한지 승인/수정/기권을 결정하고 검토자 이름을 기록한다. 현재 source/spanSHA는 모두 원문 커밋 및 현행 코드와 일치했다. 원문 일치는 후보 답의 정확성·질문 적합성·독립성을 사람이 승인한 것이 아니다.

프로토콜 초안의 기존 조건을 개발 실험에 사용하는 부분 검토도 요청한다. 이는 새 FINAL/gold·독립 cluster 승인이나 기본 활성화/출시 승인이 아니다. 실제 DEV 실행 전에는 source/capture·lock/model/binary 바인딩, 승인 기록/SHA·시점, runner 관측 보호를 검증한다.

남은 전체는 N08/09/10/11/12/14/15/16/17/18 10개다. 다음은 이 부분 검토와 N12 원문/관계 판정 반영, 그 결과를 바탕으로 새 독립 FINAL gold 작성·사전 검토 및 실행 보호 결합이다.
