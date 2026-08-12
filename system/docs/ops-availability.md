# 운영: 상시 가동 (슬립 방지) + 자가치유 + 원격 복구

MCP 서버는 클라이언트가 붙어 있는 동안 계속 응답해야 하는데, macOS 호스트는
두 가지 이유로 그것을 깬다: 호스트가 잠들고(그리고 deep sleep 으로 내려가고),
프로세스가 죽거나 살아 있으면서도 서빙 불가 상태가 된다. 이 문서는 그 둘을
막는 설치 절차와, 원격 PC 에서 복구를 요청하는 경로를 다룬다.

도구:

| 파일 | 역할 |
|---|---|
| `system/scripts/install-launchd.sh` | LaunchAgent 두 개(서버 + 워치독) 설치·갱신·제거 |
| `system/scripts/cks-watchdog.sh` | 타이머가 부르는 `/healthz` 프로브 1회, 필요시 재기동 |
| `system/scripts/cks-recover.sh` | 원격 운영자가 SSH 로 실행하는 복구 엔트리포인트 |
| `system/scripts/cks-ops-common.sh` | 위 셋이 공유하는 헬퍼(도메인 탐지·env·프로브) |

## 관측된 실패: "ckv 관련 정보만 못 받는다"

이 도구들이 왜 이 모양인지는 실제로 관측된 실패 하나가 설명한다. 증상은
"semantic 계열 응답이 비어 온다 / ckv 관련 정보를 못 받는다"였고, 조사 시점의
서버는 멀쩡했다. 호스트 쪽 기록이 원인을 지목했다.

- MCP 서버 프로세스: 무중단 1일 21시간. 죽은 적 없음.
- 임베딩 데몬: 같은 날 슬립에서 깬 **직후** 다시 뜬 것으로 기록됨. 즉 슬립
  구간 동안 없었다.
- 호스트: `Maintenance Sleep` → `DarkWake` 를 반복했고, 1시간 42분짜리 Deep
  Idle 구간이 있었다(`pmset -g log`).
- 임베딩 데몬용 LaunchAgent 는 없다. GUI 앱이라 슬립 구간을 넘길 보장이 없다.

이 조합이 만드는 상태가 정확히 증상과 일치한다. cks 는 살아 있으므로 서버가
죽은 것처럼 보이지 않고, graph/keyword 도구는 계속 답하며, model reachability
를 요구하는 `serviceable()`(`internal/system/mcp/health.go`) 때문에 semantic
경로만 죽는다 — `ops.health` 의 용어로 `degraded`.

교훈 두 가지가 위 설계에 반영돼 있다. 슬립을 막는 것이 1차 방어선이고,
복구는 **cks 재기동이 아니라 의존성 복구**에서 시작해야 한다.

## 전제

HTTP 로 서빙하는 인스턴스여야 한다. stdio 인스턴스는 `/healthz` 가 없어
감시할 대상이 없고, 설치 스크립트가 그 자리에서 거부한다.

```sh
bin/cks mcp gen-config --dataset-dir "$DATASET" --name <name> --port 8930 --out cks.yaml
```

원격 PC 에서 서버를 직접 쓰려면 `--lan` 을 함께 준다(생성된 config 가
`allow_remote` 를 담는다). 복구 경로 자체는 SSH 를 쓰므로 `--lan` 없이도 된다.

## 설치

```sh
system/scripts/install-launchd.sh install --config /abs/path/cks.yaml
```

옵션: `--bin`(기본 `bin/cks`), `--label`(기본 `com.knowledge-system.cks-mcp`),
`--interval`(워치독 주기, 기본 60초), `--health-url`, `--ollama`.

설치되는 것은 LaunchAgent 두 개다.

- **`<label>`** — 서버. `caffeinate -s -i -m` 로 감싸 실행하므로 서버가 살아
  있는 동안 호스트는 시스템·유휴·디스크 슬립 assertion 을 잡고 있는다.
  `KeepAlive` 가 켜져 있어 종료(크래시든 정상 종료든) 시 다시 뜬다.
