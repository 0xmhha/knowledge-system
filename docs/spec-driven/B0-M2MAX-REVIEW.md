# M2 Max B0 검토 자료

상태: **정답 12개·BGE-M3 승인 / 동적 사례·프로토콜 검토 대기 / 공식 품질 미측정** (2026-10-03). 사용자가 이 채팅에서 정답 검토에 “승인.”, 모델 선택에 “BGE-M3로 확정 (권고)”라고 답했다. 동적 사례·프로토콜에는 “검토 후 결정”이라고 답했으므로 그 둘은 초안으로 유지한다. [실제 결정 기록](../../system/eval/b0-knowledge-system/human-review-m2max-2026-10-03.json)의 `chat-user`는 이 채팅 사용자를 뜻하는 로컬 감사 ID이며 인증된 실명이라는 주장이 아니다.

## 확인된 실행 입력

- 개발 HEAD: `9e907b43b8cf038ab60bbc4d0a775c632e4defd6` (원격과 일치). 코퍼스: `71cb71cd55960833e930269e272f7a4a060be3aa`, tree `f020f8f30fd209b8de045756dedd12ff83f65cd9`.
- Apple M2 Max, 12 CPU, 64 GiB; 디스크 여유 약 115 GiB. 다른 앱/메모리 압축이 있어 전체 RAM을 여유 메모리로 간주하지 않는다.
- Ollama 서버 0.35.1. BGE-M3는 이전 머신과 동일한 모델 바이트지만 서버/하드웨어가 달라 지연은 다시 측정한다.
- 승인된 기준선 모델: `bge-m3:latest`, digest `7907646426070047a77226ac3e684fbbe8410524f7b4a74d02837e43f2146bab`, 1024차원; `num_ctx=8192`, `num_batch=8192`, 원문 청크 6,144바이트, query prefix `registry`, 완전 임베딩 모드.
- 설치된 대안: Qwen3 embedding 0.6b (639 MB, 1024차원 메타데이터), 4b (2.5 GB, 2560차원 메타데이터). 대안의 차원은 이번 임베딩 프로브로 검증하지 않았다. 자원은 충분하지만 모델을 바꾸면 별도 기준선/입력 정책 검증이 필요하므로 우선 BGE-M3를 권고한다.

## 12개 정답 검토

아래는 정확한 평가 질문과 후보 답이다. 인용은 현재 브랜치가 아닌 고정 코퍼스의 줄이다. 모든 앵커를 기계 확인했고 제시된 코드/문서 범위를 읽었지만, 기계·코드 검토 자체는 사람 승인 기록을 대신하지 않는다. 이번 12개 정답은 이후 실제 사용자 응답에 따라 승인됐다.

| ID / 제안 구분 | 정확한 질문 | 후보 답 / 기대 동작 | 고정 근거 |
|---|---|---|---|
| B0-CODE-01 / 개발 | CKG와 CKV가 서로 다른 소스 스냅샷을 색인했을 때 어디서 거부하나요? | VerifyAlignment이 프로젝트, 스냅샷, 데이터셋, 파일 매니페스트와 캡처 정책의 일치를 확인한다. (`cite`) | internal/setup/verify.go:57–85 |
| B0-CODE-02 / 최종 | Where does CKV report an incomplete search when the exact candidate limit is reached? | SearchDetailed returns an incomplete result with a candidate limit reason instead of treating partial hits as complete. (`cite`) | internal/vector/store/sqlitevec/store.go:613–654 |
| B0-CODE-03 / 최종 | 기존 v1 데이터셋에 v2 문맥 질의를 했을 때 재색인을 요구하는 경계는 어디인가요? | get_for_task_v2 핸들러가 보관 데이터셋의 pinned v2 신원을 서비스 가능성 검사보다 먼저 확인한다. (`cite`) | internal/system/mcp/get_for_task_v2.go:33–55 |
| B0-WHY-01 / 최종 | 왜 테스트가 CHECKED_BY로 연결되어도 수용 기준이 사람에게 승인된 것으로 취급하지 않나요? | CHECKED_BY는 검토된 테스트 연결이며 CriterionDecision.approved가 별도의 사람 수용 판정이다. (`cite`) | docs/spec-driven/PUBLIC-CONTRACT-V2.md:67–67 |
| B0-WHY-02 / 최종 | Why does a project knowledge pack change create a new dataset identity even when code bytes are unchanged? | The build recipe includes the lock digest, so a changed pack or policy becomes a distinct dataset rather than mutating the active one. (`cite`) | docs/spec-driven/DOMAIN-PACK-CONTRACT-V1.md:40–40 |
| B0-WHY-03 / 개발 | 왜 패치의 테스트 실행이 성공해도 자동으로 승격되지 않나요? | 테스트 보고와 사람의 기준별 승인 결정을 분리하며, 패치 승격은 연결된 근거와 모든 필수 기준의 approved 판정을 요구한다. (`cite`) | internal/system/patch/review.go:205–281 |
| B0-POLICY-01 / 개발 | 같은 범위와 시점에 적용되는 검토된 프로젝트 정책 두 개가 충돌하면 어느 쪽을 자동 우선해야 하나요? | 자동 우선순위를 부여하지 않고 충돌을 드러낸다. 승인된 대체·예외 또는 범위·유효 기간이 해소될 때만 결정적 답을 낸다. (`cite`) | docs/spec-driven/DOMAIN-PACK-CONTRACT-V1.md:23–23<br>internal/system/knowledgepack/instances.go:195–242 |
| B0-POLICY-02 / 최종 | May an industry pack override a verified project policy when the two disagree? | No. An industry pack is a reusable template, while the reviewed project overlay is authoritative for that project; disagreement must be recorded or excluded. (`cite`) | docs/spec-driven/DOMAIN-PACK-CONTRACT-V1.md:23–23 |
| B0-POLICY-03 / 최종 | 비공개 정책이 존재하지만 조회자가 접근 권한이 없으면 정책 본문이나 ID를 코딩 문맥에 넣을 수 있나요? | 넣을 수 없다. 비공개 정책은 응답에서 숨기고 불확실 상태를 유지한다. (`cite`) | internal/system/knowledgepack/instances.go:203–241 |
| B0-TRACE-01 / 개발 | What must match before CKS can claim an ADR requirement has a verified code and test implementation path? | The reviewed trace link, current dataset coordinates, retained source references, and semantic code and test anchors must match. (`cite`) | internal/system/evidencev2/knowledge_trace.go:20–115 |
| B0-ABS-01 / 최종 | 이 조직의 운영 블록체인 가스 환불 정책을 승인한 담당자는 누구인가요? | 이 코퍼스에는 승인된 조직 운영 정책과 담당자 근거가 없으므로 답변을 보류해야 한다. (`abstain`) | 아래 부재 검토 참조 |
| B0-ABS-02 / 최종 | Which production signing key and private key holder are used for this project's release? | The corpus does not contain an approved production signing key or holder; do not infer one from preview signing tests. (`abstain`) | 아래 부재 검토 참조 |

