# B0 실모델 평가 준비와 승인 경계

상태: **준비 중 / 품질 수치 없음** (2026-10-01). A0–A8의 구조 게이트는 통과했다. B0의 질문 정답은 아직 사람이 승인하지 않았고 이 호스트의 로컬 Ollama API는 응답하지 않았다. 따라서 B0/B1 품질 통과나 C 단계 시작을 선언하지 않는다.

## 고정 입력

| 항목 | 현재 값 | 고정 이유 |
|---|---|---|
| 파일럿 | `knowledge-system` | 사용자 지정 대상 프로젝트 |
| 코퍼스 커밋 | `71cb71cd55960833e930269e272f7a4a060be3aa` | A 완료 상태의 코드·설계 문서 바이트를 보존; 이후 B0 준비 파일은 코퍼스에 섞지 않음 |
| 질문셋 | [`questions.json`](../../system/eval/b0-knowledge-system/questions.json) | 코드 위치 3, 설계 이유 4, 정책 충돌/권한 3, 답 없음 2. 총 12개 모두 `draft` |
| 호스트 | macOS arm64, Apple M2, 8코어, 8 GiB | 동일 호스트 재실행 비교를 위한 최소 하드웨어 기록 |
| 임베딩 모델 | **미확정** | 로컬 Ollama의 정확한 태그·SHA-256 다이제스트·벡터 차원을 실행 시 기록해야 함 |
| 데이터셋 | **미생성** | 모델과 질문 정답이 고정된 뒤 코퍼스 커밋을 별도 깨끗한 checkout으로 색인 |

질문 파일의 `candidate_answer`와 `evidence`는 검토 제안이다. `review_state=draft`를 정답으로 점수화하거나 `verified` 정책으로 승격하지 않는다. 정책 질문은 프로젝트의 **설계 계약과 구현 코드**에 관한 것이며 특정 조직의 실제 업무 정책이 승인됐다는 뜻이 아니다. 답 없음 두 문항은 출처의 부재를 사람이 다시 확인해야 한다. 질문군과 한영 비율, 파일/줄 근거를 보고 편향이나 정답 누출이 없는지 검토한다.

이 12개는 첫 검토 묶음이다. `END-TO-END-DESIGN.md` 9절이 요구하는 희소 필터, 긴 문서 끝, 다의어·미등록 표현, 오래된 인용, 두 프로젝트 격리의 **동적 실패 fixture**는 아직 별도로 채워야 한다. 현재 질문만으로 B0 질문셋이 완성됐다고 보지 않는다.

## B0 선행 게이트

1. 사용자 또는 지정 검토자가 각 질문의 문장, 후보 답, 예상 동작(`cite|abstain`), 근거의 **파일·줄 범위**를 검토한다. 승인 시 질문별 `review_state=approved`, `reviewer`, `reviewed_at`을 기록한다. 변경된 질문 파일의 SHA-256은 평가 실행마다 다시 기록한다. 로컬 리뷰 문자열은 인증된 신원이라는 주장이 아니다.
2. 로컬 Ollama에서 선택한 **정확한 모델 태그**와 다이제스트를 `/api/tags`로 읽는다. 한 번의 `/api/embed`로 차원·유한값을 검사하고 전후 다이제스트가 같은지 확인한다. 모델을 바꾸면 별도 실행군으로 취급한다.
3. [`b0-preflight.py`](../../scripts/b0-preflight.py)는 고정 커밋의 tree ID와 각 근거 앵커의 정확한 한 번 출현을 검사하고 모델·하드웨어·승인 수를 JSON으로 남긴다. `pending`은 0점도 통과도 아니다. `--require-ready`는 어느 선행 조건이든 없으면 종료 코드 2로 멈춘다.
4. 고정 커밋을 깨끗한 별도 checkout에서 색인한다. 모델, 코퍼스 tree, CKS/CKV/CKG 빌드 커밋, `project_id/snapshot_id/dataset_id`, 잠금 팩, query prefix, K, 필터, 소독 규칙, 실행 하드웨어와 시각을 원자료와 함께 보존한다. 현재 작업 브랜치의 HEAD를 코퍼스 커밋과 혼동하지 않는다.

