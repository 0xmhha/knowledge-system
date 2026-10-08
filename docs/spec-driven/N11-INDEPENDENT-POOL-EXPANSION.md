# N11 독립 FINAL 후보 보강과 검토 범위의 부족분

2026-10-08 · **검토 자료 준비 완료 / N11 전체 미완료** · [후보 책](../../system/eval/b0-knowledge-system/refactoring-n11-pool-expansion-2026-10-08/candidate-question-book.json) · [실측 대조](../../system/eval/b0-knowledge-system/refactoring-n11-pool-expansion-2026-10-08/summary.json) · [판정 양식](../../system/eval/b0-knowledge-system/refactoring-n11-pool-expansion-2026-10-08/review-worksheet.json)

목적은 반복·번역·기존 관측을 새 표본으로 세지 않고, 기존 목표의 검색 품질/정책/추적/기권을 평가할 독립 사실군을 확보하는 것이다. 질문을 많이 작성했다는 이유로 평가 준비나 제품을 완료 처리하지 않는다. 중요군별 **승인된 독립 FINAL 10개 이상** 조건은 유지한다.

## 이번에 확인한 진행

이전 검토 대상9문항/6제안군에 CODE·WHY4문항/2제안군을 보강했다. 별도로 작성한4문항/2군은 이미 다룬 개념 모호성·trace 수용 범위와 겹칠 위험 때문에 격리했다. 기존 quorum의2제외 문항도 보존한다. 새 책은 DEV7+검토 대상 FINAL13=20문항, 제외 catalog6문항이며 **승인된 독립 FINAL은 모든 중요군에서0개**다. CODE·WHY 쌍은 동일한 사실군을 공유한다.

| 중요군 | 제안 사실군 | 실제 승인 | 승인 기준까지 실제 부족 | 모든 제안이 승인될 경우에도 추가로 필요한 군 |
|---|---:|---:|---:|---:|
| CODE | 4 | 0 | 10 | 6 |
| WHY | 4 | 0 | 10 | 6 |
| POLICY | 1 | 0 | 10 | 9 |
| TRACE | 2 | 0 | 10 | 8 |
| ABS | 1 | 0 | 10 | 9 |

마지막 열은 낙관적 준비량 계산이며 승인·독립성을 예측한 수치가 아니다. 서로 공유하는 CODE/WHY 군 때문에 열의 합을 전체 고유 표본 수로 쓰지 않는다. 전체 고유 제안군은8개다.

## 새4문항의 목적·원문·후보 답

**CAND12 · CODE / CAND13 · WHY — semantic-term-index-migration-integrity**

- 질문12: 용어 색인 마이그레이션은 기존 projection에서 어떤 무결성·신원 검사를 마친 뒤 user_version=3을 커밋하는가?
- 후보 답12: 저장 document SHA-256/digest, JSON 해석, Projection.Validate, 행 project/dataset과 Snapshot 일치를 확인한다. 같은 트랜잭션에서 용어를 삽입하고 rows 오류·Close 오류 없이 user_version=3을 설정해 Commit한다. 오류 경로는 defer Rollback으로 커밋하지 않는다.
- 질문13: 손상되거나 다른 project/dataset의 projection에서 만든 용어 색인의 부분 커밋을 어떻게 막는가?
- 후보 답13: 위 검증·삽입·버전 변경을 같은 트랜잭션에 두므로 오류가 나면 Commit에 도달하지 않는다. 이는 코드 흐름에서 추론한 동작이다. 별도 ADR의 설계 의도나 실제 장애 복구 보장은 이 원문으로 증명하지 않는다.
- 원문: [store_terms.go:13](../../internal/system/semantic/store_terms.go:13), 13–68행. WHY 범주와 추론의 타당성을 별도로 사람에게 판정받는다.

**CAND14 · CODE / CAND15 · WHY — spec-span-identity-construction**

