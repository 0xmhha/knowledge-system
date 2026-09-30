# D5 상세 설계: 사용자 소유 도메인 지식 팩

상태: **D5-01–07 사용자 설계 승인** · 2026-10-01. 승인 내용은 [`D5-DECISION-REVIEW.md`](./D5-DECISION-REVIEW.md)에 기록한다. D1의 [`ontology-pilot.yaml`](./ontology-pilot.yaml) 20개 ID·정의·종류는 변경하지 않는다. 이 문서는 설치형 CKS가 산업별 어휘와 조직의 정책·설계 이유를 선택적으로 수용하기 위한 목표 계약이다. 현행 `ExtractOntology`/CLI가 이 형식을 지원한다는 뜻은 아니다.

## 1. 목적과 경계

코드/AST는 현재 구현의 구조를 관찰한 근거다. 정책과 설계 결정은 **사람 또는 권위 있는 문서가 제공한 의도**다. 둘을 잇는 관계는 별도 검토 대상이다. 코드에 조건문이 있다는 사실만으로 정책의 존재나 결정 이유를 추론해 `verified`로 만들 수 없다. 지식이 없을 때의 응답은 `unknown`, 서로 양립하지 않는 검토된 지식이 있을 때의 응답은 `conflict`다.

CKS는 팩을 읽고 근거 문맥을 코딩 에이전트에게 제공하지만 업무 시스템에 직접 쓰기 작업을 수행하지 않는다. 패치 실행·검증·사람의 수용 판정은 D1의 `patch-attempt`/`criterion-decision` 경로에 남는다. 산업 팩은 선택 사항이고, 단순 설치에는 공통 20개와 CKV+CKG 기본 질의만으로 동작한다.

## 2. 소유 계층과 권위

| 층 | 내용 | 지식의 권위 | 적용 방식 |
|---|---|---|---|
| `core` | 승인된 공통 20개 타입과 CKS 원천 좌표 | 제품 스키마 | 모든 프로젝트에 내장; 재정의 불가 |
| `engineering.decisions` | `design-decision`, `rationale`, `alternative`, `assumption`, `business-policy` 등 범용 확장 타입 | 타입 정의만 제공 | 선택해 설치; 프로젝트 정책이 자동 생성되지 않음 |
| 산업/기술 팩 | 예: `industry.blockchain`, `platform.evm`의 객체·행위·관계 타입·검증 규칙 | 재사용 템플릿 | 프로젝트가 명시 선택·버전 고정; 그 산업의 실제 정책이라는 뜻 아님 |
| 프로젝트 오버레이 | 조직의 용어·규칙·예외·ADR·책임자·유효 범위 | 검토 후 프로젝트의 규범적 원천 | 프로젝트별 별도 입력; 다른 프로젝트로 자동 전파 불가 |
| 데이터 인스턴스 | 실제 정책, 요구, 코드 심볼, 테스트, 결정, 근거 관계 | 각각의 원문과 검토 결과에 따름 | 한 데이터셋/스냅샷에서만 검증해 사용 |

공통 `policy`는 현재 CKS의 색인·접근·검토 정책을 가리킨다. 업무 규칙은 확장 팩의 `business-policy`로 분리한다. 공통 `criterion-decision`은 **수용 기준 판정**이고, 확장 `design-decision`은 **설계 선택과 이유**다. 새 타입들은 공통 20개에 합산하지 않는다.

산업 템플릿에는 조직 정책의 최종 권위가 없다. 특정 프로젝트의 `verified` 정책이 산업 팩의 일반 설명과 충돌하면 산업 설명으로 정책을 덮어쓰지 않고 충돌 또는 적용 제외를 기록한다. **검토된 프로젝트 정책이라도** 다른 검토된 프로젝트 정책과 충돌하면 자동 우선순위를 부여하지 않는다. 적용 범위·유효 기간·승인된 대체/예외가 모호성을 해소할 때만 결정적 답을 낸다.