```sh
python3 scripts/b0-preflight.py --model MODEL_NAME \
  --output /tmp/ks-b0-preflight.json --require-ready
```

## 측정 경계와 B1 연결

기존 `cks eval`은 **v1 문맥 도구의 인용 검색**에 대해 파일/줄 recall·MRR·정밀도·기권과 지연을 기록한다. 이것만으로 v2 정책의 적용 여부, 이유의 정확성, 모델이 작성한 답변의 진실성을 측정했다고 표시하지 않는다. B0에서 우선 동일 코퍼스·모델·질문에 대한 CKV/CKG/CKS 기본 기준선을 저장한다. B1에서는 같은 입력을 기본, 개념 텍스트, 관계, 결합, 팩 유무로 비교하고 v2 의미 필드 및 주장별 사람 검토를 별도로 채점한다. 질문별 오류, 원자료, p50/p95, 색인 시간·크기와 기능 상태를 함께 남긴다.

실제 승인 질문과 모델이 준비되면 시나리오의 인용 범위를 확정하고 `cks eval --verify-anchors`를 실행한다. 답 없음 문항은 `expect_no_citations`와 사람의 답변 기권 판정을 분리한다. 기본 paired 출시 판정 임계치는 `END-TO-END-DESIGN.md` 9절에 이미 고정되어 있다. 표본 수와 질문군별 추가 임계치는 결과를 본 뒤 유리하게 바꾸지 않도록 B1 실행 전에 승인해 버전으로 고정한다. 현재 `system/eval/scenarios`의 과거 사례는 파일럿 참고 자료이지 B0 승인 정답이 아니다.

[`b0-export-scenarios.py`](../../scripts/b0-export-scenarios.py)는 **12개 질문 전부 승인된 경우에만** `cks eval`의 v1 YAML 시나리오와 질문셋 SHA-256 매니페스트를 새 디렉터리로 내보낸다. 지금은 draft 상태이므로 의도적으로 실패한다. 이 스크립트는 후보 답변 문장을 MCP 평가 입력에 포함하지 않는다. 승인·모델·데이터셋이 준비된 뒤 실행할 명령 형태는 다음과 같다.

```sh
python3 scripts/b0-export-scenarios.py --out-dir /tmp/ks-b0-scenarios
./bin/cks eval --scenarios /tmp/ks-b0-scenarios \
  --config /path/to/pinned-cks.yaml \
  --verify-anchors /path/to/clean-pinned-corpus \
  --output /tmp/ks-b0-cks-eval.json
```

## 현재 점검 결과

`python3 scripts/b0-preflight.py --model bge-m3 --output /tmp/ks-b0-preflight.json`을 읽기 전용으로 실행했다. 고정 커밋의 12개 질문 근거 앵커는 모두 확인됐고 호스트 메모리도 기록됐다. 결과는 `pending_reasons=[gold_answers_need_human_approval, ollama_unavailable_or_probe_failed]`, `metrics=null`이다. `bge-m3`은 기존 설계의 예시 이름으로 조회했을 뿐 사용 모델로 확정하지 않았다. 실제 모델 선택과 설치/가동은 후속 입력이 필요하다.

`python3 -m unittest discover -s scripts -p test_b0_preflight.py -v`의 6개 시험은 잘못된 앵커/줄 범위, 승인 전 시나리오 내보내기, 원격 엔드포인트, 프로브 중 모델 다이제스트 변경을 거부하고, 안정 모델의 차원 기록과 합성 승인 fixture의 정답 문장 없는 내보내기를 확인했다. `make docs-check`는 104개 문서에서 통과했다. 합성 승인 fixture는 실제 질문 승인 기록이 아니다.
