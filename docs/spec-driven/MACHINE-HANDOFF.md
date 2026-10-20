# 새 머신 개발 인수인계

2026-10-20 기준. 이 문서는 설치·재개 절차이고, 현재 진행 상황과 남은 전체 13묶음은 [현재 인수인계](./CURRENT-HANDOFF.md)와 [실행 원장](./REMAINING-13-EXECUTION.json)이 기준이다. 코드를 읽지 않고 완료 표시부터 옮기지 않는다. 후속 담당자는 아래 검증을 거친 뒤 다음 세부 작업을 진행한다.

## 1. 저장소와 증거 이력 복원

원격 저장소는 `https://github.com/0xmhha/knowledge-system.git`이다. 새 머신의 원하는 작업 디렉터리에서 실행한다. GitHub 접근 권한은 해당 머신에서 준비하며 인증 정보를 문서에 복사하지 않는다.

```sh
git clone https://github.com/0xmhha/knowledge-system.git
cd knowledge-system
git switch main
git pull --ff-only origin main
git fetch origin '+refs/tags/evidence/2026-10-20/*:refs/tags/evidence/2026-10-20/*'
python3 scripts/check-handoff-history.py --restore-generated-inputs
git switch -c codex/wiv1-subscription-generation
```

스쿼시된 main의 파일만으로는 과거 `git show REVISION:PATH` 검증이 불가능하다. [고정 이력 목록](./HANDOFF-HISTORY.json)에 기록한 원본 커밋들을 원격 evidence 태그로 보존한다. 태그는 완료된 개발 브랜치와 별개인 읽기 전용 증거 기준점이다. 브랜치가 제거되어도 위 fetch로 복원한다. 최초 별도 clone 시험에서 기존 run/·log ignore 규칙으로 빠진 manifest 증거38개를 발견했고 원래 해시 그대로 추가했다. 추가로 현재 native source binding에 포함된 viewer 정적 입력46개(약2.5MiB)도 보존했다. 복원 검사는 증거38개와 압축 보존된 viewer 입력46개의 해시를 확인하며, 동일 입력46개는 기본 build와 문서 경로 검사에도 필요하므로 원본 바이트로 추적하며, 생성 번들의 Git 줄 끝 변환을 막는다. 명시 복원 옵션은 누락된 viewer 입력만 생성한다. 이미 있는 파일이 다르면 덮어쓰지 않는다. 최초 점검이 실패하면 과거 보고서를 현재 내용으로 대체하지 말고 누락 객체/태그를 먼저 복구한다. shallow clone도 태그 fetch 이후 검사로 확인한다. [증거 복원 설명](./EVIDENCE-RECOVERY.md)을 함께 따른다.

## 2. 실행 도구와 캐시 위치

Python 3.10 이상, Git, C/C++ 빌드 도구, Go를 준비한다. 저장소의 기본 Go 최소 버전은 go.mod가 기준이고, 현재 CI 본체는 Go 1.26.9와 golangci-lint 2.14.0을 사용한다. 이전 macOS 본체 증거는 Go 1.26.8이었다. upstream go-stablenet native 시험은 별도 Go 1.23.12였으므로 본체 도구와 섞지 않는다. macOS와 Linux의 실제 실행 결과는 별도 기록한다.

```sh
python3 --version
go version
export PYTHONDONTWRITEBYTECODE=1
export GOCACHE="$(mktemp -d)"
export GOLANGCI_LINT_CACHE="$(mktemp -d)"
make build
make lint
make test-race
make manager-test
python3 scripts/check-refactoring-progress.py
python3 scripts/check-operational-acceptance.py
python3 scripts/factory_native_overlap.py
python3 -m unittest discover -s scripts -p 'test_factory_native_overlap.py'
```

의존 라이브러리가 없는 새 머신은 처음에 모듈 다운로드와 고정 lint 설치가 필요하다. 이전 머신의 GVM 경로나 오프라인 환경변수를 복사하지 않는다. lint 설치/선택은 Makefile의 고정 버전을 따른다. 검사 출력은 소유한 작업 디렉터리에 보존한다. 실행 종료와 자식 프로세스 종료를 확인한 뒤 이번 작업에서 만든 GOCACHE/GOLANGCI_LINT_CACHE, 임시 데이터와 생성 bin만 제거한다. 공용 모듈 캐시·기존 프로젝트·기존 DB를 지우지 않는다. `make`로 재생성 가능한 viewer 의존성도 필요할 때만 설치한다. 현재 검증에 결속된 정적 viewer 입력은 저장소에서 복원되며, 재빌드로 덮어쓰면 별도 source drift로 기록해야 한다.

