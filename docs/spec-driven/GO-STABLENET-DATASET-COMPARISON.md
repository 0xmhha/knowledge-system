# go-stablenet 지식 데이터 생성·비교

현재 상태: 요청한 출처 수정·42질문 재평가·SNK06 검토3/3 완료. 현재 소스와 커밋의1833개 SHA 및 기존 진행 원장 검증이 통과했다. 아래 최초 비교와 수정 후 재평가를 구분해 읽는다.

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

## 최초 비교 결과 · 출처 수정 전

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

## 최초 비교의 source_missing 대표 원인

prepare-handler 질문의 v1 본문에는 `entries/A1.wbft_core.message_handlers.md`가 포함된다. 이 파일은 generated domain corpus에 있으며 retained repo manifest에는 없다. v2 생산자는 보관 원문의 repo 경로로 이 참조를 읽으려다 source_missing을 반환한다. 동일 질문은 domain/flow가 없는 refinputs 데이터의 v2에서 정상11인용을 반환했다. 대표사례의 실제 경로 진단이며20개 실패 질문의 모든 경로를 각각 조사했다는 뜻은 아니다.

최초 비교에서 generated domain/flow 인용의 origin·캡처·namespace·v2 본문 매핑 보완 → 같은42질문 재실행 → SNK06 기대/검색 범위 검토 순서를 도출했다. 세 후속 작업의 완료 근거는 아래 작업표와 재평가 결과를 따른다. 기존 데이터와 최초 원자료는 보존했다.

사용자가 별도로 보류한 출시 사항은 요청 전까지 일반 진행 보고에 포함하지 않는다. 이 작업의 진행/남은 일/다음 단계만 보고한다.

프로토콜과 실행 근거: [evaluation packet](../../system/eval/stablenet-dataset-comparison-2026-10-13/), [protocol](../../system/eval/stablenet-dataset-comparison-2026-10-13/protocol.json), [최종 결과](../../system/eval/stablenet-dataset-comparison-2026-10-13/result.json). 실제 경로와 검증 바이너리는 프로토콜을 따른다.

## 후속 수정과 수용 조건

사용자가 요청한 순서에 따라 아래 세 작업을 수행한다. 앞선 비교 원자료와 기존 current는 보존한다. 새 실행 근거가 확보되기 전에는 완료로 표시하지 않는다.

| 순서 | 작업 | 상태 | 완료에 필요한 근거 |
|---|---|---|---|
| 1 | domain/flow 출처 보관과 v2 매핑 | 완료 | 생성 문서를 identity 고정 전에 캡처; 코드와 문서 경로 충돌 방지; JSONL flow 인용의 실제 행; namespace 검증; 변조·누락·동명 경로 회귀 시험; 별도 실측 build/doctor |
| 2 | 같은 42질문 재평가 | 완료 · 종료 경고 보존 | 원래 requests SHA 유지; 수정 바이너리/새 dataset 좌표 고정; 기존 오류와 비교; 모든 정상 pack의 원문·좌표·hash 검증; 반복 조건·결과·지연 한계 기록 |
| 3 | SNK06 범위·기대 결과 검토 | 완료 | 원 fixture 실패 보존; 추가 test.*의 실제 검색 일치 여부 확인; 전역 검색 계약과 package 기대 범위 불일치 판정; 별도 명시한 검토 fixture 검증 |

설계: pinned build의 외부 corpus는 CKV `docs_roots` 순서와 일치하는 `docs:corpus.<index>` origin으로 보관한다. v1 검색·본문 선택 동안 `@cks/docs/<index>/<path>` 경로를 사용하여 코드와 동일한 파일명을 구분한다. v2는 이를 origin과 원래 상대 경로로 분리하고 기존 보관 원문/hash/예산/소독 검증을 적용한다. flow step은 구현 코드 인용을 유지하며, flow spine/curated invariant는 corpus JSONL의 실제 행을 인용한다. 이전 dataset에 새 원문을 사후 삽입하지 않으며 재색인이 필요하다. 이 출처 구분은 사람의 검토나 정책 승인을 부여하지 않는다.

