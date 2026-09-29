# CKS 의미 투영 계약 v1

이 문서는 W2.1–W2.2의 현재 구현 계약을 기록한다. 전체 온톨로지나 자동 사실 판정이 완성됐다는 뜻은 아니다. 상위 설계와 수용 조건은 `study/docs/reviews/knowledge-system/spec-driven-architecture.md` 및 `spec-driven-requirements.md`에 있다.

## 원본과 신원

하나의 `Projection`은 `project_id`, `dataset_id`, 40자리 Git 커밋과 스키마 버전에 고정된다. 현재 쓰기 버전은 2이며 과거 버전 1 투영은 읽기와 롤백이 가능하다. 의미 데이터는 CKG/CKV 원본을 대체하지 않는다. `EvidenceSpan`은 문서·코드·테스트 종류, 저장소 상대 경로, 1부터 시작하는 포함 줄 범위, 그 범위의 원문 SHA-256, 추출기 이름을 가진다. `canonical_id`와 `chunk_id`는 선택적인 조인 키이며 원문 증명을 대신하지 않는다.

`Validate`는 ID 중복, 고아 참조, 경로 탈출, 잘못된 상태, 프로젝트/데이터셋/커밋 혼합을 거부한다. `ValidateSources`는 **기록된 커밋의 Git 객체**를 읽어 줄 범위와 원문 해시를 다시 계산한다. 현재 작업 트리의 수정 사항으로 과거 근거를 덮어쓰지 않는다. 현재 구현의 근거 검증 대상은 같은 Git 저장소에 커밋된 파일이다. 외부 문서와 작업 트리 스냅샷은 별도 계약이 필요하다.

## 사실의 상태

`DocumentSection`은 CKV와 동일한 Markdown 제목 파서의 줄 경계를 사용한다. 섹션 추출은 `Claim`을 만들어 사실이라고 주장하지 않는다. `Claim`은 최소한 원천 섹션과 그 섹션의 `EvidenceSpan`을 참조해야 한다. 추출·제안된 주장은 `proposed`로 시작한다. `verified`에는 명시적 `reviewed_by`가 필요하며 승격 전 `ValidateSources`를 통과해야 한다. 검토자가 있더라도 스냅샷이 다르거나 원문 해시가 맞지 않으면 저장할 수 없다. `rejected`는 삭제 대신 검토 이력에 남길 상태다.

`cks semantic build`는 문서 섹션의 원문 파일·커밋·줄 범위와 겹치는 실제 CKV Markdown `doc` 청크를 조회해 `DocumentSection.chunk_ids`에 안정 ID를 기록하고, 연결된 섹션 수를 출력한다. 청크가 없는 섹션은 비어 있는 목록으로 남아 있어 연결률을 확인할 수 있다. 승격·활성화·활성 조회는 **기록된 청크 ID가 현재 CKV DB의 동일 파일·커밋·줄 위치를 여전히 가리키는지** 다시 검사한다. 삭제되거나 다른 위치에 재사용된 ID는 의미 경로를 반환하지 않는다. 원문 `EvidenceSpan` 해시 검증과 CKV 청크 조인은 별도 검사다.

`cks semantic review --input projection.json --repo <git-root> --sample 20 --salt <audit-id>`는 JSON 투영의 모든 근거를 기록된 커밋에서 재검증한 뒤 검토 대기 주장 샘플과 집계 JSON을 출력한다. 표본은 데이터셋 ID·주장 ID·salt의 해시로 결정되므로 같은 입력은 같은 목록을 만든다. `verified`와 `rejected` 모두 검토자 ID가 있어야 한다. 정밀도는 **검토된 주장 중 verified 비율**이며 검토 건수가 0이면 `null`이다. 이 수치는 실제로 검토된 표본의 품질만 설명한다. 대표 표본의 외부 검토가 없으면 추출기 전체의 정밀도나 관계 정확도로 해석하지 않는다. 현재 자동 관계 추출과 관계 평가셋은 없다.

