# 읽기 전용 다중 프로젝트 비교 MCP

2026-10-16 기준 KM-06의4/4를 구현·검증했다. 당시 KM-06 checkpoint는24/28·6/7이었고 이후 OE-01의 제한 연구까지 완료해 현재28/28·7/7이다. 독립 품질 수용은 별도다. [전체 시험표](./KNOWLEDGE-ROADMAP-TEST-MATRIX.md), [명세](./KNOWLEDGE-MANAGER-AND-COMPARISON-SPEC.md), [원장](./KNOWLEDGE-ROADMAP-PROGRESS.json)을 따른다. 실제 원문/MCP 시험과 독립 품질 수용을 구분한다.

## 사용 목적과 공개 도구

공통 개념과 비교 축을 선택해 프로젝트별 코드·문서·설정/정책·시험의 근거를 모은다. 프로젝트 신원·구현·정책은 각각 유지한다. LLM/사용자는 반환된 원문·인용·매핑 이유·조건·누락을 검토해 최종 해석을 내린다. 코드 열람만으로 TPS·보안성·우월성이나 실제 네트워크 동작을 판정하지 않는다.

실행은 `cks mcp comparison --profile /absolute/profile.json`이다. 이 독립 stdio 서버는 cks.comparison.describe와 cks.comparison.extract2도구만 등록한다. 두 도구 모두 readOnly=true, destructive=false, idempotent=true, openWorld=false다. 기본 composer·maintenance·쓰기·current 전환·job 접수를 등록하지 않는다. manager plugin0.6.0의 reviewed18/updates6도구에 이 두 도구가 자동 proxy된 것으로 세지 않는다.

- describe: 고정 profile/catalog pin·등록 scope·공통 개념/별칭/축·retained-source capability·최대 예산을 반환한다. state=configured, source_readiness=not_checked이므로 설정 읽기를 원문 조회 준비 완료로 표현하지 않는다. 로컬 설정/DB 경로를 노출하지 않는다.
- extract: 명시적인 참여 scope·개념/축에 따라 프로젝트별 source 행렬과 구조화 bundle을 반환한다. 구현 대상이 필요하면 기존 collection 구현 모드를 사용하며, 비교 요청은 patch target을 받지 않는다.

## 설정과 고정 신원

ComparisonProfileDocument schema1은 name, catalog_path, scheme_root, 선택적 bindings_path/reviews_path, sanitize_rules_path, scopes를 갖는1MiB 로컬 JSON이다. 최대128개 scope를 등록한다. loader는 절대·정규화 경로를 no-follow로 읽고 symlink 경로를 거부한다. profile/catalog·선택 연결/검토·정제 파일의 실제 byte hash와 scheme tree digest를 합쳐 profile digest를 고정한다. 파일 변경은 실행 중 반영하지 않으며 재시작 후 새 pin을 사용한다. active binding은 등록된 전체 좌표/조건 scope 안에 있어야 한다.

scope는 전체 V2Coordinates + industry + network/configuration/component_role/observed_at이다. scope ID는 이 tuple의 SHA-256이며 project name·로컬 binding ID·URI를 전역 신원으로 사용하지 않는다. 동등한 timezone 표현과 industry 대소문자/공백만 정규화한다. 프로젝트·버전·network/config/역할을 임의로 합치지 않는다. 같은 프로젝트의 다른 조건도 명시적으로 비교할 수 있다. catalog 밖 좌표나 정규화 후 중복 scope를 거부한다.

catalog digest는 기존 canonical collection 계약을 재사용한다. 요청 pin이 다르면 stale_configuration으로 거부한다. 서버는 다른 current·바뀐 설정·다른 프로젝트로 암묵 재선택하지 않는다. 호출자가 반환값을 수정해도 서버의 소유 상태는 바뀌지 않는다. 등록/설정 계약과 실제 데이터셋 읽기의 성공 상태를 구분한다.

## 요청 계약

