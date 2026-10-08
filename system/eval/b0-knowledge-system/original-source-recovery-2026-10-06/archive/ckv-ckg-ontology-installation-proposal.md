# CKV·CKG 개선, 온톨로지 기반 개발 흐름, 범용 설치형 CKS 제안

작성 기준: 2026-09-29. 대상 소스: `knowledge-system`의 `main` 커밋 `1ded9b3e47bc2e09062dba329c2f918423746fa4`. 이 문서는 분석·설계 제안이며 제품 코드는 수정하지 않았다. 첨부 『데이터베이스 설계와 구축』의 PDF 페이지는 뷰어에서 1부터 센 번호다. OCR 전사본 `database-design-and-build-ocr.md`(로컬 자료)은 자동 추출물이므로 도표·수식·제품별 수치를 원본과 구별한다. 특히 PDF의 예시 성능 수치는 이 프로젝트의 성능 증거가 아니다.

## PDF 원문 대조와 설계 연결

다음의 짧은 발췌는 원본 PDF(로컬 제공 자료)의 해당 페이지 이미지를 눈으로 대조했다. 발췌 외 설명은 원문 내용을 요약한 것이며, 오른쪽은 이 프로젝트에 대한 **우리의 설계 판단**이다. PDF의 사례나 권고를 현행 코드의 기능 또는 실측 성능으로 읽어서는 안 된다. 페이지 번호는 인쇄 면수가 아니라 PDF 뷰어의 1부터 시작하는 번호다.

| PDF 페이지와 짧은 원문 | 해당 페이지의 실제 논지 | knowledge-system에 적용하는 판단 |
|---|---|---|
| 57쪽: “동일한 임베딩 모델” | 같은 벡터 공간에서 비교하려면 차원, 모델, 처리 방식의 일관성을 유지한다. | CKV가 이미 저장하는 `EmbeddingIdentity`를 색인·질의·마이그레이션 게이트에도 적용하고, 다른 정체성의 벡터 혼합을 막는다. |
| 79쪽: “부모 청크(Chapter)와 자식 청크(Phrase)” | 문서 구조에 따라 큰 맥락과 작은 검색 단위를 분리할 수 있으며, 메타데이터는 검색 범위와 결과 품질에 관여한다. | 긴 Markdown은 제목·단락의 자식 청크를 검색하고 부모 문맥 및 정확한 원문 줄 범위를 인용한다. 책의 Chapter/Phrase 예시를 코드 문서에 그대로 강제하지 않는다. |
| 95쪽: “모르겠습니다.” | 관련 근거가 없거나 결론을 낼 수 없는 경우 답변을 꾸며내지 말아야 한다는 예시다. | CKS 평가에 답 없는 질문과 근거 부족 질문을 구분하고 기권 정확도를 추가한다. |
| 134쪽: 후필터링 설명 | 먼저 벡터 후보를 고른 뒤 메타데이터를 걸러내면 원하는 개수보다 적은 결과가 남을 수 있다. | CKV 일반 필터의 고정 3배 후보 방식에 대해 필터된 정확 검색을 오라클로 삼아 결과 개수와 Recall@K를 검증한다. |
| 154쪽: “엣지는 방향성을 가지며” | 그래프 관계는 이름, 방향, 속성을 갖는다는 설명이다. | `IMPLEMENTED_BY`, `VALIDATED_BY` 등은 시작·끝 타입과 방향, 근거 범위를 명시한다. 단순 문자열 연결을 검증된 사실로 취급하지 않는다. |
| 172쪽: “개념(Concept)을 명확히 정의하는 것이다.” | 온톨로지에서는 이름 자체보다 개념의 경계와 뜻을 먼저 정하는 것이 중요하다. | 안정적인 `concept_id`, 정의문, 포함·제외 기준을 코드 앵커보다 먼저 작성한다. |
| 179쪽: “용어 정의는 문장으로 쓴다.” | 공식 용어 합의와 문장형 정의를 매핑보다 앞세운다. | 다국어 별칭은 검색에 쓰되, 개념의 정체성과 검증 관계는 별도 필드로 관리한다. |
| 180·182쪽: “Core는 단순해야 한다.” | 핵심 구조를 작게 유지하고 정책을 구조와 분리하며 변경·롤백을 가능하게 하는 설명이다. | 공통 메타 모델은 작게 시작하고 정책 정의·판정은 별도 버전으로 둔다. 각 데이터셋은 온톨로지/정책 버전을 기록한다. |

이에 연결된 구현 계약과 검증 게이트는 [요구사항 명세](./spec-driven-requirements.md), [기술 설계](./spec-driven-architecture.md), [개발 WBS](./spec-driven-wbs.md)에 기록한다.

## 판단 요약

