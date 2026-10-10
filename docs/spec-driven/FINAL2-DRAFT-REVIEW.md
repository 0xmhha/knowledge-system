# FINAL2 초안 검토 요청 (N08·N09·N10)

2026-10-10 · **초안, 미검토, 미동결, 미실행**. 전체13/18, 남은5개.

## 1. 왜 새 FINAL이 필요한가

N10(요청 단위 검증 공유), N08(사용 타입 선언 추가), N09(미지정 사실 기권)은 모두 이전 FINAL을 관측한 뒤에 바꾼 코드다. 동결 규칙은 관측 뒤에 바꾼 것을 새 DEV와 새 독립 FINAL로 판정하라고 정한다. 사용자는 소규모 FINAL을 골랐다. 이 규모에서 판정할 수 있는 범위는 다음과 같다.

| 작업 | 이 FINAL로 정할 수 있는 것 |
|---|---|
| N10 | 후보 binary 실행에서 기준 외 7개 arm의 warm p95비가 모두 1.25 이하인지. 소표본 규칙과 무관하다 |
| N08 | 이전·새 binary의 문항별 paired 차이(서술). 중요군이 모두 10단위 미만이라 **inconclusive** |
| N09 | 무답 2문항과 답가능 대조 2문항에서 pack on/off의 기계 집계. **inconclusive**, 답변 수준 기권은 null |

## 2. A: 기존 corpus에서 이전·새 binary 비교

- **corpus.** 동결 N11 corpus(`e693b312`, dataset `cd9a22e3`)와 semantic store를 그대로 쓴다. 재색인과 projection 재검토가 필요 없다.
- **binary.** 후보는 `7077741f`에서 빌드한 cks(SHA `dc6cf0a9…`)다. 제품 코드는 그 뒤로 바뀌지 않았다. 참조는 이전 FINAL에 쓴 `cks-7c57a388`(SHA `e53c308e…`)이다.
- **실행.** runner가 FINAL 예약 하나 안에서 후보 8arm matrix를 먼저 돌리고, 성공하면 같은 요청으로 참조 8arm을 돌린다(`1d25abf8`). 채점기는 두 capture를 비교한다(`ec27582f`).
- **질문.** DEV 5문항(CODE2·WHY1·POLICY1·TRACE1)과 FINAL 10문항(CODE3·WHY3·POLICY2·TRACE2)이다. 모두 새 사실군이며, 이전 DEV·FINAL 사실군 17개와 이전 B0 질문의 범위를 피했다. 보조 agent가 초안을 쓰고 주 agent가 원문 span과 hash를 검증했다. 후보 답보다 span이 좁았던 2문항(FINAL-01, FINAL-04)은 넓혔다.
- **입력 검사.** 근거 원문 오류와 이전 관측 중복은 0건이다. 남은 사유는 검토와 동결뿐이다([검사](../../system/eval/b0-knowledge-system/refactoring-final2-draft-2026-10-10/part-a-input-check-draft.json)).

