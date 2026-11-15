# Graph build design before vectors (decision record, 2026-11-15)

Interview held on 2026-11-15 on the build machine, after the go-stablenet graphs were rebuilt with the D2-D4 and D8
fixes and `ckg verify` was corrected (#149). Raw interview records stay outside the repository
(`~/.claude/decisions/knowledge-system/20261115-graph-build-design/`); the user's answers are quoted verbatim at the
end. The session closed at an ambiguity of 0.148 (threshold 0.2, twice in a row) with all 14 decisions above the
decision-quality gate.

## Goal (confirmed by the user, "맞다")

Build the graph only from the code a build command actually includes and its tests; resolve it per language at the
level of the compiler; bind the definitions and implementations the linker joins in each build configuration; add
runtime links (interface implementations, goroutines and channels), data flow and change intent (commits, PRs); and
put the per-language parts under one shared abstraction. In this graph, callers and callees, and writers and readers
of a value, can be followed hop by hop from any symbol, so that an LLM finds the root cause of a bug and the fix
point accurately with fewer tokens, also in large mixed-language projects.

Priority order of values for all decisions (user accepted: "추정이 맞아"): accuracy (no construction bugs) > scope
(many languages, large complex projects) > structural simplicity (low dependency, coupling, complexity) > build cost
(time, memory) > schedule (seeing pack improvements early).

## Decisions

| ID | Decision | Why | Rejected | Weakest element |
|---|---|---|---|---|
| D1 | Correctness is shown by a compiler-backed verifier per language plus cross-language link metrics | `ckg verify` against go/types found the bn256 node-id collision and the main/init lookups; fixtures (B) cover only their cases and consistency checks (C) only form | B hand-made fixtures, C consistency checks plus Go only | information 0.75 |
| D2 | Shared symbol contract plus per-language frontends that resolve with their own compiler tools; the core handles only ids | 8 duplicated pipelines and two MakeID copies produced D8 and D11 | B keep and patch, C build on SCIP (a frontend may still wrap a SCIP indexer, option D) | information 0.75 |
| D3 | Go runtime links come from SSA plus VTA inside the Go frontend; build-file extraction is a separate capability; analyses such as SSA live inside each language implementation | no edge from interface methods to implementations; most channel edges point to call sites | B CHA (noise on wide interfaces), C traces as the source (they are the precision check instead) | information 0.8 |
| D4 | Change requests form a git-like DAG of nodes and edges as the source; summary attributes on declarations are computed from it | no commit-to-commit edges, no messages, pack composer reads no history; the right structure is unknown, so both are kept to be measured | A only, B only | information 0.75 |
| D5 | Order: shared contract design, build-scope extractor, Go frontend on the contract (fixes D11, D12), Go verifier, test layer and configuration labels, Go SSA/VTA, history DAG, go-stablenet's Solidity/TS/Proto, then vectors and pack measurement, then the remaining languages one by one; the upper layer is one shared abstraction | fixing D11 in the old code is discarded at the refactor; schedule ranks last | B fix first and measure, C all languages shallow, D early D11 fix | information 0.75 |
| D6 | Test code is a layer: default search uses production only, tests come in a `related_tests` slot or on request | in tc-2026-11 bge-m3 pr-67, 5 of 13 bodies were tests and the owning function was cut despite the 0.25 demotion | B stronger demotion, C `exclude_tests` by default | commitment (planned) |
| D7 | The files in the graph are the files the build command includes (go list per target and tags), plus their tests; main packages are listed, and projects swap vendor implementations at build time (kernel) | `--files-from-main` keeps whole directories, so files of other build tags (gfp_decl.go and gfp_generic.go) both entered | B whole module, C union of targets | reasoning 0.5 |
| D8 | One graph with build-configuration labels; outside modules as the declarations the project uses; the file-list extractor may be a separate project | gstable pulls 94 outside modules and none of their code is in the graph; vendor implementations per configuration must be comparable | B graph per configuration, C outside modules in full, D allowlist (can be added later) | reasoning 0.5 |
| D9 | Languages: Go, Rust, Python, C, C++, C#, TypeScript, JavaScript, Java plus Kotlin, Swift, Solidity, Proto and IDLs | large complex projects are the target; IDLs and contracts join mixed systems; Solidity and Proto frontends exist | B nine plus Solidity and Proto, C plus Scala and Objective-C | reasoning 0.5 |
| D10 | Server builds the full graph and heavy analysis, local machines add incremental changes; limits: server 1 h and 32 GB at go-stablenet scale, 8 h and 64 GB at kernel scale, local 30 s and 8 GB per edited file | user: low-spec machines need the server, what fits runs locally, the long-term aim is everything local, and GitHub-link projects of non-developers need a cloud server | A local only, B server only | none below 1 except commitment |
| D11 | Numbers fixed per language before measuring: misresolved 0, unsupported edges 0 (except recorded verifier limits), missing declarations 0, call recall >= 0.99 and type-reference recall >= 0.95 for static languages, Python and JS bounds set after their first measurement and before use, incremental equals cold, runtime-link precision >= 0.9 against test traces, fewer intent violations on the intent task, packs by the D6 rule plus pack precision | the measurement rule fixes criteria before results; Go already reaches 0.9986 call recall | B correctness only, C criteria after measuring | reasoning 0.5 |
| D12 | Overlapping mechanisms are replaced (test demotion once the layer exists; `changed_in` and `node_prs` once the DAG exists); parts with another role stay (vector-store PR and commit text for text search, linked by change-request id); baseline indexes and evaluation records stay | two mechanisms for one job make pack changes impossible to attribute | B keep all, C history only in the graph | information 0.75 |
| D13 | Compiler tools follow the toolchain the target project declares, installed from official sources, version and checksum recorded per build; a pinned default when nothing is declared | on the build machine solc-select failed with HTTP 403 and Homebrew protoc failed to start; go-stablenet declares go 1.22 while go 1.26.8 is installed | A own pinned versions, B in-process libraries | reasoning 0.5 |
| D14 | Graph queries aim at the root cause and fix point with the fewest tokens: from the symptom walk callers upstream and data flow (writers of the values read; `reads_field` must be emitted), cost edges by root-cause relevance, use ALT A* as the search engine, return a compact structure first and bodies only when the LLM asks through a walk tool; judged by tokens and correctness to the fix point | callers and callees are depth-2 BFS today, there is no path search, `reads_field` is never emitted; the user defined "efficient" as fewer tokens to a correct fix | A ALT only, B bidirectional Dijkstra, C semantic A* (not admissible), D ALT plus semantic re-rank | alternatives 0.5, reasoning 0.5 |

Dijkstra and A* are best-first searches over a priority queue, not depth-first; A* is faster only with an admissible
heuristic, which landmark distances (ALT) give on any graph.

## Constraints and what is not done

- No graph from all files: only the build command's files and their tests (D7, D8).
- SSA and similar analyses are Go-only, inside the Go frontend (D3).
- Outside modules enter as declarations only (D8).
- Vectors are rebuilt only after the graph stages of D5 for Go and go-stablenet's other languages.
- Python and JavaScript have no compiler-grade truth; their bounds are set before they are used for a judgment (D11).
- Linker-level binding still to be specified per language: C/C++ object and library symbols, weak symbols, linker
  scripts; Go `//go:linkname` and init order; code dropped from the binary; dynamic libraries and plugins.

## Completion criteria

As D11, per stage, plus the D10 limits. Each stage ends with `ckg verify` (or the language's verifier) numbers and,
from the vector stage on, pack measurements on go-stablenet (development set) and indexer-go (holdout, D6 rule).

## Assumptions (not decided by the user)

- Each required tool has an official distribution channel (accepted by the user for D13, "추천 A 진행"); checked tool
  by tool when the build-scope extractor is built.
- go-stablenet's deployment target is linux/amd64 (used for the 128-package closure; not confirmed).

## Deferred

- Fields of the derived history summaries, how intent is inferred when a commit has no message, and the granularity of
  history links (D4).
- Edge-cost values for the root-cause walk and how relevance is measured (D14); fixed before measuring per D11.
- Repairing the build machine's protoc (missing abseil library) and solc (solc-select 403).
- The 2026-11-03 decision folder still holds its raw records in the repository (#139).

## Answers as given

## 결정 기록 원문: Graph build design before vectors: correctness, languages, structure, runtime links, history

### D1. Completion criterion: what counts as a graph with no construction bugs, per language and across languages (decided)
- 질문 9 [defined]: D1: what counts as a graph with no construction bugs? A per-language compiler-backed verifier plus cross-language link metrics / B Go only plus hand-made fixtures / C language-independent consistency checks plus Go
  - 답: 결정 D1은 추천 A "언어마다 컴파일러 기준 검증기 + 언어 간 연결 지표"

### D2. Frontend structure: shared contract and per-language frontends, lower coupling and complexity (decided)
- 질문 5: user instruction 2026-11-15
  - 답: 그래프 db 생성에 대한 알고리즘을 어떻게 정리할것인지, 여러 언어 지원을 위한 확장을 어떻게 가져갈것인지도 고민되고 코드 구조가 개선되어야 한다. 종속성, 결합도, 복잡도가 낮아지도록 해야 한다.
- 질문 12: user instruction 2026-11-15 (local declarations)
  - 답: 함수 안의 지역 타입, 지역 선언 문제는 변수의 라이프 사이클을 고려해야해.
- 질문 15 [defined]: D2: structure for per-language graph building. A shared symbol contract plus per-language frontends that resolve with their own compiler tools / B keep the structure, fix defects per language / C build on SCIP indexers
  - 답: 추천 A로 진행

### D3. Runtime links: interface to implementation, goroutines, channels, callbacks (decided)
- 질문 6: user instruction 2026-11-15
  - 답: 현재는 코드가 실행 바이너리로 동작하는것은 모든 함수와 변수, 동적 처리까지 코드가 다 연결된다는것에서 착안한 아이디어로 코드를 컴파일할때 진행하는 ast 기반으로 분리해서 연결 구조를 만드는것이었는데, 링커가 링킹하는 알고리즘이 누락되어, 인터페이스의 연결이 누락되는 문제가 있었다. 이는 고루틴으로 동적 생성 및 동작하는 로직들도 이어지지 못하는 문제가 존재했다는 것을 의미한다. 사람이 코드를 분석할때는 동적으로 처리되는것을 머릿속에서 이미지로 형상화하여 동적으로 생성된 루틴에 의한 데이터 전달을 고려하고, 그에 따라 채널을 통해 데이터가 들어오는 경우도 생각하게 된다. 즉, 연결이 없어보이는것도 런타임에서는 이어지는 것도 사람은 이어서 생각하는데, 정적 코드만으로는 이 부분이 부족하다.
- 질문 24: user question 2026-11-15
  - 답: SSA 방식이 좋은 방식이야? 지금 상황에 맞나? 실제 빌드 될때 처리되는 파일이 아니라 다른 파일을 포함하면 데이터 오류가 발생하지 않아?? 빌드 명령어들 중에 실제 빌드에 포함되는 파일을 추출하는 방식이 있지 않어??
- 질문 26 [defined]: D3: runtime links. A SSA plus VTA value flow / B CHA type hierarchy / C runtime traces from tests
  - 답: ssa 적용은 그것대로 필요하긴 한데, 이것은 go언어의 경우에서만 동작해야하고, 이것과 별도로, 실제 빌드 바이너리에 포함되는 파일리스트 추출 기능은 따로 필요해 보여. 그리고, 언어별 특징을 고려해서 공통된 형태의 지점들을 추상화하고, 추상화된 부분을 언어별로 구현체를 만들도록 하고, ssa와 같은 기능은 그 내부에 포함되는 구조가 되도록 해줘.

### D4. History knowledge: commits and PRs as node attributes, as nodes and edges, or both; split with the vector store (decided)
- 질문 7: user instruction 2026-11-15
  - 답: 형상관리 툴(git, svn) 기반의 프로젝트는 타임 축 기준으로 스냅샷된 코드들이라고 볼수있다. ... 코드를 변경 하도록 요청한 커밋은 상태 변경 요청(transaction)으로 볼 수 있으며, 그것에는 어떤 의도가 있는지를 커밋 메시지나 pr의 내용에 존재할 수 있고, 없다면, 변경 요청 코드로 부터 의도를 유추할 수 있다. ... 관련 코드 노드에 담을 것인가, 아니면 git 의 코어 구조 처럼 상태 변화 요청도 노드 엣지로 구성해서 deck 의 형태로 그래프에 같이 연결할 것인가? 이것은 무엇이 정답이 될지는 모른다. ... 현재는 이 정보를 vector db 에 넣고 있다. ... 효율의 측면에서 봤을때, 나의 예상보다는 효과가 적어서, 그래프 db 에도 추가하여 테스트 해보는것을 검토해볼 필요가 있다.
- 질문 31 [defined]: D4: history knowledge. A change requests as nodes and edges (git DAG) / B summary attributes on declaration nodes / C A as the source plus B computed from it
  - 답: 추천 C로.

### D5. Order of work among the verifier fix, D11, D1-D4 and the vector rebuild (decided)
- 질문 4: user instruction 2026-11-15
  - 답: 벡터로 넘어가기 전에 그래프 개선은 코드를 그래프로 구성함에 있어서, 잘못 구성하는 버그가 없어야 한다. 현재는 golang 에 집중하는데, golang 으로 구현된 코드와 다른 언어로 구성된 코드가 같이 섞여 있는 프로젝트도 커버가 가능한 수준을 만들어야 한다.
- 질문 8: user instruction 2026-11-15 (verifier)
  - 답: 검증기가 엣지가 빠진 경우는 엣지가 누락되었다. 엣지가 빠졌다 등으로 표현되게 해주고, 없는것이 맞는것을 오탐하지 않도록해.
- 질문 32 [defined]: D5: order of work. A foundation first, Go end to end, then go-stablenet's other languages, then vectors and pack measurement, then remaining languages one by one / B fix D11 and D12 in the current structure and measure first / C all languages shallow first
  - 답: 추천 A로 진행한다. 단, 여러 언어가 확장으로 존재하려면, 이들을 묶는 상위레이어는 공통된 추상화 개념이어야 한다.

### D6. Build scope and layers: graph only the files the target build compiles (build tags), keep test code a separate layer from production code (decided)
- 질문 10: user instruction 2026-11-15
  - 답: 빌드 캐그로 갈린 선언은 "go-stablenet" 에도 존재해. 이것은 빌드시 포함되는 파일 리스트를 수집해서, 해당 파일리스트 중심으로 처리를 할 필요가 있어. 또한, 유닛 테스트 코드들도 상당히 많은데, 유닛 테스트 코드와 실제 사용되는 코드는 반드시 구분되어야해. 즉, 그래프로 서로 연결되어 있어도, 서로 다른 레이어가 되어야 해. 이유는 코드를 찾을때, 테스트 코드가 포함되면, 제대로된 코드 서치가 안되고, 테스트 코드는 테스트 코드가 필요할때 별도로 찾을수 있어야해.
- 질문 16: user instruction 2026-11-15 (test layer)
  - 답: 테스트 레이어 분리는 별도 검토해줘. 검색해 포함되면 테스트 코드로 밀리는것이 많아.
- 질문 18 [defined]: D6: test code layer. A layer on nodes and cross-layer edges, default search uses production only, tests in a separate related_tests slot / B stronger demotion / C exclude_tests on by default
  - 답: D6 은 추천 A로 하자.

### D7. Build scope: which target build (main packages, GOOS/GOARCH, tags) decides the files in the graph (decided)
- 질문 20 [defined]: D7: build scope. A one target plus listed main packages, files go list puts in the build / B one target, whole module / C union of targets with per-node target labels
  - 답: 추천 A로 하는데, main 패키지만 여러개가 아니야. 인터페이스로 추상화되어서, 3rd party, vendor 사들의 코드가 빌드시에 바뀌어 들어가는것의 형태로 동작하는 프로젝트가 많아. 커널만해도 그렇게 동작하니깐.

### D8. Build variants and outside code: one graph with configuration labels or one graph per configuration; how much third-party code enters the graph (decided)
- 질문 22 [defined]: D8: build variants and outside code. A one graph with configuration labels, outside modules as declarations only / B one graph per configuration / C one graph, outside modules in full
  - 답: 추천 A로 진행하되, 빌드 명령어 기반으로 빌드시에 포함되는 파일리스트를 추출해서, 모든 코드를 기반으로 그래프를 생성하지 말고, 사용하는 빌드 명령어에의해 포함되는 파일리스트와 그것과 관련된 테스트 코드들(테스트 코드는 빌드 바이너리에 포함되지 않으므로)로 그래프 db 를 구성하도록 한다. 필요하다면, 빌드시 포함되는 파일들을 추출하는 툴을 별도 프로젝트 코드로 두고, 해당 기능을 사용해서 파일리스트를 추출할수 있도록 한다.

### D9. Language set: which languages the frontend contract must cover beyond the required nine (decided)
- 질문 27: user instruction 2026-11-15
  - 답: 고려할 언어는 golang, rust, python, c, c++, c#, ts, js, java 는 반드시 포함되어야해. 복잡도가 높은 프로젝트의 구현 언어는 반드시 포함해줘. 이 프로젝트의 장점은 매우 복잡하고 규모가 큰 프로젝트에서 포텐셜이 터질꺼야
- 질문 28 [defined]: D9: languages beyond the required nine. A Kotlin, Swift, Solidity, Proto and IDLs / B keep Solidity and Proto only / C A plus Scala and Objective-C
  - 답: 추천 A로 진행

### D10. Operating environment: where graphs are built, and how heavy analysis is split between server and local machines (decided)
- 질문 33 [defined]: Where are graphs built and how much time and memory may they use? A all on developer machines / B server (CI) builds, local machines only query / C server builds the full graph and heavy analysis, local machines add incremental changes
  - 답: 추천 C로
- 질문 35: Time and memory limits. A server: go-stablenet scale with all analyses within 1 h and 32 GB, kernel scale within 8 h and 64 GB; local: one edited file re-built within 30 s and 8 GB / B server limits only, local measured and reported / C no limits, measure and report
  - 답: 추천 A로 진행.
- 질문 81: values: why C (server full build plus local incremental) over B (server only)
  - 답: 둘다 인데, 더 중요한것은 현재 머신에서는 차라리 서버에서 처리하는것이 좋을정도로 머신 스펙이 낮은데, 현재 머신에서 충분히 가능한정도는 로컬에서 처리할수 있어. 둘다 지원하고, 개선 및 최적화를 통해 궁극적으로는 로컬에서 모두 처리가 가능한것이 가장 좋긴해. 그렇지만 비개발자가 바이브코딩으로 진행한 프로젝트이고, github 의 링크를 통해서 쉽게 진행한다면, 클라우드 서버가 유용하긴하거든

### D11. Success criteria: pass thresholds for graph correctness, runtime links, history and evidence packs (decided)
- 질문 34 [defined]: Success criteria. A fix numbers per language before measuring: graph correctness (misresolved 0, unsupported edges 0 except recorded verifier limits, missing declarations 0, static languages call recall >= 0.99 and type-reference recall >= 0.95, Python and JS lower bounds set after their first measurement and before they are used for a judgment), incremental equals cold (difference 0), runtime links precision >= 0.9 against test run traces, history fewer intent violations on the intent task, packs judged by the D6 rule plus pack precision / B numbers only for graph correctness / C set criteria after first measurements
  - 답: 추천 A로 진행

### D12. What existing parts stay: test demotion, vector-store history, old graph history, baseline indexes (decided)
- 질문 36 [defined]: What stays when moving to the new structure? A replace overlapping parts (test demotion removed once the test layer exists; old changed_in and node_prs replaced by the change-request DAG, then removed), keep parts with a different role (vector-store PR and commit text kept for text search, linked by change-request id), keep baseline indexes and evaluation records / B keep everything and add / C history in the graph only
  - 답: 추천 A로 진행

### D13. Dependency policy for per-language compiler tools: how they are obtained, run and pinned (decided)
- 질문 38 [accepted]: D13: dependency policy for per-language compiler tools. A pin our own versions / B prefer in-process libraries / C follow the toolchain the target project declares, install from official sources, record version and checksum per build
  - 추천 C → 선택 C
  - 답: 추천 C로 진행

### D14. Graph queries: caller and callee traversal, hop expansion, and path search with which edge cost and heuristic (decided)
- 질문 86: user definition of efficient (2026-11-15)
  - 답: 여기서 "효율적"의 의미는 코드의 버그를 찾고, 수정을 할때, 관련된 코드의 호출구조가 어떻게되고, 데이터가 어떻게 변경되는 등의 관련된 코드를 같이 검토해야할때가 있어. 따라서 root cause 를 찾으려면, 내가 찾은 심볼의 단계가 아니라, 그 심볼을 호출하는 이전의 hop 을 따라 갔을때, 그곳에 root cause 의 원인이 존재할 수도 있기 때문에, llm 에게 호출 구조의 그래프 구조를 제공하거나, llm의 요청에 따라 정보를 제공할수 있어야 하는데, 그때, 더 적은 리소스로 더 빠르게 정확한 값을 찾을수 있고, 정확한 정보 제공을 통하여, 더 확실한 수정 포인트를 찾아서 수정을 진행할수 있다면, 더 적은 토큰으로 문제를 해결하게 되는것과 같다. 따라서 "효율적" 이다.
- 추천 변경 [new_requirement]: D: ALT shortest paths then semantic re-rank → E: root-cause walk (The user defined efficient as reaching the root cause and the fix point with the fewest tokens, which needs upstream callers and data flow, compact structure first and details on request, not the shortest path by edge cost)
- 질문 88 [accepted]: D14: graph queries. A ALT only / B bidirectional Dijkstra / C semantic A* / D ALT plus semantic re-rank / E root-cause walk (callers upstream and data flow, relevance costs, ALT A* engine, compact structure first, bodies on request, judged by tokens and correctness to the fix point)
  - 추천 E → 선택 E
  - 답: 좋아 추천 E로 해

### 주제 밖 답변
- 질문 84: goal refinement 2026-11-15 (linking and graph queries)
  - 답: 확장해줘. 컴파일 단계까지는 코드간의 연결 구조가 보장되지 않는것이 많아. 또한, Graph DB 인데, 찾고자 하는 심볼이 호출하는 연결 노드들과 심볼을 호출하는 연결 구조들을 모두 찾을수 있어야 해. 즉, caller 와 callee 관계를 Graph DB 통해서 찾을수 있어야 하고, walk through 를 통해 구조를 찾아 나갈수 있어야 하며, Graph 로 구성된다는 것은 결과적으로 특정 노드 중심으로 hop 을 통해 연결된 노드와 그 다음 hop 등, hop 을 타고 확장되면서, 관련된 호출 구조에서 가장 효율적인 호출구조 path를 찾을수 있어야 하는데, 일종의 다익스트라 알고리즘이나, a* 알고리즘 처럼 최단 거리 검색 알고리즘 처럼 dfs 기반의 서치가 가능해야해. 성능은 a* 알고리즘이 훨씬 좋아.
- 질문 89: merge: goal sentence confirmation
  - 답: 맞다

