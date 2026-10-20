# 정책·보안 평가 입력 검토 자료

2026-10-15. 단계0 준비 자료이며 승인된 정답·독립 사실군·DEV/FINAL·품질 수용은0이다. [manifest](./operational-preparation/policy-security-drafts/manifest.json)에 고정 소스·원자료·도구·초기 수정 이력을 결속했다. Go 표준 AST를 실행한 source 조사이며 CKG/CKS 탐지기·실제 모델·node 평가를 실행하지 않았다. 원장의 runtime_executed=false는 이 평가 대상의 미실행을 뜻한다.

## 기존 자료와 실제 준비 공백

기존 [graph 정책](../../projects/stablenet/policies/graph.yaml)은42개 정책,146개 대상 참조·123개 distinct target 문자열을 갖는다. 이 파일은 policies overlay이며 이번에 새로 찾은 독립 보안 정답이 아니다. 원본을 수정하지 않고 현재 knowledge-system commit b77af6cd의 YAML bytes를 보존했다. 평가용 Go source commit은 별도로0bf2f4d1bfeb6605006d556957ef8c045d8f8ed8이다. 두 commit이나 SHA를 섞지 않는다.

| 조사 | 결과 | 해석 경계 |
|---|---|---|
| tracked non-test Go 파일 |882개| build tag/플랫폼 선택 없이 구문 조사; build·example·지원 코드 포함 |
| generated 제외 |35개| generated 함수의 존재·부재를 단정하지 않음 |
| 정책 target와 유일한 source 이름 힌트 |140/146개| 실제 graph/canonical/edge 연결 성공이 아님 |
| source 이름 힌트 미연결 |6개| 인터페이스·생성 코드·이름 형태·source drift 등 원인 검토 필요; graph 실패 확정 아님 |
| 공개 source 질의 후보 |12개| 독립성·정답·split 미승인 |
| WI-2 관련 호출 구문 |68개| 실제 탐지 결과·취약점·양성/음성 정답이 아님 |

미연결 참조6개는 다음과 같다. compiler.solcVersion은 두 정책에서 참조하므로 서로 다른 독립 사례2개로 세지 않는다. 후속 [실제 프런트엔드 진단](./POLICY-REFERENCE-DIAGNOSIS.md)에서6개 경고를 재현했고 경고 없는 다른 함수 연결1개도 확인했다. 수정7개 사본은146개 연결·경고0개지만 원본에 미적용·미승인이다. 이 source-only packet의 과거 graph_resolution=not_run은 변경하지 않는다.

| policy | target 문자열 |
|---|---|
| A10.codegen.contract_regen_procedure | compiler.compiledTy.ExportContractCode |
| A10.codegen.contract_regen_procedure | compiler.solcVersion |
| A11.txpool.type_taxonomy_admission | types.ErrInvalidFeePayer |
| A13.sealing.reorg_serialization | legacypool.queueTxEvent |
| A3.validator_set.epoch_transition | engine.Engine.GetValidators |
| A4.system_contracts.build_deploy_pipeline | compiler.solcVersion |

## WI-SP —12개 source 질의 초안

공개 자료는 [public/inputs.json](./operational-preparation/policy-security-drafts/public/inputs.json)과 그 안의 source excerpt12개다. 정답·그룹·기대 annotation·보안 제안은 [private/review-drafts.json](./operational-preparation/policy-security-drafts/private/review-drafts.json)에 둔다. 향후 host에는 public 디렉터리만 복사하고 이 문서·private·원본 packet 전체를 mount/add-dir로 노출하지 않는다. 승인된 overlay를 향후 index에 넣는 작업과 별도 기대 정답을 host에 노출하는 작업을 혼동하지 않는다.

| ID | symbol 이름 힌트 | 검토할 그룹 제안 |
|---|---|---|
| SPC-001 | systemcontracts.GetMaxMinterAllowance | 기존 정책 target |
| SPC-002 | systemcontracts.GetMintProposalAmount | 기존 정책 target |
| SPC-003 | systemcontracts.initializeValidator | 기존 정책 target |
| SPC-004 | systemcontracts.CalculateDynamicSlot | 기존 정책 target |
| SPC-005 | systemcontracts.GetMinterAt | 수동 security metadata 제안 |
| SPC-006 | systemcontracts.GetBLSPublicKey | 수동 security metadata 제안 |
| SPC-007 | systemcontracts.GetRefundableBalance | 수동 security metadata 제안 |
| SPC-008 | systemcontracts.VarLenBytesToMultipleHash | 수동 security metadata 제안 |
| SPC-009 | systemcontracts.GetMinterCount | 선택 overlay에서 직접 참조 없음 |
| SPC-010 | systemcontracts.HashToAddress | 선택 overlay에서 직접 참조 없음 |
| SPC-011 | systemcontracts.IncrementHash | 선택 overlay에서 직접 참조 없음 |
| SPC-012 | systemcontracts.checkSystemContractVersions | 선택 overlay에서 직접 참조 없음 |

