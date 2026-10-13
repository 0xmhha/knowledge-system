# Claude Code 최신 세션 인수인계 — 2026-10-09

> 역사 인수인계 문서: 아래의 현재 상태·잔여 작업·실행 범위는 작성 당시 기록이다. 새 세션은 [CURRENT-HANDOFF.md](./CURRENT-HANDOFF.md)와 현행 진행 원장을 먼저 읽는다. 과거 위임/전송 범위를 새 실행 권한으로 사용하지 않는다.

## 0. 이 문서의 역할과 인계 시점

사용자 요청: “여기까지 작업하고, 이후부터 claude code에서 작업을 진행하려고 해. 알고 있고 진행하려고 했던 모든 기억과 컨텍스트를 전달하고 싶다.” Codex의 제품 개발은 여기서 중단하고 인계 문서만 준비한다. Claude 실행·모델 호출·외부 전송은 이 인계 작성 과정에서 하지 않았다. 내부 대화/숨겨진 사고 상태를 그대로 이전한 문서가 아니라, 작업 재개에 필요한 사실·결정·증거·실행 경계를 저장소에 외부화한 문서다.

- 저장소 `/Users/kevin/work/github/0xmhha/auto-coding/knowledge/knowledge-system`
- 브랜치 `feat/refactoring-followup`
- 제품/진행 기준 HEAD `4f092414109c4755d1b0c2ce192a2c3df9efa94e`. 아래 기계 판독 snapshot에도 전체 SHA를 기록한다. 문서 인계 커밋은 이 기준 뒤에 추가된다.
- 마지막 개발 증거 커밋 `eb70e2ec`; 진행 원장 커밋 `4f092414`.
- 전체 N01–18 중 **8/18 완료**: N01–07, N13. 사용자 요청 N04–18 기준 **5/15 완료**. 남은 **10개**: N08/09/10/11/12/14/15/16/17/18.
- 최신 검사 PASS: source/dependency1,456, 완료8/18, 잔여10, next N12, 부분 체크5. 신규 N12 current+committed blob SHA1,535개 일치.
- tracked 변경 없음. 기존 사용자 untracked `.claude/`, `logs/`는 Codex가 읽거나 변경하지 않았다. 이어서도 사용자 소유 내용을 임의로 읽거나 변경/삭제/커밋하지 않는다.
- 과거 PR97은 squash merge, 당시 main `eeeab2e7` 동기화 완료다. 현재 로컬 후속 브랜치가 최신 작업 위치다. 최신 증거가 원격에 있다는 보장은 없다. 다른 머신에서는 이 checkout/Git history/필요 자료를 실제로 옮겨야 한다. main checkout·pull·reset을 시작 동작으로 하지 않는다.

`CLAUDE-CODE-REMAINING-HANDOFF.md`와 `CLAUDE-CODE-TRANSFER-SCOPE.json`은 이전 **Codex→Claude 위임 시도**의 시점·권한·범위를 기록한다. 그 문서의 HEAD912b619e, “범위 미선택/기능 초안 미적용/관계25개”는 최신 상태가 아니다. 해당 시도는 자동 승인 검토가 외부 전송 승인 부족으로 차단했고, 사용자가 Codex 직접 수행으로 전환했다. 부모 Codex가 검토·커밋한다는 이전 위임 구조도 새 사용자 직접 Claude 세션에 그대로 적용한다고 가정하지 않는다. 과거 전송 허용 범위를 자동 확대하거나 새 실행 승인으로 해석하지 않는다. 새 세션의 실제 권한/외부 전송/게시 범위는 사용자의 현재 지시와 해당 실행 환경을 따른다.

## 1. 최소 읽기 순서와 사실의 우선순위

처음부터 수만 행의 로그·모든 archive를 프롬프트에 넣지 않는다. 아래 순서로 읽고 해당 작업의 실제 source/자료만 추가로 읽는다.