관계 후보 `Assertion`은 자체 근거·상태·검토자를 가진다. 허용 타입은 `DocumentSection SUPPORTS Claim`, `Claim CONTRADICTS Claim`, `Claim ABOUT Concept`, `Concept IMPLEMENTED_BY CodeSymbol`이다. 관계마다 양쪽 원천 근거를 요구하며 `verified` 관계는 연결된 주장·개념도 검토 완료 상태여야 한다. 코드 구현 관계는 코드 근거의 `canonical_id`가 객체 ID와 같아야 하고, 승격 단계에서 실제 CKG의 동일 커밋 AST 심볼·파일·줄 범위를 검사한다. 방향 오류, 고아 참조, 무근거, 무검토 승격은 거부한다. 검토 도구는 주장·관계·개념의 정밀도를 따로 출력한다. `RELATED_TO` 같은 막연한 관계는 확정 타입으로 사용하지 않는다.

코드 근거를 붙일 때는 CKG 공개 읽기 API에서 `canonical_id`를 정확 조회하고, CKG의 소스 커밋·파일·AST 노드 줄 범위가 근거와 일치하는지 검사한다. 이 검사는 원문 SHA-256 검사와 별개이며 `verified`로 자동 승격하지 않는다.

## 온톨로지 시범 팩

[`ontology-pilot.yaml`](./ontology-pilot.yaml)은 knowledge-system 자체를 대상으로 한 **제안 상태**의 18개 개념과 6개 능력 질문이다. 각 개념은 안정 ID, `entity/artifact/process/rule` 중 한 종류, 문장형 정의, 포함·제외 범위, 언어별 선호·대체 용어, 검토 상태를 가진다. `ExtractOntology`는 YAML의 각 개념 블록을 정확한 Git 원문 줄·SHA-256 근거로 만든다. `ValidateSources`는 원문 해시뿐 아니라 정의·용어·상태를 YAML에서 다시 읽어 투영과 비교하므로 JSON 투영만 수정해 정의를 바꿀 수 없다. `verified`/`rejected`에는 검토자 이름과 원본 YAML 변경이 필요하다.

`cks semantic build`의 `--ontology`에 저장소 상대 YAML 경로를 주면 Markdown 섹션과 함께 동일한 프로젝트/데이터셋/커밋 투영에 저장한다. `cks semantic review`는 개념 표본·검토된 정밀도·여러 개념이 공유하는 동일 언어 용어를 별도 항목으로 출력한다. 동음이의어는 자동으로 같은 개념이라고 병합하지 않는다. 현재 시범 팩에는 전문가 리뷰가 없으므로 18개 모두 `proposed`다.

## SQLite 투영과 롤백

`semantic_projections`는 `(project_id,dataset_id)`별 JSON 문서와 SHA-256을 불변으로 저장한다. 같은 내용의 재시도는 허용하고 같은 ID의 다른 내용은 거부한다. `semantic_current`는 프로젝트마다 활성 데이터셋 ID를 가리킨다. `Activate`는 기존 버전 사이를 원자적으로 전환하므로 이전 버전으로 다시 지정하면 롤백된다. 읽을 때 문서 해시와 구조를 다시 확인한다. 현재 DB 스키마는 `PRAGMA user_version=2`이며 v1 DB는 기존 투영 JSON을 수정하지 않고 v2로 마이그레이션한다. 더 높은 버전은 구버전 바이너리에서 거부한다.

CKG·CKV와 함께 쓰는 승격 경로는 `PutAligned`와 `ActivateAligned`다. 두 호출 모두 그래프·벡터 매니페스트의 소스 커밋, 기록된 CKG 다이제스트 핀, 소스 루트와 투영의 커밋을 비교한다. 값이 빠진 구버전 인덱스는 의미 투영 승격에서 거부한다. 활성화 시점에도 다시 검사하므로 오래 전에 저장한 후보를 최신 인덱스에 잘못 연결하지 않는다. `Put`/`Activate`는 엔진이 없는 의미 투영 준비·롤백용 저수준 호출이다. 현재 원본 파일이 커밋과 동일한지는 `ValidateSources`가 검사하지만, 같은 HEAD의 서로 다른 작업 트리 바이트를 구별하는 다이제스트는 아직 구현되지 않았다.

코드 앵커 조인을 위한 그래프 스키마는 `canonical_id`가 있는 1.19 이상이어야 한다. 이 게이트는 매니페스트 좌표의 일치를 검사하며 인덱스 파일 자체의 암호학적 무결성 검증이나 휴대 가능한 프로젝트 ID는 후속 작업이다.

작은 데이터셋의 실제 입력 경로는 다음과 같다. `--docs`는 저장소 상대 경로이며 여러 번 지정할 수 있다. 명령은 Git HEAD의 파일 내용을 읽고 섹션만 만든다. 주장과 관계를 자동 생성하지 않는다.

