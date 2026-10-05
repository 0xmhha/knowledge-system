# C0 개발 실패: v2의 본문 예산과 그래프 인용

2026-10-06 · FIX-25 수정·개발 재검증 완료. 전체 C0/공식 품질 게이트 완료와 구분한다.

승인된 정적 DEV의 WHY 질문은 v1에서 인용22/본문10개를 반환한다. composer는 본문을 예산으로 선택한 뒤 v1의 그래프 연결용 edge-only 인용을 별도로 붙인다. v2 handler가 전체22개를 본문으로 확장해 최대12개 계약에 걸렸고, 모든8arms의 warmup 각2회가 v2_evidence_failed였다. 오류를 source/integrity 실패로 숨기지 않고 실제 원인을 분류했다.

첫 matrix는77/960행에서 종료하고 원자료·17실패(기존 WHY16, 종료 중 취소1)·미측정883행·사전 환경 valid와 종료 취소 후 환경 확인 불가를 보존한다. 미완료 지연 결과를 공식 비교로 사용하지 않는다. v1 실행 exit0도 CODE/POLICY/TRACE의 miss3개를 품질 합격으로 바꾸지 않는다.

요구사항은 v2가 composer의 선택 본문과 그 순위를 유지하고, 각 본문을 retained archive에서 다시 읽고 sanitize/좌표/원문 SHA/무결성을 검증하는 것이다. v2에서 아직 검증·공개하지 않는 graph overlay의 edge-only 인용을 전체 본문으로 확장하지 않는다. v1의 그래프/인용·검색 K·정답·threshold·v2의12개/32,000byte 상한은 유지한다.

수정 전 실제 composer→handler 시험은 v2_evidence_failed로 실패했다. 최초 시험 코드의 Hit 필드명 착오는 별도 컴파일 로그로 보존한 뒤 올바른 실행 실패를 얻었다. 수정 후 knowledge off/on의 선택 순위·본문 개수·graph 비공개·Go v2 Verify를 확인하고 handler/evidence/composer/공개 contract4패키지 시험, 관련3 race와4 vet를 통과했다.

보존된 실제 DEV20개 v1 응답을 같은 archive에 Go로 재생했다. 기존 WHY5회는 상한 오류, 수정된 본문 선택20개는 source/좌표/무결성 검증 통과다. 새 실제 SDK DEV4응답은 CODE8/WHY10/POLICY8/TRACE11개 인용과 동일 개수 본문·오류0이다. 별도 Python reader는 모든 인용의 실제 파일/줄 SHA와 v2 integrity를 확인했다. 이는 수정 smoke이며 전체 승인 반복 평가와 구분한다.

다음은 동적 source/chunk/vector·F-01 exact/budget·F-02 parent·F-04 state 오라클 감사 후 새 runtime으로 전체8arm/승인 횟수를 실행하는 것이다. 세 검색 miss의 후보 순위·root cause, 전체 B0/B1/사람 판정, 수정 동결·독립 FINAL/운영 조건은 남아 있다.
