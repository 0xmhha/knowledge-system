# C1 복구·구 데이터 보존 검증

2026-10-04 · **프리뷰 진단 통과 / C1-06 진행**. [작업리스트](./EXECUTION-WORKLIST.md)의 복구 준비 증거다. 최종 운영 후보·담당자·운영 복구 승인과 공식 품질은 남아 있다.

## 실행 범위와 결과

[원장](../../system/eval/b0-knowledge-system/recovery-m2max-2026-10-04/summary.json)은 FIX-15 macOS arm64 시험 패키지의 추출된 세 바이너리 SHA에 연결된다. 패키지는 Go 1.26.8, dirty preview이며 archive SHA는 `923bd83d9f0686f69f32a2b7349291f9b9a576e5aec82e0b4eb56dbe2524b57c`다. 이번 실행은 작은 인공 프로젝트와 mock 또는 합성 HTTP 임베딩을 사용한다. 실제 BGE-M3 품질·대규모 비용·Linux 운영 복구를 판정하지 않는다.

| 경로 | 관측 결과 |
|---|---|
| 새 후보 테스트 게이트 실패 | `/bin/false`의 실패 보고서를 보존하고 기존 current 유지 |
| 벡터 빌드 실패 | 전용 wrapper가 build에서 종료 코드 23을 반환; 부분 후보와 graph DB를 보존하고 기존 current 유지 |
| 손상된 이전 대상 | 보관 blob 변조 후 rollback이 snapshot_mismatch로 거부됨 |
| 정상 업데이트와 실행 중 MCP | current는 새 버전으로 이동; 이미 실행 중인 프로세스의 건강 상태·인용 커밋/파일은 이전 버전에 고정 |
| 정상 rollback과 새 MCP | 이전 dataset/snapshot/commit과 인용 재생 |
| 손상된 활성 버전 | 활성 blob 변조 후 이전 정상 버전으로의 직접 rollback도 snapshot_mismatch로 거부됨; 손상된 루트 보존 |
| 검증된 백업의 새 루트 복원 | 기존 루트를 덮어쓰지 않고 백업 버전을 새 루트에 복사한 뒤 rollback 명령으로 current 생성; 이전 신원/인용 재생 |
| 원본 소스 제거 | 전용 fixture 소스를 이동한 뒤 보관 원문으로 v1/v2 인용·본문 재생 |
| 구 데이터 마이그레이션 | 구 커밋 1ded9b3e47bc2e09062dba329c2f918423746fa4의 v1 소비자 재생; 새 소비자는 legacy_unpinned/reindex_required로 진단 |
| 새 strict 재색인 후 구버전 rollback | 별도 reindexed 버전을 만들고 v2 출처/integrity 확인; 이전 current로 돌아가 구 v1 소비자 재생 |

현재 버전의 payload 12파일과 구버전 7파일을 크기·SHA·실행 비트로 대조했다. 구버전은 **첫 신규 소비자 실행 전**의 해시를 기록했고, 재색인/rollback/백업의 해시가 같다. 구 v1 반환 commit·인용 파일·serviceable 값도 같다. 구 v1 조회에서 확정한 baseline 이후의 보존 증거이며, 구버전 최초 setup 이후 모든 소비자의 내부 동작을 불변이라고 주장하지 않는다.

SQLite graph/vector의 SHM과 **0바이트 WAL**은 조회·프로세스 종료에 따라 생성/삭제된다. 실측 파일 차이와 매 snapshot의 sidecar 크기/SHA를 별도 보존했다. payload 해시에서 제외한 경로는 이 네 파일뿐이고, 비어 있지 않은 WAL은 검증기가 거부한다. DB·manifest·identity·보관 원문·Git bundle 등 나머지 payload는 그대로 검사한다. 이는 live SQLite 파일을 단순 복사해도 안전하다는 보증이 아니다.

