# 로컬 지식 팩 구현 체크포인트

상태: **A5.4/A5.5/A6 부분 구현** (2026-10-01). 설계 기준은 [D5 도메인 팩 계약](./DOMAIN-PACK-CONTRACT-V1.md)이다. 공통 20개 타입은 [D1 어휘](./ONTOLOGY-20-USAGE.md)에 그대로 둔다. 이 문서는 현재 실행되는 부분과 남은 계약을 구분한다.

## 현재 명령

```sh
cks knowledge init --project-root /path/to/project --project-id my-project
cks knowledge digest --project-root /path/to/project --source vendor/my-pack
cks knowledge validate --project-root /path/to/project
cks knowledge lock --project-root /path/to/project
cks knowledge review --project-root /path/to/project
cks knowledge review --version-dir /path/to/data/v1
cks semantic build --repo /path/to/project --project-id my-project \
  --dataset-id DATASET_ID --graph /path/to/data/v1/graph --vector /path/to/data/v1/vector \
  --version-dir /path/to/data/v1 --include-packs --extract-only \
  --store /path/to/semantic.db --out /path/to/pack-v4.json
cks setup --src /path/to/project --out /path/to/data \
  --project-id my-project --source-mode snapshot-only --version v1 --embedder mock
```

`init`은 `.cks/knowledge/manifest.yaml`과 빈 `domain/`, `policies/`, `decisions/`, `relations/`, `questions/` 디렉터리를 만들며 기존 매니페스트를 덮어쓰지 않는다. `digest`는 팩을 매니페스트에 등록하기 전에 로컬 파일 다이제스트를 계산한다. `validate`는 선택 팩의 스키마·원천 파일·정확 버전/해시·의존 DAG·타입 참조를 확인한다. 잠금 파일이 없으면 `unlocked`, 저장된 잠금이 맞으면 `locked`를 보고한다. `lock`은 원문 바이트를 해시한 `knowledge.lock.json`을 원자 교체한다. `review`는 정책·ADR·관계의 상태·원천·검토자·보류 사유를 읽기 전용으로 보여준다. 프로젝트에 매니페스트가 있으면 `cks setup`은 잠금을 검증하고, 오버레이와 선택 팩 파일을 데이터셋 입력 다이제스트에 포함한다. 빌드 중 입력이 바뀌면 승격하지 않는다.

새 매니페스트는 `review_policy.min_approvals: 1`을 명시한다. 생략한 기존 매니페스트도 1로 읽고, 명시한 0이나 음수는 거부한다. 현재 정책/ADR의 인라인 `reviewed_by`는 한 명만 기록할 수 있다. `min_approvals`를 2 이상으로 설정했을 때 이 한 명을 승인 정족수로 오판하지 않도록 검토된 인스턴스의 잠금·빌드를 거부한다. 복수 검토 기록과 원문 해시·스냅샷 결합 입력은 아직 구현 전이다.

선택 팩은 현재 프로젝트 루트 아래의 상대 경로만 받는다. `pack.yaml`의 `pack_schema_version: 1`, `pack_id`, `version`, `owner`, `scope`, `requires`, `concepts`, `relation_types`, `constraints`, `competency_questions`를 엄격하게 파싱한다. 참조 키는 `(pack_id, local_id)` 튜플이다. 서로 다른 팩에 같은 로컬 ID가 있어도 자동 병합하지 않는다. `core` 네임스페이스의 타입 참조는 고정된 20개에서만 찾는다. 누락·해시 불일치·중복·순환 의존, 잘못된 관계 끝점과 상속 순환은 오류다. 파일 인벤토리는 링크/비정규 파일/대소문자 충돌/숨김 파일을 거부하고 macOS·Linux에서는 경로 성분을 no-follow로 연다.

## 검증과 남은 범위

`scripts/wbs-knowledge-lock-smoke.sh`는 비 Git 후보를 만들고, 오버레이 파일을 바꾼 뒤 오래된 잠금으로는 후보가 만들어지지 않음을 확인한다. 다시 잠근 뒤 만든 후보는 다른 `snapshot_id`와 `dataset_id`를 가지며 `current`가 새 버전으로 이동한다. 이어 선택 팩, 상충하는 검토된 정책 2개와 미검토 ADR 1개를 추가해 충돌 수와 보류 사유를 확인한 뒤 세 번째 후보를 승격한다. 단위 테스트는 의존 순서·누락·순환, 같은 로컬 ID의 네임스페이스 격리, 공통 타입 참조, 오버레이 바이트 변경과 링크 거부를 확인한다.