| 필드 | 계약 |
|---|---|
| schema_version, mode | 1, comparison만 허용 |
| profile_digest, catalog_digest, collection_id | 선택 snapshot과 정확히 일치 |
| participants | collection 소속의 명시 scope ID2–16개; unknown/중복 거부 |
| concept | 선택 pack ID/version/digest/concept ID 전체 tuple |
| facets | 해당 개념에 정의된1–8개 축; unknown/중복 거부 |
| question | 선택적4KiB 이하 의도 텍스트; 실행 지시나 자동 사실 해석으로 사용하지 않음 |
| as_of | 명시 RFC3339; 서버 시각 이후 미래나 잘못된 시각 거부 |
| budget | source body byte1–262144, citation 객체1–128, timeout1–30000ms |
| include_proposed | 기본 false; 명시 true에서만 경고/근거가 있는 미검토 후보 포함 |

기본 예산 생성기는 위 최대값을 반환한다. 요청은 양수 상한을 명시하고0·초과·형식 오류를 거부한다. 엄격 JSON·lower_snake_case 필드를 사용한다. 요청으로 설정/경로·backend·patch target을 바꾸지 못한다. 자연어 일부 문자열에서 프로젝트나 개념을 추측하지 않으며 describe의 개념/별칭/축에서 명시 key를 선택한다. 원문과 question은 데이터로 취급한다.

## 행렬·검토·누락·예산

각 participant/facet 셀은 공통 정의, 로컬 구현/정책의 source 종류·본문·인용, 매핑 이유와 검토 상태, 조건, unknown/누락을 분리한다. exact/similar도 직접 매핑의 범위이며 entity identity·자동 추이·행동 동일성의 증명이 아니다. test source는 테스트 실행 결과가 아니다. 매핑 검토는 정책 내용 승인이나 검토자 신원 인증이 아니다. [공통 개념/연결](./INDUSTRY-CONCEPT-COMPARISON.md)의 상태/검토 계약을 따른다.

기본은 현재 recorded review가 있는 연결이며 proposed는 명시 후보 모드에서만 제공한다. 같은 scope의 상충은 conflict, 다른 프로젝트의 정책 차이는 각각의 근거로 유지한다. not_collected/unknown은 기능 부재가 아니고 unsupported는 reader capability의 한계다. not_applicable에는 해당 축의 명시적인 문서/설정/정책 출처가 필요하다.

한 버전이 없거나 손상되면 해당 프로젝트를 unavailable로 남기고 다른 프로젝트의 정상 결과를 유지한다. 모든 참여자를 읽은 뒤 각 archive를 다시 검증해, 뒤쪽 읽기 동안 앞 자료가 바뀌어도 이전 근거를 내보내지 않는다. 요청 취소/기한 만료에는 부분 성공을 반환하지 않는다. Close는 진행 중 reader 정리를 기다리며 EOF/신호 종료를 지원한다.

body 예산은 출력 셀의 source 본문 byte 합계다. citation 예산은 셀과 비교 후보 proof의 모든 인용 객체를 합친 값이다. 예산에 맞지 않는 셀/후보는 통째로 제외하며 본문을 조용히 자르지 않는다. 후보 누락은 omitted_candidates/candidate_state로 표시한다. 전체 응답8MiB를 넘으면 오류로 거부한다. complete는 요청한 행/후보 처리 상태이며 프로젝트 근거 전체의 완전성이나 품질 승인이 아니다.

## 실제 검증과 사용 범위

[최종 동결 증거](../../system/eval/knowledge-roadmap-2026-10-16/km-06-real-projects/manifest.json)와 [검토용 행렬](../../system/eval/knowledge-roadmap-2026-10-16/km-06-real-projects/runtime/review.md)을 보존한다.

| 프로젝트 | 고정 원문과 캡처 범위 | 결과 |
|---|---|---|
| go-stablenet | 0bf2f4d1bfeb6605006d556957ef8c045d8f8ed8, committed | 실제 graph/vector·보관 원문2,224파일 |
| go-wemix | 1350376a680489298b3203a44d9ebddb7747f9cc, working-tree | 실제 graph/vector·보관 원문2,019파일; 소유 복제본의 cmd/gwemix→geth alias만 제외 |

