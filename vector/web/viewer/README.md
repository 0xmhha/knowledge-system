# ckv Atlas — data pipeline + search backend

ckv 인덱스(vector.db)의 지식 분포를 3D 로 보여주고, 검색을 **실제 ckv 엔진**
(`ckv query --json` = MCP `semantic_search` 와 동일한 `engine.Search` 경로:
bge-m3 임베딩 → sqlite-vec KNN → threshold → citation 검증)으로 수행해
히트를 공간에서 붉게 하이라이트한다.

**프런트엔드(뷰어 UI)는 통합 Next 앱 `tools/viewer` 의 `/atlas`
라우트로 이동**했다(구 standalone `index.html` 은 그 React 컴포넌트로 대체·삭제).
이 디렉토리는 이제 그 앱의 **Atlas 백엔드** — 투영 데이터 생성(`export_projection.py`)
+ 검색/설정/데이터 서빙(`serve.py`) — 만 담당한다.

Embedding Projector 의 UX(3D 축·회전·타이핑 즉시 하이라이트)를 따르되,
매칭이 *메타데이터 문자열*이 아니라 **의미(bge-m3)** 라는 점이 다르다.
질의 벡터도 같은 PCA 행렬로 투영해 ◇ 로 표시한다 — "질의가 공간 어디에
떨어져서 무엇이 선택되는가"를 눈으로 확인.

## 실행

```bash
# 0) 전제: ollama serve + ollama pull bge-m3, ckv 바이너리(or go run)
# 설정은 전부 env/플래그 — 코드에 머신 종속 경로 없음:
#   CKV_DB   : ckv 데이터 디렉토리 (기본 ./ckv-data — ckv CLI --out 기본값과 동일)
#   CKV_BIN  : ckv 실행 커맨드    (기본 PATH 의 ckv)
#   CKV_OLLAMA_ENDPOINT : ollama 주소 (기본 http://localhost:11434)

export CKV_DB=/path/to/ckv-data-dir
export CKV_BIN=/path/to/repo/bin/ckv        # 또는 "go run ./cmd/ckv"

# 1) 투영 데이터 생성 (인덱스 바뀔 때마다 1회; numpy 필요)
python3 export_projection.py

# 2) 백엔드 기동 (데이터 + 검색)
python3 serve.py                            # → :8098

# 3) 프런트엔드: 통합 Next 앱의 /atlas 라우트
cd ../../../tools/viewer && npm run dev   # → http://localhost:3001/atlas
#   (dev 프록시가 /query·/config·/data/* 를 위 :8098 백엔드로 전달)
```

## 구성
- `export_projection.py` — vector.db shadow 테이블 → PCA top-3 → `data/points.json`
  (좌표+메타) + `data/projection.json` (질의 투영용 mean/components/scale)
- `serve.py` — 데이터 서빙(`/data/*`, `/config`) + `/query`(진짜 ckv 실행 + 질의 임베딩 투영)
- 프런트엔드: `tools/viewer/src/components/Atlas.tsx` (`/atlas` 라우트)

## 조작 (`/atlas` 페이지)
드래그=회전 · 휠=줌 · 타이핑=350ms 디바운스 실검색 · 패널 hover=해당 점 강조
