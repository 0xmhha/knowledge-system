현재 진행: [상위 WI 구현 명세](./spec-driven/UPPER-WORK-ITEMS-IMPLEMENTATION.md)의 요청 범위에서 구현·명시 범위 검증 **8/9** 완료이며 WI-SP를 진행한다. 남은 전체 구현 순서는 **WI-SP**다. production/독립 전체 수용은 별도 미판정이며 WI-5는 이번 순서에 포함하지 않는다.

WI-6 bounded 함수 문맥 확장의 [최종 packet](../system/eval/upper-work-items-implementation-2026-10-14/wi-6/README.md)은12개 actual 공개 검사/전체 회귀·독립 분모4의 회수율1/4→4/4+noise1과 calls/curated/value의 구분·한계를 보존한다.

WI-C 부분 심층 리뷰·보고서 전달 게이트의 [최종 packet](../system/eval/upper-work-items-implementation-2026-10-14/wi-c/README.md)은16개 실제 공개 경로 검사/142개 plugin 테스트·전체 primary 회귀와 출처/추론/unknown 한계를 보존한다.

WI-2 초기 Go pack·CKG/CKS 공개 후보 분석은 [최종 packet](../system/eval/upper-work-items-implementation-2026-10-14/wi-2/README.md)으로 고정했다.23개 실제 사례·39개 공개 검증·기존15개 회귀와 전체 build/test/fmt/lint PASS, 오탐2/미탐4와 한계·독립 수용 미판정은 유지한다.

WI-4 원래 모드9 완료 근거는 [관찰 packet](../system/eval/upper-work-items-implementation-2026-10-14/wi-4-mode9/README.md), WI-1 Go 값/제어 의존과 공개 질의는 [최종 packet](../system/eval/upper-work-items-implementation-2026-10-14/wi-1/README.md)에 보존했다. 소스337300b0 clean binary로15개 실제 compile/race→index→MCP 검증과 source/hash/unknown/bound/refusal/EOF 검증 PASS, 증거6332da9d·현재1617개 source/dependency SHA 확인. 다음은 opt-in 결정론 Go 규칙 후보와 수동 annotation 공존이다.

추가 요청 완료: [서버 종료 보완 SD-01–04](./spec-driven/GO-STABLENET-DATASET-COMPARISON.md)4/4 완료. 원인 재현·구현·실측2세션·EOF/SIGTERM·커밋 SHA/진행 원장 검증 PASS. 이전 두 종료 오류는 보존했다. 이 요청의 남은 작업0개·다음 작업없음.

# 남은 전체 작업

2026-10-13 · 코드 기준 main9a397188. 기존 리팩토링 **17/18**, 온톨로지 **4/4** 완료. 기존 필수 잔여는 **N16 한 개**이며 미충족·보류다. PR #99 스쿼시 머지·main 동기화와 main CI5/5 확인은 완료했다. 문서 정리는 별도 유지보수이며 제품의 품질·수용 범위를 변경하지 않는다.

## 승인된 필수 잔여와 보류 결정

| 우선순위 | 작업 | 상태·착수 조건 | 완료에 필요한 증거 |
|---|---|---|---|
| 1 | N16 공개키·신뢰·권한 | 사용자 결정으로 보류. 재개 지시 전 키 생성이나 완료 체크를 하지 않음 | 실제 공개키 fingerprint·권한자·신뢰 경로·서명 대상·보관/교체/폐기·고지 적합성 판정·회귀 근거 |
| 2 | 태그·공개 배포 | 보류. 별도 범위·사용자 승인 필요 | 지원 대상, 신뢰·복구·내용/품질/비용 한계, 배포 대상과 권한을 구체적으로 판정 |

다음 필수 작업은 **N16 재개 여부 결정**이다. 보류 유지 중에는 승인 없이 구현을 이어갈 필수 항목이 없다. 완료한17개/4개 항목의 수용 조건과 증거는 [실행 명세](./spec-driven/REFACTORING-EXECUTION-SPEC.md), [이력 원장](./spec-driven/REFACTORING-PROGRESS.json), [범위별 작업표](./spec-driven/REFACTORING-REMAINING-WORKLIST.md)로 검증한다.

## 별도 범위가 필요한 후속 작업

이 목록은 기존 완료 수에 더하지 않는다. 소표본으로 종료한 기능을 품질 성공으로 바꾸거나 새 기능 구현을 승인한 목록도 아니다.

