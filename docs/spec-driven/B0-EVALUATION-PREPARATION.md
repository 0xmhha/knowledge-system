# B0 실모델 평가 준비와 승인 경계

상태: **준비 중 / 품질 수치 없음** (2026-10-01). A0–A8의 구조 게이트는 통과했다. 로컬 Ollama 임베딩 프로브는 성공했으나 B0의 질문 정답은 아직 사람이 승인하지 않았다. 따라서 B0/B1 품질 통과나 C 단계 시작을 선언하지 않는다.

## 고정 입력

| 항목 | 현재 값 | 고정 이유 |
|---|---|---|
| 파일럿 | `knowledge-system` | 사용자 지정 대상 프로젝트 |
| 코퍼스 커밋 | `71cb71cd55960833e930269e272f7a4a060be3aa` | A 완료 상태의 코드·설계 문서 바이트를 보존; 이후 B0 준비 파일은 코퍼스에 섞지 않음 |
| 질문셋 | [`questions.json`](../../system/eval/b0-knowledge-system/questions.json) | 코드 위치 3, 설계 이유 4, 정책 충돌/권한 3, 답 없음 2. 총 12개 모두 `draft` |
| 호스트 | macOS arm64, Apple M2, 8코어, 8 GiB | 동일 호스트 재실행 비교를 위한 최소 하드웨어 기록 |
| 임베딩 모델 | `bge-m3:latest` **임시 후보** | 사용자의 최종 모델 선택 전 설치·프로브만 수행. 관측 다이제스트와 차원은 아래 기록 참조 |
| 데이터셋 | **미생성** | 모델과 질문 정답이 고정된 뒤 코퍼스 커밋을 별도 깨끗한 checkout으로 색인 |

질문 파일의 `candidate_answer`와 `evidence`는 검토 제안이다. `review_state=draft`를 정답으로 점수화하거나 `verified` 정책으로 승격하지 않는다. 정책 질문은 프로젝트의 **설계 계약과 구현 코드**에 관한 것이며 특정 조직의 실제 업무 정책이 승인됐다는 뜻이 아니다. 답 없음 두 문항은 출처의 부재를 사람이 다시 확인해야 한다. 질문군과 한영 비율, 파일/줄 근거를 보고 편향이나 정답 누출이 없는지 검토한다.

이 12개는 첫 검토 묶음이다. `END-TO-END-DESIGN.md` 9절이 요구하는 희소 필터, 긴 문서 끝, 다의어·미등록 표현, 오래된 인용, 두 프로젝트 격리의 **동적 실패 fixture**는 [`B0-DYNAMIC-FIXTURES.md`](./B0-DYNAMIC-FIXTURES.md)에 재현 절차와 기존 구조 시험을 정리했다. 실제 모델 질의와 사람의 예상 결과 검토는 아직 남아 있다. 현재 질문만으로 B0 질문셋이 완성됐다고 보지 않는다.

## B0 선행 게이트

1. 사용자 또는 지정 검토자가 각 질문의 문장, 후보 답, 예상 동작(`cite|abstain`), 근거의 **파일·줄 범위**를 검토한다. 승인 시 질문별 `review_state=approved`, `reviewer`, `reviewed_at`을 기록한다. 변경된 질문 파일의 SHA-256은 평가 실행마다 다시 기록한다. 로컬 리뷰 문자열은 인증된 신원이라는 주장이 아니다.
2. 로컬 Ollama에서 선택한 **정확한 모델 태그**와 다이제스트를 `/api/tags`로 읽는다. `/api/version`으로 서버 버전을 기록한다. 한 번의 `/api/embed`로 차원·유한값을 검사하고 전후 다이제스트가 같은지 확인한다. 모델을 바꾸면 별도 실행군으로 취급한다.
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