기권 문항의 `abstain`은 답변 기권이다. v1의 `expect_no_citations=true`는 **인용도 0개**라는 별도 강한 조건이다. 문맥이 테스트용 서명이라는 근거를 인용하면서 운영 키를 모른다고 답하면 사람의 기권은 맞지만 v1 인용 기권 검사는 실패할 수 있다. 이 차이를 평가 전에 승인해야 한다.

## 두 부재 문항의 검토 근거

고정 코퍼스의 추적 파일 1,937개 모두 UTF-8 텍스트로 검색했다. [검색 패턴·파일·줄·파일 해시 원자료](../../system/eval/b0-knowledge-system/absence-review-m2max-2026-10-03.json)는 재검토 후보 목록이다. 어휘 검색의 무매칭을 의미적 부재 증명으로 취급하지 않는다.

- ABS-01: 가스 환급 관련 6줄은 테스트 주석, Stablenet 기술 흐름, 이전 영향 평가 자료다. 기술상 환급 처리 자료가 존재하므로 “가스 환급 자료가 없다”로 답을 바꾸면 틀린다. **조직 운영 정책을 승인한 담당자의 근거**는 이번 검토에서 확인되지 않았다.
- ABS-02: signing/private-key 관련 185줄은 54개 파일에서 발견됐다. 고정 A8 보고서 24·46줄은 시험 서명 preview와 운영 릴리스 서명 승인을 구분한다. 시험용 키 생성 코드나 CLI의 키 경로 인자를 운영 개인키/보유자라고 추론하지 않는다. 검토자는 원자료의 후보 파일 전체를 확인한 뒤 부재 여부를 결정해야 한다.

## 동적 사례의 고정 제안

[정확한 소스 바이트·질의·파일별 SHA-256·기대 결과](../../system/eval/b0-knowledge-system/dynamic-fixtures-m2max-draft.json)에 각 사례의 개발/최종 변형을 미리 분리했다. 모두 합성 코퍼스이며 파일럿 수치와 합산하지 않는다. 아직 어떤 변형도 실모델 검색 평가로 실행하지 않았다.