- 질문14: ExtractSpec의 EvidenceSpan ID/ContentSHA256은 어떤 바이트·좌표로 만들어지는가?
- 후보 답14: LF를 보존한 start–end 양끝 포함 본문 SHA-256, project_id/dataset_id/상대 파일/start:end/content digest를 NUL로 구분한 key의 SHA-256 앞12바이트 hex와 prefix로 ID를 만든다. ContentSHA256은 본문 digest 전체 hex다.
- 질문15: 같은 본문을 다른 project/dataset 또는 파일·행에서 추출하면 같은 ID로 취급하는가?
- 후보 답15: 좌표가 key에 포함되어 해시 입력이 달라진다. ID는96비트로 잘리므로 충돌 불가능을 보장한다고 말하지 않는다. 분리 효과는 구현에서의 추론이며 ADR의 의도 확인은 아니다.
- 원문: [extract_spec.go:55](../../internal/system/semantic/extract_spec.go:55), 55–74행. WHY 분류·추론과 다른 source-evidence 사실군의 개념적 중복을 검토한다.

두 군 모두 해당 커밋의 구현 계약이 근거 범위다. 조직 권위의 정책이나 사람이 수용한 실제 trace 경로로 확대하지 않는다. 새 질문·후보 답은 모델 검색 결과를 보고 조정하지 않았다.

## 작성 후 격리한4문항

CAND16/17은 MatchConcepts의 cutoff 동점 유지, CAND18/19는 ResolveTracePath의 단일 exact 경로와 테스트 실행/사람 수용의 구분이다. 과거 원자료에서 이 함수명의 literal 검색/직접 인용은0이지만 이를 새 사실이라고 판단하지 않았다.

- `B0-DYNAMIC-FIXTURES.md:15`, `EXECUTION.md:207`, N12의 P12-R03/A04는 이미 개념 모호성·기본 후보 보존을 다뤘다.
- `B0-M2MAX-REVIEW.md:465`에는 ResolveTracePath 호출이 공개된 검토 자료에 포함되어 있다. B0-TRACE-01, B0-WHY-01/03, N14의 실제 변경 검토도 reviewed trace와 사람 수용의 분리를 다뤘다.

그래서 신규 두 군은 제외 catalog에 기록해 FINAL 요청 대상으로 넣지 않는다. 이는 보수적 준비 선택이다. 직접 인용 부재를 개념적 오염 부재로 바꾸지 않으며, 자동화가 사람의 완전한 오염 판정을 했다고 주장하지 않는다. [커밋 문서 노출7건](../../system/eval/b0-knowledge-system/refactoring-n11-pool-expansion-2026-10-08/committed-document-mentions.json)에는 store_terms.go/extract_spec.go의 초기 목적 문서 언급도 남겼다. 나머지 두 군도 source 노출이 없었다고 보장하지 않는다.

## 이중 확인과 실제 실행 한계

