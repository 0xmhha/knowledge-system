# CKS v1/v2 공개 계약과 호환 골든

상태: 선택형 v2 DTO·MCP 도구·보관 원문 인용·정화·무결성 해시와 CKS MCP의 보관본 후보 검색 경로를 검증했다. 선택 팩의 외부 origin 보관·인용, 검토된 정책/ADR 오버레이, 정렬된 의미 투영에 한정한 ADR→요구→코드→테스트 추적 경로를 구현했다. 조직 인증 어댑터와 운영 도메인 사실의 사람 검토는 별도다. 이 문서는 `END-TO-END-DESIGN.md` 7절의 직렬화와 버전 경계를 고정한다. D1-02 사용자 승인에 따라 의미 술어 계약을 아래처럼 개정했다.

## 버전 선택

| 입력/도구 | committed v1 | committed v2 | working-tree v2 | snapshot-only v2 |
|---|---|---|---|---|
| 기존 `cks.context.get_for_task` | v1 DTO/해시 | v1 DTO/해시 | `requires_v2` | `requires_v2` |
| 새 `cks.context.get_for_task_v2` | `reindex_required` | v2 DTO/해시 | v2 DTO/해시 | v2 DTO/해시 |
| `cks setup --version` (설정·project pin 포함) | 기존 플래그 유지 | 같은 v2 후보 빌더 또는 v1 읽기 | 모드 미지정이면 committed만 | 모드 미지정이면 committed만 |
| `ckv query`/CKG export 구 형식 | 기존 응답 | 기존 커밋형 응답 | `requires_v2` | `requires_v2` |
| 명시 `--format=v2`/v2 MCP·HTTP | `reindex_required` | v2 응답 | v2 응답 | v2 응답 |

구현 상태: `cks.context.get_for_task_v2`와 비커밋 데이터셋의 기존 CKS 문맥 도구 거부는 실제 MCP 스모크로 검증했다. 고정된 main 기준선의 v1 MCP 응답을 가짜 Ollama와 별도 바이너리로 재생했고, 새 `doctor`의 `legacy_unpinned` 및 새 v2 MCP의 `reindex_required`를 확인했다. 직접 `ckv query`는 보관본 마운트 없이 비커밋 데이터셋을 거부한다. CKG 독립 export와 일반 `--format=v2`/HTTP 표면은 아직 이 표의 설계 계약에 머무른다.

구 바이너리가 새 pinned v2 DB를 읽는 역방향 호환은 제공하지 않는다. 새 CKV의 임베딩 신원 포맷을 구 CKV가 이해하지 못하므로 새 데이터셋에는 새 바이너리를 사용한다.

기존 v1 응답은 **별도의 v1 DTO**로 직렬화한다. 기존 필드에 v2 좌표를 단순 추가하지 않는다. 현재 v1 팩의 `integrity_hash` 계산은 알지 못하는 JSON 필드를 버리고 다시 계산하는 소비자와 결합돼 있으므로, 혼합 DTO는 해시 불일치를 낳을 수 있다. v1 `Citation.Key()`는 커밋도 무시하므로 v2 중복 제거에 사용하지 않는다. 기존 도구가 돌려주는 committed v2 자료의 원문은 보관본으로 검증하지만 응답 자체는 v1 모양과 기존 SHA-256 해시를 유지한다. 호출자는 v1 응답을 다른 스냅샷과 조인하지 않는다.

## 유지보수 쓰기 경계 (N-02)

`ops.index`는 평면 legacy 데이터셋의 직접 갱신에 한정한다. 버전 고정/blue-green 경로와 손상된 pin은 export/엔진 실행 전에 `versioned_setup_required`로 거부한다. 현재 MCP `ops.setup`/`ops.reindex` 입력은 v2의 전체 설정·팩·소스·모델 신원을 전달하지 않으므로 pinned 데이터셋의 제자리 빌드와 legacy 후보로의 downgrade를 허용하지 않는다. 새 v2 후보는 동일한 검토 설정과 project ID를 사용하는 `cks setup --version <새 버전>`으로 빌드/검증/승격하며 `--hold-for-review`로 보류할 수 있다. 기존 버전 이름 재사용은 legacy도 거부한다. 평면 legacy 갱신 및 새로운 legacy blue-green 후보 빌드는 유지한다. 기존 서버는 승격 후에도 시작 시 pin한 버전을 사용하므로 새 버전을 읽으려면 재시작한다. low-level 엔진을 직접 실행한 쓰기는 이 관리 경로의 보호 밖이다. [설계·수용 조건·증거](./REFACTORING-EXECUTION-SPEC.md)를 따른다.

