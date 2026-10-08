# 대규모 보관 데이터셋 비용 진단

2026-10-05. [실측 원장](../../system/eval/b0-knowledge-system/large-cost-diagnostic-m2max-2026-10-05/summary.json), [메모리 관측](../../system/eval/b0-knowledge-system/large-cost-diagnostic-m2max-2026-10-05/memory-summary.json), [파일 해시](../../system/eval/b0-knowledge-system/large-cost-diagnostic-m2max-2026-10-05/evidence-manifest.json)을 보존했다. C1-04의 독립 준비 범위이며 공식 품질·지연·운영 verdict와 사람 검토 비용은 null이다.

## 입력과 측정 범위

최신 FIX-19 macOS arm64 preview의 `cks`(SHA `9c7ed2a0b45aed0f1172c21404d538e34caf52163cfd18d8aae2162f008120ad`)와 Ollama0.35.1의 승인된 BGE-M3 digest/1024차원을 사용했다. 기존 고정 commit `71cb71cd55960833e930269e272f7a4a060be3aa`의 선택된 1,569파일/13,575청크 데이터셋을 별도 사본으로 조회했다. 원래 Go 소스 29개 제외 및 수정 전 빌더의 한계는 유지한다. 새 공식 strict 빌드나 전체 저장소 완전성 증거가 아니다.

개발 문항 B0-CODE-01 하나를 baseline/knowledge 두 경로로 실행했다. 요청에는 프롬프트와 scope만 들어가며 정답/앵커는 전달하지 않았다. K10·별도 knowledge K6, 날짜2026-10-01/subsystem knowledge-system은 이번 진단 설정이며 STV2 승인을 대신하지 않는다. 경로별 warmup2·검색5·warm20·새 프로세스3, 전체60 SDK를 모두 수집했다. 순서는 baseline 후 knowledge이며 공식8arm 회전 비교가 아니다. warm20은 반복 관측이며 독립 표본20개가 아니다.

## 관측 결과

| 경로 | warm p50 / p95 (20회) | 새 프로세스 시작부터 응답 p50 / 최대 (3회) | 응답 JSON 크기 |
|---|---|---|---:|
| baseline | 3.521 / 3.737초 | 6.627 / 6.972초 | 24,200바이트 |
| knowledge | 3.824 / 4.408초 | 6.224 / 6.373초 | 24,200바이트 |

p95는 nearest rank다. 응답 크기는 저장된 SDK 응답을 UTF-8 JSON으로 재직렬화한 크기이며 wire bytes가 아니다. cold는 MCP 프로세스를 새로 시작하며 모델을 unload하지 않는다. 처음 모델은 비상주, 이후 상주했고 다른 호스트 작업을 격리하지 않았다. 기본90초 deadline에서 이번60 호출이 완료됐다는 사실을 Linux 초기화 실패의 해소나 운영 지연 합격으로 확장하지 않는다.

60 SDK의 480인용/480본문은 독립 Python의 파일·줄·본문 SHA/좌표/outer integrity와 공개 Go Verify를 통과했다. 반복 및 baseline/knowledge 기본 근거 집합은 같다. 실패/누락 SDK는0이다. 다만 backend 수준 오류는 별도다. 요청 scope의 논리 호출1,980 중 CKG BM25480·neighbors360이 backend_error이고 HTTP639는 모두200이다. 이를 전체 backend 성공으로 기록하지 않는다. 별도7개 startup intent anchor scope에는 embed455·HTTP1,365(모두200)가 기록됐다. 다른 constructor/SQL/모델 내부 작업은 이 카운터 밖이다.

전체 command의 `time -l` 원문은 real247.29/user119.11/sys88.10초·reported max RSS96,681,984바이트다. 이것을 MCP subtree나 모델 서버의 peak로 해석하지 않는다. 별도2 SDK의 0.25초 간격 프로세스 관측에서 MCP max observed RSS82,992KiB(약81.05MiB), caller/MCP 등 동시 RSS 합의 관측 최대106,656KiB(약104.16MiB)였다. 샘플 사이 peak·공유 페이지 중복·sampler overhead·모델 서버 미포함을 명시한다. 보완2응답의16인용/본문도 독립 원문 및 Go Verify를 통과했고 주 측정과 근거가 같다. 총62호출을 주 측정60의 분모로 합치지 않는다.

## Backend 오류 재현과 다음 확인

[읽기 전용 재현](../../system/eval/b0-knowledge-system/large-cost-diagnostic-m2max-2026-10-05/backend-replay.json)은 저장 청크에서 유도한 후보의 SHA를 실제 BM25 오류 input SHA8개와 모두 결합했다. 하이픈이 있는 문서 후보 이름8개가 bare FTS에서 `no such column`을 일으키고, literal quoted term으로는 오류 없이0개를 반환한다. Python SQLite 직접 재현이며 Go adapter의 수정/개선 합격이 아니다. 선택 문서6인용 중 단순 overlap이 없는3개도 확인했지만 neighbors360 전체 원인으로 단정하지 않는다. query 구성 경계와 실제 노드 해석을 별도로 확인해야 한다.

원본·사본의 DB/manifest/source와 고정 source tree, 패키지 binary/config/request SHA, 실제 사람 입력5개는 측정 및 보완 후 유지됐다. 같은 model tag/digest/version도 유지됐다. 48개 증거 파일에 raw 응답·계측·입력/봉인·보완·읽기 전용 재현·helper 원문을 결합했다. DB/바이너리/모델 blob/private key는 Git에 넣지 않았다. raw text 로그는 바이트/SHA/원문을 가진 lossless JSON으로 보존했다.

다음 독립 작업은 확인된 BM25 후보→FTS 경계의 회귀 재현과 영향 검토다. 공식 최우선은 B0-01/02/03/05·STV2 결정→입력 동결→새 strict B0-07이다. 전체30개 중 완료2/진행11/대기17·미완료28개·공식 게이트0/4를 유지한다. B0-01–09, B1-03–08, C0-01–06, C1-01–07 및 STV2-01·OP-01–08·native amd64·최종 고지/복구/지원/출시가 남는다. [현황판](./EXECUTION-STATUS.md)을 함께 확인한다.

후속 [FIX-20](./B0-FTS-KEYWORD-DIAGNOSTIC.md)은 후보→FTS 경계의 두 회귀/실제SQLite/관련 패키지·race·vet와 같은 실모델2 SDK를 확인했다. BM25 오류는 호출당8→0, 기존 인용/본문/좌표는 같다. 이 문서의60호출 및 비용은 수정 전 FIX-19 package의 원래 측정으로 유지한다. neighbors 오류와 공식 품질/최종 package 범위는 별도다.