```sh
cks semantic build --repo /path/to/repo --project-id sample --dataset-id cut-001 \
  --graph /path/to/dataset/graph --vector /path/to/dataset/vector \
  --store /path/to/semantic.db --out /path/to/projection.json \
  --docs README.md --docs docs/spec.md \
  --ontology docs/spec-driven/ontology-pilot.yaml \
  --min-canonical-ratio 0.94 --activate
cks semantic review --input /path/to/projection.json --repo /path/to/repo --sample 20
```

`--activate`를 생략하면 검증된 후보 데이터셋만 저장한다. JSON은 원문 범위와 해시를 사람이 검토할 수 있도록 출력한다. 실제 CKV·CKG 빌드 후 `scripts/wbs-smoke.sh`에서 위 빌드와 검토 명령을 실행한다.

검토자가 코드 근거와 관계를 추가하는 흐름에서는 새 `dataset_id`로 `build --extract-only`를 실행하고 JSON의 `evidence`/`assertions`를 작성한 뒤 `semantic promote --input <json> --repo ... --graph ... --vector ... --store ... --activate`를 실행한다. 추출 전용 단계는 SQLite에 저장하지 않는다. 승격은 원문·관계 타입·CKG 코드 앵커·CKV 정렬률을 다시 검사한다. 같은 데이터셋 ID의 다른 내용은 불변 저장소가 거부하므로 검토 수정마다 새 데이터셋 ID를 사용한다. 작은 스모크는 실제 `Alpha` AST 심볼을 `IMPLEMENTED_BY`로 연결해 승격한 뒤 존재하지 않는 `canonical_id`의 승격이 실패하고 이전 활성 버전이 유지됨을 확인한다.

`--min-canonical-ratio`는 프로젝트 실측 기준으로 선택한다. 0이면 게이트를 끄며, 양수이면 CKV 매니페스트의 `canonical_count / symbol_count`가 기준보다 낮거나 카운터가 없을 때 승격을 막는다. 예시의 0.94는 이 프로젝트의 구조 기준선(약 94.28%)에 맞춘 값이지 다른 저장소의 기본값이 아니다. 실제 임베딩 검색 품질과는 별도 지표다.

CKS Stage 2에는 **선택형 Go API**인 `WithOntologyResolver`가 있다. 호출자는 `Store.CurrentAligned`로 활성 투영을 다시 검증해 전달할 수 있다. 질의의 선호·대체 용어를 복수 개념 후보로 해석하고, `verified` 개념과 `verified IMPLEMENTED_BY` 관계가 CKV 결과의 canonical ID·커밋·파일·줄과 일치할 때만 기존 상위 K 인용의 점수를 최대 20% 올린다. 동점 의미를 합치거나 결과를 새로 추가하지 않는다. 오류가 나면 기존 검색 결과를 유지한다. 현재 CKS CLI/MCP에서는 이 옵션을 전달하지 않아 기본 검색은 기존 CKV+CKG 경로 그대로다. 실제 임베딩 모델의 A/B 품질·지연 평가가 남아 있으므로 운영 기본 활성화는 보류한다.

정확한 용어의 중복 의미를 검토할 때는 `cks semantic lookup-term --project-id <id> --repo <repo> --graph <graph-dir> --vector <vector-dir> --store <semantic.db> --lang ko --term <용어>`를 사용한다. 활성 CKG·CKV·원문 스냅샷을 재검증한 뒤 SQLite의 프로젝트·데이터셋·언어·정규화 용어 색인에서 후보를 찾는다. 대소문자를 접어 비교하며 같은 용어의 복수 개념을 모두 반환한다. 거부된 개념은 후보에서 제외하고 제안 개념은 낮은 점수로 표시한다. 이 명령은 질의 문장을 재작성하지 않으며, 색인 행이 빠졌거나 저장 문서와 맞지 않으면 실패한다. SQLite 스키마 v3으로 옮길 때 과거 v1/v2 JSON 문서는 다시 쓰지 않는다.

