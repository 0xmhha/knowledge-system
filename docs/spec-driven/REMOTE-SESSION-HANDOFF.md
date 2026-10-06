# 다른 머신의 B/C 단계 작업 인계

## 최신 실행: N-05 보존/GC 완료, N-06 설계 (2026-10-06)

현재 **N-01–05 완료5/18, 잔여13개 N-06–18, 다음 N-06 자원 상한/deadline**. 사용자가 요청한 잔여15개 가운데 N-04/05 2개를 완료했다. [GC DEV 원자료](../../system/eval/b0-knowledge-system/refactoring-n05-2026-10-06/cli-results.json)와 [실행 명세](./REFACTORING-EXECUTION-SPEC.md)를 우선한다. 결정적 dry-run·reviewed plan digest·실제 rollback 참조·reader OS lease·미해제 review/unknown legacy 보호·trash journal/취소/2 process crash 복구를 구현했다. actual CKS/CKG/CKV 5버전·공개 CLI12사례·실제 stdio MCP의 양 backend/alignment/serviceable·v1 pin을 확인하고 current v5를 보존하며 v2/v3만 회수했다. 서버 종료 뒤 v1을 회수했다. 6패키지 race/vet·경계·문서를 검증했다. 구/unleased engine reader를 GC와 병행하지 않는 업그레이드 경계 및 native Darwin 한계를 유지한다. 모든15개 완료 goal은 활성이고 종료/운영 사실을 만들지 않는다. 아래는 이전 시점의 기록이다.

## 최신 실행: N-04 내구성 완료, N-05 설계 (2026-10-06)

현재 **N-01–04 완료4/18, 잔여14개 N-05–18, 다음 N-05 보존/reader/GC 실패 재현과 구현**이다. [명세](./REFACTORING-EXECUTION-SPEC.md)와 [DEV 원자료](../../system/eval/b0-knowledge-system/refactoring-n04-2026-10-06/manifest.json)를 우선한다. DB/WAL/close·전체 파일/디렉터리 sync·current rename/parent sync를 보강했고 8+5 실패 주입 경계, 7 SIGKILL 복구, 실제 CLI 7+2, 9패키지 race·vet·경계·문서를 검증했다. 검토 intent와 release를 분리해 같은 승인/base만 복구하며 post-rename 실패는 durability_uncertain이다. native Darwin 실측이며 Linux/전원 차단/운영 판정을 만들지 않았다. main 동기화/열린 PR0 확인은 완료했고 최신 작업은 기존 feat 브랜치에 보존한다. 전체15개 완료 목표는 계속 활성이다. 아래 문단은 이전 시점의 이력이다.

## 최신 사용자 목표: 15개 완료까지 계속 (2026-10-06)

사용자가 열린 PR 스쿼시 머지/main 동기화 후 N-04–18 전체 완료까지 계속하고 완료 시 알리도록 요청했다. GitHub 열린 PR은 0개였다. main/origin/main은 `1ded9b3e`로 동일하며 main checkout/ff 동기화를 확인했다. 최신 작업 코드/50개 로컬 커밋을 보존해 feat/spec-driven-knowledge-system으로 돌아왔다. 새 PR/전체 로컬 자료 push는 이 조건부 머지 요청만으로 생성하지 않았다. 앱 goal을 N-04–18 전체 완료로 설정했으며 끝나지 않은 목표를 완료로 표시하지 않는다. N-04 reviewed intent/포인터/release 복구 설계를 확정하고 실패 재현·구현을 진행한다. 현재3/18·잔여15를 유지한다.

## 최신 실행 관리: 완료 오표시 방지·N-04 설계 (2026-10-06)

[진행 원장](./REFACTORING-PROGRESS.json)과 `python3 scripts/check-refactoring-progress.py`로 완료3/18·잔여15·다음N-04, 커밋된 수용 명세/원자료/코드 SHA 및 현행 소스/입력 바인딩을 확인한다. N-01의 과거 원자료13/보고서7도 실제 커밋 SHA로 다시 대조한다. N-04는 [실행 명세](./REFACTORING-EXECUTION-SPEC.md)에 DB/WAL·모든 후보 파일/디렉터리·포인터 sync·review-release 복구의 단계별 설계/실패 주입표를 작성했으며 **완료 아님**이다. 다음은 review-release 복구 설계 확정과 단계별 실패 재현/구현이다. 제품 N-04–18 모두 잔여이고 상위 WI10/조건부 운영은 별도다. 후속 source inventory의1100은 Go1098+go.mod/go.sum2 바인딩이다. 아래 문단은 시점별 이력이다.

## 최신 실행: N-03 공통 OS 잠금 완료 (2026-10-06)

[실행 명세](./REFACTORING-EXECUTION-SPEC.md)와 [후속 전체 목록](./REFACTORING-REMAINING-WORKLIST.md)을 우선한다. **N-01/02/03 완료 3/18, 잔여 N-04–18 15개. 다음 N-04 저장 내구성/전환 순서.** 변경 전 긴 live holder 나이 탈취 및 build 중 pointer 승격을 재현하고, 공통 persistent OS flock+기존 pointer lock으로 수정했다. separate process 8 contender 거부/SIGKILL 회수/동일 inode·legacy live PID·symlink/nonregular 거부·Reindex 자체 승격, 실제 CLI build/rollback 동시 요청 거부 및 N-02 7안전 사례, 5패키지 race/vet를 검증했다. [N-03 새 DEV](../../system/eval/b0-knowledge-system/refactoring-n03-2026-10-06/cli-results.json). 기존 `.reindex.lock`는 unlink하지 않는다. 오래된 writer를 모두 중단 후 업그레이드하며 혼합 writer/network FS를 지원한다고 주장하지 않는다. 실제 Linux/전원 차단은 별도 잔여다. 아래 2/18·16개와 제품1096 불변 기록은 당시 이력이다. 원 FINAL/입력5개·사용자 폴더/Ollama 유지, 새 push/운영 배포 승인 없음.

## 최신 실행: 후속 spec-driven 개발 (2026-10-06)

사용자가 후속 작업을 순차 수행하고 매 항목의 목적·설계·수용 조건·실행 증거를 문서로 갱신하며 미완료를 완료로 체크하지 않도록 요청했다. [실행 명세](./REFACTORING-EXECUTION-SPEC.md)와 [전체 목록](./REFACTORING-REMAINING-WORKLIST.md)을 우선한다. **N-01/02 완료(2/18), 잔여 N-03–18 16개. 다음 N-03 OS 잠금.** N-02는 공개 JSON-RPC RED→GREEN, pinned/손상/별칭 쓰기 및 async 우회 거부, legacy 호환·공유 builder, 실제 새 CLI 7사례의 이전 current/원문 SHA 보존 및 동일 신원 후보 보류/승격, 관련 5패키지 race/vet·경계·문서로 검증했다. [새 DEV 증거](../../system/eval/b0-knowledge-system/refactoring-n02-2026-10-06/cli-results.json).

후속 승인 범위에서 제품 코드가 변경됐으므로 아래 역사 기록의 source1096 불변/FINAL 결과를 현재 코드 검증으로 재사용하지 않는다. 원 FINAL과 승인 입력5개는 그대로 보존하며 새로운 소스 바인딩은 N-02 evidence manifest에 기록한다. 긴 writer age reclaim·공통 OS 잠금과 내구성은 N-03/04 미완료다. BGE 품질·native Linux/실운영/파일럿 사람 승인으로 확대하지 않는다. 사용자 `.claude/`, `logs/` 및 Ollama 서버 유지, 새 push/배포 승인 없음. 아래 잔여17/미재현 문단은 N-02 구현 전 이력이다.

