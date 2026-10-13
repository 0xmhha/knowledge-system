# 온톨로지 적용 전후 평가 입력 검토안

> 이전 단계 기록이다. 아래의 proposed/대기/완료 수는 해당 검토·실측 시점의 상태다. 현재 작업 상태와 최종 한계 판정은 [O04 최종 판정](./ONTOLOGY-FINAL-VERDICT-2026-10-13.md), [현재 인수인계](./CURRENT-HANDOFF.md)를 따른다. 과거 원자료와 수치는 변경하지 않았다.

상태: **proposed · 내용/프로토콜 검토 대기**. O04-A 완료, O04-B/C/D 미완료를 유지한다. 기존17/18·온톨로지3/4 완료, 전체 잔여 N16/O04의2개다.

목적은 승인된 WBFT 의미가 실제 검색에서 어떤 인용·본문·순위·비용 차이를 만드는지 측정하는 것이다. 개념 내용 승인, 코드와 시험의 관계 승인, 실행 기준 통과, 검색 품질 판정은 각각 다르다. 이번 자료는 앞의 내용 승인을 반복해서 요청하지 않는다.

## 검토할 질문과 후보 답

[기계 입력](../../system/eval/b0-knowledge-system/ontology-o04-evaluation-review-2026-10-13/question-book.json)은 질문 원문, 후보 답, commit/파일/행/원문 SHA, 보관 원문 사본을 포함한다. 코드 위치 gold10개는 실제 graph canonical과 vector chunk에 읽기 전용으로 대조했다. 이 대조는 IMPLEMENTED_BY/TESTED_BY 관계 승인이 아니다.

| ID | 질문 원문 | 후보 답과 gold 범위 |
|---|---|---|
| OP-01 | minimum agreement: 검증자 N=8일 때 필요한 동의 수와 F의 계산식을 찾아줘. | F=(N−1)/3(float64), ceil(N−F)=6. validator/default.go:222,226–229. 정적 계산이며 시험 실행 근거는 아니다. |
| OP-02 | 뷰 체인지에서 더 높은 preparedRound를 받더라도 justification 검증이 실패하면 최고 prepared block을 갱신하는가? | 갱신하지 않는다. 더 높은 round이고 hasMatchingRoundChangeAndPrepares가 nil일 때만 갱신한다. core/roundchange.go:266–272. |
| OP-03 | 프리페어 표가 충분한 상태에서 이미 StatePrepared인 경우에도 Prepared 전이와 broadcastCommit을 반복하는가? | 해당 분기는 표 수와 state<StatePrepared를 함께 요구하므로 반복하지 않는다. core/prepare.go:119–140. |
| OP-04 | 커밋 메시지의 digest가 현재 proposal hash와 다르면 저장과 commitWBFT 호출 전에 어디서 거부하는가? | digest 불일치에서 errInvalidMessage 반환. 저장과 quorum 분기는 뒤에 있다. core/commit.go:95–99,114–125. |
| OP-05 | BFT message handlers의 표 집합에서 동일 Source 주소의 메시지를 두 번 Add하면 Size가 늘어나는가? | 주소별 map 항목을 덮어써 키 수는 늘지 않는다. 전체 핸들러 idempotency 증명은 아니다. core/qbft_msg_set.go:66–71,84–88. |
| OP-06 | lock discipline: IsCurrentProposal의 읽기와 startNewRound의 c.current nil 확인은 각각 어떤 currentMutex 락 안에 있는가? | 각각 RLock과 Lock 구역이다. 전체 race-free 판정은 아니다. core/core.go:154–158,161–169. |
| OP-07 | 요구사항 내용이 verified이면 N=5/6/8 검증 시험도 통과했다고 말할 수 있는가? | 아니다. req-wbft-quorum 추적의 missing_code, acceptance_evaluated=false를 확인한다. 승인 spec:5–24는 기준 원문이며 실행 결과가 아니다. |
| OP-08 | 락 요구사항이 verified이면 구현과 시험의 검토된 추적 경로가 이미 있는가? | 아니다. req-wbft-current-lock 추적의 missing_code, acceptance_evaluated=false를 확인한다. 승인 spec:55–71은 내용 승인이다. |

코드 파일은 모두 consensus/wbft 하위다. spec의 정확한 범위는 기계 입력 SHA/행을 기준으로 한다. OP-07/08은 검색 질문 대신 cks.semantic.trace_requirement 읽기 호출로 검사하며, 기권·인용0개 오라클로 쓰지 않는다.

## 제안 실행과 채점 계약

[프로토콜 초안](../../system/eval/b0-knowledge-system/ontology-o04-evaluation-review-2026-10-13/protocol-proposed.json)을 승인 후 별도 파일에 동결한다. proposed 입력이나 기존 승인본을 덮어쓰지 않는다.

