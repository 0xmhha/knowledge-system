# 온톨로지 비용 진단·독립 평가 입력 준비 — 2026-10-13

상태: **O04-A/B 완료, C/D·전체O04 미완료**. 기존17/18·온톨로지3/4 완료, 전체 잔여 N16/O04의2개.

## 확인한 비용 경로

기존 DEV의 OP-03(개념 매칭)과 OP-05(no_match)를 같은 binary/held dataset/model/config에서 baseline, relations, concept_text로 비교했다. 승인된 구현 관계0이므로 relations는 projection·원문/인덱스 정합성을 검증하지만 관계 가산0이어야 한다. 모든 모드는 명시 설정으로 같은 raw-query-first를 사용한다. 세 실행 순서를3블록에서 순환했다. 블록별 warmup1·검색1·warm5·cold0, 총126응답/84대조·실패/누락/pending0. 관계 전용과 baseline의 공개 인용/본문은42쌍 모두 같다.

| 문항 | baseline p95(ms) | 관계 전용 p95(ms) | 개념 텍스트 p95(ms) |
|---|---:|---:|---:|
| OP-03 | 385.003 | 465.904 | 561.529 |
| OP-05 | 385.751 | 438.959 | 432.161 |

관계 전용은 추가 embedding을 하지 않는데도 매칭 문항에서 p95비1.2101, no_match에서1.1379였다. 텍스트 검색을 하는 OP-03만 CKV검색/embedding HTTP가 arm별63→84/87→108로 늘었고 tags174→216이다(각arm21응답). 이는 검증 경로와 추가 검색 경로를 구분하는 관측이다. 중첩 span 또는 서로 다른 요청의 분위수 차이를 실제 실행 단계 시간처럼 합산하지 않는다.

## 적용한 작은 비용 보완

graph manifest는8,888,975bytes다. semantic 좌표 검사에서 매 요청마다 해당 JSON 전체를 해석했다. 35457f5c에서 해석 결과만 최대16개 보관하도록 수정했다. 매번 실제 파일을 읽고 전체 바이트SHA256과 목적 타입을 키로 삼는다. 파일 metadata만으로 reuse하지 않는다. 원문/anchor/chunk/프로젝트·dataset·snapshot·commit 검증과 deadline/fallback은 유지한다. 전체 manifest bytes, 검색 결과, embedding vector 또는 검증 합격 결과는 캐시하지 않는다.

동일한 byte만1회해석·반환값 사본, graph/vector타입 분리, 동일크기/mtime 복원 변조, 파일삭제·필드삭제·잘못된JSON, bounded eviction·32동시읽기·기존 Unmarshal merge semantics를 확인했다. 관련4패키지 race247시험249하위시험·실패/skip0, vet·계층경계 pass. 기준binary24d33fc4…aec는 보존하고 별도cks-coordinate-cache 후보로 재측정했다.

## 수정 후 재측정과 한계

같은 사전동결 질문/블록/횟수의126응답/84대조가 통과했다. 수정 전후126응답의 query/coordinates·공개 인용/본문은 모두 같다. 원graph/vector/semantic DB SHA·운영HEAD/current/config도 보존했다. 추가 embedding 호출은 그대로다.

| 문항 | 텍스트 median 전→후(ms) | 텍스트 p95 전→후(ms) | p95비(text/baseline) 전→후 |
|---|---:|---:|---:|
| OP-03 | 510.989→457.532 | 561.529→476.989 | 1.4585→1.0997 |
| OP-05 | 411.394→373.471 | 432.161→728.054 | 1.1203→1.9404 |

OP-05 마지막 블록에서 text728.054ms/relations689.456ms 이상치가 발생했다. 같은 요청에서 HTTP span도 늘었으나 이것만으로 공유부하 또는 수정의 영향이라고 단정할 수 없다. 이 이상치를 제거하거나 앞뒤 표본을 유리하게 합치지 않는다. 문항/arm당 warm15개에서 nearest-rank p95는 최대값이며 변동에 민감하다. 수정이 JSON 재해석을 줄였다는 것은 시험으로 확인했지만 전체지연 예산 통과/성능 수용은 **아니다**. 각실행은 고정 순환 순서의2DEV 질문이며 무작위·격리된머신·cold/독립FINAL 평가가 아니다. 기본off/opt-in 유지다.

## 새 질문·gold 후보와 노출 검사

[기계 검토 자료](../../system/eval/b0-knowledge-system/ontology-o04-independent-candidates-2026-10-13/candidate-book.json)에 제안10문항·11코드span의 source/행SHA와 실제graph canonical/vector chunk 위치를 결합했다. 두 함수를 아우르는IC-10은 각함수별gold로 분리했다. 코드 관계 assertion/시험 기준 승인0이다.

