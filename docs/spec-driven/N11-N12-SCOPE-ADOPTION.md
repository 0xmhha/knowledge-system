# N11/N12 권고 범위 선택과 소스 준비

2026-10-08 최신 하위 단계: **N12 기능 version2의 새 실제 source/CKG·CKV/관계15 준비 검증**. [증거·기준별 누락](./N12-FUNCTIONAL-V2-TRACE-PREPARATION.md) — source1,437/CKG62·CKV65/742chunks·621canonical,15proposed 관계/11canonical/74span/27문서,related race7시험24하위시험·5거부·원 source/dataset/store 없이 archive 재생 통과. 전체Go DRIFT992는 유지했다. 개별 판정0/spec_unapproved3/활성0/새FINAL0, 전체8/18·요청5/15·남은10개 유지. 다음 N12 actual foreign-code-anchor 승격·active 보존 통합 시험 보강과 N11 gold·독립성/실제 검토자 판정. 남은 전체 N08/09/10/11/12/14/15/16/17/18. 이전 후보·승인input5는 보존하며 새 관계를 독립 FINAL로 세지 않는다.

2026-10-08 · 사용자 직접 응답 **“모두 권고 방향으로 진행”**. [선택 기록](../../system/eval/b0-knowledge-system/refactoring-scope-adoption-2026-10-08/scope-selection.json). **범위2개 선택 반영·문서 소스 검증 완료, 전체8/18·남은10개 유지**.

## 선택한 내용과 목적

N11 POLICY는 코드 입력·접근·버전 규칙인 `code_contract`와 조직 정책인 `organization_policy`를 포함한다. TRACE는 코드 검증 계약 `code_contract`와 사람이 검토한 실제 경로 `reviewed_path`를 구분한다. 코드 계약의 합격을 실제 운영 권위나 reviewed 경로 수용으로 바꾸지 않는다.

[새 질문 책](../../system/eval/b0-knowledge-system/refactoring-scope-adoption-2026-10-08/candidate-question-book.json)·[새 protocol](../../system/eval/b0-knowledge-system/refactoring-scope-adoption-2026-10-08/candidate-protocol.json)·[검토 양식](../../system/eval/b0-knowledge-system/refactoring-scope-adoption-2026-10-08/review-worksheet.json)을 작성했다. POLICY/TRACE7행은 code_contract로 분류했다(기존 검토 대상5행, 기존 제외2행). 기존 prompt/후보 답/군/원문/개별 검토 상태는 동일하다. 프로토콜의 candidate book SHA만 새 파일에 결합했고 runtime binding·freeze·개별 review는 미작성이다. 기존 pool의 DEV7/FINAL후보13/8제안군/제외6 및 중요군별 승인0을 유지한다.

N12는 요구3개와 수용 기준4개의 문장을 유지하고 requirement version2에서 기능 개념3개를 사용한다. 공통 vocabulary20개는 [원 core 파일](./ontology-pilot.yaml)에 그대로 남겼다. 선택 파일은 [기능 포함 vocabulary](./ontology-pilot-functional.yaml)23개와 [version2 명세](./spec-pilot-v2.yaml)이며, [PILOT-SCOPE.json](./PILOT-SCOPE.json)이 선택 경로를 기록한다. 원래 [version1 명세](./spec-pilot.yaml)와 과거 held source/projection/lock을 덮어쓰지 않는다.

| 요구 | 선택 개념 | 기준 |
|---|---|---|
| req-filtered-recall v2 | filtered-nearest-chunk-search | ac-filtered-recall, ac-filtered-completeness |
| req-source-evidence v2 | snapshot-bound-semantic-relations | ac-source-evidence |
| req-ontology-fallback v2 | optional-ontology-baseline-preservation | ac-ontology-fallback |

새23개 파일은 프로젝트 파일럿의 선택 vocabulary다. 기존20개 core의 ID/종류/정의/terms 등 내용은 생산 파서 결과와 대조해 같다. core 전용 ValidateCoreOntology는 원20개에 적용했고 새23개는 일반 ontology parser/validator로 검증했다. core 정의 수 제한을23개로 완화하거나 runtime schema를 바꾸지 않았다.