## 최신 요청: 원래 리팩토링 목적 대비 잔여 재감사 (2026-10-06)

사용자가 최초 작업리스트/목적 문서를 찾고 현재 코드와 대조해 남은 작업을 정리하도록 요청했다. [최초 목적 재감사](./REFACTORING-ORIGINAL-GOALS-REAUDIT.md)와 [후속 목록](./REFACTORING-REMAINING-WORKLIST.md)을 관리한다. study/main `1c350c1`에서 초기 원문을 확보하고 [시작 배경·전체 ID/v2 대응](./REFACTORING-ORIGIN-AND-V2-TRACE.md)을 확인해 N-01 완료/잔여17개(N-02–18)로 갱신했다. 기존 preview30/30 종료는 유효하며 제품 구현을 새로 완료했다고 표시하지 않았다. 다음N-02 활성 MCP 쓰기 경로의 실패 재현/후보 빌더 계약. 운영 N-15–18은 이전 제외 범위이며 배포/새push/운영 사실 승인으로 확장하지 않는다.

시작은 2026-09-29 baseline1ded9b3e/첫 실행41fa8817. 최초 EXECUTION·v2순서·상세설계 Git snapshot과 current04d3bfdf 코드/문서39SHA의 과거 감사 증거를 보존했다. 초기 요구사항 FR10/INV7/NFR5·S9와 WBS29개 하위ID를 복구 원문으로 확인했고 v2가 실행 기준이다. 현재 차이는 ops.index live write, v2빌드 age/PID잠금(승격만flock), current rename의부모sync없음, GC/보관용량status없음, capture상한CLI/2h builddeadline없음이다. 신규 장애 재현은 아직 수행하지 않은 정적관측이다. actual core20/spec3 proposed, 품질fail/disabled/운영출시보류 유지. 제품1096/기존승인5입력 SHA 동일, 모델/운영설정/사용자폴더 불변.

## 최신 상태: 2026-10-06 실제 사람 승인·시험 preview 목표 종료

사용자 **“HC-01–11 해석 승인. 시험 preview 범위로 종료.”**. [실제 승인 원장](../../system/eval/b0-knowledge-system/human-approved-preview-closure-m2max-2026-10-06/human-approval.json)·[사람/paired 종료](./B1-HUMAN-AND-PAIRED-CLOSURE.md)·[최종 종료 보고](./C1-FINAL-DELIVERY-REVIEW.md)·[작업리스트46절](./EXECUTION-WORKLIST.md)·[현황판](./EXECUTION-STATUS.md)을 현재 기준으로 읽는다. **30/30 완료·승인 범위 잔여0·4/4 stage 판정 완료**. B0 baseline pass/C0 수정·회귀 pass, B1/C1 품질fail·소그룹inconclusive·ontology disabled·운영출시보류다. 작업 종료를 품질 합격/운영 출시로 확장하지 않는다.

HC 해석11/11와 사람 기권2/2를 기록했고 생성 답변 전체3840/4800 오류율을0으로 만들지 않았다. 검토 비용not_measured/null·기록 시각과 메시지 수신 시각 구분. 기존 pending 증거와 승인 전 표는 역사적 snapshot으로 보존하며 frozen5입력/제품1096파일/FINAL/패키지 원자료는 그대로다. OP01–08 실제 운영 역할/키/신뢰/관리/폐기·법무·실운영 환경/백업/실패전환·native AMD64/전체 native 적합성은 사용자 선택 scope 밖·미확인·미승인이다. 시험 키는 운영 키가 아니다.

승인 범위의 다음 작업 없음, 남은 전체0. 별도 품질 개선은 새 DEV와 사전 검토한 독립 FINAL이 필요하며 기존 관측 FINAL을 독립 holdout으로 재사용하지 않는다. 사용자 `.claude/`, `logs/`는 보존·커밋 제외. 새 push/배포 승인 없음. 아래26/30·24/30·pending 기록은 당시 이력이다. [최종30항목 감사](../../system/eval/b0-knowledge-system/human-approved-preview-closure-m2max-2026-10-06/completed-30-reaudit.json)에 완료조건·동결SHA·793평가증거/721패키지증거 검증을 보관했다. app goal 완료 반영 대상이며 이후 재개에 자동으로 운영 출시를 수행하지 않는다.

## 이전 재개 지점: 2026-10-06 보고 완료조건 재검토·26/30 완료

사용자의 다음 진행 지시 후 [작업리스트45절](./EXECUTION-WORKLIST.md)의 원래 완료조건을 다시 읽었다. B1-06은 실제비용/원시시간/환경·순서/build-query 분리와 사람비용 not_measured/null을 [비용 보고](./B1-COST-REPORT.md)에 기록하여 완료했다. 없는 측정을 정직하게 기록하는 일에 추가 승인을 기다린 이전 분류를 바로잡았다. 사람 승인/0비용/절감효과를 만들지 않았다. C1-07은 원래 “명시적 잔여 범위” OR조건의 [지원·known limits·출시보류 보고](./C1-FINAL-DELIVERY-REVIEW.md)를 완료했고 실제 운영scope/OP/HC 잔여는 그대로다.

**현재26완료/4미완료: B1-04·07·08, C1-05.** B1-04의 오개념·정책오용·근거없는이유/사람기권은 미판정이므로 전체0건으로 세지 않는다. HC01–11 실제사람판정·그에 따른 B1-08 비교종료, C1-05 실제운영역할/키·신뢰·적합성/scope 결정을 기다린다. 일반 다음 진행은 이 실제사람판정을 자동 승인하지 않는다. 기존질문이 유지되고 비용미측정 승인은 다시 요구하지 않는다. 다음은 실제HC/운영응답 결합이다.

제품1096/승인5입력/공식캡처는 그대로, 품질fail·소그룹inconclusive·disabled·운영출시보류·officialstage0/4·전체목표미완료를 유지한다. 사용자 `.claude/`, `logs/` 보존·커밋제외, 새로운push권한은없다. 아래24/30 등의 수치는 이전 단계 기록이다.

## 이전 재개 지점: 2026-10-06 기술 수집·최신 패키지/복구 완료, 사람 종료 대기

입력①–⑤ 승인 완료. [현황](./EXECUTION-STATUS.md), [작업리스트44절](./EXECUTION-WORKLIST.md), [DEV](./B0-B1-APPROVED-DEVELOPMENT-REPORT.md), [FINAL](./C1-APPROVED-FINAL-REPORT.md), [최종 패키지](./C1-FINAL-PACKAGE-VALIDATION.md), [최종 종료 검토](./C1-FINAL-DELIVERY-REVIEW.md)를 우선한다. **24완료/6미완료**, official stage0/4. 남은 전체 B1-04·06·07·08, C1-05·07. 아래 과거 수치/승인 대기는 당시 기록이다.