## 3. 입력 배치와 잠금

프로젝트 소유 입력은 `.cks/knowledge/` 아래에 둔다. Git 저장소이면 일반 저장소 파일로, 비Git이면 A4의 `snapshot-only` 캡처 원천으로 처리한다. 사용자가 별도 로컬 경로를 등록한 팩은 네트워크에서 자동 다운로드하지 않고 캡처 시 `origin_id=knowledge:<pack_id>`로 보관한다. 등록되지 않은 절대 경로나 심볼릭 링크는 읽지 않는다. 제외·민감 경로는 CKV/CKG와 같은 캡처 정책을 거친다.

```text
.cks/knowledge/manifest.yaml          # 프로젝트 ID, 선택 팩, 소스 범위
.cks/knowledge/knowledge.lock.json     # 정확한 팩 버전·파일 SHA-256·정렬 순서
.cks/knowledge/domain/*.yaml           # 프로젝트 객체·행위·용어와 타입 매핑
.cks/knowledge/policies/*.yaml         # 정책·예외·유효 범위와 출처
.cks/knowledge/decisions/*.md          # 결정·이유·대안·근거의 구조화된 ADR
.cks/knowledge/questions/*.yaml        # 업무 질문·기대 경로·금지 해석
```

`cks knowledge init`은 템플릿을 만들지만 기존 파일을 덮어쓰지 않는다. `cks knowledge validate`는 읽기 전용으로 구문·타입·출처·충돌을 보고한다. `cks knowledge lock`은 명시 선택된 로컬 팩의 바이트를 해시해 재현 가능한 잠금 파일을 만든다. `cks knowledge review`는 제안 인스턴스/관계와 원천을 제시하고 사람의 검토 기록을 입력 원본에 남긴다. 실제 인덱스 빌드는 잠금 파일이 입력 바이트와 다르면 `pack_lock_mismatch`로 실패한다. 새 명령과 파일 형식은 A5.4/A5.5/A7.1 구현 과제다.

잠금 파일은 `project_id`, `pack_schema_version`, 정렬된 `{pack_id, version, digest, dependencies, origin_id}`와 `overlay_digest`를 담고, 정규화된 바이트의 SHA-256으로 자체 `lock_digest`를 만든다. 자가 참조를 피하기 위해 `lock_digest` 필드는 해시 입력에서 제외한다. **`overlay_digest`의 입력에서는 `knowledge.lock.json` 전체를 제외**하고, `manifest.yaml` 및 `domain/`, `policies/`, `decisions/`, `questions/`의 등록된 일반 파일을 `(origin_id,path)` 순서로 해시한다. 각 파일 레코드는 A4의 상대 경로 정규화·바이트 SHA-256 규칙을 따르며 임시/숨김/미등록 파일을 묵인하지 않는다. `manifest.yaml`의 `source`는 저장소 루트 기준 상대 경로 또는 별도 등록한 로컬 루트의 `origin_id` 참조이며, 절대 경로·심볼릭 링크·루트 탈출은 거부한다. 외부 팩의 정확한 파일 목록과 다이제스트는 A4 원천 매니페스트에도 포함한다. `build_recipe_digest`는 `lock_digest`, resolver/renderer 버전과 관계 검증기 버전을 포함한다. 그러므로 같은 코드라도 팩이나 프로젝트 정책 바이트가 바뀌면 새 `dataset_id`가 된다. 기존 데이터셋을 제자리 수정하지 않는다.

잠금의 `digest`와 `overlay_digest`는 정렬된 파일 레코드의 길이 접두어 직렬화에 대해 계산한다. 파일 해시는 **원문 바이트**에 대해 계산하므로 YAML 키 순서나 Markdown 공백 변경도 새 잠금을 요구한다. `lock_digest`만 잠금 객체에서 해당 필드를 제거한 뒤 키 순서가 고정된 정규 JSON 바이트에 대해 계산한다. 이 두 해시 절차의 골든 바이트와 충돌/누락 오류 코드는 A5.4 fixture로 고정한다.

