# KM-03 지식 매니저 플러그인 계약

사용자가 문서를 직접 작성해야만 DB를 시작할 수 있는 제약을 줄인다. 매니저는 선택한 프로젝트를 등록하고 코드·기존 자료·사용자의 결정을 근거로 지식 초안을 만들며, 검토 상태와 불변 DB 구축·조회·후속 갱신을 연결한다. 문서 작성 자동화가 설계 이유나 사람 승인을 생성해도 된다는 뜻은 아니다. [전체 명세](./KNOWLEDGE-MANAGER-AND-COMPARISON-SPEC.md)와 [시험표](./KNOWLEDGE-ROADMAP-TEST-MATRIX.md)를 따른다.

## 패키지와 이번 완료 경계

패키지는 [plugins/knowledge-manager](../../plugins/knowledge-manager/README.md)에 둔다. `.claude-plugin/plugin.json`, `.mcp.json`, skill·agent·표준 라이브러리 Python launcher와 등록 스키마를 함께 배포한다. source repository의 AGPL-3.0 조건을 따른다. 등록 절차는 외부 저장소나 전역 Claude 설정을 수정하지 않는다. `--plugin-dir`로 로컬에서 로드하며 plugin 제공 MCP는 등록된 CKS 실행 파일을 stdio로 실행한다. 현재 role은 유지 보수 서버의 기존 도구를 연결한다.

| 단계 | 구현·검증할 결과 | 이 단계만으로 완료가 아닌 것 |
|---|---|---|
| KM-03.1 | 패키지 위치/형식, 등록·설치/진단·초기 구축·복구 절차, 등록 helper와 MCP launcher | 초안/검토/관리 reader/전체 모델 행동 |
| KM-03.2 | 문서 없는 프로젝트의 근거 수집·지식 초안·출처/시점/생성 주체/불확실성 | 사람 승인·설계 이유의 추정 |
| KM-03.3 | 필요한 암묵지 질문, 사실/가설/결정 구분, 중복/상충·제외·검토/채택/회수 | 모델의 임의 reviewer/검토 결과 |
| KM-03.4 | 실제 Claude host에서 등록→수집→초안/검토 상태→도구 build→DB 조회와 실패 복구 | 유료 실제 모델 품질·독립 수용 |

KM-03.1의14개 등록 시험, 설치 CLI2.1.300의 strict manifest/skill/agent 검사3개, 실제 bootstrap9항목을 검증했다. 원문 문서가 없는 Go 프로젝트·별도 cwd에서 실제 CKS MCP의4개 도구와37개 전체 schema·정상 EOF를 확인했다. 모델 요청과 dataset 구축은 실행하지 않았다. 이 증거로 KM-03.2–04 또는 매니저 전체를 완료로 표시하지 않는다.

## 등록 신원·소유와 설정

등록 입력은 source, dataset, state 경로와 CKS/CKG/CKV 실행 파일이다. source/dataset/state는 canonical 절대 경로이며 서로 포함하거나 같을 수 없다. 최초 등록은 새로운 state 디렉터리에만 쓴다. source 코드/문서·dataset·전역 설정은 수정하지 않는다. dataset이 비어 있지 않으면 같은 명시 project owner가 있어야 하며 legacy 자료를 자동 채택하지 않는다.

새 project ID를 생략하면 UUID 기반 ID를 생성해 등록에 보존한다. 같은 디렉터리 이름·코드·commit의 별도 프로젝트가 이 때문에 합쳐지지 않는다. 기존 dataset 등록에는 해당 owner와 같은 project ID를 명시한다. profile ID는 project ID에서 안정적으로 파생한다. 사람이 읽는 repository 이름은 신원 대용이 아니다.