DEV3840/독립 FINAL4800 종료. FINAL 원본 failed_preserved, 정상팩4560 source/공개 Go 유효·POLICY03 오류240. 양성 정적1/6·MRR0.5/6, v1 기대 양성1pass/5miss·두 인용0개 guard fail. raw20/F01 자격/F02 noise 동점/F04 stale/v1 계측40 완료. actual Go retained body33104bytes가32000 상한을 넘는 budget_exceeded 원인을 확인했다. FINAL을 보고 query/gold/K/상한/제품소스를 튜닝하지 않았다. source1096·승인5입력/native 저장소 불변, opt-in/disabled·품질fail·중요그룹inconclusive·운영출시보류 유지.

실행 동결CKS `/private/tmp/ks-approved-scope-20261006/runtime-header-batch/cks` SHA3b80c36d…89d0e6. FINAL raw archive는 `system/eval/b0-knowledge-system/approved-evaluation-m2max-2026-10-06/final-after-development-freeze`와 final-oracles. 최종집계 final-summary-corrected.json. 초기분모/Go exit/record가정 및 진단1회의 공식원장9줄 append는 full/prefix/tail·3SHA·original MID240 복원 증거를 보존했고 original report/rows는 유지했다.

새 checkpoint **10cbffaa104e062052e34d19a27cec3ee8b3548c**에서 최신 Darwin/LinuxARM/LinuxAMD(emulated)를 빌드·서명·두 검증기·mock3project 설치/업데이트/롤백·고지/SHA 검증. Darwin archive재빌드 SHA동일. 새CKS SHAb420a559…22d6e8은 공식runtime과다르며 제품source1096은동일하다. 실제 BGE Darwin guards10(정상6/음성4), Linux3project/SDK6(8인용), strict1598/13820의60SDK/480인용·별도RSS2/16인용 source/public/backend 결합 완료. backend neighbors360/12/Linux6 오류는보존. 단일DEV 비회전질문/공유host/rResidentmodel 비용이며 officialFINAL아니다. LinuxCPU4/4GiB+caller1/512MiB·180초 deadline·실제nativeAMD64없음. 사용자Ollama 유지, 소유Linux서버만종료.

최종 extractedDarwin의 실패/손상·pin/update/rollback·backup새root/source없는재생·구1ded9b3 v1/pristineSHA/재색인/legacyrollback 독립11SDK/11인용/v2integrity6 검증. C1-04/06 기술조건 완료. proof는 `system/eval/b0-knowledge-system/final-package-preview-m2max-2026-10-06`에 약5MB·721파일+inventory, losslessraw/SHA/package비바이너리자산·작은source블롭만보존. archive-only68SDK/504인용 재생통과. 보관helper 경로/키표지 오탐/중복SHA 오류도보존. privatekey/SQLite/modelblob/실행파일은Git제외.

**다음은 사람 응답 기록**이다. HC01–08/실제 검토 비용·C1 시험preview vs실제운영 scope의 기존비동기질문은대기. FINAL추가HC09–11 실제행 해석/기권 질문도요청했다. `not_measured`를0비용이나사람승인으로만들지않는다. 입력①–⑤승인을 출력/운영역할/키/법무/지원승인으로확장하지않는다. reply오면 frozen5입력대신 별도검토파일에 원문·시각·근거SHA를기록한다. 시험scope선택시명시적운영잔여를기록해종료, 운영선택시OP01–08실제역할/키·신뢰/법무/native환경/backup·failover가필요하다. 제품튜닝이나대규모재수집을반복하며기다리지않는다.

사용자 `.claude/`, `logs/` 보존·커밋제외. 이전특정push승인329becf3은처리됐으며 새push승인없음. 이후로컬자료commit은운영배포/push권한이아니다. 매세부완료마다 현재단계·다음·남은전체6항목을화면에표시한다. 기존appgoal blocked지만실제동일목표작업을계속수행했고 목표완료로바꾸지않았다.

작성일: 2026-10-03. 이 문서는 다른 머신의 Codex 세션에 그대로 전달할 작업 프롬프트다. 실행 전 원격 브랜치와 문서의 최신 상태를 확인하고, 이후 변경 사항이 있으면 이 문서보다 최신 커밋과 WBS를 우선한다.

## 재개 후 보고 규칙

2026-10-06 최신 C1-05 기술 보완: [원래 native 출처](./C1-ORIGINAL-NATIVE-ORIGIN-AUDIT.md)는 SQLite 3.53.3 원래 C 공식 SHA3·C/공개 헤더 신원을 배포 모듈에 연결했다. 고정 musl 원문·두 ZIP h1·선택501파일·Linux 헤더436개(원문430/생성4/overlay2)를 대조하고 좁은 생성 명령4개를 재생했다. 49원자료/원문 공백 보존·입력 불변·법무/공식null·전체28미완료다. Darwin SDK·실제 include-use/전체 C→Go 재생성·OP-06은 pending이며 제품/운영 키/출시 변경 없음이다. 다음은 실제 B0-01/02/03/05·STV2 결정→동결→새 strict B0-07이다.

2026-10-05 최신 계측 보완: [FIX-21 neighbors 입력 SHA](./B0-NEIGHBOR-BINDING-DIAGNOSTIC.md)는 수정 전 실패→관련6패키지/race2/vet→같은 BGE/부분13,575청크4 SDK/32인용·본문/Go Verify를 확인했다. 계측 on/off 전체 응답은 같으며 새20실제 seed의 SHA/길이/옵션/결과를 real Go로 일대일 재생했다. 문서12개 symbol없음/no node at·코드8 정상, 과거 미결합360/12의 제한은 유지한다. source/DB/실제 사람 입력5개 불변·39원자료·공식null·미완료28개다. 다음은 실제 B0-01/02/03/05·STV2 결정→동결→새 strict B0-07이다. 기존 FIX-19 preview는 FIX-20/21 최종 검증 패키지가 아니므로 동결 후 재빌드한다.

2026-10-05 최신 추가 확인: [선택 문서 neighbors](./B0-NEIGHBOR-REPLAY-DIAGNOSTIC.md)의 실제 Go adapter 직접8재생은 문서6개 symbol없음/no node at·코드2 정상이다. 기존360/새SDK12전체seed와 직접 결합할 수 없는 계측 한계는 유지한다. 임시 테스트 원문보관 후 제거·제품변경 없음·DB/source/사람 입력5개 불변·5원자료·공식null·전체28미완료다. 다음은 실제 B0/SF/STV2 결정→동결→새 strict B0-07이다.

2026-10-05 직전 기술 수정: [FIX-20 FTS 후보](./B0-FTS-KEYWORD-DIAGNOSTIC.md)는 대규모 DEV 후보8개의 bare FTS 오류를 수정 전2회귀→literal 경계/실제SQLite→관련7패키지/race3/vet→같은 BGE/부분13,575청크2 SDK/16인용·본문/Go Verify로 검증했다. BM25 오류 호출당8→0·기본 근거/좌표·source/DB/model/config/binary·사람 입력5개 불변이다. neighbors12오류는 유지하며 다음 독립 확인은 실제 seed/노드 경계다. 공식 null·전체28미완료·B0/SF/STV2 결정→동결→새 strict B0-07이 최우선이다. FIX-19 preview는 FIX-20 검증 패키지가 아니고 최종 동결 후 재빌드한다.