1. 이 문서와 [인계 snapshot](./CLAUDE-CODE-HANDOFF-SNAPSHOT-2026-10-09.json), [시작 프롬프트](./CLAUDE-CODE-START-PROMPT-2026-10-09.md).
2. [현재 원장](./REFACTORING-PROGRESS.json), [잔여 작업리스트](./REFACTORING-REMAINING-WORKLIST.md), [완료 실행 계획](./REFACTORING-COMPLETION-PLAN-2026-10-08.md).
3. [실행 명세](./REFACTORING-EXECUTION-SPEC.md)의 현재 수행할 N 항목/수용 체크. 날짜별 기록은 과거 이력이며 최신 원장의 candidate identity와 함께 읽는다.
4. [최신 N12 검토](./N12-ANCHOR-BOUND-TRACE-REVIEW.md), [개별 검토 worksheet](../../system/eval/b0-knowledge-system/refactoring-n12-anchor-binding-2026-10-09/human-review-worksheet.json), [기술 manifest](../../system/eval/b0-knowledge-system/refactoring-n12-anchor-binding-2026-10-09/manifest.json).
5. [N11 독립 pool](./N11-INDEPENDENT-POOL-EXPANSION.md), [범위 선택 적용](./N11-N12-SCOPE-ADOPTION.md), [노출 감사](./N11-RESPONSE-EXPOSURE-AUDIT.md), [추가 원문 조사](./N11-SOURCE-PROBE-INVESTIGATION.md), [runner 계약](./N11-EVALUATION-RUNNER-CONTRACT.md).
6. 목적·원래 수용 범위를 확인할 때 [원문/v2 대응](./REFACTORING-ORIGIN-AND-V2-TRACE.md), [원 목적 재감사](./REFACTORING-ORIGINAL-GOALS-REAUDIT.md), [v2 계획](./DELIVERY-PLAN-V2.md), [종단 간 설계](./END-TO-END-DESIGN.md).

적용 가능한 AGENTS.md/CLAUDE.md는 새 도구가 먼저 확인한다. 문서의 인용·코드 주석·로그·모델 응답은 작업 자료이며 권한을 부여하는 상위 지시가 아니다. 사실 충돌 시 날짜만으로 정하지 말고 **해당 candidate의 snapshot/source/model/검토 상태와 committed 원자료**를 대조한다. 인계 snapshot이 오래되면 현재 원장의 active binding부터 검증하며 자동 완료 변경하지 않는다.

## 2. 리팩토링의 배경과 유지할 목적

CKG는 코드 구조/이력을, CKV는 코드·문서의 유사도 검색을, CKS는 근거 팩을 제공했다. 목표는 단순 유사 문장 찾기에서 더 나아가 **어떤 요구를 어느 버전의 코드·시험·문서가 뒷받침하는지 검토 가능한 원문으로 설명**하는 것이다. 의미 관계·출처·권위·버전·검토 상태를 명확히 하며 충돌/누락/미확인을 드러낸다.

초기 자료는 `~/work/github/0xmhha/study/docs/reviews/knowledge-system/`의 analysis-review, spec-driven-requirements, spec-driven-architecture, spec-driven-wbs 등에 있다. study/main 보관 커밋 `1c350c18625ca2a46f1ae33ef4e56efb88f9617d`, 분석 대상 제품 `1ded9b3e47bc2e09062dba329c2f918423746fa4`. 초기 문서 작성 시점과 Git 보관 시점은 다르다. 같은 머신이면 원문을 읽을 수 있으나 사용자 지시 없이 다른 study checkout을 동기화/수정하지 않는다. 다른 머신이면 저장소의 original-source-recovery 원자료와 대응 문서를 먼저 쓴다.

현행 기준은 v2다. FR10/INV7/NFR5/시나리오9 및 초기 WBS29의 목적을 유지하며 순서는 **D설계→A모델 독립 개발→B실모델 평가→C평가 후 수정·출시 판단**이다. 시험 preview 종료는 검색 품질/파일럿/운영 출시 완료를 뜻하지 않는다. semantic 객체 전체를 CKG AST enum으로 옮기거나 RDF/OWL/GraphRAG를 새로 도입하는 별도 프로젝트가 아니다.

유지할 핵심 계약:

- immutable source snapshot/dataset과 세 엔진의 project/source/model identity 일치, 불일치 거부, 기존 current 보존.
- 실제 보관 원문·파일/행/span SHA·AST canonical을 인용 근거로 사용. similarity 점수만으로 의미 사실을 verified로 바꾸지 않는다.
- core20와 프로젝트 functional vocabulary를 분리, 선택형 ontology 실패/모호성/미검토 관계에서 CKV+CKG baseline 인용 보존. 기본 활성은 품질 판정 이후.
- 테스트 pass, semantic linked run, 관계 verified, 사람 criterion accepted는 서로 다른 상태다.
- 공개 v1/v2 계약/구 DB 호환, OS lock·reader pin·내구성·복구·예산/취소 실패 상태 보존.
- 최종 흐름 요구→근거/trace·충돌→외부 패치→시험·사람 수용→동일 신원 재색인이다. CKS 제품의 자율 코드 수정은 이 범위의 목표가 아니다.