| ID | 군 | 질문 | 후보 답(요약) | gold |
|---|---|---|---|---|
| F2-DEV-01 | CODE | 추출된 키워드를 ckg FTS 질의로 넘길 때 일반 식별자와 구두점·연산자가 섞인 키워드를 어떻게 인코딩하나요? | 영숫자·밑줄이고 연산자가 아니면 그대로, 아니면 따옴표로 감싸고 내부 따옴표를 이스케이프 | `ckgclient/keyword.go:5-21` |
| F2-DEV-02 | CODE | 측정 스코프 종료 시 끝나지 않은 호출과 늦게 끝난 호출은 어떻게 반영되나요? | in_flight를 PendingCalls로 세고, 닫힌 뒤에는 기존 요약을 그대로 반환 | `backendmeasure/recorder.go:66-89` |
| F2-DEV-03 | WHY | CLI가 내부 도구의 원래 오류 메시지를 출력하지 않는 이유와 대신 출력하는 것은? | 소스 바이트·자격 증명·staging 경로 노출 방지. 공개 오류 코드만 출력, 없으면 operation_failed | `cmd/cks/main.go:83-91` |
| F2-DEV-04 | POLICY | 평가 시나리오의 실행 횟수, 인용 없음 기대, 기대 커밋 규칙은? | runs≥1, 두 기대의 동시 지정 거부, 40자 소문자 SHA | `eval/scenario.go:184-194` |
| F2-DEV-05 | TRACE | 기준의 시험을 실행할 때 대상을 지정하지 않으면 어떻게 고르고 언제 거부하나요? | 검토 경로가 없으면 거부, 하나뿐이면 자동 선택, 여럿이면 선택 요구, 검토 대상이 아니면 거부 | `semantic/test_run.go:114-128` |
| F2-FINAL-01 | CODE | 예산 단계에서 토크나이저 없이 토큰 수를 어떻게 추정하나요? | 빈 문자열 0, 그 외 UTF-8 룬 수 ÷ 4 | `composer/budget/tokens.go:18-32` |
| F2-FINAL-02 | CODE | redaction 기록의 excerpt에는 무엇이 들어가나요? | rule ID와 일치 횟수만, 일치한 바이트는 넣지 않음 | `composer/sanitize/engine.go:230-248` |
| F2-FINAL-03 | CODE | 소스 identity 해시는 여러 필드를 어떻게 이어 붙여 해시하나요? | domain+NUL, 필드마다 8바이트 big-endian 길이와 바이트, SHA-256 | `setup/identity.go:47-60` |
| F2-FINAL-04 | WHY | 아키텍처 설명 의도에서 defines 관계를 거르지 않고 점수만 낮추는 이유는? | 필드 edge가 calls를 19위로 밀어냈음. 필드도 신호라 강등(defines 0.5, imports 0.7) | `composer/stage3/relation_weights.go:5-44` |
| F2-FINAL-05 | WHY | 평가 지표에 MRR을 따로 두는 이유는? | precision/recall은 순서를 보지 않음. 순위 튜닝은 MRR을 움직여야 함 | `eval/metrics.go:161-175` |
| F2-FINAL-06 | WHY | 서브그래프 기본 간선 상한이 80인 이유는? | 허브 depth>1에서 수천 개. 80개는 약 33KB로 응답 한도 안, 200개는 초과 | `ckgclient/real.go:28-36` |
| F2-FINAL-07 | POLICY | canonical 심볼 커버리지 하한 검사는 어떤 값을 허용하고 언제 거부하나요? | 0–1 범위, 0이면 끔. 카운터가 잘못되거나 비율이 하한 미만이면 거부 | `semantic/alignment.go:43-64` |
| F2-FINAL-08 | POLICY | 지식 팩 입력 트리 digest에서 어떤 경로를 거부하나요? | 비UTF-8·비NFC·역슬래시·상위·절대 경로, 빈·점·숨김 구성요소, 심볼릭 링크 | `knowledgepack/digest.go:51-63` |
| F2-FINAL-09 | TRACE | 검토된 trace 경로에서 앵커를 찾을 때 여러 개 또는 0개가 맞으면? | 여러 개면 ambiguous, 0개면 missing으로 evidence_unverified | `semantic/trace_anchor.go:34-51` |
| F2-FINAL-10 | TRACE | 패치 시도를 등록할 때 기준·결과 버전 관계로 언제 거부하나요? | ID 형식, base 없음, 다른 프로젝트, 같은 snapshot, 변경 파일 없음 | `patch/patch.go:166-196` |

## 3. B: knowledge가 있는 새 corpus에서 기권

**corpus.** 동결 시점의 저장소 HEAD에 `.cks/knowledge`(manifest와 unknowns 2건)를 더한 파생 commit이다. 이번 FINAL2 문서, N11 문서, `system/eval/**`는 제외한다. semantic store는 없다. 그래서 ontology arm은 projection 없이 동작하며, B의 지연은 판정에 쓰지 않는다.

**미지정 사실 레코드 초안.** 상태는 `proposed`이며, 승인되면 `verified`가 되고 검토자를 기록한다([초안](../../system/eval/b0-knowledge-system/refactoring-final2-draft-2026-10-10/knowledge-overlay/)).

| ID | 문장 | 일치 용어 묶음 | 근거 |
|---|---|---|---|
| UNK-POLICY-OWNER | knowledge 정책 레코드를 작성·승인할 정책 담당자가 지정되지 않아 정책 instance가 없다 | [정책, 담당자] / [정책, 소유자] / [policy, owner] | N14-CHANGE-ACCEPTANCE.md 34행 |
| UNK-RELEASE-VERDICT | ontology·knowledge 기능의 기본값·출시 판정이 내려지지 않았고, 그전까지 disabled·출시 보류 | [출시, 판정] / [출시, 결정] / [기본값, 판정] / [release, verdict] / [release, decision] | 작업리스트 N-18 행(줄은 동결 시 결합) |

