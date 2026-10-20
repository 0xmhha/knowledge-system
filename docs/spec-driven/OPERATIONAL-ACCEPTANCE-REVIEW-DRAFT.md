# 운영 검증 정답·리뷰 초안 검토 자료

2026-10-15. 이 자료는 Codex가 고정된 Go 원문을 직접 읽어 작성한 **AI 초안**이다. 새 독립 gold·품질 통과·사람 수용은0이다. public 입력과 gold 초안을 분리했고 [원장](./OPERATIONAL-ACCEPTANCE-PROGRESS.json)의 단계0은 preparing으로 유지한다.

## 이번에 준비한 내용

| 자료 | 수 | 현재 판정 |
|---|---|---|
| 공개 review 입력 초안 |10개 /8개 모듈| 미승인, DEV/FINAL 미정 |
| source 기반 review rubric 초안 |10개| 사람 원문 검토 필요 |
| 값·제어 의존 anchor 초안 |3개 함수| 비완전, precision/recall 채점 불가 |

[공개 입력 JSON](./operational-preparation/review-drafts/public-inputs.json)에는 정답·우선순위·결함 확정 label을 넣지 않았다. [별도 gold 초안 JSON](./operational-preparation/review-drafts/gold-drafts.json)은 실제 host가 읽는 입력에 포함하지 않는다. 실제 실행 공간에는 public 자료와 필요한 원문 excerpt만 별도로 복사하며 이 디렉터리 전체를 mount/add-dir로 허용하지 않는다. 현재는 모델·agent·node 실행을 하지 않았다.

## 입력 선정에서 수정한 범위

SRC-026은 miner/stress/clique/main.go의 개발 stress 프로그램이다. 함수40개 source 후보 pool에는 준비 관측으로 남기되 이번 production source review10개에서는 제외하고 SRC-027 worker.newWorkLoopOrigin으로 대체했다. 다른40개 후보나 원래 독립 FINAL을 완료로 변경하지 않았다. 이 선정은 prototype 스트레스/closure 편향을 포함하는 사람이 검토할 초안이며 대표 표본이 아니다.

## 원문을 보며 검토할10개 항목

원문 excerpt는 원래 commit0bf2f4d1bfeb6605006d556957ef8c045d8f8ed8의 함수 bytes다. 각 링크 파일의 첫 줄은 표의 시작 line에 대응한다. 전체 file/function/span SHA는 JSON에 있다. 아래 사실은 외부 callee나 실제 실행을 확인한 결함 판정이 아니다.

### SRC-001 — Status

원문: [consensus/wbft/backend/api.go:169–220](./operational-preparation/review-drafts/SRC-001-source.txt).

| Anchor | 원문 line | 기대 사실 초안 |
|---|---|---|
| SRC-001-R1 | 171–174 | range 계산 오류를 먼저 반환한다; 이 지점에서 validator 조회는 아직 수행하지 않았다. |
| SRC-001-R2 | 177–180 | validator 조회 오류를 반환한다. |
| SRC-001-R3 | 183–190 | counter들을 초기화하고 start..end 블록을 analyzeBlock으로 순회하여 roundDistribution을 갱신한다. |
| SRC-001-R4 | 193–200 | round count가0이면 삭제하고 아니면 round*count를 totalRounds에 누적한다. |
| SRC-001-R5 | 201–219 | 결과에 block 범위·counter·round 통계를 반환한다. |

사람이 함께 확인할 한계: analyzeBlock의 map 갱신 효과·범위 제한·잠금은 이 함수 본문만으로 확정하지 않는다. 블록 수에 비례하는 조회 비용과 실제 실행시간은 미측정이다.

판정: pending. source 사실의 정확성, 호출자/callee를 더 봐야 할 범위, 필수 중요 항목과 우선순위를 채택·수정·기각할 수 있다.

### SRC-002 — Gossip

원문: [consensus/wbft/backend/backend.go:176–210](./operational-preparation/review-drafts/SRC-002-source.txt).

