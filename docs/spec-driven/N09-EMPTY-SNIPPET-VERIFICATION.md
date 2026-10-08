# N09 빈 snippet 검증 오판 수정

2026-10-08 · **평가 보조 함수 수정·관련 검증 완료 / N09 실제 기권 평가 미완료**.

## 목적·재현

CKV 평가 보조 함수 `VerifyHit`는 snippet을 공백 정규화한 뒤 인용 줄 범위에 포함되는지 검사한다. 빈 문자열은 모든 문자열의 부분 문자열이므로, 유효한 파일/줄 범위에 빈 snippet이나 ASCII 공백·탭·CR/LF만 있는 snippet을 넣어도 `Verified=true`였다. 내용 없는 결과를 검증된 원문 근거로 세면 평가 분모·판정을 오해할 수 있다.

기존 코드를 유지하고 새 회귀 시험만 추가해 실제 RED를 확인했다. body/blank-line 각각 empty/whitespace의4사례가 잘못된 Verified=true를 반환했고, Hits/Examples의4항목 집계는 비검증3개를 기대했지만1개였다. [실패 원자료](../../system/eval/b0-knowledge-system/refactoring-n09-empty-snippet-2026-10-08/red-test.jsonl.gz)·[재현 판정](../../system/eval/b0-knowledge-system/refactoring-n09-empty-snippet-2026-10-08/red-verification.json).

## 변경·설계 범위

[검증 함수](../../internal/vector/query/hallucination.go)에서 기존 파일·좌표 검사를 마친 뒤 정규화된 snippet이 비어 있으면 `Verified=false`, `Reason=empty_snippet`, `ExpectedFile`을 반환한다. 기존 no_src_root/file_missing/out_of_range의 우선순위를 유지한다. 비어 있지 않은 정상 본문·signature-only·공백 차이 일치는 기존 경로를 사용한다.

[회귀 시험](../../internal/vector/query/hallucination_test.go)은 원문이 있는 fixture와 실제 blank line을 각각 검증하며, 응답의 hits와 examples 순서·분모도 확인한다. 새 집계는4verdict/비검증3으로 정상 signature1, 내용 없음2, 실제 내용 불일치1을 구분한다. `empty_snippet`은 내용 불일치 `snippet_not_found`와 다른 이유이며, 사람이 답을 기권해야 한다는 의미 판정은 아니다. 기존 aggregate의 `hallucinated` 값은 모든 비검증을 세는 기존 계약이다.

이번 변경은 평가 보조 함수다. 실제 검색 결과 생성·v1/v2 출처 검증·원 corpus의 답가능 여부나 모델 기권 성능을 새로 측정하지 않았다. 새 strict 인용0 판정, N11 입력 승인/동결, source/dataset 재색인·실제 linked run/사람 수용을 수행한 것으로 기록하지 않는다.

## 검증과 환경 제약

- Darwin ARM64/Go1.26.8, 관련 query와 공개 CKV wrapper 2패키지 race: **103시험·4하위시험 통과**, **1시험 skip**, 실패0. [성공 원자료](../../system/eval/b0-knowledge-system/refactoring-n09-empty-snippet-2026-10-08/green-verified-race.jsonl.gz)·[정확한 명령/분모](../../system/eval/b0-knowledge-system/refactoring-n09-empty-snippet-2026-10-08/green-verified-summary.json).
- skip은 `TestGoStablenetBgeM3Smoke`다. 실제 go-stablenet/BGE smoke를 실행하거나 성공했다고 쓰지 않는다.
- go vet2패키지, engine boundary, gofmt/diff 검사 통과. 새 전체Go회귀/Linux 시험은 실행하지 않았다.
- 첫 수정 후 race에서는 새 회귀를 통과했지만 임시 Git fixture2개가 전역 GPG 서명을 상속해 샌드박스 서명 실패가 발생했다. 다음 시도는 Go cache 접근으로 시험 전 실패했다. 각각의 원자료와 [환경 분류](../../system/eval/b0-knowledge-system/refactoring-n09-empty-snippet-2026-10-08/sandbox-check-classification.json)를 보존했다. 최종 실행은 **그 명령에만 commit.gpgsign=false**를 지정해 소유한 임시 fixture를 만들고 Go cache 접근을 허용했다. 전역 Git 설정·실제 키를 바꾸거나 읽지 않는다.
- 모델 HTTP/새 DEV·FINAL/Claude/사람 decision0, 활동 시간null. 코드/시험2파일 외 승인 입력5개·기존 후보 책/프로토콜/양식은 유지한다.

## 완료 체크·다음

- [x] 빈/공백 snippet4사례와 응답 집계의 실제 잘못된 검증 재현.
- [x] 별도 empty_snippet 진단과 기존 정상·오류 경계 보존, 관련 race/vet/boundary/format 검증.
- [x] 환경 실패와 실제 제품 RED/성공·skip의 분리, 원자료·SHA 바인딩 기록.
- [ ] N11 실제 gold·독립군·범위/프로토콜 승인과 충분한 표본 및 새 입력/runtime 동결.
- [ ] 실제 corpus에서 사람이 답가능/기권을 판정하고 strict 인용0과 별도로 독립 FINAL 평가.

**현재 전체8/18·요청5/15·남은10개 유지.** 다음은 [N11 조사](./N11-SOURCE-PROBE-INVESTIGATION.md)의 보류 원문/군 검토와 더 넓은 미사용 사실군 확보, [범위 선택](./N11-N12-SCOPE-DECISIONS.md) 및 N12 관계별 실제 판정 반영이다. 사용자 입력이 필요한 부분은 승인 화면에만 의존하지 않고 이 대화의 텍스트로 안내한다.

**남은 전체:** N08 검색 품질, N09 기권, N10 비용·지연, N11 독립 평가, N12 실제 파일럿 검토, N14 실제 변경 수용, N15 규모·설치·이식성, N16 신뢰·권한, N17 실제 복구, N18 기본값·출시 판정.