**질문 초안**([파일](../../system/eval/b0-knowledge-system/refactoring-final2-draft-2026-10-10/part-b-questions.json)).

| ID | 기대 | 질문 |
|---|---|---|
| F2B-ABS-01 | 기권 | 이 프로젝트에서 knowledge 정책 레코드를 작성하고 승인하는 정책 담당자는 누구로 지정되어 있나요? |
| F2B-ABS-02 | 기권 | ontology와 knowledge 기능을 기본으로 켜고 출시하기로 한 판정은 언제, 누가 내렸나요? |
| F2B-ANS-01 | 인용 (ABS-01의 대조) | knowledge 정책 레코드가 verified로 받아들여지려면 어떤 필드와 날짜 조건이 필요한가요? |
| F2B-ANS-02 | 인용 (ABS-02의 대조) | 로컬 운영에서 설치한 cks 바이너리를 무엇으로 신뢰하기로 했나요? |

**판정 방식.** baseline_on(knowledge 켬)과 baseline_off에서, 무답 문항의 근거 외 인용 비율과 대조 문항의 recall을 기계적으로 집계한다.

## 4. 문제점

- **[중요] 독립성이 약하다.** 미지정 사실 레코드의 일치 용어와 무답 질문을 같은 agent가 썼다. 그래서 무답 문항이 일치하는 것은 거의 예정된 결과다. 이 결과는 "기능이 설계대로 동작한다"는 확인에 가깝고, 처음 보는 무답 질문에서도 일반화된다는 증거가 아니다.
- **[중요] 일치 규칙이 단순하다.** 용어 일치 기권은 답가능 질문에 같은 용어가 들어가면 잘못 기권할 수 있다. 대조 문항 2개로는 이 위험을 다 보지 못한다.
- **[권장] 검토자 표기.** 이전처럼 사용자가 지정한 AI 라벨("opus5.5 검토")을 쓰면, 독립적인 사람의 내용 검토가 아니라는 점을 그대로 기록한다.
- **[권장] 실행 시간.** A는 FINAL 2회 실행으로 약 2.5시간, B는 corpus 준비와 색인 약 30분에 실행 약 30분이다. 부하가 높으면 지연 측정이 흔들릴 수 있다.

## 5. 승인해 주셔야 할 것

1. A의 질문 15개와 gold(필요하면 수정·철회)
2. A의 protocol 초안(binary 비교, N10·N08 판정 규칙)
3. B의 미지정 사실 레코드 2건의 문장·일치 용어·근거
4. B의 질문 4개
5. 기록할 검토자 표기

승인하면 다음 순서로 진행한다: 동결(검토 기록과 SHA 결합) → A의 DEV 실행 → A의 FINAL 실행 → B의 corpus 준비와 색인 → B 실행 → 채점 → N08·N09·N10 판정. 승인 전에는 실행하지 않는다.

**남은 전체5개:** N08 검색 품질, N09 기권, N10 비용·지연, N16 신뢰·권한(서명 키), N18 기본값·출시 판정.


## 6. v2 수정 (2026-10-10, 동결 뒤 실행 전 중단)

**[치명] 원래 A 설계의 결함.** 동결된 N11 corpus는 선택된 83개 파일만 색인한다. 그런데 승인한 A 문항 15개 가운데 11개의 gold 파일이 CKV와 CKG 어디에도 없었다. DEV 실행 중에 이를 발견하고, FINAL 예약 전에 실행을 멈췄다([기록](../../system/eval/b0-knowledge-system/refactoring-final2-a-2026-10-10/stopped-before-final.json)). 초안을 쓸 때 색인 범위를 대조하지 않은 agent의 누락이다.