2026-10-05 이전 비용 진단: [C1 대규모 비용](./C1-LARGE-COST-DIAGNOSTIC.md)의 기존 부분13,575청크·최신 FIX-19/BGE-M3·DEV1문의60 SDK/480인용·본문과 별도RSS2 SDK/16원문·Go Verify를 확인했다. baseline/knowledge warm p95는3.737/4.408초이며 공식 합격/독립표본이 아니다. backend840 오류·bare FTS 후보8개 재현·neighbors 원인 미확정을 분리했다. 입력5개/source/DB/model/binary 불변·48원자료·공식 null·전체28미완료다. 다음 독립 작업은 BM25 후보→FTS 경계 회귀/영향 검토, 최우선 공식 경로는 B0/STV2/SF 결정→동결→새 strict B0-07이다.

2026-10-05 이전 package 검증: [FIX-19 preview](./C1-LATEST-PREVIEW-VALIDATION.md)의 세 대상 시험 서명/설치·Darwin 재현성·Darwin 실제 BGE SDK10과 Linux CPU SDK6/원문/Go Verify/벡터/K/계측/model byte·최신 작은 복구/고정legacy 소비자 재생을 확인했다. Linux90초 initialize 실패와 종료 부근400·65anchor 직접200·후속180초 진단 성공을 분리한다. 초기24 SDK 계획은 미완료이며 후속6을 공식 ablation/지연 합격으로 바꾸지 않는다. 임시 서버 종료·원자료504개/hash 기록·공식 품질/운영 null·입력5개 불변·28미완료다. 다음은 실제 B0/STV2/SF 결정→입력 동결→새 strict B0-07이다.

2026-10-05 이전 기술 검증: [FIX-19 SDK 음성 제어](./B0-RETAINED-GUARD-DIAGNOSTIC.md)에서 정상 pinned 연결 후 전체 vector를 다른 DEV 상태/프로젝트로 교체한 snapshot_mismatch와 blob 변조/삭제의 snapshot_mismatch/source_missing을 수정 전 실패→계약/race/vet→실제 BGE-M3 SDK로 검증했다. 전후 각각10 SDK/정상복원6·20인용·본문/음성4/backend0, 원본·사본·설정·바이너리·모델 불변이다. 이전 비인접 설정 오류와 별도 범위이며 공식 품질/FINAL null·입력5개 불변·전체28미완료다. 다음 독립 준비는 최신 패키지 시험 범위 검증, 최우선 공식 선행은 B0/STV2 결정과 strict 입력 동결이다.

2026-10-05 이전 재감사: [전체 실행 선행 조건](./EXECUTION-READINESS-REAUDIT.md)에 여섯 DEV 가족의 진단/미충족 공식 범위를 연결했다. F-03/F-06 보관 mock/BGE-M3 SDK20응답은 source/투영/inner evidence/public Go Verify·proposed/unknown·baseline 집합 보존을 재확인했다. 모델/FINAL 새 실행 없음. 실제 static/v2 checker 둘 다 exit2/pending이며 정적12승인/동적0승인·protocol/dynamic/scope binding=false다. 다음은 실제 B0-01/02/03/05·STV2-01 결정과 입력 동결→새 strict 공식 B0-07이다. FIX-18 뒤 최종 패키지는 다시 빌드해야 한다. OP-01–08·native/legal/대규모/최종 복구/출시와 전체28미완료를 유지한다. 일반 계속 지시로 개별 결정을 만들지 않는다.

2026-10-05 이전 독립 준비: [F-01 DEV 진단](./B0-F01-SPARSE-DIAGNOSTIC.md)은 새 strict BGE-M3 Go60파일/122청크/truncated0·source61개/Git blob 동일을 확인했다. K5 무필터 대상4파일 누락·eligible Function5개/full inventory·정상 exact·budget2 incomplete/candidate_limit을 충족했고 reader diagnostic_controls_pass/invariant0이다. 원문/모델/DB 불변·FINAL 미실행·공식 품질 null이며 coarse 후보60을 eligible5로 바꾸어 기록하지 않는다. 다음은 F-03/F-06 proposed 원자료/전체 선행 조건 재감사, 실제 B0 결정 후 새 공식 strict 빌드다. 전체 28개 미완료/운영/native 범위는 현황판을 따른다.

2026-10-05 이전 독립 준비: [F-04/F-05 격리 보고](./B0-STATE-ISOLATION-REPORT-PREPARATION.md)의 네 strict DEV BGE-M3 상태·8 SDK/28인용·본문/public Go Verify를 확인했다. FIX-18 실제 old-index/new-source의 stale 인용/fresh=true 모순을 수정 전 실패→query/race/vet·공개 계약→같은 DB 실모델 재조회로 수정했다. fresh=false 외 trace ID를 제외한 응답은 같고 replay source/DB/config/model 불변이다. 혼합 native 설정 SDK는 reindex_required이며 초안 pinned snapshot_mismatch 합격으로 세지 않는다. 일반 계속 지시를 실제 B0/OP 승인으로 대신하지 않았고 입력5개 불변·FINAL 실행 없음·공식 품질 null이다. 다음 독립 작업은 F-01 희소/full eligible·상한 DEV 오라클이다. 전체 남은 작업은 현황판과 작업리스트 26절을 따른다.

2026-10-05 이전 독립 준비: FIX-17에서 archived Go 사본을 원문 유지 `.go.txt`로 바꾸고 최종 조립 경계/원장 검증 순서를 바로잡았다. 상세 실패와 후속 검증은 작업리스트 25절을 따른다.  [F-02 보고 도구](./B0-F02-DOCUMENT-REPORT-PREPARATION.md)의 새 strict DEV/BGE-M3 진단은 203줄/20,290바이트·5 child의 lossless 재조립, 두 raw query의 tail rank1, CKV 3 density×2언어·CKS v2 2응답/16인용·source/public Go Verify를 확인했다. raw/text K10과 knowledge K6·best-effort 실패14회를 분리했고 공식 품질/사람 verdict는 null이다. 입력5개/선택 모델 불변·최종 실행 없음. 다음은 F-04/F-05 상태/프로젝트 오라클 보고 준비다. 사용자는 남은 전체 작업 완료까지 계속 수행하고 세부 완료마다 현재/전체 미완료/다음을 화면·문서에 남기라고 재지시했다. 실제 판단이 필요한 B0/운영 항목은 비동기 검토 요청이 진행 중이다.

2026-10-05 이전 독립 준비: [raw CKV/F-01 보고 도구](./B0-RAW-VECTOR-REPORT-PREPARATION.md)가 실제 K·원시 후보/거리·source SHA·정상 exact·상한 상태를 분리하고 full eligible 목록을 연결한다. 역사적 BGE-M3 Alpha K1은 source archive/Git blob과 같지만 희소 조건/full 목록이 부족해 `fixture_not_qualified`이며 공식 품질 null이다. Python 15+16·Go probe 6/race/vet 통과. 다음 독립 작업은 F-02 장문 꼬리/split·parent 오라클 보고 연결이다. 실제 B0-01/02/03/05·STV2-01 결정 후 새 strict B0 빌드가 최우선이며 일반 계속 지시는 개별 승인으로 기록하지 않는다.

사용자의 2026-10-04 지시: **마지막에는 현재 단계, 다음 작업, 앞으로 남은 전체 작업을 항상 함께 보여준다.** 완료/진행/대기를 눈으로 구분하고 도구 검증을 공식 품질 합격으로 세지 않는다. [EXECUTION-STATUS.md](./EXECUTION-STATUS.md)와 작업리스트를 먼저 읽고 변경 후 현황판을 동기화한다. 이 요청을 일반 진행 지시나 개별 입력/운영 승인으로 해석하지 않는다.

