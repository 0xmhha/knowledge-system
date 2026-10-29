# grpc-go 평가 표본 확인 (seed 20261024, random.Random, PR 번호 정렬 목록에서 비복원 추출)

## A. 정답 표본 10건: 정답 함수가 PR 의도와 맞는가

### A1. PR #8769 [bug_fix] transport: http2 server must validates header list size when early aborting stream
https://github.com/grpc/grpc-go/pull/8769
- internal/transport/controlbuf.go:152-158 `earlyAbortStream`
- internal/transport/controlbuf.go:845-878 `earlyAbortStreamHandler`
- internal/transport/http2_server.go:367-661 `operateHeaders`
- internal/transport/http2_server.go:972-987 `checkForHeaderListSize`
- internal/transport/http2_server.go:1028-1061 `writeHeaderLocked`
- internal/transport/http2_server.go:1063-1134 `writeStatus`
- test/servertester.go:91-117 `greet`

### A2. PR #8802 [refactor] xds: Revert workaround to get SubConn locality
https://github.com/grpc/grpc-go/pull/8802
- balancer/balancer.go:76-80 `init`
- balancer/subconn.go:107-117 `SubConnState`
- balancer/subconn.go:119-123 `connectedAddress`
- balancer/subconn.go:125-128 `setConnectedAddress`
- balancer_wrapper.go:39-39 `setConnectedAddress`
- balancer_wrapper.go:306-343 `updateState`
- clientconn.go:1276-1301 `updateConnectivityState`
- internal/internal.go:214-216 `ConnectedAddress`
- internal/internal.go:218-219 `SetConnectedAddress`
- internal/xds/balancer/clusterimpl/clusterimpl.go:61-61 `connectedAddress`
- internal/xds/balancer/clusterimpl/clusterimpl.go:407-425 `scWrapper`
- internal/xds/balancer/clusterimpl/clusterimpl.go:427-429 `updateLocalityID`
- internal/xds/balancer/clusterimpl/clusterimpl.go:431-437 `localityID`
- internal/xds/balancer/clusterimpl/clusterimpl.go:439-477 `NewSubConn`
- internal/xds/balancer/clusterimpl/clusterimpl.go:483-498 `UpdateAddresses`
- internal/xds/balancer/clusterimpl/picker.go:106-205 `Pick`

### A3. PR #8829 [refactor] xds/googlec2p: remove legacy `SetFallbackBootstrapConfig` and fix tests.
https://github.com/grpc/grpc-go/pull/8829
- internal/xds/bootstrap/bootstrap.go:650-685 `GetConfiguration`
- internal/xds/xdsclient/pool.go:44-56 `Pool`
- internal/xds/xdsclient/pool.go:83-97 `NewPool`
- internal/xds/xdsclient/pool.go:99-108 `NewClientWithConfig`
- internal/xds/xdsclient/pool.go:110-119 `NewClient`
- internal/xds/xdsclient/pool.go:121-150 `NewClientForTesting`
- internal/xds/xdsclient/pool.go:175-183 `SetFallbackBootstrapConfig`
- internal/xds/xdsclient/pool.go:207-219 `BootstrapConfigForTesting`
- internal/xds/xdsclient/pool.go:221-230 `UnsetBootstrapConfigForTesting`
- internal/xds/xdsclient/pool.go:268-311 `newRefCounted`

### A4. PR #8831 [bug_fix] credentials/tls: verify overwritten authority against only leaf certificate
https://github.com/grpc/grpc-go/pull/8831
- credentials/tls.go:54-71 `ValidateAuthority`

### A5. PR #8869 [bug_fix] xds/googlec2p: Revert PR 8829 that removed `SetFallbackBootstrapConfig`
https://github.com/grpc/grpc-go/pull/8869
- internal/xds/bootstrap/bootstrap.go:650-685 `GetConfiguration`
- internal/xds/xdsclient/pool.go:43-53 `Pool`
- internal/xds/xdsclient/pool.go:80-96 `NewPool`
- internal/xds/xdsclient/pool.go:98-107 `getBootstrapConfiguration`
- internal/xds/xdsclient/pool.go:109-116 `getConfig`
- internal/xds/xdsclient/pool.go:118-134 `NewClientWithConfig`
- internal/xds/xdsclient/pool.go:136-149 `NewClient`
- internal/xds/xdsclient/pool.go:151-189 `NewClientForTesting`
- internal/xds/xdsclient/pool.go:236-245 `BootstrapConfigForTesting`
- internal/xds/xdsclient/pool.go:247-255 `UnsetBootstrapConfigForTesting`
- internal/xds/xdsclient/pool.go:293-319 `newRefCounted`

