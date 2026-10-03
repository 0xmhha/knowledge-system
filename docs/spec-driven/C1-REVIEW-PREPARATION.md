# C1 출시 검토 준비

2026-10-04 · 기준 커밋 `46020e4f` · 운영 출시 판정 대기.

현재 바이너리의 `go version -m`을 기준으로 연결된 28개 모듈의 root LICENSE/NOTICE를 수집했다. 누락 0개이며 복사한 모든 파일의 SHA를 확인했다. 프로젝트 LICENSE SHA와 바이너리 SHA, 현재 CPU·메모리·OS·load/swap도 기록했다. [검토 원장](../../system/eval/b0-knowledge-system/c1-review-preparation-m2max-2026-10-04/review-summary.json), [모듈별 원문 목록](../../system/eval/b0-knowledge-system/c1-review-preparation-m2max-2026-10-04/third-party-licenses.json).

| 결정·검증 | 현재 증거 | 완료에 필요한 항목 |
|---|---|---|
| Go 모듈 원문 수집 | 28개·root 원문 누락 0·해시 확인 | 라이선스 적합성·고지 충족의 사람 검토 |
| 내장 native 자산 | 기존 패키지 수집 도구 존재 | parser/grammar·SQLite 등 비-root 고지 범위 별도 감사 |
| 운영 서명 | 기존 signer는 test-signed-preview만 생성 | 운영 소유자·public key fingerprint·보관/회전/폐기 정책·신뢰 루트 승인 |
| 지원 플랫폼 | 이전 macOS/Linux mock preview 재검증 기록 | 최종 코드 패키지와 실제 Linux Ollama·대규모 운영 비용 |
| 복구·마이그레이션 | 기존 A 구조 시험·문서 | 최종 C1 패키지의 이전 데이터 보존·rollback 재생 |
| 품질·출시 | 공식 품질 지표 null | 승인 입력/프로토콜·B0/B1·C0/C1 판정과 출시 결정 |

운영 키를 새로 만들거나 시험 키를 운영 키로 승격하지 않았다. root LICENSE 수집만으로 내장 자산 전체 고지나 적합성을 판정하지 않는다. 현재 머신 snapshot은 진단 후 기록이며 공식 latency 실행의 전후 환경 원장이 아니다. 추가 native 고지와 운영 정책 자료를 갖춘 후 사람이 검토할 구체적인 출시 후보에 연결한다.
