# 새 머신 개발 인수인계

2026-10-20 기준. [별도 checkout·외부 소스 복원 검증 기록](./HANDOFF-VALIDATION.json)을 보존했다. 이 문서는 설치·재개 절차이고, 현재 진행 상황과 남은 전체 13묶음은 [현재 인수인계](./CURRENT-HANDOFF.md)와 [실행 원장](./REMAINING-13-EXECUTION.json)이 기준이다. 코드를 읽지 않고 완료 표시부터 옮기지 않는다. 후속 담당자는 아래 검증을 거친 뒤 다음 세부 작업을 진행한다.

## 1. 저장소와 증거 이력 복원

원격 저장소는 `https://github.com/0xmhha/knowledge-system.git`이다. 새 머신의 원하는 작업 디렉터리에서 실행한다. GitHub 접근 권한은 해당 머신에서 준비하며 인증 정보를 문서에 복사하지 않는다.

```sh
git clone https://github.com/0xmhha/knowledge-system.git
cd knowledge-system
git switch main
git pull --ff-only origin main
git fetch origin '+refs/tags/evidence/2026-10-20/*:refs/tags/evidence/2026-10-20/*'
python3 scripts/check-handoff-history.py --restore-generated-inputs
git switch -c feat/wiv1-progress-true-generation
```

스쿼시된 main의 파일만으로는 과거 `git show REVISION:PATH` 검증이 불가능하다. [고정 이력 목록](./HANDOFF-HISTORY.json)에 기록한 원본 커밋들을 원격 evidence 태그로 보존한다. 태그는 완료된 개발 브랜치와 별개인 읽기 전용 증거 기준점이다. 브랜치가 제거되어도 위 fetch로 복원한다. 최초 별도 clone 시험에서 기존 run/·log ignore 규칙으로 빠진 manifest 증거38개를 발견했고 원래 해시 그대로 추가했다. 추가로 현재 native source binding에 포함된 viewer 정적 입력46개(약2.5MiB)도 보존했다. 복원 검사는 증거38개와 압축 보존된 viewer 입력46개의 해시를 확인하며, 동일 입력46개는 기본 build와 문서 경로 검사에도 필요하므로 원본 바이트로 추적하며, 생성 번들의 Git 줄 끝 변환을 막는다. 명시 복원 옵션은 누락된 viewer 입력만 생성한다. 이미 있는 파일이 다르면 덮어쓰지 않는다. 최초 점검이 실패하면 과거 보고서를 현재 내용으로 대체하지 말고 누락 객체/태그를 먼저 복구한다. shallow clone도 태그 fetch 이후 검사로 확인한다. [증거 복원 설명](./EVIDENCE-RECOVERY.md)을 함께 따른다.

## 2. 실행 도구와 캐시 위치

