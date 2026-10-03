# M2 Max B0 준비 게이트

상태: **환경·선택 입력 진단 완료 / 정답·모델 승인 / 동적 사례·프로토콜 검토 대기 / 품질 미측정** (2026-10-03). 이 문서는 이전 머신의 B0-H 구조 게이트를 재서술하지 않고, 인계 후 새 호스트에서 관측한 입력과 실행을 기록한다. 공식 B0/B1 또는 C 단계 완료 기록이 아니다.

## 환경과 입력

- 브랜치: `feat/spec-driven-knowledge-system`. 시작 시 개발 HEAD와 원격 HEAD는 `9e907b43b8cf038ab60bbc4d0a775c632e4defd6`으로 같았다. 원격 조회는 실제 `git ls-remote` 결과다. 기존 미추적 `.claude/`, `logs/`를 보존했다.
- Apple M2 Max, macOS arm64, 12 CPU, RAM 64 GiB; 점검 시 디스크 여유 약 115 GiB. 메모리 압축과 다른 작업이 존재하므로 총 RAM을 가용 메모리로 표시하지 않는다. [환경·모델 목록·바이너리 해시](../../system/eval/b0-knowledge-system/environment-m2max-2026-10-03.json)를 보존했다.
- Go 1.26.8에서 `make build-bins`로 현재 브랜치의 세 바이너리를 새로 빌드했다. 이전 머신의 바이너리를 재사용하지 않았다.
- Ollama 서버 0.35.1, `bge-m3:latest`, digest `7907646426070047a77226ac3e684fbbe8410524f7b4a74d02837e43f2146bab`, 1024차원. 모델 바이트는 이전 관측과 같지만 서버와 하드웨어는 다르다. 진단을 시작할 때는 후보였으며, 이후 사용자의 실제 응답으로 BGE-M3 선택을 승인받았다. [사람 결정 기록](../../system/eval/b0-knowledge-system/human-review-m2max-2026-10-03.json)에 이전 질문셋 해시·새 해시·검토자와 시각을 기록했다.
- 초기 [preflight](../../system/eval/b0-knowledge-system/preflight-m2max-2026-10-03.json)는 고정 코퍼스의 모든 앵커를 확인했으며 `pending_reasons=[gold_answers_need_human_approval]`, 승인 수 `0/12`, `metrics=null`이다. `--require-ready`의 종료 코드 2는 예상된 승인 게이트 동작이다. 사용자 승인 뒤의 [재점검](../../system/eval/b0-knowledge-system/preflight-m2max-approved-2026-10-03.json)은 `ready`, 승인 수 `12/12`, 종료 코드 0이다. 이 스크립트는 동적 사례·프로토콜의 승인을 검사하지 않으므로 전체 B0 완료로 해석하지 않는다.
- [독립 코퍼스 기록](../../system/eval/b0-knowledge-system/corpus-isolation-m2max-2026-10-03.json): commit `71cb71cd55960833e930269e272f7a4a060be3aa`, tree `f020f8f30fd209b8de045756dedd12ff83f65cd9`, 독립 Git common dir, alternates 없음, 도달 불가 커밋 0개, 깨끗한 checkout. 원본 개발 저장소의 untracked 파일은 코퍼스에 포함되지 않는다.

## 실행 검증

[검사 로그·스모크 요약](../../system/eval/b0-knowledge-system/verification-m2max-2026-10-03.json)을 보존했다.

| 실행 | 관측 | 범위 |
|---|---|---|
| B0 Python 검사 | 9개 통과 | 앵커·승인 전 export 거부·가짜 Ollama·Git 격리 |
| 관련 Go 검사 | 8개 패키지 통과 | setup, vector chunk/build/store, evidencev2, MCP, semantic, graph |
| 작은 실제 Ollama 스모크 | CKV 4개 히트, CKS README/main 인용, 커밋·모델 정합성 통과 | 별도 작은 임시 프로젝트의 API 통합 |
| 긴 원문 실제 Ollama 스모크 | 원문과 두 배 길이 원문 모두 완전 모드 게시 | 6,144바이트 분할과 서버 수용 |
| 문서 검사 | 110개 살아있는 문서 통과 | 검사 시점 CLI·경로 계약; 최종 문서 변경 뒤 다시 확인 |

일반 샌드박스는 Git 쓰기, 로컬 소켓, Go 캐시 및 일부 `sysctl` 접근을 제한했다. 같은 명령을 해당 작업 권한으로 실행해 관측했으며 샌드박스 실패를 제품 결함으로 기록하지 않는다. B0 프로브는 로컬 loopback 서버에만 연결했다.

## 전체 코퍼스 완전 색인 진단

새 바이너리와 후보 BGE-M3로 독립 코퍼스의 전체 선택 입력 색인을 완료했다. 이 실행은 정답·모델 승인 전에 시작했으며 진단 범위를 유지한다. `CKV_REQUIRE_COMPLETE_EMBEDDINGS=1`, `project_id=b0-knowledge-system`, 별도 임시 버전 `m2max-diagnostic`을 사용한다. 정답 점수화와 검색 결과에 따른 튜닝은 하지 않았다.