### A6. PR #8965 [feature_add] xds: Add SNI related field in handshake info
https://github.com/grpc/grpc-go/pull/8965
- internal/credentials/xds/handshake_info.go:47-67 `Equal`
- internal/credentials/xds/handshake_info.go:83-93 `HandshakeInfo`
- internal/credentials/xds/handshake_info.go:95-104 `NewHandshakeInfo`
- internal/credentials/xds/handshake_info.go:121-158 `ClientSideTLSConfig`
- internal/credentials/xds/handshake_info.go:160-212 `buildVerifyFunc`
- internal/xds/balancer/clusterimpl/clusterimpl.go:78-101 `Build`
- internal/xds/balancer/clusterimpl/clusterimpl.go:306-364 `handleSecurityConfig`
- internal/xds/server/conn_wrapper.go:91-123 `XDSHandshakeInfo`

### A7. PR #8990 [bug_fix] xds/rbac: add additional handling for addresses with ports
https://github.com/grpc/grpc-go/pull/8990
- internal/xds/rbac/matchers.go:346-349 `match`
- internal/xds/rbac/matchers.go:364-367 `match`

### A8. PR #9031 [bug_fix] transport: Fix handling of closed connections in ReadyReader
https://github.com/grpc/grpc-go/pull/9031
- internal/transport/ready_reader.go:50-77 `ReadOnReady`

### A9. PR #9100 [bug_fix] xds/resolver: support HTTP filters for routes using cluster specifier plugins
https://github.com/grpc/grpc-go/pull/9100
- internal/xds/resolver/xds_resolver.go:382-488 `newConfigSelector`

### A10. PR #9299 [security] credentials/sts: do not follow redirects during token exchange
https://github.com/grpc/grpc-go/pull/9299
- credentials/sts/sts.go:196-205 `makeHTTPClient`

## B. 질문 표본 10건: 정답을 흘리지 않는가, 개발자가 실제로 물을 만한 말투인가

### B1. PR #8792 [refactor] xdsdepmgr: add functionality for cluster subscription
> xDS 의존성 관리에서 클러스터를 구독하면 참조 수를 올리고 해지하면 내리는 기능을 만들고 싶어. 동적 클러스터 감시에 쓸 거야

### B2. PR #8807 [feature_add] xds/internal/client: Add async gauge metrics for Connected and Resources (A78)
> xDS 클라이언트가 리소스 수와 ADS 스트림 연결 여부를 비동기 게이지 지표로 보고하게 하고 싶어

### B3. PR #8877 [feature_add] experimental/stats: Expose Telemetry Label Callback
> OpenTelemetry를 안 쓰는 클라이언트도 통계 핸들러에 레이블을 넘길 수 있게, 텔레메트리 레이블 콜백을 등록하는 실험 API를 만들고 싶어

### B4. PR #8878 [refactor] xds: refactor server-side HTTP filter processing
> 서버 쪽 xDS HTTP 필터 처리 구조를 필터 상태를 유지하기 쉽게 다시 짜고 싶어

### B5. PR #8956 [bug_fix] xds: leaf clusters provide the handshake info instead of top level cluster
> 집계 클러스터에서 핸드셰이크가 최상위 클러스터의 보안 설정을 써. 실제 말단 클러스터의 보안 설정을 쓰게 고치고 싶어

### B6. PR #8959 [test_add] cds: increase defaultTestTimeout to 10s for e2e tests
> CDS e2e 시험이 RPC를 많이 보내다가 기본 시험 시간 제한에 걸려 실패해. 제한을 10초로 늘리고 싶어

