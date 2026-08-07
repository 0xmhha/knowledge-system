# Knowledge System Dashboard (tools/viewer)

Next.js 15 + TypeScript + `react-force-graph-{2d,3d}` 기반 코드 지식 그래프 viewer.
`output: 'export'`로 만들어진 정적 자산이 `ckg` Go 바이너리에 `go:embed`로 박혀,
`cks viewer`가 `:8080`에서 대시보드를 serve하고 `/api/*`는 스폰된 `ckg api`로 proxy.

> **첫 방문 시 이 문서부터 읽어라.** dev/prod 경로가 헷갈리기 쉽고,
> `next.config.mjs`의 `trailingSlash`가 dev에서 backend와 충돌하므로
> 무작정 `npm run dev`로 띄우면 막힌다.

---

## 1. 정상 경로 (Prod) — 가장 흔한 사용

`make -C graph viewer`는 이 앱을 정적 export해
`internal/system/viewer/web_assets/`로 복사하고, 이어지는 `make build-bins`가
그 결과를 **cks 바이너리**에 embed한다 (대시보드는 합성 엔진 `cks viewer`가
서빙; ckg는 API 전용). 자산 빌드 없이 `make build-bins`만 하면 embed된
대시보드는 stub이다.

```bash
# 0. 한 번만: 대시보드 자산 + 엔진 바이너리 빌드
make -C graph viewer && make build-bins
# → ./bin/ckg ./bin/ckv ./bin/cks

# 1. 분석 대상 프로젝트의 그래프 생성
./bin/ckg build --src=/path/to/project --out=/path/to/graph-out
# → /path/to/graph-out/graph.db + manifest.json

# 2. 대시보드 열기 (ckg api 백엔드를 자동 스폰)
./bin/cks viewer --graph=/path/to/graph-out --open
# → http://localhost:8080 자동 오픈
```

한 줄로 통합:

```bash
./bin/ckg quickstart --src=/path/to/project --out=/tmp/graph --port=8080
```

> **viewer를 수정했다면 `make -C graph viewer && make build-bins` 다시 실행.**
> embed된 대시보드는 컴파일 시점에 고정된다. 개발 루프에서는
> `CKS_DEV_VIEWER_DIR`로 디스크 오버레이를 쓰면 재빌드 없이 리로드된다.

---

## 2. Dev — viewer만 빠르게 반복 (권장)

`CKS_DEV_VIEWER_DIR` 환경변수가 set되면, cks는 embed된 대시보드 대신
그 디렉토리의 정적 파일을 serve. 즉 **`ckg` 바이너리 재컴파일 없이도**
viewer 변경분을 확인할 수 있다.

```bash
# 한 번만
cd tools/viewer && npm install

# 매 수정 사이클
cd tools/viewer && npm run build       # 약 3~5s
CKS_DEV_VIEWER_DIR=$(pwd)/tools/viewer/out \
  ./bin/cks viewer --graph=/path/to/graph-out --open
```

`trailingSlash` 충돌 없음 (Next dev 서버를 거치지 않으므로). 풀 hot reload는
아니지만 `npm run build`가 충분히 빨라 실용적.

---

## 3. Dev — Next hot reload (실험적, 비권장)

Next dev 서버의 hot reload를 쓰고 싶을 때. `npm run dev`로 :3001을 띄우고,
`/api/*`는 `:8080`의 `ckg api`로 proxy.

```bash
# Term 1
./bin/ckg api --graph=/path/to/graph-out --port=8080

# Term 2
cd tools/viewer && npm run dev   # :3001
```

**알려진 함정**:

- `next.config.mjs`의 `trailingSlash: true`는 정적 export 요구 사항(디렉토리
  구조 `/foo/index.html`을 만들기 위함)이지만, dev 서버에서 `/api/*` rewrite와
  충돌해 `:8080`의 Go backend가 404를 반환(`/api/manifest/` → 404,
  `/api/manifest` → 200). NODE_ENV 분기로 dev에서 끄는 시도도 redirect loop가
  남아 미해결.
- 위 문제 풀기 전엔 §2의 `CKS_DEV_VIEWER_DIR` 경로를 권장.

---

## 4. 데이터 모드 자동 감지

`src/lib/api.ts`의 `detectMode()`가 `./manifest.json` fetch로 판별:

| `./manifest.json` 응답 | 모드 | 구현 |
|---|---|---|
| 200 OK | `static` | `StaticAPI` — `chunks/*.json`을 fetch |
| 그 외 | `serve` | `API` — `/api/*` REST 호출 |

