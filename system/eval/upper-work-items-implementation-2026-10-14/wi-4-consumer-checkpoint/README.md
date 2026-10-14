# WI-4 소비자 통합 중간 증거

coding-agent source `e834ccc4687e1c30bd7d879d5f0e6f899c53369a`와
ChainBench `348a6ec5fb9aec5b5966ce768a622be4e5296db4`를 연결했다.
source-bindings.json과 checkpoint.json을 따른다. WI-4는 in_progress다.

불변 phase input·plan·실행 명령/exit·원자료·verifier·결과 SHA를 ticket과
bounded evaluation cycle1..3에 결속한다. 통계·node ownership·cleanup은
ChainBench가 담당한다. 정상/재현/방어/관찰의 typed 분기를 유지하며 실제
Write/Edit hook과 oracle completion CLI에서 검증한다. primary/independent
oracle와 candidate selection 요구를 제거하지 않았다. 기존 비타이밍 legacy
ChainBench 레시피의 전체 호환성을 이번 새 경로의 검증으로 주장하지 않는다.

실제4-validator go-stablenet 소표본 시험의 원자료 검증은 PASS지만 결과는
INCONCLUSIVE다. 실제 hook의 PLANNING 거부와 state unchanged를 보존했다.
controlled Go RPC 프로그램을 실제 public composition으로 반복 실행한
재현60회·비교60쌍은 60/60→0/60과180개 trial cleanup PASS다. fixture는
Stablenet launch/config 형식으로 실행하는 프로토콜·동시성 시험이며 blockchain
consensus 검증이 아니다. fixture Git bundle과 실제 before/after binary archive,
source metadata를 보존했다. oracle-coordinates-not-executed.json은 실제 source
좌표만 담고 NOT_EXECUTED이며 전체 oracle PASS가 아니다.

전체 plugin114개 시험, timing18개 반례/취소 검사, bench 회귀와 tool-name lint
PASS. 정상 대조의 부분 분모, 미재현·관찰·오류·raw 변경·명령 변경·출처
불일치·증거 삭제·분류 제거·generation 초과를 성공으로 승격하지 않는다.
실행 중 한 번의 취소는 native controller에 전달하고 그 cleanup을 기다린다.

남은 범위는 실제 native fault/timeout/부분 완료/주입 오류/cleanup 반례,
제어 프로세스의 parent 종료 감시·실제 signal cleanup 검증, 최종 커밋 기반
통합 replay·실제 host alias 근거와 독립 production 수용이다. controlled
실행을 사람의 원인/수정 수용이나 전체 pipeline 완료로 기록하지 않는다.

복구 bundle은 coding-agent b9290af checkpoint를 prerequisite로 사용한다.
고정 patch·commit·로그·원자료는 bytes를 바꾸지 않는다. runner/input/source
변경 시 이전 실행을 현재 코드의 PASS로 사용하지 않고 새 bounded 근거를 만든다.