| 사례 | 고정 입력 / 질의 | 기대 결과 | 실행 전 남은 작업 |
|---|---|---|---|
| F-01 | Go 함수 60개 중 단일 깊이 경로의 5개만 대상; “Where is the gas refund route implemented?”; Go·Function·symbol·경로·테스트 제외 결합, K=5 | 5개 대상 회수; 예산 2의 별도 직접 API 실행은 incomplete/candidate_limit | 실제 벡터 exact 오라클·예산 주입 runner. 무필터 top-5가 대상을 빠뜨리지 않으면 fixture_not_qualified로 기록 |
| F-02 | 200줄 반복 문단 뒤 203줄의 정족수만 정답; 한영 질문 | 개발 seven / 최종 eleven; 203줄 인용·부모 좌표 보존·원문 재결합·축약 0 | 원문·자식 범위와 해시 검증 runner |
| F-03 | 고정 ontology에서 project/dataset에 같은 한영 용어; 별도의 미등록 표현 | 복수 개념 유지·미등록은 개념 후보 없음·기본 상위 K 집합 보존 | 별도 snapshot에 묶인 합성 의미 투영 검토와 ablation adapters |
| F-04 | 같은 Alpha 함수의 old/new 토큰 두 커밋 | 현재는 새 커밋/본문; 과거 조회는 보관 원문; 혼합 좌표 거부; 직접 CKV stale 경고 구분 | 두 Git commit/tree 및 불변 데이터셋 좌표 고정 |
| F-05 | 상대 경로·Alpha 이름이 같은 A/B; 서로 다른 코드 표식과 환급 한도 10/20 | 양방향 다른 프로젝트 인용/본문 0; 혼합 정렬 거부 | 독립 Git 저장소와 프로젝트 ID; 선택 팩 실행 전 fixture 내부 overlay 잠금·검토 필요 |
| F-06 | 명세 ≤10, 코드 20, 무관 TestHealth 통과; 한영 질문 | 명세와 코드 모두 회수, 사람은 충돌 판정; 테스트 통과를 수용 승인으로 주장 0 | CKG canonical ID·요구/기준·추적 원문 검토. v2가 임의 수치 충돌을 자동 판정한다고 가정하지 않음 |

## 전체 진단에서 확인한 검색 범위

선택된 1,569파일/13,575청크의 완전 임베딩은 성공했고 저장 청크 해시·분할 원문은 검증됐다. 그러나 CKV 기본 `build/` 제외 규칙이 `internal/vector/build/`의 실제 Go 소스 29개를 제외한다. CKG의 Go 입력 992개 중 CKV input_files에 있는 것은 963개다. `cks doctor`의 ready/pinned와 엄격 임베딩 성공을 전체 저장소 검색 범위의 완전성으로 해석하지 않는다. [정확한 누락 경로와 감사 결과](../../system/eval/b0-knowledge-system/full-corpus-m2max-2026-10-03.json)를 프로토콜 검토에 포함한다.

## 반복·표본·판정 권고

[프로토콜 초안](../../system/eval/b0-knowledge-system/protocol-m2max-draft.json)을 결과 확인 전에 승인해 해시로 봉인한다.

- 정적 개발 4개(CODE-01, WHY-03, POLICY-01, TRACE-01), 최종 8개; 동적 6개 각각 개발/최종 별도 변형. 최종 입력은 튜닝에 사용하지 않는다.
- 정적 K=10, 필터 없음; F-01만 K=5/고정 결합 필터. 기본·개념 텍스트·관계·결합 × 팩 없음/있음 8개 paired arm. source/model/query/K/filter와 팩별 잠금을 기록한다.
- 인용 검색은 문항별 5회(현재 exporter 설정과 같음). 지연은 arm별 2회 warmup 제외 후 20회; p95는 nearest-rank. cold는 새 CKS 프로세스 3회이며 모델 상주 여부를 명시한다. 사용자 모델을 임의로 unload하지 않는다.
- 기존 설계 임계치 유지: 혼입 0, 거짓 인용 증가 없음, 전체 Recall@10/MRR 변화 ≥−0.02, 중요 질문군 ≥−0.05, warm p95 ≤기본 1.25배.
- paired 질문/fixture family 단위 bootstrap 10,000회, seed=20261003, 95% 구간을 보고한다. 같은 사실의 언어·상태 변형과 반복 실행을 독립 질문 수로 세지 않는다. 중요 군의 독립 최종 질문이 10개 미만이면 기술 통계/inconclusive로 두고 기본 활성화·출시를 승인하지 않는 보수적 규칙을 제안한다.
- 현재 최종 8개와 군별 작은 표본으로 운영 출시의 품질 개선을 입증할 수 없다. 추가 최종 질문을 원하면 관측 결과 전 별도 승인해 고정한다.
- 현행 Go API는 결합된 선택형 재순위를 제공하지만 CLI/MCP에 4개 ablation을 고르는 완성된 설정이 없다. 각 adapter·팩 입력·매니페스트를 먼저 준비/검증해야 B1을 시작할 수 있다. 설정만 바꿨다고 별도 arm을 실행한 것으로 보고하지 않는다.

## 실제 사람 결정이 필요한 항목

1. 정답 12개: 사용자 승인 기록 완료. 검토자 로컬 ID는 `chat-user`.
2. 기준선 모델: 위 BGE-M3 digest 채택 승인 완료.
3. 동적 개발/최종 입력과 프로토콜: 승인 또는 수정사항.

