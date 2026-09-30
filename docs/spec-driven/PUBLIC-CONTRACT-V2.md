# CKS v1/v2 공개 계약과 호환 골든

상태: 설계 계약. v2 도구·포맷은 아직 구현되지 않았다. 이 문서는 `END-TO-END-DESIGN.md` 7절의 직렬화와 버전 경계를 고정한다. D1-02 사용자 승인에 따라 의미 술어 계약을 아래처럼 개정했다. 구현 수용은 실제 CLI/MCP/HTTP 소비자 재생으로 별도 판정한다.

## 버전 선택

| 입력/도구 | committed v1 | committed v2 | working-tree v2 | snapshot-only v2 |
|---|---|---|---|---|
| 기존 `cks.context.get_for_task` | v1 DTO/해시 | v1 DTO/해시 | `requires_v2` | `requires_v2` |
| 새 `cks.context.get_for_task_v2` | `reindex_required` | v2 DTO/해시 | v2 DTO/해시 | v2 DTO/해시 |
| 기존 `cks setup`/`mcp` | 기존 플래그 유지 | 같은 v2 후보 빌더 또는 v1 읽기 | 모드 미지정이면 committed만 | 모드 미지정이면 committed만 |
| `ckv query`/CKG export 구 형식 | 기존 응답 | 기존 커밋형 응답 | `requires_v2` | `requires_v2` |
| 명시 `--format=v2`/v2 MCP·HTTP | `reindex_required` | v2 응답 | v2 응답 | v2 응답 |

기존 v1 응답은 **별도의 v1 DTO**로 직렬화한다. 기존 필드에 v2 좌표를 단순 추가하지 않는다. 현재 v1 팩의 `integrity_hash` 계산은 알지 못하는 JSON 필드를 버리고 다시 계산하는 소비자와 결합돼 있으므로, 혼합 DTO는 해시 불일치를 낳을 수 있다. v1 `Citation.Key()`는 커밋도 무시하므로 v2 중복 제거에 사용하지 않는다. 기존 도구가 돌려주는 committed v2 자료의 원문은 보관본으로 검증하지만 응답 자체는 v1 모양과 기존 SHA-256 해시를 유지한다. 호출자는 v1 응답을 다른 스냅샷과 조인하지 않는다.

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

### D5 선택형 도메인 문맥

사용자가 도메인 팩을 선택하고 검토된 프로젝트 정책/결정이 존재하면 명시 v2 응답의 `semantic.knowledge_context`에 `{state, lock_digest, applicable_policies, decisions, constraints, related_requirements, test_links, unknowns, conflicts}`를 싣는다. 각 항목은 안정 타입/ID와 상태, 적용 범위, **같은 응답의 v2 인용 좌표**로 검증할 수 있는 출처 참조만 가진다. 본문은 권한/정화 검사를 거친 `bodies`/인용 경로로만 노출한다. `state=complete`는 이 문맥의 근거 무결성을 뜻하며 업무 정답을 보증하지 않는다. `unknown|conflict|stale|restricted|budget_exceeded` 항목은 확정 이유나 정책 준수로 직렬화하지 않는다. `semantic` 전체가 위 `sha256-v2` 해시 범위에 든다. 팩 미설치 또는 기능 꺼짐은 `semantic=null`을 허용하며 기존 후보·v1 DTO/해시는 유지한다. 정확 필드/상태 규칙은 [`DOMAIN-PACK-CONTRACT-V1.md`](./DOMAIN-PACK-CONTRACT-V1.md)에 있다.

## 오류와 마이그레이션

### D1-02 의미 관계명 개정

새 통합 데이터셋의 정식 술어는 `AcceptanceCriterion CHECKED_BY TestCase`다. 이것은 **검토된 테스트 연결**만 뜻한다. 사람의 의미 수용은 `CriterionDecision.approved` 사건에서만 나온다. 현재 실행 중인 의미 투영 스키마 v2는 `ACCEPTED_BY`를 저장하므로, A5.1에서 새 내부 스키마 버전을 도입할 때 기존 v1/v2 투영을 **읽을 때만** 이 연결로 해석하고 새 데이터셋에는 `CHECKED_BY`만 기록한다. 구 투영의 `ACCEPTED_BY`를 사람 승인으로 바꾸거나 기존 바이트/해시를 제자리 수정하지 않는다. 구버전 소비자는 기존 DTO/술어를 계속 읽고, v2 소비자는 정식 이름과 `link_only` 상태를 받는다. 정식 술어가 아닌 `ACCEPTED_BY`의 새 쓰기는 거부한다. 양쪽 방향·원천·검토자 조건은 동일하게 유지한다.

마이그레이션 골든은 구 투영 읽기, 새 투영 쓰기, 구 술어 새 쓰기 거부, 사람 판정 부재 시 `accepted` 오판정 금지, v1 응답/해시 불변을 포함한다. 실행 전에는 기존 `ACCEPTED_BY`만 지원하며 `CHECKED_BY`를 공개 지원한다고 표시하지 않는다.

MCP 도구 실패는 `IsError=true`와 `code=<고정 코드>` 텍스트로 전달한다. v2 도구는 같은 코드를 기계 판독형 오류 데이터에도 기록한다. CLI 실패는 비정상 종료와 `{ "code": "...", "message": "...", "dataset_id": "..." }` 한 객체를 내보낸다. `dataset_id`를 알 수 없으면 생략한다. `requires_v2`, `reindex_required`, `snapshot_mismatch`, `source_missing`은 서로 바꾸어 쓰지 않는다. 오류 메시지에는 원문 본문·비밀·임시 staging 경로를 넣지 않는다.

v1 데이터는 커밋형 읽기 전용 `legacy_unpinned`이며 좌표를 추측해 보충하지 않는다. v2 후보는 옆에 만들고 성공 시 단일 `current` 포인터를 전환한다. 롤백은 대상 버전의 매니페스트·DB 해시·원문 보관본과 바이너리 호환 범위를 먼저 확인한다. 미검증 디렉터리 존재만으로는 롤백 대상이 아니다. 기존 서버는 시작 시 열었던 버전을 계속 서빙하고 새 버전은 재시작 뒤 연다.

## 구현 전·구현 후 골든

1. 현재 v1 인용 JSON 네 필드와 v1 팩 해시를 고정한다. `get_for_task`의 실제 필수 입력은 `prompt`이며 계약 fixture와 등록 도구의 속성/필수 목록을 비교한다.
2. committed v2에서 구 도구의 출력은 v1 DTO와 해시 검증을 유지한다. working-tree/snapshot-only에서 구 도구는 본문을 반환하지 않고 `requires_v2`를 낸다.
3. 세 source mode의 v2 인용·오류·정규화 해시를 별도 골든으로 고정한다. 좌표/본문/semantic 어느 한 바이트를 바꾸어도 검증에 실패한다.
4. v1→v2 재색인, 실패 후보, 롤백, 기존 프로세스와 새 프로세스의 서빙 ID를 재생한다. 빈 좌표의 v1과 v2 근거를 같은 팩에 섞지 않는다.

현재 코드의 `internal/system/mcp/testdata/agent-mcp.schema.json`은 과거 `task` 입력을 적고 있었지만 등록 도구는 `prompt`를 받았다. 설계 점검에서 fixture를 실제 입력으로 고치고 그 필드 이름/필수 집합을 검사하는 회귀 시험을 추가했다. 나머지 v2 골든은 v2 DTO 구현과 함께 채운다.
