'use client';

// Atlas — vector/web/viewer/index.html (734줄 vanilla-JS "ckv Atlas") 의
// React 포팅. 핵심 규율: 프레임 단위 캔버스 렌더링/인터랙션은 원본 그대로
// 명령형(imperative)으로 useEffect 안에 두고, DOM querySelector 접근만
// React ref 로, 컨트롤 위젯만 React state 로 바꾼다. 렌더 루프의 클로저가
// 최신 값을 보도록 가변 상태는 단일 stateRef 객체에 담아 공유한다.
//
// 백엔드 계약(vector/web/viewer/serve.py)과 동일한 경로로 fetch 한다:
//   GET /data/points.json  · GET /config  · GET /query?q=&k=&lang=
// (Next 정적 export 이므로 서버 코드 없음 — next.config rewrite 가
//  이 경로들을 Python 백엔드로 프록시한다. 라우팅/프록시는 별도 처리.)

import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react';
import styles from './Atlas.module.css';

// ---- 데이터 형상 (serve.py + index.html 사용처와 일치) ----
// meta 튜플: [chunk_id, sym, file, category, lang, kind, line, isTest, cluster]
type Meta = [string, string, string, string, string, string, number, number, number];

interface PointsData {
  n: number;
  xyz: number[];
  meta: Meta[];
  cluster_labels?: string[];
}

interface Axis { pos: string; neg: string; }

interface Config {
  dim?: number;
  evr?: number[];
  db: string;
  model: string;
  projection: boolean;
  axes?: Axis[] | null;
}

interface Hit {
  chunk_id: string;
  symbol?: string;
  score: { normalized: number };
  citation: { file: string; start_line: number; end_line: number };
  snippet?: string;
  category?: string;
}

interface QueryResult {
  hits?: Hit[];
  query_xyz?: [number, number, number] | null;
  sim?: (number | null)[][] | null;
  metadata?: { tokens_used?: number };
  error?: string;
  stderr?: string;
}

type ViewMode = 'space' | 'score' | 'matrix';

// hits Map 값: 화면 렌더용으로 정규화 점수·순위·원본 hit 을 함께 보관.
interface HitEntry { score: number; rank: number; h: Hit; }
// score 뷰 히트 지오메트리 (hover 판정용)
interface SGeo { x: number; y: number; r: number; idx: number; cid: string; }
// matrix 뷰 지오메트리 (hover 판정용)
interface MGeo { ox: number; oy: number; cell: number; K: number; }

// 렌더 루프가 읽는 전체 가변 상태 — 명령형 클로저가 항상 최신 값을 보도록
// 단일 ref 에 모은다.
interface RenderState {
  N: number;
  XYZ: Float32Array | null;
  META: Meta[];
  byChunk: Map<string, number>;
  CLUSTER_LABELS: string[];
  AXES: Axis[] | null;
  PBUF: Float32Array | null;   // 전 점 투영 버퍼 (draw ↔ pick 공유)
  ORD: number[] | null;        // 깊이 정렬 순서
  VMODE: ViewMode;
  QRES: QueryResult | null;
  SGEO: SGeo[];
  MGEO: MGeo | null;
  yaw: number; pitch: number; dist: number;
  colorBy: number; psize: number; labelMode: string; dimBg: boolean;
  hits: Map<string, HitEntry>;
  queryXYZ: [number, number, number] | null;
  curQuery: string;
  hoverIdx: number; selIdx: number; selChunk: string | null;
  rotTimer: number | null;
  seq: number;
  DPR: number;
  k: string; lang: string;
}

// ---- 카메라 초기값 ----
const CAM0 = { yaw: 0.6, pitch: 0.35, dist: 2.6 };

// ---- 색: 라이트 배경용 팔레트 (진하고 선명한 톤) ----
const FIXED: Record<string, string> = {
  // category
  consensus: '#7b2ff2', state: '#1a73e8', crypto: '#e37400', p2p: '#188038',
  txpool: '#d01884', vm: '#9334e6', rpc: '#e8710a',
  // language
  go: '#00758d', solidity: '#b06000', sol: '#b06000', markdown: '#5f6368',
  // test 여부
  '0': '#1a73e8', '1': '#d93025',
  '': '#80868b',
};

const HINTS: Record<ViewMode, string> = {
  space: '드래그=회전 · 휠=줌 · 클릭=선택',
  score: '가로축 = 질의와의 유사도 · 컷라인 왼쪽은 ckv가 제외한 영역',
  matrix: '셀(i,j) = i위·j위 히트 간 코사인 유사도 — 진하면 중복, 옅으면 다양',
};

// dangerouslySetInnerHTML / 인라인 style 문자열에서 쓰는 dot 스타일
// (CSS 모듈 해시 클래스는 innerHTML 에 매핑되지 않으므로 인라인).
const DOT_STYLE =
  'display:inline-block;width:9px;height:9px;border-radius:50%;margin-right:6px;vertical-align:middle;';

function colorOf(v: string | number): string {
  const key = String(v ?? '');
  if (Object.prototype.hasOwnProperty.call(FIXED, key)) return FIXED[key];
  let h = 0;
  for (const ch of key) h = (h * 31 + ch.charCodeAt(0)) >>> 0;
  return `hsl(${h % 360} 62% 40%)`;
}

// cluster 색: 황금각 HSL — 인접 id 가 시각적으로 확실히 다른 색이 되도록.
function clusterColor(id: number | null): string {
  if (id == null || id < 0) return '#80868b';
  return `hsl(${(id * 137.508) % 360} 62% 42%)`;
}

// 유사도 히트맵 색 (흰색 → 붉은색)
function simColor(v: number): string {
  const t = Math.max(0, Math.min(1, (v - 0.3) / 0.7));
  const r = Math.round(255 + (217 - 255) * t);
  const g = Math.round(255 + (48 - 255) * t);
  const b = Math.round(255 + (37 - 255) * t);
  return `rgb(${r},${g},${b})`;
}

function shortDir(s: string | undefined): string { return (s || '').split(',')[0].trim(); }

const LEGEND_NAMES: Record<number, string> = { 3: 'category', 4: 'language', 5: 'kind', 7: 'test', 8: 'cluster' };