Python 3.10 이상, Git, C/C++ 빌드 도구, Go를 준비한다. 저장소의 기본 Go 최소 버전은 go.mod가 기준이고, 현재 CI 본체는 Go 1.26.10과 golangci-lint 2.14.0을 사용한다. 1.26.9에서 crypto/x509 공지(GO-2026-6809)가 나와 2026-10-21에 올렸다. 이전 macOS 본체 증거는 Go 1.26.8이었다. manager broker 시험 fixture가 `go 1.26.9`를 요구하므로 `GOTOOLCHAIN=local`로 1.26.8을 고정하면 11개가 실패한다. CI는 race 검출기 없이 `make test`만 실행한다. race 시험(`make test-race`)은 PR을 올리기 전에 로컬에서 통과시킨다(2026-10-21 결정). 본체 검증은 CI와 같은 Go 버전으로 실행한다(2026-10-20 새 머신 기록: [검증 이력](./operational-preparation/source-review-supplement/followups/wiv1-generation-boundary-v1/VALIDATION.json)). upstream go-stablenet native 시험은 별도 Go 1.23.12였으므로 본체 도구와 섞지 않는다. macOS와 Linux의 실제 실행 결과는 별도 기록한다.

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
python3 scripts/wiv1_generation_boundary.py
python3 -m unittest discover -s scripts -p 'test_wiv1_generation_boundary.py'
python3 scripts/wiv1_transport_generation.py
python3 -m unittest discover -s scripts -p 'test_wiv1_transport_generation.py'
python3 scripts/wiv1_notify_transport_generation.py
python3 -m unittest discover -s scripts -p 'test_wiv1_notify_transport_generation.py'
```

### scripts 시험 전체

`scripts/` 아래 시험 전체는 CI가 돌리지 않는다. 새 머신에서는 아래처럼 준비한 뒤 실행한다.

```sh
KS_VENV="$HOME/.venvs/knowledge-system"
python3 -m venv "$KS_VENV"
"$KS_VENV/bin/pip" install 'Cython==0.29.37' 'setuptools==80.9.0' 'wheel==0.45.1'
"$KS_VENV/bin/pip" install --no-build-isolation 'PyYAML==6.0'
"$KS_VENV/bin/pip" install -r scripts/requirements.txt
make build
"$KS_VENV/bin/python" -m unittest discover -s scripts -p 'test_*.py'
```

venv는 저장소 밖에 만든다. `.gitignore`는 현재 검증기 여러 개가 바인딩한 입력이라 venv 때문에 바꾸지 않는다.

운영 수용 문서의 현재 준비 근거(guardian 패킷)는 Go 파일 전체와 CI workflow의 해시를 묶는다. 그래서 코드가 바뀌면 살아 있는 트리와 맞지 않는다. 두 문서의 `current_readiness_source_revision`에 패킷이 마지막으로 통과한 main 커밋을 적었다. 검사기는 그 커밋이 HEAD의 조상이고 패킷이 그 뒤로 바뀌지 않았는지 확인한 뒤, 그 커밋의 worktree에서 그때 코드로 패킷을 검증한다(2026-10-21, CI 수정 뒤). 같은 결속을 쓰는 리팩토링 진행 검사도 `REFACTORING-PROGRESS.json`의 `active_source_binding.source_revision`에 같은 커밋을 적고, 그 커밋이 HEAD의 조상인지 확인한 뒤 그 커밋 시점의 파일 해시와 Go 파일 목록을 비교한다(2026-10-21). 그 뒤 바뀐 코드는 이 두 검사가 잡지 않으므로 새 결속을 만들 때까지 race 시험과 CI로 확인한다.

- 외부 모듈은 [고정 목록](../../scripts/requirements.txt)의 jsonschema와 PyYAML뿐이다. 보존 패킷이 PyYAML 6.0을 parser 버전으로 기록했으므로 같은 버전을 쓴다. Python 3.12용 wheel이 없어 소스에서 빌드한다.
- `check-lexical-coverage.py`는 PATH의 `go`를 `GOTOOLCHAIN=local`로 실행한다. PATH의 `go`가 go.mod의 Go 1.25.13 이상이어야 한다. 1.25.12를 두면 87건이 버전 오류로 실패한다.
- Go 1.23.12 native 증거를 다시 계산하는 시험은 `KS_GO1_23_ROOT`(기본 `~/.gvm/gos/go1.23.12`)의 SDK를 쓴다. 기록된 이전 머신 경로는 [위치 대응](../../scripts/recorded_locations.py)이 도구 이름으로 바꾸고, 바이너리 해시는 기록과 비교한다.
- 과거 단계 시험 110개 모듈은 끝의 `load_tests` hook으로 [기록된 커밋](../../scripts/evidence-revisions.json)의 임시 worktree에서 돌린다. 이후 단계가 바인딩된 소스·도구·정식 상태를 바꿨기 때문이다. 그 커밋 이후 패킷 파일이 수정되거나 지워졌으면 실패한다. evidence 태그가 없어도 실패한다. 동작 방식은 [실행 모듈](../../scripts/evidence_revision.py)에 적었다.
- `test_stablenet_comparison`의 소유 index 재조회 1건은 건너뛴다. 원래 실행이 임시 루트에 만든 DB가 지워져 재현할 수 없다. 나머지 오프라인 검사와 변조 시험은 그대로 돈다.

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
|ChainBench|고정 commit `bdf5ad2758662310b23c508e61b13ee318e92b40`와 채택 소스/객체는 아래 schedule-gap 증거가 기준이다. 원격은 https://github.com/0xmhha/chainbench.git 이며 아래 bundle 절차로 실제 채택 커밋을 복원한다.|

데이터 구축·복구는 [설치 절차](../INSTALLATION.md), [setup](../../system/docs/SETUP.md), [매니저 플러그인](./KNOWLEDGE-MANAGER-PLUGIN.md), [지속 갱신 계약](./KNOWLEDGE-MANAGER-UPDATES.md)을 따른다. baseline은 `bge-m3:latest`, digest `7907646426070047a77226ac3e684fbbe8410524f7b4a74d02837e43f2146bab`, 1024차원이다. 모델명만 같은 것으로 승인하지 않는다. 원래 dataset/project ID·pin·설정과 별도 재구축 ID를 구분하고 go-stablenet/go-wemix 검색 결과의 project/source provenance를 검사한다.

기존 자료의 `/Users/kevin/...`와 `/private/tmp/ks-*`는 당시 실행의 출처 기록이다. 특히 최근 단계89/90의 시험 루트·캐시·바이너리는 검증 후 삭제했다. 그 경로를 live 경로로 사용할 수 없다. retained-origins-v2 DB는 별도 로컬 자원이며 원격 clone으로 복구되지 않는다.

### 외부 소비자 소스 복원

coding-agent d1eb와 ChainBench bdf5는 원격에 게시되지 않은 로컬 채택 커밋이다. 원격 SHA fetch만으로 복구되지 않음을 확인했다. 위 고정 이력 목록에는 각 bundle의 commit/tree/SHA-256을 기록했다. coding-agent는 완전한 history bundle, ChainBench는 원격에서 확인한 기반662fff 위의 차이 bundle이다. 원본 저장소는 수정하지 않았고, ChainBench의 기존5개 보존 bundle을 순서대로 복구해 완전한 Git tree도 확인했다. 다음은 새 소유 디렉터리에서 복원하는 예다.

```sh
KS_ROOT="$(pwd)"
KS_RESTORE="$(mktemp -d)"
git init "$KS_RESTORE/coding-agent"
git -C "$KS_RESTORE/coding-agent" fetch "$KS_ROOT/docs/spec-driven/handoff-inputs/coding-agent.bundle" HEAD
git -C "$KS_RESTORE/coding-agent" switch --detach d1ebafb8ddf4eb487b8f1d6285135e174c40834b
git -C "$KS_RESTORE/coding-agent" remote add origin https://github.com/0xmhha/coding-agent.git
git clone https://github.com/0xmhha/chainbench.git "$KS_RESTORE/chainbench"
git -C "$KS_RESTORE/chainbench" fetch origin 662fff77382eb031df687d371a8bc8f63cad581f
git -C "$KS_RESTORE/chainbench" bundle verify "$KS_ROOT/docs/spec-driven/handoff-inputs/chainbench.bundle"
git -C "$KS_RESTORE/chainbench" fetch "$KS_ROOT/docs/spec-driven/handoff-inputs/chainbench.bundle" refs/heads/handoff
git -C "$KS_RESTORE/chainbench" switch --detach bdf5ad2758662310b23c508e61b13ee318e92b40
```

새 경로를 다음 시험의 source_root에 명시하고 `git status`, commit/tree와 보존된 source-files 목록을 대조한다. bundle은 소스와 기존 라이선스/이력을 보존하며 의존성·설치된 CLI·런타임 바이너리·인증·DB를 포함하지 않는다. 소비자 의존성/빌드는 각 복원 checkout의 manifest와 문서를 따른다. 기존 KS 보존 packet 안의 절대 경로를 바꾸지 말고 새 실행 adapter에 매핑을 기록한다.

## 4. 최근 증거의 재검토와 실제 재실행 구분

증거 루트는 `docs/spec-driven/operational-preparation/source-review-supplement`이다.

|패킷|이미 확인한 범위|남은 한계|
|---|---|---|
|followups/guardian-deadline-diagnosis-v1|최초 종료 시한 실패와 시험 자체의 pre-return marker 혼동을 구분. 생산 guardian 변경 없이 시험 보완. 본체 일반/race 각각 4273 pass, 18 skip.|과거 최초 실패의 내부 순서를 소급 확정하지 않음.|
|followups/wiv1-contract-consumer-review-v1|12계약과 설치 소비자 12기준 검토, 기존 증거 범위 대조.|전체 native 12계약 수용 아님.|
|followups/latest-four-role-api-v1|실제 4역할의 partial·미승인 전달 거부, 실제 API 오류 12종×10노드, CKS MCP 오류35종.|승인된 양성 독립 oracle·최종 HEAD 품질 수용 아님.|
|followups/factory-native-overlap-v1|원래 60초 timer·실제 peer sync·factory owner·같은 세션 RPC callback 취소 연결. 일반/race 10슬롯, 실제20노드. 원본/별칭 대기4, 후보 drain→삭제/ack4, 조기반환 대조2. 회귀121종 통과.|선언 gate 아래 done=false/Err 분기. true 진행 알림·transport close·모든 post-ack 사용·독립 수용 미증명.|
|followups/wiv1-generation-boundary-v1|새 머신 adapter 사전 점검(단계90 입력 4역할 재현)과 owner ack 이후 status channel 사용·세대 경계. 일반/race 16슬롯, 실제32노드. 대조군 아닌 슬롯의 ack 후 같은 세대 사용0, stale-ack 대조군 검출2, 같은 channel 세대2 전달8, 새 RPC 전달16.|transport close·같은 channel timer broadcast 경로 미시험. 독립 수용 아님.|
|followups/wiv1-transport-timer-generation-v1|단계91 adapter 재사용. 연결 종료(transport close)로 끝난 옛 세대와, 같은 channel 세대2를 둔 채 맞는 자연 done broadcast. 일반/race 16슬롯, 실제32노드. ack 후 같은 세대 사용0, 연결 종료 callback은 rpcSub.Err 8/8(notifier.Closed 0), 같은 channel 세대2 전달16(자연 broadcast 8), stale-ack 항목은 install 직전 membership으로만 드러남2.|진행 중 true 알림·Notify 도중 종료·gate 없는 transport close 미시험. channel 사용 집계는 같은 키에 합쳐진 stale 항목을 검출하지 못함. 독립 수용 아님.|
|followups/wiv1-notify-transport-generation-v1|단계91·92 adapter와 계측 재사용. Notify 직전에 잡힌 callback의 연결 종료와, 타이머 전 gate 없는 연결 종료. 일반/race 16슬롯, 실제32노드. ack 후 같은 세대 사용0, Notify 도중 종료 callback은 rpcSub.Err4·notifier.Closed4, gate 없는 종료는 rpcSub.Err8, stale-ack 항목은 membership으로만 드러남2.|진행 중 true 알림 미시험. 독립 수용 아님.|
|schedule-gap-v1/adopted-source/chainbench|채택 source·Git 객체·hash 보존.|소스 존재만으로 새 머신 설치·운영 수용을 주장하지 않음.|

원자료 checksum/의미 검증은 Python 검사로 다시 할 수 있다. native 재실행은 별도 작업이다. 최근 패킷 drivers의 gzip 소스와 초기 실패/명시 corrective1을 먼저 읽고 소유 작업 디렉터리에 추출한다. 고정 절대 경로·Go SDK·원본 checkout·출력 경로를 새 머신 경로로 바꾸는 별도 실행 adapter를 작성하고 그 diff를 기록한다. 기준 구현은 wiv1-generation-boundary-v1의 `drivers/ks_adapter.py.gz`와 `adapter/`이며, 로컬 경로는 커밋하지 않는 설정 파일로만 넘긴다. 원본 packet/manifest/summary를 재봉인하거나 덮어쓰지 않는다. 기존 candidate overlay는 followups/ticket-dev-gold-v1/private/overlays, 실제 owner 범위 승인은 followups/owner-schedule-contract-v1/ADOPTION-MANIFEST.json이 기준이다. timer 단축·가짜 mux event·가짜 ack로 같은 검증이라고 주장하지 않는다. 새 실행은 새 패킷에 출처·입력 해시·schedule·receipt·정리 결과를 남긴다.

## 5. 다음 작업과 완료 기준

단계91에서 owner ack 이후 옛 구독 세대의 status channel 사용과 명시적 새 등록 세대 경계를 관측했다([결과](./operational-preparation/source-review-supplement/followups/wiv1-generation-boundary-v1/SUMMARY.json)). 단계92에서 transport close와 같은 channel timer broadcast 경로를 관측했다([결과](./operational-preparation/source-review-supplement/followups/wiv1-transport-timer-generation-v1/SUMMARY.json)). 단계93에서 Notify 도중 종료와 gate 없는 transport close를 관측했다([결과](./operational-preparation/source-review-supplement/followups/wiv1-notify-transport-generation-v1/SUMMARY.json)). 다음 WI-V1 작업은 진행 중 true 알림(Syncing=true)의 세대 경계와 독립 ticket/oracle 준비다. true 알림은 60초 시점에 동기화가 끝나지 않아야 하므로 긴 체인과 동기화 도중 peer를 잃는 fixture를 먼저 만든다. 승인된 실제 역할 양성 경로와 최종 HEAD도 남았다. 같은 channel 재등록에서는 stale 항목이 한 키로 합쳐지므로 membership 관측을 함께 둔다. ack 시점과 RPC wire true를 혼동하지 않는다. 실제 owner4개 일정과 별도 통제 late-drain1개를 합쳐 독립 표본으로 세지 않는다. 요구사항·실제 소스·검증 기준의 누락을 먼저 대조하고 이후 구현한다.

각 세부 작업마다 목적/범위 명세 → 구현/실제 경로·대조군 검증 → receipt/hash/시도 실패 보존 → 문서·원장 업데이트 → 서명 가능 환경에서 커밋한다. 완료되지 않은 상위 작업에 완료 체크를 하지 않는다. 새 소스 변경으로 기존 source binding이 달라지면 새 검증을 남기며 기존 캡처의 source hash를 바꾸지 않는다. 매 단계 화면에 검증 결과·현재 단계·다음 작업·남은 전체 목록을 보여준다.

남은 전체는 [현재 인수인계의 13묶음](./CURRENT-HANDOFF.md)과 원장의 항목을 그대로 이어간다. 정식 수용은 WI-1/2/3/4/V1/V2/6/SP/C **0/9**, 상위 **0/13**이다. 개별 연결 시험 성공이 독립 품질 승인·운영 적용·전체 완료를 뜻하지 않는다.

D04는 미정이고 익명 역할을 허용한다. 실명·연락처를 요구하지 않는다. 새 DEV/FINAL 표본·독립 완전 정답/매칭/충분성·F01~08 승인/동결 전 `execution_allowed=false`, FINAL0, freeze=null을 유지한다. 온톨로지 평가는 동일 corpus/모델에서 off/on 검색·개념/구현 연결과 독립 정답이 필요하다. 실제 LLM 평가는 모델·인증·총 비용 상한 결정 뒤 진행한다. CoreML은 사용자 제외, 보안 전용 평가는 사용자 보류로 유지한다.

## 새 세션에 전달할 프롬프트

> docs/spec-driven/MACHINE-HANDOFF.md와 CURRENT-HANDOFF.md, REMAINING-13-EXECUTION.json을 읽고 HANDOFF-HISTORY.json의 이력을 복원·검증해 주세요. 기존 증거와 독립 수용을 구분하고 현재 코드/실제 경로를 대조한 뒤 WI-V1의 남은 진행 중 true 알림 세대 경로와 독립 oracle 준비부터 진행하세요. 기존 원본/DB/전역 설정은 보존하고 시험 자원은 소유 디렉터리에 만드세요. 매 단계 명세·검증·실패 시도·문서·커밋을 남기고 현재 단계/다음 작업/남은 전체13묶음을 보여주세요. D04 미정, 익명 역할 허용, 정식0/9, freeze=null, 실제 LLM 비용 호출 보류, 사용자 제외/보류 범위를 유지하세요. 삭제된 이전 머신 임시 경로가 존재한다고 가정하지 마세요.