이전 ontology 실제DEV 및 이번 수정 전 비용진단의34캡처파일/882응답을 대조했다. IC-01(Size) gold는54응답, IC-10(서명payload) gold는108응답 본문에 이미 노출돼 격리했다. 나머지8문항은 이 corpus에서 같은파일gold 행 교집합0·질문원문 중복0이다. 수정 후126응답도 이전126과 인용/본문이 같아 새gold 노출이 추가되지 않았다. 하지만 **행 노출 없음은 의미적 독립성을 증명하지 않는다**. 이전B0/FINAL 전체이력과 사람의gold/범위·사실군 검토는 아직없고 작성자는 기존DEV를 본Codex다. 독립 승인 단위0·FINAL가능문항0이다.

| 검토 ID | 질문 | 후보 답 / gold |
|---|---|---|
| IC-02 | validator 집합에 이미 있는 주소를 AddValidator로 추가하면 키와 quorum 입력 수는 어떻게 되는가? | 주소가 같으면 false를 반환하고 append를 하지 않는다. 실제 quorum 재계산 또는 테스트 통과를 주장하지 않는다. validator/default.go:186–196. |
| IC-03 | RemoveValidator에서 대상 주소를 찾지 못하면 무엇을 반환하며 목록을 바꾸는가? | 찾지 못하면 false를 반환하며 삭제를 수행하지 않는다. 찾으면 목록을 재구성하고 true를 반환한다. validator/default.go:198–209. |
| IC-04 | 라운드 체인지 타이머에 양수 MaxRequestTimeoutSeconds가 설정된 경우 timeout을 어떻게 증가시키고 overflow를 처리하는가? | baseTimeout을 시작으로 round만큼2배씩 증가시키며 상한 초과 시 상한으로 제한한다. timeout<baseTimeout인 overflow 의심 경로도 상한으로 설정한다. 상한이0일 때의 다른 경로와 혼동하지 않는다. core/core.go:347–373. |
| IC-05 | 더 높은 여러 round에서 같은 validator가 보낸 ROUND-CHANGE는 higherRoundMessages에서 여러 표로 계산되는가? | 주어진 round보다 높은 round만 대상으로 주소 map을 만들고 len(addresses)를 반환하므로 여러 round에 같은 주소가 있어도1개로 센다. core/roundchange.go:280–293. |
| IC-06 | getMinRoundChange에서 현재보다 큰 round가 없으면 nil을 반환하는가? | 아니다. 큰 round의 키가 없으면 인자로 받은 round를 반환하며, 있으면 오름차순 정렬한 첫 키를 반환한다. core/roundchange.go:308–323. |
| IC-07 | ClearLowerThan은 이전 round만 지우는가, 메시지가 빈 집합도 함께 지우는가? | 빈 집합 또는 기준보다 작은 round를 지우고 roundChanges/highestPreparedRound/highestPreparedBlock/prepareMessages의 네 map에서 해당 키를 함께 삭제한다. core/roundchange.go:326–338. |
| IC-08 | prepared 상태가 없는 ROUND-CHANGE quorum을 셀 때 PreparedRound와 PreparedDigest에는 어떤 조건이 필요한가? | PreparedRound가 nil 또는0이고 PreparedDigest가 zero hash여야 카운트한다. nilCount가 quorumSize와 같아지면 nil을 반환한다. 전부 돌아도 충족하지 못하면 error다. core/justification.go:75–87. |
| IC-09 | checkValidatorSignature는 언제 현재 validator 집합 대신 이전 validator 집합을 사용하는가? | view가 현재보다 작고 상태가 StateAcceptRequest이며 이전 sequence/current.sequence-1 및 PriorRound와 정확히 같은 view이고 PriorValidators가 nil이 아닐 때만 이전 집합을 사용한다. 그 외는 현재 valSet이다. core/core.go:433–443. |

IC-02/03은같은validator-membership 군으로 합산한다. 나머지제안군은round-timeout/higher-round-count/round-selection/round-pruning/nil-prepared-justification/previous-view-signer-selection이다. **8문항/제안7군**이며 모두 승인돼도 독립단위10의 바닥에 최소3군 부족하다. 의미적 상관성 또는 추가이력 노출을 반영하면 부족분은 더 커질 수 있다. 새질문·단순파일diff·반복횟수로독립표본을 부풀리지 않는 auditor12시험을 통과했다.

## 다음 순서와 남은 전체 작업

1. O04-C: 이전B0/FINAL 전체 이력을 포함하도록 노출 대조를 확장하고, 신규 후보의 실제내용·gold·개념범위·사실군/의미적독립성을 검토한다. 부족한새사실·질문을 추가한다. 이번8문항은 검토대기이며 새독립FINAL로 실행하지 않는다.
2. O04-C: 승인·사전동결한 독립입력으로 off/on 품질·지연·실패 분모를 판정한다. 작은군은inconclusive, 개선/비용근거가없으면기본off를유지한다.
3. O04-D: 최종수용/한계·사전조건·원자료·현재/커밋SHA를결합한뒤완료여부를체크한다.
4. N16: 공개키fingerprint/독립신뢰조건은기존사용자결정대로미충족유지한다.

O04의 독립평가요구를 파일럿종료로 대신하려면 별도의 명시적인 사용자 범위변경 결정이 필요하다. 이번 “1번 진행”에서는 그 결정을 추론하지 않았다.
