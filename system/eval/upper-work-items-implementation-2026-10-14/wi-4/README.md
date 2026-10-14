# WI-4 실행·종료 반례 중간 증거

coding-agent `18aca435c4f8c9e17378ad40f60c76f09ee7bc7a`와 ChainBench
`5f6483864a4a6a91f32ff02d532275039c72db80`의 깨끗한 커밋을 실행했다.
manifest.json과 source-bindings.json을 따른다. 전체 WI-4 구현 완료는 아직
false이며 원래 master workplan의 모드9 로그 삽입 경로가 남아 있다.

최종 source의 실제 public composition 재현60회·전후60쌍은
60/60→0/60, 수정 후 단측95% 상한0.0487과180개 trial cleanup을 확인했다.
실제 소유 RPC 프로그램의 Git bundle·before/after binary archive·build metadata,
불변 consumer phase 계획·실행·검증 receipt를 보존한다. 프로토콜·동시성
fixture이며 blockchain consensus/독립 원인 수용이 아니다. 별도의 실제
4-validator go-stablenet·읽기 전용 RPC 관찰은 앞선 public checkpoint를 따른다.

실제 restart, 지원하지 않는 peer injection, DSL timeout, 부분 분모,
SIGTERM 취소, consumer 강제 종료, ownership mismatch 정리 거부의7개
반례를 실행했다. 정리 거부는 cleanup_ok=false와 cleanup_unresolved로 기록해
후속 trial을 중단한다. 시험이 수정한 자기 기록만 복원한 뒤 ownership-verified
stop과 actual observation으로 복구했다. 복구를 native 성공으로 덮어쓰지 않는다.
Peer-drop receipt는 지속적인 물리적 네트워크 격리를 증명하지 않는다.

부모 종료 시 표준 pipe reader가 사라져 SIGPIPE가 cancellation/cleanup을
중단하는 실제 실패를 발견했다. 진단 전에 취소하고 parent-watch 모드에서
SIGPIPE를 처리하며 parent PID를 동기적으로 캡처한다. 실제 subprocess RED,
수정 후 race GREEN, 실제 consumer 강제 종료 후 raw native report 저장과
시험 소유 node 정리를 확인했다. consumer는 parent watch를 켜고 receipt에
이 정책을 요구한다. 직접 취소는 native cleanup을 기다리며 controller를
Python timeout으로 강제 종료하지 않는다.

전체 ChainBench build/test/race/fmt/vet/고정 lint와 plugin116개·bench 회귀·
20개 timing 시험 및 tool lint를 확인했다. 첫 전체 race 시 live process table의
변화로 vacancy가 unknown이 된 역사 결과도 보존한다. 제품 ownership 판정을
완화하지 않고 같은 source의 전체 재검사를 통과했다.

소유 임시 Claude host에서 정확한 도구
mcp__plugin_coding-agent_chainbench__chainbench_experiment를1회 호출했다.
의도한 잘못된 입력은 실제 unknown-field 오류로 거부됐다. 실제 전체 installed
role flow나 사람의 원인/수정 수용을 그 namespace 확인으로 주장하지 않는다.

원래 master workplan §2.2는 미재현 시 observation-only 코드 로그 삽입과
재발 대기를 요구한다. 외부 read-only RPC sampling만으로 이를 대신하지
않는다. source patch·삽입 위치·log event/원자료·behavior/race 대조·실제
로그 기반 재발/재개와 최종 원래 범위 audit가 남아 있다.

복구 bundle prerequisite은 기존 b9290af coding-agent 및 eebc18ff ChainBench
checkpoint다. 고정 commit/patch/log/receipt bytes를 정규화하거나 변경하지 않는다.