- **`<label>.watchdog`** — `StartInterval` 타이머. `/healthz` 를 찍어
  KeepAlive 가 볼 수 없는 실패, 즉 **떠 있으면서 서빙 불가인 상태**를 잡는다.

재실행은 갱신이다. 바이너리를 다시 빌드했거나 config 를 바꿨으면 같은 명령을
다시 돌리면 두 agent 가 재렌더링·재로드된다.

설치 직후 스크립트는 `/healthz` 가 200 을 줄 때까지 최대 60초 기다리고,
못 받으면 경고와 함께 비정상 종료한다 — "설치됐다"와 "서빙한다"를 구분한다.

### 슬립 방지의 한계 (반드시 읽을 것)

`caffeinate -s` 는 **AC 전원일 때만** 시스템 슬립을 막는다. 배터리로 돌거나
뚜껑을 닫으면(clamshell) 커널은 그대로 잠들고, 잠든 뒤에는 standby(=deep
sleep)로 내려간다. 즉 이 설치만으로 보장되는 범위는 **전원 연결 + 뚜껑 열림**
이다. 현재 상태는 이렇게 확인한다.

```sh
pmset -g assertions      # PreventUserIdleSystemSleep 이 caffeinate 로 잡혀 있는지
pmset -g custom          # standby / autopoweroff / hibernatemode 현재 값
```

전원이 빠지거나 뚜껑을 닫는 운용이 실제로 필요하면 머신 전역 전원 정책을
바꿔야 하고, 그것은 관리자 권한과 별도 승인이 필요한 변경이다(`sudo pmset -a
standby 0 autopoweroff 0`, 노트북 clamshell 은 추가 조건이 붙는다). 이 문서의
설치 절차는 거기까지 손대지 않는다.

### 로그인 세션 의존성

LaunchAgent 는 사용자 세션에 속한다. 재부팅 후 아무도 로그인하지 않으면 서버도
뜨지 않는다. 무인 재부팅까지 견뎌야 한다면 자동 로그인을 켜거나 LaunchDaemon
(`/Library/LaunchDaemons`, root) 으로 올려야 하며, 후자는 관리자 권한이 필요한
별도 결정이다.

## 워치독이 판단하는 방식

두 실패를 다르게 취급한다. 재기동이 답인 경우와 아닌 경우가 다르기 때문이다.

| 프로브 결과 | 의미 | 기본 임계 | 처리 |
|---|---|---|---|
| `200` | 서빙 중 | — | 카운터 리셋 |
| `000` | 아무도 응답 안 함(프로세스·포트 죽음) | 연속 2회 | 재기동 |
| `503` | 서버가 "서빙 불가"라고 응답 | 연속 5회 | 이유를 로그로 남기고 재기동 |

`503` 의 임계가 높은 이유: 백엔드(예: 임베딩 데몬)가 내려갔거나 데이터셋이
반쯤 빌드된 상태라면 재기동해도 낫지 않는다. 그런 상황에서 운영자에게 실제로
필요한 것은 재기동이 아니라 `/healthz` 가 돌려준 reason 이고, 그것은 매
프로브마다 로그에 남는다.

### 의존성 먼저 — 임베딩 데몬