1. **CKV:** 기존 심볼·문서 청킹, 임베딩 정체성, BM25 재순위, 출처 관리, 평가 도구를 활용한다. 일반 메타데이터의 `KNN → 3배 후보 → 후필터`가 선택도가 높은 필터에서 정답을 놓칠 수 있고, 긴 Markdown 섹션의 끝이 잘릴 수 있다. 필터 계획과 계층 청킹을 우선 실험한다.
2. **CKG:** AST 구조 그래프와 시간·정책 관계는 좋은 기반이다. 감사의 역사 `Hunk` 오집계는 먼저 고친다. 이후 `Document/Section/Claim/Evidence`와 검증된 의미 관계를 코어 코드 그래프에 무차별적으로 합치기보다 CKS 소유의 의미 계층으로 도입한다.
3. **CKS:** 온톨로지는 CKV의 자연어 후보 회수와 CKG의 구조 탐색을 아우르는 **공유 의미 계약**이다. 개념을 질의의 사전 차단 필터로 쓰지 않고, 검색 확장·재순위·주장 검증에 쓴다.
4. **개발 방법:** 모델 기반, 스펙 기반, 지식 데이터 기반 개발의 결합은 가능하다. 다만 이는 현재 구현된 단일 알고리즘이나 검증된 표준 명칭이 아니라, `의도·명세 → 근거 검색 → 추적성 검사 → 변경 제안/구현 → 테스트·근거 검증 → 색인 갱신`이라는 **제안 워크플로**다. 현 CKS는 근거를 제공하지만 일반 코드 변경을 자율적으로 수행하는 엔진은 아니다.
5. **설치형:** 현재 `cks setup`, `mcp gen-config`, 버전별 승격·롤백과 세 바이너리 빌드가 출발점이다. 임의 저장소 지원에는 자동 탐지, 프로젝트 격리, 언어별 기능 선언, 임베더 준비, 배포 패키지, 스냅샷 정합성, 기존 저장소용 온보딩이 추가로 필요하다. 모든 언어에 같은 AST 품질을 보장할 수는 없다.

## 1. PDF 기준과 현재 구현의 간극