| Anchor | 원문 line | 기대 사실 초안 |
|---|---|---|
| SRC-002-R1 | 177–185 | payload hash를 knownMessages에 넣고 자기 주소를 제외한 validator 주소로 targets를 만든다. |
| SRC-002-R2 | 186–188 | broadcaster가nil이 아니고 targets가 비어 있지 않을 때 peer들을 순회한다. |
| SRC-002-R3 | 189–200 | peer가 hash를 이미 알면 건너뛰고, 신규 peer cache와 hash를 갱신한다. |
| SRC-002-R4 | 202–206 | 기본 outboundCode는 istanbulMsg이고 등록된 message code이면 입력 code를 사용한다; 전송은 goroutine으로 시작한다. |
| SRC-002-R5 | 209–209 | 전송 goroutine의 성공/실패를 기다리지 않고 nil을 반환한다. |

사람이 함께 확인할 한계: 고루틴의 전송 완료·에러·payload 소유권과 cache 동시성 안전성은 별도 확인해야 한다. nil 반환을 peer 수신 성공으로 판정하지 않는다.

판정: pending. source 사실의 정확성, 호출자/callee를 더 봐야 할 범위, 필수 중요 항목과 우선순위를 채택·수정·기각할 수 있다.

### SRC-006 — ReadHeaderRange

원문: [core/rawdb/accessors_chain.go:295–340](./operational-preparation/review-drafts/SRC-006-source.txt).

| Anchor | 원문 line | 기대 사실 초안 |
|---|---|---|
| SRC-006-R1 | 297–304 | count==0이면 즉시 반환하며 count-1>number일 때 count를 number+1로 조정한다. |
| SRC-006-R2 | 305–319 | Ancients 경계와 canonical hash를 이용해 live header를 역순 조회한다; 데이터가 없으면 loop를 중단한다. |
| SRC-006-R3 | 321–330 | 남은 count가0이면 반환; 아니면 maxBytes=count*8000으로 AncientRange를 호출하며 오류 시 이미 모은 결과를 반환한다. |
| SRC-006-R4 | 331–338 | ancient 결과 개수가 count와 다르면 부분 결과를 반환하며 일치하면 결과 순서를 역전하여 append한다. |
| SRC-006-R5 | 324–326 | 주석에는 cap at 2M이 있지만 이 위치의 계산식에는 명시적인 2M 상수 clamp가 없다. |

사람이 함께 확인할 한계: 상위 호출의 요청 제한과 uint64 산술 입력 범위·overflow 가능성은 따로 확인해야 한다. 부분 반환을 완전한 요청 충족 또는 DB 안전성 입증으로 해석하지 않는다.

판정: pending. source 사실의 정확성, 호출자/callee를 더 봐야 할 범위, 필수 중요 항목과 우선순위를 채택·수정·기각할 수 있다.

### SRC-007 — ReadAllBadBlocks

원문: [core/rawdb/accessors_chain.go:875–889](./operational-preparation/review-drafts/SRC-007-source.txt).

| Anchor | 원문 line | 기대 사실 초안 |
|---|---|---|
| SRC-007-R1 | 876–879 | DB Get 오류가 있으면 nil을 반환한다. |
| SRC-007-R2 | 880–883 | RLP decode 오류가 있으면 nil을 반환한다. |
| SRC-007-R3 | 884–888 | 디코딩된 badBlock 각각의 header/body/withdrawals를 이용해 Block을 구성하고 slice를 반환한다. |

사람이 함께 확인할 한계: Get/decode 오류와 비어 있는 목록이 반환값만으로 구분되는지 호출자 요구를 검토한다. decode의 pointer write·range binding은 scalar reaching-definition의 확정 근거로 쓰지 않는다.

판정: pending. source 사실의 정확성, 호출자/callee를 더 봐야 할 범위, 필수 중요 항목과 우선순위를 채택·수정·기각할 수 있다.

### SRC-011 — DumpToCollector

원문: [core/state/dump.go:118–213](./operational-preparation/review-drafts/SRC-011-source.txt).