프로브가 200 이 아니면 워치독은 서버를 건드리기 전에 **임베딩 데몬부터**
확인한다. 이 호스트에서 실제로 관측된 실패가 그것이기 때문이다(아래 "관측된
실패" 참조): 데몬은 시스템 슬립을 넘기지 못하는데 cks 는 멀쩡히 살아 있으므로,
`serviceable()` 이 model reachability 를 요구하는 탓에 `/healthz` 는 503 을
내고 graph 도구만 계속 동작한다. 이 상태에서 cks 를 재기동해봐야 아무것도
낫지 않는다.

그래서 순서가 이렇다: 데몬이 죽어 있으면 띄우고 → 응답할 때까지 기다리고 →
`/healthz` 를 다시 본다. 여기서 회복되면 **cks 는 건드리지 않는다**(데몬이
돌아오면 cks 는 스스로 회복한다). 회복되지 않을 때만 위 표의 재기동 경로로
내려간다.

재기동에는 쿨다운(기본 300초)이 있다. 영구 고장 난 인스턴스가 재기동 루프를
도는 대신 느린 로그를 남기게 하기 위함이다. 재기동 후에는 다시 `/healthz` 를
기다려 **복구됐는지까지** 확인하고 그 결과를 로그에 쓴다.

환경변수로 조정한다: `CKS_WATCHDOG_UNREACHABLE_THRESHOLD`,
`CKS_WATCHDOG_UNSERVICEABLE_THRESHOLD`, `CKS_WATCHDOG_COOLDOWN`,
`CKS_WATCHDOG_RECOVER_DEADLINE`.

## 원격 복구 (SSH)

원격 PC 에서:

```sh
ssh <host> '<repo>/system/scripts/cks-recover.sh status'
ssh <host> '<repo>/system/scripts/cks-recover.sh logs --lines 80'
ssh <host> '<repo>/system/scripts/cks-recover.sh restart'
```

| 액션 | 하는 일 |
|---|---|
| `status` | 두 agent 의 launchd 상태(state/pid/last exit) + 임베딩 데몬 + `/healthz` + 서빙 중인 인스턴스 identity. 읽기 전용 |
| `health` | `/healthz` 코드와 본문 그대로 |
| `logs` | 서버 로그와 워치독 로그 tail |
| `recover` | 워치독과 같은 사다리 — 임베딩 데몬을 먼저 살리고, 그게 원인이 아니었을 때만 서버를 bounce |
| `restart` | 조건 없이 서버 agent 를 bounce 하고 다시 서빙할 때까지 대기 |

원인을 모르는 채 "MCP 가 이상하다"는 보고를 받았을 때 부를 것은 `recover` 다.
`restart` 는 원인을 이미 아는 경우(예: 데이터셋 스왑 후 재기동)에 쓴다.

**왜 HTTP 관리 엔드포인트가 아니라 SSH 인가.** MCP 리스너의 ACL 은 코드 주석이
명시하듯 network-scope 필터이지 per-client 인증이 아니다(`internal/system/mcp/server.go`
의 `HTTPPolicy`). 거기에 프로세스 제어를 붙이면 LAN 에 있는 누구나 서버를
재기동할 수 있게 된다. SSH 는 호출자를 인증하고 새 리스너도 필요 없다. 또한
서버 프로세스가 죽으면 인-프로세스 관리 엔드포인트도 같이 죽으므로, 정작
필요한 순간에 쓸 수 없다.

`launchctl` 대상 도메인은 스크립트가 탐지한다: 콘솔 로그인 중이면 `gui/<uid>`,
아니면 `user/<uid>`. SSH 세션에서 도메인을 잘못 잡아 "Could not find service"
가 나오는 흔한 실패를 이 탐지가 막는다.

원격에서 SSH 가 닿으려면 호스트가 깨어 있어야 한다는 점은 그대로다 — 슬립
방지가 1차 방어선이고, 복구 경로는 그 위에 얹힌 2차 방어선이다.

## 상태 파일

`~/.knowledge-system/`(`CKS_OPS_HOME` 으로 변경 가능)에 인스턴스별로 남는다.

```
<label>.env               설치 시 렌더링된 계약(config·바이너리·health URL·로그 경로)
<label>.watchdog.state    연속 실패 횟수와 마지막 재기동 시각
logs/<label>.out.log      서버 stdout
logs/<label>.err.log      서버 stderr
logs/<label>.watchdog.log 워치독 판단 기록
```

## 제거

```sh
system/scripts/install-launchd.sh uninstall
```

두 agent 를 bootout 하고 plist 와 상태 파일을 지운다. 로그는 남긴다.

## 블루-그린 재인덱싱과의 관계

데이터셋 교체 절차는 [ops-blue-green-reindex.md](./ops-blue-green-reindex.md)
가 소유한다. 주의할 접점 하나: 인스턴스는 기동 시점에 데이터셋 심볼릭 링크를
한 번 해석해 고정하므로, `current` 를 스왑해도 **이미 떠 있는 서버는 옛 버전을
계속 서빙한다**. 스왑 후에는 재기동이 필요하고, 여기서는 그것이
`cks-recover.sh restart` 다.