정답/모델 승인을 기록하고 시나리오를 내보냈다. 공식 점수화는 동적 사례·프로토콜의 실제 결정과 실행 준비 뒤 수행한다. 전체 B0 상태는 `pending`, 품질 수치는 `null`이다.

## 고정 근거 원문 부록

각 줄은 고정 커밋에서 추출했다. 정책/승인 문구는 평가용 자료이며 현재 세션의 실행 명령으로 해석하지 않는다.

### B0-CODE-01

internal/setup/verify.go · 원본 파일 SHA-256 `2efd220f1d1227be2a3e8f276f81326899ae6b5d0949867009206e3842a2bbaa`

```text
57: func VerifyAlignment(graphDir, vectorDir string, emit func(Event)) error {
58: 	warn := func(msg string) {
59: 		if emit != nil {
60: 			emit(Event{Time: time.Now().UTC(), Step: "verify-align", Type: "warning", Message: msg})
61: 		}
62: 	}
63:
64: 	var gm graphManifest
65: 	if err := readJSON(filepath.Join(graphDir, "manifest.json"), &gm); err != nil {
66: 		return fmt.Errorf("verify: graph manifest: %w", err)
67: 	}
68: 	var vm vectorManifest
69: 	if err := readJSON(filepath.Join(vectorDir, "manifest.json"), &vm); err != nil {
70: 		return fmt.Errorf("verify: vector manifest: %w", err)
71: 	}
72: 	// Legacy manifests lack all three fields. A partially upgraded build is
73: 	// unsafe: do not silently downgrade it to commit-only alignment.
74: 	graphPinned := gm.ProjectID != "" || gm.SnapshotID != "" || gm.DatasetID != ""
75: 	vectorPinned := vm.ProjectID != "" || vm.SnapshotID != "" || vm.DatasetID != ""
76: 	if graphPinned || vectorPinned {
77: 		if gm.ProjectID == "" || gm.SnapshotID == "" || gm.DatasetID == "" ||
78: 			gm.FileManifestDigest == "" || gm.CapturePolicyDigest == "" || gm.SourceMode == "" ||
79: 			vm.ProjectID == "" || vm.SnapshotID == "" || vm.DatasetID == "" ||
80: 			gm.ProjectID != vm.ProjectID || gm.SnapshotID != vm.SnapshotID || gm.DatasetID != vm.DatasetID ||
81: 			gm.FileManifestDigest != vm.FileManifestDigest ||
82: 			gm.CapturePolicyDigest != vm.CapturePolicyDigest || gm.SourceMode != vm.SourceMode {
83: 			return fmt.Errorf("verify: project/snapshot/dataset identity missing or mismatched across graph and vector")
84: 		}
85: 	}
```

### B0-CODE-02

internal/vector/store/sqlitevec/store.go · 원본 파일 SHA-256 `bb9a0da336806f3a587e52a0950220e759801f6e3a2a28420d5b8efdf156bc71`

```text
613: func (s *Store) SearchDetailed(ctx context.Context, query []float32, k int, filter types.Filter, opts SearchOptions) (SearchResult, error) {
614: 	if err := ctx.Err(); err != nil {
615: 		return interruptedSearch(err), nil
616: 	}
617: 	if got := len(query); got != s.dim {
618: 		return SearchResult{}, fmt.Errorf("sqlitevec: query dim %d != store dim %d", got, s.dim)
619: 	}
620: 	if k <= 0 {
621: 		return SearchResult{Status: SearchComplete}, nil
622: 	}
623: 	if k > DefaultMaxSearchK {
624: 		return SearchResult{Status: SearchIncomplete, Reason: "requested_k_exceeds_limit"}, nil
625: 	}
626: 	maxExact := opts.MaxExactCandidates
627: 	if maxExact <= 0 {
628: 		maxExact = DefaultMaxExactCandidates
629: 	}
630: 	result := SearchResult{Status: SearchComplete}
631: 	if !filter.IsZero() {
632: 		count, err := s.candidateCount(ctx, filter)
633: 		if err != nil {
634: 			if ctx.Err() != nil {
635: 				return interruptedSearch(ctx.Err()), nil
636: 			}
637: 			return SearchResult{}, err
638: 		}
639: 		result.CandidateCount = count
640: 		if count <= 2048 {
641: 			if count > maxExact {
642: 				result.Status, result.Reason = SearchIncomplete, "candidate_limit"
643: 				return result, nil
644: 			}
645: 			var eligible int
646: 			hits, err := s.searchExact(ctx, query, k, filter, &eligible)
647: 			if err != nil {
648: 				if ctx.Err() != nil {
649: 					return interruptedSearch(ctx.Err()), nil
650: 				}
651: 				return SearchResult{}, err
652: 			}
653: 			result.Hits, result.EligibleCount, result.SearchedCount = hits, &eligible, eligible
654: 			return result, nil
```

### B0-CODE-03