### B7. PR #8978 [test_add] binarylog: replace testServer with StubServer in binarylog_end2end_test.go
> 바이너리 로그 e2e 시험에서 직접 만든 시험 서버 대신 공용 스텁 서버를 쓰게 바꾸고 싶어

### B8. PR #9004 [test_add] test: Fix flaky test `TestServerStreaming_ClientCallSendMsgTwice` in `end2end_test.go`.
> 서버 스트리밍에서 클라이언트가 메시지를 두 번 보내는 경우를 보는 시험이 가끔 실패해. 고치고 싶어

### B9. PR #9031 [bug_fix] transport: Fix handling of closed connections in ReadyReader
> 비차단 읽기를 쓰는 준비 완료 읽기 경로에서 상대가 연결을 닫았을 때 처리가 틀려. 닫힌 연결을 제대로 다루게 고치고 싶어

### B10. PR #9114 [security] grpc: limit decompression size in legacy gzip Decompressor to prevent OOM
> 예전 방식 gzip 압축 해제기가 크기 제한 없이 끝까지 읽어서 메모리가 터질 수 있어. 해제 크기를 제한하고 싶어

## 확인 결과

사람의 확인은 아직 받지 않았다. 사용자 메시지를 받으면 그대로 여기에 옮긴다.

### AI 검토 (2026-10-27, 사용자 위임, 사람 확인 아님)

사용자가 확인을 직접 하지 않고 AI에게 맡겼다. 사용자 메시지 원문: "너가 직접 판단해줘. 중요한것은 문제를 해결할수있는 실마리 코드를 찾았는가여부야. 만약 찾지 못했다면, 찾지못한 이유를 디버깅할수있도록 준비하고, 정리후에 디버깅을 진행해."

아래는 AI(Claude)가 각 PR의 설명과 바뀐 파일 목록(`gh pr view -R grpc/grpc-go`)을 읽고 내린 판단이다. 사람 확인이나 독립 검토로 세지 않는다. 1차 결과와 진단 결과를 본 뒤에 판단했다. 표본과 정답은 바꾸지 않는다.

정답 표본(A):

- A1, A2, A4, A6, A7, A8, A9, A10: 정답 함수가 PR 설명이 말하는 변경과 맞는다.
- A1의 `test/servertester.go` `greet`는 시험 보조 함수다. 파일 이름이 `_test.go`가 아니어서 시험이 아닌 정답으로 센다. 고칠 곳이 아닌 부수 정답이다.
- A2는 되돌린 우회 코드 전체(16개 함수)가 정답이다. 하나만 맞혀도 적중으로 세므로, 문제의 중심(`clusterimpl.go`)이 아닌 곳을 맞혀도 적중이 된다.
- A3과 A5는 서로를 되돌리는 한 쌍이다. 정답 함수는 각 PR의 변경과 맞는다. A5의 의도 라벨 `bug_fix`는 되돌림(내부 사용처 때문에 API를 되살림)이라 `refactor`로도 읽힌다.

질문 표본(B):

- 열 개 모두 파일, 함수, 식별자 이름을 그대로 쓰지 않는다. 자동 누설 검사 결과와 같다.
- B7("공용 스텁 서버" = `StubServer`), B8(시험 이름 `TestServerStreaming_ClientCallSendMsgTwice`를 풀어 씀), B9("준비 완료 읽기" = `ReadyReader`)는 식별자를 한국어로 옮긴 표현이 들어 있다. 개발자가 실제로 이렇게 묻기도 하므로 허용하되, 결과를 읽을 때 약한 누설로 표시한다.
- B6은 해결 값("10초")이 질문에 있다. 실제 요청에서도 흔하므로 허용한다.
- 말투는 모두 개발자가 할 법한 요청형이다. B4("필터 상태를 유지하기 쉽게 다시 짜고 싶어")는 무엇을 바꾸는지가 모호해 정답을 찾기 어려운 질문이다.

판정: 정답과 질문 표본에서 평가를 무효로 할 결함은 찾지 못했다. 위의 부수 정답(A1, A2)과 약한 누설(B6~B9)은 결과 해석에서 따로 표시한다.