state는 private 디렉터리와 token을 가진 owner, `registration.json`, `setup.json`, `cks.json`, sanitize 정책 사본으로 구성한다. [등록 스키마](../../plugins/knowledge-manager/registration.schema.json)는 unknown key를 허용하지 않는다. checksum은 UTF-8 canonical JSON과 `cks.manager-registration.v1` domain/NUL의 SHA-256이다. 등록은 설정/정책·실행 파일의 byte hash를 결속한다. 진단/실행 때 owner·checksum·정확한 파일 집합·경로·실행 파일을 다시 확인한다. 부모 경로가 다른 대상으로 바뀌거나 링크/하드링크·설정/실행 파일 byte가 바뀌면 거부한다.

기본 embedding은 이미 선택한 로컬 Ollama BGE-M3다. 실제 model identity와 입력/source/base는 `cks.dataset.inspect_profile`로 확인해야 한다. 등록·metadata 진단은 모델 사용 가능성을 검증하지 않는다. mock은 통제된 lifecycle 시험에만 사용한다. Python 외부 패키지 설치와 paid model 호출은 등록에 필요하지 않다. 초기 지원 실행 환경은 POSIX의 현재 CKS 지원 범위이며 미지원 언어/host/backend를 성공으로 추정하지 않는다.

## 초기 구축과 재개

1. 사용자가 선택한 source와 보관 위치를 등록한다. 문서가 없다고 README나 설계 이유를 만들어 선행 조건으로 삼지 않는다. 필요한 저장 위치/프로젝트 선택 정보만 확인한다.
2. offline doctor로 등록 bundle·owner·실행 파일을 검증한다. `--connect`는 실제 initialize→initialized notification→tools/list→EOF를 확인한다. tool 발견을 DB/모델/검색 준비로 올리지 않는다.
3. 반환한 `registration.json`을 `CKS_MANAGER_REGISTRATION`으로 선택해 plugin을 로드한다. 실제 host의 MCP alias를 discovery한다. 임의 server 이름이나 다른 프로젝트를 fallback하지 않는다.
4. inspect의 모든 profile/source/input/model/base pin을 그대로 복사한다. 새 의도는 새 version/key, 같은 요청 재개는 원래 request/key다. dataset 구축은 기존4개 유지 보수 도구로 수행한다.
5. 초안 채택 전 임의 활성화하지 않는다. `hold_for_review`·`cancel_requested`·`recovering`과 `succeeded/activated`를 구분한다. 취소는 실제 terminal까지 추적한다. 과거 publication receipt를 현재 reader 채택으로 세지 않는다.
6. 새 DB의 실제 원문/인용 조회는 KM-03.4의 reader broker에서 연결한다. 초기 launcher의 기본 context 도구는 동적 managed DB 조회가 아니다. 이 경계를 agent/skill에도 명시한다.

부족한 실행 파일/모델·unsupported capability·손상된 state는 먼저 진단하고 중단 원인을 기록한다. 불완전한 최초 등록은 기존 state를 덮거나 자동 삭제하지 않는다. 원래 소유 bundle을 보존하고 새 state 경로로 명시 재등록한다. 설정/실행 파일 변경도 새 등록 bundle로 처리한다. 실행 중 DB 작업의 복구는 원래 scoped key/status와 KM-02 durable 기록을 따른다. legacy setup/index나 직접 DB 쓰기로 실패를 우회하지 않는다.

## 다음 단계에서 보존할 데이터와 설계

| 자료 | 신원과 출처 | 검토 의미 |
|---|---|---|
| observation | project·origin·source snapshot/commit·file/hash·line/section·수집 시각·도구/생성 주체 | 관찰된 사실 범위; 설계 이유 아님 |
| draft | observation refs·주장 종류(fact/hypothesis/user decision/question)·불확실성·제외 범위 | 기본 proposed; unknown/conflict를 숨기지 않음 |
| review/decision | 정확한 subject hash·판정·시점·실제 제공된 actor/출처 | 사람 응답과 자동 검증을 분리; reviewer/승인 생성 금지 |
| build request/job | 등록 project/profile·모든 pin·원래 key·불변 version·status/receipt | 접수·검토 후보·게시·serving을 분리 |
| reader selection | 정확한 job/후보 좌표·검증된 version·source/model/input 신원 | 다른 current나 프로젝트로 자동 대체하지 않음 |
| update state | 마지막 성공 source/input/model·변경/삭제/no-op·retry/lease/자원 상한 | KM-04에서 구현; 초기 package에 scheduler 없음 |