| Anchor | 원문 line | 기대 사실 초안 |
|---|---|---|
| SRC-011-R1 | 120–135 | nil config를 기본 config로 바꾸고 OnRoot를 호출한 뒤 iterator 생성 오류이면 로그 후 nil을 반환한다. |
| SRC-011-R2 | 138–142 | account RLP decode 오류는 panic으로 처리한다. |
| SRC-011-R3 | 154–180 | 주소 preimage 부족과 OnlyWithAddresses 조건에 따른 skip 및 SkipCode/SkipStorage 조건을 처리한다; storage trie 오류는 해당 account 처리를 건너뛴다. |
| SRC-011-R4 | 183–203 | storage decode 오류는 storage loop에서 continue; OnAccount 후 count를 올리고 Max에 도달하면 다음 iterator key를 얻어 중단한다. |
| SRC-011-R5 | 206–212 | missingPreimages 경고와 종료 로그를 남기고 nextKey를 반환한다. |

사람이 함께 확인할 한계: collector/trie의 외부 상태·callback 동시성·실제 메모리 비용과 panic의 호출자 계약은 미확인이다. 이 함수가 dump의 완전성을 보장한다고 단정하지 않는다.

판정: pending. source 사실의 정확성, 호출자/callee를 더 봐야 할 범위, 필수 중요 항목과 우선순위를 채택·수정·기각할 수 있다.

### SRC-016 — Init

원문: [core/txpool/blobpool/blobpool.go:347–436](./operational-preparation/review-drafts/SRC-016-source.txt).

| Anchor | 원문 line | 기대 사실 초안 |
|---|---|---|
| SRC-016-R1 | 348–363 | reserver를 저장하고 Datadir가 있으면 queue/limbo directory를 만든다; 생성 오류는 반환한다. |
| SRC-016-R2 | 367–374 | head state 조회 실패 시 empty-root state로 재시도하고 둘 다 실패하면 오류를 반환한다. |
| SRC-016-R3 | 378–398 | index callback으로 parse 실패 ID를 모으고 store를 연다; invalid 항목 삭제가 실패하면 Close 후 오류를 반환한다. |
| SRC-016-R4 | 402–425 | index recheck·fee/heap 생성·limbo 연결·gasTip 설정을 수행한다; newLimbo 실패 시 Close 후 오류를 반환한다. |
| SRC-016-R5 | 429–435 | stored가 Datacap을 초과하는 동안 drop하고 metrics를 갱신한 뒤 nil을 반환한다. |

사람이 함께 확인할 한계: Close/drop/parseTransaction의 실제 자원 해제·진행 보장·잠금은 별도 source와 실행으로 확인해야 한다. callback 내부 mutation은 outer 함수의 scalar def-use gold로 자동 확정하지 않는다.

판정: pending. source 사실의 정확성, 호출자/callee를 더 봐야 할 범위, 필수 중요 항목과 우선순위를 채택·수정·기각할 수 있다.

### SRC-021 — eventLoop

원문: [eth/downloader/api.go:67–130](./operational-preparation/review-drafts/SRC-021-source.txt).

| Anchor | 원문 line | 기대 사실 초안 |
|---|---|---|
| SRC-021-R1 | 69–87 | StartEvent subscription·subscription map·60초 timer를 만들고 timer.Stop을 defer한다. |
| SRC-021-R2 | 91–101 | subscription 등록/해제 요청을 처리하며 event channel에서 nil이 오면 반환한다. |
| SRC-021-R3 | 104–111 | StartEvent를 받으면 started=true; 아직 시작하지 않은 timer tick에는 timer를 reset하고 continue한다. |
| SRC-021-R4 | 112–127 | 진행 중에는 status notification을 보내고 timer를 reset; 완료면 subscriber에 false를 보내고 done=true로 둔다. |
| SRC-021-R5 | 118–125 | notification 채널 전송에는 이 위치에서 별도 timeout/exit select가 없다. |

사람이 함께 확인할 한계: 채널 buffer와 소비자·subscription 종료 계약 없이는 deadlock/누수를 확정하지 않는다. eventLoop의 exit는 timer.Stop을 수행하나 이 본문에는 sub.Unsubscribe 호출이 보이지 않는다.