검토된 의미 텍스트는 `cks semantic export-text --project-id <id> --repo <repo> --graph <graph-dir> --vector <vector-dir> --store <semantic.db> --out <new-corpus-dir>`로 별도 디렉터리에 출력할 수 있다. 출력은 `verified` 개념·요구사항만 포함하고 ID 순서에 따라 결정적인 Markdown 파일명·내용을 생성한다. `manifest.json`은 원본 저장소, 커밋, 의미 투영 SHA-256과 모든 Markdown 바이트의 SHA-256을 기록한다. 제안·거부 상태의 항목은 제외된다. `cks setup --src <repo> --out <dataset> --version <new-version> --semantic-corpus <new-corpus-dir> ...`는 그래프 빌드 후 코퍼스의 저장소·커밋·파일 목록·해시를 검증한 다음 CKV의 추가 문서 루트로 전달한다. 바이트 변경, 다른 프로젝트, 추가 Markdown은 벡터 빌드 전 실패하며 활성 데이터셋은 유지된다. 현재 스모크는 mock 임베더의 구조 검색만 검증한다. 자연어 품질이 입증될 때까지 온톨로지 재순위 기본값은 그대로 꺼져 있다.

## 요구사항과 수용 기준 입력

`cks semantic build --spec docs/spec-driven/spec-pilot.yaml`은 같은 프로젝트의 커밋된 YAML에서 `Requirement`와 Given/When/Then `AcceptanceCriterion`을 읽는다. 각 항목은 안정 ID, 요구사항 버전, 원본 파일·줄 범위·SHA-256 근거를 가진다. `proposed`는 초안, `verified`는 사람이 **명세 내용을 승인**했다는 뜻이며 구현 또는 테스트 통과를 뜻하지 않는다. `verified`/`rejected`에는 YAML 원본의 `reviewed_by`가 필요하다. JSON 투영의 문구만 바꾸면 원본 재파싱에서 거부된다. `concept_ids`는 같은 투영의 개념을 참조하며 선택 사항이므로 코드가 전혀 없는 초기 명세도 저장할 수 있다. `cks semantic review`는 제안 요구사항의 출처와 수용 기준을 검토 대기 목록으로 출력한다. 구현·테스트 상태는 이후 검증된 연결 근거로 별도 계산할 예정이다.

`TESTED_BY`는 CKG 코드 심볼에서 CKG 테스트 심볼로 향하며 두 코드 근거를 요구한다. `ACCEPTED_BY`는 수용 기준에서 CKG 테스트 심볼로 향하며 명세 근거와 테스트 근거를 요구한다. `verified ACCEPTED_BY`는 해당 요구사항이 원본 YAML에서 승인된 경우에만 허용한다. 두 관계 모두 검토자가 필요하고, 실제 CKG 노드·커밋·파일·AST 범위를 승격 단계에서 검사한다.

`cks semantic trace --project-id <id> --repo <repo> --graph <graph-dir> --vector <vector-dir> --store <semantic.db>`는 활성 데이터셋을 다시 대조한 뒤 요구사항별 상태와 근거 경로를 JSON으로 출력한다. `linked`는 검토된 Requirement→Concept→Code→Test 및 AcceptanceCriterion→Test **연결의 존재**만 뜻한다. 테스트가 실제 실행되어 통과했다는 뜻은 아니다. 누락된 개념·구현·테스트·수용 테스트와 검토된 상충 주장 관계를 별도로 표시한다. 인덱스가 오래되거나 원문과 어긋나면 추적 경로를 출력하지 않고 실패한다. 실제 테스트 결과 수집 및 실패 패치 미승격은 후속 W4.4 작업이다.

기계 판독형 오류 진단이 필요하면 `trace --diagnose-stale`을 사용한다. 활성 의미 투영은 저장되어 있지만 소스·CKG·CKV 근거가 어긋나면 `state=stale`, 스냅샷, 이유 및 빈 `requirements`를 JSON으로 출력한 뒤 비정상 종료한다. 이전 검토 경로를 최신 경로처럼 재출력하지 않는다. 정렬이 유효하면 `state=current`와 일반 추적 결과를 출력한다. 활성 투영 자체가 없거나 손상된 경우는 별도의 읽기 오류로 처리한다.

`cks semantic plan`은 동일한 정렬 검증 뒤 각 요구사항에 필요한 다음 조치를 읽기 전용 JSON으로 제시한다. 근거 ID, 이미 알려진 코드·테스트 심볼 ID, 누락 관계, 충돌 관계 ID를 유지하며 모든 단계에 `unconfirmed=true`를 단다. 연결이 완성되어도 계획은 수용 테스트 실행을 요구한다. 이 계획은 코드 변경이나 승인 결정을 생성하지 않는다.