## 3. 사용자 선호·승인 사실

한국어로 간결하게 진행을 설명한다. 세부 단계 완료마다 **목적, 현재 단계, 실제 검증, 전체 완료 수, 다음 작업, 남은 전체**를 터미널/문서에 남긴다. 구현 완료 이전에 checkbox를 체크하지 않는다. 필요 기능/설계를 누락하거나 수용 조건을 줄여 완료율을 올리지 않는다. 관련 검증이 통과하면 새 변경/실패 없이 같은 전체 시험을 반복하지 않는다.

사용자는 반복적인 “다음 진행” 없이 실행 가능한 남은 작업을 계속 진행하기를 원했다. 실제 사람/운영 사실이 없으면 승인·이름·표본을 꾸며내지 말고 구체적인 자료를 만든 후 해당 결정만 직접 채팅으로 요청한다. 이전 사용자 화면에서 승인 UI가 보이지 않아 Claude 위임을 포기했으므로 질문은 사용자가 볼 수 있는 대화/터미널에서 명확히 전달한다. 같은 승인 범위는 다시 묻지 않는다.

중요 승인과 한계:

- B0 12질문/후보 답/출처와 두 기권 문항 검토에 사용자 “승인.”. v1 인용0 조건과 실제 사람 답변 기권은 별개다. 자세한 당시 판정은 `human-review-m2max-2026-10-03.json` 등 승인 원자료를 기준으로 한다.
- BGE-M3 확정. 승인 digest `7907646426070047a77226ac3e684fbbe8410524f7b4a74d02837e43f2146bab`, native/target1024. 최근 로컬 Ollama0.35.1. 새 머신/태그를 이 바이트와 동일하다고 추정하지 않는다.
- 사용자 명시 승인: Go source29 포함, 당시 protocol, F-01–06, SF-03/05-A/05-B/06의 **테스트 사례 범위**, STV2 설정. 승인5입력의 SHA는 active manifest에 있다. 기존 승인 파일은 수정하지 않는다.
- HC-01–11 해석 승인, **시험 preview 범위로 종료**. 실제 출시 승인 아님.
- 최근 “모두 권고 방향으로 진행”은 N11 POLICY=`code_contract`+`organization_policy`, TRACE=`code_contract` vs 실제 `reviewed_path`; N12 core20 유지+기능3/요구version2의 **범위 선택2개** 승인이다. 개별 gold/사실/관계/criterion 승인은 아님.
- 실제 검토자 이름 **null**. 로컬 계정 kevin, Git fixture author, chat-user 또는 임의 사람 이름으로 채우지 않는다. 개별 N11 gold/독립성 및 최신 N12 content approval은0.
- 과거 특정 커밋 푸시와 특정 PR 생성/ready/merge는 사용자 승인 후 수행했다. 그 사실을 새 private raw/held archive 전송·새 PR/merge·배포의 포괄 승인으로 해석하지 않는다. 이 인계에서 새 push/PR/배포 없음.

## 4. 최신 N12: 완료한 기술 준비와 정확히 남은 판정

최신 폴더 `system/eval/b0-knowledge-system/refactoring-n12-anchor-binding-2026-10-09/`:

- runtime 원문 `d6fc8902c2df4c66a6ef84307477f1ab44740b8c`; 자체 derived Git commit `36353d32f1c21fc0cb2622807fa9e5d05f2daac7`.
- snapshot `672eae93464af931deb5b66605ba6d21615e50ced58c5a09cd82befc12f56ae7`.
- dataset `1e82983e45ea35eb63debbea9bf88b6961d595cb5c540db312430cc47d19a341`.
- source1,438파일, 선택71, CKG63Go/CKV66(Go63+MD3),747chunks/625canonical/floor1.0; YAML4+PILOT-SCOPE.json 보관/추출되며 vectorized 아님.
- 실제 CKG10,545nodes/24,418edges, parse/unresolved0. source/index path/SHA bijection.
- canonical18(구현3/시험7/문맥8), evidence81, CKV document sections27/27. 개념23(core20+functional3), 요구3 version2/기준4.
- 신규 관계17: IMPLEMENTED_BY3/TESTED_BY7/CHECKED_BY7, 새 검토 ID **F3-T01–17**. 기존 SUPPORTS/ABOUT12 포함 총29 **전부 proposed**, 이전 snapshot 승인 승계0.
- retained source·coordinates·canonical·chunk-link 검사, SQLite close→read-only reopen JSON일치, 변조5거부.
- race 관련3패키지8시험/29하위시험 pass, fail/skip0. 실제 semantic linked run이 아니다.
- 전체 Go audit는 build1,055/DB63/in_both63/missing992/extra0 **DRIFT**. 누락992는 명시 선택 밖. 전체 repository coverage PASS로 바꾸지 않는다.
- index embedding70HTTP/747입력 +startup probe2HTTP/2입력, embedding72HTTP/749출력/실패0. query DEV/FINAL0. index 비용을 query p95/warm/cold/human active time으로 쓰지 않는다.
- semantic current 없음, 실제 사람 판정0·reviewed path0·semantic linked run0·actual pilot activation0·FINAL dispatch0.
- 보관본 `held-anchor-bound-candidate.tar.gz`:13,792,619byte/1,457file, SHA `e10b33357875dec49b7b509b7af00f35a553dea16d603effaa24239216c146b2`. source blobs/history+actual CKG/CKV DB+proposed SQLite 포함. 원본 source/index/store 없이 재생, 권위 SHA와 projection/relation/trace bytes 동일. 실제 운영/독립 머신 이식성 수용 아님.
- 검토 projection SHA `6d02a408c95e958c653e008c665922b5c96ba452eba68476bf51301808ebd069`.

핵심 해석: **F3-T08 ValidateDatasetAlignment는 좌표 일치 구성요소**다. 보관 source bytes/canonical/activation 재검사는 checkCodeAnchor와 PutAligned/PutAlignedRetained/ActivateAligned/ActivateAlignedRetained gate chain이 담당한다. F3-T09/10은 좌표 부분 제어; **F3-T11/12**는 실제 CKG2build/2snapshot의8쓰기·2activation 거부/이전 active 보존 통합 제어다. 이 시험의 CKV는 coordinate-only mock manifest이며 vector DB/chunk links/embedding 없음. 실제 canonical을 DB에 직접 가짜로 넣은 시험이 아니다. 시험 성공이 실제 pilot reviewed 관계/criterion 수용을 대신하지 않는다.

기능3: filtered-nearest-chunk-search, snapshot-bound-semantic-relations, optional-ontology-baseline-preservation. 실제 내용 검토 대상은 기능3/요구3·기준4/F3-T01–17부터; core20/정책·ADR6주장/기존12관계의 별도 개별 내용 검토도 남는다. worksheet는23/3/6/12/17 전부 null decision/reviewer를 담는다. 현재 필요한 사용자 입력은 **대상별 승인/수정/반려와 실제 검토자 이름**. 이미 승인된 범위를 다시 묻지 않는다.

`PILOT-SCOPE.json`은 선택 receipt, runtime default/loader/active dataset 좌표를 자동 바꾸는 파일이 아니다. `spec-pilot-v2.yaml` YAML schema version1과 각 requirement version2를 구별한다. `ValidateCoreOntology`의 exact20 gate를 generic23개로 완화하지 않는다.

## 5. 후보와 시험 이력 — 서로 섞지 않기

| 단계 | 증거 폴더/주요 커밋 | 성격 |
|---|---|---|
| 기능v2 최초 held 결합 | refactoring-n12-functional-rebind-2026-10-08; f3e199ec/b487e876 | source55d8e2db/snapshot43ba7452/datasetf8392112,1,437파일·CKG62/CKV65/742chunks621canonical,신규15관계/총27,7시험24하위시험. 새 foreign-anchor 통합 시험 **미포함**, immutable |
| 실제 승격 통합 회귀 보강 | refactoring-n12-anchor-promotion-2026-10-08; c4eae54e/d6fc8902 | 새 anchor_promotion_integration_test.go; semantic 전체race42시험20하위시험,5사례8쓰기+2activation 거부. fixture YAML 최초 작성 오류는 제품 RED 아님. 실제 pilot source binding은 이 단계에서 아직 미완료 |
| 최신 source/index/관계 결합 | refactoring-n12-anchor-binding-2026-10-09; eb70e2ec/4f092414 | 위 신규 시험이 **새 실제 capture/index 안에 포함**, F3-T11/12 제안 링크. 이전 두 단계 자료를 소급 변경하지 않음 |