## 3. clone에 있는 것과 다시 준비할 것

|구분|복원 방법과 주의점|
|---|---|
|KS 코드·설계·시험·증거|이 저장소와 evidence 태그. 최종 스쿼시 SHA는 실제 머지 결과로 확인한다. 마지막 native 연결 증거 checkpoint는 `e8f3d18252c22f923f70ea14440372a08677def1`이다.|
|실행 바이너리·SDK·캐시|clone에 포함하지 않는다. 현재 소스에서 새로 빌드하고 Go/OS/arch/commit/dirty 여부와 입력·출력 해시를 기록한다.|
|기존 검색 DB·Ollama 모델|clone에 전체 DB/모델이 들어 있다고 가정하지 않는다. 기존 DB를 옮길 경우 보존 소스/history·설정·지식 입력·모델 identity를 함께 백업/검증한다. 없으면 별도의 새 비운영 dataset으로 구축한다.|
|go-stablenet|`https://github.com/stable-net/go-stablenet.git`, commit `0bf2f4d1bfeb6605006d556957ef8c045d8f8ed8`. 원본은 변경하지 않고 후보별 소유 clone을 만든다.|
|go-wemix|`https://github.com/wemixarchive/go-wemix.git`, commit `1350376a680489298b3203a44d9ebddb7747f9cc`. 이전 비교의 source_mode는 working-tree이고 upstream의 cmd/gwemix 삭제가 있었으므로 commit만 같다고 같은 입력으로 판정하지 않는다.|
|coding-agent|`https://github.com/0xmhha/coding-agent.git`, 실제 역할 검증 commit `d1ebafb8ddf4eb487b8f1d6285135e174c40834b`. 설치 호스트는 Claude Code 2.1.300, 실제 hook/MCP와 로컬 시험 응답기를 사용했다. 실제 LLM 답변 품질 검증은 아니다.|
|ChainBench|고정 commit `bdf5ad2758662310b23c508e61b13ee318e92b40`와 채택 소스/객체는 아래 schedule-gap 증거가 기준이다. 새 checkout은 객체·파일 해시로 확인하며 저장소 URL을 추측하지 않는다.|

데이터 구축·복구는 [설치 절차](../INSTALLATION.md), [setup](../../system/docs/SETUP.md), [매니저 플러그인](./KNOWLEDGE-MANAGER-PLUGIN.md), [지속 갱신 계약](./KNOWLEDGE-MANAGER-UPDATES.md)을 따른다. baseline은 `bge-m3:latest`, digest `7907646426070047a77226ac3e684fbbe8410524f7b4a74d02837e43f2146bab`, 1024차원이다. 모델명만 같은 것으로 승인하지 않는다. 원래 dataset/project ID·pin·설정과 별도 재구축 ID를 구분하고 go-stablenet/go-wemix 검색 결과의 project/source provenance를 검사한다.

기존 자료의 `/Users/kevin/...`와 `/private/tmp/ks-*`는 당시 실행의 출처 기록이다. 특히 최근 단계89/90의 시험 루트·캐시·바이너리는 검증 후 삭제했다. 그 경로를 live 경로로 사용할 수 없다. retained-origins-v2 DB는 별도 로컬 자원이며 원격 clone으로 복구되지 않는다.

## 4. 최근 증거의 재검토와 실제 재실행 구분

증거 루트는 `docs/spec-driven/operational-preparation/source-review-supplement`이다.

|패킷|이미 확인한 범위|남은 한계|
|---|---|---|
|followups/guardian-deadline-diagnosis-v1|최초 종료 시한 실패와 시험 자체의 pre-return marker 혼동을 구분. 생산 guardian 변경 없이 시험 보완. 본체 일반/race 각각 4273 pass, 18 skip.|과거 최초 실패의 내부 순서를 소급 확정하지 않음.|
|followups/wiv1-contract-consumer-review-v1|12계약과 설치 소비자 12기준 검토, 기존 증거 범위 대조.|전체 native 12계약 수용 아님.|
|followups/latest-four-role-api-v1|실제 4역할의 partial·미승인 전달 거부, 실제 API 오류 12종×10노드, CKS MCP 오류35종.|승인된 양성 독립 oracle·최종 HEAD 품질 수용 아님.|
|followups/factory-native-overlap-v1|원래 60초 timer·실제 peer sync·factory owner·같은 세션 RPC callback 취소 연결. 일반/race 10슬롯, 실제20노드. 원본/별칭 대기4, 후보 drain→삭제/ack4, 조기반환 대조2. 회귀121종 통과.|선언 gate 아래 done=false/Err 분기. true 진행 알림·transport close·모든 post-ack 사용·독립 수용 미증명.|
|schedule-gap-v1/adopted-source/chainbench|채택 source·Git 객체·hash 보존.|소스 존재만으로 새 머신 설치·운영 수용을 주장하지 않음.|

