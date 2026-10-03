# C1 출시 검토 준비

2026-10-04 · 운영 출시 판정 대기.

첫 검토는 `46020e4f`에서 당시 `cks` 바이너리에 연결된 28개 모듈의 root LICENSE/NOTICE를 수집했다. 실제 바이너리 build info는 `624efa89`+dirty이며 원장에 보존했다. 누락 0개, 모든 복사 SHA와 프로젝트·바이너리 SHA, CPU·메모리·OS·load/swap을 기록했다. [첫 검토 원장](../../system/eval/b0-knowledge-system/c1-review-preparation-m2max-2026-10-04/review-summary.json).

후속 재감사에서 패키지 수집기가 로컬 Solidity grammar의 LICENSE를 누락함을 발견했다. `cc590573`+변경 사항으로 macOS arm64의 세 바이너리를 다시 빌드한 preview는 연결 모듈 35개·root 고지 41개, graph/vector grammar 고지 각 1개와 소스 파일 8개 SHA를 포함한다. 모듈 캐시·저장소 원문·복사본의 해시를 대조했다. [패키지 재검증 원장](../../system/eval/b0-knowledge-system/native-license-packaging-m2max-2026-10-04/summary.json), [패키지 고지 목록](../../system/eval/b0-knowledge-system/native-license-packaging-m2max-2026-10-04/third-party-licenses.json).

독립 Python 검증기와 설치 후 `cks package verify`가 두 고지를 검사한다. 이전 Go 검증기가 원장과 다른 고지를 가진 재서명 패키지를 수락하는 실패를 재현했고 수정 후 거부했다. 정상 고지·이전 schema 1의 native 배열 없는 패키지는 통과한다. Python 21개 시험·관련 Go 시험/vet·경계 검사, 시험 서명 preview와 세 독립 저장소의 설치 스모크를 통과했다. 최초 Python 시험의 localhost 샌드박스 오류와 CLI 오류 원문 노출을 가정한 잘못된 assertion도 보존했고 수정 후 재실행했다.

두 preview를 비교해 build info에 임시 경로가 섞이는 재현성 문제도 발견·수정했다. 같은 `19fad138`+변경 사항으로 순차 빌드한 두 패키지의 52개 파일 SHA·크기·모드와 전체 archive SHA가 같고, 동일 시험 sidecar로 두 archive를 Python/Go에서 검증했다. [재현성 원장](../../system/eval/b0-knowledge-system/package-reproducibility-m2max-2026-10-04/summary.json). 이 검증의 범위는 현재 도구 체인·macOS arm64의 같은 소스 상태다.

후속 source 감사에서는 Go Tree-sitter runtime C/header 44개를 고정 upstream commit `16aaed78ae6582ea55a94419828922c7b0960e10`과 대조했고 모두 바이트가 같았다. 로컬 Solidity의 두 버전은 고정 upstream의 LICENSE·parser·header 각 3개가 같았다. graph/vector parser header는 각각 Tree-sitter v0.24.7/v0.25.8과 같고 해당 버전의 원문 고지가 같다. 이 범위의 upstream 출처는 확인했지만 모든 native 자산의 출처·적합성을 확정하지 않는다. [원자료](../../system/eval/b0-knowledge-system/native-notice-audit-m2max-2026-10-04/summary.json), [source recipe](../../third-party-notices/tree-sitter/provenance.json).

Tree-sitter runtime 원문 고지와 내부 Unicode LICENSE, Go toolchain 및 vendor 원문 5개를 추가했다. 세 바이너리의 모듈 35개·고지 50개·missing 0은 이 수집 범위의 결과다. Go vendor 고지는 보수적 포함 목록이며 전부 연결됐다는 뜻이 아니다. 현재 버전/소스 집합·바이트·고지 및 검토한 Solidity 바이트가 달라지면 packaging을 거부한다. 원장 전체의 review_status는 pending이다.

새 preview는 디렉터리를 포함해 110개 tar 항목이라 두 검증기의 기존 100개 상한에 걸렸다. Python과 Go의 수정 전 실패를 보존하고 상한을 256개로 맞췄다. 256개 통과·257개 거부, 기존 형식·고지 변조 거부를 시험했다. 압축 및 총 해제 크기 512 MiB·개별 파일 256 MiB·경로/타입/중복 검사도 유지한다. Python 11개 회귀(Go CLI 연동 포함), 관련 Go compile/vet, 실제 시험 서명 preview의 두 검증기와 세 독립 프로젝트의 설치·재시작·업데이트·롤백이 통과했다. Unicode 고지 한 개만 바꾼 재서명 preview도 둘 다 거부했다. 해당 Go 패키지는 Go test 파일이 없어 compile-only이며 실제 검증 동작은 위 통합 시험으로 확인했다.