정책 YAML과 ADR Markdown의 필드·타입·원천 경로·검토자/이유·유효 날짜를 검증한다. 같은 범위·시점에 서로 `conflicts_with`로 지정한 검토된 정책은 충돌로 보고한다. 내부 `SelectPolicies`는 범위·시점·제한 여부를 계산하지만 공개 근거 인용이 없으면 상태를 `needs_citation`으로 남긴다. 이 기록의 `verified`는 YAML에 적힌 로컬 검토 이력이며 외부 신원 인증을 뜻하지 않는다.

`relations/*.yaml`은 팩의 `relation_types` 술어를 `(pack_id, local_id)`로 참조하고, 주체·객체 ID와 타입, `status`, 검토자·이유, 공개 범위, 자기 원천과 `evidence_refs`를 기록한다. 로더는 술어 방향·끝점 타입·중복 ID를 확인한다. `verified` 관계는 양쪽 끝점이 이 오버레이의 검토된 정책/ADR이고 두 원천이 근거 목록에 들어있을 때만 받는다. 요구사항·CKG 코드 심볼·테스트 등 외부 끝점의 관계는 앵커 검증이 연결되기 전까지 `proposed`만 허용한다. 등록된 관계 원문은 잠금과 보관 후보에 포함된다. v2 문맥은 충돌 없는 현재 범위의 공개 검토 관계만 별도 정화된 원문 인용과 함께 `relations`에 싣고, 아직 `related_requirements`나 `test_links`로 출력하지 않는다.

선택 팩 원문은 `knowledge:<pack_id>` 출처로 후보의 content-addressed archive에 보관한다. 저장소와 팩이 같은 상대 경로를 써도 다른 원천이다. `review --version-dir`는 보관본만으로 팩과 정책을 재구성하고 잠금을 검증한다. 라이브 파일 편집 뒤에도 과거 결과가 같고 blob 손상은 거부한다. `cks.context.get_for_task_v2`는 `include_knowledge=true`와 명시 날짜·서브시스템을 받으면 검토된 공개 정책과 ADR만 선택한다. ADR은 선택 날짜 이전, 서브시스템 범위 일치, 대체 관계와 공개 권한 검사를 통과해야 한다. 정책은 v2 인용·정화된 본문에 연결되고 의미 객체까지 `sha256-v2` 해시에 포함된다. 비공개 정책 또는 ADR은 ID와 본문을 숨기고 `restricted` 상태만 남긴다. 지식 레이어 오류와 예산 초과 시 기본 코드 근거를 유지하고 불확실 상태를 출력한다.

v4 팩 투영은 보관된 `pack.yaml`에서 `(pack_id, local_id)` 타입·관계·제약을 추출한다. 잠금 다이제스트, 팩 원문 해시, 그래프/벡터 좌표를 대조하며 구 v1–v3 JSON은 수정하지 않고 읽는다. 위 명령은 `--include-packs`를 명시한 보관 후보에만 v4를 쓴다. 원천 정의를 위조한 v4 후보는 승격되지 않는다. `wbs-knowledge-lock-smoke.sh`가 실제 후보의 v4 추출·승격·위조 거부를 검증한다.

아직 **외부 끝점의 관계 승격, 검토 명령의 판정 쓰기, 조직별 권한 어댑터와 코드 앵커, ADR→요구→테스트의 검증된 의미 경로, 운영 릴리스**는 구현되지 않았다. 현재 선택형 v2 문맥은 정책·ADR 원문 인용까지 다룬다. ADR에 선언된 요구사항 ID는 관계 검증 증거가 아니라 원문 메타데이터다. 따라서 v2의 `decisions[].requirement_ids`와 `related_requirements`에는 이를 승격하지 않고 `unverified_adr_requirement_links`를 `unknowns`에 기록하며 상태를 `partial`로 낮춘다. 팩을 설치하거나 잠근 사실은 정책의 `verified` 판정이 아니다. A5.4/A5.5/A6/A7은 이 남은 계약을 통과하기 전 완료로 표시하지 않는다.