CKG는 이전 독립 코퍼스와 같은 `graph_digest=fa58e1e7c5ec7ff022459a773d74fee2b9ba40f36f00a3302c2bff24d832d98e`, 102,112노드/414,613엣지, 파싱 오류 0개를 만들었다. 진단용 데이터셋을 공식 B0 품질 기준선으로 승격하지 않는다. [원자료 요약](../../system/eval/b0-knowledge-system/full-corpus-m2max-2026-10-03.json)과 [전체 실행 로그](../../system/eval/b0-knowledge-system/setup-raw-m2max-2026-10-03.txt)를 보존했다.

- 실제 벡터 입력 1,569파일, 13,575청크, 저장 본문 최대 6,144바이트, 축약 표식 0개. 생성된 패키지 요약 경로를 소스 파일 수에 합산하지 않는다.
- 모든 13,575개 청크의 UTF-8 저장 텍스트와 content SHA-256, 커밋, 실제 입력 파일 1,569개의 바이트 해시, DB 해시가 일치했다. 분할 부모 63개/자식 169개의 ordinal과 재결합 바이트가 원문 인용 범위 안에 일치했다.
- `cks doctor --strict`: ready/pinned, dirty=false, history=available. 세 계층의 프로젝트·스냅샷·데이터셋 신원이 일치했다.
- 전체 진단 약 813.42초(13분 33초); 데이터셋 384,780,036바이트(약 367 MiB), Git bundle 36,550,909바이트. 초기 구간에는 별도 Go 검사도 실행됐으므로 이 시간은 통제된 공식 지연 기준선이 아니다.
- **범위 차이:** CKG Go 입력 992개 중 CKV input_files에는 963개가 있다. CKV 기본 `build/` 제외 규칙 때문에 `internal/vector/build/` 아래 실제 소스 29개가 제외됐다. 입력·해시 계약은 선택된 파일이 보관본에서 왔는지 검증하며, CKG/CKV의 전체 파일 집합이 같은지는 강제하지 않는다. 따라서 저장 무결성은 확인됐지만 범위 감사는 `partial`로 기록한다. 엄격 임베딩 성공을 전체 저장소 검색 커버리지의 완료로 주장하지 않는다.
- CKG 검증 오류 0개와 기존 schema 경고 8개가 있었다. canonical coverage 임계 게이트는 기본값 0으로 비활성이다. 이 실행의 CKG go audit PARITY(992/992)는 CKG 빌드/DB의 대조이며 CKV/CKG 파일 일치를 뜻하지 않는다.

## 검토 자료와 남은 선행 조건

[B0-M2MAX-REVIEW.md](./B0-M2MAX-REVIEW.md)에 12개의 정확한 질문/후보 답·고정 원문을 담았다. [부재 후보 검색](../../system/eval/b0-knowledge-system/absence-review-m2max-2026-10-03.json)은 추적 텍스트 1,937개에서 가스 환급 기술 자료 6줄과 서명 관련 185줄을 찾았다. 어휘 검색의 무매칭은 의미적 부재 증명이 아니며, 기술 자료를 조직 승인 담당자나 운영 서명 신원으로 취급하지 않는다.

[동적 fixture 초안](../../system/eval/b0-knowledge-system/dynamic-fixtures-m2max-draft.json)은 F-01–F-06의 개발/최종 변형 총 12개에 소스 바이트·파일 SHA-256·질의·기대 파일/줄·상태를 기록한다. [프로토콜 초안](../../system/eval/b0-knowledge-system/protocol-m2max-draft.json)은 K, 질문 분리, paired arms, 반복·지연·bootstrap·작은 표본 규칙을 제안한다. 둘은 사용자의 “검토 후 결정”에 따라 계속 초안이며 실모델 검색 평가로 실행하지 않았다. 제안된 선택 범위에는 이번에 발견한 기본 제외 규칙을 명시했다.

1. **완료:** 실제 사용자의 12개 정답 승인을 `chat-user`·기록 시각과 함께 저장했다. 이름이 따로 주어지지 않아 이 로컬 감사 ID를 사용하며 인증된 실명으로 주장하지 않는다.
2. **모델 완료 / 동적·프로토콜 대기:** BGE-M3 선택은 사용자 승인됐고, 동적 fixture·프로토콜은 검토 대기다. 29개 소스 제외를 포함한 입력 범위와 판정 규칙을 결정한 뒤 해시를 봉인한다.
3. 동적 fixture의 독립 Git 좌표·canonical ID·검토된 합성 의미 투영·잠금 팩, 예산 주입 및 4개 ablation adapter를 완성하고 실행 신원을 봉인한다. 현 CLI에 없는 개별 개념/관계 arm을 설정만으로 실행했다고 보고하지 않는다.
4. 정답 승인 뒤 공식 시나리오 12개를 새 임시 경로에 내보냈다. export 매니페스트는 승인된 질문셋 해시를 포함하며 후보 답 문장을 MCP 평가 입력에 넣지 않는다. 동적·프로토콜 결정과 준비 뒤 `cks eval --verify-anchors` 및 별도 v2/주장별 판정을 수행한다. 최종 질문은 튜닝에 쓰지 않는다.

정적 최종 8문항과 중요 군별 작은 표본으로 운영 출시 품질을 입증하지 않는다. 제안한 작은 표본 규칙을 승인하면 해당 비교는 `inconclusive`로 기록한다. `natural_language_quality=unmeasured`, `ontology_runtime=disabled`, `release=preview`를 유지한다.