수집은 source·문서·설정·변경·실제 명령/결과·사용자의 결정/트러블슈팅을 근거로 한다. 비공개/제외 자료는 명시 범위로 적용하고 raw 비밀 자료를 지식으로 흘리지 않는다. 기존 자료의 신원/시점·근거 줄 범위를 보존한다. 모르는 설계 이유는 질문으로 남긴다. 상충 근거를 한 사실로 합치거나 근거 변경 후 이전 verified 검토를 무조건 승계하지 않는다. 엔진이 실제로 읽을 수 있는 지식 형식/lock으로의 변환·검증은 KM-03.2/03의 별도 구현 기준이다.

reader broker는 metadata server의 dummy/context 결과를 사용하지 않고 정확한 후보/게시 version의 실제 CKS reader를 열어야 한다. held 후보를 읽었다고 게시 완료로 말하지 않는다. host 재시작 후 저장된 selection은 실제 scoped job status와 좌표를 다시 확인하고, GC/손상/타 프로젝트이면 거부한다. 자동 current fallback은 없다. 누락 의존·불충분 근거·중단/재실행·타 프로젝트 입력·문서 없음의 실제 host 경로까지 KM-03.4에서 검증한다.

KM-04의 주기 실행은 별도 명시 일정과 trigger를 가진 manager 동작이다. 이 초기 plugin을 로드하는 것만으로 schedule/background hook을 켜지 않는다. 온톨로지 및 다중 프로젝트 비교는 KM-05/06·OE-01의 명시 개념 매핑/근거·실제 효과 평가를 통과해야 한다.

