# Linux ARM64 실제 BGE-M3 진단

2026-10-04. C1-04 지원 환경 준비와 B0-06 측정 도구 연결의 소형 합성 진단이다. 공식 B0/B1 품질·지연 판정, 네이티브 amd64, 대규모 비용, 운영 키/출시 승인을 입증하지 않는다.

FIX-14의 linux/arm64 시험 서명 패키지(합성 source commit 331ff24cc6c7ffe30107a4c70fa1c728dc0f864d)를 독립 Python 검증기로 먼저 검증했다. 원래 release commit의 운영 후보가 아니다. 기존 FIX-14 원장에는 Python/설치 후 Go 서명 검사와 반복 archive SHA 증거가 있다.

Ollama 0.35.1 공식 arm64 이미지의 고정 platform digest는 sha256:149e245141f5d0370bd46f774229e87781276a44050a02e8fe7e35b82a15adb5다. bge-m3:latest 승인 manifest digest는 7907646426070047a77226ac3e684fbbe8410524f7b4a74d02837e43f2146bab다. 호스트 원본 manifest와 참조 blob 세 개의 SHA를 확인했다. 별도 model store에는 이 모델만 노출했고 실제 weight는 단일 read-only bind mount로 사용했다. NO_CLOUD=1, network none, 공개 포트 없음. 클라이언트는 서버의 loopback namespace만 공유한다. 호스트 Ollama와 다른 모델/키 디렉터리는 마운트하거나 수정하지 않았다.

모델 서버는 2 CPU/4 GiB, Go 없는 Debian 클라이언트는 1 CPU/512 MiB다. Docker VM 커널 노출값은 12 CPU/약 8 GiB이며 물리 자원 독점 증거가 아니다. caller와 model의 PID namespace가 다르므로 두 container inspect와 별도 API residency를 함께 읽는다. 환경 전후 두 snapshot은 순간 경쟁 작업 전체를 증명하지 않는다. size_vram=0은 이 실행의 CPU residency 관측이다.

API에서 한·영 입력 두 개를 truncate=false, num_ctx=8192/num_batch=8192로 처리하고 1024차원·유한값·정규화를 검사했다. 실제 설치/업데이트 빌드는 strict embeddings를 요구했다. empty Go, TypeScript, 미지원 Python은 모두 인공 입력이며 Python 분석기 지원 증거가 아니다. 원래 v1 MCP 연결/재시작·두 커밋의 빌드·이전 version rollback·보관 커밋 인용을 확인한다.

프로젝트마다 8개 arm × retrieval/warm/cold 각 1회로 수집했다. warmup은 0이며 공식 프로토콜의 5/20 반복과 다르다. cold는 MCP process 시작부터 첫 응답까지이고 모델 daemon/residency는 유지된다. 미검토 운영 사실/팩/관계 저장소가 없으므로 ontology 비기본 arm은 unavailable 폴백이다. 소형 guide의 fallback 계약을 확인하며 관계 검색의 품질 이점을 판정하지 않는다. 품질 지표는 null이다. 각 합성 요청의 날짜/subsystem은 진단 scope이며 공식 정적 질문의 범위 결정을 대신하지 않는다.

초기 owned archive 추출 helper는 Debian Python의 extractall(filter='data') 미지원으로 프로젝트 생성 전에 실패했다. 원문과 수정 전 helper를 보존했다. 사전에 signature/SHA/tar 구조를 검증한 owned archive에만 시스템 tar를 사용하는 진단 adapter로 재실행했다. 이후 TypeScript v1 MCP initialize는 기존 helper의 20초 client deadline에서 실패했다. 해당 stderr/log와 명령 exit=1을 보존했다. 제품/config/3초 backend timeout을 바꾸지 않고 별도 진단 client deadline만 90초로 늘려 남은 프로젝트를 계속했다. 재실행 성공은 기존 20초 조건 통과가 아니다. 원래 공개 probe와 다른 임시 helper라는 제한을 유지한다.

독립 검증기는 report/rows/footprint를 대조해 measurement ID, arm 회전, 실제 K20, HTTP 성공/경로, 모델 digest/차원/옵션/CPU residency, quota, binary/config/입력/DB/source HEAD/실행 비트 전후, lock 원문 사본의 해시/권한을 확인한다. 보관 snapshot의 파일/줄/본문 SHA와 v2 canonical integrity를 재계산하고 rollback indexed_commit과 현재 HEAD 차이를 확인한다. sqlite-vec shadow 저장소의 실제 할당 vector를 read-only로 읽어 청크 집합과 차원/유한값/정규화를 확인한다. 비심볼 Neighbors 등 내부 best-effort 실패를 성공 응답과 별도로 보존한다.

원자료의 private input/report/rows 권한은 실행 시 검사한 값이다. Git 보관 파일 권한으로 비공개 출력을 보증하지 않는다. model weight/DB/바이너리/archive/개인 키는 이 원장에 포함하지 않는다. 시험 공개 키는 운영 신뢰 루트가 아니다.

재현은 environment/run-real-install.py 및 resume-real-install.py, mcp-pin-probe-90s.py를 참조한다. 경로·CPU·model digest·platform image·signed package는 원장에 고정한 값으로 준비해야 한다. 독립 검증: python3 environment/independent-check.py <원래 scratch root>. 보관 원문에는 raw /out 경로가 남아 있으므로 작은 source/metadata를 복원하거나 원래 진단 scratch에서 실행해야 한다. 현재 수집물에서 SDK/source SHA 검증을 재생할 때는 projects/<kind>/matrix와 versions/<version>의 보관 파일을 연결한다.

보관 SDK/source 재생: `python3 environment/replay-evidence.py <이 원장 디렉터리>`. 실제 실행의 model/DB/권한/하드웨어를 다시 수행하는 검사는 아니다. prepare-* 파일은 당시 준비 스냅샷이며 직접 실행 재현 recipe가 아니다.