1. source commit `479491baa50f818a5f7bb7651f35e9c7192ad906`에서 기존16문항의 positive/ABS basis 파일·span SHA/좌표가 그대로임을 확인하고 source_commit만 재결합했다. 신규4개를 합친20원문 참조 및 제외6문항도 현행 커밋 바이트와 일치한다. 과거 책·승인 입력·gold·runtime은 수정하지 않았다.
2. 과거 FINAL9gzip/4,800행의 compressed/decompressed SHA를 이전 대조 원장과 다시 확인했다. DEV/공식 정적 기록까지 **27gzip/11,632행**을 읽고 compressed 바이트를 현재 커밋과 확인했다. 나머지18파일의 decompressed SHA는 이번 대조에서 산출한 값이며 이전 원 manifest의 decomp SHA를 모두 재확인했다는 뜻은 아니다.
3. 신규·격리8문항 모두 명시적 citations에서 동일 path/동일 fileSHA+span 교집합0, structuredContent의 선택 함수명 문자열0이다. 직접 인용·선택 literal만 검사했다. graph body·의미 중복·개발 중 튜닝·사람의 과거 노출을 완전히 찾는 검사가 아니다. B0/N11DEV/N12 원문 참조 catalog도 별도 대조했다.
4. 기존 held protocol의 정보 필드가 FINAL4/군2 및 이전 candidate_book SHA를 유지했으나 실제 책은9/군6이었다. 기존 기록은 그대로 보존하고 새 초안의 정보 필드를13/군8/새 책SHA로 맞췄다. 이 필드는 실행 승인을 증명하지 않는다.
5. 새 책/source와 과거 held dataset source19803801은 다르다. 기존 runtime binding을 새 초안에 자동 복사하지 않고 `execution_binding=null`, `execution_allowed=false`, review/freeze/results=null로 둔다. 새20문항 전체가 이전 색인과 정렬됐다고 주장하지 않는다. 필요한 corpus 범위와 검토가 정해진 뒤 한번에 새 신원을 동결한다.
6. 실제 input checker는 `--require-ready` exit2로 거부했다. source/gold/ABS basis 오류·source mismatch·제외 재삽입 오류는0, 실제 승인/단위/동결은 누락 상태다. DEV7이 원문 목록에 존재해도 승인된 cluster가0이라 `DEV_missing`이 나오는 것은 목록이 없다는 뜻이 아니다. 모델 HTTP/검색/FINAL 예약/코퍼스 빌드0, 실제 사람 활동 시간null, sampling=inconclusive, 전체 완료false다.

소스 코드 변경은 없다. 이전25개 checker/runner 시험 결과를 이번에 다시 실행한 것처럼 쓰지 않는다. 이번 검증은 실제 확장 입력의 원문·과거 자료·메타데이터·거부 결과다. [재현용 감사 소스](../../system/eval/b0-knowledge-system/refactoring-n11-pool-expansion-2026-10-08/pool-audit.py.txt)는 저장했고 현재 미승인 결과를 제품 합격으로 세지 않는다.

## 다음 우선순위와 남은 전체

**P1 N11/N12:** 새 CAND12–15와 기존 후보·DEV/protocol의 gold/군/독립성/오염/범위 판정. CODE/WHY 추론을 WHY에 넣을지도 판정한다. POLICY는 코드 입력 규칙을 어느 정책 유형으로 인정할지와 실제 소유자/유효 시점/권위를 정해야 한다. TRACE는 코드의 검증 계약과 실제 reviewed ADR→요구→기준→code/test 경로의 평가 범위를 구분한다. ABS는 bound corpus/시점의 무답 필요성과 strict 인용0을 별도 판정한다. 각각 같은 fixture 변형을10개로 세지 않고 부족한 실제 독립 범위를 보강한다. N12의33항목/12관계 판정과 연결한다.

**P2 N08/09/10/N14:** 위 입력과 runtime을 동결한 뒤 DEV 개선→새 독립 FINAL 품질/기권·8arm 비용/지연을 평가한다. 실제 변경은 canonical trace·held patch/linked run·사람 criterion 수용과 promote/reject/rollback까지 수행한다.

**P3 N15/16/17 → N18:** 실제 규모·모델·독립 consumer 갱신, 역할/키 신뢰·회전/폐기·전이 notices, 실제 담당자/RTO/RPO와 복구 drill을 수용한 뒤 기본값/최종 출시 판정한다. 현재 source의 Linux 검증도 남아 있다.

현재 전체 **8/18**, 사용자 요청 N04–18 **5/15** 완료. **남은 전체10개:** N08 검색 품질, N09 무답·기권, N10 비용·지연, N11 독립 DEV/FINAL, N12 실제 파일럿 검토, N14 실제 변경 추적·수용, N15 규모·설치·이식성, N16 운영 신뢰·권한, N17 운영 복구, N18 최종 기본값·출시 판정. N11-A–D 및 다른 전체 미완료 항목의 checkbox를 변경하지 않았다.