| 후속 범위 | 현재 코드·증거의 경계 | 다음에 필요한 일 |
|---|---|---|
| 조직 인증·권한 어댑터 | 로컬 reviewer ID 기록·감사는 외부 신원 인증과 다름 | 조직 사용 범위 채택 시 인증·권한·서명·trust·거부/감사 조건과 실제 검증 |
| 공개 v2 CLI/HTTP·독립 graph export | 통합 MCP v2 생산자는 구현됨. 일반 --format=v2·독립 export 확장은 설계 범위 | 표면별 지원/거부·소비자 호환·원문/무결성·실제 실행 계약을 새 범위로 고정 |
| 독립 온톨로지 효과·지연 | 구현·실측 기록 완료, 독립승인0·품질inconclusive·성능수용false | 새 독립 질문/사실군·원문·reviewer·사전 임계치/분모·새 동결·실측 |
| graph expansion 점수·관계 튜닝 | intent별 가중치·max 집계·hop 제한 구현 있음, 독립 최적성 미판정 | 새 per-intent 입력·분모·recall/noise/cost 기준을 동결하여 decay/count 보너스/관계 변화 비교 |
| domain flow 정답 평가 | flow 도구와 curated corpus 경로 있음 | 실제 정본 corpus·get_flow/find_branches 정답과 precision/recall |
| 대형 corpus throughput·장함수 sliding split | 코드·작은 fixture 시험 있음 | 실제 대형/긴 함수 입력·모델/하드웨어 pin·비용/회수율 측정 |
| PR 시점의 문서/flow 지식 | v2 원문 snapshot은 고정됨. 과거 PR에 어떤 도메인 시점 입력을 쓸지 별도 문제 | as-of 정책·과거 원천·leakage 판정과 평가 입력 동결 |
| legacy 본문 출처 보강 | flat 설정은 FilesystemFetcher, checkout drift 경고. v2 retained-source 읽기와 구분 | 유지 범위·코드/DocsRoots byte 검증·citation-only 실패 정책·health 출처 설계와 회귀; [현재 경계](../system/docs/design-pack-body-provenance.md) |
| native CoreML embedder | native 구현 있음, 일반 CLI의 embedder dispatch에는 연결되지 않음 | 지원/실험 범위 결정·명시 연결·native 검증 |
| embedding LRU cache | wrapper 구현 있으나 기본 생성 경로에 연결되지 않음 | 메모리/invalidations·효과 기준·명시 옵션 검토 |
| append-only audit integration | auditlog/observe 구현과 audit_dir 설정은 있음. 운영 composer에 연결 완료 근거 없음 | 감사 대상·보관/개인정보·권한·failure 정책과 실제 wiring |
| 상위 WI10개 | [재감사](./spec-driven/UPPER-WORK-ITEMS-AUDIT.md) 기록10/10. 기능 전체 완료0/10; WI-5 담당 미확보 | WI-V1 우선. WI-3/SP 소비자 입력 정합은 연결 실행의 선행 조건. 아래 WI 잔여표 참조 |
| 선택적 viewer 상호작용 | 통합 viewer/Atlas 있음. BFS-ripple은 과거 UI 제안 | 채택 시 redraw·성능·접근성 완료 기준과 시험 범위 결정 |
| 배포 머신·다운스트림 동기화 | 로컬 코드/CI만으로 원격 서비스나 다른 저장소의 상태를 알 수 없음 | 필요 시 대상·권한을 정하고 실제 binary/dataset/model/agent/network/power 상태와 소비자 결과 확인 |

## 상위 WI 재감사 후 잔여 작업

2026-10-14 기준 감사 기록10/10 완료, 기능 전체의 완료·독립 수용0/10. 구현 없음으로 단정할 수 없는 항목과 부분 구현을 구분했다. 상세 설계 부족분·시험·source revision은 [재감사 명세](./spec-driven/UPPER-WORK-ITEMS-AUDIT.md)에 있다. 이 표는 기존 후속 범위의 상위 WI 행을 세분화하며 기존 승인 기능 완료 수에 더하지 않는다.

| 우선순위 | ID | 남은 작업·담당 |
|---|---|---|
| 1 | WI-V1 | 독립 오라클·무효/과적합 반례·차단/경고 정책·실제 에이전트 판정; coding-agent |
| 2 | WI-V2 | 동일 증상의 복수 수정안·평가축·패널 선택/기각; coding-agent |
| 3 | WI-3 | 명시 범위 구현 검증 완료. production text seed·설치된 전체 bugfix role·독립 수용 남음; coding-agent+CKS |
| 4 | WI-4 | 반복 분모·증상 실패율·타이밍 주입·재현 불가 관찰의 실제 연결 실행; coding-agent+ChainBench |
| 5 | WI-1 | 언어/비용/정확도 계약·CFG/def-use/control/data dependency; CKG |
| 6 | WI-2 | 수동/자동 구분·결정론 타입/규칙·source→sink 탐지·양성/음성 평가; CKG/CKS |
| 7 | WI-C | sequence/module/data/state/guard/cost/priority의 구조화 리뷰·소비자 연결·독립 수용; coding-agent |
| 8 | WI-6 | WI-1 이후 값 의존 확장·recall/noise/cost·상한/중단 검증; CKS |
| 9 | WI-SP | CKG 간접 노출을 CKS에 연결·typed 태그/출처/권한/없는 태그·소비자 질의; CKS+CKG |
| 별도 | WI-5 | 담당 저장소 지정 후 수집→파일 ingest→관리자 검토→승격/회수→색인 검증 |

다음 권고는 WI-V1이다. 소비자 입력 schema 정합(WI-3/SP)을 연결 실행 전에 확인한다. 학습 저장소 확보는 별도 의존성이며 다른9개를 중단시키지 않는다. 외부 저장소 변경·실제 체인·LLM 실행·새 수용 기준은 각 기능의 구체 범위를 고정해 진행한다.

과거 “worksheet 내 프로젝트 catalog”, “v2 유지보수 제자리 쓰기”, “온톨로지 MCP 미노출”은 현재 구현과 다르므로 신규 미완료 목록에 남기지 않는다. Catalog는 명시 선택 팩으로 분리됐고 v2 pinned 쓰기는 거부/후보 빌드 경로이며 읽기 MCP 네 도구와 bounded explanation은 구현됐다. legacy 직접 갱신은 지원되는 별도 경계다.

이 저장소의 문서 정리로 외부 상위 목표나 운영 호스트 검증까지 완료 처리하지 않는다. 다음 작업을 채택할 때 목적·담당·입력·설계·완료 기준·실행 증거를 먼저 고정한다.
