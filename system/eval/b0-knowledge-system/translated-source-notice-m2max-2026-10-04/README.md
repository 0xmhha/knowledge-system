# Translated SQLite·전이 소스와 추가 고지 재검증

2026-10-04 · FIX-15 구조 수정 완료 범위. C1-05 적합성/운영 출시는 pending이며 공식 B0/B1/C0/C1 품질은 null이다.

기존 실제 시험 서명 Linux archive의 modernc.org/sqlite v1.54.0 원장에는 Go wrapper LICENSE만 있었다. 모듈은 별도 SQLITE-LICENSE도 배포하지만 root prefix scan이 놓쳤다. source SHA·기존 inventory·수정 전 회귀 실패를 보존했다. 해당 고정 버전의 required native notice 경로에 SQLITE-LICENSE를 추가했다. 파일이 없으면 wrapper LICENSE가 존재해도 missing_source_license와 missing_required_notices를 기록한다. wrapper 고지와 upstream 고지를 모두 복사하고 바이트를 보존한다. 고지 내용의 공개 설명은 [SQLite 공식 copyright 문서](https://www.sqlite.org/copyright.html)를 참조한다. 이 원장의 포함/해시 검사가 법적 적합성 판정은 아니다.

Darwin arm64 Go1.26.8, Linux arm64 및 amd64 에뮬레이션 Go1.25.13에서 실제 package-host로 세 바이너리를 새로 빌드했다. macOS dirty-preview는 base 7b5d7bb8+수집기 변경이며 Linux는 5,230개 추적 입력 bytes/실행 비트를 빌더에서 검사한 합성 commit 854464d9383eb8864edab1d1ed66f1e97adafd5d다. 원래 release commit의 운영 후보가 아니다. 두 Linux 대상은 동일 합성 commit이고 서로 다른 대상의 archive SHA 동일성은 주장하지 않는다. 실제 공개 경로는 cmd/graph·cmd/vector·cmd/cks다. 처음 cmd/ckg·cmd/ckv를 사용한 0개 선택/명령 오류를 보존하고 실제 build info 경로로 바로잡았다.

모든 새 패키지에서 wrapper와 SQLITE-LICENSE, libc의 기존 LICENSE-3RD-PARTY.md 포함과 원문 SHA를 확인했다. Darwin은 35모듈/51고지/111 tar 항목, Linux 두 대상은 각각 33모듈/49고지/107항목이다. 기존 missing=0은 그때 선언한 수집 범위의 결과로 유지하고 새 고지 누락 발견을 전체 적합성 통과로 해석하지 않는다.

선택된 modernc 네 모듈의 source/notice를 고정 Go module ZIP의 해당 member와 바이트 대조했다. Darwin은 30 package/225 file/13,390,778 bytes, Linux arm64는 8/197/13,934,278, amd64는 8/201/14,668,056이다. 선택된 GoFile 목록은 package dependency 선택이며 모든 파일/함수가 최종 linker에 남았다는 뜻은 아니다. 모듈 ZIP h1은 go.sum과 실제 binary build info의 버전/h1에 연결된다. 호스트·각 Linux builder의 go mod verify도 통과했다. 네 모듈의 root 고지 9개도 ZIP과 같았다. Origin URL/commit/tag는 Go download cache metadata다. GitLab raw 웹 조회는 cache miss로 실패했으므로 고정 Git blob과 직접 비교했다고 기록하지 않는다.

lib/sqlite.go의 선언값은 SQLite 3.53.3/source ID d4c0e51e4aeb96955b99185ab9cde75c339e2c29c3f3f12428d364a10d782c62다. generated Go를 원래 C/generator·입력 header로 다시 생성한 증거는 아니다. platform Go file의 header에는 libc/libz/libtcl include 경로가 있으나 그 경로를 원래 외부 source/라이선스의 완전한 검증으로 세지 않는다. 원래 C/ABI/system header 및 다른 native 전이 범위·고지 적합성은 사람 검토와 남은 source 감사에 연결한다.

새 시험 서명 패키지는 각각 Python·설치 후 Go 검증기를 통과했고, 추출한 바이너리로 세 독립 mock 프로젝트의 설치·조회·v1 MCP 재시작·두 version·rollback을 확인했다. Go 없는 Linux Debian runtime의 같은 경로도 통과했다. SQLITE-LICENSE 한 파일만 바꾼 재서명 archive는 세 대상의 두 검증기 모두 거부했다. signer의 키 보유 증명과 내부 고지 SHA 검사를 구분한다. 임시 동일 시험 키를 대상별 scope가 있는 sidecar에 사용했으며 운영 키/신뢰 루트가 아니다.

Python 16개 회귀(5 수집/6 signed Go 연동/1 source recipe/4 dependency)가 통과했다. Go 코드 변경은 없어 전체 Go suite를 반복하지 않았으며 실제 새 바이너리/검증기와 세 플랫폼 설치는 재실행했다. 본 턴에는 실제 BGE 모델을 실행하지 않았다. 이전 실모델 진단은 FIX-14 패키지의 제한된 증거로 유지한다. 운영 역할·scope·키/폐기·지원·최종 복구·출시와 공식 평가 승인은 미완료다.

environment/audit-translated*.py·build-linux.py·verify-darwin.py·verify-linux.py는 당시 실제 명령 recipe이고 경로를 원래 scratch/cache/toolchain에 맞춰야 한다. 각 target의 verification·translated-source-audit·source-copy-audit·원시 로그와 source/scripts의 수정 내용을 함께 읽는다. image-inspect-after는 named tag의 실행 후 metadata snapshot이며 종료된 컨테이너의 실행 상태를 재구성하는 증거가 아니다. evidence-manifest의 모든 파일 SHA를 대조할 수 있다. 바이너리/archive/DB/model/개인 키와 사용자 .claude/logs 파일은 보관하지 않는다.
