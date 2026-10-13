# 온톨로지 실제 DEV 파일럿 결과 — 2026-10-13

상태: **O04-A/B 완료, O04-C/D 미완료**. 기존17/18·온톨로지3/4 완료, 전체 잔여 N16/O04의2개.

사용자 “다음 진행”을 앞서 제시한8문항·DEV 프로토콜의 실행 지시로 해석해 50dea115에서 실행 전 동결했다. 독립 내용 검토자 지정이나 독립 평가 요구의 면제·파일럿 범위 종료를 추론하지 않았다. 기존 proposed 입력은 그대로 보관한다.

## 결과와 의미

324검색 응답/162쌍·별도trace2회, 캡처12회 모두 정상. 실패·누락·pending·범위 미결합0. 같은 tuple/모델/binary·질문별 같은 설정을 확인했고 v2 무결성·모든 인용/본문의 retained 원문 SHA·같은 query를 검증했다. OP-07/08 모두 missing_code/acceptance_evaluated=false이며 내용 승인을 구현·시험 완료로 바꾸지 않는다.

| 지표 | off | on |
|---|---:|---:|
| Recall@10 | 0.666667 | 0.750000 |
| Precision@10 | 0.150000 | 0.158333 |
| 평균 expected reciprocal rank@10 | 0.352431 | 0.369097 |
| warm p95(ms), arm별120회 | 454.329 | 592.742 |
| 요청 범위 embedding HTTP | 662 | 801 |
| 요청 범위 tags HTTP | 1324 | 1602 |

warm p95비 **1.3047**는 DEV 비용 예산1.25를 초과했다. 이 실행에서 기본 on을 뒷받침하는 결과는 없다. 기본 off/opt-in·운영 활성0·공개 배포 보류를 유지한다.

지표는 질문별5검색 평균 뒤 제안 사실군4개를 같은 가중치로 평균했다. OP-03/04/05를 독립3군으로 부풀리지 않는다. 위치 gold 교집합 채점이며 넓은 함수 hit가 사실 답변의 정확성을 증명하지 않는다. 작성자가 기존 DEV를 봤고 모든 군이 기존 DEV와 겹치므로 독립 승인 단위0·새FINAL0, 품질 판정은 inconclusive다. 생성 답변과 사람 기권 점수는 null이다.

| 질문 | Recall off→on | rank/body 변경(검색5쌍) | on 설명 |
|---|---:|---:|---|
| OP-01 | 1.000→1.000 | 0/0 | applied:5 |
| OP-02 | 0.000→0.000 | 5/5 | applied:5 |
| OP-03 | 0.000→1.000 | 5/5 | applied:5 |
| OP-04 | 1.000→1.000 | 5/5 | applied:5 |
| OP-05 | 1.000→1.000 | 0/0 | no_match:5 |
| OP-06 | 1.000→1.000 | 5/5 | applied:5 |

OP-03의 Prepared 전이 함수가 새로 gold에 들어왔다. OP-02는 순서/본문이 바뀌어도 필요한 highest-prepared 갱신 분기를 찾지 못했다. OP-01은 설명 applied여도 공개 인용/본문·정답 위치 순위는 유지했다. 따라서 applied를 품질 성공으로 해석할 수 없다. OP-05의 “BFT message handlers의”는 Latin 용어 뒤 한글 조사를 단어 경계로 인정하지 않는 현재 matcher 정책에서 no_match였다. 관측 뒤 질문이나 gold·matcher를 바꿔 이번 결과에 소급 적용하지 않았다.

## 비용 원자료의 추가 대조

새 모델 호출 없이 저장된 request scopes를 분석했다. 매칭되는5문항은 warm 요청당 embed HTTP가 약4.05–4.10회에서5.10회로 늘었다. no_match인 OP-05는4.05→4.10회다. tags도 각 embed 전후에 측정된다. 이는 추가 호출 경로의 관측이며 전체 지연 증가의 원인을 단독으로 증명하지 않는다.

startup scopes는 별도로 보관했고 constructor probe의 범위 밖 호출은 미계측이다. 중첩 logical embedding/HTTP 시간은 겹칠 수 있어 독립 비용처럼 합산하지 않는다. 질문별 arm 순서 교대·warmup2/검색5/warm20/cold0이며 무작위/quiet-machine/cold acceptance가 아니다. 측정 중 입력/채점 회귀와 진행 guard11시험(11.025초)을 같은 머신에서 실행했다. 공유 자원의 영향을 배제하지 않았으므로 이 p95를 운영 SLA나 독립 성능 수용으로 일반화하지 않는다.

## 검증·보존·다음 작업

입력/채점39시험과 진행guard11시험 통과. 실제 retained 응답의 verifier 통과와 다시 해시한 foreign tuple 거부를 확인했다. 원래5개 B0 동결 입력과 의미 승인본은 변경하지 않았다. held graph/vector/semantic DB SHA, 원본 프로젝트 HEAD·운영 config SHA/current target을 보존했다.

[실행 승인·동결 입력·압축 원자료](../../system/eval/b0-knowledge-system/ontology-o04-dev-pilot-2026-10-13/)에 캡처와 비교, 실패 분모, 호출·지연, 범위 한계를 보관한다. O04-B는 같은 조건의 실제 DEV 기술 대조 완료이며 독립 품질 수용은 아니다. C의 독립 평가 부족과 D의 최종 수용 기록은 미완료로 유지한다.

다음 우선순위는 추가 embedding·projection 검증의 지연 기여를 분리하는 진단과 미관측 독립 질문/gold/사실군 보강이다. 최종 독립 평가·비용 판정 후 C/D의 근거·진행 원장을 연결한다. 파일럿 한계를 명시하고 O04를 종료하려면 사용자의 명시적인 범위 변경이 필요하다. N16은 공개키 fingerprint·독립 신뢰 조건 미충족 상태를 기존 사용자 결정대로 유지한다.