## 2026-10-04 실행 재개 안내

현재 작업의 우선순위, 완료 항목 재검증, 남은 B0/B1/C0/C1 단계는 [EXECUTION-WORKLIST.md](./EXECUTION-WORKLIST.md)를 먼저 확인한다. 정적 gold와 BGE-M3는 승인됐지만 동적 사례·측정 프로토콜은 사람 검토 대기다. 공식 품질과 운영 출시는 완료하지 않았다. 아래 내용은 이전 인계 시점의 기록이다.

최신 후속 검증: [B1 팩 축](./B1-RUNTIME-ADAPTERS.md)의 `--pack-matrix`로 같은 dataset/lock에서 8개 v2 경로를 실행했고 mock/BGE-M3 각 48요청의 6정책 상태·원문/인용/본문·무결성·소스/DB 무변경을 확인했다. 합성 `fixture-reviewer`는 운영 사실 승인 기록이 아니다. [원장](../../system/eval/b0-knowledge-system/b1-pack-matrix-m2max-2026-10-04.json)의 현재 K는 raw/text20·Stage2 cap30·knowledge pass6이며, 초안 K10의 공식 평가가 아니다. B1-02 구현 검증은 완료됐고 다음 독립 작업은 B0-06의 arm 순서 회전·호출별 실제 K·knowledge pass/Stage3 포함 전체 backend 호출 수 원장이다. 아래의 “다음은 B1-02 팩 축”은 이전 체크포인트다. 공식 B0/B1/C0/C1과 사람 판정은 남아 있다.


후속 도구/수정: [B0-MEASUREMENT-TOOLS.md](./B0-MEASUREMENT-TOOLS.md)의 v1/v2 응답 수집·CKV exact/budget 프로브가 구현됐고 실모델 진단을 통과했다. CKV EOF 개행의 가상 인용 줄과 SQLite `?`/`#` 파일명 처리 오류를 재현 후 수정했다. 기존 전체 진단 데이터는 수정 전 빌더이므로 공식 B0-07에서 새 데이터셋으로 범위 감사를 수행한다. B0-06 전체 원장/계측은 남아 있다. [B1 네 실행 경로](./B1-RUNTIME-ADAPTERS.md)의 baseline/개념 텍스트/관계/결합은 실제 MCP에서 mock/BGE-M3 각 20요청·출처/후보 보존/오류 폴백 검증을 완료했다. B1-01 구현 검증이며 공식 품질 통과는 아니다. [B0-05 개발 결합 검토 자료](./B0-SEMANTIC-FIXTURE-REVIEW.md)는 실제 앵커·보관 원문·proposed 팩/스펙으로 준비했고 사람 판정 대기다. F-03 project_id 불일치와 온톨로지+지식 요청의 stamp 순서 오류도 재현 후 수정했다. 다음은 B1-02 팩 축과 B0-06 전체 호출 수·입력 잠금·순서 원장이다. 팩 축·공식 B0/B1·C0/C1은 남아 있다.

후속 B0-06: [선택형 계측](./B0-MEASUREMENT-TOOLS.md)이 knowledge pass·Stage3·health·intent를 포함한 논리 호출과 Ollama HTTP transport 시도를 캡처 measurement_id에 연결한다. mock/BGE-M3 각 48요청과 기존 v1/v2 20요청, 실모델 비계측 12응답 동등성을 확인했다. [호출 원장](../../system/eval/b0-knowledge-system/backend-calls-m2max-2026-10-04.json)에 실제 K20/6·옵션·크기·실패·모델 digest·DB SHA를 기록한다. HTTP 수는 logical 검색 수와 분리하며 startup constructor pin/probe·SQL·모델 내부 작업은 카운터 밖이다. 다음은 arm 순서 회전과 승인 K 적용 경로이며 공식 B0/B1/C0/C1은 대기다.

후속 B1 준비/FIX-16: [비교 보고 도구](./B1-PAIRED-REPORT-PREPARATION.md)가 기존 네 진단 캡처의 624 SDK 응답을 source/integrity/measurement ID에 결합했다. 독립 unit은 묶음별 1이며 inconclusive·공식 품질 null이다. metadata-only 프로토콜/동적 승인과 pending v2 scope 결정의 잘못된 hash 결합을 재현 후 수정했고 42개 관련 회귀·실제 CLI 대기/최종 출력 미생성을 확인했다. 사람 입력 5개·원자료 118개 바이트는 같다. B1-04/06/08은 도구 준비 진행이고 공식 평가/사람 판정은 대기다. 최신 우선순위는 작업리스트 21절의 B0-01/02/03/05·STV2-01 결정과 이후 새 strict 빌드/공식 실행이다.

## 새 세션에 전달할 프롬프트

당신은 `knowledge-system`의 스펙 기반 WBS를 이어서 수행한다. 저장소는 `https://github.com/0xmhha/knowledge-system.git`, 작업 브랜치는 `feat/spec-driven-knowledge-system`이다. 이 인계 작성 시점의 HEAD는 `021c837 fix(cks): preserve executable modes in pinned snapshots`이고 로컬 작업 트리는 깨끗했다. **`main`이 아니라 이 브랜치의 최신 원격 상태를 확인하고 시작하라.** 다른 머신에서는 경로가 다를 수 있으므로 아래의 모든 상대 경로는 저장소 루트 기준으로 해석하라. 기존 사용자 변경이 있으면 보존하라.

### 목표와 이미 확정된 결정

- CKV(벡터), CKG(AST 기반 코드 그래프), CKS(두 저장소의 하이브리드 조정 계층)를 결합하여 임의 프로젝트의 근거 있는 검색·코딩 문맥·설치 워크플로를 제공한다.
- 코어 온톨로지 개념 20개는 유지한다. 산업/조직별 지식·정책·ADR·요구사항은 별도의 사용자 소유 지식 팩으로 확장한다. 원문 검색은 유지하고 온톨로지는 선택형 검토 신호로 사용한다. 정책·설계 의도·테스트 통과·사람의 수용 승인은 서로 다른 사실로 취급한다.
- 원본 PDF 『데이터베이스 설계와 구축』에서 도출한 CKV/CKG 개선점도 범위에 포함된다. `docs/spec-driven/PDF-IMPROVEMENT-TRACE.md`가 PDF 근거 → 요구사항 → WBS → 검증을 잇는다. PDF에서 구현이나 성능 수치를 직접 명령한 것으로 해석하지 말라.
- 대상은 macOS arm64, Linux arm64/amd64이며 실제 임베더는 로컬 Ollama다. 개발은 기능별 명세·실패 오라클·회귀 검증을 따른다.
- 사용자는 먼저 모델 독립 리팩토링을 완료하고 실모델 평가 후 한 번 더 리팩토링하도록 순서를 정했다. D1–D5 설계 승인, A0–A8 구조 개발, B0-H Git 이력 봉인/격리는 완료됐다. 이제 자원이 충분한 머신에서 **B0 → B1 → C0 → C1**을 진행한다. A의 완료를 실모델 검색 품질이나 운영 릴리스 승인으로 표시하지 말라.

### 반드시 읽을 기준 문서