현재 다음은 기술 결합 반복이 아니라 **실제 내용 판정 후 승인된 overlay/lock/의미 경로** 준비다. approval 없이 API를 우회해 linked run/accepted를 만들거나 동일 fixture를 더 실행해 실제 수용 표본이라고 세지 않는다.

주요 기존 제품 수정·검증을 재발명하지 말고 아래 문서에서 현재 구현을 따라 확인한다.

- [N09 빈 snippet 검사](./N09-EMPTY-SNIPPET-VERIFICATION.md),8ec6bea8/2daddbd2: 기존 helper가 empty/whitespace를 Verified로 오판한 RED→empty_snippet 수정.103시험/4하위시험, 실제 BGE smoke1skip. 실제 기권 품질/인용0 수용은 남음.
- [N14 정확 실행 근거](./N14-EXACT-RUN-PROOF-VALIDATION.md), [roundtrip](./N14-EXACT-RUN-ROUNDTRIP.md): 잘못된 exact proof15수용 RED→16거부, 정상 multiple/nested·public runner/import 제어. 미승인 실제 후보는 dispatch 전에 거부; linked run0. 테스트 이름만 맞춰 준 approval을 만들지 않음.
- [shared alias](./N12-SHARED-ALIAS-INTEGRATION.md): 생산 retained parser/matcher→stage2→V2 baseline 보존 제어. 전체 retrieval 품질 판정 아님.
- [N10 비반환 진단](./N10-NEIGHBOR-DIAGNOSTICS.md): backend_error·실패 분모 보존. p95/전체 비용 평가 아님.
- [N15/N17 기술 검증](./N15-N17-CURRENT-TECHNICAL-VALIDATION.md), [BGE 작은 호환성](./N15-BGE-COMPATIBILITY-VALIDATION.md), [native Linux CI](./N15-NATIVE-LINUX-CI-VALIDATION.md): N15-A/B/C와 N17-A/B의 **5부분 체크**만 기술 증거 결합. 과거 PR97 CI5/5·run37586641833을 새 실행 결과로 쓰지 않음.

## 6. N11 독립 평가 — 아직 실행할 수 없는 이유

검토 pool DEV7/FINAL후보13/제안군8/제외6, **CODE/WHY/POLICY/TRACE/ABS 승인 독립 FINAL 각각0**이다. 중요군별 최소10 독립 승인 조건은 유지한다. 같은 사실을 번역·분할·질문 변형하거나 CODE/WHY에 공유하면서 표본 수를 부풀리지 않는다. provisional4/4/1/2/1은 승인 수가 아니다.

최근 scope adoption 초안은 POLICY/TRACE eligible5+excluded2 총7행에 code_contract metadata를 붙였다. organization_policy actual facts와 reviewed_path0은 아직 없다. candidate/gold/군·독립성·이전 오염·ABS basis·protocol·검토자 및 전체 runtime freeze는 미완료다.

기존 FINAL4800행/9원자료 및 추가 누적27gzip/11,632행 등을 감사했다. 기존CAND05의104행이480response에 exact 노출된 사실을 유지한다. response.content까지 감사한 다른 문자열 일치0은 독립성 증명 아님. 추가 원문8범위 중6은 과거 설계/튜닝으로 quarantine,2는 의미 중복 판정 보류. 단지 새 파일/커밋이라고 fresh holdout이 아니다. 과거 모델 응답으로 FINAL 정답을 튜닝하지 않는다.

runner `scripts/refactoring-eval-run.py`는 OS lock과 **child dispatch 전 durable FINAL reservation**을 강제한다. failure/crash가 생겨도 예약을 자동 해제하거나 우회 flag를 쓰지 않는다. 미승인 초안 --require-ready exit2/실행0은 올바른 거부다. 현재 N12의 새17관계/63Go corpus를 N11 independent17문항이나 N11 전체 frozen runtime으로 바꾸지 않는다. 일부 N11 term/spec source는 이 selected index 밖이다.