`cks semantic annotate-pack --project-id <id> --repo <repo> --graph <graph-dir> --vector <vector-dir> --store <semantic.db> --input <base-pack.json> --out <new-pack.json>`은 기존 EvidencePack의 무결성을 확인한 뒤, **이미 포함된 인용의 커밋·파일·줄 범위**와 겹치는 검토된 추적 경로 ID만 `semantic` 선택 필드에 추가한다. 기존 인용·본문을 늘리거나 원문을 복사하지 않는다. 의미 필드는 별도 SHA-256 다이제스트로 검증하고, 기본 팩 해시는 기존 소비자가 새 필드를 무시한 채 검증할 수 있도록 동일하게 유지한다. 이로 인해 기본 팩 해시는 의미 필드를 보호하지 않으므로 새 소비자는 `semantic.digest`도 반드시 검사해야 한다. 모든 링크는 `unconfirmed=true`이며 테스트 통과를 뜻하지 않는다. 동일 커밋의 일치하는 인용이 없으면 의미 필드는 생략된다.

`cks semantic test --project-id <id> --repo <repo> --graph <graph-dir> --vector <vector-dir> --store <semantic.db> --criterion-id <id> --out <new-report.json> -- go test ./...`는 **검토된 연결이 있는 수용 기준**에서만 명령을 실행한다. 해당 기준에 검토된 테스트 심볼이 여러 개라면 `--test-canonical-id <CKG ID>`로 하나를 선택해야 한다. 보고서에는 선택한 테스트 ID 및 이를 뒷받침하는 `TESTED_BY`/`ACCEPTED_BY` 단언 ID를 기록한다. 실행 전 소스 HEAD가 투영 커밋과 같고 추적·미추적·인덱싱 가능한 Git 무시 파일의 변경이 없어야 한다. 실행 후에도 같은 검사를 수행한다. 보고서는 명령 인자와 출력의 SHA-256, 출력 길이, 종료 코드, OS/아키텍처/Go 런타임, 소요 시간 및 스냅샷 일치 여부를 기록하되 원문 출력은 저장하지 않고 기존 파일을 덮어쓰지 않는다. 실패 명령은 비정상 종료와 함께 실패 보고서를 남긴다. 테스트 심볼의 선택은 임의 명령이 그 심볼을 실제 실행했다는 증명이 아니다. 명령이 성공해도 해당 Given/When/Then을 검증했는지 자동 판정할 수 없으므로 요구사항이나 관계의 상태를 승격하지 않는다. 외부 패치 식별·실패 패치 미승격·재색인 연결은 후속 구현이다.

Go 테스트의 실제 실행을 확인하려면 임의 명령 대신 `--go-test-exact`를 지정한다. 이 모드는 검토된 CKG 테스트 근거가 가리키는 커밋의 `*_test.go`를 Go AST로 읽고, `canonical_id`·줄 범위 안의 `TestXxx` 함수가 정확히 일치하는지 확인한다. 그 파일의 패키지에 대해 `go test -json -count=1 -run '^TestXxx$'`를 실행하고, JSON 스트림에서 해당 테스트의 `run`과 `pass` 이벤트가 모두 있어야 성공한다. 캐시된 테스트 결과, 잘못된 함수 범위 또는 `no tests to run`은 성공 증거가 아니다. 보고서의 `framework`, `test_name`, `test_observed`, `test_passed`가 이 판정을 설명한다. 원문 테스트 출력은 여전히 해시만 저장한다. 이 검사는 **그 테스트 함수의 실행·통과**만 증명하며, 수용 기준의 의미나 패치 전체의 정확성을 자동 승인하지 않는다.

재색인 후보에 대해서는 `cks setup --version <name> --gate-test-bin go --gate-test-arg test --gate-test-arg ./...`를 사용할 수 있다. CKG/CKV 정렬·커버리지 게이트 다음, 활성 포인터 교체 전에 소스 HEAD·청결성을 확인하고 명령을 실행한다. 종료 코드가 0이 아니거나 실행 뒤 스냅샷이 달라지면 후보는 남기되 활성 포인터는 보존한다. 후보 디렉터리의 `test-gate.json`은 후보 커밋과 그래프 다이제스트, 실행·출력 해시 및 환경을 기록한다. 성공 결과도 특정 수용 기준의 행동을 증명하지 않으며 새 커밋의 의미 투영은 별도로 재생성·검토해야 한다.