| 결정·검증 | 현재 증거 | 완료에 필요한 항목 |
|---|---|---|
| Go 모듈 원문 수집 | 세 바이너리 연결 35개·root 고지 41개·누락 0·해시 확인 | 라이선스 적합성·고지 충족의 사람 검토 |
| 내장 native 자산 | 로컬 Solidity grammar 2개 고지·소스 8개 SHA·두 검증기의 변조 거부 | Tree-sitter 44개·Solidity 6개·JS/TS/SQLite/vec 41개 출처 확인; 전이/native/system-header/translated SQLite 범위와 적합성 검토 |
| 운영 서명 | 기존 signer는 test-signed-preview만 생성 | 운영 소유자·public key fingerprint·보관/회전/폐기 정책·신뢰 루트 승인 |
| 지원 플랫폼 | 이전 macOS/Linux mock preview 재검증 기록 | 최신 세 플랫폼 mock preview·FIX-14 Linux paired archive 통과; native amd64·실제 Linux Ollama·대규모 운영 비용 |
| 복구·마이그레이션 | 기존 A 구조 시험·문서 | 최종 C1 패키지의 이전 데이터 보존·rollback 재생 |
| 품질·출시 | 공식 품질 지표 null | 승인 입력/프로토콜·B0/B1·C0/C1 판정과 출시 결정 |

시험 개인 키·패키지/바이너리는 저장소에 넣지 않았으며 공개 시험 키는 운영 신뢰 루트가 아니다. 내장 자산 전체 고지나 적합성은 미판정이다. 이번에 대조한 6개 Solidity 파일의 출처와 나머지 native 자산의 미검증 범위를 구분한다. 첫 머신 snapshot은 진단 후 기록이며 공식 latency 실행의 전후 환경 원장이 아니다. 추가 native 감사와 운영 정책 자료를 갖춘 후 사람이 검토할 출시 후보에 연결한다.


2026-10-04 소스·플랫폼 후속: [원장](../../system/eval/b0-knowledge-system/native-source-platform-followup-m2max-2026-10-04/summary.json)의 JS v0.25.0·TS v0.23.2·go-sqlite3 v1.14.44·sqlite-vec pseudo-version은 module cache의 고정 origin commit에서 원문 41개를 읽어 모두 바이트가 같았다. 범위는 선택 CGO Go 파일·관련 C/header 디렉터리·root 고지다. 다른 언어 binding/example 및 모든 전이 system header를 포함한 완전한 소스 감사는 아니다. SQLite 본문의 고지 줄 관측과 Linux 빌드 이미지의 SQLite 헤더 SHA·배포 copyright·설치 버전도 보존했다. 적합성은 사람 검토 대기다.

고지 수집 후 호스트 스모크에 남아 있던 ‘자산 두 개’ 가정이 실제 네 자산 패키지를 거부했다. FIX-13에서 필수 소스/고지 전용 자산·중복·실제 count·원문 SHA를 각각 확인하게 수정했다. 수정 전 실패와 후 세 프로젝트 설치/재시작/업데이트/롤백을 보존했다. 최신 macOS는 35모듈·50고지, Linux arm64/amd64 에뮬레이션은 각각 33모듈·48고지다. OS와 Go 도구 체인이 모두 달라 단일 원인으로 이 차이를 해석하지 않는다. 두 Linux 고지 48개는 실제 빌드 이미지 Go 원문·읽기 전용 캐시·저장소와 같고, 두 시험 서명 검증기와 Go 없는 런타임의 세 프로젝트 설치/복구가 통과했다. native amd64·Linux 실모델은 미검증이다.

추가 검증에서 같은 Linux 바이너리의 반복 `ldd` 주소가 달라짐을 확인했다. packaging이 그 주소를 manifest에 보존하므로 FIX-14로 metadata 안정성을 수정해야 한다. 현재 Linux 서명/설치 통과를 재현성 통과로 세지 않는다. 늦은 빌더 소스 대조는 임시 컨테이너 종료로 수행하지 못했으며, 동결 입력 선언과 독립 실제 대조를 구분한다. 다음 우선순위는 FIX-14, 이후 운영 신뢰·지원·복구 검토 자료다.


2026-10-04 FIX-14 수정·검증: [재현성 원장](../../system/eval/b0-knowledge-system/fix14-linux-package-reproducibility-m2max-2026-10-04/summary.json)의 Linux ldd 주소만 안정화하고 첫 의존/정적 링크 진단을 유지했다. Python 15개 시험과 실제 기존 macOS 세 바이너리의 dependency 버전/신원 유지 확인을 통과했다. Linux 각 대상의 실제 builder copy-input 4,624개를 대조한 뒤 같은 합성 source commit으로 공개 packager를 두 번 실행했다. 두 fresh 빌드의 106개 항목과 전체 archive SHA가 같고 같은 시험 sidecar를 두 archive에 적용해 Python/Go 모두 통과했다. 실제 세 Linux 바이너리의 반복 dependency metadata와 세 프로젝트 설치·재시작·업데이트·rollback도 확인했다. FIX-14는 이 도구 체인/대상 범위에서 완료다. Linux 실모델·native amd64·운영 비용/최종 후보는 별도다.

운영 role·scope·키·신뢰 경로·교체/폐기·지원/복구의 실제 결정은 [C1 운영 검토표](./C1-OPERATIONS-REVIEW.md)에 분리했다. 현재 도구가 시험 scope만 검증한다는 사실과 아직 없는 운영 신뢰/효력 정책을 구분한다. public DER fingerprint와 실제 소유자/경로를 시험 키 원장에서 채우지 않는다. C1-05의 사람 적합성·운영 판정 및 C1-04/06/07 종료는 미완료다.