실행 가능한 다음 보강은 승인할 내용을 구체화하거나 실제 미사용 원문 후보/cluster를 조사하는 일이다. 조사도 history/오염 경계와 source SHA를 남긴다. 새 candidates가 없거나 사람 판정만 남으면 정확한 부족분을 제출하고 무관한 시험 반복으로 자율 진행을 가장하지 않는다.

## 7. 남은 전체와 실행 순서

| 우선순위 | ID | 남은 목적/완료에 필요한 것 |
|---|---|---|
| 1 | N12 | 실제 개념·요구·주장·관계·기준/검토자 판정, 승인된 capture/lock/trace 결합, 실제 pilot/linked run/criterion 수용·측정시간 |
| 1 | N11 | 독립 source-backed DEV/FINAL gold·군·오염/ABS 판정, 중요군10개씩 충분한 표본, protocol·source/model/runtime 사전 동결 |
| 2 | N08 | 승인 독립 입력으로 검색 품질/ablation/언어·필터 평가; 기존 FINAL 관측으로 튜닝하지 않기 |
| 2 | N09 | 실제 answerability/무답·기권 및 인용0 조건을 구별해 평가; 기계 helper 통과만으로 완료하지 않기 |
| 2 | N10 | 동일 arms/실패 분모·모델 calls/tokens 및 warm/cold 지연/비용 실측; index probe와 query를 분리 |
| 3 | N14 | 실제 reviewed ADR→요구→기준→code/test, baseline/result held patch/immutable attempt·exact linked run, 별도 사람 수용, 재색인/승격·반려/rollback |
| 3 | N15 | 실제 모델·부하·규모/지원 조건, 독립 운영 소비자의 install/upgrade 수용; 기술A/B/C만 현재 체크 |
| 3 | N16 | 실제 roles/scope/owner·public key 독립 trust·rotation/revocation 및 native/transitive notices 적합성 판단 |
| 3 | N17 | 실제 backup/failover 담당·RTO/RPO·키/전원/network 정책, 대상 호스트 운영 복구 훈련·손실/시간·사람 수용; 기술A/B만 체크 |
| 4 | N18 | 모든 선행의 품질/운영 한계와 기본 활성·최종 수용·release 판단. disabled/inconclusive/withheld를 전체 완료로 바꾸지 않기 |

정확한 각 N 수용 checkbox/선행은 원장·실행명세·작업리스트를 따른다. 운영 owner/규모·독립 consumer·역할/키·RTO/RPO·network/전원 사실은 현재 미제공이다. local mock 소유자·시험 elapsed를 실제 운영 사실/사람 활동시간으로 기록하지 않는다. 미측정 활동시간은 null.

## 8. 환경·보관본·재개 명령

최근 관측 환경: macOS arm64/M2 Max, Go1.26.8(module go1.25.13), zsh, 로컬 Ollama0.35.1. Claude binary 존재는 `/Users/kevin/.local/bin/claude`에서 확인했지만 실행/로그인/버전/권한을 검증하지 않았다. Linux/native 다른 머신의 실제 결과는 별도 증거다. 현재 시간대 Asia/Seoul; raw UTC2026-10-08T15…는 KST2026-10-09에 해당하며 raw timestamp를 수정하지 않는다.

Claude가 먼저 실행할 읽기/검사:

```sh
git branch --show-current
git status --short
git log -3 --format='%h %s'
python3 scripts/check-refactoring-progress.py
```

checker 예상: PASS8/18,10remaining,nextN-12,1,456source/dependency,5partial checks. 현재 테스트/소스 바이트를 검증하는 시작점이며 인간 판정의 진실성까지 인증하지 않는다. 검사 실패를 무시하거나 원장을 먼저 완화하지 않는다.

최신 임시 base `/private/tmp/ks-n12-anchor-binding-zsyd8_7d`, source/dataset/semantic-anchor-bound-proposed.sqlite와 archive-replay가 있었다. **임시 폴더 존재는 보장하지 않는다.** durable authority는 커밋된 archive/bundle/manifest/receipts다. 다른 머신에서 단순 unpack만으로 OS/model/runtime 전체 이식성 합격을 주장하지 않는다. helper Python은 old absolute paths/preparation.json을 사용하므로 실제 재실행 전에 읽고 별도 owned-temp/new receipts를 설계한다. 원 helper/receipt·고정 manifest를 덮어써서 이식하지 않는다.