판정: pending. source 사실의 정확성, 호출자/callee를 더 봐야 할 범위, 필수 중요 항목과 우선순위를 채택·수정·기각할 수 있다.

### SRC-027 — newWorkLoopOrigin

원문: [miner/worker.go:510–605](./operational-preparation/review-drafts/SRC-027-source.txt).

| Anchor | 원문 line | 기대 사실 초안 |
|---|---|---|
| SRC-027-R1 | 511–520 | WaitGroup Done과 timer.Stop을 defer하고 초기 timer tick을 비운다. |
| SRC-027-R2 | 523–545 | commit closure는 기존 interrupt를 갱신하고 새 request를 전송하거나 exitCh에 반응한다; clearPending closure는 pendingMu 아래 오래된 항목을 삭제한다. |
| SRC-027-R3 | 549–568 | start/head event는 pending 정리·timestamp 갱신·commit을 요청하고 timer event는 실행/합의 조건 및 새 tx 여부를 확인한다. |
| SRC-027-R4 | 571–599 | 수동 interval을 최소값으로 보정하며 feedback 분기에서 recalcRecommit을 적용하고 hook이 있으면 호출한다. |
| SRC-027-R5 | 601–602 | outer select의 exitCh에서 return한다. |

사람이 함께 확인할 한계: closure 내부·atomic/lock·channel happens-before는 지역 scalar 분석의 지원 범위와 구분한다. 실제 timeout·cancel·timer reset race와 비용은 미측정이며 mutex 사용만으로 전체 동시성 안전성을 입증하지 않는다.

판정: pending. source 사실의 정확성, 호출자/callee를 더 봐야 할 범위, 필수 중요 항목과 우선순위를 채택·수정·기각할 수 있다.

### SRC-031 — Resolve

원문: [p2p/discover/v4_udp.go:175–201](./operational-preparation/review-drafts/SRC-031-source.txt).

| Anchor | 원문 line | 기대 사실 초안 |
|---|---|---|
| SRC-031-R1 | 177–185 | 직접 RequestENR 성공 시 반환; 실패하면 더 높은 sequence의 table node로 n을 바꾸고 다시 요청한다. |
| SRC-031-R2 | 188–192 | secp256k1 key를 load할 수 없으면 현재 n을 반환하고 가능하면 pubkey lookup을 수행한다. |
| SRC-031-R3 | 193–200 | lookup 결과의 ID가 현재 n과 같은 항목을 요청하여 성공하면 반환하며, 모두 실패하면 현재 n을 반환한다. |

사람이 함께 확인할 한계: lookup/request의 deadline·반환 identity 검증·외부 node의 mutation은 별도 확인 대상이다. 함수 반환을 네트워크상의 최신 node 확보 성공으로 항상 해석하지 않는다.

판정: pending. source 사실의 정확성, 호출자/callee를 더 봐야 할 범위, 필수 중요 항목과 우선순위를 채택·수정·기각할 수 있다.

### SRC-036 — DialOptions

원문: [rpc/client.go:193–223](./operational-preparation/review-drafts/SRC-036-source.txt).

| Anchor | 원문 line | 기대 사실 초안 |
|---|---|---|
| SRC-036-R1 | 194–202 | URL parse 오류를 반환하고 options를 cfg에 적용한다. |
| SRC-036-R2 | 205–219 | http/https·ws/wss·stdio·빈 scheme에 대해 각각 transport constructor를 선택하고 알 수 없는 scheme이면 오류를 반환한다. |
| SRC-036-R3 | 209–213 | WS constructor 오류는 반환하고 성공한 rc를 reconnect에 넣는다. |
| SRC-036-R4 | 222–222 | ctx·cfg·선택한 reconnect를 newClient에 전달한다. |

사람이 함께 확인할 한계: constructor/newClient의 재연결·TLS·context 취소·resource 소유권은 callee 원문 및 실행이 필요하다. scheme 선택을 실제 연결 성공 또는 transport 안전성 판정으로 쓰지 않는다.