export default function Atlas() {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const centerRef = useRef<HTMLDivElement | null>(null);
  const tipRef = useRef<HTMLDivElement | null>(null);
  const drawRef = useRef<(() => void) | null>(null);
  const timerRef = useRef<number | null>(null);

  const stateRef = useRef<RenderState>({
    N: 0, XYZ: null, META: [], byChunk: new Map(), CLUSTER_LABELS: [], AXES: null,
    PBUF: null, ORD: null, VMODE: 'space', QRES: null, SGEO: [], MGEO: null,
    yaw: CAM0.yaw, pitch: CAM0.pitch, dist: CAM0.dist,
    colorBy: 8, psize: 2.5, labelMode: 'hits', dimBg: true,
    hits: new Map(), queryXYZ: null, curQuery: '',
    hoverIdx: -1, selIdx: -1, selChunk: null,
    rotTimer: null, seq: 0, DPR: 1, k: '20', lang: '',
  });

  // ---- 컨트롤 위젯 state (JSX 재렌더용; draw 는 stateRef 를 읽는다) ----
  const [vmode, setVmode] = useState<ViewMode>('space');
  const [colorBy, setColorBy] = useState(8);
  const [psize, setPsize] = useState(2.5);
  const [labelMode, setLabelMode] = useState('hits');
  const [dimBg, setDimBg] = useState(true);
  const [autoRot, setAutoRot] = useState(false);
  const [kValue, setKValue] = useState('20');
  const [langValue, setLangValue] = useState('');
  const [query, setQuery] = useState('');

  const [dataLoaded, setDataLoaded] = useState(false);
  const [statusNode, setStatusNode] = useState<ReactNode>('데이터 로딩중…');
  const [infoNode, setInfoNode] = useState<ReactNode>('—');
  const [detailNode, setDetailNode] = useState<ReactNode>(null);
  // undefined = 초기(검색 전) · null = 검색어 비움 · QueryResult = 결과
  const [panelData, setPanelData] = useState<QueryResult | null | undefined>(undefined);
  const [hoverHit, setHoverHit] = useState<string | null>(null);
  const [cvLegendVisible, setCvLegendVisible] = useState(false);

  // ---- 검색: 타이핑 → 디바운스 → 진짜 ckv ----
  const runSearch = useCallback(async (q: string) => {
    const st = stateRef.current;
    const my = ++st.seq;
    setStatusNode(<>검색중… &quot;<b>{q}</b>&quot; → ckv(bge-m3)</>);
    try {
      const res = await fetch(`/query?q=${encodeURIComponent(q)}&k=${st.k}&lang=${st.lang}`);
      const d: QueryResult = await res.json();
      if (my !== st.seq) return;
      if (d.error) {
        setStatusNode(
          <>
            <span className={styles.err}>에러: {d.error}</span>{' '}
            <span style={{ color: 'var(--sub)' }}>(ollama/ckv 확인)</span>
          </>,
        );
        return;
      }
      st.hits = new Map();
      (d.hits || []).forEach((h, idx) => st.hits.set(h.chunk_id, { score: h.score.normalized, rank: idx + 1, h }));
      st.queryXYZ = d.query_xyz || null;
      st.curQuery = q;
      st.selChunk = null;
      st.QRES = d;
      setCvLegendVisible(st.VMODE === 'space');
      setPanelData(d);
      drawRef.current?.();
      const inCloud = [...st.hits.keys()].filter((c) => st.byChunk.has(c)).length;
      setStatusNode(
        <>
          &quot;<b>{q}</b>&quot; → <b>{st.hits.size}</b> hits (공간 표시 {inCloud}) · tokens{' '}
          {d.metadata?.tokens_used ?? '-'} ·{' '}
          <span style={{ color: 'var(--sub)' }}>실제 ckv engine.Search 결과</span>
        </>,
      );
    } catch (e) {
      if (my === stateRef.current.seq) {
        setStatusNode(<span className={styles.err}>요청 실패: {String(e)}</span>);
      }
    }
  }, []);

  const onQueryInput = useCallback((val: string) => {
    setQuery(val);
    const st = stateRef.current;
    if (timerRef.current != null) clearTimeout(timerRef.current);
    const q = val.trim();
    if (!q) {
      st.hits = new Map();
      st.queryXYZ = null;
      st.curQuery = '';
      st.selChunk = null;
      st.QRES = null;
      setCvLegendVisible(false);
      setPanelData(null);
      drawRef.current?.();
      setStatusNode(<>{st.N} chunks — 대기중</>);
      return;
    }
    timerRef.current = window.setTimeout(() => runSearch(q), 350);
  }, [runSearch]);

  // ---- 명령형 캔버스 렌더링/인터랙션 (원본 index.html 을 그대로 포팅) ----
  useEffect(() => {
    const canvas = canvasRef.current;
    const center = centerRef.current;
    if (!canvas || !center) return;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    const st = stateRef.current;
    st.DPR = window.devicePixelRatio || 1;

    const getWH = (): [number, number] => {
      const r = center.getBoundingClientRect();
      return [r.width, r.height];
    };

    // ---- 3D → 2D ----
    function rot(x: number, y: number, z: number): [number, number, number] {
      const { yaw, pitch } = st;
      const cy = Math.cos(yaw), sy = Math.sin(yaw), cp = Math.cos(pitch), sp = Math.sin(pitch);
      const x1 = cy * x + sy * z, z1 = -sy * x + cy * z;
      const y2 = cp * y - sp * z1, z2 = sp * y + cp * z1;
      return [x1, y2, z2];
    }
    function projPt(x: number, y: number, z: number, W: number, H: number): [number, number, number] | null {
      const [rx, ry, rz] = rot(x, y, z);
      const zc = rz + st.dist;
      if (zc <= 0.05) return null;
      const f = (Math.min(W, H) * 0.85) / zc;
      return [W / 2 + rx * f, H / 2 - ry * f, zc];
    }
    function line3(ax: number, ay: number, az: number, bx: number, by: number, bz: number,
      W: number, H: number, color: string, width: number) {
      const p1 = projPt(ax, ay, az, W, H), p2 = projPt(bx, by, bz, W, H);
      if (!p1 || !p2) return;
      ctx!.strokeStyle = color; ctx!.lineWidth = width;
      ctx!.beginPath(); ctx!.moveTo(p1[0], p1[1]); ctx!.lineTo(p2[0], p2[1]); ctx!.stroke();
    }

    function pointColor(i: number): string {
      if (st.colorBy === 8) return clusterColor(st.META[i][8]);
      const v = st.META[i][st.colorBy];
      return colorOf(st.colorBy === 7 ? String(v) : (v || ''));
    }

    // 기준면: 바닥 그리드(y=−1) + 경계 큐브 (단안 깊이 단서).
    function drawCubeGrid(W: number, H: number) {
      for (let g = -1; g <= 1.001; g += 0.5) {
        line3(g, -1, -1, g, -1, 1, W, H, '#eef1f4', 1);
        line3(-1, -1, g, 1, -1, g, W, H, '#eef1f4', 1);
      }
      const E = [[-1, -1, -1, 1, -1, -1], [1, -1, -1, 1, -1, 1], [1, -1, 1, -1, -1, 1], [-1, -1, 1, -1, -1, -1],
        [-1, 1, -1, 1, 1, -1], [1, 1, -1, 1, 1, 1], [1, 1, 1, -1, 1, 1], [-1, 1, 1, -1, 1, -1],
        [-1, -1, -1, -1, 1, -1], [1, -1, -1, 1, 1, -1], [1, -1, 1, 1, 1, 1], [-1, -1, 1, -1, 1, 1]];
      for (const e of E) line3(e[0], e[1], e[2], e[3], e[4], e[5], W, H, '#e4e8ec', 1);
    }

    function pill(x: number, y: number, txt: string, color: string) {
      ctx!.font = 'bold 10.5px sans-serif';
      const w = ctx!.measureText(txt).width;
      ctx!.fillStyle = 'rgba(255,255,255,.93)'; ctx!.strokeStyle = '#e0e0e0'; ctx!.lineWidth = 1;
      ctx!.beginPath();
      if (typeof ctx!.roundRect === 'function') ctx!.roundRect(x - w / 2 - 6, y - 9, w + 12, 15, 7);
      else ctx!.rect(x - w / 2 - 6, y - 9, w + 12, 15);
      ctx!.fill(); ctx!.stroke();
      ctx!.fillStyle = color; ctx!.textAlign = 'center';
      ctx!.fillText(txt, x, y + 2.5);
    }

    function drawAxes(W: number, H: number) {
      const AX: [[number, number, number], string, string][] = [
        [[1, 0, 0], 'PC1', '#c5221f'], [[0, 1, 0], 'PC2', '#188038'], [[0, 0, 1], 'PC3', '#1a73e8'],
      ];
      for (let a = 0; a < 3; a++) {
        const [d, name, c] = AX[a];
        const p0 = projPt(-d[0], -d[1], -d[2], W, H), p1 = projPt(d[0], d[1], d[2], W, H);
        const pm = projPt(0, 0, 0, W, H);
        if (!p0 || !p1 || !pm) continue;
        ctx!.strokeStyle = '#d5d9dd'; ctx!.lineWidth = 1;
        ctx!.beginPath(); ctx!.moveTo(p0[0], p0[1]); ctx!.lineTo(pm[0], pm[1]); ctx!.stroke();
        ctx!.strokeStyle = c; ctx!.globalAlpha = .65; ctx!.lineWidth = 1.4;
        ctx!.beginPath(); ctx!.moveTo(pm[0], pm[1]); ctx!.lineTo(p1[0], p1[1]); ctx!.stroke();
        const vx = p1[0] - pm[0], vy = p1[1] - pm[1], L = Math.hypot(vx, vy) || 1;
        const ux = vx / L, uy = vy / L;
        ctx!.beginPath();
        ctx!.moveTo(p1[0], p1[1]);
        ctx!.lineTo(p1[0] - ux * 9 - uy * 4, p1[1] - uy * 9 + ux * 4);
        ctx!.lineTo(p1[0] - ux * 9 + uy * 4, p1[1] - uy * 9 - ux * 4);
        ctx!.closePath(); ctx!.fillStyle = c; ctx!.fill();
        ctx!.globalAlpha = 1;
        const pos = st.AXES ? shortDir(st.AXES[a].pos) : '';
        const neg = st.AXES ? shortDir(st.AXES[a].neg) : '';
        pill(p1[0], p1[1] - 14, pos ? `${name}+ · ${pos}` : `${name}+`, c);
        pill(p0[0], p0[1] + 16, neg ? `${name}− · ${neg}` : `${name}−`, '#5f6368');
      }
    }

    // ---- 점수 축 뷰: beeswarm + threshold 컷라인 ----
    function drawScore(W: number, H: number) {
      st.SGEO = [];
      const arr = st.QRES && st.QRES.hits ? st.QRES.hits : [];
      const x0 = 80, x1 = W - 50, yAxis = H - 90;
      const smin = 0.3, smax = 1.0;
      const X = (s: number) => x0 + (Math.max(smin, Math.min(smax, s)) - smin) / (smax - smin) * (x1 - x0);
      ctx!.strokeStyle = '#dadce0'; ctx!.lineWidth = 1.5;
      ctx!.beginPath(); ctx!.moveTo(x0, yAxis); ctx!.lineTo(x1, yAxis); ctx!.stroke();
      ctx!.font = '11px sans-serif'; ctx!.textAlign = 'center';
      for (let s = smin; s <= smax + 1e-9; s += 0.1) {
        const x = X(s);
        ctx!.strokeStyle = '#dadce0'; ctx!.beginPath(); ctx!.moveTo(x, yAxis); ctx!.lineTo(x, yAxis + 6); ctx!.stroke();
        ctx!.fillStyle = '#5f6368'; ctx!.fillText(s.toFixed(1), x, yAxis + 20);
      }
      ctx!.fillStyle = '#202124'; ctx!.font = 'bold 12px sans-serif';
      ctx!.fillText('질의와의 코사인 유사도 (normalized score) →', (x0 + x1) / 2, yAxis + 42);
      const xt = X(0.4);
      ctx!.strokeStyle = '#d93025'; ctx!.setLineDash([5, 4]); ctx!.lineWidth = 1.5;
      ctx!.beginPath(); ctx!.moveTo(xt, 60); ctx!.lineTo(xt, yAxis); ctx!.stroke(); ctx!.setLineDash([]);
      ctx!.fillStyle = '#d93025'; ctx!.font = 'bold 11px sans-serif'; ctx!.textAlign = 'left';
      ctx!.fillText('임계값 0.4 — 미만은 ckv가 제외(응답에 없음)', xt + 8, 74);
      if (!arr.length) {
        ctx!.fillStyle = '#80868b'; ctx!.font = '13px sans-serif'; ctx!.textAlign = 'center';
        ctx!.fillText('검색하면 히트가 이 축 위에 배치됩니다', W / 2, H / 2);
        return;
      }
      const placed: { x: number; lane: number }[] = [];
      arr.forEach((h, idx) => {
        const sc = h.score.normalized, x = X(sc);
        let lane = 0;
        while (placed.some((p) => p.lane === lane && Math.abs(p.x - x) < 18)) lane++;
        placed.push({ x, lane });
        const y = yAxis - 34 - lane * 36;
        const top5 = idx < 5;
        const rr = top5 ? 11 : 8;
        ctx!.strokeStyle = 'rgba(217,48,37,.18)'; ctx!.lineWidth = 1;
        ctx!.beginPath(); ctx!.moveTo(x, y + rr); ctx!.lineTo(x, yAxis); ctx!.stroke();
        ctx!.beginPath(); ctx!.arc(x, y, rr, 0, 7);
        ctx!.fillStyle = (h.chunk_id === st.selChunk) ? '#202124' : (top5 ? '#d93025' : '#f3a29b');
        ctx!.fill(); ctx!.strokeStyle = '#fff'; ctx!.lineWidth = 1.5; ctx!.stroke();
        ctx!.fillStyle = top5 ? '#fff' : '#8c3b35'; ctx!.font = `bold ${top5 ? 11 : 9}px sans-serif`; ctx!.textAlign = 'center';
        ctx!.fillText(String(idx + 1), x, y + (top5 ? 4 : 3));
        if (top5) {
          const sym = (h.symbol || h.citation.file.split('/').pop() || '').slice(0, 16);
          ctx!.fillStyle = '#a50e0e'; ctx!.font = 'bold 11px sans-serif';
          ctx!.fillText(`${sym} · ${sc.toFixed(3)}`, x, y - rr - 6);
        }
        st.SGEO.push({ x, y, r: rr, idx, cid: h.chunk_id });
      });
    }

    // ---- 유사도 행렬 뷰 ----
    function drawMatrix(W: number, H: number) {
      st.MGEO = null;
      const arr = st.QRES && st.QRES.hits ? st.QRES.hits : [];
      const S = st.QRES ? st.QRES.sim : null;
      if (!arr.length || !S) {
        ctx!.fillStyle = '#80868b'; ctx!.font = '13px sans-serif'; ctx!.textAlign = 'center';
        ctx!.fillText('검색하면 히트 간 유사도 행렬이 표시됩니다', W / 2, H / 2);
        return;
      }
      const K = arr.length;
      const cell = Math.max(10, Math.min(32, Math.floor(Math.min(W - 200, H - 160) / K)));
      const ox = 120, oy = 70;
      ctx!.font = '10px sans-serif';
      for (let i = 0; i < K; i++) {
        if (cell >= 13 || i % 5 === 0) {
          ctx!.fillStyle = i < 5 ? '#a50e0e' : '#80868b'; ctx!.textAlign = 'center';
          ctx!.fillText(String(i + 1), ox + i * cell + cell / 2, oy - 6);
          ctx!.textAlign = 'right';
          ctx!.fillText(String(i + 1), ox - 6, oy + i * cell + cell / 2 + 3);
        }
        for (let j = 0; j < K; j++) {
          const v = S[i] && S[i][j] != null ? S[i][j] : null;
          ctx!.fillStyle = v == null ? '#f1f3f4' : simColor(v);
          ctx!.fillRect(ox + j * cell, oy + i * cell, cell - 1, cell - 1);
        }
      }
      ctx!.fillStyle = '#202124'; ctx!.font = 'bold 12px sans-serif'; ctx!.textAlign = 'left';
      ctx!.fillText('히트 간 코사인 유사도 — 진할수록 서로 비슷(중복), 옅을수록 다양', ox, oy - 28);
      const lx = ox + K * cell + 24, ly = oy, lh = Math.min(160, K * cell);
      for (let t = 0; t <= 1; t += 0.02) {
        ctx!.fillStyle = simColor(0.3 + t * 0.7);
        ctx!.fillRect(lx, ly + lh * (1 - t), 14, lh * 0.021 + 1);
      }
      ctx!.strokeStyle = '#dadce0'; ctx!.strokeRect(lx, ly, 14, lh);
      ctx!.fillStyle = '#5f6368'; ctx!.font = '10px sans-serif';
      ctx!.fillText('1.0', lx + 18, ly + 8); ctx!.fillText('0.3', lx + 18, ly + lh);
      st.MGEO = { ox, oy, cell, K };
    }

    function draw() {
      const [W, H] = getWH();
      ctx!.setTransform(st.DPR, 0, 0, st.DPR, 0, 0); ctx!.clearRect(0, 0, W, H);
      if (st.VMODE === 'score') { drawScore(W, H); return; }
      if (st.VMODE === 'matrix') { drawMatrix(W, H); return; }
      drawCubeGrid(W, H);
      drawAxes(W, H);
      const XYZ = st.XYZ;
      if (!XYZ) return;
      const N = st.N;
      const searching = st.hits.size > 0;

      // ---- 전 점 1회 투영 → PBUF (hover pick 도 이 버퍼 재사용) ----
      if (!st.PBUF) { st.PBUF = new Float32Array(N * 3); st.ORD = new Array(N); }
      const PBUF = st.PBUF; const ORD = st.ORD!;
      const f0 = Math.min(W, H) * 0.85;
      const cy = Math.cos(st.yaw), sy = Math.sin(st.yaw), cp = Math.cos(st.pitch), sp = Math.sin(st.pitch);
      for (let i = 0; i < N; i++) {
        const x = XYZ[i * 3], y = XYZ[i * 3 + 1], z = XYZ[i * 3 + 2];
        const x1 = cy * x + sy * z, z1 = -sy * x + cy * z;
        const y2 = cp * y - sp * z1, z2 = sp * y + cp * z1;
        const zc = z2 + st.dist;
        PBUF[i * 3 + 2] = zc;
        if (zc <= 0.05) { PBUF[i * 3] = NaN; PBUF[i * 3 + 1] = NaN; ORD[i] = i; continue; }
        const f = f0 / zc;
        PBUF[i * 3] = W / 2 + x1 * f; PBUF[i * 3 + 1] = H / 2 - y2 * f;
        ORD[i] = i;
      }
      // 깊이 정렬(painter's algorithm): 먼 점부터 그려 가까운 점이 가리게.
      ORD.sort((a, b) => PBUF[b * 3 + 2] - PBUF[a * 3 + 2]);

      // ---- 1패스: 일반 점 — 원근 크기 + 깊이 안개 ----
      const zNear = st.dist - 1.05, zSpan = 2.1;
      for (const i of ORD) {
        if (searching && st.hits.has(st.META[i][0])) continue; // 히트는 2패스
        const px = PBUF[i * 3];
        if (!isFinite(px)) continue;
        const zc = PBUF[i * 3 + 2];
        const t = Math.max(0, Math.min(1, (zc - zNear) / zSpan));
        ctx!.globalAlpha = (searching && st.dimBg) ? 0.10 : (0.95 - 0.62 * t);
        const s = Math.max(0.9, st.psize * 1.9 / zc);
        ctx!.fillStyle = pointColor(i);
        ctx!.fillRect(px - s / 2, PBUF[i * 3 + 1] - s / 2, s, s);
      }
      ctx!.globalAlpha = 1;

      // 질의→히트 연결선 + 드롭라인
      const qxyz = st.queryXYZ;
      const qp = qxyz ? projPt(qxyz[0], qxyz[1], qxyz[2], W, H) : null;
      if (searching) {
        for (const [cid] of st.hits) {
          const i = st.byChunk.get(cid);
          if (i === undefined || !isFinite(PBUF[i * 3])) continue;
          const px = PBUF[i * 3], py = PBUF[i * 3 + 1];
          const fp = projPt(XYZ[i * 3], -1, XYZ[i * 3 + 2], W, H);
          if (fp) {
            ctx!.strokeStyle = 'rgba(217,48,37,.16)'; ctx!.lineWidth = 1;
            ctx!.beginPath(); ctx!.moveTo(px, py); ctx!.lineTo(fp[0], fp[1]); ctx!.stroke();
          }
          if (qp) {
            ctx!.strokeStyle = 'rgba(217,48,37,.25)'; ctx!.lineWidth = 1;
            ctx!.beginPath(); ctx!.moveTo(qp[0], qp[1]); ctx!.lineTo(px, py); ctx!.stroke();
          }
        }
        if (qp && qxyz) {
          const fq = projPt(qxyz[0], -1, qxyz[2], W, H);
          if (fq) {
            ctx!.strokeStyle = 'rgba(32,33,36,.3)'; ctx!.setLineDash([3, 3]); ctx!.lineWidth = 1;
            ctx!.beginPath(); ctx!.moveTo(qp[0], qp[1]); ctx!.lineTo(fq[0], fq[1]); ctx!.stroke(); ctx!.setLineDash([]);
          }
        }
      }
      // 2패스: 검색 히트 — top-5 강조
      if (searching) {
        for (const [cid, h] of st.hits) {
          const i = st.byChunk.get(cid);
          if (i === undefined) continue;
          const p = projPt(XYZ[i * 3], XYZ[i * 3 + 1], XYZ[i * 3 + 2], W, H);
          if (!p) continue;
          const top5 = h.rank <= 5;
          const s = top5 ? Math.max(9, 5 + h.score * 7) : 4 + h.score * 6;
          ctx!.beginPath(); ctx!.arc(p[0], p[1], s, 0, 7);
          ctx!.fillStyle = (cid === st.selChunk) ? '#202124' : (top5 ? '#d93025' : '#f3a29b');
          ctx!.fill();
          ctx!.lineWidth = top5 ? 1.5 : 1; ctx!.strokeStyle = '#fff'; ctx!.stroke();
          ctx!.textAlign = 'center';
          if (top5) {
            ctx!.fillStyle = '#fff'; ctx!.font = 'bold 11px sans-serif';
            ctx!.fillText(String(h.rank), p[0], p[1] + 4);
            if (st.labelMode !== 'none') {
              ctx!.fillStyle = '#a50e0e'; ctx!.font = 'bold 12px sans-serif';
              ctx!.fillText(`${st.META[i][1].slice(0, 18)} · ${h.score.toFixed(2)}`, p[0], p[1] - s - 5);
            }
          } else {
            ctx!.fillStyle = '#8c3b35'; ctx!.font = 'bold 9px sans-serif';
            ctx!.fillText(String(h.rank), p[0], p[1] + 3);
            if (st.labelMode === 'hits10' && h.rank <= 10) {
              ctx!.fillStyle = '#b3564e'; ctx!.font = '11px sans-serif';
              ctx!.fillText(`${st.META[i][1].slice(0, 16)} · ${h.score.toFixed(2)}`, p[0], p[1] - s - 4);
            }
          }
        }
      }
      // 질의 점 (◆ 검정 다이아 + 검색어 라벨 필)
      if (qp) {
        ctx!.fillStyle = '#202124'; ctx!.strokeStyle = '#fff'; ctx!.lineWidth = 1.5;
        ctx!.beginPath();
        ctx!.moveTo(qp[0], qp[1] - 9); ctx!.lineTo(qp[0] + 9, qp[1]); ctx!.lineTo(qp[0], qp[1] + 9); ctx!.lineTo(qp[0] - 9, qp[1]);
        ctx!.closePath(); ctx!.fill(); ctx!.stroke();
        const txt = `질의 “${st.curQuery.slice(0, 18)}${st.curQuery.length > 18 ? '…' : ''}”`;
        ctx!.font = 'bold 12px sans-serif';
        const tw = ctx!.measureText(txt).width;
        const bx = qp[0] - tw / 2 - 7, by = qp[1] - 32;
        ctx!.fillStyle = 'rgba(255,255,255,.95)'; ctx!.strokeStyle = '#202124'; ctx!.lineWidth = 1;
        ctx!.beginPath();
        if (typeof ctx!.roundRect === 'function') ctx!.roundRect(bx, by, tw + 14, 19, 9);
        else ctx!.rect(bx, by, tw + 14, 19);
        ctx!.fill(); ctx!.stroke();
        ctx!.fillStyle = '#202124'; ctx!.textAlign = 'center';
        ctx!.fillText(txt, qp[0], by + 14);
        ctx!.beginPath(); ctx!.moveTo(qp[0], by + 19); ctx!.lineTo(qp[0], qp[1] - 9); ctx!.stroke();
      }
      // 선택/hover 링
      for (const [idx, color] of [[st.selIdx, '#202124'], [st.hoverIdx, '#5f6368']] as [number, string][]) {
        if (idx < 0) continue;
        const p = projPt(XYZ[idx * 3], XYZ[idx * 3 + 1], XYZ[idx * 3 + 2], W, H);
        if (p) { ctx!.strokeStyle = color; ctx!.lineWidth = 1.6; ctx!.beginPath(); ctx!.arc(p[0], p[1], 7, 0, 7); ctx!.stroke(); }
      }
    }
    drawRef.current = draw;

    function resize() {
      const [W, H] = getWH();
      canvas!.width = W * st.DPR; canvas!.height = H * st.DPR;
      draw();
    }

    // ---- pick: draw() 가 채운 투영 버퍼 재사용 (O(N) 스캔) ----
    function pick(mx: number, my: number): number {
      const PBUF = st.PBUF;
      if (!PBUF) return -1;
      let best = -1, bd = 81; // 9px^2
      const scan = st.hits.size > 0
        ? [...st.hits.keys()].map((c) => st.byChunk.get(c)).filter((i): i is number => i !== undefined)
        : null;
      const check = (i: number) => {
        const px = PBUF[i * 3]; if (!isFinite(px)) return;
        const d = (px - mx) ** 2 + (PBUF[i * 3 + 1] - my) ** 2; if (d < bd) { bd = d; best = i; }
      };
      if (scan) scan.forEach(check); else { for (let i = 0; i < st.N; i++) check(i); }
      return best;
    }

    // ---- 인터랙션 ----
    let dragging = false, moved = false, lx = 0, ly = 0;

    const onCanvasMouseDown = (e: MouseEvent) => {
      if (st.VMODE !== 'space') return;
      dragging = true; moved = false; lx = e.clientX; ly = e.clientY; canvas!.style.cursor = 'grabbing';
    };
    const onWindowMouseUp = () => { dragging = false; canvas!.style.cursor = 'grab'; };
    const onWindowMouseMove = (e: MouseEvent) => {
      if (dragging) {
        if (Math.abs(e.clientX - lx) + Math.abs(e.clientY - ly) > 2) moved = true;
        st.yaw += (e.clientX - lx) * 0.006; st.pitch += (e.clientY - ly) * 0.006;
        st.pitch = Math.max(-1.5, Math.min(1.5, st.pitch));
        lx = e.clientX; ly = e.clientY; draw();
      }
    };
    const onCanvasWheel = (e: WheelEvent) => {
      if (st.VMODE !== 'space') return; e.preventDefault();
      st.dist = Math.max(0.6, Math.min(8, st.dist * (e.deltaY > 0 ? 1.08 : 0.93))); draw();
    };

    const onCanvasHover = (e: MouseEvent) => {
      if (dragging) return;
      const r = canvas!.getBoundingClientRect(), mx = e.clientX - r.left, my = e.clientY - r.top;
      const tip = tipRef.current;
      if (!tip) return;
      // ---- 점수 축 모드: SGEO 기반 hover ----
      if (st.VMODE === 'score') {
        let g: SGeo | null = null;
        for (const s of st.SGEO) { if ((s.x - mx) ** 2 + (s.y - my) ** 2 <= (s.r + 4) ** 2) { g = s; break; } }
        if (g && st.QRES && st.QRES.hits) {
          const h = st.QRES.hits[g.idx];
          st.selChunk = h.chunk_id;
          tip.style.display = 'block'; tip.style.left = (mx + 14) + 'px'; tip.style.top = (my + 10) + 'px';
          tip.innerHTML = `<b>#${g.idx + 1} ${h.symbol || h.citation.file.split('/').pop()}</b> ` +
            `<span style="color:var(--hl)">${h.score.normalized.toFixed(3)}</span><br>` +
            `${h.citation.file}:${h.citation.start_line}-${h.citation.end_line}`;
        } else { st.selChunk = null; tip.style.display = 'none'; }
        draw(); return;
      }
      // ---- 행렬 모드: 셀 hover ----
      if (st.VMODE === 'matrix') {
        if (st.MGEO && st.QRES && st.QRES.sim && st.QRES.hits) {
          const j = Math.floor((mx - st.MGEO.ox) / st.MGEO.cell), i = Math.floor((my - st.MGEO.oy) / st.MGEO.cell);
          if (i >= 0 && j >= 0 && i < st.MGEO.K && j < st.MGEO.K) {
            const v = st.QRES.sim[i] ? st.QRES.sim[i][j] : null;
            const a = st.QRES.hits[i], b = st.QRES.hits[j];
            tip.style.display = 'block'; tip.style.left = (mx + 14) + 'px'; tip.style.top = (my + 10) + 'px';
            tip.innerHTML = `<b>#${i + 1} × #${j + 1}</b> = <b style="color:var(--hl)">${v == null ? '-' : v.toFixed(3)}</b><br>` +
              `${a.symbol || a.citation.file.split('/').pop()} ↔ ${b.symbol || b.citation.file.split('/').pop()}`;
            return;
          }
        }
        tip.style.display = 'none'; return;
      }
      if (!st.XYZ) return;
      const best = pick(mx, my);
      st.hoverIdx = best;
      if (best >= 0) {
        const m = st.META[best];
        tip.style.display = 'block'; tip.style.left = (mx + 14) + 'px'; tip.style.top = (my + 10) + 'px';
        const cl = (m[8] != null && m[8] >= 0)
          ? `<br><span style="${DOT_STYLE}background:${clusterColor(m[8])}"></span><span style="color:var(--sub)">군집: ${st.CLUSTER_LABELS[m[8]] || ('c' + m[8])}</span>`
          : '';
        tip.innerHTML = `<b>${m[1]}</b> <span style="color:var(--sub)">${m[5]}</span><br>${m[2]}:${m[6]}<br>` +
          `<span style="color:var(--sub)">${m[3] || '-'} · ${m[4]}${m[7] === 1 ? ' · test' : ''}</span>${cl}`;
      } else tip.style.display = 'none';
      draw();
    };

    const onCanvasClick = (e: MouseEvent) => {
      if (moved || !st.XYZ) return;
      const r = canvas!.getBoundingClientRect();
      const best = pick(e.clientX - r.left, e.clientY - r.top);
      st.selIdx = best;
      if (best >= 0) {
        const m = st.META[best];
        setDetailNode(
          <>
            <b>{m[1]}</b> <span style={{ color: 'var(--sub)' }}>{m[5]}</span><br />
            {m[2]}:{m[6]}<br />
            category: {m[3] || '-'} · lang: {m[4]}
            {m[7] === 1 ? <> · <span style={{ color: 'var(--hl)' }}>test</span></> : null}<br />
            {(m[8] != null && m[8] >= 0)
              ? <><span className={styles.dot} style={{ background: clusterColor(m[8]) }} />군집: {st.CLUSTER_LABELS[m[8]] || ('c' + m[8])}<br /></>
              : null}
            <span style={{ color: 'var(--sub)', fontSize: '11px' }}>chunk {m[0].slice(0, 16)}…</span>
          </>,
        );
      } else {
        setDetailNode(null);
      }
      draw();
    };

    canvas.addEventListener('mousedown', onCanvasMouseDown);
    window.addEventListener('mouseup', onWindowMouseUp);
    window.addEventListener('mousemove', onWindowMouseMove);
    canvas.addEventListener('wheel', onCanvasWheel, { passive: false });
    canvas.addEventListener('mousemove', onCanvasHover);
    canvas.addEventListener('click', onCanvasClick);
    window.addEventListener('resize', resize);

    // ---- 부팅 ----
    (async () => {
      try {
        const d: PointsData = await (await fetch('/data/points.json')).json();
        st.N = d.n; st.XYZ = new Float32Array(d.xyz); st.META = d.meta;
        st.CLUSTER_LABELS = d.cluster_labels || [];
        st.byChunk = new Map();
        st.META.forEach((m, i) => st.byChunk.set(m[0], i));
        const cfg: Config = await (await fetch('/config')).json();
        const evr = (cfg.evr || []).map((v) => (v * 100).toFixed(1) + '%').join(' / ');
        setInfoNode(
          <>
            points <b>{st.N.toLocaleString()}</b> · dim <b>{cfg.dim || 1024}</b><br />
            PCA 설명분산: <b>{evr || '-'}</b><br />
            db: {cfg.db.split('/').slice(-2).join('/')}<br />
            model: <b>{cfg.model}</b> · 질의투영 {cfg.projection ? 'ON' : 'OFF'}
          </>,
        );
        st.AXES = (cfg.axes && cfg.axes.length === 3) ? cfg.axes : null;
        setDataLoaded(true);
        setStatusNode(<>{st.N.toLocaleString()} chunks 로드 완료 — 타이핑하면 실제 ckv 로 검색합니다</>);
        resize();
      } catch {
        setStatusNode(
          <span className={styles.err}>points.json 로드 실패 — 먼저 <b>python3 export_projection.py</b> 실행</span>,
        );
      }
    })();
    resize();

    return () => {
      canvas.removeEventListener('mousedown', onCanvasMouseDown);
      window.removeEventListener('mouseup', onWindowMouseUp);
      window.removeEventListener('mousemove', onWindowMouseMove);
      canvas.removeEventListener('wheel', onCanvasWheel);
      canvas.removeEventListener('mousemove', onCanvasHover);
      canvas.removeEventListener('click', onCanvasClick);
      window.removeEventListener('resize', resize);
      if (st.rotTimer != null) cancelAnimationFrame(st.rotTimer);
      if (timerRef.current != null) clearTimeout(timerRef.current);
      drawRef.current = null;
    };
  }, []);

  // ---- 컨트롤 핸들러 ----
  const onColorBy = (v: number) => { setColorBy(v); stateRef.current.colorBy = v; drawRef.current?.(); };
  const onPsize = (v: number) => { setPsize(v); stateRef.current.psize = v; drawRef.current?.(); };
  const onLabels = (v: string) => { setLabelMode(v); stateRef.current.labelMode = v; drawRef.current?.(); };
  const onDimBg = (v: boolean) => { setDimBg(v); stateRef.current.dimBg = v; drawRef.current?.(); };
  const onCamReset = () => {
    const st = stateRef.current;
    st.yaw = CAM0.yaw; st.pitch = CAM0.pitch; st.dist = CAM0.dist; drawRef.current?.();
  };
  const onAutoRot = () => {
    const st = stateRef.current;
    if (st.rotTimer != null) { cancelAnimationFrame(st.rotTimer); st.rotTimer = null; setAutoRot(false); return; }
    setAutoRot(true);
    const spin = () => { st.rotTimer = requestAnimationFrame(spin); st.yaw += 0.0035; drawRef.current?.(); };
    spin();
  };
  const onMode = (m: ViewMode) => {
    const st = stateRef.current;
    st.VMODE = m; setVmode(m);
    setCvLegendVisible(m === 'space' && st.hits.size > 0);
    if (tipRef.current) tipRef.current.style.display = 'none';
    drawRef.current?.();
  };

  // ---- 범례 (색 기준 · 데이터 변경 시 갱신) — JSX 로 렌더 ----
  const legendItems = (): ReactNode => {
    const META = stateRef.current.META;
    if (!dataLoaded || META.length === 0) return '—';
    if (colorBy === 8) {
      const cnt: Record<number, number> = {};
      META.forEach((m) => { const c = m[8]; if (c != null && c >= 0) cnt[c] = (cnt[c] || 0) + 1; });
      return Object.entries(cnt).sort((a, b) => b[1] - a[1]).slice(0, 16).map(([id, n]) => (
        <div key={id}>
          <span className={styles.dot} style={{ background: clusterColor(+id) }} />
          {stateRef.current.CLUSTER_LABELS[+id] || ('c' + id)} <span style={{ color: 'var(--sub)' }}>{n}</span>
        </div>
      ));
    }
    const cnt: Record<string, number> = {};
    META.forEach((m) => {
      const raw = m[colorBy];
      const v = colorBy === 7 ? String(m[7]) : (raw ? String(raw) : '(없음)');
      cnt[v] = (cnt[v] || 0) + 1;
    });
    const disp = (v: string) => (colorBy === 7 ? (v === '1' ? 'test' : 'production') : v);
    return Object.entries(cnt).sort((a, b) => b[1] - a[1]).slice(0, 10).map(([v, n]) => (
      <div key={v}>
        <span className={styles.dot} style={{ background: colorOf(v === '(없음)' ? '' : v) }} />
        {disp(v)} <span style={{ color: 'var(--sub)' }}>{n}</span>
      </div>
    ));
  };

  return (
    <div className={styles.atlas}>
      <header className={styles.header}>
        <h1>ckv <b>Atlas</b></h1>
        <input
          className={styles.q}
          value={query}
          onChange={(e) => onQueryInput(e.target.value)}
          placeholder={'자연어 질의 — 예: "0번 블록" · "gasTip 복원 시 tx 정체" (타이핑 즉시 실검색)'}
          autoFocus
        />
        <select value={kValue} onChange={(e) => { setKValue(e.target.value); stateRef.current.k = e.target.value; }}>
          <option>10</option>
          <option>20</option>
          <option>30</option>
        </select>
        <select value={langValue} onChange={(e) => { setLangValue(e.target.value); stateRef.current.lang = e.target.value; }}>
          <option value="">전체 언어</option>
          <option>go</option>
          <option>solidity</option>
          <option>markdown</option>
        </select>
        <span style={{ display: 'flex', gap: 4 }}>
          <button
            className={`${styles.btn} ${vmode === 'space' ? styles.on : ''}`}
            onClick={() => onMode('space')}
            title="전체 분포·군집 — '무엇끼리 뭉치나'"
          >공간 3D</button>
          <button
            className={`${styles.btn} ${vmode === 'score' ? styles.on : ''}`}
            onClick={() => onMode('score')}
            title="검색 결과의 유사도 축 — '왜 선택/탈락됐나'"
          >점수 축</button>
          <button
            className={`${styles.btn} ${vmode === 'matrix' ? styles.on : ''}`}
            onClick={() => onMode('matrix')}
            title="히트 간 유사도 행렬 — '결과가 중복인가 다양한가'"
          >유사도 행렬</button>
        </span>
      </header>

      <div className={styles.status}>{statusNode}</div>

      <div className={styles.wrap}>
        <div className={styles.side}>
          <h3>DATA</h3>
          <div className={styles.kv}>{infoNode}</div>

          <h3>좌표축 (PC1 · PC2 · PC3)</h3>
          <div className={styles.kv} style={{ fontSize: '11.5px' }}>
            PCA <b>주성분</b> 축 — 1024차원 임베딩을 분산이 가장 큰 세 방향으로 투영.
          </div>
          <div className={styles.kv} style={{ fontSize: '11px', color: 'var(--sub)', marginTop: 4 }}>
            축에 고정된 의미는 없으며 위 요약은 <b>축 양끝에 몰린 코드의 경험적 해석</b>.
            실제 유사도 순위는 1024차원 코사인(오른쪽 패널)이 기준.
          </div>

          <h3>표시 설정</h3>
          <div className={styles.row}>
            <label>색 기준</label>
            <select value={colorBy} onChange={(e) => onColorBy(+e.target.value)}>
              <option value={8}>cluster (의미 군집)</option>
              <option value={3}>category</option>
              <option value={4}>language</option>
              <option value={5}>kind</option>
              <option value={7}>test 여부</option>
            </select>
          </div>
          <div className={styles.row}>
            <label>점 크기</label>
            <input
              type="range" min={1} max={6} step={0.5} value={psize}
              onChange={(e) => onPsize(+e.target.value)} style={{ flex: 1 }}
            />
          </div>
          <div className={styles.row}>
            <label>라벨</label>
            <select value={labelMode} onChange={(e) => onLabels(e.target.value)}>
              <option value="hits">히트 상위 5</option>
              <option value="none">없음</option>
              <option value="hits10">히트 상위 10</option>
            </select>
          </div>
          <div className={styles.row}>
            <label>검색 시 배경 dim</label>
            <input type="checkbox" checked={dimBg} onChange={(e) => onDimBg(e.target.checked)} />
          </div>

          <h3>카메라</h3>
          <div className={styles.row}>
            <button className={styles.btn} onClick={onCamReset}>리셋</button>
            <button className={`${styles.btn} ${autoRot ? styles.on : ''}`} onClick={onAutoRot}>자동 회전</button>
          </div>

          <h3>범례 <span style={{ color: 'var(--sub)', fontWeight: 400 }}>{`(${LEGEND_NAMES[colorBy]})`}</span></h3>
          <div className={styles.legend}>{legendItems()}</div>

          <h3>선택한 점</h3>
          {detailNode ? <div className={styles.detail} style={{ display: 'block' }}>{detailNode}</div> : null}
        </div>

        <div className={styles.center} ref={centerRef}>
          <canvas className={styles.cv} ref={canvasRef} />
          <div className={styles.tip} ref={tipRef} />
          <div className={styles.cvLegend} style={{ display: cvLegendVisible ? 'block' : 'none' }}>
            <div>
              <span style={{ display: 'inline-block', width: 10, height: 10, borderRadius: '50%', background: '#d93025', border: '1.5px solid #fff', boxShadow: '0 0 0 1px #d93025', marginRight: 6, verticalAlign: 'middle' }} />
              <b>검색 히트</b> — 질의와 <b>코사인 유사도</b> 상위 결과 · <span style={{ color: '#d93025' }}><b>원(1~5)</b></span> = top-5, <span style={{ color: '#f3a29b' }}><b>원</b></span> = 6위 이하 (크기 = 점수)
            </div>
            <div>
              <span style={{ display: 'inline-block', width: 9, height: 9, background: '#202124', transform: 'rotate(45deg)', margin: '0 7px 0 1px', verticalAlign: 'middle' }} />
              <b>질의 임베딩 위치</b> — 검색어를 bge-m3로 임베딩해 같은 공간에 투영
            </div>
            <div style={{ color: 'var(--sub)', fontSize: 11, marginTop: 2 }}>
              화면 거리 = PCA 3D <b>근사</b>(설명분산 ~11%) — 실제 순위는 1024차원 유사도(오른쪽 패널).
              연결선은 질의→히트 관계 표시.
            </div>
          </div>
          <div className={styles.hint}>{HINTS[vmode]}</div>
        </div>

        <div className={styles.right}>
          {panelData === undefined ? (
            <div className={styles.empty}>질의를 입력하면 <b>실제 ckv</b>(bge-m3 + sqlite-vec)가 검색하고, 히트가 공간에서 붉게 표시됩니다.</div>
          ) : panelData === null ? (
            <div className={styles.empty}>질의를 입력하면 실제 ckv가 검색합니다.</div>
          ) : (panelData.hits && panelData.hits.length > 0) ? (
            panelData.hits.map((h, i) => {
              const sym = h.symbol || h.citation.file.split('/').pop();
              return (
                <div
                  key={h.chunk_id}
                  className={`${styles.hit} ${hoverHit === h.chunk_id ? styles.sel : ''}`}
                  onMouseEnter={() => { stateRef.current.selChunk = h.chunk_id; setHoverHit(h.chunk_id); drawRef.current?.(); }}
                  onMouseLeave={() => { stateRef.current.selChunk = null; setHoverHit(null); drawRef.current?.(); }}
                >
                  <div className={styles.top}>
                    <span className={styles.rank}>{i + 1}</span>
                    <span className={styles.sym}>{sym}</span>
                    <span className={styles.score}>{h.score.normalized.toFixed(3)}</span>
                  </div>
                  <div className={styles.file}>
                    {h.citation.file}:{h.citation.start_line}-{h.citation.end_line}{h.category ? ` · ${h.category}` : ''}
                  </div>
                  {h.snippet ? <pre>{h.snippet}</pre> : null}
                </div>
              );
            })
          ) : (
            <div className={styles.empty}>결과 없음 (threshold 0.4 미만은 ckv가 버림)</div>
          )}
        </div>
      </div>
    </div>
  );
}
