# D1 승인 도메인 어휘 20개: 소유 데이터와 사용 경로

상태: **D1 개념 설계 승인** (2026-09-30). 이 문서는 [`ontology-pilot.yaml`](./ontology-pilot.yaml)의 20개 타입을 설명한다. 사용자 승인으로 타입의 뜻과 범위를 설계 기준선에 넣었지만, 원천별 `Concept`·`Assertion`·`Requirement`의 `status: verified` 또는 코드·테스트 통과로 자동 승격하지 않는다. 실제 객체의 원천 해시, 현재 데이터셋 좌표, 검토자를 확인해 별도로 승격한다. 현행 구현과 A 단계 목표를 구분한다.

## 20개의 정확한 의미

이 항목들은 CKS의 **공통 온톨로지 타입/어휘**다. 블록체인·의료·제조 등 산업의 업무 객체나 조직별 정책을 망라하지 않는다. 산업·프로젝트 지식의 확장안은 [`DOMAIN-KNOWLEDGE-PACK-PROPOSAL.md`](./DOMAIN-KNOWLEDGE-PACK-PROPOSAL.md)에 **D5 제안**으로 분리했다. 각각이 CKG의 AST 노드 종류, CKV 벡터 하나, SQLite 테이블 하나를 뜻하지 않는다. `Concept` 레코드는 ID·정의·범위·한영 용어·검토 상태를 가진다. `cks semantic build --ontology ...`가 YAML 블록마다 원문 줄과 SHA-256을 붙여 CKS 의미 투영에 싣고, 선택형 리졸버가 질의를 복수 개념 후보로 해석한다. 기본 검색은 원래 CKV+CKG 후보를 보존한다. 실행 기능이 없는 타입은 설계 어휘로만 존재한다.

| # | 타입 (`kind`) | 실제 데이터의 주 소유 위치 | 어떻게 사용되는가 | 현 상태 |
|---|---|---|---|---|
| 1 | `project` (`entity`) | CKS 프로젝트 설정·의미 투영의 `project_id` | 다른 저장소의 같은 경로/이름을 섞지 않는 격리 경계 | 현재 의미 투영에 ID; 안정 프로젝트 ID는 A3 |
| 2 | `dataset` (`artifact`) | CKS 활성 후보와 CKV/CKG/의미 매니페스트 | 같은 소스에서도 모델·빌드 입력별로 다른 불변 인덱스 묶음을 식별 | 현재 투영에 ID; 통합 후보는 A3 |
| 3 | `source-snapshot` (`artifact`) | CKS 원문 보관본·소스 매니페스트 | 커밋, 캡처 작업 트리, 비Git 바이트를 구별해 모든 인용의 기준으로 사용 | 현재 커밋 좌표; 확장은 A4 |
| 4 | `source-file` (`artifact`) | 캡처 원문과 CKV/CKG의 상대 경로 | 특정 스냅샷 안의 코드·문서 파일을 청크/심볼/근거와 조인 | 현재 Git 파일 경로; 확장은 A4 |
| 5 | `code-symbol` (`entity`) | CKG AST 그래프 | 실제 함수·타입·메서드의 `canonical_id`와 줄 범위를 `IMPLEMENTED_BY`/`TESTED_BY` 근거로 사용 | 현재 CKG/의미 앵커 존재 |
| 6 | `vector-chunk` (`artifact`) | CKV 벡터 DB | 문서·코드 원문 일부를 임베딩/BM25로 회수; 유사도는 사실 검증이 아님 | 현재 CKV 청크 존재 |
| 7 | `document-section` (`artifact`) | CKS 의미 투영, CKV 문서 청크와 조인 | 제목 범위와 원문 줄을 `Claim`의 문서 출처로 사용 | 현재 Markdown 섹션 추출 존재 |
| 8 | `claim` (`entity`) | CKS 의미 투영 | 검토 가능한 명제를 만들고 `SUPPORTS`/`CONTRADICTS`/`ABOUT` 관계로 연결 | 현재 수동·검토 경로 존재; 자동 사실 생성 아님 |
| 9 | `evidence-span` (`artifact`) | CKS 의미 투영·인용 | 원천 ID, 파일, 줄, 파일/범위 해시로 개념·관계·주장·테스트의 근거를 재검증 | 현재 커밋 줄 해시; v2 원천 좌표는 A4/A5.1 |
| 10 | `semantic-assertion` (`entity`) | CKS 의미 투영의 방향 있는 `Assertion` | 관계 자체의 양쪽 원천·상태·검토자를 보관해 단순 연결을 사실로 오인하지 않게 함 | 현재 타입 관계 검증 존재; 확장은 A5.1 |
| 11 | `concept` (`entity`) | 온톨로지 YAML → CKS 의미 투영 | 도메인 의미의 안정 ID; 질의 후보, `Claim ABOUT`, `IMPLEMENTED_BY`와 스펙 연결의 기준 | 현재 파싱/조회·선택형 재순위 존재 |
| 12 | `term` (`artifact`) | 각 `Concept`의 한영 용어 목록 | 표현을 개념 후보로 매칭; 다의어는 여러 후보로 남기고 일반어를 자동 확장하지 않음 | 현재 다의어 조회; 제한된 런타임은 A6 |
| 13 | `requirement` (`rule`) | 스펙 YAML → CKS 의미 투영 | 버전별 의도와 관련 `concept_ids`를 기록하고 구현/검증 상태를 추적 | 현재 파일럿 추출·추적 존재 |
| 14 | `acceptance-criterion` (`rule`) | 스펙 YAML의 Given/When/Then | 요구사항별 관찰 조건; 테스트 연결·실행·사람 판정을 각각 대조 | 현재 파일럿 추출; v2 판정은 A5.3 |
| 15 | `test-case` (`artifact`) | CKG 테스트 AST 심볼과 테스트 원문 | 코드 또는 수용 기준에 연결되는 실제 테스트 식별자 | 현재 CKG 앵커·정확 Go 테스트 경로 존재 |
| 16 | `test-run` (`process`) | CKS `semantic test`/설치 검사 보고서 | 특정 스냅샷/명령/환경/테스트 결과를 남김; 성공만으로 기준 승인하지 않음 | 현재 보고서 존재; 패치 연결은 A5.3 |
| 17 | `policy` (`rule`) | 프로젝트 설정·캡처/색인 정책·매니페스트 | 제외 경로, 접근/정화, 빌드 입력 규칙을 버전별로 기록해 데이터셋 신원에 반영 | 정책 일부 존재; 통합 좌표는 A3/A4 |
| 18 | `evidence-pack` (`artifact`) | CKS 공개 응답 | CKV 인용 본문과 CKG 이웃, 선택형 의미 주석을 제한·정화·무결성 검증해 반환 | 현재 v1 응답; v2 인용은 A7.1 |
| 19 | `patch-attempt` (`process`) | CKS 불변 변경 시도 기록 | 기준/결과 스냅샷, 변경 파일 다이제스트, 검사 보고서와 후보 데이터셋을 `patch_id`로 묶음 | **설계 승인, 저장/조회는 A5.3** |
| 20 | `criterion-decision` (`process`) | CKS 사람 판정 기록 | 특정 패치·명세 버전·수용 기준을 근거/검토자/이유로 `approved|rejected|needs_review` 판정 | **설계 승인, 저장/조회는 A5.3** |