- 실제 held source baca2869118774c5f6219932dc76e197a47e6ecd, project p-3fa6368dd3f6757c6b6a86121f0a9b7d, 기존 O03 dataset/snapshot, BGE-M3 digest790764642607…16bab/1024를 유지한다.
- 같은 binary/질문/config로 off=baseline, on=concept_text를 비교한다. 두 모드의 Stage1은 raw-query-first다. ontology_mode와 별도 로그 경로만 달라야 한다. 각 질문의 arm 순서를 번갈아 배치한다. 첫 arm 고정에 따른 편향을 줄이지만 무작위 독립 시험이라고 주장하지 않는다.
- OP-01–06 각각 arm별 warmup2·검색5·warm지연20·cold0: 총324검색 호출, 이 중 품질 채점 대상은60검색 반복이다. OP-07/08은 별도 trace2회다. 반복은 독립 표본이 아니다.
- 기존 b1 citation_metrics의 Recall@10/Precision@10/평균 expected reciprocal rank@10을 재사용한다. 동일 commit/파일과 행 교집합으로 채점하며 질문별 반복 평균 뒤 사실군별 평균을 낸다. 넓은 함수가 gold 행과 겹치는 것은 기계적 위치 hit이며 사실 답변의 정확성 증명은 아니다. 생성 답변/사람 기권 점수는 null이다.
- 오류·누락·timeout 검색은 예정 분모에 남겨 위치 지표0으로 집계한다. warm 실패는 지연 분위수와 분리해 예정 분모의 실패율을 함께 표시한다. 누락/오류가 있으면 지연 합격을 주장하지 않는다. v2 무결성·인용/본문 원문 SHA·같은 tuple 검사를 먼저 통과해야 한다.
- 요청 범위의 /api/embed와 /api/tags 호출·실패·pending을 따로 센다. startup은 별도 표시하며 범위 밖 constructor probe의 비용은 미계측임을 남긴다. 인용 순서/구성/본문 변화를 품질 개선으로 대신하지 않는다.
- **제안 비용 예산: warm p95(on)/p95(off)≤1.25. 아직 새 범위의 승인이 아니다.** 기존 B0 동결 임계치와 입력5개의 SHA는 변경하지 않는다. 이전 DEV 관측의 비율1.3023은 참고 관측이며 이 새 계획의 합격/실패 판정으로 소급하지 않는다.

## 독립성 한계와 종료 범위 결정

작성자는 앞선 DEV 응답을 본 Codex다. OP-01–06은 quorum/round-change/message-handlers/current-lock의 **4개 제안 사실군**이며 모두 기존 DEV 사실군과 겹친다. OP-03/04/05를 독립3군으로 쪼개거나 반복5회를 독립5표본으로 세지 않는다. OP-07/08은 품질 군에 추가하지 않는다. 승인된 독립 FINAL 단위0·새 FINAL 실행0이다.

따라서 이8문항을 승인해도 새 독립 FINAL로 바뀌지 않는다. 중요군 독립 단위10개 미만은 inconclusive로 유지한다. 이 입력으로 할 수 있는 것은 검토된 **DEV 파일럿의 기술 효과·비용 관측**이다. 기본 off/opt-in과 운영 활성0, 공개 배포 보류를 유지한다.

추천은 위8문항·위치 gold·프로토콜을 DEV 파일럿 범위에서 검토하고 측정한 뒤, 결과와 부족분을 보고하는 것이다. 기존 O04-C의 독립 평가 요구를 유지하면 별도 작성/검토된 미관측 질문·gold와 독립 단위 보강이 추가로 필요하다. 파일럿 한계를 명시하고 O04 자체를 그 범위에서 종료하려면 **사용자의 명시적인 수용 범위 변경 결정**이 필요하다. 현재 문서에서는 어느 쪽도 승인 또는 종료로 기록하지 않았다.

## 준비 검증과 다음 작업

입력 검사기는 보관 원문 또는 실제 retained source를 대조하고, 중복/타 tuple/타 commit/변조된 gold/경로 탈출/독립성·승인 오기/관계 승인 오기/기준 통과 오기/질문 변경을 거부한다. 준비 검사는 평가 실행 권한을 만들지 않으며 기본 실행은 검토 대기 exit2다. 이 도구에는 MCP/모델 실행 코드가 없다.

다음 순서는 내용·프로토콜과 종료 범위 판정 → 승인 receipt/별도 frozen 입력 → 같은 조건 측정·실패 분모와 원자료 보존 → 위치 품질/비용/한계 판정 → 최종 문서·진행 원장·커밋 SHA 결합이다. N16은 기존 사용자 결정에 따라 공개키 fingerprint/독립 신뢰 조건이 미충족이며 그대로 남긴다.