internal/system/mcp/get_for_task_v2.go · 원본 파일 SHA-256 `eaae91fb9a383fb7dba0c159f01cb8a000d943538fb5e155a14270b10174e04a`

```text
33: func handleGetForTaskV2(ctx context.Context, d Deps, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
34: 	if d.EvidenceVersionDir == "" || d.EvidenceSanitizer == nil {
35: 		return v2ToolError("reindex_required"), nil
36: 	}
37: 	prompt := req.GetString("prompt", "")
38: 	if prompt == "" {
39: 		return v2ToolError("invalid_request"), nil
40: 	}
41: 	callerIntent := contract.IntentUnknown
42: 	if raw := req.GetString("intent", ""); raw != "" {
43: 		parsed, ok := contract.ParseIntent(raw)
44: 		if !ok {
45: 			return v2ToolError("invalid_request"), nil
46: 		}
47: 		callerIntent = parsed
48: 	}
49: 	identity, identityErr := setup.InspectVersionIdentity(d.EvidenceVersionDir)
50: 	if identityErr != nil || identity == nil {
51: 		return v2ToolError("reindex_required"), nil
52: 	}
53: 	if ok, _ := serviceable(ctx, d); !ok {
54: 		return v2ToolError("service_unavailable"), nil
55: 	}
```

### B0-WHY-01

docs/spec-driven/PUBLIC-CONTRACT-V2.md · 원본 파일 SHA-256 `8de59a63c70b003cbf018fcedd6c5c0821d157ac4566ca8ca9900bcf1518b569`

```text
67: 새 통합 데이터셋의 정식 술어는 `AcceptanceCriterion CHECKED_BY TestCase`다. 이것은 **검토된 테스트 연결**만 뜻한다. 사람의 의미 수용은 `CriterionDecision.approved` 사건에서만 나온다. 현재 새 의미 투영 스키마 v3는 `CHECKED_BY`를 기록하고 기존 v1/v2 투영의 `ACCEPTED_BY`는 **읽을 때만** 연결로 해석한다. 구 투영의 `ACCEPTED_BY`를 사람 승인으로 바꾸거나 기존 바이트/해시를 제자리 수정하지 않는다. 구버전 소비자는 기존 DTO/술어를 계속 읽고, v2 소비자는 정식 이름과 `link_only` 상태를 받는다. 정식 술어가 아닌 `ACCEPTED_BY`의 새 쓰기는 거부한다. 양쪽 방향·원천·검토자 조건은 동일하게 유지한다.
```

### B0-WHY-02

docs/spec-driven/DOMAIN-PACK-CONTRACT-V1.md · 원본 파일 SHA-256 `2ebf644aeb5c385795267390bd2e540f52b7cdbf42338d38af0f2d085d36e779`

```text
40: 잠금 파일은 `project_id`, `pack_schema_version`, 정렬된 `{pack_id, version, digest, dependencies, origin_id}`와 `overlay_digest`를 담고, 정규화된 바이트의 SHA-256으로 자체 `lock_digest`를 만든다. 자가 참조를 피하기 위해 `lock_digest` 필드는 해시 입력에서 제외한다. **`overlay_digest`의 입력에서는 `knowledge.lock.json` 전체를 제외**하고, `manifest.yaml` 및 `domain/`, `policies/`, `decisions/`, `questions/`의 등록된 일반 파일을 `(origin_id,path)` 순서로 해시한다. 각 파일 레코드는 A4의 상대 경로 정규화·바이트 SHA-256 규칙을 따르며 임시/숨김/미등록 파일을 묵인하지 않는다. `manifest.yaml`의 `source`는 저장소 루트 기준 상대 경로 또는 별도 등록한 로컬 루트의 `origin_id` 참조이며, 절대 경로·심볼릭 링크·루트 탈출은 거부한다. 외부 팩의 정확한 파일 목록과 다이제스트는 A4 원천 매니페스트에도 포함한다. `build_recipe_digest`는 `lock_digest`, resolver/renderer 버전과 관계 검증기 버전을 포함한다. 그러므로 같은 코드라도 팩이나 프로젝트 정책 바이트가 바뀌면 새 `dataset_id`가 된다. 기존 데이터셋을 제자리 수정하지 않는다.
```

### B0-WHY-03

internal/system/patch/review.go · 원본 파일 SHA-256 `343d29f9ed725906b29bc9feb28ecd8eb9470e40af1743f31efb054f2de5350f`