- **`cks viewer`** → manifest는 `/api/manifest`(proxy)로만 제공 → `./manifest.json` 404 → **serve mode**
- **`ckg export-static`** → 정적 dir 루트에 `manifest.json` 둠 → **static mode**

즉 viewer 코드는 두 모드를 모두 지원하지만, 각 모드는 데이터를 다르게 fetch한다.

---

## 5. 디렉토리 구조

| 경로 | 역할 |
|---|---|
| `app/{page,layout}.tsx`, `app/globals.css` | Next App Router 엔트리, 전역 CSS |
| `src/components/App.tsx` | 부팅·네비게이션·키보드 단축키·패널 레이아웃 |
| `src/components/GraphCanvas.tsx` | 2D/3D 그래프 본체. react-force-graph 래핑 |
| `src/components/{NodeDetail,NodeList,TopBar,…}.tsx` | UI 패널들 |
| `src/store/store.ts` | zustand 전역 상태 (visibleIds, focusDistance, anchor, history…) |
| `src/lib/api.ts` | `IAPI`, `API`, `StaticAPI`, `detectMode` |
| `src/lib/{edges,encoding,depth,trace}.ts` | 색·엣지 그룹·BFS·trace 유틸 |
| `out/` (`.gitignore`) | `npm run build` 결과. `make viewer`가 `internal/server/web_assets/`로 복사 |

---

## 6. 빌드 / 검증 스크립트

```bash
cd tools/viewer
npm run typecheck     # tsc --noEmit
npm run lint          # eslint .
npm run build         # 정적 export → out/
npm run test:smoke    # Playwright 스모크
```

production 통합 검증:

```bash
make -C graph viewer && make build-bins
./bin/cks viewer --graph=/some/graph-out --open
```

---

## 7. 흔한 함정 체크리스트

| 증상 | 원인 | 해결 |
|---|---|---|
| viewer를 고쳤는데 화면이 그대로 | embed된 viewer는 컴파일 시점에 고정 | `make -C graph viewer && make build-bins` 다시 / 또는 §2 `CKS_DEV_VIEWER_DIR` |
| `npm run dev`에서 `/api/* → 500` | backend(ckg api) 안 떠 있음 또는 trailingSlash 충돌 | §2 경로로 전환 권장 |
| `cks viewer` 후 빈 캔버스 | `--graph` 디렉토리에 `graph.db` 없음 | `ckg build`부터 |
| "1 Issue" 좌하단 빨간 배지 | `/api/*` 호출 실패 (manifest/edges/nodes 중 하나) | 콘솔 / 서버 로그 확인 |
| `ERR_TOO_MANY_REDIRECTS` on `/api/*` | Next dev + trailingSlash 처리 충돌 | §2로 전환 |
| `THREE.WARNING: Multiple instances` | `react-force-graph-3d`가 자체 three.js 번들. 무시 가능 | — |

---

## 8. 시각 효과 (2026-05-21 시점)

이 viewer가 지금 가진 4가지 시각 효과의 정의 위치:

| 효과 | 위치 | 키 변수 |
|---|---|---|
| 배경 보라/네이비 radial gradient | `app/globals.css .canvas-host` | `radial-gradient(...)` + `backgroundColor="rgba(0,0,0,0)"` |
| Dolly-in (거리 ×0.6, 400ms) | `GraphCanvas.tsx centerOnNode` 3D 분기 | `DOLLY_FACTOR`, `CENTER_DURATION_MS` |
| Focus-locked orbit (선택 노드 중심 회전) | `GraphCanvas.tsx centerOnNode` 3D + `zoomReset` | `controls().target.set(...)` |
| Edge gradient (src 불투명 → dst 투명) | `GraphCanvas.tsx linkCanvasObject` 2D | `linkCanvasObjectMode='after'`, alpha 0.85→0 |

대기 중: BFS-ripple (선택 시 1회 0.5초 펄스) — react-force-graph cooldown 이후
redraw 강제 필요라 별도 라운드.

---

## 9. 다음에 헷갈리지 않으려면

- "viewer 안 켜져요" → §1 또는 §2.
- "viewer 수정했는데 안 보여요" → §1 또는 §2의 `CKS_DEV_VIEWER_DIR`. 절대 §3 먼저 시도하지 말 것.
- "데이터가 안 보여요" → `ckg build`로 `graph.db`가 진짜 생겼는지 + `--graph` 경로가 맞는지부터.
- "정적 export로 배포" → `ckg export-static --graph=X --out=Y` 후 Y를 정적 호스팅.