### dataset mutation 잠금 (N-03)

versioned 빌드·gate·보류/승격과 외부 승격/rollback은 동일한 OS 배타 잠금으로 직렬화한다. `.reindex.lock` 파일을 영구 유지하고 OS가 owner 종료 후 소유권을 회수한다. 잠금 파일 나이로 live holder를 탈취하거나 해제 시 unlink하지 않는다. 이전 O_EXCL writer의 살아 있는 PID는 시각과 무관하게 거부하며, 구 writer를 모두 종료한 뒤 새 writer로 업그레이드한다. 혼합 버전 writer와 flock 의미가 보장되지 않는 network filesystem은 이 계약 밖이다. 후보 DB checkpoint/close·전체 파일/디렉터리 sync 이후 current를 rename하고 dataset 부모를 sync한다. linked/nonregular artifact와 busy/잔여 WAL·변경된 DB pin은 승격하지 않는다. 검토 승격은 durable intent를 먼저 기록하고 포인터 전환/부모 sync 후 release를 기록한다. 같은 승인/base/신원만 중단 후 복구할 수 있다. rename 후 sync/release 실패는 `durability_uncertain`으로 보고하므로 current가 변경되지 않았다고 추측하지 않는다. 실제 지원 환경과 전원 차단 검증은 별도다.

## v2 응답의 필수 필드

```json
{
  "format_version": 2,
  "coordinates": {
    "project_id": "knowledge-system", "dataset_id": "<64-hex>",
    "snapshot_id": "<64-hex>", "source_mode": "working-tree",
    "base_commit": "<40-hex>"
  },
  "query": "Which symbol implements the requirement?",
  "citations": [{
    "file": "main.go", "start_line": 2, "end_line": 2, "commit_hash": "",
    "project_id": "knowledge-system", "dataset_id": "<64-hex>",
    "snapshot_id": "<64-hex>", "source_mode": "working-tree",
    "base_commit": "<40-hex>", "origin_id": "repo",
    "file_sha256": "<64-hex>", "content_sha256": "<64-hex>"
  }],
  "bodies": [], "graph_neighbors": [], "semantic": null,
  "evidence_state": "complete",
  "metadata": {"integrity_hash_algo": "sha256-v2", "integrity_hash": "<64-hex>"}
}
```

`coordinates`와 각 인용의 ID/모드는 완전히 같아야 한다. `origin_id`와 `file`은 보관본의 등록된 상대 경로를 가리킨다. `committed`에서는 `commit_hash=base_commit`, `working-tree`와 `snapshot-only`에서는 `commit_hash=""`다. 비Git `snapshot-only`의 `base_commit`도 빈 문자열이다. `file_sha256`은 전체 보관 파일, `content_sha256`은 지정 줄 범위의 정화 전 원문 바이트다. 두 값을 재검증한 뒤 본문을 정화한다. 상태가 `partial`이면 확정 답변으로 사용할 수 없다.

현재 CKV의 `chunks.content_sha256`은 청크 텍스트의 다이제스트다. v2 인용의 `content_sha256`은 **보관 원문 줄 바이트에서 새로 계산**하며 이 값을 CKV의 기존 열에서 그대로 복사하지 않는다. 줄 끝 규칙은 `END-TO-END-DESIGN.md` 2.1절의 원문 바이트·줄 경계를 따른다.

v2 인용 중복 키는 `(project_id,dataset_id,snapshot_id,origin_id,file,start_line,end_line,file_sha256,content_sha256)`의 순서 있는 튜플이다. 문자열 결합으로 충돌을 만들지 않도록 길이 접두어 직렬화하거나 튜플 자체로 비교한다. 서로 다른 스냅샷에서 경로와 줄이 같아도 합치지 않는다.