## 4. 팩 스키마와 합성

팩 정의의 최소 필드는 `pack_schema_version=1`, `pack_id`, `version`, `owner`, `scope`, `requires`(정확 버전/다이제스트), `concepts`, `relation_types`, `constraints`, `competency_questions`다. `pack_id`는 소문자 점 구분 네임스페이스, 로컬 타입 ID는 D1의 소문자 케밥 형식을 사용한다. 참조의 논리 키는 **`(pack_id, local_id)` 튜플**이고 직렬화·해시는 D2의 길이 접두어 규칙을 쓴다. 화면에 표시할 `pack_id#local_id` 문자열은 조인 키나 파서의 유일 근거가 아니다. 같은 ID/라벨이 서로 다른 팩에 있더라도 자동 병합하지 않는다.

입력의 최소 모양은 다음과 같다. **목표 형식 예시**이며 현행 파서는 읽지 못한다. `source`는 명시 등록된 로컬 디렉터리만 가리키고 빌더가 원문을 캡처해 고정한다.

```yaml
# .cks/knowledge/manifest.yaml
schema_version: 1
project_id: sample-wallet
selected_packs:
  - pack_id: engineering.decisions
    version: 1.0.0
    source: ./vendor/cks-packs/engineering-decisions
    sha256: <64-hex>
  - pack_id: industry.blockchain
    version: 1.0.0
    source: ./vendor/cks-packs/blockchain
    sha256: <64-hex>
overlay_root: .cks/knowledge
```

```yaml
# .cks/knowledge/policies/BR-17.yaml — 이 프로젝트가 작성한 초안
id: BR-17
type: {pack_id: engineering.decisions, local_id: business-policy}
statement: A transfer above the project limit requires separate approval.
scope: {subsystem: transfers}
effective_from: "2026-09-30"
status: proposed
source_ref: {origin_id: repo, path: .cks/knowledge/policies/BR-17.yaml}
```

`sha256`과 잠금 파일은 예시 문자열을 허용하지 않는다. 실제 명령은 등록된 팩의 정규화 파일 목록/바이트 SHA-256을 계산해 채운다. `BR-17`의 줄·범위 해시는 캡처 후 추출기가 기록하며 사람이 YAML에 임의 해시를 쓰지 않는다. `status: verified`로 바꾸려면 원본 정책의 담당자·검토자·이유와 해당 스냅샷 근거가 필요하다. 블록체인 팩이 설치됐다는 사실만으로 이 정책의 사실성이나 적용이 승인되지는 않는다.

팩은 의존성 DAG 순서로만 합성한다. 정확한 의존 버전/다이제스트가 없거나 순환하면 실패한다. `core` ID/정의 재정의는 실패한다. 하위 팩의 `extends`/`maps_to`는 상위 타입과 호환 가능한 속성·관계 범위만 명시적으로 추가할 수 있고, 기존 뜻을 바꾸는 경우 새 ID/주버전과 이전 매핑을 요구한다. 프로젝트 오버레이도 타입을 몰래 덮어쓰지 않는다. 한영 용어 충돌은 오류가 아니라 **범위가 있는 복수 후보**로 기록하며, 검토되지 않은 후보는 기본 코딩 문맥에 규범으로 넣지 않는다.