**수정.** 색인된 파일에 걸린 4문항(F2-DEV-05, F2-FINAL-03, F2-FINAL-07, F2-FINAL-10)은 유지했다. 나머지 11문항은 색인된 파일 안의 새 사실로 바꿨다. 각 문항이 대체한 원래 문항과 같은 군을 유지하므로, 구성은 DEV 5와 FINAL 10으로 그대로다. 모든 gold span이 CKV 청크와 CKG node에 있는지 기계적으로 검사하는 [스크립트](../../system/eval/b0-knowledge-system/refactoring-final2-a-2026-10-10/index-scope-check.py)를 추가했고, v2는 15문항 모두 통과했다. 입력 검사에서도 원문 오류와 관측 중복은 0건이다.

**[권장] 주의.** 교체 문항이 vector/build와 graph/contract 타입 쪽에 몰려 있다. 그리고 F2R-DEV-03(macOS 메모리 읽기)과 F2R-FINAL-06(메모리 사전 검사)은 같은 하위 시스템이다. 함수와 사실은 다르지만, 이 짝을 문제 삼는다면 교체할 수 있다.

| ID (대체 대상) | 군 | 질문 | gold |
|---|---|---|---|
| F2R-DEV-01 (F2-DEV-01) | CODE | 중단된 벡터 재색인을 이어서 할 때 남아 있는 재개 기록 파일은 어떻게 읽히고, 다른 목표 head용으로 쓰인 기록이면 어떻게 처리되나요? | `vector/build/checkpoint.go:30-55` |
| F2R-DEV-02 (F2-DEV-02) | CODE | 벡터 청크에 붙는 불변식(invariant)은 탐지 방식에 따라 몇 단계로 나뉘고 각 단계는 무엇을 뜻하나요? | `pkg/vector/types/chunk.go:90-104` |
| F2R-DEV-03 (F2-DEV-03) | WHY | macOS에서 빌드 메모리 가드가 사용 가능 메모리를 읽을 때 정확한 커널 VM 통계 API 대신 vm_stat 명령을 쓰는 이유는 무엇인가요? | `vector/build/memory_darwin.go:3-8` |
| F2R-DEV-04 (F2-DEV-04) | POLICY | 지식 팩의 워크시트 카탈로그를 불러올 때 헤더와 텍스트 필드에는 어떤 제한이 적용되나요? | `system/knowledgepack/worksheet.go:129-143` |
| F2R-FINAL-01 (F2-FINAL-01) | CODE | CKV가 빌드된 버전을 current로 올릴 때 current 포인터는 구체적으로 어떻게 교체되나요? | `vector/build/promote.go:18-22, vector/build/promote.go:46-57` |
| F2R-FINAL-02 (F2-FINAL-02) | CODE | 그래프 간선의 dispatch 종류 메타데이터는 어떤 간선에 어떤 값으로 기록되고, 구버전 reader는 이 열을 어떻게 다루나요? | `pkg/graph/types/edge.go:5-18` |
| F2R-FINAL-03 (F2-FINAL-04) | WHY | 심볼 주변 PR 이력을 조회할 때 병합 시각 기준 cutoff를 SQL 계층에서 적용하는 이유는 무엇인가요? | `pkg/graph/types/pr_ref.go:20-28` |
| F2R-FINAL-04 (F2-FINAL-05) | WHY | v2 인용의 중복 제거 키를 기존 인용 키 대신 별도 튜플로 정의한 이유는 무엇이며, 어떤 필드로 구성되나요? | `pkg/system/contract/v2.go:28-39` |
| F2R-FINAL-05 (F2-FINAL-06) | WHY | 벡터 데이터셋 빌드·재색인을 직렬화할 때 advisory flock을 쓰는 이유와, 다른 프로세스가 이미 잠금을 갖고 있으면 어떻게 되나요? | `vector/build/lock.go:15-24` |
| F2R-FINAL-06 (F2-FINAL-08) | POLICY | 임베딩 모델을 올리기 전 메모리 사전 검사는 언제 빌드를 거부하고, 언제 검사를 건너뛰나요? | `vector/build/memory.go:49-53, vector/build/memory.go:100-120` |
| F2R-FINAL-07 (F2-FINAL-09) | TRACE | 시맨틱 투영에서 요구사항 기준·개념·코드·테스트를 잇는 관계 술어는 각각 무엇에서 무엇으로 연결되나요? | `system/semantic/model.go:132-140` |

**다시 승인해 주셔야 할 것:** A v2의 교체 문항 11개. protocol과 B는 바뀌지 않았다.
