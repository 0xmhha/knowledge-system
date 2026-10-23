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

아직 받지 않았다. 사용자 메시지를 받으면 그대로 여기에 옮긴다. AI가 대신 확인하지 않는다.
