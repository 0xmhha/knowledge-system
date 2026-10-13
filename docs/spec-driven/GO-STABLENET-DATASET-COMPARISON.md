# go-stablenet 지식 데이터 생성·비교

현재 작업의 목적은 전체 go-stablenet 코드에서 새 지식 데이터를 만들고, 보존한 기존 데이터와 실제 구조·검색·인용·지연을 비교하는 것이다. 기존 데이터의 파일 수·health 성공만으로 품질 개선을 선언하지 않는다.

## 입력과 비교 조건

- 권고 기준: 기존 설정의 chain/go-stablenet, source commit0bf2f4d1. 다른 로컬 dev 저장소는 다른 커밋이므로 사용자 선택 시 별도 source-change 비교로 구분한다.
- 주 기준: 기존 go-stablenet/current를 해결한 고정 버전. refinputs·legacy는 참고 기준이며 입력/스키마 차이를 표시한다.
- 모델: 기존과 동일 BGE-M3 digest790764642607…16bab,1024차원. 모델·바이너리·팩·DB/manifest·current target을 실행 전에 기록한다.
- 새 데이터는 별도 dataset root/version에 만든다. 기존 current/config/DB와 source HEAD를 변경하지 않는다. 보호된 개인 자료는 입력에 포함하지 않는다.
- 전체 코드 범위를 기존과 맞추고 정책·용어·domain·flow와 build/ Go 소스 목록을 명시한다. 완전한 embedding과 좌표/보관 원문 검증이 끝나야 build 완료로 체크한다.

## 작업표와 실행 증거

| ID | 상태 | 확인한 실행 증거 |
|---|---|---|
| DC-01 | 완료 | 소스·기존3개 버전/DB SHA·모델·바이너리·팩·42질문/8fixture 고정 |
| DC-02 | 완료 · gate 경고 기록 | 실제26분16초 build, graph/vector 좌표·strict embeddings·DB hash·canonical 직접 비율 확인 |
| DC-03 | 완료 | strict doctor ready, health28도구·실제 Info 질의 성공; 기존 DB/current/HEAD 보존 |
| DC-04 | 완료 | 기존 주 기준과 구조 비교 전 항목 동일; 참고 입력 차이 분리 |
| DC-05 | 완료 · fixture1개 실패 보존 | 네 graph 모두8개 중7개, recall1.0/precision0.735294/F1 0.847458 |
| DC-06 | 완료 · API 오류 보존 | 1512응답·누락0; 각756회 중360 source_missing; 정상792개 pack 검증,396쌍 비교 |
| DC-07 | 완료 | 결과·대표 오류 경로·한계·다음 작업 및 원자료/SHA를 기록. 커밋 조회 근거는 진행 원장을 따름 |

완료는 데이터 생성과 비교 실행·오류 판정을 기록했다는 뜻이다. 전체 질문의 정상 동작이나 품질 개선을 수용했다는 뜻은 아니다.

## 비교 결과

| 지표 | 기존 주 기준 | 새 데이터 |
|---|---:|---:|
| graph 파일 / 노드 / 관계 | 1323 / 244822 / 1963773 | 동일 |
| vector 청크 / domain / flow | 21304 / 346 / 96 | 동일 |
| symbol canonical | 18691/18837 (99.2249%) | 동일 |
| keyword8개 통과 | 7/8 | 7/8 |
| keyword recall / precision / F1 | 1.0 / 0.735294 / 0.847458 | 동일 |
| 42질문 반복 호출 | 756 | 756 |
| source_missing 오류 | 360 (47.619%) | 360 (47.619%) |
| 정상 v2 pack·원문/좌표/hash 확인 | 396 | 396 |
| source-derived12개 signature hit@10 / RR@10 | 0.5 / 0.388889 | 동일 |
| 성공 응답 warm p95(ms), n220 | 1238.934 | 1191.894 |
| 요청 scope embed / tags 시도 | 3138 / 6276 | 3136 / 6272 |

정상396쌍의 인용 순서·본문 변화0. 구조·정상 검색 결과·오류율에서 개선 차이를 관측하지 못했다. 전체 성공 p95 비율은 약0.9620이지만 질문별 비율 범위는0.7664–1.9370이며 성공한22질문만 지연 집계에 들어간다. 반복10회·warm/shared-host·교대 순서의 기술 관측이다. 성능 향상 수용으로 바꾸지 않는다.

SNK06 mint allowance는 기대 systemcontracts6개를 모두 회수했지만 test.*9개가 추가되어 해당 precision0.4다. fixture는 전역 검색과 제한된 기대 목록을 사용한다. 원래 입력·실패를 그대로 유지하며 새 데이터의 회귀로 단정하지 않는다.

## source_missing 대표 원인과 다음 작업

prepare-handler 질문의 v1 본문에는 `entries/A1.wbft_core.message_handlers.md`가 포함된다. 이 파일은 generated domain corpus에 있으며 retained repo manifest에는 없다. v2 생산자는 보관 원문의 repo 경로로 이 참조를 읽으려다 source_missing을 반환한다. 동일 질문은 domain/flow가 없는 refinputs 데이터의 v2에서 정상11인용을 반환했다. 대표사례의 실제 경로 진단이며20개 실패 질문의 모든 경로를 각각 조사했다는 뜻은 아니다.

우선순위1: generated domain/flow 인용의 origin·캡처·namespace·v2 본문 매핑 계약을 보완하고, 오류가 난42질문을 동일 조건으로 다시 실행한다. 기존 데이터와 원자료를 보존한 상태에서 수정·회귀 기준을 고정한다. 이후 필요하면 SNK06 기대 범위와 실제 도구의 검색 범위를 별도 검토한다.

사용자가 별도로 보류한 출시 사항은 요청 전까지 일반 진행 보고에 포함하지 않는다. 이 작업의 진행/남은 일/다음 단계만 보고한다.

프로토콜과 실행 근거: [evaluation packet](../../system/eval/stablenet-dataset-comparison-2026-10-13/), [protocol](../../system/eval/stablenet-dataset-comparison-2026-10-13/protocol.json), [최종 결과](../../system/eval/stablenet-dataset-comparison-2026-10-13/result.json). 실제 경로와 검증 바이너리는 프로토콜을 따른다.
