# 독립 평가 입력 검토 자료 — 2026-10-13

상태: O04-C/D 미완료. 독립 승인 0, FINAL 실행 가능 문항 0.

## 확인한 이력과 제외

JSON/JSONL/gzip 4,758개와 Markdown/YAML 1,618개를 목록화했다. 빈 파일 3개는 그대로 기록했고, 나머지 6,373개를 해석했다. 관측 응답 34,011개에서 동일 응답 표현을 제거한 1,005개, authored gold 표현 624개를 대조했다. 중복과 반복은 독립 표본으로 세지 않는다. 본문은 인용 누락 시에도 실제 gold 코드 문자열을 대조한다.

IC-01·10·13·14는 과거 인용/본문 노출로 격리했다. IC-15는 기존 설계에 미래 view 오류 평가 맥락이 있어 작성자가 보수적으로 제외했다. 해당 제외는 같은 행의 응답 노출 판정과 구별한다. 모든 채팅·외부·삭제 이력을 복구했다는 뜻은 아니다.

## 검토할 질문·후보 답·근거

전체 경로 접두사는 consensus/wbft/이며 원문 SHA·행 SHA·canonical·chunk는 candidate-book.json 및 source-index-check.json에 있다. source copies는 해당 원문 전체 파일이다. 검색 호출이나 관계 assertion 승인을 하지 않았다.

| ID | 질문 | 후보 답 | 근거 | 제안 검토군 |
|---|---|---|---|---|
| IC-02 | validator 집합에 이미 있는 주소를 AddValidator로 추가하면 키와 quorum 입력 수는 어떻게 되는가? | 주소가 같으면 false를 반환하고 append를 하지 않는다. 실제 quorum 재계산 또는 테스트 통과를 주장하지 않는다. | validator/default.go:186–196 | validator-membership |
| IC-03 | RemoveValidator에서 대상 주소를 찾지 못하면 무엇을 반환하며 목록을 바꾸는가? | 찾지 못하면 false를 반환하며 삭제를 수행하지 않는다. 찾으면 목록을 재구성하고 true를 반환한다. | validator/default.go:198–209 | validator-membership |
| IC-04 | 라운드 체인지 타이머에 양수 MaxRequestTimeoutSeconds가 설정된 경우 timeout을 어떻게 증가시키고 overflow를 처리하는가? | baseTimeout을 시작으로 round만큼2배씩 증가시키며 상한 초과 시 상한으로 제한한다. timeout<baseTimeout인 overflow 의심 경로도 상한으로 설정한다. 상한이0일 때의 다른 경로와 혼동하지 않는다. | core/core.go:347–373 | round-timeout |
| IC-05 | 더 높은 여러 round에서 같은 validator가 보낸 ROUND-CHANGE는 higherRoundMessages에서 여러 표로 계산되는가? | 주어진 round보다 높은 round만 대상으로 주소 map을 만들고 len(addresses)를 반환하므로 여러 round에 같은 주소가 있어도1개로 센다. | core/roundchange.go:280–293 | higher-round-count |
| IC-06 | getMinRoundChange에서 현재보다 큰 round가 없으면 nil을 반환하는가? | 아니다. 큰 round의 키가 없으면 인자로 받은 round를 반환하며, 있으면 오름차순 정렬한 첫 키를 반환한다. | core/roundchange.go:308–323 | round-selection |
| IC-07 | ClearLowerThan은 이전 round만 지우는가, 메시지가 빈 집합도 함께 지우는가? | 빈 집합 또는 기준보다 작은 round를 지우고 roundChanges/highestPreparedRound/highestPreparedBlock/prepareMessages의 네 map에서 해당 키를 함께 삭제한다. | core/roundchange.go:326–338 | round-pruning |
| IC-08 | prepared 상태가 없는 ROUND-CHANGE quorum을 셀 때 PreparedRound와 PreparedDigest에는 어떤 조건이 필요한가? | PreparedRound가 nil 또는0이고 PreparedDigest가 zero hash여야 카운트한다. nilCount가 quorumSize와 같아지면 nil을 반환한다. 전부 돌아도 충족하지 못하면 error다. | core/justification.go:75–87 | nil-prepared-justification |
| IC-09 | checkValidatorSignature는 언제 현재 validator 집합 대신 이전 validator 집합을 사용하는가? | view가 현재보다 작고 상태가 StateAcceptRequest이며 이전 sequence/current.sequence-1 및 PriorRound와 정확히 같은 view이고 PriorValidators가 nil이 아닐 때만 이전 집합을 사용한다. 그 외는 현재 valSet이다. | core/core.go:433–443 | previous-view-signer-selection |
| IC-11 | BFT message handlers에서 setState를 현재와 같은 StateAcceptRequest로 다시 호출해도 pending request와 backlog를 처리하는가? | state 값이 달라질 때만 값과 로그를 바꾸지만, StateAcceptRequest이면 processPendingRequests를 호출하고 모든 호출에서 processBacklog를 호출한다. 실제 큐 처리 성공이나 동시성 준수까지 이 함수만으로 판단하지 않는다. | core/core.go:289–302 | message-queue-entry-and-state-drain |
| IC-12 | BFT message handlers의 PrepareSeal은 round와 sealType을 서명 대상에 어떻게 결합하는가? | header 사본에 WBFTHashWithRoundNumber(round)를 적용한 바이트 뒤에 byte(sealType)을 붙이고 Keccak256Hash의 바이트를 반환한다. 서명 생성이나 검증의 성공을 증명하는 함수는 아니다. | core/core.go:446–450 | seal-domain-hash |
| IC-16 | BFT 메시지 핸들러에서 자신의 주소로부터 온 메시지를 addToBacklog에 넘기면 큐에 저장하는가? | msg.Source가 c.Address와 같으면 경고 후 바로 반환한다. 다른 주소는 backlogsMu를 잡고 주소별 큐를 필요시 생성한 뒤 메시지를 view 기반 우선순위로 넣는다. | core/backlog.go:185–206 | message-queue-entry-and-state-drain |
| IC-17 | BFT 메시지 핸들러의 checkRequestMsg는 nil 요청과 없는 current, 과거·미래 proposal number를 어떻게 구분하는가? | request 또는 Proposal이 nil이면 errInvalidMessage, current가 nil이면 errCurrentIsNil이다. current.sequence가 proposal.Number보다 크면 errOldMessage, 작으면 errFutureMessage, 같으면 nil이다. 이것은 입구 분류이고 실제 합의 수용은 별도다. | core/request.go:63–79 | request-entry-validation-and-broadcast |
| IC-18 | BFT 메시지 핸들러의 handleRequest에서 유효한 요청을 받아도 round가 양수이면 바로 PRE-PREPARE를 보내는가? | checkRequestMsg 통과 후 pendingRequest를 저장하지만, StateAcceptRequest이면서 current.Round가 0일 때만 sendPreprepareMsg를 호출한다. 양수 round에서는 여기서 즉시 전송하지 않는다. ROUND-CHANGE quorum 후 전송 성공을 이 함수로 증명하지 않는다. | core/request.go:33–57 | request-entry-validation-and-broadcast |