## 실제 검증과 경계

4문서만 넣은 소유 Git 소스 `33546d2339f3f1a4328fcf4d2b9d79a8f37a0a44`를 생성해 생산 ExtractOntology/ExtractSpec/ValidateSources와 Store.Put/Load를 실행했다. 임시 commit author는 준비 작업 표식이며 사람 검토자 이름이 아니다. prep dataset ID는 소스 준비용이며 coordinated CKG/CKV dataset/runtime 신원을 주장하지 않는다.

- core20 보존, functional3, 요구3/version2·criteria4의 동일 statement/Given/When/Then 확인.
- 원문30범위 SHA 검증 및30범위 각각의 해시 변조 거부. 작업 트리의 미커밋 편집이 과거 Git 근거를 바꾸지 않는 것도 확인.
- source-only SQLite 저장→닫기→재열기→Load의 JSON 바이트 동일, semantic_current0.
- [4문서 bundle](../../system/eval/b0-knowledge-system/refactoring-scope-adoption-2026-10-08/source-only-inputs.bundle)만 독립 temp Git로 복원해 동일 projection 재생. 생산 Trace로3요구 spec_unapproved/paths없음을 확인했다. [재생 결과](../../system/eval/b0-knowledge-system/refactoring-scope-adoption-2026-10-08/bundle-replay/replay-comparison.json).
- 실제 N11 `--require-ready` input checker는 exit2, execution_ready=false다. source/gold/basis 바이트 오류가 아니라 개별 판단·독립군·freeze 미작성 때문에 실행을 거부했다. 범위 승인으로 run 준비가 된 것으로 판정하지 않았다.
- 첫 Go 실행의 캐시 접근 거부는 별도 sandbox 원자료로 보존했고 캐시 허용 후 완료했다. runtime Go 코드 변경0이라 전체 회귀·실모델·Linux 시험을 새로 실행하지 않았다.

기존 원 core/spec/pool/protocol/review와 승인 input5개는 그대로다. 실제 held 후보·25proposed 관계·기존 색인은 변경하지 않는다. 새 개념 관계를 과거 snapshot에 승인된 것처럼 이식하지 않았다. source-only4파일에는 코드/시험 색인이 없어 canonical 관계와 engine 정렬을 증명하지 않는다. 다음 source capture는 선택 docs와 필요한 구현·시험 원문을 함께 결합해야 한다.

**실제 범위 선택2건, 개별 gold/개념/관계/criterion 승인0, 검토자 이름null, 활동 시간null.** 추천을 선택한 응답을 이름 입력이나 개별 내용 판정으로 해석하지 않는다. Claude·실모델 HTTP·새 FINAL·색인 빌드·운영 활성화0이다.

## 체크·다음

- [x] 직접 응답의 범위2개 선택 기록, N11 유형별 구분과 비실행 초안/양식 SHA 갱신.
- [x] N12 선택 source 파일 작성, 기존 core20와 requirement statement/criteria 보존.
- [x] 생산 source parser/30원문·30해시 거부·SQLite 재조회·bundle 독립 복원과 unapproved trace 상태 확인.
- [ ] 실제 검토자 이름과 문항별 gold/군·독립성·오염/ABS·protocol 판정.
- [ ] N12 선택 개념/요구/정확한 관계를 검토하고 새 full source capture/CKG·CKV/projection에 결합.
- [ ] 각 중요군10승인 FINAL 확보·동결 후 N08/09/10 평가, N14 실제 linked run/사람 criterion 수용.

**다음 우선순위:** scope 선택은 해결됐다. 개별 내용 검토 자료와 더 넓은 독립 후보를 보강하고, 필요한 구현·시험을 포함한 새 source 범위를 결합한다. 실제 검토자 이름은 채팅 텍스트로 받는다.

**현재8/18·남은 전체10개:** N08 검색 품질, N09 기권, N10 비용·지연, N11 독립 평가, N12 실제 파일럿 검토, N14 실제 변경 수용, N15 규모·설치·이식성, N16 신뢰·권한, N17 실제 복구, N18 기본값·출시 판정.