관계 타입은 `{predicate, subject_type, object_type, direction, cardinality, required_evidence, review_rule}`을 가진다. 확장 관계명은 팩으로 한정한다. 관계 인스턴스는 양쪽 원천, 스냅샷, 검토자, 상태와 관계 타입 버전을 가진다. `Concept IMPLEMENTED_BY CodeSymbol` 같은 D1 관계는 예약어로 유지한다. 산업 팩은 새 타입 관계를 선언할 수 있지만 CKG의 AST 사실이나 CKV 청크를 복제하지 않는다. 제약 검증은 타입·필수 필드·카디널리티·참조 무결성을 검사한다. RDF/OWL 또는 SHACL 런타임은 필수가 아니며, [SHACL](https://www.w3.org/TR/shacl/)과 유사한 형상 검증을 구현 계약으로 삼는다.

현행 `OntologyPack`은 `project_id`를 필수로 하는 단일 파일이며 `Concept.ID`와 허용 술어가 전역으로 고정돼 있다. A5.4는 이 파일을 **legacy core 입력**으로 읽는 어댑터를 둔다. 새 팩 인덱스와 의미 투영에는 튜플 키를 기록하고, 기존 `project` 등 20개 ID/구 투영 JSON은 읽기 전용으로 보존한다. D1의 `ACCEPTED_BY`→`CHECKED_BY` 이전과 팩 이전은 별도 골든으로 검증한다. 새 팩 투영의 정확한 파일 스키마 버전은 A5.1의 좌표/술어 마이그레이션 뒤 **v4**로 지정한다. 현재 SQLite `StoreSchemaVersion=3`은 다른 버전 축이며 v4 투영 번호와 같다고 가정하지 않는다.

## 5. 사용자가 제공하는 지식 인스턴스

프로젝트 정책에는 `id`, 원문/요약, 소유자, `source_ref`, 적용 서브시스템·행위, `effective_from`/`effective_to` 또는 명시된 명세 버전, 예외와 대체 관계, 상태/검토자를 요구한다. 시점/버전을 명시하지 않은 초안은 과거 설명 후보가 될 수 있지만 “현재 반드시 지켜야 할 정책”으로 표시하지 않는다. ADR에는 문제, 결정, 이유, 비교한 대안, 가정, 채택/폐기/대체 날짜, 영향받는 요구사항과 원천을 요구한다. 원문은 Markdown으로 보존하고 구조화 필드는 파생·검증된 레코드로 저장한다. 검토자가 원문을 확인하지 않은 LLM 추출은 `proposed`다.

상태 축은 둘로 나눈다. 저장된 검토 판정은 `proposed|verified|rejected`이고, 조회 시 계산되는 유효 상태는 `current|stale|conflict|unknown|restricted`다. `verified`가 과거 스냅샷에만 맞거나 유효 기간이 끝나면 현재 질의에서는 `stale`이다. 같은 범위·시점의 양립 불가능한 검토된 정책은 `conflict`다. 근거가 없으면 `unknown`이다. 권한으로 원문을 공개할 수 없으면 후보 ID나 본문을 노출하지 않고 `restricted`만 표시한다. 삭제 대신 새 버전/대체 관계를 남겨 과거 패치 이유를 재현한다.

프로젝트 `manifest.yaml`의 `review_policy.min_approvals`는 기본 1이며, 명시적으로 1 이상의 정수만 허용한다. 각 승인은 검토자 ID, 대상 원문 해시·스냅샷, 판정, 이유, 시각을 가진다. 동일 검토자의 중복 기록은 승인 수를 늘리지 않는다. 로컬 설치형에서는 검토자 ID의 **기록·감사**를 보장하며 외부 신원 제공자 없이 신원을 암호학적으로 인증한다고 주장하지 않는다. 조직이 서명 또는 인증을 요구하면 프로젝트별 권한 어댑터를 추가하고 그 전에는 해당 조직의 검토 요건을 충족했다고 표시하지 않는다.

자동 추출은 정책/결정/코드 연결의 **후보**만 만든다. `verified` 승격에는 해당 입력 스냅샷의 원문 해시 재검증, 타입/범위 검증, 사람 검토자와 이유가 필요하다. 코드 심볼 앵커는 CKG의 `canonical_id`·파일·줄을, 문서 근거는 CKV/원문 보관본의 `origin_id`·경로·줄·해시를 대조한다. 소스에서 발견한 조건문은 정책 인스턴스를 생성하거나 승인하지 않는다. 정책의 작성 주체와 검토자는 W3C [PROV-O](https://www.w3.org/TR/prov-o/)의 Entity/Activity/Agent 구분에 맞게 표현할 수 있지만, 현행 시스템의 표준 준수를 주장하지 않는다.

## 6. CKS 질의와 코딩 문맥

`cks.context.get_for_task_v2`는 원문 질의를 먼저 기존 CKV/BM25/정확 심볼 경로에 보내고, 결과의 프로젝트/데이터셋과 요청 서브시스템에 적용되는 잠금 팩만 연다. 접근 허용된 원천을 먼저 필터한 뒤 검토된 정책·결정→요구→심볼→테스트 경로를 제한 깊이로 탐색한다. 적용 범위가 맞지 않는 산업 팩, 제안 상태 관계, 오래된/충돌 정책은 규범 근거로 추가하지 않는다. 복수 의미 용어는 후보로 남긴다. 의미 경로 오류/시간 초과/예산 초과에서도 기존 상위 K CKV+CKG **후보 집합**을 유지한다.

v2 `semantic.knowledge_context`는 `{state, lock_digest, applicable_policies, decisions, constraints, related_requirements, test_links, unknowns, conflicts}`를 가진다. 각 항목은 안정 ID, 타입, 적용 범위, 검토/유효 상태, **v2 citation 좌표에 대한 참조**와 관계 ID를 포함한다. 텍스트 본문은 별도 인용에서만 제공하고 정화·권한 검사를 거친다. `unknowns`는 “왜”의 원천이 없거나 적용 범위가 모호한 이유를, `conflicts`는 양립하지 않는 근거의 허용 가능한 식별자만 설명한다. `state=complete`는 반환한 **근거 경로의 무결성**만 뜻하고 업무 정답률을 뜻하지 않는다. `partial|conflict|unavailable|budget_exceeded|restricted`에서는 확정적인 정책 준수/이유 문장을 만들지 않는다. 의미 필드와 모든 인용은 v2 팩 무결성 해시에 포함하며 v1 DTO/해시는 변경하지 않는다.

기본 코딩 문맥은 `implemented behavior`, `required behavior`, `rationale`, `constraints`, `evidence`, `unknowns`를 명시적으로 분리한다. LLM이나 사용자가 이를 한 문장으로 뭉쳐 사실처럼 주장하지 않도록 기계 판독형 상태를 제공한다. 정책/ADR 문서는 지식 **데이터**이지 에이전트의 명령이 아니다. 사용자 요청과 시스템 권한을 뒤집는 문구가 문서에 있어도 실행 지시로 해석하지 않는다. 실모델 품질/지연의 합격 여부는 B/C에서 평가한다.

## 7. 설치·이전·실패 정책

배포물은 공통 팩과 빈 프로젝트 템플릿만 포함한다. 블록체인 등 산업 팩은 설치/프로젝트 초기화 때 명시적으로 선택한다. 오프라인 로컬 경로만 허용하고 설치 대상 macOS arm64·Linux arm64/amd64에서 동일 잠금 파일/원천 해시를 검증한다. 팩 없음은 정상 `core_only`, 팩은 있으나 잠금 불일치는 후보 빌드 실패 `pack_lock_mismatch`, 누락 의존/주버전 불일치는 `pack_incompatible`, 무근거 관계는 `evidence_unverified`, 실제 정책 충돌은 질의 `conflict`다. 실패 후보는 활성 포인터를 건드리지 않는다.

v1/현행 v2 소비자는 새 팩 필드를 조용히 의미 있는 사실로 해석하지 않는다. 기존 `get_for_task`는 기존 v1 DTO/해시를 유지하고, 새 의미 문맥은 명시 v2 도구에서만 반환한다. 기존 단일 YAML의 20개는 마이그레이션 시 바이트와 출처를 보존하며 `core` 참조로 매핑한다. 구 데이터셋은 읽기 전용으로 남겨 롤백 가능하다. 팩 변경은 원문 캡처→후보 빌드→근거·권한·충돌 검사→활성 포인터 교체로 처리한다. 롤백은 대상 팩과 정책 원문 blob, lock/DB/인용 해시를 확인한 뒤 수행한다.

## 8. 설계 검증 행렬

| 사례 | 입력 | 기대 판정 |
|---|---|---|
| 기본 사용 | 도메인 팩 없음 | 공통 20개/CKV+CKG 결과와 이전 API 골든 유지 |
| 격리 | 같은 코드 바이트·서로 다른 프로젝트 팩 | 다른 데이터셋 ID, 팩·캐시·답변 혼입 0 |
| 버전 | 팩 바이트/정책 변경, 고정 코드 | 잠금 불일치 빌드 실패 또는 새 데이터셋; 과거 인용/롤백 보존 |
| 그래프 타입 | 반대 방향/틀린 끝점/누락 의존/순환/중복 ID | 승격 거부와 안정 오류 코드; 이전 `current` 유지 |
| 불확실성 | 이유 문서 없음, 충돌/만료/미검토 정책 | `unknown|conflict|stale`와 확정 이유 0건 |
| 권한 | 비공개 정책/인용 | 권한 없는 응답에 ID·본문·존재 정보 누출 0건 |
| 검색 | 모호한 한영 용어, 팩 오류, 예산 초과 | 기본 CKV+CKG 후보 보존; 의미 상태만 하향 |
| 코딩 변경 | 정책→결정→요구→심볼→테스트→패치 | 각 관계 원천 검증, 테스트 통과만으로 승인 불가 |
| 산업 독립성 | 블록체인 예시와 무관한 Go 문서 프로젝트 | 산업 개념 자동 적용 0건 |

이 행렬은 모델 독립 A 단계에서 고정 fixture와 실패 주입으로 검증한다. B0에서는 사용자 승인 질문/정답에 “왜”, 적용 정책, 충돌/기권을 포함하고, B1에서 기본 CKV+CKG와 팩 사용/미사용을 비교한다. 품질이 입증되기 전 기본 자동 확장·재순위는 꺼진다.

## 9. 구현에 사용할 운영 기본값

사용자는 **공통 20개 고정 및 조직별 팩 분리**를 승인했다. 세부 설계가 한 산업의 실제 정책이나 운영 조직을 가정하지 않도록 다음 기본값으로 WBS를 작성한다.

| 항목 | 권고 기본값 | 변경 경계 |
|---|---|---|
| 첫 파일럿 | `knowledge-system` 자신의 정책/ADR을 실사용 사례로 사용; 블록체인/EVM은 선택형 구조 fixture, 비블록체인 프로젝트는 격리 대조 | 사용자가 실제 블록체인 프로젝트를 지정하면 B0 정답/정책 승인자를 별도 기록 |
| 입력 위치 | 저장소 `.cks/knowledge`가 기본; 등록한 별도 **로컬** 문서/팩 루트도 1급 입력 | 외부 원천은 origin/경로/해시/권한을 캡처해 네트워크 자동 수집 금지 |
| 검토 역할 | 첫 버전은 프로젝트 소유자 1명 이상의 명시적 검토·이유를 요구; `min_approvals`를 설정할 수 있게 설계 | 복수 승인자가 필요한 조직은 프로젝트 정책에 따라 2명 이상 지정, 미충족 시 `proposed` 유지 |

사용자는 2026-10-01에 이 세 운영 기본값과 D5-02–07 전체 권고를 제안대로 승인했다. 따라서 A5.4/A5.5/A6/A7.1의 설계 입력으로 고정한다. 향후 다른 산업/검토 체계를 선택하면 버전·fixture·WBS를 함께 개정한다. 정책 인스턴스의 `verified` 승격과 B0 정답 승인은 별도 사람 판정을 기다린다. D1의 20개와 D2–D4 계약은 이 선택으로 자동 변경하지 않는다.