1. `docs/spec-driven/DELIVERY-PLAN-V2.md`: 현재 WBS·선행 관계·종료 증거. D → A → B → C가 전체 순서다.
2. `docs/spec-driven/END-TO-END-DESIGN.md`, 특히 9절: paired 비교, 품질·안전·지연 출시 기준. 표본 수와 질문군별 판정 기준은 결과를 보기 전에 고정한다.
3. `docs/spec-driven/B0-EVALUATION-PREPARATION.md`, `B0-GOLD-REVIEW.md`, `B0-DYNAMIC-FIXTURES.md`: 고정 코퍼스, 질문·정답의 초안 상태, 동적 실패 사례, 측정과 스크립트.
4. `docs/spec-driven/A8-GATE-REPORT.md`, `B0-H-GATE-REPORT.md`, `EXECUTION.md`: 이미 통과한 구조 게이트, 알려진 한계, 실제 실행 기록.
5. `docs/spec-driven/PDF-IMPROVEMENT-TRACE.md`, `DOMAIN-PACK-CONTRACT-V1.md`, `PUBLIC-CONTRACT-V2.md`, `EVAL-CONTRACT.md`: PDF 기반 개선점, 팩·인용·평가 계약.

**자료 이동 주의:** 원본 PDF `/Users/0xtopaz/Downloads/vFlat/데이터베이스-설계와구축.pdf`와 원본 OCR·초기 분석 파일은 이전 머신의 `study/docs/reviews/knowledge-system/`에만 있으며, 이 작성 시점에는 `study` 원격에 커밋되지 않았다. 이 저장소에는 PDF 개선점의 추적 요약이 들어 있다. PDF의 정확한 문구를 다시 확인해야 하면 사용자에게 원본 파일을 새 머신에 안전하게 제공받아 확인하라. 저작권이 있는 책 전체 OCR을 공개 저장소에 자동 업로드하지 말라.

### 현재 평가 상태와 고정 입력

- B0 공식 질문셋 `system/eval/b0-knowledge-system/questions.json`의 12개(코드 위치 3, 설계 이유 4, 정책·충돌·권한 3, 답 없음 2)는 **전부 `draft`**다. 후보 답과 파일·줄은 사람의 의미 검토를 기다린다. 부재를 묻는 두 질문은 고정 코퍼스 전체에서 부재 확인이 필요하다. `b0-export-scenarios.py`는 전부 승인되기 전에는 의도적으로 실패한다. 검토 기록·승인을 만들어내거나 임의로 `approved`로 바꾸지 말라.
- F-01–F-06 동적 실패 사례도 초안이다. 실제 모델 입력, 예상 파일·줄·관계·기권, 검토자와 개발/최종 구분을 결과 확인 전에 고정해야 한다.
- 파일럿 대상은 `knowledge-system`이며 **평가 코퍼스 커밋**은 `71cb71cd55960833e930269e272f7a4a060be3aa`, **tree**는 `f020f8f30fd209b8de045756dedd12ff83f65cd9`다. 이것은 개발 브랜치 HEAD가 아니다. CKG가 버려진 Git 커밋을 읽기 때문에 `git worktree`는 평가 코퍼스 격리 수단이 아니다. `scripts/b0-isolate-corpus.py`로 독립 Git 객체 저장소를 만들라.
- 이전 머신에서는 Ollama 0.34.4와 `bge-m3:latest`(관측 digest `7907646426070047a77226ac3e684fbbe8410524f7b4a74d02837e43f2146bab`, 1024차원)를 시험했다. **이는 최종 모델 승인이 아니다.** 새 머신의 모델 목록·digest·차원·Ollama 버전·메모리·디스크를 다시 확인하고 정확한 모델을 고정하라. 같은 태그가 같은 바이트라는 가정을 하지 말라.
- 이전 실모델 전체 색인 진단은 이후의 해시/UTF-8 및 B0-H v5 수정 전 코드에서 실행됐다. B0-H의 1,569파일/13,469청크 전체 재빌드는 **mock 임베더**였다. 둘 다 공식 B0 품질 기준선이 아니다. **B0/B1 품질 지표는 아직 `null`/`unmeasured`**다.
- 실제 평가 빌드는 `CKV_REQUIRE_COMPLETE_EMBEDDINGS=1` 완전 모드를 사용한다. 축약·누락 청크가 있으면 이를 측정·수정하고 점수 통과로 처리하지 말라. `bge-m3`의 6,144바이트 분할과 정확한 인용/원문 해시를 검증하라.

### 이어서 할 일

1. **환경·입력 확인:** 원격 브랜치 최신 상태, 깨끗한 작업 트리, Ollama 가용 자원·버전·모델 태그/digest/차원, 충분한 디스크를 기록한다. `python3 scripts/b0-preflight.py --model <정확한 모델명>`으로 준비 상태를 확인한다. 이 스크립트는 읽기 전용이며 사람의 정답 승인을 대신하지 않는다.
2. **사람의 정답·모델 검토:** 12개 질문의 후보 답·출처 줄·기권을 검토 가능한 표로 제시하고, F-01–F-06 동적 사례의 기대 결과를 정리한다. 사용자/도메인 검토자의 실제 결정을 받은 항목만 검토자·시각과 함께 승인한다. 모델 선택도 관측 자원과 기존 후보를 제시해 확정한다. 판정 임계치·반복 수·표본 수를 실측 결과 확인 전에 고정한다.
3. **B0 기준선:** `scripts/b0-isolate-corpus.py --source <repo> --out <새 빈 경로> --commit 71cb71cd55960833e930269e272f7a4a060be3aa --tree f020f8f30fd209b8de045756dedd12ff83f65cd9`로 코퍼스를 격리한다. 승인 후 `scripts/b0-export-scenarios.py --out-dir <새 경로>`와 `cks eval --verify-anchors`를 사용한다. 최신 CKV/CKG/CKS 바이너리로 같은 코퍼스·모델·질문·필터·K·잠금 팩을 고정하여 실제 Ollama 기준선을 만들고 원시 결과·빌드 신원·하드웨어·시간·색인 크기·청크 완전성을 기록한다. `cks eval`의 v1 인용 지표를 v2 정책/이유/답변의 의미 정확성으로 오인하지 말고 주장별 사람 판정을 별도로 수집한다.
4. **B1 비교:** 기본 CKV+CKG, 개념 텍스트만, 관계만, 결합, 팩 유무를 같은 조건의 paired 실험으로 비교한다. Recall@K, MRR, 무관 인용, 기권, 오개념 확정, 정책 오용, 근거 없는 이유, 안전·스냅샷 혼입, warm p50/p95·cold 시작, 색인 시간/크기를 질문군별 원자료와 함께 보고한다. F-01–F-06을 실제 모델로 실행한다. 작은 표본으로 결론이 나지 않으면 `inconclusive`로 둔다.
5. **C0/C1:** 관측된 실패를 명세·골든·재현 테스트로 먼저 고정한 다음 필요한 품질/성능 리팩토링을 한다. 기존 공개 계약, 기본 검색 상위 K, 권한·인용·원자 승격, 구 데이터 읽기를 회귀 검증한다. 같은 B0 조건으로 재평가해 온톨로지 런타임 기본값, macOS/Linux 지원 범위, 롤백과 출시 여부를 결정한다. Linux의 실모델 대용량 비용, 운영 서명 키/라이선스 적합성도 C1 판정에 포함한다. 완료 전에는 `preview`/`unmeasured` 상태를 유지한다.