`source-origin`은 별도 21번째 개념으로 만들지 않는다. `origin_id`와 상대 경로는 원문·인용의 **출처 좌표**다. `project`, `dataset`, `source-snapshot`도 서로 다르다. 같은 소스 스냅샷으로 모델 또는 색인 정책을 바꾸면 데이터셋이 달라진다.

## CKV·CKG·CKS 사이에서 쓰이는 순서

1. **입력과 신원:** `project`의 `policy`에 따라 `source-snapshot`/`source-file`을 캡처한다. `dataset`은 스냅샷과 CKV/CKG/의미 빌드 입력을 고정한다.
2. **기본 증거:** CKG는 `code-symbol`과 `test-case` AST 앵커를, CKV는 `vector-chunk`를 만든다. CKS는 `document-section`과 `evidence-span`을 원문 줄에 붙여 원천을 재검증한다.
3. **의미 연결:** YAML의 `concept`/`term`, 스펙의 `requirement`/`acceptance-criterion`, 검토된 `claim`/`semantic-assertion`으로 요구→의미→코드→테스트 경로를 만든다. `AcceptanceCriterion CHECKED_BY TestCase`는 **검토된 연결**일 뿐 수용 판정이 아니다.
4. **실행과 사람 판정:** `test-run`은 그 스냅샷에서 실행·통과했는지를 기록한다. 외부 변경은 `patch-attempt`로 묶고 사람의 `criterion-decision`이 기준의 의미 충족을 별도로 판단한다. 실패 또는 미승인 패치는 활성 `dataset`으로 승격하지 않는다.
5. **질의:** CKS는 원문 질의를 CKV/CKG에 먼저 보내고 `evidence-pack`을 만든다. 선택형 온톨로지는 검토된 개념·관계로 **기존 후보**를 주석/제한적으로 재순위화한다. 모호성·오류·예산 초과에서는 CKV+CKG 기본 후보를 유지한다. B/C 실모델 평가 전 기본 활성화는 꺼져 있다.

## D1 관계와 상태의 해석

`DocumentSection SUPPORTS Claim`, `Claim CONTRADICTS Claim`, `Claim ABOUT Concept`, `Concept IMPLEMENTED_BY CodeSymbol`, `CodeSymbol TESTED_BY TestCase`, `AcceptanceCriterion CHECKED_BY TestCase`가 설계상 최소 경로다. `CHECKED_BY`는 기존 `ACCEPTED_BY`의 이름이 만든 승인 오해를 제거한 **새 정식 술어**다. 현재 실행 코드와 기존 의미 DB는 `ACCEPTED_BY`를 사용하므로 A5.1에서 읽기 전용 호환 변환과 새 쓰기/검증 계약을 구현하고, 구버전 소비자 회귀를 확인한다. 명칭만 바꿔 기존 기록이 사람 승인으로 바뀌지는 않는다.

`linked`(검토된 경로 존재), `tested`(실행 성공), `accepted`(사람의 `CriterionDecision.approved`)는 서로 다른 상태다. D1은 이 **의미 모델**을 승인한 것이며, 실제 프로젝트의 요구사항 충족률·검색 성능·20개 개념별 검토 완료를 선언하지 않는다. 여섯 능력 질문의 구조적 허용 답/금지 해석은 [`DESIGN-GATES-EVIDENCE.md`](./DESIGN-GATES-EVIDENCE.md)에 고정하고 질문별 실모델 정답 파일/줄과 검색 품질은 B0/B1에서 평가한다.