```text
205: func Promote(ctx context.Context, dataset, patchID, storePath string) (string, error) {
206: 	a, err := Load(dataset, patchID)
207: 	if err != nil {
208: 		return "", err
209: 	}
210: 	current, err := os.Readlink(filepath.Join(dataset, "current"))
211: 	if err != nil || current != a.BaseVersion {
212: 		return "", fmt.Errorf("patch base is no longer current")
213: 	}
214: 	vdir := filepath.Join(dataset, a.ResultVersion)
215: 	if err := verifyTestGate(vdir, a); err != nil {
216: 		return "", err
217: 	}
218: 	p, err := CandidateProjection(ctx, dataset, a, storePath)
219: 	if err != nil {
220: 		return "", err
221: 	}
222: 	trace, err := p.Trace()
223: 	if err != nil {
224: 		return "", err
225: 	}
226: 	if len(trace.Requirements) == 0 {
227: 		return "", fmt.Errorf("patch candidate has no reviewed requirements")
228: 	}
229: 	for _, r := range trace.Requirements {
230: 		if r.State != semantic.TraceLinked {
231: 			return "", fmt.Errorf("requirement %q is %s, not linked", r.RequirementID, r.State)
232: 		}
233: 	}
234: 	decisions, err := readDecisions(dataset, patchID)
235: 	if err != nil {
236: 		return "", err
237: 	}
238: 	byCriterion := map[string]Decision{}
239: 	for _, d := range decisions {
240: 		if d.PatchID != patchID || d.ProjectID != a.ProjectID || d.DatasetID != a.ResultDatasetID || d.SnapshotID != a.ResultSnapshotID ||
241: 			byCriterion[d.CriterionID].DecisionID != "" || d.Outcome != "approved" || d.ReviewedBy == "" || d.Reason == "" {
242: 			return "", fmt.Errorf("patch has rejected, duplicate, or foreign criterion decision")
243: 		}
244: 		byCriterion[d.CriterionID] = d
245: 	}
246: 	criteria := 0
247: 	for _, r := range p.Requirements {
248: 		for _, c := range r.AcceptanceCriteria {
249: 			criteria++
250: 			d, ok := byCriterion[c.ID]
251: 			if !ok || d.SpecVersion != r.Version {
252: 				return "", fmt.Errorf("criterion %q lacks current human approval", c.ID)
253: 			}
254: 			evidenceSHA := ""
255: 			for _, e := range p.Evidence {
256: 				if e.ID == c.EvidenceID {
257: 					evidenceSHA = e.ContentSHA256
258: 					break
259: 				}
260: 			}
261: 			if evidenceSHA == "" || d.EvidenceSHA256 != evidenceSHA {
262: 				return "", fmt.Errorf("criterion %q approval evidence differs", c.ID)
263: 			}
264: 			run, err := loadRun(dataset, patchID, c.ID)
265: 			if err != nil {
266: 				return "", err
267: 			}
268: 			if err := validateRun(p, c.ID, run); err != nil {
269: 				return "", err
270: 			}
271: 		}
272: 	}
273: 	if len(byCriterion) != criteria {
274: 		return "", fmt.Errorf("patch has decisions for foreign criteria")
275: 	}
276: 	data, err := json.Marshal(decisions)
277: 	if err != nil {
278: 		return "", err
279: 	}
280: 	sum := sha256.Sum256(data)
281: 	return setup.PromoteReviewedCandidateIfBase(dataset, a.ResultVersion, a.BaseVersion, patchID, hex.EncodeToString(sum[:]))
```

### B0-POLICY-01

docs/spec-driven/DOMAIN-PACK-CONTRACT-V1.md · 원본 파일 SHA-256 `2ebf644aeb5c385795267390bd2e540f52b7cdbf42338d38af0f2d085d36e779`

```text
23: 산업 템플릿에는 조직 정책의 최종 권위가 없다. 특정 프로젝트의 `verified` 정책이 산업 팩의 일반 설명과 충돌하면 산업 설명으로 정책을 덮어쓰지 않고 충돌 또는 적용 제외를 기록한다. **검토된 프로젝트 정책이라도** 다른 검토된 프로젝트 정책과 충돌하면 자동 우선순위를 부여하지 않는다. 적용 범위·유효 기간·승인된 대체/예외가 모호성을 해소할 때만 결정적 답을 낸다.
```

internal/system/knowledgepack/instances.go · 원본 파일 SHA-256 `931cff918a97e2a2a50008dcfdeb0ad571e901098501fb34d52ba26bed296731`

