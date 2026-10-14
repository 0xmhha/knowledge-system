# 개발 목적과 현재 범위

코드 구조·변경 이력·검토된 문서와 정책을 같은 프로젝트·소스 스냅샷에 연결하여, 외부 코딩 에이전트가 변경 이유와 필요한 검증을 근거로 판단하도록 한다. 검색 도구나 연결 그래프가 사람의 판단을 자동 대체한다는 목표는 아니다. 품질 개선·총비용 감소는 충분한 독립 평가로 확인해야 한다.

현재 제품은 하나의 Go 모듈에 CKG(구조·키워드), CKV(의미 회수), CKS(근거 합성·MCP)를 둔다. 프로젝트 고유 지식은 팩으로 선택한다. 모델·원문·소스·팩·스냅샷 신원을 검증하고, 입력과 실행의 불일치를 숨기지 않으며, 테스트 성공과 기준별 사람 수용을 구분한다.

## 원래 요구사항의 유지

초기 요구사항은 INV7/FR10/NFR5/반례9다. 원문과 v2 대응은 원장의 N-01 복구 증거에 보관되어 있다. 아래 항목의 목적은 유지되며, 오래된 단계별 미완료 서술은 현재 작업리스트로 대체한다. 이 목록 전체의 production 품질이나 외부 조직 수용을 완료했다고 선언하지 않는다.

| 원 ID | 유지하는 목적 |
|---|---|
| INV-01 | 프로젝트/소스 버전 격리 |
| INV-02 | 공통manifest·부분build 비활성 |
| INV-03 | 원문·버전·줄로 인용 재현 |
| INV-04 | verified의 타입·출처·검토자 |
| INV-05 | 의미 확장으로 원 검색 차단 금지 |
| INV-06 | 공개 계약/구DB 호환 |
| INV-07 | 모델/차원/전처리 동일성 |
| FR-01 | 현재File만 감사 |
| FR-02 | 필터 내 K수와 회수 품질 |
| FR-03 | 장문child 검색·parent 문맥 |
| FR-04 | 실패/근거 부족/무답/충돌 구분 |
| FR-05 | 문서/claim/span·코드 연결 |
| FR-06 | 작은core/프로젝트 어휘 버전 관리 |
| FR-07 | 원 질문·복수 후보·soft 신호 |
| FR-08 | 요구→개념→코드→test·충돌 보고 |
| FR-09 | 임의project 설치/진단/복구 |
| FR-10 | committed/working-tree 분리 |
| NFR-01 | 품질 비열화·비용/지연/규모 |
| NFR-02 | 비밀/제외/링크/프로젝트 격리 |
| NFR-03 | 의미 버전/이전/출처 |
| NFR-04 | 파서 실패/미지원 능력 공개 |
| NFR-05 | 문서·CLI/MCP 계약 일치 |
| S-01 | 선택도 높은 필터의 exact oracle |
| S-02 | 긴 문서 끝·정확 줄 |
| S-03 | 과거 Git파일 Hunk |
| S-04 | 없는API/정책의 무관 인용·단정 |
| S-05 | 스펙 문장과 코드 조건의 차이 |
| S-06 | 다의어/한영별칭/미등록어 |
| S-07 | 동명symbol·다른project |
| S-08 | 같은HEAD의 수정/신규 파일 |
| S-09 | 한engine 실패에서 승격 금지 |

현재 수용된 구현·시험 범위는 [실행 명세](./REFACTORING-EXECUTION-SPEC.md), 측정과 부족분은 [평가 상태](./EVALUATION-STATUS.md), 실제 남은 일은 [남은 작업](../REMAINING-WORK.md)을 따른다. 기존18개는17개 완료와 N16 보류이며, 온톨로지4개는4개 완료다. 이 완료 수를 다음 상위 목표의 완료 수로 사용하지 않는다.

## 외부 상위 목표의 책임

현재 구현 진행은 [상위 WI 구현 명세](./UPPER-WORK-ITEMS-IMPLEMENTATION.md)를 따른다. 아래 표는 재감사의 기준 상태이며 구현 이후의 검증·잔여는 구현 원장에서 갱신한다.

6월 상위 계획의 WI10개는 지식 제품의 9월 리팩토링18개와 범위가 다르다. 2026-10-14에 담당 소스와 안전한 시험을 [재감사](./UPPER-WORK-ITEMS-AUDIT.md)했다. 감사 결과 기록은10/10, 원래 기능 전체의 완료·독립 수용 판정은0/10이다. WI-5의 담당 저장소는 미확보다. 코드 도구·절차 문서·단위 시험만으로 외부 전체 워크플로의 완료를 추정하지 않는다.

| ID | 목표·담당 | 감사 후 현재 경계·잔여 |
|---|---|---|
| WI-V1 | 오라클 타당성; coding-agent | RED/GREEN·수정 타당성 분리와 sibling 검사 있음. 일부 무효 오라클·과적합은 WARN 통과; 독립 오라클·실제 false-done 반례 미수용 |
| WI-V2 | 수정 후보 생성·judge-panel 선택; coding-agent | 경쟁 원인 probe는 있음. 같은 증상의 복수 수정안 비교·패널 선택 계약·실행 미확보 |
| WI-3 | 증상→코드 후보·객관 신호; coding-agent+CKS | 개별 탐색·거리/score/impact·원인 라벨 있음. 입력 schema 차이·통합 후보 신호/provenance/null·반복 확장 계약 남음 |
| WI-4 | 비결정/타이밍 재현·방어·관찰; coding-agent+ChainBench | 장애 주입·bounded monitor·simulation/e2e 절차와 모의 시험 있음. 반복 분모·실패율·실제 연결 실행 미수용 |
| WI-1 | dataflow/CFG/PDG; CKG | AST·호출·field 관계 있음. def-use·control/data dependency 레이어 근거 미확보 |
| WI-2 | 결정론 규칙 태깅/취약점; CKG/CKS | 수동 Policy/SecurityPattern resolve는 있음. source→sink 자동 탐지·타입/룰 팩 효과 미확보 |
| WI-C | 깊은 구조화 리뷰; coding-agent | light findings 보고서·CKS 함수 근거 있음. sequence/module/data/state/guard/cost 산출물·소비자 연결·독립 수용 남음 |
| WI-6 | 다단·dataflow-shaped 확장; CKS | bounded 호출 hop·curated flow 있음. 자동 값 의존 확장은 WI-1 의존, 독립 recall/noise/cost 미수용 |
| WI-SP | security-pattern/policy MCP; CKS+CKG | CKG 일반 조회의 간접 노출 실측됨. CKS annotation 관계 변환·전용 schema/출처/권한·소비자 연결 남음 |
| WI-5 | 암묵지 수집·관리자 학습; 별도 ingest MCP/플러그인 | 담당 미확보로 구현·실행 판정 불가. 개정된 명령 수집→파일 ingest→관리자 store/승격/회수 루프 필요 |

감사한 revision·원문·시험과 반례는 재감사 문서의 증거 패킷에 고정했다. 기능 구현은 구체 명세와 담당 저장소별 실행·수용을 확인한 후 완료 처리한다. 이번 감사에서 외부 코드·운영 호스트·학습 서비스는 변경하지 않았다.
