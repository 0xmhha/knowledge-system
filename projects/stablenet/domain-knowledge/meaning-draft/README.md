# Stablenet meaning draft — 검토 자료

2026-10-13 · **전체 proposed, 승인·운영 연결·활성화·품질 판정0**. 기존 verified entry를 새 온톨로지/spec/코드 관계의 승인으로 옮기지 않는다.

이 초안은 기존 팩 중 WBFT 합의의 네 항목을 좁게 옮긴 검토 입력이다. [ontology.yaml](./ontology.yaml), [spec.yaml](./spec.yaml), [필드 출처·관계 후보·한계](./provenance.json)가 한 묶음이다. 기존 glossary는 수정하지 않았다. 이 파일들은 knowledge-system 저장소에 있으므로 현재 go-stablenet retained snapshot의 입력이 아니다. 지금 이 파일 경로를 운영 semantic build에 연결해서는 안 된다. 적용 전 선택한 원문과 함께 새로운 held candidate에 포함하고 source·CKG·CKV·semantic tuple을 새로 검증해야 한다.

## 검토할 내용

| 검토 ID | 개념 / 요구사항 | 제안 범위와 판단할 내용 |
|---|---|---|
| C01 / R01 | wbft-quorum / req-wbft-quorum | ceil(N-F), F=(N-1)/3 및 N=5/6/8의 기대값 4/5/6. 정의·별칭·규칙을 새 버전의 요구로 채택할지 검토. |
| C02 / R02 | wbft-round-change / req-wbft-round-change | prepared block과 ROUND-CHANGE의 정당화. 원문 규칙을 현재 프로토콜 전체 조건과 대조해야 함. |
| C03 / R03 | wbft-message-handlers / req-wbft-message-handlers | 세 handler와 중복 메시지의 멱등성. 심볼 존재·위치 확인은 멱등성 증명이 아님. |
| C04 / R04 | wbft-current-lock / req-wbft-current-lock | currentMutex의 읽기 잠금과 쓰기 잠금 안의 읽기를 구분한 수정 제안. 원자료의 표현 차이를 해소할지 검토. |

정의는 각 entry summary의 첫 문단을 그대로 복사했고, 별칭도 원래 한국어/영어 alias에서 가져왔다. concept ID/kind, scope의 제외 조건, preferred 표시, Given/When 변환은 새로 작성한 제안이므로 검토가 필요하다. 요구 statement/Then은 원래 첫 invariant를 복사하되 **R04만 아래 표현 차이를 반영한 수정 제안**이다. R01의 숫자 예시는 원래 summary에서 criterion으로 변환했으며 테스트를 실행하거나 통과한 것으로 기록하지 않았다.

## R04에서 판단할 차이

기존 summary는 current 상태의 읽기를 currentMutex.RLock **또는 currentMutex.Lock** 아래에서 허용한다. 첫 invariant는 모든 읽기가 RLock 안에 있다고만 적었다. 현재 source의 core.go:162에는 Lock, :166에는 c.current 읽기가 있다. 이 한 곳의 코드 관측을 전체 접근/race 검사로 확대하지 않는다.

수정 제안은 읽기를 RLock 또는 해당 쓰기 Lock 구간 안에서 허용하고, 쓰기는 전체 변경 구간에 Lock을 유지하도록 구분한다. priorState.RWMutex만 잡은 접근은 충족으로 보지 않는다. 이 문장은 기존 승인 문장의 자동 정규화가 아니라 **새 내용 판정이 필요한 proposed 요구사항**이다.

## 앵커·검토 상태

기존 4개 entry는 status verified, verified_by mhha이며 2026-06-07/09의 검토 기록이 있다. pack code baseline은 ec6a4e96b, 이번 관측 대상 source는 0bf2f4d1bfeb6605006d556957ef8c045d8f8ed8이다. 문서와 코드 검토의 날짜·범위를 함께 보존하며, 새 semantic ReviewedBy는 비워 뒀다.

선택한 앵커16개 중 심볼 기반14개는12개 같은 정의 위치·2개 이동이다. Core.currentMutex는108→106, Core.handlePreprepareMsg는115→113으로 이동했다. 위치만 있는2개는 심볼 검사 대상이 아니며 별도 검토가 필요하다. 로컬 소스 저장소에는 baseline ec6a4e96b 객체가 없어 16개 앵커의 이전·현재 파일 바이트 비교는 미확인이다. 현재 파일 SHA만 기록하며 파일 변경/불변을 판정하지 않는다. 위치 유지나 이동도 의미·행동 보존 판정이 아니다.

전체 팩의 읽기 전용 anchors --check 결과는 checked134/clean105/drift26/review2/unresolved1/loc_skipped16이며 exit1은 drift가 있음을 뜻한다. 선택 범위 밖 문제도 남아 있어 전체 팩이 현재 코드에 다시 검증됐다고 표시하지 않는다. 원본47개 입력의 SHA는 그대로다.

관계 후보16개는 출처에서 찾은 파일/심볼·과거 line을 보존한 **비실행 검토 목록**이다. canonical ID/evidence ID는 확정하지 않았고 IMPLEMENTED_BY/TESTED_BY semantic assertion을 만들지 않았다. 요구별 code/test·criterion 연결과 행동 검토가 남았다.

현재 tracked WBFT 테스트 파일 목록과 validator 테스트의 SHA는 provenance에 있다. TestValidatorSet 파일에 QuorumSize 문자열이 직접 나오지 않는다는 관측만 기록했다. 이는 간접 실행/전체 coverage가 없다는 뜻이 아니다. code 테스트 실행0, criterion mapping 승인0이다.

## 검증·다음 작업·전체 잔여

실제 ontology/spec 추출기로 전용 Git fixture에서 YAML/source span·프로젝트 경계를 검증한다. 개념4/요구4/기준5는13개 source span, review queue8개이며 모두 proposed다. trace는 spec_unapproved, 구현/test 경로0으로 남아야 한다. 전용 fixture의 파서 통과는 실제 운영 index 결합이나 내용 승인이 아니다.

다음은 이 subset과 R04 해석의 내용 검토, 요구별 code/test·criterion 관계 후보 검토다. 내용을 채택한 뒤에만 선택 source를 새 held candidate로 캡처해 실제 MCP 상태/조회와 off/on 평가를 진행한다. 운영 연결/활성화는 아직 수행하지 않았다.

전체 잔여3개: N16 서명 키/fingerprint(미충족 유지), O03 실제 의미 자료 검토·대상 결합, O04 실제 도메인 같은 조건 실행·독립 효과 판정. 기존17/18·온톨로지2/4 완료 상태를 유지한다.