```text
195: func (i Instances) SelectPolicies(asOf string, queryScope map[string]string, allowRestricted bool) (PolicyContext, error) {
196: 	if !validDate(asOf) {
197: 		return PolicyContext{}, fmt.Errorf("invalid policy query date")
198: 	}
199: 	result := PolicyContext{State: "unknown", Applicable: []PolicySummary{}, Conflicts: []Conflict{}, Unknowns: []string{}}
200: 	current := map[string]bool{}
201: 	restricted := false
202: 	stale := false
203: 	for _, p := range i.Policies {
204: 		if !policyScopeMatches(p.Scope, queryScope) {
205: 			continue
206: 		}
207: 		if p.Status != "verified" {
208: 			continue
209: 		}
210: 		if asOf < p.EffectiveFrom || p.EffectiveTo != "" && asOf > p.EffectiveTo {
211: 			stale = true
212: 			continue
213: 		}
214: 		if p.Visibility == "restricted" && !allowRestricted {
215: 			restricted = true
216: 			continue
217: 		}
218: 		current[p.ID] = true
219: 		result.Applicable = append(result.Applicable, PolicySummary{ID: p.ID, State: "current", SourceRef: p.SourceRef,
220: 			ReviewedBy: p.ReviewedBy, EffectiveFrom: p.EffectiveFrom, EffectiveTo: p.EffectiveTo})
221: 	}
222: 	for _, conflict := range i.Conflicts {
223: 		if current[conflict.LeftID] && current[conflict.RightID] {
224: 			result.Conflicts = append(result.Conflicts, conflict)
225: 		}
226: 	}
227: 	switch {
228: 	case restricted:
229: 		result.State = "restricted"
230: 		result.Applicable = nil
231: 		result.Conflicts = nil
232: 	case len(result.Conflicts) > 0:
233: 		result.State = "conflict"
234: 	case len(result.Applicable) > 0:
235: 		result.State = "needs_citation"
236: 	case stale:
237: 		result.State = "stale"
238: 		result.Unknowns = append(result.Unknowns, "no_current_policy")
239: 	default:
240: 		result.Unknowns = append(result.Unknowns, "no_reviewed_applicable_policy")
241: 	}
242: 	return result, nil
```

### B0-POLICY-02

docs/spec-driven/DOMAIN-PACK-CONTRACT-V1.md · 원본 파일 SHA-256 `2ebf644aeb5c385795267390bd2e540f52b7cdbf42338d38af0f2d085d36e779`

```text
23: 산업 템플릿에는 조직 정책의 최종 권위가 없다. 특정 프로젝트의 `verified` 정책이 산업 팩의 일반 설명과 충돌하면 산업 설명으로 정책을 덮어쓰지 않고 충돌 또는 적용 제외를 기록한다. **검토된 프로젝트 정책이라도** 다른 검토된 프로젝트 정책과 충돌하면 자동 우선순위를 부여하지 않는다. 적용 범위·유효 기간·승인된 대체/예외가 모호성을 해소할 때만 결정적 답을 낸다.
```

### B0-POLICY-03

internal/system/knowledgepack/instances.go · 원본 파일 SHA-256 `931cff918a97e2a2a50008dcfdeb0ad571e901098501fb34d52ba26bed296731`

```text
203: 	for _, p := range i.Policies {
204: 		if !policyScopeMatches(p.Scope, queryScope) {
205: 			continue
206: 		}
207: 		if p.Status != "verified" {
208: 			continue
209: 		}
210: 		if asOf < p.EffectiveFrom || p.EffectiveTo != "" && asOf > p.EffectiveTo {
211: 			stale = true
212: 			continue
213: 		}
214: 		if p.Visibility == "restricted" && !allowRestricted {
215: 			restricted = true
216: 			continue
217: 		}
218: 		current[p.ID] = true
219: 		result.Applicable = append(result.Applicable, PolicySummary{ID: p.ID, State: "current", SourceRef: p.SourceRef,
220: 			ReviewedBy: p.ReviewedBy, EffectiveFrom: p.EffectiveFrom, EffectiveTo: p.EffectiveTo})
221: 	}
222: 	for _, conflict := range i.Conflicts {
223: 		if current[conflict.LeftID] && current[conflict.RightID] {
224: 			result.Conflicts = append(result.Conflicts, conflict)
225: 		}
226: 	}
227: 	switch {
228: 	case restricted:
229: 		result.State = "restricted"
230: 		result.Applicable = nil
231: 		result.Conflicts = nil
232: 	case len(result.Conflicts) > 0:
233: 		result.State = "conflict"
234: 	case len(result.Applicable) > 0:
235: 		result.State = "needs_citation"
236: 	case stale:
237: 		result.State = "stale"
238: 		result.Unknowns = append(result.Unknowns, "no_current_policy")
239: 	default:
240: 		result.Unknowns = append(result.Unknowns, "no_reviewed_applicable_policy")
241: 	}
```

### B0-TRACE-01

internal/system/evidencev2/knowledge_trace.go · 원본 파일 SHA-256 `6e80ee9a2358394db034580e5faa8bef558f82e700376cea462c007fa450b5eb`