## 판정이 필요한 부분

1. 13문항 각각의 질문·후보 답·gold 범위가 소스 사실을 정확히 설명하는지, 승인 또는 수정할 ID를 지정한다.
2. 개념 범위와 사실군 묶음을 검토한다. IC-02/03은 validator-membership, IC-11/16은 큐 입구·상태 처리, IC-17/18은 요청 검증·전송 입구로 묶었다. 나머지 군도 의미적으로 상관되면 합쳐야 한다.
3. 이력 제외 범위와 의미적 독립성을 실제로 판단한다. 제안 10군은 최소치이며 승인된 독립 단위가 아니다. 작성자는 이전 DEV를 봤다. 독립성을 판단할 수 없는 군은 그렇게 기록한다.
4. 검토 후 실제 독립 조건이 충족될 때 입력·실행 조건·채점 규칙을 커밋으로 사전 동결한다. 그 전 새 FINAL을 실행하지 않는다. 부족하면 보강 또는 명시적인 평가 범위 재판정이 필요하다.

## 진행과 남은 전체 작업

- O01/O02/O03 완료; O04-A/B 완료; O04-C/D 미완료. 기존 17/18, 온톨로지 3/4.
- 다음: 위 입력 내용·gold·범위·사실군·의미적 독립성 검토 → 조건 충족 시 별도 동결 → off/on 품질·지연·실패 평가 → 최종 수용과 근거 결합.
- 전체 잔여: O04, N16. N16은 사용자 결정대로 공개키 fingerprint/독립 신뢰 조건 미충족을 유지한다.
- 기본 off·opt-in 유지. 이번에는 운영 소스/current/config/DB, 모델, push/PR/merge/deploy를 변경하지 않았다.