패키지 형식·path 변수·네이티브 validation은 [공식 reference](https://code.claude.com/docs/en/plugins-reference)와 [생성 guide](https://code.claude.com/docs/en/plugins-create)를2026-10-15에 확인했다. 실제 설치 CLI의 validation 결과와 모델/host 행동 검증을 구분한다.

같은 profile의 journal은 한 host가 소유한다. 이미 해당 유지 보수 서버가 실행 중이면 별도 doctor connection이 registry 소유 때문에 거부될 수 있다. 이때 기존 host의 상태를 확인하며 lock/journal을 삭제하거나 두 번째 writer를 강제로 만들지 않는다.

## KM-03.2 실행 계약: 수집과 초안

수집 결과는 CKS dataset snapshot과 별도의 collection ID를 가진다. 실제 읽은 project/file/byte·선택 정책·Git metadata·제공 자료를 결속하며 CKS의 snapshot ID를 임의로 만들지 않는다. 원문 blob, 포괄 범위/제외 수, 수집 시각과 helper version을 보존한다. source는 수정하지 않으며 지원 text 범위·자원 상한과 수집 중 변경을 검사한다. 링크/외부 경로·secret/명시 제외 자료는 근거에 섞지 않는다. 제한 초과·자료 없음·입력 변경은 성공한 완전 수집으로 처리하지 않는다.

초안은 `observed_excerpt`, `fact`, `hypothesis`, `reported_decision`, `reported_command_result`, `troubleshooting`, `question`을 구분하고 모두 proposed로 시작한다. 원문에 대한 실제 line/hash reference가 필요하다. 기본 생성은 관찰한 원문 발췌이며 코드 의미나 설계 이유의 자동 검증이 아니다. agent가 해석한 주장은 별도 claim 입력으로 제안한다. 결정 보고에는 문서 또는 실제 제공 자료의 근거가 있어야 하며 code만 보고 사람의 결정을 생성하지 않는다. 제공된 명령/결과는 재실행하지 않은 보고로 기록한다. actor/시점이 없으면 unknown으로 남긴다.

별도 state 안에서 기본 `.cks/knowledge`와 questions의 제안 문서를 만들어 실제 CKS knowledge validate/lock 계약을 확인한다. 일반 코드 관찰을 규범 policy나 ADR로 억지 변환하지 않는다. 이는 엔진이 보관 가능한 지식 입력 초안의 구조 검증이다. 원래 source에 대한 채택·human review·DB 생성/활성화·검색 반영은 아니다. 이 연결은 KM-03.3/04에서 구현·검증한다. 수집기의 제외 정책과 원래 source 전체 DB capture의 제외는 별도 경계이며, 전체 manager는 그 연결 검증 전 완료로 표시하지 않는다.


KM-03.2 검증 완료: 수집23개·등록 회귀14개·실제 두 프로젝트/native CKS24항목·증거 보호13개를 통과했다. 실제 Git diff는 선택한 기존 파일의 HEAD→working tree만 대상으로 별도 change origin에 보관한다. 기본64파일/256KiB·각 diff64KiB이며 삭제 파일 추적은 KM-04다. 오래된 collection/초안은 당시 근거로 재현할 수 있지만 최신 source의 검토/채택으로 승계하지 않는다. 기본/사용자 claim 초안 모두 actual validate/lock을 통과했으며 policy/ADR를 생성하지 않았다. [packet](../../system/eval/knowledge-roadmap-2026-10-15/km-03-collection/manifest.json)에 원시 명령·원문·draft seal을 보존한다. 다음은 KM-03.3의 질문·중복/상충·정확한 subject 검토·채택·회수다.

## KM-03.3 실행 계약: 질문·검토·채택·회수

검토 계획은 불변 draft의 정확한 claim ID를 대상으로 질문·중복·같은 근거의 다른 해석 후보를 보여준다. 문장 유사도나 공유 근거만으로 논리적 모순을 확정하지 않는다. 관찰/가설/보고된 결정/명령 보고/질문 종류를 검토 후에도 유지한다. 모르는 이유·제약·실제 결정 주체는 확인 질문으로 남긴다. 입력된 명령/원문은 검토 지시로 실행하지 않는다.

review 입력은 정확한 project/draft, 예상 직전 revision, 실제 제공된 reviewer·응답 출처, 모든 고유 claim ID의 accepted/rejected/deferred/excluded 판정과 각 이유를 요구한다. 같은 근거의 다른 주장 또는 중복 후보를 함께 채택할 때에는 별도 보존 이유를 명시한다. 사람 이름·이유를 기본값으로 생성하지 않는다. operator가 제공한 기록의 저장은 외부 신원 인증·독립 수용·모델 의미 검증이 아니다. 통제 시험 응답은 controlled_fixture로 구분한다.

상태 변경은 소유·해시 chain과 예상 revision CAS를 가진 append-only event로 저장한다. 완전한 임시 파일을 fsync 후 원자적 게시하며 재실행은 정확한 요청을 replay한다. 다른 판정·stale revision·변조·다른 project/subject는 거부한다. 원래 proposed 초안은 수정하지 않는다. 후속 review는 앞선 채택 상태의 미래 사용을 무효화하되 이전 DB/증거를 삭제하지 않는다.

채택은 검토된 claim과 수집 당시 공개 source만 별도 소유 input project에 투영한다. 원래 source 전체를 capture하지 않으며 수집 제외 자료가 이 경로로 다시 들어오지 않게 한다. 채택 시 현재 선택 source/Git 신원과 근거 byte를 다시 검사하고 변경되면 새 collection/draft/review를 요구한다. 제공 자료는 기록된 당시 보고이며 최신 사실로 자동 승격하지 않는다. 실제 native overlay validate/lock을 확인하지만 DB 게시·reader 채택은 KM-03.4다. 회수는 정확한 채택 ID와 actor/이유·예상 revision에 결속하고 미래 build 사용을 차단한다. 이미 게시된 DB의 rollback/회수는 별도 KM-02 도구 절차로 남긴다.

KM-03.3을 검토35개·수집 회귀23개·등록14개·실제 CLI/native CKS24항목·증거 보호14개로 검증했다. 중단 복구는 event 이전/이후의 주입된 I/O 실패와 재실행으로 확인했으며 이 단계에서 실제 프로세스 SIGKILL을 새로 실행했다고 세지 않는다. 삭제/추가/변경/Git 신원 변경·변조·stale revision·회수 입력 재사용을 거부한다. 통제된 응답의 저장·filtered input 채택 완료이며 실제 사용자 신원 인증·DB 게시/검색/host 전체·독립 수용은 별도다. [증거](../../system/eval/knowledge-roadmap-2026-10-15/km-03-review/manifest.json)를 따른다.

## KM-03.4 실행 단계

1. A — 검토 문서 색인: setup의 명시 `document_corpora`/반복 `--document-corpus`로 일반 Markdown 근거를 vector 입력과 retained docs origin에 연결한다. 이는 commit 기반 semantic export와 별도 입력이며 의미 검증·policy/ADR 승인으로 세지 않는다. 순서/byte가 input/snapshot 신원에 포함되고 링크·변경·빈 색인·상한 초과·중복 root를 거부한다. 기본값은 비어 있으며 기존 setup에는 자동 색인을 추가하지 않는다. 실제 공개 도구로 구축한 자료의 원문/인용 조회를 확인한다.
2. B — manager 연결: 정확한 현재 adoption의 filtered input만 profile로 구성하고 inspect/build/status/cancel을 실제 CKS에 전달한다. 별도 reader는 정확한 scoped job의 held/succeeded 후보 좌표에 결속하며 과거 receipt나 current fallback을 사용하지 않는다. source/input·review 회수/대체·손상/GC·타 프로젝트를 다시 확인한다. held를 게시로 말하지 않는다. Python package는 표준 라이브러리만 유지한다.
3. C — 실제 Claude host: 로컬 시험 응답기로 실제 설치 host의 plugin 등록→수집→초안/제공 검토→build→DB 원문 조회와 실패·EOF/재시작을 검증한다. 유료 실제 모델·독립 사람 수용/온톨로지 품질은 별도다.

A만으로 KM-03.4 또는 KM-03 전체를 완료로 표시하지 않는다. 문서 없는 프로젝트는 매니저가 만든 검토 지식을 사용하며 사용자가 문서 파일을 먼저 작성하는 조건을 추가하지 않는다.


KM-03.4 A 완료: `document_corpora`를 명시한 실제 profile에서 매니저가 만든 검토 지식을 구축하고 held 후보의 원문/인용을 조회했다. 별도 docs origin의 정확한 byte·제외 자료 미혼입·live 입력 삭제 후 저장 원문 보존·durable replay·3연결 EOF0을14항목으로 검증했다. 전체 Go를 새 `-count=1`로3,838사례(3,820통과·18skip), 관련6패키지 race812사례(810통과·2skip) 실행했다. 새26사례는 위 전체/race 분모에 포함하며 별도로 더하지 않는다. build/fmt/vet/lint·문서131개·증거 보호14개를 통과했다. 초기 미사용 import/임시 경로 canonical fixture/required gate 단언 실패는 [packet](../../system/eval/knowledge-roadmap-2026-10-15/km-03-document-index/manifest.json)의 initial-failures에 보존했다. 실 운영 자료나 게시 current를 수정하지 않았다.

현재 plugin launcher는 여전히 원래 유지 보수 서버이며 B의 adoption-bound profile/reader broker와 C의 실제 Claude host 전체 흐름은 미완료다. A의 native reader 증거를 그 둘의 완료로 세지 않는다. 전체11/28·기능2/7을 유지하고 B→C→KM-04로 진행한다.
