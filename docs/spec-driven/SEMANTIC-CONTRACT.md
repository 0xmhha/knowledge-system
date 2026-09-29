# CKS 의미 투영 계약 v1

이 문서는 W2.1–W2.2의 현재 구현 계약을 기록한다. 전체 온톨로지나 자동 사실 판정이 완성됐다는 뜻은 아니다. 상위 설계와 수용 조건은 `study/docs/reviews/knowledge-system/spec-driven-architecture.md` 및 `spec-driven-requirements.md`에 있다.

## 원본과 신원

하나의 `Projection`은 `project_id`, `dataset_id`, 40자리 Git 커밋과 스키마 버전 1에 고정된다. 의미 데이터는 CKG/CKV 원본을 대체하지 않는다. `EvidenceSpan`은 문서·코드·테스트 종류, 저장소 상대 경로, 1부터 시작하는 포함 줄 범위, 그 범위의 원문 SHA-256, 추출기 이름을 가진다. `canonical_id`와 `chunk_id`는 선택적인 조인 키이며 원문 증명을 대신하지 않는다.

`Validate`는 ID 중복, 고아 참조, 경로 탈출, 잘못된 상태, 프로젝트/데이터셋/커밋 혼합을 거부한다. `ValidateSources`는 **기록된 커밋의 Git 객체**를 읽어 줄 범위와 원문 해시를 다시 계산한다. 현재 작업 트리의 수정 사항으로 과거 근거를 덮어쓰지 않는다. 현재 구현의 근거 검증 대상은 같은 Git 저장소에 커밋된 파일이다. 외부 문서와 작업 트리 스냅샷은 별도 계약이 필요하다.

## 사실의 상태

`DocumentSection`은 CKV와 동일한 Markdown 제목 파서의 줄 경계를 사용한다. 섹션 추출은 `Claim`을 만들어 사실이라고 주장하지 않는다. `Claim`은 최소한 원천 섹션과 그 섹션의 `EvidenceSpan`을 참조해야 한다. 추출·제안된 주장은 `proposed`로 시작한다. `verified`에는 명시적 `reviewed_by`가 필요하며 승격 전 `ValidateSources`를 통과해야 한다. 검토자가 있더라도 스냅샷이 다르거나 원문 해시가 맞지 않으면 저장할 수 없다. `rejected`는 삭제 대신 검토 이력에 남길 상태다.

`cks semantic review --input projection.json --repo <git-root> --sample 20 --salt <audit-id>`는 JSON 투영의 모든 근거를 기록된 커밋에서 재검증한 뒤 검토 대기 주장 샘플과 집계 JSON을 출력한다. 표본은 데이터셋 ID·주장 ID·salt의 해시로 결정되므로 같은 입력은 같은 목록을 만든다. `verified`와 `rejected` 모두 검토자 ID가 있어야 한다. 정밀도는 **검토된 주장 중 verified 비율**이며 검토 건수가 0이면 `null`이다. 이 수치는 실제로 검토된 표본의 품질만 설명한다. 대표 표본의 외부 검토가 없으면 추출기 전체의 정밀도나 관계 정확도로 해석하지 않는다. 현재 자동 관계 추출과 관계 평가셋은 없다.

관계 후보 `Assertion`은 자체 근거·상태·검토자를 가진다. 현재 허용한 좁은 타입은 `DocumentSection SUPPORTS Claim`과 `Claim CONTRADICTS Claim`뿐이다. 관계 근거가 실제 섹션·양쪽 주장에 닿아야 하며, `verified` 관계는 연결된 주장도 검토 완료 상태여야 한다. 방향 오류, 고아 참조, 무근거, 무검토 승격은 거부한다. 검토 도구는 주장과 관계의 정밀도를 따로 출력한다. `RELATED_TO` 같은 막연한 관계나 코드 구현 관계는 아직 확정 타입으로 사용하지 않는다.

코드 근거를 붙일 때는 CKG 공개 읽기 API에서 `canonical_id`를 정확 조회하고, CKG의 소스 커밋·파일·AST 노드 줄 범위가 근거와 일치하는지 검사한다. 이 검사는 원문 SHA-256 검사와 별개이며 `verified`로 자동 승격하지 않는다.

## 온톨로지 시범 팩

[`ontology-pilot.yaml`](./ontology-pilot.yaml)은 knowledge-system 자체를 대상으로 한 **제안 상태**의 18개 개념과 6개 능력 질문이다. 각 개념은 안정 ID, `entity/artifact/process/rule` 중 한 종류, 문장형 정의, 포함·제외 범위, 언어별 선호·대체 용어, 검토 상태를 가진다. `ExtractOntology`는 YAML의 각 개념 블록을 정확한 Git 원문 줄·SHA-256 근거로 만든다. `ValidateSources`는 원문 해시뿐 아니라 정의·용어·상태를 YAML에서 다시 읽어 투영과 비교하므로 JSON 투영만 수정해 정의를 바꿀 수 없다. `verified`/`rejected`에는 검토자 이름과 원본 YAML 변경이 필요하다.

`cks semantic build`의 `--ontology`에 저장소 상대 YAML 경로를 주면 Markdown 섹션과 함께 동일한 프로젝트/데이터셋/커밋 투영에 저장한다. `cks semantic review`는 개념 표본·검토된 정밀도·여러 개념이 공유하는 동일 언어 용어를 별도 항목으로 출력한다. 동음이의어는 자동으로 같은 개념이라고 병합하지 않는다. 현재 시범 팩에는 전문가 리뷰가 없으므로 18개 모두 `proposed`다.

## SQLite 투영과 롤백

`semantic_projections`는 `(project_id,dataset_id)`별 JSON 문서와 SHA-256을 불변으로 저장한다. 같은 내용의 재시도는 허용하고 같은 ID의 다른 내용은 거부한다. `semantic_current`는 프로젝트마다 활성 데이터셋 ID를 가리킨다. `Activate`는 기존 버전 사이를 원자적으로 전환하므로 이전 버전으로 다시 지정하면 롤백된다. 읽을 때 문서 해시와 구조를 다시 확인한다. 현재 DB 스키마는 `PRAGMA user_version=1`이며 더 높은 버전은 구버전 바이너리에서 거부한다.

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

`--min-canonical-ratio`는 프로젝트 실측 기준으로 선택한다. 0이면 게이트를 끄며, 양수이면 CKV 매니페스트의 `canonical_count / symbol_count`가 기준보다 낮거나 카운터가 없을 때 승격을 막는다. 예시의 0.94는 이 프로젝트의 구조 기준선(약 94.28%)에 맞춘 값이지 다른 저장소의 기본값이 아니다. 실제 임베딩 검색 품질과는 별도 지표다.

이 저장소는 아직 CKS Composer의 질의 경로에 연결되지 않았다. 의미 관계의 타입 제약, CKV 텍스트 투영, 근거 충돌, 프로젝트별 A/B 품질 측정이 통과한 후 기능 플래그로 연결한다. 기본 검색은 기존 CKV+CKG 경로를 유지한다.
