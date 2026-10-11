# N12 실제 code anchor 승격·active 보존 통합 회귀

2026-10-09 후속: [새 source/index/관계 결합](./N12-ANCHOR-BOUND-TRACE-REVIEW.md)을 별도 후보로 검증했다. 아래 체크와 원자료는 이 문서 작성 당시의 immutable 후보·단계 기록이다. 새 후보는 시험을 포함하지만 실제 내용 승인/파일럿 수용은 미완료다.

2026-10-08 · **누락된 통합 시험 보강 완료 / N12 전체 미완료**. 전체8/18·요청5/15·남은10개 유지.

## 목적과 이전 부족분

[기능 version2 준비](./N12-FUNCTIONAL-V2-TRACE-PREPARATION.md)의 ac-source-evidence는 다른 snapshot의 code anchor를 promotion에 넣을 때 거부하고 이전 active를 보존해야 한다. 기존 alignment 시험은 manifest 변조를, code anchor 시험은 helper를 검사했다. 이 두 통과를 하나의 actual canonical promotion 경로를 시험한 결과로 합치지 않았다.

[새 시험](../../internal/system/semantic/anchor_promotion_integration_test.go)은 공개 CKG CLI를 빌드해 소유 Git의 실제2commit/2graph DB를 생성한다. Alpha의 canonical ID는 같고 source bytes/commit은 다르다. 생산 source capture·ontology parser·AttachCodeEvidence·PutAligned/PutAlignedRetained·ActivateAligned/ActivateAlignedRetained와 SQLite를 거친다. 임의 AST/node를 DB에 직접 넣어 actual canonical이라고 주장하지 않는다.

CKV는 coordinate-only mock manifest이며 vector DB/chunk links/embeddings가 없다. 실제 파일럿 코퍼스/배포/실모델/사람 수용 시험은 별도다. fixture vocabulary/구현 관계는 proposed이며 fixture commit author를 검토자로 쓰지 않는다.

## 통과한 정확한 제어

| 사례 | 관측 |
|---|---|
| foreign-evidence-snapshot | 실제 이전 source의 code evidence를 candidate에 넣으면 snapshot 검사에서 거부 |
| relabelled-foreign-source-bytes | 이전 code anchor의 snapshot label만 candidate로 고쳐도 실제 보관 source span SHA에서 거부 |
| stale-actual-graph-with-matching-JSON | candidate graph JSON/CKV 좌표는 일치하도록 두고 실제 DB만 이전 것으로 교체; ValidateDatasetAlignment positive control은 통과하지만 실제 CKG canonical/commit 검증에서 거부 |
| missing-actual-canonical | typed proof/관계 대상은 서로 맞춰도 실제 CKG에 없는 symbol은 거부 |
| activation-rechecks-actual-graph | valid candidate는 저장하고 active는 base로 유지; 실제 이전 DB를 넣은 두 activation API는 거부 |

첫4사례는 live Git와 retained의2쓰기 API에서 각각 거부되어 **8거부**다. 거부된 후보는 Store.Load에서 ErrNoRows이며 매 거부 뒤 기존 current와 원 저장 projection이 정확히 같다. 마지막 activation2거부 뒤 닫기→read-only 재열기에서도 base current 전체 projection이 같다. 실제 fixture baseline activation은 실행마다1회이며 실제 파일럿/운영 current 변경은0이다.

## 검증과 작성 오류

- [x] 실제 CKG2build/2source snapshots, 같은 canonical·다른 source,5통합 사례/8쓰기·2activation 거부, positive control과 저장/active 보존·read-only reopen 확인.
- [x] 수정한 focused race1시험/5하위시험 통과. semantic 패키지 전체 race42시험/20하위시험/1패키지 pass, fail0/skip0. [원자료 요약](../../system/eval/b0-knowledge-system/refactoring-n12-anchor-promotion-2026-10-08/related-race-summary.json).
- [x] semantic vet·엔진 경계·gofmt·diff 검사 통과. 새 runtime 제품 코드 수정0이며 추가한 Go 파일은 시험뿐이다. 전체 repository·Linux·실모델 시험을 새로 실행했다고 기록하지 않는다.
- [x] 첫 시험 fixture의 competency_questions flow 문자열을 YAML parser가 거부한 작성 오류를 initial source/command/raw로 보존. 문자열을 인용해 수정한 뒤 통과했다. 제품 RED·승격 거부·성공 사례로 세지 않는다.
- [x] 직전 held 준비66artifact와 승인5입력의 SHA 보존. source/current/projection을 덮어쓰거나 과거 승인/checked 관계를 새 시험에 이식하지 않았다.
- [ ] 새 시험을 새 pilot source/index와 canonical TESTED_BY/CHECKED_BY 검토 자료에 결합. 기존 immutable55d8/43ba/f839 후보에는 이 시험이 포함되지 않는다.
- [ ] 실제 검토자·개념/요구/관계/criterion 판단과 actual linked run·파일럿 수용.

[기술 요약](../../system/eval/b0-knowledge-system/refactoring-n12-anchor-promotion-2026-10-08/technical-summary.json). 이 단계는 이미 있던 production gates의 전체 경로 회귀 coverage를 추가했다. 제품 버그를 고쳤다는 주장이나 실제 사용자 반려/사람 승인 기록은 아니다. model/FINAL/actual pilot activation/semantic linked run0·사람 판정0·활동시간null·검토자null이다. fixture를 한 번 더 실행한 결과를 독립 평가 표본으로 세지 않는다.

## 다음과 남은 전체

다음은 새 시험을 포함한 별도 source/canonical 관계 준비다. 실제 체크할 내용은 새 시험 source SHA/AST, ValidateDatasetAlignment 간접 호출과 actual canonical·source gates의 역할, 원 ac-source-evidence 문장 보존 및 기준 연결의 의미다. 이후 개별 내용 검토를 받고 N11 독립 gold/군·오염/ABS·protocol·runtime을 동결한다.

| 우선순위 | 남은 전체 작업 |
|---|---|
| 1 | N12 실제 파일럿 검토·새 시험 source/관계 결합 |
| 1 | N11 독립 평가 입력·사람 gold/군/범위 판정·충분한 FINAL 확보 |
| 2 | N08 검색 품질, N09 기권, N10 비용·warm/cold 지연 실측 |
| 3 | N14 실제 변경/linked run·criterion 수용, N15 규모·설치·이식성·독립 소비자 |
| 3 | N16 신뢰·owner·역할·키·고지, N17 실제 운영 복구 |
| 4 | N18 기본값·최종 수용·출시 판정 |

완료는 N01–07/N13의8개다. N11/N12의 전체 체크박스를 완료로 바꾸지 않았다. Codex 직접 수행이며 새 외부 transfer/Claude0이다.
