# C1 출시 검토 준비

2026-10-04 · 운영 출시 판정 대기.

첫 검토는 `46020e4f`에서 당시 `cks` 바이너리에 연결된 28개 모듈의 root LICENSE/NOTICE를 수집했다. 실제 바이너리 build info는 `624efa89`+dirty이며 원장에 보존했다. 누락 0개, 모든 복사 SHA와 프로젝트·바이너리 SHA, CPU·메모리·OS·load/swap을 기록했다. [첫 검토 원장](../../system/eval/b0-knowledge-system/c1-review-preparation-m2max-2026-10-04/review-summary.json).

후속 재감사에서 패키지 수집기가 로컬 Solidity grammar의 LICENSE를 누락함을 발견했다. `cc590573`+변경 사항으로 macOS arm64의 세 바이너리를 다시 빌드한 preview는 연결 모듈 35개·root 고지 41개, graph/vector grammar 고지 각 1개와 소스 파일 8개 SHA를 포함한다. 모듈 캐시·저장소 원문·복사본의 해시를 대조했다. [패키지 재검증 원장](../../system/eval/b0-knowledge-system/native-license-packaging-m2max-2026-10-04/summary.json), [패키지 고지 목록](../../system/eval/b0-knowledge-system/native-license-packaging-m2max-2026-10-04/third-party-licenses.json).

독립 Python 검증기와 설치 후 `cks package verify`가 두 고지를 검사한다. 이전 Go 검증기가 원장과 다른 고지를 가진 재서명 패키지를 수락하는 실패를 재현했고 수정 후 거부했다. 정상 고지·이전 schema 1의 native 배열 없는 패키지는 통과한다. Python 21개 시험·관련 Go 시험/vet·경계 검사, 시험 서명 preview와 세 독립 저장소의 설치 스모크를 통과했다. 최초 Python 시험의 localhost 샌드박스 오류와 CLI 오류 원문 노출을 가정한 잘못된 assertion도 보존했고 수정 후 재실행했다.

| 결정·검증 | 현재 증거 | 완료에 필요한 항목 |
|---|---|---|
| Go 모듈 원문 수집 | 세 바이너리 연결 35개·root 고지 41개·누락 0·해시 확인 | 라이선스 적합성·고지 충족의 사람 검토 |
| 내장 native 자산 | 로컬 Solidity grammar 2개 고지·소스 8개 SHA·두 검증기의 변조 거부 | 모듈 내부 parser/header·SQLite 등 나머지 고지와 upstream 출처 범위 별도 감사 |
| 운영 서명 | 기존 signer는 test-signed-preview만 생성 | 운영 소유자·public key fingerprint·보관/회전/폐기 정책·신뢰 루트 승인 |
| 지원 플랫폼 | 이전 macOS/Linux mock preview 재검증 기록 | 최종 코드 패키지와 실제 Linux Ollama·대규모 운영 비용 |
| 복구·마이그레이션 | 기존 A 구조 시험·문서 | 최종 C1 패키지의 이전 데이터 보존·rollback 재생 |
| 품질·출시 | 공식 품질 지표 null | 승인 입력/프로토콜·B0/B1·C0/C1 판정과 출시 결정 |

시험 개인 키·패키지/바이너리는 저장소에 넣지 않았으며 공개 시험 키는 운영 신뢰 루트가 아니다. 내장 자산 전체 고지나 적합성은 미판정이다. grammar 소스 주석의 upstream 버전은 외부 검증 없이 출처 확정으로 취급하지 않는다. 첫 머신 snapshot은 진단 후 기록이며 공식 latency 실행의 전후 환경 원장이 아니다. 추가 native 감사와 운영 정책 자료를 갖춘 후 사람이 검토할 출시 후보에 연결한다.