원자료 checksum/의미 검증은 Python 검사로 다시 할 수 있다. native 재실행은 별도 작업이다. 최근 패킷 drivers의 gzip 소스와 초기 실패/명시 corrective1을 먼저 읽고 소유 작업 디렉터리에 추출한다. 고정 절대 경로·Go SDK·원본 checkout·출력 경로를 새 머신 경로로 바꾸는 별도 실행 adapter를 작성하고 그 diff를 기록한다. 원본 packet/manifest/summary를 재봉인하거나 덮어쓰지 않는다. 기존 candidate overlay는 followups/ticket-dev-gold-v1/private/overlays, 실제 owner 범위 승인은 followups/owner-schedule-contract-v1/ADOPTION-MANIFEST.json이 기준이다. timer 단축·가짜 mux event·가짜 ack로 같은 검증이라고 주장하지 않는다. 새 실행은 새 패킷에 출처·입력 해시·schedule·receipt·정리 결과를 남긴다.

## 5. 다음 작업과 완료 기준

다음은 WI-V1: 실제 owner ack 이후 **옛 구독의 모든 channel-use**와 **명시적 새 등록의 세대 경계**를 관측한다. 같은 channel의 새 등록은 허용된 계약이다. ack 시점과 RPC wire true를 혼동하지 않고, 실제 owner의 등록/삭제/ack와 send 대상의 세대를 trace로 연결한다. 정상 경로와 위반 대조군을 구별하고, 실제 owner4개 일정과 별도 통제 late-drain1개를 합쳐 독립 표본으로 세지 않는다. 요구사항·실제 소스·검증 기준의 누락을 먼저 대조하고 이후 구현한다.

각 세부 작업마다 목적/범위 명세 → 구현/실제 경로·대조군 검증 → receipt/hash/시도 실패 보존 → 문서·원장 업데이트 → 서명 가능 환경에서 커밋한다. 완료되지 않은 상위 작업에 완료 체크를 하지 않는다. 새 소스 변경으로 기존 source binding이 달라지면 새 검증을 남기며 기존 캡처의 source hash를 바꾸지 않는다. 매 단계 화면에 검증 결과·현재 단계·다음 작업·남은 전체 목록을 보여준다.

남은 전체는 [현재 인수인계의 13묶음](./CURRENT-HANDOFF.md)과 원장의 항목을 그대로 이어간다. 정식 수용은 WI-1/2/3/4/V1/V2/6/SP/C **0/9**, 상위 **0/13**이다. 개별 연결 시험 성공이 독립 품질 승인·운영 적용·전체 완료를 뜻하지 않는다.

D04는 미정이고 익명 역할을 허용한다. 실명·연락처를 요구하지 않는다. 새 DEV/FINAL 표본·독립 완전 정답/매칭/충분성·F01~08 승인/동결 전 `execution_allowed=false`, FINAL0, freeze=null을 유지한다. 온톨로지 평가는 동일 corpus/모델에서 off/on 검색·개념/구현 연결과 독립 정답이 필요하다. 실제 LLM 평가는 모델·인증·총 비용 상한 결정 뒤 진행한다. CoreML은 사용자 제외, 보안 전용 평가는 사용자 보류로 유지한다.

## 새 세션에 전달할 프롬프트

> docs/spec-driven/MACHINE-HANDOFF.md와 CURRENT-HANDOFF.md, REMAINING-13-EXECUTION.json을 읽고 HANDOFF-HISTORY.json의 이력을 복원·검증해 주세요. 기존 증거와 독립 수용을 구분하고 현재 코드/실제 경로를 대조한 뒤 WI-V1의 owner ack 후 옛 구독 channel-use 및 새 등록 세대 경계부터 진행하세요. 기존 원본/DB/전역 설정은 보존하고 시험 자원은 소유 디렉터리에 만드세요. 매 단계 명세·검증·실패 시도·문서·커밋을 남기고 현재 단계/다음 작업/남은 전체13묶음을 보여주세요. D04 미정, 익명 역할 허용, 정식0/9, freeze=null, 실제 LLM 비용 호출 보류, 사용자 제외/보류 범위를 유지하세요. 삭제된 이전 머신 임시 경로가 존재한다고 가정하지 마세요.