| PDF 근거 | 이미 구현된 토대 | 개선·추가 제안 | 우선순위와 검증 |
|---|---|---|---|
| 49–57, 79–85쪽: 원문·벡터·메타데이터와 컬렉션 경계 | [Chunk](https://github.com/0xmhha/knowledge-system/blob/1ded9b3e47bc2e09062dba329c2f918423746fa4/pkg/vector/types/chunk.go#L201)는 위치, 커밋, 내용 해시, `canonical_id`, 카테고리를 보존한다. [EmbeddingIdentity](https://github.com/0xmhha/knowledge-system/blob/1ded9b3e47bc2e09062dba329c2f918423746fa4/pkg/vector/types/embed.go#L8)는 모델·차원·전처리 정체성을 기록한다. | 프로젝트마다 색인을 물리 격리하고, 논리 메타데이터로 `corpus_kind`, 문서 상태, 접근 범위, 버전, `concept_id`를 정의한다. 여러 물리 컬렉션은 실제 접근 패턴·부하가 요구할 때만 선택한다. | P1. 두 프로젝트의 동일 이름 심볼, 오래된 커밋, 비공개 문서가 교차 회수되지 않음을 확인. |
| 57–78, 119–128쪽: 의미 경계 청킹, 부모·자식 맥락 | CKV는 함수·타입·Markdown 제목 섹션, 긴 함수의 분할, 선택적 전체 파일 청크를 지원한다. | 긴 Markdown 섹션은 현재 앞부분만 [절단](https://github.com/0xmhha/knowledge-system/blob/1ded9b3e47bc2e09062dba329c2f918423746fa4/internal/vector/chunk/chunk.go#L110)될 수 있다. 제목 계층과 단락 경계로 자식 청크를 만들고 `parent_id`, `heading_path`, `order`, 원문 줄 범위를 보존한다. 검색은 자식에서, 인용·설명은 부모와 인접 자식에서 구성한다. 긴 함수의 무겹침 분할도 경계 누락 평가 뒤 선택적으로 조정한다. | P1. 문서 뒷부분 질문의 Recall@K와 인용 범위를 기존 대비 비교. 중복·토큰 비용도 함께 측정. |
| 92–103쪽: 검색 적중과 답변 정합성 분리, 근거 없는 질문 | CKV는 Recall@K·MRR·인용 검사, CKS는 시나리오별 citation 정밀도·재현율을 평가한다. | “원천에 답 없음”, “검색은 됐지만 주장을 지지하지 않음”, “문서와 코드 버전 충돌”을 별도 정답 유형으로 추가한다. 답변의 주장별 근거 적합도와 기권 정확도를 평가한다. | P1. 부재 질문에서 근거 없는 단정 0, 정답 질문의 회수율 비열화 방지. 절대 성능 수치는 데이터셋 확정 후 정한다. |
| 128–140쪽: 키워드+벡터와 필터 위치 | CKS Stage 2는 CKV 결과, BM25, 심볼 검색을 RRF로 병합한다. CKV는 일부 희귀 청크 종류에 대해 필터된 정확 스캔을 구현했다. | [일반 CKV 검색](https://github.com/0xmhha/knowledge-system/blob/1ded9b3e47bc2e09062dba329c2f918423746fa4/internal/vector/store/sqlitevec/store.go#L518)은 전역 KNN 결과를 최대 3배 가져온 뒤 후필터한다. 제한적 언어·경로·커밋·테스트 제외에서 정답 K개를 보장하지 않는다. 선택도별 계획: 필터 후보가 작으면 후보 내 정확 스캔, 크면 적응형 증량 또는 인덱스 분할. 계획과 누락률을 계측한다. | P0/P1. 필터 충족 후보가 K개 이상이면 결과 K개를 얻는지, 필터된 정확 검색 대비 Recall@K와 p95를 측정. 구현 방식은 sqlite-vec 제약과 부하 측정 후 결정. |
| 151–162쪽: 방향 있는 관계, 문서의 사실 단위, 그래프·벡터 결합 | CKG는 37종 노드·43종 엣지, 변경 이력과 코드 관계를 가진다. | Markdown의 문서·섹션·주장·근거 범위를 추출해 코드 `canonical_id`, 청크 ID와 연결한다. LLM이 추출한 관계는 `proposed`로 두고 검증된 관계와 구분한다. 모든 문장을 원자 명제로 만들거나 Leiden 커뮤니티를 항상 만들지는 않는다. | P1/P2. 질문이 요구한 관계 경로와 인용 범위가 일치하는지, 잘못 연결된 관계 비율과 빌드 비용을 측정. |
| 172–182쪽: 먼저 개념 정의, 단순 핵심 모델, 정책 분리 | 도메인 YAML에 별칭·코드 앵커·검증 상태가 있고 정책/용어집 내보내기를 지원한다. | `B1–B7` 문서 유형과 도메인 **개념 유형**을 분리한다. 개념 정의와 포함·제외 범위를 합의한 후 코드·문서를 매핑한다. 개념 스키마는 비교적 안정적으로, 판정·권한 정책은 참조 ID로 연결하여 별도 버전 관리한다. | P1. 미정의 개념·잘못된 관계 타입·무근거 `verified` 주장을 빌드에서 차단하고 이전 버전 재현. |

### CKV 설계에서 특히 중요한 두 결함 가능성

**후필터 누락:** [검색 구현](https://github.com/0xmhha/knowledge-system/blob/1ded9b3e47bc2e09062dba329c2f918423746fa4/internal/vector/store/sqlitevec/store.go#L518)은 `ChunkKinds`가 아닌 필터에 대해 함수가 받은 `k`의 3배만큼 전역 최근접 후보를 읽는다. 예를 들어 10만 청크에서 특정 언어·경로·커밋의 청크가 0.1%이면, 전체 상위 후보에 그 집합의 정답이 없을 수 있다. PDF 132–135쪽이 설명한 후필터의 반환 개수·회수율 위험과 동일한 형태다. 단, 현재 실제 데이터에서 발생한 비율은 측정하지 않았다. `searchWithinKinds`는 이미 희귀 `ChunkKinds`를 정확 스캔하므로 “필터링 기능이 전혀 없다”는 평가는 틀리다.

**문서 뒷부분 누락:** [Markdown 파서](https://github.com/0xmhha/knowledge-system/blob/1ded9b3e47bc2e09062dba329c2f918423746fa4/internal/vector/parse/markdown/parser.go#L52)는 제목마다 한 섹션을 만들지만, 긴 prose 섹션은 [청커](https://github.com/0xmhha/knowledge-system/blob/1ded9b3e47bc2e09062dba329c2f918423746fa4/internal/vector/chunk/chunk.go#L110)가 분할하지 않고 입력 상한에서 자른다. 제안하는 부모·자식 구조는 검색 단위와 설명 단위를 분리한다. 단락 분할과 제목 조상을 검색 텍스트에 포함하되, 인용은 원문 범위로 유지해야 한다.

### CKG와 CKS에서 우선할 정합성

- 이미 재현한 `ckg audit`의 과거 `Hunk` 1,056개를 현재 Go 파일로 세는 오류를 고친다. 현재 Go 빌드 파일 877개는 모두 색인돼 있었다. 감사가 거짓 실패하면 패키지 승격 게이트를 믿기 어렵다. [파일 집계](https://github.com/0xmhha/knowledge-system/blob/1ded9b3e47bc2e09062dba329c2f918423746fa4/internal/graph/persist/sqlite_reader.go#L84)를 현재 파일 집합으로 제한한다.
- CKS는 [커밋·그래프 다이제스트 정합성](https://github.com/0xmhha/knowledge-system/blob/1ded9b3e47bc2e09062dba329c2f918423746fa4/internal/setup/verify.go#L35)을 검사한다. 다만 [승격의 최소 `canonical_id` 커버리지 기본값](https://github.com/0xmhha/knowledge-system/blob/1ded9b3e47bc2e09062dba329c2f918423746fa4/cmd/cks/setupcli/setup.go#L163)은 0이라 꺼져 있다. 프로필별 기준과 정확/완화 정렬 비율을 공개해야 한다.
- [실제 Composer](https://github.com/0xmhha/knowledge-system/blob/1ded9b3e47bc2e09062dba329c2f918423746fa4/internal/system/composer/composer.go#L1)는 분류 → CKV 기반 키워드·후보 → CKG 검색/RRF → 관계 확장 → 예산·정화 → `EvidencePack`의 순차 흐름이다. 일부 README의 “병렬 fan-out” 도식은 실제 흐름과 다르므로 문서를 고쳐야 한다.
- `Document → Section → Claim → EvidenceSpan` 및 `Claim ABOUT Concept`, `Concept IMPLEMENTED_BY CodeSymbol`, `Test VALIDATES Claim`은 **제안 모델**이다. 현재 그래프에 구현됐다는 의미가 아니다. 구문 추출과 사람이 승인한 사실을 구분한다. PDF 156–157쪽의 커뮤니티 클러스터링도 모든 코드 질문의 기본 경로로 넣지 않는다. 전체 저장소의 주제·구성 요소를 요약하는 전역 질문에서만 별도 실험한다. GraphRAG 논문이 평가한 대상도 이런 전역 요약 질문이며, 이 프로젝트의 국소 코드 위치 검색에서 같은 효과가 보장되는 것은 아니다.[^graphrag]

## 2. 온톨로지와 세 개발 방식의 통합

### 2.1 먼저 답할 질문에서 개념을 도출한다

범용 코어가 모든 도메인의 상세 객체를 선제 정의하면 과대 모델링이 된다. 다음의 능력 질문에서 시작한다: “요구사항 R을 구현하는 코드는?”, “이 불변식을 실제로 강제하는 조건과 검증 테스트는?”, “스펙과 현재 코드가 충돌하는가?”, “변경 시 어느 호출자·테스트·문서가 영향받는가?”, “이 답은 어느 버전과 근거에 유효한가?” PDF 177–182쪽은 용어 합의를 매핑보다 앞에 두고 핵심 구조를 단순하게 하라고 권한다.

| 층 | 최소 객체 | 책임과 원천 |
|---|---|---|
| 공통 메타 모델 | `Project`, `Snapshot`, `Concept`, `Term`, `Requirement`, `AcceptanceCriterion`, `CodeSymbol`, `DocumentSection`, `Claim`, `TestCase`, `EvidenceSpan`, `Assertion` | CKS가 관리하는 형식과 ID 규칙. 코드 AST와 문서는 관찰된 사실, 스펙은 의도된 요구를 제공한다. |
| 프로젝트 도메인 모델 | 예: `consensus:QuorumThreshold`, `consensus:ValidatorSetSize` | 프로젝트 전문가가 정의문, 포함·제외 범위, 한국어·영어 표기, 소유자, 상태를 승인. 안정적 개념 ID는 코드 이름과 독립. |
| 관계 스키마 | `SPECIFIES`, `REQUIRES`, `IMPLEMENTED_BY`, `ENFORCED_BY`, `VALIDATED_BY`, `DOCUMENTED_BY`, `DEPENDS_ON`, `CONTRADICTS`, `SUPERSEDES` | 각 관계의 시작/끝 타입, 방향, 역관계, 필요한 근거와 유효 범위. `RELATED_TO`는 후보 확장용 약한 연결. |
| 사실 | `Assertion(subject, predicate, object, evidence, status, valid_range)` | 사실 한 건마다 프로젝트·커밋 또는 문서 다이제스트·원천 위치·추출 방식·검토자·검증 시간을 기록. `proposed`, `verified`, `rejected`, `superseded`를 분리. |
| 정책 | 접근·우선순위·자동화 허용 규칙 | 온톨로지 객체는 적용 `policy_id`를 참조하고, 정책 정의·결정·감사는 독립 버전으로 저장. PDF 180–185쪽의 구조/정책 분리와 의미 기반 거버넌스 설명을 이렇게 양립시킨다. |

현재 [도메인 엔트리](https://github.com/0xmhha/knowledge-system/blob/1ded9b3e47bc2e09062dba329c2f918423746fa4/internal/system/inventory/types.go#L81)의 `B1–B7`은 지식 글의 종류이지 `Invariant`나 `Subsystem` 같은 의미 타입이 아니다. 기존 YAML을 버리지 말고 `concept_id`와 검증된 `relations`를 점진적으로 추가한다. 예: `consensus:QuorumThreshold ENFORCED_BY code:<canonical_id>`는 해당 코드의 정확한 커밋·줄과 검토 상태를 가져야 한다. 단순 문자열이 같은 다른 저장소의 심볼에 붙으면 안 된다. 현재 [검증기](https://github.com/0xmhha/knowledge-system/blob/1ded9b3e47bc2e09062dba329c2f918423746fa4/internal/system/inventory/validate.go#L132)는 `verified` 엔트리에 코드 앵커를 요구하므로, 아직 코드가 없는 요구사항·도메인 개념을 검증할 새 근거 유형을 별도로 정의해야 한다.

사람이 읽는 정의와 다국어 별칭은 CKV의 개념 설명 청크에도 색인하고, 타입 있는 관계는 CKG 또는 CKS의 별도 SQLite 의미 저장소에 저장한다. 원본은 검토 가능한 파일(YAML/Markdown 등)로 두고 DB는 빌드 산출물로 만드는 방법을 권한다. RDF·OWL·SHACL을 준수하는 별도 저장·추론·검증 스택은 상호운용성 또는 복잡한 추론 요구가 입증될 때 선택한다. SKOS의 preferred/alternative label과 SHACL의 데이터/제약 분리는 설계 참고가 된다.[^skos][^shacl]

### 2.2 자연어 약점을 낮추는 질의 흐름

질문을 온톨로지 개념 한 개로 조기에 고정하지 않는다. `raw_query`를 보존한 채 용어집으로 `concept_candidates` 여러 개를 만들고, 기존 CKV 자연어 검색·BM25·정확 심볼 검색을 수행한다. 일치하는 개념은 가산점이나 관계 확장 신호로 쓰되 **온톨로지에 없는 좋은 청크를 탈락시키지 않는다**. 후보가 모호하면 복수 가설을 유지한다. 최종 주장을 할 때만 `verified` 관계와 출처·버전을 확인한다. 벡터 유사도는 사실 증명이 아니며, 그래프 경로도 원천과 검증 상태가 없으면 사실 증명이 아니다.

검색 실패는 `검색 결과 없음`, `결과는 있으나 주장 근거 부족`, `스펙-코드 충돌`, `버전 불일치`로 구분해 출력한다. 기존 CKS [인용 평가](https://github.com/0xmhha/knowledge-system/blob/1ded9b3e47bc2e09062dba329c2f918423746fa4/internal/system/eval/metrics.go#L109)는 예상 인용이 비어 있고 실제 인용이 있을 때 별도 기권 점수 없이 정밀도·재현율을 1.0으로 취급하므로, 답 없는 질문의 평가는 전용 판정 규칙이 필요하다. PDF 92–103쪽과 RAG 실패 연구는 검색 실패와 생성·응답 실패를 나눠 측정할 필요를 뒷받침한다.[^failures]

### 2.3 제안하는 통합 개발 알고리즘

여기서 **모델 기반 개발**은 개념·관계·불변식과 시스템 구조를 명시적으로 모델링한다는 뜻이고, **스펙 기반 개발**은 요구사항·수용 기준을 변경의 계약으로 삼는다는 뜻이다. **지식 데이터 기반 개발**은 코드·문서·테스트·변경 이력에서 얻은 버전 있는 근거로 변경을 제안·검증한다는 이 문서의 설계 용어다. 후자를 이미 표준화된 개발 방법론의 고유명사로 주장하지 않는다. OMG의 MDA는 추상 모델과 구현 모델을 구분하는 참고점이고, GitHub Spec Kit은 사양→계획→작업→구현→수렴의 산출물 흐름을 제공하는 비교 사례다.[^mda][^speckit]

```text
입력: 변경 요청, 승인된 스펙, 프로젝트 스냅샷
1. 요청에서 목표·비목표·수용 기준을 명시하고 관련 개념 후보를 만든다.
2. CKV에서 자연어·문서·유사 구현을, CKG에서 정확 심볼·호출자·테스트를 찾는다.
3. 같은 스냅샷에서 Requirement → Concept/Invariant → CodeSymbol → TestCase
   추적 경로를 구성하고, 빈 연결과 상충하는 원천을 표시한다.
4. 계획과 변경 범위를 제안한다. 불명확한 스펙은 승인된 사실로 승격하지 않는다.
5. 외부 코딩 에이전트 또는 개발자가 패치를 작성한다.
6. 테스트·정적 검사·수용 기준·근거 인용·스냅샷 정합성을 검증한다.
7. 승인된 결과만 지식 사실에 반영하고 CKG/CKV를 같은 버전으로 재색인한다.
출력: 변경/실패 원인, 통과·실패 기준, 관련 코드·문서·테스트 근거.
```

**진실의 종류를 섞지 않는다:** 스펙은 *의도*, 코드/AST는 *현재 구현 관찰*, 테스트는 *특정 실행의 검증*, 도메인 엔트리는 *검토된 설명*이다. 코드와 스펙이 다르면 한쪽을 최신이라는 이유로 덮어쓰지 않고 `CONTRADICTS`로 보고한다. LLM 추출 관계는 `proposed`이며 테스트 통과만으로 모든 스펙의 충족을 증명하지 않는다. 수정은 첫 단계에서 읽기 전용 제안으로 시작하고, 사람의 승인과 실제 실행 검증 뒤에 자동화 범위를 넓힌다.

현 프로젝트는 [EvidencePack](https://github.com/0xmhha/knowledge-system/blob/1ded9b3e47bc2e09062dba329c2f918423746fa4/pkg/system/contract/pack.go#L175)을 만들어 상위 코딩 도구에 전달한다. `cks agent`와 평가 도구에서 LLM을 사용하지만 기본 Composer는 LLM을 호출하거나 소스를 패치하지 않는다. 그러므로 위의 폐루프를 **현행 구현**으로 부르지 않는다. RepoCoder의 반복 회수·생성, RepoGraph의 저장소 그래프 탐색은 방향을 뒷받침하지만 이 프로젝트에서 효과는 별도 평가가 필요하다.[^repocoder][^repograph]

### 2.4 단점을 줄이는 운영·평가 조건

| 실패 위험 | 제한 방법 | 측정 |
|---|---|---|
| 새 표현·동음이의어를 개념으로 잘못 고정 | 한국어·영어 표기, 복수 후보, 원문 쿼리 경로, 무온톨로지 폴백 | 미등록 표현 질문의 Recall@K, 잘못된 개념 확정률 |
| 그래프가 과대해지고 쿼리가 느려짐 | 고가치 개념부터, 사실/문서/코드 층 분리, 관계 유형과 홉 상한 | 노드·엣지 증가량, p95 질의 지연, 증분 빌드 시간 |
| LLM 추출 관계가 사실처럼 유통 | `proposed`와 `verified` 격리, 타입·근거 검사, 고위험 항목 검토 | 관계 연결 정밀도, 무근거 답변 비율 |
| 스펙·코드·색인 버전 혼합 | 원자적 데이터셋 버전: 소스 스냅샷 + 온톨로지/스펙 버전 + 그래프 다이제스트 + 임베딩 정체성 | 서로 다른 버전 조인 0건, 롤백 재현 |
| 자동 생성이 기존 계약 위반 | 제안-패치-테스트-승인 분리, 스펙별 실행 가능한 수용 기준 | 실제 패치 성공률, 회귀 건수, 영향 범위 커버리지 |

평가에는 **기존 CKV/CKG/CKS, 어휘만 추가, typed relation만 추가, 둘 다 추가**의 절제 실험이 필요하다. 벤치마크 질문은 기능 위치, 변경 영향, 스펙-코드 충돌, 다국어 표현, 같은 이름의 다른 심볼, 답 없는 질문, 문서 뒷부분, 타 프로젝트 격리를 포함한다. 검색은 Recall@K/MRR·관계 경로 정확도, 답변은 주장별 근거 적합도·기권 정확도, 개발 흐름은 테스트 통과·수용 기준 충족·사람 수정 비용으로 각각 측정한다. RepoGraph와 SWE-bench의 연구는 저장소 수준 문맥과 실행 검증을 평가해야 하는 근거로 활용하며, 특정 수치를 본 프로젝트에 전이하지 않는다.[^repograph][^swebench]

## 3. 임의 프로젝트에 적용하는 설치형 CKS

### 3.1 현재 자산과 실제 제약

| 현재 자산 | 일반화할 때 남는 과제 |
|---|---|
| [세 바이너리 빌드](https://github.com/0xmhha/knowledge-system/blob/1ded9b3e47bc2e09062dba329c2f918423746fa4/Makefile#L60), [cks setup](https://github.com/0xmhha/knowledge-system/blob/1ded9b3e47bc2e09062dba329c2f918423746fa4/cmd/cks/setupcli/setup.go#L120), [설정 생성](https://github.com/0xmhha/knowledge-system/blob/1ded9b3e47bc2e09062dba329c2f918423746fa4/cmd/cks/mcpcli/genconfig.go#L18) | 현재 저장소를 빌드해야 하는 진입점 대신 사전 빌드 배포물, 설치·진단 명령, 템플릿 팩이 필요. 첫 릴리스는 `cks`, `ckg`, `ckv` 세 바이너리를 함께 배포하는 편이 현재 실행 구조와 맞다. |
| [버전별 데이터셋 승격·롤백](https://github.com/0xmhha/knowledge-system/blob/1ded9b3e47bc2e09062dba329c2f918423746fa4/internal/setup/reindex.go#L1) | 서버가 `current`를 시작 시 한 번 열어 승격 후 재시작해야 한다. 운영 도구는 이를 자동 안내·재시작하거나, 검증된 스냅샷에 대한 명시적 핸들 교체 기능을 후속 구현해야 한다. |
| Go/TS·JS/Solidity/Markdown 및 proto 일부 지원 | 모든 저장소에서 동일한 AST·호출 그래프를 만들 수는 없다. 설치 시 탐지한 언어와 플러그인별 **파싱·심볼·타입·호출·문서** 기능을 보고해야 한다. 미지원 언어는 “텍스트 검색만”으로 명시적으로 낮춘다. |
| Ollama 기본 임베더와 선택적 ONNX 경로, SQLite/CGO | 설치 패키지의 OS·CPU별 네이티브 의존성을 시험하고, 모델 다운로드·오프라인·원격 임베더 사용 여부를 프로필로 분리한다. `mock` 임베더는 설치 확인용이며 의미 검색 품질을 대표하지 않는다. |
| `projects/stablenet/setup.yaml`의 검증된 팩 | 이 팩을 범용 기본값으로 복사하지 않는다. 자동 생성한 최소 팩과 선택적 도메인 팩을 분리한다. 프로젝트별 정책·용어·스펙을 별도 승인한다. |

### 3.2 설치 후 사용 경험: 제안하는 명령, 현행 CLI 아님

```text
cks doctor                         # OS/CGO/임베더/공간/권한/언어 탐지
cks init --src /path/to/repo       # 프로젝트 ID, 제외 경로, 문서·스펙 위치, 모델 선택
cks index --project <id>           # 버전 있는 graph+vector+semantic 빌드와 게이트
cks eval --project <id>            # 기본·프로젝트별 대표 질문 평가
cks serve --project <id>           # MCP/로컬 API, 활성 스냅샷 사용
cks status --project <id>          # 능력, 색인 범위, 커버리지, 신선도
cks rollback --project <id> <ver>  # 검증된 이전 데이터셋 활성화
```

기본 모드는 로컬 저장소를 읽고 별도 사용자 데이터 디렉터리에 색인을 둔다. 저장소에는 명시적 선택 전에는 파일을 쓰지 않는다. 각 프로젝트는 설정·인덱스·로그·정책·권한 범위를 분리한다. 설치 단계에서 `.gitignore`, 생성 코드, 바이너리, 비밀 파일 등을 스캔하고 기본 제외를 제안하되, 실제 색인 범위를 manifest와 `status`에 공개한다. 배포 시 `--skip-vector` 또는 텍스트 전용 모드는 가능하나 의미 검색이 없다는 사실을 표시한다.

**스냅샷 정합성:** 현재 `--version auto`는 추적 파일이 깨끗한 Git HEAD를 전제로 하며, 추적되지 않은 파일은 검사에서 예외로 둔다. 개발 중 저장소에는 커밋되지 않은 변경이 흔하다. 설치형 제품은 `committed` 모드와 `working-tree` 모드를 분리하고, 후자에서 색인한 파일 목록·내용 다이제스트를 스냅샷 ID에 반영해야 한다. 단순히 HEAD를 표시하면 색인 내용과 이름이 달라진다.

**언어 확장:** `Detect → Parse → Symbols/Relations → Chunk → Citation` 어댑터 계약을 정의한다. 새 언어는 최소한 파일/심볼/줄 위치의 골든 테스트와 실제 저장소 규모 성능을 통과해야 “AST 지원”이라고 표시한다. 타입·호출 관계는 언어별 정확도와 확신 등급을 따로 보고한다. 미지원 언어의 구문 그래프를 추측으로 채우지 않는다. GraphCodeBERT의 데이터 흐름 연구는 장기적으로 AST 외 구조를 검토할 근거이지만, 모든 언어에 데이터 흐름을 즉시 구현해야 한다는 뜻은 아니다.[^graphcodebert]

**배포 범위:** 첫 릴리스는 실제 빌드 가능한 플랫폼별 아카이브에 세 바이너리, 기본 정책, 샘플 설정, 스키마 마이그레이션과 버전/체크섬을 포함한다. `doctor`가 네이티브 의존성과 임베더 연결을 검사하고, 실패 시 그래프 전용 모드를 명시적으로 제시한다. 패키지 매니저 배포와 서비스 자동 설치는 아카이브 경로가 검증된 뒤 추가한다. [저장소 라이선스](/Users/0xtopaz/work/github/0xmhha/knowledge-mcp/knowledge-system/LICENSE)는 AGPL-3.0이므로 배포 준비에서 라이선스·의존성 검토가 필요하다.

### 3.3 순서와 통과 기준

| 단계 | 범위 | 완료를 확인할 관찰값 |
|---|---|---|
| 0. 정확성 기준선 | `ckg audit` 수정, CKV 필터 질문과 문서 뒤쪽 질문 추가, 실제 CKV+CKG+CKS 평가 실행 | 현재 877개 Go 파일 감사 일치; 필터된 정확 검색 대비 누락률 파악; 기존 Recall/MRR·지연 기록 |
| 1. 검색 품질 | 선택도 기반 필터, 계층 문서 청크와 부모 문맥, 출처·버전 확인 | 관련 후보가 K 이상인 필터 질문에서 K개 반환; 문서 뒤쪽 질문 회수율 개선; 인용 줄 범위 정확 |
| 2. 의미 계약 | 최소 개념·관계·Assertion·Evidence 스키마와 도메인 10–20개 시범 적용 | 잘못된 타입·무근거 검증 사실 차단; 무온톨로지 검색 경로 보존; A/B 품질 측정 |
| 3. 개발 흐름 | Requirement→Concept→Code→Test 추적, 변경 계획·충돌 보고, 외부 에이전트 연동 | 스펙 변경 시 영향 경로 재현; 사람 승인 전 자동 수정 없음; 테스트·수용 기준 결과 연결 |
| 4. 배포 | 일반 프로젝트 `init/doctor/index/serve`, 플랫폼별 배포물, 프로젝트 격리 | 새 저장소에서 수동 코드 수정 없이 색인·질의; 지원/미지원 기능 표시; 교차 프로젝트 근거 혼입 0; 재시작/롤백 절차 검증 |

현재 검토는 실제 CKG 빌드·검증과 소스 분석에 근거한다. CKV 임베딩 색인과 CKS 종단 간 질문 품질은 이 검토에서 실행하지 않았다. 따라서 개선안의 성능 효과, 범용 설치 성공률, 통합 개발 루프의 패치 성공률은 **제안과 평가 계획**이지 측정 결과가 아니다.

## 연구·표준 근거

[^skos]: W3C, [SKOS Primer](https://www.w3.org/TR/skos-primer/): 선호 표기, 대체 표기, 개념 관계를 분리해 정의한다.
[^shacl]: W3C, [Shapes Constraint Language](https://www.w3.org/TR/shacl/): 데이터 그래프와 제약 그래프를 별도로 입력받아 검증한다.
[^mda]: Object Management Group, [Model Driven Architecture](https://www.omg.org/mda/): 플랫폼 독립 모델과 플랫폼별 구현 모델을 구분한다.
[^speckit]: GitHub, [Spec Kit 문서](https://github.com/github/spec-kit/blob/main/docs/index.md): 사양, 계획, 작업, 구현, 수렴을 잇는 워크플로의 공개 사례다.
[^failures]: Barnett 외, [Seven Failure Points When Engineering a Retrieval Augmented Generation System](https://arxiv.org/abs/2401.05856), CAIN 2024: 검색·생성 과정의 실패 유형을 연구한 경험 보고서다.
[^repocoder]: Zhang 외, [RepoCoder](https://arxiv.org/abs/2303.12570), 2023: 저장소 수준 코드 완성의 반복 검색·생성 접근.
[^repograph]: Ouyang 외, [RepoGraph](https://arxiv.org/abs/2410.14684), ICLR 2025: 저장소 그래프가 코딩 에이전트의 탐색을 돕는 접근을 평가했다.
[^graphrag]: Edge 외, [From Local to Global: A Graph RAG Approach to Query-Focused Summarization](https://arxiv.org/abs/2404.16130), 2024: 대규모 문서 집합의 전역 질문을 대상으로 그래프 커뮤니티 요약을 연구했다.
[^swebench]: Jimenez 외, [SWE-bench](https://arxiv.org/abs/2310.06770), 2023: 실제 저장소 이슈 해결을 코드 수정과 실행 가능한 검증으로 평가한다.
[^graphcodebert]: Guo 외, [GraphCodeBERT](https://arxiv.org/abs/2009.08366), 2020: 코드 표현에 데이터 흐름 구조를 활용한다.