진행 중에는 기능별 검증과 원자료를 `docs/spec-driven/EXECUTION.md` 및 해당 게이트 보고서에 갱신하라. 큰 모델 파일·데이터셋·비밀·책 전체 OCR을 Git에 넣지 말라. 중요한 변경은 커밋/푸시하고, 막히면 정확한 선행 조건과 완료된 범위를 사용자에게 보고하라. 사용자의 승인이 필요한 실제 정답·정책 사실·출시 결정은 임의로 대신하지 말라.

2026-10-04 호출 원장 독립 재감사: v2 106응답·116개 요청 scope·12개 실모델 응답 동등성 확인. 성공 응답 안의 비치명 오류도 보존했다. `<convention>` 합성 지식 경로가 BM25 후보로 전달되는 기존 FTS 문법 오류를 읽기 전용으로 재현했고, 작업리스트 FIX-07로 별도 수정한다. README/header의 그래프 노드 부재는 best-effort 확장 실패다. 계측 구현 통과를 모든 내부 호출 성공으로 판정하지 않는다.

2026-10-04 FIX-07 수정 후 검증: broad recall의 invariant/convention 청크를 코드 검색어로 사용하지 않도록 수정했다. 지식 증거·별도 pass·raw K/필터를 유지했고, 같은 실모델 데이터셋의 8경로 48개 전체 SDK 응답은 수정 전과 같았다. BM25 오류 96→0, 출처/integrity·정책 상태·DB 무변경 재감사 통과. 문서/header의 비심볼 Neighbors 실패는 보존했다. [수정 원장](../../system/eval/b0-knowledge-system/fix07-knowledge-keywords-m2max-2026-10-04.json). 공식 품질은 미판정이며 다음 작업은 arm 회전과 승인 K 적용 경로다.

2026-10-04 공유 K 설정 후속: `retrieval.recall_k`가 raw/concept-text에 같은 값으로 전달된다. 실모델 8경로 48요청 K10과 기본 v1/v2·폴백 20요청 K20을 호출 원장으로 확인했고 knowledge pass K6·Stage2 cap30을 유지했다. [원장](../../system/eval/b0-knowledge-system/recall-k-m2max-2026-10-04.json). 공식 프로토콜은 여전히 사람 검토 대기이며 다음 독립 작업은 arm 순서 회전·공식 입력/하드웨어 원장이다.

2026-10-04 회전 캡처 도구 후속: `cks eval matrix`가 같은 기본 설정의 8개 경로를 순차 호출하고 시작 위치를 회전한다. mock/BGE-M3 각 288응답·36 group·56개 프로세스 시작, warm/cold 분리·기존 SDK 응답 동등성·출처/integrity/정책·K/옵션·모델/입력/HEAD/실행 비트 무변경을 독립 감사했다. 실제 SDK 초기화 실패의 typed-nil cold 패닉은 수정 전 실패/후 96 오류행 보존으로 검증했다. 원자료와 verifier 가정 수정 기록은 [회전 원장](../../system/eval/b0-knowledge-system/matrix-capture-m2max-2026-10-04.json). 공식 품질/사람 판정과 B1-03 공식 실행은 대기이며 다음 독립 작업은 승인 입력·전체 환경 원장 연결과 C1 검토 자료다.

2026-10-04 환경 원장/Linux 후속: `cks eval matrix --environment-ledger`의 모델·하드웨어 전후 관측과 lock 원문 사본을 실제 MCP에 연결했다. macOS mock/BGE-M3 각 24응답, 이후 Linux arm64·amd64 에뮬레이션 각 3프로젝트·72응답을 검증했다. Linux ARM CPU 식별자·`/proc` process 압력·cgroup 제한 수집 실패/후 수정 증거는 [작업리스트 12절](./EXECUTION-WORKLIST.md)에 연결한다. 공식 품질은 계속 null이며 다음 독립 P1은 정적 gold/프로토콜의 개발·최종 경계를 검사하는 입력 preflight 준비다. 범위·프로토콜·동적 입력·의미 사실의 사람 승인과 Linux 실모델/네이티브 amd64·운영 출시 판정은 남아 있다.

2026-10-04 정적 입력 경계 후속: [B0-STATIC-INPUT-PREFLIGHT.md](./B0-STATIC-INPUT-PREFLIGHT.md)의 도구가 정적 개발 4/최종 8·동적 12개·입력/검토 해시·모델·K/임계치 선언을 검사한다. 현재 프로토콜/동적 미승인으로 pending이며 개발 v1 4개만 진단 준비, 최종/ready 발행은 거부했다. 기존 질문 ID의 경로/대소문자 충돌을 다섯 실패로 재현 후 수정했고 정상 이전 export 13개 파일은 동일했다. 실제 독립 코퍼스도 재감사했다. 26개 Python 회귀 통과이며 모델/MCP/최종 질문 평가를 실행하지 않았다. 다음 독립 작업은 C1-05(P3)의 나머지 native 고지·운영/지원 검토 자료이고 공식 B0 선행 승인은 남아 있다.


2026-10-04 C1-05/FIX-12 후속: [작업리스트 14절](./EXECUTION-WORKLIST.md)의 Tree-sitter runtime 44개·Solidity 6개를 고정 upstream과 대조했다. Unicode/runtime/Go toolchain 고지를 추가한 50고지·110항목 macOS preview가 기존 검증기 100항목 상한에 걸려 수정 전 실패/후 256통과·257거부를 검증했다. 두 서명 검증기·세 프로젝트 설치/재시작/업데이트/롤백·재서명 Unicode 변조 거부가 통과했다. 운영 키/라이선스 적합성/공식 품질은 미판정이고 다음 독립 작업은 다른 native 범위·최신 Linux recipe 재검증이다. 승인된 공식 입력 실행은 계속 선행 사람 결정 대기다.


2026-10-04 추가 native/플랫폼 후속: [작업리스트 15절](./EXECUTION-WORKLIST.md)에서 JS/TS·SQLite/vec의 고정 upstream 41개 원문 일치를 확인했다. 호스트 스모크의 오래된 두 자산 가정은 FIX-13으로 수정 후 세 프로젝트 재검증 통과. 최신 Linux arm64·amd64 에뮬레이션은 33모듈/48고지·두 시험 서명 검증기·Go 없는 런타임의 세 프로젝트 설치/복구 통과. native amd64/Linux 실모델/운영 판정은 미완료다. 같은 Linux 바이너리의 반복 ldd 주소가 metadata를 바꾸는 FIX-14를 재현하여 다음 P1로 등록했다. 늦은 source 대조는 임시 빌더 종료 때문에 수행되지 않았다. 서명·설치 통과를 Linux 재현성이나 공식 품질로 세지 않는다.


2026-10-04 FIX-14 후속: [작업리스트 16절](./EXECUTION-WORKLIST.md)의 ldd load address를 제거하고 첫 의존/정적 링크 진단을 유지했다. Linux 각 대상의 actual copy-input 4,624개 대조·같은 합성 commit의 공개 packager fresh build 두 번·106개 항목/전체 archive SHA 동일·하나의 시험 서명을 두 archive의 Python/Go에서 검증·세 프로젝트 설치/복구를 통과했다. 실제 macOS dependency metadata도 유지했다. [운영 검토표](./C1-OPERATIONS-REVIEW.md)의 OP-01–08은 미정이며 시험 scope/키로 운영 신뢰를 승인하지 않는다. B0 범위·프로토콜/동적 입력·의미 사실 결정을 다시 요청했다. 다음 독립 작업은 Linux 실제 모델 가용성/진단과 native/translated source 검토 자료이며 공식 품질·운영 판정은 남아 있다.