보안4건은 운영자가 검토할 수동 메타데이터를 제안한 것이다. 확인된 취약점·독립 위험도 판정이 아니다. severity=low도 전송·보존 검토를 위한 미승인 제안일 뿐이다. 기존 정책과 함께 태그가 있는지, 수정 힌트·설명·severity를 정확히 보존하는지 사람이 채택한 기대값으로 검토한다. 아직 overlay를 프로젝트 설정이나 graph에 적용하지 않았다.

태그 없음은 선택한 overlay 기준이며 안전·정책 준수의 증명이 아니다. 실제 index에서 node/canonical/source pin을 새로 고정해야 한다. 원문 YAML hash/행·reviewer·시간·승인은 현재 API에서 unknown으로 남아야 한다. 외부 검토 자료에 원본 bytes가 있다는 이유로 API의 미보존 provenance를 채우지 않는다.

첫 초안의 splitAndTrim은 과거42질문에 이름이 나와 제외했다. 현재12개는 그 literal name 중복과 기존40함수 pool의 동일 span 중복이 없다. 의미상 family 중복은 미검토다. 모두 같은 systemcontracts 영역이므로12개 독립 사실군이나 대표 표본이라고 주장하지 않는다. DEV/FINAL 배정도 하지 않았다.

## WI-2 — 실제 project source의 구문 목록

[비공개 source census](./operational-preparation/policy-security-drafts/private/rule-source-census.json)는 os/exec.Command20개, strconv.Atoi16개, ParseUint20개, ParseInt12개를 기록한다. import 별칭·인자 구문·enclosing function·대입 좌변·직접 return 이름과 원문 span을 보존했다. import shadow·closure·재정의·호출 target·타입·data flow를 확정하지 않는다.

명령 이름이 함수 인자와 같은 spelling인 힌트2개는 internal/build/util.go의 MustRunCommand와 MustRunCommandWithOutput이다. 둘 다 build 지원 코드이며 같은 wrapper 계열일 수 있다. 실행 node의 취약점이나 독립 결함2개로 세지 않는다. 오류를 버린 변환 결과와 직접 return 이름이 같은 spelling 힌트는0개다. 함수 밖 initializer·생성 코드·미지원 alias 등은 이 census의 경계 밖이며0을 안전·부재 증명으로 해석하지 않는다.

현재 결과로는 지원 규칙2개 각각의 새 양성10·음성10개를 독립 채택할 수 없다. 다음 검토에서는 실제 project 표본의 부족을 inconclusive로 남기고, 새 독립 controlled 표본을 별도 분모로 준비할지 결정해야 한다. 기존 fixture를 다시 사용하거나 두 wrapper를20개 사례로 복제해 기준을 채우지 않는다. WI-SP의 수동 security 제안4개도 자동 rule의 양성4개가 아니다.

## 검증과 재현

[generator](../../scripts/prepare-policy-security-inputs.py)는 Git commit의882개 blob을 batch로 읽으며 현재 checkout 내용에 의존하지 않는다. Go AST helper는 표준 라이브러리만 사용하고 대상 소스의 package/build/test를 실행하지 않는다. YAML 준비 parsing은 PyYAML6.0을 사용했다. frozen JSON/bytes 검사에는 PyYAML이 필요 없다. helper·generator bytes/SHA와 source file/blob/span, public/private artifact hash를 기록했다.

새 source helper 시험8개는 alias·shadow·closure의 미확정성, 변환 tuple, generated 제외, 동명이인과 dirty checkout의 pinned commit 읽기를 확인한다. packet 시험8개는 재해시한 가짜 승인, public 정답 노출, FINAL/채점 조기 배정, source 힌트의 graph/flow/gold 승격과 변조를 거부한다. 초기 필터 전 조사와 생성기의 구문 오류도 보존하며 최종 통과로 세지 않는다.

## 사람이 결정할 부분과 다음 작업

- 위12개 symbol·4/4/4 그룹과 수동 보안 metadata를 채택·수정·기각하고, 독립 family 및 source 범위를 검토한다.
- 미연결6개의 실제 AST 프런트엔드/정책 resolver 재확인은 완료했다. 경고 없는 잘못된 연결1개를 포함한 수정7개를 별도 검토한다. persisted graph·MCP의 재확인과 새 평가 overlay 채택은 남는다.
- WI-2의 실제 project 표본 부족 처리와 별도 controlled 사례 범위를 정한다. 임계치·분모를 결과에 맞춰 낮추지 않는다.
- 정답 작성자·독립 내용 검토자·최종 수용자·실행 담당자를 지정한다. 실제 provider/model·인증 방식·총 비용 상한도 지정해야 한다. 키 값은 문서나 채팅에 적지 않고 지정된 실행 환경에 설정한다.
- 이 검토 뒤에도 새 결함/증상/seed·완전한 값/제어 정답·review/intent 관련성 정답·DEV/FINAL·통계/자원 기준과 단계0 동결이 남는다.

이번 source 준비는 운영·독립 수용0/9를 변경하지 않는다. [전체 작업표](../REMAINING-WORK.md)의 지식 매니저·비교 작업, 선택 후속12개·태그/공개 배포 결정도 남는다. 온톨로지 효과 평가는 새 명세의 필수 OE-01로 이동했고 WI-5는 KM-03/04로 구체화했다.
