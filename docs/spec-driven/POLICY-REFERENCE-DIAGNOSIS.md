# 정책 참조 연결 진단과 수정안

2026-10-15. 단계0 준비 작업이다. [기존 입력 조사](./POLICY-SECURITY-INPUT-REVIEW.md)의 미연결6개를 실제 CKG Go AST 프런트엔드와 `policy.Resolve`로 재현했다. 추가로 경고 없이 다른 함수에 붙는 참조1개를 확인했다. 수정안7개는 별도 YAML 사본이며 원본 overlay·기존 평가 packet·운영 index에는 적용하지 않았다. 승인된 수정·독립 gold·운영 수용은0이다.

## 범위와 재현 근거

[manifest](./operational-preparation/policy-reference-diagnosis/manifest.json)에 원자료·실행 명령·도구·소스 SHA를 기록했다. Go 원문은 commit0bf2f4d1bfeb6605006d556957ef8c045d8f8ed8의 tracked non-test882개 Git blob이며 이전 packet과 파일 목록·blob·SHA가 같다. generated35개를 명시적으로 제외했다. 플랫폼/build tag 선택·대상 package build·go/packages 로드·대상 코드 실행은 하지 않았다.

프런트엔드는 knowledge-system commit179d1c8d의 Go 코드다. 소유한 임시 module 복사본에 준비 helper만 추가해 빌드하고, 실제 `golang.Parser.ParseFile` 및 `policy.LoadFromFile/Resolve`를 호출했다. module·프런트엔드 관련504개 파일 inventory와 helper bytes를 보존했다. 126,397개 추출 node는 package/file/import/callsite 등도 포함하며 독립 symbol이나 평가 사례 수가 아니다. type-aware canonical identity·buildpipe 후처리·저장 DB·MCP·모델·node 실행은 이 결과의 범위 밖이다.

| 입력 | 정책 node | governed_by edge | 미연결 경고 | 검토 경계 |
|---|---:|---:|---:|---|
| 기존 YAML 사본 |42|140|6| 경고 없이 잘못 연결되는 참조1개도 포함 |
| 수정 YAML 사본 |42|146|0| 수정7개가 의도한 파일·행에 유일하게 연결됨; 전체 정책 의미의 독립 검토는 아님 |

146개 참조는 같은 대상을 여러 정책이 참조하는 경우를 포함한다. `solcVersion`2개는 같은 선언이다. 수정 후7개 참조를7개 독립 결함/사실군으로 세지 않는다. 나머지139개 참조도 이름 연결 성공만 관측했으며 정책 설명의 의미까지 승인한 것은 아니다.

## 검토할 수정7개

[수정 제안 JSON](./operational-preparation/policy-reference-diagnosis/correction-proposals.json)은 원래·수정된 node ID, 종류, 파일, line/byte 위치와 applied=false·approved=false를 기록한다. [제안 YAML](./operational-preparation/policy-reference-diagnosis/proposed-policy.yaml.txt)은 아래 target7행만 변경한다. [원본 사본](./operational-preparation/policy-reference-diagnosis/original-policy.yaml.txt)을 함께 보존했다.

| 정책 | 기존 target → 제안 target | 실제 선언과 원인 |
|---|---|---|
| A10.codegen.contract_regen_procedure | `compiler.Compile` → `compile.Compile` | systemcontracts/compile/compiler/compiler.go:40. 디렉터리는 compiler이나 package 선언은 compile이다. 기존 target는 cmd/evm/internal/compiler/compiler.go:26의 EVM assembler 함수에 붙었다. |
| A10.codegen.contract_regen_procedure | `compiler.compiledTy.ExportContractCode` → `compile.compiledTy.ExportContractCode` | 같은 파일:123. package 이름 불일치다. |
| A10.codegen.contract_regen_procedure | `compiler.solcVersion` → `compile.solcVersion` | 같은 파일:37. package 이름 불일치다. |
| A4.system_contracts.build_deploy_pipeline | `compiler.solcVersion` → `compile.solcVersion` | 위와 같은 선언이다. |
| A11.txpool.type_taxonomy_admission | `types.ErrInvalidFeePayer` → `txpool.ErrInvalidFeePayer` | core/txpool/errors.go:69. 이 source pin에서는 txpool package에 선언돼 있다. 이동 시점이나 과거 원인을 추정하지 않는다. |
| A13.sealing.reorg_serialization | `legacypool.queueTxEvent` → `legacypool.LegacyPool.queueTxEvent` | core/txpool/legacypool/legacypool.go:1284. 메서드 receiver가 빠졌다. |
| A3.validator_set.epoch_transition | `engine.Engine.GetValidators` → `wbftengine.Engine.GetValidators` | consensus/wbft/engine/engine.go:1097. package 선언은 wbftengine이다. backend API 등의 동명 메서드로 대체하지 않는다. |