2026-10-04 Linux 실제 CPU 모델 후속: [작업리스트 17절](./EXECUTION-WORKLIST.md)의 Linux arm64 Ollama 0.35.1/승인 BGE-M3 bytes와 API/DB 1024차원·CPU residency를 확인했다. 세 인공 프로젝트 설치/재시작/업데이트/rollback, 72 v2 응답/96 인용/48 startup scope·보관 SHA/integrity·실제 입력/DB/HEAD·5 저장 벡터를 검증하고 전용 서버를 정리했다. 초기 Python 추출 filter 오류, TypeScript 기존 20초 initialize 실패/90초 별도 client 성공, parser-node/전체 줄 verifier 가정 수정은 보존했다. 공식 품질은 null, C1-04는 준비 진행이며 운영 지원 승인이 아니다. 다음 독립 작업은 translated/native/transitive source 범위와 실패 복구 자료, 사람 B0 결정 이후에는 정적 v2 범위/K 동결과 공식 strict 빌드다.


2026-10-04 FIX-15 후속: [작업리스트 18절](./EXECUTION-WORKLIST.md)의 modernc SQLite 별도 SQLITE-LICENSE 누락을 재현 후 수집/누락 표시를 수정했다. 새 Darwin 51고지·Linux 두 대상 49고지의 시험 서명/세 프로젝트 mock 설치·두 검증기 재서명 고지 변조 거부와 Python 16개 회귀(Go 연동)를 통과했다. modernc 선택 source Darwin 225/Linux arm64 197/amd64 에뮬레이션 201개를 module ZIP/h1에 대조했다. 원래 C/header/native 전체 적합성·사람 운영 결정과 공식 품질은 남는다. 다음 독립 작업은 실패 복구/원본 보존 검증, B0 사람 결정 이후에는 정적 v2/K 동결과 공식 strict 빌드다.


2026-10-04 실패 복구/legacy 후속: [작업리스트 19절](./EXECUTION-WORKLIST.md)과 [복구 문서](./C1-RECOVERY-VALIDATION.md)에 최신 Darwin 시험 preview의 실패/손상 거부·pin/update/rollback·새 루트 백업 복원·소스 없는 보관 재생을 기록했다. 구 v1 baseline의 첫 신규 소비자 전 payload SHA·별도 재색인·구 v1 rollback도 검증했고 SDK 11응답/11인용·v2 integrity 6개를 독립 재생했다. 빈 WAL/SHM은 별도 기록이며 운영 live backup 보증이 아니다. C1-06은 진행이고 최종 운영 후보/OP-08·공식 품질은 미완료다. 최상위 P0 사람 결정 후 v2 범위/실제 K 동결과 B0-07부터 공식 실행한다.


2026-10-04 재개 후 정적 v2 범위 후속: [작업리스트 20절](./EXECUTION-WORKLIST.md)의 STV2-01은 실제 corpus committer time에서 query date=2026-10-01, corpus_project에서 subsystem=knowledge-system, runtime K10 patch를 제안한다. 새 6개/기존 10개 회귀 및 실제 CLI의 pending·불변 출력/gold·입력 SHA 검증을 통과했다. source-root .cks는 비어 있어 외부 검토된 pack/semantic inventory·잠금은 별도 선행 조건이다. 최종 프롬프트/답·공식 점수·승인 상태를 발행하지 않았다. B0-01/02/03/05와 STV2-01 결정을 요청했으며 이후 실제 K/config와 새 strict 빌드에 연결한다.


### 2026-10-06 N-06 완료 / 다음 N-07

공개CLI의 상한/deadline 무시 RED4개를 측정하고 실제CLI12사례 및 상한/외부origin/취소/프로세스/내구성 경계와 관련race로 검증했다. [실행 명세](./REFACTORING-EXECUTION-SPEC.md), [N-06 DEV](../../system/eval/b0-knowledge-system/refactoring-n06-2026-10-06/manifest.json)를 기준으로 한다. N-01–06 완료6/18, 요청15개 중3완료, 잔여N-07–18 12개. 다음은N-07 본문32000bytes/12citation의 원문span선택·partial/typedbudget 계약이다. N-11 새로운 사전평가/실제파일럿/운영은 이전preview 승인으로 채우지 않는다. 열린PR없음,main/origin/main1ded9b3동기화; 후속로컬소스는기존feature브랜치이며새PR/푸시는아직수행하지않았다.


### 2026-10-06 N-07 완료 / 다음 N-11

[N-07 새 DEV](../../system/eval/b0-knowledge-system/refactoring-n07-2026-10-06/manifest.json)는 본문32000bytes/12citation의 전체 span 선택·partial 진단·typed budget 오류를 검증했다. registered public RPC에서 경계를 재현했고 native 실제 엔진 MCP는 upstream 큰본문 제외로 별도 호환성 증거다.13개 패키지 race와 byte/hash/sanitize/scope/semantic 경계를 통과했다. N-01–07 완료7/18, 요청15개 중4완료, 잔여N-08–18 11개. 다음은 튜닝 전 N-11 새 사전평가 입력/프로토콜, N-12 실제 내용 검토, N-13 팩 분리다. 이전 FINAL을 새 holdout/새소스 합격으로 재사용하지 않는다.

### 2026-10-06 N-13 선택형 worksheet catalog 완료

[N13 DEV](../../system/eval/b0-knowledge-system/refactoring-n13-2026-10-06/manifest.json), [catalog 계약](./WORKSHEET-CATALOG-CONTRACT-V1.md). 기존 hardcoded StableNet8항목을 명시 선택 pack 자료로 이전하고 version/digest/의존/type/출처SHA·좌표를 검증한다. 무팩/구팩 공통경로,8매핑/필터/앵커/빈queue, 실제CLI11회와 기존 파일80개 변경0/자동승격0, 관련5패키지 race 통과. 앵커 검사를 실행하지 않는 생성기가 검사 완료를 주장하던 문구와 존재하지 않는 승인 CLI 예제도 교정했다. N-01–07/N-13 완료8/18, 요청15개 중5완료, 잔여N-08–12/N-14–18 10개. 다음은N-11 새 평가와N-12 실제 내용 검토 자료; 운영 환경/담당/RTO/RPO 질문은 답변 대기다. 새 원자료 푸시/배포는 하지 않았다.

### 2026-10-06 N11/N12 검토 입력 준비 — 완료 수 증가 없음

N11 [DEV7개/프로토콜 초안](./N11-FRESH-EVALUATION-PREPARATION.md)은 source-bound이며 미승인, 새 FINAL0, 실행0, execution_allowed/release_eligible false다. N12 [실제 검토 자료](./N12-ACTUAL-PILOT-REVIEW.md)는20타입/3요구/4기준·27원문span SHA와6정확테스트를 대조했다. 모든 객체 proposed, 사람 판정0, 실제 통합 snapshot/lock/관계 및 N14 변경 수용 미완료다. 사용자에게 내용/후보 관계 판정·검토자·실제 활동 시간을 요청했다. N15–17 native Linux/운영 담당·공개키 신뢰/RTO/RPO 질문도 대기다. 완료8/18(요청15개 중5), 잔여N08–12/N14–18 10개를 유지한다. 이 준비를 합격 또는 사용자 승인으로 기록하지 않는다.
