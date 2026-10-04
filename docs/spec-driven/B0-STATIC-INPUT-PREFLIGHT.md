# B0 프로토콜 입력 경계와 정적 입력 준비

2026-10-04 · B0-06 준비 도구 검증 · **프로토콜·동적 사례 승인 대기, 공식 실행 준비 미완료**.

`scripts/b0-prepare-static-inputs.py`는 질문·프로토콜·동적 소스·기존 사람 결정 원문을 읽고 서로의 해시와 개발/최종 경계를 검사한다. 모델 서버나 MCP를 호출하지 않고 승인 상태를 바꾸지 않는다. 기존 `b0-preflight.py`의 `ready`는 정적 정답·모델 준비 검사이며 전체 B0 준비 판정이 아니다.

## 검사 범위

- 정적 질문 원문과 12문항 승인 기록의 해시·ID, 실제 Git commit/tree·앵커를 대조한다. 프로토콜이 참조하는 질문·동적 소스 원문 해시도 일치해야 한다.
- 개발 4개/최종 8개가 중복 없이 모든 질문을 포함하는지, 동적 F-01–06의 개발/최종 12개와 소스 SHA·가족·분할 선언이 맞는지 검사한다.
- BGE-M3의 선언된 모델·digest·차원이 실제 사용자 선택 기록과 맞는지 검사한다. 이 단계는 실행 중 모델 바이트나 차원을 새로 관측하지 않는다.
- 네 모드·팩 off/on, K10/F-01 K5, 기존 회귀 임계치, 반복 수의 정수/범위, warm/cold·불확실성 선언을 검사한다. JSON 중복 키·잘못된 해시·임계치 완화·최종/개발 혼입을 거부한다.
- 현재 최종 질문은 군별 독립 질문 2개씩이다. 반복 5회·warm 20회는 독립 질문 수를 늘리지 않는다. 군별 출시 추론은 inconclusive다.

현재 결과는 `pending_reasons=[protocol_review_pending, dynamic_fixture_review_pending]`, 정적 승인 12/12·동적 승인 0/12다. 프로토콜·동적 문서·모든 동적 사례의 선언된 승인 메타데이터가 맞으면 `approved_input_definition`이 되지만, 이 상태도 입력 정의만의 판정이다. `official_execution_ready=false`, `v2_matrix_ready=false`, `metrics=null`을 유지한다. 실제 소스 범위·의미 사실 검토·새 strict 빌드·팩/semantic 신원·v2 날짜/subsystem·실제 K/필터·환경/모델/바이너리 잠금은 별도 실행 선행 조건이다.

## 재현 명령

읽기 전용 검사 결과를 새 파일에 기록한다. 이미 존재하는 출력 파일은 덮어쓰지 않는다. 승인 대기 중 `--require-ready`는 보고서를 남기고 종료 코드 2를 반환하며 입력 디렉터리를 발행하지 않는다.

```sh
python3 scripts/b0-prepare-static-inputs.py \
  --output /private/tmp/b0-input-definition-new.json --require-ready
```

승인된 정적 개발 4개는 진단용 v1 시나리오로 준비할 수 있다. 기존 디렉터리를 거부하며 `manifest.json`을 발행 표시로 사용한다. 원문 질문집·프로토콜·승인 기록을 바꾸지 않는다. 출력 디렉터리는 0700, 파일은 0600이다.

```sh
python3 scripts/b0-prepare-static-inputs.py \
  --partition development --out-dir /private/tmp/b0-development-inputs-new
```

manifest에는 `diagnostic_only=true`, 선택/보류 ID, 입력 해시, 제안된 반복 수·K와 개별 시나리오 SHA를 기록한다. 실제 개발 파일에는 최종 질문 프롬프트와 후보 답이 없다. 이 파일 생성은 초안 프로토콜로 공식 점수화를 승인하지 않는다. v2 matrix 요청을 생성하지 않으므로 지식 날짜/subsystem이나 실제 K 설정을 임의로 추정하여 8-arm 입력으로 사용하지 않는다.

```sh
python3 scripts/b0-prepare-static-inputs.py \
  --partition final --out-dir /private/tmp/b0-final-inputs-new
```

현재 이 명령은 승인 대기 때문에 종료 코드 2이며 최종 출력 디렉터리가 없다. `expect_no_citations`는 기존 v1의 인용 0개 오라클이고, 사람이 답변을 기권했다는 판정과 다르다. 임의의 상태 플래그나 시험용 검토자를 실제 사용자 승인으로 기록하지 않는다.

## 재감사와 원자료

기존 질문 검증기는 경로 문자와 대소문자만 다른 ID를 허용했다. exporter가 ID를 파일명으로 사용하고 소문자로 바꾸므로 경로 이탈/파일 충돌 가능성이 있었다. 원래 HEAD의 검증기로 다섯 실패를 재현하고, 안전한 ASCII ID와 대소문자 구분 없는 유일성을 요구하도록 수정했다. 기존 정상 12문항의 13개 export 파일은 수정 전후 바이트가 같다. 기존 exporter는 호환을 위해 전체 12개를 내보내는 동작을 유지하므로 개발/최종 분리가 필요한 공식 입력에는 새 준비기를 사용한다.

[입력 준비 원장](../../system/eval/b0-knowledge-system/static-input-preflight-m2max-2026-10-04/summary.json)에 실제 CLI 종료 코드·개발 4개 파일·최종/ready 발행 거부·입력 무변경·정상 이전 export 동등성·26개 Python 회귀와 독립 코퍼스 재감사를 연결한다. 첫 회귀 실행의 로컬 HTTP bind 제한 두 건은 샌드박스 오류이며 제품 ID 검증 실패 다섯 건과 분리한다. 실제 모델/MCP·최종 질문 평가를 실행하지 않았고 품질은 null이다.

우선순위와 전체 완료 조건은 [작업리스트](./EXECUTION-WORKLIST.md)를 따른다. 필요한 사람 결정 이후 입력 정의를 새 해시로 동결하고, B0-07의 실제 데이터셋·런타임 설정 검증 뒤 공식 평가를 진행한다.


정적 v2의 미지정 날짜/subsystem·K 결합은 [STV2-01 검토 제안](./B0-STATIC-V2-SCOPE-REVIEW.md)과 별도 checker로 구체화했다. 현재는 draft·승인 대기이며, 실제 runtime YAML/계측 K와 source-bound pack/semantic 신원 및 새 strict 데이터셋에 연결하기 전에는 v2_matrix_ready가 아니다.
