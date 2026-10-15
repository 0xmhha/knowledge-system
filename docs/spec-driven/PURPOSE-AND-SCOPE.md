# 개발 목적과 현재 범위

코드 구조·변경 이력·검토된 문서와 정책을 같은 프로젝트·소스 스냅샷에 연결하여, 외부 코딩 에이전트가 변경 이유와 필요한 검증을 근거로 판단하도록 한다. 검색 도구나 연결 그래프가 사람의 판단을 자동 대체한다는 목표는 아니다. 품질 개선·총비용 감소는 충분한 독립 평가로 확인해야 한다.

현재 제품은 하나의 Go 모듈에 CKG(구조·키워드), CKV(의미 회수), CKS(근거 합성·MCP)를 둔다. 프로젝트 고유 지식은 팩으로 선택한다. 모델·원문·소스·팩·스냅샷 신원을 검증하고, 입력과 실행의 불일치를 숨기지 않으며, 테스트 성공과 기준별 사람 수용을 구분한다.

## 원래 요구사항의 유지

초기 요구사항은 INV7/FR10/NFR5/반례9다. 원문과 v2 대응은 원장의 N-01 복구 증거에 보관되어 있다. 아래 항목의 목적은 유지되며, 오래된 단계별 미완료 서술은 현재 작업리스트로 대체한다. 이 목록 전체의 production 품질이나 외부 조직 수용을 완료했다고 선언하지 않는다.

| 원 ID | 유지하는 목적 |
|---|---|
| INV-01 | 프로젝트/소스 버전 격리 |
| INV-02 | 공통manifest·부분build 비활성 |
| INV-03 | 원문·버전·줄로 인용 재현 |
| INV-04 | verified의 타입·출처·검토자 |
| INV-05 | 의미 확장으로 원 검색 차단 금지 |
| INV-06 | 공개 계약/구DB 호환 |
| INV-07 | 모델/차원/전처리 동일성 |
| FR-01 | 현재File만 감사 |
| FR-02 | 필터 내 K수와 회수 품질 |
| FR-03 | 장문child 검색·parent 문맥 |
| FR-04 | 실패/근거 부족/무답/충돌 구분 |
| FR-05 | 문서/claim/span·코드 연결 |
| FR-06 | 작은core/프로젝트 어휘 버전 관리 |
| FR-07 | 원 질문·복수 후보·soft 신호 |
| FR-08 | 요구→개념→코드→test·충돌 보고 |
| FR-09 | 임의project 설치/진단/복구 |
| FR-10 | committed/working-tree 분리 |
| NFR-01 | 품질 비열화·비용/지연/규모 |
| NFR-02 | 비밀/제외/링크/프로젝트 격리 |
| NFR-03 | 의미 버전/이전/출처 |
| NFR-04 | 파서 실패/미지원 능력 공개 |
| NFR-05 | 문서·CLI/MCP 계약 일치 |
| S-01 | 선택도 높은 필터의 exact oracle |
| S-02 | 긴 문서 끝·정확 줄 |
| S-03 | 과거 Git파일 Hunk |
| S-04 | 없는API/정책의 무관 인용·단정 |
| S-05 | 스펙 문장과 코드 조건의 차이 |
| S-06 | 다의어/한영별칭/미등록어 |
| S-07 | 동명symbol·다른project |
| S-08 | 같은HEAD의 수정/신규 파일 |
| S-09 | 한engine 실패에서 승격 금지 |

현재 수용된 구현·시험 범위는 [실행 명세](./REFACTORING-EXECUTION-SPEC.md), 측정과 부족분은 [평가 상태](./EVALUATION-STATUS.md), 실제 남은 일은 [남은 작업](../REMAINING-WORK.md)을 따른다. 기존18개는17개 완료와 N16 보류이며, 온톨로지4개는4개 완료다. 이 완료 수를 다음 상위 목표의 완료 수로 사용하지 않는다.

## 외부 상위 목표의 책임

원 상위 WI10개와 기존 리팩토링 작업은 범위가 다르다. 구현 전 [재감사](./UPPER-WORK-ITEMS-AUDIT.md)는 그때의 담당·source·부족분을 보존하는 자료다. 현재 구현 상태는 [구현 명세](./UPPER-WORK-ITEMS-IMPLEMENTATION.md)와 [원장](./UPPER-WORK-ITEMS-PROGRESS.json)을 따른다. 선택9개는 구현·명시 검증9/9 완료이며 knowledge-system은 PR #102로 main에 반영됐다.9개 전체의 독립 production/사람 수용은 not_recorded다.

| ID | 현재 구현·책임 | 남은 후속 범위 |
|---|---|---|
| WI-V1 | 오라클·무효/과적합 반례·완료 gate; coding-agent | 실제 host/프로젝트 연결·독립 수용 |
| WI-V2 | 복수 수정 후보·판정 패널·선택/기각 계약; coding-agent | 실제 후보 선택 품질·비용의 독립 수용 |
| WI-3 | bounded 의심 후보·typed 신호/provenance/unknown; coding-agent+CKS | production 입력·설치된 전체 bugfix 역할 연결 |
| WI-4 | 반복·주입·관찰·모드9 실행 근거; coding-agent+ChainBench | 실제 chain/host 반복·방어·재발 수용 |
| WI-1 | retained Go CFG·값/제어 의존 공개 계산; CKG | 실제 프로젝트의 정확도·비용·지원 범위 수용 |
| WI-2 | opt-in Go rule pack·source→sink 후보와 수동 구분; CKG/CKS | 독립 사례의 오탐·미탐·효과 평가 |
| WI-C | typed partial review·보고서/그림·전달 gate; coding-agent | 실제 리뷰 절차·독립 내용 검토 |
| WI-6 | SQL-bounded 지역 값/제어 계산의 함수 문맥 확장; CKS | 실제 corpus recall/noise/cost 수용 |
| WI-SP | typed 정책·보안 MCP 질의·출처 unknown·권한 경계; CKS+CKG | 실제 소비자·운영 신뢰 경계 수용 |
| WI-5 | 지식 구축·지속 갱신 매니저 플러그인으로 범위 구체화; 미구현 | KM-03/04 및 기반 KM-01/02로 구현·검증; 중복 집계하지 않음 |

이 표의9개 구현 완료는 원래 기능 전체의 production 품질·독립 수용과 분리한다. source bundle을 보존한 다른 저장소의 변경은 해당 저장소 게시·설치 상태를 별도로 확인한다. 전체 후속·선택 범위·보류 결정은 [남은 작업](../REMAINING-WORK.md)에만 집계한다.

## 2026-10-15 채택한 다음 범위

[지식 매니저·비교 명세](./KNOWLEDGE-MANAGER-AND-COMPARISON-SPEC.md)에 문서 작성 부담을 줄이는 Claude 매니저, 지속 갱신, 다중 프로젝트 격리와 산업 개념별 비교 근거 제공을 등록했다. 온톨로지 효과 평가도 우선 필수 범위다. 현재는 요구 등록/설계 초안이며 구현 완료는0/7이다. [전용 원장](./KNOWLEDGE-ROADMAP-PROGRESS.json)은 기존9개 정식 평가와 별도로 관리하며, 독립 평가의 미동결은 로컬 개발 전체의 착수 조건이 아니다.
