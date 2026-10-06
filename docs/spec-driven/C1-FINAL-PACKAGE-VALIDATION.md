# 최종 동결 소스의 패키지·실모델·비용·복구 검증

2026-10-06. 제품 소스 동결 후 로컬 checkpoint `10cbffaa104e062052e34d19a27cec3ee8b3548c`에서 다시 만든 **시험 preview**다. [원장](../../system/eval/b0-knowledge-system/final-package-preview-m2max-2026-10-06/summary.json), [원자료 SHA](../../system/eval/b0-knowledge-system/final-package-preview-m2max-2026-10-06/evidence-manifest.json), [전체 보관 목록](../../system/eval/b0-knowledge-system/final-package-preview-m2max-2026-10-06/archive-inventory.json)을 연결한다. 제품 소스 1,096개·승인 입력 5개의 SHA는 개발 동결과 동일하다. 공식 평가 바이너리와 새 패키지 바이너리는 VCS/빌드 메타데이터·SHA가 다르므로 바이트 동일성을 주장하지 않는다.

## 세 플랫폼의 현재 패키지

| 대상 | 도구 체인·실행 | archive SHA-256 | 검증 범위 |
|---|---|---|---|
| Darwin ARM64 | Go 1.26.8 / M2 Max | `6435e1dd56fe5ea3a5fa458e34620ece3e39c3524f2ec9eb5830908400eb7ec2` | 시험 서명·독립 Python/설치 후 Go 검증·세 mock 프로젝트 설치/재시작/업데이트/롤백·실제 BGE·대규모 비용·별도 RSS·실패 복구 |
| Linux ARM64 | Go 1.25.13 / Docker VM | `9155a589885bf601f9f7dd81667019ac8b9c356ef859478e2243b4ae5d1db3af` | Go 없는 Debian 런타임의 서명/세 mock 설치·실제 CPU BGE 세 프로젝트 설치/업데이트/롤백·SDK |
| Linux AMD64 | Go 1.25.13 / ARM64 위 에뮬레이션 | `7a96bf63bae4f8ef7a2045b386f8ac2fa21ac031c629a89603ea10aa1377e7c6` | 서명·세 mock 설치/재시작/업데이트/롤백. 네이티브 실제 모델·비용은 미확인 |

Darwin은 사용자 untracked 파일 때문에 dirty-preview다. Linux는 Git tracked 바이트만 복사한 합성 fixture commit(`a630e838fcb9`, `36e175bd021f`)이며 원본 release commit의 운영 빌드로 취급하지 않는다. 사용자 `.claude/`, `logs/`는 보존·제외했다. Darwin 동일 조건 재빌드의 archive SHA가 일치했다. 각 archive의 세 바이너리 SHA·manifest/inventory·고지 원문을 독립 대조했고 Darwin 35모듈/51고지/111항목, Linux 각각 33모듈/49고지/107항목이다. 포함 범위의 누락 고지는 0이며 법적 적합성은 pending이다. 시험 개인 키는 임시 디렉터리에만 있고 보관 자료에는 공개 키·sidecar·서명만 포함한다.

## 실제 모델과 대규모 비용

Darwin 추출 패키지 `cks` SHA는 `b420a559fb84e3520a5dcf5d420f067d6f38ded821479784e515063cd822d6e8`이다. 승인 DEV F-04/F-05의 정상/복원 6·음성 4, 전체 SDK 10건은 mixed snapshot/변조 blob/누락 source의 기대 코드와 공개 Go 계약·독립 원문/좌표 검사를 통과했다. 원본/사본·설정·바이너리 전후 SHA를 확인했다. 이는 FINAL 질문을 개발 튜닝에 다시 사용한 검사가 아니다.

승인 strict 전체 데이터 1,598파일·13,820벡터를 별도 사본으로 조회했다. DEV CODE-01 한 질문의 지식 OFF/ON 각각 warmup 2·검색 5·warm 20·process cold 3, **60 SDK·480인용/480본문**의 원문·공개 Go 계약·measurement ID/backend 결합이 유효하다. [비용](../../system/eval/b0-knowledge-system/final-package-preview-m2max-2026-10-06/checks/large-cost.json), [호출·크기 감사](../../system/eval/b0-knowledge-system/final-package-preview-m2max-2026-10-06/large-cost/source-backend-public-audit.json)를 따른다.

| 축 | warm p50 / p95 | process cold p50 / p95 |
|---|---|---|
| OFF | 1.637 / 1.682초 | 3.993 / 4.011초 |
| ON | 1.926 / 1.995초 | 4.269 / 4.277초 |