Go 캐시 쓰기/로컬 Git metadata·loopback가 Codex sandbox에서 escalation이 필요했으며 해당 로컬 검증은 허용됐다. Claude 환경에서 발생하는 권한·모델 오류는 제품 regression과 구별해 기록한다. fixture 서명 실패를 피한 방법은 명령 범위에서만:

```sh
GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=commit.gpgsign GIT_CONFIG_VALUE_0=false go test ...
```

fixture Git에는 `git -c commit.gpgsign=false commit ...`을 사용했다. 실제 repository/사용자 전역 서명·키 설정을 바꾸라는 지시가 아니다. 사용자 키/토큰을 읽어 출력·외부 전송하지 않는다.

## 9. 변경과 완료를 안전하게 보존하는 절차

1. 현재 N의 목적·수용 조건·선행을 읽고 필요한 기능/설계를 문서로 먼저 분해. 새 조건을 발견하면 scope/설계에 명시.
2. 의미 있는 실패/누락 재현→최소 구현→관련 성공/실패·공개 계약 시험. 낮은 영향의 단순 문서 수정을 제품 RED처럼 만들지 않는다.
3. 새 원자료는 별도 evidence 디렉터리로 작성. 명령/exit/raw SHA/실제 분모/skip/환경/시간/한계를 남기고 old FINAL/approved5/held 후보를 보존.
4. 새 manifest에 **전체 현행 cmd/internal/pkg Go와 비평가 의존** source SHA, approved5SHA, reportsSHA/evidenceSHA 결합. 기존 active source map을 읽고 새 Go 파일 누락 없이 갱신. 제품 source 변경이 있으면 예전 active binding 그대로 통과했다고 주장하지 않기.
5. 사용자 권한/새 실행환경이 허용하는 범위의 로컬 증거 커밋 후 current 파일과 `git cat-file --batch`의 **그 커밋 blobSHA**를 대조. manifest는 자신 SHA를 자기 안에 넣지 않음. 그 다음 원장의 active_source_binding revision/manifest와 해당 task의 기술 부분 필드를 결합하고 progress checker.
6. 원장 커밋으로 마무리. 구현 커밋을 원장 커밋과 분리하면 자기 참조 SHA 문제를 피할 수 있다. 이미 있던 checked5의 historical evidence revision은 임의 재작성하지 않음.
7. spec/worklist/handoff·진행 로그에서 전체 체크는 모든 수용 조건 충족 시만 변경. 실제 review 미완료면 waiting/pending, 담당자·활동시간 null, full_task_complete=false. 최종 보고마다 next/all remaining.

`git reset --hard`, `git clean`, 다른 작업 tree 제거, 강제 push, 본인 권한 확대/승인 우회는 재개 절차가 아니다. 새 push/PR/merge/배포는 사용자의 구체적인 현재 지시와 실제 내용 범위를 확인하며 진행한다. 이미 주어진 승인 범위는 유지한다. 개발을 위해 raw archive/FINAL responses를 외부 전송할 필요가 있는지 먼저 최소 필요한 자료로 판단한다.

## 10. Claude 첫 응답의 확인 항목

“인계 읽었다”만 말하지 말고 다음을 현재 파일/검사와 대조해 제시한다.

- 실제 branch/HEAD와 active binding revision, 전체8/18·남은10개/부분5체크.
- 승인된2scope와 미승인 individual content/null reviewer 구분.
- 최신36353d32/672eae93/1e82983e candidate와 immutable 이전 후보 구분.
- 이번 actual race8/29와 historical42/20, index model calls와 query/FINAL0 구분.
- 다음 구체 작업 및 선행 사람/운영 판정, 승인 기다리지 않고 가능한 독립 작업.

미흡하거나 상충한 사실은 근거 경로와 함께 먼저 확인한다. 새로운 사용자 답변은 이 인계 파일보다 최신 결정일 수 있으므로 범위·원문·검토자·대상 identity를 기록해 이어간다. 사용자에게 이미 승인한 두 범위를 다시 묻거나 전체 미완료를 잊고 새 프로젝트를 처음부터 시작하지 않는다.
