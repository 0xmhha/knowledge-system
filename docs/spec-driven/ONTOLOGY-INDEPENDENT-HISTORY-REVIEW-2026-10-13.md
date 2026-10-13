# 온톨로지 독립 입력 이력·검토 준비 — 2026-10-13

> 이전 단계 기록이다. 아래의 proposed/대기/완료 수는 해당 검토·실측 시점의 상태다. 현재 작업 상태와 최종 한계 판정은 [O04 최종 판정](./ONTOLOGY-FINAL-VERDICT-2026-10-13.md), [현재 인수인계](./CURRENT-HANDOFF.md)를 따른다. 과거 원자료와 수치는 변경하지 않았다.

현재 **O04-A/B 완료, C/D와 전체 O04 미완료**. 기존 17/18, 온톨로지 3/4, 전체 잔여 O04/N16의 2개다. 이번 작업은 독립 평가 입력을 준비한 것이며 새 FINAL이나 품질 판정을 실행하지 않았다.

## 과거 자료 대조 범위

이전 B0, N11, FINAL2와 모든 저장소 평가 JSON/JSONL/gzip 4,758개, docs 및 system/eval 아래 Markdown/YAML 1,618개를 목록화했다. 현재 후보 준비 폴더 두 곳은 자체 gold로 인한 거짓 노출을 피하려고 제외했다. 총 6,376개 파일의 SHA가 이력 포착 커밋 b4c343e633415e6865825b3062691a852dbb21ae의 바이트와 모두 같다. 후속 문서 변경은 이 과거 catalog를 갱신하지 않는다.

단독 응답·중첩 MCP 응답·structuredContent/텍스트 mirror·JSONL·여러 pretty JSON 객체를 이어 저장한 Go 의존성 형식을 지원한다. JSON 값별 64MiB 제한과 UTF-8/중복키/비유한 수 거부를 유지하며, 전체 FINAL archive 크기를 단일 값 크기로 오판하지 않는다. 원문 SHA가 같거나 동일 commit인 경우에만 행을 대조한다. 인용이 없더라도 전달 본문의 gold 문자열은 확인한다. 다른 원문 revision과 빈/잘못된 자료는 노출 없음으로 덮지 않는다.

비어 있는 query-tiny-0.json, response-matches.jsonl, whole-tree-audit.json의 3개는 증거 없는 파일로 기록했다. 나머지 6,373개는 모두 해석했다. 따라서 nonempty_catalog_fully_parsed=true이나 machine_catalog_fully_parsed=false다. 관측 응답 34,011개에서 중복을 제거한 응답 표현 1,005개, authored gold 표현 624개와 text 표현 1,329개를 대조했다. 표현 개수·반복은 독립 사실군 수가 아니다. 채팅·외부·삭제 자료의 전체 복구나 의미적 독립성 판정을 뜻하지 않는다.

## 후보 보강과 제외

기존 10문항에 8문항을 보강해 총 18문항·19 gold 범위를 준비했다. 원문 7파일 전체/행 SHA, 실제 graph canonical 포함, vector chunk 원문 해시 및 gold 전체 행 포함을 대조했다. 승인본에 존재하지 않는 current-lock 개념 ID는 준비 과정에서 실제 wbft-current-lock으로 바로잡았다. 코드 관계 assertion과 criterion 실행 수용은 0이다.

- IC-01(Size), IC-10(ROUND-CHANGE 서명 payload)은 기존 노출을 재확인했다.
- IC-13(bad prepared block 초기화), IC-14(retry current 사본)도 과거 응답에 이미 노출돼 격리했다. 중복 제거한 노출 표현은 각각 9개/1개다.
- IC-15(future view 검사)는 기존 evaluation-design-2026-05-22.md:511–517에 동일 함수의 future-view off-by-one 평가 맥락이 있어 작성자가 보수적으로 제외했다. 이것은 응답 gold 행 교집합 판정과 구별한다.

검토 대상은 **IC-02–09, IC-11/12/16/17/18의 13문항**이다. 개별 제안 12군에서 IC-11/16(큐 입구·상태 처리), IC-17/18(요청 검증·전송 입구)을 각각 합쳐 **보수적 제안 10군**으로 정리했다. IC-02/03은 기존처럼 validator-membership 한 군이다. 원 candidate book의 세부 군 제안은 보존하고 review-selection에 검토군 mapping을 기록했다. 최소 독립단위 10에 제안 수만 도달했으며, 추가 상관성 통합이나 제외가 있으면 다시 부족해진다. 독립 승인 단위 0, FINAL 가능 문항 0을 유지한다. 작성자는 이전 DEV를 본 Codex이며 같은 승인 개념군을 사용한다.

[13문항의 질문·후보 답·gold·검토군 전체 표](../../system/eval/b0-knowledge-system/ontology-o04-history-and-pool-2026-10-13/REVIEW.md), [선택·제외·제안군 mapping](../../system/eval/b0-knowledge-system/ontology-o04-history-and-pool-2026-10-13/review-selection.json), [이력 요약](../../system/eval/b0-knowledge-system/ontology-o04-history-and-pool-2026-10-13/history-summary.json), [이력 SHA 결합](../../system/eval/b0-knowledge-system/ontology-o04-history-and-pool-2026-10-13/history-catalog-binding-check.json)에 기록했다. 전체 압축 catalog는 history-audit.json.gz에 있다.

## 검증 결과와 다음 순서

입력·채점·노출 검사 70시험(신규 이력 검사 19 포함)이 통과했다. 신규 검사는 단독/구형/중첩 응답, mirror 중복 제거, authored gold, SHA/commit 차이, 인용 없는 본문, 설계 문자열과 단순 함수명 cue 구별, 빈/잘못된 JSON, 큰 gzip archive/값 크기 경계, UTF-8 경계를 확인한다. 기존 운영 HEAD/current/config와 held graph/vector/semantic DB SHA는 보존했다. 새 MCP/model 호출, 운영 전환, push/PR/merge/deploy는 0이다. 비용 진단의 no_match 이상치와 기본 off 결정은 그대로이며 성능 합격으로 재해석하지 않는다.

1. **O04-C:** 13문항의 내용·gold·개념 범위, 10군 묶음 및 의미적 독립성을 실제로 검토한다. 승인 또는 수정할 ID와 독립성을 판단할 수 없는 군을 기록한다. “다음 진행”을 그 승인으로 간주하지 않는다.
2. **O04-C:** 실제 독립 조건이 충족된 입력·프로토콜을 별도 커밋으로 사전 동결하고 같은 조건의 off/on 품질·지연·실패 분모를 평가한다. 미충족이면 보강하거나 명시적인 범위 재판정을 받는다. 소표본은 inconclusive다.
3. **O04-D:** 최종 수용/한계·사전조건과 구현/시험/원자료/current·committed SHA를 결합한 후 완료를 판정한다.
4. **N16:** 사용자 결정대로 공개키 fingerprint/독립 신뢰 조건 미충족을 유지한다. 로컬 commit/binary SHA 신뢰 범위다.

O04-C/D 체크박스는 변경하지 않는다. 코드/gold 위치 검사나 제안 군 개수는 독립 품질 수용을 대신하지 않는다. WBFT criterion trace의 missing_code와 acceptance_evaluated=false도 그대로다.