이 호스트에서 Ollama 0.34.4와 `bge-m3:latest`를 설치하고 임시 프로세스로 서버를 가동했다. [`preflight-2026-10-01.json`](../../system/eval/b0-knowledge-system/preflight-2026-10-01.json)에 그 시점의 읽기 전용 사전 점검 결과를 보존했다. 고정 코퍼스 tree `f020f8f30fd209b8de045756dedd12ff83f65cd9`의 12개 질문 근거 앵커가 유효했고 임베딩은 1024차원이었다. 모델 다이제스트는 `7907646426070047a77226ac3e684fbbe8410524f7b4a74d02837e43f2146bab`이다. 결과는 `pending_reasons=[gold_answers_need_human_approval]`, `metrics=null`이다. 이 관측은 모델 확정이나 품질 평가가 아니며 서버가 종료되면 실행 전에 다시 점검해야 한다. 디스크 잔여 공간은 설치 전 약 9.7 GiB였으므로 고정 코퍼스 색인 전 저장 공간을 다시 확인한다.

`python3 -m unittest discover -s scripts -p test_b0_preflight.py -v`의 6개 시험은 잘못된 앵커/줄 범위, 승인 전 시나리오 내보내기, 원격 엔드포인트, 프로브 중 모델 다이제스트 변경을 거부하고, 안정 모델의 차원·서버 버전 기록과 합성 승인 fixture의 정답 문장 없는 내보내기를 확인했다. 합성 승인 fixture는 실제 질문 승인 기록이 아니다.

[`b0-ollama-smoke.sh`](../../scripts/b0-ollama-smoke.sh)를 작은 **별도 임시 Git 프로젝트**에 실행했다. 같은 로컬 모델로 CKG 1개 Go 파일, CKV 2개 파일/4개 청크를 색인하고, CKV 질의 4개 히트와 CKS MCP의 `README.md`/`main.go` 인용을 확인했다. 모든 인용 커밋은 임시 프로젝트의 커밋과 같고, CKV 매니페스트의 모델 태그·다이제스트·차원도 실행 직전 사전 점검과 일치했다. 이 결과는 실 API 통합 스모크이며 파일럿 `knowledge-system`의 검색 품질, 질문 정답, 지연 기준선이 아니다. 원시 로그와 데이터셋은 `/tmp/ks-b0-ollama-smoke-20261001c`에 있다.

고정 코퍼스의 별도 깨끗한 체크아웃 `/tmp/ks-b0-pinned-corpus`에서도 **임시 전체 색인**을 시도했다. CKG는 102,155 노드/414,667 엣지를 생성했다. CKV는 1,569개 대상 파일 중 40개를 처리한 시점에 자원 점검을 위해 중단했다. 처음 42초의 파일별 처리율은 입력 크기에 따라 크게 변했으므로 이 구간의 ETA를 전체 색인 시간으로 단정하지 않는다. 중단 당시 호스트 잔여 디스크가 약 4.5 GiB였고, 실패 후보 데이터셋은 제거해 약 6.7 GiB를 회복했다. 원시 로그는 `/tmp/ks-b0-pilot-setup.log`에 보존했다. 완성된 파일럿 벡터 DB나 품질 기준선은 아직 없다.

이 시도에서 `cmd/cks/knowledgecli/knowledge.go`의 한 청크는 Ollama가 12,205바이트 원문 임베딩을 거부해 CKV의 복구 경로가 4,000바이트로 줄여 임베딩했다. [`embedResilient`](../../internal/vector/build/builder.go)은 저장된 청크 본문을 유지하면서 벡터만 짧은 입력에서 생성한다. 이는 API 요청의 `truncate:false`와 별개인 **애플리케이션 복구 동작**이다. 긴 문서 끝부분의 검색 누락 가능성과 축약 건수의 미계측을 B0의 관측 위험으로 등록한다. 완전한 기준선에서는 축약·건너뛴 청크 수를 수집해 보고하고, F-02 긴 문서 끝 사례로 회수 영향과 C0의 분할/표시 개선을 판단한다. 이 한 건만으로 전체 검색 품질을 단정하지 않는다.
