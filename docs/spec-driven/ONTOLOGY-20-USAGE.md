# 공통 온톨로지20개와 현재 사용 경로

[ontology-pilot.yaml](./ontology-pilot.yaml)의 공통 타입 정의다. 각각이 AST 노드 하나나 DB 테이블 하나라는 뜻은 아니다. 타입 설계 승인과 실제 객체의 verified·원문/스냅샷 검증·사람 수용은 분리한다. 산업/조직 타입은 [선택형 팩](./DOMAIN-PACK-CONTRACT-V1.md)에 두며 core20개를 재정의하지 않는다.

| # | 타입(kind) | 실제 소유 위치 | 사용 목적 |
|---|---|---|---|
|  1  |  `project` (`entity`)  |  CKS 프로젝트 설정·의미 투영의 `project_id`  |  다른 저장소의 같은 경로/이름을 섞지 않는 격리 경계  |
|  2  |  `dataset` (`artifact`)  |  CKS 활성 후보와 CKV/CKG/의미 매니페스트  |  같은 소스에서도 모델·빌드 입력별로 다른 불변 인덱스 묶음을 식별  |
|  3  |  `source-snapshot` (`artifact`)  |  CKS 원문 보관본·소스 매니페스트  |  커밋, 캡처 작업 트리, 비Git 바이트를 구별해 모든 인용의 기준으로 사용  |
|  4  |  `source-file` (`artifact`)  |  캡처 원문과 CKV/CKG의 상대 경로  |  특정 스냅샷 안의 코드·문서 파일을 청크/심볼/근거와 조인  |
|  5  |  `code-symbol` (`entity`)  |  CKG AST 그래프  |  실제 함수·타입·메서드의 `canonical_id`와 줄 범위를 `IMPLEMENTED_BY`/`TESTED_BY` 근거로 사용  |
|  6  |  `vector-chunk` (`artifact`)  |  CKV 벡터 DB  |  문서·코드 원문 일부를 임베딩/BM25로 회수; 유사도는 사실 검증이 아님  |
|  7  |  `document-section` (`artifact`)  |  CKS 의미 투영, CKV 문서 청크와 조인  |  제목 범위와 원문 줄을 `Claim`의 문서 출처로 사용  |
|  8  |  `claim` (`entity`)  |  CKS 의미 투영  |  검토 가능한 명제를 만들고 `SUPPORTS`/`CONTRADICTS`/`ABOUT` 관계로 연결  |
|  9  |  `evidence-span` (`artifact`)  |  CKS 의미 투영·인용  |  원천 ID, 파일, 줄, 파일/범위 해시로 개념·관계·주장·테스트의 근거를 재검증  |
|  10  |  `semantic-assertion` (`entity`)  |  CKS 의미 투영의 방향 있는 `Assertion`  |  관계 자체의 양쪽 원천·상태·검토자를 보관해 단순 연결을 사실로 오인하지 않게 함  |
|  11  |  `concept` (`entity`)  |  온톨로지 YAML → CKS 의미 투영  |  도메인 의미의 안정 ID; 질의 후보, `Claim ABOUT`, `IMPLEMENTED_BY`와 스펙 연결의 기준  |
|  12  |  `term` (`artifact`)  |  각 `Concept`의 한영 용어 목록  |  표현을 개념 후보로 매칭; 다의어는 여러 후보로 남기고 일반어를 자동 확장하지 않음  |
|  13  |  `requirement` (`rule`)  |  스펙 YAML → CKS 의미 투영  |  버전별 의도와 관련 `concept_ids`를 기록하고 구현/검증 상태를 추적  |
|  14  |  `acceptance-criterion` (`rule`)  |  스펙 YAML의 Given/When/Then  |  요구사항별 관찰 조건; 테스트 연결·실행·사람 판정을 각각 대조  |
|  15  |  `test-case` (`artifact`)  |  CKG 테스트 AST 심볼과 테스트 원문  |  코드 또는 수용 기준에 연결되는 실제 테스트 식별자  |
|  16  |  `test-run` (`process`)  |  CKS `semantic test`/설치 검사 보고서  |  특정 스냅샷/명령/환경/테스트 결과를 남김; 성공만으로 기준 승인하지 않음  |
|  17  |  `policy` (`rule`)  |  프로젝트 설정·캡처/색인 정책·매니페스트  |  제외 경로, 접근/정화, 빌드 입력 규칙을 버전별로 기록해 데이터셋 신원에 반영  |
|  18  |  `evidence-pack` (`artifact`)  |  CKS 공개 응답  |  CKV 인용 본문과 CKG 이웃, 선택형 의미 주석을 제한·정화·무결성 검증해 반환  |
|  19  |  `patch-attempt` (`process`)  |  CKS 불변 변경 시도 기록  |  기준/결과 스냅샷, 변경 파일 다이제스트, 검사 보고서와 후보 데이터셋을 `patch_id`로 묶음  |
|  20  |  `criterion-decision` (`process`)  |  CKS 사람 판정 기록  |  특정 패치·명세 버전·수용 기준을 근거/검토자/이유로 `approved |

공유 원문 좌표의 origin_id는21번째 개념이 아니다. dataset은 source snapshot에 모델/팩/빌드 정책을 더한 불변 신원이다. 정책의 의도와 AST의 구현 사실을 별도 검토된 관계로 연결한다.

현재 의미 투영은 core schema3과 tuple-scoped pack schema4를 지원한다. 새 기준→시험 연결은 CHECKED_BY이며 구 ACCEPTED_BY는 읽기 호환 경로로 처리한다. 연결 존재(linked), 시험 실행(tested), 사람 기준 판정(accepted)은 별개다. patch-attempt와 criterion-decision은 저장/조회·실제 검증 흐름에 구현되어 있다.

온톨로지 MCP의 status/lookup/get/trace와 요청별 bounded 설명은 [현재 가이드](../../system/docs/ontology-mcp.md)를 따른다. 조회 가능한 데이터와 검색 활성 상태는 다르며 기본 검색 모드는off다. 개별 도메인 효과와 독립 품질은 [평가 상태](./EVALUATION-STATUS.md)의 한계를 따른다.