단일 질문의 비회전 반복으로 독립 품질 표본이나 공식 FINAL 지연 게이트가 아니다. 모델은 resident이고 호스트 자원은 독점하지 않았다. 비용 실행 중 작업 소유 빌드/다른 모델 요청을 겹치지 않았다. 실제 CKV K는 호출별 10/6이다. 논리 backend 1,980회·HTTP 609회, canonical compact response JSON 총 1,452,000바이트다. 선택적 CKG neighbors 오류 **360회**를 보존했다. 응답 계약이 유효하다는 사실을 모든 backend 호출 성공으로 바꾸지 않는다. HTTP 응답 바이트와 SDK JSON 크기는 transport wire 측정과 구분한다.

별도 2 SDK·16인용/본문의 RSS 진단도 원문/공개 Go·계측 결합을 통과했다. 250ms 표본 23개에서 MCP 최대 관측 RSS 82,528KiB, capture 프로세스 23,520KiB, 합 최대 106,144KiB였다. 공유 페이지 중복·표본 사이 peak 누락·sampler overhead가 가능하고 모델 서버/runner는 제외했다. 지연 표본과 합산하지 않는다. 원본/사본 DB·source/identity·모델 버전/digest·바이너리 SHA는 불변이다.

Linux ARM64는 고정 Ollama 0.35.1 image와 승인 BGE-M3 바이트를 read-only로 마운트한 별도 서버(4CPU/4GiB), 별도 caller(1CPU/512MiB), 외부 network/공개 port 없는 환경이다. 세 프로젝트의 install/update/rollback 후 baseline/knowledge **6 SDK·8인용/8본문**이 원문·좌표·public Go·backend K/HTTP200·설정/요청/바이너리 전후 SHA를 통과했다. 저장 임베딩은 1024차원·finite·정규화됐고 digest/모델 blob 전후 SHA가 같다. 선택적 neighbors 오류 6회도 보존했다. 작은 합성 설치이며 unavailable knowledge fallback에서 기본 근거가 같다. 180초 initialize/call deadline의 진단으로 기본 deadline이나 운영 지연 합격을 주장하지 않는다. 이 실행의 GPU residency를 별도로 측정하지 않았다. 소유 서버만 제거했고 사용자 Ollama는 그대로 유지했다.

## 최종 패키지 복구

최신 추출 Darwin 패키지에서 후보 테스트/벡터 빌드 실패 보존, 실행 중 MCP pin, 업데이트/rollback, 손상 대상/활성 source 거부, 일관된 백업의 새 루트 복원, 원본 source 없이 retained 인용 재생을 확인했다. 구 `1ded9b3` 소비자 v1과 첫 신규 소비자 전 payload SHA, 신규 strict 재색인, 구 current rollback·구 v1 재조회도 확인했다. 기존 version 12파일·legacy 7파일의 payload SHA 보존과 비어 있는 WAL/SHM 별도 원장을 따른다.

[독립 복구 감사](../../system/eval/b0-knowledge-system/final-package-preview-m2max-2026-10-06/independent-audit.json)는 11 SDK/11인용·v2 integrity 6응답·typed 실패·pin·이전/이후/backup payload를 재생한다. 손상된 updated blob은 원본 그대로 따로 보존했고, 해시가 일치하는 독립 fixture source로 정상 SDK 당시의 보관 원문을 복원해 감사했다. 운영 legacy 자료·실제 backup/failover·담당자/RTO/RPO/키 교체는 이 작은 mock/합성 HTTP 시험의 범위 밖이다.

최초 보관 스크립트의 과거 경로 참조, 검사 코드 안의 PRIVATE KEY 표지 문자열 오탐, 재개 때 같은 경로의 구 SHA 중복은 실패 로그/원문을 보존하고 보관·감사만 수정했다. 제품/질문/실모델 요청·지연 측정을 다시 실행하거나 결과를 덮어쓰지 않았다. 최종 원장 각 lossless 파일과 package 비바이너리 자산을 재검사했다. [원자료만으로 하는 출처 재생](../../system/eval/b0-knowledge-system/final-package-preview-m2max-2026-10-06/archive-source-replay.json)은 대규모 62+Linux 6 SDK의 504인용을 TEMP 원본 없이 검증했다.

## 종료 범위

C1-04는 최신 세 대상 preview·가용한 두 플랫폼 실제 모델·전체 strict 입력 비용·환경 없는 AMD64의 pending 표시 조건에서 검증 완료다. C1-06은 최종 패키지의 구 데이터 보존/실패·재색인/이전 current 복구 조건에서 검증 완료다. 운영 OP-08은 별도 미완료로 남긴다. [FINAL 품질](./C1-APPROVED-FINAL-REPORT.md)은 fail·소그룹 inconclusive이고 온톨로지는 disabled를 유지한다. B1-06 비용 보고와 C1-07 지원/출시보류 보고는 이후 [완료조건 재검토](./B1-COST-REPORT.md)로 닫았다. B1-04/07/08의 사람 해석·기권, C1-05 운영 신뢰/적합성·scope 결정은 남는다. 전체 완료나 운영 출시 승인으로 표기하지 않는다.