`sha256-v2`는 `metadata.integrity_hash`를 **제외**하고 `metadata.integrity_hash_algo`를 포함한 전체 v2 응답을 [RFC 8785 JSON Canonicalization Scheme](https://www.rfc-editor.org/rfc/rfc8785.html)으로 정규화한 UTF-8 바이트의 SHA-256이다. 정화된 본문과 `semantic` 필드까지 포함한다. 중복 JSON 키, 유효하지 않은 UTF-8, I-JSON 밖의 수치는 거부한다. 구현은 Go 생산자와 독립 소비자의 동일 바이트·해시 골든을 갖춘다. v1의 `ComputeIntegrityHash`는 바꾸지 않는다.

현재 v2 생산자는 고정 DTO의 ASCII 속성명, 문자열, null, 배열, 안전 정수만 정규화한다. 부동소수점과 사용자 정의 오버레이 객체는 거부하며 `semantic=null`, `graph_neighbors=[]`로 출력한다. 이는 완성된 범용 JCS 구현을 뜻하지 않는다. JavaScript 소비자 골든은 현재 DTO의 정규화 바이트를 고정하며, 오버레이를 공개할 때 추가 골든이 필요하다.

### D5 선택형 도메인 문맥

사용자가 도메인 팩을 선택하고 검토된 프로젝트 정책/결정이 존재하면 명시 v2 응답의 `semantic.knowledge_context`에 `{state, lock_digest, applicable_policies, decisions, relations, constraints, related_requirements, test_links, unknowns, conflicts}`를 싣는다. 각 항목은 안정 타입/ID와 상태, 적용 범위, **같은 응답의 v2 인용 좌표**로 검증할 수 있는 출처 참조만 가진다. 본문은 권한/정화 검사를 거친 `bodies`/인용 경로로만 노출한다. `state=complete`는 이 문맥의 근거 무결성을 뜻하며 업무 정답을 보증하지 않는다. `unknown|conflict|stale|restricted|budget_exceeded` 항목은 확정 이유나 정책 준수로 직렬화하지 않는다. `semantic` 전체가 위 `sha256-v2` 해시 범위에 든다. 팩 미설치 또는 기능 꺼짐은 `semantic=null`을 허용하며 기존 후보·v1 DTO/해시는 유지한다. 정확 필드/상태 규칙은 [`DOMAIN-PACK-CONTRACT-V1.md`](./DOMAIN-PACK-CONTRACT-V1.md)에 있다.

현재 실행되는 선택형 정책 경로는 MCP 입력 `include_knowledge=true`, `knowledge_as_of=YYYY-MM-DD`, `knowledge_subsystem=<명시 범위>`를 요구한다. 정책 원문은 보관본 전체 파일 범위로 인용하고 정화 후 `bodies`에 둔다. 공개 가능하고 검토된 정책만 `applicable_policies`에 넣으며 `conflict`, `restricted`, `stale`, `unknown`, `budget_exceeded`, `unavailable`을 확정 답으로 취급하지 않는다. 오류 시 기존 코드 인용을 유지한다. `decisions`에는 같은 보관본의 검토된 공개 ADR을 날짜·범위·대체 관계에 따라 인용할 수 있다. `relations`는 검토된 로컬 정책/ADR 끝점 두 개가 모두 현재 질의에 선택되고 충돌하지 않을 때만 별도 보관 원문 인용과 함께 출력한다. `related_requirements`와 `test_links`는 검증된 외부 앵커가 없으면 빈 배열이다. 조직 인증/권한 어댑터나 자연어 범위 추론이 구현됐다는 뜻은 아니다.

선택형 `semantic.coding_context`는 `implemented_behavior`, `required_behavior`, `rationale`, `constraints`, `evidence`, `unknowns`를 별도 배열로 반환한다. 현재 `required_behavior`는 검토된 정책 인용, `rationale`은 검토된 ADR 인용에서만 파생한다. 정책이 충돌하면 정책 원문은 `knowledge_context`의 상충 근거로 인용하되 `required_behavior`를 비우고 `conflicting_policies_require_review`를 기록한다. 검증된 코드 구현 링크가 없으면 `implemented_behavior`와 `constraints`는 비워두고 `implementation_link_unverified`를 `unknowns`에 적는다. `evidence`는 응답 안의 v2 인용 좌표만 담는다. 이 객체도 `sha256-v2` 해시 범위에 든다. 필드가 없던 이전 v2 팩은 기존 해시/바이트를 수정하지 않고 읽는다.

## 오류와 마이그레이션

### D1-02 의미 관계명 개정

새 통합 데이터셋의 정식 술어는 `AcceptanceCriterion CHECKED_BY TestCase`다. 이것은 **검토된 테스트 연결**만 뜻한다. 사람의 의미 수용은 `CriterionDecision.approved` 사건에서만 나온다. 현재 새 의미 투영 스키마 v3는 `CHECKED_BY`를 기록하고 기존 v1/v2 투영의 `ACCEPTED_BY`는 **읽을 때만** 연결로 해석한다. 구 투영의 `ACCEPTED_BY`를 사람 승인으로 바꾸거나 기존 바이트/해시를 제자리 수정하지 않는다. 구버전 소비자는 기존 DTO/술어를 계속 읽고, v2 소비자는 정식 이름과 `link_only` 상태를 받는다. 정식 술어가 아닌 `ACCEPTED_BY`의 새 쓰기는 거부한다. 양쪽 방향·원천·검토자 조건은 동일하게 유지한다.

마이그레이션 골든은 구 투영 읽기, 새 투영 쓰기, 구 술어 새 쓰기 거부, 사람 판정 부재 시 `accepted` 오판정 금지, v1 응답/해시 불변을 포함한다. 의미 투영 v3의 `CHECKED_BY` 쓰기, v1/v2의 `ACCEPTED_BY` 읽기, 새 쓰기 거부, v3 추적 필드 분리, 별도의 사람 기준 결정과 공개 v2 EvidencePack을 구조 시험했다. 실무 기준의 진실성과 검사 품질은 B 평가에서 검토한다.

MCP 도구 실패는 `IsError=true`와 `code=<고정 코드>` 텍스트로 전달한다. v2 도구는 같은 코드를 기계 판독형 오류 데이터에도 기록한다. CLI 실패는 비정상 종료와 `{ "code": "...", "message": "...", "dataset_id": "..." }` 한 객체를 내보낸다. `dataset_id`를 알 수 없으면 생략한다. `requires_v2`, `reindex_required`, `snapshot_mismatch`, `source_missing`은 서로 바꾸어 쓰지 않는다. 오류 메시지에는 원문 본문·비밀·임시 staging 경로를 넣지 않는다.

v1 데이터는 커밋형 읽기 전용 `legacy_unpinned`이며 좌표를 추측해 보충하지 않는다. v2 후보는 옆에 만들고 성공 시 단일 `current` 포인터를 전환한다. 롤백은 대상 버전의 매니페스트·DB 해시·원문 보관본과 바이너리 호환 범위를 먼저 확인한다. 미검증 디렉터리 존재만으로는 롤백 대상이 아니다. 기존 서버는 시작 시 열었던 버전을 계속 서빙하고 새 버전은 재시작 뒤 연다.

## 구현 전·구현 후 골든

1. 현재 v1 인용 JSON 네 필드와 v1 팩 해시를 고정한다. `get_for_task`의 실제 필수 입력은 `prompt`이며 계약 fixture와 등록 도구의 속성/필수 목록을 비교한다.
2. committed v2에서 구 도구의 출력은 v1 DTO와 해시 검증을 유지한다. working-tree/snapshot-only에서 구 도구는 본문을 반환하지 않고 `requires_v2`를 낸다.
3. 세 source mode의 v2 인용·오류·정규화 해시를 별도 골든으로 고정한다. 좌표/본문/semantic 어느 한 바이트를 바꾸어도 검증에 실패한다.
4. v1→v2 재색인, 실패 후보, 롤백, 기존 프로세스와 새 프로세스의 서빙 ID를 재생한다. 빈 좌표의 v1과 v2 근거를 같은 팩에 섞지 않는다.

과거 `internal/system/mcp/testdata/agent-mcp.schema.json`은 `task` 입력을 적고 있었지만 등록 도구는 `prompt`를 받았다. fixture와 회귀 시험을 실제 입력에 맞췄다. v2 DTO의 좌표·본문·선택형 의미 필드와 정규화 해시를 변조/왕복 시험했다.

## 보존 계획·GC와 reader 계약

`cks gc --out <dataset> --dry-run --plan-file <plan.json>`은 기본 최근2개·최소30일과 보호 이유·파일 바이트/미복구 trash/회수·capacity 상태를 보여 준다. `--apply --plan-file`은 동일한 계획 digest와 현재 참조/파일 상태에서만 삭제한다. `--resume`은 검증된 중단 journal만 재개하며 foreign/unknown trash를 지우거나 복구 완료라고 보고하지 않는다. 보호 때문에 capacity 목표를 못 맞추면 over_budget을 유지한다.

current·실제 rollback 참조·명시 보호·유효하게 해제되지 않은 review hold/intent·살아 있는 reader를 보호한다. 새 pinned 후보는 reader-flock-v1을 기록하고 managed CKS는 backend/보관 원문을 열기 전에 shared OS lease를 얻는다. lease inode를 보존하고 종료/crash 시 OS가 해제한다. 이전 후보와 검증 불가/부분 후보는 자동 회수하지 않는다. 구 서버·직접 low-level engine reader는 GC와 병행하지 않고 중단 후 새 managed CKS로 전환한다. 이는 임의 consumer/network filesystem에 대한 reader 추적 보장이 아니다. [전체 명세·원자료](./REFACTORING-EXECUTION-SPEC.md)의 검증/플랫폼 경계를 따른다.
