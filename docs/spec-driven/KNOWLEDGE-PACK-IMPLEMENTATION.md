# 로컬 지식 팩 구현 체크포인트

상태: **A5.4 부분 구현** (2026-10-01). 설계 기준은 [D5 도메인 팩 계약](./DOMAIN-PACK-CONTRACT-V1.md)이다. 공통 20개 타입은 [D1 어휘](./ONTOLOGY-20-USAGE.md)에 그대로 둔다. 이 문서는 현재 실행되는 부분과 남은 계약을 구분한다.

## 현재 명령

```sh
cks knowledge init --project-root /path/to/project --project-id my-project
cks knowledge digest --project-root /path/to/project --source vendor/my-pack
cks knowledge validate --project-root /path/to/project
cks knowledge lock --project-root /path/to/project
cks knowledge review --project-root /path/to/project
cks setup --src /path/to/project --out /path/to/data \
  --project-id my-project --source-mode snapshot-only --version v1 --embedder mock
```

`init`은 `.cks/knowledge/manifest.yaml`과 빈 `domain/`, `policies/`, `decisions/`, `questions/` 디렉터리를 만들며 기존 매니페스트를 덮어쓰지 않는다. `digest`는 팩을 매니페스트에 등록하기 전에 로컬 파일 다이제스트를 계산한다. `validate`는 선택 팩의 스키마·원천 파일·정확 버전/해시·의존 DAG·타입 참조를 확인한다. 잠금 파일이 없으면 `unlocked`, 저장된 잠금이 맞으면 `locked`를 보고한다. `lock`은 원문 바이트를 해시한 `knowledge.lock.json`을 원자 교체한다. `review`는 정책과 ADR의 상태·원천·검토자·보류 사유를 읽기 전용으로 보여준다. 프로젝트에 매니페스트가 있으면 `cks setup`은 잠금을 검증하고, 오버레이와 선택 팩 파일을 데이터셋 입력 다이제스트에 포함한다. 빌드 중 입력이 바뀌면 승격하지 않는다.

선택 팩은 현재 프로젝트 루트 아래의 상대 경로만 받는다. `pack.yaml`의 `pack_schema_version: 1`, `pack_id`, `version`, `owner`, `scope`, `requires`, `concepts`, `relation_types`, `constraints`, `competency_questions`를 엄격하게 파싱한다. 참조 키는 `(pack_id, local_id)` 튜플이다. 서로 다른 팩에 같은 로컬 ID가 있어도 자동 병합하지 않는다. `core` 네임스페이스의 타입 참조는 고정된 20개에서만 찾는다. 누락·해시 불일치·중복·순환 의존, 잘못된 관계 끝점과 상속 순환은 오류다. 파일 인벤토리는 링크/비정규 파일/대소문자 충돌/숨김 파일을 거부하고 macOS·Linux에서는 경로 성분을 no-follow로 연다.

## 검증과 남은 범위

`scripts/wbs-knowledge-lock-smoke.sh`는 비 Git 후보를 만들고, 오버레이 파일을 바꾼 뒤 오래된 잠금으로는 후보가 만들어지지 않음을 확인한다. 다시 잠근 뒤 만든 후보는 다른 `snapshot_id`와 `dataset_id`를 가지며 `current`가 새 버전으로 이동한다. 이어 선택 팩, 상충하는 검토된 정책 2개와 미검토 ADR 1개를 추가해 충돌 수와 보류 사유를 확인한 뒤 세 번째 후보를 승격한다. 단위 테스트는 의존 순서·누락·순환, 같은 로컬 ID의 네임스페이스 격리, 공통 타입 참조, 오버레이 바이트 변경과 링크 거부를 확인한다.

정책 YAML과 ADR Markdown의 필드·타입·원천 경로·검토자/이유·유효 날짜를 검증한다. 같은 범위·시점에 서로 `conflicts_with`로 지정한 검토된 정책은 충돌로 보고한다. 내부 `SelectPolicies`는 범위·시점·제한 여부를 계산하지만 공개 근거 인용이 없으면 상태를 `needs_citation`으로 남긴다. 이 기록의 `verified`는 YAML에 적힌 로컬 검토 이력이며 외부 신원 인증을 뜻하지 않는다.

아직 **외부 프로젝트 경로의 `knowledge:<pack_id>` 보관본, v4 의미 투영/읽기 어댑터, 팩 관계 인스턴스, 검토 명령의 판정 쓰기, 권한 어댑터와 코드 앵커, 선택형 v2 문맥과 플랫폼 릴리스**는 구현되지 않았다. 팩을 설치하거나 잠근 사실은 정책의 `verified` 판정이 아니다. A5.4/A5.5/A6/A7은 이 남은 계약을 통과하기 전 완료로 표시하지 않는다.