```text
20: func AttachVerifiedTraces(ctx context.Context, base contract.EvidencePackV2, versionDir, asOf, subsystem string, projection semantic.ActiveProjection, cleaner *sanitize.Engine) (contract.EvidencePackV2, error) {
21: 	if err := Verify(base); err != nil {
22: 		return contract.EvidencePackV2{}, err
23: 	}
24: 	k, ok := base.Semantic.(contract.KnowledgeSemanticV2)
25: 	if !ok || cleaner == nil || asOf == "" || subsystem == "" {
26: 		return base, nil
27: 	}
28: 	k, err := cloneKnowledgeSemantic(k)
29: 	if err != nil {
30: 		return contract.EvidencePackV2{}, err
31: 	}
32: 	if k.KnowledgeContext.State == "restricted" || k.KnowledgeContext.State == "conflict" || k.KnowledgeContext.State == "unavailable" {
33: 		return base, nil
34: 	}
35: 	if snapshot := projection.Snapshot(); snapshot.ProjectID != base.Coordinates.ProjectID || snapshot.DatasetID != base.Coordinates.DatasetID || snapshot.SnapshotID != base.Coordinates.SnapshotID {
36: 		return traceUnknown(base, "trace_snapshot_mismatch")
37: 	}
38: 	instances, lock, err := knowledgepack.LoadRetainedProjectInstances(versionDir)
39: 	if err != nil || lock.LockDigest != k.KnowledgeContext.LockDigest {
40: 		return traceUnknown(base, "trace_archive_unavailable")
41: 	}
42: 	decisions, err := instances.SelectDecisions(asOf, map[string]string{"subsystem": subsystem}, false)
43: 	if err != nil || decisions.State == "restricted" {
44: 		return traceUnknown(base, "trace_scope_unavailable")
45: 	}
46: 	selected := map[string]knowledgepack.DecisionSummary{}
47: 	for _, decision := range decisions.Applicable {
48: 		selected[decision.ID] = decision
49: 	}
50: 	links := append([]knowledgepack.TraceLink{}, instances.TraceLinks...)
51: 	sort.Slice(links, func(i, j int) bool { return links[i].ID < links[j].ID })
52: 	type resolved struct {
53: 		link    knowledgepack.TraceLink
54: 		anchors semantic.TraceAnchors
55: 		refs    [5]Ref
56: 	}
57: 	chosen := []resolved{}
58: 	unverified := false
59: 	for _, link := range links {
60: 		if link.Status != "verified" || link.Visibility != "public" {
61: 			continue
62: 		}
63: 		decision, exists := selected[link.DecisionID]
64: 		if !exists || !containsString(decision.RequirementIDs, link.RequirementID) {
65: 			unverified = true
66: 			continue
67: 		}
68: 		anchors, err := projection.ResolveTracePath(link.RequirementID, link.CriterionID, link.CodeCanonicalID, link.TestCanonicalID)
69: 		if err != nil {
70: 			unverified = true
71: 			continue
72: 		}
73: 		linkRef, err := traceSourceRef(versionDir, link.SourceRef)
74: 		if err != nil {
75: 			unverified = true
76: 			continue
77: 		}
78: 		chosen = append(chosen, resolved{link: link, anchors: anchors,
79: 			refs: [5]Ref{linkRef, traceSpanRef(anchors.Requirement), traceSpanRef(anchors.Criterion), traceSpanRef(anchors.Code), traceSpanRef(anchors.Test)}})
80: 	}
81: 	if len(chosen) == 0 {
82: 		if unverified {
83: 			return traceUnknown(base, "trace_anchor_unverified")
84: 		}
85: 		return base, nil
86: 	}
87: 	if len(chosen) > 2 || len(base.Citations)+5*len(chosen) > 12 {
88: 		return traceUnknown(base, "trace_citation_budget")
89: 	}
90: 	refs := make([]Ref, 0, 5*len(chosen))
91: 	for _, item := range chosen {
92: 		refs = append(refs, item.refs[:]...)
93: 	}
94: 	addition, err := BuildFromRefs(ctx, versionDir, base.Query, refs, cleaner)
95: 	if err != nil || addition.Coordinates != base.Coordinates {
96: 		return traceUnknown(base, "trace_citation_unavailable")
97: 	}
98: 	all := append(append([]contract.CitationV2{}, base.Citations...), addition.Citations...)
99: 	availableBodies := map[contract.CitationV2]bool{}
100: 	for _, body := range append(append([]contract.BodyV2{}, base.Bodies...), addition.Bodies...) {
101: 		availableBodies[body.Citation] = true
102: 	}
103: 	claims := make([]contract.KnowledgeTraceLinkV2, 0, len(chosen))
104: 	for _, item := range chosen {
105: 		citations := [5]contract.CitationV2{}
106: 		for index, ref := range item.refs {
107: 			match := false
108: 			for _, citation := range all {
109: 				if citation.OriginID != ref.OriginID || citation.File != ref.Citation.File || citation.StartLine != ref.Citation.StartLine || citation.EndLine != ref.Citation.EndLine || !availableBodies[citation] {
110: 					continue
111: 				}
112: 				if index > 0 && citation.ContentSHA256 != []semantic.EvidenceSpan{item.anchors.Requirement, item.anchors.Criterion, item.anchors.Code, item.anchors.Test}[index-1].ContentSHA256 {
113: 					continue
114: 				}
115: 				citations[index], match = citation, true
```
