# WI-4 공개 실행 경로 중간 증거

ChainBench source `348a6ec5fb9aec5b5966ce768a622be4e5296db4`.
`checkpoint.json`과 `source-bindings.json`을 따른다. 구현 검증 전체 완료와
독립 수용은 미기록이며 WI-4 상태는 in_progress다.

깨끗한 source commit에서 CLI/MCP를 다시 빌드하고 실제 공개 stdio MCP,
새로 만든4-validator go-stablenet 시험, 기존 시험 소유 네트워크의 읽기 전용
관찰 window/재개, 정상 대조와 cleanup을 실행했다. 관찰 전후 구성 기록은
같으며 최종 actual process 관찰은4개 node 모두 stopped다.
실행 파일·genesis·config는 실제 node machine checksum으로 확인한다.
보고서의 raw assertion/injection receipt·분모·판정은 공개 verify CLI로 검사했다.

별도 controlled Go concurrent node 시험은 전/후60회씩 실행했다. 세부 실패율과
상한은 timing-report.json을 따르며 node120/120 cleanup PASS다. 생성 fixture의
source는 보존된 ChainBench test에 있으며 이 packet은 그 fixture의 별도 Git
object/binary archive를 제공하지 않는다. source commit 복구는 ChainBench
구현에 대한 복구이며 독립 production 원인 증명으로 확대하지 않는다.

build·전체 test·전체 race·fmt·vet·고정 lint PASS. 최초1-validator genesis
quorum 실패와5초 readiness 실패는 커밋 전 시도이며 역사 오류 자료로만
보존했다. 증상 실패나 최종 커밋 실행의 결과로 합산하지 않는다.

로컬 복구는 이전 eebc18ff checkpoint가 필요하다. chainbench.bundle의 HEAD를
가져오거나 chainbench.patch를 그 prerequisite에 적용한다. patch/commit은
원본 bytes다. 공개 probe의 입력 경로는 실행 당시 임시 경로이며 재실행에는
새 artifact root와 해당 source의 실제 clean binary가 필요하다.

남은 범위는 coding-agent 조사/계약/완료 gate 연결, 실제 fault/timeout/
주입 오류/정리 반례 보강, 최종 통합 source replay와 독립 production 수용이다.
전체 목표의 다음 순서는 WI-4→WI-1→WI-2→WI-C→WI-6→WI-SP다.