원본 저장소는 변경하지 않았다. go-wemix의 BaseCommit·제외 목록·준비 상태를 기록했으며 cmd/geth의 정규 소스는 포함한다. 비교하는7개 Go 파일은 원본 커밋과 byte가 같다. 초기 매니저 수집 상한과 committed symlink 거부는 [준비 이력](../../system/eval/knowledge-roadmap-2026-10-16/km-06-real-preparation/preparation.json)과 최종 packet의 initial-preparation에 보존했다. 전체 원본이 순수 committed 캡처인 것으로 표현하지 않는다.

선택한 WBFT/Clique source excerpt와6개 개념(staking/staking-policy/consensus/finality/mempool-admission/transaction-ordering)·40셀을 조회했다.7개similar·18개unknown·15개not_collected는 미검토 매핑의 관찰이다. 실제 network/config를 실행하지 않았으며 staking 존재/부재, 블록 조립의 Finalize와 합의 finality, L2 settlement·공정성·검열 저항을 추정하지 않는다.

실제 비교MCP22항목·19도구호출·3EOF0, 원문/범위 hash와 본문·프로젝트/모드별 인용·양쪽 후보·기본 미검토 제외·live 소스 삭제 후 동일 응답·한쪽 archive 누락/오매핑·범위/예산 거부·원본/DB 보존을 검증했다. mock embedding과 유료 모델0회이며 의미 품질/온톨로지 효과·독립 수용은 아니다. 이전 계약/실행/부분 실패의 source-bound 증거는 각각 km-06-contract, km-06-runtime, km-06-failures packet에 보존한다.

Go1273개와 모든 결속 입력이 직전 서명 커밋dd554e79와 동일함을 확인해 전체Go4054(4036통과18skip)·race1028(1026통과2skip,7패키지)·빌드/fmt/vet/lint 결과를 재사용했다. 새 corpus/MCP 시험과 과거 Go 실행을 구분한다. 기존 온톨로지 기본 off·quality=inconclusive는 유지한다. 다음은 OE-01의 새 질문/기대 근거·동일 자료4모드·근거/순위/unknown/지연·다중 프로젝트 효과 평가이며 정책 생성기 보완·운영/독립 수용9개·선택 후속12개는 별도 남는다.


## SN-P04 동일 BGE 실제 프로젝트 재검증 (2026-10-18)

새 go-wemix BGE-M3/native1024 held 버전과 기존 go-stablenet retained 버전으로6개념·40셀·22검사를 재실행했다. 기존 mock64 자료는 그대로 보존한다. 두 프로젝트의 같은 canonical symbol도 다른 원문 hash를 가지며, source/version/project pin을 포함해 구분한다. 미검토 매핑은 기본 제외하고 staking 미수집과 finality unknown을 유지한다. 실제 엔진 선택은 설정에 의존하며 선택한 WBFT/Clique excerpt를 운영망 전체 정책으로 확대하지 않는다. 비교 question은 자동 개념 추론이 아니므로 concept/facets/조건·근거 연결을 명시해야 한다.

legacy raw hits는 자체 project 좌표가 없어 선택 서버/버전과 결속해야 하며 working-tree raw 요청은 requires_v2로 거부된다. Text는 색인 설명/추출 문구 또는 밀도 축약이며 코드 인용문이 아니다. 실제 v2 body는 보관 원문 byte로 다시 읽어 검증한다. 프로젝트 결과를 함께 전달할 때는 v2/비교의 전체 좌표를 유지한다. 실제 graph/vector 교차 구성2개는 요청 전 기동 거부했고 정상 세션은 EOF0으로 종료했다. corpus/정책 범위 비대칭 때문에 같은 모델의 결과 차이를 검색 우열/온톨로지 독립 효과로 판정하지 않는다. [SN-P04 증거](./operational-preparation/source-review-supplement/followups/stablenet-comparison-v1/manifest.json)와 작업리스트를 따른다.
