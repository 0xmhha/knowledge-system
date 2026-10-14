# 학습 루프 — 현재 방향 (사용자 비전 기록)

> 목적: 학습(암묵지 축적) 루프에 대한 **사용자의 현재 의도**를 정확히 기록. 이후 별도 repo로 플러그인을 구축할 때의 기준.
> 상태: **방향/설계 의도만. 지금 구현하지 않음.** (플러그인 repo는 사용자가 따로 준비 예정.)
> 기록일: 2026-06-25. 이 문서가 `08`/`09`의 plumbing 가정을 **개정(supersede)** 함.

---

## 1. 제1원칙 — 검토 없이 학습 금지

학습 루프는 **아무거나 학습하면 안 된다.** 잘못 학습하면 **전체 시스템 안정성을 떨어뜨린다.**
따라서 **검토를 통과한 것만** 학습되어야 한다. 자동 학습(auto-verify) 없음 — 사용자/관리자가 **명령어로 직접** 게이트한다.

## 2. 학습 대상 — DB에 없는 "암묵지"

학습 대상은 **코드/DB에 존재하지 않는 암묵지**(도메인 이해, 합의·동시성 규칙, "왜 이렇게" 같은 결정 맥락)다.
- 이것은 기존 **PR→Sync 와 다르다.** PR→Sync = PR이 스쿼시 머지되어 *코드*가 업데이트되면 ckv/ckg를 재인덱싱하는 것(= 코드 재색인). 본 학습은 *코드에 없는 지식*을 사람 검토로 넣는 것.

## 3. 두 부분 — 둘 다 명령어 기반, 사람/관리자 게이트

```
[coding-agent 사용 중 알게 된 fact]
   │  (A) 수집  — coding-agent 명령어로 사용자가 직접 실행
   ▼
[별도 전용 MCP] (cks 아님)  ── fact 수신 → 특정 경로에 파일로 저장(인덱스 아님)
   │
   ▼  (B) 분석 + 학습  — 별도 플러그인의 기능, 관리자가 명령어로 직접 실행
[별도 학습 플러그인]
   관리자 검토 → 선별 → 수정 → ckv / ckg 에 store (학습)
   │
   ▼
[이후 cks 활용 작업]이 학습 데이터를 사용 → 시스템이 더 똑똑 · go-stablenet 이해도↑
```

- **(A) 수집(collection):** coding-agent를 쓰며 얻은 fact를 **coding-agent의 명령어**로 모은다. 저장은 **cks가 아닌 별도 전용 MCP**가 받아 **특정 경로에 파일**로 둔다(인덱스에 바로 쓰지 않음).
- **(B) 분석+학습(store):** 모인 후보를 분석해 ckv/ckg에 학습시키는 일은 **별도 플러그인**의 기능. **관리자가 명령어로** 검토·선별·수정 후 store.
- 둘 다 **사용자/관리자가 커맨드로 직접 수행**. 자동 없음.

## 4. 아키텍처 원칙 (확정)

- **cks는 read-only 유지.** 수집 ingest는 **별도 전용 MCP**가 담당(cks 계약 불변).
- **별도 학습 플러그인** — 별도 **repo**로 준비. **command + skill + hook** 지원, 명령어로 학습 구동.
- **지금 구현하지 않음.** 플러그인 repo는 사용자가 따로 만든다.
- **store 대상(ckv/ckg 구체)** 은 플러그인 구축 시 구체화.

## 5. 기존 설계(continuous-learning-loop.md)와의 관계

- **공통(불변):** 사람 게이트, no-auto-verify, sanitize, "검증된 것만 활성".
- **차이(개정):**
  - 전달: 그쪽 "PR→deterministic sync" → 본 방향 "**전용 MCP ingest(파일) + 관리자 store 명령어**".
  - 거주: 그쪽 "coding-agent 세션층 capture + code-knowledge-system 승격" → 본 방향 "**수집은 coding-agent 명령어 / 분석·학습은 별도 플러그인(별도 repo)**".
  - 트리거: 자동 transition → **명령어 기반(사용자·관리자 직접)**.
  - cks read-only 제약은 **유지**(별도 MCP가 ingest를 맡으므로 충돌 없음).

## 6. 현재 상태 / 정리

- coding-agent에 시험 구현했던 capture-learning skill + Orchestrator 자동 hook 브랜치는 **revert함** — 자동 hook·로컬 queue·coding-agent 내부 거주가 본 방향(명령어 기반·전용 MCP·별도 플러그인)과 어긋나서.
- 지금 coding-agent/cks에 적용한 기능 변경 **없음**(main 깨끗).

## 7. 나중에 구축 시 재사용할 자산 (prior work)

- **수확 필드 매핑** (`09` §3): 세션 산출물의 어느 필드 → 후보의 어느 필드. (`analysis.md` Root cause, `related-code.json` affected_sites, `state.json` 등.)
- **candidate 스키마 / provenance 게이트 / dedup·sanitize 규칙** (`09`).
- **dry-run·backfill 검증·발견** (`09-sample-candidates`, `learning-backfill/DIGEST.md`): 게이트 동작·앵커 fallback·관용 헤딩·신호 밀도·중첩 섹션 수확 등 실데이터 교훈.
- **cks/ckv 사실** (`07` §6): CKV `Filter`에 ChunkKind 없음 / canonical_id 청크 상속(#9) / embedding-identity 강제(#12) / Qwen3 권고 / Slither·CPG 경로(`07` §2b).

## 8. 플러그인 구축 시 구체화할 열린 항목

- 전용 MCP: ingest 도구 스키마, 저장 경로 규약.
- 분석/학습 플러그인: command(수집·큐레이션·store) / skill(분석 로직) / hook(트리거) 정의.
- store 대상: ckv(텍스트) / ckg(앵커·관계) 분담, 빌드·재인덱싱 연계.
- 게이트·dedup 파라미터(임계 등), 암묵지 후보의 타입 체계.
