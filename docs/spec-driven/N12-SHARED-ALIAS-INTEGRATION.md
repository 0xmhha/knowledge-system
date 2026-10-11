# N12 공유 alias: 생산 matcher와 근거 응답의 통합 검증

2026-10-08 · **소유한 원문 제어 fixture 검증 완료 / 실제 파일럿 검토 미완료**.

[영구 회귀 시험](../../internal/system/composer/stage2/ontology_matcher_integration_test.go) · [실행 요약](../../system/eval/b0-knowledge-system/refactoring-n12-shared-alias-2026-10-08/integration-summary.json) · [race 결과](../../system/eval/b0-knowledge-system/refactoring-n12-shared-alias-2026-10-08/race-tests-summary.json).

## 목적과 남아 있던 검증 차이

`ac-ontology-fallback`은 모호하거나 검토되지 않은 의미 해석 때문에 기준 검색 인용을 대체하지 않도록 요구한다. 이전 stage2 회귀는 가짜 matcher의 동률 후보를 받았고, 실제 held matcher 통합 자료는 서로 다른 두 용어를 확인했다. 같은 alias의 실제 동률이 생산 파서·저장소·matcher·stage2·근거 묶음을 함께 통과하는지는 별도로 검증할 필요가 있었다.

이번 시험은 실제 파일럿 원문이나 검토 상태를 변경하지 않고, 별도 소유 source에 공유 alias를 선언한다. 개념2개와9개의 두 snapshot-only corpus를 캡처한다. 영어 `shared`, 한글 `공유어`는 각 개념의 alternative term이며 전부 proposed/reviewer 없음이다. 구현 관계·code canonical·chunk link는 없다. graph/vector는 mock 좌표 manifest만 존재하고 그래프 검색 backend는 `ckgclient.Fake`, CKV 입력은 보관 README의 제어 인용3개다. 실제 hybrid 검색이나 임베딩 품질 시험으로 확대하지 않는다.

## 설계와 관측 결과

생산 `ExtractOntology`로 원문의 정확 spans를 추출하고 `PutAlignedRetained`로 보관 원문·좌표를 검증한다. 저장소를 닫은 다음 소유 source를 삭제하고 SQLite를 다시 열어 `LookupTerm` 및 `LoadAlignedRetained`를 사용한다. 이후 모든 vocabulary proof와 본문은 retained source에서 얻는다. semantic current는 선택하지 않는다.

| 입력 | matcher 직접 limit=1 | stage2 결과 | 기준 인용·점수·출처 |
|---|---|---|---|
| shared / 공유어를 확인, 개념2개 | 동률2개 모두 보존 | active, 후보2, boost0 | 동일 |
| shared / 공유어를 확인, 개념9개 | 동률9개 모두 보존 | budget_exceeded/concept_limit, 후보 출력 없음 | 동일 |
| meaning-00 | 파이프라인에서 단일 preferred match | active, 후보1, boost0 | 동일 |
| unshared / unknown | 파이프라인에서 match0 | no_match | 동일 |

indexed term lookup도 두 언어에서2/9개를 보존하고 영어 대문자 조회를 허용한다. 직접 cutoff4사례는 후보 ID 순서·proposed 상태·alternative score0.4 및 verified implementation0을 확인한다.9개를 임의의8개 또는 한 개로 잘라 의미를 확정하지 않는다. stage2의 상한 거부와 matcher의 동률 보존은 별도 계약이다.

각10질문 제어 사례에서 기준/optional stage2 결과의 capped scored citations2개가 완전히 같다. retained V2 Build는 동일 좌표·인용2개를 유지하며, SECRET 제어 marker를 포함한 본문을 정제하여 공개 본문1개/evidence_state=partial을 반환한다. ontology diagnostic을 붙이고 다시 Stamp/Verify해도 근거 내용은 동일하다. diagnostic이 포함되므로 전체 무결성 hash는 달라지고 diagnostic 변조10건은 거부된다.

이 시험은 source-backed vocabulary의 생산 matcher 통합이다. 승인된 구현의 rerank, reviewed overlay, 채워진 graph neighbors, 실제 모델/CKG/CKV 검색, 원 파일럿의 동일 alias 수용 또는 사람의 criterion 결정은 포함하지 않는다. 전부 proposed인 제어 사례의 성공을 실제 내용 승인으로 집계하지 않는다.

## 완료 체크와 실제 남은 일

- [x] 생산 파서·SQLite 재열기·동일 alias lookup/동률 matcher를 연결한 영구 통합 회귀 추가.
- [x] 영문/한글 동률2/9개, stage2 상한, 단일 preferred·영문 부분 문자열 배제·no-match10질문 제어 확인.
- [x] source 삭제 후 retained 원문·정제 본문·좌표/인용 보존과 diagnostic 무결성/변조10거부 확인.
- [x] focused1시험12하위시험 및 관련3패키지 race **116시험104하위시험**, 실패0; vet/boundaries/대상 format 통과.
- [ ] 실제 N12 명세 범위 선택과 개념·정확 관계의 실제 검토자/판정 기록.
- [ ] 선택 결과를 새 source capture/색인/semantic projection에 결합하고 실제 파일럿의 내용/기준을 검토.
- [ ] N11 gold/군/독립성·오염 판정 및 부족 독립 FINAL 표본 보강 후 동결·품질/비용 평가.

제어용 source2개와 SQLite2개만 만들었으며 기존 실제 held source/index/store는 열거나 변경하지 않았다. stage2 control Search20회/V2 Build20회, 실제 모델 HTTP0·새 DEV/FINAL dispatch0·새 독립 FINAL0·linked report0·실제 patch/승격0·사람 decision0·사람 활동 시간null이다. 생산 코드 변경 없이 시험만 추가했다. Go1.26.8 Darwin ARM64 실행이며 새 Linux/전체 Go suite 재실행은 없다. 이전 실행은 해당 커밋의 역사적 증거로 보존한다.

**전체8/18·요청5/15·남은10개 유지, N12-A–D/N14-A–E는 미체크다.** 다음은 실제 명세 범위·개념/관계와 N11 독립성 판정을 반영하는 것이다. N11 중요군별 승인0과 부족량을 변경하지 않는다. 남은 전체: N08 검색 품질, N09 기권, N10 비용·지연, N11 독립 평가, N12 파일럿 검토, N14 변경 수용, N15 규모·이식성, N16 신뢰·권한, N17 실제 복구, N18 기본값·출시 판정.
