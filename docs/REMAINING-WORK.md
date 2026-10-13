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
| 상위 WI10개 | 주 담당은 coding-agent/ChainBench/학습 계층; 이번 저장소에서 외부 최신 구현은 미감사 | [목적과 책임표](./spec-driven/PURPOSE-AND-SCOPE.md)의 WI별 담당 코드·실행·수용 재감사 |
| 선택적 viewer 상호작용 | 통합 viewer/Atlas 있음. BFS-ripple은 과거 UI 제안 | 채택 시 redraw·성능·접근성 완료 기준과 시험 범위 결정 |
| 배포 머신·다운스트림 동기화 | 로컬 코드/CI만으로 원격 서비스나 다른 저장소의 상태를 알 수 없음 | 필요 시 대상·권한을 정하고 실제 binary/dataset/model/agent/network/power 상태와 소비자 결과 확인 |

과거 “worksheet 내 프로젝트 catalog”, “v2 유지보수 제자리 쓰기”, “온톨로지 MCP 미노출”은 현재 구현과 다르므로 신규 미완료 목록에 남기지 않는다. Catalog는 명시 선택 팩으로 분리됐고 v2 pinned 쓰기는 거부/후보 빌드 경로이며 읽기 MCP 네 도구와 bounded explanation은 구현됐다. legacy 직접 갱신은 지원되는 별도 경계다.

이 저장소의 문서 정리로 외부 상위 목표나 운영 호스트 검증까지 완료 처리하지 않는다. 다음 작업을 채택할 때 목적·담당·입력·설계·완료 기준·실행 증거를 먼저 고정한다.