각 대상의 전체 source 파일은 packet의 `sources/`에 고정했다. 정책 A10의 설명은 systemcontracts/compile과 solc0.8.14를 가리키므로 EVM assembler 연결은 설명과 맞지 않는다. 이는 준비 담당자의 원문 대조 판단이며 독립 수용자의 정책 의미 승인은 남는다.

## 사용자에게 미치는 영향과 검증 기준

현재 resolver는 `qualified_name` 문자열을 정확히 맞추고 없는 대상은 경고로 남긴다. 따라서 동일 조건으로 원본 overlay를 로드하면 위6개 정책 연결은 빠지고, `compiler.Compile`은 다른 함수에 정책을 붙일 수 있다. 정책 질의를 사용하는 검색/컨텍스트에도 잘못된 연결이 전달될 가능성이 있다. 실제 운영 DB/MCP에서 이 현상이 발생했는지는 이번에 측정하지 않았다.

경고0개만으로 정책 연결의 정확성을 판정하지 않는다. 후속 실행에서는 source pin, node 종류·file/span, canonical identity, 기대 정책 방향과 설명의 의미를 함께 확인한다. probe는 모든146개 참조의 매치와 edge를 보존하며 이 pin에서 해당 target들의 다중 매치는0이다. resolver 자체는 중복 qname을 거부하지 않고 마지막 node를 선택하므로 다른 corpus에 이 유일성을 일반화하지 않는다.

개발 시험은 실제 프런트엔드로 package/디렉터리 차이, pointer receiver, 경고 없는 잘못된 연결, 누락 receiver, 동명 node, generated 제외, parse 실패 경계를 확인한다. packet 검사는 재해시한 승인·실행 승격·잘못된 source·edge 변조와 경로 이탈을 거부한다. 원장 guard는 이 진단을 단계0 준비로만 연결한다. 시험 수·최종 결과는 packet의 검증 기록을 따른다.

최종 검증은 진단 시험13개·원장 회귀33개·문서 계약20개 통과, 문서119개 확인, 기존 source/input 결속2069개 통과다. 원래 두 구현 원장과 project overlay는 byte 단위로 그대로다. 동결 필수 검사는 예상대로 exit1이며 미충족 구조 요건101개를 유지한다. [검증 기록](./operational-preparation/policy-reference-diagnosis/verification.json)은 준비 결과이며 운영·품질·사람 수용으로 집계하지 않는다.

초기 generator는 probe 성공 뒤 결과 요약에서 null 경고 목록을 세다가 실패했다. 그 실행을 성공으로 세지 않고 실패 기록을 보존했다. null 처리 수정 뒤 generator 전체를 재실행한 결과만 최종 packet으로 채택했다. 초기 수동 build의 상대 경로 오류와 packet 생성 전 검사도 개발 실패/미완성 확인 기록으로 남기며 최종 통과 수에 포함하지 않는다.

## 다음 순서와 남은 작업

1. 위7개 이름 수정의 정책 의미를 검토하고 새 평가용 overlay로 채택할지 결정한다. 기존 source-bound YAML/packet을 덮어써 과거 증거를 바꾸지 않는다. 아직 approved cases·DEV/FINAL·금액/시간 한도·동결은 없다.
2. 정책/security query12개·4/4/4 그룹·독립 family를 검토한다. WI-2 project 표본 부족은 inconclusive로 유지하고 별도 controlled 분모의 채택 여부를 정한다. 이번 연결 진단은 취약점 탐지나 자동 rule 양성 검증이 아니다.
3. 정답 작성자·독립 내용 검토자·최종 수용자·실행 담당자, 실제 모델·인증 방식·비용 한도를 지정한다. 새 결함/증상/seed·완전한 의존 정답·review/intent 정답과 DEV/FINAL·기준·실행 recipe를 확정해 단계0을 동결한다.
4. 이후 단계1–4의 실제 runtime·품질·독립 수용을 수행한다. 현재9개 모두 not_run/not_recorded다. WI-5는 새 지식 매니저 KM-03/04로 구체화했고 온톨로지 효과 평가는 필수 OE-01로 이동했다. 새 기능·평가7개와 선택 후속12개·태그/공개 배포 결정은 [전체 작업표](../REMAINING-WORK.md)에 남아 있다.
