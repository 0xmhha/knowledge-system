# 최신 FIX-19 패키지·실모델·복구 준비 검증

2026-10-05. [요약 원장](../../system/eval/b0-knowledge-system/latest-package-preview-m2max-2026-10-05/summary.json)과 [파일 해시 원장](../../system/eval/b0-knowledge-system/latest-package-preview-m2max-2026-10-05/evidence-manifest.json)은 `31def76b`의 최신 시험 범위를 기록한다. 최종 품질/운영 승인과 공식 C1 verdict는 null이다. 이전 FIX-15 패키지 증거로 FIX-18/19 수정본을 검증한 것으로 처리하지 않고 새 preview를 만들었다.

## 세 대상의 빌드·서명·설치

| 대상 | source/build 범위 | 모듈/원문 고지 | tar 항목 | 확인한 실행 |
|---|---|---:|---:|---|
| macOS arm64 | 31def76b + untracked 때문에 dirty-preview, Go1.26.8 | 35/51 | 111 | 시험 서명·Python/설치 후 Go 검증·변조 거부·세 mock 프로젝트 설치/재시작/업데이트/롤백 |
| Linux arm64 | 현재 tracked bytes의 별도 synthetic commit, Go1.25.13 | 33/49 | 107 | 별도 runtime 컨테이너의 설치/조회/업데이트/롤백·시험 서명 검증 |
| Linux amd64 | 같은 tracked bytes의 별도 synthetic commit, Go1.25.13·에뮬레이션 | 33/49 | 107 | 에뮬레이션 runtime의 설치/조회/업데이트/롤백·시험 서명 검증 |

각 archive SHA, 세 바이너리 SHA, manifest/inventory SHA와 실제 복사 고지의 바이트를 독립 대조했다. 포함 범위의 missing 고지는 0이며 원래 C/header·전체 전이 native·운영 고지 적합성 판정은 pending이다. 동일 macOS source를 두 번 빌드한 전체 archive SHA도 같다. 비바이너리 package 파일은 원래 경로·크기·SHA와 lossless text/base64로 보관했다. private key·DB·실행 파일·모델 blob은 Git에 넣지 않았다.

첫 Linux arm64 시도는 실제 GOMODCACHE가 아닌 경로를 지정하여 read-only module-cache 빌드에서 실패했다. 실패 로그를 보존하고 실제 경로로 재실행해 통과했다. 이 helper 설정 오류를 제품 실패로 분류하지 않았다. 사용자 `.claude/`와 `logs/`는 그대로 두었으며 Linux source 사본에는 untracked를 넣지 않았다.

## 설치 대상 바이너리의 실제 모델 확인

macOS archive에서 꺼낸 `cks`로 [FIX-19 제어](./B0-RETAINED-GUARD-DIAGNOSTIC.md)를 다시 실행했다. 실제 BGE-M3 SDK10응답/정상복원6·20인용·20본문/음성4가 기대한 코드·Go Verify·자기 source SHA를 통과했다. 원본/사본 payload와 바이너리/설정 SHA도 같다.

Linux arm64에서는 승인된 같은 BGE-M3 바이트를 read-only로 연결한 Ollama0.35.1 CPU 서버를 새로 띄웠다. 모델2CPU/4GiB와 별도 caller1CPU/512MiB의 제한·이미지 식별자를 보존했다. empty-go/TypeScript/unsupported-Python 세 소유 fixture의 실제 설치·벡터 빌드·조회·업데이트·과거 버전 rollback을 확인했다. rollback 후 SDK baseline/knowledge 각1응답, 전체6응답·8인용·8본문은 원문 줄/파일/본문 SHA·불변 좌표·public Go Verify를 통과했다. baseline/knowledge의 기본 인용·본문 집합은 같다. 원자료의 실제 rawK10/별도K6, HTTP200·measurement ID, 바이너리/설정/요청 전후 SHA와 저장 벡터의 finite/1024차원/정규화를 확인했다. benign backend non-returned도 원장에 남겨 전체 성공 호출로 바꾸지 않는다.

모델 blob 바이트 SHA·digest·버전·metadata가 유지됐고 residency의 size_vram=0이다. 공유 network 외 PID/CPU/메모리 namespace는 분리했다. 이번에 만든 서버는 종료/자동 삭제했고 read-only 원본 model store와 다른 컨테이너를 변경하지 않았다.

첫 실제 MCP probe는 90초 initialize deadline에서 실패했다. 종료 부근의 모델 HTTP400을 보존했으며, 고정 옵션으로 원래 startup anchor65개를 직접 재생하면 모두200이었다. 따라서 입력 자체의 제품 결함으로 단정하지 않았다. 진단 deadline180초를 명시한 집중 SDK 확인은 통과했다. 이를 기본90초의 합격이나 공식 지연 게이트 합격으로 취급하지 않는다. 첫 계획의 8arm×3fixture 진단24 SDK는 완료되지 않았고, 후속 확인은 별도의 baseline/knowledge6 SDK다. 실패 계획을 성공 분모로 바꾸지 않는다. 공식 warm/cold·대규모 비용·배포 초기화 한도는 별도 검토/측정이 필요하다.

## 최신 복구·기존 소비자 확인

macOS 최신 extracted preview의 작은 TypeScript/mock fixture에서 실패 후보 유지, 활성 pin/update/rollback, 손상 대상 거부, 새 루트에 backup clone 복원과 원본 소스 제거 후 재조회가 통과했다. 바이너리 SHA는 package manifest와 같다. 기존 손상 루트·실패 후보를 보존했으며 이것이 운영 backup/failover 복구 성공을 뜻하지 않는다.

고정 pre-WBS `1ded9b3` 소비자의 v1 조회와 최신 소비자의 reindex_required, mock v2 재빌드·legacy rollback·과거 v1 재조회를 재검증했다. 임베딩 서버는 deterministic synthetic HTTP다. 실제 운영 legacy 자료와 모델·키·담당자·복구 RTO/RPO의 migration 승인으로 합산하지 않는다.

## 현재 단계와 다음

세부 시험 준비는 완료했지만 전체30개 중 완료2/진행11/대기17·게이트0/4·미완료28개는 유지한다. B0-01/02/03/05·STV2-01 실제 결정과 source/팩/의미/scope/K 동결→B0-07 공식 strict 빌드→공식 B0/B1→사람 판정/관측 C0→독립 C1이 다음 경로다. 최종 후보는 공식 품질 및 수정 동결 후 다시 패키징해야 한다.

OP-01–08의 운영 역할/배포scope/키·독립 신뢰/개인키 접근·교체·폐기/전체 native 적합성/지원·비용/복구 담당 결정, 실제 native amd64·대규모 실모델 비용·최종 운영 백업/실패 전환·지원/출시도 남는다. 시험 키와 synthetic commit·fixture reviewer를 운영 책임자로 바꾸지 않는다. [전체 현황](./EXECUTION-STATUS.md)에서 남은 각 ID와 선행 조건을 확인한다.
