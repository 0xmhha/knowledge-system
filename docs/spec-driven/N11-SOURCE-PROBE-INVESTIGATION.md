# N11 추가 원문 조사와 후보 보류

2026-10-08 · **원문 조사·노출 대조 검증 완료 / N11 전체 미완료**. 사용자 요청에 따라 Codex가 직접 수행했다. Claude 실행/Anthropic 전송은0이다.

목적은 새 독립 FINAL의 부족분을 채울 수 있는 실제 원문을 찾고, 이미 개발·관측한 사실을 새 질문으로 바꿔 표본 수를 부풀리는 일을 막는 것이다. [조사 catalog](../../system/eval/b0-knowledge-system/refactoring-n11-source-probes-2026-10-08/probe-catalog.json)는 실행 책이나 승인 gold가 아니다.

## 조사 결과

| 조사 | 원문 사실 | 처리 |
|---|---|---|
| P01 | legacy context source-mode 가드 | 이전 A4 문서 범위와 겹쳐 격리 |
| P02 | registeredDir 경로 정규화·Lstat 디렉터리 허용 | 원문 기반 초안 보류: 기존 pack 상대 경로·링크 거부 및 DEV maintenance 범위와 의미 중복 검토 필요 |
| P03 | UTF-8 rune/4 토큰 추정 | 기존 heuristic·Phase E 교체 요구 범위로 격리 |
| P04 | keyword rerank 최고점·case-fold | 코드 주석의 과거 측정·튜닝 범위로 격리 |
| P05 | neighbor target별 max와 provenance | 기존 Stage3 설계·튜닝 범위로 격리 |
| P06 | Markdown/CKV commit·file·행 교집합 | 기존 의미 계약으로 격리 |
| P07 | corpus 파일명의 ID digest와 content digest 구분 | 원문 기반 초안 보류: 기존 결정적 corpus·manifest 계약을 세분화한 질문일 가능성 검토 필요 |
| P08 | fully linked trace의 advisory 실행 계획 | 기존 계획·실행/수용 구분 범위로 격리 |

**6격리·2보류 초안, 신규 승인 독립군0**이다. 두 초안을 기존13FINAL/8제안군 책에 넣지 않았다. P02는 CODE의 구현 설명이며 조직 POLICY로 자동 분류하지 않는다. P07도128비트 파일명 충돌 불가능·전체 corpus 원자성·실제 사람 검토를 보장하지 않는다. [구체적인 이전 원문 근거](../../system/eval/b0-knowledge-system/refactoring-n11-source-probes-2026-10-08/scope-overlap-review.json)에 파일/행/바이트 SHA와 보류 사유를 기록했다.

## 검증과 한계

- reference `982334b0`에서8조사 span과 기존 제외 CAND05의 검증용 span을 확인했다. source file/span SHA와 현행 바이트 모두 일치한다. 기존 후보의 span 교집합도 기록했다.
- 고정 커밋의 docs/system-docs/graph-docs/vector-docs Markdown **259개**를 inventory/SHA로 보존하고 선택 함수명·파일명의 literal **34언급**을 조사했다. `lock.go`가 `concurrency_underlock.go`의 일부인 경우처럼 단순 basename 일치는 해당 의미 사실의 노출 증명이 아니다. 수동 원문 범위 대조를 별도로 남겼다. 문서 전체 의미 노출·사람의 과거 노출을 완전히 판정한 감사는 아니다.
- 보관27gzip/11,632행의 압축/해제 SHA·행 수와 Git 바이트를 재확인했다. structured object11,631/body71,542/graph neighbor0, structured 문자열1,778,430개다. 기존 감사에서 두 번째 표면으로 제외한 **response.content 문자열23,262개**도 별도로 검사했다. 두 표면의 반복을 독립 표본으로 세지 않는다.
- 같은 full span/8nonempty행/긴 정보1행 방법과 선택 identifier literal을 사용했다. 전체 response의 고유 문자열은764개다. 8조사의 method/좌표 일치0은 의미 독립성·오염 부재를 증명하지 않는다. 실제 graph neighbor가0이므로 채워진 graph를 이번에 검증했다고 쓰지 않는다.
- 실제 기존 제외 CAND05를 양성 검증 기준으로 넣어 **480행에서 same-file-SHA citation 교집합 및 정보1행 일치**를 다시 확인했다. 기존 제외를 유지한다. response.content 표면에서는 이 정확 행 방법의 일치0이며 JSON 이스케이프/재서술까지 검사한 것으로 확대하지 않는다.
- 초기8조사 결과, 기존 제외 기준을 추가한 결과, system/docs까지 늘린 결과를 별도 보존했다. 최종 driver는 문서 inventory를 고정 revision의 Git tree에서 읽어 이후 문서 편집·새 문서를 과거 노출 자료에 혼합하지 않는다. 재생 과정에서 summary JSON의 set 순회로 키 삽입 순서가 달라지는 문제가 있었다. 해석된 JSON 값은 같았고 원자료는 보존했으며, 종류 순서를 정렬해 최종 새 출력 재생4파일 byte-identical 및 기존 출력 디렉터리 재시도 거부·바이트 보존을 확인했다. 이는 감사 산출물 직렬화 문제이며 제품 실패나 새 모델 관측이 아니다. [재생 검증](../../system/eval/b0-knowledge-system/refactoring-n11-source-probes-2026-10-08/committed-doc-replay.json).

생산 코드/기존 책/protocol/승인 입력5개 변경0, 모델 HTTP/새 FINAL/사람 decision0, 사람 활동 시간null이다. 이번 작업은 원문·노출 조사이며 Go 시험/품질·비용 평가를 새로 실행하지 않았다.

## 체크와 다음 단계

- [x] 실제8범위·검증용 기존 제외 원문과 바이트 확인.
- [x] 보관 응답 두 표면/인용 좌표·커밋 문서 범위 대조, 기존480노출 검증.
- [x] 6격리·2보류 초안의 이유와 비승인 상태, immutable 재생·재시도 거부 기록.
- [ ] 기존13후보/8군 및 DEV의 실제 gold·군/독립성·오염·ABS/프로토콜 검토.
- [ ] POLICY/TRACE 범위와 N12 version2 명세 선택·관계별 판정, 실제 검토자 기록.
- [ ] 각 중요군10승인 독립 FINAL을 확보하고 source/model/runtime을 동결한 뒤 평가.

**현재 전체8/18·요청5/15·남은10개.** 다음은 P02/P07의 구체적 중복 판정과 더 넓은 미사용 사실군 확보, 기존 N11/N12 실제 판정 반영이다. 기존 pool DEV7/검토FINAL13/8제안군/제외6과 승인0은 유지한다. corpus/권위/시점 부재를 확인하지 않은 ABS를 새로 만들지 않는다.

**남은 전체:** N08 검색 품질, N09 기권, N10 비용·지연, N11 독립 평가, N12 파일럿 검토, N14 실제 변경 수용, N15 규모·설치·이식성, N16 운영 신뢰·권한, N17 실제 복구, N18 기본값·출시 판정.