현재 복구 SDK 10응답/10인용과 재색인 SDK 1응답/1인용을 보존했다. [독립 재생 감사](../../system/eval/b0-knowledge-system/recovery-m2max-2026-10-04/independent-audit.json)는 11응답의 출처·본문과 v2 6응답의 canonical integrity, 이전/이후 payload 원장, typed 실패, pin/legacy 연결을 검사한다. DB를 다시 실행한 감사나 전체 검색 품질 판정은 아니다. 품질 지표는 null이다.

## 재현 명령

시험 서명/구조 검증을 마친 **추출 패키지**를 지정한다. 새 빈 출력 경로와 Git/Python을 준비한다. 구버전 실행은 이전 코드를 빌드하므로 호스트 Go 도구 체인도 필요하다.

```sh
python3 scripts/wbs-recovery-smoke.py \
  --bin-dir /path/to/verified-preview \
  --out /tmp/knowledge-recovery-new

KS_BIN_DIR=/path/to/verified-preview \
KS_LEGACY_SMOKE_DIR=/tmp/knowledge-legacy-recovery-new \
  python3 scripts/wbs-legacy-recovery-smoke.py

python3 system/eval/b0-knowledge-system/recovery-m2max-2026-10-04/audit-artifact.py
```

두 스크립트는 자체 임시 Git/데이터 루트에만 실패와 손상을 주입한다. 백업·부분 후보·손상 루트·실패 출력을 지우지 않는다. 구버전 HTTP 서버는 localhost 임의 포트의 합성 벡터 fixture이며 finally에서 종료한다. 실제 승인 모델 서버가 아니다.

초기 보조 도구의 기본 반복 수로 요청한 SDK 수집이 120초 제한에서 종료된 기록, 단일 JSON 출력을 디렉터리로 취급한 가정, v1에 v2 해시를 요구한 가정, sidecar 수명/읽기 전용 blob 권한 처리, legacy 출력 경로 누락과 최초 baseline 시점 보완을 원자료에 남겼다. 이를 제품 실패나 성공 점수로 합산하지 않는다. 정리한 저장소 스크립트 자체를 새 출력 디렉터리에서 다시 실행해 모두 통과했다. 최초 SDK 시간 제한의 원인은 확정하지 않았으며, 명시한 군별 1회/30초 호출 제한의 후속 실행을 통과 범위로 삼는다. 제품 Go 코드 변경은 없다.

## 손상된 활성 루트의 복원 절차

1. 해당 루트의 MCP와 writer를 종료하고 원래 current·버전·DB·manifest·identity·팩 잠금·보관 소스를 보존한다. 검증된 일관된 백업을 사용한다. 비어 있지 않은 WAL을 버리거나 실행 중인 DB 일부만 복사하지 않는다.
2. 손상된 활성 버전을 검증할 수 없으면 직접 rollback이 거부될 수 있다. current symlink를 수동 교체해 검증을 우회하지 않는다.
3. 백업 전체 버전을 새 루트의 단일 버전 디렉터리로 복원한다. 원본 루트는 남겨 둔다. 검증된 복원 대상에 아래 명령을 실행한다.

```sh
./cks rollback restored --out /path/to/fresh-dataset
./cks mcp gen-config --dataset-dir /path/to/fresh-dataset/current \
  --source-root /path/to/project \
  --sanitize-rules /path/to/verified-preview/policies/sanitization_rules.yaml \
  --embed-model YOUR_EMBEDDING_MODEL --out /path/to/new-runtime.yaml
```

4. 사용 모델/provider와 새 경로를 확인한 설정으로 새 MCP를 시작한다. 복원 대상의 dataset/snapshot/indexed commit·원문/본문 인용을 확인하고 운영 담당자가 서비스 전환을 판정한다. doctor의 현재 소스 HEAD와 indexed commit의 차이는 별도로 기록한다.

이 절차의 새 루트 복원은 위 소형 프리뷰에서 실행했다. 최종 패키지, 실제 운영 백업 방법·대규모 데이터·팩 잠금·서비스 중단/전환·지원 대상과 담당자, [운영 검토표](./C1-OPERATIONS-REVIEW.md)의 OP-08 판정은 남아 있다. 데이터 rollback과 운영 서명 키 교체/폐기는 각각 검토한다.
