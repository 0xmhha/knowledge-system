# 선택형 검증 worksheet catalog 계약 v1

N-13 · 2026-10-06. `cks domain --project PROJECT_DIR worksheet`는 공통 검토 질문과 원문 앵커를 출력한다. 조직의 위험 목록은 명시적으로 선택한 로컬 pack 자료에만 들어 있다. 생성은 앵커 검사를 실행하거나 항목을 승인·verified로 승격하지 않는다. 검토자는 먼저 `cks domain --project PROJECT_DIR check` 결과와 현행 원문을 확인한다.

## 선택과 호환

`PROJECT_DIR/project.yaml`에 선택한다. 생략하거나 빈 배열이면 공통 경로다. project ID나 조직 이름으로 목록을 자동 선택하지 않는다.

```yaml
worksheet_packs:
  - pack_id: organization.example
    version: 1.0.0
    source: packs/example
    sha256: EXACT_PACK_TREE_SHA256
```

source는 PROJECT_DIR 내부의 실제 디렉터리이고 symlink·상위 경로를 허용하지 않는다. ID/정확 version/digest는 [기존 domain pack](./DOMAIN-PACK-CONTRACT-V1.md)의 `pack.yaml`, 모든 자료를 포함한 `cks.knowledge-files.v1` tree digest와 일치해야 한다. 필요한 의존 pack도 같은 배열에 명시한다. 선택 최대32개다. 기존 pack에 catalog가 없으면 오류 없이 공통 경로를 쓴다. 이 선택은 실행 ontology 활성화나 운영 권위를 승인하지 않는다.

## 자료 schema

선택 pack의 `worksheet-catalog.yaml`은 독립 schema1이다. 필수 header는 `catalog_schema_version: 1`, `pack_id`, `pack_version`, `title`, `authority`, `scope`다. 알 수 없는 필드·중복 YAML 키·여러 YAML 문서를 거부한다. `references`는 최대32개인 label/source 배열, `items`는1–128개인 다음 배열이다.

```yaml
items:
  - id: risk-1
    label: Lock review
    mapping_label: RISK-1
    keywords: [lock, mutex]
    type: {pack_id: organization.example, local_id: concurrency-rule}
    source:
      path: catalog-source.md
      first: 1
      last: 3
      file_sha256: EXACT_FULL_FILE_SHA256
      content_sha256: EXACT_RETAINED_SPAN_SHA256
```

type은 자신의 pack 또는 직접 선언한 의존 pack에 존재하는 concept여야 한다. 같은 선택 배열에 있다는 이유로 선언하지 않은 namespace를 참조할 수 없다. item ID는 중복 불가이고 keywords는1–64개, 각128bytes 이하, 대소문자를 무시한 중복 불가다. header/label/권위/scope는 빈 값·제어 문자·개행·backtick 없이2048bytes 이하다. 자료 파일과 각 출처 파일은1MiB 이하의 일반 파일이다.

출처는 pack 내부의 실제 파일이다. 좌표는1-based inclusive이며 원문 줄 종결자도 span SHA에 포함한다. 전체 파일SHA와 정확 span SHA를 모두 검증한다. 범위/해시/타입/의존/선택 pin이 틀리거나 파일이 누락·변조되면 기존 출력 파일을 열기 전에 실패한다. 검증 뒤 pack tree digest를 다시 확인한다. 이 검증은 원문 신원·좌표의 일치이고 내용의 사실성이나 현재 권위에 대한 사람의 판정은 아니다.

## 출력과 검토 경계

header에 선택 pack ID/version/digest, authority/scope와 참조 출처 좌표를 표시한다. `Maps to`는 title/summary/invariants/pitfalls/ID의 소문자 문자열에 각 keyword가 포함된 수를 점수로 사용한다. 최고 점수의 첫 항목을 고르고0점은 매핑하지 않는다. 동률은 기존 pack 의존 순서, 각 catalog 자료 순서로 결정된다. 기존 상태·우선순위 필터와 ID 정렬, 앵커 힌트, 검토 질문·빈 queue를 유지한다. 매핑에는 namespace/type·출처 좌표/파일SHA/spanSHA를 함께 표시한다.

APPROVE/REVISE/REJECT 칸은 모두 비어 있다. 실제 검토 후 `cks domain --project PROJECT_DIR verify --entry ENTRY_ID --by REVIEWER_HANDLE`을 별도로 실행한다. 생성만으로 자동 승격하지 않는다. 후속 sync/glossary 및 새 pinned 후보 setup도 각각 별도 작업이다.

기존 StableNet8항목은 `projects/stablenet/domain-knowledge/packs/worksheet-risks/`로 이전했고 같은 프로젝트에만 명시 선택했다. 복사한 legacy heuristic은 운영 정책이나 원래 외부 문서의 신규 사실 승인으로 취급하지 않는다. 기존 생성 worksheet와 entry 원문을 수정하지 않고 별도 출력으로 비교했다.

## 검증 증거와 한계

[N-13 DEV 증거](../../system/eval/b0-knowledge-system/refactoring-n13-2026-10-06/manifest.json): 무팩 누출 RED→GREEN,8매핑·필터·앵커·빈queue·구팩 호환, 잘못된 pin/의존/type/출처/경로/자료 거부, 기존 출력 보호, 실제 Darwin ARM64 CLI 비교와 관련 race. 이 검증은 새 FINAL 검색 품질이나 실제 사람/운영 승인을 대신하지 않는다.