1번 검증: 실제29분24초 index, 21304청크, namespace 파일45개, JSONL flow spine/invariant34개 실제 행 확인. repo2224/domain44/flow자료27개 원문을 출처별로 보관했다. strict doctor ready·모델/DB/좌표 일치·기존4개 root/current·source HEAD 보존. ANCHOR-01은 domain 문서 원문을 포함한 complete11인용으로 정상 응답했다. 전체 Go118패키지·관련 race8패키지·vet·lint0issues·comparison Python8시험 통과. lint 수정 뒤 setup/build2패키지를 다시 확인했다. canonical18691/18837을 SQL로 직접 확인했고, 비활성 threshold gate를 통과로 바꾸지 않았다. [수정 검증 자료](../../system/eval/stablenet-corpus-repair-2026-10-14/).

2번 재평가: 동일 requests SHA로 교대2블록1512회, 누락0. 기존 source_missing360/756 → 수정0/756, 이전에 실패한20질문의 모든 반복 호출이 정상화됐다. 정상1152개 v2 pack의 원문·좌표·무결성 검증 실패0. 기계적으로 소스에서 고정한12개 signature hit@10은0.5 →1.0, RR@10은0.388889 →0.791667이다. 이전 API 실패가 해소된 검색 진단이며 독립 답변 정확도 판정은 아니다. 정상396쌍 중198쌍의 인용 순서·본문이 바뀌었다. 동일 성공 질문의 warm220쌍 p95는2154.965ms →1918.729ms지만 성능 향상 수용은 하지 않는다. 수정 arm2회에서 close warm session: signal: killed를 보존해 capture harness full pass=false로 기록했다. 요청별 오류율과 캡처 종료 오류를 섞지 않는다. [재평가 검증](../../system/eval/stablenet-corpus-repair-2026-10-14/replay-verification.json).

3번 검토: 전역15개 모두 `name/qualified_name/signature/doc_comment`의 case-insensitive substring AND 계약 및 보관된 선언·SHA·행 범위에 부합했다. 추가9개는 systemcontracts/test 하위의 Go package test 심볼이다. 기존 fixture 원문·실패7/8과precision0.4는 그대로 둔다. 패키지 범위를 명시한 별도 질의 `qualified_name:systemcontracts AND mint* AND allowance*`는 raw FTS AND를 사용하며 mode=or로 자연어 substring 필터를 적용하지 않는다. symbol-only 기본 필터가 CallSite/ReturnStmt를 제외한다. 기존6개 기대 목록을 바꾸지 않고 기존4개·새1개 dataset 모두 precision/recall/F1=1.0으로 확인했다. [별도 scoped fixture](../../projects/stablenet/eval/graph-keyword/scoped-fixtures/SNK06-systemcontracts-mint-allowance.yaml), [출처별 검토](../../system/eval/stablenet-corpus-repair-2026-10-14/SNK06-scope-review.json). 사후 범위 진단이며 새 독립 gold나 모델 품질 승인으로 바꾸지 않는다.

현재 요청한 순차 작업은3/3 실행 완료다. 최종 커밋의 SHA·원문·기존 진행 원장 검증은 PASS이며 기록은 active source binding을 따른다. 남은 요청 작업0개. 별도 운영 유지보수 후보는 캡처 서버 종료 deadline 진단이며, 기존 운영 current/config 전환과 원격 게시를 자동 실행하지 않았다.

현재 소스의 일반 workspace `bin/ckg`, `bin/ckv`, `bin/cks`도 다시 빌드했다. 수정 CKS와 별도 새 데이터의 조합이 필요하다. stdio 설정 경로는 `/Users/kevin/work/github/0xmhha/auto-coding/knowledge-data/go-stablenet-retained-corpus-20261014-cks-stdio.yaml`이며, 실행은 `bin/cks mcp --config /Users/kevin/work/github/0xmhha/auto-coding/knowledge-data/go-stablenet-retained-corpus-20261014-cks-stdio.yaml`다. 원래 운영 설정을 제자리 변경하지 않았다.

최종 material commit `3fa62e15`의 현재·커밋 SHA1833개 검증 및 기존 진행 guard PASS. [커밋 근거](../../system/eval/stablenet-corpus-repair-2026-10-14/committed-binding-proof.json).
