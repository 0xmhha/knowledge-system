# CKS 의미 투영 계약 v1

이 문서는 W2.1–W2.2의 현재 구현 계약을 기록한다. 전체 온톨로지나 자동 사실 판정이 완성됐다는 뜻은 아니다. 상위 설계와 수용 조건은 `study/docs/reviews/knowledge-system/spec-driven-architecture.md` 및 `spec-driven-requirements.md`에 있다.

## 원본과 신원

하나의 `Projection`은 `project_id`, `dataset_id`, 40자리 Git 커밋과 스키마 버전 1에 고정된다. 의미 데이터는 CKG/CKV 원본을 대체하지 않는다. `EvidenceSpan`은 문서·코드·테스트 종류, 저장소 상대 경로, 1부터 시작하는 포함 줄 범위, 그 범위의 원문 SHA-256, 추출기 이름을 가진다. `canonical_id`와 `chunk_id`는 선택적인 조인 키이며 원문 증명을 대신하지 않는다.

`Validate`는 ID 중복, 고아 참조, 경로 탈출, 잘못된 상태, 프로젝트/데이터셋/커밋 혼합을 거부한다. `ValidateSources`는 **기록된 커밋의 Git 객체**를 읽어 줄 범위와 원문 해시를 다시 계산한다. 현재 작업 트리의 수정 사항으로 과거 근거를 덮어쓰지 않는다. 현재 구현의 근거 검증 대상은 같은 Git 저장소에 커밋된 파일이다. 외부 문서와 작업 트리 스냅샷은 별도 계약이 필요하다.

## 사실의 상태

`DocumentSection`은 CKV와 동일한 Markdown 제목 파서의 줄 경계를 사용한다. 섹션 추출은 `Claim`을 만들어 사실이라고 주장하지 않는다. `Claim`은 최소한 원천 섹션과 그 섹션의 `EvidenceSpan`을 참조해야 한다. 추출·제안된 주장은 `proposed`로 시작한다. `verified`에는 명시적 `reviewed_by`가 필요하며 승격 전 `ValidateSources`를 통과해야 한다. 검토자가 있더라도 스냅샷이 다르거나 원문 해시가 맞지 않으면 저장할 수 없다. `rejected`는 삭제 대신 검토 이력에 남길 상태다.

`cks semantic review --input projection.json --repo <git-root> --sample 20 --salt <audit-id>`는 JSON 투영의 모든 근거를 기록된 커밋에서 재검증한 뒤 검토 대기 주장 샘플과 집계 JSON을 출력한다. 표본은 데이터셋 ID·주장 ID·salt의 해시로 결정되므로 같은 입력은 같은 목록을 만든다. `verified`와 `rejected` 모두 검토자 ID가 있어야 한다. 정밀도는 **검토된 주장 중 verified 비율**이며 검토 건수가 0이면 `null`이다. 이 수치는 실제로 검토된 표본의 품질만 설명한다. 대표 표본의 외부 검토가 없으면 추출기 전체의 정밀도나 관계 정확도로 해석하지 않는다. 현재 자동 관계 추출과 관계 평가셋은 없다.

코드 근거를 붙일 때는 CKG 공개 읽기 API에서 `canonical_id`를 정확 조회하고, CKG의 소스 커밋·파일·AST 노드 줄 범위가 근거와 일치하는지 검사한다. 이 검사는 원문 SHA-256 검사와 별개이며 `verified`로 자동 승격하지 않는다.

## SQLite 투영과 롤백

`semantic_projections`는 `(project_id,dataset_id)`별 JSON 문서와 SHA-256을 불변으로 저장한다. 같은 내용의 재시도는 허용하고 같은 ID의 다른 내용은 거부한다. `semantic_current`는 프로젝트마다 활성 데이터셋 ID를 가리킨다. `Activate`는 기존 버전 사이를 원자적으로 전환하므로 이전 버전으로 다시 지정하면 롤백된다. 읽을 때 문서 해시와 구조를 다시 확인한다. 현재 DB 스키마는 `PRAGMA user_version=1`이며 더 높은 버전은 구버전 바이너리에서 거부한다.

이 저장소는 아직 CKS Composer의 질의 경로에 연결되지 않았다. 의미 관계의 타입 제약, CKV 텍스트 투영, 근거 충돌, 프로젝트별 A/B 품질 측정이 통과한 후 기능 플래그로 연결한다. 기본 검색은 기존 CKV+CKG 경로를 유지한다.