판정: pending. source 사실의 정확성, 호출자/callee를 더 봐야 할 범위, 필수 중요 항목과 우선순위를 채택·수정·기각할 수 있다.

## 값·제어 의존 anchor3개

| 함수 | data anchor | control anchor | 채점 전에 남은 내용 |
|---|---|---|
| SRC-007 / ReadAllBadBlocks | 4 | 3 | 전체 edge/occurrence·declaration identity·제어 의미·negative·review·분리 |
| SRC-036 / DialOptions | 7 | 2 | 전체 edge/occurrence·declaration identity·제어 의미·negative·review·분리 |
| SRC-031 / Resolve | 4 | 3 | 전체 edge/occurrence·declaration identity·제어 의미·negative·review·분리 |

definition/use는 statement line 위치의 lexical may-reach 초안이다. call 결과의 값·메모리 alias·실행 경로 가능성은 주장하지 않는다. SRC-007의 range/pointer write, SRC-036의 option 외부 mutation, SRC-031의 ENR/network 효과는 별도 unknown이다. source nesting을 CFG control dependence와 동일하다고 자동 매칭하지 않는다. 이 anchor 목록은 exhaustive gold가 아니므로 WI-1의 precision/recall 합격 판정을 산출할 수 없다.

## 실행 경계에서 추가 확인한 내용

coding-agent의 Stop hook은 auto-mode의 비terminal state에서 실행을 다시 이어가도록 요청한다. 평가 gate에서 prompt로만 멈추면 재진입해 push/PR 단계로 갈 수 있으므로 기존 Stop 동작과 게시 경계를 함께 다뤄야 한다. interactive mode는 source에 있는 옵션이나 이를 지정하는 것만으로 게시가 차단됐다고 판정하지 않는다. required hook을 끄거나 가짜 COMPLETION을 써서 해결하지 않는다.

실행 준비의 완료 조건은 다음과 같다.

1. 실제 소비자에 명시적인 게시 전 종료 정책/허용 경계를 적용하고, Stop 재진입·Bash push/PR·직접 게시 경로·Jira MCP를 포함하는 negative 제어가 실제 host에서 거부되는지 확인한다. regex shell hook이나 prompt만으로 모든 전송 경로를 차단했다고 주장하지 않는다. 이 범위의 consumer 변경이 필요하면 새 owner revision/source binding과 검증을 기록한다. 현재는 정책 구현·실제 차단 모두 미확인이다.
2. 새 owned 서버마다 EOF/idle SIGTERM/active SIGTERM을 각각3회 선언하고 raw exit·cleanup을 수집한다. 현재 ChainBench MCP entrypoint의 signal context 부재는 source risk다. 필요 시 실제 해당 서버를 수정하고 재측정한다. wrapper가143을0으로 바꾸거나 강제 kill을 성공으로 바꾸는 처리는 허용하지 않는다.
3. timeout 시 원래 exit/취소/forced cleanup을 구분하고 owned PID·lease·port만 정리한다. source/data를 고정한 ready recipe와 예산이 갖춰지기 전에는 이번 실제 운영 시험을 시작하지 않는다.

## 검토 요청과 다음 단계

이10개 source review rubric과3개 비완전 anchor를 검토용 자료로 제시한다. source 사실과 한계를 검토한 뒤 수정할 Anchor ID와 필요한 중요 항목/범위를 기록한다. 단순 전체 승인으로30개 함수의 독립 edge gold나 새 결함/증상·rule·annotation·seed 입력을 완료 처리하지 않는다.

미지정 역할·LLM provider/model·비용 한도, 실제 결함 티켓2건, 증상/원인 사실군20개, seed/schedule5개, rule별 양성/음성10개씩, annotation12개, 전체 값·제어 edge 정답, 확장 관련 함수 정답20개가 남는다. gold 작성/검토·family 중복·DEV/FINAL·sample 충분성·protocol/recipe 동결 후 실제 실행한다. [전체 작업리스트](../REMAINING-WORK.md)의 나머지 범위는 별도로 유지한다.
